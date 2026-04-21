package handler

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/nullne/multica/server/internal/logger"
	db "github.com/nullne/multica/server/pkg/db/generated"
)

// BotUserResponse describes a bot user — a non-human "member" used as the
// author for system-driven comments (e.g. webhook triggered comments).
type BotUserResponse struct {
	ID        string  `json:"id"`
	Name      string  `json:"name"`
	Email     string  `json:"email"`
	AvatarURL *string `json:"avatar_url"`
	Kind      string  `json:"kind"`
	CreatedAt string  `json:"created_at"`
}

func botUserToResponse(u db.User) BotUserResponse {
	return BotUserResponse{
		ID:        uuidToString(u.ID),
		Name:      u.Name,
		Email:     u.Email,
		AvatarURL: textToPtr(u.AvatarUrl),
		Kind:      u.Kind,
		CreatedAt: timestampToString(u.CreatedAt),
	}
}

func (h *Handler) ListBotUsers(w http.ResponseWriter, r *http.Request) {
	workspaceID := workspaceIDFromURL(r, "id")

	bots, err := h.Queries.ListBotsInWorkspace(r.Context(), parseUUID(workspaceID))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list bot users")
		return
	}

	resp := make([]BotUserResponse, len(bots))
	for i, b := range bots {
		resp[i] = botUserToResponse(b)
	}
	writeJSON(w, http.StatusOK, resp)
}

type CreateBotUserRequest struct {
	Name      string  `json:"name"`
	AvatarURL *string `json:"avatar_url"`
}

// CreateBotUser provisions a new bot user and adds it as a member of the
// workspace. Bot users have a synthetic email derived from a random suffix
// to satisfy the unique constraint on user.email — they never log in.
func (h *Handler) CreateBotUser(w http.ResponseWriter, r *http.Request) {
	workspaceID := workspaceIDFromURL(r, "id")

	var req CreateBotUserRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	req.Name = strings.TrimSpace(req.Name)
	if req.Name == "" {
		writeError(w, http.StatusBadRequest, "name is required")
		return
	}

	suffix, err := randomHex(8)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to generate bot id")
		return
	}
	email := fmt.Sprintf("bot+%s@multica.local", suffix)

	tx, err := h.TxStarter.Begin(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to start transaction")
		return
	}
	defer tx.Rollback(r.Context())

	qtx := h.Queries.WithTx(tx)

	user, err := qtx.CreateUser(r.Context(), db.CreateUserParams{
		Name:      req.Name,
		Email:     email,
		AvatarUrl: ptrToText(req.AvatarURL),
		Kind:      "bot",
	})
	if err != nil {
		slog.Warn("create bot user failed", append(logger.RequestAttrs(r), "error", err)...)
		writeError(w, http.StatusInternalServerError, "failed to create bot user")
		return
	}

	if _, err := qtx.CreateMember(r.Context(), db.CreateMemberParams{
		WorkspaceID: parseUUID(workspaceID),
		UserID:      user.ID,
		Role:        "member",
	}); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to add bot to workspace")
		return
	}

	if err := tx.Commit(r.Context()); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to commit transaction")
		return
	}

	writeJSON(w, http.StatusCreated, botUserToResponse(user))
}

// DeleteBotUser removes a bot user. Webhook actions referencing this bot
// (via webhook.bot_user_id) will have the column set to NULL automatically
// (ON DELETE SET NULL), but their comment_issue actions will fail validation
// until a new bot is bound.
func (h *Handler) DeleteBotUser(w http.ResponseWriter, r *http.Request) {
	workspaceID := workspaceIDFromURL(r, "id")
	userID := chi.URLParam(r, "userId")

	user, err := h.Queries.GetUser(r.Context(), parseUUID(userID))
	if err != nil {
		writeError(w, http.StatusNotFound, "bot user not found")
		return
	}
	if user.Kind != "bot" {
		writeError(w, http.StatusBadRequest, "user is not a bot")
		return
	}

	// Confirm the bot is a member of this workspace before deletion.
	if _, err := h.Queries.GetMemberByUserAndWorkspace(r.Context(), db.GetMemberByUserAndWorkspaceParams{
		UserID:      pgtype.UUID{Bytes: user.ID.Bytes, Valid: true},
		WorkspaceID: parseUUID(workspaceID),
	}); err != nil {
		writeError(w, http.StatusNotFound, "bot user not in this workspace")
		return
	}

	// CASCADE: deleting the user removes the member row and sets
	// webhook.bot_user_id to NULL on the partial-FK column.
	if err := h.Queries.DeleteUser(r.Context(), user.ID); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to delete bot user")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func randomHex(byteLen int) (string, error) {
	b := make([]byte, byteLen)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
