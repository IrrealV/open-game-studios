# Delta for installer-wizard

## MODIFIED Requirements

### Requirement: Optional summary

The wizard MUST show selected optionals, asset-pipeline intent, selected visual adapter metadata, `visual-workflow/v1` defaults/readiness metadata, and consent states in summaries/final artifacts without implying installation, generation, conversion, approval, or execution.
(Previously: summaries surfaced optionals, asset-pipeline intent, adapter metadata, and consent states without visual-workflow readiness metadata.)

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
