package handler

import (
	"errors"
	"net/http"

	"jdShopServer/internal/middleware"
	"jdShopServer/internal/model"
	"jdShopServer/internal/service"
)

type UserHandler struct {
	userService *service.UserService
}

func NewUserHandler(userService *service.UserService) *UserHandler {
	return &UserHandler{userService: userService}
}

func (h *UserHandler) GetProfile(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())

	user, err := h.userService.GetProfile(userID)
	if err != nil {
		if errors.Is(err, service.ErrUserNotFound) {
			respondError(w, 10004, "用户不存在")
			return
		}
		respondError(w, 10500, "服务内部错误")
		return
	}

	respondOK(w, user)
}

func (h *UserHandler) UpdateProfile(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())

	var req model.UpdateProfileRequest
	if err := decodeBody(r, &req); err != nil {
		respondError(w, 10001, "请求格式错误")
		return
	}

	user, err := h.userService.UpdateProfile(userID, req)
	if err != nil {
		respondError(w, 10500, "服务内部错误")
		return
	}

	respondOK(w, user)
}

func (h *UserHandler) ChangePassword(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())

	var req model.ChangePasswordRequest
	if err := decodeBody(r, &req); err != nil {
		respondError(w, 10001, "请求格式错误")
		return
	}
	if msg := req.Validate(); msg != "" {
		respondError(w, 10001, msg)
		return
	}

	err := h.userService.ChangePassword(userID, req)
	if err != nil {
		if errors.Is(err, service.ErrWrongPassword) {
			respondError(w, 10001, "旧密码错误")
			return
		}
		respondError(w, 10500, "服务内部错误")
		return
	}

	respondMessage(w, 0, "密码修改成功")
}
