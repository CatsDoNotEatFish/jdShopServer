package handler

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

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
	req.Level = strings.ToLower(req.Level)
	if req.Level != "" && req.Level != "info" && req.Level != "warning" && req.Level != "critical" {
		respondError(w, 10001, "level须为info/warning/critical")
		return
	}

	a, err := h.svc.Create(req.Title, req.Content, req.Level, userID)
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

	if req.Level != nil {
		l := strings.ToLower(*req.Level)
		if l != "info" && l != "warning" && l != "critical" {
			respondError(w, 10001, "level须为info/warning/critical")
			return
		}
		req.Level = &l
	}

	a, err := h.svc.Update(id, req.Title, req.Content, req.Level)
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
