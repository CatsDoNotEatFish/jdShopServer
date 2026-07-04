package handler

import (
	"errors"
	"net/http"

	"jdShopServer/internal/model"
	"jdShopServer/internal/service"
)

type VersionHandler struct {
	svc *service.VersionService
}

func NewVersionHandler(svc *service.VersionService) *VersionHandler {
	return &VersionHandler{svc: svc}
}

func (h *VersionHandler) CheckLatest(w http.ResponseWriter, r *http.Request) {
	platform := r.URL.Query().Get("platform")
	if platform == "" {
		platform = "windows"
	}

	currentCode := getQueryInt64(r, "current_version_code", 0)

	resp, err := h.svc.CheckLatest(platform, currentCode)
	if err != nil {
		respondError(w, 10500, "服务内部错误")
		return
	}
	respondOK(w, resp)
}

func (h *VersionHandler) AdminList(w http.ResponseWriter, r *http.Request) {
	page := getQueryInt(r, "page", 1)
	pageSize := getQueryInt(r, "page_size", 20)
	platform := r.URL.Query().Get("platform")

	items, total, err := h.svc.List(page, pageSize, platform)
	if err != nil {
		respondError(w, 10500, "服务内部错误")
		return
	}

	if items == nil {
		items = []model.AppVersion{}
	}
	respondPaginated(w, items, total, page, pageSize)
}

func (h *VersionHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req model.CreateVersionRequest
	if err := decodeBody(r, &req); err != nil {
		respondError(w, 10001, "请求格式错误")
		return
	}
	if msg := req.Validate(); msg != "" {
		respondError(w, 10001, msg)
		return
	}

	v, err := h.svc.Create(req)
	if err != nil {
		respondError(w, 10500, "服务内部错误")
		return
	}
	respondOK(w, v)
}

func (h *VersionHandler) Update(w http.ResponseWriter, r *http.Request) {
	id, err := parseIDParam(r, "id")
	if err != nil {
		respondError(w, 10001, "ID格式错误")
		return
	}

	var req model.UpdateVersionRequest
	if err := decodeBody(r, &req); err != nil {
		respondError(w, 10001, "请求格式错误")
		return
	}

	v, err := h.svc.Update(id, req)
	if err != nil {
		if errors.Is(err, service.ErrVersionNotFound) {
			respondError(w, 10004, "版本不存在")
			return
		}
		respondError(w, 10500, "服务内部错误")
		return
	}
	respondOK(w, v)
}

func (h *VersionHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id, err := parseIDParam(r, "id")
	if err != nil {
		respondError(w, 10001, "ID格式错误")
		return
	}

	if err := h.svc.Delete(id); err != nil {
		if errors.Is(err, service.ErrVersionNotFound) {
			respondError(w, 10004, "版本不存在")
			return
		}
		respondError(w, 10500, "服务内部错误")
		return
	}
	respondMessage(w, 0, "删除成功")
}
