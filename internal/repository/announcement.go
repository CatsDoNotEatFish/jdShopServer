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

func NewAnnouncementRepo(db *sql.DB) *AnnouncementRepo {
	return &AnnouncementRepo{db: db}
}

func (r *AnnouncementRepo) Create(a *model.Announcement) error {
	result, err := r.db.Exec(
		`INSERT INTO announcements (title, content, level, is_published, created_by)
		 VALUES (?, ?, ?, ?, ?)`,
		a.Title, a.Content, a.Level, a.IsPublished, a.CreatedBy,
	)
	if err != nil {
		return err
	}
	id, _ := result.LastInsertId()
	a.ID = id
	return nil
}

func (r *AnnouncementRepo) FindByID(id int64) (*model.Announcement, error) {
	a := &model.Announcement{}
	err := r.db.QueryRow(
		`SELECT id, title, content, level, is_published, published_at, created_by, created_at, updated_at
		 FROM announcements WHERE id = ?`, id,
	).Scan(&a.ID, &a.Title, &a.Content, &a.Level, &a.IsPublished, &a.PublishedAt,
		&a.CreatedBy, &a.CreatedAt, &a.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return a, nil
}

func (r *AnnouncementRepo) List(page, pageSize int, level string, publishedOnly bool) ([]model.Announcement, int64, error) {
	var conditions []string
	var args []any

	if level != "" {
		conditions = append(conditions, "level = ?")
		args = append(args, level)
	}
	if publishedOnly {
		conditions = append(conditions, "is_published = 1")
	}

	where := ""
	if len(conditions) > 0 {
		where = "WHERE " + strings.Join(conditions, " AND ")
	}

	var total int64
	countSQL := fmt.Sprintf("SELECT COUNT(*) FROM announcements %s", where)
	if err := r.db.QueryRow(countSQL, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	offset := (page - 1) * pageSize
	query := fmt.Sprintf(
		`SELECT id, title, content, level, is_published, published_at, created_by, created_at, updated_at
		 FROM announcements %s ORDER BY id DESC LIMIT ? OFFSET ?`, where,
	)
	args = append(args, pageSize, offset)

	rows, err := r.db.Query(query, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var items []model.Announcement
	for rows.Next() {
		var a model.Announcement
		if err := rows.Scan(&a.ID, &a.Title, &a.Content, &a.Level, &a.IsPublished,
			&a.PublishedAt, &a.CreatedBy, &a.CreatedAt, &a.UpdatedAt); err != nil {
			return nil, 0, err
		}
		items = append(items, a)
	}
	return items, total, nil
}

func (r *AnnouncementRepo) Update(id int64, title, content, level *string) error {
	now := time.Now().UTC().Format(time.RFC3339)
	_, err := r.db.Exec(
		`UPDATE announcements SET title = COALESCE(?, title), content = COALESCE(?, content),
		 level = COALESCE(?, level), updated_at = ? WHERE id = ?`,
		title, content, level, now, id,
	)
	return err
}

func (r *AnnouncementRepo) Publish(id int64) error {
	now := time.Now().UTC().Format(time.RFC3339)
	_, err := r.db.Exec(
		`UPDATE announcements SET is_published = 1, published_at = ?, updated_at = ? WHERE id = ?`,
		now, now, id,
	)
	return err
}

func (r *AnnouncementRepo) Unpublish(id int64) error {
	now := time.Now().UTC().Format(time.RFC3339)
	_, err := r.db.Exec(
		`UPDATE announcements SET is_published = 0, published_at = NULL, updated_at = ? WHERE id = ?`,
		now, id,
	)
	return err
}

func (r *AnnouncementRepo) Delete(id int64) error {
	_, err := r.db.Exec(`DELETE FROM announcements WHERE id = ?`, id)
	return err
}
