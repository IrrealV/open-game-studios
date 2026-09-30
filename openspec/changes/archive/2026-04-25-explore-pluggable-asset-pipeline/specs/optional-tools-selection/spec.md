# Delta for optional-tools-selection

## MODIFIED Requirements

### Requirement: Catalog

The system MUST expose stable-ID integrations with label, category, default, hints, diagnostics, consent metadata, and metadata-only asset-adapter capability descriptors. Initial IDs MUST include `engram-monitor`, `metronous`, `comfyui-workflows`, and `blender-reference-modeling`; initial asset-adapter IDs MUST NOT include Hunyuan3D, TripoSR, Stable Fast 3D, TRELLIS.2, or ComfyUI-3D-Pack standalone entries.
(Previously: Catalog exposed initial optional integrations and allowed future asset-adapter categories without naming the initial asset adapter catalog.)

#### Scenario: Initial
- GIVEN the catalog is listed
- WHEN entries render
- THEN both initial non-asset IDs MUST appear outside tools/MCPs.

#### Scenario: Adapter metadata
- GIVEN an asset-adapter entry
- WHEN selected
- THEN no install, clone, build, run, API call, or backend execution SHALL occur.

#### Scenario: Initial visual adapter catalog
- GIVEN asset-adapter catalog metadata is listed
- WHEN entries render
- THEN only `comfyui-workflows` and `blender-reference-modeling` MUST appear.

#### Scenario: Deferred adapter excluded
- GIVEN a deferred 3D adapter is requested as an initial standalone entry
- WHEN the catalog validates
- THEN the entry MUST be excluded before generic unknown-ID handling with future-scope metadata.
