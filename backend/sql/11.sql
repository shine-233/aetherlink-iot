-- ✅2025/9.9 增加可视化类型字段
ALTER TABLE public.boards ADD vis_type varchar(50) NULL;
COMMENT ON COLUMN public.boards.vis_type IS '可视化类型';

-- ✅2025/9.18 脱敏后的SQL语句（保留数据样例格式）
INSERT INTO public.notification_services_config
(id, config, notice_type, status, remark)
VALUES('286a116e-c25f-0f4c-890a-8a72128ef355',
       '{"host":"smtp.example.com","port":465,"from_password":"CHANGE_ME_SMTP_PASSWORD","from_email":"noreply@example.com","ssl":true}'::json,
       'EMAIL',
       'OPEN',
       '');
