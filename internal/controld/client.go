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

type fileConfig struct {
	APIToken string `json:"api_token"`
}

// NewClient resolves a ControlD API token from --api-token, then
// CONTROLD_API_TOKEN, then the api_token field of a Hujson config file, and
// constructs a client. baseURL overrides the client's target host when
// non-empty — production callers pass "" (the real API); tests pass an
// httptest.Server URL.
func NewClient(ctx context.Context, apiTokenFlag, configPath, baseURL string, getenv func(string) string) (*API, error) {
	token, err := resolveAPIToken(ctx, apiTokenFlag, configPath, getenv)
	if err != nil {
		return nil, err
	}

	opts := []controld.Option{}
	if baseURL != "" {
		opts = append(opts, controld.BaseURL(baseURL))
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
