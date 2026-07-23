-- 003_user_auth_versions.sql: Permanently invalidate access tokens after account revocation.

CREATE TABLE IF NOT EXISTS user_auth_versions (
    user_id     INTEGER PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    version     INTEGER NOT NULL DEFAULT 1,
    updated_at  TEXT NOT NULL DEFAULT (datetime('now'))
);

INSERT OR IGNORE INTO user_auth_versions (user_id, version)
SELECT id, 1 FROM users;
