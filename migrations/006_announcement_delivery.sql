ALTER TABLE announcements ADD COLUMN display_mode TEXT NOT NULL DEFAULT 'center';
ALTER TABLE announcements ADD COLUMN show_policy TEXT NOT NULL DEFAULT 'once';
ALTER TABLE announcements ADD COLUMN starts_at TEXT;
ALTER TABLE announcements ADD COLUMN ends_at TEXT;
ALTER TABLE announcements ADD COLUMN target_type TEXT NOT NULL DEFAULT 'all';
ALTER TABLE announcements ADD COLUMN target_platform TEXT NOT NULL DEFAULT 'all';
ALTER TABLE announcements ADD COLUMN min_version_code INTEGER;
ALTER TABLE announcements ADD COLUMN max_version_code INTEGER;
ALTER TABLE announcements ADD COLUMN action_text TEXT;
ALTER TABLE announcements ADD COLUMN action_url TEXT;
ALTER TABLE announcements ADD COLUMN revision INTEGER NOT NULL DEFAULT 0;

UPDATE announcements
SET revision = CASE WHEN is_published = 1 THEN 1 ELSE 0 END;

CREATE TABLE IF NOT EXISTS announcement_targets (
    announcement_id INTEGER NOT NULL REFERENCES announcements(id) ON DELETE CASCADE,
    user_id         INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    PRIMARY KEY (announcement_id, user_id)
);

CREATE TABLE IF NOT EXISTS announcement_receipts (
    announcement_id INTEGER NOT NULL REFERENCES announcements(id) ON DELETE CASCADE,
    user_id         INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    revision        INTEGER NOT NULL,
    delivered_at    TEXT,
    read_at         TEXT,
    acknowledged_at TEXT,
    PRIMARY KEY (announcement_id, user_id, revision)
);

CREATE INDEX IF NOT EXISTS idx_announcement_targets_user
ON announcement_targets(user_id, announcement_id);

CREATE INDEX IF NOT EXISTS idx_announcement_receipts_user
ON announcement_receipts(user_id, announcement_id, revision);

CREATE INDEX IF NOT EXISTS idx_announcements_delivery
ON announcements(is_published, starts_at, ends_at, target_platform);
