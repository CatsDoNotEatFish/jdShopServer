package config

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestCORSAllowedOriginsEnvironmentOverride(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte("cors:\n  allowed_origins:\n    - http://127.0.0.1:8788\n"), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	t.Setenv("CORS_ALLOWED_ORIGINS", "https://www.jdshop.bbroot.com, http://127.0.0.1:8788")

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	want := []string{"https://www.jdshop.bbroot.com", "http://127.0.0.1:8788"}
	if !reflect.DeepEqual(cfg.CORS.AllowedOrigins, want) {
		t.Fatalf("allowed origins = %#v, want %#v", cfg.CORS.AllowedOrigins, want)
	}
}

func TestSMSBaoConfigurationEnvironmentOverrides(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte("sms:\n  enabled: false\n  endpoint: https://api.smsbao.com/sms\n"), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	t.Setenv("SMSBAO_ENABLED", "true")
	t.Setenv("SMSBAO_USERNAME", "environment-user")
	t.Setenv("SMSBAO_API_KEY", "environment-api-key")
	t.Setenv("SMSBAO_GOODSID", "123456")
	t.Setenv("SMSBAO_CONTENT_TEMPLATE", "【jdShop】您的验证码是%s，5分钟内有效。")

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if !cfg.SMS.Enabled || cfg.SMS.Username != "environment-user" || cfg.SMS.APIKey != "environment-api-key" {
		t.Fatalf("SMS credentials were not loaded from environment: %+v", cfg.SMS)
	}
	if cfg.SMS.GoodsID != "123456" || cfg.SMS.ContentTemplate == "" {
		t.Fatalf("SMS product/template override missing: %+v", cfg.SMS)
	}
}
