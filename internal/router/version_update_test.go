package router_test

import (
	"encoding/json"
	"net/http"
	"strconv"
	"testing"
)

func TestVersionPublishingAndHeartbeatComparison(t *testing.T) {
	server, _ := newTestServer(t)
	client := server.Client()

	_, adminLogin := postJSON(t, client, server.URL+"/api/v1/auth/login", map[string]string{
		"username": "admin",
		"password": "admin123",
	}, "")
	adminToken := accessTokenFromLogin(t, adminLogin)

	status, created := postJSON(t, client, server.URL+"/api/v1/admin/versions", map[string]any{
		"platform":     "windows",
		"version_code": 2026072202,
		"version_name": "0.2.2",
		"title":        "自动更新测试",
		"description":  "包含独立更新器",
		"download_url": "https://downloads.example.invalid/JDMonitor-0.2.2.zip",
		"file_size":    12345,
		"file_hash":    "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		"is_force":     true,
	}, adminToken)
	if status != http.StatusOK || created.Code != 0 {
		t.Fatalf("create version failed: status=%d code=%d message=%s", status, created.Code, created.Message)
	}
	var createdVersion struct {
		ID int64 `json:"id"`
	}
	if err := json.Unmarshal(created.Data, &createdVersion); err != nil || createdVersion.ID == 0 {
		t.Fatalf("decode created version: id=%d err=%v", createdVersion.ID, err)
	}

	status, latest := requestJSON(t, client, http.MethodGet,
		server.URL+"/api/v1/version/latest?platform=windows&current_version_code=2026072201", nil, "")
	if status != http.StatusOK || latest.Code != 0 {
		t.Fatalf("version check failed: status=%d code=%d", status, latest.Code)
	}
	var check struct {
		HasUpdate bool `json:"has_update"`
		IsForce   bool `json:"is_force"`
		Version   struct {
			VersionCode int64  `json:"version_code"`
			FileHash    string `json:"file_hash"`
		} `json:"version"`
	}
	if err := json.Unmarshal(latest.Data, &check); err != nil {
		t.Fatalf("decode version check: %v", err)
	}
	if !check.HasUpdate || !check.IsForce || check.Version.VersionCode != 2026072202 || check.Version.FileHash == "" {
		t.Fatalf("unexpected version response: %+v", check)
	}

	_, registered := postJSON(t, client, server.URL+"/api/v1/auth/register", map[string]string{
		"username": "version_user",
		"password": "secret123",
	}, "")
	if registered.Code != 0 {
		t.Fatalf("register user failed: %s", registered.Message)
	}
	_, userLogin := postJSON(t, client, server.URL+"/api/v1/auth/login", map[string]string{
		"username": "version_user",
		"password": "secret123",
	}, "")
	userToken := accessTokenFromLogin(t, userLogin)
	_, heartbeat := postJSON(t, client, server.URL+"/api/v1/heartbeat", map[string]any{
		"device_id":        "update-device",
		"platform":         "windows",
		"app_version":      "0.2.1",
		"app_version_code": 2026072201,
	}, userToken)
	var heartbeatData struct {
		HasNewVersion     bool   `json:"has_new_version"`
		LatestVersionCode int64  `json:"latest_version_code"`
		LatestFileHash    string `json:"latest_file_hash"`
		IsForceUpdate     bool   `json:"is_force_update"`
	}
	if err := json.Unmarshal(heartbeat.Data, &heartbeatData); err != nil {
		t.Fatalf("decode heartbeat: %v", err)
	}
	if !heartbeatData.HasNewVersion || !heartbeatData.IsForceUpdate || heartbeatData.LatestVersionCode != 2026072202 || heartbeatData.LatestFileHash == "" {
		t.Fatalf("unexpected heartbeat update response: %+v", heartbeatData)
	}

	_, newer := postJSON(t, client, server.URL+"/api/v1/admin/versions", map[string]any{
		"platform":     "windows",
		"version_code": 2026072203,
		"version_name": "0.2.3",
		"title":        "newer",
	}, adminToken)
	if newer.Code != 0 {
		t.Fatalf("create newer version failed: %s", newer.Message)
	}
	status, promoted := requestJSON(t, client, http.MethodPut,
		server.URL+"/api/v1/admin/versions/"+strconv.FormatInt(createdVersion.ID, 10),
		map[string]any{"is_latest": true}, adminToken)
	if status != http.StatusOK || promoted.Code != 0 {
		t.Fatalf("promote previous version failed: status=%d message=%s", status, promoted.Message)
	}
	_, latest = requestJSON(t, client, http.MethodGet,
		server.URL+"/api/v1/version/latest?platform=windows&current_version_code=2026072201", nil, "")
	if err := json.Unmarshal(latest.Data, &check); err != nil {
		t.Fatalf("decode promoted version check: %v", err)
	}
	if check.Version.VersionCode != 2026072202 {
		t.Fatalf("expected promoted version 2026072202, got %d", check.Version.VersionCode)
	}
}
