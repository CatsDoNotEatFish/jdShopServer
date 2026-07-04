package handler

import (
	"errors"
	"net/http"

	"jdShopServer/internal/model"
	"jdShopServer/internal/service"
)

type AuthHandler struct {
	authService *service.AuthService
}

func NewAuthHandler(authService *service.AuthService) *AuthHandler {
	return &AuthHandler{authService: authService}
}

func (h *AuthHandler) Register(w http.ResponseWriter, r *http.Request) {
	var req model.RegisterRequest
	if err := decodeBody(r, &req); err != nil {
		respondError(w, 10001, "请求格式错误")
		return
	}
	if msg := req.Validate(); msg != "" {
		respondError(w, 10001, msg)
		return
	}

	user, err := h.authService.Register(req)
	if err != nil {
		if errors.Is(err, service.ErrUsernameTaken) {
			respondError(w, 10005, "用户名已被占用")
			return
		}
		respondError(w, 10500, "服务内部错误")
		return
	}

	respondOK(w, map[string]any{
		"id":       user.ID,
		"username": user.Username,
		"nickname": user.Nickname,
	})
}

func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	var req model.LoginRequest
	if err := decodeBody(r, &req); err != nil {
		respondError(w, 10001, "请求格式错误")
		return
	}
	if msg := req.Validate(); msg != "" {
		respondError(w, 10001, msg)
		return
	}

	ip := r.Header.Get("X-Real-IP")
	if ip == "" {
		ip = r.RemoteAddr
	}
	userAgent := r.Header.Get("User-Agent")

	result, err := h.authService.Login(req, ip, userAgent)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrInvalidCredentials):
			respondError(w, 10001, "用户名或密码错误")
		case errors.Is(err, service.ErrAccountDisabled):
			respondError(w, 10003, "账号已被禁用")
		case errors.Is(err, service.ErrTooManyAttempts):
			respondError(w, 10003, "登录尝试过于频繁，请15分钟后再试")
		default:
			respondError(w, 10500, "服务内部错误")
		}
		return
	}

	respondOK(w, result)
}

func (h *AuthHandler) Refresh(w http.ResponseWriter, r *http.Request) {
	var req model.RefreshRequest
	if err := decodeBody(r, &req); err != nil {
		respondError(w, 10001, "请求格式错误")
		return
	}
	if req.RefreshToken == "" {
		respondError(w, 10001, "refresh_token不能为空")
		return
	}

	result, err := h.authService.RefreshToken(req.RefreshToken)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrTokenRevoked):
			respondError(w, 10002, "Token已失效，请重新登录")
		case errors.Is(err, service.ErrTokenExpired):
			respondError(w, 10002, "Token已过期，请重新登录")
		case errors.Is(err, service.ErrAccountDisabled):
			respondError(w, 10003, "账号已被禁用")
		default:
			respondError(w, 10500, "服务内部错误")
		}
		return
	}

	respondOK(w, result)
}
