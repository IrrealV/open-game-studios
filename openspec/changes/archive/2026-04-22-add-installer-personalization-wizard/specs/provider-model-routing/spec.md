# provider-model-routing Specification

## Purpose

Define provider discovery and model routing behavior.

## Requirements

### Requirement: Connected-provider discovery only

The system MUST show only OpenCode-connected providers and SHALL display `Solo se muestran los providers conectados a opencode`.

#### Scenario: Providers discovered

- GIVEN OpenCode auth has connected providers
- WHEN the wizard reaches provider discovery
- THEN only connected providers SHALL be listed
- AND the helper text SHALL be visible

#### Scenario: No connected providers available

- GIVEN no connected providers are resolved
- WHEN discovery runs
- THEN the wizard SHALL continue without fatal error
- AND routing MUST remain in fallback state

### Requirement: Balanced preset defaults and scoped overrides

The system MUST use `balanced` as default, SHALL initialize tier defaults from snapshot, and MAY accept gated overrides.

#### Scenario: Default routing in simple mode

- GIVEN complexity `simple`
- WHEN routing is configured
- THEN balanced tier defaults SHALL be applied
- AND role/phase overrides SHALL NOT be required

#### Scenario: Advanced and expert overrides

- GIVEN complexity `advanced` or `expert`
- WHEN overrides are provided
- THEN role overrides SHALL be allowed in advanced/expert
- AND phase overrides SHALL be allowed only in expert

#### Scenario: Invalid override

- GIVEN an override references a model outside snapshot
- WHEN override validation runs
- THEN the system MUST reject that override
