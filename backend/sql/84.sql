-- P0.2 设备影子 ACK 闭环：把“已下发”与“已确认”分离。
-- 此前 dispatch 成功即标记 delivered，等于把“发出去”当成“设备收到了”，属于虚假成功。
-- 现引入 sent（已下发待 ACK）与 failed（超过最大尝试仍未 ACK），未确认按退避重试。

ALTER TABLE public.device_shadow_messages
    ADD COLUMN IF NOT EXISTS attempts        integer NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS sent_at         timestamptz NULL,
    ADD COLUMN IF NOT EXISTS ack_at          timestamptz NULL,
    ADD COLUMN IF NOT EXISTS next_attempt_at timestamptz NULL,
    ADD COLUMN IF NOT EXISTS last_error      text NULL;

-- 历史数据回填：旧语义下 delivered 表示已投递，其 delivered_at 即送达时间，
-- 补写 ack_at 让新旧状态语义对齐，不谎称这些行曾经等待过 ACK。
UPDATE public.device_shadow_messages
   SET ack_at = delivered_at
 WHERE status = 'delivered'
   AND delivered_at IS NOT NULL
   AND ack_at IS NULL;

-- 状态词表扩展（含 sent / failed）。先删除旧约束再重建，保证可重复执行。
DO $$
BEGIN
    IF EXISTS (
        SELECT 1 FROM pg_constraint
        WHERE conname = 'device_shadow_messages_status_check'
          AND conrelid = 'public.device_shadow_messages'::regclass
    ) THEN
        ALTER TABLE public.device_shadow_messages
            DROP CONSTRAINT device_shadow_messages_status_check;
    END IF;
END $$;

ALTER TABLE public.device_shadow_messages
    ADD CONSTRAINT device_shadow_messages_status_check
    CHECK (status IN ('pending', 'sent', 'delivered', 'failed', 'expired', 'canceled'));

-- 待重投扫描索引：仅覆盖 sent 行，避免全表扫描。
CREATE INDEX IF NOT EXISTS idx_device_shadow_messages_retry
    ON public.device_shadow_messages (status, next_attempt_at)
    WHERE status = 'sent';
