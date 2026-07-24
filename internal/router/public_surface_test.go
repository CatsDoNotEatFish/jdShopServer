package router_test

import (
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestAPIApplicationDoesNotExposeWebConsoles(t *testing.T) {
	server, _ := newTestServer(t)
	client := server.Client()

	for _, path := range []string{"/", "/admin", "/static/admin.html"} {
		resp, err := client.Get(server.URL + path)
		if err != nil {
			t.Fatalf("GET %s: %v", path, err)
		}
		resp.Body.Close()
		if resp.StatusCode != http.StatusNotFound {
			t.Fatalf("GET %s returned %d, want 404", path, resp.StatusCode)
		}
	}
}

func TestAdminOriginCanCallTheAPIDomain(t *testing.T) {
	server, _ := newTestServer(t)
	req, err := http.NewRequest(http.MethodOptions, server.URL+"/api/v1/auth/login", nil)
	if err != nil {
		t.Fatalf("create preflight request: %v", err)
	}
	req.Header.Set("Origin", "https://www.jdshop.bbroot.com")
	req.Header.Set("Access-Control-Request-Method", http.MethodPost)
	req.Header.Set("Access-Control-Request-Headers", "content-type")

	resp, err := server.Client().Do(req)
	if err != nil {
		t.Fatalf("send preflight request: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("preflight status = %d, want 200", resp.StatusCode)
	}
	if got := resp.Header.Get("Access-Control-Allow-Origin"); got != "https://www.jdshop.bbroot.com" {
		t.Fatalf("Access-Control-Allow-Origin = %q", got)
	}
}

func TestProductionNginxSeparatesAPIAndAdminDomains(t *testing.T) {
	_, filename, _, _ := runtime.Caller(0)
	repoRoot := filepath.Join(filepath.Dir(filename), "..", "..")

	nginxBytes, err := os.ReadFile(filepath.Join(repoRoot, "deploy", "nginx-https.conf"))
	if err != nil {
		t.Fatalf("read nginx config: %v", err)
	}
	sections := strings.Split(string(nginxBytes), "# 管理台专用域名")
	if len(sections) != 2 {
		t.Fatal("nginx config must have a distinct admin-domain server section")
	}
	apiSection, adminSection := sections[0], sections[1]
	if !strings.Contains(apiSection, "server_name api.yourdomain.com;") ||
		!strings.Contains(apiSection, "location / {\n        return 404;") {
		t.Fatal("API domain must return 404 outside /api/*")
	}
	for _, marker := range []string{
		"map $request_method $api_limit_key",
		"OPTIONS     \"\"",
		"zone=admin_api_limit:10m rate=300r/m",
		"zone=api_limit:10m rate=120r/m",
		"zone=login_limit:10m rate=5r/m",
		"location ^~ /api/v1/admin/",
		"limit_req zone=admin_api_limit burst=100 nodelay",
		"limit_req zone=api_limit burst=40 nodelay",
		"limit_req_status 429",
		"location @api_rate_limited",
		`Access-Control-Allow-Origin "https://www.jdshop.bbroot.com" always`,
	} {
		if !strings.Contains(apiSection, marker) {
			t.Fatalf("API rate-limit config is missing %q", marker)
		}
	}
	if strings.Contains(apiSection, "zone=api_limit:10m rate=30r/m") ||
		strings.Contains(apiSection, "limit_req_zone $binary_remote_addr zone=login_limit") {
		t.Fatal("production API rate limit must not use the obsolete 30 requests/minute value")
	}
	if !strings.Contains(adminSection, "server_name www.yourdomain.com;") ||
		!strings.Contains(adminSection, "alias /opt/jdshop/static/admin.html;") ||
		strings.Contains(adminSection, "proxy_pass") {
		t.Fatal("admin domain must serve only the static console and must not proxy APIs")
	}

	adminBytes, err := os.ReadFile(filepath.Join(repoRoot, "static", "admin.html"))
	if err != nil {
		t.Fatalf("read admin console: %v", err)
	}
	if !strings.Contains(string(adminBytes), `content="https://api.jdshop.bbroot.com"`) {
		t.Fatal("admin console must call the production API origin")
	}
	adminHTML := string(adminBytes)
	if !strings.Contains(adminHTML, "/api/v1/auth/encryption-key") ||
		!strings.Contains(adminHTML, "application/jdshop-encrypted+json") ||
		!strings.Contains(adminHTML, "RSA-OAEP") ||
		!strings.Contains(adminHTML, "AES-GCM") {
		t.Fatal("admin console login must use the encrypted authentication envelope")
	}
	if strings.Contains(adminHTML, `body:JSON.stringify({username:u,password:p})`) {
		t.Fatal("admin console must not send the administrator password as plaintext JSON")
	}
	for _, marker := range []string{
		`id="ann-display"`,
		`id="ann-policy"`,
		`id="ann-target-users"`,
		`id="ann-starts"`,
		`id="ann-ends"`,
		"delivered_count",
		"acknowledged_count",
	} {
		if !strings.Contains(adminHTML, marker) {
			t.Fatalf("admin console is missing announcement delivery control %q", marker)
		}
	}
	if _, err := os.Stat(filepath.Join(repoRoot, "static", "index.html")); !os.IsNotExist(err) {
		t.Fatal("the interactive API test page must not be shipped")
	}
}
