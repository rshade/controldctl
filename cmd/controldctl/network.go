package main

import (
	"github.com/spf13/cobra"

	"github.com/rshade/ax-go"

	"github.com/rshade/controldctl/internal/controld"
)

// networkListPayload embeds controld.Network directly: the vendor already tags
// every point-of-presence field lowercase snake_case.
type networkListPayload struct {
	Network []controld.Network `json:"network"`
}

func newNetworkCommand(factory clientFactory) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "network",
		Short: "Inspect ControlD's own point-of-presence health",
	}
	cmd.AddCommand(newNetworkListCommand(factory))
	return cmd
}

func newNetworkListCommand(factory clientFactory) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List ControlD's points of presence and their service health",
		Long: "List ControlD's points of presence with the IATA code, city, country,\n" +
			"geographic location, and API, DNS and proxy health of each.\n\n" +
			"Point-of-presence health changes continuously: this reports the state\n" +
			"at call time, not a stable inventory. It describes ControlD's own\n" +
			"infrastructure, not this account's configuration or query statistics.\n\n" +
			"Read-only: this command never mutates remote state.",
		RunE: func(cmd *cobra.Command, _ []string) error {
			client, err := factory(cmd)
			if err != nil {
				return err
			}
			network, err := client.ListNetwork(cmd.Context())
			if err != nil {
				return controld.MapError(cmd.Context(), err)
			}
			payload := networkListPayload{Network: make([]controld.Network, 0, len(network))}
			payload.Network = append(payload.Network, network...)
			return ax.WriteJSON(cmd.OutOrStdout(), newEnvelope(cmd.Context(), payload))
		},
	}
	ax.WithNonDeterministicFields[networkListPayload](cmd)
	return cmd
}
