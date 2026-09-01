package main

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/rshade/ax-go"

	"github.com/rshade/controldctl/internal/controld"
)

// failingFactory returns a clientFactory that fails the test if invoked.
// Used to ensure validation-only errors never reach the HTTP client.
func failingFactory(t *testing.T) clientFactory {
	t.Helper()
	return func(cmd *cobra.Command) (*controld.API, error) {
		t.Fatal("client factory must not be called for a validation-only case")
		return nil, nil
	}
}

// TestValidationErrorsIncludeActionableFix asserts that every local
// flag-validation error (ax.NewError with ax.ExitValidation in the command
// files) carries an actionable_fix hint naming the flag that fixes it. The
// table has one row per ax.NewError validation call site.
func TestValidationErrorsIncludeActionableFix(t *testing.T) {
	cases := []struct {
		name string
		args []string
		want []string // substrings the actionable_fix must contain
	}{
		{
			name: "devices create requires --name and --profile-id",
			args: []string{"devices", "create", "--format=json"},
			want: []string{"--name", "--profile-id"},
		},
		{
			name: "devices update requires --device-id",
			args: []string{"devices", "update", "--format=json"},
			want: []string{"--device-id"},
		},
		{
			name: "devices delete requires --device-id",
			args: []string{"devices", "delete", "--format=json"},
			want: []string{"--device-id"},
		},
		{
			name: "profiles create requires --name",
			args: []string{"profiles", "create", "--format=json"},
			want: []string{"--name"},
		},
		{
			name: "profiles update requires --profile-id",
			args: []string{"profiles", "update", "--format=json"},
			want: []string{"--profile-id"},
		},
		{
			name: "profiles delete requires --profile-id",
			args: []string{"profiles", "delete", "--format=json"},
			want: []string{"--profile-id"},
		},
		{
			name: "profiles options update requires --profile-id and --name",
			args: []string{"profiles", "options", "update", "--format=json"},
			want: []string{"--profile-id", "--name"},
		},
		{
			name: "profiles rules list requires --profile-id and --folder-id",
			args: []string{"profiles", "rules", "list", "--format=json"},
			want: []string{"--profile-id", "--folder-id"},
		},
		{
			name: "profiles rules create requires --profile-id and --hostnames",
			args: []string{"profiles", "rules", "create", "--format=json"},
			want: []string{"--profile-id", "--hostnames"},
		},
		{
			name: "profiles rules create requires --via when --do=3 (redirect)",
			args: []string{"profiles", "rules", "create", "--profile-id=p1", "--hostnames=example.com", "--do=3", "--format=json"},
			want: []string{"--via"},
		},
		{
			name: "profiles rules update requires --profile-id and --hostnames",
			args: []string{"profiles", "rules", "update", "--format=json"},
			want: []string{"--profile-id", "--hostnames"},
		},
		{
			name: "profiles rules update requires --do and --enabled when using --group",
			args: []string{"profiles", "rules", "update", "--profile-id=p1", "--hostnames=example.com", "--group=7", "--format=json"},
			want: []string{"--do", "--enabled"},
		},
		{
			name: "profiles rules update requires --via when --do=3 (redirect)",
			args: []string{"profiles", "rules", "update", "--profile-id=p1", "--hostnames=example.com", "--do=3", "--format=json"},
			want: []string{"--via"},
		},
		{
			name: "profiles rules delete requires --profile-id and --hostname",
			args: []string{"profiles", "rules", "delete", "--format=json"},
			want: []string{"--profile-id", "--hostname"},
		},
		{
			name: "profiles services list requires --profile-id",
			args: []string{"profiles", "services", "list", "--format=json"},
			want: []string{"--profile-id"},
		},
		{
			name: "profiles services update requires --profile-id and --service",
			args: []string{"profiles", "services", "update", "--format=json"},
			want: []string{"--profile-id", "--service"},
		},
		{
			name: "profiles folders list requires --profile-id",
			args: []string{"profiles", "folders", "list", "--format=json"},
			want: []string{"--profile-id"},
		},
		{
			name: "profiles folders create requires --profile-id and --name",
			args: []string{"profiles", "folders", "create", "--format=json"},
			want: []string{"--profile-id", "--name"},
		},
		{
			name: "profiles folders update requires --profile-id and --folder-id",
			args: []string{"profiles", "folders", "update", "--format=json"},
			want: []string{"--profile-id", "--folder-id"},
		},
		{
			name: "profiles folders delete requires --profile-id and --folder-id",
			args: []string{"profiles", "folders", "delete", "--format=json"},
			want: []string{"--profile-id", "--folder-id"},
		},
		{
			name: "profiles filters list requires --profile-id",
			args: []string{"profiles", "filters", "list", "--format=json"},
			want: []string{"--profile-id"},
		},
		{
			name: "profiles filters list rejects invalid --source",
			args: []string{"profiles", "filters", "list", "--profile-id=p1", "--source=bogus", "--format=json"},
			want: []string{"--source"},
		},
		{
			name: "profiles filters update requires --profile-id and --filter",
			args: []string{"profiles", "filters", "update", "--format=json"},
			want: []string{"--profile-id", "--filter"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root := newRootCommand(failingFactory(t))
			root.SetArgs(tc.args)

			var stdout, stderr bytes.Buffer
			code := ax.Execute(context.Background(), root,
				ax.WithStdout(&stdout),
				ax.WithStderr(&stderr),
				ax.WithEnv(func(string) string { return "" }),
			)
			if code != ax.ExitValidation {
				t.Fatalf("exit code = %d, want %d (ExitValidation); stderr=%s", code, ax.ExitValidation, stderr.String())
			}

			var envelope struct {
				ErrorCode     string `json:"error_code"`
				ActionableFix string `json:"actionable_fix"`
			}
			if err := json.Unmarshal(stderr.Bytes(), &envelope); err != nil {
				t.Fatalf("unmarshal stderr error envelope %q: %v", stderr.String(), err)
			}
			if envelope.ErrorCode != "validation_error" {
				t.Fatalf("error_code = %q, want %q; stderr=%s", envelope.ErrorCode, "validation_error", stderr.String())
			}
			if envelope.ActionableFix == "" {
				t.Fatalf("actionable_fix is empty; stderr=%s", stderr.String())
			}
			for _, want := range tc.want {
				if !strings.Contains(envelope.ActionableFix, want) {
					t.Fatalf("actionable_fix = %q, want it to contain %q", envelope.ActionableFix, want)
				}
			}
		})
	}
}
