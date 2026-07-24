package handler

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"

	"jdShopServer/internal/middleware"
	"jdShopServer/internal/model"
	"jdShopServer/internal/service"
)

type UserHandler struct {
	userService      *service.UserService
	cipher           *service.RequestCipher
	requireEncrypted bool
}

func NewUserHandler(userService *service.UserService, cipher *service.RequestCipher, requireEncrypted bool) *UserHandler {
	return &UserHandler{userService: userService, cipher: cipher, requireEncrypted: requireEncrypted}
}

func (h *UserHandler) decodeUserBody(r *http.Request, v any) error {
	raw, err := io.ReadAll(r.Body)
	if err != nil {
		return err
	}
	contentType := strings.ToLower(strings.TrimSpace(strings.Split(r.Header.Get("Content-Type"), ";")[0]))
	if contentType == "application/jdshop-encrypted+json" {
		if h.cipher == nil {
			return errors.New("encrypted request service unavailable")
		}
		raw, err = h.cipher.Decrypt(r.URL.Path, raw)
		if err != nil {
			return err
		}
	} else if h.requireEncrypted {
		return errors.New("该接口必须使用加密请求")
	}
	return json.NewDecoder(bytes.NewReader(raw)).Decode(v)
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
	if err := h.decodeUserBody(r, &req); err != nil {
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
	if err := h.decodeUserBody(r, &req); err != nil {
		respondError(w, 10001, "请求格式错误")
		return
	}
	if msg := req.Validate(); msg != "" {
		respondError(w, 10001, msg)
		return
	}

	err := h.userService.ChangePassword(userID, req)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrWrongPassword):
			respondError(w, 10001, "旧密码错误")
		case errors.Is(err, service.ErrSMSCodeInvalid), errors.Is(err, service.ErrSMSCodeNotFound):
			respondError(w, 10001, "短信验证码错误")
		case errors.Is(err, service.ErrSMSCodeExpired):
			respondError(w, 10001, "短信验证码已过期，请重新获取")
		default:
			respondError(w, 10500, "服务内部错误")
		}
		return
	}

	respondMessage(w, 0, "密码修改成功")
}
