# installer-wizard Specification

## Purpose

Define the Pi-only staged installer/personalization wizard for one Game-Studio
profile concept. The wizard plans, previews, and, after explicit approval,
installs missing prerequisites, then writes local profile artifacts. It never
inspects provider credentials or models: Pi owns authentication and model
selection, and Gentle Shell owns orchestration.

This Pi-only direction explicitly overrides the earlier OpenCode-only and
fixed-step wizard wording. Archived OpenSpec changes and the frozen historical
follow-up are not rewritten.

## Requirements

### Requirement: Staged flow without a fixed step count

The system MUST run a staged guided sequence that ends with generation and
validation. It MUST NOT be specified as a fixed number of steps and MUST NOT
require any credential, provider, or model inventory.

#### Scenario: Completed

- GIVEN `game-studio wizard` starts
- WHEN the required selections are made and generation is confirmed
- THEN the staged sequence completes and validates the generated artifacts.

#### Scenario: No credential inventory

- GIVEN the wizard reaches the runtime-ownership step
- WHEN it runs
- THEN it MUST NOT read OpenCode or Pi auth files, provider lists, or model inventories
- AND it MUST record provider/model state as not-inspected.

#### Scenario: Deprecated flags

- GIVEN `--complexity` or `--start` are supplied
- WHEN the wizard parses flags
- THEN they MAY map to setup depth/use mode as deprecated aliases
- AND no provider discovery is performed.

### Requirement: Conditional Godot prerequisite

Godot MUST be a required prerequisite only for the active Godot workflow modes;
deferred engine packs and design/visual-only modes MUST NOT require it.

#### Scenario: Godot required

- GIVEN the active engine pack is Godot and the use mode creates, repairs, or hands off Godot work
- WHEN the prerequisite plan is prepared
- THEN the plan MUST mark Godot required.

#### Scenario: Godot not required

- GIVEN a design/narrative-only or visual-artifacts-only mode, or a deferred engine pack
- WHEN the prerequisite plan is prepared
- THEN Godot MUST NOT be planned or marked required.

### Requirement: Full prerequisite plan and fingerprint consent

The wizard MUST prepare a complete prerequisite plan (detected components,
proposed immutable actions, outputs, effects, and a stable fingerprint) before
any install write. Non-interactive installation MUST require the exact freshly
prepared fingerprint.

#### Scenario: Plan-only preview

- GIVEN `--plan-only`
- WHEN the plan is prepared
- THEN the preview MUST print the plan fingerprint and MUST NOT execute installation, config, artifact, or memory writes
- AND readiness probes MAY run.

#### Scenario: Matching approval

- GIVEN a prepared plan and `--non-interactive --approve-plan <fingerprint>`
- WHEN the fingerprint matches
- THEN execution MAY proceed with the approved consent.

#### Scenario: Non-interactive without approval

- GIVEN `--non-interactive` without a matching `--approve-plan`
- WHEN the run starts
- THEN installation MUST NOT be authorized
- AND the wizard MUST require `--plan-only` or a matching `--approve-plan`.

#### Scenario: Interactive informed approval

- GIVEN an interactive run
- WHEN the plan is presented
- THEN the user MUST explicitly approve the displayed fingerprint before execution.

#### Scenario: Decline or blocked plan

- GIVEN the user declines approval or the prepared plan is blocked
- WHEN the run continues
- THEN no planned install or configuration write MUST occur.

### Requirement: Metadata-only distinction

The wizard MUST distinguish prerequisite installation from metadata-only
artifact generation and MUST NOT claim runtime readiness it did not verify.

#### Scenario: Metadata-only writes

- GIVEN `--metadata-only`
- WHEN the run completes
- THEN it MUST write local profile/pack artifacts, workspace config, and the final artifact
- AND it MUST attempt the Engram memory write-through
- AND it MUST NOT claim that prerequisites were installed or that the run was read-only
- AND a missing Engram companion MAY return a partial memory error.

### Requirement: Explicit prerequisite and memory outcomes

The final artifact and report MUST record explicit prerequisite and memory
outcomes instead of implying success.

#### Scenario: Memory outcomes

- GIVEN the Engram write-through
- WHEN it runs
- THEN the memory outcome MUST be one of pending, saved, failed, canceled, or disabled
- AND a failure or cancellation MUST NOT be reported as a completed whole-setup success.

#### Scenario: Partial or failed install

- GIVEN Execute ran
- WHEN any planned step is failed, unverified, blocked, declined, or missing
- THEN the run MUST report an incomplete/partial prerequisite outcome
- AND it MUST NOT print a launch command for an incomplete runtime.

#### Scenario: Cancellation

- GIVEN the run context is cancelled, for example by Ctrl-C
- WHEN the wizard reaches a write boundary
- THEN it MUST stop with an explicit cancellation outcome instead of a false success.

### Requirement: Managed tool availability and preservation

When prerequisites are installed, the wizard MUST report usable launch
information through owned per-user destinations and MUST NOT modify the user's
shell profile, PATH, or unrelated configuration.

#### Scenario: Launch information

- GIVEN a complete verified runtime setup
- WHEN the report is printed
- THEN it MUST print a direct launch command built from the reported Node/Pi/npm/Engram/Godot paths
- AND it MUST NOT edit the ambient shell or PATH.

#### Scenario: Unknown or conflicting install

- GIVEN an unknown or incompatible existing installation, or colliding managed destinations
- WHEN the plan or write boundaries are validated
- THEN the wizard MUST block rather than overwrite unrelated files.

### Requirement: Resource selection separates MCPs

The system MUST keep connectors, packs, tools, optional integrations, and MCP
adapters distinct, and MUST NOT require or implicitly enable any MCP adapter.

#### Scenario: Distinct

- GIVEN selections are active
- WHEN optional integrations and MCP selections are chosen
- THEN neither MUST mix with tools or each other.

#### Scenario: MCP adapters optional

- GIVEN the generated artifacts
- WHEN MCP metadata is inspected
- THEN every MCP adapter entry MUST be optional with `required=false`
- AND memory MUST be described as the Pi-native Engram companion.

### Requirement: Optional summary and consent

The wizard MUST show selected optionals, asset-pipeline intent, selected visual
adapter metadata, `visual-workflow/v1` defaults/readiness metadata, and consent
states in summaries/final artifacts without implying installation, generation,
conversion, approval, or execution.

#### Scenario: Selection-only

- GIVEN `engram-monitor` is selected
- WHEN summarized
- THEN it SHALL appear optional, not installed or started.

#### Scenario: Consent surfaced

- GIVEN `metronous` is selected with consent
- WHEN artifacts generate
- THEN metadata MUST include explicit consent semantics.

#### Scenario: Asset-pipeline preferences surfaced

- GIVEN visual asset-pipeline intent and adapter metadata are selected
- WHEN summaries and final artifacts generate
- THEN they MUST record preferences as metadata only.

#### Scenario: No execution implied

- GIVEN `comfyui-workflows` or `blender-reference-modeling` is selected
- WHEN generation completes
- THEN artifacts MUST NOT claim tools were installed, models downloaded, APIs called, or assets generated.

#### Scenario: Visual workflow defaults surfaced

- GIVEN Art Bible, Asset Spec, or audit defaults are selected
- WHEN summaries and final artifacts generate
- THEN they MUST be shown as metadata/readiness guidance, not approved art or executed production.
