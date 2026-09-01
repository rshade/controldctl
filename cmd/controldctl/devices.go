package main

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/rshade/ax-go"

	"github.com/rshade/controldctl/internal/controld"
)

type devicePayload struct {
	PK          string `json:"pk"           ax:"nondeterministic"`
	DeviceID    string `json:"device_id"    ax:"nondeterministic"`
	Name        string `json:"name"`
	Status      int    `json:"status"`
	ProfilePK   string `json:"profile_pk"`
	ProfileName string `json:"profile_name"`
	CreatedAt   int64  `json:"created_at"   ax:"nondeterministic"`
}

func toDevicePayload(d controld.Device) devicePayload {
	return devicePayload{
		PK:          d.PK,
		DeviceID:    d.DeviceID,
		Name:        d.Name,
		Status:      int(d.Status),
		ProfilePK:   d.Profile.PK,
		ProfileName: d.Profile.Name,
		CreatedAt:   d.Ts.Unix(),
	}
}

type devicesListPayload struct {
	Devices []devicePayload `json:"devices"`
}

type deviceDeletePayload struct {
	DeviceID string `json:"device_id"`
	Deleted  bool   `json:"deleted"`
}

func newDevicesCommand(factory clientFactory) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "devices",
		Short: "Manage ControlD devices (endpoints)",
	}
	cmd.AddCommand(newDevicesListCommand(factory))
	cmd.AddCommand(newDevicesCreateCommand(factory))
	cmd.AddCommand(newDevicesUpdateCommand(factory))
	cmd.AddCommand(newDevicesDeleteCommand(factory))
	cmd.AddCommand(newDevicesTypesCommand(factory))
	return cmd
}

func newDevicesListCommand(factory clientFactory) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List all devices",
		RunE: func(cmd *cobra.Command, _ []string) error {
			client, err := factory(cmd)
			if err != nil {
				return err
			}
			devices, err := client.ListDevices(cmd.Context())
			if err != nil {
				return controld.MapError(cmd.Context(), err)
			}
			payload := devicesListPayload{Devices: make([]devicePayload, 0, len(devices))}
			for _, d := range devices {
				payload.Devices = append(payload.Devices, toDevicePayload(d))
			}
			return ax.WriteJSON(cmd.OutOrStdout(), ax.NewEnvelope(cmd.Context(), payload))
		},
	}
	ax.WithNonDeterministicFields[devicesListPayload](cmd)
	return cmd
}

func newDevicesCreateCommand(factory clientFactory) *cobra.Command {
	var name, profileID, icon string
	var ddnsSubdomain, ddnsExtHost string
	var remapDeviceID, remapClientID string
	var legacyIPv4Status, ddnsStatus, ddnsExtStatus bool

	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a device",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if name == "" || profileID == "" {
				return validationError(cmd, "--name and --profile-id are required",
					"pass --name with the device name and --profile-id with the profile PK")
			}
			client, err := factory(cmd)
			if err != nil {
				return err
			}

			params := controld.CreateDeviceParams{
				Name:      name,
				ProfileID: profileID,
				Icon:      controld.IconName(icon),
			}
			if cmd.Flags().Changed("legacy-ipv4-status") {
				v := controld.IntBool(legacyIPv4Status)
				params.LegacyIPv4Status = &v
			}
			if cmd.Flags().Changed("ddns-status") {
				v := controld.IntBool(ddnsStatus)
				params.DDNSStatus = &v
			}
			if ddnsSubdomain != "" {
				params.DDNSSubdomain = &ddnsSubdomain
			}
			if cmd.Flags().Changed("ddns-ext-status") {
				v := controld.IntBool(ddnsExtStatus)
				params.DDNSExtStatus = &v
			}
			if ddnsExtHost != "" {
				params.DDNSExtHost = &ddnsExtHost
			}
			if remapDeviceID != "" {
				params.RemapDeviceID = &remapDeviceID
			}
			if remapClientID != "" {
				params.RemapClientID = &remapClientID
			}

			var result controld.Device
			ran, err := ax.Guard(cmd.Context(), func(ctx context.Context) error {
				var createErr error
				result, createErr = client.CreateDevice(ctx, params)
				return createErr
			})
			if err != nil {
				return controld.MapError(cmd.Context(), err)
			}
			var payload *devicePayload
			if ran {
				device := toDevicePayload(result)
				payload = &device
			}
			return ax.WriteJSON(cmd.OutOrStdout(), ax.NewEnvelope(cmd.Context(), payload))
		},
	}
	cmd.Flags().StringVar(&name, "name", "", "device name (required)")
	cmd.Flags().StringVar(&profileID, "profile-id", "", "profile PK to assign (required)")
	cmd.Flags().StringVar(&icon, "icon", string(controld.DesktopLinux), "device icon name")
	cmd.Flags().BoolVar(&legacyIPv4Status, "legacy-ipv4-status", false, "enable (true) or disable (false) the legacy IPv4 resolver")
	cmd.Flags().BoolVar(&ddnsStatus, "ddns-status", false, "enable (true) or disable (false) ControlD DDNS")
	cmd.Flags().StringVar(&ddnsSubdomain, "ddns-subdomain", "", "ControlD DDNS subdomain")
	cmd.Flags().BoolVar(&ddnsExtStatus, "ddns-ext-status", false, "enable (true) or disable (false) external DDNS")
	cmd.Flags().StringVar(&ddnsExtHost, "ddns-ext-host", "", "external DDNS hostname")
	cmd.Flags().StringVar(&remapDeviceID, "remap-device-id", "", "device PK to remap this device to")
	cmd.Flags().StringVar(&remapClientID, "remap-client-id", "", "client ID to remap this device to")
	ax.WithNonDeterministicFields[devicePayload](cmd)
	return cmd
}

func newDevicesUpdateCommand(factory clientFactory) *cobra.Command {
	var deviceID, name, profileID string
	var ddnsSubdomain, ddnsExtHost string
	var legacyIPv4Status, ddnsStatus, ddnsExtStatus bool

	cmd := &cobra.Command{
		Use:   "update",
		Short: "Update a device",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if deviceID == "" {
				return validationError(cmd, "--device-id is required",
					"pass --device-id with the device PK")
			}
			client, err := factory(cmd)
			if err != nil {
				return err
			}

			params := controld.UpdateDeviceParams{DeviceID: deviceID}
			if name != "" {
				params.Name = &name
			}
			if profileID != "" {
				params.ProfileID = &profileID
			}
			if cmd.Flags().Changed("legacy-ipv4-status") {
				v := controld.IntBool(legacyIPv4Status)
				params.LegacyIPv4Status = &v
			}
			if cmd.Flags().Changed("ddns-status") {
				v := controld.IntBool(ddnsStatus)
				params.DDNSStatus = &v
			}
			if ddnsSubdomain != "" {
				params.DDNSSubdomain = &ddnsSubdomain
			}
			if cmd.Flags().Changed("ddns-ext-status") {
				v := controld.IntBool(ddnsExtStatus)
				params.DDNSExtStatus = &v
			}
			if ddnsExtHost != "" {
				params.DDNSExtHost = &ddnsExtHost
			}

			var result controld.Device
			ran, err := ax.Guard(cmd.Context(), func(ctx context.Context) error {
				var updateErr error
				result, updateErr = client.UpdateDevice(ctx, params)
				return updateErr
			})
			if err != nil {
				return controld.MapError(cmd.Context(), err)
			}
			var payload *devicePayload
			if ran {
				device := toDevicePayload(result)
				payload = &device
			}
			return ax.WriteJSON(cmd.OutOrStdout(), ax.NewEnvelope(cmd.Context(), payload))
		},
	}
	cmd.Flags().StringVar(&deviceID, "device-id", "", "device PK to update (required)")
	cmd.Flags().StringVar(&name, "name", "", "new device name")
	cmd.Flags().StringVar(&profileID, "profile-id", "", "new profile PK to assign")
	cmd.Flags().BoolVar(&legacyIPv4Status, "legacy-ipv4-status", false, "enable (true) or disable (false) the legacy IPv4 resolver")
	cmd.Flags().BoolVar(&ddnsStatus, "ddns-status", false, "enable (true) or disable (false) ControlD DDNS")
	cmd.Flags().StringVar(&ddnsSubdomain, "ddns-subdomain", "", "ControlD DDNS subdomain")
	cmd.Flags().BoolVar(&ddnsExtStatus, "ddns-ext-status", false, "enable (true) or disable (false) external DDNS")
	cmd.Flags().StringVar(&ddnsExtHost, "ddns-ext-host", "", "external DDNS hostname")
	ax.WithNonDeterministicFields[devicePayload](cmd)
	return cmd
}

func newDevicesDeleteCommand(factory clientFactory) *cobra.Command {
	var deviceID string

	cmd := &cobra.Command{
		Use:   "delete",
		Short: "Delete a device",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if deviceID == "" {
				return validationError(cmd, "--device-id is required",
					"pass --device-id with the device PK")
			}

			subject := fmt.Sprintf("delete device %s", deviceID)
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
						ax.NewEnvelope(cmd.Context(), deviceDeletePayload{DeviceID: deviceID, Deleted: false}))
				}
			}

			client, err := factory(cmd)
			if err != nil {
				return err
			}

			ran, err := ax.Guard(cmd.Context(), func(ctx context.Context) error {
				_, deleteErr := client.DeleteDevice(ctx, controld.DeleteDeviceParams{DeviceID: deviceID})
				return deleteErr
			})
			if err != nil {
				return controld.MapError(cmd.Context(), err)
			}
			return ax.WriteJSON(cmd.OutOrStdout(),
				ax.NewEnvelope(cmd.Context(), deviceDeletePayload{DeviceID: deviceID, Deleted: ran}))
		},
	}
	cmd.Flags().StringVar(&deviceID, "device-id", "", "device PK to delete (required)")
	ax.WithNonDeterministicFields[deviceDeletePayload](cmd)
	return cmd
}

func newDevicesTypesCommand(factory clientFactory) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "types",
		Short: "List available device types and icons",
		RunE: func(cmd *cobra.Command, _ []string) error {
			client, err := factory(cmd)
			if err != nil {
				return err
			}
			types, err := client.ListDeviceType(cmd.Context())
			if err != nil {
				return controld.MapError(cmd.Context(), err)
			}
			return ax.WriteJSON(cmd.OutOrStdout(), ax.NewEnvelope(cmd.Context(), types))
		},
	}
	ax.WithNonDeterministicFields[controld.DeviceTypes](cmd)
	return cmd
}
