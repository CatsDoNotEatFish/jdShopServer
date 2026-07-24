package router_test

import (
	"encoding/json"
	"net/http"
	"strconv"
	"testing"
)

type announcementListPayload struct {
	Items []struct {
		ID             int64   `json:"id"`
		Revision       int     `json:"revision"`
		Title          string  `json:"title"`
		DisplayMode    string  `json:"display_mode"`
		ShowPolicy     string  `json:"show_policy"`
		TargetType     string  `json:"target_type"`
		TargetUserIDs  []int64 `json:"target_user_ids"`
		IsRead         bool    `json:"is_read"`
		IsAcknowledged bool    `json:"is_acknowledged"`
		DeliveredCount int64   `json:"delivered_count"`
		ReadCount      int64   `json:"read_count"`
		AckCount       int64   `json:"acknowledged_count"`
	} `json:"items"`
	Total            int `json:"total"`
	UnreadCount      int `json:"unread_count"`
	RequiresAckCount int `json:"requires_ack_count"`
}

func registerAndLoginAnnouncementUser(t *testing.T, serverURL string, client *http.Client, username string) (int64, string) {
	t.Helper()
	_, registered := postJSON(t, client, serverURL+"/api/v1/auth/register", map[string]string{
		"username": username,
		"password": "secret123",
	}, "")
	if registered.Code != 0 {
		t.Fatalf("register %s failed: %s", username, registered.Message)
	}
	_, login := postJSON(t, client, serverURL+"/api/v1/auth/login", map[string]string{
		"username": username,
		"password": "secret123",
	}, "")
	var payload struct {
		AccessToken string `json:"access_token"`
		User        struct {
			ID int64 `json:"id"`
		} `json:"user"`
	}
	if err := json.Unmarshal(login.Data, &payload); err != nil {
		t.Fatalf("decode login for %s: %v", username, err)
	}
	if payload.User.ID == 0 || payload.AccessToken == "" {
		t.Fatalf("login for %s returned incomplete identity", username)
	}
	return payload.User.ID, payload.AccessToken
}

func TestAnnouncementTargetingReceiptsAndPublishedRevision(t *testing.T) {
	server, _ := newTestServer(t)
	client := server.Client()
	userID, userToken := registerAndLoginAnnouncementUser(t, server.URL, client, "announcement_user")
	_, otherToken := registerAndLoginAnnouncementUser(t, server.URL, client, "announcement_other")

	_, adminLogin := postJSON(t, client, server.URL+"/api/v1/auth/login", map[string]string{
		"username": "admin",
		"password": "admin123",
	}, "")
	adminToken := accessTokenFromLogin(t, adminLogin)

	status, created := postJSON(t, client, server.URL+"/api/v1/admin/announcements", map[string]any{
		"title":            "指定账号紧急公告",
		"content":          "请阅读并确认",
		"level":            "critical",
		"display_mode":     "modal",
		"show_policy":      "require_ack",
		"target_type":      "users",
		"target_platform":  "windows",
		"target_user_ids":  []int64{userID},
		"min_version_code": 2026072401,
		"action_text":      "查看详情",
		"action_url":       "https://www.jdshop.bbroot.com/help",
	}, adminToken)
	if status != http.StatusOK || created.Code != 0 {
		t.Fatalf("create announcement failed: status=%d code=%d message=%s", status, created.Code, created.Message)
	}
	var announcement struct {
		ID int64 `json:"id"`
	}
	if err := json.Unmarshal(created.Data, &announcement); err != nil || announcement.ID == 0 {
		t.Fatalf("decode announcement: id=%d err=%v", announcement.ID, err)
	}

	status, published := postJSON(t, client,
		server.URL+"/api/v1/admin/announcements/"+strconv.FormatInt(announcement.ID, 10)+"/publish",
		nil, adminToken)
	if status != http.StatusOK || published.Code != 0 {
		t.Fatalf("publish announcement failed: status=%d code=%d", status, published.Code)
	}

	status, userList := requestJSON(t, client, http.MethodGet,
		server.URL+"/api/v1/user/announcements?platform=windows&version_code=2026072401",
		nil, userToken)
	if status != http.StatusOK || userList.Code != 0 {
		t.Fatalf("user list failed: status=%d code=%d", status, userList.Code)
	}
	var payload announcementListPayload
	if err := json.Unmarshal(userList.Data, &payload); err != nil {
		t.Fatalf("decode user announcements: %v", err)
	}
	if payload.Total != 1 || payload.UnreadCount != 1 || payload.RequiresAckCount != 1 {
		t.Fatalf("unexpected announcement counters: %+v", payload)
	}
	item := payload.Items[0]
	if item.Revision != 1 || item.DisplayMode != "modal" || item.ShowPolicy != "require_ack" || item.IsRead {
		t.Fatalf("unexpected announcement item: %+v", item)
	}

	_, otherList := requestJSON(t, client, http.MethodGet,
		server.URL+"/api/v1/user/announcements?platform=windows&version_code=2026072401",
		nil, otherToken)
	var otherPayload announcementListPayload
	if err := json.Unmarshal(otherList.Data, &otherPayload); err != nil || otherPayload.Total != 0 {
		t.Fatalf("targeted announcement leaked to another user: %+v err=%v", otherPayload, err)
	}

	_, lowVersionList := requestJSON(t, client, http.MethodGet,
		server.URL+"/api/v1/user/announcements?platform=windows&version_code=2026072400",
		nil, userToken)
	var lowVersionPayload announcementListPayload
	if err := json.Unmarshal(lowVersionList.Data, &lowVersionPayload); err != nil || lowVersionPayload.Total != 0 {
		t.Fatalf("version-filtered announcement was returned: %+v err=%v", lowVersionPayload, err)
	}

	_, read := postJSON(t, client,
		server.URL+"/api/v1/user/announcements/"+strconv.FormatInt(announcement.ID, 10)+"/read",
		nil, userToken)
	if read.Code != 0 {
		t.Fatalf("mark read failed: %s", read.Message)
	}
	_, acknowledged := postJSON(t, client,
		server.URL+"/api/v1/user/announcements/"+strconv.FormatInt(announcement.ID, 10)+"/acknowledge",
		nil, userToken)
	if acknowledged.Code != 0 {
		t.Fatalf("acknowledge failed: %s", acknowledged.Message)
	}

	_, afterReceipt := requestJSON(t, client, http.MethodGet,
		server.URL+"/api/v1/user/announcements?platform=windows&version_code=2026072401",
		nil, userToken)
	if err := json.Unmarshal(afterReceipt.Data, &payload); err != nil {
		t.Fatalf("decode announcements after receipt: %v", err)
	}
	if payload.UnreadCount != 0 || payload.RequiresAckCount != 0 || !payload.Items[0].IsAcknowledged {
		t.Fatalf("receipt state was not persisted: %+v", payload)
	}

	status, adminList := requestJSON(t, client, http.MethodGet,
		server.URL+"/api/v1/admin/announcements?page=1&page_size=20",
		nil, adminToken)
	if status != http.StatusOK || adminList.Code != 0 {
		t.Fatalf("admin list failed: status=%d code=%d", status, adminList.Code)
	}
	var adminPayload struct {
		Items []struct {
			ID                int64 `json:"id"`
			DeliveredCount    int64 `json:"delivered_count"`
			ReadCount         int64 `json:"read_count"`
			AcknowledgedCount int64 `json:"acknowledged_count"`
		} `json:"items"`
	}
	if err := json.Unmarshal(adminList.Data, &adminPayload); err != nil {
		t.Fatalf("decode admin announcements: %v", err)
	}
	if len(adminPayload.Items) != 1 || adminPayload.Items[0].DeliveredCount != 1 ||
		adminPayload.Items[0].ReadCount != 1 || adminPayload.Items[0].AcknowledgedCount != 1 {
		t.Fatalf("admin receipt statistics are wrong: %+v", adminPayload.Items)
	}

	newTitle := "指定账号紧急公告（已更新）"
	status, updated := requestJSON(t, client, http.MethodPut,
		server.URL+"/api/v1/admin/announcements/"+strconv.FormatInt(announcement.ID, 10),
		map[string]any{
			"title":            newTitle,
			"content":          "更新后需要重新确认",
			"level":            "critical",
			"display_mode":     "modal",
			"show_policy":      "require_ack",
			"target_type":      "users",
			"target_platform":  "windows",
			"target_user_ids":  []int64{userID},
			"min_version_code": 2026072401,
			"max_version_code": 0,
			"action_text":      "",
			"action_url":       "",
		},
		adminToken)
	if status != http.StatusOK || updated.Code != 0 {
		t.Fatalf("update published announcement failed: status=%d code=%d message=%s", status, updated.Code, updated.Message)
	}
	_, revisedList := requestJSON(t, client, http.MethodGet,
		server.URL+"/api/v1/user/announcements?platform=windows&version_code=2026072401",
		nil, userToken)
	if err := json.Unmarshal(revisedList.Data, &payload); err != nil {
		t.Fatalf("decode revised announcements: %v", err)
	}
	if payload.Items[0].Revision != 2 || payload.Items[0].IsRead || payload.UnreadCount != 1 {
		t.Fatalf("published edit did not create a new unread revision: %+v", payload)
	}
}
