-- P0.3 批次作业进度回写：把设备上报的进度写到明细行本身，而不只留在事件表里。
-- 设计要点：
--   1. progress_percent 刻意允许 NULL 且不给默认值：NULL 表示"该设备从未上报进度"，
--      与"上报了 0%"是两种不同事实。若默认成 0，会把"没有进度"伪装成"已开始但没推进"，
--      这正是本轮路线图审计要消灭的假成功。
--   2. 0..100 由 CHECK 强约束：服务层已有校验，数据库是最后一道闸，越界写入直接失败。
--   3. progress_at 存"进度发生时间"而非"写入时间"（写入时间已有 updated_at）。
--      回写只允许推进到更新的 progress_at，乱序到达的旧进度不得覆盖新进度，
--      否则网络重排会让明细行显示回退的假进度。
--   4. 新增列不回填历史：存量明细行的进度保持 NULL，不存在任何由回填凭空生成的进度。

ALTER TABLE public.command_job_details
    ADD COLUMN IF NOT EXISTS progress_percent int4 NULL,
    ADD COLUMN IF NOT EXISTS progress_status varchar(32) NULL,
    ADD COLUMN IF NOT EXISTS progress_error text NULL,
    ADD COLUMN IF NOT EXISTS progress_at timestamptz NULL;

-- 约束先删后加，使本迁移可重复执行（迁移号位只跑一次，但重跑不应炸库）。
ALTER TABLE public.command_job_details
    DROP CONSTRAINT IF EXISTS command_job_details_progress_percent_range;

ALTER TABLE public.command_job_details
    ADD CONSTRAINT command_job_details_progress_percent_range
        CHECK (progress_percent IS NULL OR (progress_percent >= 0 AND progress_percent <= 100));

COMMENT ON COLUMN public.command_job_details.progress_percent IS
    'Latest device-reported progress percentage (0-100); NULL means no progress was ever reported, which is not the same as 0';
COMMENT ON COLUMN public.command_job_details.progress_status IS
    'Device-side status carried by the latest accepted progress report';
COMMENT ON COLUMN public.command_job_details.progress_error IS
    'Device-side error carried by the latest accepted progress report';
COMMENT ON COLUMN public.command_job_details.progress_at IS
    'Occurrence time of the accepted progress report; write-back only accepts a report newer than this value';
