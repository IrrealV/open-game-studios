# provider-model-routing Specification

## Purpose

Define the provider/model routing policy constraints that remain as metadata for
the Pi-only wizard. Pi owns authentication and actual model selection; OGS does
not discover, inspect, or inventory providers, models, or accounts.

> Superseded: the earlier OpenCode-only connected-provider discovery and the
> `Solo se muestran los providers conectados a opencode` helper-text SHALL
> scenarios are retired by the authorized Pi-only direction. Archived OpenSpec
> changes are not rewritten.

## Requirements

### Requirement: No provider or model discovery

The system MUST NOT read credentials, auth files, provider lists, or model
inventories. Provider/model state MUST be recorded as not-inspected and
unconfigured, and any user preference MUST remain metadata only.

#### Scenario: Runtime ownership

- GIVEN the wizard reaches the runtime-ownership step
- WHEN it runs
- THEN it MUST NOT list providers or models
- AND it MUST record provider/model state as not-inspected
- AND it MUST NOT invent a snapshot, discovery, or account-probing API.

#### Scenario: No credentials required

- GIVEN no provider credentials are present
- WHEN the wizard runs
- THEN it SHALL continue without fatal error
- AND routing MUST remain in the `not_configured` fallback state.

### Requirement: Balanced preset defaults and scoped overrides

The system MUST keep a declarative routing-preset policy and MAY expose gated
metadata overrides. Overrides MUST reference a provider/model the user owns in Pi
and MUST NOT execute providers.

#### Scenario: Default routing in simple mode

- GIVEN a minimal or recommended setup depth
- WHEN routing metadata is produced
- THEN balanced metadata defaults SHALL be applied
- AND tier defaults SHALL resolve to `not_configured:not_configured`
- AND role/phase overrides SHALL NOT be required.

#### Scenario: Advanced and expert overrides

- GIVEN a full or custom setup depth
- WHEN overrides are supplied as metadata
- THEN role and phase overrides SHALL be recorded as metadata only
- AND no provider API call, model download, or model execution MUST occur.

#### Scenario: Invalid override

- GIVEN an override references a provider/model that is not a user-owned Pi choice
- WHEN override validation runs
- THEN the system MUST reject that override.
