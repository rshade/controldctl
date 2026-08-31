package controld

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/baptistecdr/controld-go"
	"github.com/rshade/ax-go"
)

func wrap(err error) error {
	return fmt.Errorf("error from makeRequest: %w", err)
}

func TestMapErrorClassifiesWrappedUpstreamErrors(t *testing.T) {
	ctx := context.Background()

	t.Run("401 (controld-go's AuthorizationError) maps to auth, not retryable", func(t *testing.T) {
		upstream := controld.NewAuthorizationError(&controld.Error{
			StatusCode: http.StatusUnauthorized,
			Error:      controld.ResponseInfo{Message: "invalid token"},
		})
		mapped := MapError(ctx, wrap(&upstream))

		var axErr *ax.Error
		if !errors.As(mapped, &axErr) {
			t.Fatalf("expected *ax.Error, got %T: %v", mapped, mapped)
		}
		if axErr.ExitCode() != ax.ExitAuth {
			t.Errorf("exit code = %d, want %d", axErr.ExitCode(), ax.ExitAuth)
		}
	})

	t.Run("403 (controld-go's AuthenticationError) maps to auth, not retryable", func(t *testing.T) {
		upstream := controld.NewAuthenticationError(&controld.Error{
			StatusCode: http.StatusForbidden,
			Error:      controld.ResponseInfo{Message: "insufficient scope"},
		})
		mapped := MapError(ctx, wrap(&upstream))

		var axErr *ax.Error
		if !errors.As(mapped, &axErr) {
			t.Fatalf("expected *ax.Error, got %T: %v", mapped, mapped)
		}
		if axErr.ExitCode() != ax.ExitAuth {
			t.Errorf("exit code = %d, want %d", axErr.ExitCode(), ax.ExitAuth)
		}
	})

	t.Run("404 maps to validation", func(t *testing.T) {
		upstream := controld.NewNotFoundError(&controld.Error{
			StatusCode: http.StatusNotFound,
			Error:      controld.ResponseInfo{Message: "device not found"},
		})
		mapped := MapError(ctx, wrap(&upstream))

		var axErr *ax.Error
		if !errors.As(mapped, &axErr) {
			t.Fatalf("expected *ax.Error, got %T: %v", mapped, mapped)
		}
		if axErr.ExitCode() != ax.ExitValidation {
			t.Errorf("exit code = %d, want %d", axErr.ExitCode(), ax.ExitValidation)
		}
	})

	t.Run("429 maps to network and is retryable", func(t *testing.T) {
		upstream := controld.NewRatelimitError(&controld.Error{
			StatusCode: http.StatusTooManyRequests,
			Error:      controld.ResponseInfo{Message: "slow down"},
		})
		mapped := MapError(ctx, wrap(&upstream))

		var axErr *ax.Error
		if !errors.As(mapped, &axErr) {
			t.Fatalf("expected *ax.Error, got %T: %v", mapped, mapped)
		}
		if axErr.ExitCode() != ax.ExitNetwork {
			t.Errorf("exit code = %d, want %d", axErr.ExitCode(), ax.ExitNetwork)
		}
		if axErr.Retryable == nil || !*axErr.Retryable {
			t.Errorf("retryable = %v, want true", axErr.Retryable)
		}
	})

	t.Run("500 maps to network and is retryable", func(t *testing.T) {
		upstream := controld.NewServiceError(&controld.Error{
			StatusCode: http.StatusInternalServerError,
			Error:      controld.ResponseInfo{Message: "internal service error"},
		})
		mapped := MapError(ctx, wrap(&upstream))

		var axErr *ax.Error
		if !errors.As(mapped, &axErr) {
			t.Fatalf("expected *ax.Error, got %T: %v", mapped, mapped)
		}
		if axErr.ExitCode() != ax.ExitNetwork {
			t.Errorf("exit code = %d, want %d", axErr.ExitCode(), ax.ExitNetwork)
		}
	})

	t.Run("other 4xx maps to validation", func(t *testing.T) {
		upstream := controld.NewRequestError(&controld.Error{
			StatusCode: http.StatusBadRequest,
			Error:      controld.ResponseInfo{Message: "bad payload"},
		})
		mapped := MapError(ctx, wrap(&upstream))

		var axErr *ax.Error
		if !errors.As(mapped, &axErr) {
			t.Fatalf("expected *ax.Error, got %T: %v", mapped, mapped)
		}
		if axErr.ExitCode() != ax.ExitValidation {
			t.Errorf("exit code = %d, want %d", axErr.ExitCode(), ax.ExitValidation)
		}
	})

	t.Run("nil passes through unchanged", func(t *testing.T) {
		if MapError(ctx, nil) != nil {
			t.Fatal("expected nil")
		}
	})

	t.Run("unrecognized error passes through unchanged for ax.Execute's default internal_error handling", func(t *testing.T) {
		plain := errors.New("boom")
		if MapError(ctx, plain) != plain {
			t.Fatalf("expected the original error back, got %v", MapError(ctx, plain))
		}
	})
}

// TestMapErrorHandlesVendorRetryExhaustion exercises controld-go's real HTTP
// retry loop (not a hand-built typed error): the loop intercepts 429s and
// 5xxs itself, retries a couple of times against a fake server, then gives up
// and returns a plain error. This is the regression test for that class of
// bug — the table above only proves MapError's errors.As branches work
// against typed errors constructed directly, which never happens for a real
// rate-limit or outage response.
func TestMapErrorHandlesVendorRetryExhaustion(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name       string
		statusCode int
	}{
		{"repeated 500s exhaust the vendor's retries", http.StatusInternalServerError},
		{"repeated 429s exhaust the vendor's retries", http.StatusTooManyRequests},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(tt.statusCode)
			}))
			defer server.Close()

			client, err := controld.New("test-token",
				controld.BaseURL(server.URL),
				controld.UsingRetryPolicy(1, 0, 0), // 1 retry, no backoff, to keep the test fast
			)
			if err != nil {
				t.Fatalf("construct client: %v", err)
			}

			_, callErr := client.ListDevices(ctx)
			if callErr == nil {
				t.Fatalf("expected an error after the vendor's retries against repeated HTTP %d responses", tt.statusCode)
			}

			mapped := MapError(ctx, callErr)
			var axErr *ax.Error
			if !errors.As(mapped, &axErr) {
				t.Fatalf("expected *ax.Error, got %T: %v", mapped, mapped)
			}
			if axErr.ExitCode() != ax.ExitNetwork {
				t.Errorf("exit code = %d, want %d (ExitNetwork)", axErr.ExitCode(), ax.ExitNetwork)
			}
			if axErr.Retryable == nil || !*axErr.Retryable {
				t.Errorf("retryable = %v, want true", axErr.Retryable)
			}
		})
	}
}
