## Purpose

Read-only visibility into ControlD's own point-of-presence and resolver health,
so an operator diagnosing a resolution problem can distinguish a ControlD-side
outage from a problem in their own configuration.

## ADDED Requirements

### Requirement: Point-Of-Presence Health Listing
The system SHALL provide a command that lists ControlD's points of presence
with the health of each of their services.

#### Scenario: Points of presence returned
- **WHEN** the network command is run
- **THEN** the envelope data contains a list in which each entry carries the
  IATA code, city name, country name, geographic location, and the health
  status of the point's API, DNS, and proxy services

#### Scenario: Empty network list
- **WHEN** the upstream API returns no points of presence
- **THEN** the envelope data contains an empty list rather than a null value,
  and the command exits successfully

### Requirement: Network Health Is Documented As Real-Time
The command SHALL document that its output is real-time health data, and SHALL
NOT tag its fields non-deterministic, so the payload compares byte-stably
against a fixture in tests.

#### Scenario: Help text states volatility
- **WHEN** the network command's help text is displayed
- **THEN** it states that point-of-presence health changes continuously and
  reflects the state at call time

### Requirement: Network Command Is Read-Only
The network command SHALL NOT mutate any remote state and SHALL NOT accept
confirmation or dry-run flags.

#### Scenario: No mutation flags offered
- **WHEN** the network command's help text is displayed
- **THEN** neither a confirmation flag nor a dry-run flag is listed
