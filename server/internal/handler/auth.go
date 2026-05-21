package handler

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/nullne/multica/server/internal/auth"
	"github.com/nullne/multica/server/internal/logger"
	"github.com/nullne/multica/server/internal/middleware"
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

type FirebaseLoginRequest struct {
	IDToken string `json:"id_token"`
}

type DevLoginRequest struct {
	Email string `json:"email"`
	Name  string `json:"name"`
}

// devAuthBypassEnabled reports whether the DEV_AUTH_BYPASS env flag is set to a
// truthy value. The dev login endpoint is only accepted when this is true so
// production builds can never use it.
func devAuthBypassEnabled() bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv("DEV_AUTH_BYPASS"))) {
	case "1", "true", "yes", "on":
		return true
	}
	return false
}

func setAuthCookie(w http.ResponseWriter, r *http.Request, token string) {
	http.SetCookie(w, &http.Cookie{
		Name:     middleware.AuthCookieName,
		Value:    token,
		Path:     "/api/attachments/",
		MaxAge:   int((72 * time.Hour).Seconds()),
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   r.TLS != nil || strings.EqualFold(r.Header.Get("X-Forwarded-Proto"), "https") || os.Getenv("APP_ENV") == "production",
	})
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

func (h *Handler) findOrCreateUser(ctx context.Context, email, name string, avatarURL *string) (db.User, error) {
	user, err := h.Queries.GetUserByEmail(ctx, email)
	if err != nil {
		if !isNotFound(err) {
			return db.User{}, err
		}
		displayName := strings.TrimSpace(name)
		if displayName == "" {
			displayName = email
			if at := strings.Index(email, "@"); at > 0 {
				displayName = email[:at]
			}
		}
		user, err = h.Queries.CreateUser(ctx, db.CreateUserParams{
			Name:      displayName,
			Email:     email,
			AvatarUrl: ptrToText(avatarURL),
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

func (h *Handler) LoginWithFirebase(w http.ResponseWriter, r *http.Request) {
	if h.FirebaseVerifier == nil {
		writeError(w, http.StatusServiceUnavailable, "firebase auth is not configured")
		return
	}

	var req FirebaseLoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	idToken := strings.TrimSpace(req.IDToken)
	if idToken == "" {
		writeError(w, http.StatusBadRequest, "id_token is required")
		return
	}

	identity, err := h.FirebaseVerifier.VerifyIDToken(r.Context(), idToken)
	if err != nil {
		slog.Warn("firebase login rejected", append(logger.RequestAttrs(r), "error", err)...)
		writeError(w, http.StatusUnauthorized, "invalid firebase token")
		return
	}

	email := strings.ToLower(strings.TrimSpace(identity.Email))
	if !isEmailAllowed(email) {
		// An invited member may not be on the env allowlist but already has a
		// member record from the workspace invite flow. Treat that as proof of
		// invitation.
		isMember, memberErr := h.Queries.IsWorkspaceMemberByEmail(r.Context(), email)
		if memberErr != nil {
			slog.Warn("firebase login membership check failed", append(logger.RequestAttrs(r), "error", memberErr, "email", email)...)
			writeError(w, http.StatusInternalServerError, "failed to verify membership")
			return
		}
		if !isMember {
			writeError(w, http.StatusForbidden, "email not allowed")
			return
		}
	}

	user, err := h.findOrCreateUser(r.Context(), email, identity.Name, identity.PictureURL)
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
		slog.Warn("login failed", append(logger.RequestAttrs(r), "error", err, "email", email)...)
		writeError(w, http.StatusInternalServerError, "failed to generate token")
		return
	}

	// Set CloudFront signed cookies for CDN access.
	if h.CFSigner != nil {
		for _, cookie := range h.CFSigner.SignedCookies(time.Now().Add(72 * time.Hour)) {
			http.SetCookie(w, cookie)
		}
	}
	setAuthCookie(w, r, tokenString)

	slog.Info("user logged in", append(logger.RequestAttrs(r), "user_id", uuidToString(user.ID), "email", user.Email)...)
	writeJSON(w, http.StatusOK, LoginResponse{
		Token: tokenString,
		User:  userToResponse(user),
	})
}

// LoginDev issues a JWT for an arbitrary email/name without verifying any
// identity provider. It is gated by the DEV_AUTH_BYPASS env flag so it cannot
// be enabled in production. Used by the docker-compose dev seed script and the
// E2E test suite to bootstrap a local user without provisioning Firebase.
func (h *Handler) LoginDev(w http.ResponseWriter, r *http.Request) {
	if !devAuthBypassEnabled() {
		writeError(w, http.StatusNotFound, "not found")
		return
	}

	var req DevLoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	email := strings.ToLower(strings.TrimSpace(req.Email))
	if email == "" {
		writeError(w, http.StatusBadRequest, "email is required")
		return
	}

	user, err := h.findOrCreateUser(r.Context(), email, strings.TrimSpace(req.Name), nil)
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
		slog.Warn("dev login failed", append(logger.RequestAttrs(r), "error", err, "email", email)...)
		writeError(w, http.StatusInternalServerError, "failed to generate token")
		return
	}

	setAuthCookie(w, r, tokenString)
	slog.Info("dev login", append(logger.RequestAttrs(r), "user_id", uuidToString(user.ID), "email", user.Email)...)
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
