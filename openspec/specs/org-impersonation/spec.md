# org-impersonation Specification

## Purpose

Lets a parent-organization token act as an administrator of one of its child
sub-organizations for the duration of a command, so multi-tenant operators can
drive a child tenant without swapping credentials — with guard rails strong
enough that neither a human nor an agent can act on the wrong tenant silently.

## Requirements

### Requirement: Organization Selection Flag
The system SHALL accept an organization identifier that applies to every
command, resolved from an explicit flag first, then an environment variable,
then a configuration-file field, mirroring the precedence already used for the
API token.

#### Scenario: Flag takes precedence
- **WHEN** an organization identifier is supplied by flag and also by
  environment variable and config file
- **THEN** the flag value is used

#### Scenario: Environment variable used when flag absent
- **WHEN** no organization flag is supplied but the environment variable is set
- **THEN** the environment variable value is used

#### Scenario: Config field used when flag and environment absent
- **WHEN** neither the flag nor the environment variable supplies an
  organization identifier and the config file declares one
- **THEN** the config file value is used

#### Scenario: No organization configured
- **WHEN** no organization identifier is supplied by any source
- **THEN** commands operate against the token's own account and behave exactly
  as they did before this capability existed

### Requirement: Impersonation Header Transmission
When an organization identifier is in effect, every request the system makes to
the upstream API SHALL carry that identifier in the organization-impersonation
request header.

#### Scenario: Header sent on a read
- **WHEN** any read command runs with an organization identifier in effect
- **THEN** the upstream request carries the organization-impersonation header
  set to that identifier

#### Scenario: Header sent on a mutation
- **WHEN** any mutating command runs with an organization identifier in effect
- **THEN** the upstream request carries the organization-impersonation header
  set to that identifier

#### Scenario: Header absent when no organization is configured
- **WHEN** a command runs with no organization identifier in effect
- **THEN** the upstream request carries no organization-impersonation header

### Requirement: Unconditional Organization Echo
Every success envelope SHALL report the organization the command actually acted
against, so a machine consumer can assert which tenant was affected without
inspecting its own configuration.

#### Scenario: Echo present under impersonation
- **WHEN** a command completes successfully with an organization identifier in
  effect
- **THEN** the envelope reports that identifier as the effective organization

#### Scenario: Echo present without impersonation
- **WHEN** a command completes successfully with no organization identifier in
  effect
- **THEN** the envelope still reports the effective organization, using an
  empty value to mean the token's own account

#### Scenario: Echo survives dry run
- **WHEN** a mutating command is run with both the dry-run flag and an
  organization identifier in effect
- **THEN** the envelope reports the dry run and still reports the effective
  organization

### Requirement: All Mutations Are Gated Under Impersonation
When an organization identifier is in effect, every mutating command — creates
and updates as well as deletes — SHALL require confirmation, strengthening the
rule that otherwise gates deletes alone.

#### Scenario: Create gated under impersonation
- **WHEN** a create command is run in machine mode with an organization
  identifier in effect and without the confirmation flag
- **THEN** the command fails with the confirmation-required error and exit
  code, and no upstream mutation is made

#### Scenario: Update gated under impersonation
- **WHEN** an update command is run in machine mode with an organization
  identifier in effect and without the confirmation flag
- **THEN** the command fails with the confirmation-required error and exit
  code, and no upstream mutation is made

#### Scenario: Ungated create without impersonation
- **WHEN** a create command is run in machine mode with no organization
  identifier in effect and without the confirmation flag
- **THEN** the command proceeds, preserving the behavior established before
  this capability existed

#### Scenario: Confirmation satisfied
- **WHEN** a mutating command is run with an organization identifier in effect
  and the confirmation flag set
- **THEN** the mutation proceeds and the envelope reports the effective
  organization

### Requirement: Impersonation Authorization Failures Surface Unchanged
An organization identifier the token is not entitled to act for SHALL surface
the upstream authorization failure through the existing error mapping, without
being reinterpreted or masked.

#### Scenario: Unentitled organization
- **WHEN** a command runs with an organization identifier the token may not act
  for and the upstream API rejects the request as forbidden
- **THEN** the command fails with the mapped authorization error and its
  existing exit code, and writes no success envelope
