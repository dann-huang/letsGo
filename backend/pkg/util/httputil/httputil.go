package httputil

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"time"
)

func RespondErr(w http.ResponseWriter, status int, msg string, err error) {
	if err != nil {
		slog.Error(msg, "error", err)
	}
	RespondJSON(w, status, map[string]string{
		"error": msg,
	})
}

func RespondJSON(w http.ResponseWriter, code int, payload any) {
	response, err := json.Marshal(payload)
	if err != nil {
		slog.Error("failed to marshal response", "error", err)
		// If we can't marshal the error, we can't return a proper error response
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	if _, err := w.Write(response); err != nil {
		slog.Error("failed to write response", "error", err)
	}
}

func SetAuthCookie(w http.ResponseWriter, name, value, path string, expires time.Time) {
	cookie := &http.Cookie{
		Name:     name,
		Value:    value,
		Path:     path,
		Expires:  expires,
		HttpOnly: true,
		Secure:   false,
		SameSite: http.SameSiteStrictMode,
	}

	http.SetCookie(w, cookie)
}
