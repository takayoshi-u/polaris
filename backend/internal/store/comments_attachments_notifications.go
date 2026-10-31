package store

import (
	"bytes"
	"context"
	"crypto/sha512"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/takayoshi-u/polaris/backend/internal/domain"
	"github.com/takayoshi-u/polaris/backend/internal/pkg/id"
)

func (s *Store) verifyTargetInProject(ctx context.Context, projectID, targetType, targetID string) (ownerUserID string, title string, err error) {
	if targetType == domain.TargetIssue {
		err = s.db.QueryRowContext(ctx, `SELECT owner_user_id, title FROM issues WHERE id = $1 AND project_id = $2`, targetID, projectID).Scan(&ownerUserID, &title)
	} else if targetType == domain.TargetTask {
		err = s.db.QueryRowContext(ctx, `SELECT owner_user_id, title FROM tasks WHERE id = $1 AND project_id = $2`, targetID, projectID).Scan(&ownerUserID, &title)
	} else {
		return "", "", fmt.Errorf("%w: target_type must be 'issue' or 'task'", ErrBadRequest)
	}
	if errors.Is(err, sql.ErrNoRows) {
		return "", "", ErrNotFound
	}
	return ownerUserID, title, err
}

// Comments
func (s *Store) ListComments(ctx context.Context, projectID, targetType, targetID, viewerUserID string) ([]domain.Comment, error) {
	proj, err := s.GetProject(ctx, projectID, viewerUserID)
	if err != nil {
		return nil, err
	}
	if !proj.IsMember {
		return nil, ErrForbidden
	}
	if _, _, err := s.verifyTargetInProject(ctx, projectID, targetType, targetID); err != nil {
		return nil, err
	}

	rows, err := s.db.QueryContext(ctx, `
		SELECT id, target_type, target_id, author_user_id, body, created_at, updated_at
		FROM comments WHERE target_type = $1 AND target_id = $2 ORDER BY created_at ASC, id ASC
	`, targetType, targetID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	comments := []domain.Comment{}
	for rows.Next() {
		var c domain.Comment
		if err := rows.Scan(&c.ID, &c.TargetType, &c.TargetID, &c.AuthorUserID, &c.Body, &c.CreatedAt, &c.UpdatedAt); err != nil {
			return nil, err
		}
		c.Mentions, _ = s.getCommentMentions(ctx, c.ID)
		comments = append(comments, c)
	}
	return comments, rows.Err()
}

func (s *Store) getCommentMentions(ctx context.Context, commentID string) ([]domain.SubjectRef, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT mention_type, mention_id FROM comment_mentions WHERE comment_id = $1
	`, commentID)
	if err != nil {
		return []domain.SubjectRef{}, err
	}
	defer rows.Close()
	mentions := []domain.SubjectRef{}
	for rows.Next() {
		var m domain.SubjectRef
		if err := rows.Scan(&m.Type, &m.ID); err == nil {
			mentions = append(mentions, m)
		}
	}
	return mentions, rows.Err()
}

func (s *Store) CreateComment(ctx context.Context, projectID, targetType, targetID, authorUserID, body string, mentions []domain.SubjectRef) (*domain.Comment, error) {
	body = strings.TrimSpace(body)
	if body == "" {
		return nil, fmt.Errorf("%w: comment body is required", ErrBadRequest)
	}
	proj, err := s.GetProject(ctx, projectID, authorUserID)
	if err != nil {
		return nil, err
	}
	if domain.RolePriority(proj.CurrentUserRole) < domain.RolePriority(domain.RoleEditor) {
		return nil, ErrForbidden
	}
	_, itemTitle, err := s.verifyTargetInProject(ctx, projectID, targetType, targetID)
	if err != nil {
		return nil, err
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	commentID := id.NewV7String()
	now := time.Now().UTC()
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO comments (id, target_type, target_id, author_user_id, body, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $6)
	`, commentID, targetType, targetID, authorUserID, body, now); err != nil {
		return nil, err
	}

	cleanMentions := []domain.SubjectRef{}
	for _, m := range mentions {
		if m.Type != domain.SubjectUser && m.Type != domain.SubjectGroup {
			continue
		}
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO comment_mentions (id, comment_id, mention_type, mention_id)
			VALUES ($1, $2, $3, $4)
			ON CONFLICT DO NOTHING
		`, id.NewV7String(), commentID, m.Type, m.ID); err != nil {
			return nil, err
		}
		cleanMentions = append(cleanMentions, m)
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}

	if len(cleanMentions) > 0 {
		_ = s.notifySubjects(ctx, projectID, domain.NotificationTypeMention, targetType, targetID, authorUserID, cleanMentions, fmt.Sprintf("「%s」のコメントでメンションされました", itemTitle))
	}

	return &domain.Comment{
		ID:           commentID,
		TargetType:   targetType,
		TargetID:     targetID,
		AuthorUserID: authorUserID,
		Body:         body,
		Mentions:     cleanMentions,
		CreatedAt:    now,
		UpdatedAt:    now,
	}, nil
}

func (s *Store) UpdateComment(ctx context.Context, projectID, commentID, actorUserID, body string, mentions []domain.SubjectRef) (*domain.Comment, error) {
	body = strings.TrimSpace(body)
	if body == "" {
		return nil, fmt.Errorf("%w: comment body is required", ErrBadRequest)
	}
	proj, err := s.GetProject(ctx, projectID, actorUserID)
	if err != nil {
		return nil, err
	}
	if !proj.IsMember {
		return nil, ErrForbidden
	}

	var c domain.Comment
	err = s.db.QueryRowContext(ctx, `
		SELECT id, target_type, target_id, author_user_id, created_at
		FROM comments WHERE id = $1
	`, commentID).Scan(&c.ID, &c.TargetType, &c.TargetID, &c.AuthorUserID, &c.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	if _, _, err := s.verifyTargetInProject(ctx, projectID, c.TargetType, c.TargetID); err != nil {
		return nil, err
	}
	if c.AuthorUserID != actorUserID {
		return nil, fmt.Errorf("%w: only the comment author can edit this comment", ErrForbidden)
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	now := time.Now().UTC()
	if _, err := tx.ExecContext(ctx, `UPDATE comments SET body = $1, updated_at = $2 WHERE id = $3`, body, now, commentID); err != nil {
		return nil, err
	}
	if mentions != nil {
		if _, err := tx.ExecContext(ctx, `DELETE FROM comment_mentions WHERE comment_id = $1`, commentID); err != nil {
			return nil, err
		}
		for _, m := range mentions {
			if m.Type == domain.SubjectUser || m.Type == domain.SubjectGroup {
				_, _ = tx.ExecContext(ctx, `
					INSERT INTO comment_mentions (id, comment_id, mention_type, mention_id)
					VALUES ($1, $2, $3, $4)
					ON CONFLICT DO NOTHING
				`, id.NewV7String(), commentID, m.Type, m.ID)
			}
		}
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}

	c.Body = body
	c.UpdatedAt = now
	c.Mentions, _ = s.getCommentMentions(ctx, commentID)
	return &c, nil
}

func (s *Store) DeleteComment(ctx context.Context, projectID, commentID, actorUserID string) error {
	proj, err := s.GetProject(ctx, projectID, actorUserID)
	if err != nil {
		return err
	}
	if !proj.IsMember {
		return ErrForbidden
	}

	var targetType, targetID, authorID string
	err = s.db.QueryRowContext(ctx, `
		SELECT target_type, target_id, author_user_id FROM comments WHERE id = $1
	`, commentID).Scan(&targetType, &targetID, &authorID)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return err
	}
	if _, _, err := s.verifyTargetInProject(ctx, projectID, targetType, targetID); err != nil {
		return err
	}
	if authorID != actorUserID && proj.CurrentUserRole != domain.RoleAdmin {
		return fmt.Errorf("%w: only comment author or project admin can delete this comment", ErrForbidden)
	}

	_, err = s.db.ExecContext(ctx, `DELETE FROM comments WHERE id = $1`, commentID)
	return err
}

// Unified Attachments (max 100 per issue/task, max 1GB per file, SHA-512 deduplication, 2MB chunked storage)
func (s *Store) ListAttachments(ctx context.Context, projectID, targetType, targetID, viewerUserID string) ([]domain.Attachment, error) {
	proj, err := s.GetProject(ctx, projectID, viewerUserID)
	if err != nil {
		return nil, err
	}
	if !proj.IsMember {
		return nil, ErrForbidden
	}
	if _, _, err := s.verifyTargetInProject(ctx, projectID, targetType, targetID); err != nil {
		return nil, err
	}

	rows, err := s.db.QueryContext(ctx, `
		SELECT a.id, a.target_type, a.target_id, a.blob_id, b.sha512_hash,
		       a.file_name, b.size_bytes, b.mime_type, a.uploaded_by, a.created_at
		FROM attachments a
		JOIN file_blobs b ON b.id = a.blob_id
		WHERE a.target_type = $1 AND a.target_id = $2
		ORDER BY a.created_at ASC, a.id ASC
	`, targetType, targetID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	list := []domain.Attachment{}
	for rows.Next() {
		var att domain.Attachment
		if err := rows.Scan(&att.ID, &att.TargetType, &att.TargetID, &att.BlobID, &att.SHA512Hash, &att.FileName, &att.SizeBytes, &att.MimeType, &att.UploadedBy, &att.CreatedAt); err != nil {
			return nil, err
		}
		list = append(list, att)
	}
	return list, rows.Err()
}

// UploadAttachment uploads a file (whether from Issue/Task form or from Comment editor, always linking to the Issue or Task),
// enforces the 100-attachment limit and 1GB size limit, and deduplicates binary storage by SHA-512 hash.
func (s *Store) UploadAttachment(ctx context.Context, projectID, targetType, targetID, uploaderUserID, fileName, mimeType string, reader io.Reader) (*domain.Attachment, error) {
	fileName = strings.TrimSpace(fileName)
	if fileName == "" {
		return nil, fmt.Errorf("%w: file_name is required", ErrBadRequest)
	}
	if mimeType == "" {
		mimeType = "application/octet-stream"
	}

	proj, err := s.GetProject(ctx, projectID, uploaderUserID)
	if err != nil {
		return nil, err
	}
	if domain.RolePriority(proj.CurrentUserRole) < domain.RolePriority(domain.RoleEditor) {
		return nil, ErrForbidden
	}
	if _, _, err := s.verifyTargetInProject(ctx, projectID, targetType, targetID); err != nil {
		return nil, err
	}

	var currentCount int
	if err := s.db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM attachments WHERE target_type = $1 AND target_id = $2
	`, targetType, targetID).Scan(&currentCount); err != nil {
		return nil, err
	}
	if currentCount >= domain.MaxAttachmentsPerItem {
		return nil, fmt.Errorf("%w: maximum %d attachments per %s reached", ErrBadRequest, domain.MaxAttachmentsPerItem, targetType)
	}

	// Read in chunks while calculating SHA-512 and enforcing the 1GB max size
	hasher := sha512.New()
	var chunks [][]byte
	var totalBytes int64
	buf := make([]byte, domain.FileChunkSizeBytes)

	for {
		n, readErr := io.ReadFull(reader, buf)
		if n > 0 {
			totalBytes += int64(n)
			if totalBytes > domain.MaxAttachmentBytes {
				return nil, fmt.Errorf("%w: file size exceeds maximum limit of 1GB", ErrBadRequest)
			}
			chunkCopy := make([]byte, n)
			copy(chunkCopy, buf[:n])
			hasher.Write(chunkCopy)
			chunks = append(chunks, chunkCopy)
		}
		if errors.Is(readErr, io.EOF) || errors.Is(readErr, io.ErrUnexpectedEOF) {
			break
		}
		if readErr != nil {
			return nil, readErr
		}
	}

	sha512Hex := hex.EncodeToString(hasher.Sum(nil))
	now := time.Now().UTC()

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	// Check if a blob with this SHA-512 already exists (Deduplication)
	var blobID string
	err = tx.QueryRowContext(ctx, `SELECT id FROM file_blobs WHERE sha512_hash = $1`, sha512Hex).Scan(&blobID)
	if errors.Is(err, sql.ErrNoRows) {
		blobID = id.NewV7String()
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO file_blobs (id, sha512_hash, size_bytes, mime_type, created_at)
			VALUES ($1, $2, $3, $4, $5)
		`, blobID, sha512Hex, totalBytes, mimeType, now); err != nil {
			return nil, err
		}
		for idx, ch := range chunks {
			if _, err := tx.ExecContext(ctx, `
				INSERT INTO file_chunks (id, blob_id, chunk_index, data)
				VALUES ($1, $2, $3, $4)
			`, id.NewV7String(), blobID, idx, ch); err != nil {
				return nil, err
			}
		}
	} else if err != nil {
		return nil, err
	}

	attID := id.NewV7String()
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO attachments (id, target_type, target_id, blob_id, file_name, uploaded_by, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`, attID, targetType, targetID, blobID, fileName, uploaderUserID, now); err != nil {
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}

	return &domain.Attachment{
		ID:         attID,
		TargetType: targetType,
		TargetID:   targetID,
		BlobID:     blobID,
		SHA512Hash: sha512Hex,
		FileName:   fileName,
		SizeBytes:  totalBytes,
		MimeType:   mimeType,
		UploadedBy: uploaderUserID,
		CreatedAt:  now,
	}, nil
}

// DownloadAttachment streams the chunks of an attachment to writer if viewerUserID is a project member.
func (s *Store) DownloadAttachment(ctx context.Context, projectID, attachmentID, viewerUserID string, w io.Writer) (*domain.Attachment, error) {
	proj, err := s.GetProject(ctx, projectID, viewerUserID)
	if err != nil {
		return nil, err
	}
	if !proj.IsMember {
		return nil, ErrForbidden
	}

	var att domain.Attachment
	err = s.db.QueryRowContext(ctx, `
		SELECT a.id, a.target_type, a.target_id, a.blob_id, b.sha512_hash,
		       a.file_name, b.size_bytes, b.mime_type, a.uploaded_by, a.created_at
		FROM attachments a
		JOIN file_blobs b ON b.id = a.blob_id
		WHERE a.id = $1
	`, attachmentID).Scan(&att.ID, &att.TargetType, &att.TargetID, &att.BlobID, &att.SHA512Hash, &att.FileName, &att.SizeBytes, &att.MimeType, &att.UploadedBy, &att.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	if _, _, err := s.verifyTargetInProject(ctx, projectID, att.TargetType, att.TargetID); err != nil {
		return nil, err
	}

	if w != nil {
		rows, err := s.db.QueryContext(ctx, `
			SELECT data FROM file_chunks WHERE blob_id = $1 ORDER BY chunk_index ASC
		`, att.BlobID)
		if err != nil {
			return nil, err
		}
		defer rows.Close()
		for rows.Next() {
			var chunk []byte
			if err := rows.Scan(&chunk); err != nil {
				return nil, err
			}
			if _, err := io.Copy(w, bytes.NewReader(chunk)); err != nil {
				return nil, err
			}
		}
		if err := rows.Err(); err != nil {
			return nil, err
		}
	}
	return &att, nil
}

// DeleteAttachment deletes an attachment if actorUserID is the uploader, the Issue/Task owner, or a project Admin.
// Automatically deletes the underlying file_blob if no other attachments reference it.
func (s *Store) DeleteAttachment(ctx context.Context, projectID, attachmentID, actorUserID string) error {
	proj, err := s.GetProject(ctx, projectID, actorUserID)
	if err != nil {
		return err
	}
	if !proj.IsMember {
		return ErrForbidden
	}

	var targetType, targetID, blobID, uploadedBy string
	err = s.db.QueryRowContext(ctx, `
		SELECT target_type, target_id, blob_id, uploaded_by FROM attachments WHERE id = $1
	`, attachmentID).Scan(&targetType, &targetID, &blobID, &uploadedBy)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return err
	}

	itemOwnerID, _, err := s.verifyTargetInProject(ctx, projectID, targetType, targetID)
	if err != nil {
		return err
	}

	if actorUserID != uploadedBy && actorUserID != itemOwnerID && proj.CurrentUserRole != domain.RoleAdmin {
		return fmt.Errorf("%w: only uploader, item owner, or project admin can delete this attachment", ErrForbidden)
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx, `DELETE FROM attachments WHERE id = $1`, attachmentID); err != nil {
		return err
	}
	if err := cleanupUnreferencedBlobTx(ctx, tx, blobID); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) deleteAllAttachmentsForTarget(ctx context.Context, targetType, targetID string) error {
	rows, err := s.db.QueryContext(ctx, `SELECT DISTINCT blob_id FROM attachments WHERE target_type = $1 AND target_id = $2`, targetType, targetID)
	if err != nil {
		return err
	}
	var blobIDs []string
	for rows.Next() {
		var bid string
		if err := rows.Scan(&bid); err == nil {
			blobIDs = append(blobIDs, bid)
		}
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return err
	}
	rows.Close()

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx, `DELETE FROM attachments WHERE target_type = $1 AND target_id = $2`, targetType, targetID); err != nil {
		return err
	}
	for _, bid := range blobIDs {
		_ = cleanupUnreferencedBlobTx(ctx, tx, bid)
	}
	return tx.Commit()
}

func cleanupUnreferencedBlobTx(ctx context.Context, tx *sql.Tx, blobID string) error {
	var refCount int
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM attachments WHERE blob_id = $1`, blobID).Scan(&refCount); err != nil {
		return err
	}
	if refCount == 0 {
		_, err := tx.ExecContext(ctx, `DELETE FROM file_blobs WHERE id = $1`, blobID)
		return err
	}
	return nil
}

// Notifications & FCM
func (s *Store) notifySubjects(ctx context.Context, projectID, notificationType, targetType, targetID, actorUserID string, subjects []domain.SubjectRef, message string) error {
	recipientSet := make(map[string]bool)
	for _, sub := range subjects {
		if sub.Type == domain.SubjectUser {
			if sub.ID != actorUserID {
				recipientSet[sub.ID] = true
			}
		} else if sub.Type == domain.SubjectGroup {
			memberIDs, err := s.getGroupMemberIDs(ctx, sub.ID)
			if err == nil {
				for _, uid := range memberIDs {
					if uid != actorUserID {
						recipientSet[uid] = true
					}
				}
			}
		}
	}

	if len(recipientSet) == 0 {
		return nil
	}

	now := time.Now().UTC()
	var allFCMTokens []string
	for uid := range recipientSet {
		// Verify user is active
		var status string
		if err := s.db.QueryRowContext(ctx, `SELECT status FROM users WHERE id = $1`, uid).Scan(&status); err != nil || status != domain.UserStatusActive {
			continue
		}
		_, _ = s.db.ExecContext(ctx, `
			INSERT INTO notifications (id, user_id, notification_type, target_type, target_id, project_id, message, is_read, created_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7, FALSE, $8)
		`, id.NewV7String(), uid, notificationType, targetType, targetID, projectID, message, now)

		tRows, err := s.db.QueryContext(ctx, `SELECT token FROM user_fcm_tokens WHERE user_id = $1`, uid)
		if err == nil {
			for tRows.Next() {
				var tok string
				if err := tRows.Scan(&tok); err == nil && tok != "" {
					allFCMTokens = append(allFCMTokens, tok)
				}
			}
			_ = tRows.Err()
			tRows.Close()
		}
	}

	if len(allFCMTokens) > 0 && s.fcmClient != nil {
		_ = s.fcmClient.SendMulticast(ctx, allFCMTokens, "Polaris 通知", message, map[string]string{
			"notification_type": notificationType,
			"target_type":       targetType,
			"target_id":         targetID,
			"project_id":        projectID,
		})
	}
	return nil
}

func (s *Store) ListNotifications(ctx context.Context, userID string, unreadOnly bool) ([]domain.Notification, error) {
	query := `
		SELECT id, user_id, notification_type, target_type, target_id, project_id, message, is_read, created_at
		FROM notifications WHERE user_id = $1
	`
	if unreadOnly {
		query += ` AND is_read = FALSE`
	}
	query += ` ORDER BY created_at DESC, id DESC LIMIT 100`

	rows, err := s.db.QueryContext(ctx, query, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	list := []domain.Notification{}
	for rows.Next() {
		var n domain.Notification
		if err := rows.Scan(&n.ID, &n.UserID, &n.NotificationType, &n.TargetType, &n.TargetID, &n.ProjectID, &n.Message, &n.IsRead, &n.CreatedAt); err != nil {
			return nil, err
		}
		list = append(list, n)
	}
	return list, rows.Err()
}

func (s *Store) MarkNotificationRead(ctx context.Context, userID, notificationID string) error {
	res, err := s.db.ExecContext(ctx, `
		UPDATE notifications SET is_read = TRUE WHERE id = $1 AND user_id = $2
	`, notificationID, userID)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) MarkAllNotificationsRead(ctx context.Context, userID string) error {
	_, err := s.db.ExecContext(ctx, `UPDATE notifications SET is_read = TRUE WHERE user_id = $1`, userID)
	return err
}

func (s *Store) RegisterFCMToken(ctx context.Context, userID, token, deviceName string) error {
	token = strings.TrimSpace(token)
	if token == "" {
		return fmt.Errorf("%w: token is required", ErrBadRequest)
	}
	now := time.Now().UTC()
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO user_fcm_tokens (id, user_id, token, device_name, updated_at)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (token)
		DO UPDATE SET user_id = EXCLUDED.user_id, device_name = EXCLUDED.device_name, updated_at = EXCLUDED.updated_at
	`, id.NewV7String(), userID, token, strings.TrimSpace(deviceName), now)
	return err
}

func (s *Store) RemoveFCMToken(ctx context.Context, userID, token string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM user_fcm_tokens WHERE user_id = $1 AND token = $2`, userID, token)
	return err
}
