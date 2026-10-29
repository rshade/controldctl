package main

import (
	"bytes"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"github.com/rshade/ax-go"
)

// orgHeader is ControlD's cross-tenant impersonation header. It is spelled out
// here rather than imported: the exact wire name is the contract under test.
const orgHeader = "X-Force-Org-Id"

func TestOrgIDSendsImpersonationHeader(t *testing.T) {
	var got []string
	var seen bool
	_, stderr, code := executeCommand(t, func(w http.ResponseWriter, r *http.Request) {
		got, seen = r.Header[orgHeader]
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"success": true, "body": {"devices": []}}`))
	}, []string{"devices", "list", "--org-id=org-42", "--format=json"})
	if code != ax.ExitSuccess {
		t.Fatalf("exit code = %d, want %d; stderr=%s", code, ax.ExitSuccess, stderr)
	}
	if !seen {
		t.Fatal("request carried no impersonation header")
	}
	if len(got) != 1 || got[0] != "org-42" {
		t.Fatalf("%s = %v, want [org-42]", orgHeader, got)
	}
}

func TestNoOrgIDSendsNoImpersonationHeader(t *testing.T) {
	var seen bool
	_, stderr, code := executeCommand(t, func(w http.ResponseWriter, r *http.Request) {
		_, seen = r.Header[orgHeader]
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"success": true, "body": {"devices": []}}`))
	}, []string{"devices", "list", "--format=json"})
	if code != ax.ExitSuccess {
		t.Fatalf("exit code = %d, want %d; stderr=%s", code, ax.ExitSuccess, stderr)
	}
	if seen {
		t.Fatalf("request carried %s with no organization in effect", orgHeader)
	}
}

// TestOrgIDSendsHeaderOnMutations pins that impersonation is not read-only:
// the client-wide header option applies to every verb, not just GETs.
func TestOrgIDSendsHeaderOnMutations(t *testing.T) {
	var got string
	_, stderr, code := executeCommand(t, func(w http.ResponseWriter, r *http.Request) {
		got = r.Header.Get(orgHeader)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"success": true, "body": []}`))
	}, []string{"devices", "delete", "--device-id=dev1", "--org-id=org-42", "--yes", "--format=json"})
	if code != ax.ExitSuccess {
		t.Fatalf("exit code = %d, want %d; stderr=%s", code, ax.ExitSuccess, stderr)
	}
	if got != "org-42" {
		t.Fatalf("%s = %q, want %q", orgHeader, got, "org-42")
	}
}

// TestImpersonatedMutationsRequireYes pins the deliberate break from the v1
// rule: under impersonation a create and an update are gated the same way a
// delete always was.
func TestImpersonatedMutationsRequireYes(t *testing.T) {
	cases := []struct {
		name string
		args []string
	}{
		{
			name: "create",
			args: []string{"devices", "create", "--name=laptop", "--profile-id=p1", "--org-id=org-42", "--format=json"},
		},
		{
			name: "update",
			args: []string{"devices", "update", "--device-id=dev1", "--name=renamed", "--org-id=org-42", "--format=json"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			stdout, stderr, code := executeCommand(t, func(http.ResponseWriter, *http.Request) {
				t.Error("an unconfirmed cross-tenant mutation must never reach the API")
			}, tc.args)
			if code != ax.ExitValidation {
				t.Fatalf("exit code = %d, want %d (ExitValidation); stderr=%s", code, ax.ExitValidation, stderr)
			}
			if len(stdout) != 0 {
				t.Fatalf("gated mutation leaked a success envelope: %s", stdout)
			}
			if !bytes.Contains(stderr, []byte("confirmation_required")) {
				t.Fatalf("stderr = %s, want the standard confirmation-required error", stderr)
			}
		})
	}
}

// TestUnimpersonatedMutationsUnchanged pins that the stronger gate costs
// existing automation nothing: without --org-id, a create and an update still
// succeed with no --yes, and emit the same bytes v1 emitted.
func TestUnimpersonatedMutationsUnchanged(t *testing.T) {
	cases := []struct {
		name   string
		golden string
		args   []string
	}{
		{
			name:   "create",
			golden: "devices_create.golden.json",
			args:   []string{"devices", "create", "--name=laptop", "--profile-id=p1", "--format=json"},
		},
		{
			name:   "update",
			golden: "devices_update.golden.json",
			args:   []string{"devices", "update", "--device-id=dev1", "--name=renamed", "--format=json"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			stdout, stderr, code := runGolden(t, jsonResponse(deviceBody), tc.args)
			if code != ax.ExitSuccess {
				t.Fatalf("exit code = %d, want %d; stderr=%s", code, ax.ExitSuccess, stderr)
			}
			want, err := os.ReadFile(filepath.Join("testdata", tc.golden))
			if err != nil {
				t.Fatalf("read golden: %v", err)
			}
			if got := maskNonDeterministic(stdout); !bytes.Equal(got, want) {
				t.Fatalf("output drifted from the ungated fixture\nwant: %s\ngot:  %s", want, got)
			}
		})
	}
}

// TestImpersonatedDryRunEchoesOrg pins that the two pieces of metadata a
// cross-tenant rehearsal needs — that nothing happened, and which tenant it
// would have happened to — are both present.
func TestImpersonatedDryRunEchoesOrg(t *testing.T) {
	stdout, stderr, code := executeCommand(t, func(http.ResponseWriter, *http.Request) {
		t.Error("dry-run performed a real API call")
	}, []string{
		"devices", "create", "--name=laptop", "--profile-id=p1",
		"--org-id=org-42", "--yes", "--dry-run", "--format=json",
	})
	if code != ax.ExitSuccess {
		t.Fatalf("exit code = %d, want %d; stderr=%s", code, ax.ExitSuccess, stderr)
	}
	for _, want := range []string{`"dry_run":true`, `"org_id":"org-42"`} {
		if !bytes.Contains(stdout, []byte(want)) {
			t.Fatalf("stdout = %s, want it to contain %s", stdout, want)
		}
	}
}

func TestImpersonatedSuccessEchoesOrg(t *testing.T) {
	stdout, stderr, code := executeCommand(t, jsonResponse(`{"success": true, "body": {"devices": []}}`),
		[]string{"devices", "list", "--org-id=org-42", "--format=json"})
	if code != ax.ExitSuccess {
		t.Fatalf("exit code = %d, want %d; stderr=%s", code, ax.ExitSuccess, stderr)
	}
	if !bytes.Contains(stdout, []byte(`"org_id":"org-42"`)) {
		t.Fatalf("stdout = %s, want the effective organization echoed", stdout)
	}
}

// TestUnentitledOrgSurfacesUpstreamError pins that impersonation adds no error
// handling of its own: a 403 for an organization the token may not act for
// reaches the operator through the existing mapping.
func TestUnentitledOrgSurfacesUpstreamError(t *testing.T) {
	stdout, stderr, code := executeCommand(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"success": false, "error": {"message": "forbidden", "code": 403}}`))
	}, []string{"devices", "list", "--org-id=org-42", "--format=json"})
	if code != ax.ExitAuth {
		t.Fatalf("exit code = %d, want %d (ExitAuth); stderr=%s", code, ax.ExitAuth, stderr)
	}
	if len(stdout) != 0 {
		t.Fatalf("error path leaked a success envelope: %s", stdout)
	}
	if !bytes.Contains(stderr, []byte("controld_permission_denied")) {
		t.Fatalf("stderr = %s, want the existing mapped permission error", stderr)
	}
}
