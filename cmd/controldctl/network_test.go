package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/rshade/ax-go"
)

const networkBody = `{"success": true, "body": {"network": [
	{"iata_code": "LAX", "city_name": "Los Angeles", "country_name": "United States",
	 "location": {"lat": 33.94, "long": -118.4}, "status": {"api": 1, "dns": 1, "pxy": 0}}
], "time": 1700000000, "current_pop": "lax"}}`

func TestNetworkListWritesEnvelope(t *testing.T) {
	var gotPath, gotMethod string
	stdout, stderr, code := executeCommand(t, func(w http.ResponseWriter, r *http.Request) {
		gotPath, gotMethod = r.URL.Path, r.Method
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(networkBody))
	}, []string{"network", "list", "--format=json"})
	if code != ax.ExitSuccess {
		t.Fatalf("exit code = %d, want %d; stderr=%s", code, ax.ExitSuccess, stderr)
	}
	if gotMethod != http.MethodGet || gotPath != "/network" {
		t.Fatalf("unexpected request: %s %s", gotMethod, gotPath)
	}

	var envelope struct {
		Data struct {
			Network []struct {
				IataCode string `json:"iata_code"`
				CityName string `json:"city_name"`
				Status   struct {
					API int `json:"api"`
					DNS int `json:"dns"`
					Pxy int `json:"pxy"`
				} `json:"status"`
			} `json:"network"`
		} `json:"data"`
	}
	if err := json.Unmarshal(stdout, &envelope); err != nil {
		t.Fatalf("unmarshal stdout %q: %v", stdout, err)
	}
	if len(envelope.Data.Network) != 1 {
		t.Fatalf("network = %+v, want one point of presence", envelope.Data.Network)
	}
	pop := envelope.Data.Network[0]
	if pop.IataCode != "LAX" || pop.CityName != "Los Angeles" || pop.Status.API != 1 || pop.Status.Pxy != 0 {
		t.Fatalf("unexpected point of presence: %+v", pop)
	}
}

// TestNetworkListEmitsEmptyList pins the "empty list, never null" rule: a
// machine consumer must be able to range over the result unconditionally.
func TestNetworkListEmitsEmptyList(t *testing.T) {
	stdout, stderr, code := executeCommand(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"success": true, "body": {"network": [], "time": 1700000000, "current_pop": "lax"}}`))
	}, []string{"network", "list", "--format=json"})
	if code != ax.ExitSuccess {
		t.Fatalf("exit code = %d, want %d; stderr=%s", code, ax.ExitSuccess, stderr)
	}
	if !bytes.Contains(stdout, []byte(`"data":{"network":[]}`)) {
		t.Fatalf("stdout = %s, want an empty network list rather than null", stdout)
	}
}

func TestNetworkListHelpDeclaresContinuousChange(t *testing.T) {
	long := newNetworkListCommand(noopFactory).Long
	for _, want := range []string{"changes continuously", "at call time"} {
		if !strings.Contains(long, want) {
			t.Errorf("network list help = %q, want it to contain %q", long, want)
		}
	}
}
