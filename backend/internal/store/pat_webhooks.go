package store

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/takayoshi-u/polaris/backend/internal/domain"
	"github.com/takayoshi-u/polaris/backend/internal/pkg/crypto"
	"github.com/takayoshi-u/polaris/backend/internal/pkg/id"
)

// ==========================================
// PERSONAL ACCESS TOKENS (API KEYS)
// ==========================================

// CreatePAT generates a new Personal Access Token, stores its SHA-256 hash, and returns the plaintext token once.
func (s *Store) CreatePAT(ctx context.Context, userID, name string, scopes []string, expiresAt *time.Time) (plainToken string, pat *domain.PersonalAccessToken, err error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return "", nil, fmt.Errorf("%w: token name is required", ErrBadRequest)
	}
	if scopes == nil {
		scopes = []string{}
	}

	rawHex, _, err := crypto.GenerateRandomToken(24)
	if err != nil {
		return "", nil, err
	}
	plainToken = "plr_pat_" + rawHex
	tokenHash := crypto.HashToken(plainToken)
	tokenPrefix := "plr_pat_" + rawHex[:6]

	patID := id.NewV7String()
	scopesJSON, _ := json.Marshal(scopes)
	now := time.Now().UTC()

	_, err = s.db.ExecContext(ctx, `
		INSERT INTO personal_access_tokens (id, user_id, name, token_hash, token_prefix, scopes, expires_at, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`, patID, userID, name, tokenHash, tokenPrefix, scopesJSON, expiresAt, now)
	if err != nil {
		return "", nil, err
	}

	return plainToken, &domain.PersonalAccessToken{
		ID:          patID,
		UserID:      userID,
		Name:        name,
		TokenPrefix: tokenPrefix,
		Scopes:      scopes,
		ExpiresAt:   expiresAt,
		CreatedAt:   now,
	}, nil
}

// ListPATs returns all personal access tokens for a given user.
func (s *Store) ListPATs(ctx context.Context, userID string) ([]domain.PersonalAccessToken, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, user_id, name, token_prefix, scopes, expires_at, last_used_at, revoked_at, created_at
		FROM personal_access_tokens
		WHERE user_id = $1
		ORDER BY created_at DESC
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	list := []domain.PersonalAccessToken{}
	for rows.Next() {
		var p domain.PersonalAccessToken
		var scopesRaw []byte
		if err := rows.Scan(&p.ID, &p.UserID, &p.Name, &p.TokenPrefix, &scopesRaw, &p.ExpiresAt, &p.LastUsedAt, &p.RevokedAt, &p.CreatedAt); err != nil {
			return nil, err
		}
		_ = json.Unmarshal(scopesRaw, &p.Scopes)
		list = append(list, p)
	}
	return list, rows.Err()
}

// RevokePAT revokes a personal access token belonging to the given user.
func (s *Store) RevokePAT(ctx context.Context, userID, patID string) error {
	res, err := s.db.ExecContext(ctx, `
		UPDATE personal_access_tokens
		SET revoked_at = NOW()
		WHERE id = $1 AND user_id = $2 AND revoked_at IS NULL
	`, patID, userID)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

// VerifyPAT validates a raw personal access token and returns the corresponding active user and scopes.
func (s *Store) VerifyPAT(ctx context.Context, rawToken string) (user *domain.User, scopes []string, patID string, err error) {
	if !strings.HasPrefix(rawToken, "plr_pat_") {
		return nil, nil, "", ErrUnauthorized
	}
	tokenHash := crypto.HashToken(rawToken)

	var u domain.User
	var scopesRaw []byte
	var pID string
	var expiresAt, revokedAt *time.Time

	err = s.db.QueryRowContext(ctx, `
		SELECT pat.id, pat.scopes, pat.expires_at, pat.revoked_at,
		       u.id, u.email, u.display_name, u.avatar_url, u.status, u.created_at, u.updated_at
		FROM personal_access_tokens pat
		JOIN users u ON pat.user_id = u.id
		WHERE pat.token_hash = $1
	`, tokenHash).Scan(
		&pID, &scopesRaw, &expiresAt, &revokedAt,
		&u.ID, &u.Email, &u.DisplayName, &u.AvatarURL, &u.Status, &u.CreatedAt, &u.UpdatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil, "", ErrUnauthorized
	} else if err != nil {
		return nil, nil, "", err
	}

	if revokedAt != nil {
		return nil, nil, "", fmt.Errorf("%w: token has been revoked", ErrUnauthorized)
	}
	if expiresAt != nil && expiresAt.Before(time.Now().UTC()) {
		return nil, nil, "", fmt.Errorf("%w: token has expired", ErrUnauthorized)
	}
	if u.Status != domain.UserStatusActive {
		return nil, nil, "", fmt.Errorf("%w: user account is not active", ErrForbidden)
	}

	_ = json.Unmarshal(scopesRaw, &scopes)

	// Update last_used_at in background
	go func(id string) {
		_, _ = s.db.Exec(`UPDATE personal_access_tokens SET last_used_at = NOW() WHERE id = $1`, id)
	}(pID)

	// Fetch user's permissions
	perms, _ := s.GetUserSystemPermissions(ctx, u.ID)
	u.Permissions = perms

	return &u, scopes, pID, nil
}

// ==========================================
// OUTBOUND WEBHOOKS
// ==========================================

// CreateWebhook creates a new outbound webhook.
func (s *Store) CreateWebhook(ctx context.Context, projectID *string, name, url, secret string, events []string, createdBy string) (*domain.Webhook, error) {
	name = strings.TrimSpace(name)
	url = strings.TrimSpace(url)
	if name == "" {
		return nil, fmt.Errorf("%w: webhook name is required", ErrBadRequest)
	}
	if url == "" || (!strings.HasPrefix(url, "https://") && !strings.HasPrefix(url, "http://")) {
		return nil, fmt.Errorf("%w: valid webhook URL (http/https) is required", ErrBadRequest)
	}
	if secret == "" {
		secretHex, _, _ := crypto.GenerateRandomToken(20)
		secret = "whsec_" + secretHex
	}
	if events == nil {
		events = []string{}
	}

	webhookID := id.NewV7String()
	eventsJSON, _ := json.Marshal(events)
	now := time.Now().UTC()

	_, err := s.db.ExecContext(ctx, `
		INSERT INTO webhooks (id, project_id, name, url, secret, events, is_active, created_by, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, true, $7, $8, $8)
	`, webhookID, projectID, name, url, secret, eventsJSON, createdBy, now)
	if err != nil {
		return nil, err
	}

	return &domain.Webhook{
		ID:        webhookID,
		ProjectID: projectID,
		Name:      name,
		URL:       url,
		Secret:    secret,
		Events:    events,
		IsActive:  true,
		CreatedBy: createdBy,
		CreatedAt: now,
		UpdatedAt: now,
	}, nil
}

// ListWebhooks returns all webhooks for a project (or system-wide if projectID is nil).
func (s *Store) ListWebhooks(ctx context.Context, projectID *string) ([]domain.Webhook, error) {
	var rows *sql.Rows
	var err error
	if projectID == nil {
		rows, err = s.db.QueryContext(ctx, `
			SELECT id, project_id, name, url, secret, events, is_active, created_by, created_at, updated_at
			FROM webhooks
			WHERE project_id IS NULL
			ORDER BY created_at DESC
		`)
	} else {
		rows, err = s.db.QueryContext(ctx, `
			SELECT id, project_id, name, url, secret, events, is_active, created_by, created_at, updated_at
			FROM webhooks
			WHERE project_id = $1
			ORDER BY created_at DESC
		`, *projectID)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	list := []domain.Webhook{}
	for rows.Next() {
		var w domain.Webhook
		var pid sql.NullString
		var eventsRaw []byte
		if err := rows.Scan(&w.ID, &pid, &w.Name, &w.URL, &w.Secret, &eventsRaw, &w.IsActive, &w.CreatedBy, &w.CreatedAt, &w.UpdatedAt); err != nil {
			return nil, err
		}
		if pid.Valid {
			w.ProjectID = &pid.String
		}
		_ = json.Unmarshal(eventsRaw, &w.Events)
		list = append(list, w)
	}
	return list, rows.Err()
}

// GetWebhook returns a single webhook by ID.
func (s *Store) GetWebhook(ctx context.Context, webhookID string) (*domain.Webhook, error) {
	var w domain.Webhook
	var pid sql.NullString
	var eventsRaw []byte
	err := s.db.QueryRowContext(ctx, `
		SELECT id, project_id, name, url, secret, events, is_active, created_by, created_at, updated_at
		FROM webhooks
		WHERE id = $1
	`, webhookID).Scan(&w.ID, &pid, &w.Name, &w.URL, &w.Secret, &eventsRaw, &w.IsActive, &w.CreatedBy, &w.CreatedAt, &w.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	} else if err != nil {
		return nil, err
	}
	if pid.Valid {
		w.ProjectID = &pid.String
	}
	_ = json.Unmarshal(eventsRaw, &w.Events)
	return &w, nil
}

// UpdateWebhook updates an existing webhook's configuration.
func (s *Store) UpdateWebhook(ctx context.Context, webhookID, name, url, secret string, events []string, isActive bool) (*domain.Webhook, error) {
	name = strings.TrimSpace(name)
	url = strings.TrimSpace(url)
	if name == "" {
		return nil, fmt.Errorf("%w: webhook name is required", ErrBadRequest)
	}
	if url == "" || (!strings.HasPrefix(url, "https://") && !strings.HasPrefix(url, "http://")) {
		return nil, fmt.Errorf("%w: valid webhook URL is required", ErrBadRequest)
	}
	if events == nil {
		events = []string{}
	}
	eventsJSON, _ := json.Marshal(events)
	now := time.Now().UTC()

	res, err := s.db.ExecContext(ctx, `
		UPDATE webhooks
		SET name = $1, url = $2, secret = CASE WHEN $3 = '' THEN secret ELSE $3 END,
		    events = $4, is_active = $5, updated_at = $6
		WHERE id = $7
	`, name, url, secret, eventsJSON, isActive, now, webhookID)
	if err != nil {
		return nil, err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return nil, ErrNotFound
	}

	return s.GetWebhook(ctx, webhookID)
}

// DeleteWebhook deletes a webhook by ID.
func (s *Store) DeleteWebhook(ctx context.Context, webhookID string) error {
	res, err := s.db.ExecContext(ctx, `DELETE FROM webhooks WHERE id = $1`, webhookID)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

// CreateWebhookDelivery records the result of an HTTP delivery attempt.
func (s *Store) CreateWebhookDelivery(ctx context.Context, webhookID, event string, statusCode *int, durationMs int, reqBody any, respBody, errMsg *string) (*domain.WebhookDelivery, error) {
	deliveryID := id.NewV7String()
	reqJSON, _ := json.Marshal(reqBody)
	now := time.Now().UTC()

	_, err := s.db.ExecContext(ctx, `
		INSERT INTO webhook_deliveries (id, webhook_id, event, status_code, duration_ms, request_body, response_body, error_message, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
	`, deliveryID, webhookID, event, statusCode, durationMs, reqJSON, respBody, errMsg, now)
	if err != nil {
		return nil, err
	}

	return &domain.WebhookDelivery{
		ID:           deliveryID,
		WebhookID:    webhookID,
		Event:        event,
		StatusCode:   statusCode,
		DurationMs:   durationMs,
		RequestBody:  reqBody,
		ResponseBody: respBody,
		ErrorMessage: errMsg,
		CreatedAt:    now,
	}, nil
}

// ListWebhookDeliveries returns recent deliveries for a specific webhook.
func (s *Store) ListWebhookDeliveries(ctx context.Context, webhookID string, limit int) ([]domain.WebhookDelivery, error) {
	if limit <= 0 || limit > 50 {
		limit = 20
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, webhook_id, event, status_code, duration_ms, request_body, response_body, error_message, created_at
		FROM webhook_deliveries
		WHERE webhook_id = $1
		ORDER BY created_at DESC
		LIMIT $2
	`, webhookID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	list := []domain.WebhookDelivery{}
	for rows.Next() {
		var d domain.WebhookDelivery
		var reqRaw []byte
		if err := rows.Scan(&d.ID, &d.WebhookID, &d.Event, &d.StatusCode, &d.DurationMs, &reqRaw, &d.ResponseBody, &d.ErrorMessage, &d.CreatedAt); err != nil {
			return nil, err
		}
		var parsed any
		_ = json.Unmarshal(reqRaw, &parsed)
		d.RequestBody = parsed
		list = append(list, d)
	}
	return list, rows.Err()
}

// DispatchWebhooks dispatches an event asynchronously to all active matching webhooks.
func (s *Store) DispatchWebhooks(ctx context.Context, projectID *string, event string, payload any) {
	go func() {
		bgCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		var rows *sql.Rows
		var err error
		if projectID == nil {
			rows, err = s.db.QueryContext(bgCtx, `
				SELECT id, url, secret, events
				FROM webhooks
				WHERE is_active = true AND project_id IS NULL
			`)
		} else {
			rows, err = s.db.QueryContext(bgCtx, `
				SELECT id, url, secret, events
				FROM webhooks
				WHERE is_active = true AND (project_id = $1 OR project_id IS NULL)
			`, *projectID)
		}
		if err != nil {
			return
		}
		defer rows.Close()

		type hookInfo struct {
			id     string
			url    string
			secret string
			events []string
		}
		var hooks []hookInfo
		for rows.Next() {
			var h hookInfo
			var eventsRaw []byte
			if err := rows.Scan(&h.id, &h.url, &h.secret, &eventsRaw); err == nil {
				_ = json.Unmarshal(eventsRaw, &h.events)
				hooks = append(hooks, h)
			}
		}

		for _, h := range hooks {
			// Check if subscribed
			subscribed := false
			for _, e := range h.events {
				if e == event || e == "*" {
					subscribed = true
					break
				}
			}
			if !subscribed {
				continue
			}

			s.sendWebhookDelivery(bgCtx, h.id, h.url, h.secret, event, payload)
		}
	}()
}

// TestWebhook sends a test ping delivery synchronously and records the delivery result.
func (s *Store) TestWebhook(ctx context.Context, webhookID string) (*domain.WebhookDelivery, error) {
	hook, err := s.GetWebhook(ctx, webhookID)
	if err != nil {
		return nil, err
	}

	testPayload := map[string]any{
		"event":     "ping",
		"event_id":  id.NewV7String(),
		"timestamp": time.Now().UTC().Format(time.RFC3339),
		"message":   "This is a test webhook delivery from Polaris.",
		"webhook": map[string]any{
			"id":   hook.ID,
			"name": hook.Name,
		},
	}

	return s.sendWebhookDelivery(ctx, hook.ID, hook.URL, hook.Secret, "ping", testPayload)
}

func (s *Store) sendWebhookDelivery(ctx context.Context, webhookID, url, secret, event string, payload any) (*domain.WebhookDelivery, error) {
	bodyBytes, err := json.Marshal(payload)
	if err != nil {
		errMsg := err.Error()
		return s.CreateWebhookDelivery(ctx, webhookID, event, nil, 0, payload, nil, &errMsg)
	}

	// Compute HMAC-SHA256 signature
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(bodyBytes)
	sig := "sha256=" + hex.EncodeToString(mac.Sum(nil))

	deliveryID := id.NewV7String()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(bodyBytes))
	if err != nil {
		errMsg := err.Error()
		return s.CreateWebhookDelivery(ctx, webhookID, event, nil, 0, payload, nil, &errMsg)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "Polaris-Webhook/1.0")
	req.Header.Set("X-Polaris-Event", event)
	req.Header.Set("X-Polaris-Delivery", deliveryID)
	req.Header.Set("X-Polaris-Signature", sig)

	client := &http.Client{Timeout: 10 * time.Second}
	startTime := time.Now()
	resp, reqErr := client.Do(req)
	durationMs := int(time.Since(startTime).Milliseconds())

	if reqErr != nil {
		errMsg := reqErr.Error()
		return s.CreateWebhookDelivery(ctx, webhookID, event, nil, durationMs, payload, nil, &errMsg)
	}
	defer resp.Body.Close()

	respBodyBytes, _ := io.ReadAll(io.LimitReader(resp.Body, 1024*32)) // max 32KB
	respBodyStr := string(respBodyBytes)

	return s.CreateWebhookDelivery(ctx, webhookID, event, &resp.StatusCode, durationMs, payload, &respBodyStr, nil)
}
