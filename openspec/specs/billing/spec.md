# billing Specification

## Purpose

Read-only visibility into the account's payment history, active subscriptions,
and the products active on it, so an operator can reconcile
ControlD spend and entitlement without opening the dashboard.

## Requirements

### Requirement: Payment History Listing
The system SHALL provide a command that lists the account's payment history as
a success envelope on stdout.

#### Scenario: Payments returned
- **WHEN** the billing payments command is run
- **THEN** the envelope data contains the account's payment records

#### Scenario: No payment history
- **WHEN** the account has no payment history
- **THEN** the envelope data contains an empty list rather than a null value,
  and the command exits successfully

### Requirement: Subscription Listing
The system SHALL provide a command that lists the account's subscriptions as a
success envelope on stdout.

#### Scenario: Subscriptions returned
- **WHEN** the billing subscriptions command is run
- **THEN** the envelope data contains the account's subscription records

#### Scenario: No subscriptions
- **WHEN** the account has no subscriptions
- **THEN** the envelope data contains an empty list rather than a null value,
  and the command exits successfully

### Requirement: Active Product Listing
The system SHALL provide a command that lists the products active on the
account, each with its expiry and the subscription that provides it, as a
success envelope on stdout. It does not list products the account could
subscribe to but has not.

#### Scenario: Products returned
- **WHEN** the billing products command is run
- **THEN** the envelope data contains the account's active product records, each with
  its expiry and subscription

### Requirement: Billing Commands Are Read-Only
Billing commands SHALL NOT mutate any remote state and SHALL NOT accept
confirmation or dry-run flags.

#### Scenario: No mutation flags offered
- **WHEN** a billing command's help text is displayed
- **THEN** neither a confirmation flag nor a dry-run flag is listed

### Requirement: Billing Output Sensitivity Is Declared
Because every command is also callable as a tool by an autonomous agent,
billing commands SHALL state in their help text that their output contains
financial account data.

#### Scenario: Help text declares sensitivity
- **WHEN** a billing command's help text is displayed
- **THEN** it states that the output contains financial account information

### Requirement: Undocumented Billing Response Fields Are Tolerated
Because the upstream API publishes no response schema for billing, the system
SHALL emit the fields it recognizes and SHALL NOT fail when the upstream
response carries fields it does not.

#### Scenario: Unrecognized field present
- **WHEN** a billing response contains a field the system does not model
- **THEN** the command still exits successfully and emits the fields it does
  model

#### Scenario: Response body is not parseable
- **WHEN** a billing response cannot be parsed at all
- **THEN** the command fails with a mapped error and its exit code, and writes
  no partial success envelope

### Requirement: Billing Payload Keys Are Normalized
Billing payloads SHALL use lowercase snake_case keys regardless of the casing
used by the upstream API, and timestamp fields SHALL be tagged
non-deterministic while monetary amounts SHALL NOT be.

#### Scenario: Keys are lowercase
- **WHEN** a billing command emits its envelope
- **THEN** every key in the data payload is lowercase and snake_case

#### Scenario: Timestamps tagged, amounts untagged
- **WHEN** a billing command's schema is inspected
- **THEN** timestamp fields are reported as non-deterministic and monetary
  amount fields are not, because a recorded payment amount is a stable
  historical fact
