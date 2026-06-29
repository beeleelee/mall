CREATE TABLE IF NOT EXISTS a2a_tasks (
    id BIGINT PRIMARY KEY,
    user_id BIGINT NOT NULL,
    skill_id VARCHAR(255) NOT NULL DEFAULT '',
    status_state VARCHAR(50) NOT NULL DEFAULT 'submitted',
    status_message TEXT NOT NULL DEFAULT '',
    status_updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    status_completed_at TIMESTAMP WITH TIME ZONE,
    context_id VARCHAR(255),
    history JSONB NOT NULL DEFAULT '[]'::jsonb,
    artifacts JSONB NOT NULL DEFAULT '[]'::jsonb,
    metadata JSONB,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_a2a_tasks_user_id ON a2a_tasks(user_id);
CREATE INDEX idx_a2a_tasks_context_id ON a2a_tasks(context_id);
CREATE INDEX idx_a2a_tasks_status_state ON a2a_tasks(status_state);
CREATE INDEX idx_a2a_tasks_updated_at ON a2a_tasks(updated_at DESC);

CREATE TABLE IF NOT EXISTS a2a_push_notification_configs (
    id BIGINT PRIMARY KEY,
    task_id BIGINT NOT NULL REFERENCES a2a_tasks(id) ON DELETE CASCADE,
    url TEXT NOT NULL,
    auth_scheme VARCHAR(50) NOT NULL DEFAULT '',
    auth_credentials TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_a2a_push_configs_task_id ON a2a_push_notification_configs(task_id);
