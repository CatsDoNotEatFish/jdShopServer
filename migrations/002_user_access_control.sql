-- 002_user_access_control.sql: Per-account usage period and module visibility.

CREATE TABLE IF NOT EXISTS user_access_control (
    user_id             INTEGER PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    competitor_monitor  INTEGER NOT NULL DEFAULT 1,
    merchant_backend    INTEGER NOT NULL DEFAULT 0,
    analysis_center     INTEGER NOT NULL DEFAULT 0,
    expires_at          TEXT,
    updated_at          TEXT NOT NULL DEFAULT (datetime('now'))
);

-- Existing accounts remain usable. Module visibility still defaults to competitor monitoring only.
INSERT OR IGNORE INTO user_access_control (
    user_id, competitor_monitor, merchant_backend, analysis_center, expires_at
)
SELECT id, 1, 0, 0, NULL FROM users;
