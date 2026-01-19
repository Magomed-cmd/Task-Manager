-- +goose Up
-- +goose StatementBegin

ALTER TABLE task_events ADD COLUMN IF NOT EXISTS room_id TEXT;
ALTER TABLE task_events ADD COLUMN IF NOT EXISTS payload JSONB;
ALTER TABLE task_events ADD COLUMN IF NOT EXISTS created_at TIMESTAMP WITH TIME ZONE;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

ALTER TABLE task_events DROP COLUMN IF EXISTS created_at;
ALTER TABLE task_events DROP COLUMN IF EXISTS payload;
ALTER TABLE task_events DROP COLUMN IF EXISTS room_id;

-- +goose StatementEnd
