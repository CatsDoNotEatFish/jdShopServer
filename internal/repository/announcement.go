package repository

import (
	"database/sql"
	"fmt"
	"strings"
	"time"

	"jdShopServer/internal/model"
)

type AnnouncementRepo struct {
	db *sql.DB
}

type rowScanner interface {
	Scan(dest ...any) error
}

const announcementColumns = `
	id, title, content, level, display_mode, show_policy,
	starts_at, ends_at, target_type, target_platform,
	min_version_code, max_version_code, action_text, action_url,
	revision, is_published, published_at, created_by, created_at, updated_at`

const announcementColumnsA = `
	a.id, a.title, a.content, a.level, a.display_mode, a.show_policy,
	a.starts_at, a.ends_at, a.target_type, a.target_platform,
	a.min_version_code, a.max_version_code, a.action_text, a.action_url,
	a.revision, a.is_published, a.published_at, a.created_by, a.created_at, a.updated_at`

func NewAnnouncementRepo(db *sql.DB) *AnnouncementRepo {
	return &AnnouncementRepo{db: db}
}

func scanAnnouncement(scanner rowScanner, a *model.Announcement) error {
	return scanner.Scan(
		&a.ID, &a.Title, &a.Content, &a.Level, &a.DisplayMode, &a.ShowPolicy,
		&a.StartsAt, &a.EndsAt, &a.TargetType, &a.TargetPlatform,
		&a.MinVersionCode, &a.MaxVersionCode, &a.ActionText, &a.ActionURL,
		&a.Revision, &a.IsPublished, &a.PublishedAt, &a.CreatedBy, &a.CreatedAt, &a.UpdatedAt,
	)
}

func (r *AnnouncementRepo) Create(a *model.Announcement) error {
	tx, err := r.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	result, err := tx.Exec(
		`INSERT INTO announcements (
			title, content, level, display_mode, show_policy, starts_at, ends_at,
			target_type, target_platform, min_version_code, max_version_code,
			action_text, action_url, revision, is_published, created_by
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 0, 0, ?)`,
		a.Title, a.Content, a.Level, a.DisplayMode, a.ShowPolicy, a.StartsAt, a.EndsAt,
		a.TargetType, a.TargetPlatform, a.MinVersionCode, a.MaxVersionCode,
		a.ActionText, a.ActionURL, a.CreatedBy,
	)
	if err != nil {
		return err
	}
	a.ID, _ = result.LastInsertId()
	if err := replaceAnnouncementTargets(tx, a.ID, a.TargetType, a.TargetUserIDs); err != nil {
		return err
	}
	return tx.Commit()
}

func (r *AnnouncementRepo) FindByID(id int64) (*model.Announcement, error) {
	a := &model.Announcement{}
	err := scanAnnouncement(
		r.db.QueryRow(`SELECT `+announcementColumns+` FROM announcements WHERE id = ?`, id),
		a,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	a.TargetUserIDs, err = r.targetUserIDs(id)
	return a, err
}

func (r *AnnouncementRepo) List(page, pageSize int, level string, publishedOnly bool) ([]model.Announcement, int64, error) {
	var conditions []string
	var args []any
	if level != "" {
		conditions = append(conditions, "a.level = ?")
		args = append(args, level)
	}
	if publishedOnly {
		conditions = append(conditions,
			"a.is_published = 1",
			"a.target_type = 'all'",
			"(a.starts_at IS NULL OR a.starts_at = '' OR a.starts_at <= ?)",
			"(a.ends_at IS NULL OR a.ends_at = '' OR a.ends_at > ?)",
		)
		now := time.Now().UTC().Format(time.RFC3339)
		args = append(args, now, now)
	}
	where := ""
	if len(conditions) > 0 {
		where = "WHERE " + strings.Join(conditions, " AND ")
	}

	var total int64
	if err := r.db.QueryRow(fmt.Sprintf("SELECT COUNT(*) FROM announcements a %s", where), args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	offset := (page - 1) * pageSize
	query := fmt.Sprintf(
		`SELECT `+announcementColumnsA+`,
			(SELECT COUNT(DISTINCT ar.user_id) FROM announcement_receipts ar
			 WHERE ar.announcement_id = a.id AND ar.revision = a.revision AND ar.delivered_at IS NOT NULL),
			(SELECT COUNT(DISTINCT ar.user_id) FROM announcement_receipts ar
			 WHERE ar.announcement_id = a.id AND ar.revision = a.revision AND ar.read_at IS NOT NULL),
			(SELECT COUNT(DISTINCT ar.user_id) FROM announcement_receipts ar
			 WHERE ar.announcement_id = a.id AND ar.revision = a.revision AND ar.acknowledged_at IS NOT NULL)
		 FROM announcements a %s ORDER BY a.id DESC LIMIT ? OFFSET ?`, where,
	)
	rows, err := r.db.Query(query, append(args, pageSize, offset)...)
	if err != nil {
		return nil, 0, err
	}

	items := make([]model.Announcement, 0)
	for rows.Next() {
		var a model.Announcement
		if err := rows.Scan(
			&a.ID, &a.Title, &a.Content, &a.Level, &a.DisplayMode, &a.ShowPolicy,
			&a.StartsAt, &a.EndsAt, &a.TargetType, &a.TargetPlatform,
			&a.MinVersionCode, &a.MaxVersionCode, &a.ActionText, &a.ActionURL,
			&a.Revision, &a.IsPublished, &a.PublishedAt, &a.CreatedBy, &a.CreatedAt, &a.UpdatedAt,
			&a.DeliveredCount, &a.ReadCount, &a.AcknowledgedCount,
		); err != nil {
			return nil, 0, err
		}
		items = append(items, a)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, 0, err
	}
	if err := rows.Close(); err != nil {
		return nil, 0, err
	}
	for index := range items {
		items[index].TargetUserIDs, err = r.targetUserIDs(items[index].ID)
		if err != nil {
			return nil, 0, err
		}
	}
	return items, total, nil
}

func (r *AnnouncementRepo) ListForUser(userID int64, platform string, versionCode int64) ([]model.Announcement, error) {
	now := time.Now().UTC().Format(time.RFC3339)
	rows, err := r.db.Query(
		`SELECT `+announcementColumnsA+`,
			ar.delivered_at, ar.read_at, ar.acknowledged_at
		 FROM announcements a
		 LEFT JOIN announcement_receipts ar
		   ON ar.announcement_id = a.id AND ar.user_id = ? AND ar.revision = a.revision
		 WHERE a.is_published = 1
		   AND (a.starts_at IS NULL OR a.starts_at = '' OR a.starts_at <= ?)
		   AND (a.ends_at IS NULL OR a.ends_at = '' OR a.ends_at > ?)
		   AND (a.target_platform = 'all' OR a.target_platform = ?)
		   AND (a.min_version_code IS NULL OR a.min_version_code <= ?)
		   AND (a.max_version_code IS NULL OR a.max_version_code >= ?)
		   AND (
			 a.target_type = 'all'
			 OR EXISTS (
			   SELECT 1 FROM announcement_targets at
			   WHERE at.announcement_id = a.id AND at.user_id = ?
			 )
		   )
		 ORDER BY CASE a.level WHEN 'critical' THEN 0 WHEN 'warning' THEN 1 ELSE 2 END,
		          a.published_at DESC, a.id DESC`,
		userID, now, now, platform, versionCode, versionCode, userID,
	)
	if err != nil {
		return nil, err
	}

	items := make([]model.Announcement, 0)
	for rows.Next() {
		var a model.Announcement
		if err := rows.Scan(
			&a.ID, &a.Title, &a.Content, &a.Level, &a.DisplayMode, &a.ShowPolicy,
			&a.StartsAt, &a.EndsAt, &a.TargetType, &a.TargetPlatform,
			&a.MinVersionCode, &a.MaxVersionCode, &a.ActionText, &a.ActionURL,
			&a.Revision, &a.IsPublished, &a.PublishedAt, &a.CreatedBy, &a.CreatedAt, &a.UpdatedAt,
			&a.DeliveredAt, &a.ReadAt, &a.AcknowledgedAt,
		); err != nil {
			return nil, err
		}
		a.IsRead = a.ReadAt != nil
		a.IsAcknowledged = a.AcknowledgedAt != nil
		items = append(items, a)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, err
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	if err := r.markDelivered(userID, items); err != nil {
		return nil, err
	}
	return items, nil
}

func (r *AnnouncementRepo) Update(a *model.Announcement) error {
	tx, err := r.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	now := time.Now().UTC().Format(time.RFC3339)
	_, err = tx.Exec(
		`UPDATE announcements SET
			title=?, content=?, level=?, display_mode=?, show_policy=?,
			starts_at=?, ends_at=?, target_type=?, target_platform=?,
			min_version_code=?, max_version_code=?, action_text=?, action_url=?,
			revision=CASE WHEN is_published=1 THEN revision+1 ELSE revision END,
			updated_at=?
		 WHERE id=?`,
		a.Title, a.Content, a.Level, a.DisplayMode, a.ShowPolicy,
		a.StartsAt, a.EndsAt, a.TargetType, a.TargetPlatform,
		a.MinVersionCode, a.MaxVersionCode, a.ActionText, a.ActionURL,
		now, a.ID,
	)
	if err != nil {
		return err
	}
	if err := replaceAnnouncementTargets(tx, a.ID, a.TargetType, a.TargetUserIDs); err != nil {
		return err
	}
	return tx.Commit()
}

func (r *AnnouncementRepo) Publish(id int64) error {
	now := time.Now().UTC().Format(time.RFC3339)
	_, err := r.db.Exec(
		`UPDATE announcements SET
			is_published=1, published_at=?, revision=CASE WHEN revision < 1 THEN 1 ELSE revision+1 END,
			updated_at=?
		 WHERE id=?`,
		now, now, id,
	)
	return err
}

func (r *AnnouncementRepo) Unpublish(id int64) error {
	now := time.Now().UTC().Format(time.RFC3339)
	_, err := r.db.Exec(
		`UPDATE announcements SET is_published=0, published_at=NULL, updated_at=? WHERE id=?`,
		now, id,
	)
	return err
}

func (r *AnnouncementRepo) Delete(id int64) error {
	_, err := r.db.Exec(`DELETE FROM announcements WHERE id = ?`, id)
	return err
}

func (r *AnnouncementRepo) MarkRead(announcementID, userID int64) error {
	return r.markReceipt(announcementID, userID, false)
}

func (r *AnnouncementRepo) Acknowledge(announcementID, userID int64) error {
	return r.markReceipt(announcementID, userID, true)
}

func (r *AnnouncementRepo) AvailableForUser(announcementID, userID int64) (bool, error) {
	now := time.Now().UTC().Format(time.RFC3339)
	var count int
	err := r.db.QueryRow(
		`SELECT COUNT(*) FROM announcements a
		 WHERE a.id=? AND a.is_published=1
		   AND (a.starts_at IS NULL OR a.starts_at='' OR a.starts_at<=?)
		   AND (a.ends_at IS NULL OR a.ends_at='' OR a.ends_at>?)
		   AND (a.target_type='all' OR EXISTS (
		     SELECT 1 FROM announcement_targets at
		     WHERE at.announcement_id=a.id AND at.user_id=?
		   ))`,
		announcementID, now, now, userID,
	).Scan(&count)
	return count > 0, err
}

func (r *AnnouncementRepo) targetUserIDs(announcementID int64) ([]int64, error) {
	rows, err := r.db.Query(
		`SELECT user_id FROM announcement_targets WHERE announcement_id=? ORDER BY user_id`,
		announcementID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	ids := make([]int64, 0)
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

func replaceAnnouncementTargets(tx *sql.Tx, announcementID int64, targetType string, userIDs []int64) error {
	if _, err := tx.Exec(`DELETE FROM announcement_targets WHERE announcement_id=?`, announcementID); err != nil {
		return err
	}
	if targetType != "users" {
		return nil
	}
	seen := make(map[int64]struct{})
	for _, userID := range userIDs {
		if userID <= 0 {
			continue
		}
		if _, exists := seen[userID]; exists {
			continue
		}
		seen[userID] = struct{}{}
		if _, err := tx.Exec(
			`INSERT INTO announcement_targets(announcement_id, user_id) VALUES (?, ?)`,
			announcementID, userID,
		); err != nil {
			return err
		}
	}
	return nil
}

func (r *AnnouncementRepo) markDelivered(userID int64, items []model.Announcement) error {
	if len(items) == 0 {
		return nil
	}
	tx, err := r.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	now := time.Now().UTC().Format(time.RFC3339)
	for _, a := range items {
		if _, err := tx.Exec(
			`INSERT INTO announcement_receipts(announcement_id, user_id, revision, delivered_at)
			 VALUES (?, ?, ?, ?)
			 ON CONFLICT(announcement_id, user_id, revision)
			 DO UPDATE SET delivered_at=COALESCE(announcement_receipts.delivered_at, excluded.delivered_at)`,
			a.ID, userID, a.Revision, now,
		); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (r *AnnouncementRepo) markReceipt(announcementID, userID int64, acknowledge bool) error {
	var revision int
	if err := r.db.QueryRow(`SELECT revision FROM announcements WHERE id=?`, announcementID).Scan(&revision); err != nil {
		return err
	}
	now := time.Now().UTC().Format(time.RFC3339)
	if acknowledge {
		_, err := r.db.Exec(
			`INSERT INTO announcement_receipts(
				announcement_id, user_id, revision, delivered_at, read_at, acknowledged_at
			 ) VALUES (?, ?, ?, ?, ?, ?)
			 ON CONFLICT(announcement_id, user_id, revision) DO UPDATE SET
			   delivered_at=COALESCE(announcement_receipts.delivered_at, excluded.delivered_at),
			   read_at=COALESCE(announcement_receipts.read_at, excluded.read_at),
			   acknowledged_at=COALESCE(announcement_receipts.acknowledged_at, excluded.acknowledged_at)`,
			announcementID, userID, revision, now, now, now,
		)
		return err
	}
	_, err := r.db.Exec(
		`INSERT INTO announcement_receipts(announcement_id, user_id, revision, delivered_at, read_at)
		 VALUES (?, ?, ?, ?, ?)
		 ON CONFLICT(announcement_id, user_id, revision) DO UPDATE SET
		   delivered_at=COALESCE(announcement_receipts.delivered_at, excluded.delivered_at),
		   read_at=COALESCE(announcement_receipts.read_at, excluded.read_at)`,
		announcementID, userID, revision, now, now,
	)
	return err
}
