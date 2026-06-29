# amagi-data
who is json?

## Maintenance Commands

`cmd/belfast_data_audit` is the maintenance audit entrypoint. `cmd/belfast_json_mvp` is still retained because the validation and update workflows invoke it, and its conversion logic stays centralized in `internal/belfastconv`.
