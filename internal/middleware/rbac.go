package middleware

import (
	"net/http"
	"slices"
)

func RequireRole(role string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			roles := GetUserRoles(r.Context())
			if !slices.Contains(roles, role) {
				writeForbidden(w)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func RequirePermission(perm string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			roles := GetUserRoles(r.Context())
			// admin role always has all permissions
			if slices.Contains(roles, "admin") {
				next.ServeHTTP(w, r)
				return
			}
			// For non-admin, check specific permission via DB
			// The handler should verify specific permissions
			writeForbidden(w)
		})
	}
}

func writeForbidden(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusForbidden)
	w.Write([]byte(`{"code":10003,"message":"无操作权限","data":null}`))
}
