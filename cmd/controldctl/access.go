package main

import (
	"context"
	"fmt"
	"net"

	"github.com/spf13/cobra"

	"github.com/rshade/ax-go"

	"github.com/rshade/controldctl/internal/controld"
)

// knownIPPayload converts KnownIP.Ts to a plain Unix timestamp the same way
// devicePayload converts Device.Ts, and is the only field here ControlD
// regenerates per observation.
type knownIPPayload struct {
	IP       string `json:"ip"`
	LastSeen int64  `json:"last_seen" ax:"nondeterministic"`
	Country  string `json:"country"`
	City     string `json:"city"`
	ISP      string `json:"isp"`
	ASN      int    `json:"asn"`
	ASName   string `json:"as_name"`
}

func toKnownIPPayload(k controld.KnownIP) knownIPPayload {
	return knownIPPayload{
		IP:       k.IP.String(),
		LastSeen: k.Ts.Unix(),
		Country:  k.Country,
		City:     k.City,
		ISP:      k.ISP,
		ASN:      k.Asn,
		ASName:   k.AsName,
	}
}

type accessListPayload struct {
	IPs []knownIPPayload `json:"ips"`
}

type accessLearnPayload struct {
	DeviceID string   `json:"device_id"`
	IPs      []string `json:"ips"`
	Learned  bool     `json:"learned"`
}

type accessDeletePayload struct {
	DeviceID string   `json:"device_id"`
	IPs      []string `json:"ips"`
	Deleted  bool     `json:"deleted"`
}

func newAccessCommand(factory clientFactory) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "access",
		Short: "Manage the known IPs that authorize a device's resolver",
	}
	cmd.AddCommand(newAccessListCommand(factory))
	cmd.AddCommand(newAccessLearnCommand(factory))
	cmd.AddCommand(newAccessDeleteCommand(factory))
	return cmd
}

// parseIPArgs validates every --ips value up front so a malformed address
// fails before any request is made, rather than being rejected upstream after
// the well-formed addresses in the same call have already been applied.
func parseIPArgs(cmd *cobra.Command, values []string) ([]net.IP, []string, error) {
	parsed := make([]net.IP, 0, len(values))
	canonical := make([]string, 0, len(values))
	for _, value := range values {
		ip := net.ParseIP(value)
		if ip == nil {
			return nil, nil, validationError(cmd,
				fmt.Sprintf("%q is not a valid IP address", value),
				"pass --ips with comma-separated IPv4 or IPv6 addresses")
		}
		parsed = append(parsed, ip)
		canonical = append(canonical, ip.String())
	}
	return parsed, canonical, nil
}

func newAccessListCommand(factory clientFactory) *cobra.Command {
	var deviceID string

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List the known IPs recorded for a device",
		Long: "List the IPs ControlD has learned for a device, with the last-seen\n" +
			"time and the geographic and autonomous-system metadata attached to\n" +
			"each.\n\n" +
			"Read-only: this command never mutates remote state.",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if deviceID == "" {
				return validationError(cmd, "--device-id is required",
					"pass --device-id with the device PK")
			}
			client, err := factory(cmd)
			if err != nil {
				return err
			}
			ips, err := client.ListKnownIPs(cmd.Context(), controld.ListKnownIPsParams{DeviceID: deviceID})
			if err != nil {
				return controld.MapError(cmd.Context(), err)
			}
			payload := accessListPayload{IPs: make([]knownIPPayload, 0, len(ips))}
			for _, ip := range ips {
				payload.IPs = append(payload.IPs, toKnownIPPayload(ip))
			}
			return ax.WriteJSON(cmd.OutOrStdout(), newEnvelope(cmd.Context(), payload))
		},
	}
	cmd.Flags().StringVar(&deviceID, "device-id", "", "device PK whose known IPs to list (required)")
	ax.WithNonDeterministicFields[accessListPayload](cmd)
	return cmd
}

func newAccessLearnCommand(factory clientFactory) *cobra.Command {
	var deviceID string
	var ips []string

	cmd := &cobra.Command{
		Use:   "learn",
		Short: "Add IPs to a device's known-IP list",
		Long: "Add one or more IP addresses to a device's known-IP list, authorizing\n" +
			"that device's resolver from those addresses.",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if deviceID == "" {
				return validationError(cmd, "--device-id is required",
					"pass --device-id with the device PK")
			}
			if len(ips) == 0 {
				return validationError(cmd, "--ips is required",
					"pass --ips with one or more comma-separated IP addresses")
			}
			parsed, canonical, err := parseIPArgs(cmd, ips)
			if err != nil {
				return err
			}

			if requireConfirmation(cmd, false) {
				approved, confirmErr := confirmed(cmd,
					fmt.Sprintf("learn IPs %v for device %s", canonical, deviceID))
				if confirmErr != nil {
					return confirmErr
				}
				if !approved {
					return ax.WriteJSON(cmd.OutOrStdout(), newEnvelope(cmd.Context(),
						accessLearnPayload{DeviceID: deviceID, IPs: canonical, Learned: false}))
				}
			}

			client, err := factory(cmd)
			if err != nil {
				return err
			}

			ran, err := ax.Guard(cmd.Context(), func(ctx context.Context) error {
				_, learnErr := client.LearnNewIPs(ctx, controld.LearnNewIPsParams{
					DeviceID: deviceID,
					IPs:      parsed,
				})
				return learnErr
			})
			if err != nil {
				return controld.MapError(cmd.Context(), err)
			}
			return ax.WriteJSON(cmd.OutOrStdout(), newEnvelope(cmd.Context(),
				accessLearnPayload{DeviceID: deviceID, IPs: canonical, Learned: ran}))
		},
	}
	cmd.Flags().StringVar(&deviceID, "device-id", "", "device PK to teach the addresses to (required)")
	cmd.Flags().StringSliceVar(&ips, "ips", nil, "comma-separated IP addresses to learn (required)")
	ax.WithNonDeterministicFields[accessLearnPayload](cmd)
	return cmd
}

func newAccessDeleteCommand(factory clientFactory) *cobra.Command {
	var deviceID string
	var ips []string

	cmd := &cobra.Command{
		Use:   "delete",
		Short: "Remove learned IPs from a device",
		Long: "Remove one or more learned IP addresses from a device.\n\n" +
			"Removing the address a network resolves from cuts off DNS for every\n" +
			"client behind it, so this is gated like any other destructive command.",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if deviceID == "" {
				return validationError(cmd, "--device-id is required",
					"pass --device-id with the device PK")
			}
			if len(ips) == 0 {
				return validationError(cmd, "--ips is required",
					"pass --ips with one or more comma-separated IP addresses")
			}
			parsed, canonical, err := parseIPArgs(cmd, ips)
			if err != nil {
				return err
			}

			if requireConfirmation(cmd, true) {
				approved, confirmErr := confirmed(cmd,
					fmt.Sprintf("delete learned IPs %v from device %s", canonical, deviceID))
				if confirmErr != nil {
					return confirmErr
				}
				if !approved {
					return ax.WriteJSON(cmd.OutOrStdout(), newEnvelope(cmd.Context(),
						accessDeletePayload{DeviceID: deviceID, IPs: canonical, Deleted: false}))
				}
			}

			client, err := factory(cmd)
			if err != nil {
				return err
			}

			ran, err := ax.Guard(cmd.Context(), func(ctx context.Context) error {
				_, deleteErr := client.DeleteLearnedIPs(ctx, controld.DeleteLearnedIPsParams{
					DeviceID: deviceID,
					IPs:      parsed,
				})
				return deleteErr
			})
			if err != nil {
				return controld.MapError(cmd.Context(), err)
			}
			return ax.WriteJSON(cmd.OutOrStdout(), newEnvelope(cmd.Context(),
				accessDeletePayload{DeviceID: deviceID, IPs: canonical, Deleted: ran}))
		},
	}
	cmd.Flags().StringVar(&deviceID, "device-id", "", "device PK to remove the addresses from (required)")
	cmd.Flags().StringSliceVar(&ips, "ips", nil, "comma-separated IP addresses to remove (required)")
	ax.WithNonDeterministicFields[accessDeletePayload](cmd)
	return cmd
}
