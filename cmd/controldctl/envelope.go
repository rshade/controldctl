package main

import (
	"context"

	"github.com/rshade/ax-go"
)

// meta extends ax-go's closed envelope metadata with the effective
// organization. ax.Metadata is embedded rather than reproduced field by
// field, so ax-go keeps owning trace, span, idempotency-key and dry-run, and
// the embedded struct JSON-promotes flat into one meta object.
//
// org_id deliberately carries no omitempty. The key is present on every
// success envelope, with "" meaning the token's own account, so a machine
// consumer can never confuse "this run did not impersonate" with "this build
// predates the echo".
type meta struct {
	ax.Metadata
	OrgID string `json:"org_id"`
}

// envelope mirrors ax.Envelope with the extended metadata.
type envelope[T any] struct {
	Data T    `json:"data"`
	Meta meta `json:"meta"`
}

// newEnvelope re-wraps ax.NewEnvelope's output rather than rebuilding the
// metadata: ax-go's withTraceMetadata is unexported, so the trace fields can
// only be obtained by asking ax-go for them.
func newEnvelope[T any](ctx context.Context, data T) envelope[T] {
	built := ax.NewEnvelope(ctx, data)
	return envelope[T]{
		Data: built.Data,
		Meta: meta{Metadata: built.Meta, OrgID: orgIDFromContext(ctx)},
	}
}
