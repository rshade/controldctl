package main

import (
	"context"

	"github.com/spf13/cobra"

	"github.com/rshade/ax-go"

	"github.com/rshade/controld-go-mcp/internal/controld"
)

type profilesListPayload struct {
	Profiles []controld.Profile `json:"profiles"`
}

type profileDeletePayload struct {
	ProfileID string `json:"profile_id"`
	Deleted   bool   `json:"deleted"`
}

func newProfilesCommand(factory clientFactory) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "profiles",
		Short: "Manage ControlD profiles",
	}
	cmd.AddCommand(newProfilesListCommand(factory))
	cmd.AddCommand(newProfilesCreateCommand(factory))
	cmd.AddCommand(newProfilesUpdateCommand(factory))
	cmd.AddCommand(newProfilesDeleteCommand(factory))
	cmd.AddCommand(newProfilesOptionsCommand(factory))
	return cmd
}

func newProfilesListCommand(factory clientFactory) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List all profiles",
		RunE: func(cmd *cobra.Command, _ []string) error {
			client, err := factory(cmd)
			if err != nil {
				return err
			}
			profiles, err := client.ListProfiles(cmd.Context())
			if err != nil {
				return controld.MapError(cmd.Context(), err)
			}
			return ax.WriteJSON(cmd.OutOrStdout(), ax.NewEnvelope(cmd.Context(), profilesListPayload{Profiles: profiles}))
		},
	}
}

func newProfilesCreateCommand(factory clientFactory) *cobra.Command {
	var name, cloneProfileID string

	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a profile",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if name == "" {
				return ax.NewError(cmd.Context(), "validation_error", "--name is required",
					ax.WithErrorExitCode(ax.ExitValidation))
			}
			client, err := factory(cmd)
			if err != nil {
				return err
			}

			params := controld.CreateProfileParams{Name: name}
			if cloneProfileID != "" {
				params.CloneProfileID = &cloneProfileID
			}

			var result []controld.Profile
			ran, err := ax.Guard(cmd.Context(), func(ctx context.Context) error {
				var createErr error
				result, createErr = client.CreateProfile(ctx, params)
				return createErr
			})
			if err != nil {
				return controld.MapError(cmd.Context(), err)
			}
			if !ran {
				result = nil
			}
			return ax.WriteJSON(cmd.OutOrStdout(), ax.NewEnvelope(cmd.Context(), profilesListPayload{Profiles: result}))
		},
	}
	cmd.Flags().StringVar(&name, "name", "", "profile name (required)")
	cmd.Flags().StringVar(&cloneProfileID, "clone-profile-id", "", "existing profile PK to clone from")
	return cmd
}

func newProfilesUpdateCommand(factory clientFactory) *cobra.Command {
	var profileID, name string

	cmd := &cobra.Command{
		Use:   "update",
		Short: "Update a profile",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if profileID == "" {
				return ax.NewError(cmd.Context(), "validation_error", "--profile-id is required",
					ax.WithErrorExitCode(ax.ExitValidation))
			}
			client, err := factory(cmd)
			if err != nil {
				return err
			}

			params := controld.UpdateProfileParams{ProfileID: profileID}
			if name != "" {
				params.Name = &name
			}

			var result []controld.Profile
			ran, err := ax.Guard(cmd.Context(), func(ctx context.Context) error {
				var updateErr error
				result, updateErr = client.UpdateProfile(ctx, params)
				return updateErr
			})
			if err != nil {
				return controld.MapError(cmd.Context(), err)
			}
			if !ran {
				result = nil
			}
			return ax.WriteJSON(cmd.OutOrStdout(), ax.NewEnvelope(cmd.Context(), profilesListPayload{Profiles: result}))
		},
	}
	cmd.Flags().StringVar(&profileID, "profile-id", "", "profile PK to update (required)")
	cmd.Flags().StringVar(&name, "name", "", "new profile name")
	return cmd
}

func newProfilesDeleteCommand(factory clientFactory) *cobra.Command {
	var profileID string

	cmd := &cobra.Command{
		Use:   "delete",
		Short: "Delete a profile",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if profileID == "" {
				return ax.NewError(cmd.Context(), "validation_error", "--profile-id is required",
					ax.WithErrorExitCode(ax.ExitValidation))
			}

			subject := "delete profile " + profileID
			outcome, err := ax.Confirm(cmd.Context(), subject)
			if err != nil {
				return err
			}
			if outcome == ax.ConfirmationPromptRequired {
				approved, promptErr := promptForConfirmation(cmd, subject)
				if promptErr != nil {
					return promptErr
				}
				if !approved {
					return ax.WriteJSON(cmd.OutOrStdout(),
						ax.NewEnvelope(cmd.Context(), profileDeletePayload{ProfileID: profileID, Deleted: false}))
				}
			}

			client, err := factory(cmd)
			if err != nil {
				return err
			}

			ran, err := ax.Guard(cmd.Context(), func(ctx context.Context) error {
				_, deleteErr := client.DeleteProfile(ctx, controld.DeleteProfileParams{ProfileID: profileID})
				return deleteErr
			})
			if err != nil {
				return controld.MapError(cmd.Context(), err)
			}
			return ax.WriteJSON(cmd.OutOrStdout(),
				ax.NewEnvelope(cmd.Context(), profileDeletePayload{ProfileID: profileID, Deleted: ran}))
		},
	}
	cmd.Flags().StringVar(&profileID, "profile-id", "", "profile PK to delete (required)")
	return cmd
}
