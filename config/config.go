package config

import (
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Server   ServerConfig   `yaml:"server"`
	Database DatabaseConfig `yaml:"database"`
	Auth     AuthConfig     `yaml:"auth"`
	SMS      SMSConfig      `yaml:"sms"`
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
	// AllowLegacyRegistration is test-only. Production YAML cannot enable
	// username-only registration; existing legacy users can still log in.
	AllowLegacyRegistration  bool `yaml:"-"`
	BcryptCost               int  `yaml:"bcrypt_cost"`
	AccessTokenTTL           int  `yaml:"access_token_ttl"`
	RefreshTokenTTL          int  `yaml:"refresh_token_ttl"`
	LoginMaxAttempts         int  `yaml:"login_max_attempts"`
	LoginLockMinutes         int  `yaml:"login_lock_minutes"`
	RequireEncryptedRequests bool `yaml:"require_encrypted_requests"`
}

type SMSConfig struct {
	Endpoint                    string `yaml:"endpoint"`
	Username                    string `yaml:"username"`
	APIKey                      string `yaml:"api_key"`
	GoodsID                     string `yaml:"goods_id"`
	ContentTemplate             string `yaml:"content_template"`
	Enabled                     bool   `yaml:"enabled"`
	ExposeCaptchaCodeForTesting bool   `yaml:"-"`
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
			JWTSecret:                "change-me-in-production",
			SuperAdminUsername:       "admin",
			BcryptCost:               10,
			AccessTokenTTL:           7200,
			RefreshTokenTTL:          2592000,
			LoginMaxAttempts:         5,
			LoginLockMinutes:         15,
			RequireEncryptedRequests: false,
		},
		SMS: SMSConfig{
			Endpoint: "https://api.smsbao.com/sms",
			Enabled:  false,
		},
		Log: LogConfig{
			Level: "info",
			File:  "",
		},
		CORS: CORSConfig{
			AllowedOrigins: []string{"http://localhost:8788", "http://127.0.0.1:8788"},
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
	if s := os.Getenv("CORS_ALLOWED_ORIGINS"); s != "" {
		origins := make([]string, 0)
		for _, origin := range strings.Split(s, ",") {
			if origin = strings.TrimSpace(origin); origin != "" {
				origins = append(origins, origin)
			}
		}
		if len(origins) > 0 {
			cfg.CORS.AllowedOrigins = origins
		}
	}
	if s := os.Getenv("SMSBAO_ENDPOINT"); s != "" {
		cfg.SMS.Endpoint = s
	}
	if s := os.Getenv("SMSBAO_USERNAME"); s != "" {
		cfg.SMS.Username = s
	}
	if s := os.Getenv("SMSBAO_API_KEY"); s != "" {
		cfg.SMS.APIKey = s
	}
	if s := os.Getenv("SMSBAO_GOODSID"); s != "" {
		cfg.SMS.GoodsID = s
	}
	if s := os.Getenv("SMSBAO_CONTENT_TEMPLATE"); s != "" {
		cfg.SMS.ContentTemplate = s
	}
	if s := os.Getenv("SMSBAO_ENABLED"); s != "" {
		cfg.SMS.Enabled = strings.EqualFold(strings.TrimSpace(s), "true") || s == "1"
	}
	if s := os.Getenv("AUTH_REQUIRE_ENCRYPTED_REQUESTS"); s != "" {
		cfg.Auth.RequireEncryptedRequests = strings.EqualFold(strings.TrimSpace(s), "true") || s == "1"
	}

	return cfg, nil
}
