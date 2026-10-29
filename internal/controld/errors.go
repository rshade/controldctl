package controld

import (
	"context"
	"errors"

	"github.com/baptistecdr/controld-go"
	"github.com/rshade/ax-go"
)

// defaultRateLimitRetrySeconds is advised when controld-go surfaces a
// RatelimitError. controld-go already retries a 429 internally (up to its own
// RetryPolicy, default 3 attempts with exponential backoff capped at 30s)
// before giving up and returning this error, so an immediate client-side
// retry is unlikely to help — 30s matches the library's own max backoff.
const defaultRateLimitRetrySeconds = 30

// MapError classifies an error returned by any controld-go API call into an
// ax.Error with the matching exit code. controld-go wraps its typed errors
// via fmt.Errorf("%w", ...) in every call site, so this uses errors.As (not a
// type switch) to unwrap them. controld-go's HTTP-status-to-type mapping is
// counter-intuitively swapped: AuthorizationError actually fires on HTTP 401
// (bad/missing credentials) and AuthenticationError fires on HTTP 403
// (insufficient permission) — see controld-go's controld.go,
// makeRequestWithAuthTypeAndHeadersComplete. Both map to ax.ExitAuth here
// regardless, so the swap doesn't affect exit codes, only which
// actionable_fix text attaches to which upstream type.
func MapError(ctx context.Context, err error) error {
	if err == nil {
		return nil
	}

	var authzErr *controld.AuthorizationError // fires on HTTP 401
	if errors.As(err, &authzErr) {
		return ax.NewError(ctx, "controld_authentication_failed", authzErr.Error(),
			ax.WithActionableFix("re-authenticate: check CONTROLD_API_TOKEN"),
			ax.WithRetryable(false),
			ax.WithErrorExitCode(ax.ExitAuth),
		)
	}

	var authnErr *controld.AuthenticationError // fires on HTTP 403
	if errors.As(err, &authnErr) {
		return ax.NewError(ctx, "controld_permission_denied", authnErr.Error(),
			ax.WithActionableFix("the token lacks permission for this operation; use a Read+Write token"),
			ax.WithRetryable(false),
			ax.WithErrorExitCode(ax.ExitAuth),
		)
	}

	var notFoundErr *controld.NotFoundError
	if errors.As(err, &notFoundErr) {
		return ax.NewError(ctx, "controld_resource_not_found", notFoundErr.Error(),
			ax.WithActionableFix("check the resource ID"),
			ax.WithRetryable(false),
			ax.WithErrorExitCode(ax.ExitValidation),
		)
	}

	var rateLimitErr *controld.RatelimitError
	if errors.As(err, &rateLimitErr) {
		return ax.NewError(ctx, "controld_rate_limited", rateLimitErr.Error(),
			ax.WithActionableFix("wait before retrying"),
			ax.WithRetryable(true),
			ax.WithRetryAfterSeconds(defaultRateLimitRetrySeconds),
			ax.WithErrorExitCode(ax.ExitNetwork),
		)
	}

	var serviceErr *controld.ServiceError
	if errors.As(err, &serviceErr) {
		return ax.NewError(ctx, "controld_upstream_unavailable", serviceErr.Error(),
			ax.WithActionableFix("retry after a short wait or check ControlD's status page"),
			ax.WithRetryable(true),
			ax.WithErrorExitCode(ax.ExitNetwork),
		)
	}

	var requestErr *controld.RequestError
	if errors.As(err, &requestErr) {
		return ax.NewError(ctx, "controld_invalid_request", requestErr.Error(),
			ax.WithActionableFix("check the command's arguments against ControlD's API requirements"),
			ax.WithRetryable(false),
			ax.WithErrorExitCode(ax.ExitValidation),
		)
	}

	return err
}
