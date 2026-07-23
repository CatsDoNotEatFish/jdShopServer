package config

import (
	"os"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Server   ServerConfig   `yaml:"server"`
	Database DatabaseConfig `yaml:"database"`
	Auth     AuthConfig     `yaml:"auth"`
	Log      LogConfig      `yaml:"log"`
	CORS     CORSConfig     `yaml:"cors"`
}

type ServerConfig struct {
	Port         int    `yaml:"port"`
	Host         string `yaml:"host"`
	ReadTimeout  int    `yaml:"read_timeout"`
	WriteTimeout int    `yaml:"write_timeout"`
}

type DatabaseConfig struct {
	Path         string `yaml:"path"`
	MaxOpenConns int    `yaml:"max_open_conns"`
	WALMode      bool   `yaml:"wal_mode"`
}

type AuthConfig struct {
	JWTSecret          string `yaml:"jwt_secret"`
	SuperAdminUsername string `yaml:"super_admin_username"`
	BcryptCost         int    `yaml:"bcrypt_cost"`
	AccessTokenTTL     int    `yaml:"access_token_ttl"`
	RefreshTokenTTL    int    `yaml:"refresh_token_ttl"`
	LoginMaxAttempts   int    `yaml:"login_max_attempts"`
	LoginLockMinutes   int    `yaml:"login_lock_minutes"`
	DefaultUsageDays   int    `yaml:"default_usage_days"`
}

type LogConfig struct {
	Level string `yaml:"level"`
	File  string `yaml:"file"`
}

type CORSConfig struct {
	AllowedOrigins []string `yaml:"allowed_origins"`
}

func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	cfg := &Config{
		Server: ServerConfig{
			Port:         8080,
			Host:         "127.0.0.1",
			ReadTimeout:  10,
			WriteTimeout: 10,
		},
		Database: DatabaseConfig{
			Path:         "data/app.db",
			MaxOpenConns: 1,
			WALMode:      true,
		},
		Auth: AuthConfig{
			JWTSecret:          "change-me-in-production",
			SuperAdminUsername: "admin",
			BcryptCost:         10,
			AccessTokenTTL:     7200,
			RefreshTokenTTL:    2592000,
			LoginMaxAttempts:   5,
			LoginLockMinutes:   15,
			DefaultUsageDays:   30,
		},
		Log: LogConfig{
			Level: "info",
			File:  "",
		},
		CORS: CORSConfig{
			AllowedOrigins: []string{"http://localhost:8787", "http://127.0.0.1:8787"},
		},
	}

	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, err
	}

	// Environment variable overrides
	if s := os.Getenv("JWT_SECRET"); s != "" {
		cfg.Auth.JWTSecret = s
	}
	if s := os.Getenv("SUPER_ADMIN_USERNAME"); s != "" {
		cfg.Auth.SuperAdminUsername = s
	}
	if s := os.Getenv("DB_PATH"); s != "" {
		cfg.Database.Path = s
	}

	return cfg, nil
}
