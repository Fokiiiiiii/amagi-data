# Belfast Expansion Audit

## Counting Model
- source_region_files_total: 3120
- comparable_source_files_count: 3110
- excluded_source_files_count: 10
- safe_to_promote_count: 3022

## Excluded Source Files
- CN/buffCfg.json [buffCfg handled separately]: These files exist in the source region tree, but are excluded from ordinary comparable region-layout counting because they are handled through special Belfast root files or special audit handling.
- CN/skillCfg.json [skillCfg handled separately]: These files exist in the source region tree, but are excluded from ordinary comparable region-layout counting because they are handled through special Belfast root files or special audit handling.
- EN/buffCfg.json [buffCfg handled separately]: These files exist in the source region tree, but are excluded from ordinary comparable region-layout counting because they are handled through special Belfast root files or special audit handling.
- EN/skillCfg.json [skillCfg handled separately]: These files exist in the source region tree, but are excluded from ordinary comparable region-layout counting because they are handled through special Belfast root files or special audit handling.
- JP/buffCfg.json [buffCfg handled separately]: These files exist in the source region tree, but are excluded from ordinary comparable region-layout counting because they are handled through special Belfast root files or special audit handling.
- JP/skillCfg.json [skillCfg handled separately]: These files exist in the source region tree, but are excluded from ordinary comparable region-layout counting because they are handled through special Belfast root files or special audit handling.
- KR/buffCfg.json [buffCfg handled separately]: These files exist in the source region tree, but are excluded from ordinary comparable region-layout counting because they are handled through special Belfast root files or special audit handling.
- KR/skillCfg.json [skillCfg handled separately]: These files exist in the source region tree, but are excluded from ordinary comparable region-layout counting because they are handled through special Belfast root files or special audit handling.
- TW/buffCfg.json [buffCfg handled separately]: These files exist in the source region tree, but are excluded from ordinary comparable region-layout counting because they are handled through special Belfast root files or special audit handling.
- TW/skillCfg.json [skillCfg handled separately]: These files exist in the source region tree, but are excluded from ordinary comparable region-layout counting because they are handled through special Belfast root files or special audit handling.

## Source Region Coverage
- CN: 624
- EN: 622
- JP: 624
- KR: 628
- TW: 622

## Classification Summary
- exact_raw_match: 290
- match_after_empty_normalization: 0
- match_after_dict_keyed_to_list_by_id: 2721
- match_after_both_transformations: 1
- count_mismatch: 0
- schema_mismatch: 15
- missing_reference: 36
- unsupported: 0
- belfast_only: 48

## Count Mismatch Buckets
- none

## Schema Mismatch Buckets
- field_value_delta
  - file_count: 10
  - status: rejected
  - candidate_rule: narrow field-level adjustments only
  - notes: These files differ by a small number of field values after shape normalization.
  - files:
    - CN/ShareCfg/auto_pilot_template.json
    - CN/ShareCfg/class_upgrade_group.json
    - EN/ShareCfg/auto_pilot_template.json
    - EN/ShareCfg/class_upgrade_group.json
    - JP/ShareCfg/auto_pilot_template.json
    - JP/ShareCfg/class_upgrade_group.json
    - KR/ShareCfg/auto_pilot_template.json
    - KR/ShareCfg/class_upgrade_group.json
    - TW/ShareCfg/auto_pilot_template.json
    - TW/ShareCfg/class_upgrade_group.json
  - representative_files:
    - CN/ShareCfg/auto_pilot_template.json
    - CN/ShareCfg/class_upgrade_group.json
    - EN/ShareCfg/auto_pilot_template.json
- scalar_vs_array
  - file_count: 5
  - status: rejected
  - candidate_rule: wrap scalar fields in singleton arrays
  - notes: These files differ by nested scalar-versus-array shape and have no proven exact promotion rule.
  - files:
    - CN/ShareCfg/guildset.json
    - EN/ShareCfg/guildset.json
    - JP/ShareCfg/guildset.json
    - KR/ShareCfg/guildset.json
    - TW/ShareCfg/guildset.json
  - representative_files:
    - CN/ShareCfg/guildset.json
    - EN/ShareCfg/guildset.json
    - JP/ShareCfg/guildset.json


## Safe To Promote Summary
- Total: 3022
- exact_raw_match: 290
- match_after_empty_normalization: 0
- match_after_dict_keyed_to_list_by_id: 2721
- match_after_both_transformations: 1
- match_after_reference_id_subset: 10
- Examples:
  - CN/GameCfg/buff.json [CN/exact_raw_match]
  - CN/GameCfg/card.json [CN/exact_raw_match]
  - CN/GameCfg/dorm.json [CN/exact_raw_match]
  - CN/GameCfg/dungeon.json [CN/exact_raw_match]
  - CN/GameCfg/skill.json [CN/exact_raw_match]
  - CN/GameCfg/story.json [CN/exact_raw_match]
  - CN/ShareCfg/activity_coloring_template.json [CN/exact_raw_match]
  - CN/ShareCfg/activity_const.json [CN/exact_raw_match]
  - ... 3014 more

## Count Mismatch Summary
- Count: 0
- Examples:

## Schema Mismatch Summary
- Count: 15
- Examples:
  - CN/ShareCfg/auto_pilot_template.json [CN/schema_mismatch]
  - CN/ShareCfg/class_upgrade_group.json [CN/schema_mismatch]
  - CN/ShareCfg/guildset.json [CN/schema_mismatch]
  - EN/ShareCfg/auto_pilot_template.json [EN/schema_mismatch]
  - EN/ShareCfg/class_upgrade_group.json [EN/schema_mismatch]
  - EN/ShareCfg/guildset.json [EN/schema_mismatch]
  - JP/ShareCfg/auto_pilot_template.json [JP/schema_mismatch]
  - JP/ShareCfg/class_upgrade_group.json [JP/schema_mismatch]
  - ... 7 more

## Belfast Only Summary
- Count: 48
- Examples:
  - CN/ShareCfg/battle_nodes_cfg.json: no comparable source file was found
  - CN/ShareCfg/dorm3_d_collect.json: no comparable source file was found
  - CN/ShareCfg/dorm3_d_dolly.json: no comparable source file was found
  - CN/ShareCfg/dorm3_d_recall.json: no comparable source file was found
  - CN/ShareCfg/inform_cfg.json: no comparable source file was found
  - CN/ShareCfg/inform_for_back_yard_theme_template_cfg.json: no comparable source file was found
  - CN/ShareCfg/voice_actor_cn.json: no comparable source file was found
  - CN/ShareCfg/world_sl_gbuff_data.json: no comparable source file was found
  - ... 40 more

## Missing Reference Summary
- Count: 36
- Examples:
  - CN/ShareCfg/BattleNodesCfg.json: no Belfast reference file was found
  - CN/ShareCfg/InformForBackYardThemeTemplateCfg.json: no Belfast reference file was found
  - CN/ShareCfg/dorm3D_collect.json: no Belfast reference file was found
  - CN/ShareCfg/dorm3D_dolly.json: no Belfast reference file was found
  - CN/ShareCfg/informCfg.json: no Belfast reference file was found
  - CN/ShareCfg/voice_actor_CN.json: no Belfast reference file was found
  - CN/ShareCfg/world_SLGbuff_data.json: no Belfast reference file was found
  - EN/ShareCfg/BattleNodesCfg.json: no Belfast reference file was found
  - ... 28 more

## Transform Rule Evidence
- confirmed: `JP/sharecfgdata/ship_data_statistics.json` Full match after dict-keyed records -> id-sorted list and empty object {} -> empty array [] normalization.
- confirmed: `JP/sharecfgdata/weapon_property.json` Full match after dict-keyed records -> id-sorted list and empty object {} -> empty array [] normalization.
- confirmed: `JP/sharecfgdata/equip_data_template.json` Full match after dict-keyed records -> id-sorted list and empty object {} -> empty array [] normalization.
- confirmed: `JP/ShareCfg/ship_skin_template.json` Full match after dict-keyed records -> id-sorted list and empty object {} -> empty array [] normalization.

## Helper Data Notes
- `build_pools.json` [observed]: Currently treated as fallback/generated helper output; exact source fields are not confirmed.
- `build_times.json` [observed]: Currently treated as fallback/generated helper output; exact source fields are not confirmed.
- `requisition_ships.json` [observed]: Currently treated as fallback/generated helper output.
- `versions.json` [hypothesis]: Currently treated as fallback/generated helper output; versions.json generation is not confirmed from public upstream code.

## Special Files
- buff_cfg.json: special root reference from JP/GameCfg/buff.json
- build_pools.json: helper fallback/generated
- build_times.json: helper fallback/generated
- requisition_ships.json: helper fallback/generated
- skill_cfg.json: special root reference from JP/GameCfg/skill.json
- versions.json: helper generated/fallback

## Recommended Next Steps
1. Generate only the committed safe audited manifest files from the converter.
2. Keep helper fallback and helper-generated outputs separate from audited region files.
3. Leave count-mismatch, schema-mismatch, and item_data_statistics out of promotion until a future audit proves them safe.
