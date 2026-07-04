package repository

import (
	"database/sql"
	"fmt"
	"strings"
	"time"

	"jdShopServer/internal/model"
)

type UserRepo struct {
	db *sql.DB
}

func NewUserRepo(db *sql.DB) *UserRepo {
	return &UserRepo{db: db}
}

func (r *UserRepo) Create(user *model.User) error {
	result, err := r.db.Exec(
		`INSERT INTO users (username, email, password_hash, nickname, avatar_url, status)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		user.Username, user.Email, user.PasswordHash, user.Nickname, user.AvatarURL, user.Status,
	)
	if err != nil {
		return err
	}
	id, _ := result.LastInsertId()
	user.ID = id
	return nil
}

func (r *UserRepo) FindByUsername(username string) (*model.User, error) {
	user := &model.User{}
	err := r.db.QueryRow(
		`SELECT id, username, email, password_hash, nickname, avatar_url, status, last_login_at, created_at, updated_at
		 FROM users WHERE username = ?`, username,
	).Scan(&user.ID, &user.Username, &user.Email, &user.PasswordHash, &user.Nickname,
		&user.AvatarURL, &user.Status, &user.LastLoginAt, &user.CreatedAt, &user.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	roles, _ := r.UserRoles(user.ID)
	user.Roles = roles
	return user, nil
}

func (r *UserRepo) FindByID(id int64) (*model.User, error) {
	user := &model.User{}
	err := r.db.QueryRow(
		`SELECT id, username, email, password_hash, nickname, avatar_url, status, last_login_at, created_at, updated_at
		 FROM users WHERE id = ?`, id,
	).Scan(&user.ID, &user.Username, &user.Email, &user.PasswordHash, &user.Nickname,
		&user.AvatarURL, &user.Status, &user.LastLoginAt, &user.CreatedAt, &user.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	roles, _ := r.UserRoles(user.ID)
	user.Roles = roles
	return user, nil
}

func (r *UserRepo) List(page, pageSize int, keyword string, status *int) ([]model.UserWithRoles, int64, error) {
	var conditions []string
	var args []any

	if keyword != "" {
		conditions = append(conditions, "(u.username LIKE ? OR u.nickname LIKE ? OR u.email LIKE ?)")
		kw := "%" + keyword + "%"
		args = append(args, kw, kw, kw)
	}
	if status != nil {
		conditions = append(conditions, "u.status = ?")
		args = append(args, *status)
	}

	where := ""
	if len(conditions) > 0 {
		where = "WHERE " + strings.Join(conditions, " AND ")
	}

	var total int64
	countSQL := fmt.Sprintf("SELECT COUNT(*) FROM users u %s", where)
	if err := r.db.QueryRow(countSQL, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	offset := (page - 1) * pageSize
	query := fmt.Sprintf(
		`SELECT u.id, u.username, u.email, u.password_hash, u.nickname, u.avatar_url, u.status,
		        u.last_login_at, u.created_at, u.updated_at,
		        COALESCE(GROUP_CONCAT(r.name), '') as role_names
		 FROM users u
		 LEFT JOIN user_roles ur ON u.id = ur.user_id
		 LEFT JOIN roles r ON ur.role_id = r.id
		 %s
		 GROUP BY u.id
		 ORDER BY u.id DESC
		 LIMIT ? OFFSET ?`, where,
	)
	args = append(args, pageSize, offset)

	rows, err := r.db.Query(query, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var users []model.UserWithRoles
	for rows.Next() {
		var u model.UserWithRoles
		if err := rows.Scan(&u.ID, &u.Username, &u.Email, &u.PasswordHash, &u.Nickname,
			&u.AvatarURL, &u.Status, &u.LastLoginAt, &u.CreatedAt, &u.UpdatedAt, &u.RoleNames); err != nil {
			return nil, 0, err
		}
		users = append(users, u)
	}
	return users, total, nil
}

func (r *UserRepo) UpdateStatus(id int64, status int) error {
	_, err := r.db.Exec(`UPDATE users SET status = ?, updated_at = ? WHERE id = ?`,
		status, time.Now().UTC().Format(time.RFC3339), id)
	return err
}

func (r *UserRepo) UpdateProfile(id int64, nickname, email, avatarURL *string) error {
	now := time.Now().UTC().Format(time.RFC3339)
	_, err := r.db.Exec(
		`UPDATE users SET nickname = COALESCE(?, nickname), email = COALESCE(?, email),
		 avatar_url = COALESCE(?, avatar_url), updated_at = ? WHERE id = ?`,
		nickname, email, avatarURL, now, id,
	)
	return err
}

func (r *UserRepo) UpdatePassword(id int64, hash string) error {
	_, err := r.db.Exec(`UPDATE users SET password_hash = ?, updated_at = ? WHERE id = ?`,
		hash, time.Now().UTC().Format(time.RFC3339), id)
	return err
}

func (r *UserRepo) UpdateLastLogin(id int64) error {
	now := time.Now().UTC().Format(time.RFC3339)
	_, err := r.db.Exec(`UPDATE users SET last_login_at = ? WHERE id = ?`, now, id)
	return err
}

func (r *UserRepo) AssignRoles(userID int64, roleIDs []int64) error {
	tx, err := r.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.Exec(`DELETE FROM user_roles WHERE user_id = ?`, userID); err != nil {
		return err
	}
	for _, rid := range roleIDs {
		if _, err := tx.Exec(`INSERT OR IGNORE INTO user_roles (user_id, role_id) VALUES (?, ?)`, userID, rid); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (r *UserRepo) UserRoles(userID int64) ([]string, error) {
	rows, err := r.db.Query(
		`SELECT r.name FROM roles r
		 INNER JOIN user_roles ur ON r.id = ur.role_id
		 WHERE ur.user_id = ?`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var roles []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		roles = append(roles, name)
	}
	return roles, nil
}

func (r *UserRepo) GetUserPermissions(userID int64) ([]string, error) {
	rows, err := r.db.Query(
		`SELECT DISTINCT p.code FROM permissions p
		 INNER JOIN role_permissions rp ON p.id = rp.permission_id
		 INNER JOIN user_roles ur ON rp.role_id = ur.role_id
		 WHERE ur.user_id = ?`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var perms []string
	for rows.Next() {
		var code string
		if err := rows.Scan(&code); err != nil {
			return nil, err
		}
		perms = append(perms, code)
	}
	return perms, nil
}
