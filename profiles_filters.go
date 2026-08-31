package main

import (
	"context"

	"github.com/spf13/cobra"

	"github.com/rshade/ax-go"

	"github.com/rshade/controld-go-mcp/internal/controld"
)

type filterPayload struct {
	PK          string                    `json:"pk"`
	Name        string                    `json:"name"`
	Description string                    `json:"description"`
	Additional  *string                   `json:"additional,omitempty"`
	Sources     []string                  `json:"sources"`
	Levels      []controld.FilterLevel    `json:"levels,omitempty"`
	Status      controld.IntBool          `json:"status"`
	Resolvers   *controld.FilterResolvers `json:"resolvers,omitempty"`
}

func toFilterPayload(f controld.Filter) filterPayload {
	return filterPayload{
		PK:          f.PK,
		Name:        f.Name,
		Description: f.Description,
		Additional:  f.Additional,
		Sources:     f.Sources,
		Levels:      f.Levels,
		Status:      f.Status,
		Resolvers:   f.Resolvers,
	}
}

func toFilterPayloads(filters []controld.Filter) []filterPayload {
	payloads := make([]filterPayload, 0, len(filters))
	for _, f := range filters {
		payloads = append(payloads, toFilterPayload(f))
	}
	return payloads
}

type filtersListPayload struct {
	Filters []filterPayload `json:"filters"`
}

func newProfilesFiltersCommand(factory clientFactory) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "filters",
		Short: "Manage a profile's category filters",
	}
	cmd.AddCommand(newProfilesFiltersListCommand(factory))
	cmd.AddCommand(newProfilesFiltersUpdateCommand(factory))
	return cmd
}

func newProfilesFiltersListCommand(factory clientFactory) *cobra.Command {
	var profileID, source string

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List a profile's filters",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if profileID == "" {
				return ax.NewError(cmd.Context(), "validation_error", "--profile-id is required",
					ax.WithErrorExitCode(ax.ExitValidation))
			}
			if source != "native" && source != "external" {
				return ax.NewError(cmd.Context(), "validation_error", "--source must be native or external",
					ax.WithErrorExitCode(ax.ExitValidation))
			}
			client, err := factory(cmd)
			if err != nil {
				return err
			}

			params := controld.ListProfileFiltersParams{ProfileID: profileID}
			var filters []controld.Filter
			if source == "external" {
				filters, err = client.ListProfileExternalFilters(cmd.Context(), params)
			} else {
				filters, err = client.ListProfileNativeFilters(cmd.Context(), params)
			}
			if err != nil {
				return controld.MapError(cmd.Context(), err)
			}
			payload := filtersListPayload{Filters: toFilterPayloads(filters)}
			return ax.WriteJSON(cmd.OutOrStdout(), ax.NewEnvelope(cmd.Context(), payload))
		},
	}
	cmd.Flags().StringVar(&profileID, "profile-id", "", "profile PK (required)")
	cmd.Flags().StringVar(&source, "source", "native", "native or external")
	return cmd
}

type filterUpdatePayload struct {
	ProfileID string `json:"profile_id"`
	Filter    string `json:"filter"`
	Updated   bool   `json:"updated"`
}

func newProfilesFiltersUpdateCommand(factory clientFactory) *cobra.Command {
	var profileID, filter string
	var enabled bool

	cmd := &cobra.Command{
		Use:   "update",
		Short: "Enable or disable a filter on a profile",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if profileID == "" || filter == "" {
				return ax.NewError(cmd.Context(), "validation_error", "--profile-id and --filter are required",
					ax.WithErrorExitCode(ax.ExitValidation))
			}
			client, err := factory(cmd)
			if err != nil {
				return err
			}

			params := controld.UpdateProfileFilterParams{
				ProfileID: profileID,
				Filter:    filter,
				Status:    controld.IntBool(enabled),
			}
			ran, err := ax.Guard(cmd.Context(), func(ctx context.Context) error {
				_, updateErr := client.UpdateProfileFilter(ctx, params)
				return updateErr
			})
			if err != nil {
				return controld.MapError(cmd.Context(), err)
			}
			return ax.WriteJSON(cmd.OutOrStdout(),
				ax.NewEnvelope(cmd.Context(), filterUpdatePayload{ProfileID: profileID, Filter: filter, Updated: ran}))
		},
	}
	cmd.Flags().StringVar(&profileID, "profile-id", "", "profile PK (required)")
	cmd.Flags().StringVar(&filter, "filter", "", "filter PK, e.g. ads (required)")
	cmd.Flags().BoolVar(&enabled, "enabled", true, "enable (true) or disable (false) the filter")
	return cmd
}
