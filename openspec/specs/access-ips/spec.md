# access-ips Specification

## Purpose

Management of the known-IP allowlist that authorizes a device's ControlD
resolver, covering listing the IPs ControlD has learned for a device, teaching
it new ones, and removing learned ones — the last of which can cut off DNS for
a whole network and so carries the same confirmation gate as other destructive
commands.

## Requirements

### Requirement: Known IP Listing
The system SHALL provide a command that lists the known IPs recorded for a
single device, identified by a required device identifier.

#### Scenario: Known IPs returned for a device
- **WHEN** the access list command is run with a device identifier
- **THEN** the envelope data contains a list in which each entry carries the IP
  address, last-seen timestamp, country, city, ISP, autonomous system number,
  and autonomous system name

#### Scenario: Device identifier omitted
- **WHEN** the access list command is run without a device identifier
- **THEN** the command fails with a validation error whose actionable fix names
  the required flag, and writes no success envelope

#### Scenario: Last-seen timestamp is marked non-deterministic
- **WHEN** the access list command's schema is inspected
- **THEN** the last-seen timestamp field is reported as non-deterministic

#### Scenario: Device has no known IPs
- **WHEN** the upstream API returns no known IPs for the device
- **THEN** the envelope data contains an empty list rather than a null value,
  and the command exits successfully

### Requirement: Learning New IPs
The system SHALL provide a command that adds one or more IP addresses to a
device's known-IP list, identified by a required device identifier and a
required non-empty set of IP addresses.

#### Scenario: IPs learned
- **WHEN** the access learn command is run with a device identifier and one or
  more IP addresses
- **THEN** the addresses are submitted to the upstream API and the envelope
  data names the device and the addresses that were submitted

#### Scenario: Required inputs omitted
- **WHEN** the access learn command is run without a device identifier or
  without any IP address
- **THEN** the command fails with a validation error whose actionable fix names
  the missing flag, and writes no success envelope

#### Scenario: Malformed IP address rejected before the call
- **WHEN** the access learn command is run with a value that is not a valid IP
  address
- **THEN** the command fails with a validation error naming the offending value
  and makes no upstream request

#### Scenario: Dry run performs no mutation
- **WHEN** the access learn command is run with the dry-run flag
- **THEN** no upstream request is made
- **AND** the envelope reports the dry run and its data is an identity-only
  payload naming the device and addresses a real run would have submitted

### Requirement: Deleting Learned IPs
The system SHALL provide a command that removes one or more learned IP
addresses from a device, identified by a required device identifier and a
required non-empty set of IP addresses.

#### Scenario: Learned IPs deleted after confirmation
- **WHEN** the access delete command is run and the deletion is confirmed
- **THEN** the addresses are removed through the upstream API and the envelope
  data names the device, the addresses, and a past-tense deleted flag

#### Scenario: Confirmation required in machine mode
- **WHEN** the access delete command is run in machine mode without the
  confirmation flag
- **THEN** the command fails with the standard confirmation-required error and
  exit code, and no upstream deletion request is made

#### Scenario: Interactive confirmation declined
- **WHEN** the access delete command prompts interactively and the operator
  answers anything other than yes
- **THEN** no upstream deletion request is made and the command reports that
  the deletion was not performed

#### Scenario: Dry run performs no deletion
- **WHEN** the access delete command is run with the dry-run flag
- **THEN** no upstream request is made
- **AND** the envelope reports the dry run and its data is an identity-only
  payload naming the device and addresses with a false deleted flag
