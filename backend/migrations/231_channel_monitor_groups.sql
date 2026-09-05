-- Migration: 231_channel_monitor_groups
-- 管理员维护的渠道监控分组及组内渠道排序。
-- 旧 group_name 保留，已有监控默认进入未分组（NULL）。

CREATE TABLE IF NOT EXISTS channel_monitor_groups (
    id BIGSERIAL PRIMARY KEY,
    name VARCHAR(100) NOT NULL,
    sort_order INT NOT NULL DEFAULT 0,
    created_by BIGINT NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

ALTER TABLE channel_monitors
    ADD COLUMN IF NOT EXISTS monitor_group_id BIGINT
        REFERENCES channel_monitor_groups(id) ON DELETE SET NULL;

ALTER TABLE channel_monitors
    ADD COLUMN IF NOT EXISTS monitor_sort_order INT NOT NULL DEFAULT 0;

CREATE INDEX IF NOT EXISTS idx_channel_monitors_monitor_group_order
    ON channel_monitors (monitor_group_id, monitor_sort_order, id);

CREATE INDEX IF NOT EXISTS idx_channel_monitor_groups_sort_order
    ON channel_monitor_groups (sort_order, id);
