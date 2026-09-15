package config

import "testing"

func TestOrigins(t *testing.T) {
	cases := []struct {
		name            string
		appDomain       string
		mediaDomain     string
		wantAppOrigin   string
		wantMediaOrigin string
	}{
		{"production domains use https", "hitsync.example.com", "hitsync-media.example.com", "https://hitsync.example.com", "https://hitsync-media.example.com"},
		{"bare localhost uses http", "localhost", "localhost", "http://localhost", "http://localhost"},
		{"localhost with port uses http", "localhost:5173", "localhost:8090", "http://localhost:5173", "http://localhost:8090"},
		{"127.0.0.1 uses http", "127.0.0.1:5173", "127.0.0.1:8090", "http://127.0.0.1:5173", "http://127.0.0.1:8090"},
		{"a domain merely containing localhost is not local", "notlocalhost.example.com", "media.example.com", "https://notlocalhost.example.com", "https://media.example.com"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			cfg := &Config{AppDomain: c.appDomain, MediaDomain: c.mediaDomain}
			if got := cfg.AppOrigin(); got != c.wantAppOrigin {
				t.Errorf("AppOrigin() = %q, want %q", got, c.wantAppOrigin)
			}
			if got := cfg.MediaOrigin(); got != c.wantMediaOrigin {
				t.Errorf("MediaOrigin() = %q, want %q", got, c.wantMediaOrigin)
			}
		})
	}
}
