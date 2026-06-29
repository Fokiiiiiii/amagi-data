# Belfast Expansion Audit

## Counting Model
- source_region_files_total: 3120
- comparable_source_files_count: 3110
- excluded_source_files_count: 10
- safe_to_promote_count: 604

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
- match_after_dict_keyed_to_list_by_id: 313
- match_after_both_transformations: 1
- count_mismatch: 2322
- schema_mismatch: 40
- missing_reference: 36
- unsupported: 0
- belfast_only: 48

## Safe To Promote Summary
- Total: 604
- exact_raw_match: 290
- match_after_empty_normalization: 0
- match_after_dict_keyed_to_list_by_id: 313
- match_after_both_transformations: 1
- Examples:
  - CN/GameCfg/buff.json [CN/exact_raw_match]
  - CN/GameCfg/card.json [CN/exact_raw_match]
  - CN/GameCfg/dorm.json [CN/exact_raw_match]
  - CN/GameCfg/dungeon.json [CN/exact_raw_match]
  - CN/GameCfg/skill.json [CN/exact_raw_match]
  - CN/GameCfg/story.json [CN/exact_raw_match]
  - CN/ShareCfg/activity_coloring_template.json [CN/exact_raw_match]
  - CN/ShareCfg/activity_const.json [CN/exact_raw_match]
  - ... 596 more

## Count Mismatch Summary
- Count: 2322
- Examples:
  - CN/ShareCfg/achievement_data_template.json [CN/count_mismatch]
  - CN/ShareCfg/activity_7_day_sign.json [CN/count_mismatch]
  - CN/ShareCfg/activity_banner.json [CN/count_mismatch]
  - CN/ShareCfg/activity_banner_notice.json [CN/count_mismatch]
  - CN/ShareCfg/activity_dreamland_event.json [CN/count_mismatch]
  - CN/ShareCfg/activity_dreamland_explore.json [CN/count_mismatch]
  - CN/ShareCfg/activity_dreamland_map.json [CN/count_mismatch]
  - CN/ShareCfg/activity_drop_type.json [CN/count_mismatch]
  - ... 2314 more

## Schema Mismatch Summary
- Count: 40
- Examples:
  - CN/ShareCfg/auto_pilot_template.json [CN/schema_mismatch]
  - CN/ShareCfg/error_message.json [CN/schema_mismatch]
  - CN/ShareCfg/expedition_data_by_map.json [CN/schema_mismatch]
  - CN/ShareCfg/guildset.json [CN/schema_mismatch]
  - CN/ShareCfg/illustrator.json [CN/schema_mismatch]
  - CN/ShareCfg/pay_level_award.json [CN/schema_mismatch]
  - CN/ShareCfg/ship_data_group.json [CN/schema_mismatch]
  - CN/ShareCfg/world_item_data_origin.json [CN/schema_mismatch]
  - ... 32 more

## Schema Mismatch Buckets
- map_vs_list_shape (30): CN/ShareCfg/error_message.json, CN/ShareCfg/expedition_data_by_map.json, CN/ShareCfg/illustrator.json, CN/ShareCfg/pay_level_award.json, CN/ShareCfg/ship_data_group.json, CN/ShareCfg/world_item_data_origin.json, EN/ShareCfg/error_message.json, EN/ShareCfg/expedition_data_by_map.json, EN/ShareCfg/illustrator.json, EN/ShareCfg/pay_level_award.json, EN/ShareCfg/ship_data_group.json, EN/ShareCfg/world_item_data_origin.json, JP/ShareCfg/error_message.json, JP/ShareCfg/expedition_data_by_map.json, JP/ShareCfg/illustrator.json, JP/ShareCfg/pay_level_award.json, JP/ShareCfg/ship_data_group.json, JP/ShareCfg/world_item_data_origin.json, KR/ShareCfg/error_message.json, KR/ShareCfg/expedition_data_by_map.json, KR/ShareCfg/illustrator.json, KR/ShareCfg/pay_level_award.json, KR/ShareCfg/ship_data_group.json, KR/ShareCfg/world_item_data_origin.json, TW/ShareCfg/error_message.json, TW/ShareCfg/expedition_data_by_map.json, TW/ShareCfg/illustrator.json, TW/ShareCfg/pay_level_award.json, TW/ShareCfg/ship_data_group.json, TW/ShareCfg/world_item_data_origin.json
- empty_object_vs_empty_array (0)
- scalar_vs_array (5): CN/ShareCfg/guildset.json, EN/ShareCfg/guildset.json, JP/ShareCfg/guildset.json, KR/ShareCfg/guildset.json, TW/ShareCfg/guildset.json
- key_order_or_id_sort (0)
- field_value_delta (5): CN/ShareCfg/auto_pilot_template.json, EN/ShareCfg/auto_pilot_template.json, JP/ShareCfg/auto_pilot_template.json, KR/ShareCfg/auto_pilot_template.json, TW/ShareCfg/auto_pilot_template.json
- unknown_schema_mismatch (0)

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
- rejected: `CN/sharecfgdata/item_data_statistics.json` [usage_drop_rule_validation] AzurLaneData: 3030 records; Belfast: 2568 records; filtered source after excluding usage == "usage_drop" and applying canonical transforms: 2517; exact match still fails and remains 51 records short.
- rejected: `EN/sharecfgdata/item_data_statistics.json` [usage_drop_rule_validation] AzurLaneData: 2628 records; Belfast: 2250 records; filtered source after excluding usage == "usage_drop" and applying canonical transforms: 2155; exact match still fails and remains 95 records short.
- rejected: `JP/sharecfgdata/item_data_statistics.json` [usage_drop_rule_validation] AzurLaneData: 2734 records; Belfast: 2378 records; filtered source after excluding usage == "usage_drop" and applying canonical transforms: 2327; exact match still fails and remains 51 records short.
- rejected: `KR/sharecfgdata/item_data_statistics.json` [usage_drop_rule_validation] AzurLaneData: 2549 records; Belfast: 2209 records; filtered source after excluding usage == "usage_drop" and applying canonical transforms: 2158; exact match still fails and remains 51 records short.
- rejected: `TW/sharecfgdata/item_data_statistics.json` [usage_drop_rule_validation] AzurLaneData: 2051 records; Belfast: 1730 records; filtered source after excluding usage == "usage_drop" and applying canonical transforms: 1677; exact match still fails and remains 53 records short.

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
