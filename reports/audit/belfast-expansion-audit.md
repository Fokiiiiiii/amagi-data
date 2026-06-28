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
- EN: 620
- JP: 624
- KR: 626
- TW: 616

## Safe To Promote Summary
- Total: 604
- exact_raw_match: 290
- match_after_empty_normalization: 0
- match_after_dict_keyed_to_list_by_id: 313
- match_after_both_transformations: 1
- Examples:
  - CN/GameCfg/buff.json [CN/exact_raw_match]: raw JSON values are identical
  - CN/GameCfg/card.json [CN/exact_raw_match]: raw JSON values are identical
  - CN/GameCfg/dorm.json [CN/exact_raw_match]: raw JSON values are identical
  - CN/GameCfg/dungeon.json [CN/exact_raw_match]: raw JSON values are identical
  - CN/GameCfg/skill.json [CN/exact_raw_match]: raw JSON values are identical
  - CN/GameCfg/story.json [CN/exact_raw_match]: raw JSON values are identical
  - CN/ShareCfg/activity_clue.json [CN/match_after_dict_keyed_to_list_by_id]: equal after converting dict-keyed records into an id-sorted list
  - CN/ShareCfg/activity_clue_ending.json [CN/match_after_dict_keyed_to_list_by_id]: equal after converting dict-keyed records into an id-sorted list
  - ... 596 more
## Count Mismatch Summary
- Count: 2389
- Examples:
  - CN/ShareCfg/achievement_data_template.json [CN/count_mismatch]: record counts differ after canonicalization (source=11 reference=10 delta=1)
  - CN/ShareCfg/activity_7_day_sign.json [CN/count_mismatch]: record counts differ after canonicalization (source=71 reference=70 delta=1)
  - CN/ShareCfg/activity_banner.json [CN/count_mismatch]: record counts differ after canonicalization (source=21 reference=19 delta=2)
  - CN/ShareCfg/activity_banner_notice.json [CN/count_mismatch]: record counts differ after canonicalization (source=37 reference=36 delta=1)
  - CN/ShareCfg/activity_dreamland_event.json [CN/count_mismatch]: record counts differ after canonicalization (source=20 reference=19 delta=1)
  - CN/ShareCfg/activity_dreamland_explore.json [CN/count_mismatch]: record counts differ after canonicalization (source=30 reference=28 delta=2)
  - CN/ShareCfg/activity_dreamland_map.json [CN/count_mismatch]: record counts differ after canonicalization (source=7 reference=6 delta=1)
  - CN/ShareCfg/activity_drop_type.json [CN/count_mismatch]: record counts differ after canonicalization (source=8 reference=6 delta=2)
  - ... 2381 more
## Schema Mismatch Summary
- Count: 71
- Examples:
  - CN/ShareCfg/auto_pilot_template.json [CN/schema_mismatch]: record counts match but record structure differs ($[9].id: source=15002 reference=15003)
  - CN/ShareCfg/error_message.json [CN/schema_mismatch]: record counts match but record structure differs ($: source=map[string]interface {} reference=[]interface {})
  - CN/ShareCfg/expedition_data_by_map.json [CN/schema_mismatch]: record counts match but record structure differs ($: source=map[string]interface {} reference=[]interface {})
  - CN/ShareCfg/guildset.json [CN/schema_mismatch]: record counts match but record structure differs ($.base_capital.key_args: source= reference=[]interface {})
  - CN/ShareCfg/illustrator.json [CN/schema_mismatch]: record counts match but record structure differs ($: source=map[string]interface {} reference=[]interface {})
  - CN/ShareCfg/pay_level_award.json [CN/schema_mismatch]: record counts match but record structure differs ($: source=map[string]interface {} reference=[]interface {})
  - CN/ShareCfg/ship_data_group.json [CN/schema_mismatch]: record counts match but record structure differs ($: source=map[string]interface {} reference=[]interface {})
  - CN/ShareCfg/world_item_data_origin.json [CN/schema_mismatch]: record counts match but record structure differs ($: source=map[string]interface {} reference=[]interface {})
  - ... 63 more
## Belfast Only Summary
- Count: 58
- Examples:
  - CN/ShareCfg/battle_nodes_cfg.json [CN/belfast_only]: no comparable source file was found
  - CN/ShareCfg/dorm3_d_collect.json [CN/belfast_only]: no comparable source file was found
  - CN/ShareCfg/dorm3_d_dolly.json [CN/belfast_only]: no comparable source file was found
  - CN/ShareCfg/dorm3_d_recall.json [CN/belfast_only]: no comparable source file was found
  - CN/ShareCfg/inform_cfg.json [CN/belfast_only]: no comparable source file was found
  - CN/ShareCfg/inform_for_back_yard_theme_template_cfg.json [CN/belfast_only]: no comparable source file was found
  - CN/ShareCfg/voice_actor_cn.json [CN/belfast_only]: no comparable source file was found
  - CN/ShareCfg/world_sl_gbuff_data.json [CN/belfast_only]: no comparable source file was found
  - ... 50 more
## Missing Reference Summary
- Count: 48
- Examples:
  - CN/ShareCfg/BattleNodesCfg.json [CN/missing_reference]: no Belfast reference file was found
  - CN/ShareCfg/InformForBackYardThemeTemplateCfg.json [CN/missing_reference]: no Belfast reference file was found
  - CN/ShareCfg/dorm3D_collect.json [CN/missing_reference]: no Belfast reference file was found
  - CN/ShareCfg/dorm3D_dolly.json [CN/missing_reference]: no Belfast reference file was found
  - CN/ShareCfg/informCfg.json [CN/missing_reference]: no Belfast reference file was found
  - CN/ShareCfg/voice_actor_CN.json [CN/missing_reference]: no Belfast reference file was found
  - CN/ShareCfg/world_SLGbuff_data.json [CN/missing_reference]: no Belfast reference file was found
  - CN/buffCfg.json [CN/missing_reference]: no Belfast reference file was found
  - ... 40 more
## Special Files
- buff_cfg.json: reference_missing
- build_pools.json: fallback/generated
- build_times.json: fallback/generated
- requisition_ships.json: fallback/generated
- skill_cfg.json: reference_missing
- versions.json: fallback/generated

## Transform Rule Evidence
- confirmed: `JP/sharecfgdata/ship_data_statistics.json`, `JP/sharecfgdata/weapon_property.json`, `JP/sharecfgdata/equip_data_template.json`, and `JP/ShareCfg/ship_skin_template.json` all match after dict-keyed records -> id-sorted list and empty object normalization.
- observed: `JP/sharecfgdata/item_data_statistics.json` has a count mismatch (Belfast 2378, AzurLaneData 2734, delta 356) and the missing records are strongly correlated with `usage == "usage_drop"`.

## Helper Data Notes
- `build_pools.json` and `build_times.json` are currently treated as fallback/generated helper outputs, and their exact source fields are not confirmed.
- `requisition_ships.json` is currently treated as a fallback/generated helper output.
- `versions.json` is currently treated as a fallback/generated helper output, and generation from public upstream code is not confirmed.

## Recommended Next Implementation Steps
1. Expand main generator to walk region directories and apply matching transforms.
2. Exclude `build_pools.json`, `build_times.json`, `requisition_ships.json` and keep fallback mechanism.
3. Handle `buff_cfg.json` and `skill_cfg.json` using exact transforms.
