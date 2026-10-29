package main

import (
	"github.com/spf13/cobra"

	"github.com/rshade/ax-go"

	"github.com/rshade/controldctl/internal/controld"
)

// userPayload normalizes the account record's casing: the vendor's User tags
// its primary key "PK", and every envelope key this tool emits is lowercase
// snake_case. Only last_active is server-generated per call — the account PK,
// email, signup date and feature flags are stable between calls, so tagging
// them non-deterministic would make the whole payload unassertable.
type userPayload struct {
	PK            string        `json:"pk"`
	Email         string        `json:"email"`
	Status        bool          `json:"status"`
	Date          controld.Date `json:"date"`
	LastActive    int64         `json:"last_active"    ax:"nondeterministic"`
	Twofa         bool          `json:"twofa"`
	ProxyAccess   bool          `json:"proxy_access"`
	StatsEndpoint string        `json:"stats_endpoint"`
	ResolverUID   string        `json:"resolver_uid"`
}

func toUserPayload(u controld.User) userPayload {
	return userPayload{
		PK:            u.PK,
		Email:         u.Email,
		Status:        bool(u.Status),
		Date:          u.Date,
		LastActive:    u.LastActive.Unix(),
		Twofa:         bool(u.Twofa),
		ProxyAccess:   bool(u.ProxyAccess),
		StatsEndpoint: u.StatsEndpoint,
		ResolverUID:   u.ResolverUid,
	}
}

func newUsersCommand(factory clientFactory) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "users",
		Short: "Inspect the ControlD account behind the API token",
	}
	cmd.AddCommand(newUsersShowCommand(factory))
	return cmd
}

func newUsersShowCommand(factory clientFactory) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "show",
		Short: "Show the account record for the authenticated token",
		Long: "Show the account record for the authenticated token: its identifier,\n" +
			"email, status, signup date, last-active time, two-factor and\n" +
			"proxy-access flags, resolver identifier, and analytics storage\n" +
			"endpoint.\n\n" +
			"Read-only: this command never mutates remote state.",
		RunE: func(cmd *cobra.Command, _ []string) error {
			client, err := factory(cmd)
			if err != nil {
				return err
			}
			user, err := client.ListUser(cmd.Context())
			if err != nil {
				return controld.MapError(cmd.Context(), err)
			}
			return ax.WriteJSON(cmd.OutOrStdout(), newEnvelope(cmd.Context(), toUserPayload(user)))
		},
	}
	ax.WithNonDeterministicFields[userPayload](cmd)
	return cmd
}
