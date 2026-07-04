package middleware

import (
	"context"
	"net/http"
	"strings"

	"github.com/golang-jwt/jwt/v5"
)

type contextKey string

const (
	ContextUserID      contextKey = "user_id"
	ContextUsername    contextKey = "username"
	ContextUserRoles   contextKey = "user_roles"
)

func Auth(jwtSecret string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			header := r.Header.Get("Authorization")
			if header == "" || !strings.HasPrefix(header, "Bearer ") {
				writeAuthError(w, 10002, "未提供认证凭证")
				return
			}

			tokenStr := strings.TrimPrefix(header, "Bearer ")
			token, err := jwt.Parse(tokenStr, func(t *jwt.Token) (any, error) {
				if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
					return nil, jwt.ErrSignatureInvalid
				}
				return []byte(jwtSecret), nil
			})
			if err != nil || !token.Valid {
				writeAuthError(w, 10002, "认证凭证无效或已过期")
				return
			}

			claims, ok := token.Claims.(jwt.MapClaims)
			if !ok {
				writeAuthError(w, 10002, "认证凭证解析失败")
				return
			}

			userID, _ := claims["sub"].(float64)
			username, _ := claims["username"].(string)
			rolesRaw, _ := claims["roles"].([]any)

			var roles []string
			for _, r := range rolesRaw {
				if s, ok := r.(string); ok {
					roles = append(roles, s)
				}
			}

			ctx := context.WithValue(r.Context(), ContextUserID, int64(userID))
			ctx = context.WithValue(ctx, ContextUsername, username)
			ctx = context.WithValue(ctx, ContextUserRoles, roles)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func GetUserID(ctx context.Context) int64 {
	if v, ok := ctx.Value(ContextUserID).(int64); ok {
		return v
	}
	return 0
}

func GetUsername(ctx context.Context) string {
	if v, ok := ctx.Value(ContextUsername).(string); ok {
		return v
	}
	return ""
}

func GetUserRoles(ctx context.Context) []string {
	if v, ok := ctx.Value(ContextUserRoles).([]string); ok {
		return v
	}
	return nil
}

func writeAuthError(w http.ResponseWriter, code int, msg string) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusUnauthorized)
	w.Write([]byte(`{"code":` + itoa(code) + `,"message":"` + msg + `","data":null}`))
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	s := ""
	for n > 0 {
		s = string(rune('0'+n%10)) + s
		n /= 10
	}
	return s
}
