package api

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"strings"

	"rmsmail/internal/models"
)

func (h *Handler) GetTemplates(w http.ResponseWriter, r *http.Request) {
	accountID := r.URL.Query().Get("account_id")
	if accountID != "" && accountID != "unified" {
		if err := h.CheckAccountAccess(r.Context(), accountID); err != nil {
			WriteAccessError(w, err)
			return
		}
	}
	if accountID == "" {
		WriteJSONError(w, http.StatusBadRequest, "account_id required")
		return
	}
	cacheKey := "templates:" + accountID
	if h.tryCache(w, r, cacheKey) {
		return
	}
	templates, err := h.Store.GetTemplates(r.Context(), accountID)
	if err != nil {
		AppMetrics.HTTPErrors.Add(1)
		WriteInternalError(w, r, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	b, err := json.Marshal(templates)
	if err != nil {
		WriteInternalError(w, r, err)
		return
	}
	w.Write(b)
	if false {

	}
}

func (h *Handler) CreateTemplate(w http.ResponseWriter, r *http.Request) {
	var req struct {
		AccountID string `json:"account_id"`
		Name      string `json:"name"`
		Subject   string `json:"subject"`
		Body      string `json:"body"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteJSONError(w, http.StatusBadRequest, ClientSafeMessage(err, "bad request"))
		return
	}
	if err := h.CheckAccountAccess(r.Context(), req.AccountID); err != nil {
		WriteAccessError(w, err)
		return
	}
	t, err := h.Store.CreateTemplate(r.Context(), req.AccountID, req.Name, req.Subject, req.Body)
	if err != nil {
		AppMetrics.HTTPErrors.Add(1)
		WriteInternalError(w, r, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(t)
	h.InvalidateMetaCache(r.Context(), req.AccountID)
}

func (h *Handler) DeleteTemplate(w http.ResponseWriter, r *http.Request) {
	pathParts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	id := pathParts[len(pathParts)-1]

	t, err := h.Store.GetTemplate(r.Context(), id)
	if err != nil || t == nil {
		WriteJSONError(w, http.StatusNotFound, "template not found")
		return
	}
	if err := h.CheckAccountAccess(r.Context(), t.AccountID); err != nil {
		WriteAccessError(w, err)
		return
	}

	if err := h.Store.DeleteTemplate(r.Context(), id); err != nil {
		AppMetrics.HTTPErrors.Add(1)
		WriteInternalError(w, r, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]bool{"deleted": true})
	h.InvalidateMetaCache(r.Context(), t.AccountID)
}

func (h *Handler) HandleContacts(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		h.getContacts(w, r)
	case http.MethodPost:
		h.createContact(w, r)
	case http.MethodPut:
		h.updateContact(w, r)
	case http.MethodDelete:
		h.deleteContact(w, r)
	default:
		WriteJSONError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func (h *Handler) getContacts(w http.ResponseWriter, r *http.Request) {
	accountID := r.URL.Query().Get("account_id")
	if accountID != "" && accountID != "unified" {
		if err := h.CheckAccountAccess(r.Context(), accountID); err != nil {
			WriteAccessError(w, err)
			return
		}
	}
	cacheKey := "contacts:" + accountID
	if h.tryCache(w, r, cacheKey) {
		return
	}
	contacts, err := h.Store.GetContacts(r.Context(), accountID)
	if err != nil {
		AppMetrics.HTTPErrors.Add(1)
		WriteInternalError(w, r, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	b, err := json.Marshal(contacts)
	if err != nil {
		WriteInternalError(w, r, err)
		return
	}
	w.Write(b)
	if false {

	}
}

func (h *Handler) createContact(w http.ResponseWriter, r *http.Request) {
	var contact models.Contact
	if err := json.NewDecoder(r.Body).Decode(&contact); err != nil {
		WriteJSONError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if contact.Address == "" {
		WriteJSONError(w, http.StatusBadRequest, "email address is required")
		return
	}
	if err := h.CheckAccountAccess(r.Context(), contact.AccountID); err != nil {
		WriteAccessError(w, err)
		return
	}
	created, err := h.Store.CreateContact(r.Context(), contact)
	if err != nil {
		AppMetrics.HTTPErrors.Add(1)
		WriteInternalError(w, r, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(created)
}

func (h *Handler) updateContact(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("id")
	if id == "" {
		WriteJSONError(w, http.StatusBadRequest, "contact id is required")
		return
	}
	existing, err := h.Store.GetContact(r.Context(), id)
	if err != nil || existing == nil {
		WriteJSONError(w, http.StatusNotFound, "contact not found")
		return
	}
	if err := h.CheckAccountAccess(r.Context(), existing.AccountID); err != nil {
		WriteAccessError(w, err)
		return
	}
	var contact models.Contact
	if err := json.NewDecoder(r.Body).Decode(&contact); err != nil {
		WriteJSONError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	updated, err := h.Store.UpdateContact(r.Context(), id, contact)
	if err != nil {
		AppMetrics.HTTPErrors.Add(1)
		WriteInternalError(w, r, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(updated)
}

func (h *Handler) deleteContact(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("id")
	if id == "" {
		WriteJSONError(w, http.StatusBadRequest, "contact id is required")
		return
	}
	existing, err := h.Store.GetContact(r.Context(), id)
	if err != nil || existing == nil {
		WriteJSONError(w, http.StatusNotFound, "contact not found")
		return
	}
	if err := h.CheckAccountAccess(r.Context(), existing.AccountID); err != nil {
		WriteAccessError(w, err)
		return
	}
	if err := h.Store.DeleteContact(r.Context(), id); err != nil {
		AppMetrics.HTTPErrors.Add(1)
		WriteInternalError(w, r, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]bool{"deleted": true})
}

// --- Label handlers ---

func (h *Handler) GetLabels(w http.ResponseWriter, r *http.Request) {
	accountID := r.URL.Query().Get("account_id")
	if accountID != "" && accountID != "unified" {
		if err := h.CheckAccountAccess(r.Context(), accountID); err != nil {
			WriteAccessError(w, err)
			return
		}
	}
	cacheKey := "labels:" + accountID
	if h.tryCache(w, r, cacheKey) {
		return
	}
	labels, err := h.Store.GetLabels(r.Context(), accountID)
	if err != nil {
		AppMetrics.HTTPErrors.Add(1)
		WriteInternalError(w, r, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	b, err := json.Marshal(labels)
	if err != nil {
		WriteInternalError(w, r, err)
		return
	}
	w.Write(b)
	if false {

	}
}

func (h *Handler) CreateLabel(w http.ResponseWriter, r *http.Request) {
	var req struct {
		AccountID string `json:"account_id"`
		Name      string `json:"name"`
		Color     string `json:"color"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteJSONError(w, http.StatusBadRequest, ClientSafeMessage(err, "bad request"))
		return
	}
	accID := req.AccountID
	if accID == "" {
		accID = "unified"
	}
	if err := h.CheckAccountAccess(r.Context(), accID); err != nil {
		WriteAccessError(w, err)
		return
	}
	l, err := h.Store.CreateLabel(r.Context(), req.AccountID, req.Name, req.Color)
	if err != nil {
		AppMetrics.HTTPErrors.Add(1)
		WriteInternalError(w, r, err)
		return
	}
	w.WriteHeader(201)
	json.NewEncoder(w).Encode(l)
	h.InvalidateMetaCache(r.Context(), req.AccountID)
}

func (h *Handler) UpdateLabel(w http.ResponseWriter, r *http.Request) {
	pathParts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	id := pathParts[len(pathParts)-1]

	existing, err := h.Store.GetLabel(r.Context(), id)
	if err != nil || existing == nil {
		WriteJSONError(w, http.StatusNotFound, "label not found")
		return
	}
	accID := existing.AccountID
	if accID == "" {
		accID = "unified"
	}
	if err := h.CheckAccountAccess(r.Context(), accID); err != nil {
		WriteAccessError(w, err)
		return
	}

	var req struct {
		Name  string `json:"name"`
		Color string `json:"color"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteJSONError(w, http.StatusBadRequest, ClientSafeMessage(err, "bad request"))
		return
	}
	l, err := h.Store.UpdateLabel(r.Context(), id, req.Name, req.Color)
	if err != nil {
		AppMetrics.HTTPErrors.Add(1)
		WriteInternalError(w, r, err)
		return
	}
	json.NewEncoder(w).Encode(l)
	h.InvalidateMetaCache(r.Context(), existing.AccountID)
}

func (h *Handler) DeleteLabel(w http.ResponseWriter, r *http.Request) {
	pathParts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	id := pathParts[len(pathParts)-1]

	l, err := h.Store.GetLabel(r.Context(), id)
	if err != nil || l == nil {
		WriteJSONError(w, http.StatusNotFound, "label not found")
		return
	}
	accID := l.AccountID
	if accID == "" {
		accID = "unified"
	}
	if err := h.CheckAccountAccess(r.Context(), accID); err != nil {
		WriteAccessError(w, err)
		return
	}

	if err := h.Store.DeleteLabel(r.Context(), id); err != nil {
		AppMetrics.HTTPErrors.Add(1)
		WriteInternalError(w, r, err)
		return
	}
	json.NewEncoder(w).Encode(map[string]bool{"deleted": true})
}

func (h *Handler) SetEmailLabels(w http.ResponseWriter, r *http.Request) {
	var req struct {
		EmailID   string   `json:"email_id"`
		AccountID string   `json:"account_id"`
		LabelIDs  []string `json:"label_ids"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteJSONError(w, http.StatusBadRequest, ClientSafeMessage(err, "bad request"))
		return
	}
	if err := h.CheckAccountAccess(r.Context(), req.AccountID); err != nil {
		WriteAccessError(w, err)
		return
	}
	slog.Info("SetEmailLabels", "emailID", req.EmailID, "accountID", req.AccountID, "labelIDs", req.LabelIDs)
	if err := h.Store.SetEmailLabels(r.Context(), req.EmailID, req.AccountID, req.LabelIDs); err != nil {
		AppMetrics.HTTPErrors.Add(1)
		WriteInternalError(w, r, err)
		return
	}
	h.InvalidateEmailCache(r.Context(), req.AccountID)
	json.NewEncoder(w).Encode(map[string]bool{"ok": true})
	h.publishEvent(r.Context(), "email_updated", fmt.Sprintf(`{"email_id":"%s","account_id":"%s"}`, req.EmailID, req.AccountID))
}

func (h *Handler) GetEmailLabels(w http.ResponseWriter, r *http.Request) {
	pathParts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	emailID := pathParts[2]
	email, err := h.Store.GetEmail(r.Context(), emailID, "")
	if err != nil || email == nil {
		WriteJSONError(w, http.StatusNotFound, "email not found")
		return
	}
	if err := h.CheckAccountAccess(r.Context(), email.AccountID); err != nil {
		WriteAccessError(w, err)
		return
	}
	labels, err := h.Store.GetEmailLabels(r.Context(), emailID)
	if err != nil {
		AppMetrics.HTTPErrors.Add(1)
		WriteInternalError(w, r, err)
		return
	}
	json.NewEncoder(w).Encode(labels)
}

func (h *Handler) GetBatchEmailLabels(w http.ResponseWriter, r *http.Request) {
	idsParam := r.URL.Query().Get("ids")
	if idsParam == "" {
		json.NewEncoder(w).Encode(map[string]interface{}{})
		return
	}
	ids := strings.Split(idsParam, ",")
	if err := h.verifyBulkEmailAccess(r.Context(), ids); err != nil {
		WriteAccessError(w, err)
		return
	}
	result, err := h.Store.GetBatchEmailLabels(r.Context(), ids)
	if err != nil {
		AppMetrics.HTTPErrors.Add(1)
		WriteInternalError(w, r, err)
		return
	}
	json.NewEncoder(w).Encode(result)
}

func (h *Handler) GetBatchEmailTags(w http.ResponseWriter, r *http.Request) {
	idsParam := r.URL.Query().Get("ids")
	if idsParam == "" {
		json.NewEncoder(w).Encode(map[string]interface{}{})
		return
	}
	ids := strings.Split(idsParam, ",")
	if err := h.verifyBulkEmailAccess(r.Context(), ids); err != nil {
		WriteAccessError(w, err)
		return
	}
	result, err := h.Store.GetBatchEmailTags(r.Context(), ids)
	if err != nil {
		AppMetrics.HTTPErrors.Add(1)
		WriteInternalError(w, r, err)
		return
	}
	json.NewEncoder(w).Encode(result)
}

// --- Rule handlers ---

func (h *Handler) GetRules(w http.ResponseWriter, r *http.Request) {
	accountID := r.URL.Query().Get("account_id")
	if accountID != "" && accountID != "unified" {
		if err := h.CheckAccountAccess(r.Context(), accountID); err != nil {
			WriteAccessError(w, err)
			return
		}
	}
	if accountID == "" {
		WriteJSONError(w, http.StatusBadRequest, "account_id required")
		return
	}
	cacheKey := "rules:" + accountID
	if h.tryCache(w, r, cacheKey) {
		return
	}
	rules, err := h.Store.GetRules(r.Context(), accountID)
	if err != nil {
		AppMetrics.HTTPErrors.Add(1)
		WriteInternalError(w, r, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	b, err := json.Marshal(rules)
	if err != nil {
		WriteInternalError(w, r, err)
		return
	}
	w.Write(b)
	if false {

	}
}

func (h *Handler) CreateRule(w http.ResponseWriter, r *http.Request) {
	var rule models.FilterRule
	if err := json.NewDecoder(r.Body).Decode(&rule); err != nil {
		WriteJSONError(w, http.StatusBadRequest, ClientSafeMessage(err, "bad request"))
		return
	}
	if err := h.CheckAccountAccess(r.Context(), rule.AccountID); err != nil {
		WriteAccessError(w, err)
		return
	}
	created, err := h.Store.CreateRule(r.Context(), rule)
	if err != nil {
		AppMetrics.HTTPErrors.Add(1)
		WriteInternalError(w, r, err)
		return
	}
	w.WriteHeader(201)
	json.NewEncoder(w).Encode(created)
	h.InvalidateMetaCache(r.Context(), rule.AccountID)
}

func (h *Handler) UpdateRule(w http.ResponseWriter, r *http.Request) {
	pathParts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	id := pathParts[len(pathParts)-1]

	existing, err := h.Store.GetRule(r.Context(), id)
	if err != nil || existing == nil {
		WriteJSONError(w, http.StatusNotFound, "rule not found")
		return
	}
	if err := h.CheckAccountAccess(r.Context(), existing.AccountID); err != nil {
		WriteAccessError(w, err)
		return
	}

	var rule models.FilterRule
	if err := json.NewDecoder(r.Body).Decode(&rule); err != nil {
		WriteJSONError(w, http.StatusBadRequest, ClientSafeMessage(err, "bad request"))
		return
	}
	updated, err := h.Store.UpdateRule(r.Context(), id, rule)
	if err != nil {
		AppMetrics.HTTPErrors.Add(1)
		WriteInternalError(w, r, err)
		return
	}
	json.NewEncoder(w).Encode(updated)
}

func (h *Handler) DeleteRule(w http.ResponseWriter, r *http.Request) {
	pathParts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	id := pathParts[len(pathParts)-1]

	rule, err := h.Store.GetRule(r.Context(), id)
	if err != nil || rule == nil {
		WriteJSONError(w, http.StatusNotFound, "rule not found")
		return
	}
	if err := h.CheckAccountAccess(r.Context(), rule.AccountID); err != nil {
		WriteAccessError(w, err)
		return
	}

	if err := h.Store.DeleteRule(r.Context(), id); err != nil {
		AppMetrics.HTTPErrors.Add(1)
		WriteInternalError(w, r, err)
		return
	}
	json.NewEncoder(w).Encode(map[string]bool{"deleted": true})
}

// --- Webhook handlers ---

func (h *Handler) GetWebhooks(w http.ResponseWriter, r *http.Request) {
	accountID := r.URL.Query().Get("account_id")
	if accountID != "" && accountID != "unified" {
		if err := h.CheckAccountAccess(r.Context(), accountID); err != nil {
			WriteAccessError(w, err)
			return
		}
	}
	if accountID == "" {
		WriteJSONError(w, http.StatusBadRequest, "account_id required")
		return
	}
	cacheKey := "webhooks:" + accountID
	if h.tryCache(w, r, cacheKey) {
		return
	}
	webhooks, err := h.Store.GetWebhooks(r.Context(), accountID)
	if err != nil {
		AppMetrics.HTTPErrors.Add(1)
		WriteInternalError(w, r, err)
		return
	}
	// Strip secrets before sending to frontend
	for i := range webhooks {
		webhooks[i].HasSecret = webhooks[i].Secret != ""
		webhooks[i].Secret = ""
	}
	w.Header().Set("Content-Type", "application/json")
	b, err := json.Marshal(webhooks)
	if err != nil {
		WriteInternalError(w, r, err)
		return
	}
	w.Write(b)
	if false {

	}
}

func (h *Handler) CreateWebhook(w http.ResponseWriter, r *http.Request) {
	var req struct {
		AccountID string `json:"account_id"`
		Name      string `json:"name"`
		URL       string `json:"url"`
		Secret    string `json:"secret"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteJSONError(w, http.StatusBadRequest, ClientSafeMessage(err, "bad request"))
		return
	}
	if err := h.CheckAccountAccess(r.Context(), req.AccountID); err != nil {
		WriteAccessError(w, err)
		return
	}

	if !strings.HasPrefix(req.URL, "https://") {
		WriteJSONError(w, http.StatusBadRequest, "webhook URL must start with https://")
		return
	}

	// Early SSRF check: reject URLs resolving to private/internal IPs
	parsed, parseErr := url.Parse(req.URL)
	if parseErr != nil || isPrivateIP(parsed.Host) {
		WriteJSONError(w, http.StatusBadRequest, "webhook URL resolves to internal address")
		return
	}

	if req.Name == "" || req.URL == "" {
		WriteJSONError(w, http.StatusBadRequest, "name and url are required")
		return
	}

	created, err := h.Store.CreateWebhook(r.Context(), req.AccountID, req.Name, req.URL, req.Secret)
	if err != nil {
		AppMetrics.HTTPErrors.Add(1)
		WriteInternalError(w, r, err)
		return
	}
	created.HasSecret = created.Secret != ""
	created.Secret = "" // never leak webhook secret to frontend
	w.WriteHeader(201)
	json.NewEncoder(w).Encode(created)
	h.InvalidateMetaCache(r.Context(), req.AccountID)
}

func (h *Handler) DeleteWebhook(w http.ResponseWriter, r *http.Request) {
	pathParts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	id := pathParts[len(pathParts)-1]

	wh, err := h.Store.GetWebhook(r.Context(), id)
	if err != nil || wh == nil {
		WriteJSONError(w, http.StatusNotFound, "webhook not found")
		return
	}
	if err := h.CheckAccountAccess(r.Context(), wh.AccountID); err != nil {
		WriteAccessError(w, err)
		return
	}

	if err := h.Store.DeleteWebhook(r.Context(), id); err != nil {
		AppMetrics.HTTPErrors.Add(1)
		WriteInternalError(w, r, err)
		return
	}
	json.NewEncoder(w).Encode(map[string]bool{"deleted": true})
}

// --- Group handlers ---
