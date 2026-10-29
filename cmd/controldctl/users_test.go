package main

import (
	"encoding/json"
	"net/http"
	"regexp"
	"slices"
	"testing"

	"github.com/rshade/ax-go"
)

const userBody = `{"success": true, "body": {
	"PK": "acct1", "resolver_ip": "1.2.3.4", "email_status": 1, "tutorials": 0, "v": 1,
	"resolver_status": 1, "rule_profile": "p1", "date": "2023-01-15", "status": 1,
	"email": "ops@example.com", "resolver_uid": "uid1", "proxy_access": 1,
	"stats_endpoint": "https://stats.example", "last_active": 1700000000, "twofa": 0,
	"debug": []}}`

// reLowerSnake matches the only key shape any envelope payload may emit.
var reLowerSnake = regexp.MustCompile(`^[a-z0-9_]+$`)

// commandSchema is the subset of ax-go's __schema output these tests assert on.
type commandSchema struct {
	Use                    string          `json:"use"`
	Commands               []commandSchema `json:"commands"`
	NonDeterministicFields []string        `json:"non_deterministic_fields"`
}

// schemaFor runs __schema and returns the node at the given command path.
func schemaFor(t *testing.T, path ...string) commandSchema {
	t.Helper()
	stdout, stderr, code := executeCommand(t, func(http.ResponseWriter, *http.Request) {
		t.Error("__schema must not call the API")
	}, []string{"__schema", "--format=json"})
	if code != ax.ExitSuccess {
		t.Fatalf("__schema exit code = %d, want %d; stderr=%s", code, ax.ExitSuccess, stderr)
	}

	var schema struct {
		Command commandSchema `json:"command"`
	}
	if err := json.Unmarshal(stdout, &schema); err != nil {
		t.Fatalf("unmarshal __schema output: %v", err)
	}

	node := schema.Command
	for _, segment := range path {
		index := slices.IndexFunc(node.Commands, func(c commandSchema) bool { return c.Use == segment })
		if index < 0 {
			t.Fatalf("command %q not found under %q", segment, node.Use)
		}
		node = node.Commands[index]
	}
	return node
}

func TestUsersShowWritesEnvelope(t *testing.T) {
	var gotPath, gotMethod string
	stdout, stderr, code := executeCommand(t, func(w http.ResponseWriter, r *http.Request) {
		gotPath, gotMethod = r.URL.Path, r.Method
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(userBody))
	}, []string{"users", "show", "--format=json"})
	if code != ax.ExitSuccess {
		t.Fatalf("exit code = %d, want %d; stderr=%s", code, ax.ExitSuccess, stderr)
	}
	if gotMethod != http.MethodGet || gotPath != "/users" {
		t.Fatalf("unexpected request: %s %s", gotMethod, gotPath)
	}

	var envelope struct {
		Data map[string]json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(stdout, &envelope); err != nil {
		t.Fatalf("unmarshal stdout %q: %v", stdout, err)
	}

	want := map[string]string{
		"pk":             `"acct1"`,
		"email":          `"ops@example.com"`,
		"status":         `true`,
		"date":           `"2023-01-15"`,
		"last_active":    `1700000000`,
		"twofa":          `false`,
		"proxy_access":   `true`,
		"stats_endpoint": `"https://stats.example"`,
		"resolver_uid":   `"uid1"`,
	}
	if len(envelope.Data) != len(want) {
		t.Fatalf("payload has %d keys, want %d: %v", len(envelope.Data), len(want), envelope.Data)
	}
	for key, value := range want {
		got, ok := envelope.Data[key]
		if !ok {
			t.Fatalf("payload is missing key %q: %v", key, envelope.Data)
		}
		if string(got) != value {
			t.Errorf("data.%s = %s, want %s", key, got, value)
		}
	}
}

// TestUsersShowPayloadKeysAreLowercase pins the casing normalization the
// wrapper exists for: the vendor's User tags its primary key "PK".
func TestUsersShowPayloadKeysAreLowercase(t *testing.T) {
	stdout, stderr, code := executeCommand(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(userBody))
	}, []string{"users", "show", "--format=json"})
	if code != ax.ExitSuccess {
		t.Fatalf("exit code = %d, want %d; stderr=%s", code, ax.ExitSuccess, stderr)
	}

	var envelope struct {
		Data map[string]json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(stdout, &envelope); err != nil {
		t.Fatalf("unmarshal stdout %q: %v", stdout, err)
	}
	if len(envelope.Data) == 0 {
		t.Fatalf("payload is empty: %s", stdout)
	}
	for key := range envelope.Data {
		if !reLowerSnake.MatchString(key) {
			t.Errorf("payload key %q is not lowercase snake_case", key)
		}
	}
}

func TestUsersShowSchemaTagsOnlyLastActive(t *testing.T) {
	fields := schemaFor(t, "users", "show").NonDeterministicFields
	if !slices.Contains(fields, "data.last_active") {
		t.Errorf("non_deterministic_fields = %v, want it to contain data.last_active", fields)
	}
	if slices.Contains(fields, "data.pk") {
		t.Errorf("non_deterministic_fields = %v, want data.pk absent: an account PK is "+
			"assigned once and does not change between calls", fields)
	}
}
