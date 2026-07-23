package service

import (
	"errors"
	"time"

	"jdShopServer/internal/model"
	"jdShopServer/internal/repository"
)

const defaultLeaseSeconds = 600

type AccessService struct {
	repo               *repository.AccessRepo
	superAdminUsername string
}

var ErrInvalidExpiry = errors.New("invalid expiry")

func NewAccessService(repo *repository.AccessRepo, superAdminUsername string) *AccessService {
	if superAdminUsername == "" {
		superAdminUsername = "admin"
	}
	return &AccessService{repo: repo, superAdminUsername: superAdminUsername}
}

func (s *AccessService) CreateDefault(userID int64, usageDays int) error {
	return s.repo.CreateDefault(userID, usageDays)
}

func (s *AccessService) Evaluate(user *model.User) (model.AccountAccess, error) {
	if user.Username == s.superAdminUsername {
		access := model.AccountAccess{
			Allowed:      user.Status == 1,
			Reason:       "active",
			LeaseSeconds: defaultLeaseSeconds,
			Modules: model.ModulePermissions{
				CompetitorMonitor: true,
				MerchantBackend:   true,
				AnalysisCenter:    true,
			},
		}
		if !access.Allowed {
			access.Reason = "disabled"
		}
		return access, nil
	}
	policy, err := s.repo.FindByUserID(user.ID)
	if err != nil {
		return model.AccountAccess{}, err
	}
	if policy == nil {
		if err := s.repo.CreateDefault(user.ID, 30); err != nil {
			return model.AccountAccess{}, err
		}
		policy, err = s.repo.FindByUserID(user.ID)
		if err != nil {
			return model.AccountAccess{}, err
		}
	}

	access := model.AccountAccess{
		Allowed:      true,
		Reason:       "active",
		ExpiresAt:    policy.ExpiresAt,
		LeaseSeconds: defaultLeaseSeconds,
		Modules: model.ModulePermissions{
			CompetitorMonitor: policy.CompetitorMonitor == 1,
			MerchantBackend:   policy.MerchantBackend == 1,
			AnalysisCenter:    policy.AnalysisCenter == 1,
		},
	}
	if user.Status != 1 {
		access.Allowed = false
		access.Reason = "disabled"
		return access, nil
	}
	if policy.ExpiresAt != nil && *policy.ExpiresAt != "" {
		expiresAt, parseErr := time.Parse(time.RFC3339, *policy.ExpiresAt)
		if parseErr == nil {
			remaining := int64(time.Until(expiresAt).Seconds())
			if remaining < 0 {
				remaining = 0
				access.Allowed = false
				access.Reason = "expired"
			}
			access.RemainingSeconds = remaining
		}
	}
	return access, nil
}

func (s *AccessService) Update(user *model.User, req model.UpdateUserAccessRequest) (model.AccountAccess, error) {
	if req.ExpiresAt != nil {
		normalized, err := normalizeExpiry(*req.ExpiresAt)
		if err != nil {
			return model.AccountAccess{}, err
		}
		req.ExpiresAt = normalized
	}
	if err := s.repo.Update(user.ID, req); err != nil {
		return model.AccountAccess{}, err
	}
	return s.Evaluate(user)
}

func normalizeExpiry(value string) (*string, error) {
	if value == "" {
		return nil, nil
	}
	formats := []string{time.RFC3339, "2006-01-02T15:04", "2006-01-02 15:04:05", "2006-01-02"}
	for _, format := range formats {
		parsed, err := time.Parse(format, value)
		if err != nil {
			continue
		}
		if format == "2006-01-02" {
			parsed = parsed.Add(24*time.Hour - time.Second)
		}
		normalized := parsed.UTC().Format(time.RFC3339)
		return &normalized, nil
	}
	return nil, ErrInvalidExpiry
}
