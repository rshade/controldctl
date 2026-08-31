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
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/profiles/p1/groups" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
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
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		t.Fatal("unexpected HTTP call for missing required flags")
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
}

func TestProfilesFoldersCreateWritesEnvelope(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("unexpected method: %s", r.Method)
		}
		if r.URL.Path != "/profiles/p1/groups" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		var body struct {
			Name string `json:"name"`
			Do   int    `json:"do"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode request body: %v", err)
		}
		if body.Name != "Ads" {
			t.Fatalf("unexpected name in request: %+v", body)
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
	if !bytes.Contains(stdout.Bytes(), []byte(`"group":"Ads"`)) {
		t.Fatalf("expected created folder name in output: %s", stdout.String())
	}
	if bytes.Contains(stdout.Bytes(), []byte(`"PK"`)) {
		t.Fatalf("expected folder PK to be remapped to lowercase \"pk\", found uppercase \"PK\" in output: %s", stdout.String())
	}
}

func TestProfilesFoldersCreateRequiresProfileIDAndName(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		t.Fatal("unexpected HTTP call for missing required flags")
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
}

func TestProfilesFoldersUpdateWritesEnvelope(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			t.Fatalf("unexpected method: %s", r.Method)
		}
		if r.URL.Path != "/profiles/p1/groups/f1" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
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
	if !bytes.Contains(stdout.Bytes(), []byte(`"do":1`)) {
		t.Fatalf("expected updated folder do in output: %s", stdout.String())
	}
}

func TestProfilesFoldersUpdateRequiresProfileIDAndFolderID(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		t.Fatal("unexpected HTTP call for missing required flags")
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
}

func TestProfilesFoldersDeleteRequiresYes(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		t.Fatal("the real API should never be called without --yes in machine mode")
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
}

func TestProfilesFoldersDeleteWithYes(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			t.Fatalf("unexpected method: %s", r.Method)
		}
		if r.URL.Path != "/profiles/p1/groups/f1" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
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
	if !bytes.Contains(stdout.Bytes(), []byte(`"deleted":true`)) {
		t.Fatalf("expected deleted:true in output: %s", stdout.String())
	}
}
