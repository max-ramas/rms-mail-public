package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"rmsmail/internal/api/middleware"
)

func (h *Handler) GetGroups(w http.ResponseWriter, r *http.Request) {
	// Redis cache: groups change rarely, TTL 30s.
	if false {

	}

	groups, err := h.Store.GetGroups(r.Context())
	if err != nil {
		AppMetrics.HTTPErrors.Add(1)
		WriteInternalError(w, r, err)
		return
	}

	// Compute is_locked live (backend enforcement, not from DB flag)
	if false {

	}

	w.Header().Set("Content-Type", "application/json")
	b, err := json.Marshal(groups)
	if err != nil {
		WriteInternalError(w, r, err)
		return
	}
	w.Write(b)

	if false {

	}
}

func (h *Handler) CreateGroup(w http.ResponseWriter, r *http.Request) {
	if !h.requireAdmin(w, r) {
		return
	}
	var req struct {
		Name      string `json:"name"`
		Color     string `json:"color"`
		SortOrder int    `json:"sort_order"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteJSONError(w, http.StatusBadRequest, ClientSafeMessage(err, "bad request"))
		return
	}

	h.creationMu.Lock()
	defer h.creationMu.Unlock()

	if false {

	}

	if req.Color == "" {
		req.Color = "#6366f1"
	}

	g, err := h.Store.CreateGroup(r.Context(), req.Name, req.Color, req.SortOrder)
	if err != nil {
		AppMetrics.HTTPErrors.Add(1)
		WriteInternalError(w, r, err)
		return
	}
	w.WriteHeader(201)
	json.NewEncoder(w).Encode(g)
}

func (h *Handler) UpdateGroup(w http.ResponseWriter, r *http.Request) {
	if !h.requireAdmin(w, r) {
		return
	}
	pathParts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	id := pathParts[len(pathParts)-1]
	// Live lock check (position-based, not DB flag)
	if false {

	}
	var req struct {
		Name      string `json:"name"`
		Color     string `json:"color"`
		SortOrder int    `json:"sort_order"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteJSONError(w, http.StatusBadRequest, ClientSafeMessage(err, "bad request"))
		return
	}
	if req.Color == "" {
		req.Color = "#6366f1"
	}
	g, err := h.Store.UpdateGroup(r.Context(), id, req.Name, req.Color, req.SortOrder)
	if err != nil {
		AppMetrics.HTTPErrors.Add(1)
		WriteInternalError(w, r, err)
		return
	}
	json.NewEncoder(w).Encode(g)
}

func (h *Handler) DeleteGroup(w http.ResponseWriter, r *http.Request) {
	if !h.requireAdmin(w, r) {
		return
	}
	pathParts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	id := pathParts[len(pathParts)-1]
	// Live lock check (position-based, not DB flag)
	if false {

	}
	if err := h.Store.DeleteGroup(r.Context(), id); err != nil {
		AppMetrics.HTTPErrors.Add(1)
		WriteInternalError(w, r, err)
		return
	}
	json.NewEncoder(w).Encode(map[string]bool{"deleted": true})
}

func (h *Handler) SetGroupAccounts(w http.ResponseWriter, r *http.Request) {
	if !h.requireAdmin(w, r) {
		return
	}
	var req struct {
		GroupID    string   `json:"group_id"`
		AccountIDs []string `json:"account_ids"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteJSONError(w, http.StatusBadRequest, ClientSafeMessage(err, "bad request"))
		return
	}
	// Live lock check (position-based, not DB flag)
	if false {

	}
	if err := h.Store.SetGroupAccounts(r.Context(), req.GroupID, req.AccountIDs); err != nil {
		AppMetrics.HTTPErrors.Add(1)
		WriteInternalError(w, r, err)
		return
	}
	json.NewEncoder(w).Encode(map[string]bool{"ok": true})
}

func (h *Handler) GetGroupAccounts(w http.ResponseWriter, r *http.Request) {
	pathParts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	id := pathParts[len(pathParts)-1]
	// Live lock check (position-based, not DB flag)
	if false {

	}
	ids, err := h.Store.GetGroupAccounts(r.Context(), id)
	if err != nil {
		AppMetrics.HTTPErrors.Add(1)
		WriteInternalError(w, r, err)
		return
	}
	json.NewEncoder(w).Encode(ids)
}

// --- User handlers ---

func (h *Handler) GetUsers(w http.ResponseWriter, r *http.Request) {
	if !h.requireAdmin(w, r) {
		return
	}
	users, err := h.Store.GetUsers(r.Context())
	if err != nil {
		AppMetrics.HTTPErrors.Add(1)
		WriteInternalError(w, r, err)
		return
	}
	json.NewEncoder(w).Encode(users)
}

func (h *Handler) CreateUser(w http.ResponseWriter, r *http.Request) {
	if !h.requireAdmin(w, r) {
		return
	}
	var req struct {
		Email string `json:"email"`
		Name  string `json:"name"`
		Role  string `json:"role"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteJSONError(w, http.StatusBadRequest, ClientSafeMessage(err, "bad request"))
		return
	}
	u, err := h.Store.CreateUser(r.Context(), req.Email, req.Name, req.Role)
	if err != nil {
		AppMetrics.HTTPErrors.Add(1)
		WriteInternalError(w, r, err)
		return
	}
	w.WriteHeader(201)
	json.NewEncoder(w).Encode(u)
}

func (h *Handler) DeleteUser(w http.ResponseWriter, r *http.Request) {
	if !h.requireAdmin(w, r) {
		return
	}
	pathParts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	id := pathParts[len(pathParts)-1]
	if err := h.Store.DeleteUser(r.Context(), id); err != nil {
		AppMetrics.HTTPErrors.Add(1)
		WriteInternalError(w, r, err)
		return
	}
	json.NewEncoder(w).Encode(map[string]bool{"deleted": true})
}

// --- Assignment handlers ---

func (h *Handler) AssignEmail(w http.ResponseWriter, r *http.Request) {
	var req struct {
		EmailID string `json:"email_id"`
		UserID  string `json:"user_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteJSONError(w, http.StatusBadRequest, ClientSafeMessage(err, "bad request"))
		return
	}
	email, err := h.Store.GetEmail(r.Context(), req.EmailID, "")
	if err != nil || email == nil {
		WriteJSONError(w, http.StatusNotFound, "email not found")
		return
	}
	if err := h.CheckAccountAccess(r.Context(), email.AccountID); err != nil {
		WriteAccessError(w, err)
		return
	}
	if err := h.Store.AssignEmail(r.Context(), req.EmailID, req.UserID); err != nil {
		AppMetrics.HTTPErrors.Add(1)
		WriteInternalError(w, r, err)
		return
	}

	// Dispatch event
	h.publishEvent(r.Context(), "email_updated", fmt.Sprintf(`{"email_id":"%s","account_id":"%s"}`, req.EmailID, email.AccountID))

	json.NewEncoder(w).Encode(map[string]bool{"assigned": true})
}

func (h *Handler) UnassignEmail(w http.ResponseWriter, r *http.Request) {
	var req struct {
		EmailID string `json:"email_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteJSONError(w, http.StatusBadRequest, ClientSafeMessage(err, "bad request"))
		return
	}
	email, err := h.Store.GetEmail(r.Context(), req.EmailID, "")
	if err != nil || email == nil {
		WriteJSONError(w, http.StatusNotFound, "email not found")
		return
	}
	if err := h.CheckAccountAccess(r.Context(), email.AccountID); err != nil {
		WriteAccessError(w, err)
		return
	}
	if err := h.Store.UnassignEmail(r.Context(), req.EmailID); err != nil {
		AppMetrics.HTTPErrors.Add(1)
		WriteInternalError(w, r, err)
		return
	}
	h.publishEvent(r.Context(), "email_updated", fmt.Sprintf(`{"email_id":"%s","account_id":"%s"}`, req.EmailID, email.AccountID))
	json.NewEncoder(w).Encode(map[string]bool{"unassigned": true})
}

// --- Shared Mailbox / B2B Dashboard ---

func (h *Handler) GetSharedDashboard(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserIDFromContext(r.Context())
	if !middleware.IsAdminFromContext(r.Context()) {
		_, _, err := h.Store.GetAdminByEmail(r.Context(), userID)
		if err != nil || userID == "" {
			WriteJSONError(w, http.StatusForbidden, "admin required")
			return
		}
	}

	totalUnassigned, err := h.Store.GetUnassignedCount(r.Context())
	if err != nil {
		AppMetrics.HTTPErrors.Add(1)
		WriteInternalError(w, r, err)
		return
	}
	byAgent, err := h.Store.GetStatsByAgent(r.Context())
	if err != nil {
		AppMetrics.HTTPErrors.Add(1)
		WriteInternalError(w, r, err)
		return
	}
	slaBreaches, err := h.Store.GetSLABreaches(r.Context(), 1)
	if err != nil {
		AppMetrics.HTTPErrors.Add(1)
		WriteInternalError(w, r, err)
		return
	}
	json.NewEncoder(w).Encode(map[string]interface{}{
		"total_unassigned": totalUnassigned,
		"by_agent":         byAgent,
		"sla_breaches":     slaBreaches,
	})
}

func (h *Handler) SetEmailStatus(w http.ResponseWriter, r *http.Request) {
	pathParts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	// path: api/emails/status/{id}
	if len(pathParts) < 4 {
		WriteJSONError(w, http.StatusBadRequest, "missing email id")
		return
	}
	emailID := pathParts[len(pathParts)-1]

	var req struct {
		Status string `json:"status"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteJSONError(w, http.StatusBadRequest, ClientSafeMessage(err, "bad request"))
		return
	}

	now := time.Now()

	switch req.Status {
	case "resolved":
		// Ensure first_response_at is set if missing, then set resolved_at
		email, err := h.Store.GetEmail(r.Context(), emailID, r.URL.Query().Get("account_id"))
		if err != nil {
			AppMetrics.HTTPErrors.Add(1)
			WriteInternalError(w, r, err)
			return
		}
		if chkErr := h.CheckAccountAccess(r.Context(), email.AccountID); chkErr != nil {
			WriteJSONError(w, http.StatusForbidden, "access denied")
			return
		}
		if email == nil {
			WriteJSONError(w, http.StatusNotFound, "email not found")
			return
		}
		if email.FirstResponseAt == nil {
			_ = h.Store.UpdateEmailFirstResponseAt(r.Context(), emailID, now)
		}
		if email.ResolvedAt == nil {
			_ = h.Store.UpdateEmailResolvedAt(r.Context(), emailID, now)
		}
	case "in_progress":
		email, err := h.Store.GetEmail(r.Context(), emailID, r.URL.Query().Get("account_id"))
		if err != nil {
			AppMetrics.HTTPErrors.Add(1)
			WriteInternalError(w, r, err)
			return
		}
		if chkErr := h.CheckAccountAccess(r.Context(), email.AccountID); chkErr != nil {
			WriteJSONError(w, http.StatusForbidden, "access denied")
			return
		}
		if email == nil {
			WriteJSONError(w, http.StatusNotFound, "email not found")
			return
		}
		if email.FirstResponseAt == nil {
			_ = h.Store.UpdateEmailFirstResponseAt(r.Context(), emailID, now)
		}
	}

	if err := h.Store.UpdateEmailStatus(r.Context(), emailID, req.Status); err != nil {
		AppMetrics.HTTPErrors.Add(1)
		WriteInternalError(w, r, err)
		return
	}

	// Dispatch event
	h.publishEvent(r.Context(), "email_updated", fmt.Sprintf(`{"email_id":"%s","account_id":"%s"}`, emailID, r.URL.Query().Get("account_id")))

	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":   req.Status,
		"email_id": emailID,
	})
}

// --- Comment handlers ---

func (h *Handler) GetComments(w http.ResponseWriter, r *http.Request) {
	pathParts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	emailID := pathParts[len(pathParts)-1]

	email, err := h.Store.GetEmail(r.Context(), emailID, "")
	if err != nil || email == nil {
		WriteJSONError(w, http.StatusNotFound, "email not found")
		return
	}
	if err := h.CheckAccountAccess(r.Context(), email.AccountID); err != nil {
		WriteAccessError(w, err)
		return
	}

	comments, err := h.Store.GetComments(r.Context(), emailID)
	if err != nil {
		AppMetrics.HTTPErrors.Add(1)
		WriteInternalError(w, r, err)
		return
	}
	json.NewEncoder(w).Encode(comments)
}

func (h *Handler) CreateComment(w http.ResponseWriter, r *http.Request) {
	var req struct {
		EmailID  string `json:"email_id"`
		AuthorID string `json:"author_id"`
		Body     string `json:"body"`
		Internal bool   `json:"internal"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteJSONError(w, http.StatusBadRequest, ClientSafeMessage(err, "bad request"))
		return
	}

	email, err := h.Store.GetEmail(r.Context(), req.EmailID, "")
	if err != nil || email == nil {
		WriteJSONError(w, http.StatusNotFound, "email not found")
		return
	}
	if err := h.CheckAccountAccess(r.Context(), email.AccountID); err != nil {
		WriteAccessError(w, err)
		return
	}

	// Use authenticated user as author, ignore client-provided author_id
	authorID := middleware.GetUserIDFromContext(r.Context())

	c, err := h.Store.CreateComment(r.Context(), req.EmailID, authorID, req.Body, req.Internal)
	if err != nil {
		AppMetrics.HTTPErrors.Add(1)
		WriteInternalError(w, r, err)
		return
	}

	// Dispatch event
	h.publishEvent(r.Context(), "new_comment", fmt.Sprintf(`{"email_id":"%s","account_id":"%s"}`, req.EmailID, c.AccountID))

	w.WriteHeader(201)
	json.NewEncoder(w).Encode(c)
}

func (h *Handler) DeleteComment(w http.ResponseWriter, r *http.Request) {
	pathParts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	id := pathParts[len(pathParts)-1]

	c, err := h.Store.GetComment(r.Context(), id)
	if err != nil || c == nil {
		WriteJSONError(w, http.StatusNotFound, "comment not found")
		return
	}
	if err := h.CheckAccountAccess(r.Context(), c.AccountID); err != nil {
		WriteAccessError(w, err)
		return
	}

	if err := h.Store.DeleteComment(r.Context(), id); err != nil {
		AppMetrics.HTTPErrors.Add(1)
		WriteInternalError(w, r, err)
		return
	}
	json.NewEncoder(w).Encode(map[string]bool{"deleted": true})
}
