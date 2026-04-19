ALTER TABLE tasks
    ADD COLUMN IF NOT EXISTS kind TEXT NOT NULL DEFAULT 'single',
    ADD COLUMN IF NOT EXISTS scheduled_for DATE NULL,
    ADD COLUMN IF NOT EXISTS parent_task_id BIGINT NULL,
    ADD COLUMN IF NOT EXISTS is_active BOOLEAN NOT NULL DEFAULT TRUE;

ALTER TABLE tasks
    ADD CONSTRAINT fk_tasks_parent_task
    FOREIGN KEY (parent_task_id)
    REFERENCES tasks (id)
    ON DELETE CASCADE;

CREATE INDEX IF NOT EXISTS idx_tasks_kind ON tasks (kind);
CREATE INDEX IF NOT EXISTS idx_tasks_scheduled_for ON tasks (scheduled_for);
CREATE INDEX IF NOT EXISTS idx_tasks_parent_task_id ON tasks (parent_task_id);
CREATE INDEX IF NOT EXISTS idx_tasks_is_active ON tasks (is_active);

CREATE TABLE IF NOT EXISTS task_recurrence_rules (
    task_id BIGINT PRIMARY KEY REFERENCES tasks(id) ON DELETE CASCADE,
    recurrence_type TEXT NOT NULL,
    start_date DATE NOT NULL,
    end_date DATE NULL,
    time_zone TEXT NOT NULL DEFAULT 'UTC',
    every_n_days INTEGER NULL,
    day_of_month INTEGER NULL,
    day_parity TEXT NULL,
    specific_dates DATE[] NOT NULL DEFAULT '{}'::DATE[],
    last_generated_for DATE NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX IF NOT EXISTS uq_template_instance_date
    ON tasks (parent_task_id, scheduled_for)
    WHERE kind = 'instance' AND parent_task_id IS NOT NULL AND scheduled_for IS NOT NULL;
