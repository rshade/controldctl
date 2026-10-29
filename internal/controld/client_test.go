package controld

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/rshade/ax-go"
)

func fakeGetenv(values map[string]string) func(string) string {
	return func(key string) string { return values[key] }
}

func TestResolveAPITokenPrecedence(t *testing.T) {
	ctx := context.Background()

	t.Run("flag wins over everything", func(t *testing.T) {
		token, err := resolveAPIToken(ctx, "flag-token", "", fakeGetenv(map[string]string{"CONTROLD_API_TOKEN": "env-token"}))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if token != "flag-token" {
			t.Fatalf("token = %q, want %q", token, "flag-token")
		}
	})

	t.Run("env wins over config file", func(t *testing.T) {
		dir := t.TempDir()
		configPath := filepath.Join(dir, "config.hujson")
		if err := os.WriteFile(configPath, []byte(`{"api_token": "config-token"}`), 0o600); err != nil {
			t.Fatalf("write config: %v", err)
		}

		token, err := resolveAPIToken(ctx, "", configPath, fakeGetenv(map[string]string{"CONTROLD_API_TOKEN": "env-token"}))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if token != "env-token" {
			t.Fatalf("token = %q, want %q", token, "env-token")
		}
	})

	t.Run("falls back to config file", func(t *testing.T) {
		dir := t.TempDir()
		configPath := filepath.Join(dir, "config.hujson")
		if err := os.WriteFile(configPath, []byte(`{"api_token": "config-token"}`), 0o600); err != nil {
			t.Fatalf("write config: %v", err)
		}

		token, err := resolveAPIToken(ctx, "", configPath, fakeGetenv(nil))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if token != "config-token" {
			t.Fatalf("token = %q, want %q", token, "config-token")
		}
	})

	t.Run("errors when nothing is set", func(t *testing.T) {
		_, err := resolveAPIToken(ctx, "", "", fakeGetenv(nil))
		if err == nil {
			t.Fatal("expected an error, got nil")
		}
		if got := ax.ErrorExitCode(err); got != ax.ExitAuth {
			t.Fatalf("ax.ErrorExitCode(err) = %d, want %d (ExitAuth)", got, ax.ExitAuth)
		}
	})
}

func TestResolveOrgIDPrecedence(t *testing.T) {
	ctx := context.Background()

	writeConfig := func(t *testing.T, body string) string {
		t.Helper()
		configPath := filepath.Join(t.TempDir(), "config.hujson")
		if err := os.WriteFile(configPath, []byte(body), 0o600); err != nil {
			t.Fatalf("write config: %v", err)
		}
		return configPath
	}

	cases := []struct {
		name       string
		flag       string
		env        map[string]string
		configBody string
		want       string
	}{
		{
			name: "flag wins over everything",
			flag: "flag-org",
			env:  map[string]string{"CONTROLD_ORG_ID": "env-org"},
			// A config file is written for this case too, so the flag is
			// shown to beat both lower-precedence sources at once.
			configBody: `{"api_token": "t", "org_id": "config-org"}`,
			want:       "flag-org",
		},
		{
			name:       "env wins over config file",
			env:        map[string]string{"CONTROLD_ORG_ID": "env-org"},
			configBody: `{"api_token": "t", "org_id": "config-org"}`,
			want:       "env-org",
		},
		{
			name:       "falls back to config file",
			configBody: `{"api_token": "t", "org_id": "config-org"}`,
			want:       "config-org",
		},
		{
			name: "empty when no source supplies one",
			want: "",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			configPath := ""
			if tc.configBody != "" {
				configPath = writeConfig(t, tc.configBody)
			}
			orgID, err := ResolveOrgID(ctx, tc.flag, configPath, fakeGetenv(tc.env))
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if orgID != tc.want {
				t.Fatalf("org id = %q, want %q", orgID, tc.want)
			}
		})
	}
}
