package main

import (
	"context"

	"github.com/spf13/cobra"

	"github.com/rshade/ax-go"

	"github.com/rshade/controld-go-mcp/internal/controld"
)

type servicePayload struct {
	PK             string          `json:"pk"`
	Name           string          `json:"name"`
	Category       string          `json:"category"`
	UnlockLocation string          `json:"unlock_location"`
	Locations      []string        `json:"locations,omitempty"`
	Action         controld.Action `json:"action"`
	Warning        *string         `json:"warning,omitempty"`
}

func toServicePayload(s controld.ProfileService) servicePayload {
	return servicePayload{
		PK:             s.PK,
		Name:           s.Name,
		Category:       s.Category,
		UnlockLocation: s.UnlockLocation,
		Locations:      s.Locations,
		Action:         s.Action,
		Warning:        s.Warning,
	}
}

func toServicePayloads(services []controld.ProfileService) []servicePayload {
	payloads := make([]servicePayload, 0, len(services))
	for _, s := range services {
		payloads = append(payloads, toServicePayload(s))
	}
	return payloads
}

type servicesListPayload struct {
	Services []servicePayload `json:"services"`
}

func newProfilesServicesCommand(factory clientFactory) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "services",
		Short: "Manage a profile's service (app/site) rules",
	}
	cmd.AddCommand(newProfilesServicesListCommand(factory))
	cmd.AddCommand(newProfilesServicesUpdateCommand(factory))
	return cmd
}

func newProfilesServicesListCommand(factory clientFactory) *cobra.Command {
	var profileID string

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List a profile's services",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if profileID == "" {
				return ax.NewError(cmd.Context(), "validation_error", "--profile-id is required",
					ax.WithErrorExitCode(ax.ExitValidation))
			}
			client, err := factory(cmd)
			if err != nil {
				return err
			}
			services, err := client.ListProfileServices(cmd.Context(), controld.ListProfileServicesParams{ProfileID: profileID})
			if err != nil {
				return controld.MapError(cmd.Context(), err)
			}
			payload := servicesListPayload{Services: toServicePayloads(services)}
			return ax.WriteJSON(cmd.OutOrStdout(), ax.NewEnvelope(cmd.Context(), payload))
		},
	}
	cmd.Flags().StringVar(&profileID, "profile-id", "", "profile PK (required)")
	ax.WithNonDeterministicFields[servicesListPayload](cmd)
	return cmd
}

type serviceUpdatePayload struct {
	ProfileID string `json:"profile_id"`
	Service   string `json:"service"`
	Updated   bool   `json:"updated"`
}

func newProfilesServicesUpdateCommand(factory clientFactory) *cobra.Command {
	var profileID, service string
	var do int
	var enabled bool

	cmd := &cobra.Command{
		Use:   "update",
		Short: "Set the block/bypass/spoof/redirect action for a service on a profile",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if profileID == "" || service == "" {
				return ax.NewError(cmd.Context(), "validation_error", "--profile-id and --service are required",
					ax.WithErrorExitCode(ax.ExitValidation))
			}
			client, err := factory(cmd)
			if err != nil {
				return err
			}

			params := controld.UpdateProfileServiceParams{
				ProfileID: profileID,
				Service:   service,
				Do:        controld.DoType(do),
				Status:    controld.IntBool(enabled),
			}
			ran, err := ax.Guard(cmd.Context(), func(ctx context.Context) error {
				_, updateErr := client.UpdateProfileService(ctx, params)
				return updateErr
			})
			if err != nil {
				return controld.MapError(cmd.Context(), err)
			}
			return ax.WriteJSON(cmd.OutOrStdout(),
				ax.NewEnvelope(cmd.Context(), serviceUpdatePayload{ProfileID: profileID, Service: service, Updated: ran}))
		},
	}
	cmd.Flags().StringVar(&profileID, "profile-id", "", "profile PK (required)")
	cmd.Flags().StringVar(&service, "service", "", "service PK, e.g. netflix (required)")
	cmd.Flags().IntVar(&do, "do", int(controld.Block), "action: 0=block, 1=bypass, 2=spoof, 3=redirect")
	cmd.Flags().BoolVar(&enabled, "enabled", true, "enable (true) or disable (false) the rule")
	ax.WithNonDeterministicFields[serviceUpdatePayload](cmd)
	return cmd
}
