package repository

import (
	"database/sql"

	"jdShopServer/internal/model"
)

type TokenRepo struct {
	db *sql.DB
}

func NewTokenRepo(db *sql.DB) *TokenRepo {
	return &TokenRepo{db: db}
}

func (r *TokenRepo) Create(token *model.RefreshToken) error {
	result, err := r.db.Exec(
		`INSERT INTO refresh_tokens (user_id, token_hash, expires_at) VALUES (?, ?, ?)`,
		token.UserID, token.TokenHash, token.ExpiresAt,
	)
	if err != nil {
		return err
	}
	id, _ := result.LastInsertId()
	token.ID = id
	return nil
}

func (r *TokenRepo) FindByHash(hash string) (*model.RefreshToken, error) {
	token := &model.RefreshToken{}
	err := r.db.QueryRow(
		`SELECT id, user_id, token_hash, expires_at, revoked, created_at
		 FROM refresh_tokens WHERE token_hash = ?`, hash,
	).Scan(&token.ID, &token.UserID, &token.TokenHash, &token.ExpiresAt, &token.Revoked, &token.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return token, nil
}

func (r *TokenRepo) Revoke(hash string) error {
	_, err := r.db.Exec(`UPDATE refresh_tokens SET revoked = 1 WHERE token_hash = ?`, hash)
	return err
}

func (r *TokenRepo) RevokeAllForUser(userID int64) error {
	_, err := r.db.Exec(`UPDATE refresh_tokens SET revoked = 1 WHERE user_id = ? AND revoked = 0`, userID)
	return err
}

func (r *TokenRepo) DeleteExpired() error {
	_, err := r.db.Exec(`DELETE FROM refresh_tokens WHERE expires_at < datetime('now') AND revoked = 0`)
	return err
}
