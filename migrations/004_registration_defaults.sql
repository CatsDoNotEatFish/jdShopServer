-- 004_registration_defaults.sql: Defaults applied only when a new account registers.

CREATE TABLE IF NOT EXISTS registration_defaults (
    id                  INTEGER PRIMARY KEY CHECK (id = 1),
    usage_days          INTEGER NOT NULL DEFAULT 30 CHECK (usage_days BETWEEN 1 AND 3650),
    competitor_monitor  INTEGER NOT NULL DEFAULT 1 CHECK (competitor_monitor IN (0, 1)),
    merchant_backend    INTEGER NOT NULL DEFAULT 0 CHECK (merchant_backend IN (0, 1)),
    analysis_center     INTEGER NOT NULL DEFAULT 0 CHECK (analysis_center IN (0, 1)),
    updated_at          TEXT NOT NULL DEFAULT (datetime('now'))
);

INSERT OR IGNORE INTO registration_defaults (
    id, usage_days, competitor_monitor, merchant_backend, analysis_center
) VALUES (1, 30, 1, 0, 0);
