package handler

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"strings"

	"jdShopServer/internal/model"
	"jdShopServer/internal/service"
)

type AuthHandler struct {
	authService      *service.AuthService
	smsService       *service.SMSService
	cipher           *service.RequestCipher
	requireEncrypted bool
}

func NewAuthHandler(authService *service.AuthService, smsService *service.SMSService, cipher *service.RequestCipher, requireEncrypted bool) *AuthHandler {
	return &AuthHandler{authService: authService, smsService: smsService, cipher: cipher, requireEncrypted: requireEncrypted}
}

func (h *AuthHandler) EncryptionKey(w http.ResponseWriter, r *http.Request) {
	respondOK(w, h.cipher.PublicKey())
}

func (h *AuthHandler) decodeAuthBody(r *http.Request, v any) error {
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

func (h *AuthHandler) Captcha(w http.ResponseWriter, r *http.Request) {
	id, image, testCode, expiresAt, err := h.smsService.Captcha()
	if err != nil {
		respondError(w, 10500, "图形验证码生成失败")
		return
	}
	data := map[string]any{
		"captcha_id": id,
		"image":      image,
		"expires_at": expiresAt.Format("2006-01-02T15:04:05Z07:00"),
	}
	if testCode != "" {
		data["captcha_code"] = testCode
	}
	respondOK(w, data)
}

func (h *AuthHandler) SendSMS(w http.ResponseWriter, r *http.Request) {
	var req model.SendSMSRequest
	if err := h.decodeAuthBody(r, &req); err != nil {
		respondError(w, 10001, "请求格式错误")
		return
	}
	if msg := req.Validate(); msg != "" {
		respondError(w, 10001, msg)
		return
	}
	ip := requestIP(r)
	err := h.smsService.Send(req.Phone, req.Purpose, req.CaptchaID, req.CaptchaCode, ip)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrInvalidPhone):
			respondError(w, 10001, "手机号格式错误")
		case errors.Is(err, service.ErrCaptchaInvalid):
			respondError(w, 10001, "图形验证码错误或已过期，请刷新后重试")
		case errors.Is(err, service.ErrSMSTooFrequent):
			respondError(w, 10006, "同一手机号每分钟只能发送一条验证码")
		case errors.Is(err, service.ErrSMSDailyLimit):
			respondError(w, 10006, "该手机号今日验证码发送次数已达6次")
		case errors.Is(err, service.ErrSMSIPLimit):
			respondError(w, 10006, "当前网络验证码请求过多，请1小时后再试")
		case errors.Is(err, service.ErrSMSNotConfigured):
			respondError(w, 10503, "短信服务尚未配置")
		case errors.Is(err, service.ErrSMSInsecureEndpoint):
			respondError(w, 10503, "短信服务必须使用HTTPS安全接口")
		case errors.Is(err, service.ErrSMSProvider):
			respondError(w, 10503, "短信平台暂时不可用，请稍后重试")
		default:
			respondError(w, 10500, "短信发送失败")
		}
		return
	}
	respondMessage(w, 0, "验证码已发送，5分钟内有效")
}

func (h *AuthHandler) Register(w http.ResponseWriter, r *http.Request) {
	var req model.RegisterRequest
	if err := h.decodeAuthBody(r, &req); err != nil {
		respondError(w, 10001, "请求格式错误")
		return
	}
	if msg := req.Validate(); msg != "" {
		respondError(w, 10001, msg)
		return
	}

	user, err := h.authService.Register(req)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrUsernameTaken), errors.Is(err, service.ErrPhoneTaken):
			respondError(w, 10005, "该手机号已注册")
		case errors.Is(err, service.ErrInvalidPhone):
			respondError(w, 10001, "手机号格式错误")
		case errors.Is(err, service.ErrPhoneRequired):
			respondError(w, 10001, "新用户必须使用手机号注册")
		case errors.Is(err, service.ErrSMSCodeInvalid), errors.Is(err, service.ErrSMSCodeNotFound):
			respondError(w, 10001, "短信验证码错误")
		case errors.Is(err, service.ErrSMSCodeExpired):
			respondError(w, 10001, "短信验证码已过期，请重新获取")
		default:
			respondError(w, 10500, "服务内部错误")
		}
		return
	}

	respondOK(w, map[string]any{
		"id":       user.ID,
		"username": user.Username,
		"phone":    user.Phone,
		"nickname": user.Nickname,
	})
}

func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	var req model.LoginRequest
	if err := h.decodeAuthBody(r, &req); err != nil {
		respondError(w, 10001, "请求格式错误")
		return
	}
	if msg := req.Validate(); msg != "" {
		respondError(w, 10001, msg)
		return
	}

	ip := requestIP(r)
	userAgent := r.Header.Get("User-Agent")

	result, err := h.authService.Login(req, ip, userAgent)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrInvalidCredentials):
			respondError(w, 10001, "手机号或密码错误")
		case errors.Is(err, service.ErrInvalidPhone):
			respondError(w, 10001, "手机号格式错误")
		case errors.Is(err, service.ErrAccountDisabled):
			respondError(w, 10003, "账号已被禁用")
		case errors.Is(err, service.ErrAccountExpired):
			respondError(w, 10003, "账号使用时间已到期")
		case errors.Is(err, service.ErrTooManyAttempts):
			respondError(w, 10003, "登录尝试过于频繁，请15分钟后再试")
		default:
			respondError(w, 10500, "服务内部错误")
		}
		return
	}

	respondOK(w, result)
}

func requestIP(r *http.Request) string {
	if ip := strings.TrimSpace(r.Header.Get("X-Real-IP")); ip != "" {
		return ip
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err == nil {
		return host
	}
	return strings.TrimSpace(r.RemoteAddr)
}

func (h *AuthHandler) Refresh(w http.ResponseWriter, r *http.Request) {
	var req model.RefreshRequest
	if err := h.decodeAuthBody(r, &req); err != nil {
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
		case errors.Is(err, service.ErrAccountExpired):
			respondError(w, 10003, "账号使用时间已到期")
		default:
			respondError(w, 10500, "服务内部错误")
		}
		return
	}

	respondOK(w, result)
}

func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	var req model.RefreshRequest
	if err := h.decodeAuthBody(r, &req); err != nil {
		respondError(w, 10001, "请求格式错误")
		return
	}
	if err := h.authService.Logout(req.RefreshToken); err != nil {
		respondError(w, 10500, "服务内部错误")
		return
	}
	respondMessage(w, 0, "退出成功")
}
