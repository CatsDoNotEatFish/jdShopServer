# 开发指南

## 环境要求

- Go 1.22+（实际构建需要 1.25，因为 modernc.org/sqlite 依赖）
- 无需 CGO、无需安装 SQLite（纯 Go 驱动 `modernc.org/sqlite`）
- Windows / macOS / Linux 均可开发

## 初始化

```bash
cd G:\jdShopServer
go mod tidy      # 下载依赖
go build .       # 编译
```

## 本地运行

```bash
# 直接运行
go run . serve

# 编译后运行
go build -o jdshop-server.exe . && ./jdshop-server.exe serve

# 仅执行数据库迁移
go run . migrate

# 查看版本
go run . version

# 指定配置
CONFIG_PATH=config.dev.yaml JWT_SECRET=dev-secret go run . serve
```

默认监听 `127.0.0.1:8080`。

## 分层开发规范

### 1. Model 层 (`internal/model/`)

定义数据结构、请求/响应类型、参数校验方法。

```go
// 示例：请求类型 + Validate 方法
type LoginRequest struct {
    Username string `json:"username"`
    Password string `json:"password"`
}

func (r LoginRequest) Validate() string {
    if r.Username == "" { return "用户名不能为空" }
    if r.Password == "" { return "密码不能为空" }
    return ""
}
```

### 2. Repository 层 (`internal/repository/`)

封装 SQL 操作。方法返回模型对象或 `(items, total, error)` 元组。

```go
type UserRepo struct { db *sql.DB }
func NewUserRepo(db *sql.DB) *UserRepo { return &UserRepo{db: db} }

func (r *UserRepo) FindByID(id int64) (*model.User, error) { ... }
func (r *UserRepo) List(page, pageSize int, keyword string, status *int) ([]model.UserWithRoles, int64, error) { ... }
```

规则:
- 查询无结果返回 `nil, nil`（不是 error）
- 列表无结果返回空切片 `[]T{}`（不是 nil）
- 分页方法返回 `(items, total, error)` 三元组

### 3. Service 层 (`internal/service/`)

封装业务逻辑。注入需要的 Repository。

```go
type AuthService struct {
    userRepo     *repository.UserRepo
    tokenRepo    *repository.TokenRepo
    loginLogRepo *repository.LoginLogRepo
    cfg          config.AuthConfig
}

func NewAuthService(...) *AuthService { ... }

func (s *AuthService) Login(req model.LoginRequest, ip, userAgent string) (*model.LoginResponse, error) {
    // 1. 检查登录频率
    // 2. 查找用户
    // 3. 验证密码
    // 4. 生成 Token
    // 5. 记录日志
}
```

Service 层定义项目特有的错误变量:

```go
var (
    ErrInvalidCredentials = errors.New("invalid credentials")
    ErrAccountDisabled    = errors.New("account disabled")
    ErrTooManyAttempts    = errors.New("too many login attempts")
)
```

### 4. Handler 层 (`internal/handler/`)

处理 HTTP 请求/响应。从 context 获取用户信息，调用 Service，统一返回格式。

```go
func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
    var req model.LoginRequest
    if err := decodeBody(r, &req); err != nil { ... }
    if msg := req.Validate(); msg != "" { ... }

    result, err := h.authService.Login(req, clientIP(r), r.Header.Get("User-Agent"))
    if err != nil {
        switch {
        case errors.Is(err, service.ErrInvalidCredentials):
            respondError(w, 10001, "用户名或密码错误")
        default:
            respondError(w, 10500, "服务内部错误")
        }
        return
    }
    respondOK(w, result)
}
```

### 5. Router (`internal/router/router.go`)

集中注册所有路由和中间件，完成依赖注入（手写构造函数，不用 DI 框架）。

路由分组:

```go
// 公开路由（限流）
r.Group(func(r chi.Router) {
    r.Use(rateLimiter.Handler)
    r.Get("/api/v1/health", healthH.Check)
    r.Post("/api/v1/auth/login", authH.Login)
    // ...
})

// 需鉴权路由
r.Group(func(r chi.Router) {
    r.Use(authMW)
    r.Get("/api/v1/user/profile", userH.GetProfile)
    // ...
})

// 管理员路由
r.Group(func(r chi.Router) {
    r.Use(authMW)
    r.Use(adminMW)
    r.Get("/api/v1/admin/users", adminH.ListUsers)
    // ...
})
```

## 添加新功能的标准流程

1. **定义模型**: `internal/model/models.go` 中添加请求/响应类型
2. **编写迁移**: `migrations/002_xxx.sql`（如需新表或新字段）
3. **实现 Repository**: `internal/repository/xxx.go`
4. **实现 Service**: `internal/service/xxx.go`
5. **实现 Handler**: `internal/handler/xxx.go`
6. **注册路由**: `internal/router/router.go`
7. **手动测试**: curl 测试端点
8. **更新文档**: `docs/api-reference.md` 和 `docs/database-schema.md`（如表结构有变化）

## 测试

当前采用手动 curl 测试 + 生产验证策略。后续可加入单元测试。

```bash
# 快速验证
curl http://127.0.0.1:8080/api/v1/health

# 全量测试脚本（参考 docs/api-reference.md 中的 curl 示例）
```

## 编译

```bash
# 开发编译
go build -o jdshop-server.exe .

# 生产编译（去除符号表 + 调试信息，体积从 17MB 降至 ~12MB）
go build -ldflags="-s -w" -o jdshop-server .

# 跨平台编译 Linux
GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o jdshop-server .

# 查看编译体积
ls -lh jdshop-server
```

## 依赖列表

| 包 | 用途 |
|----|------|
| `github.com/go-chi/chi/v5` | HTTP 路由 + 中间件 |
| `github.com/go-chi/cors` | CORS 中间件 |
| `github.com/golang-jwt/jwt/v5` | JWT 签发与验证 |
| `golang.org/x/crypto` | bcrypt 密码哈希 |
| `modernc.org/sqlite` | 纯 Go SQLite 驱动（无 CGO） |
| `gopkg.in/yaml.v3` | YAML 配置解析 |

## 数据库迁移

迁移脚本放在 `migrations/` 目录，按文件名排序执行。

```sql
-- migrations/001_init.sql
CREATE TABLE IF NOT EXISTS users (...);
-- 幂等: 使用 IF NOT EXISTS / INSERT OR IGNORE
```

应用启动时自动执行所有尚未执行的迁移。SQLite 不支持 DDL 事务，所以迁移脚本必须保证幂等。

## 调试

```bash
# 查看服务日志
journalctl -u jdshop -f

# 直接查看 SQLite 数据
sqlite3 data/app.db "SELECT * FROM users;"
sqlite3 data/app.db "SELECT * FROM announcements WHERE is_published=1;"
sqlite3 data/app.db "SELECT * FROM login_logs ORDER BY created_at DESC LIMIT 10;"

# 统计
sqlite3 data/app.db "SELECT result, COUNT(*) FROM login_logs GROUP BY result;"
```
