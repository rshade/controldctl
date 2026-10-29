package controld

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/baptistecdr/controld-go"
)

// probeTokenEnv gates the Phase C endpoint probe. ControlD publishes empty
// response schemas for /billing/* and documents /organizations/* and
// /provision nowhere at all, so the only way to learn their real shapes is to
// ask a live business-tier account. The probe is therefore opt-in, never runs
// in CI, and records field names and JSON types only — never values.
const probeTokenEnv = "CONTROLD_PROBE_API_TOKEN"

// probeEndpoints is one read per Phase C resource group.
//
//nolint:gochecknoglobals // test-only fixture list
var probeEndpoints = []string{
	"/billing/payments",
	"/billing/subscriptions",
	"/billing/products",
	"/organizations/organization",
	"/organizations/members",
	"/organizations/sub_organizations",
	"/provision",
}

// statusRecorder observes the HTTP status of each response without taking
// over the request. controld-go's typed errors keep their *Error unexported,
// so a caller cannot read the status back off them — but the client accepts a
// custom *http.Client, and a RoundTripper in front of the default transport
// sees the real code while every request is still issued by the library.
type statusRecorder struct {
	base   http.RoundTripper
	status int
}

func (s *statusRecorder) RoundTrip(req *http.Request) (*http.Response, error) {
	resp, err := s.base.RoundTrip(req)
	if resp != nil {
		s.status = resp.StatusCode
	}
	return resp, err
}

// describeShape renders a decoded JSON value as sorted "path: type" lines,
// carrying no values at all. Account identifiers, emails and payment amounts
// therefore never reach the output, which is what makes the findings safe to
// paste into a committed document.
func describeShape(prefix string, value any, out map[string]string) {
	switch typed := value.(type) {
	case map[string]any:
		if len(typed) == 0 {
			out[prefix] = "object (empty)"
			return
		}
		for key, member := range typed {
			describeShape(prefix+"."+key, member, out)
		}
	case []any:
		if len(typed) == 0 {
			out[prefix+"[]"] = "array (empty)"
			return
		}
		describeShape(prefix+"[]", typed[0], out)
	case string:
		out[prefix] = "string"
	case float64:
		out[prefix] = "number"
	case bool:
		out[prefix] = "boolean"
	case nil:
		out[prefix] = "null"
	default:
		out[prefix] = fmt.Sprintf("%T", value)
	}
}

func shapeLines(body json.RawMessage) []string {
	var decoded any
	if err := json.Unmarshal(body, &decoded); err != nil {
		return []string{fmt.Sprintf("body did not decode as JSON: %v", err)}
	}
	fields := map[string]string{}
	describeShape("body", decoded, fields)
	lines := make([]string, 0, len(fields))
	for path, kind := range fields {
		lines = append(lines, path+": "+kind)
	}
	sort.Strings(lines)
	return lines
}

// TestProbePhaseCEndpoints records the live shape of every endpoint Phase C
// needs and that v0.0.10 has no typed method for. It asserts nothing: its
// output is the input to the fork's struct definitions, transcribed by hand
// into docs/upstream-controld-go-gaps.md.
//
// Run it against a business-tier account with:
//
//	CONTROLD_PROBE_API_TOKEN=<token> go test ./internal/controld -run TestProbe -v
func TestProbePhaseCEndpoints(t *testing.T) {
	token := os.Getenv(probeTokenEnv)
	if token == "" {
		t.Skipf("set %s to a business-tier ControlD token to run the Phase C endpoint probe", probeTokenEnv)
	}
	if os.Getenv("CI") != "" {
		t.Skip("the Phase C probe talks to the live ControlD API and never runs in CI")
	}

	scope := os.Getenv("CONTROLD_PROBE_TOKEN_SCOPE")
	if scope == "" {
		scope = "unspecified — set CONTROLD_PROBE_TOKEN_SCOPE to record it"
	}
	t.Logf("probing with a token whose scope is: %s", scope)

	for _, endpoint := range probeEndpoints {
		t.Run(strings.TrimPrefix(endpoint, "/"), func(t *testing.T) {
			recorder := &statusRecorder{base: http.DefaultTransport}
			client, err := controld.New(token, controld.HTTPClient(&http.Client{
				Transport: recorder,
				Timeout:   30 * time.Second,
			}))
			if err != nil {
				t.Fatalf("construct probe client: %v", err)
			}

			ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
			defer cancel()

			raw, reqErr := client.Raw(ctx, http.MethodGet, endpoint, nil, nil)
			if reqErr != nil {
				t.Logf("GET %s -> HTTP %d, NOT reachable: %v", endpoint, recorder.status, reqErr)
				return
			}
			t.Logf("GET %s -> HTTP %d, reachable (success=%t)", endpoint, recorder.status, raw.Success)
			for _, line := range shapeLines(raw.Body) {
				t.Logf("  %s", line)
			}
		})
	}
}
