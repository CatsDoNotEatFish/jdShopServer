package service

import (
	"errors"

	"golang.org/x/crypto/bcrypt"

	"jdShopServer/config"
	"jdShopServer/internal/model"
	"jdShopServer/internal/repository"
)

var (
	ErrWrongPassword = errors.New("wrong password")
	ErrUserNotFound  = errors.New("user not found")
)

type UserService struct {
	userRepo  *repository.UserRepo
	tokenRepo *repository.TokenRepo
	accessSvc *AccessService
	smsSvc    *SMSService
	cfg       config.AuthConfig
}

func NewUserService(userRepo *repository.UserRepo, tokenRepo *repository.TokenRepo, accessSvc *AccessService, smsSvc *SMSService, cfg config.AuthConfig) *UserService {
	return &UserService{userRepo: userRepo, tokenRepo: tokenRepo, accessSvc: accessSvc, smsSvc: smsSvc, cfg: cfg}
}

func (s *UserService) GetProfile(userID int64) (*model.User, error) {
	user, err := s.userRepo.FindByID(userID)
	if err != nil {
		return nil, err
	}
	if user == nil {
		return nil, ErrUserNotFound
	}
	access, err := s.accessSvc.Evaluate(user)
	if err != nil {
		return nil, err
	}
	user.Access = access
	return user, nil
}

func (s *UserService) UpdateProfile(userID int64, req model.UpdateProfileRequest) (*model.User, error) {
	var nickname, email, avatarURL *string
	if req.Nickname != "" {
		nickname = &req.Nickname
	}
	if req.Email != "" {
		email = &req.Email
	}
	if req.AvatarURL != "" {
		avatarURL = &req.AvatarURL
	}

	if err := s.userRepo.UpdateProfile(userID, nickname, email, avatarURL); err != nil {
		return nil, err
	}
	return s.GetProfile(userID)
}

func (s *UserService) ChangePassword(userID int64, req model.ChangePasswordRequest) error {
	user, err := s.userRepo.FindByID(userID)
	if err != nil {
		return err
	}
	if user == nil {
		return ErrUserNotFound
	}

	if user.Phone != nil && *user.Phone != "" {
		if err := s.smsSvc.VerifyCode(*user.Phone, SMSPurposePasswordReset, req.SMSCode); err != nil {
			return err
		}
	} else {
		if req.OldPassword == "" {
			return ErrWrongPassword
		}
		if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.OldPassword)); err != nil {
			return ErrWrongPassword
		}
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(req.NewPassword), s.cfg.BcryptCost)
	if err != nil {
		return err
	}

	if err := s.userRepo.UpdatePassword(userID, string(hash)); err != nil {
		return err
	}

	// Revoke all refresh tokens on password change
	s.tokenRepo.RevokeAllForUser(userID)

	return nil
}
