package main

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/spf13/cobra"

	"github.com/rshade/ax-go"

	"github.com/rshade/controld-go-mcp/internal/controld"
)

func testFactory(t *testing.T, server *httptest.Server) clientFactory {
	t.Helper()
	return func(cmd *cobra.Command) (*controld.API, error) {
		return controld.NewClient(cmd.Context(), "test-token", "", server.URL, func(string) string { return "" })
	}
}

func TestDevicesListWritesEnvelope(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/devices" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"success": true,
			"body": {
				"devices": [
					{"PK": "dev1", "ts": 1700000000, "name": "laptop", "user": "u1", "device_id": "abc123",
					 "status": 1, "learn_ip": 0, "desc": "", "resolvers": {"uid": "u1", "doh": "https://x", "dot": "x:853"},
					 "legacy_ipv4": {"resolver": "1.2.3.4", "status": 0},
					 "profile": {"PK": "p1", "updated": 1700000000, "name": "Home"},
					 "icon": "desktop-linux"}
				]
			}
		}`))
	}))
	defer server.Close()

	root := newRootCommand(testFactory(t, server))
	root.SetArgs([]string{"devices", "list", "--format=json"})

	var stdout, stderr bytes.Buffer
	code := ax.Execute(context.Background(), root,
		ax.WithStdout(&stdout),
		ax.WithStderr(&stderr),
		ax.WithEnv(func(string) string { return "" }),
	)
	if code != ax.ExitSuccess {
		t.Fatalf("exit code = %d, want %d; stderr=%s", code, ax.ExitSuccess, stderr.String())
	}

	var envelope struct {
		Data struct {
			Devices []struct {
				PK   string `json:"pk"`
				Name string `json:"name"`
			} `json:"devices"`
		} `json:"data"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &envelope); err != nil {
		t.Fatalf("unmarshal stdout %q: %v", stdout.String(), err)
	}
	if len(envelope.Data.Devices) != 1 || envelope.Data.Devices[0].Name != "laptop" {
		t.Fatalf("unexpected devices: %+v", envelope.Data.Devices)
	}
}

func TestDevicesListMapsAuthError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"success": false, "error": {"message": "invalid token", "code": 401}}`))
	}))
	defer server.Close()

	root := newRootCommand(testFactory(t, server))
	root.SetArgs([]string{"devices", "list", "--format=json"})

	var stdout, stderr bytes.Buffer
	code := ax.Execute(context.Background(), root,
		ax.WithStdout(&stdout),
		ax.WithStderr(&stderr),
		ax.WithEnv(func(string) string { return "" }),
	)
	if code == ax.ExitSuccess {
		t.Fatalf("expected a failure exit code; stdout=%s", stdout.String())
	}
	if code != ax.ExitAuth {
		t.Fatalf("exit code = %d, want %d (ExitAuth); stderr=%s", code, ax.ExitAuth, stderr.String())
	}
}
