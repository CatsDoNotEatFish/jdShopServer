package router

import (
	"database/sql"
	"log"
	"net/http"
	"path/filepath"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"

	"jdShopServer/config"
	ih "jdShopServer/internal/handler"
	imw "jdShopServer/internal/middleware"
	"jdShopServer/internal/repository"
	"jdShopServer/internal/service"
)

func New(cfg *config.Config, db *sql.DB, version string) chi.Router {
	logger := log.Default()

	// Repositories
	userRepo := repository.NewUserRepo(db)
	tokenRepo := repository.NewTokenRepo(db)
	roleRepo := repository.NewRoleRepo(db)
	announcementRepo := repository.NewAnnouncementRepo(db)
	versionRepo := repository.NewVersionRepo(db)
	heartbeatRepo := repository.NewHeartbeatRepo(db)
	loginLogRepo := repository.NewLoginLogRepo(db)
	accessRepo := repository.NewAccessRepo(db)
	registrationDefaultsRepo := repository.NewRegistrationDefaultsRepo(db)
	smsVerificationRepo := repository.NewSMSVerificationRepo(db)

	// Services
	accessSvc := service.NewAccessService(accessRepo, registrationDefaultsRepo, cfg.Auth.SuperAdminUsername)
	smsSvc := service.NewSMSService(smsVerificationRepo, cfg.SMS, cfg.Auth.JWTSecret)
	requestCipher, err := service.NewRequestCipher(filepath.Join(filepath.Dir(cfg.Database.Path), "auth_encryption_private.pem"))
	if err != nil {
		panic(err)
	}
	authSvc := service.NewAuthService(userRepo, tokenRepo, loginLogRepo, accessSvc, smsSvc, cfg.Auth)
	userSvc := service.NewUserService(userRepo, tokenRepo, accessSvc, smsSvc, cfg.Auth)
	controlHub := service.NewControlHub()
	announcementSvc := service.NewAnnouncementService(announcementRepo, controlHub)
	versionSvc := service.NewVersionService(versionRepo)
	heartbeatSvc := service.NewHeartbeatService(heartbeatRepo, versionRepo, userRepo, accessSvc)
	adminSvc := service.NewAdminService(userRepo, roleRepo, accessSvc, tokenRepo, controlHub, cfg.Auth.SuperAdminUsername)

	// Handlers
	healthH := ih.NewHealthHandler(version)
	authH := ih.NewAuthHandler(authSvc, smsSvc, requestCipher, cfg.Auth.RequireEncryptedRequests)
	userH := ih.NewUserHandler(userSvc, requestCipher, cfg.Auth.RequireEncryptedRequests)
	announcementH := ih.NewAnnouncementHandler(announcementSvc)
	versionH := ih.NewVersionHandler(versionSvc)
	heartbeatH := ih.NewHeartbeatHandler(heartbeatSvc)
	controlH := ih.NewControlHandler(controlHub)
	adminH := ih.NewAdminHandler(adminSvc)

	// Middleware
	authMW := imw.Auth(cfg.Auth.JWTSecret, userRepo)
	adminMW := imw.RequireSuperAdmin(cfg.Auth.SuperAdminUsername)
	rateLimiter := imw.NewRateLimiter(60, time.Minute)

	r := chi.NewRouter()

	// Global middleware
	r.Use(middleware.RealIP)
	r.Use(middleware.Recoverer)
	r.Use(timeoutExceptControlStream(30 * time.Second))
	r.Use(imw.Logger(logger))

	// CORS
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   cfg.CORS.AllowedOrigins,
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type"},
		ExposedHeaders:   []string{"Link"},
		AllowCredentials: true,
		MaxAge:           300,
	}))

	// Public routes (no auth)
	r.Group(func(r chi.Router) {
		r.Use(rateLimiter.Handler)

		r.Get("/api/v1/health", healthH.Check)
		r.Get("/api/v1/auth/encryption-key", authH.EncryptionKey)
		r.Post("/api/v1/auth/register", authH.Register)
		r.Post("/api/v1/auth/login", authH.Login)
		r.Get("/api/v1/auth/captcha", authH.Captcha)
		r.Post("/api/v1/auth/sms/send", authH.SendSMS)
		r.Post("/api/v1/auth/refresh", authH.Refresh)
		r.Post("/api/v1/auth/logout", authH.Logout)
		r.Get("/api/v1/announcements", announcementH.PublicList)
		r.Get("/api/v1/version/latest", versionH.CheckLatest)
	})

	// Authenticated routes
	r.Group(func(r chi.Router) {
		r.Use(authMW)

		r.Get("/api/v1/user/profile", userH.GetProfile)
		r.Put("/api/v1/user/profile", userH.UpdateProfile)
		r.Put("/api/v1/user/password", userH.ChangePassword)
		r.Post("/api/v1/heartbeat", heartbeatH.Report)
		r.Get("/api/v1/control/stream", controlH.Stream)
		r.Get("/api/v1/user/announcements", announcementH.UserList)
		r.Post("/api/v1/user/announcements/{id}/read", announcementH.MarkRead)
		r.Post("/api/v1/user/announcements/{id}/acknowledge", announcementH.Acknowledge)
	})

	// Admin routes
	r.Group(func(r chi.Router) {
		r.Use(authMW)
		r.Use(adminMW)

		// Users
		r.Get("/api/v1/admin/users", adminH.ListUsers)
		r.Put("/api/v1/admin/users/{id}/status", adminH.UpdateUserStatus)
		r.Post("/api/v1/admin/users/{id}/roles", adminH.AssignUserRoles)
		r.Put("/api/v1/admin/users/{id}/access", adminH.UpdateUserAccess)
		r.Get("/api/v1/admin/registration-defaults", adminH.GetRegistrationDefaults)
		r.Put("/api/v1/admin/registration-defaults", adminH.UpdateRegistrationDefaults)

		// Roles
		r.Get("/api/v1/admin/roles", adminH.ListRoles)
		r.Post("/api/v1/admin/roles", adminH.CreateRole)
		r.Put("/api/v1/admin/roles/{id}", adminH.UpdateRole)
		r.Delete("/api/v1/admin/roles/{id}", adminH.DeleteRole)

		// Permissions
		r.Get("/api/v1/admin/permissions", adminH.ListPermissions)

		// Announcements
		r.Get("/api/v1/admin/announcements", announcementH.AdminList)
		r.Post("/api/v1/admin/announcements", announcementH.Create)
		r.Put("/api/v1/admin/announcements/{id}", announcementH.Update)
		r.Delete("/api/v1/admin/announcements/{id}", announcementH.Delete)
		r.Post("/api/v1/admin/announcements/{id}/publish", announcementH.Publish)
		r.Post("/api/v1/admin/announcements/{id}/unpublish", announcementH.Unpublish)

		// Versions
		r.Get("/api/v1/admin/versions", versionH.AdminList)
		r.Post("/api/v1/admin/versions", versionH.Create)
		r.Put("/api/v1/admin/versions/{id}", versionH.Update)
		r.Delete("/api/v1/admin/versions/{id}", versionH.Delete)
	})

	return r
}

func timeoutExceptControlStream(timeout time.Duration) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		timed := middleware.Timeout(timeout)(next)
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/api/v1/control/stream" {
				next.ServeHTTP(w, r)
				return
			}
			timed.ServeHTTP(w, r)
		})
	}
}
