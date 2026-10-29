// Package controld adapts baptistecdr/controld-go for controldctl. Files in
// this package import "github.com/baptistecdr/controld-go" directly and
// reference it as controld.X: that's legal even though this package is also
// named controld, because a Go file never self-qualifies its own package's
// members with the package name — controld.X here can only resolve to the
// import. Every other package in this module imports only this package
// (internal/controld) and never the upstream library directly.
package controld

import (
	"context"
	"fmt"
	"net/http"

	"github.com/baptistecdr/controld-go"
	"github.com/rshade/ax-go"
)

// API is controldctl's handle to the ControlD API client. Re-exported so
// package main never imports the upstream library directly.
type API = controld.API

// IntBool and DoType (with its Block/Bypass/Spoof/Redirect values) are
// re-exported because CreateProfileCustomRuleParams, UpdateProfileServiceParams,
// and friends use them directly.
type IntBool = controld.IntBool

type DoType = controld.DoType

const (
	Block    = controld.Block
	Bypass   = controld.Bypass
	Spoof    = controld.Spoof
	Redirect = controld.Redirect
)

// Device-related re-exports for the devices command.
type Device = controld.Device
type CreateDeviceParams = controld.CreateDeviceParams
type UpdateDeviceParams = controld.UpdateDeviceParams
type DeleteDeviceParams = controld.DeleteDeviceParams
type DeviceTypes = controld.DeviceTypes
type IconName = controld.IconName

// A type alias re-exports the type but NOT its package-level constants — Go
// constants aren't attached to a type in a way aliasing carries forward, so
// devices.go's --icon default needs this re-exported explicitly.
const DesktopLinux = controld.DesktopLinux

// Profile-related re-exports for the profiles command.
type Profile = controld.Profile
type CreateProfileParams = controld.CreateProfileParams
type UpdateProfileParams = controld.UpdateProfileParams
type DeleteProfileParams = controld.DeleteProfileParams
type ProfilesOption = controld.ProfilesOption
type UpdateProfilesOption = controld.UpdateProfilesOption

// Filter-related re-exports for the profiles filters command.
type Filter = controld.Filter
type ListProfileFiltersParams = controld.ListProfileFiltersParams
type UpdateProfileFilterParams = controld.UpdateProfileFilterParams
type FilterLevel = controld.FilterLevel
type FilterResolvers = controld.FilterResolvers
type Opt = controld.Opt

// Service-related re-exports for the profiles services command.
type ProfileService = controld.ProfileService
type ListProfileServicesParams = controld.ListProfileServicesParams
type UpdateProfileServiceParams = controld.UpdateProfileServiceParams
type Action = controld.Action

// Custom rule-related re-exports for the profiles rules command.
type Rule = controld.Rule
type CustomRule = controld.CustomRule
type ListProfileCustomRulesParams = controld.ListProfileCustomRulesParams
type CreateProfileCustomRuleParams = controld.CreateProfileCustomRuleParams
type UpdateProfileCustomRuleParams = controld.UpdateProfileCustomRuleParams
type DeleteProfileCustomRuleParams = controld.DeleteProfileCustomRuleParams

// Rule folder-related re-exports for the profiles folders command.
type Group = controld.Group
type GroupAction = controld.GroupAction
type ListProfileRuleFoldersParams = controld.ListProfileRuleFoldersParams
type CreateProfileRuleFolderParams = controld.CreateProfileRuleFolderParams
type UpdateProfileRuleFolderParams = controld.UpdateProfileRuleFolderParams
type DeleteProfileRuleFolderParams = controld.DeleteProfileRuleFolderParams

// Account and network re-exports for the users, ip, and network commands.
// Date and UnixTime are the vendor's own JSON codecs for ControlD's
// date-only and Unix-seconds wire formats; they are re-exported so payload
// wrappers in package main can hold them without importing the library.
type User = controld.User
type Date = controld.Date
type UnixTime = controld.UnixTime
type IP = controld.IP
type Network = controld.Network

// Billing re-exports for the billing command.
type Payment = controld.Payment
type Subscription = controld.Subscription
type Product = controld.Product
type BillingProduct = controld.BillingProduct
type PricePoint = controld.PricePoint
type ProductSubscription = controld.ProductSubscription

// Known-IP re-exports for the access command.
type KnownIP = controld.KnownIP
type ListKnownIPsParams = controld.ListKnownIPsParams
type LearnNewIPsParams = controld.LearnNewIPsParams
type DeleteLearnedIPsParams = controld.DeleteLearnedIPsParams

type fileConfig struct {
	APIToken string `json:"api_token"`
	OrgID    string `json:"org_id"`
}

// orgHeader is ControlD's cross-tenant impersonation header: a
// parent-organization token that sends it acts as an administrator of the
// named sub-organization for that request.
const orgHeader = "X-Force-Org-Id"

// NewClient resolves a ControlD API token from --api-token, then
// CONTROLD_API_TOKEN, then the api_token field of a Hujson config file, and
// constructs a client. baseURL overrides the client's target host when
// non-empty — production callers pass "" (the real API); tests pass an
// httptest.Server URL.
//
// orgID is the already-resolved effective organization (see ResolveOrgID),
// not a raw flag value: the caller resolves it once per run so the envelope
// echo and this header can never disagree. Empty means the token's own
// account, and no impersonation header is sent at all.
func NewClient(ctx context.Context, apiTokenFlag, orgID, configPath, baseURL string, getenv func(string) string) (*API, error) {
	token, err := resolveAPIToken(ctx, apiTokenFlag, configPath, getenv)
	if err != nil {
		return nil, err
	}

	opts := []controld.Option{}
	if baseURL != "" {
		opts = append(opts, controld.BaseURL(baseURL))
	}
	if orgID != "" {
		opts = append(opts, controld.Headers(http.Header{orgHeader: {orgID}}))
	}

	client, err := controld.New(token, opts...)
	if err != nil {
		return nil, fmt.Errorf("construct ControlD client: %w", err)
	}
	return client, nil
}

func resolveAPIToken(ctx context.Context, apiTokenFlag, configPath string, getenv func(string) string) (string, error) {
	if apiTokenFlag != "" {
		return apiTokenFlag, nil
	}
	if token := getenv("CONTROLD_API_TOKEN"); token != "" {
		return token, nil
	}
	if configPath != "" {
		var cfg fileConfig
		if err := ax.ParseConfigFile(ctx, configPath, &cfg); err != nil {
			return "", fmt.Errorf("parse config: %w", err)
		}
		if cfg.APIToken != "" {
			return cfg.APIToken, nil
		}
	}
	return "", ax.NewError(ctx, "controld_no_api_token",
		"no ControlD API token found: pass --api-token, set CONTROLD_API_TOKEN, or set api_token in --config",
		ax.WithErrorExitCode(ax.ExitAuth),
	)
}

// ResolveOrgID resolves the organization controldctl acts as, mirroring
// resolveAPIToken's precedence: --org-id, then CONTROLD_ORG_ID, then the
// org_id field of the Hujson config file. Unlike a missing token, a missing
// organization is not an error — an empty result means the token's own
// account.
func ResolveOrgID(ctx context.Context, orgIDFlag, configPath string, getenv func(string) string) (string, error) {
	if orgIDFlag != "" {
		return orgIDFlag, nil
	}
	if orgID := getenv("CONTROLD_ORG_ID"); orgID != "" {
		return orgID, nil
	}
	if configPath != "" {
		var cfg fileConfig
		if err := ax.ParseConfigFile(ctx, configPath, &cfg); err != nil {
			return "", fmt.Errorf("parse config: %w", err)
		}
		return cfg.OrgID, nil
	}
	return "", nil
}
