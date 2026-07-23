package router_test

import (
	"bufio"
	"bytes"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"runtime"
	"strconv"
	"testing"
	"time"

	"jdShopServer/config"
	"jdShopServer/internal/repository"
	"jdShopServer/internal/router"
)

type apiEnvelope struct {
	Code    int             `json:"code"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data"`
}

func newTestServer(t *testing.T) (*httptest.Server, *sql.DB) {
	t.Helper()
	db, err := repository.Open(filepath.Join(t.TempDir(), "app.db"), 1)
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	_, filename, _, _ := runtime.Caller(0)
	migrationsDir := filepath.Join(filepath.Dir(filename), "..", "..", "migrations")
	if err := repository.RunMigrations(db, migrationsDir); err != nil {
		db.Close()
		t.Fatalf("run migrations: %v", err)
	}
	cfg := &config.Config{
		Auth: config.AuthConfig{
			JWTSecret:          "test-secret-at-least-32-characters",
			SuperAdminUsername: "admin",
			BcryptCost:         4,
			AccessTokenTTL:     7200,
			RefreshTokenTTL:    2592000,
			LoginMaxAttempts:   5,
			LoginLockMinutes:   15,
		},
		CORS: config.CORSConfig{AllowedOrigins: []string{"http://127.0.0.1"}},
	}
	server := httptest.NewServer(router.New(cfg, db, "test"))
	t.Cleanup(func() {
		server.Close()
		db.Close()
	})
	return server, db
}

func requestJSON(t *testing.T, client *http.Client, method, url string, body any, token string) (int, apiEnvelope) {
	t.Helper()
	var payload []byte
	var err error
	if body != nil {
		payload, err = json.Marshal(body)
		if err != nil {
			t.Fatalf("marshal request: %v", err)
		}
	}
	req, err := http.NewRequest(method, url, bytes.NewReader(payload))
	if err != nil {
		t.Fatalf("create request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("request %s: %v", url, err)
	}
	defer resp.Body.Close()
	var envelope apiEnvelope
	if err := json.NewDecoder(resp.Body).Decode(&envelope); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	return resp.StatusCode, envelope
}

func postJSON(t *testing.T, client *http.Client, url string, body any, token string) (int, apiEnvelope) {
	t.Helper()
	return requestJSON(t, client, http.MethodPost, url, body, token)
}

func accessTokenFromLogin(t *testing.T, envelope apiEnvelope) string {
	t.Helper()
	var data struct {
		AccessToken string `json:"access_token"`
	}
	if err := json.Unmarshal(envelope.Data, &data); err != nil {
		t.Fatalf("decode access token: %v", err)
	}
	if data.AccessToken == "" {
		t.Fatal("login response did not contain an access token")
	}
	return data.AccessToken
}

func TestRegisteredAccountDefaultsToCompetitorMonitorAccess(t *testing.T) {
	server, _ := newTestServer(t)
	client := server.Client()

	status, registered := postJSON(t, client, server.URL+"/api/v1/auth/register", map[string]string{
		"username": "new_user",
		"password": "secret123",
	}, "")
	if status != http.StatusOK || registered.Code != 0 {
		t.Fatalf("register failed: status=%d code=%d message=%s", status, registered.Code, registered.Message)
	}

	status, loggedIn := postJSON(t, client, server.URL+"/api/v1/auth/login", map[string]string{
		"username": "new_user",
		"password": "secret123",
	}, "")
	if status != http.StatusOK || loggedIn.Code != 0 {
		t.Fatalf("login failed: status=%d code=%d message=%s", status, loggedIn.Code, loggedIn.Message)
	}
	var data struct {
		User struct {
			Access struct {
				Allowed          bool    `json:"allowed"`
				ExpiresAt        *string `json:"expires_at"`
				RemainingSeconds int64   `json:"remaining_seconds"`
				Modules          struct {
					CompetitorMonitor bool `json:"competitor_monitor"`
					MerchantBackend   bool `json:"merchant_backend"`
					AnalysisCenter    bool `json:"analysis_center"`
				} `json:"modules"`
			} `json:"access"`
		} `json:"user"`
	}
	if err := json.Unmarshal(loggedIn.Data, &data); err != nil {
		t.Fatalf("decode login data: %v", err)
	}
	access := data.User.Access
	if !access.Allowed {
		t.Fatal("new account should be allowed")
	}
	if access.ExpiresAt == nil || *access.ExpiresAt == "" || access.RemainingSeconds <= 0 {
		t.Fatalf("new account should have remaining usage time: %+v", access)
	}
	if !access.Modules.CompetitorMonitor || access.Modules.MerchantBackend || access.Modules.AnalysisCenter {
		t.Fatalf("unexpected default modules: %+v", access.Modules)
	}
}

func TestAdminAccessUpdateIsReturnedByLoginAndHeartbeat(t *testing.T) {
	server, _ := newTestServer(t)
	client := server.Client()

	_, registered := postJSON(t, client, server.URL+"/api/v1/auth/register", map[string]string{
		"username": "controlled_user",
		"password": "secret123",
	}, "")
	var registeredUser struct {
		ID int64 `json:"id"`
	}
	if err := json.Unmarshal(registered.Data, &registeredUser); err != nil {
		t.Fatalf("decode registered user: %v", err)
	}

	status, adminLogin := postJSON(t, client, server.URL+"/api/v1/auth/login", map[string]string{
		"username": "admin",
		"password": "admin123",
	}, "")
	if status != http.StatusOK {
		t.Fatalf("admin login failed: status=%d message=%s", status, adminLogin.Message)
	}
	adminToken := accessTokenFromLogin(t, adminLogin)
	expiresAt := time.Now().UTC().Add(48 * time.Hour).Truncate(time.Second).Format(time.RFC3339)
	status, updated := requestJSON(
		t,
		client,
		http.MethodPut,
		server.URL+"/api/v1/admin/users/"+strconv.FormatInt(registeredUser.ID, 10)+"/access",
		map[string]any{
			"competitor_monitor": true,
			"merchant_backend":   true,
			"analysis_center":    false,
			"expires_at":         expiresAt,
		},
		adminToken,
	)
	if status != http.StatusOK || updated.Code != 0 {
		t.Fatalf("access update failed: status=%d code=%d message=%s", status, updated.Code, updated.Message)
	}

	status, userLogin := postJSON(t, client, server.URL+"/api/v1/auth/login", map[string]string{
		"username": "controlled_user",
		"password": "secret123",
	}, "")
	if status != http.StatusOK {
		t.Fatalf("controlled user login failed: status=%d message=%s", status, userLogin.Message)
	}
	userToken := accessTokenFromLogin(t, userLogin)

	status, heartbeat := postJSON(t, client, server.URL+"/api/v1/heartbeat", map[string]string{
		"device_id":   "test-device",
		"platform":    "windows",
		"app_version": "0.2.1",
	}, userToken)
	if status != http.StatusOK || heartbeat.Code != 0 {
		t.Fatalf("heartbeat failed: status=%d code=%d message=%s", status, heartbeat.Code, heartbeat.Message)
	}
	var heartbeatData struct {
		Access struct {
			Allowed   bool    `json:"allowed"`
			ExpiresAt *string `json:"expires_at"`
			Modules   struct {
				CompetitorMonitor bool `json:"competitor_monitor"`
				MerchantBackend   bool `json:"merchant_backend"`
				AnalysisCenter    bool `json:"analysis_center"`
			} `json:"modules"`
		} `json:"access"`
	}
	if err := json.Unmarshal(heartbeat.Data, &heartbeatData); err != nil {
		t.Fatalf("decode heartbeat: %v", err)
	}
	access := heartbeatData.Access
	if !access.Allowed || access.ExpiresAt == nil || *access.ExpiresAt != expiresAt {
		t.Fatalf("heartbeat returned stale access: %+v", access)
	}
	if !access.Modules.CompetitorMonitor || !access.Modules.MerchantBackend || access.Modules.AnalysisCenter {
		t.Fatalf("heartbeat returned wrong modules: %+v", access.Modules)
	}

	status, users := requestJSON(
		t,
		client,
		http.MethodGet,
		server.URL+"/api/v1/admin/users?keyword=controlled_user",
		nil,
		adminToken,
	)
	if status != http.StatusOK || users.Code != 0 {
		t.Fatalf("admin user list failed: status=%d code=%d message=%s", status, users.Code, users.Message)
	}
	var userList struct {
		Items []struct {
			LastHeartbeatAt  *string `json:"last_heartbeat_at"`
			HeartbeatDevice  *string `json:"heartbeat_device_id"`
			HeartbeatVersion *string `json:"heartbeat_app_version"`
		} `json:"items"`
	}
	if err := json.Unmarshal(users.Data, &userList); err != nil {
		t.Fatalf("decode admin user list: %v", err)
	}
	if len(userList.Items) != 1 || userList.Items[0].LastHeartbeatAt == nil || userList.Items[0].HeartbeatDevice == nil || *userList.Items[0].HeartbeatDevice != "test-device" || userList.Items[0].HeartbeatVersion == nil || *userList.Items[0].HeartbeatVersion != "0.2.1" {
		t.Fatalf("admin user list did not expose latest heartbeat: %+v", userList.Items)
	}
}

func TestExpiredAccountCannotLogin(t *testing.T) {
	server, _ := newTestServer(t)
	client := server.Client()
	_, registered := postJSON(t, client, server.URL+"/api/v1/auth/register", map[string]string{
		"username": "expired_user",
		"password": "secret123",
	}, "")
	var registeredUser struct {
		ID int64 `json:"id"`
	}
	if err := json.Unmarshal(registered.Data, &registeredUser); err != nil {
		t.Fatalf("decode registered user: %v", err)
	}
	_, adminLogin := postJSON(t, client, server.URL+"/api/v1/auth/login", map[string]string{
		"username": "admin",
		"password": "admin123",
	}, "")
	adminToken := accessTokenFromLogin(t, adminLogin)
	var adminLoginData struct {
		User struct {
			Access struct {
				ExpiresAt *string `json:"expires_at"`
				Modules   struct {
					CompetitorMonitor bool `json:"competitor_monitor"`
					MerchantBackend   bool `json:"merchant_backend"`
					AnalysisCenter    bool `json:"analysis_center"`
				} `json:"modules"`
			} `json:"access"`
		} `json:"user"`
	}
	if err := json.Unmarshal(adminLogin.Data, &adminLoginData); err != nil {
		t.Fatalf("decode supervisor access: %v", err)
	}
	modules := adminLoginData.User.Access.Modules
	if !modules.CompetitorMonitor || !modules.MerchantBackend || !modules.AnalysisCenter || adminLoginData.User.Access.ExpiresAt != nil {
		t.Fatalf("supervisor must have permanent full access: %+v", adminLoginData.User.Access)
	}
	expiredAt := time.Now().UTC().Add(-time.Hour).Format(time.RFC3339)
	status, response := requestJSON(
		t,
		client,
		http.MethodPut,
		server.URL+"/api/v1/admin/users/"+strconv.FormatInt(registeredUser.ID, 10)+"/access",
		map[string]any{
			"competitor_monitor": true,
			"merchant_backend":   false,
			"analysis_center":    false,
			"expires_at":         expiredAt,
		},
		adminToken,
	)
	if status != http.StatusOK || response.Code != 0 {
		t.Fatalf("access update failed: status=%d code=%d message=%s", status, response.Code, response.Message)
	}

	status, response = postJSON(t, client, server.URL+"/api/v1/auth/login", map[string]string{
		"username": "expired_user",
		"password": "secret123",
	}, "")
	if status != http.StatusForbidden || response.Code != 10003 {
		t.Fatalf("expired login should be forbidden: status=%d code=%d message=%s", status, response.Code, response.Message)
	}
}

func TestDisabledAccountInvalidatesAllExistingTokens(t *testing.T) {
	server, _ := newTestServer(t)
	client := server.Client()
	_, registered := postJSON(t, client, server.URL+"/api/v1/auth/register", map[string]string{
		"username": "disabled_user",
		"password": "secret123",
	}, "")
	var registeredUser struct {
		ID int64 `json:"id"`
	}
	if err := json.Unmarshal(registered.Data, &registeredUser); err != nil {
		t.Fatalf("decode registered user: %v", err)
	}
	_, userLogin := postJSON(t, client, server.URL+"/api/v1/auth/login", map[string]string{
		"username": "disabled_user",
		"password": "secret123",
	}, "")
	userToken := accessTokenFromLogin(t, userLogin)
	_, adminLogin := postJSON(t, client, server.URL+"/api/v1/auth/login", map[string]string{
		"username": "admin",
		"password": "admin123",
	}, "")
	adminToken := accessTokenFromLogin(t, adminLogin)
	status, response := requestJSON(
		t,
		client,
		http.MethodPut,
		server.URL+"/api/v1/admin/users/"+strconv.FormatInt(registeredUser.ID, 10)+"/status",
		map[string]any{"status": 0},
		adminToken,
	)
	if status != http.StatusOK || response.Code != 0 {
		t.Fatalf("disable failed: status=%d code=%d message=%s", status, response.Code, response.Message)
	}

	status, response = postJSON(t, client, server.URL+"/api/v1/heartbeat", map[string]string{
		"device_id": "disabled-device",
	}, userToken)
	if status != http.StatusUnauthorized || response.Code != 10002 {
		t.Fatalf("disabled account access token should be invalid: status=%d code=%d message=%s", status, response.Code, response.Message)
	}

	status, response = requestJSON(
		t,
		client,
		http.MethodPut,
		server.URL+"/api/v1/admin/users/"+strconv.FormatInt(registeredUser.ID, 10)+"/status",
		map[string]any{"status": 1},
		adminToken,
	)
	if status != http.StatusOK || response.Code != 0 {
		t.Fatalf("enable failed: status=%d code=%d message=%s", status, response.Code, response.Message)
	}

	status, response = postJSON(t, client, server.URL+"/api/v1/heartbeat", map[string]string{
		"device_id": "old-token-after-enable",
	}, userToken)
	if status != http.StatusUnauthorized || response.Code != 10002 {
		t.Fatalf("old access token must remain invalid after re-enable: status=%d code=%d message=%s", status, response.Code, response.Message)
	}

	status, response = postJSON(t, client, server.URL+"/api/v1/auth/login", map[string]string{
		"username": "disabled_user",
		"password": "secret123",
	}, "")
	if status != http.StatusOK || response.Code != 0 {
		t.Fatalf("re-enabled account should require and accept a new login: status=%d code=%d message=%s", status, response.Code, response.Message)
	}
	newUserToken := accessTokenFromLogin(t, response)
	status, response = postJSON(t, client, server.URL+"/api/v1/heartbeat", map[string]string{
		"device_id": "new-token-after-enable",
	}, newUserToken)
	if status != http.StatusOK || response.Code != 0 {
		t.Fatalf("new token should work after re-login: status=%d code=%d message=%s", status, response.Code, response.Message)
	}
}

func TestAdminAccessUpdatePushesControlEvent(t *testing.T) {
	server, _ := newTestServer(t)
	client := server.Client()
	_, registered := postJSON(t, client, server.URL+"/api/v1/auth/register", map[string]string{
		"username": "stream_user",
		"password": "secret123",
	}, "")
	var registeredUser struct {
		ID int64 `json:"id"`
	}
	if err := json.Unmarshal(registered.Data, &registeredUser); err != nil {
		t.Fatalf("decode registered user: %v", err)
	}
	_, userLogin := postJSON(t, client, server.URL+"/api/v1/auth/login", map[string]string{
		"username": "stream_user",
		"password": "secret123",
	}, "")
	userToken := accessTokenFromLogin(t, userLogin)
	_, adminLogin := postJSON(t, client, server.URL+"/api/v1/auth/login", map[string]string{
		"username": "admin",
		"password": "admin123",
	}, "")
	adminToken := accessTokenFromLogin(t, adminLogin)

	req, err := http.NewRequest(http.MethodGet, server.URL+"/api/v1/control/stream", nil)
	if err != nil {
		t.Fatalf("create stream request: %v", err)
	}
	req.Header.Set("Authorization", "Bearer "+userToken)
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("open control stream: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK || resp.Header.Get("Content-Type") != "text/event-stream; charset=utf-8" {
		t.Fatalf("unexpected stream response: status=%d content-type=%s", resp.StatusCode, resp.Header.Get("Content-Type"))
	}

	events := make(chan map[string]string, 2)
	errors := make(chan error, 1)
	go func() {
		scanner := bufio.NewScanner(resp.Body)
		for scanner.Scan() {
			line := scanner.Text()
			if len(line) < 6 || line[:6] != "data: " {
				continue
			}
			var event map[string]string
			if err := json.Unmarshal([]byte(line[6:]), &event); err != nil {
				errors <- err
				return
			}
			events <- event
		}
		if err := scanner.Err(); err != nil {
			errors <- err
		}
	}()

	assertControlEventType(t, events, errors, "connected")
	expiresAt := time.Now().UTC().Add(24 * time.Hour).Format(time.RFC3339)
	status, updated := requestJSON(
		t,
		client,
		http.MethodPut,
		server.URL+"/api/v1/admin/users/"+strconv.FormatInt(registeredUser.ID, 10)+"/access",
		map[string]any{
			"competitor_monitor": true,
			"merchant_backend":   true,
			"analysis_center":    true,
			"expires_at":         expiresAt,
		},
		adminToken,
	)
	if status != http.StatusOK || updated.Code != 0 {
		t.Fatalf("access update failed: status=%d code=%d message=%s", status, updated.Code, updated.Message)
	}
	assertControlEventType(t, events, errors, "access_changed")
}

func assertControlEventType(t *testing.T, events <-chan map[string]string, errors <-chan error, expected string) {
	t.Helper()
	select {
	case event := <-events:
		if event["type"] != expected || event["issued_at"] == "" {
			t.Fatalf("unexpected control event: %+v", event)
		}
	case err := <-errors:
		t.Fatalf("read control stream: %v", err)
	case <-time.After(2 * time.Second):
		t.Fatalf("timed out waiting for %q control event", expected)
	}
}

func TestLogoutRevokesRefreshToken(t *testing.T) {
	server, _ := newTestServer(t)
	client := server.Client()
	_, _ = postJSON(t, client, server.URL+"/api/v1/auth/register", map[string]string{
		"username": "logout_user",
		"password": "secret123",
	}, "")
	_, loggedIn := postJSON(t, client, server.URL+"/api/v1/auth/login", map[string]string{
		"username": "logout_user",
		"password": "secret123",
	}, "")
	var loginData struct {
		RefreshToken string `json:"refresh_token"`
	}
	if err := json.Unmarshal(loggedIn.Data, &loginData); err != nil || loginData.RefreshToken == "" {
		t.Fatalf("decode refresh token: %v", err)
	}
	status, response := postJSON(t, client, server.URL+"/api/v1/auth/logout", map[string]string{
		"refresh_token": loginData.RefreshToken,
	}, "")
	if status != http.StatusOK || response.Code != 0 {
		t.Fatalf("logout failed: status=%d code=%d", status, response.Code)
	}
	status, response = postJSON(t, client, server.URL+"/api/v1/auth/refresh", map[string]string{
		"refresh_token": loginData.RefreshToken,
	}, "")
	if status != http.StatusUnauthorized || response.Code != 10002 {
		t.Fatalf("revoked token should not refresh: status=%d code=%d", status, response.Code)
	}
}

func TestAdminRoleAloneCannotAccessSupervisorConsole(t *testing.T) {
	server, db := newTestServer(t)
	client := server.Client()

	_, registered := postJSON(t, client, server.URL+"/api/v1/auth/register", map[string]string{
		"username": "role_only_admin",
		"password": "secret123",
	}, "")
	var user struct {
		ID int64 `json:"id"`
	}
	if err := json.Unmarshal(registered.Data, &user); err != nil {
		t.Fatalf("decode registered user: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO user_roles (user_id, role_id) SELECT ?, id FROM roles WHERE name = 'admin'`, user.ID); err != nil {
		t.Fatalf("assign admin role directly: %v", err)
	}

	status, loggedIn := postJSON(t, client, server.URL+"/api/v1/auth/login", map[string]string{
		"username": "role_only_admin",
		"password": "secret123",
	}, "")
	if status != http.StatusOK || loggedIn.Code != 0 {
		t.Fatalf("role-only admin login failed: status=%d code=%d", status, loggedIn.Code)
	}
	status, response := requestJSON(t, client, http.MethodGet, server.URL+"/api/v1/admin/users", nil, accessTokenFromLogin(t, loggedIn))
	if status != http.StatusForbidden || response.Code != 10003 {
		t.Fatalf("role-only admin must be forbidden: status=%d code=%d", status, response.Code)
	}
}

func TestSupervisorAccountAndAdminRoleAreProtected(t *testing.T) {
	server, db := newTestServer(t)
	client := server.Client()
	_, adminLogin := postJSON(t, client, server.URL+"/api/v1/auth/login", map[string]string{
		"username": "admin",
		"password": "admin123",
	}, "")
	adminToken := accessTokenFromLogin(t, adminLogin)

	var adminID, adminRoleID int64
	if err := db.QueryRow(`SELECT id FROM users WHERE username = 'admin'`).Scan(&adminID); err != nil {
		t.Fatalf("find supervisor: %v", err)
	}
	if err := db.QueryRow(`SELECT id FROM roles WHERE name = 'admin'`).Scan(&adminRoleID); err != nil {
		t.Fatalf("find admin role: %v", err)
	}

	protectedRequests := []struct {
		name   string
		method string
		path   string
		body   any
	}{
		{"disable supervisor", http.MethodPut, "/api/v1/admin/users/" + strconv.FormatInt(adminID, 10) + "/status", map[string]int{"status": 0}},
		{"change supervisor access", http.MethodPut, "/api/v1/admin/users/" + strconv.FormatInt(adminID, 10) + "/access", map[string]any{"competitor_monitor": true}},
		{"change supervisor roles", http.MethodPost, "/api/v1/admin/users/" + strconv.FormatInt(adminID, 10) + "/roles", map[string]any{"role_ids": []int64{}}},
		{"change admin role", http.MethodPut, "/api/v1/admin/roles/" + strconv.FormatInt(adminRoleID, 10), map[string]any{"permission_ids": []int64{}}},
		{"delete admin role", http.MethodDelete, "/api/v1/admin/roles/" + strconv.FormatInt(adminRoleID, 10), nil},
	}
	for _, tc := range protectedRequests {
		t.Run(tc.name, func(t *testing.T) {
			status, response := requestJSON(t, client, tc.method, server.URL+tc.path, tc.body, adminToken)
			if status != http.StatusForbidden || response.Code != 10003 {
				t.Fatalf("protected request should be forbidden: status=%d code=%d message=%s", status, response.Code, response.Message)
			}
		})
	}

	var statusValue int
	if err := db.QueryRow(`SELECT status FROM users WHERE id = ?`, adminID).Scan(&statusValue); err != nil || statusValue != 1 {
		t.Fatalf("supervisor must remain enabled: status=%d err=%v", statusValue, err)
	}

	_, registered := postJSON(t, client, server.URL+"/api/v1/auth/register", map[string]string{
		"username": "ordinary_user",
		"password": "secret123",
	}, "")
	var ordinary struct {
		ID int64 `json:"id"`
	}
	if err := json.Unmarshal(registered.Data, &ordinary); err != nil {
		t.Fatalf("decode ordinary user: %v", err)
	}
	status, response := postJSON(t, client, server.URL+"/api/v1/admin/users/"+strconv.FormatInt(ordinary.ID, 10)+"/roles", map[string]any{
		"role_ids": []int64{adminRoleID},
	}, adminToken)
	if status != http.StatusForbidden || response.Code != 10003 {
		t.Fatalf("admin role assignment should be forbidden: status=%d code=%d message=%s", status, response.Code, response.Message)
	}
}
