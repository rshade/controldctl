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

func TestProfilesServicesListWritesEnvelope(t *testing.T) {
	var gotPath string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"success": true, "body": {"services": [` +
			`{"PK": "netflix", "name": "Netflix", "category": "streaming", "unlock_location": "", ` +
			`"action": {"do": 0, "status": 1}}]}}`))
	}))
	defer server.Close()

	root := newRootCommand(testFactory(t, server))
	root.SetArgs([]string{"profiles", "services", "list", "--profile-id=p1", "--format=json"})

	var stdout, stderr bytes.Buffer
	code := ax.Execute(context.Background(), root,
		ax.WithStdout(&stdout),
		ax.WithStderr(&stderr),
		ax.WithEnv(func(string) string { return "" }),
	)
	if code != ax.ExitSuccess {
		t.Fatalf("exit code = %d, want %d; stderr=%s", code, ax.ExitSuccess, stderr.String())
	}
	if gotPath != "/profiles/p1/services" {
		t.Fatalf("unexpected path: %s", gotPath)
	}

	var envelope struct {
		Data struct {
			Services []struct {
				PK     string `json:"pk"`
				Name   string `json:"name"`
				Action struct {
					Do     int `json:"do"`
					Status int `json:"status"`
				} `json:"action"`
			} `json:"services"`
		} `json:"data"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &envelope); err != nil {
		t.Fatalf("unmarshal stdout %q: %v", stdout.String(), err)
	}
	if len(envelope.Data.Services) != 1 || envelope.Data.Services[0].Name != "Netflix" {
		t.Fatalf("unexpected services: %+v", envelope.Data.Services)
	}
	if envelope.Data.Services[0].PK != "netflix" {
		t.Fatalf("expected lowercase \"pk\" field to decode to \"netflix\", got %+v", envelope.Data.Services[0])
	}
	if envelope.Data.Services[0].Action.Status != 1 {
		t.Fatalf("expected nested action status 1, got %+v", envelope.Data.Services[0].Action)
	}
	if bytes.Contains(stdout.Bytes(), []byte(`"PK"`)) {
		t.Fatalf("expected service PK to be remapped to lowercase \"pk\", found uppercase \"PK\" in output: %s", stdout.String())
	}
}

func TestProfilesServicesListRequiresProfileID(t *testing.T) {
	called := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		called = true
	}))
	defer server.Close()

	root := newRootCommand(testFactory(t, server))
	root.SetArgs([]string{"profiles", "services", "list", "--format=json"})

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
		t.Fatal("unexpected HTTP call for missing --profile-id")
	}
}

func TestProfilesServicesUpdateSetsAction(t *testing.T) {
	var gotMethod, gotPath string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"success": true, "body": {"services": [{"do": 1, "status": 1}]}}`))
	}))
	defer server.Close()

	root := newRootCommand(testFactory(t, server))
	root.SetArgs([]string{
		"profiles", "services", "update",
		"--profile-id=p1", "--service=netflix", "--do=1", "--format=json",
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
	if gotPath != "/profiles/p1/services/netflix" {
		t.Fatalf("unexpected path: %s", gotPath)
	}
	if !bytes.Contains(stdout.Bytes(), []byte(`"updated":true`)) {
		t.Fatalf("expected updated:true in output: %s", stdout.String())
	}
}

func TestProfilesServicesUpdateRequiresProfileIDAndService(t *testing.T) {
	called := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		called = true
	}))
	defer server.Close()

	root := newRootCommand(testFactory(t, server))
	root.SetArgs([]string{"profiles", "services", "update", "--profile-id=p1", "--format=json"})

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
