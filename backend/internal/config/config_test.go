package config

import "testing"

func TestOrigins(t *testing.T) {
	cases := []struct {
		name          string
		appDomain     string
		wantAppOrigin string
	}{
		{"production domains use https", "hitsync.example.com", "https://hitsync.example.com"},
		{"bare localhost uses http", "localhost", "http://localhost"},
		{"localhost with port uses http", "localhost:5173", "http://localhost:5173"},
		{"127.0.0.1 uses http", "127.0.0.1:5173", "http://127.0.0.1:5173"},
		{"a domain merely containing localhost is not local", "notlocalhost.example.com", "https://notlocalhost.example.com"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			cfg := &Config{AppDomain: c.appDomain}
			if got := cfg.AppOrigin(); got != c.wantAppOrigin {
				t.Errorf("AppOrigin() = %q, want %q", got, c.wantAppOrigin)
			}
		})
	}
}
