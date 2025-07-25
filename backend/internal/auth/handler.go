package auth

import (
	"errors"
	"gonext/internal/config"
	"gonext/internal/mdw"
	"gonext/internal/model"
	"gonext/internal/repo"
	"gonext/pkg/util/httputil"
	"log/slog"
	"net/http"
	"time"
)

type handler interface {
	indexHandler() http.HandlerFunc
	guestHandler() http.HandlerFunc
	logoutHandler() http.HandlerFunc
	refreshHandler() http.HandlerFunc

	setEmailHandler() http.HandlerFunc
	verifyEmailHandler() http.HandlerFunc
	reqPassHandler() http.HandlerFunc
	setPassHandler() http.HandlerFunc

	emailCodeHandler() http.HandlerFunc
	emailLoginHandler() http.HandlerFunc
	passLoginHandler() http.HandlerFunc
}

type handlerImpl struct {
	service   service
	cfg       *config.Auth
	validator *httputil.Validator
}

func newHandler(service service, config *config.Auth, validator *httputil.Validator) handler {
	return &handlerImpl{
		service:   service,
		cfg:       config,
		validator: validator,
	}
}

func (h *handlerImpl) indexHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		httputil.RespondJSON(w, http.StatusOK, map[string]string{"message": "Auth is running"})
	}
}

func (h *handlerImpl) setAuthCookies(w http.ResponseWriter, accessToken, refreshToken string) time.Time {
	httputil.SetAuthCookie(
		w,
		h.cfg.RefCookieName,
		refreshToken,
		"/api/auth/refresh",
		time.Now().Add(h.cfg.RefTTL),
	)
	expires := time.Now().Add(h.cfg.AccTTL)
	httputil.SetAuthCookie(
		w,
		h.cfg.AccCookieName,
		accessToken,
		"/",
		expires,
	)
	return expires
}

func (h *handlerImpl) authResponse(w http.ResponseWriter, code int, user *model.User, expiresAt time.Time) {
	httputil.RespondJSON(w, code, &authRes{
		User: &userInfo{
			Username:    user.Username,
			DisplayName: user.DisplayName,
			AccountType: string(user.AccountType),
		},
		AccessExp: expiresAt.UnixMilli(),
	})
}

func (h *handlerImpl) guestHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req guestReq
		if !h.validator.DecodeValidate(w, r, &req) {
			return
		}
		result, err := h.service.createGuest(r.Context(), req.Name)
		if err != nil {
			httputil.RespondErr(w, http.StatusInternalServerError, "Something went wrong; please try again.", err)
			return
		}
		expires := h.setAuthCookies(w, result.access, result.refresh)
		h.authResponse(w, http.StatusCreated, result.user, expires)
	}
}

func (h *handlerImpl) logoutHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie("refresh_token")
		if err != nil {
			httputil.RespondErr(w, http.StatusBadRequest, "No refresh token provided", nil)
			return
		}

		if err := h.service.logoutUser(r.Context(), cookie.Value); err != nil {
			slog.Error("failed to logout user", "error", err)
			httputil.RespondErr(w, http.StatusInternalServerError, "Failed to logout", nil)
			return
		}

		httputil.SetAuthCookie(w, "access_token", "", "/", time.Unix(0, 0))
		httputil.SetAuthCookie(w, "refresh_token", "", "/auth/refresh", time.Unix(0, 0))

		httputil.RespondJSON(w, http.StatusOK, map[string]string{"message": "Successfully logged out"})
	}
}

func (h *handlerImpl) refreshHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie("refresh_token")
		if err != nil {
			httputil.RespondErr(w, http.StatusBadRequest, "No refresh token provided", nil)
			return
		}

		result, err := h.service.refreshUser(r.Context(), cookie.Value)
		if err != nil {
			slog.Error("failed to refresh token", "error", err)
			httputil.RespondErr(w, http.StatusUnauthorized, "Invalid refresh token", nil)
			return
		}

		expires := h.setAuthCookies(w, result.access, result.refresh)
		h.authResponse(w, http.StatusOK, result.user, expires)
	}
}

func (h *handlerImpl) setEmailHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user := mdw.GetUser(r.Context())
		if user == nil {
			httputil.RespondErr(w, http.StatusUnauthorized, "User not authenticated", nil)
			return
		}
		var req setEmailReq
		if !h.validator.DecodeValidate(w, r, &req) {
			return
		}
		if err := h.service.setupEmail(r.Context(), user, req.Email); err != nil {
			slog.Error("failed to initiate upgrade", "error", err)
			httputil.RespondErr(w, http.StatusInternalServerError, "Failed to initiate upgrade", nil)
			return
		}
		httputil.RespondJSON(w, http.StatusOK, map[string]string{
			"message": "email may be sent",
		})
	}
}

func (h *handlerImpl) verifyEmailHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user := mdw.GetUser(r.Context())
		if user == nil {
			httputil.RespondErr(w, http.StatusUnauthorized, "User not authenticated", nil)
			return
		}
		var req verifyEmailReq
		if !h.validator.DecodeValidate(w, r, &req) {
			return
		}
		result, err := h.service.verifyEmail(r.Context(), user, req.Code)
		if err != nil {
			if errors.Is(err, ErrInvalidCode) {
				httputil.RespondErr(w, http.StatusBadRequest, "Invalid or expired verification code", nil)
			} else if errors.Is(err, ErrCollision) {
				httputil.RespondErr(w, http.StatusBadRequest, "Email is already in use by another account", nil)
			} else if errors.Is(err, ErrUsernameCollision) {
				httputil.RespondErr(w, http.StatusBadRequest, "Username is already in use by another account", nil)
			} else {
				httputil.RespondErr(w, http.StatusInternalServerError, "Failed to verify email", err)
			}
			return
		}

		expires := h.setAuthCookies(w, result.access, result.refresh)
		h.authResponse(w, http.StatusOK, result.user, expires)
	}
}

func (h *handlerImpl) reqPassHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user := mdw.GetUser(r.Context())
		if user == nil {
			httputil.RespondErr(w, http.StatusUnauthorized, "User not authenticated", nil)
			return
		}

		if err := h.service.reqPassCode(r.Context(), user); err != nil {
			slog.Error("failed to set password", "error", err)
			switch err.Error() {
			case "current password is required":
				httputil.RespondErr(w, http.StatusBadRequest, err.Error(), nil)
			case "invalid current password":
				httputil.RespondErr(w, http.StatusUnauthorized, err.Error(), nil)
			default:
				httputil.RespondErr(w, http.StatusInternalServerError, "Failed to set password", nil)
			}
			return
		}

		httputil.RespondJSON(w, http.StatusOK, map[string]string{
			"message": "Password updated successfully",
		})
	}
}

func (h *handlerImpl) setPassHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user := mdw.GetUser(r.Context())
		if user == nil {
			httputil.RespondErr(w, http.StatusUnauthorized, "User not authenticated", nil)
			return
		}
		var req passReq
		if !h.validator.DecodeValidate(w, r, &req) {
			return
		}
		result, err := h.service.setPassword(r.Context(), user, req.Code, req.Pass)
		if err != nil {
			httputil.RespondErr(w, http.StatusUnauthorized, "Invalid password", nil)
			return
		}
		expires := h.setAuthCookies(w, result.access, result.refresh)
		h.authResponse(w, http.StatusOK, result.user, expires)
	}
}

func (h *handlerImpl) emailCodeHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req setEmailReq
		if !h.validator.DecodeValidate(w, r, &req) {
			return
		}

		if err := h.service.sendEmailCode(r.Context(), req.Email); err != nil && !errors.Is(err, repo.ErrAlreadyExists) {
			slog.Error("failed to send email code", "error", err, "email", req.Email)
		}
		httputil.RespondJSON(w, http.StatusOK, map[string]string{"message": "Email may be sent"})
	}
}

func (h *handlerImpl) emailLoginHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req emailLoginReq
		if !h.validator.DecodeValidate(w, r, &req) {
			slog.Info("email login validation failed", "email", req.Email)
			return
		}

		result, err := h.service.emailLogin(r.Context(), req.Email, req.Code)
		if err != nil {
			slog.Info("email code login failed", "email", req.Email, "error", err)
			httputil.RespondErr(w, http.StatusUnauthorized, "Invalid or expired code", nil)
			return
		}

		expires := h.setAuthCookies(w, result.access, result.refresh)
		h.authResponse(w, http.StatusOK, result.user, expires)
	}
}

func (h *handlerImpl) passLoginHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req passLoginReq
		if !h.validator.DecodeValidate(w, r, &req) {
			return
		}
		result, err := h.service.passwordLogin(r.Context(), req.Email, req.Pass)
		if err != nil {
			httputil.RespondErr(w, http.StatusUnauthorized, "Invalid email or password", nil)
			return
		}
		expires := h.setAuthCookies(w, result.access, result.refresh)
		h.authResponse(w, http.StatusOK, result.user, expires)
	}
}
