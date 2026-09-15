package httpapi

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/julianhuber/hitsync/backend/internal/config"
	"github.com/julianhuber/hitsync/backend/internal/tokens"
)

func testAPI() *API {
	cfg := &config.Config{
		AppAccessCode: "letmein",
		AdminPassword: "supersecretadmin",
		JWTSecret:     "a-very-secret-key-that-is-32chars!!",
		MinPlayers:    2, MaxPlayers: 12, DefaultTargetCards: 10, DefaultStartTokens: 2,
		RuleEnableSongGuess: true,
		AppDomain:           "hitsync.example.com",
		LiveKitURL:          "wss://livekit.example.com",
	}
	return &API{
		cfg:           cfg,
		issuer:        tokens.NewIssuer(cfg.JWTSecret),
		accessLimiter: newIPRateLimiter(10, time.Minute),
		adminLimiter:  newIPRateLimiter(10, time.Minute),
	}
}

func doJSON(t *testing.T, handler http.HandlerFunc, method, target string, body any) *httptest.ResponseRecorder {
	t.Helper()
	var buf bytes.Buffer
	if body != nil {
		if err := json.NewEncoder(&buf).Encode(body); err != nil {
			t.Fatalf("encode body: %v", err)
		}
	}
	req := httptest.NewRequest(method, target, &buf)
	req.RemoteAddr = "203.0.113.1:12345"
	rr := httptest.NewRecorder()
	handler(rr, req)
	return rr
}

func TestHandleAccess(t *testing.T) {
	a := testAPI()

	rr := doJSON(t, a.handleAccess, http.MethodPost, "/api/auth/access", accessRequest{Code: "wrong"})
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 for wrong code, got %d", rr.Code)
	}

	rr = doJSON(t, a.handleAccess, http.MethodPost, "/api/auth/access", accessRequest{Code: "letmein"})
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 for correct code, got %d: %s", rr.Code, rr.Body.String())
	}
	cookies := rr.Result().Cookies()
	found := false
	for _, c := range cookies {
		if c.Name == accessCookieName {
			found = true
			if !c.HttpOnly || !c.Secure {
				t.Error("expected access cookie to be HttpOnly and Secure")
			}
		}
	}
	if !found {
		t.Error("expected hs_access cookie to be set")
	}
}

func TestHandleAccessCookieNotSecureOnLocalDomain(t *testing.T) {
	a := testAPI()
	a.cfg.AppDomain = "localhost:5173"

	rr := doJSON(t, a.handleAccess, http.MethodPost, "/api/auth/access", accessRequest{Code: "letmein"})
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	for _, c := range rr.Result().Cookies() {
		if c.Name == accessCookieName && c.Secure {
			t.Error("expected access cookie to NOT be Secure on a local dev domain, or the browser will refuse to store it over plain HTTP")
		}
	}
}

func TestHandleAccessRateLimited(t *testing.T) {
	a := testAPI()
	limited := rateLimited(a.accessLimiter, a.handleAccess)

	var last *httptest.ResponseRecorder
	for i := 0; i < 11; i++ {
		last = doJSON(t, limited, http.MethodPost, "/api/auth/access", accessRequest{Code: "wrong"})
	}
	if last.Code != http.StatusTooManyRequests {
		t.Fatalf("expected 429 after 10 attempts, got %d", last.Code)
	}
	if last.Header().Get("Retry-After") == "" {
		t.Error("expected Retry-After header on rate limit response")
	}
}

func TestHandleAdminLogin(t *testing.T) {
	a := testAPI()

	rr := doJSON(t, a.handleAdminLogin, http.MethodPost, "/api/admin/login", adminLoginRequest{Password: "wrong"})
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 for wrong password, got %d", rr.Code)
	}

	rr = doJSON(t, a.handleAdminLogin, http.MethodPost, "/api/admin/login", adminLoginRequest{Password: "supersecretadmin"})
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 for correct password, got %d", rr.Code)
	}
}

func TestRequireAppScope(t *testing.T) {
	a := testAPI()
	protected := a.requireAppScope(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/api/config", nil)
	rr := httptest.NewRecorder()
	protected(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 without cookie, got %d", rr.Code)
	}

	token, err := a.issuer.IssueScope("app", time.Hour)
	if err != nil {
		t.Fatalf("issue scope: %v", err)
	}
	req = httptest.NewRequest(http.MethodGet, "/api/config", nil)
	req.AddCookie(&http.Cookie{Name: accessCookieName, Value: token})
	rr = httptest.NewRecorder()
	protected(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 with valid cookie, got %d", rr.Code)
	}
}

func TestHandleConfig(t *testing.T) {
	a := testAPI()
	rr := httptest.NewRecorder()
	a.handleConfig(rr, httptest.NewRequest(http.MethodGet, "/api/config", nil))
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	var body map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body["livekitUrl"] != "wss://livekit.example.com" {
		t.Errorf("livekitUrl = %v, want wss://livekit.example.com", body["livekitUrl"])
	}
}
