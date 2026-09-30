# installer-wizard Specification

## Purpose

Define OpenCode-only wizard behavior for one profile concept.

## Requirements

### Requirement: 14-step guided flow

The system MUST run a 14-step sequence (0..14), ending with generation and smoke validation.

#### Scenario: Full completed flow

- GIVEN `game-studio wizard` starts
- WHEN the user confirms generation
- THEN steps 0..14 SHALL execute in order
- AND step 14 SHALL complete runtime smoke validation before reporting success

### Requirement: Start-from-scratch and profile renaming

The system SHALL support `scratch`, `existing`, and `import`, and MUST allow profile renaming.

#### Scenario: Scratch without existing repo/auth

- GIVEN `starting_point=scratch`
- WHEN provider auth is missing
- THEN the wizard MUST continue with an empty provider snapshot and warning

#### Scenario: User-controlled profile name

- GIVEN default profile name `Game-Studio`
- WHEN the user enters another valid name
- THEN generated profile artifacts SHALL use that name

### Requirement: Resource selection separates MCPs

The system MUST collect connectors, packs, and tools in one step, and SHALL configure MCP assistants separately.

#### Scenario: Distinct selection stages

- GIVEN resource selection is active
- WHEN selecting connectors/packs/tools
- THEN MCP choices MUST NOT be mixed into that selection
- AND required MCP assistants SHALL be configured in the MCP step
