package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/rshade/ax-go"
)

const knownIPsBody = `{"success": true, "body": {"ips": [
	{"ip": "198.51.100.7", "ts": 1700000000, "country": "US", "city": "Los Angeles",
	 "isp": "Example ISP", "asn": 64500, "as_name": "EXAMPLE"}
]}}`

func TestAccessListWritesEnvelope(t *testing.T) {
	var gotPath, gotMethod string
	stdout, stderr, code := executeCommand(t, func(w http.ResponseWriter, r *http.Request) {
		gotPath, gotMethod = r.URL.Path, r.Method
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(knownIPsBody))
	}, []string{"access", "list", "--device-id=dev1", "--format=json"})
	if code != ax.ExitSuccess {
		t.Fatalf("exit code = %d, want %d; stderr=%s", code, ax.ExitSuccess, stderr)
	}
	if gotMethod != http.MethodGet || gotPath != "/access" {
		t.Fatalf("unexpected request: %s %s", gotMethod, gotPath)
	}

	want := `"data":{"ips":[{"ip":"198.51.100.7","last_seen":1700000000,"country":"US",` +
		`"city":"Los Angeles","isp":"Example ISP","asn":64500,"as_name":"EXAMPLE"}]}`
	if !bytes.Contains(stdout, []byte(want)) {
		t.Fatalf("stdout = %s, want it to contain %s", stdout, want)
	}
}

func TestAccessListEmitsEmptyList(t *testing.T) {
	stdout, stderr, code := executeCommand(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"success": true, "body": {"ips": []}}`))
	}, []string{"access", "list", "--device-id=dev1", "--format=json"})
	if code != ax.ExitSuccess {
		t.Fatalf("exit code = %d, want %d; stderr=%s", code, ax.ExitSuccess, stderr)
	}
	if !bytes.Contains(stdout, []byte(`"data":{"ips":[]}`)) {
		t.Fatalf("stdout = %s, want an empty ips list rather than null", stdout)
	}
}

func TestAccessListRequiresDeviceID(t *testing.T) {
	stdout, stderr, code := executeCommand(t, func(http.ResponseWriter, *http.Request) {
		t.Error("the API must not be called for a missing required flag")
	}, []string{"access", "list", "--format=json"})
	if code != ax.ExitValidation {
		t.Fatalf("exit code = %d, want %d (ExitValidation); stderr=%s", code, ax.ExitValidation, stderr)
	}
	if len(stdout) != 0 {
		t.Fatalf("validation failure leaked a success envelope: %s", stdout)
	}

	var envelope struct {
		ErrorCode     string `json:"error_code"`
		ActionableFix string `json:"actionable_fix"`
	}
	if err := json.Unmarshal(stderr, &envelope); err != nil {
		t.Fatalf("unmarshal stderr %q: %v", stderr, err)
	}
	if envelope.ErrorCode != "validation_error" {
		t.Fatalf("error_code = %q, want validation_error", envelope.ErrorCode)
	}
	if !strings.Contains(envelope.ActionableFix, "--device-id") {
		t.Fatalf("actionable_fix = %q, want it to name --device-id", envelope.ActionableFix)
	}
}

func TestAccessListTagsLastSeenNonDeterministic(t *testing.T) {
	fields := schemaFor(t, "access", "list").NonDeterministicFields
	found := false
	for _, field := range fields {
		if field == "data.ips.last_seen" {
			found = true
		}
	}
	if !found {
		t.Errorf("non_deterministic_fields = %v, want it to contain data.ips.last_seen", fields)
	}
}

func TestAccessLearnSubmitsIPs(t *testing.T) {
	var gotPath, gotMethod string
	var body struct {
		DeviceID string   `json:"device_id"`
		IPs      []string `json:"ips"`
	}
	var decodeErr error
	stdout, stderr, code := executeCommand(t, func(w http.ResponseWriter, r *http.Request) {
		gotPath, gotMethod = r.URL.Path, r.Method
		decodeErr = json.NewDecoder(r.Body).Decode(&body)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"success": true, "body": []}`))
	}, []string{"access", "learn", "--device-id=dev1", "--ips=198.51.100.7,2001:db8::1", "--format=json"})
	if code != ax.ExitSuccess {
		t.Fatalf("exit code = %d, want %d; stderr=%s", code, ax.ExitSuccess, stderr)
	}
	if gotMethod != http.MethodPost || gotPath != "/access" {
		t.Fatalf("unexpected request: %s %s", gotMethod, gotPath)
	}
	if decodeErr != nil {
		t.Fatalf("decode request body: %v", decodeErr)
	}
	if body.DeviceID != "dev1" || len(body.IPs) != 2 {
		t.Fatalf("unexpected request body: %+v", body)
	}
	want := `"data":{"device_id":"dev1","ips":["198.51.100.7","2001:db8::1"],"learned":true}`
	if !bytes.Contains(stdout, []byte(want)) {
		t.Fatalf("stdout = %s, want it to contain %s", stdout, want)
	}
}

func TestAccessLearnRequiresDeviceIDAndIPs(t *testing.T) {
	cases := []struct {
		name string
		args []string
		want string
	}{
		{"missing device id", []string{"access", "learn", "--ips=198.51.100.7", "--format=json"}, "--device-id"},
		{"missing ips", []string{"access", "learn", "--device-id=dev1", "--format=json"}, "--ips"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, stderr, code := executeCommand(t, func(http.ResponseWriter, *http.Request) {
				t.Error("the API must not be called for a missing required flag")
			}, tc.args)
			if code != ax.ExitValidation {
				t.Fatalf("exit code = %d, want %d (ExitValidation); stderr=%s", code, ax.ExitValidation, stderr)
			}
			if !bytes.Contains(stderr, []byte(tc.want)) {
				t.Fatalf("stderr = %s, want it to name %s", stderr, tc.want)
			}
		})
	}
}

// TestAccessLearnRejectsMalformedIP pins that validation happens before the
// call: a batch containing one bad address must not partially apply.
func TestAccessLearnRejectsMalformedIP(t *testing.T) {
	_, stderr, code := executeCommand(t, func(http.ResponseWriter, *http.Request) {
		t.Error("a malformed IP must be rejected before any request is made")
	}, []string{"access", "learn", "--device-id=dev1", "--ips=198.51.100.7,not-an-ip", "--format=json"})
	if code != ax.ExitValidation {
		t.Fatalf("exit code = %d, want %d (ExitValidation); stderr=%s", code, ax.ExitValidation, stderr)
	}
	if !bytes.Contains(stderr, []byte("not-an-ip")) {
		t.Fatalf("stderr = %s, want it to name the offending value", stderr)
	}
}

func TestAccessLearnDryRunMakesNoRequest(t *testing.T) {
	stdout, stderr, code := executeCommand(t, func(http.ResponseWriter, *http.Request) {
		t.Error("dry-run performed a real API call")
	}, []string{"access", "learn", "--device-id=dev1", "--ips=198.51.100.7", "--dry-run", "--format=json"})
	if code != ax.ExitSuccess {
		t.Fatalf("exit code = %d, want %d; stderr=%s", code, ax.ExitSuccess, stderr)
	}
	if !bytes.Contains(stdout, []byte(`"learned":false`)) {
		t.Fatalf("stdout = %s, want learned:false under --dry-run", stdout)
	}
	if !bytes.Contains(stdout, []byte(`"dry_run":true`)) {
		t.Fatalf("stdout = %s, want the envelope to report dry_run", stdout)
	}
}

func TestAccessDeleteWithYes(t *testing.T) {
	var gotPath, gotMethod string
	stdout, stderr, code := executeCommand(t, func(w http.ResponseWriter, r *http.Request) {
		gotPath, gotMethod = r.URL.Path, r.Method
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"success": true, "body": []}`))
	}, []string{"access", "delete", "--device-id=dev1", "--ips=198.51.100.7", "--yes", "--format=json"})
	if code != ax.ExitSuccess {
		t.Fatalf("exit code = %d, want %d; stderr=%s", code, ax.ExitSuccess, stderr)
	}
	if gotMethod != http.MethodDelete || gotPath != "/access" {
		t.Fatalf("unexpected request: %s %s", gotMethod, gotPath)
	}
	if !bytes.Contains(stdout, []byte(`"deleted":true`)) {
		t.Fatalf("stdout = %s, want deleted:true", stdout)
	}
}

func TestAccessDeleteRequiresYes(t *testing.T) {
	_, stderr, code := executeCommand(t, func(http.ResponseWriter, *http.Request) {
		t.Error("the real API must never be called without --yes in machine mode")
	}, []string{"access", "delete", "--device-id=dev1", "--ips=198.51.100.7", "--format=json"})
	if code != ax.ExitValidation {
		t.Fatalf("exit code = %d, want %d (ExitValidation); stderr=%s", code, ax.ExitValidation, stderr)
	}
	if !bytes.Contains(stderr, []byte("confirmation_required")) {
		t.Fatalf("stderr = %s, want the standard confirmation-required error", stderr)
	}
}

func TestAccessDeleteDeclinedInteractively(t *testing.T) {
	stdout, stderr, code := executeCommand(t, func(http.ResponseWriter, *http.Request) {
		t.Error("a declined prompt must not delete anything")
	}, []string{"access", "delete", "--device-id=dev1", "--ips=198.51.100.7", "--format=human"},
		ax.WithStdin(strings.NewReader("n\n")))
	if code != ax.ExitSuccess {
		t.Fatalf("exit code = %d, want %d; stderr=%s", code, ax.ExitSuccess, stderr)
	}
	if !bytes.Contains(stdout, []byte(`"deleted":false`)) {
		t.Fatalf("stdout = %s, want deleted:false after a declined prompt", stdout)
	}
}

func TestAccessDeleteUnderOrgIDDeclinedPromptsOnceAndReportsDeleted(t *testing.T) {
	stdout, stderr, code := executeCommand(t, func(http.ResponseWriter, *http.Request) {
		t.Error("a declined prompt must not delete anything")
	}, []string{"access", "delete", "--device-id=dev1", "--ips=198.51.100.7", "--org-id=org-42", "--format=human"},
		ax.WithStdin(strings.NewReader("n\n")))
	if code != ax.ExitSuccess {
		t.Fatalf("exit code = %d, want %d; stderr=%s", code, ax.ExitSuccess, stderr)
	}
	if got := bytes.Count(stderr, []byte("[y/N]")); got != 1 {
		t.Fatalf("stderr = %s, want exactly one confirmation prompt, got %d", stderr, got)
	}
	if !bytes.Contains(stderr, []byte("delete learned IPs")) || bytes.Contains(stderr, []byte("learn IPs")) {
		t.Fatalf("stderr = %s, want the delete prompt and no learn prompt", stderr)
	}
	if !bytes.Contains(stdout, []byte(`"deleted":false`)) || bytes.Contains(stdout, []byte(`"learned"`)) {
		t.Fatalf("stdout = %s, want a deleted:false payload and no learned flag", stdout)
	}
}

func TestAccessDeleteUnderOrgIDConfirmedOnceDeletes(t *testing.T) {
	var gotMethod string
	stdout, stderr, code := executeCommand(t, func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"success": true, "body": []}`))
	}, []string{"access", "delete", "--device-id=dev1", "--ips=198.51.100.7", "--org-id=org-42", "--format=human"},
		ax.WithStdin(strings.NewReader("y\n")))
	if code != ax.ExitSuccess {
		t.Fatalf("exit code = %d, want %d; stderr=%s", code, ax.ExitSuccess, stderr)
	}
	if gotMethod != http.MethodDelete {
		t.Fatalf("method = %q, want a DELETE after one confirmation", gotMethod)
	}
	if !bytes.Contains(stdout, []byte(`"deleted":true`)) {
		t.Fatalf("stdout = %s, want deleted:true", stdout)
	}
}

func TestAccessDeleteDryRunMakesNoRequest(t *testing.T) {
	stdout, stderr, code := executeCommand(t, func(http.ResponseWriter, *http.Request) {
		t.Error("dry-run performed a real API call")
	}, []string{"access", "delete", "--device-id=dev1", "--ips=198.51.100.7", "--yes", "--dry-run", "--format=json"})
	if code != ax.ExitSuccess {
		t.Fatalf("exit code = %d, want %d; stderr=%s", code, ax.ExitSuccess, stderr)
	}
	if !bytes.Contains(stdout, []byte(`"deleted":false`)) {
		t.Fatalf("stdout = %s, want deleted:false under --dry-run", stdout)
	}
	if !bytes.Contains(stdout, []byte(`"dry_run":true`)) {
		t.Fatalf("stdout = %s, want the envelope to report dry_run", stdout)
	}
}
