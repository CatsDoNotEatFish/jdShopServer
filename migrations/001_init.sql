-- 001_init.sql: Initial schema

PRAGMA journal_mode=WAL;
PRAGMA synchronous=NORMAL;
PRAGMA cache_size=-8000;
PRAGMA busy_timeout=5000;
PRAGMA foreign_keys=ON;
PRAGMA temp_store=MEMORY;

CREATE TABLE IF NOT EXISTS users (
    id            INTEGER PRIMARY KEY AUTOINCREMENT,
    username      TEXT    NOT NULL UNIQUE,
    email         TEXT,
    password_hash TEXT    NOT NULL,
    nickname      TEXT,
    avatar_url    TEXT,
    status        INTEGER NOT NULL DEFAULT 1,
    last_login_at TEXT,
    created_at    TEXT    NOT NULL DEFAULT (datetime('now')),
    updated_at    TEXT    NOT NULL DEFAULT (datetime('now'))
);

CREATE INDEX IF NOT EXISTS idx_users_username ON users(username);
CREATE INDEX IF NOT EXISTS idx_users_status    ON users(status);

CREATE TABLE IF NOT EXISTS refresh_tokens (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id    INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token_hash TEXT    NOT NULL UNIQUE,
    expires_at TEXT    NOT NULL,
    revoked    INTEGER NOT NULL DEFAULT 0,
    created_at TEXT    NOT NULL DEFAULT (datetime('now'))
);

CREATE INDEX IF NOT EXISTS idx_refresh_tokens_user ON refresh_tokens(user_id);
CREATE INDEX IF NOT EXISTS idx_refresh_tokens_hash ON refresh_tokens(token_hash);

CREATE TABLE IF NOT EXISTS roles (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    name        TEXT NOT NULL UNIQUE,
    description TEXT,
    created_at  TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE IF NOT EXISTS permissions (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    code        TEXT NOT NULL UNIQUE,
    name        TEXT NOT NULL,
    description TEXT
);

CREATE TABLE IF NOT EXISTS role_permissions (
    role_id       INTEGER NOT NULL REFERENCES roles(id) ON DELETE CASCADE,
    permission_id INTEGER NOT NULL REFERENCES permissions(id) ON DELETE CASCADE,
    PRIMARY KEY (role_id, permission_id)
);

CREATE TABLE IF NOT EXISTS user_roles (
    user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    role_id INTEGER NOT NULL REFERENCES roles(id) ON DELETE CASCADE,
    PRIMARY KEY (user_id, role_id)
);

CREATE TABLE IF NOT EXISTS announcements (
    id           INTEGER PRIMARY KEY AUTOINCREMENT,
    title        TEXT    NOT NULL,
    content      TEXT    NOT NULL,
    level        TEXT    NOT NULL DEFAULT 'info',
    is_published INTEGER NOT NULL DEFAULT 0,
    published_at TEXT,
    created_by   INTEGER NOT NULL REFERENCES users(id),
    created_at   TEXT    NOT NULL DEFAULT (datetime('now')),
    updated_at   TEXT    NOT NULL DEFAULT (datetime('now'))
);

CREATE INDEX IF NOT EXISTS idx_announcements_published ON announcements(is_published, published_at DESC);

CREATE TABLE IF NOT EXISTS app_versions (
    id           INTEGER PRIMARY KEY AUTOINCREMENT,
    platform     TEXT    NOT NULL,
    version_code INTEGER NOT NULL,
    version_name TEXT    NOT NULL,
    title        TEXT    NOT NULL,
    description  TEXT,
    download_url TEXT,
    file_size    INTEGER,
    file_hash    TEXT,
    is_force     INTEGER NOT NULL DEFAULT 0,
    is_latest    INTEGER NOT NULL DEFAULT 1,
    created_at   TEXT    NOT NULL DEFAULT (datetime('now')),
    UNIQUE(platform, version_code)
);

CREATE INDEX IF NOT EXISTS idx_app_versions_latest ON app_versions(platform, is_latest);

CREATE TABLE IF NOT EXISTS heartbeat_logs (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id     INTEGER NOT NULL REFERENCES users(id),
    device_id   TEXT    NOT NULL,
    platform    TEXT,
    app_version TEXT,
    ip_address  TEXT,
    created_at  TEXT    NOT NULL DEFAULT (datetime('now'))
);

CREATE INDEX IF NOT EXISTS idx_heartbeat_user ON heartbeat_logs(user_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_heartbeat_time ON heartbeat_logs(created_at);

CREATE TABLE IF NOT EXISTS login_logs (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id     INTEGER REFERENCES users(id),
    username    TEXT    NOT NULL,
    ip_address  TEXT,
    user_agent  TEXT,
    result      TEXT    NOT NULL,
    fail_reason TEXT,
    created_at  TEXT    NOT NULL DEFAULT (datetime('now'))
);

CREATE INDEX IF NOT EXISTS idx_login_logs_user ON login_logs(user_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_login_logs_ip   ON login_logs(ip_address, created_at);

-- Seed permissions
INSERT OR IGNORE INTO permissions (code, name, description) VALUES
('user:list',           '查看用户列表',   '查看所有注册用户'),
('user:update',         '修改用户状态',   '启用或禁用用户'),
('user:role:assign',    '分配用户角色',   '为用户分配角色'),
('role:list',           '查看角色列表',   '查看所有角色'),
('role:create',         '创建角色',       '创建新角色'),
('role:update',         '修改角色',       '修改角色信息'),
('role:delete',         '删除角色',       '删除角色'),
('announcement:list',   '查看公告列表',   '查看公告（含草稿）'),
('announcement:create', '创建公告',       '创建新公告'),
('announcement:update', '修改公告',       '修改公告内容'),
('announcement:delete', '删除公告',       '删除公告'),
('announcement:publish','发布公告',       '发布或下架公告'),
('version:list',        '查看版本列表',   '查看所有版本'),
('version:create',      '发布新版本',     '创建新版本'),
('version:update',      '修改版本',       '修改版本信息'),
('version:delete',      '删除版本',       '删除版本');

-- Seed roles
INSERT OR IGNORE INTO roles (id, name, description) VALUES (1, 'admin', '系统管理员');
INSERT OR IGNORE INTO roles (id, name, description) VALUES (2, 'user',  '普通用户');

-- Grant all permissions to admin
INSERT OR IGNORE INTO role_permissions (role_id, permission_id)
SELECT 1, id FROM permissions;

-- Grant basic permissions to user
INSERT OR IGNORE INTO role_permissions (role_id, permission_id)
SELECT 2, id FROM permissions WHERE code IN (
    'announcement:list',
    'version:list'
);

-- Create default admin user: admin / admin123
-- bcrypt hash for 'admin123' with cost=10
INSERT OR IGNORE INTO users (id, username, password_hash, nickname, status)
VALUES (1, 'admin', '$2a$10$qJFq7K9sY5mWZd8xOSTJWOkNu0PFKv6j6iWhS/9Q/NrVtG.sgjRYm', '管理员', 1);

-- Assign admin role to default admin user
INSERT OR IGNORE INTO user_roles (user_id, role_id) VALUES (1, 1);
