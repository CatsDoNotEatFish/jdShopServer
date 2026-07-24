package repository

import (
	"database/sql"
	"time"
)

type SMSVerification struct {
	ID        int64
	Phone     string
	Purpose   string
	CodeHash  string
	SentAt    time.Time
	ExpiresAt time.Time
	Attempts  int
	Consumed  bool
}

type SMSVerificationRepo struct {
	db *sql.DB
}

func NewSMSVerificationRepo(db *sql.DB) *SMSVerificationRepo {
	return &SMSVerificationRepo{db: db}
}

func (r *SMSVerificationRepo) LastSentAt(phone string) (*time.Time, error) {
	var value string
	err := r.db.QueryRow(
		`SELECT sent_at FROM sms_verifications WHERE phone = ? ORDER BY id DESC LIMIT 1`,
		phone,
	).Scan(&value)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil {
		parsed, err = time.Parse("2006-01-02 15:04:05", value)
	}
	if err != nil {
		return nil, err
	}
	return &parsed, nil
}

func (r *SMSVerificationRepo) LastCodeHash(phone string) (string, error) {
	var codeHash string
	err := r.db.QueryRow(
		`SELECT code_hash FROM sms_verifications WHERE phone = ? ORDER BY id DESC LIMIT 1`,
		phone,
	).Scan(&codeHash)
	if err == sql.ErrNoRows {
		return "", nil
	}
	return codeHash, err
}

func (r *SMSVerificationRepo) CountSentSince(phone string, since time.Time) (int, error) {
	var count int
	err := r.db.QueryRow(
		`SELECT COUNT(*) FROM sms_verifications WHERE phone = ? AND sent_at >= ?`,
		phone, since.UTC().Format(time.RFC3339),
	).Scan(&count)
	return count, err
}

func (r *SMSVerificationRepo) CountSentByIPSince(requestIP string, since time.Time) (int, error) {
	var count int
	err := r.db.QueryRow(
		`SELECT COUNT(*) FROM sms_verifications WHERE request_ip = ? AND sent_at >= ?`,
		requestIP, since.UTC().Format(time.RFC3339),
	).Scan(&count)
	return count, err
}

func (r *SMSVerificationRepo) Create(phone, purpose, codeHash string, sentAt, expiresAt time.Time, requestIP string) error {
	_, err := r.db.Exec(
		`INSERT INTO sms_verifications (phone, purpose, code_hash, sent_at, expires_at, request_ip)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		phone, purpose, codeHash, sentAt.UTC().Format(time.RFC3339), expiresAt.UTC().Format(time.RFC3339), requestIP,
	)
	return err
}

func (r *SMSVerificationRepo) Latest(phone, purpose string) (*SMSVerification, error) {
	var verification SMSVerification
	var sentAt, expiresAt string
	var consumed int
	err := r.db.QueryRow(
		`SELECT id, phone, purpose, code_hash, sent_at, expires_at, attempts, consumed
		 FROM sms_verifications WHERE phone = ? AND purpose = ? ORDER BY id DESC LIMIT 1`,
		phone, purpose,
	).Scan(&verification.ID, &verification.Phone, &verification.Purpose, &verification.CodeHash,
		&sentAt, &expiresAt, &verification.Attempts, &consumed)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	verification.SentAt, err = parseTimestamp(sentAt)
	if err != nil {
		return nil, err
	}
	verification.ExpiresAt, err = parseTimestamp(expiresAt)
	if err != nil {
		return nil, err
	}
	verification.Consumed = consumed == 1
	return &verification, nil
}

func (r *SMSVerificationRepo) IncrementAttempts(id int64, consume bool) error {
	consumed := 0
	if consume {
		consumed = 1
	}
	_, err := r.db.Exec(
		`UPDATE sms_verifications SET attempts = attempts + 1, consumed = CASE WHEN ? = 1 THEN 1 ELSE consumed END WHERE id = ?`,
		consumed, id,
	)
	return err
}

func (r *SMSVerificationRepo) Consume(id int64) error {
	_, err := r.db.Exec(`UPDATE sms_verifications SET consumed = 1 WHERE id = ?`, id)
	return err
}

func parseTimestamp(value string) (time.Time, error) {
	parsed, err := time.Parse(time.RFC3339, value)
	if err == nil {
		return parsed, nil
	}
	return time.Parse("2006-01-02 15:04:05", value)
}
