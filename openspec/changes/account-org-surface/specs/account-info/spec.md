## Purpose

Read-only visibility into the ControlD account behind the configured API token,
and into the egress IP address that ControlD observes for the caller, so an
operator or agent can confirm which account and which network vantage point a
session is actually operating from before making changes.

## ADDED Requirements

### Requirement: Account Record Retrieval
The system SHALL provide a command that retrieves the account record for the
authenticated token and emits it as a success envelope on stdout.

#### Scenario: Account record returned
- **WHEN** the account command is run with a valid token
- **THEN** the envelope data contains the account identifier, email, account
  status, signup date, last-active time, two-factor status, proxy-access
  status, resolver identifier, and analytics storage endpoint

#### Scenario: Account field keys are lowercase
- **WHEN** the account command emits its envelope
- **THEN** every key in the data payload is lowercase and snake_case,
  regardless of the casing the upstream API used

#### Scenario: Last-active time is marked non-deterministic
- **WHEN** the account command's schema is inspected
- **THEN** the last-active field is reported as non-deterministic and the
  account identifier is not, because the identifier is assigned once per
  account and does not change between calls

#### Scenario: Missing token
- **WHEN** the account command is run with no token available from flag,
  environment, or config file
- **THEN** the command fails with the existing authentication error and exit
  code, and writes no success envelope

### Requirement: Egress IP Retrieval
The system SHALL provide a command that reports the caller's egress IP address
as ControlD observes it, together with the network metadata ControlD attaches
to that address.

#### Scenario: Egress IP returned
- **WHEN** the egress-IP command is run
- **THEN** the envelope data contains the observed IP address, its type,
  owning organization, country, handler, and the serving point of presence

#### Scenario: Environmental output is documented, not tagged
- **WHEN** the egress-IP command's help text is displayed
- **THEN** it states that the output is point-in-time environmental state that
  varies with the caller's network location
- **AND** no field is tagged non-deterministic, so the payload compares
  byte-stably against a fixture in tests

### Requirement: Account Commands Are Read-Only
Account-info commands SHALL NOT mutate any remote state and SHALL NOT accept
confirmation or dry-run flags.

#### Scenario: No mutation flags offered
- **WHEN** an account-info command's help text is displayed
- **THEN** neither a confirmation flag nor a dry-run flag is listed
