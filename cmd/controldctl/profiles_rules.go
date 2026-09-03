package main

import (
	"context"
	"strings"

	"github.com/spf13/cobra"

	"github.com/rshade/ax-go"

	"github.com/rshade/controldctl/internal/controld"
)

// rulePayload remaps controld.Rule's uppercase "PK" json tag to lowercase.
// controld.CustomRule needs no such wrapper: it is declared as
// `type CustomRule Action`, and Action already uses all-lowercase json tags.
type rulePayload struct {
	PK     string          `json:"pk"`
	Order  int             `json:"order"`
	Group  int             `json:"group"`
	Action controld.Action `json:"action"`
}

func toRulePayload(r controld.Rule) rulePayload {
	return rulePayload{PK: r.PK, Order: r.Order, Group: r.Group, Action: r.Action}
}

func toRulePayloads(rules []controld.Rule) []rulePayload {
	payloads := make([]rulePayload, 0, len(rules))
	for _, r := range rules {
		payloads = append(payloads, toRulePayload(r))
	}
	return payloads
}

type rulesListPayload struct {
	Rules []rulePayload `json:"rules"`
}

type customRulesPayload struct {
	Rules []controld.CustomRule `json:"rules"`
}

type ruleDeletePayload struct {
	ProfileID string `json:"profile_id"`
	Hostname  string `json:"hostname"`
	Deleted   bool   `json:"deleted"`
}

func newProfilesRulesCommand(factory clientFactory) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "rules",
		Short: "Manage a profile's custom domain rules",
	}
	cmd.AddCommand(newProfilesRulesListCommand(factory))
	cmd.AddCommand(newProfilesRulesCreateCommand(factory))
	cmd.AddCommand(newProfilesRulesUpdateCommand(factory))
	cmd.AddCommand(newProfilesRulesDeleteCommand(factory))
	return cmd
}

func newProfilesRulesListCommand(factory clientFactory) *cobra.Command {
	var profileID, folderID string

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List custom rules in a rule folder",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if profileID == "" || folderID == "" {
				return validationError(cmd, "--profile-id and --folder-id are required",
					"pass --profile-id with the profile PK and --folder-id with the folder ID; run 'profiles folders list --profile-id=<id>' to find a folder ID")
			}
			client, err := factory(cmd)
			if err != nil {
				return err
			}
			rules, err := client.ListProfileCustomRules(cmd.Context(), controld.ListProfileCustomRulesParams{
				ProfileID: profileID,
				FolderID:  folderID,
			})
			if err != nil {
				return controld.MapError(cmd.Context(), err)
			}
			payload := rulesListPayload{Rules: toRulePayloads(rules)}
			return ax.WriteJSON(cmd.OutOrStdout(), ax.NewEnvelope(cmd.Context(), payload))
		},
	}
	cmd.Flags().StringVar(&profileID, "profile-id", "", "profile PK (required)")
	cmd.Flags().StringVar(&folderID, "folder-id", "", "rule folder ID (required)")
	ax.WithNonDeterministicFields[rulesListPayload](cmd)
	return cmd
}

func newProfilesRulesCreateCommand(factory clientFactory) *cobra.Command {
	var profileID, hostnames, via, viaV6 string
	var do, group int
	var enabled bool

	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create custom rules for one or more hostnames",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if profileID == "" || hostnames == "" {
				return validationError(cmd, "--profile-id and --hostnames are required",
					"pass --profile-id with the profile PK and --hostnames with comma-separated hostnames")
			}
			if controld.DoType(do) == controld.Redirect && via == "" {
				return validationError(cmd, "--via is required when --do=3 (redirect)",
					"pass --via with the IPv4 redirect target")
			}
			client, err := factory(cmd)
			if err != nil {
				return err
			}

			params := controld.CreateProfileCustomRuleParams{
				ProfileID: profileID,
				Do:        controld.DoType(do),
				Status:    controld.IntBool(enabled),
				Hostnames: strings.Split(hostnames, ","),
			}
			if via != "" {
				params.Via = &via
			}
			if viaV6 != "" {
				params.ViaV6 = &viaV6
			}
			if cmd.Flags().Changed("group") {
				params.Group = &group
			}

			var result []controld.CustomRule
			ran, err := ax.Guard(cmd.Context(), func(ctx context.Context) error {
				var createErr error
				result, createErr = client.CreateProfileCustomRule(ctx, params)
				return createErr
			})
			if err != nil {
				return controld.MapError(cmd.Context(), err)
			}
			var payload *customRulesPayload
			if ran {
				payload = &customRulesPayload{Rules: result}
			}
			return ax.WriteJSON(cmd.OutOrStdout(), ax.NewEnvelope(cmd.Context(), payload))
		},
	}
	cmd.Flags().StringVar(&profileID, "profile-id", "", "profile PK (required)")
	cmd.Flags().StringVar(&hostnames, "hostnames", "", "comma-separated hostnames (required)")
	cmd.Flags().IntVar(&do, "do", int(controld.Block), "action: 0=block, 1=bypass, 2=spoof, 3=redirect")
	cmd.Flags().BoolVar(&enabled, "enabled", true, "enable (true) or disable (false) the rule")
	cmd.Flags().StringVar(&via, "via", "", "IPv4 redirect target (required when --do=3)")
	cmd.Flags().StringVar(&viaV6, "via-v6", "", "IPv6 redirect target (used with --do=3)")
	cmd.Flags().IntVar(&group, "group", 0, "rule folder PK to place the rules in (0 = default folder)")
	ax.WithNonDeterministicFields[customRulesPayload](cmd)
	return cmd
}

func newProfilesRulesUpdateCommand(factory clientFactory) *cobra.Command {
	var profileID, hostnames, via, viaV6 string
	var do, group int
	var enabled bool

	cmd := &cobra.Command{
		Use:   "update",
		Short: "Update custom rules for one or more hostnames",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if profileID == "" || hostnames == "" {
				return validationError(cmd, "--profile-id and --hostnames are required",
					"pass --profile-id with the profile PK and --hostnames with comma-separated hostnames")
			}
			usingRedirectOrFolderFlags := cmd.Flags().Changed("via") || cmd.Flags().Changed("via-v6") || cmd.Flags().Changed("group")
			if usingRedirectOrFolderFlags && (!cmd.Flags().Changed("do") || !cmd.Flags().Changed("enabled")) {
				return validationError(cmd, "--do and --enabled are required when using --via, --via-v6, or --group",
					"pass --do (0=block, 1=bypass, 2=spoof, 3=redirect) and --enabled (true or false) along with --via, --via-v6, and/or --group")
			}
			if controld.DoType(do) == controld.Redirect && via == "" {
				return validationError(cmd, "--via is required when --do=3 (redirect)",
					"pass --via with the IPv4 redirect target")
			}
			client, err := factory(cmd)
			if err != nil {
				return err
			}

			params := controld.UpdateProfileCustomRuleParams{
				ProfileID: profileID,
				Do:        controld.DoType(do),
				Status:    controld.IntBool(enabled),
				Hostnames: strings.Split(hostnames, ","),
			}
			if via != "" {
				params.Via = &via
			}
			if viaV6 != "" {
				params.ViaV6 = &viaV6
			}
			if cmd.Flags().Changed("group") {
				params.Group = &group
			}

			var result []controld.CustomRule
			ran, err := ax.Guard(cmd.Context(), func(ctx context.Context) error {
				var updateErr error
				result, updateErr = client.UpdateProfileCustomRule(ctx, params)
				return updateErr
			})
			if err != nil {
				return controld.MapError(cmd.Context(), err)
			}
			var payload *customRulesPayload
			if ran {
				payload = &customRulesPayload{Rules: result}
			}
			return ax.WriteJSON(cmd.OutOrStdout(), ax.NewEnvelope(cmd.Context(), payload))
		},
	}
	cmd.Flags().StringVar(&profileID, "profile-id", "", "profile PK (required)")
	cmd.Flags().StringVar(&hostnames, "hostnames", "", "comma-separated hostnames (required)")
	cmd.Flags().IntVar(&do, "do", int(controld.Block), "action: 0=block, 1=bypass, 2=spoof, 3=redirect")
	cmd.Flags().BoolVar(&enabled, "enabled", true, "enable (true) or disable (false) the rule")
	cmd.Flags().StringVar(&via, "via", "", "IPv4 redirect target (required when --do=3)")
	cmd.Flags().StringVar(&viaV6, "via-v6", "", "IPv6 redirect target (used with --do=3)")
	cmd.Flags().IntVar(&group, "group", 0, "rule folder PK to place the rules in (0 = default folder)")
	ax.WithNonDeterministicFields[customRulesPayload](cmd)
	return cmd
}

func newProfilesRulesDeleteCommand(factory clientFactory) *cobra.Command {
	var profileID, hostname string

	cmd := &cobra.Command{
		Use:   "delete",
		Short: "Delete the custom rule for a hostname",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if profileID == "" || hostname == "" {
				return validationError(cmd, "--profile-id and --hostname are required",
					"pass --profile-id with the profile PK and --hostname with the rule's hostname")
			}

			subject := "delete custom rule for " + hostname
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
						ax.NewEnvelope(cmd.Context(), ruleDeletePayload{ProfileID: profileID, Hostname: hostname, Deleted: false}))
				}
			}

			client, err := factory(cmd)
			if err != nil {
				return err
			}

			ran, err := ax.Guard(cmd.Context(), func(ctx context.Context) error {
				_, deleteErr := client.DeleteProfileCustomRule(ctx, controld.DeleteProfileCustomRuleParams{
					ProfileID: profileID,
					Hostname:  hostname,
				})
				return deleteErr
			})
			if err != nil {
				return controld.MapError(cmd.Context(), err)
			}
			return ax.WriteJSON(cmd.OutOrStdout(),
				ax.NewEnvelope(cmd.Context(), ruleDeletePayload{ProfileID: profileID, Hostname: hostname, Deleted: ran}))
		},
	}
	cmd.Flags().StringVar(&profileID, "profile-id", "", "profile PK (required)")
	cmd.Flags().StringVar(&hostname, "hostname", "", "hostname to remove the rule for (required)")
	ax.WithNonDeterministicFields[ruleDeletePayload](cmd)
	return cmd
}
