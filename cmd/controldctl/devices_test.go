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

// executeCommand runs a command with a mock HTTP server and captures output.
// The handler runs in a closure so it can capture variables from the test scope.
// Returns stdout, stderr, and exit code.
func executeCommand(t *testing.T, handler http.HandlerFunc, args []string, opts ...ax.ExecuteOption) (stdout, stderr []byte, code int) {
	t.Helper()
	server := httptest.NewServer(handler)
	defer server.Close()

	root := newRootCommand(testFactory(t, server))
	root.SetArgs(args)

	var out, errBuf bytes.Buffer
	execOpts := []ax.ExecuteOption{
		ax.WithStdout(&out),
		ax.WithStderr(&errBuf),
		ax.WithEnv(func(string) string { return "" }),
	}
	execOpts = append(execOpts, opts...)

	code = ax.Execute(context.Background(), root, execOpts...)
	return out.Bytes(), errBuf.Bytes(), code
}

func TestDevicesListWritesEnvelope(t *testing.T) {
	var gotPath string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
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
	if gotPath != "/devices" {
		t.Fatalf("unexpected path: %s", gotPath)
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
	var gotMethod, gotPath string
	var body struct {
		Name      string `json:"name"`
		ProfileID string `json:"profile_id"`
	}
	var decodeErr error
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			decodeErr = err
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
	if gotMethod != http.MethodPost {
		t.Fatalf("unexpected method: %s", gotMethod)
	}
	if gotPath != "/devices" {
		t.Fatalf("unexpected path: %s", gotPath)
	}
	if decodeErr != nil {
		t.Fatalf("decode request body: %v", decodeErr)
	}
	if body.Name != "laptop" || body.ProfileID != "p1" {
		t.Fatalf("unexpected request body: %+v", body)
	}
	if !bytes.Contains(stdout.Bytes(), []byte(`"name":"laptop"`)) {
		t.Fatalf("expected created device name in output: %s", stdout.String())
	}
}

func TestDevicesCreateRequiresNameAndProfileID(t *testing.T) {
	called := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		called = true
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
	if called {
		t.Fatal("unexpected HTTP call for missing required flags")
	}
}

func TestDevicesUpdateWritesEnvelope(t *testing.T) {
	var gotMethod, gotPath string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
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
	if gotMethod != http.MethodPut {
		t.Fatalf("unexpected method: %s", gotMethod)
	}
	if gotPath != "/devices/dev1" {
		t.Fatalf("unexpected path: %s", gotPath)
	}
	if !bytes.Contains(stdout.Bytes(), []byte(`"name":"renamed"`)) {
		t.Fatalf("expected updated device name in output: %s", stdout.String())
	}
}

func TestDevicesUpdateRequiresDeviceID(t *testing.T) {
	called := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		called = true
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
	if called {
		t.Fatal("unexpected HTTP call for missing required flags")
	}
}

func TestDevicesDeleteRequiresYes(t *testing.T) {
	called := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		called = true
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
	if called {
		t.Fatal("the real API should never be called without --yes in machine mode")
	}
}

func TestDevicesDeleteWithYes(t *testing.T) {
	var gotMethod, gotPath string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
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
	if gotMethod != http.MethodDelete {
		t.Fatalf("unexpected method: %s", gotMethod)
	}
	if gotPath != "/devices/dev1" {
		t.Fatalf("unexpected path: %s", gotPath)
	}
	if !bytes.Contains(stdout.Bytes(), []byte(`"deleted":true`)) {
		t.Fatalf("expected deleted:true in output: %s", stdout.String())
	}
}

func TestDevicesTypesWritesEnvelope(t *testing.T) {
	var gotPath string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
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
	if gotPath != "/devices/types" {
		t.Fatalf("unexpected path: %s", gotPath)
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

func TestDevicesCreateSendsDeferredFlags(t *testing.T) {
	deviceResponse := `{"success": true, "body":
		{"PK": "dev1", "ts": 1700000000, "name": "laptop", "user": "u1", "device_id": "abc123",
		 "status": 1, "learn_ip": 0, "desc": "", "resolvers": {"uid": "u1", "doh": "https://x", "dot": "x:853"},
		 "legacy_ipv4": {"resolver": "1.2.3.4", "status": 0},
		 "profile": {"PK": "p1", "updated": 1700000000, "name": "Home"},
		 "icon": "desktop-linux"}}`

	t.Run("set flags are sent in the request body", func(t *testing.T) {
		var body map[string]json.RawMessage
		var decodeErr error
		_, stderr, code := executeCommand(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				decodeErr = err
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(deviceResponse))
		}), []string{
			"devices", "create",
			"--name=laptop", "--profile-id=p1",
			"--legacy-ipv4-status", "--ddns-status", "--ddns-subdomain=myhost",
			"--ddns-ext-status", "--ddns-ext-host=ddns.example.com",
			"--remap-device-id=dev9", "--remap-client-id=cli9",
			"--format=json",
		})

		if code != ax.ExitSuccess {
			t.Fatalf("exit code = %d, want %d; stderr=%s", code, ax.ExitSuccess, stderr)
		}
		if decodeErr != nil {
			t.Fatalf("decode request body: %v", decodeErr)
		}
		want := map[string]string{
			"legacy_ipv4_status": "1",
			"ddns_status":        "1",
			"ddns_subdomain":     `"myhost"`,
			"ddns_ext_status":    "1",
			"ddns_ext_host":      `"ddns.example.com"`,
			"remap_device_id":    `"dev9"`,
			"remap_client_id":    `"cli9"`,
		}
		for key, wantRaw := range want {
			got, ok := body[key]
			if !ok {
				t.Fatalf("expected request body to contain %q, got: %+v", key, body)
			}
			if string(got) != wantRaw {
				t.Fatalf("request body %q = %s, want %s", key, got, wantRaw)
			}
		}
	})

	t.Run("unset flags stay omitted from the request body", func(t *testing.T) {
		var body map[string]json.RawMessage
		var decodeErr error
		_, stderr, code := executeCommand(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				decodeErr = err
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(deviceResponse))
		}), []string{"devices", "create", "--name=laptop", "--profile-id=p1", "--format=json"})

		if code != ax.ExitSuccess {
			t.Fatalf("exit code = %d, want %d; stderr=%s", code, ax.ExitSuccess, stderr)
		}
		if decodeErr != nil {
			t.Fatalf("decode request body: %v", decodeErr)
		}
		for _, key := range []string{
			"legacy_ipv4_status", "ddns_status", "ddns_subdomain",
			"ddns_ext_status", "ddns_ext_host", "remap_device_id", "remap_client_id",
		} {
			if _, ok := body[key]; ok {
				t.Fatalf("expected request body to omit %q, got: %+v", key, body)
			}
		}
	})
}

func TestDevicesUpdateSendsDeferredFlags(t *testing.T) {
	deviceResponse := `{"success": true, "body":
		{"PK": "dev1", "ts": 1700000000, "name": "laptop", "user": "u1", "device_id": "abc123",
		 "status": 1, "learn_ip": 0, "desc": "", "resolvers": {"uid": "u1", "doh": "https://x", "dot": "x:853"},
		 "legacy_ipv4": {"resolver": "1.2.3.4", "status": 0},
		 "profile": {"PK": "p1", "updated": 1700000000, "name": "Home"},
		 "icon": "desktop-linux"}}`

	t.Run("set flags are sent in the request body", func(t *testing.T) {
		var body map[string]json.RawMessage
		var decodeErr error
		_, stderr, code := executeCommand(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				decodeErr = err
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(deviceResponse))
		}), []string{
			"devices", "update",
			"--device-id=dev1", "--legacy-ipv4-status", "--ddns-status",
			"--ddns-subdomain=myhost", "--ddns-ext-status", "--ddns-ext-host=ddns.example.com",
			"--format=json",
		})

		if code != ax.ExitSuccess {
			t.Fatalf("exit code = %d, want %d; stderr=%s", code, ax.ExitSuccess, stderr)
		}
		if decodeErr != nil {
			t.Fatalf("decode request body: %v", decodeErr)
		}
		want := map[string]string{
			"legacy_ipv4_status": "1",
			"ddns_status":        "1",
			"ddns_subdomain":     `"myhost"`,
			"ddns_ext_status":    "1",
			"ddns_ext_host":      `"ddns.example.com"`,
		}
		for key, wantRaw := range want {
			got, ok := body[key]
			if !ok {
				t.Fatalf("expected request body to contain %q, got: %+v", key, body)
			}
			if string(got) != wantRaw {
				t.Fatalf("request body %q = %s, want %s", key, got, wantRaw)
			}
		}
	})

	t.Run("explicit false is sent while unset flags stay omitted", func(t *testing.T) {
		var body map[string]json.RawMessage
		var decodeErr error
		_, stderr, code := executeCommand(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				decodeErr = err
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(deviceResponse))
		}), []string{"devices", "update", "--device-id=dev1", "--ddns-status=false", "--format=json"})

		if code != ax.ExitSuccess {
			t.Fatalf("exit code = %d, want %d; stderr=%s", code, ax.ExitSuccess, stderr)
		}
		if decodeErr != nil {
			t.Fatalf("decode request body: %v", decodeErr)
		}
		if got, ok := body["ddns_status"]; !ok || string(got) != "0" {
			t.Fatalf("expected request body ddns_status=0, got: %+v", body)
		}
		for _, key := range []string{"legacy_ipv4_status", "ddns_subdomain", "ddns_ext_status", "ddns_ext_host"} {
			if _, ok := body[key]; ok {
				t.Fatalf("expected request body to omit %q, got: %+v", key, body)
			}
		}
	})
}
