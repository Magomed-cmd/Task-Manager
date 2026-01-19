-- +goose Up
-- +goose StatementBegin

ALTER TABLE task_events RENAME COLUMN event_ts TO created_at;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

ALTER TABLE task_events RENAME COLUMN created_at TO event_ts;

-- +goose StatementEnd
