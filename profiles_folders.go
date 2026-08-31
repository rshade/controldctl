package main

import (
	"context"

	"github.com/spf13/cobra"

	"github.com/rshade/ax-go"

	"github.com/rshade/controld-go-mcp/internal/controld"
)

// groupPayload remaps controld.Group's uppercase "PK" json tag to lowercase.
// Unlike Filter/Service/Rule (stable, caller-known catalog identifiers), a
// folder's PK is server-generated fresh on every create, so it is tagged
// nondeterministic the same way devicePayload.PK and profilePayload.PK are.
// controld.GroupAction needs no wrapper of its own: its Status/Do fields
// already use all-lowercase json tags.
type groupPayload struct {
	PK     int                  `json:"pk"    ax:"nondeterministic"`
	Group  string               `json:"group"`
	Action controld.GroupAction `json:"action"`
	Count  int                  `json:"count"`
}

func toGroupPayload(g controld.Group) groupPayload {
	return groupPayload{PK: g.PK, Group: g.Group, Action: g.Action, Count: g.Count}
}

func toGroupPayloads(groups []controld.Group) []groupPayload {
	payloads := make([]groupPayload, 0, len(groups))
	for _, g := range groups {
		payloads = append(payloads, toGroupPayload(g))
	}
	return payloads
}

type foldersListPayload struct {
	Groups []groupPayload `json:"groups"`
}

type folderDeletePayload struct {
	ProfileID string `json:"profile_id"`
	FolderID  string `json:"folder_id"`
	Deleted   bool   `json:"deleted"`
}

func newProfilesFoldersCommand(factory clientFactory) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "folders",
		Short: "Manage a profile's custom-rule folders",
	}
	cmd.AddCommand(newProfilesFoldersListCommand(factory))
	cmd.AddCommand(newProfilesFoldersCreateCommand(factory))
	cmd.AddCommand(newProfilesFoldersUpdateCommand(factory))
	cmd.AddCommand(newProfilesFoldersDeleteCommand(factory))
	return cmd
}

func newProfilesFoldersListCommand(factory clientFactory) *cobra.Command {
	var profileID string

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List a profile's rule folders",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if profileID == "" {
				return ax.NewError(cmd.Context(), "validation_error", "--profile-id is required",
					ax.WithErrorExitCode(ax.ExitValidation))
			}
			client, err := factory(cmd)
			if err != nil {
				return err
			}
			groups, err := client.ListProfileRuleFolders(cmd.Context(), controld.ListProfileRuleFoldersParams{ProfileID: profileID})
			if err != nil {
				return controld.MapError(cmd.Context(), err)
			}
			payload := foldersListPayload{Groups: toGroupPayloads(groups)}
			return ax.WriteJSON(cmd.OutOrStdout(), ax.NewEnvelope(cmd.Context(), payload))
		},
	}
	cmd.Flags().StringVar(&profileID, "profile-id", "", "profile PK (required)")
	ax.WithNonDeterministicFields[foldersListPayload](cmd)
	return cmd
}

func newProfilesFoldersCreateCommand(factory clientFactory) *cobra.Command {
	var profileID, name string
	var do int
	var enabled bool

	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a rule folder",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if profileID == "" || name == "" {
				return ax.NewError(cmd.Context(), "validation_error", "--profile-id and --name are required",
					ax.WithErrorExitCode(ax.ExitValidation))
			}
			client, err := factory(cmd)
			if err != nil {
				return err
			}

			doVal := controld.DoType(do)
			statusVal := controld.IntBool(enabled)
			params := controld.CreateProfileRuleFolderParams{
				ProfileID: profileID,
				Name:      name,
				Do:        &doVal,
				Status:    &statusVal,
			}

			var result []controld.Group
			ran, err := ax.Guard(cmd.Context(), func(ctx context.Context) error {
				var createErr error
				result, createErr = client.CreateProfileRuleFolder(ctx, params)
				return createErr
			})
			if err != nil {
				return controld.MapError(cmd.Context(), err)
			}
			var payload foldersListPayload
			if ran {
				payload = foldersListPayload{Groups: toGroupPayloads(result)}
			}
			return ax.WriteJSON(cmd.OutOrStdout(), ax.NewEnvelope(cmd.Context(), payload))
		},
	}
	cmd.Flags().StringVar(&profileID, "profile-id", "", "profile PK (required)")
	cmd.Flags().StringVar(&name, "name", "", "folder name (required)")
	cmd.Flags().IntVar(&do, "do", int(controld.Block), "default action for rules in this folder: 0=block, 1=bypass, 2=spoof, 3=redirect")
	cmd.Flags().BoolVar(&enabled, "enabled", true, "enable (true) or disable (false) the folder")
	ax.WithNonDeterministicFields[foldersListPayload](cmd)
	return cmd
}

func newProfilesFoldersUpdateCommand(factory clientFactory) *cobra.Command {
	var profileID, folderID string
	var do int
	var enabled bool

	cmd := &cobra.Command{
		Use:   "update",
		Short: "Update a rule folder",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if profileID == "" || folderID == "" {
				return ax.NewError(cmd.Context(), "validation_error", "--profile-id and --folder-id are required",
					ax.WithErrorExitCode(ax.ExitValidation))
			}
			client, err := factory(cmd)
			if err != nil {
				return err
			}

			params := controld.UpdateProfileRuleFolderParams{
				ProfileID: profileID,
				FolderID:  folderID,
			}
			if cmd.Flags().Changed("do") {
				doVal := controld.DoType(do)
				params.Do = &doVal
			}
			if cmd.Flags().Changed("enabled") {
				statusVal := controld.IntBool(enabled)
				params.Status = &statusVal
			}

			var result []controld.Group
			ran, err := ax.Guard(cmd.Context(), func(ctx context.Context) error {
				var updateErr error
				result, updateErr = client.UpdateProfileRuleFolder(ctx, params)
				return updateErr
			})
			if err != nil {
				return controld.MapError(cmd.Context(), err)
			}
			var payload foldersListPayload
			if ran {
				payload = foldersListPayload{Groups: toGroupPayloads(result)}
			}
			return ax.WriteJSON(cmd.OutOrStdout(), ax.NewEnvelope(cmd.Context(), payload))
		},
	}
	cmd.Flags().StringVar(&profileID, "profile-id", "", "profile PK (required)")
	cmd.Flags().StringVar(&folderID, "folder-id", "", "folder ID to update (required)")
	cmd.Flags().IntVar(&do, "do", int(controld.Block), "default action for rules in this folder: 0=block, 1=bypass, 2=spoof, 3=redirect")
	cmd.Flags().BoolVar(&enabled, "enabled", true, "enable (true) or disable (false) the folder")
	ax.WithNonDeterministicFields[foldersListPayload](cmd)
	return cmd
}

func newProfilesFoldersDeleteCommand(factory clientFactory) *cobra.Command {
	var profileID, folderID string

	cmd := &cobra.Command{
		Use:   "delete",
		Short: "Delete a rule folder",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if profileID == "" || folderID == "" {
				return ax.NewError(cmd.Context(), "validation_error", "--profile-id and --folder-id are required",
					ax.WithErrorExitCode(ax.ExitValidation))
			}

			subject := "delete rule folder " + folderID
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
						ax.NewEnvelope(cmd.Context(), folderDeletePayload{ProfileID: profileID, FolderID: folderID, Deleted: false}))
				}
			}

			client, err := factory(cmd)
			if err != nil {
				return err
			}

			ran, err := ax.Guard(cmd.Context(), func(ctx context.Context) error {
				_, deleteErr := client.DeleteProfileRuleFolder(ctx, controld.DeleteProfileRuleFolderParams{
					ProfileID: profileID,
					FolderID:  folderID,
				})
				return deleteErr
			})
			if err != nil {
				return controld.MapError(cmd.Context(), err)
			}
			return ax.WriteJSON(cmd.OutOrStdout(),
				ax.NewEnvelope(cmd.Context(), folderDeletePayload{ProfileID: profileID, FolderID: folderID, Deleted: ran}))
		},
	}
	cmd.Flags().StringVar(&profileID, "profile-id", "", "profile PK (required)")
	cmd.Flags().StringVar(&folderID, "folder-id", "", "folder ID to delete (required)")
	ax.WithNonDeterministicFields[folderDeletePayload](cmd)
	return cmd
}
