package main

import (
	"bytes"
	"net/http"
	"strings"
	"testing"

	"github.com/rshade/ax-go"
)

const ipBody = `{"success": true, "body": {
	"ip": "203.0.113.10", "type": "residential", "org": "Example ISP",
	"country": "US", "handler": "pop-lax", "pop": "lax"}}`

func TestIPShowWritesEnvelope(t *testing.T) {
	var gotPath, gotMethod string
	stdout, stderr, code := executeCommand(t, func(w http.ResponseWriter, r *http.Request) {
		gotPath, gotMethod = r.URL.Path, r.Method
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(ipBody))
	}, []string{"ip", "show", "--format=json"})
	if code != ax.ExitSuccess {
		t.Fatalf("exit code = %d, want %d; stderr=%s", code, ax.ExitSuccess, stderr)
	}
	if gotMethod != http.MethodGet || gotPath != "/ip" {
		t.Fatalf("unexpected request: %s %s", gotMethod, gotPath)
	}

	want := `"data":{"ip":"203.0.113.10","type":"residential","org":"Example ISP",` +
		`"country":"US","handler":"pop-lax","pop":"lax"}`
	if !bytes.Contains(stdout, []byte(want)) {
		t.Fatalf("stdout = %s, want it to contain %s", stdout, want)
	}
}

// TestIPShowHelpDeclaresEnvironmentalState pins the spec requirement that
// volatility is documented rather than tagged: the payload stays byte-stable
// against a fixture, so the caller has to be told in prose instead.
func TestIPShowHelpDeclaresEnvironmentalState(t *testing.T) {
	long := newIPShowCommand(noopFactory).Long
	for _, want := range []string{"point-in-time environmental state", "network location"} {
		if !strings.Contains(long, want) {
			t.Errorf("ip show help = %q, want it to contain %q", long, want)
		}
	}
}

func TestIPShowTagsNoFieldsNonDeterministic(t *testing.T) {
	for _, field := range schemaFor(t, "ip", "show").NonDeterministicFields {
		if strings.HasPrefix(field, "data.") {
			t.Errorf("ip show tags %q non-deterministic; the payload must compare "+
				"byte-stably against a fixture", field)
		}
	}
}
