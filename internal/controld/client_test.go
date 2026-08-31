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
