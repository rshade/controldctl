package main

import (
	"github.com/spf13/cobra"

	"github.com/rshade/ax-go"

	"github.com/rshade/controldctl/internal/controld"
)

func newIPCommand(factory clientFactory) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "ip",
		Short: "Inspect the egress IP ControlD observes for this caller",
	}
	cmd.AddCommand(newIPShowCommand(factory))
	return cmd
}

func newIPShowCommand(factory clientFactory) *cobra.Command {
	// controld.IP already tags every field lowercase snake_case, so it is
	// embedded directly rather than wrapped — the v1 rule is that a casing
	// wrapper exists only where the vendor's own tags are wrong.
	cmd := &cobra.Command{
		Use:   "show",
		Short: "Show the caller's egress IP as ControlD sees it",
		Long: "Show the caller's egress IP as ControlD sees it, along with the\n" +
			"network metadata ControlD attaches to that address: its type, owning\n" +
			"organization, country, handler, and the point of presence serving it.\n\n" +
			"This output is point-in-time environmental state. It describes the\n" +
			"caller's current network vantage point, not account configuration, and\n" +
			"changes whenever the caller's network location does.\n\n" +
			"Read-only: this command never mutates remote state.",
		RunE: func(cmd *cobra.Command, _ []string) error {
			client, err := factory(cmd)
			if err != nil {
				return err
			}
			ip, err := client.ListIP(cmd.Context())
			if err != nil {
				return controld.MapError(cmd.Context(), err)
			}
			return ax.WriteJSON(cmd.OutOrStdout(), newEnvelope(cmd.Context(), ip))
		},
	}
	ax.WithNonDeterministicFields[controld.IP](cmd)
	return cmd
}
