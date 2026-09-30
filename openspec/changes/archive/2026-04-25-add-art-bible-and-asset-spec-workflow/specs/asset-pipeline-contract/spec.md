# Delta for asset-pipeline-contract

## MODIFIED Requirements

### Requirement: Validation, Approval, and Import Boundaries

The system MUST distinguish visual-workflow readiness, manifest validation status, and human approval state. Adapter production metadata SHALL NOT be marked ready unless the related `visual-workflow/v1` Art Bible is human-approved and the Asset Spec passes read-only audit/readiness. Godot and Blender import metadata SHALL describe target paths, settings, hints, and ownership boundaries only; adapters SHALL own processing/conversion in future changes.
(Previously: validation and human approval were separate, but no upstream Art Bible/Asset Spec readiness gate existed.)

#### Scenario: Awaiting approval
- GIVEN outputs pass metadata validation
- WHEN human review is pending
- THEN approval state MUST remain separate from validation status.

#### Scenario: Import metadata only
- GIVEN Godot or Blender import targets are declared
- WHEN metadata is generated
- THEN no project files SHALL be mutated.

#### Scenario: Visual workflow gate blocks adapter readiness
- GIVEN an asset lacks approved Art Bible or passing Asset Spec audit
- WHEN adapter production metadata is evaluated
- THEN it MUST NOT be marked ready.
