package main

import (
	"context"

	"github.com/spf13/cobra"

	"github.com/rshade/ax-go"

	"github.com/rshade/controldctl/internal/controld"
)

type profilePayload struct {
	PK        string `json:"pk"         ax:"nondeterministic"`
	Name      string `json:"name"`
	UpdatedAt int64  `json:"updated_at" ax:"nondeterministic"`
}

func toProfilePayload(p controld.Profile) profilePayload {
	return profilePayload{PK: p.PK, Name: p.Name, UpdatedAt: p.Updated.Unix()}
}

type profilesListPayload struct {
	Profiles []profilePayload `json:"profiles"`
}

func toProfilesListPayload(profiles []controld.Profile) profilesListPayload {
	payload := profilesListPayload{Profiles: make([]profilePayload, 0, len(profiles))}
	for _, p := range profiles {
		payload.Profiles = append(payload.Profiles, toProfilePayload(p))
	}
	return payload
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
	cmd.AddCommand(newProfilesFiltersCommand(factory))
	cmd.AddCommand(newProfilesServicesCommand(factory))
	cmd.AddCommand(newProfilesRulesCommand(factory))
	cmd.AddCommand(newProfilesFoldersCommand(factory))
	return cmd
}

func newProfilesListCommand(factory clientFactory) *cobra.Command {
	cmd := &cobra.Command{
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
			return ax.WriteJSON(cmd.OutOrStdout(), ax.NewEnvelope(cmd.Context(), toProfilesListPayload(profiles)))
		},
	}
	ax.WithNonDeterministicFields[profilesListPayload](cmd)
	return cmd
}

func newProfilesCreateCommand(factory clientFactory) *cobra.Command {
	var name, cloneProfileID string

	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a profile",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if name == "" {
				return validationError(cmd, "--name is required",
					"pass --name with the profile name")
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
			var payload *profilesListPayload
			if ran {
				profiles := toProfilesListPayload(result)
				payload = &profiles
			}
			return ax.WriteJSON(cmd.OutOrStdout(), ax.NewEnvelope(cmd.Context(), payload))
		},
	}
	cmd.Flags().StringVar(&name, "name", "", "profile name (required)")
	cmd.Flags().StringVar(&cloneProfileID, "clone-profile-id", "", "existing profile PK to clone from")
	ax.WithNonDeterministicFields[profilesListPayload](cmd)
	return cmd
}

func newProfilesUpdateCommand(factory clientFactory) *cobra.Command {
	var profileID, name, lockMessage, password string
	var disableTTL int
	var lockStatus bool

	cmd := &cobra.Command{
		Use:   "update",
		Short: "Update a profile",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if profileID == "" {
				return validationError(cmd, "--profile-id is required",
					"pass --profile-id with the profile PK")
			}
			client, err := factory(cmd)
			if err != nil {
				return err
			}

			params := controld.UpdateProfileParams{ProfileID: profileID}
			if name != "" {
				params.Name = &name
			}
			if cmd.Flags().Changed("disable-ttl") {
				params.DisableTTL = &disableTTL
			}
			if cmd.Flags().Changed("lock-status") {
				v := controld.IntBool(lockStatus)
				params.LockStatus = &v
			}
			if lockMessage != "" {
				params.LockMessage = &lockMessage
			}
			if password != "" {
				params.Password = &password
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
			var payload *profilesListPayload
			if ran {
				profiles := toProfilesListPayload(result)
				payload = &profiles
			}
			return ax.WriteJSON(cmd.OutOrStdout(), ax.NewEnvelope(cmd.Context(), payload))
		},
	}
	cmd.Flags().StringVar(&profileID, "profile-id", "", "profile PK to update (required)")
	cmd.Flags().StringVar(&name, "name", "", "new profile name")
	cmd.Flags().IntVar(&disableTTL, "disable-ttl", 0, "disable (1) or keep (0) TTL on filtered responses")
	cmd.Flags().BoolVar(&lockStatus, "lock-status", false, "lock (true) or unlock (false) the profile")
	cmd.Flags().StringVar(&lockMessage, "lock-message", "", "message shown when the profile is locked")
	cmd.Flags().StringVar(&password, "password", "", "password required to unlock the profile")
	ax.WithNonDeterministicFields[profilesListPayload](cmd)
	return cmd
}

func newProfilesDeleteCommand(factory clientFactory) *cobra.Command {
	var profileID string

	cmd := &cobra.Command{
		Use:   "delete",
		Short: "Delete a profile",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if profileID == "" {
				return validationError(cmd, "--profile-id is required",
					"pass --profile-id with the profile PK")
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
	ax.WithNonDeterministicFields[profileDeletePayload](cmd)
	return cmd
}
