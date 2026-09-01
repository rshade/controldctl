package main

import (
	"bytes"
	"context"
	"flag"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"testing"

	"github.com/rshade/ax-go"
)

// updateGolden, when set via `go test -update`, rewrites the golden fixtures in
// testdata/ from the current command output instead of comparing against them.
//
//nolint:gochecknoglobals // test-only golden-file update flag must be package-scoped for the flag package
var updateGolden = flag.Bool("update", false, "update golden files in testdata/")

// goldenVersion is the fixed version injected for golden runs so
// ax.Error.version stays byte-stable across commits (ax.ResolveVersion would
// otherwise fall through to the git revision, which changes every commit).
const goldenVersion = "v0.0.0-golden"

// reNonDeterministic matches the envelope metadata fields ax-go tags as
// non-deterministic: ax.Execute generates fresh random trace_id/span_id and a
// fresh idempotency_key on every run. This mirrors ax-go's own
// internal/testutil.MaskNonDeterministic, which is not importable outside that
// module.
var reNonDeterministic = regexp.MustCompile(`"(trace_id|span_id|idempotency_key)":"[^"]*"`)

func maskNonDeterministic(b []byte) []byte {
	return reNonDeterministic.ReplaceAll(b, []byte(`"${1}":"MASKED"`))
}

// assertGolden compares got against testdata/<name>, or rewrites it under
// -update. Regenerate every fixture with:
//
//	go test ./cmd/controldctl -run TestGolden -update
func assertGolden(t *testing.T, name string, got []byte) {
	t.Helper()
	path := filepath.Join("testdata", name)
	if *updateGolden {
		if err := os.WriteFile(path, got, 0o600); err != nil {
			t.Fatalf("update golden %s: %v", path, err)
		}
		return
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read golden %s: %v (run `go test ./cmd/controldctl -run TestGolden -update`)", path, err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("golden mismatch for %s\nwant: %s\ngot:  %s", path, want, got)
	}
}

// runGolden executes args through ax.Execute against a mock API server with
// every non-deterministic input pinned: an empty environment and a fixed
// version. trace_id/span_id/idempotency_key are still random per run, so
// callers must mask them via maskNonDeterministic before comparing.
func runGolden(t *testing.T, handler http.HandlerFunc, args []string) (stdout, stderr []byte, code int) {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)

	root := newRootCommand(testFactory(t, server))
	root.SetArgs(args)

	var out, errBuf bytes.Buffer
	code = ax.Execute(context.Background(), root,
		ax.WithStdout(&out),
		ax.WithStderr(&errBuf),
		ax.WithEnv(func(string) string { return "" }),
		ax.WithVersion(goldenVersion),
	)
	return out.Bytes(), errBuf.Bytes(), code
}

// jsonResponse replies with a fixed canned ControlD API body. Golden fixtures
// pin output bytes only; request path/method verification stays in the
// per-command tests.
func jsonResponse(body string) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(body))
	}
}

// TestGoldenCommandEnvelopes pins the exact stdout envelope of every
// success-path command against checked-in fixtures, mirroring the golden
// convention of ax-go's examples/integration. The canned API bodies match the
// ones in the per-command tests.
func TestGoldenCommandEnvelopes(t *testing.T) {
	deviceBody := `{"success": true, "body":
		{"PK": "dev1", "ts": 1700000000, "name": "laptop", "user": "u1", "device_id": "abc123",
		 "status": 1, "learn_ip": 0, "desc": "", "resolvers": {"uid": "u1", "doh": "https://x", "dot": "x:853"},
		 "legacy_ipv4": {"resolver": "1.2.3.4", "status": 0},
		 "profile": {"PK": "p1", "updated": 1700000000, "name": "Home"},
		 "icon": "desktop-linux"}}`

	cases := []struct {
		name    string
		args    []string
		handler http.HandlerFunc
	}{
		{
			name: "devices_list",
			args: []string{"devices", "list", "--format=json"},
			handler: jsonResponse(`{
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
			}`),
		},
		{
			name:    "devices_create",
			args:    []string{"devices", "create", "--name=laptop", "--profile-id=p1", "--format=json"},
			handler: jsonResponse(deviceBody),
		},
		{
			name:    "devices_update",
			args:    []string{"devices", "update", "--device-id=dev1", "--name=renamed", "--format=json"},
			handler: jsonResponse(deviceBody),
		},
		{
			name:    "devices_delete",
			args:    []string{"devices", "delete", "--device-id=dev1", "--yes", "--format=json"},
			handler: jsonResponse(`{"success": true, "body": []}`),
		},
		{
			name: "devices_types",
			args: []string{"devices", "types", "--format=json"},
			handler: jsonResponse(`{"success": true, "body": {"types": {
				"os": {"name": "macOS", "icons": {}},
				"browser": {"name": "Chrome", "icons": {}},
				"tv": {"name": "Apple TV", "icons": {}},
				"router": {"name": "ASUS", "icons": {}, "setup_url": ""}
			}}}`),
		},
		{
			name:    "profiles_list",
			args:    []string{"profiles", "list", "--format=json"},
			handler: jsonResponse(`{"success": true, "body": {"profiles": [{"PK": "p1", "updated": 1700000000, "name": "Home"}]}}`),
		},
		{
			name:    "profiles_create",
			args:    []string{"profiles", "create", "--name=Kids", "--format=json"},
			handler: jsonResponse(`{"success": true, "body": {"profiles": [{"PK": "p2", "updated": 1700000000, "name": "Kids"}]}}`),
		},
		{
			name:    "profiles_update",
			args:    []string{"profiles", "update", "--profile-id=p1", "--name=Renamed", "--format=json"},
			handler: jsonResponse(`{"success": true, "body": {"profiles": [{"PK": "p1", "updated": 1700000000, "name": "Renamed"}]}}`),
		},
		{
			name:    "profiles_delete",
			args:    []string{"profiles", "delete", "--profile-id=p1", "--yes", "--format=json"},
			handler: jsonResponse(`{"success": true, "body": []}`),
		},
		{
			name: "profiles_options_list",
			args: []string{"profiles", "options", "list", "--format=json"},
			handler: jsonResponse(`{"success": true, "body": {"options": [
				{"PK": "opt1", "title": "Block Page", "description": "d", "type": "toggle", "default_value": false, "info_url": ""}]}}`),
		},
		{
			name:    "profiles_options_update",
			args:    []string{"profiles", "options", "update", "--profile-id=p1", "--name=block_page", "--format=json"},
			handler: jsonResponse(`{"success": true, "body": {"options": true}}`),
		},
		{
			name: "profiles_filters_list",
			args: []string{"profiles", "filters", "list", "--profile-id=p1", "--format=json"},
			handler: jsonResponse(`{"success": true, "body": {"filters": [{
				"PK": "custom1",
				"name": "Custom",
				"description": "d",
				"sources": [],
				"status": 1,
				"levels": [
					{"title": "Strict", "type": "toggle", "name": "strict", "status": 1,
					 "opt": [{"PK": "opt1", "value": true}]}
				]
			}]}}`),
		},
		{
			name:    "profiles_filters_update",
			args:    []string{"profiles", "filters", "update", "--profile-id=p1", "--filter=ads", "--format=json"},
			handler: jsonResponse(`{"success": true, "body": {"filters": null}}`),
		},
		{
			name: "profiles_folders_list",
			args: []string{"profiles", "folders", "list", "--profile-id=p1", "--format=json"},
			handler: jsonResponse(`{"success": true, "body": {"groups": [
				{"PK": 0, "group": "Default", "action": {"status": 1}, "count": 3}]}}`),
		},
		{
			name: "profiles_folders_create",
			args: []string{"profiles", "folders", "create", "--profile-id=p1", "--name=Ads", "--format=json"},
			handler: jsonResponse(`{"success": true, "body": {"groups": [
				{"PK": 42, "group": "Ads", "action": {"status": 1}, "count": 0}]}}`),
		},
		{
			name: "profiles_folders_update",
			args: []string{
				"profiles", "folders", "update",
				"--profile-id=p1", "--folder-id=f1", "--do=1", "--enabled=false", "--format=json",
			},
			handler: jsonResponse(`{"success": true, "body": {"groups": [
				{"PK": 42, "group": "Ads", "action": {"status": 0, "do": 1}, "count": 0}]}}`),
		},
		{
			name: "profiles_folders_delete",
			args: []string{
				"profiles", "folders", "delete",
				"--profile-id=p1", "--folder-id=f1", "--yes", "--format=json",
			},
			handler: jsonResponse(`{"success": true, "body": []}`),
		},
		{
			name: "profiles_rules_list",
			args: []string{"profiles", "rules", "list", "--profile-id=p1", "--folder-id=f1", "--format=json"},
			handler: jsonResponse(`{"success": true, "body": {"rules": [
				{"PK": "example.com", "order": 0, "group": 0, "action": {"do": 0, "status": 1}}]}}`),
		},
		{
			name: "profiles_rules_create",
			args: []string{
				"profiles", "rules", "create",
				"--profile-id=p1", "--hostnames=example.com,example.org", "--format=json",
			},
			handler: jsonResponse(`{"success": true, "body": {"rules": [{"do": 0, "status": 1, "order": 1}]}}`),
		},
		{
			name: "profiles_rules_update",
			args: []string{
				"profiles", "rules", "update",
				"--profile-id=p1", "--hostnames=example.com", "--do=1", "--format=json",
			},
			handler: jsonResponse(`{"success": true, "body": {"rules": [{"do": 1, "status": 1, "order": 1, "group": 0}]}}`),
		},
		{
			name: "profiles_rules_delete",
			args: []string{
				"profiles", "rules", "delete",
				"--profile-id=p1", "--hostname=example.com", "--yes", "--format=json",
			},
			handler: jsonResponse(`{"success": true, "body": []}`),
		},
		{
			name: "profiles_services_list",
			args: []string{"profiles", "services", "list", "--profile-id=p1", "--format=json"},
			handler: jsonResponse(`{"success": true, "body": {"services": [
				{"PK": "netflix", "name": "Netflix", "category": "streaming", "unlock_location": "",
				 "action": {"do": 0, "status": 1}}]}}`),
		},
		{
			name: "profiles_services_update",
			args: []string{
				"profiles", "services", "update",
				"--profile-id=p1", "--service=netflix", "--do=1", "--format=json",
			},
			handler: jsonResponse(`{"success": true, "body": {"services": [{"do": 1, "status": 1}]}}`),
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			stdout, stderr, code := runGolden(t, tc.handler, tc.args)
			if code != ax.ExitSuccess {
				t.Fatalf("exit code = %d, want %d; stderr=%s", code, ax.ExitSuccess, stderr)
			}
			assertGolden(t, tc.name+".golden.json", maskNonDeterministic(stdout))
		})
	}
}

// TestGoldenErrorEnvelope pins the stderr error envelope for a mapped API
// failure (HTTP 401 → ax.ExitAuth), mirroring ax-go's per-exit-code error
// fixtures.
func TestGoldenErrorEnvelope(t *testing.T) {
	stdout, stderr, code := runGolden(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"success": false, "error": {"message": "invalid token", "code": 401}}`))
	}, []string{"devices", "list", "--format=json"})
	if code != ax.ExitAuth {
		t.Fatalf("exit code = %d, want %d (ExitAuth)", code, ax.ExitAuth)
	}
	if len(stdout) != 0 {
		t.Fatalf("error path leaked stdout: %q", stdout)
	}
	assertGolden(t, "devices_list_auth_error.golden.json", maskNonDeterministic(stderr))
}
