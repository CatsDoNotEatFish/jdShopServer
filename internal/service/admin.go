package service

import (
	"errors"

	"jdShopServer/internal/model"
	"jdShopServer/internal/repository"
)

var (
	ErrRoleNotFound        = errors.New("role not found")
	ErrSuperAdminProtected = errors.New("super admin account is protected")
	ErrAdminRoleReserved   = errors.New("admin role is reserved for the super admin")
	ErrAdminRoleProtected  = errors.New("admin role is protected")
)

type AdminService struct {
	userRepo           *repository.UserRepo
	roleRepo           *repository.RoleRepo
	accessSvc          *AccessService
	tokenRepo          *repository.TokenRepo
	controlHub         *ControlHub
	superAdminUsername string
}

func NewAdminService(userRepo *repository.UserRepo, roleRepo *repository.RoleRepo, accessSvc *AccessService, tokenRepo *repository.TokenRepo, controlHub *ControlHub, superAdminUsername string) *AdminService {
	if superAdminUsername == "" {
		superAdminUsername = "admin"
	}
	return &AdminService{
		userRepo: userRepo, roleRepo: roleRepo, accessSvc: accessSvc,
		tokenRepo: tokenRepo, controlHub: controlHub,
		superAdminUsername: superAdminUsername,
	}
}

func (s *AdminService) isSuperAdmin(user *model.User) bool {
	return user != nil && user.Username == s.superAdminUsername
}

func (s *AdminService) UpdateUserAccess(userID int64, req model.UpdateUserAccessRequest) (model.AccountAccess, error) {
	user, err := s.userRepo.FindByID(userID)
	if err != nil {
		return model.AccountAccess{}, err
	}
	if user == nil {
		return model.AccountAccess{}, ErrUserNotFound
	}
	if s.isSuperAdmin(user) {
		return model.AccountAccess{}, ErrSuperAdminProtected
	}
	access, err := s.accessSvc.Update(user, req)
	if err == nil {
		s.controlHub.Publish(userID, "access_changed")
	}
	return access, err
}

func (s *AdminService) RegistrationDefaults() (model.RegistrationDefaults, error) {
	return s.accessSvc.RegistrationDefaults()
}

func (s *AdminService) UpdateRegistrationDefaults(req model.UpdateRegistrationDefaultsRequest) (model.RegistrationDefaults, error) {
	return s.accessSvc.UpdateRegistrationDefaults(req)
}

func (s *AdminService) ListUsers(page, pageSize int, keyword string, status *int) ([]model.UserWithRoles, int64, error) {
	users, total, err := s.userRepo.List(page, pageSize, keyword, status)
	if err != nil {
		return nil, 0, err
	}
	for i := range users {
		access, accessErr := s.accessSvc.Evaluate(&users[i].User)
		if accessErr != nil {
			return nil, 0, accessErr
		}
		users[i].Access = access
	}
	return users, total, nil
}

func (s *AdminService) UpdateUserStatus(userID int64, status int) error {
	user, err := s.userRepo.FindByID(userID)
	if err != nil {
		return err
	}
	if user == nil {
		return ErrUserNotFound
	}
	if s.isSuperAdmin(user) {
		return ErrSuperAdminProtected
	}
	if err := s.userRepo.UpdateStatus(userID, status); err != nil {
		return err
	}
	if status == 0 {
		if err := s.tokenRepo.RevokeAllForUser(userID); err != nil {
			return err
		}
	}
	s.controlHub.Publish(userID, "account_status_changed")
	return nil
}

func (s *AdminService) AssignUserRoles(userID int64, roleIDs []int64) error {
	user, err := s.userRepo.FindByID(userID)
	if err != nil {
		return err
	}
	if user == nil {
		return ErrUserNotFound
	}
	if s.isSuperAdmin(user) {
		return ErrSuperAdminProtected
	}
	for _, roleID := range roleIDs {
		role, err := s.roleRepo.FindByID(roleID)
		if err != nil {
			return err
		}
		if role != nil && role.Name == "admin" {
			return ErrAdminRoleReserved
		}
	}
	return s.userRepo.AssignRoles(userID, roleIDs)
}

func (s *AdminService) ListRoles() ([]model.Role, error) {
	return s.roleRepo.List()
}

func (s *AdminService) CreateRole(req model.CreateRoleRequest) (*model.Role, error) {
	role := &model.Role{
		Name: req.Name,
	}
	if req.Description != "" {
		role.Description = &req.Description
	}

	if err := s.roleRepo.Create(role); err != nil {
		return nil, err
	}

	if len(req.PermissionIDs) > 0 {
		s.roleRepo.SetPermissions(role.ID, req.PermissionIDs)
	}

	return s.roleRepo.FindByID(role.ID)
}

func (s *AdminService) UpdateRole(id int64, req model.UpdateRoleRequest) (*model.Role, error) {
	role, err := s.roleRepo.FindByID(id)
	if err != nil {
		return nil, err
	}
	if role == nil {
		return nil, ErrRoleNotFound
	}
	if role.Name == "admin" {
		return nil, ErrAdminRoleProtected
	}

	if err := s.roleRepo.Update(id, req.Name, req.Description); err != nil {
		return nil, err
	}

	if req.PermissionIDs != nil {
		s.roleRepo.SetPermissions(id, req.PermissionIDs)
	}

	return s.roleRepo.FindByID(id)
}

func (s *AdminService) DeleteRole(id int64) error {
	role, err := s.roleRepo.FindByID(id)
	if err != nil {
		return err
	}
	if role == nil {
		return ErrRoleNotFound
	}
	if role.Name == "admin" {
		return ErrAdminRoleProtected
	}
	return s.roleRepo.Delete(id)
}

func (s *AdminService) ListPermissions() ([]model.Permission, error) {
	return s.roleRepo.ListAllPermissions()
}
