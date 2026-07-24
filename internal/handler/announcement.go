package handler

import (
	"errors"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"jdShopServer/internal/middleware"
	"jdShopServer/internal/model"
	"jdShopServer/internal/service"
)

type AnnouncementHandler struct {
	svc *service.AnnouncementService
}

func NewAnnouncementHandler(svc *service.AnnouncementService) *AnnouncementHandler {
	return &AnnouncementHandler{svc: svc}
}

func (h *AnnouncementHandler) PublicList(w http.ResponseWriter, r *http.Request) {
	page := getQueryInt(r, "page", 1)
	pageSize := getQueryInt(r, "page_size", 20)
	level := r.URL.Query().Get("level")

	items, total, err := h.svc.ListPublic(page, pageSize, level)
	if err != nil {
		respondError(w, 10500, "服务内部错误")
		return
	}

	if items == nil {
		items = []model.Announcement{}
	}
	respondPaginated(w, items, total, page, pageSize)
}

func (h *AnnouncementHandler) AdminList(w http.ResponseWriter, r *http.Request) {
	page := getQueryInt(r, "page", 1)
	pageSize := getQueryInt(r, "page_size", 20)
	level := r.URL.Query().Get("level")

	items, total, err := h.svc.ListAll(page, pageSize, level)
	if err != nil {
		respondError(w, 10500, "服务内部错误")
		return
	}

	if items == nil {
		items = []model.Announcement{}
	}
	respondPaginated(w, items, total, page, pageSize)
}

func (h *AnnouncementHandler) UserList(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	platform := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("platform")))
	if platform == "" {
		platform = "windows"
	}
	versionCode := int64(getQueryInt(r, "version_code", 0))
	items, err := h.svc.ListForUser(userID, platform, versionCode)
	if err != nil {
		respondError(w, 10500, "服务内部错误")
		return
	}
	unreadCount := 0
	requiresAckCount := 0
	for _, item := range items {
		if !item.IsRead {
			unreadCount++
		}
		if item.ShowPolicy == "require_ack" && !item.IsAcknowledged {
			requiresAckCount++
		}
	}
	respondOK(w, map[string]any{
		"items":              items,
		"total":              len(items),
		"unread_count":       unreadCount,
		"requires_ack_count": requiresAckCount,
	})
}

func (h *AnnouncementHandler) Create(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())

	var req model.CreateAnnouncementRequest
	if err := decodeBody(r, &req); err != nil {
		respondError(w, 10001, "请求格式错误")
		return
	}
	if msg := req.Validate(); msg != "" {
		respondError(w, 10001, msg)
		return
	}
	normalizeAnnouncementCreate(&req)
	if msg := validateAnnouncementConfig(
		req.Level, req.DisplayMode, req.ShowPolicy, req.TargetType, req.TargetPlatform,
		req.StartsAt, req.EndsAt, req.MinVersionCode, req.MaxVersionCode,
		req.ActionURL, req.TargetUserIDs,
	); msg != "" {
		respondError(w, 10001, msg)
		return
	}
	a, err := h.svc.Create(&model.Announcement{
		Title:          strings.TrimSpace(req.Title),
		Content:        strings.TrimSpace(req.Content),
		Level:          req.Level,
		DisplayMode:    req.DisplayMode,
		ShowPolicy:     req.ShowPolicy,
		StartsAt:       cleanRequestString(req.StartsAt),
		EndsAt:         cleanRequestString(req.EndsAt),
		TargetType:     req.TargetType,
		TargetPlatform: req.TargetPlatform,
		MinVersionCode: positiveRequestInt(req.MinVersionCode),
		MaxVersionCode: positiveRequestInt(req.MaxVersionCode),
		ActionText:     cleanRequestString(req.ActionText),
		ActionURL:      cleanRequestString(req.ActionURL),
		TargetUserIDs:  req.TargetUserIDs,
	}, userID)
	if err != nil {
		respondError(w, 10500, "服务内部错误")
		return
	}
	respondOK(w, a)
}

func (h *AnnouncementHandler) Update(w http.ResponseWriter, r *http.Request) {
	id, err := parseIDParam(r, "id")
	if err != nil {
		respondError(w, 10001, "ID格式错误")
		return
	}

	var req model.UpdateAnnouncementRequest
	if err := decodeBody(r, &req); err != nil {
		respondError(w, 10001, "请求格式错误")
		return
	}

	normalizeAnnouncementUpdate(&req)
	if req.Title != nil && strings.TrimSpace(*req.Title) == "" {
		respondError(w, 10001, "公告标题不能为空")
		return
	}
	if req.Content != nil && strings.TrimSpace(*req.Content) == "" {
		respondError(w, 10001, "公告内容不能为空")
		return
	}
	if msg := validateAnnouncementConfig(
		valueOrEmpty(req.Level), valueOrEmpty(req.DisplayMode), valueOrEmpty(req.ShowPolicy),
		valueOrEmpty(req.TargetType), valueOrEmpty(req.TargetPlatform),
		req.StartsAt, req.EndsAt, req.MinVersionCode, req.MaxVersionCode,
		req.ActionURL, req.TargetUserIDs,
	); msg != "" {
		respondError(w, 10001, msg)
		return
	}
	a, err := h.svc.Update(id, req)
	if err != nil {
		if errors.Is(err, service.ErrAnnouncementNotFound) {
			respondError(w, 10004, "公告不存在")
			return
		}
		respondError(w, 10500, "服务内部错误")
		return
	}
	respondOK(w, a)
}

func (h *AnnouncementHandler) MarkRead(w http.ResponseWriter, r *http.Request) {
	h.markReceipt(w, r, false)
}

func (h *AnnouncementHandler) Acknowledge(w http.ResponseWriter, r *http.Request) {
	h.markReceipt(w, r, true)
}

func (h *AnnouncementHandler) markReceipt(w http.ResponseWriter, r *http.Request, acknowledge bool) {
	id, err := parseIDParam(r, "id")
	if err != nil {
		respondError(w, 10001, "ID格式错误")
		return
	}
	userID := middleware.GetUserID(r.Context())
	if acknowledge {
		err = h.svc.Acknowledge(id, userID)
	} else {
		err = h.svc.MarkRead(id, userID)
	}
	if errors.Is(err, service.ErrAnnouncementNotFound) {
		respondError(w, 10004, "公告不存在或不在投放范围内")
		return
	}
	if err != nil {
		respondError(w, 10500, "服务内部错误")
		return
	}
	respondMessage(w, 0, map[bool]string{true: "已确认", false: "已读"}[acknowledge])
}

func (h *AnnouncementHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id, err := parseIDParam(r, "id")
	if err != nil {
		respondError(w, 10001, "ID格式错误")
		return
	}

	if err := h.svc.Delete(id); err != nil {
		if errors.Is(err, service.ErrAnnouncementNotFound) {
			respondError(w, 10004, "公告不存在")
			return
		}
		respondError(w, 10500, "服务内部错误")
		return
	}
	respondMessage(w, 0, "删除成功")
}

func (h *AnnouncementHandler) Publish(w http.ResponseWriter, r *http.Request) {
	id, err := parseIDParam(r, "id")
	if err != nil {
		respondError(w, 10001, "ID格式错误")
		return
	}

	if err := h.svc.Publish(id); err != nil {
		if errors.Is(err, service.ErrAnnouncementNotFound) {
			respondError(w, 10004, "公告不存在")
			return
		}
		respondError(w, 10500, "服务内部错误")
		return
	}
	respondMessage(w, 0, "发布成功")
}

func (h *AnnouncementHandler) Unpublish(w http.ResponseWriter, r *http.Request) {
	id, err := parseIDParam(r, "id")
	if err != nil {
		respondError(w, 10001, "ID格式错误")
		return
	}

	if err := h.svc.Unpublish(id); err != nil {
		if errors.Is(err, service.ErrAnnouncementNotFound) {
			respondError(w, 10004, "公告不存在")
			return
		}
		respondError(w, 10500, "服务内部错误")
		return
	}
	respondMessage(w, 0, "已下架")
}

func parseIDParam(r *http.Request, name string) (int64, error) {
	return strconv.ParseInt(chi.URLParam(r, name), 10, 64)
}

func normalizeAnnouncementCreate(req *model.CreateAnnouncementRequest) {
	req.Level = strings.ToLower(strings.TrimSpace(req.Level))
	req.DisplayMode = strings.ToLower(strings.TrimSpace(req.DisplayMode))
	req.ShowPolicy = strings.ToLower(strings.TrimSpace(req.ShowPolicy))
	req.TargetType = strings.ToLower(strings.TrimSpace(req.TargetType))
	req.TargetPlatform = strings.ToLower(strings.TrimSpace(req.TargetPlatform))
	if req.Level == "" {
		req.Level = "info"
	}
	if req.DisplayMode == "" {
		req.DisplayMode = "center"
	}
	if req.ShowPolicy == "" {
		req.ShowPolicy = "once"
	}
	if req.TargetType == "" {
		req.TargetType = "all"
	}
	if req.TargetPlatform == "" {
		req.TargetPlatform = "all"
	}
}

func normalizeAnnouncementUpdate(req *model.UpdateAnnouncementRequest) {
	for _, value := range []*string{req.Level, req.DisplayMode, req.ShowPolicy, req.TargetType, req.TargetPlatform} {
		if value != nil {
			*value = strings.ToLower(strings.TrimSpace(*value))
		}
	}
	if req.Title != nil {
		*req.Title = strings.TrimSpace(*req.Title)
	}
	if req.Content != nil {
		*req.Content = strings.TrimSpace(*req.Content)
	}
}

func validateAnnouncementConfig(
	level, displayMode, showPolicy, targetType, targetPlatform string,
	startsAt, endsAt *string,
	minVersionCode, maxVersionCode *int64,
	actionURL *string,
	targetUserIDs []int64,
) string {
	if level != "" && level != "info" && level != "warning" && level != "critical" {
		return "level须为info/warning/critical"
	}
	if displayMode != "" && displayMode != "center" && displayMode != "banner" && displayMode != "modal" {
		return "展示方式须为center/banner/modal"
	}
	if showPolicy != "" && showPolicy != "once" && showPolicy != "every_start" && showPolicy != "require_ack" {
		return "提醒策略须为once/every_start/require_ack"
	}
	if targetType != "" && targetType != "all" && targetType != "users" {
		return "投放范围须为all/users"
	}
	if targetPlatform != "" && targetPlatform != "all" && targetPlatform != "windows" {
		return "目标平台须为all/windows"
	}
	if targetType == "users" && len(targetUserIDs) == 0 {
		return "指定账号投放时至少选择一个账号"
	}
	startTime, startOK := parseOptionalRFC3339(startsAt)
	endTime, endOK := parseOptionalRFC3339(endsAt)
	if !startOK || !endOK {
		return "生效时间和失效时间必须是RFC3339格式"
	}
	if !startTime.IsZero() && !endTime.IsZero() && !endTime.After(startTime) {
		return "失效时间必须晚于生效时间"
	}
	if minVersionCode != nil && maxVersionCode != nil && *minVersionCode > 0 && *maxVersionCode > 0 && *minVersionCode > *maxVersionCode {
		return "最低版本码不能大于最高版本码"
	}
	if actionURL != nil && strings.TrimSpace(*actionURL) != "" {
		parsed, err := url.Parse(strings.TrimSpace(*actionURL))
		if err != nil || parsed.Scheme != "https" || parsed.Host == "" {
			return "操作链接必须是有效的HTTPS地址"
		}
	}
	return ""
}

func parseOptionalRFC3339(value *string) (time.Time, bool) {
	if value == nil || strings.TrimSpace(*value) == "" {
		return time.Time{}, true
	}
	parsed, err := time.Parse(time.RFC3339, strings.TrimSpace(*value))
	return parsed, err == nil
}

func cleanRequestString(value *string) *string {
	if value == nil {
		return nil
	}
	cleaned := strings.TrimSpace(*value)
	if cleaned == "" {
		return nil
	}
	return &cleaned
}

func positiveRequestInt(value *int64) *int64 {
	if value == nil || *value <= 0 {
		return nil
	}
	return value
}

func valueOrEmpty(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}
