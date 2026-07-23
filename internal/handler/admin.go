package handler

import (
	"errors"
	"net/http"

	"jdShopServer/internal/model"
	"jdShopServer/internal/service"
)

type AdminHandler struct {
	adminSvc *service.AdminService
}

func NewAdminHandler(adminSvc *service.AdminService) *AdminHandler {
	return &AdminHandler{adminSvc: adminSvc}
}

// Users

func (h *AdminHandler) ListUsers(w http.ResponseWriter, r *http.Request) {
	page := getQueryInt(r, "page", 1)
	pageSize := getQueryInt(r, "page_size", 20)
	keyword := r.URL.Query().Get("keyword")

	var status *int
	if s := r.URL.Query().Get("status"); s != "" {
		v := 0
		if s == "1" {
			v = 1
		}
		status = &v
	}

	users, total, err := h.adminSvc.ListUsers(page, pageSize, keyword, status)
	if err != nil {
		respondError(w, 10500, "服务内部错误")
		return
	}

	if users == nil {
		users = []model.UserWithRoles{}
	}
	respondPaginated(w, users, total, page, pageSize)
}

func (h *AdminHandler) UpdateUserStatus(w http.ResponseWriter, r *http.Request) {
	id, err := parseIDParam(r, "id")
	if err != nil {
		respondError(w, 10001, "ID格式错误")
		return
	}

	var req model.UpdateUserStatusRequest
	if err := decodeBody(r, &req); err != nil {
		respondError(w, 10001, "请求格式错误")
		return
	}
	if req.Status != 0 && req.Status != 1 {
		respondError(w, 10001, "status须为0或1")
		return
	}

	if err := h.adminSvc.UpdateUserStatus(id, req.Status); err != nil {
		if errors.Is(err, service.ErrUserNotFound) {
			respondError(w, 10004, "用户不存在")
			return
		}
		if errors.Is(err, service.ErrSuperAdminProtected) {
			respondError(w, 10003, "主管理员账号受保护，不能修改状态")
			return
		}
		respondError(w, 10500, "服务内部错误")
		return
	}
	respondMessage(w, 0, "状态修改成功")
}

func (h *AdminHandler) AssignUserRoles(w http.ResponseWriter, r *http.Request) {
	id, err := parseIDParam(r, "id")
	if err != nil {
		respondError(w, 10001, "ID格式错误")
		return
	}

	var req model.AssignRolesRequest
	if err := decodeBody(r, &req); err != nil {
		respondError(w, 10001, "请求格式错误")
		return
	}

	if err := h.adminSvc.AssignUserRoles(id, req.RoleIDs); err != nil {
		switch {
		case errors.Is(err, service.ErrUserNotFound):
			respondError(w, 10004, "用户不存在")
		case errors.Is(err, service.ErrSuperAdminProtected):
			respondError(w, 10003, "主管理员账号受保护，不能修改角色")
		case errors.Is(err, service.ErrAdminRoleReserved):
			respondError(w, 10003, "admin 角色为主管理员专用，不能分配给普通账号")
		default:
			respondError(w, 10500, "服务内部错误")
		}
		return
	}
	respondMessage(w, 0, "角色分配成功")
}

func (h *AdminHandler) UpdateUserAccess(w http.ResponseWriter, r *http.Request) {
	id, err := parseIDParam(r, "id")
	if err != nil {
		respondError(w, 10001, "ID格式错误")
		return
	}

	var req model.UpdateUserAccessRequest
	if err := decodeBody(r, &req); err != nil {
		respondError(w, 10001, "请求格式错误")
		return
	}
	access, err := h.adminSvc.UpdateUserAccess(id, req)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrUserNotFound):
			respondError(w, 10004, "用户不存在")
		case errors.Is(err, service.ErrSuperAdminProtected):
			respondError(w, 10003, "主管理员账号受保护，不能修改使用权限")
		case errors.Is(err, service.ErrInvalidExpiry):
			respondError(w, 10001, "到期时间格式错误")
		default:
			respondError(w, 10500, "服务内部错误")
		}
		return
	}
	respondOK(w, access)
}

// Roles

func (h *AdminHandler) ListRoles(w http.ResponseWriter, r *http.Request) {
	roles, err := h.adminSvc.ListRoles()
	if err != nil {
		respondError(w, 10500, "服务内部错误")
		return
	}
	if roles == nil {
		roles = []model.Role{}
	}
	respondOK(w, roles)
}

func (h *AdminHandler) CreateRole(w http.ResponseWriter, r *http.Request) {
	var req model.CreateRoleRequest
	if err := decodeBody(r, &req); err != nil {
		respondError(w, 10001, "请求格式错误")
		return
	}
	if msg := req.Validate(); msg != "" {
		respondError(w, 10001, msg)
		return
	}

	role, err := h.adminSvc.CreateRole(req)
	if err != nil {
		respondError(w, 10500, "服务内部错误")
		return
	}
	respondOK(w, role)
}

func (h *AdminHandler) UpdateRole(w http.ResponseWriter, r *http.Request) {
	id, err := parseIDParam(r, "id")
	if err != nil {
		respondError(w, 10001, "ID格式错误")
		return
	}

	var req model.UpdateRoleRequest
	if err := decodeBody(r, &req); err != nil {
		respondError(w, 10001, "请求格式错误")
		return
	}

	role, err := h.adminSvc.UpdateRole(id, req)
	if err != nil {
		if errors.Is(err, service.ErrRoleNotFound) {
			respondError(w, 10004, "角色不存在")
			return
		}
		if errors.Is(err, service.ErrAdminRoleProtected) {
			respondError(w, 10003, "主管理员角色受保护，不能修改")
			return
		}
		respondError(w, 10500, "服务内部错误")
		return
	}
	respondOK(w, role)
}

func (h *AdminHandler) DeleteRole(w http.ResponseWriter, r *http.Request) {
	id, err := parseIDParam(r, "id")
	if err != nil {
		respondError(w, 10001, "ID格式错误")
		return
	}

	if err := h.adminSvc.DeleteRole(id); err != nil {
		if errors.Is(err, service.ErrRoleNotFound) {
			respondError(w, 10004, "角色不存在")
			return
		}
		if errors.Is(err, service.ErrAdminRoleProtected) {
			respondError(w, 10003, "主管理员角色受保护，不能删除")
			return
		}
		respondError(w, 10500, "服务内部错误")
		return
	}
	respondMessage(w, 0, "删除成功")
}

func (h *AdminHandler) ListPermissions(w http.ResponseWriter, r *http.Request) {
	perms, err := h.adminSvc.ListPermissions()
	if err != nil {
		respondError(w, 10500, "服务内部错误")
		return
	}
	if perms == nil {
		perms = []model.Permission{}
	}
	respondOK(w, perms)
}
