package handler

import (
	"net/http"

	"jdShopServer/internal/middleware"
	"jdShopServer/internal/model"
	"jdShopServer/internal/service"
)

type HeartbeatHandler struct {
	svc *service.HeartbeatService
}

func NewHeartbeatHandler(svc *service.HeartbeatService) *HeartbeatHandler {
	return &HeartbeatHandler{svc: svc}
}

func (h *HeartbeatHandler) Report(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())

	var req model.HeartbeatRequest
	if err := decodeBody(r, &req); err != nil {
		respondError(w, 10001, "请求格式错误")
		return
	}
	if req.DeviceID == "" {
		respondError(w, 10001, "device_id不能为空")
		return
	}

	ip := r.Header.Get("X-Real-IP")
	if ip == "" {
		ip = r.RemoteAddr
	}

	resp, err := h.svc.Report(userID, req, ip)
	if err != nil {
		respondError(w, 10500, "服务内部错误")
		return
	}
	respondOK(w, resp)
}
