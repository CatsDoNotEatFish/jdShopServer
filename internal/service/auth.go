package service

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"

	"jdShopServer/config"
	"jdShopServer/internal/model"
	"jdShopServer/internal/repository"
)

var (
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrAccountDisabled    = errors.New("account disabled")
	ErrAccountExpired     = errors.New("account expired")
	ErrTooManyAttempts    = errors.New("too many login attempts")
	ErrTokenRevoked       = errors.New("token revoked")
	ErrTokenExpired       = errors.New("token expired")
	ErrUsernameTaken      = errors.New("username taken")
)

type AuthService struct {
	userRepo     *repository.UserRepo
	tokenRepo    *repository.TokenRepo
	loginLogRepo *repository.LoginLogRepo
	cfg          config.AuthConfig
	accessSvc    *AccessService
}

func NewAuthService(userRepo *repository.UserRepo, tokenRepo *repository.TokenRepo,
	loginLogRepo *repository.LoginLogRepo, accessSvc *AccessService, cfg config.AuthConfig) *AuthService {
	return &AuthService{
		userRepo:     userRepo,
		tokenRepo:    tokenRepo,
		loginLogRepo: loginLogRepo,
		accessSvc:    accessSvc,
		cfg:          cfg,
	}
}

func (s *AuthService) Register(req model.RegisterRequest) (*model.User, error) {
	existing, err := s.userRepo.FindByUsername(req.Username)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		return nil, ErrUsernameTaken
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), s.cfg.BcryptCost)
	if err != nil {
		return nil, err
	}

	user := &model.User{
		Username:     req.Username,
		PasswordHash: string(hash),
		Status:       1,
	}

	if req.Email != "" {
		user.Email = &req.Email
	}
	if req.Nickname != "" {
		user.Nickname = &req.Nickname
	}

	if err := s.userRepo.Create(user); err != nil {
		return nil, err
	}

	// Assign default "user" role
	if err := s.userRepo.AssignRoles(user.ID, []int64{2}); err != nil {
		return nil, err
	}
	if err := s.accessSvc.CreateDefault(user.ID, s.cfg.DefaultUsageDays); err != nil {
		return nil, err
	}

	return user, nil
}

func (s *AuthService) Login(req model.LoginRequest, ip, userAgent string) (*model.LoginResponse, error) {
	// Check rate limit
	failures, err := s.loginLogRepo.CountFailures(req.Username, ip, s.cfg.LoginLockMinutes)
	if err != nil {
		return nil, err
	}
	if failures >= s.cfg.LoginMaxAttempts {
		s.logLogin(nil, req.Username, ip, userAgent, "locked", "too many attempts")
		return nil, ErrTooManyAttempts
	}

	user, err := s.userRepo.FindByUsername(req.Username)
	if err != nil {
		return nil, err
	}
	if user == nil {
		s.logLogin(nil, req.Username, ip, userAgent, "failed", "user not found")
		return nil, ErrInvalidCredentials
	}

	if user.Status != 1 {
		s.logLogin(&user.ID, req.Username, ip, userAgent, "failed", "account disabled")
		return nil, ErrAccountDisabled
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)); err != nil {
		s.logLogin(&user.ID, req.Username, ip, userAgent, "failed", "wrong password")
		return nil, ErrInvalidCredentials
	}

	access, err := s.accessSvc.Evaluate(user)
	if err != nil {
		return nil, err
	}
	if !access.Allowed && access.Reason == "expired" {
		s.logLogin(&user.ID, req.Username, ip, userAgent, "failed", "account expired")
		return nil, ErrAccountExpired
	}

	s.logLogin(&user.ID, req.Username, ip, userAgent, "success", "")
	s.userRepo.UpdateLastLogin(user.ID)

	roles, _ := s.userRepo.UserRoles(user.ID)

	accessToken, err := s.generateAccessToken(user, roles)
	if err != nil {
		return nil, err
	}

	refreshToken, err := s.generateRefreshToken(user.ID)
	if err != nil {
		return nil, err
	}

	nickname := ""
	if user.Nickname != nil {
		nickname = *user.Nickname
	}

	return &model.LoginResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		ExpiresIn:    s.cfg.AccessTokenTTL,
		User: model.UserInfo{
			ID:       user.ID,
			Username: user.Username,
			Nickname: nickname,
			Roles:    roles,
			Status:   user.Status,
			Access:   access,
		},
	}, nil
}

func (s *AuthService) RefreshToken(refreshTokenStr string) (*model.RefreshResponse, error) {
	hash := hashToken(refreshTokenStr)

	stored, err := s.tokenRepo.FindByHash(hash)
	if err != nil {
		return nil, err
	}
	if stored == nil {
		return nil, ErrTokenRevoked
	}
	if stored.Revoked == 1 {
		return nil, ErrTokenRevoked
	}

	expiry, err := time.Parse(time.RFC3339, stored.ExpiresAt)
	if err != nil || time.Now().UTC().After(expiry) {
		s.tokenRepo.Revoke(hash)
		return nil, ErrTokenExpired
	}

	// Rotate: revoke old, issue new
	s.tokenRepo.Revoke(hash)

	user, err := s.userRepo.FindByID(stored.UserID)
	if err != nil || user == nil {
		return nil, ErrTokenRevoked
	}
	if user.Status != 1 {
		return nil, ErrAccountDisabled
	}
	access, err := s.accessSvc.Evaluate(user)
	if err != nil {
		return nil, err
	}
	if !access.Allowed {
		if access.Reason == "expired" {
			return nil, ErrAccountExpired
		}
		return nil, ErrAccountDisabled
	}

	roles, _ := s.userRepo.UserRoles(user.ID)

	accessToken, err := s.generateAccessToken(user, roles)
	if err != nil {
		return nil, err
	}

	newRefreshToken, err := s.generateRefreshToken(user.ID)
	if err != nil {
		return nil, err
	}

	return &model.RefreshResponse{
		AccessToken:  accessToken,
		RefreshToken: newRefreshToken,
		ExpiresIn:    s.cfg.AccessTokenTTL,
		Access:       access,
	}, nil
}

func (s *AuthService) Logout(refreshTokenStr string) error {
	if refreshTokenStr == "" {
		return nil
	}
	return s.tokenRepo.Revoke(hashToken(refreshTokenStr))
}

func (s *AuthService) generateAccessToken(user *model.User, roles []string) (string, error) {
	now := time.Now().UTC()
	authVersion, err := s.userRepo.AuthVersion(user.ID)
	if err != nil {
		return "", err
	}
	claims := jwt.MapClaims{
		"sub":          user.ID,
		"username":     user.Username,
		"nickname":     user.Nickname,
		"roles":        roles,
		"auth_version": authVersion,
		"iat":          now.Unix(),
		"exp":          now.Add(time.Duration(s.cfg.AccessTokenTTL) * time.Second).Unix(),
		"jti":          randomID(),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(s.cfg.JWTSecret))
}

func (s *AuthService) generateRefreshToken(userID int64) (string, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	token := base64.URLEncoding.EncodeToString(raw)

	hash := hashToken(token)
	expiry := time.Now().UTC().Add(time.Duration(s.cfg.RefreshTokenTTL) * time.Second).Format(time.RFC3339)

	rt := &model.RefreshToken{
		UserID:    userID,
		TokenHash: hash,
		ExpiresAt: expiry,
	}
	if err := s.tokenRepo.Create(rt); err != nil {
		return "", err
	}

	return token, nil
}

func (s *AuthService) logLogin(userID *int64, username, ip, userAgent, result, reason string) {
	log := &model.LoginLog{
		UserID:    userID,
		Username:  username,
		IPAddress: &ip,
		UserAgent: &userAgent,
		Result:    result,
	}
	if reason != "" {
		log.FailReason = &reason
	}
	s.loginLogRepo.Create(log)
}

func hashToken(token string) string {
	h := sha256.Sum256([]byte(token))
	return fmt.Sprintf("%x", h)
}

func randomID() string {
	b := make([]byte, 8)
	rand.Read(b)
	return fmt.Sprintf("%x", b)
}
