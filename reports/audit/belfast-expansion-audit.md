# Belfast Expansion Audit

## Short Summary
Audit of amagi-data's ability to fully generate the belfast-data layout.

## Classification Summary
- Source Files Count: 3110
- Reference Files Count: 3126
- Exact Raw Match: 290
- Match after empty normalisation: 0
- Match after dict-to-list: 313
- Match after both: 1
- Count Mismatch: 2389
- Schema Mismatch: 71
- Belfast Only: 58
- Missing Reference: 48
- Unsupported: 0

## Source Region Coverage
- CN: 624
- EN: 622
- JP: 624
- KR: 628
- TW: 622

## Special Files
- buff_cfg.json: reference_missing
- skill_cfg.json: reference_missing
- build_pools.json: fallback/generated
- build_times.json: fallback/generated
- requisition_ships.json: fallback/generated
- versions.json: fallback/generated

## Recommended Next Implementation Steps
1. Expand main generator to walk region directories and apply matching transforms.
2. Exclude `build_pools.json`, `build_times.json`, `requisition_ships.json` and keep fallback mechanism.
3. Handle `buff_cfg.json` and `skill_cfg.json` using exact transforms.
