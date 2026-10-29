package main

import (
	"bytes"
	"context"
	"encoding/json"
	"testing"

	"github.com/rshade/ax-go"
)

// decodeMeta marshals an envelope the way a command does and returns its meta
// object as a flat key set.
func decodeMeta(t *testing.T, ctx context.Context) map[string]json.RawMessage {
	t.Helper()
	var buf bytes.Buffer
	if err := ax.WriteJSON(&buf, newEnvelope(ctx, struct{}{})); err != nil {
		t.Fatalf("write envelope: %v", err)
	}
	var decoded struct {
		Meta map[string]json.RawMessage `json:"meta"`
	}
	if err := json.Unmarshal(buf.Bytes(), &decoded); err != nil {
		t.Fatalf("unmarshal envelope %q: %v", buf.String(), err)
	}
	return decoded.Meta
}

// TestNewEnvelopeFlattensMetadata pins the composition ax.Metadata's closed
// shape forces: the embedded struct must JSON-promote into the same object as
// org_id, not nest under a member of its own.
func TestNewEnvelopeFlattensMetadata(t *testing.T) {
	got := decodeMeta(t, withOrgID(context.Background(), "org-42"))

	if _, ok := got["Metadata"]; ok {
		t.Fatalf("meta did not flatten, it nests the embedded struct: %v", got)
	}
	if _, ok := got["trace_id"]; !ok {
		t.Fatalf("meta = %v, want it to carry ax-go's trace_id", got)
	}
	orgID, ok := got["org_id"]
	if !ok {
		t.Fatalf("meta = %v, want it to carry org_id", got)
	}
	if string(orgID) != `"org-42"` {
		t.Fatalf("meta.org_id = %s, want %q", orgID, "org-42")
	}
}

// TestNewEnvelopeEchoesEmptyOrgWithoutImpersonation pins the deliberate
// absence of omitempty: an absent key would be indistinguishable from a build
// that predates the echo.
func TestNewEnvelopeEchoesEmptyOrgWithoutImpersonation(t *testing.T) {
	got := decodeMeta(t, context.Background())

	orgID, ok := got["org_id"]
	if !ok {
		t.Fatalf("meta = %v, want org_id present even with no organization in effect", got)
	}
	if string(orgID) != `""` {
		t.Fatalf("meta.org_id = %s, want an empty string meaning the token's own account", orgID)
	}
}
