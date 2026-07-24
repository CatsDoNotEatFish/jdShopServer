package model

type ControlEvent struct {
	Type     string `json:"type"`
	IssuedAt string `json:"issued_at"`
}

type User struct {
	ID           int64         `json:"id" db:"id"`
	Username     string        `json:"username" db:"username"`
	Phone        *string       `json:"phone,omitempty" db:"phone"`
	Email        *string       `json:"email" db:"email"`
	PasswordHash string        `json:"-" db:"password_hash"`
	Nickname     *string       `json:"nickname" db:"nickname"`
	AvatarURL    *string       `json:"avatar_url" db:"avatar_url"`
	Status       int           `json:"status" db:"status"`
	LastLoginAt  *string       `json:"last_login_at" db:"last_login_at"`
	CreatedAt    string        `json:"created_at" db:"created_at"`
	UpdatedAt    string        `json:"updated_at" db:"updated_at"`
	Roles        []string      `json:"roles,omitempty" db:"-"`
	Access       AccountAccess `json:"access" db:"-"`
}

type UserWithRoles struct {
	User
	RoleNames         string  `db:"role_names"` // comma-separated from DB query
	LastHeartbeatAt   *string `json:"last_heartbeat_at" db:"last_heartbeat_at"`
	HeartbeatDevice   *string `json:"heartbeat_device_id" db:"heartbeat_device_id"`
	HeartbeatPlatform *string `json:"heartbeat_platform" db:"heartbeat_platform"`
	HeartbeatVersion  *string `json:"heartbeat_app_version" db:"heartbeat_app_version"`
}

type RefreshToken struct {
	ID        int64  `db:"id"`
	UserID    int64  `db:"user_id"`
	TokenHash string `db:"token_hash"`
	ExpiresAt string `db:"expires_at"`
	Revoked   int    `db:"revoked"`
	CreatedAt string `db:"created_at"`
}

type Role struct {
	ID          int64        `json:"id" db:"id"`
	Name        string       `json:"name" db:"name"`
	Description *string      `json:"description" db:"description"`
	CreatedAt   string       `json:"created_at" db:"created_at"`
	Permissions []Permission `json:"permissions,omitempty" db:"-"`
}

type Permission struct {
	ID          int64   `json:"id" db:"id"`
	Code        string  `json:"code" db:"code"`
	Name        string  `json:"name" db:"name"`
	Description *string `json:"description" db:"description"`
}

type Announcement struct {
	ID          int64   `json:"id" db:"id"`
	Title       string  `json:"title" db:"title"`
	Content     string  `json:"content" db:"content"`
	Level       string  `json:"level" db:"level"`
	IsPublished int     `json:"is_published" db:"is_published"`
	PublishedAt *string `json:"published_at,omitempty" db:"published_at"`
	CreatedBy   int64   `json:"created_by" db:"created_by"`
	CreatedAt   string  `json:"created_at" db:"created_at"`
	UpdatedAt   string  `json:"updated_at" db:"updated_at"`
}

type AppVersion struct {
	ID          int64   `json:"id" db:"id"`
	Platform    string  `json:"platform" db:"platform"`
	VersionCode int64   `json:"version_code" db:"version_code"`
	VersionName string  `json:"version_name" db:"version_name"`
	Title       string  `json:"title" db:"title"`
	Description *string `json:"description" db:"description"`
	DownloadURL *string `json:"download_url" db:"download_url"`
	FileSize    *int64  `json:"file_size" db:"file_size"`
	FileHash    *string `json:"file_hash" db:"file_hash"`
	IsForce     int     `json:"is_force" db:"is_force"`
	IsLatest    int     `json:"is_latest" db:"is_latest"`
	CreatedAt   string  `json:"created_at" db:"created_at"`
}

type HeartbeatLog struct {
	ID         int64   `json:"id" db:"id"`
	UserID     int64   `json:"user_id" db:"user_id"`
	DeviceID   string  `json:"device_id" db:"device_id"`
	Platform   *string `json:"platform" db:"platform"`
	AppVersion *string `json:"app_version" db:"app_version"`
	IPAddress  *string `json:"ip_address" db:"ip_address"`
	CreatedAt  string  `json:"created_at" db:"created_at"`
}

type LoginLog struct {
	ID         int64   `json:"id" db:"id"`
	UserID     *int64  `json:"user_id" db:"user_id"`
	Username   string  `json:"username" db:"username"`
	IPAddress  *string `json:"ip_address" db:"ip_address"`
	UserAgent  *string `json:"user_agent" db:"user_agent"`
	Result     string  `json:"result" db:"result"`
	FailReason *string `json:"fail_reason" db:"fail_reason"`
	CreatedAt  string  `json:"created_at" db:"created_at"`
}

type AccessPolicy struct {
	UserID            int64   `json:"user_id" db:"user_id"`
	CompetitorMonitor int     `json:"competitor_monitor" db:"competitor_monitor"`
	MerchantBackend   int     `json:"merchant_backend" db:"merchant_backend"`
	AnalysisCenter    int     `json:"analysis_center" db:"analysis_center"`
	ExpiresAt         *string `json:"expires_at" db:"expires_at"`
	UpdatedAt         string  `json:"updated_at" db:"updated_at"`
}

type ModulePermissions struct {
	CompetitorMonitor bool `json:"competitor_monitor"`
	MerchantBackend   bool `json:"merchant_backend"`
	AnalysisCenter    bool `json:"analysis_center"`
}

type AccountAccess struct {
	Allowed          bool              `json:"allowed"`
	Reason           string            `json:"reason"`
	ExpiresAt        *string           `json:"expires_at"`
	RemainingSeconds int64             `json:"remaining_seconds"`
	LeaseSeconds     int               `json:"lease_seconds"`
	Modules          ModulePermissions `json:"modules"`
}

// Request / Response types

type RegisterRequest struct {
	Phone    string `json:"phone,omitempty"`
	Username string `json:"username,omitempty"` // legacy compatibility for existing clients
	Password string `json:"password"`
	SMSCode  string `json:"sms_code,omitempty"`
	Email    string `json:"email,omitempty"`
	Nickname string `json:"nickname,omitempty"`
}

type LoginRequest struct {
	Phone    string `json:"phone,omitempty"`
	Username string `json:"username,omitempty"` // legacy/admin compatibility
	Password string `json:"password"`
}

type SendSMSRequest struct {
	Phone       string `json:"phone"`
	Purpose     string `json:"purpose"`
	CaptchaID   string `json:"captcha_id"`
	CaptchaCode string `json:"captcha_code"`
}

type LoginResponse struct {
	AccessToken  string   `json:"access_token"`
	RefreshToken string   `json:"refresh_token"`
	ExpiresIn    int      `json:"expires_in"`
	User         UserInfo `json:"user"`
}

type UserInfo struct {
	ID       int64         `json:"id"`
	Username string        `json:"username"`
	Phone    *string       `json:"phone,omitempty"`
	Nickname string        `json:"nickname"`
	Roles    []string      `json:"roles"`
	Status   int           `json:"status"`
	Access   AccountAccess `json:"access"`
}

type RefreshRequest struct {
	RefreshToken string `json:"refresh_token"`
}

type RefreshResponse struct {
	AccessToken  string        `json:"access_token"`
	RefreshToken string        `json:"refresh_token"`
	ExpiresIn    int           `json:"expires_in"`
	Access       AccountAccess `json:"access"`
}

type UpdateProfileRequest struct {
	Nickname  string `json:"nickname,omitempty"`
	Email     string `json:"email,omitempty"`
	AvatarURL string `json:"avatar_url,omitempty"`
}

type ChangePasswordRequest struct {
	OldPassword string `json:"old_password"`
	NewPassword string `json:"new_password"`
	SMSCode     string `json:"sms_code,omitempty"`
}

type HeartbeatRequest struct {
	DeviceID       string `json:"device_id"`
	Platform       string `json:"platform,omitempty"`
	AppVersion     string `json:"app_version,omitempty"`
	AppVersionCode int64  `json:"app_version_code,omitempty"`
}

type HeartbeatResponse struct {
	HasNewVersion     bool          `json:"has_new_version"`
	LatestVersionCode int64         `json:"latest_version_code,omitempty"`
	LatestVersionName string        `json:"latest_version_name,omitempty"`
	LatestDownloadURL *string       `json:"latest_download_url,omitempty"`
	LatestFileHash    *string       `json:"latest_file_hash,omitempty"`
	LatestFileSize    *int64        `json:"latest_file_size,omitempty"`
	IsForceUpdate     bool          `json:"is_force_update"`
	Access            AccountAccess `json:"access"`
}

type CreateAnnouncementRequest struct {
	Title   string `json:"title"`
	Content string `json:"content"`
	Level   string `json:"level,omitempty"`
}

type UpdateAnnouncementRequest struct {
	Title   *string `json:"title,omitempty"`
	Content *string `json:"content,omitempty"`
	Level   *string `json:"level,omitempty"`
}

type CreateVersionRequest struct {
	Platform    string `json:"platform"`
	VersionCode int64  `json:"version_code"`
	VersionName string `json:"version_name"`
	Title       string `json:"title"`
	Description string `json:"description,omitempty"`
	DownloadURL string `json:"download_url,omitempty"`
	FileSize    *int64 `json:"file_size,omitempty"`
	FileHash    string `json:"file_hash,omitempty"`
	IsForce     *bool  `json:"is_force,omitempty"`
}

type UpdateVersionRequest struct {
	Title       *string `json:"title,omitempty"`
	Description *string `json:"description,omitempty"`
	DownloadURL *string `json:"download_url,omitempty"`
	FileSize    *int64  `json:"file_size,omitempty"`
	FileHash    *string `json:"file_hash,omitempty"`
	IsForce     *bool   `json:"is_force,omitempty"`
	IsLatest    *bool   `json:"is_latest,omitempty"`
}

type CreateRoleRequest struct {
	Name          string  `json:"name"`
	Description   string  `json:"description,omitempty"`
	PermissionIDs []int64 `json:"permission_ids"`
}

type UpdateRoleRequest struct {
	Name          *string `json:"name,omitempty"`
	Description   *string `json:"description,omitempty"`
	PermissionIDs []int64 `json:"permission_ids,omitempty"`
}

type AssignRolesRequest struct {
	RoleIDs []int64 `json:"role_ids"`
}

type UpdateUserStatusRequest struct {
	Status int `json:"status"`
}

type UpdateUserAccessRequest struct {
	CompetitorMonitor bool    `json:"competitor_monitor"`
	MerchantBackend   bool    `json:"merchant_backend"`
	AnalysisCenter    bool    `json:"analysis_center"`
	ExpiresAt         *string `json:"expires_at"`
}

type RegistrationDefaults struct {
	UsageDays         int    `json:"usage_days"`
	CompetitorMonitor bool   `json:"competitor_monitor"`
	MerchantBackend   bool   `json:"merchant_backend"`
	AnalysisCenter    bool   `json:"analysis_center"`
	UpdatedAt         string `json:"updated_at,omitempty"`
}

type UpdateRegistrationDefaultsRequest struct {
	UsageDays         int  `json:"usage_days"`
	CompetitorMonitor bool `json:"competitor_monitor"`
	MerchantBackend   bool `json:"merchant_backend"`
	AnalysisCenter    bool `json:"analysis_center"`
}

func (r UpdateRegistrationDefaultsRequest) Validate() string {
	if r.UsageDays < 1 || r.UsageDays > 3650 {
		return "默认赠送天数须为1-3650天"
	}
	return ""
}

type PaginatedResponse struct {
	Items    any   `json:"items"`
	Total    int64 `json:"total"`
	Page     int   `json:"page"`
	PageSize int   `json:"page_size"`
}

type CheckVersionResponse struct {
	HasUpdate bool        `json:"has_update"`
	IsForce   bool        `json:"is_force"`
	Version   *AppVersion `json:"version,omitempty"`
}

func (r RegisterRequest) Validate() string {
	if r.Phone == "" && r.Username == "" {
		return "手机号不能为空"
	}
	if r.Phone != "" && len(r.Phone) != 11 {
		return "手机号格式错误"
	}
	if r.Phone == "" && (len(r.Username) < 3 || len(r.Username) > 32) {
		return "用户名长度须为3-32字符"
	}
	if r.Phone != "" && len(r.SMSCode) != 6 {
		return "短信验证码须为6位"
	}
	if len(r.Password) < 6 || len(r.Password) > 64 {
		return "密码长度须为6-64字符"
	}
	return ""
}

func (r LoginRequest) Validate() string {
	if r.Phone == "" && r.Username == "" {
		return "手机号不能为空"
	}
	if r.Password == "" {
		return "密码不能为空"
	}
	return ""
}

func (r SendSMSRequest) Validate() string {
	if len(r.Phone) != 11 {
		return "手机号格式错误"
	}
	if r.Purpose != "register" && r.Purpose != "password_reset" {
		return "验证码用途错误"
	}
	if r.CaptchaID == "" || r.CaptchaCode == "" {
		return "请先完成图形验证码"
	}
	return ""
}

func (r ChangePasswordRequest) Validate() string {
	if len(r.NewPassword) < 6 || len(r.NewPassword) > 64 {
		return "新密码长度须为6-64字符"
	}
	return ""
}

func (r CreateAnnouncementRequest) Validate() string {
	if r.Title == "" {
		return "公告标题不能为空"
	}
	if r.Content == "" {
		return "公告内容不能为空"
	}
	return ""
}

func (r CreateVersionRequest) Validate() string {
	if r.Platform == "" {
		return "平台不能为空"
	}
	if r.VersionCode <= 0 {
		return "版本码必须大于0"
	}
	if r.VersionName == "" {
		return "版本号不能为空"
	}
	if r.Title == "" {
		return "版本标题不能为空"
	}
	if r.FileSize != nil && *r.FileSize <= 0 {
		return "文件大小必须大于0"
	}
	return ""
}

func (r CreateRoleRequest) Validate() string {
	if r.Name == "" {
		return "角色名不能为空"
	}
	return ""
}
