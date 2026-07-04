package repository

import (
	"database/sql"

	"jdShopServer/internal/model"
)

type RoleRepo struct {
	db *sql.DB
}

func NewRoleRepo(db *sql.DB) *RoleRepo {
	return &RoleRepo{db: db}
}

func (r *RoleRepo) List() ([]model.Role, error) {
	rows, err := r.db.Query(`SELECT id, name, description, created_at FROM roles ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var roles []model.Role
	for rows.Next() {
		var role model.Role
		if err := rows.Scan(&role.ID, &role.Name, &role.Description, &role.CreatedAt); err != nil {
			return nil, err
		}
		perms, _ := r.getRolePermissions(role.ID)
		role.Permissions = perms
		roles = append(roles, role)
	}
	return roles, nil
}

func (r *RoleRepo) FindByID(id int64) (*model.Role, error) {
	role := &model.Role{}
	err := r.db.QueryRow(`SELECT id, name, description, created_at FROM roles WHERE id = ?`, id).
		Scan(&role.ID, &role.Name, &role.Description, &role.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	perms, _ := r.getRolePermissions(role.ID)
	role.Permissions = perms
	return role, nil
}

func (r *RoleRepo) Create(role *model.Role) error {
	result, err := r.db.Exec(`INSERT INTO roles (name, description) VALUES (?, ?)`, role.Name, role.Description)
	if err != nil {
		return err
	}
	id, _ := result.LastInsertId()
	role.ID = id
	return nil
}

func (r *RoleRepo) Update(id int64, name, description *string) error {
	_, err := r.db.Exec(
		`UPDATE roles SET name = COALESCE(?, name), description = COALESCE(?, description) WHERE id = ?`,
		name, description, id,
	)
	return err
}

func (r *RoleRepo) Delete(id int64) error {
	_, err := r.db.Exec(`DELETE FROM roles WHERE id = ?`, id)
	return err
}

func (r *RoleRepo) SetPermissions(roleID int64, permIDs []int64) error {
	tx, err := r.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.Exec(`DELETE FROM role_permissions WHERE role_id = ?`, roleID); err != nil {
		return err
	}
	for _, pid := range permIDs {
		if _, err := tx.Exec(`INSERT OR IGNORE INTO role_permissions (role_id, permission_id) VALUES (?, ?)`, roleID, pid); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (r *RoleRepo) getRolePermissions(roleID int64) ([]model.Permission, error) {
	rows, err := r.db.Query(
		`SELECT p.id, p.code, p.name, p.description FROM permissions p
		 INNER JOIN role_permissions rp ON p.id = rp.permission_id
		 WHERE rp.role_id = ? ORDER BY p.id`, roleID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var perms []model.Permission
	for rows.Next() {
		var p model.Permission
		if err := rows.Scan(&p.ID, &p.Code, &p.Name, &p.Description); err != nil {
			return nil, err
		}
		perms = append(perms, p)
	}
	return perms, nil
}

func (r *RoleRepo) ListAllPermissions() ([]model.Permission, error) {
	rows, err := r.db.Query(`SELECT id, code, name, description FROM permissions ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var perms []model.Permission
	for rows.Next() {
		var p model.Permission
		if err := rows.Scan(&p.ID, &p.Code, &p.Name, &p.Description); err != nil {
			return nil, err
		}
		perms = append(perms, p)
	}
	return perms, nil
}
