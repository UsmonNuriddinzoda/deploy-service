package middleware

import (
	"crypto/subtle"
	"deploy-service/session"
	"net/http"
	"strings"
)

// TokenAuth validates "Authorization: Bearer <token>" header only.
func TokenAuth(token string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
			http.Error(w, `{"error":"missing or malformed Authorization header"}`, http.StatusUnauthorized)
			return
		}
		if subtle.ConstantTimeCompare([]byte(parts[1]), []byte(token)) != 1 {
			http.Error(w, `{"error":"invalid token"}`, http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// SessionOrTokenAuth принимает либо валидную сессию (cookie), либо Bearer-токен.
// Используется для API-роутов: и UI-пользователи, и внешние CI/CD системы могут работать.
func SessionOrTokenAuth(token string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 1. Проверяем сессионную cookie
		if c, err := r.Cookie("ds_session"); err == nil && session.Valid(c.Value) {
			next.ServeHTTP(w, r)
			return
		}
		// 2. Проверяем Bearer-токен
		authHeader := r.Header.Get("Authorization")
		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) == 2 && strings.EqualFold(parts[0], "Bearer") {
			if subtle.ConstantTimeCompare([]byte(parts[1]), []byte(token)) == 1 {
				next.ServeHTTP(w, r)
				return
			}
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":"unauthorized"}`))
	})
}

// SessionAuth защищает UI-маршруты — редиректит на /login если нет валидной сессии.
func SessionAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, err := r.Cookie("ds_session")
		if err != nil || !session.Valid(c.Value) {
			http.Redirect(w, r, "/login", http.StatusFound)
			return
		}
		next.ServeHTTP(w, r)
	})
}
