package httpapi

import (
	"crypto/subtle"
	"net/http"
	"strings"
	"time"
)

const (
	accessCookieName = "hs_access"
	adminCookieName  = "hs_admin"
	accessTokenTTL   = 12 * time.Hour
	adminTokenTTL    = 4 * time.Hour
)

type accessRequest struct {
	Code string `json:"code"`
}

// handleAccess implements POST /api/auth/access (§11.1).
func (a *API) handleAccess(w http.ResponseWriter, r *http.Request) {
	var req accessRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "Malformed request body.")
		return
	}
	if !constantTimeEqual(req.Code, a.cfg.AppAccessCode) {
		writeError(w, http.StatusUnauthorized, "invalid_access_code", "Incorrect access code.")
		return
	}

	token, err := a.issuer.IssueScope("app", accessTokenTTL)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "Could not issue session.")
		return
	}
	setCookie(w, accessCookieName, token, accessTokenTTL, a.cookieSecure())
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

// handleLogout implements POST /api/auth/logout.
func (a *API) handleLogout(w http.ResponseWriter, r *http.Request) {
	clearCookie(w, accessCookieName, a.cookieSecure())
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

type adminLoginRequest struct {
	Password string `json:"password"`
}

// handleAdminLogin implements POST /api/admin/login (§11.3).
func (a *API) handleAdminLogin(w http.ResponseWriter, r *http.Request) {
	var req adminLoginRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "Malformed request body.")
		return
	}
	if !constantTimeEqual(req.Password, a.cfg.AdminPassword) {
		writeError(w, http.StatusUnauthorized, "invalid_admin_password", "Incorrect admin password.")
		return
	}

	token, err := a.issuer.IssueScope("admin", adminTokenTTL)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "Could not issue session.")
		return
	}
	setCookie(w, adminCookieName, token, adminTokenTTL, a.cookieSecure())
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

// handleAdminLogout implements POST /api/admin/logout.
func (a *API) handleAdminLogout(w http.ResponseWriter, r *http.Request) {
	clearCookie(w, adminCookieName, a.cookieSecure())
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func constantTimeEqual(a, b string) bool {
	return subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1
}

// cookieSecure reports whether the Secure cookie attribute should be set.
// It mirrors AppOrigin's scheme: true in production (HTTPS via Traefik),
// false only when APP_DOMAIN is a local dev address — browsers refuse to
// store a Secure cookie set over plain HTTP at all, which would otherwise
// make the access gate impossible to pass in local dev (§11.1, §11.3).
func (a *API) cookieSecure() bool {
	return strings.HasPrefix(a.cfg.AppOrigin(), "https://")
}

func setCookie(w http.ResponseWriter, name, value string, ttl time.Duration, secure bool) {
	http.SetCookie(w, &http.Cookie{
		Name:     name,
		Value:    value,
		Path:     "/",
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   int(ttl.Seconds()),
	})
}

func clearCookie(w http.ResponseWriter, name string, secure bool) {
	http.SetCookie(w, &http.Cookie{
		Name:     name,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   -1,
	})
}

// requireAppScope gates every /api/* route except /api/auth/* and /healthz (§11.1).
func (a *API) requireAppScope(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie(accessCookieName)
		if err != nil {
			writeError(w, http.StatusUnauthorized, "access_required", "App access required.")
			return
		}
		if _, err := a.issuer.VerifyScope(cookie.Value, "app"); err != nil {
			writeError(w, http.StatusUnauthorized, "access_required", "App access required.")
			return
		}
		next(w, r)
	}
}

// requireAdminScope gates every /api/admin/* route (§11.3).
func (a *API) requireAdminScope(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie(adminCookieName)
		if err != nil {
			writeError(w, http.StatusUnauthorized, "admin_required", "Admin login required.")
			return
		}
		if _, err := a.issuer.VerifyScope(cookie.Value, "admin"); err != nil {
			writeError(w, http.StatusUnauthorized, "admin_required", "Admin login required.")
			return
		}
		next(w, r)
	}
}
