-- +goose Up
-- +goose StatementBegin

CREATE EXTENSION IF NOT EXISTS pgcrypto;

CREATE TABLE IF NOT EXISTS tasks (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    title TEXT NOT NULL,
    description TEXT,
    type TEXT NOT NULL,
    target INTEGER NOT NULL,
    reward JSONB,
    is_active BOOLEAN DEFAULT true,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS task_progress (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    task_id UUID NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
    user_id TEXT NOT NULL,
    progress INTEGER DEFAULT 0,
    completed BOOLEAN DEFAULT false,
    claimed BOOLEAN DEFAULT false,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_task_progress_user_task
ON task_progress(user_id, task_id);

CREATE INDEX IF NOT EXISTS idx_task_progress_completed_not_claimed
ON task_progress(completed)
WHERE completed = true AND claimed = false;

CREATE INDEX IF NOT EXISTS idx_task_progress_user_id
ON task_progress(user_id);

CREATE TABLE IF NOT EXISTS task_events (
    event_id UUID PRIMARY KEY,
    user_id TEXT NOT NULL,
    type TEXT NOT NULL,
    room_id TEXT,
    payload JSONB,
    created_at TIMESTAMP WITH TIME ZONE,
    processed_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_task_events_user_id
ON task_events(user_id);

CREATE INDEX IF NOT EXISTS idx_tasks_active
ON tasks (is_active)
WHERE is_active = true;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

DROP INDEX IF EXISTS idx_tasks_active;
DROP INDEX IF EXISTS idx_task_events_user_id;
DROP INDEX IF EXISTS idx_task_progress_user_id;
DROP INDEX IF EXISTS idx_task_progress_completed_not_claimed;
DROP INDEX IF EXISTS idx_task_progress_user_task;
DROP TABLE IF EXISTS task_events;
DROP TABLE IF EXISTS task_progress;
DROP TABLE IF EXISTS tasks;

-- +goose StatementEnd
