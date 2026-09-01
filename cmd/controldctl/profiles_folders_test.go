package main

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/rshade/ax-go"
)

func TestProfilesFoldersListWritesEnvelope(t *testing.T) {
	var gotPath string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"success": true, "body": {"groups": [` +
			`{"PK": 0, "group": "Default", "action": {"status": 1}, "count": 3}]}}`))
	}))
	defer server.Close()

	root := newRootCommand(testFactory(t, server))
	root.SetArgs([]string{"profiles", "folders", "list", "--profile-id=p1", "--format=json"})

	var stdout, stderr bytes.Buffer
	code := ax.Execute(context.Background(), root,
		ax.WithStdout(&stdout),
		ax.WithStderr(&stderr),
		ax.WithEnv(func(string) string { return "" }),
	)
	if code != ax.ExitSuccess {
		t.Fatalf("exit code = %d, want %d; stderr=%s", code, ax.ExitSuccess, stderr.String())
	}
	if gotPath != "/profiles/p1/groups" {
		t.Fatalf("unexpected path: %s", gotPath)
	}

	var envelope struct {
		Data struct {
			Groups []struct {
				PK    int    `json:"pk"`
				Group string `json:"group"`
			} `json:"groups"`
		} `json:"data"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &envelope); err != nil {
		t.Fatalf("unmarshal stdout %q: %v", stdout.String(), err)
	}
	if len(envelope.Data.Groups) != 1 || envelope.Data.Groups[0].Group != "Default" {
		t.Fatalf("unexpected groups: %+v", envelope.Data.Groups)
	}
	if bytes.Contains(stdout.Bytes(), []byte(`"PK"`)) {
		t.Fatalf("expected folder PK to be remapped to lowercase \"pk\", found uppercase \"PK\" in output: %s", stdout.String())
	}
}

func TestProfilesFoldersListRequiresProfileID(t *testing.T) {
	called := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		called = true
	}))
	defer server.Close()

	root := newRootCommand(testFactory(t, server))
	root.SetArgs([]string{"profiles", "folders", "list", "--format=json"})

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

func TestProfilesFoldersCreateWritesEnvelope(t *testing.T) {
	var gotMethod, gotPath string
	var body struct {
		Name string `json:"name"`
		Do   int    `json:"do"`
	}
	var decodeErr error
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			decodeErr = err
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"success": true, "body": {"groups": [` +
			`{"PK": 42, "group": "Ads", "action": {"status": 1}, "count": 0}]}}`))
	}))
	defer server.Close()

	root := newRootCommand(testFactory(t, server))
	root.SetArgs([]string{"profiles", "folders", "create", "--profile-id=p1", "--name=Ads", "--format=json"})

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
	if gotPath != "/profiles/p1/groups" {
		t.Fatalf("unexpected path: %s", gotPath)
	}
	if decodeErr != nil {
		t.Fatalf("decode request body: %v", decodeErr)
	}
	if body.Name != "Ads" {
		t.Fatalf("unexpected name in request: %+v", body)
	}
	if !bytes.Contains(stdout.Bytes(), []byte(`"group":"Ads"`)) {
		t.Fatalf("expected created folder name in output: %s", stdout.String())
	}
	if bytes.Contains(stdout.Bytes(), []byte(`"PK"`)) {
		t.Fatalf("expected folder PK to be remapped to lowercase \"pk\", found uppercase \"PK\" in output: %s", stdout.String())
	}
}

func TestProfilesFoldersCreateRequiresProfileIDAndName(t *testing.T) {
	called := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		called = true
	}))
	defer server.Close()

	root := newRootCommand(testFactory(t, server))
	root.SetArgs([]string{"profiles", "folders", "create", "--profile-id=p1", "--format=json"})

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

func TestProfilesFoldersCreateDryRunSkipsRealCall(t *testing.T) {
	called := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"success": true, "body": {"groups": []}}`))
	}))
	defer server.Close()

	root := newRootCommand(testFactory(t, server))
	root.SetArgs([]string{
		"profiles", "folders", "create",
		"--profile-id=p1", "--name=Ads", "--dry-run", "--format=json",
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
	if !bytes.Contains(stdout.Bytes(), []byte(`"groups":null`)) {
		t.Fatalf("expected groups:null in dry-run output: %s", stdout.String())
	}
	if !bytes.Contains(stdout.Bytes(), []byte(`"dry_run":true`)) {
		t.Fatalf("expected dry_run:true in dry-run output: %s", stdout.String())
	}
}

func TestProfilesFoldersUpdateWritesEnvelope(t *testing.T) {
	var gotMethod, gotPath string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"success": true, "body": {"groups": [` +
			`{"PK": 42, "group": "Ads", "action": {"status": 0, "do": 1}, "count": 0}]}}`))
	}))
	defer server.Close()

	root := newRootCommand(testFactory(t, server))
	root.SetArgs([]string{
		"profiles", "folders", "update",
		"--profile-id=p1", "--folder-id=f1", "--do=1", "--enabled=false", "--format=json",
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
	if gotPath != "/profiles/p1/groups/f1" {
		t.Fatalf("unexpected path: %s", gotPath)
	}
	if !bytes.Contains(stdout.Bytes(), []byte(`"do":1`)) {
		t.Fatalf("expected updated folder do in output: %s", stdout.String())
	}
}

func TestProfilesFoldersUpdatePartialFlagsOmitUnsetFields(t *testing.T) {
	t.Run("--do alone omits status from the request body", func(t *testing.T) {
		var body map[string]json.RawMessage
		var decodeErr error
		_, stderr, code := executeCommand(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				decodeErr = err
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"success": true, "body": {"groups": [` +
				`{"PK": 42, "group": "Ads", "action": {"status": 1, "do": 1}, "count": 0}]}}`))
		}), []string{
			"profiles", "folders", "update",
			"--profile-id=p1", "--folder-id=f1", "--do=1", "--format=json",
		})

		if code != ax.ExitSuccess {
			t.Fatalf("exit code = %d, want %d; stderr=%s", code, ax.ExitSuccess, stderr)
		}
		if decodeErr != nil {
			t.Fatalf("decode request body: %v", decodeErr)
		}
		if _, ok := body["status"]; ok {
			t.Fatalf("expected request body to omit \"status\" when only --do is passed, got: %+v", body)
		}
		if _, ok := body["do"]; !ok {
			t.Fatalf("expected request body to contain \"do\", got: %+v", body)
		}
	})

	t.Run("--enabled alone omits do from the request body", func(t *testing.T) {
		var body map[string]json.RawMessage
		var decodeErr error
		_, stderr, code := executeCommand(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				decodeErr = err
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"success": true, "body": {"groups": [` +
				`{"PK": 42, "group": "Ads", "action": {"status": 0}, "count": 0}]}}`))
		}), []string{
			"profiles", "folders", "update",
			"--profile-id=p1", "--folder-id=f1", "--enabled=false", "--format=json",
		})

		if code != ax.ExitSuccess {
			t.Fatalf("exit code = %d, want %d; stderr=%s", code, ax.ExitSuccess, stderr)
		}
		if decodeErr != nil {
			t.Fatalf("decode request body: %v", decodeErr)
		}
		if _, ok := body["do"]; ok {
			t.Fatalf("expected request body to omit \"do\" when only --enabled is passed, got: %+v", body)
		}
		if _, ok := body["status"]; !ok {
			t.Fatalf("expected request body to contain \"status\", got: %+v", body)
		}
	})
}

func TestProfilesFoldersUpdateRequiresProfileIDAndFolderID(t *testing.T) {
	called := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		called = true
	}))
	defer server.Close()

	root := newRootCommand(testFactory(t, server))
	root.SetArgs([]string{"profiles", "folders", "update", "--profile-id=p1", "--format=json"})

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

func TestProfilesFoldersDeleteRequiresYes(t *testing.T) {
	called := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		called = true
	}))
	defer server.Close()

	root := newRootCommand(testFactory(t, server))
	root.SetArgs([]string{
		"profiles", "folders", "delete",
		"--profile-id=p1", "--folder-id=f1", "--format=json",
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
		t.Fatal("the real API should never be called without --yes in machine mode")
	}
}

func TestProfilesFoldersDeleteWithYes(t *testing.T) {
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
		"profiles", "folders", "delete",
		"--profile-id=p1", "--folder-id=f1", "--yes", "--format=json",
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
	if gotPath != "/profiles/p1/groups/f1" {
		t.Fatalf("unexpected path: %s", gotPath)
	}
	if !bytes.Contains(stdout.Bytes(), []byte(`"deleted":true`)) {
		t.Fatalf("expected deleted:true in output: %s", stdout.String())
	}
}

func TestProfilesFoldersCreateSendsVia(t *testing.T) {
	t.Run("--via is sent in the request body", func(t *testing.T) {
		var body map[string]json.RawMessage
		var decodeErr error
		_, stderr, code := executeCommand(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				decodeErr = err
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"success": true, "body": {"groups": [` +
				`{"PK": 42, "group": "Ads", "action": {"status": 1, "do": 3, "via": "10.0.0.1"}, "count": 0}]}}`))
		}), []string{
			"profiles", "folders", "create",
			"--profile-id=p1", "--name=Ads", "--do=3", "--via=10.0.0.1", "--format=json",
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
	})

	t.Run("unset --via stays omitted from the request body", func(t *testing.T) {
		var body map[string]json.RawMessage
		var decodeErr error
		_, stderr, code := executeCommand(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				decodeErr = err
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"success": true, "body": {"groups": [` +
				`{"PK": 42, "group": "Ads", "action": {"status": 1}, "count": 0}]}}`))
		}), []string{
			"profiles", "folders", "create",
			"--profile-id=p1", "--name=Ads", "--format=json",
		})

		if code != ax.ExitSuccess {
			t.Fatalf("exit code = %d, want %d; stderr=%s", code, ax.ExitSuccess, stderr)
		}
		if decodeErr != nil {
			t.Fatalf("decode request body: %v", decodeErr)
		}
		if _, ok := body["via"]; ok {
			t.Fatalf("expected request body to omit \"via\", got: %+v", body)
		}
	})
}

func TestProfilesFoldersUpdateSendsVia(t *testing.T) {
	t.Run("--via is sent in the request body", func(t *testing.T) {
		var body map[string]json.RawMessage
		var decodeErr error
		_, stderr, code := executeCommand(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				decodeErr = err
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"success": true, "body": {"groups": [` +
				`{"PK": 42, "group": "Ads", "action": {"status": 1, "do": 3, "via": "10.0.0.1"}, "count": 0}]}}`))
		}), []string{
			"profiles", "folders", "update",
			"--profile-id=p1", "--folder-id=f1", "--via=10.0.0.1", "--format=json",
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
	})

	t.Run("unset --via stays omitted from the request body", func(t *testing.T) {
		var body map[string]json.RawMessage
		var decodeErr error
		_, stderr, code := executeCommand(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				decodeErr = err
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"success": true, "body": {"groups": [` +
				`{"PK": 42, "group": "Ads", "action": {"status": 1, "do": 1}, "count": 0}]}}`))
		}), []string{
			"profiles", "folders", "update",
			"--profile-id=p1", "--folder-id=f1", "--do=1", "--format=json",
		})

		if code != ax.ExitSuccess {
			t.Fatalf("exit code = %d, want %d; stderr=%s", code, ax.ExitSuccess, stderr)
		}
		if decodeErr != nil {
			t.Fatalf("decode request body: %v", decodeErr)
		}
		if _, ok := body["via"]; ok {
			t.Fatalf("expected request body to omit \"via\", got: %+v", body)
		}
	})
}
