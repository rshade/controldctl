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

func TestProfilesListWritesEnvelope(t *testing.T) {
	var gotPath string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"success": true, "body": {"profiles": [{"PK": "p1", "updated": 1700000000, "name": "Home"}]}}`))
	}))
	defer server.Close()

	root := newRootCommand(testFactory(t, server))
	root.SetArgs([]string{"profiles", "list", "--format=json"})

	var stdout, stderr bytes.Buffer
	code := ax.Execute(context.Background(), root,
		ax.WithStdout(&stdout),
		ax.WithStderr(&stderr),
		ax.WithEnv(func(string) string { return "" }),
	)
	if code != ax.ExitSuccess {
		t.Fatalf("exit code = %d, want %d; stderr=%s", code, ax.ExitSuccess, stderr.String())
	}
	if gotPath != "/profiles" {
		t.Fatalf("unexpected path: %s", gotPath)
	}

	var envelope struct {
		Data struct {
			Profiles []struct {
				PK   string `json:"pk"`
				Name string `json:"name"`
			} `json:"profiles"`
		} `json:"data"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &envelope); err != nil {
		t.Fatalf("unmarshal stdout %q: %v", stdout.String(), err)
	}
	if len(envelope.Data.Profiles) != 1 || envelope.Data.Profiles[0].Name != "Home" {
		t.Fatalf("unexpected profiles: %+v", envelope.Data.Profiles)
	}
	if envelope.Data.Profiles[0].PK != "p1" {
		t.Fatalf("expected lowercase \"pk\" field to decode to \"p1\", got %+v", envelope.Data.Profiles[0])
	}
	if bytes.Contains(stdout.Bytes(), []byte(`"PK"`)) {
		t.Fatalf("expected profile PK to be remapped to lowercase \"pk\", found uppercase \"PK\" in output: %s", stdout.String())
	}
}

func TestProfilesCreateWritesEnvelope(t *testing.T) {
	var gotMethod, gotPath string
	var body struct {
		Name string `json:"name"`
	}
	var decodeErr error
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			decodeErr = err
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"success": true, "body": {"profiles": [{"PK": "p2", "updated": 1700000000, "name": "Kids"}]}}`))
	}))
	defer server.Close()

	root := newRootCommand(testFactory(t, server))
	root.SetArgs([]string{"profiles", "create", "--name=Kids", "--format=json"})

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
	if gotPath != "/profiles" {
		t.Fatalf("unexpected path: %s", gotPath)
	}
	if decodeErr != nil {
		t.Fatalf("decode request body: %v", decodeErr)
	}
	if body.Name != "Kids" {
		t.Fatalf("unexpected request body: %+v", body)
	}
	if !bytes.Contains(stdout.Bytes(), []byte(`"name":"Kids"`)) {
		t.Fatalf("expected created profile name in output: %s", stdout.String())
	}
}

func TestProfilesCreateRequiresName(t *testing.T) {
	called := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		called = true
	}))
	defer server.Close()

	root := newRootCommand(testFactory(t, server))
	root.SetArgs([]string{"profiles", "create", "--format=json"})

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

func TestProfilesUpdateWritesEnvelope(t *testing.T) {
	var gotMethod, gotPath string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"success": true, "body": {"profiles": [{"PK": "p1", "updated": 1700000000, "name": "Renamed"}]}}`))
	}))
	defer server.Close()

	root := newRootCommand(testFactory(t, server))
	root.SetArgs([]string{"profiles", "update", "--profile-id=p1", "--name=Renamed", "--format=json"})

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
	if gotPath != "/profiles/p1" {
		t.Fatalf("unexpected path: %s", gotPath)
	}
	if !bytes.Contains(stdout.Bytes(), []byte(`"name":"Renamed"`)) {
		t.Fatalf("expected updated profile name in output: %s", stdout.String())
	}
}

func TestProfilesUpdateRequiresProfileID(t *testing.T) {
	called := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		called = true
	}))
	defer server.Close()

	root := newRootCommand(testFactory(t, server))
	root.SetArgs([]string{"profiles", "update", "--name=Renamed", "--format=json"})

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

func TestProfilesDeleteRequiresYes(t *testing.T) {
	called := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		called = true
	}))
	defer server.Close()

	root := newRootCommand(testFactory(t, server))
	root.SetArgs([]string{"profiles", "delete", "--profile-id=p1", "--format=json"})

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

func TestProfilesDeleteWithYes(t *testing.T) {
	var gotMethod, gotPath string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"success": true, "body": []}`))
	}))
	defer server.Close()

	root := newRootCommand(testFactory(t, server))
	root.SetArgs([]string{"profiles", "delete", "--profile-id=p1", "--yes", "--format=json"})

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
	if gotPath != "/profiles/p1" {
		t.Fatalf("unexpected path: %s", gotPath)
	}
	if !bytes.Contains(stdout.Bytes(), []byte(`"deleted":true`)) {
		t.Fatalf("expected deleted:true in output: %s", stdout.String())
	}
}

func TestProfilesDeleteDryRunSkipsRealCall(t *testing.T) {
	called := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		called = true
	}))
	defer server.Close()

	root := newRootCommand(testFactory(t, server))
	root.SetArgs([]string{"profiles", "delete", "--profile-id=p1", "--yes", "--dry-run", "--format=json"})

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
	if !bytes.Contains(stdout.Bytes(), []byte(`"deleted":false`)) {
		t.Fatalf("expected deleted:false in dry-run output: %s", stdout.String())
	}
	if !bytes.Contains(stdout.Bytes(), []byte(`"dry_run":true`)) {
		t.Fatalf("expected dry_run:true in dry-run output: %s", stdout.String())
	}
}

func TestProfilesCreateDryRunSkipsRealCall(t *testing.T) {
	called := false
	stdout, stderr, code := executeCommand(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		called = true
	}), []string{"profiles", "create", "--name=Home", "--dry-run", "--format=json"})

	if code != ax.ExitSuccess {
		t.Fatalf("exit code = %d, want %d; stderr=%s", code, ax.ExitSuccess, stderr)
	}
	if called {
		t.Fatal("expected the real API call to be skipped under --dry-run")
	}
	if !bytes.Contains(stdout, []byte(`"profiles":null`)) {
		t.Fatalf("expected profiles:null in dry-run output: %s", stdout)
	}
	if !bytes.Contains(stdout, []byte(`"dry_run":true`)) {
		t.Fatalf("expected dry_run:true in dry-run output: %s", stdout)
	}
}

func TestProfilesOptionsListWritesEnvelope(t *testing.T) {
	var gotPath string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"success": true, "body": {"options": [{"PK": "opt1", "title": "Block Page", "description": "d", "type": "toggle", "default_value": false, "info_url": ""}]}}`))
	}))
	defer server.Close()

	root := newRootCommand(testFactory(t, server))
	root.SetArgs([]string{"profiles", "options", "list", "--format=json"})

	var stdout, stderr bytes.Buffer
	code := ax.Execute(context.Background(), root,
		ax.WithStdout(&stdout),
		ax.WithStderr(&stderr),
		ax.WithEnv(func(string) string { return "" }),
	)
	if code != ax.ExitSuccess {
		t.Fatalf("exit code = %d, want %d; stderr=%s", code, ax.ExitSuccess, stderr.String())
	}
	if gotPath != "/profiles/options" {
		t.Fatalf("unexpected path: %s", gotPath)
	}
	if !bytes.Contains(stdout.Bytes(), []byte("Block Page")) {
		t.Fatalf("expected option title in output: %s", stdout.String())
	}
}

func TestProfilesOptionsUpdateWritesEnvelope(t *testing.T) {
	var gotMethod, gotPath string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"success": true, "body": {"options": true}}`))
	}))
	defer server.Close()

	root := newRootCommand(testFactory(t, server))
	root.SetArgs([]string{"profiles", "options", "update", "--profile-id=p1", "--name=block_page", "--format=json"})

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
	if gotPath != "/profiles/p1/options/block_page" {
		t.Fatalf("unexpected path: %s", gotPath)
	}
	if !bytes.Contains(stdout.Bytes(), []byte(`"updated":true`)) {
		t.Fatalf("expected updated:true in output: %s", stdout.String())
	}
}

func TestProfilesOptionsUpdateRequiresProfileIDAndName(t *testing.T) {
	called := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		called = true
	}))
	defer server.Close()

	root := newRootCommand(testFactory(t, server))
	root.SetArgs([]string{"profiles", "options", "update", "--profile-id=p1", "--format=json"})

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

func TestProfilesUpdateSendsDeferredFlags(t *testing.T) {
	profileResponse := `{"success": true, "body": {"profiles": [{"PK": "p1", "updated": 1700000000, "name": "Home"}]}}`

	t.Run("set flags are sent in the request body", func(t *testing.T) {
		var body map[string]json.RawMessage
		var decodeErr error
		_, stderr, code := executeCommand(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				decodeErr = err
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(profileResponse))
		}), []string{
			"profiles", "update",
			"--profile-id=p1", "--disable-ttl=1", "--lock-status",
			"--lock-message=Ask an adult", "--password=s3cret",
			"--format=json",
		})

		if code != ax.ExitSuccess {
			t.Fatalf("exit code = %d, want %d; stderr=%s", code, ax.ExitSuccess, stderr)
		}
		if decodeErr != nil {
			t.Fatalf("decode request body: %v", decodeErr)
		}
		want := map[string]string{
			"disable_ttl":  "1",
			"lock_status":  "1",
			"lock_message": `"Ask an adult"`,
			"password":     `"s3cret"`,
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
			_, _ = w.Write([]byte(profileResponse))
		}), []string{"profiles", "update", "--profile-id=p1", "--name=Renamed", "--format=json"})

		if code != ax.ExitSuccess {
			t.Fatalf("exit code = %d, want %d; stderr=%s", code, ax.ExitSuccess, stderr)
		}
		if decodeErr != nil {
			t.Fatalf("decode request body: %v", decodeErr)
		}
		for _, key := range []string{"disable_ttl", "lock_status", "lock_message", "password"} {
			if _, ok := body[key]; ok {
				t.Fatalf("expected request body to omit %q, got: %+v", key, body)
			}
		}
	})
}
