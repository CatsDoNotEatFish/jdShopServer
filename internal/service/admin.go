package service

import (
	"errors"

	"jdShopServer/internal/model"
	"jdShopServer/internal/repository"
)

var (
	ErrRoleNotFound = errors.New("role not found")
)

type AdminService struct {
	userRepo *repository.UserRepo
	roleRepo *repository.RoleRepo
}

func NewAdminService(userRepo *repository.UserRepo, roleRepo *repository.RoleRepo) *AdminService {
	return &AdminService{userRepo: userRepo, roleRepo: roleRepo}
}

func (s *AdminService) ListUsers(page, pageSize int, keyword string, status *int) ([]model.UserWithRoles, int64, error) {
	return s.userRepo.List(page, pageSize, keyword, status)
}

func (s *AdminService) UpdateUserStatus(userID int64, status int) error {
	user, err := s.userRepo.FindByID(userID)
	if err != nil {
		return err
	}
	if user == nil {
		return ErrUserNotFound
	}
	return s.userRepo.UpdateStatus(userID, status)
}

func (s *AdminService) AssignUserRoles(userID int64, roleIDs []int64) error {
	user, err := s.userRepo.FindByID(userID)
	if err != nil {
		return err
	}
	if user == nil {
		return ErrUserNotFound
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
		return errors.New("不能删除admin角色")
	}
	return s.roleRepo.Delete(id)
}

func (s *AdminService) ListPermissions() ([]model.Permission, error) {
	return s.roleRepo.ListAllPermissions()
}
