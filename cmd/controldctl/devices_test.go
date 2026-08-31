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

	"github.com/rshade/controldctl/internal/controld"
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

func TestDevicesCreateWritesEnvelope(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("unexpected method: %s", r.Method)
		}
		if r.URL.Path != "/devices" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		var body struct {
			Name      string `json:"name"`
			ProfileID string `json:"profile_id"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode request body: %v", err)
		}
		if body.Name != "laptop" || body.ProfileID != "p1" {
			t.Fatalf("unexpected request body: %+v", body)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"success": true, "body":
			{"PK": "dev1", "ts": 1700000000, "name": "laptop", "user": "u1", "device_id": "abc123",
			 "status": 1, "learn_ip": 0, "desc": "", "resolvers": {"uid": "u1", "doh": "https://x", "dot": "x:853"},
			 "legacy_ipv4": {"resolver": "1.2.3.4", "status": 0},
			 "profile": {"PK": "p1", "updated": 1700000000, "name": "Home"},
			 "icon": "desktop-linux"}}`))
	}))
	defer server.Close()

	root := newRootCommand(testFactory(t, server))
	root.SetArgs([]string{"devices", "create", "--name=laptop", "--profile-id=p1", "--format=json"})

	var stdout, stderr bytes.Buffer
	code := ax.Execute(context.Background(), root,
		ax.WithStdout(&stdout),
		ax.WithStderr(&stderr),
		ax.WithEnv(func(string) string { return "" }),
	)
	if code != ax.ExitSuccess {
		t.Fatalf("exit code = %d, want %d; stderr=%s", code, ax.ExitSuccess, stderr.String())
	}
	if !bytes.Contains(stdout.Bytes(), []byte(`"name":"laptop"`)) {
		t.Fatalf("expected created device name in output: %s", stdout.String())
	}
}

func TestDevicesCreateRequiresNameAndProfileID(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		t.Fatal("unexpected HTTP call for missing required flags")
	}))
	defer server.Close()

	root := newRootCommand(testFactory(t, server))
	root.SetArgs([]string{"devices", "create", "--name=laptop", "--format=json"})

	var stdout, stderr bytes.Buffer
	code := ax.Execute(context.Background(), root,
		ax.WithStdout(&stdout),
		ax.WithStderr(&stderr),
		ax.WithEnv(func(string) string { return "" }),
	)
	if code != ax.ExitValidation {
		t.Fatalf("exit code = %d, want %d (ExitValidation); stderr=%s", code, ax.ExitValidation, stderr.String())
	}
}

func TestDevicesUpdateWritesEnvelope(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			t.Fatalf("unexpected method: %s", r.Method)
		}
		if r.URL.Path != "/devices/dev1" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"success": true, "body":
			{"PK": "dev1", "ts": 1700000000, "name": "renamed", "user": "u1", "device_id": "abc123",
			 "status": 1, "learn_ip": 0, "desc": "", "resolvers": {"uid": "u1", "doh": "https://x", "dot": "x:853"},
			 "legacy_ipv4": {"resolver": "1.2.3.4", "status": 0},
			 "profile": {"PK": "p1", "updated": 1700000000, "name": "Home"},
			 "icon": "desktop-linux"}}`))
	}))
	defer server.Close()

	root := newRootCommand(testFactory(t, server))
	root.SetArgs([]string{"devices", "update", "--device-id=dev1", "--name=renamed", "--format=json"})

	var stdout, stderr bytes.Buffer
	code := ax.Execute(context.Background(), root,
		ax.WithStdout(&stdout),
		ax.WithStderr(&stderr),
		ax.WithEnv(func(string) string { return "" }),
	)
	if code != ax.ExitSuccess {
		t.Fatalf("exit code = %d, want %d; stderr=%s", code, ax.ExitSuccess, stderr.String())
	}
	if !bytes.Contains(stdout.Bytes(), []byte(`"name":"renamed"`)) {
		t.Fatalf("expected updated device name in output: %s", stdout.String())
	}
}

func TestDevicesUpdateRequiresDeviceID(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		t.Fatal("unexpected HTTP call for missing required flags")
	}))
	defer server.Close()

	root := newRootCommand(testFactory(t, server))
	root.SetArgs([]string{"devices", "update", "--name=renamed", "--format=json"})

	var stdout, stderr bytes.Buffer
	code := ax.Execute(context.Background(), root,
		ax.WithStdout(&stdout),
		ax.WithStderr(&stderr),
		ax.WithEnv(func(string) string { return "" }),
	)
	if code != ax.ExitValidation {
		t.Fatalf("exit code = %d, want %d (ExitValidation); stderr=%s", code, ax.ExitValidation, stderr.String())
	}
}

func TestDevicesDeleteRequiresYes(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		t.Fatal("the real API should never be called without --yes in machine mode")
	}))
	defer server.Close()

	root := newRootCommand(testFactory(t, server))
	root.SetArgs([]string{"devices", "delete", "--device-id=dev1", "--format=json"})

	var stdout, stderr bytes.Buffer
	code := ax.Execute(context.Background(), root,
		ax.WithStdout(&stdout),
		ax.WithStderr(&stderr),
		ax.WithEnv(func(string) string { return "" }),
	)
	if code != ax.ExitValidation {
		t.Fatalf("exit code = %d, want %d (ExitValidation); stderr=%s", code, ax.ExitValidation, stderr.String())
	}
}

func TestDevicesDeleteWithYes(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			t.Fatalf("unexpected method: %s", r.Method)
		}
		if r.URL.Path != "/devices/dev1" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"success": true, "body": []}`))
	}))
	defer server.Close()

	root := newRootCommand(testFactory(t, server))
	root.SetArgs([]string{"devices", "delete", "--device-id=dev1", "--yes", "--format=json"})

	var stdout, stderr bytes.Buffer
	code := ax.Execute(context.Background(), root,
		ax.WithStdout(&stdout),
		ax.WithStderr(&stderr),
		ax.WithEnv(func(string) string { return "" }),
	)
	if code != ax.ExitSuccess {
		t.Fatalf("exit code = %d, want %d; stderr=%s", code, ax.ExitSuccess, stderr.String())
	}
	if !bytes.Contains(stdout.Bytes(), []byte(`"deleted":true`)) {
		t.Fatalf("expected deleted:true in output: %s", stdout.String())
	}
}

func TestDevicesDeleteDryRunSkipsRealCall(t *testing.T) {
	called := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		called = true
	}))
	defer server.Close()

	root := newRootCommand(testFactory(t, server))
	root.SetArgs([]string{"devices", "delete", "--device-id=dev1", "--yes", "--dry-run", "--format=json"})

	var stdout, stderr bytes.Buffer
	code := ax.Execute(context.Background(), root,
		ax.WithStdout(&stdout),
		ax.WithStderr(&stderr),
		ax.WithEnv(func(string) string { return "" }),
	)
	if code != ax.ExitSuccess {
		t.Fatalf("exit code = %d, want %d; stderr=%s", code, ax.ExitSuccess, stderr.String())
	}
	if called {
		t.Fatal("expected the real API call to be skipped under --dry-run")
	}
}

func TestDevicesTypesWritesEnvelope(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/devices/types" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"success": true, "body": {"types": {
			"os": {"name": "macOS", "icons": {}},
			"browser": {"name": "Chrome", "icons": {}},
			"tv": {"name": "Apple TV", "icons": {}},
			"router": {"name": "ASUS", "icons": {}, "setup_url": ""}
		}}}`))
	}))
	defer server.Close()

	root := newRootCommand(testFactory(t, server))
	root.SetArgs([]string{"devices", "types", "--format=json"})

	var stdout, stderr bytes.Buffer
	code := ax.Execute(context.Background(), root,
		ax.WithStdout(&stdout),
		ax.WithStderr(&stderr),
		ax.WithEnv(func(string) string { return "" }),
	)
	if code != ax.ExitSuccess {
		t.Fatalf("exit code = %d, want %d; stderr=%s", code, ax.ExitSuccess, stderr.String())
	}
	if !bytes.Contains(stdout.Bytes(), []byte(`"macOS"`)) {
		t.Fatalf("expected device type name in output: %s", stdout.String())
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
