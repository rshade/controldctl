package main

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"

	"github.com/rshade/ax-go"

	"github.com/rshade/controldctl/internal/controld"
)

type clientFactory func(cmd *cobra.Command) (*controld.API, error)

func newRootCommand(factory clientFactory) *cobra.Command {
	var apiToken string
	var configPath string

	root := &cobra.Command{
		Use:   "controldctl",
		Short: "Manage ControlD devices, profiles, and DNS rules",
	}

	root.PersistentFlags().StringVar(&apiToken, "api-token", "", "ControlD API token (overrides CONTROLD_API_TOKEN and --config)")
	root.PersistentFlags().StringVar(&configPath, "config", "", "Hujson config file path with an api_token field")

	root.AddCommand(newDevicesCommand(factory))
	root.AddCommand(newProfilesCommand(factory))

	return root
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
