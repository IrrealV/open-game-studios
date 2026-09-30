# Spec: Core Game Workflow

## Requirement: Core game workflow contract
Open Game Studio MUST expose a machine-readable `core-game-workflow/v1` contract that defines the first real game-intent workflow boundary.

### Scenario: Supports creation, direct phase use, and repair/change handoff
**Given** a generated Game-Studio profile artifact
**When** the core game workflow metadata is inspected
**Then** it MUST reference `core-game-workflow/v1`
**And** it MUST define zero-to-one creation, direct phase invocation, and repair/change handoff as conceptual modes
**And** it MUST define structured mode contracts for those modes, not only display labels
**And** it MUST avoid promising runtime slash commands or execution.

### Scenario: Defines structured mode contracts
**Given** the core game workflow contract
**When** mode contracts are validated
**Then** zero-to-one creation MUST define the conceptual sequence `idea/existing game → concept interview → game concept → game pillars → core loop → player fantasy → mechanics scope → narrative mode → narrative brief/minimal narrative contract → tone and mood → story constraints → gdd slice → human approval gate → downstream`
**And** direct phase invocation MUST define working on one concrete phase without restarting the full flow, with relevant artifacts, human approval, and downstream references
**And** repair/change handoff MUST preserve the triage flow before downstream handoff.

### Scenario: Requires explicit human approval and no auto-approved artifacts
**Given** the core game workflow contract
**When** approval policy metadata is inspected
**Then** every core game artifact and phase output MUST start as draft or pending human approval
**And** every core game artifact and phase output MUST expose `auto_approved: false` or an equivalent machine-readable false value
**And** creative decisions, design decisions, narrative decisions, approval gates, and downstream mutations MUST NOT be auto-approved.

### Scenario: Defines required phase identifiers
**Given** the core game workflow contract
**When** phase identifiers are validated
**Then** it MUST include `game-concept`, `game-pillars`, `core-loop`, `player-fantasy`, `mechanics-brief`, `narrative-brief`, `tone-and-mood`, `story-constraints`, `gdd-slice`, `change-brief`, and `repair-brief`.

### Scenario: Defines narrative modes
**Given** the core game workflow contract
**When** narrative options are validated
**Then** it MUST include `explicit_story`, `environmental_story`, `emergent_story`, `minimal_context`, and `none_or_mechanics_first`.

### Scenario: Defines repair/change handoff flow
**Given** a reported issue or requested change
**When** the repair/change handoff is represented
**Then** it MUST preserve the flow `reported issue → triage → classification → relevant artifacts → repair/change brief → human approval → downstream handoff → verification`.

### Scenario: Defines brief contracts
**Given** a `change-brief` or `repair-brief`
**When** the brief structure is inspected
**Then** it MUST cover observed behavior or requested change, expected behavior or intended outcome, classification, relevant artifact references, affected systems/files if known, proposed fix options, risks, human decision/approval state, acceptance criteria, verification plan, and downstream handoff target.

### Scenario: References downstream workflows without executing them
**Given** the core game workflow contract
**When** downstream references are inspected
**Then** it MUST reference Art Bible, Visual Identity Anchor, Asset Spec, Asset Manifest, Godot handoff, SDD handoff, QA/review checklist, future image/audio workflows, and future model routing by phase/capability
**And** each downstream reference MUST define structured relationship metadata including target, consumed artifacts or constraints, and approval requirement
**And** QA/review checklist MUST verify against approved intent, acceptance criteria, and verification plan
**And** Asset Spec MUST explicitly consume approved intent or approved game intent before asset planning is treated as downstream-ready
**And** future image/audio workflows MUST remain unimplemented placeholders that consume Art Bible, Asset Spec, and GDD constraints
**And** future model routing MUST remain an unimplemented placeholder that consumes phase IDs and capabilities
**And** Claude-native mechanics MUST remain outside this workflow scope and MUST NOT be represented as enabled execution behavior
**And** it MUST remain metadata-only for image generation, audio generation, debugging execution, automatic playtesting, collaboration, Claude-native mechanics, and engine mutation.
