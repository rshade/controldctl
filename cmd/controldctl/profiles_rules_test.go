package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/rshade/ax-go"
)

func TestProfilesRulesListWritesEnvelope(t *testing.T) {
	var gotPath string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"success": true, "body": {"rules": [` +
			`{"PK": "example.com", "order": 0, "group": 0, "action": {"do": 0, "status": 1}}]}}`))
	}))
	defer server.Close()

	root := newRootCommand(testFactory(t, server))
	root.SetArgs([]string{"profiles", "rules", "list", "--profile-id=p1", "--folder-id=f1", "--format=json"})

	var stdout, stderr bytes.Buffer
	code := ax.Execute(context.Background(), root,
		ax.WithStdout(&stdout),
		ax.WithStderr(&stderr),
		ax.WithEnv(func(string) string { return "" }),
	)
	if code != ax.ExitSuccess {
		t.Fatalf("exit code = %d, want %d; stderr=%s", code, ax.ExitSuccess, stderr.String())
	}
	if gotPath != "/profiles/p1/rules/f1" {
		t.Fatalf("unexpected path: %s", gotPath)
	}

	var envelope struct {
		Data struct {
			Rules []struct {
				PK    string `json:"pk"`
				Order int    `json:"order"`
			} `json:"rules"`
		} `json:"data"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &envelope); err != nil {
		t.Fatalf("unmarshal stdout %q: %v", stdout.String(), err)
	}
	if len(envelope.Data.Rules) != 1 {
		t.Fatalf("unexpected rules: %+v", envelope.Data.Rules)
	}
	if envelope.Data.Rules[0].PK != "example.com" {
		t.Fatalf("expected lowercase \"pk\" field to decode to \"example.com\", got %+v", envelope.Data.Rules[0])
	}
	if bytes.Contains(stdout.Bytes(), []byte(`"PK"`)) {
		t.Fatalf("expected rule PK to be remapped to lowercase \"pk\", found uppercase \"PK\" in output: %s", stdout.String())
	}
}

func TestProfilesRulesListRequiresProfileIDAndFolderID(t *testing.T) {
	called := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		called = true
	}))
	defer server.Close()

	root := newRootCommand(testFactory(t, server))
	root.SetArgs([]string{"profiles", "rules", "list", "--profile-id=p1", "--format=json"})

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

func TestProfilesRulesCreateWritesEnvelope(t *testing.T) {
	var gotMethod, gotPath string
	var body struct {
		Hostnames []string `json:"hostnames"`
		Do        int      `json:"do"`
		Status    int      `json:"status"`
	}
	var decodeErr error
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			decodeErr = err
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"success": true, "body": {"rules": [{"do": 0, "status": 1, "order": 1}]}}`))
	}))
	defer server.Close()

	root := newRootCommand(testFactory(t, server))
	root.SetArgs([]string{
		"profiles", "rules", "create",
		"--profile-id=p1", "--hostnames=example.com,example.org", "--format=json",
	})

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
	if gotPath != "/profiles/p1/rules" {
		t.Fatalf("unexpected path: %s", gotPath)
	}
	if decodeErr != nil {
		t.Fatalf("decode request body: %v", decodeErr)
	}
	if len(body.Hostnames) != 2 || body.Hostnames[0] != "example.com" || body.Hostnames[1] != "example.org" {
		t.Fatalf("unexpected hostnames in request: %+v", body.Hostnames)
	}
	if !bytes.Contains(stdout.Bytes(), []byte(`"order":1`)) {
		t.Fatalf("expected created rule order in output: %s", stdout.String())
	}
}

func TestProfilesRulesCreateRequiresProfileIDAndHostnames(t *testing.T) {
	called := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		called = true
	}))
	defer server.Close()

	root := newRootCommand(testFactory(t, server))
	root.SetArgs([]string{"profiles", "rules", "create", "--profile-id=p1", "--format=json"})

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

func TestProfilesRulesCreateDryRunSkipsRealCall(t *testing.T) {
	// create is not delete, so it does not require --yes; this test instead
	// confirms --dry-run skips the real POST entirely.
	called := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"success": true, "body": {"rules": []}}`))
	}))
	defer server.Close()

	root := newRootCommand(testFactory(t, server))
	root.SetArgs([]string{
		"profiles", "rules", "create",
		"--profile-id=p1", "--hostnames=example.com", "--dry-run", "--format=json",
	})

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
	if !bytes.Contains(stdout.Bytes(), []byte(`"rules":null`)) {
		t.Fatalf("expected rules:null in dry-run output: %s", stdout.String())
	}
	if !bytes.Contains(stdout.Bytes(), []byte(`"dry_run":true`)) {
		t.Fatalf("expected dry_run:true in dry-run output: %s", stdout.String())
	}
}

func TestProfilesRulesUpdateWritesEnvelope(t *testing.T) {
	var gotMethod, gotPath string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"success": true, "body": {"rules": [{"do": 1, "status": 1, "order": 1, "group": 0}]}}`))
	}))
	defer server.Close()

	root := newRootCommand(testFactory(t, server))
	root.SetArgs([]string{
		"profiles", "rules", "update",
		"--profile-id=p1", "--hostnames=example.com", "--do=1", "--format=json",
	})

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
	if gotPath != "/profiles/p1/rules" {
		t.Fatalf("unexpected path: %s", gotPath)
	}
	if !bytes.Contains(stdout.Bytes(), []byte(`"do":1`)) {
		t.Fatalf("expected updated rule do in output: %s", stdout.String())
	}
}

func TestProfilesRulesUpdateRequiresProfileIDAndHostnames(t *testing.T) {
	called := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		called = true
	}))
	defer server.Close()

	root := newRootCommand(testFactory(t, server))
	root.SetArgs([]string{"profiles", "rules", "update", "--profile-id=p1", "--format=json"})

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

func TestProfilesRulesDeleteRequiresYes(t *testing.T) {
	called := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		called = true
	}))
	defer server.Close()

	root := newRootCommand(testFactory(t, server))
	root.SetArgs([]string{
		"profiles", "rules", "delete",
		"--profile-id=p1", "--hostname=example.com", "--format=json",
	})

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
		t.Fatal("unexpected HTTP call without --yes")
	}
}

func TestProfilesRulesDeleteWithYes(t *testing.T) {
	var gotMethod, gotPath string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"success": true, "body": []}`))
	}))
	defer server.Close()

	root := newRootCommand(testFactory(t, server))
	root.SetArgs([]string{
		"profiles", "rules", "delete",
		"--profile-id=p1", "--hostname=example.com", "--yes", "--format=json",
	})

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
	if gotPath != "/profiles/p1/rules/example.com" {
		t.Fatalf("unexpected path: %s", gotPath)
	}
	if !bytes.Contains(stdout.Bytes(), []byte(`"deleted":true`)) {
		t.Fatalf("expected deleted:true in output: %s", stdout.String())
	}
}

func TestProfilesRulesCreateSendsDeferredFlags(t *testing.T) {
	t.Run("--via/--via-v6/--group are sent in the request body", func(t *testing.T) {
		var body map[string]json.RawMessage
		var decodeErr error
		_, stderr, code := executeCommand(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				decodeErr = err
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"success": true, "body": {"rules": [{"do": 3, "status": 1, "order": 1, "group": 7}]}}`))
		}), []string{
			"profiles", "rules", "create",
			"--profile-id=p1", "--hostnames=example.com", "--do=3",
			"--via=10.0.0.1", "--via-v6=fd00::1", "--group=7",
			"--format=json",
		})

		if code != ax.ExitSuccess {
			t.Fatalf("exit code = %d, want %d; stderr=%s", code, ax.ExitSuccess, stderr)
		}
		if decodeErr != nil {
			t.Fatalf("decode request body: %v", decodeErr)
		}
		want := map[string]string{
			"via":    `"10.0.0.1"`,
			"via_v6": `"fd00::1"`,
			"group":  "7",
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
			_, _ = w.Write([]byte(`{"success": true, "body": {"rules": [{"do": 0, "status": 1, "order": 1}]}}`))
		}), []string{
			"profiles", "rules", "create",
			"--profile-id=p1", "--hostnames=example.com", "--format=json",
		})

		if code != ax.ExitSuccess {
			t.Fatalf("exit code = %d, want %d; stderr=%s", code, ax.ExitSuccess, stderr)
		}
		if decodeErr != nil {
			t.Fatalf("decode request body: %v", decodeErr)
		}
		for _, key := range []string{"via", "via_v6", "group"} {
			if _, ok := body[key]; ok {
				t.Fatalf("expected request body to omit %q, got: %+v", key, body)
			}
		}
	})
}

func TestProfilesRulesUpdateSendsDeferredFlags(t *testing.T) {
	t.Run("--via/--group are sent in the request body", func(t *testing.T) {
		var body map[string]json.RawMessage
		var decodeErr error
		_, stderr, code := executeCommand(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				decodeErr = err
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"success": true, "body": {"rules": [{"do": 1, "status": 1, "order": 1, "group": 7}]}}`))
		}), []string{
			"profiles", "rules", "update",
			"--profile-id=p1", "--hostnames=example.com",
			"--do=1", "--enabled=true", "--via=10.0.0.1", "--group=7", "--format=json",
		})

		if code != ax.ExitSuccess {
			t.Fatalf("exit code = %d, want %d; stderr=%s", code, ax.ExitSuccess, stderr)
		}
		if decodeErr != nil {
			t.Fatalf("decode request body: %v", decodeErr)
		}
		if got, ok := body["via"]; !ok || string(got) != `"10.0.0.1"` {
			t.Fatalf("expected request body via=\"10.0.0.1\", got: %+v", body)
		}
		if got, ok := body["group"]; !ok || string(got) != "7" {
			t.Fatalf("expected request body group=7, got: %+v", body)
		}
		if _, ok := body["via_v6"]; ok {
			t.Fatalf("expected request body to omit \"via_v6\", got: %+v", body)
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
			_, _ = w.Write([]byte(`{"success": true, "body": {"rules": [{"do": 1, "status": 1, "order": 1, "group": 0}]}}`))
		}), []string{
			"profiles", "rules", "update",
			"--profile-id=p1", "--hostnames=example.com", "--do=1", "--format=json",
		})

		if code != ax.ExitSuccess {
			t.Fatalf("exit code = %d, want %d; stderr=%s", code, ax.ExitSuccess, stderr)
		}
		if decodeErr != nil {
			t.Fatalf("decode request body: %v", decodeErr)
		}
		for _, key := range []string{"via", "via_v6", "group"} {
			if _, ok := body[key]; ok {
				t.Fatalf("expected request body to omit %q, got: %+v", key, body)
			}
		}
	})
}

func TestProfilesRulesUpdateFlagsCombinations(t *testing.T) {
	cases := []struct {
		name        string
		args        []string
		acceptHTTP  bool
		checkDo     int
		checkStatus int
	}{
		{
			name:        "row 1: no do, no enabled, no new flag → accept",
			args:        []string{"profiles", "rules", "update", "--profile-id=p1", "--hostnames=a.com", "--format=json"},
			acceptHTTP:  true,
			checkDo:     0,
			checkStatus: 1,
		},
		{
			name:        "row 2: yes do, no enabled, no new flag → accept",
			args:        []string{"profiles", "rules", "update", "--profile-id=p1", "--hostnames=a.com", "--do=1", "--format=json"},
			acceptHTTP:  true,
			checkDo:     1,
			checkStatus: 1,
		},
		{
			name:        "row 3: no do, yes enabled, no new flag → accept",
			args:        []string{"profiles", "rules", "update", "--profile-id=p1", "--hostnames=a.com", "--enabled=false", "--format=json"},
			acceptHTTP:  true,
			checkDo:     0,
			checkStatus: 0,
		},
		{
			name:        "row 4: yes do, yes enabled, no new flag → accept",
			args:        []string{"profiles", "rules", "update", "--profile-id=p1", "--hostnames=a.com", "--do=1", "--enabled=true", "--format=json"},
			acceptHTTP:  true,
			checkDo:     1,
			checkStatus: 1,
		},
		{
			name:       "row 5: no do, no enabled, yes new flag (group) → reject",
			args:       []string{"profiles", "rules", "update", "--profile-id=p1", "--hostnames=a.com", "--group=7", "--format=json"},
			acceptHTTP: false,
		},
		{
			name:       "row 6: yes do, no enabled, yes new flag (group) → reject",
			args:       []string{"profiles", "rules", "update", "--profile-id=p1", "--hostnames=a.com", "--do=1", "--group=7", "--format=json"},
			acceptHTTP: false,
		},
		{
			name:       "row 7: no do, yes enabled, yes new flag (group) → reject",
			args:       []string{"profiles", "rules", "update", "--profile-id=p1", "--hostnames=a.com", "--enabled=true", "--group=7", "--format=json"},
			acceptHTTP: false,
		},
		{
			name:        "row 8: yes do, yes enabled, yes new flag (group) → accept",
			args:        []string{"profiles", "rules", "update", "--profile-id=p1", "--hostnames=a.com", "--do=1", "--enabled=false", "--group=7", "--format=json"},
			acceptHTTP:  true,
			checkDo:     1,
			checkStatus: 0,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			called := false
			var body map[string]json.RawMessage
			var decodeErr error
			_, stderr, code := executeCommand(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				called = true
				if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
					decodeErr = err
				}
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(`{"success": true, "body": {"rules": [{"do": 1, "status": 0, "order": 1, "group": 0}]}}`))
			}), tc.args)

			if tc.acceptHTTP {
				if code != ax.ExitSuccess {
					t.Fatalf("exit code = %d, want %d; stderr=%s", code, ax.ExitSuccess, stderr)
				}
				if !called {
					t.Fatal("expected HTTP request to be made")
				}
				if decodeErr != nil {
					t.Fatalf("decode request body: %v", decodeErr)
				}
				if got, ok := body["do"]; !ok || string(got) != fmt.Sprintf("%d", tc.checkDo) {
					t.Fatalf("expected do=%d in request body, got: %+v", tc.checkDo, body)
				}
				if got, ok := body["status"]; !ok || string(got) != fmt.Sprintf("%d", tc.checkStatus) {
					t.Fatalf("expected status=%d in request body, got: %+v", tc.checkStatus, body)
				}
			} else {
				if code != ax.ExitValidation {
					t.Fatalf("exit code = %d, want %d (ExitValidation); stderr=%s", code, ax.ExitValidation, stderr)
				}
				if called {
					t.Fatal("expected no HTTP request for validation error")
				}
			}
		})
	}
}
