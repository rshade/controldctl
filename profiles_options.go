package main

import (
	"context"

	"github.com/spf13/cobra"

	"github.com/rshade/ax-go"

	"github.com/rshade/controld-go-mcp/internal/controld"
)

type profileOptionsListPayload struct {
	Options []controld.ProfilesOption `json:"options"`
}

func newProfilesOptionsCommand(factory clientFactory) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "options",
		Short: "Manage profile-level options",
	}
	cmd.AddCommand(newProfilesOptionsListCommand(factory))
	cmd.AddCommand(newProfilesOptionsUpdateCommand(factory))
	return cmd
}

func newProfilesOptionsListCommand(factory clientFactory) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List available profile options and their current values",
		RunE: func(cmd *cobra.Command, _ []string) error {
			client, err := factory(cmd)
			if err != nil {
				return err
			}
			options, err := client.ListProfilesOptions(cmd.Context())
			if err != nil {
				return controld.MapError(cmd.Context(), err)
			}
			return ax.WriteJSON(cmd.OutOrStdout(), ax.NewEnvelope(cmd.Context(), profileOptionsListPayload{Options: options}))
		},
	}
}

type profileOptionUpdatePayload struct {
	ProfileID string `json:"profile_id"`
	Name      string `json:"name"`
	Updated   bool   `json:"updated"`
}

func newProfilesOptionsUpdateCommand(factory clientFactory) *cobra.Command {
	var profileID, name, value string
	var status bool

	cmd := &cobra.Command{
		Use:   "update",
		Short: "Update a profile option",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if profileID == "" || name == "" {
				return ax.NewError(cmd.Context(), "validation_error", "--profile-id and --name are required",
					ax.WithErrorExitCode(ax.ExitValidation))
			}
			client, err := factory(cmd)
			if err != nil {
				return err
			}

			params := controld.UpdateProfilesOption{
				ProfileID: profileID,
				Name:      name,
				Status:    controld.IntBool(status),
			}
			if value != "" {
				params.Value = &value
			}

			ran, err := ax.Guard(cmd.Context(), func(ctx context.Context) error {
				_, updateErr := client.UpdateProfilesOption(ctx, params)
				return updateErr
			})
			if err != nil {
				return controld.MapError(cmd.Context(), err)
			}
			return ax.WriteJSON(cmd.OutOrStdout(),
				ax.NewEnvelope(cmd.Context(), profileOptionUpdatePayload{ProfileID: profileID, Name: name, Updated: ran}))
		},
	}
	cmd.Flags().StringVar(&profileID, "profile-id", "", "profile PK (required)")
	cmd.Flags().StringVar(&name, "name", "", "option name (required)")
	cmd.Flags().BoolVar(&status, "enabled", false, "enable or disable the option")
	cmd.Flags().StringVar(&value, "value", "", "option value, if the option takes one")
	return cmd
}
