package handler

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"os"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/nullne/multica/server/internal/logger"
	db "github.com/nullne/multica/server/pkg/db/generated"
)

const maxUploadSize = 100 << 20 // 100 MB

// ---------------------------------------------------------------------------
// Response types
// ---------------------------------------------------------------------------

type AttachmentResponse struct {
	ID           string  `json:"id"`
	WorkspaceID  string  `json:"workspace_id"`
	IssueID      *string `json:"issue_id"`
	CommentID    *string `json:"comment_id"`
	UploaderType string  `json:"uploader_type"`
	UploaderID   string  `json:"uploader_id"`
	Filename     string  `json:"filename"`
	URL          string  `json:"url"`
	DownloadURL  string  `json:"download_url"`
	ContentType  string  `json:"content_type"`
	SizeBytes    int64   `json:"size_bytes"`
	CreatedAt    string  `json:"created_at"`
}

func (h *Handler) attachmentToResponse(r *http.Request, a db.Attachment) AttachmentResponse {
	resp := AttachmentResponse{
		ID:           uuidToString(a.ID),
		WorkspaceID:  uuidToString(a.WorkspaceID),
		UploaderType: a.UploaderType,
		UploaderID:   uuidToString(a.UploaderID),
		Filename:     a.Filename,
		URL:          a.Url,
		DownloadURL:  h.attachmentDownloadURL(r, a),
		ContentType:  a.ContentType,
		SizeBytes:    a.SizeBytes,
		CreatedAt:    a.CreatedAt.Time.Format("2006-01-02T15:04:05Z07:00"),
	}
	if a.IssueID.Valid {
		s := uuidToString(a.IssueID)
		resp.IssueID = &s
	}
	if a.CommentID.Valid {
		s := uuidToString(a.CommentID)
		resp.CommentID = &s
	}
	return resp
}

// attachmentDownloadURL returns a URL that any authenticated workspace member
// can fetch the attachment from. When CloudFront signing is configured we
// generate a short-lived signed URL pointing at the CDN (matches the browser
// cookie path). Otherwise we route the download through the app at
// /api/attachments/{id}/download — the server proxies the object from the
// underlying bucket using its own credentials, so private GCS / S3 buckets
// don't need to be world-readable.
func (h *Handler) attachmentDownloadURL(r *http.Request, a db.Attachment) string {
	if h.CFSigner != nil {
		return h.CFSigner.SignedURL(a.Url, time.Now().Add(5*time.Minute))
	}
	id := uuidToString(a.ID)
	if id == "" {
		return a.Url
	}
	return appBaseURL(r) + "/api/attachments/" + id + "/download"
}

// appBaseURL derives "scheme://host" from the incoming request so we can
// build absolute URLs that clients (browsers, the CLI, agents) can fetch
// directly without knowing the base ahead of time.
func appBaseURL(r *http.Request) string {
	if r == nil {
		return ""
	}
	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}
	if proto := r.Header.Get("X-Forwarded-Proto"); proto != "" {
		scheme = firstForwardedValue(proto)
	}
	host := r.Host
	if fwd := r.Header.Get("X-Forwarded-Host"); fwd != "" {
		host = firstForwardedValue(fwd)
	}
	if host == "" {
		return ""
	}
	if scheme == "http" && os.Getenv("APP_ENV") == "production" && !isLocalHost(host) {
		scheme = "https"
	}
	return scheme + "://" + host
}

func firstForwardedValue(value string) string {
	if i := strings.IndexByte(value, ','); i >= 0 {
		value = value[:i]
	}
	return strings.TrimSpace(value)
}

func isLocalHost(host string) bool {
	host = strings.ToLower(host)
	if h, _, err := net.SplitHostPort(host); err == nil {
		host = h
	}
	host = strings.Trim(host, "[]")
	return host == "localhost" || host == "127.0.0.1" || host == "::1"
}

func attachmentObjectKey(workspaceID, userID, filename, randomHex string, now time.Time) string {
	now = now.UTC()
	parts := []string{}
	if workspaceID != "" {
		parts = append(parts, "workspaces", workspaceID)
	}
	if userID != "" {
		parts = append(parts, "users", userID)
	}
	parts = append(parts,
		"attachments",
		fmt.Sprintf("%04d", now.Year()),
		fmt.Sprintf("%02d", int(now.Month())),
		randomHex+path.Ext(filename),
	)
	return path.Join(parts...)
}

// groupAttachments loads attachments for multiple comments and groups them by comment ID.
func (h *Handler) groupAttachments(r *http.Request, commentIDs []pgtype.UUID) map[string][]AttachmentResponse {
	if len(commentIDs) == 0 {
		return nil
	}
	attachments, err := h.Queries.ListAttachmentsByCommentIDs(r.Context(), commentIDs)
	if err != nil {
		slog.Error("failed to load attachments for comments", "error", err)
		return nil
	}
	grouped := make(map[string][]AttachmentResponse, len(commentIDs))
	for _, a := range attachments {
		cid := uuidToString(a.CommentID)
		grouped[cid] = append(grouped[cid], h.attachmentToResponse(r, a))
	}
	return grouped
}

// ---------------------------------------------------------------------------
// UploadFile — POST /api/upload-file
// ---------------------------------------------------------------------------

func (h *Handler) UploadFile(w http.ResponseWriter, r *http.Request) {
	if h.Storage == nil {
		writeError(w, http.StatusServiceUnavailable, "file upload not configured")
		return
	}

	userID, ok := requireUserID(w, r)
	if !ok {
		return
	}

	workspaceID := resolveWorkspaceID(r)

	r.Body = http.MaxBytesReader(w, r.Body, maxUploadSize)

	if err := r.ParseMultipartForm(maxUploadSize); err != nil {
		writeError(w, http.StatusBadRequest, "file too large or invalid multipart form")
		return
	}
	defer r.MultipartForm.RemoveAll()

	file, header, err := r.FormFile("file")
	if err != nil {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("missing file field: %v", err))
		return
	}
	defer file.Close()

	// Sniff actual content type from file bytes instead of trusting the client header.
	buf := make([]byte, 512)
	n, err := file.Read(buf)
	if err != nil && err != io.EOF {
		writeError(w, http.StatusBadRequest, "failed to read file")
		return
	}
	contentType := http.DetectContentType(buf[:n])
	// Seek back so the full file is uploaded.
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to read file")
		return
	}

	data, err := io.ReadAll(file)
	if err != nil {
		writeError(w, http.StatusBadRequest, "failed to read file")
		return
	}

	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		slog.Error("failed to generate file key", "error", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	key := attachmentObjectKey(workspaceID, userID, header.Filename, hex.EncodeToString(b), time.Now())

	link, err := h.Storage.Upload(r.Context(), key, data, contentType, header.Filename)
	if err != nil {
		slog.Error("file upload failed", append(logger.RequestAttrs(r),
			"error", err,
			"workspace_id", workspaceID,
			"filename", header.Filename,
			"content_type", contentType,
			"size_bytes", len(data),
		)...)
		writeError(w, http.StatusInternalServerError, "upload failed")
		return
	}

	// If workspace context is available, create an attachment record.
	if workspaceID != "" {
		uploaderType, uploaderID := h.resolveActor(r, userID, workspaceID)

		params := db.CreateAttachmentParams{
			WorkspaceID:  parseUUID(workspaceID),
			UploaderType: uploaderType,
			UploaderID:   parseUUID(uploaderID),
			Filename:     header.Filename,
			Url:          link,
			ContentType:  contentType,
			SizeBytes:    int64(len(data)),
		}

		// Optional issue_id / comment_id from form fields
		if issueID := r.FormValue("issue_id"); issueID != "" {
			params.IssueID = parseUUID(issueID)
		}
		if commentID := r.FormValue("comment_id"); commentID != "" {
			params.CommentID = parseUUID(commentID)
		}

		att, err := h.Queries.CreateAttachment(r.Context(), params)
		if err != nil {
			slog.Error("failed to create attachment record", append(logger.RequestAttrs(r),
				"error", err,
				"workspace_id", workspaceID,
				"issue_id", r.FormValue("issue_id"),
				"comment_id", r.FormValue("comment_id"),
				"filename", header.Filename,
			)...)
			// S3 upload succeeded but DB record failed — still return the link
			// so the file is usable. Log the error for investigation.
		} else {
			writeJSON(w, http.StatusOK, h.attachmentToResponse(r, att))
			return
		}
	}

	// Fallback response (no workspace context, e.g. avatar upload)
	writeJSON(w, http.StatusOK, map[string]string{
		"filename": header.Filename,
		"link":     link,
	})
}

// ---------------------------------------------------------------------------
// ListAttachments — GET /api/issues/{id}/attachments
// ---------------------------------------------------------------------------

func (h *Handler) ListAttachments(w http.ResponseWriter, r *http.Request) {
	issueID := chi.URLParam(r, "id")
	issue, ok := h.loadIssueForUser(w, r, issueID)
	if !ok {
		return
	}

	attachments, err := h.Queries.ListAttachmentsByIssue(r.Context(), db.ListAttachmentsByIssueParams{
		IssueID:     issue.ID,
		WorkspaceID: issue.WorkspaceID,
	})
	if err != nil {
		slog.Error("failed to list attachments", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to list attachments")
		return
	}

	resp := make([]AttachmentResponse, len(attachments))
	for i, a := range attachments {
		resp[i] = h.attachmentToResponse(r, a)
	}
	writeJSON(w, http.StatusOK, resp)
}

// ---------------------------------------------------------------------------
// DownloadAttachment — GET /api/attachments/{id}/download
// ---------------------------------------------------------------------------

// DownloadAttachment streams an attachment to an authenticated caller. It
// resolves the workspace from the attachment record, verifies membership, and
// proxies the object bytes from the storage backend using the server's own
// credentials — so private GCS / S3 buckets don't have to be publicly readable.
//
// When CloudFront signing is configured, the handler instead issues a 302 to a
// short-lived signed URL (cheaper than streaming, matches the browser cookie
// path).
func (h *Handler) DownloadAttachment(w http.ResponseWriter, r *http.Request) {
	userID, ok := requireUserID(w, r)
	if !ok {
		return
	}
	attachmentID := chi.URLParam(r, "id")

	att, err := h.Queries.GetAttachmentByID(r.Context(), parseUUID(attachmentID))
	if err != nil {
		writeError(w, http.StatusNotFound, "attachment not found")
		return
	}

	workspaceID := uuidToString(att.WorkspaceID)
	if _, err := h.getWorkspaceMember(r.Context(), userID, workspaceID); err != nil {
		writeError(w, http.StatusNotFound, "attachment not found")
		return
	}

	// CloudFront-signed deployments: redirect to the CDN with a signed URL.
	if h.CFSigner != nil {
		signed := h.CFSigner.SignedURL(att.Url, time.Now().Add(5*time.Minute))
		http.Redirect(w, r, signed, http.StatusFound)
		return
	}

	if h.Storage == nil {
		writeError(w, http.StatusServiceUnavailable, "file storage not configured")
		return
	}

	key := h.Storage.KeyFromURL(att.Url)
	obj, err := h.Storage.Download(r.Context(), key)
	if err != nil {
		slog.Error("failed to download attachment", append(logger.RequestAttrs(r),
			"error", err,
			"attachment_id", attachmentID,
			"key", key,
		)...)
		writeError(w, http.StatusBadGateway, "failed to fetch attachment")
		return
	}
	defer obj.Body.Close()

	contentType := obj.ContentType
	if contentType == "" {
		contentType = att.ContentType
	}
	if contentType != "" {
		w.Header().Set("Content-Type", contentType)
	}
	contentLength := obj.ContentLength
	if contentLength <= 0 {
		contentLength = att.SizeBytes
	}
	if contentLength > 0 {
		w.Header().Set("Content-Length", strconv.FormatInt(contentLength, 10))
	}
	w.Header().Set("Content-Disposition", fmt.Sprintf(`inline; filename="%s"; filename*=UTF-8''%s`,
		safeHeaderFilename(att.Filename),
		url.PathEscape(att.Filename),
	))
	w.Header().Set("Cache-Control", "private, max-age=60")
	if _, err := io.Copy(w, obj.Body); err != nil {
		slog.Warn("attachment stream interrupted", "error", err, "attachment_id", attachmentID)
	}
}

// safeHeaderFilename strips characters that could break Content-Disposition.
func safeHeaderFilename(name string) string {
	out := make([]byte, 0, len(name))
	for i := 0; i < len(name); i++ {
		c := name[i]
		if c < 0x20 || c == 0x7f || c == '"' || c == ';' || c == '\\' {
			out = append(out, '_')
			continue
		}
		out = append(out, c)
	}
	return string(out)
}

// ---------------------------------------------------------------------------
// DeleteAttachment — DELETE /api/attachments/{id}
// ---------------------------------------------------------------------------

func (h *Handler) DeleteAttachment(w http.ResponseWriter, r *http.Request) {
	attachmentID := chi.URLParam(r, "id")
	workspaceID := resolveWorkspaceID(r)
	if workspaceID == "" {
		writeError(w, http.StatusBadRequest, "workspace_id is required")
		return
	}

	userID, ok := requireUserID(w, r)
	if !ok {
		return
	}

	att, err := h.Queries.GetAttachment(r.Context(), db.GetAttachmentParams{
		ID:          parseUUID(attachmentID),
		WorkspaceID: parseUUID(workspaceID),
	})
	if err != nil {
		writeError(w, http.StatusNotFound, "attachment not found")
		return
	}

	// Only the uploader (or workspace admin) can delete
	uploaderID := uuidToString(att.UploaderID)
	isUploader := att.UploaderType == "member" && uploaderID == userID
	member, hasMember := ctxMember(r.Context())
	isAdmin := hasMember && (member.Role == "admin" || member.Role == "owner")

	if !isUploader && !isAdmin {
		writeError(w, http.StatusForbidden, "not authorized to delete this attachment")
		return
	}

	if err := h.Queries.DeleteAttachment(r.Context(), db.DeleteAttachmentParams{
		ID:          att.ID,
		WorkspaceID: att.WorkspaceID,
	}); err != nil {
		slog.Error("failed to delete attachment", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to delete attachment")
		return
	}

	h.deleteS3Object(r.Context(), att.Url)
	w.WriteHeader(http.StatusNoContent)
}

// ---------------------------------------------------------------------------
// Attachment linking
// ---------------------------------------------------------------------------

// linkAttachmentsByIDs links the given attachment IDs to a comment.
// Only updates attachments that belong to the same issue and have no comment_id yet.
func (h *Handler) linkAttachmentsByIDs(ctx context.Context, commentID, issueID pgtype.UUID, ids []string) {
	uuids := make([]pgtype.UUID, len(ids))
	for i, id := range ids {
		uuids[i] = parseUUID(id)
	}
	if err := h.Queries.LinkAttachmentsToComment(ctx, db.LinkAttachmentsToCommentParams{
		CommentID: commentID,
		IssueID:   issueID,
		Column3:   uuids,
	}); err != nil {
		slog.Error("failed to link attachments to comment", "error", err)
	}
}

// linkAttachmentsToIssueByIDs links the given attachment IDs to an issue.
// Only updates attachments that belong to the same workspace and are still
// unlinked (no issue_id, no comment_id). Used by CreateIssue when the caller
// uploads files first and then creates the issue referencing the resulting
// attachment IDs — this way the agent dispatch can wait for attachments to
// be linked before enqueueing the task.
func (h *Handler) linkAttachmentsToIssueByIDs(ctx context.Context, issueID, workspaceID pgtype.UUID, ids []string) error {
	if len(ids) == 0 {
		return nil
	}
	uuids := make([]pgtype.UUID, len(ids))
	for i, id := range ids {
		uuids[i] = parseUUID(id)
	}
	return h.Queries.LinkAttachmentsToIssue(ctx, db.LinkAttachmentsToIssueParams{
		IssueID:     issueID,
		WorkspaceID: workspaceID,
		Column3:     uuids,
	})
}

// deleteS3Object removes a single file from S3 by its CDN URL.
func (h *Handler) deleteS3Object(ctx context.Context, url string) {
	if h.Storage == nil || url == "" {
		return
	}
	h.Storage.Delete(ctx, h.Storage.KeyFromURL(url))
}

// deleteS3Objects removes multiple files from S3 by their CDN URLs.
func (h *Handler) deleteS3Objects(ctx context.Context, urls []string) {
	if h.Storage == nil || len(urls) == 0 {
		return
	}
	keys := make([]string, len(urls))
	for i, u := range urls {
		keys[i] = h.Storage.KeyFromURL(u)
	}
	h.Storage.DeleteKeys(ctx, keys)
}
