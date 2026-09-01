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

func TestProfilesFiltersListDefaultsToNative(t *testing.T) {
	var gotPath string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"success": true, "body": {"filters": [{"PK": "ads", "name": "Ads", "description": "d", "sources": [], "status": 1}]}}`))
	}))
	defer server.Close()

	root := newRootCommand(testFactory(t, server))
	root.SetArgs([]string{"profiles", "filters", "list", "--profile-id=p1", "--format=json"})

	var stdout, stderr bytes.Buffer
	code := ax.Execute(context.Background(), root,
		ax.WithStdout(&stdout),
		ax.WithStderr(&stderr),
		ax.WithEnv(func(string) string { return "" }),
	)
	if code != ax.ExitSuccess {
		t.Fatalf("exit code = %d, want %d; stderr=%s", code, ax.ExitSuccess, stderr.String())
	}
	if gotPath != "/profiles/p1/filters" {
		t.Fatalf("unexpected path: %s", gotPath)
	}

	var envelope struct {
		Data struct {
			Filters []struct {
				PK   string `json:"pk"`
				Name string `json:"name"`
			} `json:"filters"`
		} `json:"data"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &envelope); err != nil {
		t.Fatalf("unmarshal stdout %q: %v", stdout.String(), err)
	}
	if len(envelope.Data.Filters) != 1 || envelope.Data.Filters[0].Name != "Ads" {
		t.Fatalf("unexpected filters: %+v", envelope.Data.Filters)
	}
	if envelope.Data.Filters[0].PK != "ads" {
		t.Fatalf("expected lowercase \"pk\" field to decode to \"ads\", got %+v", envelope.Data.Filters[0])
	}
	if bytes.Contains(stdout.Bytes(), []byte(`"PK"`)) {
		t.Fatalf("expected filter PK to be remapped to lowercase \"pk\", found uppercase \"PK\" in output: %s", stdout.String())
	}
}

func TestProfilesFiltersListRemapsNestedOptPKCasing(t *testing.T) {
	var gotPath string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"success": true, "body": {"filters": [{
			"PK": "custom1",
			"name": "Custom",
			"description": "d",
			"sources": [],
			"status": 1,
			"levels": [
				{"title": "Strict", "type": "toggle", "name": "strict", "status": 1,
				 "opt": [{"PK": "opt1", "value": true}]}
			]
		}]}}`))
	}))
	defer server.Close()

	root := newRootCommand(testFactory(t, server))
	root.SetArgs([]string{"profiles", "filters", "list", "--profile-id=p1", "--format=json"})

	var stdout, stderr bytes.Buffer
	code := ax.Execute(context.Background(), root,
		ax.WithStdout(&stdout),
		ax.WithStderr(&stderr),
		ax.WithEnv(func(string) string { return "" }),
	)
	if code != ax.ExitSuccess {
		t.Fatalf("exit code = %d, want %d; stderr=%s", code, ax.ExitSuccess, stderr.String())
	}
	if gotPath != "/profiles/p1/filters" {
		t.Fatalf("unexpected path: %s", gotPath)
	}

	var envelope struct {
		Data struct {
			Filters []struct {
				PK     string `json:"pk"`
				Levels []struct {
					Opt []struct {
						PK string `json:"pk"`
					} `json:"opt"`
				} `json:"levels"`
			} `json:"filters"`
		} `json:"data"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &envelope); err != nil {
		t.Fatalf("unmarshal stdout %q: %v", stdout.String(), err)
	}
	if len(envelope.Data.Filters) != 1 {
		t.Fatalf("unexpected filters: %+v", envelope.Data.Filters)
	}
	filter := envelope.Data.Filters[0]
	if filter.PK != "custom1" {
		t.Fatalf("expected filter pk %q, got %q", "custom1", filter.PK)
	}
	if len(filter.Levels) != 1 || len(filter.Levels[0].Opt) != 1 || filter.Levels[0].Opt[0].PK != "opt1" {
		t.Fatalf("expected nested opt pk %q to decode via lowercase \"pk\", got %+v", "opt1", filter.Levels)
	}
	if bytes.Contains(stdout.Bytes(), []byte(`"PK"`)) {
		t.Fatalf("expected all filter PK fields (including nested levels[].opt[].PK) to be remapped to "+
			"lowercase \"pk\", found uppercase \"PK\" in output: %s", stdout.String())
	}
}

func TestProfilesFiltersListExternalSource(t *testing.T) {
	var gotPath string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"success": true, "body": {"filters": []}}`))
	}))
	defer server.Close()

	root := newRootCommand(testFactory(t, server))
	root.SetArgs([]string{"profiles", "filters", "list", "--profile-id=p1", "--source=external", "--format=json"})

	var stdout, stderr bytes.Buffer
	code := ax.Execute(context.Background(), root,
		ax.WithStdout(&stdout),
		ax.WithStderr(&stderr),
		ax.WithEnv(func(string) string { return "" }),
	)
	if code != ax.ExitSuccess {
		t.Fatalf("exit code = %d, want %d; stderr=%s", code, ax.ExitSuccess, stderr.String())
	}
	if gotPath != "/profiles/p1/filters/external" {
		t.Fatalf("unexpected path: %s", gotPath)
	}
}

func TestProfilesFiltersListRejectsInvalidSource(t *testing.T) {
	called := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		called = true
	}))
	defer server.Close()

	root := newRootCommand(testFactory(t, server))
	root.SetArgs([]string{"profiles", "filters", "list", "--profile-id=p1", "--source=bogus", "--format=json"})

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
		t.Fatal("unexpected HTTP call for invalid --source")
	}
}

func TestProfilesFiltersUpdateEnablesFilter(t *testing.T) {
	var gotMethod, gotPath string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"success": true, "body": {"filters": null}}`))
	}))
	defer server.Close()

	root := newRootCommand(testFactory(t, server))
	root.SetArgs([]string{"profiles", "filters", "update", "--profile-id=p1", "--filter=ads", "--format=json"})

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
	if gotPath != "/profiles/p1/filters/filter/ads" {
		t.Fatalf("unexpected path: %s", gotPath)
	}
	if !bytes.Contains(stdout.Bytes(), []byte(`"updated":true`)) {
		t.Fatalf("expected updated:true in output: %s", stdout.String())
	}
}

func TestProfilesFiltersUpdateRequiresProfileIDAndFilter(t *testing.T) {
	called := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		called = true
	}))
	defer server.Close()

	root := newRootCommand(testFactory(t, server))
	root.SetArgs([]string{"profiles", "filters", "update", "--profile-id=p1", "--format=json"})

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
