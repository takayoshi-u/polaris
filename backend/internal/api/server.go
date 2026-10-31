package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/takayoshi-u/polaris/backend/internal/domain"
	"github.com/takayoshi-u/polaris/backend/internal/store"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

const SessionCookieName = "polaris_session"

type contextKey string

const (
	userContextKey      contextKey = "polaris_user"
	patScopesContextKey contextKey = "polaris_pat_scopes"
)

type GoogleOAuthConfig struct {
	ClientID     string
	ClientSecret string
	RedirectURL  string
}

type Server struct {
	store             *store.Store
	corsAllowedOrigin string
	oauthCfg          *oauth2.Config
	mux               *http.ServeMux
}

func NewServer(st *store.Store, corsAllowedOrigin string, googleCfg GoogleOAuthConfig) *Server {
	if corsAllowedOrigin == "" {
		corsAllowedOrigin = "http://localhost:3000"
	}
	var ocfg *oauth2.Config
	if googleCfg.ClientID != "" {
		ocfg = &oauth2.Config{
			ClientID:     googleCfg.ClientID,
			ClientSecret: googleCfg.ClientSecret,
			RedirectURL:  googleCfg.RedirectURL,
			Scopes:       []string{"openid", "email", "profile"},
			Endpoint:     google.Endpoint,
		}
	}
	s := &Server{
		store:             st,
		corsAllowedOrigin: corsAllowedOrigin,
		oauthCfg:          ocfg,
		mux:               http.NewServeMux(),
	}
	s.registerRoutes()
	return s
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// CORS headers
	w.Header().Set("Access-Control-Allow-Origin", s.corsAllowedOrigin)
	w.Header().Set("Access-Control-Allow-Credentials", "true")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	s.mux.ServeHTTP(w, r)
}

func (s *Server) registerRoutes() {
	// Health & Initial Setup
	s.mux.HandleFunc("GET /api/v1/health", s.handleHealth)
	s.mux.HandleFunc("GET /api/v1/setup/status", s.handleSetupStatus)
	s.mux.HandleFunc("POST /api/v1/setup/init", s.handleSetupInit)

	// Public Auth & Token Verification
	s.mux.HandleFunc("POST /api/v1/auth/login", s.handleLogin)
	s.mux.HandleFunc("GET /api/v1/auth/google/url", s.handleGoogleAuthURL)
	s.mux.HandleFunc("POST /api/v1/auth/google/callback", s.handleGoogleCallback)
	s.mux.HandleFunc("POST /api/v1/auth/logout", s.handleLogout)
	s.mux.HandleFunc("POST /api/v1/auth/invite/accept", s.handleAcceptInvite)
	s.mux.HandleFunc("POST /api/v1/auth/password-reset/request", s.handleRequestPasswordReset)
	s.mux.HandleFunc("POST /api/v1/auth/password-reset/confirm", s.handleConfirmPasswordReset)
	s.mux.HandleFunc("POST /api/v1/auth/email-change/confirm", s.handleConfirmEmailChange)

	// Authenticated Account Endpoints
	s.mux.HandleFunc("GET /api/v1/auth/me", s.requireAuth(s.handleGetMe))
	s.mux.HandleFunc("PUT /api/v1/auth/me/profile", s.requireAuth(s.handleUpdateMyProfile))
	s.mux.HandleFunc("PUT /api/v1/auth/me/password", s.requireAuth(s.handleUpdateMyPassword))
	s.mux.HandleFunc("POST /api/v1/auth/me/email-change", s.requireAuth(s.handleRequestMyEmailChange))

	// System Management & Users
	s.mux.HandleFunc("GET /api/v1/system/settings", s.requireAuth(s.handleGetSystemSettings))
	s.mux.HandleFunc("PUT /api/v1/system/settings", s.requireSystemPerm(domain.PermManageSystem, s.handleUpdateSystemSettings))
	s.mux.HandleFunc("GET /api/v1/system/smtp", s.requireSystemPerm(domain.PermManageSystem, s.handleGetSMTPSettings))
	s.mux.HandleFunc("PUT /api/v1/system/smtp", s.requireSystemPerm(domain.PermManageSystem, s.handleUpdateSMTPSettings))
	s.mux.HandleFunc("POST /api/v1/system/smtp/test", s.requireSystemPerm(domain.PermManageSystem, s.handleSendTestEmail))
	s.mux.HandleFunc("GET /api/v1/system/permissions", s.requireSystemPerm(domain.PermManageSystem, s.handleListSystemPermissions))
	s.mux.HandleFunc("POST /api/v1/system/permissions", s.requireSystemPerm(domain.PermManageSystem, s.handleGrantSystemPermission))
	s.mux.HandleFunc("DELETE /api/v1/system/permissions/{id}", s.requireSystemPerm(domain.PermManageSystem, s.handleRevokeSystemPermission))

	s.mux.HandleFunc("GET /api/v1/users", s.requireAuth(s.handleListUsers))
	s.mux.HandleFunc("POST /api/v1/users", s.requireSystemPerm(domain.PermManageSystem, s.handleCreateUser))
	s.mux.HandleFunc("POST /api/v1/users/invite", s.requireSystemPerm(domain.PermManageSystem, s.handleInviteUser))
	s.mux.HandleFunc("POST /api/v1/users/bulk-invite", s.requireSystemPerm(domain.PermManageSystem, s.handleBulkInviteUsers))
	s.mux.HandleFunc("POST /api/v1/users/{userId}/resend-invite", s.requireSystemPerm(domain.PermManageSystem, s.handleResendInvite))
	s.mux.HandleFunc("PUT /api/v1/users/{userId}/email", s.requireSystemPerm(domain.PermManageSystem, s.handleAdminChangeUserEmail))
	s.mux.HandleFunc("PUT /api/v1/users/{userId}/status", s.requireSystemPerm(domain.PermManageSystem, s.handleUpdateUserStatus))

	// System Default Statuses, Custom Fields, Templates
	s.mux.HandleFunc("GET /api/v1/system/statuses", s.requireAuth(s.handleListSystemStatuses))
	s.mux.HandleFunc("POST /api/v1/system/statuses", s.requireSystemPerm(domain.PermManageSystem, s.handleCreateSystemStatus))
	s.mux.HandleFunc("POST /api/v1/system/statuses/reset", s.requireSystemPerm(domain.PermManageSystem, s.handleResetSystemStatuses))
	s.mux.HandleFunc("PUT /api/v1/system/statuses/{statusId}", s.requireSystemPerm(domain.PermManageSystem, s.handleUpdateSystemStatus))
	s.mux.HandleFunc("DELETE /api/v1/system/statuses/{statusId}", s.requireSystemPerm(domain.PermManageSystem, s.handleDeleteSystemStatus))

	s.mux.HandleFunc("GET /api/v1/system/custom-fields", s.requireAuth(s.handleListSystemCustomFields))
	s.mux.HandleFunc("POST /api/v1/system/custom-fields", s.requireSystemPerm(domain.PermManageSystem, s.handleCreateSystemCustomField))
	s.mux.HandleFunc("PUT /api/v1/system/custom-fields/{fieldId}", s.requireSystemPerm(domain.PermManageSystem, s.handleUpdateSystemCustomField))
	s.mux.HandleFunc("DELETE /api/v1/system/custom-fields/{fieldId}", s.requireSystemPerm(domain.PermManageSystem, s.handleDeleteSystemCustomField))

	s.mux.HandleFunc("GET /api/v1/system/templates", s.requireAuth(s.handleListSystemTemplates))
	s.mux.HandleFunc("POST /api/v1/system/templates", s.requireSystemPerm(domain.PermManageSystem, s.handleCreateSystemTemplate))
	s.mux.HandleFunc("GET /api/v1/system/templates/{templateId}", s.requireAuth(s.handleGetSystemTemplate))
	s.mux.HandleFunc("PUT /api/v1/system/templates/{templateId}", s.requireSystemPerm(domain.PermManageSystem, s.handleUpdateSystemTemplate))
	s.mux.HandleFunc("DELETE /api/v1/system/templates/{templateId}", s.requireSystemPerm(domain.PermManageSystem, s.handleDeleteSystemTemplate))

	// Global Groups & System Common Labels
	s.mux.HandleFunc("GET /api/v1/groups", s.requireAuth(s.handleListGlobalGroups))
	s.mux.HandleFunc("POST /api/v1/groups", s.requireSystemPerm(domain.PermManageSystem, s.handleCreateGlobalGroup))
	s.mux.HandleFunc("PUT /api/v1/groups/{groupId}", s.requireSystemPerm(domain.PermManageSystem, s.handleUpdateGlobalGroup))
	s.mux.HandleFunc("DELETE /api/v1/groups/{groupId}", s.requireSystemPerm(domain.PermManageSystem, s.handleDeleteGlobalGroup))

	s.mux.HandleFunc("GET /api/v1/labels", s.requireAuth(s.handleListSystemLabels))
	s.mux.HandleFunc("POST /api/v1/labels", s.requireSystemPerm(domain.PermManageSystem, s.handleCreateSystemLabel))
	s.mux.HandleFunc("PUT /api/v1/labels/{labelId}", s.requireSystemPerm(domain.PermManageSystem, s.handleUpdateSystemLabel))
	s.mux.HandleFunc("DELETE /api/v1/labels/{labelId}", s.requireSystemPerm(domain.PermManageSystem, s.handleDeleteSystemLabel))

	// Projects & Project-scoped Resources
	s.mux.HandleFunc("GET /api/v1/projects", s.requireAuth(s.requireScope("read:projects", s.handleListProjects)))
	s.mux.HandleFunc("POST /api/v1/projects", s.requireSystemPerm(domain.PermCreateProject, s.handleCreateProject))
	s.mux.HandleFunc("GET /api/v1/projects/{projectId}", s.requireAuth(s.requireScope("read:projects", s.handleGetProject)))
	s.mux.HandleFunc("PUT /api/v1/projects/{projectId}", s.requireProjectRole(domain.RoleAdmin, s.requireScope("write:projects", s.handleUpdateProject)))
	s.mux.HandleFunc("DELETE /api/v1/projects/{projectId}", s.requireAuth(s.requireScope("write:projects", s.handleDeleteProject)))

	s.mux.HandleFunc("GET /api/v1/projects/{projectId}/members", s.requireProjectRole(domain.RoleViewer, s.requireScope("read:projects", s.handleListProjectMembers)))
	s.mux.HandleFunc("POST /api/v1/projects/{projectId}/members", s.requireProjectRole(domain.RoleAdmin, s.requireScope("write:projects", s.handleUpsertProjectMember)))
	s.mux.HandleFunc("DELETE /api/v1/projects/{projectId}/members/{memberId}", s.requireProjectRole(domain.RoleAdmin, s.requireScope("write:projects", s.handleRemoveProjectMember)))

	s.mux.HandleFunc("GET /api/v1/projects/{projectId}/groups", s.requireProjectRole(domain.RoleViewer, s.requireScope("read:projects", s.handleListProjectGroups)))
	s.mux.HandleFunc("POST /api/v1/projects/{projectId}/groups", s.requireProjectRole(domain.RoleAdmin, s.requireScope("write:projects", s.handleCreateProjectGroup)))
	s.mux.HandleFunc("PUT /api/v1/projects/{projectId}/groups/{groupId}", s.requireProjectRole(domain.RoleAdmin, s.requireScope("write:projects", s.handleUpdateProjectGroup)))
	s.mux.HandleFunc("DELETE /api/v1/projects/{projectId}/groups/{groupId}", s.requireProjectRole(domain.RoleAdmin, s.requireScope("write:projects", s.handleDeleteProjectGroup)))

	s.mux.HandleFunc("GET /api/v1/projects/{projectId}/labels", s.requireProjectRole(domain.RoleViewer, s.requireScope("read:projects", s.handleListProjectLabels)))
	s.mux.HandleFunc("POST /api/v1/projects/{projectId}/labels", s.requireProjectRole(domain.RoleEditor, s.requireScope("write:projects", s.handleCreateProjectLabel)))
	s.mux.HandleFunc("PUT /api/v1/projects/{projectId}/labels/{labelId}", s.requireProjectRole(domain.RoleEditor, s.requireScope("write:projects", s.handleUpdateProjectLabel)))
	s.mux.HandleFunc("DELETE /api/v1/projects/{projectId}/labels/{labelId}", s.requireProjectRole(domain.RoleAdmin, s.requireScope("write:projects", s.handleDeleteProjectLabel)))

	s.mux.HandleFunc("GET /api/v1/projects/{projectId}/statuses", s.requireProjectRole(domain.RoleViewer, s.requireScope("read:projects", s.handleListProjectStatuses)))
	s.mux.HandleFunc("POST /api/v1/projects/{projectId}/statuses", s.requireProjectRole(domain.RoleAdmin, s.requireScope("write:projects", s.handleCreateProjectStatus)))
	s.mux.HandleFunc("PUT /api/v1/projects/{projectId}/statuses/{statusId}", s.requireProjectRole(domain.RoleAdmin, s.requireScope("write:projects", s.handleUpdateProjectStatus)))
	s.mux.HandleFunc("DELETE /api/v1/projects/{projectId}/statuses/{statusId}", s.requireProjectRole(domain.RoleAdmin, s.requireScope("write:projects", s.handleDeleteProjectStatus)))

	s.mux.HandleFunc("GET /api/v1/projects/{projectId}/custom-fields", s.requireProjectRole(domain.RoleViewer, s.requireScope("read:projects", s.handleListProjectCustomFields)))
	s.mux.HandleFunc("POST /api/v1/projects/{projectId}/custom-fields", s.requireProjectRole(domain.RoleAdmin, s.requireScope("write:projects", s.handleCreateProjectCustomField)))
	s.mux.HandleFunc("PUT /api/v1/projects/{projectId}/custom-fields/{fieldId}", s.requireProjectRole(domain.RoleAdmin, s.requireScope("write:projects", s.handleUpdateProjectCustomField)))
	s.mux.HandleFunc("DELETE /api/v1/projects/{projectId}/custom-fields/{fieldId}", s.requireProjectRole(domain.RoleAdmin, s.requireScope("write:projects", s.handleDeleteProjectCustomField)))

	s.mux.HandleFunc("GET /api/v1/projects/{projectId}/templates", s.requireProjectRole(domain.RoleViewer, s.requireScope("read:projects", s.handleListProjectTemplates)))
	s.mux.HandleFunc("POST /api/v1/projects/{projectId}/templates", s.requireProjectRole(domain.RoleAdmin, s.requireScope("write:projects", s.handleCreateProjectTemplate)))
	s.mux.HandleFunc("GET /api/v1/projects/{projectId}/templates/{templateId}", s.requireProjectRole(domain.RoleViewer, s.requireScope("read:projects", s.handleGetProjectTemplate)))
	s.mux.HandleFunc("PUT /api/v1/projects/{projectId}/templates/{templateId}", s.requireProjectRole(domain.RoleAdmin, s.requireScope("write:projects", s.handleUpdateProjectTemplate)))
	s.mux.HandleFunc("DELETE /api/v1/projects/{projectId}/templates/{templateId}", s.requireProjectRole(domain.RoleAdmin, s.requireScope("write:projects", s.handleDeleteProjectTemplate)))

	// Issues & Tasks
	s.mux.HandleFunc("GET /api/v1/projects/{projectId}/issues", s.requireProjectRole(domain.RoleViewer, s.requireScope("read:issues", s.handleListIssues)))
	s.mux.HandleFunc("POST /api/v1/projects/{projectId}/issues", s.requireProjectRole(domain.RoleEditor, s.requireScope("write:issues", s.handleCreateIssue)))
	s.mux.HandleFunc("GET /api/v1/projects/{projectId}/issues/{issueId}", s.requireProjectRole(domain.RoleViewer, s.requireScope("read:issues", s.handleGetIssue)))
	s.mux.HandleFunc("PUT /api/v1/projects/{projectId}/issues/{issueId}", s.requireProjectRole(domain.RoleEditor, s.requireScope("write:issues", s.handleUpdateIssue)))
	s.mux.HandleFunc("PUT /api/v1/projects/{projectId}/issues/{issueId}/owner", s.requireProjectRole(domain.RoleEditor, s.requireScope("write:issues", s.handleTransferIssueOwner)))
	s.mux.HandleFunc("DELETE /api/v1/projects/{projectId}/issues/{issueId}", s.requireProjectRole(domain.RoleEditor, s.requireScope("write:issues", s.handleDeleteIssue)))

	s.mux.HandleFunc("POST /api/v1/projects/{projectId}/tasks", s.requireProjectRole(domain.RoleEditor, s.requireScope("write:tasks", s.handleCreateTask)))
	s.mux.HandleFunc("GET /api/v1/projects/{projectId}/tasks/{taskId}", s.requireProjectRole(domain.RoleViewer, s.requireScope("read:tasks", s.handleGetTask)))
	s.mux.HandleFunc("PUT /api/v1/projects/{projectId}/tasks/{taskId}", s.requireProjectRole(domain.RoleEditor, s.requireScope("write:tasks", s.handleUpdateTask)))
	s.mux.HandleFunc("PUT /api/v1/projects/{projectId}/tasks/{taskId}/owner", s.requireProjectRole(domain.RoleEditor, s.requireScope("write:tasks", s.handleTransferTaskOwner)))
	s.mux.HandleFunc("PATCH /api/v1/projects/{projectId}/tasks/{taskId}/kanban-move", s.requireProjectRole(domain.RoleEditor, s.requireScope("write:tasks", s.handleMoveTaskKanban)))
	s.mux.HandleFunc("PATCH /api/v1/projects/{projectId}/tasks/{taskId}/gantt-dates", s.requireProjectRole(domain.RoleEditor, s.requireScope("write:tasks", s.handleUpdateTaskGanttDates)))
	s.mux.HandleFunc("DELETE /api/v1/projects/{projectId}/tasks/{taskId}", s.requireProjectRole(domain.RoleEditor, s.requireScope("write:tasks", s.handleDeleteTask)))

	// Comments & Unified Attachments
	s.mux.HandleFunc("GET /api/v1/projects/{projectId}/{targetType}/{targetId}/comments", s.requireProjectRole(domain.RoleViewer, s.requireScope("read:comments", s.handleListComments)))
	s.mux.HandleFunc("POST /api/v1/projects/{projectId}/{targetType}/{targetId}/comments", s.requireProjectRole(domain.RoleEditor, s.requireScope("write:comments", s.handleCreateComment)))
	s.mux.HandleFunc("PUT /api/v1/projects/{projectId}/comments/{commentId}", s.requireProjectRole(domain.RoleEditor, s.requireScope("write:comments", s.handleUpdateComment)))
	s.mux.HandleFunc("DELETE /api/v1/projects/{projectId}/comments/{commentId}", s.requireProjectRole(domain.RoleEditor, s.requireScope("write:comments", s.handleDeleteComment)))

	s.mux.HandleFunc("GET /api/v1/projects/{projectId}/{targetType}/{targetId}/attachments", s.requireProjectRole(domain.RoleViewer, s.requireScope("read:projects", s.handleListAttachments)))
	s.mux.HandleFunc("POST /api/v1/projects/{projectId}/{targetType}/{targetId}/attachments", s.requireProjectRole(domain.RoleEditor, s.requireScope("write:projects", s.handleUploadAttachment)))
	s.mux.HandleFunc("GET /api/v1/projects/{projectId}/attachments/{attachmentId}/download", s.requireProjectRole(domain.RoleViewer, s.requireScope("read:projects", s.handleDownloadAttachment)))
	s.mux.HandleFunc("DELETE /api/v1/projects/{projectId}/attachments/{attachmentId}", s.requireProjectRole(domain.RoleEditor, s.requireScope("write:projects", s.handleDeleteAttachment)))

	// Notifications & FCM Tokens
	s.mux.HandleFunc("GET /api/v1/notifications", s.requireAuth(s.handleListNotifications))
	s.mux.HandleFunc("PUT /api/v1/notifications/{notificationId}/read", s.requireAuth(s.handleMarkNotificationRead))
	s.mux.HandleFunc("PUT /api/v1/notifications/read-all", s.requireAuth(s.handleMarkAllNotificationsRead))
	s.mux.HandleFunc("POST /api/v1/fcm-tokens", s.requireAuth(s.handleRegisterFCMToken))
	s.mux.HandleFunc("DELETE /api/v1/fcm-tokens", s.requireAuth(s.handleRemoveFCMToken))

	// Personal Access Tokens (API Keys)
	s.mux.HandleFunc("GET /api/v1/user/tokens", s.requireAuth(s.handleListPATs))
	s.mux.HandleFunc("POST /api/v1/user/tokens", s.requireAuth(s.handleCreatePAT))
	s.mux.HandleFunc("DELETE /api/v1/user/tokens/{tokenId}", s.requireAuth(s.handleRevokePAT))

	// Project-level Outbound Webhooks
	s.mux.HandleFunc("GET /api/v1/projects/{projectId}/webhooks", s.requireProjectRole(domain.RoleAdmin, s.requireScope("manage:webhooks", s.handleListProjectWebhooks)))
	s.mux.HandleFunc("POST /api/v1/projects/{projectId}/webhooks", s.requireProjectRole(domain.RoleAdmin, s.requireScope("manage:webhooks", s.handleCreateProjectWebhook)))
	s.mux.HandleFunc("GET /api/v1/projects/{projectId}/webhooks/{webhookId}", s.requireProjectRole(domain.RoleAdmin, s.requireScope("manage:webhooks", s.handleGetWebhook)))
	s.mux.HandleFunc("PUT /api/v1/projects/{projectId}/webhooks/{webhookId}", s.requireProjectRole(domain.RoleAdmin, s.requireScope("manage:webhooks", s.handleUpdateWebhook)))
	s.mux.HandleFunc("DELETE /api/v1/projects/{projectId}/webhooks/{webhookId}", s.requireProjectRole(domain.RoleAdmin, s.requireScope("manage:webhooks", s.handleDeleteWebhook)))
	s.mux.HandleFunc("POST /api/v1/projects/{projectId}/webhooks/{webhookId}/test", s.requireProjectRole(domain.RoleAdmin, s.requireScope("manage:webhooks", s.handleTestWebhook)))
	s.mux.HandleFunc("GET /api/v1/projects/{projectId}/webhooks/{webhookId}/deliveries", s.requireProjectRole(domain.RoleAdmin, s.requireScope("manage:webhooks", s.handleListWebhookDeliveries)))

	// System-wide Outbound Webhooks
	s.mux.HandleFunc("GET /api/v1/system/webhooks", s.requireSystemPerm(domain.PermManageSystem, s.handleListSystemWebhooks))
	s.mux.HandleFunc("POST /api/v1/system/webhooks", s.requireSystemPerm(domain.PermManageSystem, s.handleCreateSystemWebhook))
	s.mux.HandleFunc("GET /api/v1/system/webhooks/{webhookId}", s.requireSystemPerm(domain.PermManageSystem, s.handleGetWebhook))
	s.mux.HandleFunc("PUT /api/v1/system/webhooks/{webhookId}", s.requireSystemPerm(domain.PermManageSystem, s.handleUpdateWebhook))
	s.mux.HandleFunc("DELETE /api/v1/system/webhooks/{webhookId}", s.requireSystemPerm(domain.PermManageSystem, s.handleDeleteWebhook))
	s.mux.HandleFunc("POST /api/v1/system/webhooks/{webhookId}/test", s.requireSystemPerm(domain.PermManageSystem, s.handleTestWebhook))
	s.mux.HandleFunc("GET /api/v1/system/webhooks/{webhookId}/deliveries", s.requireSystemPerm(domain.PermManageSystem, s.handleListWebhookDeliveries))

	// Task Dependencies
	s.mux.HandleFunc("GET /api/v1/projects/{projectId}/dependencies", s.requireProjectRole(domain.RoleViewer, s.requireScope("read:tasks", s.handleListTaskDependencies)))
	s.mux.HandleFunc("POST /api/v1/projects/{projectId}/dependencies", s.requireProjectRole(domain.RoleEditor, s.requireScope("write:tasks", s.handleCreateTaskDependency)))
	s.mux.HandleFunc("DELETE /api/v1/projects/{projectId}/dependencies/{depId}", s.requireProjectRole(domain.RoleEditor, s.requireScope("write:tasks", s.handleDeleteTaskDependency)))

	// Sprints
	s.mux.HandleFunc("GET /api/v1/projects/{projectId}/sprints", s.requireProjectRole(domain.RoleViewer, s.requireScope("read:projects", s.handleListSprints)))
	s.mux.HandleFunc("POST /api/v1/projects/{projectId}/sprints", s.requireProjectRole(domain.RoleEditor, s.requireScope("write:projects", s.handleCreateSprint)))
	s.mux.HandleFunc("GET /api/v1/sprints/{sprintId}", s.requireAuth(s.requireScope("read:projects", s.handleGetSprint)))
	s.mux.HandleFunc("PUT /api/v1/sprints/{sprintId}", s.requireAuth(s.requireScope("write:projects", s.handleUpdateSprint)))
	s.mux.HandleFunc("DELETE /api/v1/sprints/{sprintId}", s.requireAuth(s.requireScope("write:projects", s.handleDeleteSprint)))
	s.mux.HandleFunc("GET /api/v1/sprints/{sprintId}/burndown", s.requireAuth(s.requireScope("read:projects", s.handleGetSprintBurndown)))

	// Activity Logs
	s.mux.HandleFunc("GET /api/v1/projects/{projectId}/activity", s.requireProjectRole(domain.RoleViewer, s.requireScope("read:projects", s.handleListActivityLogs)))

	// Project Wiki
	s.mux.HandleFunc("GET /api/v1/projects/{projectId}/wiki", s.requireProjectRole(domain.RoleViewer, s.requireScope("read:projects", s.handleListWikiPages)))
	s.mux.HandleFunc("POST /api/v1/projects/{projectId}/wiki", s.requireProjectRole(domain.RoleEditor, s.requireScope("write:projects", s.handleCreateWikiPage)))
	s.mux.HandleFunc("GET /api/v1/wiki/{wikiId}", s.requireAuth(s.requireScope("read:projects", s.handleGetWikiPage)))
	s.mux.HandleFunc("PUT /api/v1/wiki/{wikiId}", s.requireAuth(s.requireScope("write:projects", s.handleUpdateWikiPage)))
	s.mux.HandleFunc("DELETE /api/v1/wiki/{wikiId}", s.requireAuth(s.requireScope("write:projects", s.handleDeleteWikiPage)))

	// Inbound Webhooks
	s.mux.HandleFunc("GET /api/v1/projects/{projectId}/inbound-webhooks", s.requireProjectRole(domain.RoleAdmin, s.requireScope("manage:webhooks", s.handleListInboundWebhooks)))
	s.mux.HandleFunc("POST /api/v1/projects/{projectId}/inbound-webhooks", s.requireProjectRole(domain.RoleAdmin, s.requireScope("manage:webhooks", s.handleCreateInboundWebhook)))
	s.mux.HandleFunc("DELETE /api/v1/inbound-webhooks/{webhookId}", s.requireAuth(s.requireScope("manage:webhooks", s.handleDeleteInboundWebhook)))
	s.mux.HandleFunc("POST /api/v1/inbound-webhooks/{token}", s.handleProcessInboundWebhook)

	// CSV Export & Import
	s.mux.HandleFunc("GET /api/v1/projects/{projectId}/export/csv", s.requireProjectRole(domain.RoleViewer, s.requireScope("read:projects", s.handleExportCSV)))
	s.mux.HandleFunc("POST /api/v1/projects/{projectId}/import/csv", s.requireProjectRole(domain.RoleEditor, s.requireScope("write:projects", s.handleImportCSV)))
}

func extractSessionToken(r *http.Request) string {
	authHeader := r.Header.Get("Authorization")
	if strings.HasPrefix(authHeader, "Bearer ") {
		return strings.TrimSpace(strings.TrimPrefix(authHeader, "Bearer "))
	}
	if c, err := r.Cookie(SessionCookieName); err == nil && c.Value != "" {
		return c.Value
	}
	return ""
}

func setSessionCookie(w http.ResponseWriter, token string) {
	http.SetCookie(w, &http.Cookie{
		Name:     SessionCookieName,
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Expires:  time.Now().UTC().Add(store.SessionTTL),
	})
}

func clearSessionCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     SessionCookieName,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		MaxAge:   -1,
	})
}

func currentUser(r *http.Request) *domain.User {
	u, _ := r.Context().Value(userContextKey).(*domain.User)
	return u
}

func hasScope(granted []string, required string) bool {
	for _, g := range granted {
		if g == "*" || g == "admin:system" || g == "manage:system" {
			return true
		}
		if g == required {
			return true
		}
		// Write implies read for same resource
		if strings.HasPrefix(g, "write:") && required == "read:"+strings.TrimPrefix(g, "write:") {
			return true
		}
		// Project-level scopes encompass issues, tasks, comments, attachments, and project webhooks
		if g == "write:projects" {
			if strings.HasPrefix(required, "read:") || strings.HasPrefix(required, "write:") || required == "manage:webhooks" {
				return true
			}
		}
		if g == "read:projects" {
			if strings.HasPrefix(required, "read:") {
				return true
			}
		}
		// Issue or task scopes allow comments and attachments
		if required == "read:comments" && (g == "read:issues" || g == "write:issues" || g == "read:tasks" || g == "write:tasks") {
			return true
		}
		if required == "write:comments" && (g == "write:issues" || g == "write:tasks") {
			return true
		}
		if required == "read:attachments" && (g == "read:issues" || g == "write:issues" || g == "read:tasks" || g == "write:tasks") {
			return true
		}
		if required == "write:attachments" && (g == "write:issues" || g == "write:tasks") {
			return true
		}
	}
	return false
}

func (s *Server) requireScope(scope string, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		scopes, ok := r.Context().Value(patScopesContextKey).([]string)
		if !ok {
			// Not authenticated via PAT (e.g. session cookie), bypass scope check
			next(w, r)
			return
		}
		if !hasScope(scopes, scope) {
			writeJSON(w, http.StatusForbidden, map[string]any{"error": "insufficient token scope: " + scope})
			return
		}
		next(w, r)
	}
}

func (s *Server) requireAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tok := extractSessionToken(r)
		if strings.HasPrefix(tok, "plr_pat_") {
			u, scopes, _, err := s.store.VerifyPAT(r.Context(), tok)
			if err != nil {
				writeError(w, err)
				return
			}
			ctx := context.WithValue(r.Context(), userContextKey, u)
			ctx = context.WithValue(ctx, patScopesContextKey, scopes)
			next(w, r.WithContext(ctx))
			return
		}

		u, err := s.store.GetUserBySessionToken(r.Context(), tok)
		if err != nil {
			writeError(w, err)
			return
		}
		ctx := context.WithValue(r.Context(), userContextKey, u)
		next(w, r.WithContext(ctx))
	}
}

func (s *Server) requireSystemPerm(perm string, next http.HandlerFunc) http.HandlerFunc {
	return s.requireAuth(func(w http.ResponseWriter, r *http.Request) {
		if scopes, ok := r.Context().Value(patScopesContextKey).([]string); ok {
			requiredScope := "manage:system"
			if perm == domain.PermCreateProject {
				requiredScope = "write:projects"
			}
			if !hasScope(scopes, requiredScope) {
				writeJSON(w, http.StatusForbidden, map[string]any{"error": "insufficient token scope: " + requiredScope})
				return
			}
		}
		u := currentUser(r)
		has, err := s.store.HasUserSystemPermission(r.Context(), u.ID, perm)
		if err != nil {
			writeError(w, err)
			return
		}
		if !has {
			writeJSON(w, http.StatusForbidden, map[string]any{"error": "missing system permission: " + perm})
			return
		}
		next(w, r)
	})
}

func (s *Server) requireProjectRole(minRole domain.ProjectRole, next http.HandlerFunc) http.HandlerFunc {
	return s.requireAuth(func(w http.ResponseWriter, r *http.Request) {
		u := currentUser(r)
		projectID := r.PathValue("projectId")
		role, err := s.store.GetEffectiveProjectRole(r.Context(), projectID, u.ID)
		if err != nil {
			writeError(w, err)
			return
		}
		if domain.RolePriority(role) < domain.RolePriority(minRole) {
			writeJSON(w, http.StatusForbidden, map[string]any{"error": "insufficient project role"})
			return
		}
		next(w, r)
	})
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func writeError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, store.ErrNonMemberDeleteWarning):
		writeJSON(w, http.StatusConflict, map[string]any{
			"error":              err.Error(),
			"warning_code":       "NOT_PROJECT_MEMBER",
			"is_member":          false,
			"confirmation_param": "confirm_non_member=true",
		})
	case errors.Is(err, store.ErrUnauthorized):
		writeJSON(w, http.StatusUnauthorized, map[string]any{"error": err.Error()})
	case errors.Is(err, store.ErrForbidden):
		writeJSON(w, http.StatusForbidden, map[string]any{"error": err.Error()})
	case errors.Is(err, store.ErrNotFound):
		writeJSON(w, http.StatusNotFound, map[string]any{"error": err.Error()})
	case errors.Is(err, store.ErrConflict):
		writeJSON(w, http.StatusConflict, map[string]any{"error": err.Error()})
	case errors.Is(err, store.ErrBadRequest):
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
	default:
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
	}
}

