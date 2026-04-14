package handler

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/nullne/multica/server/internal/auth"
	"github.com/nullne/multica/server/internal/logger"
	db "github.com/nullne/multica/server/pkg/db/generated"
)

type UserResponse struct {
	ID        string  `json:"id"`
	Name      string  `json:"name"`
	Email     string  `json:"email"`
	AvatarURL *string `json:"avatar_url"`
	CreatedAt string  `json:"created_at"`
	UpdatedAt string  `json:"updated_at"`
}

func userToResponse(u db.User) UserResponse {
	return UserResponse{
		ID:        uuidToString(u.ID),
		Name:      u.Name,
		Email:     u.Email,
		AvatarURL: textToPtr(u.AvatarUrl),
		CreatedAt: timestampToString(u.CreatedAt),
		UpdatedAt: timestampToString(u.UpdatedAt),
	}
}

type LoginResponse struct {
	Token string       `json:"token"`
	User  UserResponse `json:"user"`
}

type SendCodeRequest struct {
	Email string `json:"email"`
}

type VerifyCodeRequest struct {
	Email string `json:"email"`
	Code  string `json:"code"`
}

func defaultWorkspaceName(user db.User) string {
	name := strings.TrimSpace(user.Name)
	if name == "" {
		email := strings.TrimSpace(user.Email)
		if at := strings.Index(email, "@"); at > 0 {
			name = email[:at]
		}
	}
	if name == "" {
		name = "Personal"
	}
	return name + "'s Workspace"
}

func slugifyWorkspacePart(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	var b strings.Builder
	lastWasDash := false

	for _, r := range value {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
			lastWasDash = false
		case b.Len() > 0 && !lastWasDash:
			b.WriteByte('-')
			lastWasDash = true
		}
	}

	return strings.Trim(b.String(), "-")
}

func defaultWorkspaceSlug(user db.User) string {
	candidates := []string{
		slugifyWorkspacePart(user.Name),
		slugifyWorkspacePart(strings.Split(strings.TrimSpace(user.Email), "@")[0]),
		"workspace",
	}

	base := "workspace"
	for _, candidate := range candidates {
		if candidate != "" {
			base = candidate
			break
		}
	}

	userID := uuidToString(user.ID)
	if len(userID) >= 8 {
		return base + "-" + userID[:8]
	}
	return base
}

func (h *Handler) ensureUserWorkspace(ctx context.Context, user db.User) error {
	workspaces, err := h.Queries.ListWorkspaces(ctx, user.ID)
	if err != nil {
		return err
	}
	if len(workspaces) > 0 {
		return nil
	}

	tx, err := h.TxStarter.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	qtx := h.Queries.WithTx(tx)
	workspaces, err = qtx.ListWorkspaces(ctx, user.ID)
	if err != nil {
		return err
	}
	if len(workspaces) > 0 {
		return nil
	}

	wsName := defaultWorkspaceName(user)
	workspace, err := qtx.CreateWorkspace(ctx, db.CreateWorkspaceParams{
		Name:        wsName,
		Slug:        defaultWorkspaceSlug(user),
		Description: pgtype.Text{},
		IssuePrefix: generateIssuePrefix(wsName),
	})
	if err != nil {
		if isUniqueViolation(err) {
			workspaces, lookupErr := h.Queries.ListWorkspaces(ctx, user.ID)
			if lookupErr == nil && len(workspaces) > 0 {
				return nil
			}
		}
		return err
	}

	if _, err := qtx.CreateMember(ctx, db.CreateMemberParams{
		WorkspaceID: workspace.ID,
		UserID:      user.ID,
		Role:        "owner",
	}); err != nil {
		return err
	}

	if _, err := qtx.UpdateWorkspace(ctx, updateSettingsOnly(workspace.ID, defaultProviderSettings())); err != nil {
		return err
	}

	return tx.Commit(ctx)
}

func generateCode() (string, error) {
	var buf [4]byte
	if _, err := rand.Read(buf[:]); err != nil {
		return "", err
	}
	n := binary.BigEndian.Uint32(buf[:]) % 1000000
	return fmt.Sprintf("%06d", n), nil
}

func (h *Handler) issueJWT(user db.User) (string, error) {
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub":   uuidToString(user.ID),
		"email": user.Email,
		"name":  user.Name,
		"exp":   time.Now().Add(72 * time.Hour).Unix(),
		"iat":   time.Now().Unix(),
	})
	return token.SignedString(auth.JWTSecret())
}

func (h *Handler) findOrCreateUser(ctx context.Context, email string) (db.User, error) {
	user, err := h.Queries.GetUserByEmail(ctx, email)
	if err != nil {
		if !isNotFound(err) {
			return db.User{}, err
		}
		name := email
		if at := strings.Index(email, "@"); at > 0 {
			name = email[:at]
		}
		user, err = h.Queries.CreateUser(ctx, db.CreateUserParams{
			Name:  name,
			Email: email,
		})
		if err != nil {
			return db.User{}, err
		}
	}
	return user, nil
}

func isEmailAllowed(email string) bool {
	allowedEmails := os.Getenv("ALLOWED_EMAILS")
	allowedDomains := os.Getenv("ALLOWED_EMAIL_DOMAINS")

	if allowedEmails == "" && allowedDomains == "" {
		return true
	}

	if allowedEmails != "" {
		for _, e := range strings.Split(allowedEmails, ",") {
			if strings.TrimSpace(e) == email {
				return true
			}
		}
	}

	if allowedDomains != "" {
		if at := strings.LastIndex(email, "@"); at >= 0 {
			domain := email[at+1:]
			for _, d := range strings.Split(allowedDomains, ",") {
				if strings.TrimSpace(d) == domain {
					return true
				}
			}
		}
	}

	return false
}

func (h *Handler) SendCode(w http.ResponseWriter, r *http.Request) {
	var req SendCodeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	email := strings.ToLower(strings.TrimSpace(req.Email))
	if email == "" {
		writeError(w, http.StatusBadRequest, "email is required")
		return
	}

	if !isEmailAllowed(email) {
		writeError(w, http.StatusForbidden, "email not allowed")
		return
	}

	// Rate limit: max 1 code per 10 seconds per email
	latest, err := h.Queries.GetLatestCodeByEmail(r.Context(), email)
	if err == nil && time.Since(latest.CreatedAt.Time) < 10*time.Second {
		writeError(w, http.StatusTooManyRequests, "please wait before requesting another code")
		return
	}

	code, err := generateCode()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to generate code")
		return
	}

	_, err = h.Queries.CreateVerificationCode(r.Context(), db.CreateVerificationCodeParams{
		Email:     email,
		Code:      code,
		ExpiresAt: pgtype.Timestamptz{Time: time.Now().Add(10 * time.Minute), Valid: true},
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to store verification code")
		return
	}

	if err := h.EmailService.SendVerificationCode(email, code); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to send verification code")
		return
	}

	// Best-effort cleanup of expired codes
	_ = h.Queries.DeleteExpiredVerificationCodes(r.Context())

	writeJSON(w, http.StatusOK, map[string]string{"message": "Verification code sent"})
}

func (h *Handler) VerifyCode(w http.ResponseWriter, r *http.Request) {
	var req VerifyCodeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	email := strings.ToLower(strings.TrimSpace(req.Email))
	code := strings.TrimSpace(req.Code)

	if email == "" || code == "" {
		writeError(w, http.StatusBadRequest, "email and code are required")
		return
	}

	dbCode, err := h.Queries.GetLatestVerificationCode(r.Context(), email)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid or expired code")
		return
	}

	isMasterCode := code == "888888" && os.Getenv("APP_ENV") != "production"
	if !isMasterCode && subtle.ConstantTimeCompare([]byte(code), []byte(dbCode.Code)) != 1 {
		_ = h.Queries.IncrementVerificationCodeAttempts(r.Context(), dbCode.ID)
		writeError(w, http.StatusBadRequest, "invalid or expired code")
		return
	}

	if err := h.Queries.MarkVerificationCodeUsed(r.Context(), dbCode.ID); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to verify code")
		return
	}

	user, err := h.findOrCreateUser(r.Context(), email)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create user")
		return
	}

	if err := h.ensureUserWorkspace(r.Context(), user); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to provision workspace")
		return
	}

	tokenString, err := h.issueJWT(user)
	if err != nil {
		slog.Warn("login failed", append(logger.RequestAttrs(r), "error", err, "email", req.Email)...)
		writeError(w, http.StatusInternalServerError, "failed to generate token")
		return
	}

	// Set CloudFront signed cookies for CDN access.
	if h.CFSigner != nil {
		for _, cookie := range h.CFSigner.SignedCookies(time.Now().Add(72 * time.Hour)) {
			http.SetCookie(w, cookie)
		}
	}

	slog.Info("user logged in", append(logger.RequestAttrs(r), "user_id", uuidToString(user.ID), "email", user.Email)...)
	writeJSON(w, http.StatusOK, LoginResponse{
		Token: tokenString,
		User:  userToResponse(user),
	})
}

func (h *Handler) GetMe(w http.ResponseWriter, r *http.Request) {
	userID, ok := requireUserID(w, r)
	if !ok {
		return
	}

	user, err := h.Queries.GetUser(r.Context(), parseUUID(userID))
	if err != nil {
		writeError(w, http.StatusNotFound, "user not found")
		return
	}

	writeJSON(w, http.StatusOK, userToResponse(user))
}

type UpdateMeRequest struct {
	Name      *string `json:"name"`
	AvatarURL *string `json:"avatar_url"`
}

func (h *Handler) UpdateMe(w http.ResponseWriter, r *http.Request) {
	userID, ok := requireUserID(w, r)
	if !ok {
		return
	}

	var req UpdateMeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	currentUser, err := h.Queries.GetUser(r.Context(), parseUUID(userID))
	if err != nil {
		writeError(w, http.StatusNotFound, "user not found")
		return
	}

	name := currentUser.Name
	if req.Name != nil {
		name = strings.TrimSpace(*req.Name)
		if name == "" {
			writeError(w, http.StatusBadRequest, "name is required")
			return
		}
	}

	params := db.UpdateUserParams{
		ID:   currentUser.ID,
		Name: name,
	}
	if req.AvatarURL != nil {
		params.AvatarUrl = pgtype.Text{String: strings.TrimSpace(*req.AvatarURL), Valid: true}
	}

	updatedUser, err := h.Queries.UpdateUser(r.Context(), params)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to update user")
		return
	}

	writeJSON(w, http.StatusOK, userToResponse(updatedUser))
}

type OAuthCallbackRequest struct {
	Code        string `json:"code"`
	RedirectURI string `json:"redirect_uri"`
}

type oauthTokenResponse struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	ExpiresIn   int    `json:"expires_in"`
}

type oauthUserInfo struct {
	Sub     string `json:"sub"`
	Email   string `json:"email"`
	Name    string `json:"name"`
	Picture string `json:"picture"`
}

func (h *Handler) OAuthCallback(w http.ResponseWriter, r *http.Request) {
	gatewayURL := os.Getenv("AUTH_GATEWAY_URL")
	clientID := os.Getenv("AUTH_OAUTH_CLIENT_ID")
	clientSecret := os.Getenv("AUTH_OAUTH_CLIENT_SECRET")

	if gatewayURL == "" || clientID == "" {
		writeError(w, http.StatusServiceUnavailable, "OAuth is not configured")
		return
	}

	var req OAuthCallbackRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Code == "" || req.RedirectURI == "" {
		writeError(w, http.StatusBadRequest, "code and redirect_uri are required")
		return
	}

	// Exchange authorization code for access token.
	tokenParams := url.Values{
		"grant_type":   {"authorization_code"},
		"code":         {req.Code},
		"redirect_uri": {req.RedirectURI},
		"client_id":    {clientID},
	}
	if clientSecret != "" {
		tokenParams.Set("client_secret", clientSecret)
	}

	tokenResp, err := http.Post(
		gatewayURL+"/api/oauth/token",
		"application/x-www-form-urlencoded",
		bytes.NewBufferString(tokenParams.Encode()),
	)
	if err != nil {
		slog.Error("oauth: failed to exchange code", "error", err)
		writeError(w, http.StatusBadGateway, "failed to contact auth gateway")
		return
	}
	defer tokenResp.Body.Close()

	tokenBody, _ := io.ReadAll(tokenResp.Body)
	if tokenResp.StatusCode != http.StatusOK {
		slog.Warn("oauth: token exchange failed", "status", tokenResp.StatusCode, "body", string(tokenBody))
		writeError(w, http.StatusUnauthorized, "authorization code exchange failed")
		return
	}

	var tokenData oauthTokenResponse
	if err := json.Unmarshal(tokenBody, &tokenData); err != nil {
		writeError(w, http.StatusBadGateway, "invalid token response from auth gateway")
		return
	}

	// Fetch user info from auth gateway.
	userInfoReq, _ := http.NewRequestWithContext(r.Context(), http.MethodGet, gatewayURL+"/api/oauth/userinfo", nil)
	userInfoReq.Header.Set("Authorization", "Bearer "+tokenData.AccessToken)

	userInfoResp, err := http.DefaultClient.Do(userInfoReq)
	if err != nil {
		slog.Error("oauth: failed to fetch userinfo", "error", err)
		writeError(w, http.StatusBadGateway, "failed to fetch user info from auth gateway")
		return
	}
	defer userInfoResp.Body.Close()

	if userInfoResp.StatusCode != http.StatusOK {
		writeError(w, http.StatusUnauthorized, "failed to verify user with auth gateway")
		return
	}

	var userInfo oauthUserInfo
	if err := json.NewDecoder(userInfoResp.Body).Decode(&userInfo); err != nil {
		writeError(w, http.StatusBadGateway, "invalid userinfo response from auth gateway")
		return
	}

	if userInfo.Email == "" {
		writeError(w, http.StatusBadRequest, "auth gateway did not return an email")
		return
	}

	email := strings.ToLower(strings.TrimSpace(userInfo.Email))

	user, err := h.findOrCreateUser(r.Context(), email)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create user")
		return
	}

	// Update name/avatar if the user was just created with a default name.
	if userInfo.Name != "" && user.Name == strings.Split(email, "@")[0] {
		params := db.UpdateUserParams{ID: user.ID, Name: userInfo.Name}
		if userInfo.Picture != "" {
			params.AvatarUrl = pgtype.Text{String: userInfo.Picture, Valid: true}
		}
		if updated, err := h.Queries.UpdateUser(r.Context(), params); err == nil {
			user = updated
		}
	}

	if err := h.ensureUserWorkspace(r.Context(), user); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to provision workspace")
		return
	}

	tokenString, err := h.issueJWT(user)
	if err != nil {
		slog.Warn("oauth login failed", append(logger.RequestAttrs(r), "error", err, "email", email)...)
		writeError(w, http.StatusInternalServerError, "failed to generate token")
		return
	}

	if h.CFSigner != nil {
		for _, cookie := range h.CFSigner.SignedCookies(time.Now().Add(72 * time.Hour)) {
			http.SetCookie(w, cookie)
		}
	}

	slog.Info("user logged in via oauth", append(logger.RequestAttrs(r), "user_id", uuidToString(user.ID), "email", user.Email)...)
	writeJSON(w, http.StatusOK, LoginResponse{
		Token: tokenString,
		User:  userToResponse(user),
	})
}
