package main

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"

	"github.com/rshade/ax-go"

	"github.com/rshade/controldctl/internal/controld"
)

type clientFactory func(cmd *cobra.Command) (*controld.API, error)

// rootConfig holds the process-level seams newRootCommand needs but cannot
// discover for itself.
type rootConfig struct {
	getenv func(string) string
}

type rootOption func(*rootConfig)

// withGetenv supplies the environment accessor used to resolve --org-id's
// CONTROLD_ORG_ID fallback. It defaults to an empty environment so a test —
// or a developer with CONTROLD_ORG_ID exported — cannot silently impersonate
// an organization in a golden fixture.
func withGetenv(getenv func(string) string) rootOption {
	return func(cfg *rootConfig) { cfg.getenv = getenv }
}

// orgContextKey scopes the effective organization to one command run. The
// value is resolved once in PersistentPreRunE rather than at each use, so the
// impersonation header, the confirmation gate and the envelope echo can never
// disagree about which tenant a command acted on.
type orgContextKey struct{}

func withOrgID(ctx context.Context, orgID string) context.Context {
	return context.WithValue(ctx, orgContextKey{}, orgID)
}

// orgIDFromContext returns the effective organization, or "" for the token's
// own account.
func orgIDFromContext(ctx context.Context) string {
	orgID, _ := ctx.Value(orgContextKey{}).(string)
	return orgID
}

func newRootCommand(factory clientFactory, opts ...rootOption) *cobra.Command {
	cfg := rootConfig{getenv: func(string) string { return "" }}
	for _, opt := range opts {
		opt(&cfg)
	}

	var apiToken string
	var configPath string
	var orgID string

	root := &cobra.Command{
		Use:   "controldctl",
		Short: "Manage ControlD devices, profiles, and DNS rules",
	}

	root.PersistentFlags().StringVar(&apiToken, "api-token", "", "ControlD API token (overrides CONTROLD_API_TOKEN and --config)")
	root.PersistentFlags().StringVar(&configPath, "config", "", "Hujson config file path with an api_token field")
	root.PersistentFlags().StringVar(&orgID, "org-id", "",
		"ControlD sub-organization to act as (overrides CONTROLD_ORG_ID and --config); "+
			"while set, every mutation requires --yes")

	// ax.Execute wraps this hook rather than replacing it, and installs its
	// own context (mode, dry-run, approval) before calling it — so the
	// organization layers on top of ax-go's context rather than racing it.
	root.PersistentPreRunE = func(cmd *cobra.Command, _ []string) error {
		resolved, err := controld.ResolveOrgID(cmd.Context(), orgID, configPath, cfg.getenv)
		if err != nil {
			return err
		}
		cmd.SetContext(withOrgID(cmd.Context(), resolved))
		return nil
	}

	root.AddCommand(newAccessCommand(factory))
	root.AddCommand(newDevicesCommand(factory))
	root.AddCommand(newIPCommand(factory))
	root.AddCommand(newBillingCommand(factory))
	root.AddCommand(newNetworkCommand(factory))
	root.AddCommand(newProfilesCommand(factory))
	root.AddCommand(newUsersCommand(factory))

	return root
}

// requireConfirmation reports whether this mutation must be confirmed: always
// for a destructive verb, and for every verb while an organization is in
// effect. A cross-tenant mutation's blast radius lands on an account the
// caller does not own, which is reason enough to gate a create the way a
// delete is gated.
func requireConfirmation(cmd *cobra.Command, destructive bool) bool {
	return destructive || orgIDFromContext(cmd.Context()) != ""
}

// confirmed runs the confirmation gate for subject and reports whether the
// caller may proceed. In machine mode without --yes it returns ax.Confirm's
// confirmation-required error; in human mode it owns the prompt, and a false
// result means the operator declined.
func confirmed(cmd *cobra.Command, subject string) (bool, error) {
	outcome, err := ax.Confirm(cmd.Context(), subject)
	if err != nil {
		return false, err
	}
	if outcome != ax.ConfirmationPromptRequired {
		return true, nil
	}
	return promptForConfirmation(cmd, subject)
}

func promptForConfirmation(cmd *cobra.Command, subject string) (bool, error) {
	if _, err := fmt.Fprintf(cmd.ErrOrStderr(), "%s? [y/N] ", subject); err != nil {
		return false, fmt.Errorf("write confirmation prompt: %w", err)
	}

	response, err := bufio.NewReader(cmd.InOrStdin()).ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return false, fmt.Errorf("read confirmation response: %w", err)
	}
	response = strings.TrimSpace(response)
	return strings.EqualFold(response, "y") || strings.EqualFold(response, "yes"), nil
}

func validationError(cmd *cobra.Command, msg, fix string) error {
	return ax.NewError(cmd.Context(), "validation_error", msg,
		ax.WithActionableFix(fix), ax.WithErrorExitCode(ax.ExitValidation))
}
