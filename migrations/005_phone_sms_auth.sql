-- 005_phone_sms_auth.sql: Phone login, SMS verification and password-change verification.

ALTER TABLE users ADD COLUMN phone TEXT;

CREATE UNIQUE INDEX IF NOT EXISTS idx_users_phone ON users(phone) WHERE phone IS NOT NULL;

CREATE TABLE IF NOT EXISTS sms_verifications (
    id           INTEGER PRIMARY KEY AUTOINCREMENT,
    phone        TEXT NOT NULL,
    purpose      TEXT NOT NULL,
    code_hash    TEXT NOT NULL,
    sent_at      TEXT NOT NULL,
    expires_at   TEXT NOT NULL,
    attempts     INTEGER NOT NULL DEFAULT 0,
    consumed     INTEGER NOT NULL DEFAULT 0,
    request_ip   TEXT,
    created_at   TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE INDEX IF NOT EXISTS idx_sms_verifications_phone_purpose
    ON sms_verifications(phone, purpose, id DESC);
CREATE INDEX IF NOT EXISTS idx_sms_verifications_sent_at
    ON sms_verifications(sent_at);
