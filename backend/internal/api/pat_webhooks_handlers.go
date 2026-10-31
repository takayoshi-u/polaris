package api

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/takayoshi-u/polaris/backend/internal/store"
)

// ==========================================
// PERSONAL ACCESS TOKEN (PAT) HANDLERS
// ==========================================

func (s *Server) handleListPATs(w http.ResponseWriter, r *http.Request) {
	u := currentUser(r)
	list, err := s.store.ListPATs(r.Context(), u.ID)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, list)
}

func (s *Server) handleCreatePAT(w http.ResponseWriter, r *http.Request) {
	u := currentUser(r)
	var body struct {
		Name      string    `json:"name"`
		Scopes    []string  `json:"scopes"`
		ExpiresIn *int      `json:"expires_in_days"` // optional days
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid JSON body"})
		return
	}

	var expiresAt *time.Time
	if body.ExpiresIn != nil && *body.ExpiresIn > 0 {
		exp := time.Now().UTC().AddDate(0, 0, *body.ExpiresIn)
		expiresAt = &exp
	}

	plainToken, pat, err := s.store.CreatePAT(r.Context(), u.ID, body.Name, body.Scopes, expiresAt)
	if err != nil {
		writeError(w, err)
		return
	}

	writeJSON(w, http.StatusCreated, map[string]any{
		"token": plainToken,
		"pat":   pat,
	})
}

func (s *Server) handleRevokePAT(w http.ResponseWriter, r *http.Request) {
	u := currentUser(r)
	tokenID := r.PathValue("tokenId")
	if err := s.store.RevokePAT(r.Context(), u.ID, tokenID); err != nil {
		writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ==========================================
// WEBHOOK HANDLERS (PROJECT & SYSTEM)
// ==========================================

func (s *Server) handleListProjectWebhooks(w http.ResponseWriter, r *http.Request) {
	pid := r.PathValue("projectId")
	list, err := s.store.ListWebhooks(r.Context(), &pid)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, list)
}

func (s *Server) handleCreateProjectWebhook(w http.ResponseWriter, r *http.Request) {
	u := currentUser(r)
	pid := r.PathValue("projectId")
	var body struct {
		Name   string   `json:"name"`
		URL    string   `json:"url"`
		Secret string   `json:"secret"`
		Events []string `json:"events"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid JSON body"})
		return
	}

	wh, err := s.store.CreateWebhook(r.Context(), &pid, body.Name, body.URL, body.Secret, body.Events, u.ID)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, wh)
}

func (s *Server) handleGetWebhook(w http.ResponseWriter, r *http.Request) {
	whID := r.PathValue("webhookId")
	wh, err := s.store.GetWebhook(r.Context(), whID)
	if err != nil {
		writeError(w, err)
		return
	}
	pid := r.PathValue("projectId")
	if pid != "" && (wh.ProjectID == nil || *wh.ProjectID != pid) {
		writeError(w, store.ErrNotFound)
		return
	}
	if pid == "" && wh.ProjectID != nil {
		writeError(w, store.ErrNotFound)
		return
	}
	writeJSON(w, http.StatusOK, wh)
}

func (s *Server) handleUpdateWebhook(w http.ResponseWriter, r *http.Request) {
	whID := r.PathValue("webhookId")
	wh, err := s.store.GetWebhook(r.Context(), whID)
	if err != nil {
		writeError(w, err)
		return
	}
	pid := r.PathValue("projectId")
	if pid != "" && (wh.ProjectID == nil || *wh.ProjectID != pid) {
		writeError(w, store.ErrNotFound)
		return
	}
	if pid == "" && wh.ProjectID != nil {
		writeError(w, store.ErrNotFound)
		return
	}

	var body struct {
		Name     string   `json:"name"`
		URL      string   `json:"url"`
		Secret   string   `json:"secret"`
		Events   []string `json:"events"`
		IsActive bool     `json:"is_active"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid JSON body"})
		return
	}

	updated, err := s.store.UpdateWebhook(r.Context(), whID, body.Name, body.URL, body.Secret, body.Events, body.IsActive)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, updated)
}

func (s *Server) handleDeleteWebhook(w http.ResponseWriter, r *http.Request) {
	whID := r.PathValue("webhookId")
	wh, err := s.store.GetWebhook(r.Context(), whID)
	if err != nil {
		writeError(w, err)
		return
	}
	pid := r.PathValue("projectId")
	if pid != "" && (wh.ProjectID == nil || *wh.ProjectID != pid) {
		writeError(w, store.ErrNotFound)
		return
	}
	if pid == "" && wh.ProjectID != nil {
		writeError(w, store.ErrNotFound)
		return
	}

	if err := s.store.DeleteWebhook(r.Context(), whID); err != nil {
		writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleTestWebhook(w http.ResponseWriter, r *http.Request) {
	whID := r.PathValue("webhookId")
	wh, err := s.store.GetWebhook(r.Context(), whID)
	if err != nil {
		writeError(w, err)
		return
	}
	pid := r.PathValue("projectId")
	if pid != "" && (wh.ProjectID == nil || *wh.ProjectID != pid) {
		writeError(w, store.ErrNotFound)
		return
	}
	if pid == "" && wh.ProjectID != nil {
		writeError(w, store.ErrNotFound)
		return
	}

	delivery, err := s.store.TestWebhook(r.Context(), whID)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, delivery)
}

func (s *Server) handleListWebhookDeliveries(w http.ResponseWriter, r *http.Request) {
	whID := r.PathValue("webhookId")
	wh, err := s.store.GetWebhook(r.Context(), whID)
	if err != nil {
		writeError(w, err)
		return
	}
	pid := r.PathValue("projectId")
	if pid != "" && (wh.ProjectID == nil || *wh.ProjectID != pid) {
		writeError(w, store.ErrNotFound)
		return
	}
	if pid == "" && wh.ProjectID != nil {
		writeError(w, store.ErrNotFound)
		return
	}

	list, err := s.store.ListWebhookDeliveries(r.Context(), whID, 20)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, list)
}

// System-wide Webhooks
func (s *Server) handleListSystemWebhooks(w http.ResponseWriter, r *http.Request) {
	list, err := s.store.ListWebhooks(r.Context(), nil)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, list)
}

func (s *Server) handleCreateSystemWebhook(w http.ResponseWriter, r *http.Request) {
	u := currentUser(r)
	var body struct {
		Name   string   `json:"name"`
		URL    string   `json:"url"`
		Secret string   `json:"secret"`
		Events []string `json:"events"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid JSON body"})
		return
	}

	wh, err := s.store.CreateWebhook(r.Context(), nil, body.Name, body.URL, body.Secret, body.Events, u.ID)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, wh)
}
