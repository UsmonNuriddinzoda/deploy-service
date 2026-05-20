package handler

import (
	"deploy-service/session"
	"encoding/json"
	"net/http"
	"time"
)

const sessionCookie = "ds_session"

// LoginHandler — POST /ui/login
func LoginHandler(username, password string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var req struct {
			Username string `json:"username"`
			Password string `json:"password"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			jsonError(w, "invalid JSON", http.StatusBadRequest)
			return
		}

		if req.Username != username || req.Password != password {
			jsonError(w, "invalid username or password", http.StatusUnauthorized)
			return
		}

		token := session.New()
		http.SetCookie(w, &http.Cookie{
			Name:     sessionCookie,
			Value:    token,
			Path:     "/",
			HttpOnly: true,
			SameSite: http.SameSiteLaxMode,
			Expires:  time.Now().Add(12 * time.Hour),
		})
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{"message": "ok"})
	}
}

// LogoutHandler — POST /ui/logout
func LogoutHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		c, err := r.Cookie(sessionCookie)
		if err == nil {
			session.Delete(c.Value)
		}
		http.SetCookie(w, &http.Cookie{
			Name:    sessionCookie,
			Value:   "",
			Path:    "/",
			MaxAge:  -1,
			Expires: time.Unix(0, 0),
		})
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{"message": "logged out"})
	}
}

