-- sql/baseline/143.sql —— 由 cmd/migbaseline 生成，禁止手改（改了任何 sql/1..143.sql 必须重新生成）。
-- generator: cd backend && go run ./cmd/migbaseline -dsn-admin <admin-dsn> -verify
-- postgres: 17.5（AETHERLINK_TIMESCALE_MODE=off；TimescaleDB 安装不使用本基线）
-- source-range: 1..143
-- source-sha256: 0895f37e6ef92a63c2c2825eeea16a8b6889c5c7dcb5cb8742142fa2e555d139
-- 种子行的 now() 时间戳为生成时刻，而非安装时刻。

SET LOCAL check_function_bodies = false;
SET LOCAL statement_timeout = 0;
SET LOCAL lock_timeout = 0;

--
-- PostgreSQL database dump
--

-- Dumped from database version 17.5
-- Dumped by pg_dump version 17.5


--
-- Name: thingsvis; Type: SCHEMA; Schema: -; Owner: -
--

CREATE SCHEMA thingsvis;


--
-- Name: alarm_history_devices_sync(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.alarm_history_devices_sync() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
    IF TG_OP = 'UPDATE'
       AND NEW.alarm_device_list IS NOT DISTINCT FROM OLD.alarm_device_list THEN
        RETURN NEW;
    END IF;

    DELETE FROM public.alarm_history_devices WHERE alarm_history_id = NEW.id;

    INSERT INTO public.alarm_history_devices (alarm_history_id, device_id, tenant_id)
    SELECT NEW.id, btrim(elem.value), NEW.tenant_id
    FROM jsonb_array_elements_text(
             COALESCE(
                 CASE WHEN jsonb_typeof(NEW.alarm_device_list) = 'array'
                      THEN (SELECT jsonb_agg(e)
                            FROM jsonb_array_elements(NEW.alarm_device_list) AS e
                            WHERE jsonb_typeof(e) = 'string')
                 END,
                 '[]'::jsonb)
         ) AS elem(value)
    WHERE btrim(elem.value) <> ''
    ON CONFLICT (alarm_history_id, device_id) DO NOTHING;

    RETURN NEW;
END;
$$;




--
-- Name: action_info; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.action_info (
    id character varying(36) NOT NULL,
    scene_automation_id character varying(36) NOT NULL,
    action_target character varying(255),
    action_type character varying(10) NOT NULL,
    action_param_type character varying(50),
    action_param character varying(50),
    action_value text,
    remark character varying(255)
);


--
-- Name: COLUMN action_info.scene_automation_id; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.action_info.scene_automation_id IS '场景联动ID（外键-关联删除）';


--
-- Name: COLUMN action_info.action_target; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.action_info.action_target IS '动作目标id设备id、场景id、告警id；如果条件是单类设备，这里为空';


--
-- Name: COLUMN action_info.action_type; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.action_info.action_type IS '动作类型10: 单个设备11: 单类设备20: 激活场景30: 触发告警40: 服务';


--
-- Name: COLUMN action_info.action_param_type; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.action_info.action_param_type IS '遥测TEL属性ATTR命令CMD';


--
-- Name: COLUMN action_info.action_param; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.action_info.action_param IS '动作参数动作类型为10,11是有效 标识符';


--
-- Name: COLUMN action_info.action_value; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.action_info.action_value IS '目标值';


--
-- Name: ai_models; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.ai_models (
    id character varying(36) NOT NULL,
    tenant_id character varying(36) NOT NULL,
    name character varying(128) NOT NULL,
    provider character varying(32) DEFAULT 'openai'::character varying NOT NULL,
    base_url character varying(512) NOT NULL,
    model character varying(128) NOT NULL,
    api_key text NOT NULL,
    purpose character varying(32) DEFAULT 'chat'::character varying NOT NULL,
    enabled boolean DEFAULT true NOT NULL,
    created_at timestamp without time zone DEFAULT now() NOT NULL,
    updated_at timestamp without time zone DEFAULT now() NOT NULL
);


--
-- Name: alarm_assignment; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.alarm_assignment (
    id character varying(36) NOT NULL,
    tenant_id character varying(36) NOT NULL,
    alarm_history_id character varying(36) NOT NULL,
    assignee_user_id character varying(36),
    operator_user_id character varying(36) NOT NULL,
    remark text,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT alarm_assignment_alarm_check CHECK (((alarm_history_id)::text <> ''::text)),
    CONSTRAINT alarm_assignment_assignee_check CHECK (((assignee_user_id IS NULL) OR ((assignee_user_id)::text <> ''::text))),
    CONSTRAINT alarm_assignment_operator_check CHECK (((operator_user_id)::text <> ''::text)),
    CONSTRAINT alarm_assignment_tenant_check CHECK (((tenant_id)::text <> ''::text))
);


--
-- Name: alarm_comment; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.alarm_comment (
    id character varying(36) NOT NULL,
    tenant_id character varying(36) NOT NULL,
    alarm_history_id character varying(36) NOT NULL,
    content text NOT NULL,
    author_user_id character varying(36) NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT alarm_comment_alarm_check CHECK (((alarm_history_id)::text <> ''::text)),
    CONSTRAINT alarm_comment_author_check CHECK (((author_user_id)::text <> ''::text)),
    CONSTRAINT alarm_comment_content_check CHECK ((content <> ''::text)),
    CONSTRAINT alarm_comment_tenant_check CHECK (((tenant_id)::text <> ''::text))
);


--
-- Name: alarm_config; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.alarm_config (
    id character varying(36) NOT NULL,
    name character varying(255) NOT NULL,
    description character varying(255),
    alarm_level character varying(10) NOT NULL,
    notification_group_id character varying(36) NOT NULL,
    created_at timestamp(6) with time zone NOT NULL,
    updated_at timestamp(6) with time zone NOT NULL,
    tenant_id character varying(36) NOT NULL,
    remark character varying(255),
    enabled character varying(10) NOT NULL,
    trigger_duration integer DEFAULT 0,
    sla_hours integer,
    CONSTRAINT alarm_config_trigger_duration_check CHECK (((trigger_duration IS NULL) OR ((trigger_duration >= 0) AND (trigger_duration <= 86400))))
);


--
-- Name: TABLE alarm_config; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON TABLE public.alarm_config IS '告警配置';


--
-- Name: COLUMN alarm_config.name; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.alarm_config.name IS '告警名称';


--
-- Name: COLUMN alarm_config.description; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.alarm_config.description IS '告警描述';


--
-- Name: COLUMN alarm_config.alarm_level; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.alarm_config.alarm_level IS '告警级别H: 高M: 中L: 低';


--
-- Name: COLUMN alarm_config.notification_group_id; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.alarm_config.notification_group_id IS '通知组id';


--
-- Name: COLUMN alarm_config.enabled; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.alarm_config.enabled IS '是否启用Y-启用N-停止';


--
-- Name: COLUMN alarm_config.trigger_duration; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.alarm_config.trigger_duration IS 'Seconds the alarm condition must hold continuously before the alarm fires; 0 or NULL fires on the first matching sample.';


--
-- Name: COLUMN alarm_config.sla_hours; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.alarm_config.sla_hours IS '告警SLA时限（小时；空=不启用超时升级）';


--
-- Name: alarm_history; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.alarm_history (
    id character varying(36) NOT NULL,
    alarm_config_id character varying(36) NOT NULL,
    group_id character varying(36) NOT NULL,
    scene_automation_id character varying(36) NOT NULL,
    name character varying(255) NOT NULL,
    description character varying(255),
    content text,
    alarm_status character varying(3) NOT NULL,
    tenant_id character varying(36) NOT NULL,
    remark text,
    create_at timestamp(6) with time zone NOT NULL,
    alarm_device_list jsonb NOT NULL,
    sla_due_at timestamp with time zone,
    sla_breached boolean DEFAULT false NOT NULL
);


--
-- Name: COLUMN alarm_history.name; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.alarm_history.name IS '告警名称';


--
-- Name: COLUMN alarm_history.description; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.alarm_history.description IS '告警描述';


--
-- Name: COLUMN alarm_history.content; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.alarm_history.content IS '内容（什么原因导致的告警）';


--
-- Name: COLUMN alarm_history.alarm_status; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.alarm_history.alarm_status IS 'L 底 M中 H 高 N 正常';


--
-- Name: COLUMN alarm_history.tenant_id; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.alarm_history.tenant_id IS '租户';


--
-- Name: COLUMN alarm_history.remark; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.alarm_history.remark IS 'JSON audit metadata for acknowledge/reset actions; text avoids truncating cumulative action history.';


--
-- Name: COLUMN alarm_history.create_at; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.alarm_history.create_at IS '创建时间';


--
-- Name: COLUMN alarm_history.alarm_device_list; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.alarm_history.alarm_device_list IS '触发设备id';


--
-- Name: COLUMN alarm_history.sla_due_at; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.alarm_history.sla_due_at IS 'SLA到期时间（触发时刻+sla_hours；空=未启用SLA）';


--
-- Name: COLUMN alarm_history.sla_breached; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.alarm_history.sla_breached IS 'SLA是否已超时升级（cron扫描标记；TRUE后不再重复升级）';


--
-- Name: alarm_history_devices; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.alarm_history_devices (
    alarm_history_id character varying(36) NOT NULL,
    device_id character varying(36) NOT NULL,
    tenant_id character varying(36) NOT NULL
);


--
-- Name: TABLE alarm_history_devices; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON TABLE public.alarm_history_devices IS '告警历史命中设备的规范化关联表（alarm_history.alarm_device_list 的关系型投影）';


--
-- Name: COLUMN alarm_history_devices.alarm_history_id; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.alarm_history_devices.alarm_history_id IS '告警历史 id，ON DELETE CASCADE 跟随告警删除';


--
-- Name: COLUMN alarm_history_devices.device_id; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.alarm_history_devices.device_id IS '命中设备 id（由 JSON 数组元素展开而来）';


--
-- Name: COLUMN alarm_history_devices.tenant_id; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.alarm_history_devices.tenant_id IS '冗余告警历史租户，便于按 (租户, 设备) 直接过滤';


--
-- Name: alarm_info; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.alarm_info (
    id character varying(36) NOT NULL,
    alarm_config_id character varying(36) NOT NULL,
    name character varying(255) NOT NULL,
    alarm_time timestamp(6) with time zone NOT NULL,
    description character varying(255),
    content text,
    processor character varying(36),
    processing_result character varying(10) NOT NULL,
    tenant_id character varying(36) NOT NULL,
    remark character varying(255),
    alarm_level character varying(10)
);


--
-- Name: TABLE alarm_info; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON TABLE public.alarm_info IS '告警信息';


--
-- Name: COLUMN alarm_info.alarm_config_id; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.alarm_info.alarm_config_id IS '告警配置id';


--
-- Name: COLUMN alarm_info.name; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.alarm_info.name IS '告警名称';


--
-- Name: COLUMN alarm_info.alarm_time; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.alarm_info.alarm_time IS '告警时间';


--
-- Name: COLUMN alarm_info.description; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.alarm_info.description IS '告警描述';


--
-- Name: COLUMN alarm_info.content; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.alarm_info.content IS '内容';


--
-- Name: COLUMN alarm_info.processor; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.alarm_info.processor IS '处理人id';


--
-- Name: COLUMN alarm_info.processing_result; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.alarm_info.processing_result IS '处理结果DOP-已处理UND-未处理IGN-已忽略';


--
-- Name: COLUMN alarm_info.tenant_id; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.alarm_info.tenant_id IS '租户id';


--
-- Name: COLUMN alarm_info.alarm_level; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.alarm_info.alarm_level IS '告警级别L M H';


--
-- Name: api_usage_daily; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.api_usage_daily (
    tenant_id character varying(36) NOT NULL,
    usage_date date NOT NULL,
    api_calls bigint DEFAULT 0 NOT NULL,
    created_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP,
    updated_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP
);


--
-- Name: assets; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.assets (
    id character varying(36) NOT NULL,
    tenant_id character varying(36) NOT NULL,
    parent_id character varying(36) DEFAULT ''::character varying NOT NULL,
    name character varying(120) NOT NULL,
    asset_type character varying(64) DEFAULT 'device'::character varying NOT NULL,
    meta jsonb,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: TABLE assets; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON TABLE public.assets IS '租户资产树节点（设备/区域/产线等），parent_id 自引用';


--
-- Name: attribute_datas; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.attribute_datas (
    id character varying(36) NOT NULL,
    device_id character varying(36) NOT NULL,
    key character varying(255) NOT NULL,
    ts timestamp(6) with time zone NOT NULL,
    bool_v boolean,
    number_v double precision,
    string_v text,
    tenant_id character varying(36)
);


--
-- Name: COLUMN attribute_datas.device_id; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.attribute_datas.device_id IS '设备id（外键-关联删除）';


--
-- Name: COLUMN attribute_datas.key; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.attribute_datas.key IS '数据标识符';


--
-- Name: COLUMN attribute_datas.ts; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.attribute_datas.ts IS '上报时间';


--
-- Name: attribute_set_logs; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.attribute_set_logs (
    id character varying(36) NOT NULL,
    device_id character varying(36) NOT NULL,
    operation_type character varying(255),
    message_id character varying(36),
    data text,
    rsp_data text,
    status character varying(2),
    error_message character varying(500),
    created_at timestamp(6) with time zone NOT NULL,
    user_id character varying(36),
    description character varying(255)
);


--
-- Name: COLUMN attribute_set_logs.device_id; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.attribute_set_logs.device_id IS '设备id（外键-关联删除）';


--
-- Name: COLUMN attribute_set_logs.operation_type; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.attribute_set_logs.operation_type IS '操作类型1-手动操作 2-自动触发';


--
-- Name: COLUMN attribute_set_logs.message_id; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.attribute_set_logs.message_id IS '消息ID';


--
-- Name: COLUMN attribute_set_logs.data; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.attribute_set_logs.data IS '发送内容';


--
-- Name: COLUMN attribute_set_logs.rsp_data; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.attribute_set_logs.rsp_data IS '返回内容';


--
-- Name: COLUMN attribute_set_logs.status; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.attribute_set_logs.status IS '1-发送成功 2-失败';


--
-- Name: COLUMN attribute_set_logs.error_message; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.attribute_set_logs.error_message IS '错误信息';


--
-- Name: COLUMN attribute_set_logs.created_at; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.attribute_set_logs.created_at IS '创建时间';


--
-- Name: COLUMN attribute_set_logs.user_id; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.attribute_set_logs.user_id IS '操作用户';


--
-- Name: COLUMN attribute_set_logs.description; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.attribute_set_logs.description IS '描述';


--
-- Name: board_project_members; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.board_project_members (
    project_id character varying(36) NOT NULL,
    board_id character varying(36) NOT NULL,
    tenant_id character varying(36) NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: board_projects; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.board_projects (
    id character varying(36) NOT NULL,
    tenant_id character varying(36) NOT NULL,
    name character varying(255) NOT NULL,
    description character varying(500),
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: boards; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.boards (
    id character varying(36) NOT NULL,
    name character varying(255) NOT NULL,
    config json DEFAULT '{}'::json,
    tenant_id character varying(36) NOT NULL,
    created_at timestamp(6) with time zone NOT NULL,
    updated_at timestamp(6) with time zone NOT NULL,
    home_flag character varying(2) NOT NULL,
    description character varying(500),
    remark character varying(255),
    menu_flag character varying(2),
    vis_type character varying(50),
    published boolean DEFAULT false NOT NULL,
    published_at timestamp with time zone,
    share_token character varying(64),
    type_key character varying(64) DEFAULT ''::character varying,
    author character varying(99) DEFAULT ''::character varying,
    version character varying(36) DEFAULT '1.0.0'::character varying,
    preview_url character varying(255) DEFAULT ''::character varying,
    download_count bigint DEFAULT 0
);


--
-- Name: COLUMN boards.id; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.boards.id IS 'Id';


--
-- Name: COLUMN boards.name; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.boards.name IS '看板名称';


--
-- Name: COLUMN boards.config; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.boards.config IS '看板配置';


--
-- Name: COLUMN boards.tenant_id; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.boards.tenant_id IS '租户id（唯一）';


--
-- Name: COLUMN boards.home_flag; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.boards.home_flag IS '首页标志默认N，Y';


--
-- Name: COLUMN boards.description; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.boards.description IS '描述';


--
-- Name: COLUMN boards.remark; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.boards.remark IS '备注';


--
-- Name: COLUMN boards.menu_flag; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.boards.menu_flag IS '菜单标志默认N，Y';


--
-- Name: COLUMN boards.vis_type; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.boards.vis_type IS '可视化类型';


--
-- Name: COLUMN boards.published; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.boards.published IS '是否允许通过公开链接查看';


--
-- Name: COLUMN boards.published_at; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.boards.published_at IS '公开发布时间';


--
-- Name: COLUMN boards.share_token; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.boards.share_token IS '公开查看链接令牌';


--
-- Name: calcfield_recompute_tasks; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.calcfield_recompute_tasks (
    id character varying(36) NOT NULL,
    tenant_id character varying(36) NOT NULL,
    field_id character varying(36) NOT NULL,
    device_id character varying(36) NOT NULL,
    from_ts bigint NOT NULL,
    to_ts bigint NOT NULL,
    status character varying(16) DEFAULT 'pending'::character varying NOT NULL,
    processed bigint DEFAULT 0 NOT NULL,
    emitted bigint DEFAULT 0 NOT NULL,
    error_msg text,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: calculated_fields; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.calculated_fields (
    id character varying(36) NOT NULL,
    tenant_id character varying(36) NOT NULL,
    name character varying(128) NOT NULL,
    device_template_id character varying(36) NOT NULL,
    output_key character varying(128) NOT NULL,
    expression text NOT NULL,
    enabled boolean DEFAULT false,
    remark character varying(500),
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    type character varying(32) DEFAULT 'simple'::character varying NOT NULL,
    config jsonb
);


--
-- Name: casbin_rule; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.casbin_rule (
    id bigint NOT NULL,
    ptype character varying(100),
    v0 character varying(200),
    v1 character varying(200),
    v2 character varying(200),
    v3 character varying(200),
    v4 character varying(200),
    v5 character varying(200)
);


--
-- Name: casbin_rule_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.casbin_rule_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: casbin_rule_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.casbin_rule_id_seq OWNED BY public.casbin_rule.id;


--
-- Name: command_job_details; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.command_job_details (
    id character varying(36) NOT NULL,
    command_job_id character varying(36) NOT NULL,
    tenant_id character varying(36) NOT NULL,
    device_id character varying(36) NOT NULL,
    device_number character varying(100),
    name character varying(255),
    online boolean DEFAULT false NOT NULL,
    eligible boolean DEFAULT false NOT NULL,
    status character varying(32) NOT NULL,
    recommended_path character varying(32),
    message_id character varying(64),
    log_recorded boolean DEFAULT false NOT NULL,
    reason text,
    advice text,
    can_retry boolean DEFAULT false NOT NULL,
    telemetry_current_count integer DEFAULT 0 NOT NULL,
    latest_telemetry_key character varying(255),
    latest_telemetry_at timestamp with time zone,
    readiness jsonb,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    submitted_at timestamp with time zone,
    completed_at timestamp with time zone,
    response_status character varying(32),
    response_payload text,
    response_error text,
    response_at timestamp with time zone,
    dispatch_attempts integer DEFAULT 0 NOT NULL,
    dispatch_lease_token character varying(64),
    dispatch_lease_until timestamp with time zone,
    last_dispatch_started_at timestamp with time zone,
    next_retry_after timestamp with time zone,
    progress_percent integer,
    progress_status character varying(32),
    progress_error text,
    progress_at timestamp with time zone,
    CONSTRAINT command_job_details_progress_percent_range CHECK (((progress_percent IS NULL) OR ((progress_percent >= 0) AND (progress_percent <= 100))))
);


--
-- Name: TABLE command_job_details; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON TABLE public.command_job_details IS 'Per-device outcomes for persisted command job records';


--
-- Name: COLUMN command_job_details.status; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.command_job_details.status IS 'blocked, ready, dispatching, submitted, failed, or canceled';


--
-- Name: COLUMN command_job_details.message_id; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.command_job_details.message_id IS 'Tracked command publish message id when platform accepted dispatch';


--
-- Name: COLUMN command_job_details.response_status; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.command_job_details.response_status IS 'Device command response status copied from command_set_logs status';


--
-- Name: COLUMN command_job_details.response_payload; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.command_job_details.response_payload IS 'Raw device command response payload captured when the response is processed';


--
-- Name: COLUMN command_job_details.response_error; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.command_job_details.response_error IS 'Device response error or failure message when available';


--
-- Name: COLUMN command_job_details.response_at; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.command_job_details.response_at IS 'Platform time when the device command response was processed';


--
-- Name: COLUMN command_job_details.dispatch_attempts; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.command_job_details.dispatch_attempts IS 'Number of times this command job row has been claimed for dispatch';


--
-- Name: COLUMN command_job_details.dispatch_lease_token; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.command_job_details.dispatch_lease_token IS 'Internal worker lease token for the current dispatch attempt; not exposed to operators';


--
-- Name: COLUMN command_job_details.dispatch_lease_until; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.command_job_details.dispatch_lease_until IS 'Lease expiry for the current dispatch attempt';


--
-- Name: COLUMN command_job_details.last_dispatch_started_at; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.command_job_details.last_dispatch_started_at IS 'Platform time when the latest dispatch attempt was claimed';


--
-- Name: COLUMN command_job_details.next_retry_after; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.command_job_details.next_retry_after IS 'Earliest platform time when the failed row may be requeued by the retry action';


--
-- Name: COLUMN command_job_details.progress_percent; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.command_job_details.progress_percent IS 'Latest device-reported progress percentage (0-100); NULL means no progress was ever reported, which is not the same as 0';


--
-- Name: COLUMN command_job_details.progress_status; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.command_job_details.progress_status IS 'Device-side status carried by the latest accepted progress report';


--
-- Name: COLUMN command_job_details.progress_error; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.command_job_details.progress_error IS 'Device-side error carried by the latest accepted progress report';


--
-- Name: COLUMN command_job_details.progress_at; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.command_job_details.progress_at IS 'Occurrence time of the accepted progress report; write-back only accepts a report newer than this value';


--
-- Name: command_job_dispatch_quotas; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.command_job_dispatch_quotas (
    scope_type character varying(16) NOT NULL,
    scope_id character varying(64) NOT NULL,
    next_dispatch_at timestamp with time zone DEFAULT now() NOT NULL,
    max_concurrent integer NOT NULL,
    rate_per_second numeric(12,3) NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT command_job_dispatch_quotas_concurrency_check CHECK ((max_concurrent > 0)),
    CONSTRAINT command_job_dispatch_quotas_rate_check CHECK ((rate_per_second > (0)::numeric)),
    CONSTRAINT command_job_dispatch_quotas_scope_check CHECK (((scope_type)::text = ANY ((ARRAY['global'::character varying, 'tenant'::character varying])::text[])))
);


--
-- Name: TABLE command_job_dispatch_quotas; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON TABLE public.command_job_dispatch_quotas IS 'Database-locked global and tenant dispatch-rate cursors shared by every backend instance.';


--
-- Name: COLUMN command_job_dispatch_quotas.next_dispatch_at; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.command_job_dispatch_quotas.next_dispatch_at IS 'Leaky-bucket cursor advanced atomically whenever a command row is claimed.';


--
-- Name: command_job_events; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.command_job_events (
    id character varying(36) NOT NULL,
    command_job_id character varying(36) NOT NULL,
    tenant_id character varying(36) NOT NULL,
    detail_id character varying(36),
    device_id character varying(36),
    event_type character varying(64) NOT NULL,
    message text,
    created_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: TABLE command_job_events; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON TABLE public.command_job_events IS 'Append-only audit events for command job lifecycle and worker dispatch';


--
-- Name: command_jobs; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.command_jobs (
    id character varying(36) NOT NULL,
    tenant_id character varying(36) NOT NULL,
    operator_id character varying(36) NOT NULL,
    job_type character varying(32) NOT NULL,
    scope_type character varying(32) NOT NULL,
    identify character varying(255) NOT NULL,
    command_value text,
    timeout_seconds integer DEFAULT 60 NOT NULL,
    status character varying(32) NOT NULL,
    requested_count integer DEFAULT 0 NOT NULL,
    eligible_count integer DEFAULT 0 NOT NULL,
    blocked_count integer DEFAULT 0 NOT NULL,
    submitted_count integer DEFAULT 0 NOT NULL,
    failed_count integer DEFAULT 0 NOT NULL,
    can_cancel boolean DEFAULT false NOT NULL,
    can_retry_failed boolean DEFAULT false NOT NULL,
    scope_snapshot jsonb,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    timeout_at timestamp with time zone,
    last_submitted_at timestamp with time zone,
    remark text,
    scheduled_at timestamp with time zone,
    next_dispatch_at timestamp with time zone
);


--
-- Name: TABLE command_jobs; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON TABLE public.command_jobs IS 'Persisted selected-device and capped device-filter command job records';


--
-- Name: COLUMN command_jobs.scope_type; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.command_jobs.scope_type IS 'Current contract accepts selected_devices and device_filter';


--
-- Name: COLUMN command_jobs.scope_snapshot; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.command_jobs.scope_snapshot IS 'Preview scope, selected devices, counts and preview token at submit time';


--
-- Name: COLUMN command_jobs.scheduled_at; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.command_jobs.scheduled_at IS 'Optional operator-requested start time. Future jobs stay scheduled until a recovery scan atomically activates them.';


--
-- Name: COLUMN command_jobs.next_dispatch_at; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.command_jobs.next_dispatch_at IS 'Durable earliest time when a recovery scan should resume dispatch attempts for this job.';


--
-- Name: command_set_logs; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.command_set_logs (
    id character varying(36) NOT NULL,
    device_id character varying(36) NOT NULL,
    operation_type character varying(255),
    message_id character varying(36),
    data text,
    rsp_data text,
    status character varying(2),
    error_message character varying(500),
    created_at timestamp(6) with time zone NOT NULL,
    user_id character varying(36),
    description character varying(255),
    identify character varying(255)
);


--
-- Name: TABLE command_set_logs; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON TABLE public.command_set_logs IS '命令下发记录';


--
-- Name: COLUMN command_set_logs.device_id; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.command_set_logs.device_id IS '设备id（外键-关联删除）';


--
-- Name: COLUMN command_set_logs.operation_type; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.command_set_logs.operation_type IS '操作类型1-手动操作 2-自动触发';


--
-- Name: COLUMN command_set_logs.message_id; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.command_set_logs.message_id IS '消息ID';


--
-- Name: COLUMN command_set_logs.data; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.command_set_logs.data IS '发送内容';


--
-- Name: COLUMN command_set_logs.rsp_data; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.command_set_logs.rsp_data IS '返回内容';


--
-- Name: COLUMN command_set_logs.status; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.command_set_logs.status IS '1-发送成功 2-失败';


--
-- Name: COLUMN command_set_logs.error_message; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.command_set_logs.error_message IS '错误信息';


--
-- Name: COLUMN command_set_logs.created_at; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.command_set_logs.created_at IS '创建时间';


--
-- Name: COLUMN command_set_logs.user_id; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.command_set_logs.user_id IS '操作用户';


--
-- Name: COLUMN command_set_logs.description; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.command_set_logs.description IS '描述';


--
-- Name: COLUMN command_set_logs.identify; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.command_set_logs.identify IS '数据标识符';


--
-- Name: current_device_alarm_streams; Type: VIEW; Schema: public; Owner: -
--

CREATE VIEW public.current_device_alarm_streams AS
 WITH unnested_devices AS (
         SELECT ah.id,
            ah.alarm_config_id,
            ah.group_id,
            ah.scene_automation_id,
            ah.name,
            ah.description,
            ah.content,
            ah.alarm_status,
            ah.tenant_id,
            ah.remark,
            ah.create_at,
            jsonb_array_elements_text(
                CASE
                    WHEN (ah.alarm_device_list IS NULL) THEN '[]'::jsonb
                    WHEN (jsonb_typeof(ah.alarm_device_list) = 'array'::text) THEN ah.alarm_device_list
                    ELSE '[]'::jsonb
                END) AS device_id
           FROM public.alarm_history ah
        ), ranked_stream_states AS (
         SELECT unnested_devices.id,
            unnested_devices.alarm_config_id,
            unnested_devices.group_id,
            unnested_devices.scene_automation_id,
            unnested_devices.name,
            unnested_devices.description,
            unnested_devices.content,
            unnested_devices.alarm_status,
            unnested_devices.tenant_id,
            unnested_devices.remark,
            unnested_devices.create_at,
            unnested_devices.device_id,
            row_number() OVER (PARTITION BY unnested_devices.tenant_id, unnested_devices.device_id, unnested_devices.alarm_config_id, unnested_devices.group_id, unnested_devices.scene_automation_id ORDER BY unnested_devices.create_at DESC NULLS LAST, unnested_devices.id DESC) AS stream_rn
           FROM unnested_devices
        ), current_stream_states AS (
         SELECT ranked_stream_states.id,
            ranked_stream_states.alarm_config_id,
            ranked_stream_states.group_id,
            ranked_stream_states.scene_automation_id,
            ranked_stream_states.name,
            ranked_stream_states.description,
            ranked_stream_states.content,
            ranked_stream_states.alarm_status,
            ranked_stream_states.tenant_id,
            ranked_stream_states.remark,
            ranked_stream_states.create_at,
            ranked_stream_states.device_id
           FROM ranked_stream_states
          WHERE (ranked_stream_states.stream_rn = 1)
        )
 SELECT id,
    alarm_config_id,
    group_id,
    scene_automation_id,
    name,
    description,
    content,
    alarm_status,
    tenant_id,
    remark,
    create_at,
    device_id
   FROM current_stream_states;


--
-- Name: VIEW current_device_alarm_streams; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON VIEW public.current_device_alarm_streams IS 'Newest state for each tenant, device and alarm-config/group/scene stream; malformed device lists are ignored.';


--
-- Name: customer_devices; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.customer_devices (
    id character varying(36) NOT NULL,
    customer_id character varying(36) NOT NULL,
    device_id character varying(36) NOT NULL,
    tenant_id character varying(36) NOT NULL,
    assigned_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP
);


--
-- Name: customers; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.customers (
    id character varying(36) NOT NULL,
    name character varying(255) NOT NULL,
    tenant_id character varying(36) NOT NULL,
    country character varying(100),
    state character varying(100),
    city character varying(100),
    address character varying(255),
    address2 character varying(255),
    zip character varying(32),
    phone character varying(64),
    email character varying(128),
    additional_info jsonb DEFAULT '{}'::jsonb NOT NULL,
    created_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP,
    updated_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP
);


--
-- Name: data_converters; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.data_converters (
    id character varying(36) NOT NULL,
    name character varying(255) NOT NULL,
    type character varying(32) DEFAULT 'UPLINK'::character varying NOT NULL,
    converter_mode character varying(32) DEFAULT 'SCRIPT'::character varying NOT NULL,
    debug_mode boolean DEFAULT false NOT NULL,
    tenant_id character varying(36) NOT NULL,
    configuration text DEFAULT '{}'::text NOT NULL,
    script text,
    description character varying(500),
    created_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP,
    updated_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP,
    proto_schema text
);


--
-- Name: COLUMN data_converters.proto_schema; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.data_converters.proto_schema IS 'PROTOBUF 模式专属：.proto 源文件全文（protoreflect 动态解析，TB-19）；其他模式为 NULL';


--
-- Name: data_policy; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.data_policy (
    id character varying(36) NOT NULL,
    data_type character varying(1) NOT NULL,
    retention_days integer NOT NULL,
    last_cleanup_time timestamp(6) with time zone,
    last_cleanup_data_time timestamp(6) with time zone,
    enabled character varying(1) NOT NULL,
    remark character varying(255),
    tenant_id character varying(36),
    device_config_id character varying(36)
);


--
-- Name: COLUMN data_policy.id; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.data_policy.id IS 'Id';


--
-- Name: COLUMN data_policy.data_type; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.data_policy.data_type IS '清理类型:1-设备数据、2-操作日志';


--
-- Name: COLUMN data_policy.retention_days; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.data_policy.retention_days IS '数据保留时间（天）';


--
-- Name: COLUMN data_policy.last_cleanup_time; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.data_policy.last_cleanup_time IS '上次清理时间';


--
-- Name: COLUMN data_policy.last_cleanup_data_time; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.data_policy.last_cleanup_data_time IS '上次清理的数据时间节点（实际清理的数据时间点）';


--
-- Name: COLUMN data_policy.enabled; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.data_policy.enabled IS '是否启用：1启用 2停用';


--
-- Name: COLUMN data_policy.remark; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.data_policy.remark IS '备注';


--
-- Name: COLUMN data_policy.tenant_id; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.data_policy.tenant_id IS '行级策略租户id（TB-15R；NULL=全局默认，既有两行语义不变）';


--
-- Name: COLUMN data_policy.device_config_id; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.data_policy.device_config_id IS '行级策略设备档案id（TB-15R；NULL=该租户全部设备；tenant_id 为空时本列无意义）';


--
-- Name: data_retention_registry; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.data_retention_registry (
    id character varying(36) NOT NULL,
    table_name character varying(63) NOT NULL,
    time_column character varying(63) NOT NULL,
    time_kind character varying(16) DEFAULT 'timestamptz'::character varying NOT NULL,
    retention_days integer NOT NULL,
    category character varying(32) DEFAULT 'customer_data'::character varying NOT NULL,
    enabled character varying(10) DEFAULT '2'::character varying NOT NULL,
    batch_size integer DEFAULT 10000 NOT NULL,
    resolved_only boolean DEFAULT false NOT NULL,
    last_cleanup_time timestamp with time zone,
    last_cleanup_data_time timestamp with time zone,
    remark character varying(255),
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT data_retention_registry_batch_check CHECK (((batch_size > 0) AND (batch_size <= 100000))),
    CONSTRAINT data_retention_registry_category_check CHECK (((category)::text = ANY ((ARRAY['customer_data'::character varying, 'idempotency_receipt'::character varying, 'dead_letter'::character varying, 'audit_log'::character varying])::text[]))),
    CONSTRAINT data_retention_registry_days_check CHECK (((retention_days > 0) AND (retention_days <= 3650))),
    CONSTRAINT data_retention_registry_enabled_check CHECK (((enabled)::text = ANY ((ARRAY['1'::character varying, '2'::character varying])::text[]))),
    CONSTRAINT data_retention_registry_time_kind_check CHECK (((time_kind)::text = ANY ((ARRAY['timestamptz'::character varying, 'unix_ms'::character varying])::text[])))
);


--
-- Name: TABLE data_retention_registry; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON TABLE public.data_retention_registry IS '表级保留期注册表（TB-22）：登记只增不删表的 (表名, 时间列, 保留天数)，由 CleanSystemDataByCron 分批删除';


--
-- Name: COLUMN data_retention_registry.table_name; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.data_retention_registry.table_name IS '被清理的表名（Go 侧标识符白名单 + to_regclass 二次校验）';


--
-- Name: COLUMN data_retention_registry.time_column; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.data_retention_registry.time_column IS '判定过期的时间列名';


--
-- Name: COLUMN data_retention_registry.time_kind; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.data_retention_registry.time_kind IS 'timestamptz=时间戳列；unix_ms=UnixMilli bigint 列';


--
-- Name: COLUMN data_retention_registry.category; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.data_retention_registry.category IS 'customer_data=客户数据（默认关闭）/idempotency_receipt=幂等回执/dead_letter=死信/audit_log=审计';


--
-- Name: COLUMN data_retention_registry.enabled; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.data_retention_registry.enabled IS '是否启用：1启用 2停用（客户数据默认停用，清理不可逆）';


--
-- Name: COLUMN data_retention_registry.batch_size; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.data_retention_registry.batch_size IS '单轮分批删除的批大小，控制长事务与 WAL';


--
-- Name: COLUMN data_retention_registry.resolved_only; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.data_retention_registry.resolved_only IS '只删 status=''resolved'' 的死信行，未解决重放资产不按时间删';


--
-- Name: COLUMN data_retention_registry.last_cleanup_data_time; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.data_retention_registry.last_cleanup_data_time IS '上次实际清理到的时间边界（该时间点之前的数据已删）';


--
-- Name: data_scripts; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.data_scripts (
    id character varying(36) NOT NULL,
    name character varying(99) NOT NULL,
    device_config_id character varying(36) NOT NULL,
    enable_flag character varying(9) NOT NULL,
    content text,
    script_type character varying(9) NOT NULL,
    last_analog_input text,
    description character varying(255),
    created_at timestamp(6) with time zone,
    updated_at timestamp(6) with time zone,
    remark character varying(255)
);


--
-- Name: COLUMN data_scripts.id; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.data_scripts.id IS 'Id';


--
-- Name: COLUMN data_scripts.name; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.data_scripts.name IS '名称';


--
-- Name: COLUMN data_scripts.device_config_id; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.data_scripts.device_config_id IS '设备配置id 关联删除';


--
-- Name: COLUMN data_scripts.enable_flag; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.data_scripts.enable_flag IS '启用标志Y-启用 N-停用 默认启用';


--
-- Name: COLUMN data_scripts.content; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.data_scripts.content IS '内容';


--
-- Name: COLUMN data_scripts.script_type; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.data_scripts.script_type IS '脚本类型 A-遥测上报预处理B-遥测下发预处理C-属性上报预处理D-属性下发预处理';


--
-- Name: COLUMN data_scripts.last_analog_input; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.data_scripts.last_analog_input IS '上次模拟输入';


--
-- Name: COLUMN data_scripts.description; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.data_scripts.description IS '描述';


--
-- Name: COLUMN data_scripts.created_at; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.data_scripts.created_at IS '创建时间';


--
-- Name: COLUMN data_scripts.updated_at; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.data_scripts.updated_at IS '更新时间';


--
-- Name: COLUMN data_scripts.remark; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.data_scripts.remark IS '备注';


--
-- Name: device_certificates; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.device_certificates (
    id character varying(36) NOT NULL,
    tenant_id character varying(36) NOT NULL,
    device_id character varying(36) NOT NULL,
    serial_number character varying(64) NOT NULL,
    fingerprint character varying(64) NOT NULL,
    common_name character varying(255) NOT NULL,
    certificate text NOT NULL,
    not_before timestamp without time zone NOT NULL,
    not_after timestamp without time zone NOT NULL,
    status character varying(16) DEFAULT 'active'::character varying NOT NULL,
    issued_at timestamp without time zone DEFAULT now() NOT NULL,
    revoked_at timestamp without time zone,
    revoke_reason character varying(255),
    created_at timestamp without time zone DEFAULT now() NOT NULL,
    updated_at timestamp without time zone DEFAULT now() NOT NULL
);


--
-- Name: device_claim_tokens; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.device_claim_tokens (
    id character varying(36) NOT NULL,
    tenant_id character varying(36) NOT NULL,
    device_id character varying(36) NOT NULL,
    device_number character varying(64) NOT NULL,
    claim_key_hash character varying(64) NOT NULL,
    status character varying(16) DEFAULT 'active'::character varying NOT NULL,
    expires_at timestamp with time zone NOT NULL,
    previous_tenant_id character varying(36),
    consumed_by_tenant_id character varying(36),
    consumed_by_user_id character varying(36),
    consumed_at timestamp with time zone,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT device_claim_tokens_device_check CHECK (((device_id)::text <> ''::text)),
    CONSTRAINT device_claim_tokens_hash_check CHECK ((length((claim_key_hash)::text) = 64)),
    CONSTRAINT device_claim_tokens_status_check CHECK (((status)::text = ANY ((ARRAY['active'::character varying, 'consumed'::character varying, 'revoked'::character varying, 'replaced'::character varying])::text[]))),
    CONSTRAINT device_claim_tokens_tenant_check CHECK (((tenant_id)::text <> ''::text))
);


--
-- Name: TABLE device_claim_tokens; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON TABLE public.device_claim_tokens IS 'TB-12 设备认领令牌（明文仅签发响应出现一次，库内只存 SHA-256 哈希；一次性与过期由条件更新保证）';


--
-- Name: device_configs; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.device_configs (
    id character varying(36) NOT NULL,
    name character varying(99) NOT NULL,
    device_template_id character varying(36),
    device_type character varying(9) NOT NULL,
    protocol_type character varying(36),
    voucher_type character varying(36),
    protocol_config json,
    device_conn_type character varying(36),
    additional_info json DEFAULT '{}'::json,
    description character varying(255),
    tenant_id character varying(36) NOT NULL,
    created_at timestamp(6) with time zone NOT NULL,
    updated_at timestamp(6) with time zone NOT NULL,
    remark character varying(255),
    other_config json,
    template_secret character varying(255),
    auto_register smallint DEFAULT 0 NOT NULL,
    image_url character varying(255),
    payload_schema_id character varying(36),
    default_rule_chain_id uuid
);


--
-- Name: COLUMN device_configs.id; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.device_configs.id IS 'Id';


--
-- Name: COLUMN device_configs.name; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.device_configs.name IS '名称';


--
-- Name: COLUMN device_configs.device_template_id; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.device_configs.device_template_id IS '设备模板id';


--
-- Name: COLUMN device_configs.device_type; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.device_configs.device_type IS '设备类型 1直连设备 2网关设备 3网关子设备';


--
-- Name: COLUMN device_configs.protocol_type; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.device_configs.protocol_type IS '协议类型';


--
-- Name: COLUMN device_configs.voucher_type; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.device_configs.voucher_type IS '凭证类型';


--
-- Name: COLUMN device_configs.protocol_config; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.device_configs.protocol_config IS '协议表单配置';


--
-- Name: COLUMN device_configs.device_conn_type; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.device_configs.device_conn_type IS '设备连接方式（默认A）A-设备连接平台B-平台连接设备';


--
-- Name: COLUMN device_configs.additional_info; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.device_configs.additional_info IS '附加信息';


--
-- Name: COLUMN device_configs.description; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.device_configs.description IS '描述';


--
-- Name: COLUMN device_configs.tenant_id; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.device_configs.tenant_id IS '租户id';


--
-- Name: COLUMN device_configs.created_at; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.device_configs.created_at IS '创建时间';


--
-- Name: COLUMN device_configs.updated_at; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.device_configs.updated_at IS '更新时间';


--
-- Name: COLUMN device_configs.remark; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.device_configs.remark IS '备注';


--
-- Name: COLUMN device_configs.other_config; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.device_configs.other_config IS '其他配置';


--
-- Name: COLUMN device_configs.default_rule_chain_id; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.device_configs.default_rule_chain_id IS '档案级默认规则链id（可空；空=回落租户级启用链）';


--
-- Name: device_health_scores; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.device_health_scores (
    id character varying(36) NOT NULL,
    device_id character varying(36) NOT NULL,
    tenant_id character varying(36) NOT NULL,
    score numeric(5,2) DEFAULT 100.00 NOT NULL,
    health_status character varying(32) DEFAULT 'HEALTHY'::character varying NOT NULL,
    alarm_penalty numeric(5,2) DEFAULT 0.00 NOT NULL,
    offline_penalty numeric(5,2) DEFAULT 0.00 NOT NULL,
    anomaly_penalty numeric(5,2) DEFAULT 0.00 NOT NULL,
    details text DEFAULT '{}'::text NOT NULL,
    evaluated_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP,
    created_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP,
    updated_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP
);


--
-- Name: device_modbus_profiles; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.device_modbus_profiles (
    device_id character varying(36) NOT NULL,
    profile jsonb DEFAULT '{}'::jsonb NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_by character varying(36)
);


--
-- Name: TABLE device_modbus_profiles; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON TABLE public.device_modbus_profiles IS '设备 Modbus 点表（target+registers 映射；不含凭证），供前端编辑与 modbus-plugin 拉取';


--
-- Name: device_model_attributes; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.device_model_attributes (
    id character varying(36) NOT NULL,
    device_template_id character varying(36) NOT NULL,
    data_name character varying(255),
    data_identifier character varying(255) NOT NULL,
    read_write_flag character varying(10),
    data_type character varying(50),
    unit character varying(50),
    description character varying(255),
    additional_info json,
    created_at timestamp(6) with time zone NOT NULL,
    updated_at timestamp(6) with time zone NOT NULL,
    remark character varying(255),
    tenant_id character varying(36) NOT NULL
);


--
-- Name: COLUMN device_model_attributes.id; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.device_model_attributes.id IS 'id';


--
-- Name: COLUMN device_model_attributes.device_template_id; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.device_model_attributes.device_template_id IS '设备模板id';


--
-- Name: COLUMN device_model_attributes.data_name; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.device_model_attributes.data_name IS '数据名称';


--
-- Name: COLUMN device_model_attributes.data_identifier; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.device_model_attributes.data_identifier IS '数据标识符';


--
-- Name: COLUMN device_model_attributes.read_write_flag; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.device_model_attributes.read_write_flag IS '读写标志R-读 W-写 RW-读写';


--
-- Name: COLUMN device_model_attributes.data_type; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.device_model_attributes.data_type IS '数据类型String Number Boolean Enum';


--
-- Name: COLUMN device_model_attributes.unit; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.device_model_attributes.unit IS '单位';


--
-- Name: COLUMN device_model_attributes.description; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.device_model_attributes.description IS '描述';


--
-- Name: COLUMN device_model_attributes.additional_info; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.device_model_attributes.additional_info IS '附加信息';


--
-- Name: COLUMN device_model_attributes.created_at; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.device_model_attributes.created_at IS '创建时间';


--
-- Name: COLUMN device_model_attributes.updated_at; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.device_model_attributes.updated_at IS '更新时间';


--
-- Name: COLUMN device_model_attributes.remark; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.device_model_attributes.remark IS '备注';


--
-- Name: device_model_commands; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.device_model_commands (
    id character varying(36) NOT NULL,
    device_template_id character varying(36) NOT NULL,
    data_name character varying(255),
    data_identifier character varying(255) NOT NULL,
    params json,
    description character varying(255),
    additional_info json,
    created_at timestamp(6) with time zone NOT NULL,
    updated_at timestamp(6) with time zone NOT NULL,
    remark character varying(255),
    tenant_id character varying(36) NOT NULL
);


--
-- Name: COLUMN device_model_commands.id; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.device_model_commands.id IS 'id';


--
-- Name: COLUMN device_model_commands.device_template_id; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.device_model_commands.device_template_id IS '设备模板id';


--
-- Name: COLUMN device_model_commands.data_name; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.device_model_commands.data_name IS '数据名称';


--
-- Name: COLUMN device_model_commands.data_identifier; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.device_model_commands.data_identifier IS '数据标识符';


--
-- Name: COLUMN device_model_commands.params; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.device_model_commands.params IS '参数';


--
-- Name: COLUMN device_model_commands.description; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.device_model_commands.description IS '描述';


--
-- Name: COLUMN device_model_commands.additional_info; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.device_model_commands.additional_info IS '附加信息';


--
-- Name: COLUMN device_model_commands.created_at; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.device_model_commands.created_at IS '创建时间';


--
-- Name: COLUMN device_model_commands.updated_at; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.device_model_commands.updated_at IS '更新时间';


--
-- Name: COLUMN device_model_commands.remark; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.device_model_commands.remark IS '备注';


--
-- Name: device_model_custom_commands; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.device_model_custom_commands (
    id character varying(36) NOT NULL,
    device_template_id character varying(36) NOT NULL,
    buttom_name character varying(255) NOT NULL,
    data_identifier character varying(255) NOT NULL,
    description character varying(500),
    instruct text,
    enable_status character varying(10) NOT NULL,
    remark character varying(255),
    tenant_id character varying NOT NULL
);


--
-- Name: COLUMN device_model_custom_commands.id; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.device_model_custom_commands.id IS 'id';


--
-- Name: COLUMN device_model_custom_commands.device_template_id; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.device_model_custom_commands.device_template_id IS '设备模板id';


--
-- Name: COLUMN device_model_custom_commands.buttom_name; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.device_model_custom_commands.buttom_name IS '按钮名称';


--
-- Name: COLUMN device_model_custom_commands.data_identifier; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.device_model_custom_commands.data_identifier IS '数据标识符';


--
-- Name: COLUMN device_model_custom_commands.description; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.device_model_custom_commands.description IS '描述';


--
-- Name: COLUMN device_model_custom_commands.instruct; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.device_model_custom_commands.instruct IS '指令内容';


--
-- Name: COLUMN device_model_custom_commands.enable_status; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.device_model_custom_commands.enable_status IS '启用状态enable-启用disable-禁用';


--
-- Name: COLUMN device_model_custom_commands.remark; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.device_model_custom_commands.remark IS '备注';


--
-- Name: device_model_custom_control; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.device_model_custom_control (
    id character varying(36) NOT NULL,
    device_template_id character varying(36) NOT NULL,
    name character varying(255) NOT NULL,
    control_type character varying NOT NULL,
    description character varying(500),
    content text,
    enable_status character varying(10) NOT NULL,
    created_at timestamp without time zone NOT NULL,
    updated_at timestamp without time zone NOT NULL,
    remark character varying(255),
    tenant_id character varying(36) NOT NULL
);


--
-- Name: COLUMN device_model_custom_control.id; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.device_model_custom_control.id IS 'id';


--
-- Name: COLUMN device_model_custom_control.device_template_id; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.device_model_custom_control.device_template_id IS '设备模版ID';


--
-- Name: COLUMN device_model_custom_control.name; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.device_model_custom_control.name IS '名称';


--
-- Name: COLUMN device_model_custom_control.control_type; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.device_model_custom_control.control_type IS '1.控制类型2.telemetry-遥测3.attributes-属性';


--
-- Name: COLUMN device_model_custom_control.description; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.device_model_custom_control.description IS '描述';


--
-- Name: COLUMN device_model_custom_control.content; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.device_model_custom_control.content IS '指令内容';


--
-- Name: COLUMN device_model_custom_control.enable_status; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.device_model_custom_control.enable_status IS '启用状态enable-启用disable-禁用';


--
-- Name: COLUMN device_model_custom_control.created_at; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.device_model_custom_control.created_at IS '创建时间';


--
-- Name: COLUMN device_model_custom_control.updated_at; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.device_model_custom_control.updated_at IS '更新时间';


--
-- Name: COLUMN device_model_custom_control.remark; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.device_model_custom_control.remark IS '备注';


--
-- Name: device_model_events; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.device_model_events (
    id character varying(36) NOT NULL,
    device_template_id character varying(36) NOT NULL,
    data_name character varying(255),
    data_identifier character varying(255) NOT NULL,
    params json,
    description character varying(255),
    additional_info json,
    created_at timestamp(6) with time zone NOT NULL,
    updated_at timestamp(6) with time zone NOT NULL,
    remark character varying(255),
    tenant_id character varying(36) NOT NULL
);


--
-- Name: COLUMN device_model_events.id; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.device_model_events.id IS 'id';


--
-- Name: COLUMN device_model_events.device_template_id; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.device_model_events.device_template_id IS '设备模板id';


--
-- Name: COLUMN device_model_events.data_name; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.device_model_events.data_name IS '数据名称';


--
-- Name: COLUMN device_model_events.data_identifier; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.device_model_events.data_identifier IS '数据标识符';


--
-- Name: COLUMN device_model_events.params; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.device_model_events.params IS '参数';


--
-- Name: COLUMN device_model_events.description; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.device_model_events.description IS '描述';


--
-- Name: COLUMN device_model_events.additional_info; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.device_model_events.additional_info IS '附加信息';


--
-- Name: COLUMN device_model_events.created_at; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.device_model_events.created_at IS '创建时间';


--
-- Name: COLUMN device_model_events.updated_at; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.device_model_events.updated_at IS '更新时间';


--
-- Name: COLUMN device_model_events.remark; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.device_model_events.remark IS '备注';


--
-- Name: device_model_telemetry; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.device_model_telemetry (
    id character varying(36) NOT NULL,
    device_template_id character varying(36) NOT NULL,
    data_name character varying(255),
    data_identifier character varying(255) NOT NULL,
    read_write_flag character varying(10),
    data_type character varying(50),
    unit character varying(50),
    description character varying(255),
    additional_info json,
    created_at timestamp(6) with time zone NOT NULL,
    updated_at timestamp(6) with time zone NOT NULL,
    remark character varying(255),
    tenant_id character varying(36) NOT NULL
);


--
-- Name: COLUMN device_model_telemetry.id; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.device_model_telemetry.id IS 'id';


--
-- Name: COLUMN device_model_telemetry.device_template_id; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.device_model_telemetry.device_template_id IS '设备模板id';


--
-- Name: COLUMN device_model_telemetry.data_name; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.device_model_telemetry.data_name IS '数据名称';


--
-- Name: COLUMN device_model_telemetry.data_identifier; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.device_model_telemetry.data_identifier IS '数据标识符';


--
-- Name: COLUMN device_model_telemetry.read_write_flag; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.device_model_telemetry.read_write_flag IS '读写标志R-读 W-写 RW-读写';


--
-- Name: COLUMN device_model_telemetry.data_type; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.device_model_telemetry.data_type IS '数据类型String Number Boolean';


--
-- Name: COLUMN device_model_telemetry.unit; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.device_model_telemetry.unit IS '单位';


--
-- Name: COLUMN device_model_telemetry.description; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.device_model_telemetry.description IS '描述';


--
-- Name: COLUMN device_model_telemetry.additional_info; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.device_model_telemetry.additional_info IS '附加信息';


--
-- Name: COLUMN device_model_telemetry.created_at; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.device_model_telemetry.created_at IS '创建时间';


--
-- Name: COLUMN device_model_telemetry.updated_at; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.device_model_telemetry.updated_at IS '更新时间';


--
-- Name: COLUMN device_model_telemetry.remark; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.device_model_telemetry.remark IS '备注';


--
-- Name: device_pre_register_credential_grants; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.device_pre_register_credential_grants (
    id character varying(36) NOT NULL,
    tenant_id character varying(36) NOT NULL,
    batch_number character varying(36) NOT NULL,
    device_count integer DEFAULT 0 NOT NULL,
    status character varying(16) DEFAULT 'pending'::character varying NOT NULL,
    expires_at timestamp with time zone NOT NULL,
    created_by character varying(36),
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    consumed_by character varying(36),
    consumed_at timestamp with time zone,
    CONSTRAINT device_pre_register_credential_grants_status_check CHECK (((status)::text = ANY ((ARRAY['pending'::character varying, 'consumed'::character varying, 'expired'::character varying, 'revoked'::character varying])::text[])))
);


--
-- Name: device_shadow_messages; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.device_shadow_messages (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    device_id character varying(36) NOT NULL,
    message_type character varying(20) DEFAULT 'command'::character varying NOT NULL,
    payload jsonb DEFAULT '{}'::jsonb NOT NULL,
    ttl_seconds integer DEFAULT 86400 NOT NULL,
    status character varying(20) DEFAULT 'pending'::character varying NOT NULL,
    created_by character varying(36),
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    delivered_at timestamp with time zone,
    expires_at timestamp with time zone NOT NULL,
    attempts integer DEFAULT 0 NOT NULL,
    sent_at timestamp with time zone,
    ack_at timestamp with time zone,
    next_attempt_at timestamp with time zone,
    last_error text,
    CONSTRAINT device_shadow_messages_status_check CHECK (((status)::text = ANY ((ARRAY['pending'::character varying, 'sent'::character varying, 'delivered'::character varying, 'failed'::character varying, 'expired'::character varying, 'canceled'::character varying])::text[])))
);


--
-- Name: TABLE device_shadow_messages; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON TABLE public.device_shadow_messages IS '设备影子消息（离线命令缓存）：设备上线时投递 pending 消息，TTL 过期自动标记';


--
-- Name: COLUMN device_shadow_messages.status; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.device_shadow_messages.status IS 'pending=待投递 delivered=已投递 expired=已过期 canceled=已取消';


--
-- Name: device_status_history; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.device_status_history (
    id bigint NOT NULL,
    tenant_id character varying(36) NOT NULL,
    device_id character varying(36) NOT NULL,
    status smallint NOT NULL,
    change_time timestamp(6) with time zone NOT NULL
);


--
-- Name: TABLE device_status_history; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON TABLE public.device_status_history IS '设备状态历史表';


--
-- Name: COLUMN device_status_history.tenant_id; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.device_status_history.tenant_id IS '租户ID';


--
-- Name: COLUMN device_status_history.device_id; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.device_status_history.device_id IS '设备ID';


--
-- Name: COLUMN device_status_history.status; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.device_status_history.status IS '状态: 0-离线 1-在线';


--
-- Name: COLUMN device_status_history.change_time; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.device_status_history.change_time IS '状态变更时间';


--
-- Name: device_status_history_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.device_status_history_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: device_status_history_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.device_status_history_id_seq OWNED BY public.device_status_history.id;


--
-- Name: device_template_upgrade_history; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.device_template_upgrade_history (
    id character varying(36) NOT NULL,
    tenant_id character varying(36) NOT NULL,
    template_name character varying(255) NOT NULL,
    from_version character varying(36) NOT NULL,
    to_version character varying(36) NOT NULL,
    previous_payload text NOT NULL,
    actor_id character varying(36) NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT device_template_upgrade_history_tenant_check CHECK (((tenant_id)::text <> ''::text))
);


--
-- Name: device_templates; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.device_templates (
    id character varying(36) NOT NULL,
    name character varying(255) NOT NULL,
    author character varying(36) DEFAULT ''::character varying,
    version character varying(50) DEFAULT ''::character varying,
    description character varying(500) DEFAULT ''::character varying,
    tenant_id character varying(36) DEFAULT ''::character varying NOT NULL,
    created_at timestamp(6) with time zone NOT NULL,
    updated_at timestamp(6) with time zone NOT NULL,
    flag smallint DEFAULT 1,
    label character varying(255),
    web_chart_config json,
    app_chart_config json,
    remark character varying(255),
    path character varying(999),
    type_key character varying(255) DEFAULT ''::character varying,
    brand character varying(255) DEFAULT ''::character varying,
    model_number character varying(255) DEFAULT ''::character varying,
    download_count bigint DEFAULT 0 NOT NULL
);


--
-- Name: COLUMN device_templates.id; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.device_templates.id IS 'Id';


--
-- Name: COLUMN device_templates.name; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.device_templates.name IS '模板名称';


--
-- Name: COLUMN device_templates.author; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.device_templates.author IS '作者';


--
-- Name: COLUMN device_templates.version; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.device_templates.version IS '版本号';


--
-- Name: COLUMN device_templates.description; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.device_templates.description IS '描述';


--
-- Name: COLUMN device_templates.flag; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.device_templates.flag IS '标志 默认1';


--
-- Name: COLUMN device_templates.label; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.device_templates.label IS '标签';


--
-- Name: COLUMN device_templates.web_chart_config; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.device_templates.web_chart_config IS 'web图表配置';


--
-- Name: COLUMN device_templates.app_chart_config; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.device_templates.app_chart_config IS 'app图表配置';


--
-- Name: COLUMN device_templates.remark; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.device_templates.remark IS '备注';


--
-- Name: COLUMN device_templates.path; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.device_templates.path IS '图片路径';


--
-- Name: COLUMN device_templates.type_key; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.device_templates.type_key IS '类型';


--
-- Name: COLUMN device_templates.brand; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.device_templates.brand IS '品牌';


--
-- Name: COLUMN device_templates.model_number; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.device_templates.model_number IS '型号';


--
-- Name: device_topic_mappings; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.device_topic_mappings (
    id bigint NOT NULL,
    device_config_id uuid NOT NULL,
    name character varying(500) NOT NULL,
    direction character varying(50) NOT NULL,
    source_topic character varying(500) NOT NULL,
    target_topic character varying(500) NOT NULL,
    priority integer DEFAULT 100 NOT NULL,
    enabled boolean DEFAULT true NOT NULL,
    description text,
    created_at timestamp(6) with time zone DEFAULT now() NOT NULL,
    updated_at timestamp(6) with time zone DEFAULT now() NOT NULL,
    data_identifier character varying(500)
);


--
-- Name: device_topic_mappings_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.device_topic_mappings_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: device_topic_mappings_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.device_topic_mappings_id_seq OWNED BY public.device_topic_mappings.id;


--
-- Name: device_trigger_condition; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.device_trigger_condition (
    id character varying(36) NOT NULL,
    scene_automation_id character varying(36) NOT NULL,
    enabled character varying(10) NOT NULL,
    group_id character varying(36) NOT NULL,
    trigger_condition_type character varying(10) NOT NULL,
    trigger_source character varying(36),
    trigger_param_type character varying(10),
    trigger_param character varying(50),
    trigger_operator character varying(10),
    trigger_value character varying(1024) NOT NULL,
    remark character varying(255),
    tenant_id character varying(36) NOT NULL
);


--
-- Name: COLUMN device_trigger_condition.id; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.device_trigger_condition.id IS 'Id';


--
-- Name: COLUMN device_trigger_condition.scene_automation_id; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.device_trigger_condition.scene_automation_id IS '场景联动ID（外键-关联删除）';


--
-- Name: COLUMN device_trigger_condition.enabled; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.device_trigger_condition.enabled IS '是否启用';


--
-- Name: COLUMN device_trigger_condition.group_id; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.device_trigger_condition.group_id IS 'uuid';


--
-- Name: COLUMN device_trigger_condition.trigger_condition_type; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.device_trigger_condition.trigger_condition_type IS '条件类型 10：设备类型 - 单个设备 11：设备类型 - 单类设备 2：时间范围';


--
-- Name: COLUMN device_trigger_condition.trigger_source; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.device_trigger_condition.trigger_source IS '触发源有以下几种可能： 条件类型为10时，为设备id；条件类型为11时，设备配置id（device_config_id）';


--
-- Name: COLUMN device_trigger_condition.trigger_param_type; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.device_trigger_condition.trigger_param_type IS '遥测TEL属性ATTR事件EVT状态STATUS';


--
-- Name: COLUMN device_trigger_condition.trigger_param; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.device_trigger_condition.trigger_param IS '触发参数 当条件类型为10或11时有效，比如温度 temperature';


--
-- Name: COLUMN device_trigger_condition.trigger_operator; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.device_trigger_condition.trigger_operator IS '运算符 =：等于 !=：不等于 >：大于 <：小于 >=：大于等于 <=：小于等于 between：介于 in：包含在列表内';


--
-- Name: COLUMN device_trigger_condition.trigger_value; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.device_trigger_condition.trigger_value IS '取值条件类型为10,11，运算符是为7时，假设最大值6最小值2, 格式为2-6；设备状态条件类型为10,11，运算符为8时，多个值英文逗号隔开条件类型为 条件类型是22，示例137|HH:mm:ss+00:00|HH:mm:ss+00:00';


--
-- Name: COLUMN device_trigger_condition.tenant_id; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.device_trigger_condition.tenant_id IS '租户ID';


--
-- Name: device_user_logs; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.device_user_logs (
    id character varying(36) NOT NULL,
    device_nums integer DEFAULT 0 NOT NULL,
    device_on integer DEFAULT 0 NOT NULL,
    created_at timestamp(6) with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    tenant_id character varying(36) NOT NULL
);


--
-- Name: COLUMN device_user_logs.tenant_id; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.device_user_logs.tenant_id IS '租户 id';


--
-- Name: devices; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.devices (
    id character varying(36) NOT NULL,
    name character varying(255),
    voucher character varying(2048) DEFAULT ''::character varying NOT NULL,
    tenant_id character varying(36) DEFAULT ''::character varying NOT NULL,
    is_enabled character varying(36) DEFAULT ''::character varying NOT NULL,
    owner_user_id character varying(36),
    activate_flag character varying(36) DEFAULT ''::character varying NOT NULL,
    created_at timestamp(6) with time zone,
    update_at timestamp(6) with time zone,
    device_number character varying(100) DEFAULT ''::character varying NOT NULL,
    product_id character varying(36),
    parent_id character varying(36),
    protocol character varying(36),
    label character varying(255),
    location character varying(100),
    sub_device_addr character varying(36),
    current_version character varying(36),
    additional_info json DEFAULT '{}'::json,
    protocol_config json DEFAULT '{}'::json,
    remark1 character varying(255),
    remark2 character varying(255),
    remark3 character varying(255),
    device_config_id character varying(36),
    batch_number character varying(500),
    activate_at timestamp(6) with time zone,
    is_online smallint DEFAULT 0 NOT NULL,
    access_way character varying(10),
    description character varying(500),
    service_access_id character varying(36),
    last_offline_time timestamp(6) with time zone,
    voucher_hash character varying(64)
);


--
-- Name: COLUMN devices.id; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.devices.id IS 'Id';


--
-- Name: COLUMN devices.name; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.devices.name IS '设备名称';


--
-- Name: COLUMN devices.voucher; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.devices.voucher IS '凭证';


--
-- Name: COLUMN devices.tenant_id; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.devices.tenant_id IS '租户id，外键，删除时阻止';


--
-- Name: COLUMN devices.is_enabled; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.devices.is_enabled IS '启用/禁用 enabled-启用 disabled-禁用 默认禁用，激活后默认启用';


--
-- Name: COLUMN devices.owner_user_id; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.devices.owner_user_id IS '设备拥有者用户ID';


--
-- Name: COLUMN devices.activate_flag; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.devices.activate_flag IS '激活标志inactive-未激活 active-已激活';


--
-- Name: COLUMN devices.created_at; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.devices.created_at IS '创建时间';


--
-- Name: COLUMN devices.update_at; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.devices.update_at IS '更新时间';


--
-- Name: COLUMN devices.device_number; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.devices.device_number IS '设备编号 没送默认和token一样';


--
-- Name: COLUMN devices.product_id; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.devices.product_id IS '产品id 外键，删除时阻止';


--
-- Name: COLUMN devices.parent_id; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.devices.parent_id IS '子设备的网关id';


--
-- Name: COLUMN devices.protocol; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.devices.protocol IS '通讯协议';


--
-- Name: COLUMN devices.label; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.devices.label IS '标签 单标签，英文逗号隔开';


--
-- Name: COLUMN devices.location; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.devices.location IS '地理位置';


--
-- Name: COLUMN devices.sub_device_addr; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.devices.sub_device_addr IS '子设备地址';


--
-- Name: COLUMN devices.current_version; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.devices.current_version IS '当前固件版本';


--
-- Name: COLUMN devices.additional_info; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.devices.additional_info IS '其他信息 阈值、图片等';


--
-- Name: COLUMN devices.protocol_config; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.devices.protocol_config IS '协议表单配置';


--
-- Name: COLUMN devices.device_config_id; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.devices.device_config_id IS '设备配置id（外键）

';


--
-- Name: COLUMN devices.batch_number; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.devices.batch_number IS '批次编号
';


--
-- Name: COLUMN devices.activate_at; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.devices.activate_at IS '激活日期';


--
-- Name: COLUMN devices.is_online; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.devices.is_online IS '是否在线 1-在线 0-离线';


--
-- Name: COLUMN devices.access_way; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.devices.access_way IS '接入方式A-通过协议 B通过服务';


--
-- Name: COLUMN devices.description; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.devices.description IS '描述';


--
-- Name: COLUMN devices.last_offline_time; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.devices.last_offline_time IS '上次离线时间';


--
-- Name: COLUMN devices.voucher_hash; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.devices.voucher_hash IS '设备凭证 SHA-256 十六进制摘要（64 字符），存储哈希=缓存键算法（跨服务契约）；Phase 1 双模式窗口内 voucher 明文列仍为匹配兜底';


--
-- Name: edge_node_certificates; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.edge_node_certificates (
    id character varying(36) NOT NULL,
    tenant_id character varying(36) NOT NULL,
    node_id character varying(64) NOT NULL,
    serial_number character varying(64) NOT NULL,
    fingerprint character varying(64) NOT NULL,
    common_name character varying(128) NOT NULL,
    certificate text NOT NULL,
    not_before timestamp with time zone NOT NULL,
    not_after timestamp with time zone NOT NULL,
    status character varying(16) DEFAULT 'active'::character varying NOT NULL,
    issued_at timestamp with time zone NOT NULL,
    revoked_at timestamp with time zone,
    revoke_reason character varying(255),
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT edge_node_certificates_node_check CHECK (((node_id)::text <> ''::text)),
    CONSTRAINT edge_node_certificates_status_check CHECK (((status)::text = ANY ((ARRAY['active'::character varying, 'revoked'::character varying, 'expired'::character varying])::text[]))),
    CONSTRAINT edge_node_certificates_tenant_check CHECK (((tenant_id)::text <> ''::text))
);


--
-- Name: edge_node_upgrade_history; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.edge_node_upgrade_history (
    id character varying(36) NOT NULL,
    tenant_id character varying(36) NOT NULL,
    node_id character varying(64) NOT NULL,
    from_version character varying(32) NOT NULL,
    target_version character varying(32) NOT NULL,
    package_url character varying(512),
    checksum character varying(128),
    status character varying(16) DEFAULT 'pending'::character varying NOT NULL,
    operator_id character varying(36) NOT NULL,
    description text,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT edge_node_upgrade_node_check CHECK (((node_id)::text <> ''::text)),
    CONSTRAINT edge_node_upgrade_status_check CHECK (((status)::text = ANY ((ARRAY['pending'::character varying, 'dispatched'::character varying, 'success'::character varying, 'failed'::character varying, 'rolled_back'::character varying])::text[]))),
    CONSTRAINT edge_node_upgrade_tenant_check CHECK (((tenant_id)::text <> ''::text))
);


--
-- Name: edge_nodes; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.edge_nodes (
    id character varying(64) NOT NULL,
    tenant_id character varying(36) NOT NULL,
    version character varying(32) NOT NULL,
    capabilities text DEFAULT '[]'::text NOT NULL,
    status character varying(16) DEFAULT 'active'::character varying NOT NULL,
    last_seen_at timestamp with time zone,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT edge_nodes_status_check CHECK (((status)::text = ANY ((ARRAY['active'::character varying, 'revoked'::character varying])::text[]))),
    CONSTRAINT edge_nodes_tenant_check CHECK (((tenant_id)::text <> ''::text))
);


--
-- Name: edge_sync_tasks; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.edge_sync_tasks (
    id character varying(36) NOT NULL,
    tenant_id character varying(36) NOT NULL,
    gateway_device_id character varying(36) NOT NULL,
    gateway_device_number character varying(128) NOT NULL,
    resource_type character varying(32) NOT NULL,
    resource_id character varying(36) NOT NULL,
    payload text NOT NULL,
    status character varying(16) DEFAULT 'pending'::character varying NOT NULL,
    error text,
    attempts integer DEFAULT 0 NOT NULL,
    synced_at timestamp without time zone,
    created_at timestamp without time zone DEFAULT now() NOT NULL,
    updated_at timestamp without time zone DEFAULT now() NOT NULL
);


--
-- Name: email_templates; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.email_templates (
    id character varying(36) NOT NULL,
    tenant_id character varying(36) DEFAULT ''::character varying NOT NULL,
    name character varying(120) NOT NULL,
    purpose character varying(32) DEFAULT 'ALARM'::character varying NOT NULL,
    subject_template character varying(500) NOT NULL,
    body_template text NOT NULL,
    enabled boolean DEFAULT true NOT NULL,
    is_default boolean DEFAULT false NOT NULL,
    created_by character varying(36) NOT NULL,
    created_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    CONSTRAINT email_templates_purpose_check CHECK (((purpose)::text = 'ALARM'::text))
);


--
-- Name: TABLE email_templates; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON TABLE public.email_templates IS 'System and tenant scoped templates used to wrap alarm email subject and body';


--
-- Name: entity_relations; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.entity_relations (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    tenant_id character varying(36) NOT NULL,
    from_type character varying(32) NOT NULL,
    from_id character varying(36) NOT NULL,
    relation_type character varying(64) NOT NULL,
    to_type character varying(32) NOT NULL,
    to_id character varying(36) NOT NULL,
    metadata jsonb DEFAULT '{}'::jsonb NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT entity_relations_no_self_loop CHECK ((NOT (((from_type)::text = (to_type)::text) AND ((from_id)::text = (to_id)::text))))
);


--
-- Name: entity_versions; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.entity_versions (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    tenant_id character varying(36) NOT NULL,
    entity_type character varying(32) NOT NULL,
    entity_id character varying(36) NOT NULL,
    version_number integer NOT NULL,
    snapshot jsonb NOT NULL,
    remark character varying(500),
    created_by uuid,
    created_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: event_datas; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.event_datas (
    id character varying(36) NOT NULL,
    device_id character varying(36) NOT NULL,
    identify character varying(255) NOT NULL,
    ts timestamp(6) with time zone NOT NULL,
    data json,
    tenant_id character varying(36)
);


--
-- Name: COLUMN event_datas.device_id; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.event_datas.device_id IS '设备id（外键-关联删除）';


--
-- Name: COLUMN event_datas.identify; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.event_datas.identify IS '数据标识符';


--
-- Name: COLUMN event_datas.ts; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.event_datas.ts IS '上报时间';


--
-- Name: COLUMN event_datas.data; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.event_datas.data IS '数据';


--
-- Name: expected_datas; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.expected_datas (
    id character varying(36) NOT NULL,
    device_id character varying(36) NOT NULL,
    send_type character varying(50) NOT NULL,
    payload jsonb NOT NULL,
    created_at timestamp(6) with time zone NOT NULL,
    send_time timestamp(6) with time zone,
    status character varying(50) DEFAULT 'pending'::character varying NOT NULL,
    message text,
    expiry_time timestamp(6) with time zone,
    label character varying(100),
    tenant_id character varying(36) NOT NULL
);


--
-- Name: COLUMN expected_datas.id; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.expected_datas.id IS '指令唯一标识符(UUID)';


--
-- Name: COLUMN expected_datas.device_id; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.expected_datas.device_id IS '目标设备ID';


--
-- Name: COLUMN expected_datas.send_type; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.expected_datas.send_type IS '指令类型(e.g., telemetry, attribute, command)';


--
-- Name: COLUMN expected_datas.payload; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.expected_datas.payload IS '指令内容(具体指令参数)';


--
-- Name: COLUMN expected_datas.created_at; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.expected_datas.created_at IS '指令生成时间';


--
-- Name: COLUMN expected_datas.send_time; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.expected_datas.send_time IS '指令实际发送时间(如果已发送)';


--
-- Name: COLUMN expected_datas.status; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.expected_datas.status IS '指令状态(pending, sent, expired)，默认待发送';


--
-- Name: COLUMN expected_datas.message; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.expected_datas.message IS '状态附加信息(如发送失败的原因)';


--
-- Name: COLUMN expected_datas.expiry_time; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.expected_datas.expiry_time IS '指令过期时间(可选)';


--
-- Name: COLUMN expected_datas.label; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.expected_datas.label IS '指令标签(可选)';


--
-- Name: COLUMN expected_datas.tenant_id; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.expected_datas.tenant_id IS '租户ID（用于多租户系统）';


--
-- Name: fleet_saved_filters; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.fleet_saved_filters (
    id character varying(36) NOT NULL,
    tenant_id character varying(36) NOT NULL,
    user_id character varying(36) NOT NULL,
    name character varying(80) NOT NULL,
    device_filter jsonb NOT NULL,
    preview_total bigint,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    shared boolean DEFAULT false NOT NULL
);


--
-- Name: TABLE fleet_saved_filters; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON TABLE public.fleet_saved_filters IS 'Operator-owned saved fleet device_filter snapshots for Command Center and Fleet workflows';


--
-- Name: COLUMN fleet_saved_filters.device_filter; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.fleet_saved_filters.device_filter IS 'Normalized device_filter JSON contract reused by fleet command jobs';


--
-- Name: COLUMN fleet_saved_filters.shared; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.fleet_saved_filters.shared IS 'When true the filter is readable by every member of the same tenant; write access (update/delete) always stays with user_id, and the per-user save quota only counts rows owned by that user.';


--
-- Name: group_permissions; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.group_permissions (
    id character varying(36) NOT NULL,
    group_id character varying(36) NOT NULL,
    element_code character varying(100) NOT NULL,
    tenant_id character varying(36) NOT NULL,
    created_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL
);


--
-- Name: TABLE group_permissions; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON TABLE public.group_permissions IS '组权限元素绑定（TB-46 GPE v1；element_code=board:<id>/asset:<id> 资源元素，组共享的可见性映射）';


--
-- Name: COLUMN group_permissions.element_code; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.group_permissions.element_code IS '权限元素码：v1 支持 board:<board_id> / asset:<asset_id> 资源元素（组共享）';


--
-- Name: groups; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.groups (
    id character varying(36) NOT NULL,
    parent_id character varying(36) DEFAULT 0,
    tier integer DEFAULT 1 NOT NULL,
    name character varying(255) NOT NULL,
    description character varying(255),
    created_at timestamp(6) with time zone NOT NULL,
    updated_at timestamp(6) with time zone NOT NULL,
    remark character varying(255),
    tenant_id character varying(36) NOT NULL,
    owner_user_id character varying(36)
);


--
-- Name: COLUMN groups.parent_id; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.groups.parent_id IS '默认0是父分组';


--
-- Name: COLUMN groups.tier; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.groups.tier IS '层级 从1开始';


--
-- Name: COLUMN groups.name; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.groups.name IS '分组名称';


--
-- Name: COLUMN groups.description; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.groups.description IS '描述';


--
-- Name: COLUMN groups.created_at; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.groups.created_at IS '创建时间';


--
-- Name: COLUMN groups.updated_at; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.groups.updated_at IS '更新时间';


--
-- Name: COLUMN groups.tenant_id; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.groups.tenant_id IS '租户id';


--
-- Name: COLUMN groups.owner_user_id; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.groups.owner_user_id IS '分组拥有者用户ID';


--
-- Name: industry_solution_installs; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.industry_solution_installs (
    id character varying(36) NOT NULL,
    tenant_id character varying(36) NOT NULL,
    solution_id character varying(36) NOT NULL,
    solution_name character varying(128) NOT NULL,
    item_index integer NOT NULL,
    resource_type character varying(32) NOT NULL,
    resource_id character varying(36) NOT NULL,
    target_id character varying(36),
    status character varying(16) NOT NULL,
    error text,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT industry_solution_installs_status_check CHECK (((status)::text = ANY ((ARRAY['applied'::character varying, 'failed'::character varying])::text[]))),
    CONSTRAINT industry_solution_installs_tenant_check CHECK (((tenant_id)::text <> ''::text))
);


--
-- Name: TABLE industry_solution_installs; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON TABLE public.industry_solution_installs IS 'TB-19 方案安装流水（append-only，逐项 applied/failed 与目标实例 ID）';


--
-- Name: industry_solutions; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.industry_solutions (
    id character varying(36) NOT NULL,
    tenant_id character varying(36) NOT NULL,
    name character varying(128) NOT NULL,
    description character varying(512),
    resources jsonb NOT NULL,
    status character varying(16) DEFAULT 'active'::character varying NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT industry_solutions_name_check CHECK ((length((name)::text) > 0)),
    CONSTRAINT industry_solutions_status_check CHECK (((status)::text = ANY ((ARRAY['active'::character varying, 'disabled'::character varying])::text[]))),
    CONSTRAINT industry_solutions_tenant_check CHECK (((tenant_id)::text <> ''::text))
);


--
-- Name: TABLE industry_solutions; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON TABLE public.industry_solutions IS 'TB-19 行业方案模板（有序资源引用清单；安装复用资源中心应用管道，不复制内容）';


--
-- Name: integrations; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.integrations (
    id character varying(36) NOT NULL,
    name character varying(255) NOT NULL,
    tenant_id character varying(36) NOT NULL,
    connector_type character varying(32) NOT NULL,
    converter_uplink_id character varying(36),
    converter_downlink_id character varying(36),
    config jsonb DEFAULT '{}'::jsonb NOT NULL,
    enabled boolean DEFAULT true NOT NULL,
    created_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP,
    updated_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT chk_integrations_connector_type CHECK (((connector_type)::text = ANY ((ARRAY['opcua'::character varying, 'snmp'::character varying, 'plugin'::character varying])::text[])))
);


--
-- Name: latest_device_alarms; Type: VIEW; Schema: public; Owner: -
--

CREATE VIEW public.latest_device_alarms AS
 WITH ranked_device_states AS (
         SELECT current_device_alarm_streams.id,
            current_device_alarm_streams.alarm_config_id,
            current_device_alarm_streams.group_id,
            current_device_alarm_streams.scene_automation_id,
            current_device_alarm_streams.name,
            current_device_alarm_streams.description,
            current_device_alarm_streams.content,
            current_device_alarm_streams.alarm_status,
            current_device_alarm_streams.tenant_id,
            current_device_alarm_streams.remark,
            current_device_alarm_streams.create_at,
            current_device_alarm_streams.device_id,
            row_number() OVER (PARTITION BY current_device_alarm_streams.tenant_id, current_device_alarm_streams.device_id ORDER BY
                CASE
                    WHEN ((current_device_alarm_streams.alarm_status)::text = ANY ((ARRAY['H'::character varying, 'M'::character varying, 'L'::character varying])::text[])) THEN 0
                    ELSE 1
                END, current_device_alarm_streams.create_at DESC NULLS LAST, current_device_alarm_streams.id DESC) AS device_rn
           FROM public.current_device_alarm_streams
        )
 SELECT id,
    alarm_config_id,
    group_id,
    scene_automation_id,
    name,
    description,
    content,
    alarm_status,
    tenant_id,
    remark,
    create_at,
    device_id
   FROM ranked_device_states
  WHERE (device_rn = 1);


--
-- Name: VIEW latest_device_alarms; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON VIEW public.latest_device_alarms IS 'One current alarm summary per device; malformed device lists are ignored.';


--
-- Name: logo; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.logo (
    id character varying(36) NOT NULL,
    system_name character varying(99) NOT NULL,
    logo_cache character varying(255) NOT NULL,
    logo_background character varying(255) NOT NULL,
    logo_loading character varying(255) NOT NULL,
    home_background character varying(255) NOT NULL,
    remark character varying(255),
    tenant_id character varying(36) DEFAULT ''::character varying NOT NULL,
    theme_color character varying(32) DEFAULT ''::character varying,
    favicon character varying(255) DEFAULT ''::character varying
);


--
-- Name: COLUMN logo.id; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.logo.id IS 'Id';


--
-- Name: COLUMN logo.system_name; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.logo.system_name IS '系统名称';


--
-- Name: COLUMN logo.logo_cache; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.logo.logo_cache IS '站标Logo';


--
-- Name: COLUMN logo.logo_background; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.logo.logo_background IS '加载页面Logo';


--
-- Name: COLUMN logo.logo_loading; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.logo.logo_loading IS '加载页面Logo';


--
-- Name: COLUMN logo.home_background; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.logo.home_background IS '首页背景';


--
-- Name: COLUMN logo.tenant_id; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.logo.tenant_id IS '租户ID';


--
-- Name: COLUMN logo.theme_color; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.logo.theme_color IS '租户级主题色（#RRGGBB），空值回退前端默认主题';


--
-- Name: COLUMN logo.favicon; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.logo.favicon IS '页签 favicon URL，与站标 logo_cache 解耦';


--
-- Name: media_files; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.media_files (
    id character varying(36) NOT NULL,
    tenant_id character varying(36) NOT NULL,
    file_name character varying(255) NOT NULL,
    file_path character varying(500) NOT NULL,
    file_size bigint DEFAULT 0 NOT NULL,
    mime character varying(100) DEFAULT 'application/octet-stream'::character varying NOT NULL,
    referenced_count integer DEFAULT 0 NOT NULL,
    created_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP
);


--
-- Name: TABLE media_files; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON TABLE public.media_files IS '媒体文件登记表（TB-41 文件存储与媒体库，收编 ./files 上传链路）';


--
-- Name: COLUMN media_files.file_path; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.media_files.file_path IS '对外访问路径，UNIQUE(tenant_id, file_path) 防重复登记';


--
-- Name: COLUMN media_files.referenced_count; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.media_files.referenced_count IS '最近一次引用扫描的引用方数量（看板/SCADA 文档/OTA 升级包）';


--
-- Name: message_push_config; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.message_push_config (
    id character varying(60) NOT NULL,
    url character varying(255) NOT NULL,
    config_type smallint DEFAULT 1 NOT NULL,
    create_time timestamp(6) without time zone NOT NULL,
    update_time timestamp(6) without time zone
);


--
-- Name: TABLE message_push_config; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON TABLE public.message_push_config IS '消息推送配置';


--
-- Name: COLUMN message_push_config.url; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.message_push_config.url IS '推送地址';


--
-- Name: COLUMN message_push_config.config_type; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.message_push_config.config_type IS '配置类型 1 推送地址';


--
-- Name: COLUMN message_push_config.create_time; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.message_push_config.create_time IS '创建时间';


--
-- Name: COLUMN message_push_config.update_time; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.message_push_config.update_time IS '更新时间';


--
-- Name: message_push_log; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.message_push_log (
    id character varying(60) NOT NULL,
    user_id character varying(60) NOT NULL,
    message_type bigint NOT NULL,
    content json NOT NULL,
    status smallint NOT NULL,
    err_message character varying(255) NOT NULL,
    create_time timestamp(6) without time zone NOT NULL
);


--
-- Name: TABLE message_push_log; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON TABLE public.message_push_log IS '消息推送日志';


--
-- Name: COLUMN message_push_log.user_id; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.message_push_log.user_id IS '用户id';


--
-- Name: COLUMN message_push_log.message_type; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.message_push_log.message_type IS '消息类型 1告警消息';


--
-- Name: COLUMN message_push_log.content; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.message_push_log.content IS '消息体内容';


--
-- Name: COLUMN message_push_log.status; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.message_push_log.status IS '1推送成功 2推送失败';


--
-- Name: COLUMN message_push_log.err_message; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.message_push_log.err_message IS '错误信息';


--
-- Name: COLUMN message_push_log.create_time; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.message_push_log.create_time IS '发送时间';


--
-- Name: message_push_manage; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.message_push_manage (
    id character varying(60) NOT NULL,
    user_id character varying(60) NOT NULL,
    push_id character varying(60) NOT NULL,
    device_type character varying(60) NOT NULL,
    status smallint DEFAULT 1 NOT NULL,
    create_time timestamp(6) without time zone NOT NULL,
    update_time timestamp(6) without time zone,
    delete_time timestamp(6) without time zone,
    last_push_time timestamp(6) without time zone,
    err_count integer DEFAULT 0,
    inactive_time timestamp(6) without time zone
);


--
-- Name: TABLE message_push_manage; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON TABLE public.message_push_manage IS '消息推送通知';


--
-- Name: COLUMN message_push_manage.user_id; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.message_push_manage.user_id IS '用户id';


--
-- Name: COLUMN message_push_manage.push_id; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.message_push_manage.push_id IS '推送id';


--
-- Name: COLUMN message_push_manage.device_type; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.message_push_manage.device_type IS '设备类型';


--
-- Name: COLUMN message_push_manage.status; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.message_push_manage.status IS '类型 1正常 2注销';


--
-- Name: COLUMN message_push_manage.create_time; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.message_push_manage.create_time IS '创建类型';


--
-- Name: COLUMN message_push_manage.update_time; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.message_push_manage.update_time IS '更新时间';


--
-- Name: COLUMN message_push_manage.delete_time; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.message_push_manage.delete_time IS '删除时间';


--
-- Name: COLUMN message_push_manage.last_push_time; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.message_push_manage.last_push_time IS '最后一次推送时间';


--
-- Name: COLUMN message_push_manage.err_count; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.message_push_manage.err_count IS '联系推送错误次数';


--
-- Name: COLUMN message_push_manage.inactive_time; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.message_push_manage.inactive_time IS '标记不活跃时间';


--
-- Name: message_push_rule_log; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.message_push_rule_log (
    id character varying(60) NOT NULL,
    user_id character varying(60) NOT NULL,
    push_id character varying(60) NOT NULL,
    type smallint NOT NULL,
    create_time timestamp(6) without time zone NOT NULL
);


--
-- Name: TABLE message_push_rule_log; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON TABLE public.message_push_rule_log IS '失效规则记录';


--
-- Name: COLUMN message_push_rule_log.type; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.message_push_rule_log.type IS '1 主动失效 2被动失效 3定时任务 4自动清理';


--
-- Name: COLUMN message_push_rule_log.create_time; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.message_push_rule_log.create_time IS '生效时间';


--
-- Name: mobile_app_bundles; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.mobile_app_bundles (
    id character varying(36) NOT NULL,
    tenant_id character varying(36) NOT NULL,
    version character varying(50) NOT NULL,
    platform character varying(20) NOT NULL,
    file_name character varying(255) NOT NULL,
    file_path character varying(500) NOT NULL,
    file_size bigint DEFAULT 0 NOT NULL,
    checksum character varying(64) NOT NULL,
    release_notes text,
    status character varying(20) DEFAULT 'draft'::character varying NOT NULL,
    published_at timestamp with time zone,
    created_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    CONSTRAINT ck_mobile_app_bundles_platform CHECK (((platform)::text = ANY ((ARRAY['android'::character varying, 'ios'::character varying, 'h5'::character varying])::text[]))),
    CONSTRAINT ck_mobile_app_bundles_status CHECK (((status)::text = ANY ((ARRAY['draft'::character varying, 'published'::character varying, 'archived'::character varying])::text[])))
);


--
-- Name: TABLE mobile_app_bundles; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON TABLE public.mobile_app_bundles IS '移动应用包版本登记（TB-23；UNIQUE(tenant_id,platform,version)，状态机 draft→published→archived）';


--
-- Name: COLUMN mobile_app_bundles.platform; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.mobile_app_bundles.platform IS '目标平台：android/ios/h5（服务层按平台校验扩展名：apk/ipa/zip）';


--
-- Name: COLUMN mobile_app_bundles.file_path; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.mobile_app_bundles.file_path IS '文件对外访问路径（./files/apps/<platform>/<日期>/<哈希>.<ext>，磁盘位置同 BaseUploadDir 语义）';


--
-- Name: COLUMN mobile_app_bundles.checksum; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.mobile_app_bundles.checksum IS '文件 SHA-256 十六进制（上传时计算，入库后只读）';


--
-- Name: COLUMN mobile_app_bundles.status; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.mobile_app_bundles.status IS '发布状态机：draft（草稿，可改可删）→ published（已发布，不可改不可删）→ archived（已归档，可删）';


--
-- Name: COLUMN mobile_app_bundles.published_at; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.mobile_app_bundles.published_at IS '发布时间（draft→published 流转落 UTC 时刻；归档不清除，保留发布履历）';


--
-- Name: mqtt_session_revocation_acks; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.mqtt_session_revocation_acks (
    event_id character varying(36) NOT NULL,
    broker_id character varying(128) NOT NULL,
    device_id character varying(36) NOT NULL,
    revoked_at timestamp with time zone NOT NULL,
    processed_at timestamp with time zone NOT NULL,
    terminated_sessions integer DEFAULT 0 NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT mqtt_session_revocation_acks_broker_id_check CHECK ((btrim((broker_id)::text) <> ''::text)),
    CONSTRAINT mqtt_session_revocation_acks_device_id_check CHECK ((btrim((device_id)::text) <> ''::text)),
    CONSTRAINT mqtt_session_revocation_acks_terminated_sessions_check CHECK ((terminated_sessions >= 0))
);


--
-- Name: TABLE mqtt_session_revocation_acks; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON TABLE public.mqtt_session_revocation_acks IS 'Idempotent broker processing acknowledgements for durable MQTT session revocation events';


--
-- Name: mqtt_session_revocation_outbox; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.mqtt_session_revocation_outbox (
    id character varying(36) NOT NULL,
    device_id character varying(36) NOT NULL,
    revoked_at timestamp with time zone NOT NULL,
    status character varying(32) DEFAULT 'pending'::character varying NOT NULL,
    claim_token character varying(36),
    attempts integer DEFAULT 0 NOT NULL,
    last_error text,
    next_retry_at timestamp with time zone,
    published_at timestamp with time zone,
    subscriber_count bigint,
    required_broker_ids jsonb DEFAULT '[]'::jsonb NOT NULL,
    acknowledged_at timestamp with time zone,
    acknowledged_broker_count integer DEFAULT 0 NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT mqtt_session_revocation_outbox_acknowledged_broker_count_check CHECK ((acknowledged_broker_count >= 0)),
    CONSTRAINT mqtt_session_revocation_outbox_required_broker_ids_check CHECK ((jsonb_typeof(required_broker_ids) = 'array'::text)),
    CONSTRAINT mqtt_session_revocation_outbox_status_check CHECK (((status)::text = ANY ((ARRAY['pending'::character varying, 'processing'::character varying, 'awaiting_ack'::character varying, 'acknowledged'::character varying])::text[])))
);


--
-- Name: TABLE mqtt_session_revocation_outbox; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON TABLE public.mqtt_session_revocation_outbox IS 'Durable outbox for SW3-triggered MQTT session revocation publication';


--
-- Name: COLUMN mqtt_session_revocation_outbox.revoked_at; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.mqtt_session_revocation_outbox.revoked_at IS 'Device state-version cutoff; broker must not terminate sessions authenticated from a later device version';


--
-- Name: COLUMN mqtt_session_revocation_outbox.status; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.mqtt_session_revocation_outbox.status IS 'pending/processing/awaiting_ack/acknowledged; only acknowledged proves the configured broker acknowledgement policy';


--
-- Name: COLUMN mqtt_session_revocation_outbox.claim_token; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.mqtt_session_revocation_outbox.claim_token IS 'Per-claim fencing token; completion and retry writes must match the current processing owner';


--
-- Name: COLUMN mqtt_session_revocation_outbox.subscriber_count; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.mqtt_session_revocation_outbox.subscriber_count IS 'Redis subscriber count returned at publish time; not a delivery or session-termination acknowledgement';


--
-- Name: COLUMN mqtt_session_revocation_outbox.required_broker_ids; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.mqtt_session_revocation_outbox.required_broker_ids IS 'Broker IDs snapshotted when the event is created; an empty array accepts the first valid broker acknowledgement; migration marker rows are backfilled before worker delivery';


--
-- Name: COLUMN mqtt_session_revocation_outbox.acknowledged_broker_count; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.mqtt_session_revocation_outbox.acknowledged_broker_count IS 'Number of distinct persisted broker acknowledgements that contribute to the required-broker policy for this event';


--
-- Name: notification_groups; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.notification_groups (
    id character varying(36) NOT NULL,
    name character varying(99) NOT NULL,
    notification_type character varying(25) NOT NULL,
    status character varying(10) NOT NULL,
    notification_config jsonb,
    description character varying(255),
    tenant_id character varying(36) NOT NULL,
    created_at timestamp(6) with time zone NOT NULL,
    updated_at timestamp(6) with time zone NOT NULL,
    remark character varying(255)
);


--
-- Name: COLUMN notification_groups.name; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.notification_groups.name IS '名称';


--
-- Name: COLUMN notification_groups.notification_type; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.notification_groups.notification_type IS '通知类型MEMBER-成员通知 EMAIL-邮箱通知 SME-短信通知 VOICE-语音通知 WEBHOOK-webhook';


--
-- Name: COLUMN notification_groups.status; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.notification_groups.status IS '通知状态ON-启用 OFF-停用';


--
-- Name: COLUMN notification_groups.notification_config; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.notification_groups.notification_config IS '通知配置';


--
-- Name: COLUMN notification_groups.description; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.notification_groups.description IS '描述';


--
-- Name: COLUMN notification_groups.tenant_id; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.notification_groups.tenant_id IS '租户id';


--
-- Name: COLUMN notification_groups.created_at; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.notification_groups.created_at IS '创建时间';


--
-- Name: COLUMN notification_groups.updated_at; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.notification_groups.updated_at IS '更新时间';


--
-- Name: COLUMN notification_groups.remark; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.notification_groups.remark IS '备注';


--
-- Name: notification_histories; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.notification_histories (
    id character varying(36) NOT NULL,
    send_time timestamp(6) with time zone NOT NULL,
    send_content text,
    send_target character varying(255) NOT NULL,
    send_result character varying(25),
    notification_type character varying(25) NOT NULL,
    tenant_id character varying(36) NOT NULL,
    remark character varying(255)
);


--
-- Name: COLUMN notification_histories.send_time; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.notification_histories.send_time IS '发送时间';


--
-- Name: COLUMN notification_histories.send_content; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.notification_histories.send_content IS '发送内容';


--
-- Name: COLUMN notification_histories.send_target; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.notification_histories.send_target IS '发送目标';


--
-- Name: COLUMN notification_histories.send_result; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.notification_histories.send_result IS '发送结果SUCCESS-成功FAILURE-失败';


--
-- Name: COLUMN notification_histories.notification_type; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.notification_histories.notification_type IS '通知类型MEMBER-成员通知 EMAIL-邮箱通知 SME-短信通知 VOICE-语音通知 WEBHOOK-webhook';


--
-- Name: COLUMN notification_histories.tenant_id; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.notification_histories.tenant_id IS '租户id';


--
-- Name: COLUMN notification_histories.remark; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.notification_histories.remark IS '备注';


--
-- Name: notification_history_devices; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.notification_history_devices (
    notification_history_id character varying(36) NOT NULL,
    device_id character varying(36) NOT NULL,
    tenant_id character varying(36) NOT NULL
);


--
-- Name: TABLE notification_history_devices; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON TABLE public.notification_history_devices IS 'Devices whose data is included in a notification history entry';


--
-- Name: COLUMN notification_history_devices.device_id; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.notification_history_devices.device_id IS 'Historical device reference intentionally retained after a device row is deleted';


--
-- Name: notification_services_config; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.notification_services_config (
    id character varying(36) NOT NULL,
    config json,
    notice_type character varying(36) NOT NULL,
    status character varying(36) NOT NULL,
    remark character varying(255)
);


--
-- Name: COLUMN notification_services_config.config; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.notification_services_config.config IS '通知配置';


--
-- Name: COLUMN notification_services_config.notice_type; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.notification_services_config.notice_type IS '通知类型EMAIL-邮箱配置 SME-短信配置';


--
-- Name: COLUMN notification_services_config.status; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.notification_services_config.status IS '状态 OPEN-开启 CLOSE-关闭';


--
-- Name: one_time_tasks; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.one_time_tasks (
    id character varying(36) NOT NULL,
    scene_automation_id character varying(36) NOT NULL,
    execution_time timestamp(6) with time zone NOT NULL,
    executing_state character varying(10) NOT NULL,
    enabled character varying(10) NOT NULL,
    remark character varying(255),
    expiration_time bigint NOT NULL
);


--
-- Name: COLUMN one_time_tasks.scene_automation_id; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.one_time_tasks.scene_automation_id IS '场景联动ID（外键-关联删除）';


--
-- Name: COLUMN one_time_tasks.execution_time; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.one_time_tasks.execution_time IS '执行时间';


--
-- Name: COLUMN one_time_tasks.executing_state; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.one_time_tasks.executing_state IS '1.执行状态 NEX-未执行 EXE-已执行 EXP-过期未执行';


--
-- Name: COLUMN one_time_tasks.enabled; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.one_time_tasks.enabled IS '是否启用 Y-启用 N-停用';


--
-- Name: COLUMN one_time_tasks.expiration_time; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.one_time_tasks.expiration_time IS '过期时间（默认大于执行时间五分钟5min10min30min1h1day）单位分钟';


--
-- Name: open_api_keys; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.open_api_keys (
    id character varying(36) NOT NULL,
    tenant_id character varying(36) NOT NULL,
    api_key character varying(200) NOT NULL,
    status smallint,
    name character varying(200) NOT NULL,
    created_at timestamp(6) with time zone,
    updated_at timestamp(6) with time zone,
    created_id character varying(50),
    key_prefix character varying(20) DEFAULT ''::character varying NOT NULL
);


--
-- Name: COLUMN open_api_keys.key_prefix; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.open_api_keys.key_prefix IS '密钥展示前缀（sk_+8个十六进制字符），用于列表辨认，不含完整密钥';


--
-- Name: operation_logs; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.operation_logs (
    id character varying(36) NOT NULL,
    ip character varying(36) NOT NULL,
    path character varying(2000),
    user_id character varying(36) NOT NULL,
    name character varying(255),
    created_at timestamp(6) with time zone NOT NULL,
    latency bigint,
    request_message text,
    response_message text,
    tenant_id character varying(36) NOT NULL,
    remark character varying(255),
    action character varying(32),
    entity_type character varying(64),
    entity_id character varying(36),
    status_code integer
);


--
-- Name: COLUMN operation_logs.id; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.operation_logs.id IS 'Id';


--
-- Name: COLUMN operation_logs.ip; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.operation_logs.ip IS '请求IP';


--
-- Name: COLUMN operation_logs.path; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.operation_logs.path IS '请求url';


--
-- Name: COLUMN operation_logs.user_id; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.operation_logs.user_id IS '操作用户';


--
-- Name: COLUMN operation_logs.name; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.operation_logs.name IS '接口名称';


--
-- Name: COLUMN operation_logs.created_at; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.operation_logs.created_at IS '创建时间';


--
-- Name: COLUMN operation_logs.latency; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.operation_logs.latency IS '耗时(ms)';


--
-- Name: COLUMN operation_logs.request_message; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.operation_logs.request_message IS '请求内容';


--
-- Name: COLUMN operation_logs.response_message; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.operation_logs.response_message IS '响应内容';


--
-- Name: COLUMN operation_logs.tenant_id; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.operation_logs.tenant_id IS '租户id';


--
-- Name: COLUMN operation_logs.action; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.operation_logs.action IS '实体级动作（create/update/delete/read/other，映射自 HTTP 方法；旧数据为空）';


--
-- Name: COLUMN operation_logs.entity_type; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.operation_logs.entity_type IS '审计实体类型（/api/v1/<entity> 首段；旧数据为空）';


--
-- Name: COLUMN operation_logs.entity_id; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.operation_logs.entity_id IS '审计实体ID（路径第二段 UUID 形态；旧数据为空）';


--
-- Name: COLUMN operation_logs.status_code; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.operation_logs.status_code IS 'HTTP响应状态码（中间件 writer 状态；旧数据为空）';


--
-- Name: ota_upgrade_packages; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.ota_upgrade_packages (
    id character varying(36) NOT NULL,
    name character varying(200) NOT NULL,
    version character varying(36) NOT NULL,
    target_version character varying(36),
    device_config_id character varying(36) NOT NULL,
    module character varying(36),
    package_type smallint NOT NULL,
    signature_type character varying(36),
    additional_info json DEFAULT '{}'::json,
    description character varying(500),
    package_url character varying(500),
    created_at timestamp(6) with time zone NOT NULL,
    updated_at timestamp(6) with time zone,
    remark character varying(255),
    signature character varying(255),
    tenant_id character varying(36)
);


--
-- Name: COLUMN ota_upgrade_packages.id; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.ota_upgrade_packages.id IS 'Id';


--
-- Name: COLUMN ota_upgrade_packages.name; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.ota_upgrade_packages.name IS '升级包名称';


--
-- Name: COLUMN ota_upgrade_packages.version; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.ota_upgrade_packages.version IS '升级包版本号';


--
-- Name: COLUMN ota_upgrade_packages.target_version; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.ota_upgrade_packages.target_version IS '待升级版本号';


--
-- Name: COLUMN ota_upgrade_packages.device_config_id; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.ota_upgrade_packages.device_config_id IS '设备配置id';


--
-- Name: COLUMN ota_upgrade_packages.module; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.ota_upgrade_packages.module IS '模块名称';


--
-- Name: COLUMN ota_upgrade_packages.package_type; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.ota_upgrade_packages.package_type IS '升级包类型1-差分 2-整包';


--
-- Name: COLUMN ota_upgrade_packages.signature_type; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.ota_upgrade_packages.signature_type IS '签名算法MD5 SHA256';


--
-- Name: COLUMN ota_upgrade_packages.additional_info; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.ota_upgrade_packages.additional_info IS '附加信息';


--
-- Name: COLUMN ota_upgrade_packages.description; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.ota_upgrade_packages.description IS '描述';


--
-- Name: COLUMN ota_upgrade_packages.package_url; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.ota_upgrade_packages.package_url IS '包下载路径';


--
-- Name: COLUMN ota_upgrade_packages.created_at; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.ota_upgrade_packages.created_at IS '创建时间';


--
-- Name: COLUMN ota_upgrade_packages.updated_at; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.ota_upgrade_packages.updated_at IS '修改时间';


--
-- Name: COLUMN ota_upgrade_packages.remark; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.ota_upgrade_packages.remark IS '备注';


--
-- Name: COLUMN ota_upgrade_packages.signature; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.ota_upgrade_packages.signature IS '升级包签名';


--
-- Name: ota_upgrade_task_details; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.ota_upgrade_task_details (
    id character varying(36) NOT NULL,
    ota_upgrade_task_id character varying(200) NOT NULL,
    device_id character varying(200) NOT NULL,
    steps smallint,
    status smallint NOT NULL,
    status_description character varying(500),
    updated_at timestamp(6) with time zone,
    remark character varying(255),
    dispatch_attempts integer DEFAULT 0 NOT NULL,
    dispatch_lease_token character varying(64),
    dispatch_lease_until timestamp with time zone,
    last_dispatch_started_at timestamp with time zone,
    CONSTRAINT ota_upgrade_task_details_dispatch_attempts_check CHECK ((dispatch_attempts >= 0))
);


--
-- Name: COLUMN ota_upgrade_task_details.id; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.ota_upgrade_task_details.id IS 'Id';


--
-- Name: COLUMN ota_upgrade_task_details.ota_upgrade_task_id; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.ota_upgrade_task_details.ota_upgrade_task_id IS '升级任务id（外键关联删除）';


--
-- Name: COLUMN ota_upgrade_task_details.device_id; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.ota_upgrade_task_details.device_id IS '设备id（外键阻止删除）';


--
-- Name: COLUMN ota_upgrade_task_details.steps; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.ota_upgrade_task_details.steps IS '升级进度1-100';


--
-- Name: COLUMN ota_upgrade_task_details.status; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.ota_upgrade_task_details.status IS '状态1-待推送2-已推送3-升级中4-升级成功-5-升级失败-6已取消';


--
-- Name: COLUMN ota_upgrade_task_details.status_description; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.ota_upgrade_task_details.status_description IS '状态描述';


--
-- Name: COLUMN ota_upgrade_task_details.dispatch_lease_token; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.ota_upgrade_task_details.dispatch_lease_token IS 'Database claim token preventing concurrent backend instances from publishing the same pending row.';


--
-- Name: ota_upgrade_tasks; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.ota_upgrade_tasks (
    id character varying(36) NOT NULL,
    name character varying(200) NOT NULL,
    ota_upgrade_package_id character varying(36) NOT NULL,
    description character varying(500),
    created_at timestamp(6) with time zone NOT NULL,
    remark character varying(255),
    target_mode character varying(32) DEFAULT 'explicit'::character varying NOT NULL,
    target_filter jsonb,
    preview_total bigint,
    selected_count integer,
    created_by character varying(36),
    created_by_authority character varying(64),
    status character varying(32) DEFAULT 'running'::character varying NOT NULL,
    status_description text,
    scheduled_at timestamp with time zone,
    started_at timestamp with time zone,
    completed_at timestamp with time zone,
    timeout_at timestamp with time zone,
    timeout_seconds integer DEFAULT 3600 NOT NULL,
    rollout_rate_per_minute integer DEFAULT 60 NOT NULL,
    abort_failure_rate_percent numeric(5,2),
    next_dispatch_at timestamp with time zone,
    rate_window_started_at timestamp with time zone,
    rate_window_dispatched integer DEFAULT 0 NOT NULL,
    CONSTRAINT ota_upgrade_tasks_abort_failure_rate_check CHECK (((abort_failure_rate_percent IS NULL) OR ((abort_failure_rate_percent > (0)::numeric) AND (abort_failure_rate_percent <= (100)::numeric)))),
    CONSTRAINT ota_upgrade_tasks_rate_window_dispatched_check CHECK ((rate_window_dispatched >= 0)),
    CONSTRAINT ota_upgrade_tasks_rollout_rate_check CHECK (((rollout_rate_per_minute >= 1) AND (rollout_rate_per_minute <= 300))),
    CONSTRAINT ota_upgrade_tasks_status_check CHECK (((status)::text = ANY ((ARRAY['scheduled'::character varying, 'running'::character varying, 'completed'::character varying, 'partially_failed'::character varying, 'failed'::character varying, 'canceled'::character varying, 'aborted'::character varying, 'timed_out'::character varying])::text[]))),
    CONSTRAINT ota_upgrade_tasks_timeout_seconds_check CHECK (((timeout_seconds >= 60) AND (timeout_seconds <= 604800)))
);


--
-- Name: COLUMN ota_upgrade_tasks.id; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.ota_upgrade_tasks.id IS 'Id';


--
-- Name: COLUMN ota_upgrade_tasks.name; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.ota_upgrade_tasks.name IS '任务名称';


--
-- Name: COLUMN ota_upgrade_tasks.ota_upgrade_package_id; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.ota_upgrade_tasks.ota_upgrade_package_id IS '升级包id（外键，关联删除）';


--
-- Name: COLUMN ota_upgrade_tasks.description; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.ota_upgrade_tasks.description IS '描述';


--
-- Name: COLUMN ota_upgrade_tasks.created_at; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.ota_upgrade_tasks.created_at IS '创建时间';


--
-- Name: COLUMN ota_upgrade_tasks.remark; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.ota_upgrade_tasks.remark IS '备注';


--
-- Name: COLUMN ota_upgrade_tasks.target_mode; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.ota_upgrade_tasks.target_mode IS 'OTA target mode: explicit or filter';


--
-- Name: COLUMN ota_upgrade_tasks.target_filter; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.ota_upgrade_tasks.target_filter IS 'OTA target filter snapshot';


--
-- Name: COLUMN ota_upgrade_tasks.preview_total; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.ota_upgrade_tasks.preview_total IS 'Backend preview total at creation time';


--
-- Name: COLUMN ota_upgrade_tasks.selected_count; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.ota_upgrade_tasks.selected_count IS 'Selected device count at creation time';


--
-- Name: COLUMN ota_upgrade_tasks.created_by; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.ota_upgrade_tasks.created_by IS 'Creator user id';


--
-- Name: COLUMN ota_upgrade_tasks.created_by_authority; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.ota_upgrade_tasks.created_by_authority IS 'Creator authority';


--
-- Name: COLUMN ota_upgrade_tasks.status; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.ota_upgrade_tasks.status IS 'Durable rollout governance status; device-level progress remains in ota_upgrade_task_details.';


--
-- Name: COLUMN ota_upgrade_tasks.timeout_at; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.ota_upgrade_tasks.timeout_at IS 'Absolute rollout deadline set when a scheduled task enters running state.';


--
-- Name: COLUMN ota_upgrade_tasks.rollout_rate_per_minute; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.ota_upgrade_tasks.rollout_rate_per_minute IS 'Maximum device dispatch starts in one UTC minute; dispatch may occur in bounded batches.';


--
-- Name: COLUMN ota_upgrade_tasks.abort_failure_rate_percent; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.ota_upgrade_tasks.abort_failure_rate_percent IS 'Optional failure-rate threshold evaluated as failed / (succeeded + failed) * 100.';


--
-- Name: payload_schemas; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.payload_schemas (
    id character varying(36) NOT NULL,
    tenant_id character varying(36) NOT NULL,
    name character varying(128) NOT NULL,
    description character varying(500),
    strict boolean DEFAULT false NOT NULL,
    fields jsonb NOT NULL,
    created_by character varying(36),
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: TABLE payload_schemas; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON TABLE public.payload_schemas IS 'Tenant-scoped payload field-constraint registry reused by the static payload validation engine; broker-side enforcement of these schemas is a separate MQTT-contract change verified at runtime.';


--
-- Name: COLUMN payload_schemas.strict; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.payload_schemas.strict IS 'When true, a payload carrying keys not declared in fields is rejected rather than warned; this is the persisted default a broker enforcement layer would read.';


--
-- Name: COLUMN payload_schemas.fields; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.payload_schemas.fields IS 'Declared field constraints (name/type/required/min/max/enum/pattern) as a JSON array; the same shape the stateless validation engine consumes.';


--
-- Name: periodic_tasks; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.periodic_tasks (
    id character varying(36) NOT NULL,
    scene_automation_id character varying(36) NOT NULL,
    task_type character varying(255) NOT NULL,
    params character varying(50) NOT NULL,
    execution_time timestamp(6) with time zone NOT NULL,
    enabled character varying(10) NOT NULL,
    remark character varying(255),
    expiration_time bigint NOT NULL
);


--
-- Name: COLUMN periodic_tasks.scene_automation_id; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.periodic_tasks.scene_automation_id IS '场景联动ID（外键-关联删除）';


--
-- Name: COLUMN periodic_tasks.task_type; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.periodic_tasks.task_type IS '任务类型 HOUR DAY WEEK MONTH CRON';


--
-- Name: COLUMN periodic_tasks.execution_time; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.periodic_tasks.execution_time IS '执行时间';


--
-- Name: COLUMN periodic_tasks.enabled; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.periodic_tasks.enabled IS '是否启用 Y-启用 N-停用';


--
-- Name: COLUMN periodic_tasks.expiration_time; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.periodic_tasks.expiration_time IS '过期时间（默认大于执行时间五分钟）单位分钟';


--
-- Name: platform_cas; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.platform_cas (
    id character varying(36) NOT NULL,
    certificate text NOT NULL,
    private_key text NOT NULL,
    not_before timestamp without time zone NOT NULL,
    not_after timestamp without time zone NOT NULL,
    created_at timestamp without time zone DEFAULT now() NOT NULL,
    updated_at timestamp without time zone DEFAULT now() NOT NULL
);


--
-- Name: plugin_registries; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.plugin_registries (
    id character varying(36) NOT NULL,
    name character varying(128) NOT NULL,
    version character varying(64),
    transport character varying(32) DEFAULT 'grpc'::character varying NOT NULL,
    token_hash character varying(128) NOT NULL,
    status character varying(32) DEFAULT 'disabled'::character varying NOT NULL,
    last_heartbeat timestamp with time zone,
    description text,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    manifest text
);


--
-- Name: products; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.products (
    id character varying(36) NOT NULL,
    name character varying(255) NOT NULL,
    description character varying(255),
    product_type character varying(36),
    product_key character varying(255),
    product_model character varying(100),
    image_url character varying(500),
    created_at timestamp(6) with time zone NOT NULL,
    remark character varying(500),
    additional_info json,
    tenant_id character varying(36),
    device_config_id character varying(36)
);


--
-- Name: COLUMN products.id; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.products.id IS 'uuid';


--
-- Name: COLUMN products.name; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.products.name IS '产品名称';


--
-- Name: COLUMN products.description; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.products.description IS '描述';


--
-- Name: COLUMN products.product_type; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.products.product_type IS '产品类型';


--
-- Name: COLUMN products.product_key; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.products.product_key IS '产品key';


--
-- Name: COLUMN products.product_model; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.products.product_model IS '产品型号(编号)';


--
-- Name: COLUMN products.image_url; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.products.image_url IS '图片';


--
-- Name: COLUMN products.created_at; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.products.created_at IS '创建时间';


--
-- Name: COLUMN products.tenant_id; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.products.tenant_id IS '租户id';


--
-- Name: protocol_plugins; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.protocol_plugins (
    id character varying(36) NOT NULL,
    name character varying(36) NOT NULL,
    device_type smallint DEFAULT 1 NOT NULL,
    protocol_type character varying(50) NOT NULL,
    access_address character varying(500),
    http_address character varying(500),
    sub_topic_prefix character varying(500),
    description character varying(500),
    additional_info character varying(1000),
    created_at timestamp(6) with time zone NOT NULL,
    update_at timestamp(6) with time zone NOT NULL,
    remark character varying(255)
);


--
-- Name: COLUMN protocol_plugins.id; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.protocol_plugins.id IS 'Id';


--
-- Name: COLUMN protocol_plugins.name; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.protocol_plugins.name IS '插件名称';


--
-- Name: COLUMN protocol_plugins.device_type; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.protocol_plugins.device_type IS '接入设备类型 (1-直连设备 2-网关设备 默认直连设备)';


--
-- Name: COLUMN protocol_plugins.protocol_type; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.protocol_plugins.protocol_type IS '协议类型';


--
-- Name: COLUMN protocol_plugins.access_address; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.protocol_plugins.access_address IS '接入地址';


--
-- Name: COLUMN protocol_plugins.http_address; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.protocol_plugins.http_address IS 'HTTP服务地址';


--
-- Name: COLUMN protocol_plugins.sub_topic_prefix; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.protocol_plugins.sub_topic_prefix IS '插件订阅前缀';


--
-- Name: COLUMN protocol_plugins.description; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.protocol_plugins.description IS '描述';


--
-- Name: COLUMN protocol_plugins.additional_info; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.protocol_plugins.additional_info IS '附加信息';


--
-- Name: COLUMN protocol_plugins.created_at; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.protocol_plugins.created_at IS '创建时间';


--
-- Name: COLUMN protocol_plugins.update_at; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.protocol_plugins.update_at IS '更新时间';


--
-- Name: COLUMN protocol_plugins.remark; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.protocol_plugins.remark IS '备注';


--
-- Name: push_deliveries; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.push_deliveries (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    tenant_id character varying(36) NOT NULL,
    registration_id uuid,
    user_id character varying(36) NOT NULL,
    title character varying(255) NOT NULL,
    body text NOT NULL,
    data jsonb DEFAULT '{}'::jsonb NOT NULL,
    status character varying(16) DEFAULT 'pending'::character varying NOT NULL,
    attempt_count integer DEFAULT 0 NOT NULL,
    next_attempt_at timestamp with time zone,
    last_error text,
    provider character varying(32),
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT push_deliveries_attempt_count_non_negative CHECK ((attempt_count >= 0)),
    CONSTRAINT push_deliveries_status_check CHECK (((status)::text = ANY ((ARRAY['pending'::character varying, 'sent'::character varying, 'failed'::character varying, 'dead'::character varying])::text[])))
);


--
-- Name: COLUMN push_deliveries.status; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.push_deliveries.status IS 'pending | sent | failed (retryable) | dead (terminal, no further retry).';


--
-- Name: push_device_registrations; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.push_device_registrations (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    tenant_id character varying(36) NOT NULL,
    user_id character varying(36) NOT NULL,
    platform character varying(16) NOT NULL,
    token character varying(512) NOT NULL,
    provider character varying(32) DEFAULT 'fcm'::character varying NOT NULL,
    enabled boolean DEFAULT true NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT push_device_registrations_platform_check CHECK (((platform)::text = ANY ((ARRAY['ios'::character varying, 'android'::character varying, 'h5'::character varying])::text[])))
);


--
-- Name: r_group_device; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.r_group_device (
    group_id character varying(36) NOT NULL,
    device_id character varying(36) NOT NULL,
    tenant_id character varying(36) NOT NULL
);


--
-- Name: r_group_user; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.r_group_user (
    group_id character varying(36) NOT NULL,
    user_id character varying(36) NOT NULL,
    tenant_id character varying(36) NOT NULL,
    created_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL
);


--
-- Name: TABLE r_group_user; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON TABLE public.r_group_user IS '用户组成员关联（TB-46；user_id 仅限 users 登录账号，customer 客户不接入）';


--
-- Name: report_schedule_deliveries; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.report_schedule_deliveries (
    run_id character varying(36) NOT NULL,
    tenant_id character varying(36) NOT NULL,
    envelope_from text NOT NULL,
    envelope_recipients jsonb NOT NULL,
    message_id character varying(255) NOT NULL,
    subject text NOT NULL,
    payload bytea,
    payload_digest character varying(64) NOT NULL,
    payload_size bigint NOT NULL,
    row_count bigint NOT NULL,
    status character varying(16) DEFAULT 'pending'::character varying NOT NULL,
    attempt_count integer DEFAULT 0 NOT NULL,
    max_attempts integer DEFAULT 3 NOT NULL,
    next_attempt_at timestamp with time zone,
    claim_token uuid,
    lease_until timestamp with time zone,
    last_error text,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    started_at timestamp with time zone,
    accepted_at timestamp with time zone,
    failed_at timestamp with time zone,
    ambiguous_at timestamp with time zone,
    completed_at timestamp with time zone,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT report_schedule_deliveries_attempts_check CHECK (((attempt_count >= 0) AND (max_attempts >= 1) AND (attempt_count <= max_attempts))),
    CONSTRAINT report_schedule_deliveries_payload_check CHECK (((payload_size >= 0) AND (row_count >= 0) AND ((payload_digest)::text ~ '^[0-9a-f]{64}$'::text) AND ((((status)::text = ANY ((ARRAY['pending'::character varying, 'processing'::character varying, 'retrying'::character varying])::text[])) AND (payload IS NOT NULL) AND (payload_size = octet_length(payload))) OR (((status)::text = ANY ((ARRAY['accepted'::character varying, 'failed'::character varying, 'ambiguous'::character varying])::text[])) AND (payload IS NULL))))),
    CONSTRAINT report_schedule_deliveries_processing_lease_check CHECK (((((status)::text = 'processing'::text) AND (claim_token IS NOT NULL) AND (lease_until IS NOT NULL)) OR (((status)::text <> 'processing'::text) AND (claim_token IS NULL) AND (lease_until IS NULL)))),
    CONSTRAINT report_schedule_deliveries_recipients_check CHECK (((jsonb_typeof(envelope_recipients) = 'array'::text) AND (jsonb_array_length(envelope_recipients) > 0))),
    CONSTRAINT report_schedule_deliveries_status_check CHECK (((status)::text = ANY ((ARRAY['pending'::character varying, 'processing'::character varying, 'retrying'::character varying, 'accepted'::character varying, 'failed'::character varying, 'ambiguous'::character varying])::text[]))),
    CONSTRAINT report_schedule_deliveries_terminal_time_check CHECK (((((status)::text = 'accepted'::text) AND (accepted_at IS NOT NULL) AND (failed_at IS NULL) AND (ambiguous_at IS NULL) AND (completed_at IS NOT NULL)) OR (((status)::text = 'failed'::text) AND (accepted_at IS NULL) AND (failed_at IS NOT NULL) AND (ambiguous_at IS NULL) AND (completed_at IS NOT NULL)) OR (((status)::text = 'ambiguous'::text) AND (accepted_at IS NULL) AND (failed_at IS NULL) AND (ambiguous_at IS NOT NULL) AND (completed_at IS NOT NULL)) OR (((status)::text = ANY ((ARRAY['pending'::character varying, 'processing'::character varying, 'retrying'::character varying])::text[])) AND (accepted_at IS NULL) AND (failed_at IS NULL) AND (ambiguous_at IS NULL) AND (completed_at IS NULL))))
);


--
-- Name: report_schedule_runs; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.report_schedule_runs (
    id character varying(36) NOT NULL,
    tenant_id character varying(36) NOT NULL,
    schedule_id character varying(36) NOT NULL,
    trigger character varying(16) NOT NULL,
    scheduled_slot timestamp with time zone,
    retry_parent_run_id character varying(36),
    idempotency_key_hash character varying(64),
    request_fingerprint character varying(64),
    window_start_at timestamp with time zone NOT NULL,
    window_end_at timestamp with time zone NOT NULL,
    config_snapshot jsonb NOT NULL,
    misfire_count integer DEFAULT 0 NOT NULL,
    misfire_first_slot timestamp with time zone,
    misfire_last_slot timestamp with time zone,
    generation_status character varying(16) DEFAULT 'pending'::character varying NOT NULL,
    attempt_count integer DEFAULT 0 NOT NULL,
    max_attempts integer DEFAULT 3 NOT NULL,
    next_attempt_at timestamp with time zone,
    claim_token uuid,
    lease_until timestamp with time zone,
    result jsonb,
    error_code character varying(128),
    error_message text,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    started_at timestamp with time zone,
    generated_at timestamp with time zone,
    completed_at timestamp with time zone,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT report_schedule_runs_attempts_check CHECK (((attempt_count >= 0) AND (max_attempts >= 1) AND (attempt_count <= max_attempts))),
    CONSTRAINT report_schedule_runs_config_snapshot_check CHECK ((jsonb_typeof(config_snapshot) = 'object'::text)),
    CONSTRAINT report_schedule_runs_generation_status_check CHECK (((generation_status)::text = ANY ((ARRAY['pending'::character varying, 'processing'::character varying, 'retrying'::character varying, 'succeeded'::character varying, 'failed'::character varying])::text[]))),
    CONSTRAINT report_schedule_runs_idempotency_shape_check CHECK ((((idempotency_key_hash IS NULL) AND (request_fingerprint IS NULL)) OR ((idempotency_key_hash IS NOT NULL) AND (request_fingerprint IS NOT NULL) AND ((idempotency_key_hash)::text ~ '^[0-9a-f]{64}$'::text) AND ((request_fingerprint)::text ~ '^[0-9a-f]{64}$'::text)))),
    CONSTRAINT report_schedule_runs_misfire_check CHECK (((misfire_count >= 0) AND (((misfire_count = 0) AND (misfire_first_slot IS NULL) AND (misfire_last_slot IS NULL)) OR ((misfire_count > 0) AND (misfire_first_slot IS NOT NULL) AND (misfire_last_slot IS NOT NULL) AND (misfire_first_slot <= misfire_last_slot))))),
    CONSTRAINT report_schedule_runs_processing_lease_check CHECK (((((generation_status)::text = 'processing'::text) AND (claim_token IS NOT NULL) AND (lease_until IS NOT NULL)) OR (((generation_status)::text <> 'processing'::text) AND (claim_token IS NULL) AND (lease_until IS NULL)))),
    CONSTRAINT report_schedule_runs_terminal_time_check CHECK (((((generation_status)::text = ANY ((ARRAY['succeeded'::character varying, 'failed'::character varying])::text[])) AND (completed_at IS NOT NULL)) OR (((generation_status)::text <> ALL ((ARRAY['succeeded'::character varying, 'failed'::character varying])::text[])) AND (completed_at IS NULL)))),
    CONSTRAINT report_schedule_runs_trigger_check CHECK (((trigger)::text = ANY ((ARRAY['scheduled'::character varying, 'manual'::character varying, 'retry'::character varying])::text[]))),
    CONSTRAINT report_schedule_runs_trigger_shape_check CHECK (((((trigger)::text = 'scheduled'::text) AND (scheduled_slot IS NOT NULL) AND (retry_parent_run_id IS NULL)) OR (((trigger)::text = 'manual'::text) AND (scheduled_slot IS NULL) AND (retry_parent_run_id IS NULL)) OR (((trigger)::text = 'retry'::text) AND (scheduled_slot IS NULL) AND (retry_parent_run_id IS NOT NULL)))),
    CONSTRAINT report_schedule_runs_window_check CHECK ((window_start_at < window_end_at))
);


--
-- Name: report_schedules; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.report_schedules (
    id character varying(36) NOT NULL,
    tenant_id character varying(36) NOT NULL,
    name character varying(128) NOT NULL,
    cron_expr character varying(128) NOT NULL,
    recipients text NOT NULL,
    device_ids jsonb,
    keys jsonb,
    lookback_hours integer DEFAULT 24 NOT NULL,
    format character varying(16) DEFAULT 'csv'::character varying NOT NULL,
    enabled boolean DEFAULT true NOT NULL,
    last_run_at timestamp with time zone,
    last_status character varying(64),
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    timezone character varying(255) DEFAULT 'UTC'::character varying NOT NULL,
    next_run_at timestamp with time zone,
    revision bigint DEFAULT 1 NOT NULL,
    last_run_id character varying(36),
    schedule_error_code character varying(128),
    deleted_at timestamp with time zone,
    CONSTRAINT report_schedules_error_code_check CHECK (((schedule_error_code IS NULL) OR ((schedule_error_code)::text = 'invalid_schedule'::text))),
    CONSTRAINT report_schedules_revision_check CHECK ((revision >= 1))
);


--
-- Name: roles; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.roles (
    id character varying(36) NOT NULL,
    name character varying(99) NOT NULL,
    description character varying(255),
    created_at timestamp without time zone,
    updated_at timestamp without time zone,
    tenant_id character varying(36)
);


--
-- Name: COLUMN roles.id; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.roles.id IS 'Id';


--
-- Name: COLUMN roles.name; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.roles.name IS '名称';


--
-- Name: COLUMN roles.description; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.roles.description IS '描述';


--
-- Name: COLUMN roles.created_at; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.roles.created_at IS '创建时间';


--
-- Name: COLUMN roles.updated_at; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.roles.updated_at IS '更新时间';


--
-- Name: COLUMN roles.tenant_id; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.roles.tenant_id IS '租户id';


--
-- Name: rule_chain_checkpoints; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.rule_chain_checkpoints (
    id character varying(36) NOT NULL,
    exec_id character varying(36) NOT NULL,
    chain_id character varying(36) NOT NULL,
    node_id character varying(64) NOT NULL,
    tenant_id character varying(36) NOT NULL,
    device_id character varying(36),
    payload jsonb NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: rule_chain_dead_letters; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.rule_chain_dead_letters (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    tenant_id text NOT NULL,
    chain_id text NOT NULL,
    exec_id text NOT NULL,
    node_id text NOT NULL,
    node_type text NOT NULL,
    device_id text,
    error text,
    attempts integer DEFAULT 1 NOT NULL,
    created_at timestamp with time zone DEFAULT clock_timestamp() NOT NULL
);


--
-- Name: TABLE rule_chain_dead_letters; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON TABLE public.rule_chain_dead_letters IS 'P1.2 规则链节点终局失败死信记录（审计最小化，不含业务载荷）';


--
-- Name: rule_chain_edges; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.rule_chain_edges (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    chain_id uuid NOT NULL,
    source_node_key character varying(64) NOT NULL,
    target_node_key character varying(64) NOT NULL,
    label character varying(64) DEFAULT ''::character varying
);


--
-- Name: rule_chain_node_traces; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.rule_chain_node_traces (
    id character varying(36) NOT NULL,
    exec_id character varying(36) NOT NULL,
    chain_id character varying(36) NOT NULL,
    node_id character varying(64) NOT NULL,
    node_type character varying(64) NOT NULL,
    pass boolean DEFAULT true NOT NULL,
    error_msg text,
    elapsed_ms bigint DEFAULT 0 NOT NULL,
    tenant_id character varying(36) NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: rule_chain_nodes; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.rule_chain_nodes (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    chain_id uuid NOT NULL,
    node_key character varying(64) NOT NULL,
    node_type character varying(20) NOT NULL,
    subtype character varying(64) DEFAULT ''::character varying NOT NULL,
    label character varying(128) DEFAULT ''::character varying,
    config jsonb DEFAULT '{}'::jsonb NOT NULL,
    position_x double precision DEFAULT 0 NOT NULL,
    position_y double precision DEFAULT 0 NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: rule_chain_replay_records; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.rule_chain_replay_records (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    tenant_id text NOT NULL,
    chain_id text NOT NULL,
    execution_id text NOT NULL,
    node_id text NOT NULL,
    node_type text NOT NULL,
    payload jsonb DEFAULT '{}'::jsonb NOT NULL,
    metadata jsonb DEFAULT '{}'::jsonb NOT NULL,
    pass boolean NOT NULL,
    error text,
    recorded_at timestamp with time zone NOT NULL,
    created_at timestamp with time zone DEFAULT clock_timestamp() NOT NULL
);


--
-- Name: TABLE rule_chain_replay_records; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON TABLE public.rule_chain_replay_records IS 'P1.2 规则链回放输入快照；仅在运维显式开启回放留存时写入，默认不产生数据。';


--
-- Name: rule_chain_versions; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.rule_chain_versions (
    id character varying(36) NOT NULL,
    tenant_id character varying(36) NOT NULL,
    chain_id character varying(36) NOT NULL,
    version integer NOT NULL,
    status character varying(16) DEFAULT 'draft'::character varying NOT NULL,
    graph_hash character varying(128) NOT NULL,
    graph jsonb,
    rolled_back_from integer,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    published_at timestamp with time zone,
    CONSTRAINT rule_chain_versions_status_check CHECK (((status)::text = ANY ((ARRAY['draft'::character varying, 'published'::character varying, 'archived'::character varying])::text[])))
);


--
-- Name: rule_chains; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.rule_chains (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    name character varying(128) NOT NULL,
    description text DEFAULT ''::text,
    enabled boolean DEFAULT true NOT NULL,
    tenant_id character varying(36) NOT NULL,
    created_by uuid,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    graph jsonb DEFAULT '{"edges": [], "nodes": []}'::jsonb NOT NULL
);


--
-- Name: scada_control_audits; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.scada_control_audits (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    tenant_id character varying(36) NOT NULL,
    document_id uuid NOT NULL,
    widget_id character varying(64) NOT NULL,
    command character varying(64) NOT NULL,
    params jsonb DEFAULT '{}'::jsonb NOT NULL,
    actor_user_id character varying(36) NOT NULL,
    confirmation_token character varying(255) NOT NULL,
    outcome character varying(16) NOT NULL,
    detail text,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT scada_control_audits_outcome_check CHECK (((outcome)::text = ANY ((ARRAY['pending'::character varying, 'success'::character varying, 'denied'::character varying, 'failed'::character varying])::text[])))
);


--
-- Name: COLUMN scada_control_audits.confirmation_token; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.scada_control_audits.confirmation_token IS '签发的二次确认令牌全文（<过期秒>.<HMAC-SHA256 hex>，约 75 字符）；denied 空串';


--
-- Name: COLUMN scada_control_audits.outcome; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.scada_control_audits.outcome IS 'success | denied | failed; denials are recorded too, otherwise rejected attempts leave no trace.';


--
-- Name: scada_document_versions; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.scada_document_versions (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    tenant_id character varying(36) NOT NULL,
    document_id uuid NOT NULL,
    version integer NOT NULL,
    json_data jsonb NOT NULL,
    published_by character varying(36),
    published_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT scada_document_versions_version_positive CHECK ((version > 0))
);


--
-- Name: scada_documents; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.scada_documents (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    tenant_id character varying(36) NOT NULL,
    project_id uuid NOT NULL,
    name character varying(128) NOT NULL,
    status character varying(16) DEFAULT 'DRAFT'::character varying NOT NULL,
    current_version integer DEFAULT 1 NOT NULL,
    published_version integer,
    json_data jsonb DEFAULT '{}'::jsonb NOT NULL,
    created_by character varying(36),
    updated_by character varying(36),
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT scada_documents_published_not_ahead CHECK (((published_version IS NULL) OR (published_version <= current_version))),
    CONSTRAINT scada_documents_status_check CHECK (((status)::text = ANY ((ARRAY['DRAFT'::character varying, 'PUBLISHED'::character varying, 'ARCHIVED'::character varying])::text[]))),
    CONSTRAINT scada_documents_version_positive CHECK ((current_version > 0))
);


--
-- Name: COLUMN scada_documents.current_version; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.scada_documents.current_version IS 'Draft version, incremented on every save; used as the optimistic-concurrency token.';


--
-- Name: COLUMN scada_documents.published_version; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.scada_documents.published_version IS 'Version currently published; NULL means never published, which is not the same as version 0.';


--
-- Name: scada_projects; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.scada_projects (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    tenant_id character varying(36) NOT NULL,
    name character varying(128) NOT NULL,
    description text,
    created_by character varying(36),
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: COLUMN scada_projects.tenant_id; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.scada_projects.tenant_id IS 'Tenant owner; part of the unique key so identical project names in different tenants coexist.';


--
-- Name: scene_action_info; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.scene_action_info (
    id character varying(36) NOT NULL,
    scene_id character varying(36) NOT NULL,
    action_target character varying(36) NOT NULL,
    action_type character varying(10) NOT NULL,
    action_param_type character varying(20),
    action_param character varying(50),
    action_value character varying(255),
    created_at timestamp(6) with time zone NOT NULL,
    updated_at timestamp(6) with time zone,
    tenant_id character varying(36) NOT NULL,
    remark character varying(255)
);


--
-- Name: COLUMN scene_action_info.scene_id; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.scene_action_info.scene_id IS '场景id（关联删除）';


--
-- Name: COLUMN scene_action_info.action_target; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.scene_action_info.action_target IS '动作目标id设备id、设备配置id，场景id、告警id';


--
-- Name: COLUMN scene_action_info.action_type; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.scene_action_info.action_type IS '动作类型10: 单个设备11: 单类设备20: 激活场景30: 触发告警40: 服务';


--
-- Name: COLUMN scene_action_info.action_param_type; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.scene_action_info.action_param_type IS '1.参数类型TEL:遥测 2.ATTR:属性 CMD:命令';


--
-- Name: COLUMN scene_action_info.action_param; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.scene_action_info.action_param IS '动作参数';


--
-- Name: COLUMN scene_action_info.action_value; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.scene_action_info.action_value IS '目标值';


--
-- Name: COLUMN scene_action_info.created_at; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.scene_action_info.created_at IS '创建时间';


--
-- Name: COLUMN scene_action_info.updated_at; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.scene_action_info.updated_at IS '更新时间';


--
-- Name: scene_automation_log; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.scene_automation_log (
    scene_automation_id character varying(36) NOT NULL,
    executed_at timestamp(6) with time zone NOT NULL,
    detail text NOT NULL,
    execution_result character varying(10) NOT NULL,
    tenant_id character varying(36) NOT NULL,
    remark character varying(255)
);


--
-- Name: COLUMN scene_automation_log.scene_automation_id; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.scene_automation_log.scene_automation_id IS '场景联动ID（外键-关联删除）';


--
-- Name: COLUMN scene_automation_log.executed_at; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.scene_automation_log.executed_at IS '执行时间';


--
-- Name: COLUMN scene_automation_log.detail; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.scene_automation_log.detail IS '执行说明：详细的执行过程';


--
-- Name: COLUMN scene_automation_log.execution_result; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.scene_automation_log.execution_result IS '执行状态S：成功F：失败 全部执行成功才算';


--
-- Name: scene_automation_timers; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.scene_automation_timers (
    id text NOT NULL,
    tenant_id text NOT NULL,
    scene_automation_id text NOT NULL,
    cron_expr text NOT NULL,
    timezone text DEFAULT 'UTC'::text NOT NULL,
    enabled boolean DEFAULT true NOT NULL,
    next_run_at timestamp with time zone NOT NULL,
    last_run_at timestamp with time zone,
    lease_owner text,
    lease_until timestamp with time zone,
    consecutive_failures integer DEFAULT 0 NOT NULL,
    last_error text,
    created_at timestamp with time zone DEFAULT clock_timestamp() NOT NULL,
    updated_at timestamp with time zone DEFAULT clock_timestamp() NOT NULL
);


--
-- Name: TABLE scene_automation_timers; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON TABLE public.scene_automation_timers IS 'P0.4 持久化的场景定时触发；next_run_at 与租约共同保证重启后不丢任务。';


--
-- Name: COLUMN scene_automation_timers.lease_until; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.scene_automation_timers.lease_until IS '租约到期时刻。超过该时刻仍未被释放，视为领取方已崩溃，可被重新领取。';


--
-- Name: scene_automations; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.scene_automations (
    id character varying(36) NOT NULL,
    name character varying(255) NOT NULL,
    description character varying(255),
    enabled character varying(10) NOT NULL,
    tenant_id character varying(36) NOT NULL,
    creator character varying(36) NOT NULL,
    updator character varying(36) NOT NULL,
    created_at timestamp(6) with time zone NOT NULL,
    updated_at timestamp(6) with time zone,
    remark character varying(255),
    execution_starts_at timestamp with time zone,
    execution_expires_at timestamp with time zone,
    execution_timezone text,
    CONSTRAINT scene_automations_execution_timezone_not_blank CHECK (((execution_timezone IS NULL) OR (btrim(execution_timezone) <> ''::text)))
);


--
-- Name: COLUMN scene_automations.id; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.scene_automations.id IS '联动';


--
-- Name: COLUMN scene_automations.name; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.scene_automations.name IS '名称';


--
-- Name: COLUMN scene_automations.description; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.scene_automations.description IS '描述';


--
-- Name: COLUMN scene_automations.enabled; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.scene_automations.enabled IS '是否启用 Y：启用 N：停用';


--
-- Name: COLUMN scene_automations.tenant_id; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.scene_automations.tenant_id IS '租户ID';


--
-- Name: COLUMN scene_automations.creator; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.scene_automations.creator IS '创建人id';


--
-- Name: COLUMN scene_automations.updator; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.scene_automations.updator IS '修改人id';


--
-- Name: COLUMN scene_automations.created_at; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.scene_automations.created_at IS '创建时间';


--
-- Name: COLUMN scene_automations.updated_at; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.scene_automations.updated_at IS '更新时间';


--
-- Name: COLUMN scene_automations.execution_starts_at; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.scene_automations.execution_starts_at IS 'P0.4 执行窗口下界，NULL 表示无下界；区间为左闭右开 [starts_at, expires_at)。';


--
-- Name: COLUMN scene_automations.execution_expires_at; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.scene_automations.execution_expires_at IS 'P0.4 执行窗口上界，NULL 表示无上界；恰好等于该时刻不再执行。';


--
-- Name: COLUMN scene_automations.execution_timezone; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.scene_automations.execution_timezone IS 'P0.4 执行窗口时区（IANA 名）。NULL/空按 UTC；非法值在服务层 fail closed，不静默兜底。';


--
-- Name: scene_info; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.scene_info (
    id character varying(36) NOT NULL,
    name character varying(255) NOT NULL,
    description character varying(255),
    tenant_id character varying(36) NOT NULL,
    creator character varying(36) NOT NULL,
    updator character varying(36),
    created_at timestamp(6) with time zone NOT NULL,
    updated_at timestamp(6) with time zone
);


--
-- Name: COLUMN scene_info.name; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.scene_info.name IS '名称';


--
-- Name: COLUMN scene_info.description; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.scene_info.description IS '描述';


--
-- Name: COLUMN scene_info.tenant_id; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.scene_info.tenant_id IS '租户ID';


--
-- Name: COLUMN scene_info.creator; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.scene_info.creator IS '创建人ID';


--
-- Name: COLUMN scene_info.updator; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.scene_info.updator IS '修改人ID';


--
-- Name: COLUMN scene_info.created_at; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.scene_info.created_at IS '创建时间';


--
-- Name: COLUMN scene_info.updated_at; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.scene_info.updated_at IS '更新时间';


--
-- Name: scene_log; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.scene_log (
    scene_id character varying(36) NOT NULL,
    executed_at timestamp(6) with time zone NOT NULL,
    detail text NOT NULL,
    execution_result character varying(10) NOT NULL,
    tenant_id character varying(36) NOT NULL,
    remark character varying(255),
    id character varying(36) NOT NULL
);


--
-- Name: COLUMN scene_log.scene_id; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.scene_log.scene_id IS '场景id（关联删除）';


--
-- Name: COLUMN scene_log.executed_at; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.scene_log.executed_at IS '执行时间';


--
-- Name: COLUMN scene_log.detail; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.scene_log.detail IS '执行说明：详细的执行过程';


--
-- Name: COLUMN scene_log.execution_result; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.scene_log.execution_result IS '执行状态S：成功F：失败 全部执行成功才算成功';


--
-- Name: scheduler_events; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.scheduler_events (
    id character varying(36) NOT NULL,
    tenant_id character varying(36) NOT NULL,
    name character varying(128) NOT NULL,
    event_type character varying(20) NOT NULL,
    ref_type character varying(50),
    ref_id character varying(64),
    cron character varying(64),
    next_run_at timestamp with time zone,
    enabled boolean DEFAULT true NOT NULL,
    created_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    CONSTRAINT ck_scheduler_events_event_type CHECK (((event_type)::text = ANY ((ARRAY['scene'::character varying, 'report'::character varying, 'rpc'::character varying])::text[])))
);


--
-- Name: TABLE scheduler_events; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON TABLE public.scheduler_events IS 'TB-48 统一调度事件注册面；scene 事件同步 scene_automation_timers 执行，report/rpc 注册行仅做统一登记展示';


--
-- Name: COLUMN scheduler_events.event_type; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.scheduler_events.event_type IS '事件类型：scene（场景联动，落到 scene_automation_timers 执行）/ report（定时报表登记）/ rpc（一次性命令登记）';


--
-- Name: COLUMN scheduler_events.ref_type; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.scheduler_events.ref_type IS '目标类型：scene_automation / report_schedule / fleet_command_job 或自由标注；scene 事件固定 scene_automation';


--
-- Name: COLUMN scheduler_events.ref_id; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.scheduler_events.ref_id IS '目标实体 ID；scene 事件为场景自动化 ID（服务层校验同租户存在）';


--
-- Name: COLUMN scheduler_events.cron; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.scheduler_events.cron IS '5/6 段 cron 表达式；rpc 一次性事件为空、直接落 next_run_at';


--
-- Name: COLUMN scheduler_events.next_run_at; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.scheduler_events.next_run_at IS '下一次触发时刻；scene/report 事件由服务层按 cron 以 UTC 计算并随更新重算，rpc 直接给定';


--
-- Name: COLUMN scheduler_events.enabled; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.scheduler_events.enabled IS '启用开关；scene 事件与同名 id 的 scene_automation_timers 行 enabled 同步';


--
-- Name: service_access; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.service_access (
    id character varying(36) NOT NULL,
    name character varying(100) NOT NULL,
    service_plugin_id character varying(36) NOT NULL,
    voucher character varying(999) NOT NULL,
    description character varying(255),
    service_access_config json,
    remark character varying(255),
    create_at timestamp with time zone NOT NULL,
    update_at timestamp with time zone NOT NULL,
    tenant_id character varying(36) NOT NULL
);


--
-- Name: TABLE service_access; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON TABLE public.service_access IS '服务接入(租户端)';


--
-- Name: COLUMN service_access.id; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.service_access.id IS '接入ID';


--
-- Name: COLUMN service_access.name; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.service_access.name IS '名称';


--
-- Name: COLUMN service_access.service_plugin_id; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.service_access.service_plugin_id IS '服务ID';


--
-- Name: COLUMN service_access.voucher; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.service_access.voucher IS '凭证';


--
-- Name: COLUMN service_access.description; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.service_access.description IS '描述';


--
-- Name: COLUMN service_access.service_access_config; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.service_access.service_access_config IS '服务配置';


--
-- Name: COLUMN service_access.remark; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.service_access.remark IS '备注';


--
-- Name: COLUMN service_access.create_at; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.service_access.create_at IS '创建时间';


--
-- Name: COLUMN service_access.update_at; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.service_access.update_at IS '更新时间';


--
-- Name: COLUMN service_access.tenant_id; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.service_access.tenant_id IS '租户ID';


--
-- Name: service_plugins; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.service_plugins (
    id character varying(36) NOT NULL,
    name character varying(255) NOT NULL,
    service_identifier character varying(100) NOT NULL,
    service_type integer NOT NULL,
    last_active_time timestamp with time zone,
    version character varying(100),
    create_at timestamp with time zone NOT NULL,
    update_at timestamp with time zone NOT NULL,
    description character varying(255),
    service_config json,
    remark character varying(255),
    CONSTRAINT service_plugins_service_type_check CHECK ((service_type = ANY (ARRAY[1, 2])))
);


--
-- Name: TABLE service_plugins; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON TABLE public.service_plugins IS '服务管理';


--
-- Name: COLUMN service_plugins.id; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.service_plugins.id IS '服务ID';


--
-- Name: COLUMN service_plugins.name; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.service_plugins.name IS '服务名称';


--
-- Name: COLUMN service_plugins.service_identifier; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.service_plugins.service_identifier IS '服务标识符';


--
-- Name: COLUMN service_plugins.service_type; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.service_plugins.service_type IS '服务类型: 1-接入协议, 2-接入服务';


--
-- Name: COLUMN service_plugins.last_active_time; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.service_plugins.last_active_time IS '服务最后活跃时间';


--
-- Name: COLUMN service_plugins.version; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.service_plugins.version IS '版本号';


--
-- Name: COLUMN service_plugins.create_at; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.service_plugins.create_at IS '创建时间';


--
-- Name: COLUMN service_plugins.update_at; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.service_plugins.update_at IS '更新时间';


--
-- Name: COLUMN service_plugins.description; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.service_plugins.description IS '描述';


--
-- Name: COLUMN service_plugins.service_config; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.service_plugins.service_config IS '服务配置: 接入协议和接入服务的配置';


--
-- Name: COLUMN service_plugins.remark; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.service_plugins.remark IS '备注';


--
-- Name: subscription_plans; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.subscription_plans (
    id character varying(36) NOT NULL,
    code character varying(50) NOT NULL,
    name character varying(100) NOT NULL,
    description text,
    price_monthly numeric(10,2) DEFAULT 0.00 NOT NULL,
    currency character varying(10) DEFAULT 'USD'::character varying NOT NULL,
    max_devices integer DEFAULT 10 NOT NULL,
    max_tenants integer DEFAULT 1 NOT NULL,
    max_users integer DEFAULT 3 NOT NULL,
    max_telemetry_per_day integer DEFAULT 10000 NOT NULL,
    max_api_calls_per_day integer DEFAULT 5000 NOT NULL,
    features jsonb DEFAULT '[]'::jsonb NOT NULL,
    enabled smallint DEFAULT 1 NOT NULL,
    created_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL
);


--
-- Name: sys_config; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.sys_config (
    config_key character varying(255) NOT NULL,
    config_value text NOT NULL,
    remark character varying(255),
    created_at timestamp(6) with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at timestamp(6) with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL
);


--
-- Name: TABLE sys_config; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON TABLE public.sys_config IS '系统配置表';


--
-- Name: COLUMN sys_config.config_key; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.sys_config.config_key IS '配置键';


--
-- Name: COLUMN sys_config.config_value; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.sys_config.config_value IS '配置值';


--
-- Name: sys_dict; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.sys_dict (
    id character varying(36) NOT NULL,
    dict_code character varying(36) NOT NULL,
    dict_value character varying(255) NOT NULL,
    created_at timestamp(6) with time zone NOT NULL,
    remark character varying(255)
);


--
-- Name: COLUMN sys_dict.id; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.sys_dict.id IS '主键ID';


--
-- Name: COLUMN sys_dict.dict_code; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.sys_dict.dict_code IS '字典标识符';


--
-- Name: COLUMN sys_dict.dict_value; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.sys_dict.dict_value IS '字典值';


--
-- Name: COLUMN sys_dict.created_at; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.sys_dict.created_at IS '创建时间';


--
-- Name: COLUMN sys_dict.remark; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.sys_dict.remark IS '备注';


--
-- Name: sys_dict_language; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.sys_dict_language (
    id character varying(36) NOT NULL,
    dict_id character varying(36) NOT NULL,
    language_code character varying(36) NOT NULL,
    translation character varying(255) NOT NULL
);


--
-- Name: COLUMN sys_dict_language.id; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.sys_dict_language.id IS '主键ID';


--
-- Name: COLUMN sys_dict_language.dict_id; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.sys_dict_language.dict_id IS 'sys_dict.id';


--
-- Name: COLUMN sys_dict_language.language_code; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.sys_dict_language.language_code IS '语言代码';


--
-- Name: COLUMN sys_dict_language.translation; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.sys_dict_language.translation IS '翻译';


--
-- Name: sys_function; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.sys_function (
    id character varying(36) NOT NULL,
    name character varying(50) NOT NULL,
    enable_flag character varying(20) NOT NULL,
    description character varying(500),
    remark character varying(255)
);


--
-- Name: COLUMN sys_function.id; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.sys_function.id IS 'id';


--
-- Name: COLUMN sys_function.name; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.sys_function.name IS '功能名称';


--
-- Name: COLUMN sys_function.enable_flag; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.sys_function.enable_flag IS '启用标志 enable-启用 disable-禁用';


--
-- Name: COLUMN sys_function.description; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.sys_function.description IS '描述';


--
-- Name: COLUMN sys_function.remark; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.sys_function.remark IS '备注';


--
-- Name: sys_permissions; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.sys_permissions (
    code character varying(64) NOT NULL,
    name character varying(100) NOT NULL,
    module character varying(50) NOT NULL,
    description text,
    api_patterns jsonb DEFAULT '[]'::jsonb NOT NULL,
    created_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL
);


--
-- Name: sys_role_permissions; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.sys_role_permissions (
    id character varying(36) NOT NULL,
    role_id character varying(36) NOT NULL,
    permission_code character varying(64) NOT NULL,
    tenant_id character varying(36) NOT NULL,
    created_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL
);


--
-- Name: sys_secrets; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.sys_secrets (
    id character varying(36) NOT NULL,
    tenant_id character varying(36) DEFAULT ''::character varying NOT NULL,
    key character varying(64) NOT NULL,
    name character varying(128) NOT NULL,
    secret_type character varying(32) DEFAULT 'GENERIC'::character varying NOT NULL,
    description character varying(255) DEFAULT ''::character varying,
    encrypted_value text NOT NULL,
    mask_preview character varying(32) DEFAULT '****'::character varying NOT NULL,
    key_id character varying(32) DEFAULT ''::character varying NOT NULL,
    created_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP,
    updated_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP
);


--
-- Name: sys_ui_elements; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.sys_ui_elements (
    id character varying(36) NOT NULL,
    parent_id character varying(36) NOT NULL,
    element_code character varying(100) NOT NULL,
    element_type smallint NOT NULL,
    orders smallint,
    param1 character varying(255),
    param2 character varying(255),
    param3 character varying(255),
    authority json NOT NULL,
    description character varying(255),
    created_at timestamp(6) with time zone NOT NULL,
    remark character varying(255),
    multilingual character varying(100),
    route_path character varying(255)
);


--
-- Name: TABLE sys_ui_elements; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON TABLE public.sys_ui_elements IS 'UI元素表';


--
-- Name: COLUMN sys_ui_elements.id; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.sys_ui_elements.id IS '主键ID';


--
-- Name: COLUMN sys_ui_elements.parent_id; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.sys_ui_elements.parent_id IS '父元素id';


--
-- Name: COLUMN sys_ui_elements.element_code; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.sys_ui_elements.element_code IS '元素标识符';


--
-- Name: COLUMN sys_ui_elements.element_type; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.sys_ui_elements.element_type IS '元素类型1-菜单 2-目录 3-按钮 4-路由';


--
-- Name: COLUMN sys_ui_elements.orders; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.sys_ui_elements.orders IS '排序';


--
-- Name: COLUMN sys_ui_elements.authority; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.sys_ui_elements.authority IS '权限(多选)1-系统管理员 2-租户 例如[1,2]';


--
-- Name: COLUMN sys_ui_elements.description; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.sys_ui_elements.description IS '描述';


--
-- Name: COLUMN sys_ui_elements.multilingual; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.sys_ui_elements.multilingual IS '多语言标识符';


--
-- Name: telemetry_current_datas; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.telemetry_current_datas (
    device_id character varying(36) NOT NULL,
    key character varying(255) NOT NULL,
    ts timestamp(6) with time zone NOT NULL,
    bool_v boolean,
    number_v double precision,
    string_v text,
    tenant_id character varying(36)
);


--
-- Name: COLUMN telemetry_current_datas.device_id; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.telemetry_current_datas.device_id IS '设备ID';


--
-- Name: COLUMN telemetry_current_datas.key; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.telemetry_current_datas.key IS '数据标识符';


--
-- Name: COLUMN telemetry_current_datas.ts; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.telemetry_current_datas.ts IS '上报时间';


--
-- Name: telemetry_datas; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.telemetry_datas (
    device_id character varying(36) NOT NULL,
    key character varying(255) NOT NULL,
    ts bigint NOT NULL,
    bool_v boolean,
    number_v double precision,
    string_v text,
    tenant_id character varying(36)
);


--
-- Name: COLUMN telemetry_datas.device_id; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.telemetry_datas.device_id IS '设备ID';


--
-- Name: COLUMN telemetry_datas.key; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.telemetry_datas.key IS '数据标识符';


--
-- Name: COLUMN telemetry_datas.ts; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.telemetry_datas.ts IS '上报时间';


--
-- Name: telemetry_dead_letters; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.telemetry_dead_letters (
    id character varying(36) NOT NULL,
    device_id character varying(36) NOT NULL,
    tenant_id character varying(36),
    key character varying(255) NOT NULL,
    ts bigint NOT NULL,
    bool_v boolean,
    number_v double precision,
    string_v text,
    raw_payload jsonb,
    status character varying(36) DEFAULT 'pending'::character varying NOT NULL,
    attempts integer DEFAULT 1 NOT NULL,
    last_error text,
    next_retry_at timestamp(6) with time zone,
    created_at timestamp(6) with time zone DEFAULT now() NOT NULL,
    updated_at timestamp(6) with time zone DEFAULT now() NOT NULL
);


--
-- Name: TABLE telemetry_dead_letters; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON TABLE public.telemetry_dead_letters IS '遥测写入失败后的持久死信，用于后续重试或人工处理';


--
-- Name: COLUMN telemetry_dead_letters.device_id; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.telemetry_dead_letters.device_id IS '设备ID';


--
-- Name: COLUMN telemetry_dead_letters.key; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.telemetry_dead_letters.key IS '数据标识符';


--
-- Name: COLUMN telemetry_dead_letters.ts; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.telemetry_dead_letters.ts IS '原始上报时间';


--
-- Name: COLUMN telemetry_dead_letters.raw_payload; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.telemetry_dead_letters.raw_payload IS '可重放的原始遥测行';


--
-- Name: COLUMN telemetry_dead_letters.status; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.telemetry_dead_letters.status IS 'pending/retrying/processing/dead/resolved';


--
-- Name: telemetry_rollups; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.telemetry_rollups (
    device_id character varying(36) NOT NULL,
    key character varying(255) NOT NULL,
    bucket_ms bigint NOT NULL,
    bucket_start bigint NOT NULL,
    min_v double precision,
    max_v double precision,
    avg_v double precision,
    last_v double precision,
    count_v bigint DEFAULT 0 NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: telemetry_set_logs; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.telemetry_set_logs (
    id character varying(36) NOT NULL,
    device_id character varying(36) NOT NULL,
    operation_type character varying(255),
    data json,
    status character varying(2),
    error_message character varying(500),
    created_at timestamp(6) with time zone NOT NULL,
    user_id character varying(36),
    description character varying(255)
);


--
-- Name: COLUMN telemetry_set_logs.device_id; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.telemetry_set_logs.device_id IS '设备id（外键-关联删除）';


--
-- Name: COLUMN telemetry_set_logs.operation_type; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.telemetry_set_logs.operation_type IS '操作类型1-手动操作 2-自动触发';


--
-- Name: COLUMN telemetry_set_logs.data; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.telemetry_set_logs.data IS '发送内容';


--
-- Name: COLUMN telemetry_set_logs.status; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.telemetry_set_logs.status IS '1-发送成功 2-失败';


--
-- Name: COLUMN telemetry_set_logs.error_message; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.telemetry_set_logs.error_message IS '错误信息';


--
-- Name: COLUMN telemetry_set_logs.created_at; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.telemetry_set_logs.created_at IS '创建时间';


--
-- Name: COLUMN telemetry_set_logs.user_id; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.telemetry_set_logs.user_id IS '操作用户';


--
-- Name: COLUMN telemetry_set_logs.description; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.telemetry_set_logs.description IS '描述';


--
-- Name: tenant_custom_css; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.tenant_custom_css (
    tenant_id character varying(36) NOT NULL,
    css text DEFAULT ''::text NOT NULL,
    created_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL
);


--
-- Name: TABLE tenant_custom_css; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON TABLE public.tenant_custom_css IS '租户级自定义 CSS（TB-47；tenant_id 主键单行，无行=未配置）';


--
-- Name: COLUMN tenant_custom_css.css; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.tenant_custom_css.css IS '自定义样式文本（≤64KiB；前端必须 textContent 注入 style 标签，后端拒绝 </style 序列做纵深防御）';


--
-- Name: tenant_dashboard_menus; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.tenant_dashboard_menus (
    id character varying(36) NOT NULL,
    tenant_id character varying(36) NOT NULL,
    dashboard_id character varying(99) NOT NULL,
    dashboard_name character varying(99) NOT NULL,
    menu_name character varying(99) NOT NULL,
    parent_code character varying(50) DEFAULT 'home'::character varying NOT NULL,
    sort smallint DEFAULT 1 NOT NULL,
    enabled boolean DEFAULT true NOT NULL,
    created_at timestamp without time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at timestamp without time zone DEFAULT CURRENT_TIMESTAMP NOT NULL
);


--
-- Name: TABLE tenant_dashboard_menus; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON TABLE public.tenant_dashboard_menus IS '租户级 ThingsVis 仪表盘菜单绑定';


--
-- Name: COLUMN tenant_dashboard_menus.tenant_id; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.tenant_dashboard_menus.tenant_id IS '租户ID';


--
-- Name: COLUMN tenant_dashboard_menus.dashboard_id; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.tenant_dashboard_menus.dashboard_id IS 'ThingsVis 仪表盘ID';


--
-- Name: COLUMN tenant_dashboard_menus.dashboard_name; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.tenant_dashboard_menus.dashboard_name IS '仪表盘名称快照';


--
-- Name: COLUMN tenant_dashboard_menus.menu_name; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.tenant_dashboard_menus.menu_name IS '菜单显示名称';


--
-- Name: COLUMN tenant_dashboard_menus.parent_code; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.tenant_dashboard_menus.parent_code IS '父菜单编码，首版固定 home';


--
-- Name: COLUMN tenant_dashboard_menus.sort; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.tenant_dashboard_menus.sort IS '排序';


--
-- Name: COLUMN tenant_dashboard_menus.enabled; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.tenant_dashboard_menus.enabled IS '是否启用';


--
-- Name: tenant_oidc_providers; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.tenant_oidc_providers (
    id character varying(36) NOT NULL,
    tenant_id character varying(36) DEFAULT ''::character varying NOT NULL,
    name character varying(120) NOT NULL,
    issuer character varying(255) NOT NULL,
    client_id character varying(255) NOT NULL,
    client_secret text DEFAULT ''::text NOT NULL,
    discovery_url character varying(255) DEFAULT ''::character varying NOT NULL,
    scopes character varying(255) DEFAULT 'openid profile email'::character varying NOT NULL,
    frontend_redirect character varying(255) DEFAULT ''::character varying NOT NULL,
    enabled boolean DEFAULT true NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: tenant_rate_limits; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.tenant_rate_limits (
    id character varying(36) NOT NULL,
    tenant_id character varying(36) NOT NULL,
    target_type character varying(20) NOT NULL,
    target_id character varying(64) NOT NULL,
    limit_type character varying(20) NOT NULL,
    rate_limits character varying(128) NOT NULL,
    enabled boolean DEFAULT true,
    description character varying(255) DEFAULT ''::character varying,
    created_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP,
    updated_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP
);


--
-- Name: tenant_subscriptions; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.tenant_subscriptions (
    id character varying(36) NOT NULL,
    tenant_id character varying(36) NOT NULL,
    plan_code character varying(50) NOT NULL,
    status character varying(30) DEFAULT 'active'::character varying NOT NULL,
    current_period_start timestamp with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    current_period_end timestamp with time zone DEFAULT (CURRENT_TIMESTAMP + '30 days'::interval) NOT NULL,
    cancel_at_period_end boolean DEFAULT false NOT NULL,
    created_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL
);


--
-- Name: tenant_translations; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.tenant_translations (
    id character varying(36) NOT NULL,
    tenant_id character varying(36) NOT NULL,
    lang character varying(35) NOT NULL,
    key character varying(200) NOT NULL,
    value text NOT NULL,
    created_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL
);


--
-- Name: TABLE tenant_translations; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON TABLE public.tenant_translations IS '租户级 UI 翻译覆盖（TB-47；与静态四语言目录并存，覆盖项优先；UNIQUE(tenant_id,lang,key)）';


--
-- Name: COLUMN tenant_translations.tenant_id; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.tenant_translations.tenant_id IS '租户 ID；空串为系统全局行（SYS_ADMIN 作用域，语义同 logo 全局兜底行）';


--
-- Name: COLUMN tenant_translations.lang; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.tenant_translations.lang IS '语言标签（小写连字符形态，与前端 locale 目录一致：zh-cn/en-us/es-es/fr-fr，服务层白名单校验）';


--
-- Name: COLUMN tenant_translations.key; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.tenant_translations.key IS 'i18n 词条键（如 page.customer.title，服务层限制字母/数字/./-/_ 且 ≤200 字符）';


--
-- Name: COLUMN tenant_translations.value; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.tenant_translations.value IS '覆盖后的译文（≤4000 字符，允许换行）';


--
-- Name: tenants; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.tenants (
    id character varying(36) NOT NULL,
    name character varying(120) DEFAULT ''::character varying NOT NULL,
    parent_tenant_id character varying(36) DEFAULT ''::character varying NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: uplink_storage_dead_letters; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.uplink_storage_dead_letters (
    id character varying(36) NOT NULL,
    data_type character varying(16) NOT NULL,
    device_id character varying(36) NOT NULL,
    tenant_id character varying(36) NOT NULL,
    ts bigint NOT NULL,
    payload jsonb NOT NULL,
    status character varying(16) DEFAULT 'pending'::character varying NOT NULL,
    attempts integer DEFAULT 0 NOT NULL,
    last_error text,
    next_retry_at timestamp with time zone,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    claim_token character varying(36),
    lease_until timestamp with time zone,
    CONSTRAINT uplink_storage_dead_letters_attempts_check CHECK ((attempts >= 0)),
    CONSTRAINT uplink_storage_dead_letters_data_type_check CHECK (((data_type)::text = ANY ((ARRAY['attribute'::character varying, 'event'::character varying])::text[]))),
    CONSTRAINT uplink_storage_dead_letters_processing_lease_check CHECK (((((status)::text = 'processing'::text) AND (claim_token IS NOT NULL) AND (char_length((claim_token)::text) = 36) AND (lease_until IS NOT NULL)) OR (((status)::text <> 'processing'::text) AND (claim_token IS NULL) AND (lease_until IS NULL)))),
    CONSTRAINT uplink_storage_dead_letters_status_check CHECK (((status)::text = ANY ((ARRAY['pending'::character varying, 'retrying'::character varying, 'processing'::character varying, 'resolved'::character varying, 'dead'::character varying])::text[])))
);


--
-- Name: TABLE uplink_storage_dead_letters; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON TABLE public.uplink_storage_dead_letters IS 'Replayable attribute/event envelopes retained after their primary storage write fails.';


--
-- Name: COLUMN uplink_storage_dead_letters.id; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.uplink_storage_dead_letters.id IS 'Stable envelope identity shared by primary write, dead-letter, file spool and replay.';


--
-- Name: COLUMN uplink_storage_dead_letters.payload; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.uplink_storage_dead_letters.payload IS 'Versioned canonical attribute/event envelope with occurrence message_id and full SHA-256 fingerprint; it contains no credentials, raw MQTT message_id or arbitrary protocol metadata.';


--
-- Name: COLUMN uplink_storage_dead_letters.claim_token; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.uplink_storage_dead_letters.claim_token IS 'Per-replay fencing token; completion and retry writes must match the current processing owner.';


--
-- Name: COLUMN uplink_storage_dead_letters.lease_until; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.uplink_storage_dead_letters.lease_until IS 'Expired processing leases are eligible for safe claim recovery by another backend instance.';


--
-- Name: uplink_storage_receipts; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.uplink_storage_receipts (
    id character varying(36) NOT NULL,
    fingerprint character(64) NOT NULL,
    data_type character varying(16) NOT NULL,
    device_id character varying(36) NOT NULL,
    tenant_id character varying(36) NOT NULL,
    ts bigint NOT NULL,
    payload jsonb NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT uplink_storage_receipts_data_type_check CHECK (((data_type)::text = 'attribute'::text))
);


--
-- Name: TABLE uplink_storage_receipts; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON TABLE public.uplink_storage_receipts IS 'Primary-transaction receipts that make complete attribute envelopes idempotent and collision-verifiable.';


--
-- Name: COLUMN uplink_storage_receipts.id; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.uplink_storage_receipts.id IS 'UUID-shaped occurrence message_id shared with primary, fallback and replay records.';


--
-- Name: COLUMN uplink_storage_receipts.fingerprint; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.uplink_storage_receipts.fingerprint IS 'Full SHA-256 fingerprint used to detect a truncated message_id collision.';


--
-- Name: user_address; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.user_address (
    id integer NOT NULL,
    user_id character varying(36) NOT NULL,
    country character varying(50),
    province character varying(50),
    city character varying(50),
    district character varying(50),
    street character varying(100),
    detailed_address character varying(200),
    postal_code character varying(10),
    address_label character varying(50),
    longitude character varying(20),
    latitude character varying(20),
    additional_info character varying(500),
    created_time timestamp with time zone DEFAULT CURRENT_TIMESTAMP,
    updated_time timestamp with time zone DEFAULT CURRENT_TIMESTAMP
);


--
-- Name: TABLE user_address; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON TABLE public.user_address IS '用户地址表（1对1关系）';


--
-- Name: COLUMN user_address.id; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.user_address.id IS '地址ID，主键自增';


--
-- Name: COLUMN user_address.user_id; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.user_address.user_id IS '用户ID，外键关联用户表';


--
-- Name: COLUMN user_address.country; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.user_address.country IS '国家';


--
-- Name: COLUMN user_address.province; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.user_address.province IS '省份';


--
-- Name: COLUMN user_address.city; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.user_address.city IS '城市';


--
-- Name: COLUMN user_address.district; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.user_address.district IS '区县';


--
-- Name: COLUMN user_address.street; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.user_address.street IS '街道/乡镇';


--
-- Name: COLUMN user_address.detailed_address; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.user_address.detailed_address IS '详细地址';


--
-- Name: COLUMN user_address.postal_code; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.user_address.postal_code IS '邮政编码';


--
-- Name: COLUMN user_address.address_label; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.user_address.address_label IS '地址标签';


--
-- Name: COLUMN user_address.longitude; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.user_address.longitude IS '经度';


--
-- Name: COLUMN user_address.latitude; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.user_address.latitude IS '纬度';


--
-- Name: COLUMN user_address.additional_info; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.user_address.additional_info IS '附加信息';


--
-- Name: COLUMN user_address.created_time; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.user_address.created_time IS '创建时间';


--
-- Name: COLUMN user_address.updated_time; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.user_address.updated_time IS '更新时间';


--
-- Name: user_address_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.user_address_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: user_address_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.user_address_id_seq OWNED BY public.user_address.id;


--
-- Name: user_groups; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.user_groups (
    id character varying(36) NOT NULL,
    name character varying(100) NOT NULL,
    tenant_id character varying(36) NOT NULL,
    description text,
    created_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL
);


--
-- Name: TABLE user_groups; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON TABLE public.user_groups IS '用户组主表（TB-46 GPE v1，租户内用户分组与组级共享授权）';


--
-- Name: user_totp; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.user_totp (
    user_id character varying(36) NOT NULL,
    secret_cipher text NOT NULL,
    enabled boolean DEFAULT false NOT NULL,
    last_used_step bigint DEFAULT 0 NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: user_totp_recovery_codes; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.user_totp_recovery_codes (
    id character varying(36) NOT NULL,
    user_id character varying(36) NOT NULL,
    code_hash character varying(64) NOT NULL,
    used_at timestamp with time zone,
    created_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: users; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.users (
    id character varying(36) NOT NULL,
    name character varying(255),
    phone_number character varying(50) NOT NULL,
    email character varying(255) NOT NULL,
    status character varying(2),
    authority character varying(50),
    password character varying(255) NOT NULL,
    tenant_id character varying(36),
    remark character varying(255),
    additional_info json DEFAULT '{}'::json,
    created_at timestamp(6) with time zone,
    updated_at timestamp(6) with time zone,
    password_last_updated timestamp(6) with time zone,
    last_visit_time timestamp with time zone,
    last_visit_ip character varying(30),
    last_visit_device character varying(200),
    organization character varying(200),
    timezone character varying(50),
    default_language character varying(10),
    password_fail_count integer DEFAULT 0,
    avatar_url character varying(500)
);


--
-- Name: TABLE users; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON TABLE public.users IS '用户';


--
-- Name: COLUMN users.status; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.users.status IS '用户状态 F-冻结 N-正常';


--
-- Name: COLUMN users.authority; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.users.authority IS '权限类型 TENANT_ADMIN-租户管理员 TENANT_USER-租户用户 SYS_ADMIN-系统管理员';


--
-- Name: COLUMN users.last_visit_time; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.users.last_visit_time IS '上次访问时间';


--
-- Name: COLUMN users.last_visit_ip; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.users.last_visit_ip IS '上次访问IP';


--
-- Name: COLUMN users.last_visit_device; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.users.last_visit_device IS '上次访问设备信息摘要';


--
-- Name: COLUMN users.organization; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.users.organization IS '用户所属组织机构名称';


--
-- Name: COLUMN users.timezone; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.users.timezone IS '所在时区';


--
-- Name: COLUMN users.default_language; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.users.default_language IS '默认语言';


--
-- Name: COLUMN users.password_fail_count; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.users.password_fail_count IS '密码错误次数';


--
-- Name: COLUMN users.avatar_url; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.users.avatar_url IS '用户头像URL或文件路径';


--
-- Name: vis_dashboard; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.vis_dashboard (
    id character varying(36) NOT NULL,
    relation_id character varying(36),
    json_data json DEFAULT '{}'::json,
    dashboard_name character varying(99),
    create_at timestamp without time zone,
    sort integer,
    remark character varying(255),
    tenant_id character varying(36),
    share_id character varying(36)
);


--
-- Name: TABLE vis_dashboard; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON TABLE public.vis_dashboard IS '可视化插件';


--
-- Name: COLUMN vis_dashboard.sort; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.vis_dashboard.sort IS '排序';


--
-- Name: COLUMN vis_dashboard.share_id; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.vis_dashboard.share_id IS '分享id';


--
-- Name: vis_files; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.vis_files (
    id character varying(36) NOT NULL,
    vis_plugin_id character varying(36) NOT NULL,
    file_name character varying(150),
    file_url character varying(150),
    file_size character varying(20),
    create_at bigint,
    remark character varying(255)
);


--
-- Name: TABLE vis_files; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON TABLE public.vis_files IS '可视化文件表';


--
-- Name: COLUMN vis_files.vis_plugin_id; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.vis_files.vis_plugin_id IS '可视化插件id';


--
-- Name: COLUMN vis_files.file_name; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.vis_files.file_name IS '名称';


--
-- Name: COLUMN vis_files.file_url; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.vis_files.file_url IS 'url地址';


--
-- Name: COLUMN vis_files.file_size; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.vis_files.file_size IS '文件大小';


--
-- Name: vis_plugin; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.vis_plugin (
    id character varying(36) NOT NULL,
    tenant_id character varying(36) NOT NULL,
    plugin_name character varying(150) NOT NULL,
    plugin_description character varying(150),
    create_at bigint,
    remark character varying(255)
);


--
-- Name: TABLE vis_plugin; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON TABLE public.vis_plugin IS '可视化插件表';


--
-- Name: COLUMN vis_plugin.tenant_id; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.vis_plugin.tenant_id IS '租户id';


--
-- Name: COLUMN vis_plugin.plugin_name; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.vis_plugin.plugin_name IS '可视化插件名称';


--
-- Name: COLUMN vis_plugin.plugin_description; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.vis_plugin.plugin_description IS '插件描述';


--
-- Name: widget_bundles; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.widget_bundles (
    id character varying(36) NOT NULL,
    name character varying(255) NOT NULL,
    tenant_id character varying(36) NOT NULL,
    widgets jsonb DEFAULT '[]'::jsonb NOT NULL,
    description character varying(500),
    version character varying(32) DEFAULT '1.0.0'::character varying NOT NULL,
    type_key character varying(64),
    created_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP,
    updated_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP
);


--
-- Name: casbin_rule id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.casbin_rule ALTER COLUMN id SET DEFAULT nextval('public.casbin_rule_id_seq'::regclass);


--
-- Name: device_status_history id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.device_status_history ALTER COLUMN id SET DEFAULT nextval('public.device_status_history_id_seq'::regclass);


--
-- Name: device_topic_mappings id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.device_topic_mappings ALTER COLUMN id SET DEFAULT nextval('public.device_topic_mappings_id_seq'::regclass);


--
-- Name: user_address id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.user_address ALTER COLUMN id SET DEFAULT nextval('public.user_address_id_seq'::regclass);


--
-- Data for Name: action_info; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: ai_models; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: alarm_assignment; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: alarm_comment; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: alarm_config; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: alarm_history; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: alarm_history_devices; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: alarm_info; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: api_usage_daily; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: assets; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: attribute_datas; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: attribute_set_logs; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: board_project_members; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: board_projects; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: boards; Type: TABLE DATA; Schema: public; Owner: -
--

INSERT INTO public.boards (id, name, config, tenant_id, created_at, updated_at, home_flag, description, remark, menu_flag, vis_type, published, published_at, share_token, type_key, author, version, preview_url, download_count) VALUES
	('49c82316-03d0-51eb-2a71-3294d0086599', 'Home', '[]', 'aaaaaa', '2024-08-27 15:15:47.214+08', '2025-04-25 18:49:51.468+08', 'Y', '', NULL, '', NULL, false, NULL, NULL, '', '', '1.0.0', '', 0),
	('ad40389b-bb15-b10f-dc4e-54d980441778', 'Home', '[]', 'd616bcbb', '2025-04-25 18:22:16.2+08', '2025-04-25 18:22:16.2+08', 'Y', NULL, NULL, NULL, NULL, false, NULL, NULL, '', '', '1.0.0', '', 0);


--
-- Data for Name: calcfield_recompute_tasks; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: calculated_fields; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: casbin_rule; Type: TABLE DATA; Schema: public; Owner: -
--

INSERT INTO public.casbin_rule (id, ptype, v0, v1, v2, v3, v4, v5) VALUES
	(1, 'g2', 'api/v1/service/detail/:id', 'api/v1/service/detail/:id', NULL, NULL, NULL, NULL),
	(2, 'g2', 'api/v1/user', 'api/v1/user', NULL, NULL, NULL, NULL),
	(3, 'g2', 'api/v1/dict/column', 'api/v1/dict/column', NULL, NULL, NULL, NULL),
	(4, 'g2', 'api/v1/user/address/:id', 'api/v1/user/address/:id', NULL, NULL, NULL, NULL),
	(5, 'g2', 'api/v1/ui_elements/menu', 'api/v1/ui_elements/menu', NULL, NULL, NULL, NULL),
	(6, 'g2', 'api/v1/device/twin/:id/desired', 'api/v1/device/twin/:id/desired', NULL, NULL, NULL, NULL),
	(7, 'g2', 'api/v1/telemetry/datas/statistic/batch', 'api/v1/telemetry/datas/statistic/batch', NULL, NULL, NULL, NULL),
	(8, 'g2', 'api/v1/command/datas/delivery/diagnostics/:device_id', 'api/v1/command/datas/delivery/diagnostics/:device_id', NULL, NULL, NULL, NULL),
	(9, 'g2', 'api/v1/device/template/market/publish', 'api/v1/device/template/market/publish', NULL, NULL, NULL, NULL),
	(10, 'g2', 'api/v1/device/metrics/menu', 'api/v1/device/metrics/menu', NULL, NULL, NULL, NULL),
	(11, 'g2', 'api/v1/ota/task/:id', 'api/v1/ota/task/:id', NULL, NULL, NULL, NULL),
	(12, 'g2', 'api/v1/telemetry/datas/uplink-dead-letters', 'api/v1/telemetry/datas/uplink-dead-letters', NULL, NULL, NULL, NULL),
	(13, 'g2', 'api/v1/device/model/telemetry/:id', 'api/v1/device/model/telemetry/:id', NULL, NULL, NULL, NULL),
	(14, 'g2', 'api/v1/device/model/custom/commands/:id', 'api/v1/device/model/custom/commands/:id', NULL, NULL, NULL, NULL),
	(15, 'g2', 'api/v1/device/telemetry/latest', 'api/v1/device/telemetry/latest', NULL, NULL, NULL, NULL),
	(16, 'g2', 'api/v1/command/datas/pub', 'api/v1/command/datas/pub', NULL, NULL, NULL, NULL),
	(17, 'g2', 'api/v1/telemetry/datas', 'api/v1/telemetry/datas', NULL, NULL, NULL, NULL),
	(18, 'g2', 'api/v1/device/model/attributes/:id', 'api/v1/device/model/attributes/:id', NULL, NULL, NULL, NULL),
	(19, 'g2', 'api/v1/device/preRegister', 'api/v1/device/preRegister', NULL, NULL, NULL, NULL),
	(20, 'g2', 'api/v1/command/datas/jobs/submit', 'api/v1/command/datas/jobs/submit', NULL, NULL, NULL, NULL),
	(21, 'g2', 'api/v1/ota/task/preview', 'api/v1/ota/task/preview', NULL, NULL, NULL, NULL),
	(22, 'g2', 'api/v1/board/user/update', 'api/v1/board/user/update', NULL, NULL, NULL, NULL),
	(23, 'g2', 'api/v1/rdi/shared-with-me/devices', 'api/v1/rdi/shared-with-me/devices', NULL, NULL, NULL, NULL),
	(24, 'g2', 'api/v1/role', 'api/v1/role', NULL, NULL, NULL, NULL),
	(25, 'g2', 'api/v1/alarm/info/batch', 'api/v1/alarm/info/batch', NULL, NULL, NULL, NULL),
	(26, 'g2', 'api/v1/notification_history/list', 'api/v1/notification_history/list', NULL, NULL, NULL, NULL),
	(27, 'g2', 'api/v1/command/datas/jobs/:job_id/cancel', 'api/v1/command/datas/jobs/:job_id/cancel', NULL, NULL, NULL, NULL),
	(28, 'g2', 'api/v1/service/plugin/info', 'api/v1/service/plugin/info', NULL, NULL, NULL, NULL),
	(29, 'g2', 'api/v1/data_script', 'api/v1/data_script', NULL, NULL, NULL, NULL),
	(30, 'g2', 'api/v1/user/transform', 'api/v1/user/transform', NULL, NULL, NULL, NULL),
	(31, 'g2', 'api/v1/ota/task/:id/governance-preview', 'api/v1/ota/task/:id/governance-preview', NULL, NULL, NULL, NULL),
	(32, 'g2', 'api/v1/payload-schema/:schema_id', 'api/v1/payload-schema/:schema_id', NULL, NULL, NULL, NULL),
	(33, 'g2', 'api/v1/user/logout', 'api/v1/user/logout', NULL, NULL, NULL, NULL),
	(34, 'g2', 'api/v1/service/:id', 'api/v1/service/:id', NULL, NULL, NULL, NULL),
	(35, 'g2', 'api/v1/message_push/logout', 'api/v1/message_push/logout', NULL, NULL, NULL, NULL),
	(36, 'g2', 'api/v1/rdi/share-tokens/:token/accept', 'api/v1/rdi/share-tokens/:token/accept', NULL, NULL, NULL, NULL),
	(37, 'g2', 'api/v1/command/datas/jobs/:job_id/rows', 'api/v1/command/datas/jobs/:job_id/rows', NULL, NULL, NULL, NULL),
	(38, 'g2', 'api/v1/oidc/provider/list', 'api/v1/oidc/provider/list', NULL, NULL, NULL, NULL),
	(39, 'g2', 'api/v1/device/online/status/:id', 'api/v1/device/online/status/:id', NULL, NULL, NULL, NULL),
	(40, 'g2', 'api/v1/casbin/user', 'api/v1/casbin/user', NULL, NULL, NULL, NULL),
	(41, 'g2', 'api/v1/device/service/access/batch', 'api/v1/device/service/access/batch', NULL, NULL, NULL, NULL),
	(42, 'g2', 'api/v1/device/list', 'api/v1/device/list', NULL, NULL, NULL, NULL),
	(43, 'g2', 'api/v1/board/user/update/password', 'api/v1/board/user/update/password', NULL, NULL, NULL, NULL),
	(44, 'g2', 'api/v1/service', 'api/v1/service', NULL, NULL, NULL, NULL),
	(45, 'g2', 'api/v1/scene_automations/list', 'api/v1/scene_automations/list', NULL, NULL, NULL, NULL),
	(46, 'g2', 'api/v1/board/tenant/user/info', 'api/v1/board/tenant/user/info', NULL, NULL, NULL, NULL),
	(47, 'g2', 'api/v1/rule-chains', 'api/v1/rule-chains', NULL, NULL, NULL, NULL),
	(48, 'g2', 'api/v1/ota/package/:id', 'api/v1/ota/package/:id', NULL, NULL, NULL, NULL),
	(49, 'g2', 'api/v1/asset/:id', 'api/v1/asset/:id', NULL, NULL, NULL, NULL),
	(50, 'g2', 'api/v1/asset/tree', 'api/v1/asset/tree', NULL, NULL, NULL, NULL),
	(51, 'g2', 'api/v1/device/model/commands/:id', 'api/v1/device/model/commands/:id', NULL, NULL, NULL, NULL),
	(52, 'g2', 'api/v1/alarm/config', 'api/v1/alarm/config', NULL, NULL, NULL, NULL),
	(53, 'g2', 'api/v1/alarm/info/history/:id', 'api/v1/alarm/info/history/:id', NULL, NULL, NULL, NULL),
	(54, 'g2', 'api/v1/scene', 'api/v1/scene', NULL, NULL, NULL, NULL),
	(55, 'g2', 'api/v1/device/:id/mqtt-debug/session', 'api/v1/device/:id/mqtt-debug/session', NULL, NULL, NULL, NULL),
	(56, 'g2', 'api/v1/oidc/provider/:id', 'api/v1/oidc/provider/:id', NULL, NULL, NULL, NULL),
	(57, 'g2', 'api/v1/device/shadow/:deviceId', 'api/v1/device/shadow/:deviceId', NULL, NULL, NULL, NULL),
	(58, 'g2', 'api/v1/user/totp/disable', 'api/v1/user/totp/disable', NULL, NULL, NULL, NULL),
	(59, 'g2', 'api/v1/telemetry/datas/current/detail/:id', 'api/v1/telemetry/datas/current/detail/:id', NULL, NULL, NULL, NULL),
	(60, 'g2', 'api/v1/scene/:id', 'api/v1/scene/:id', NULL, NULL, NULL, NULL),
	(61, 'g2', 'api/v1/dashboard-menu/:dashboardId', 'api/v1/dashboard-menu/:dashboardId', NULL, NULL, NULL, NULL),
	(62, 'g2', 'api/v1/command/datas/jobs/:job_id/support-bundle', 'api/v1/command/datas/jobs/:job_id/support-bundle', NULL, NULL, NULL, NULL),
	(63, 'g2', 'api/v1/rdi/devices/:device_id/latest-firmware', 'api/v1/rdi/devices/:device_id/latest-firmware', NULL, NULL, NULL, NULL),
	(64, 'g2', 'api/v1/device/template/market/detail/:market_id', 'api/v1/device/template/market/detail/:market_id', NULL, NULL, NULL, NULL),
	(65, 'g2', 'api/v1/device_config/metrics/menu', 'api/v1/device_config/metrics/menu', NULL, NULL, NULL, NULL),
	(66, 'g2', 'api/v1/device/group', 'api/v1/device/group', NULL, NULL, NULL, NULL),
	(67, 'g2', 'api/v1/protocol_plugin/config_form', 'api/v1/protocol_plugin/config_form', NULL, NULL, NULL, NULL),
	(68, 'g2', 'api/v1/ai/telemetry/query', 'api/v1/ai/telemetry/query', NULL, NULL, NULL, NULL),
	(69, 'g2', 'api/v1/device/template/detail/:id', 'api/v1/device/template/detail/:id', NULL, NULL, NULL, NULL),
	(70, 'g2', 'api/v1/payload-schema', 'api/v1/payload-schema', NULL, NULL, NULL, NULL),
	(71, 'g2', 'api/v1/telemetry/datas/dead-letters/:id/status', 'api/v1/telemetry/datas/dead-letters/:id/status', NULL, NULL, NULL, NULL),
	(72, 'g2', 'api/v1/device/son/add', 'api/v1/device/son/add', NULL, NULL, NULL, NULL),
	(73, 'g2', 'api/v1/device/update/voucher', 'api/v1/device/update/voucher', NULL, NULL, NULL, NULL),
	(74, 'g2', 'api/v1/attribute/datas/set/logs', 'api/v1/attribute/datas/set/logs', NULL, NULL, NULL, NULL),
	(75, 'g2', 'api/v1/board', 'api/v1/board', NULL, NULL, NULL, NULL),
	(76, 'g2', 'api/v1/user/totp/setup', 'api/v1/user/totp/setup', NULL, NULL, NULL, NULL),
	(77, 'g2', 'api/v1/device/model/commands', 'api/v1/device/model/commands', NULL, NULL, NULL, NULL),
	(78, 'g2', 'api/v1/command/datas/set/logs', 'api/v1/command/datas/set/logs', NULL, NULL, NULL, NULL),
	(79, 'g2', 'api/v1/scene_automations/switch/:id', 'api/v1/scene_automations/switch/:id', NULL, NULL, NULL, NULL),
	(80, 'g2', 'api/v1/events', 'api/v1/events', NULL, NULL, NULL, NULL),
	(81, 'g2', 'api/v1/telemetry/datas/dead-letters', 'api/v1/telemetry/datas/dead-letters', NULL, NULL, NULL, NULL),
	(82, 'g2', 'api/v1/device/:id/mqtt-debug/session/:session_id/command', 'api/v1/device/:id/mqtt-debug/session/:session_id/command', NULL, NULL, NULL, NULL),
	(83, 'g2', 'api/v1/entity_versions/:id/restore', 'api/v1/entity_versions/:id/restore', NULL, NULL, NULL, NULL),
	(84, 'g2', 'api/v1/alarm/info/history/:id/acknowledge', 'api/v1/alarm/info/history/:id/acknowledge', NULL, NULL, NULL, NULL),
	(85, 'g2', 'api/v1/device/template/selector', 'api/v1/device/template/selector', NULL, NULL, NULL, NULL),
	(86, 'g2', 'api/v1/dict/protocol/service', 'api/v1/dict/protocol/service', NULL, NULL, NULL, NULL),
	(87, 'g2', 'api/v1/telemetry/datas/simulation/init', 'api/v1/telemetry/datas/simulation/init', NULL, NULL, NULL, NULL),
	(88, 'g2', 'api/v1/board/user/info', 'api/v1/board/user/info', NULL, NULL, NULL, NULL),
	(89, 'g2', 'api/v1/rdi/devices/:device_id/share-recipients/:user_id', 'api/v1/rdi/devices/:device_id/share-recipients/:user_id', NULL, NULL, NULL, NULL),
	(90, 'g2', 'api/v1/rdi/devices/:device_id/commands', 'api/v1/rdi/devices/:device_id/commands', NULL, NULL, NULL, NULL),
	(91, 'g2', 'api/v1/entity_versions/:id', 'api/v1/entity_versions/:id', NULL, NULL, NULL, NULL),
	(92, 'g2', 'api/v1/scene_automations/log', 'api/v1/scene_automations/log', NULL, NULL, NULL, NULL),
	(93, 'g2', 'api/v1/rule-chains/list', 'api/v1/rule-chains/list', NULL, NULL, NULL, NULL),
	(94, 'g2', 'api/v1/expected/data/:id', 'api/v1/expected/data/:id', NULL, NULL, NULL, NULL),
	(95, 'g2', 'api/v1/device_config/batch', 'api/v1/device_config/batch', NULL, NULL, NULL, NULL),
	(96, 'g2', 'api/v1/device/model/events/:id', 'api/v1/device/model/events/:id', NULL, NULL, NULL, NULL),
	(97, 'g2', 'api/v1/ota/task/:id/support-bundle', 'api/v1/ota/task/:id/support-bundle', NULL, NULL, NULL, NULL),
	(98, 'g2', 'api/v1/device/template/market/install', 'api/v1/device/template/market/install', NULL, NULL, NULL, NULL),
	(99, 'g2', 'api/v1/device/model/source/at/list', 'api/v1/device/model/source/at/list', NULL, NULL, NULL, NULL),
	(100, 'g2', 'api/v1/device/group/tree', 'api/v1/device/group/tree', NULL, NULL, NULL, NULL),
	(101, 'g2', 'api/v1/device/status/history', 'api/v1/device/status/history', NULL, NULL, NULL, NULL),
	(102, 'g2', 'api/v1/notification_group', 'api/v1/notification_group', NULL, NULL, NULL, NULL),
	(103, 'g2', 'api/v1/ui_elements/:id', 'api/v1/ui_elements/:id', NULL, NULL, NULL, NULL),
	(104, 'g2', 'api/v1/device/:id/debug/logs', 'api/v1/device/:id/debug/logs', NULL, NULL, NULL, NULL),
	(105, 'g2', 'api/v1/board/device/total', 'api/v1/board/device/total', NULL, NULL, NULL, NULL),
	(106, 'g2', 'api/v1/device/connect/form', 'api/v1/device/connect/form', NULL, NULL, NULL, NULL),
	(107, 'g2', 'api/v1/device/sub-remove', 'api/v1/device/sub-remove', NULL, NULL, NULL, NULL),
	(108, 'g2', 'api/v1/device/topic-mappings/:id', 'api/v1/device/topic-mappings/:id', NULL, NULL, NULL, NULL),
	(109, 'g2', 'api/v1/telemetry/datas/uplink-dead-letters/drain', 'api/v1/telemetry/datas/uplink-dead-letters/drain', NULL, NULL, NULL, NULL),
	(110, 'g2', 'api/v1/datapolicy', 'api/v1/datapolicy', NULL, NULL, NULL, NULL),
	(111, 'g2', 'api/v1/command/datas/direct-method', 'api/v1/command/datas/direct-method', NULL, NULL, NULL, NULL),
	(112, 'g2', 'api/v1/alarm/config/:id', 'api/v1/alarm/config/:id', NULL, NULL, NULL, NULL),
	(113, 'g2', 'api/v1/ota/package', 'api/v1/ota/package', NULL, NULL, NULL, NULL),
	(114, 'g2', 'api/v1/device/template/market/list', 'api/v1/device/template/market/list', NULL, NULL, NULL, NULL),
	(115, 'g2', 'api/v1/user/tenant/id', 'api/v1/user/tenant/id', NULL, NULL, NULL, NULL),
	(116, 'g2', 'api/v1/telemetry/datas/statistic', 'api/v1/telemetry/datas/statistic', NULL, NULL, NULL, NULL),
	(117, 'g2', 'api/v1/ai/alarm/analysis', 'api/v1/ai/alarm/analysis', NULL, NULL, NULL, NULL),
	(118, 'g2', 'api/v1/user/:id', 'api/v1/user/:id', NULL, NULL, NULL, NULL),
	(119, 'g2', 'api/v1/device/:id/onboarding/connection-guide', 'api/v1/device/:id/onboarding/connection-guide', NULL, NULL, NULL, NULL),
	(120, 'g2', 'api/v1/device/group/detail/:id', 'api/v1/device/group/detail/:id', NULL, NULL, NULL, NULL),
	(121, 'g2', 'api/v1/notification/e-mail/templates/:id/default', 'api/v1/notification/e-mail/templates/:id/default', NULL, NULL, NULL, NULL),
	(122, 'g2', 'api/v1/telemetry/datas/simulation/send', 'api/v1/telemetry/datas/simulation/send', NULL, NULL, NULL, NULL),
	(123, 'g2', 'api/v1/telemetry/datas/current/keys', 'api/v1/telemetry/datas/current/keys', NULL, NULL, NULL, NULL),
	(124, 'g2', 'api/v1/alarm/info/history/device', 'api/v1/alarm/info/history/device', NULL, NULL, NULL, NULL),
	(125, 'g2', 'api/v1/open/keys', 'api/v1/open/keys', NULL, NULL, NULL, NULL),
	(126, 'g2', 'api/v1/dashboard-menu/batch', 'api/v1/dashboard-menu/batch', NULL, NULL, NULL, NULL),
	(127, 'g2', 'api/v1/board/tenant/device/info', 'api/v1/board/tenant/device/info', NULL, NULL, NULL, NULL),
	(128, 'g2', 'api/v1/command/datas/jobs/:job_id', 'api/v1/command/datas/jobs/:job_id', NULL, NULL, NULL, NULL),
	(129, 'g2', 'api/v1/dict/language/:id', 'api/v1/dict/language/:id', NULL, NULL, NULL, NULL),
	(130, 'g2', 'api/v1/telemetry/datas/msg/count', 'api/v1/telemetry/datas/msg/count', NULL, NULL, NULL, NULL),
	(131, 'g2', 'api/v1/device/model/custom/commands', 'api/v1/device/model/custom/commands', NULL, NULL, NULL, NULL),
	(132, 'g2', 'api/v1/alarm/device/counts', 'api/v1/alarm/device/counts', NULL, NULL, NULL, NULL),
	(133, 'g2', 'api/v1/user/warning-email', 'api/v1/user/warning-email', NULL, NULL, NULL, NULL),
	(134, 'g2', 'api/v1/entity_versions', 'api/v1/entity_versions', NULL, NULL, NULL, NULL),
	(135, 'g2', 'api/v1/asset/list', 'api/v1/asset/list', NULL, NULL, NULL, NULL),
	(136, 'g2', 'api/v1/user/prefer-lang', 'api/v1/user/prefer-lang', NULL, NULL, NULL, NULL),
	(137, 'g2', 'api/v1/ui_elements', 'api/v1/ui_elements', NULL, NULL, NULL, NULL),
	(138, 'g2', 'api/v1/device/metrics/:id', 'api/v1/device/metrics/:id', NULL, NULL, NULL, NULL),
	(139, 'g2', 'api/v1/scene/detail/:id', 'api/v1/scene/detail/:id', NULL, NULL, NULL, NULL),
	(140, 'g2', 'api/v1/device/template/market/login', 'api/v1/device/template/market/login', NULL, NULL, NULL, NULL),
	(141, 'g2', 'api/v1/device/group/relation', 'api/v1/device/group/relation', NULL, NULL, NULL, NULL),
	(142, 'g2', 'api/v1/board/trend', 'api/v1/board/trend', NULL, NULL, NULL, NULL),
	(143, 'g2', 'api/v1/notification_group/list', 'api/v1/notification_group/list', NULL, NULL, NULL, NULL),
	(144, 'g2', 'api/v1/command/datas/saved-filters', 'api/v1/command/datas/saved-filters', NULL, NULL, NULL, NULL),
	(145, 'g2', 'api/v1/device/selector', 'api/v1/device/selector', NULL, NULL, NULL, NULL),
	(146, 'g2', 'api/v1/rdi/devices/:device_id/share-tokens/:token', 'api/v1/rdi/devices/:device_id/share-tokens/:token', NULL, NULL, NULL, NULL),
	(147, 'g2', 'api/v1/telemetry/datas/history', 'api/v1/telemetry/datas/history', NULL, NULL, NULL, NULL),
	(148, 'g2', 'api/v1/scene_automations/:id', 'api/v1/scene_automations/:id', NULL, NULL, NULL, NULL),
	(149, 'g2', 'api/v1/message_push/config', 'api/v1/message_push/config', NULL, NULL, NULL, NULL),
	(150, 'g2', 'api/v1/message_push', 'api/v1/message_push', NULL, NULL, NULL, NULL),
	(151, 'g2', 'api/v1/board/tenant', 'api/v1/board/tenant', NULL, NULL, NULL, NULL),
	(152, 'g2', 'api/v1/scene_automations/alarm', 'api/v1/scene_automations/alarm', NULL, NULL, NULL, NULL),
	(153, 'g2', 'api/v1/rdi/thing-model', 'api/v1/rdi/thing-model', NULL, NULL, NULL, NULL),
	(154, 'g2', 'api/v1/device/model/attributes', 'api/v1/device/model/attributes', NULL, NULL, NULL, NULL),
	(155, 'g2', 'api/v1/sys_function/:id', 'api/v1/sys_function/:id', NULL, NULL, NULL, NULL),
	(156, 'g2', 'api/v1/rdi/devices/:device_id/share-token', 'api/v1/rdi/devices/:device_id/share-token', NULL, NULL, NULL, NULL),
	(157, 'g2', 'api/v1/board/device', 'api/v1/board/device', NULL, NULL, NULL, NULL),
	(158, 'g2', 'api/v1/alarm/info/history/:id/reset', 'api/v1/alarm/info/history/:id/reset', NULL, NULL, NULL, NULL),
	(159, 'g2', 'api/v1/role/:id', 'api/v1/role/:id', NULL, NULL, NULL, NULL),
	(160, 'g2', 'api/v1/scene/log', 'api/v1/scene/log', NULL, NULL, NULL, NULL),
	(161, 'g2', 'api/v1/dict/language', 'api/v1/dict/language', NULL, NULL, NULL, NULL),
	(162, 'g2', 'api/v1/device/update/config', 'api/v1/device/update/config', NULL, NULL, NULL, NULL),
	(163, 'g2', 'api/v1/device/modbus/profile/number/:deviceNumber', 'api/v1/device/modbus/profile/number/:deviceNumber', NULL, NULL, NULL, NULL),
	(164, 'g2', 'api/v1/notification/e-mail/templates/:id', 'api/v1/notification/e-mail/templates/:id', NULL, NULL, NULL, NULL),
	(165, 'g2', 'api/v1/service/access', 'api/v1/service/access', NULL, NULL, NULL, NULL),
	(166, 'g2', 'api/v1/device/check/:deviceNumber', 'api/v1/device/check/:deviceNumber', NULL, NULL, NULL, NULL),
	(167, 'g2', 'api/v1/system/metrics/current', 'api/v1/system/metrics/current', NULL, NULL, NULL, NULL),
	(168, 'g2', 'api/v1/data_script/enable', 'api/v1/data_script/enable', NULL, NULL, NULL, NULL),
	(169, 'g2', 'api/v1/service/access/list', 'api/v1/service/access/list', NULL, NULL, NULL, NULL),
	(170, 'g2', 'api/v1/asset', 'api/v1/asset', NULL, NULL, NULL, NULL),
	(171, 'g2', 'api/v1/device/template', 'api/v1/device/template', NULL, NULL, NULL, NULL),
	(172, 'g2', 'api/v1/event/datas', 'api/v1/event/datas', NULL, NULL, NULL, NULL),
	(173, 'g2', 'api/v1/casbin/function/:id', 'api/v1/casbin/function/:id', NULL, NULL, NULL, NULL),
	(174, 'g2', 'api/v1/file/up', 'api/v1/file/up', NULL, NULL, NULL, NULL),
	(175, 'g2', 'api/v1/device/topic-mappings/dry-run', 'api/v1/device/topic-mappings/dry-run', NULL, NULL, NULL, NULL),
	(176, 'g2', 'api/v1/service/access/device/list', 'api/v1/service/access/device/list', NULL, NULL, NULL, NULL),
	(177, 'g2', 'api/v1/expected/data/list', 'api/v1/expected/data/list', NULL, NULL, NULL, NULL),
	(178, 'g2', 'api/v1/ui_elements/select/form', 'api/v1/ui_elements/select/form', NULL, NULL, NULL, NULL),
	(179, 'g2', 'api/v1/device/:id/connection/diagnostics', 'api/v1/device/:id/connection/diagnostics', NULL, NULL, NULL, NULL),
	(180, 'g2', 'api/v1/device/topic-mappings', 'api/v1/device/topic-mappings', NULL, NULL, NULL, NULL),
	(181, 'g2', 'api/v1/service/access/voucher/form', 'api/v1/service/access/voucher/form', NULL, NULL, NULL, NULL),
	(182, 'g2', 'api/v1/device/model/custom/control/:id', 'api/v1/device/model/custom/control/:id', NULL, NULL, NULL, NULL),
	(183, 'g2', 'api/v1/device_config/metrics/condition/menu', 'api/v1/device_config/metrics/condition/menu', NULL, NULL, NULL, NULL),
	(184, 'g2', 'api/v1/device/active', 'api/v1/device/active', NULL, NULL, NULL, NULL),
	(185, 'g2', 'api/v1/device/metrics/condition/menu', 'api/v1/device/metrics/condition/menu', NULL, NULL, NULL, NULL),
	(186, 'g2', 'api/v1/device/map/telemetry/:id', 'api/v1/device/map/telemetry/:id', NULL, NULL, NULL, NULL),
	(187, 'g2', 'api/v1/board/:id/publish', 'api/v1/board/:id/publish', NULL, NULL, NULL, NULL),
	(188, 'g2', 'api/v1/casbin/user/:id', 'api/v1/casbin/user/:id', NULL, NULL, NULL, NULL),
	(189, 'g2', 'api/v1/service/plugin/select', 'api/v1/service/plugin/select', NULL, NULL, NULL, NULL),
	(190, 'g2', 'api/v1/system/metrics/history', 'api/v1/system/metrics/history', NULL, NULL, NULL, NULL),
	(191, 'g2', 'api/v1/notification_group/:id', 'api/v1/notification_group/:id', NULL, NULL, NULL, NULL),
	(192, 'g2', 'api/v1/device/twin-drift', 'api/v1/device/twin-drift', NULL, NULL, NULL, NULL),
	(193, 'g2', 'api/v1/attribute/datas/key', 'api/v1/attribute/datas/key', NULL, NULL, NULL, NULL),
	(194, 'g2', 'api/v1/telemetry/datas/simulation', 'api/v1/telemetry/datas/simulation', NULL, NULL, NULL, NULL),
	(195, 'g2', 'api/v1/telemetry/datas/current/:id', 'api/v1/telemetry/datas/current/:id', NULL, NULL, NULL, NULL),
	(196, 'g2', 'api/v1/logo', 'api/v1/logo', NULL, NULL, NULL, NULL),
	(197, 'g2', 'api/v1/device/model/telemetry', 'api/v1/device/model/telemetry', NULL, NULL, NULL, NULL),
	(198, 'g2', 'api/v1/telemetry/datas/set/logs', 'api/v1/telemetry/datas/set/logs', NULL, NULL, NULL, NULL),
	(199, 'g2', 'api/v1/board/home', 'api/v1/board/home', NULL, NULL, NULL, NULL),
	(200, 'g2', 'api/v1/user/totp/status', 'api/v1/user/totp/status', NULL, NULL, NULL, NULL);
INSERT INTO public.casbin_rule (id, ptype, v0, v1, v2, v3, v4, v5) VALUES
	(201, 'g2', 'api/v1/device/:id', 'api/v1/device/:id', NULL, NULL, NULL, NULL),
	(202, 'g2', 'api/v1/scene/active/:id', 'api/v1/scene/active/:id', NULL, NULL, NULL, NULL),
	(203, 'g2', 'api/v1/device_config', 'api/v1/device_config', NULL, NULL, NULL, NULL),
	(204, 'g2', 'api/v1/device/template/chart', 'api/v1/device/template/chart', NULL, NULL, NULL, NULL),
	(205, 'g2', 'api/v1/board/:id', 'api/v1/board/:id', NULL, NULL, NULL, NULL),
	(206, 'g2', 'api/v1/device/template/menu', 'api/v1/device/template/menu', NULL, NULL, NULL, NULL),
	(207, 'g2', 'api/v1/user/refresh', 'api/v1/user/refresh', NULL, NULL, NULL, NULL),
	(208, 'g2', 'api/v1/device/group/relation/list', 'api/v1/device/group/relation/list', NULL, NULL, NULL, NULL),
	(209, 'g2', 'api/v1/attribute/datas/pub', 'api/v1/attribute/datas/pub', NULL, NULL, NULL, NULL),
	(210, 'g2', 'api/v1/device/model/events', 'api/v1/device/model/events', NULL, NULL, NULL, NULL),
	(211, 'g2', 'api/v1/service/list', 'api/v1/service/list', NULL, NULL, NULL, NULL),
	(212, 'g2', 'api/v1/device', 'api/v1/device', NULL, NULL, NULL, NULL),
	(213, 'g2', 'api/v1/rdi/devices/:device_id/config', 'api/v1/rdi/devices/:device_id/config', NULL, NULL, NULL, NULL),
	(214, 'g2', 'api/v1/telemetry/datas/history/pagination', 'api/v1/telemetry/datas/history/pagination', NULL, NULL, NULL, NULL),
	(215, 'g2', 'api/v1/data_script/quiz', 'api/v1/data_script/quiz', NULL, NULL, NULL, NULL),
	(216, 'g2', 'api/v1/device/template/chart/select', 'api/v1/device/template/chart/select', NULL, NULL, NULL, NULL),
	(217, 'g2', 'api/v1/device/tenant/list', 'api/v1/device/tenant/list', NULL, NULL, NULL, NULL),
	(218, 'g2', 'api/v1/payload-schema/validate', 'api/v1/payload-schema/validate', NULL, NULL, NULL, NULL),
	(219, 'g2', 'api/v1/dict/column/:id', 'api/v1/dict/column/:id', NULL, NULL, NULL, NULL),
	(220, 'g2', 'api/v1/device/connect/info', 'api/v1/device/connect/info', NULL, NULL, NULL, NULL),
	(221, 'g2', 'api/v1/dict/enum', 'api/v1/dict/enum', NULL, NULL, NULL, NULL),
	(222, 'g2', 'api/v1/device/model/custom/control', 'api/v1/device/model/custom/control', NULL, NULL, NULL, NULL),
	(223, 'g2', 'api/v1/user/update', 'api/v1/user/update', NULL, NULL, NULL, NULL),
	(224, 'g2', 'api/v1/device_config/:id', 'api/v1/device_config/:id', NULL, NULL, NULL, NULL),
	(225, 'g2', 'api/v1/open/keys/:id', 'api/v1/open/keys/:id', NULL, NULL, NULL, NULL),
	(226, 'g2', 'api/v1/alarm/info/history/batch-action', 'api/v1/alarm/info/history/batch-action', NULL, NULL, NULL, NULL),
	(227, 'g2', 'api/v1/rdi/devices/:device_id/history', 'api/v1/rdi/devices/:device_id/history', NULL, NULL, NULL, NULL),
	(228, 'g2', 'api/v1/notification/e-mail/templates', 'api/v1/notification/e-mail/templates', NULL, NULL, NULL, NULL),
	(229, 'g2', 'api/v1/device/template/stats', 'api/v1/device/template/stats', NULL, NULL, NULL, NULL),
	(230, 'g2', 'api/v1/device/detail/:id', 'api/v1/device/detail/:id', NULL, NULL, NULL, NULL),
	(231, 'g2', 'api/v1/rule-chains/:id', 'api/v1/rule-chains/:id', NULL, NULL, NULL, NULL),
	(232, 'g2', 'api/v1/device_config/menu', 'api/v1/device_config/menu', NULL, NULL, NULL, NULL),
	(233, 'g2', 'api/v1/alarm/info/config/device', 'api/v1/alarm/info/config/device', NULL, NULL, NULL, NULL),
	(234, 'g2', 'api/v1/device_config/voucher_type', 'api/v1/device_config/voucher_type', NULL, NULL, NULL, NULL),
	(235, 'g2', 'api/v1/user/change-email', 'api/v1/user/change-email', NULL, NULL, NULL, NULL),
	(236, 'g2', 'api/v1/notification/e-mail/templates/preview', 'api/v1/notification/e-mail/templates/preview', NULL, NULL, NULL, NULL),
	(237, 'g2', 'api/v1/ota/task', 'api/v1/ota/task', NULL, NULL, NULL, NULL),
	(238, 'g2', 'api/v1/telemetry/datas/dead-letters/drain', 'api/v1/telemetry/datas/dead-letters/drain', NULL, NULL, NULL, NULL),
	(239, 'g2', 'api/v1/notification/services/config/e-mail/test', 'api/v1/notification/services/config/e-mail/test', NULL, NULL, NULL, NULL),
	(240, 'g2', 'api/v1/device/:id/mqtt-debug/session/:session_id', 'api/v1/device/:id/mqtt-debug/session/:session_id', NULL, NULL, NULL, NULL),
	(241, 'g2', 'api/v1/scene_automations/dry-run', 'api/v1/scene_automations/dry-run', NULL, NULL, NULL, NULL),
	(242, 'g2', 'api/v1/telemetry/datas/history/page', 'api/v1/telemetry/datas/history/page', NULL, NULL, NULL, NULL),
	(243, 'g2', 'api/v1/alarm/info/history/monthly', 'api/v1/alarm/info/history/monthly', NULL, NULL, NULL, NULL),
	(244, 'g2', 'api/v1/device/model/custom/commands/:deviceId', 'api/v1/device/model/custom/commands/:deviceId', NULL, NULL, NULL, NULL),
	(245, 'g2', 'api/v1/oidc/provider', 'api/v1/oidc/provider', NULL, NULL, NULL, NULL),
	(246, 'g2', 'api/v1/device/metrics/chart', 'api/v1/device/metrics/chart', NULL, NULL, NULL, NULL),
	(247, 'g2', 'api/v1/calculated_fields/:id/toggle', 'api/v1/calculated_fields/:id/toggle', NULL, NULL, NULL, NULL),
	(248, 'g2', 'api/v1/device/:id/debug', 'api/v1/device/:id/debug', NULL, NULL, NULL, NULL),
	(249, 'g2', 'api/v1/device/preRegister/export', 'api/v1/device/preRegister/export', NULL, NULL, NULL, NULL),
	(250, 'g2', 'api/v1/notification/services/config/:type', 'api/v1/notification/services/config/:type', NULL, NULL, NULL, NULL),
	(251, 'g2', 'api/v1/device/group/:id', 'api/v1/device/group/:id', NULL, NULL, NULL, NULL),
	(252, 'g2', 'api/v1/attribute/datas/:id', 'api/v1/attribute/datas/:id', NULL, NULL, NULL, NULL),
	(253, 'g2', 'api/v1/service/access/:id', 'api/v1/service/access/:id', NULL, NULL, NULL, NULL),
	(254, 'g2', 'api/v1/scene_automations', 'api/v1/scene_automations', NULL, NULL, NULL, NULL),
	(255, 'g2', 'api/v1/command/datas/jobs/:job_id/retry', 'api/v1/command/datas/jobs/:job_id/retry', NULL, NULL, NULL, NULL),
	(256, 'g2', 'api/v1/scene/dry-run', 'api/v1/scene/dry-run', NULL, NULL, NULL, NULL),
	(257, 'g2', 'api/v1/user/detail', 'api/v1/user/detail', NULL, NULL, NULL, NULL),
	(258, 'g2', 'api/v1/telemetry/datas/uplink-dead-letters/:id/status', 'api/v1/telemetry/datas/uplink-dead-letters/:id/status', NULL, NULL, NULL, NULL),
	(259, 'g2', 'api/v1/casbin/function', 'api/v1/casbin/function', NULL, NULL, NULL, NULL),
	(260, 'g2', 'api/v1/expected/data', 'api/v1/expected/data', NULL, NULL, NULL, NULL),
	(261, 'g2', 'api/v1/device_config/connect', 'api/v1/device_config/connect', NULL, NULL, NULL, NULL),
	(262, 'g2', 'api/v1/dict', 'api/v1/dict', NULL, NULL, NULL, NULL),
	(263, 'g2', 'api/v1/calculated_fields', 'api/v1/calculated_fields', NULL, NULL, NULL, NULL),
	(264, 'g2', 'api/v1/data_script/:id', 'api/v1/data_script/:id', NULL, NULL, NULL, NULL),
	(265, 'g2', 'api/v1/scene_automations/detail/:id', 'api/v1/scene_automations/detail/:id', NULL, NULL, NULL, NULL),
	(266, 'g2', 'api/v1/command/datas/jobs', 'api/v1/command/datas/jobs', NULL, NULL, NULL, NULL),
	(267, 'g2', 'api/v1/attribute/datas/get', 'api/v1/attribute/datas/get', NULL, NULL, NULL, NULL),
	(268, 'g2', 'api/v1/command/datas/:id', 'api/v1/command/datas/:id', NULL, NULL, NULL, NULL),
	(269, 'g2', 'api/v1/user/totp/activate', 'api/v1/user/totp/activate', NULL, NULL, NULL, NULL),
	(270, 'g2', 'api/v1/device/template/:id', 'api/v1/device/template/:id', NULL, NULL, NULL, NULL),
	(271, 'g2', 'api/v1/calculated_fields/:id', 'api/v1/calculated_fields/:id', NULL, NULL, NULL, NULL),
	(272, 'g2', 'api/v1/telemetry/datas/pub', 'api/v1/telemetry/datas/pub', NULL, NULL, NULL, NULL),
	(273, 'g2', 'api/v1/rdi/devices/activate', 'api/v1/rdi/devices/activate', NULL, NULL, NULL, NULL),
	(274, 'g2', 'api/v1/device/modbus/profile/:deviceId', 'api/v1/device/modbus/profile/:deviceId', NULL, NULL, NULL, NULL),
	(275, 'g2', 'api/v1/command/datas/jobs/preview', 'api/v1/command/datas/jobs/preview', NULL, NULL, NULL, NULL),
	(276, 'g2', 'api/v1/alarm/info/history', 'api/v1/alarm/info/history', NULL, NULL, NULL, NULL),
	(277, 'g2', 'api/v1/command/datas/saved-filters/:filter_id', 'api/v1/command/datas/saved-filters/:filter_id', NULL, NULL, NULL, NULL),
	(278, 'g2', 'api/v1/device/:id/debug/status', 'api/v1/device/:id/debug/status', NULL, NULL, NULL, NULL),
	(279, 'g2', 'api/v1/user/selector', 'api/v1/user/selector', NULL, NULL, NULL, NULL),
	(280, 'g2', 'api/v1/device/shadow/:deviceId/:msgId', 'api/v1/device/shadow/:deviceId/:msgId', NULL, NULL, NULL, NULL),
	(281, 'g2', 'api/v1/device/twin/:id', 'api/v1/device/twin/:id', NULL, NULL, NULL, NULL),
	(282, 'g2', 'api/v1/ota/task/detail', 'api/v1/ota/task/detail', NULL, NULL, NULL, NULL),
	(283, 'g2', 'api/v1/device/sub-list/:id', 'api/v1/device/sub-list/:id', NULL, NULL, NULL, NULL),
	(284, 'g2', 'api/v1/notification/services/config', 'api/v1/notification/services/config', NULL, NULL, NULL, NULL),
	(285, 'g2', 'api/v1/product', 'api/v1/product', NULL, NULL, NULL, NULL),
	(286, 'g2', 'api/v1/operation_logs', 'api/v1/operation_logs', NULL, NULL, NULL, NULL),
	(287, 'g2', 'api/v1/alarm/info', 'api/v1/alarm/info', NULL, NULL, NULL, NULL),
	(288, 'p', 'SYS_ADMIN', 'api/v1/ai/alarm/analysis', 'allow', NULL, NULL, NULL),
	(289, 'p', 'TENANT_ADMIN', 'api/v1/ai/alarm/analysis', 'allow', NULL, NULL, NULL),
	(290, 'p', 'TENANT_USER', 'api/v1/ai/alarm/analysis', 'allow', NULL, NULL, NULL),
	(291, 'p', 'SYS_ADMIN', 'api/v1/ai/telemetry/query', 'allow', NULL, NULL, NULL),
	(292, 'p', 'TENANT_ADMIN', 'api/v1/ai/telemetry/query', 'allow', NULL, NULL, NULL),
	(293, 'p', 'TENANT_USER', 'api/v1/ai/telemetry/query', 'allow', NULL, NULL, NULL),
	(294, 'p', 'SYS_ADMIN', 'api/v1/alarm/config', 'allow', NULL, NULL, NULL),
	(295, 'p', 'TENANT_ADMIN', 'api/v1/alarm/config', 'allow', NULL, NULL, NULL),
	(296, 'p', 'TENANT_USER', 'api/v1/alarm/config', 'allow', NULL, NULL, NULL),
	(297, 'p', 'SYS_ADMIN', 'api/v1/alarm/config/:id', 'allow', NULL, NULL, NULL),
	(298, 'p', 'TENANT_ADMIN', 'api/v1/alarm/config/:id', 'allow', NULL, NULL, NULL),
	(299, 'p', 'TENANT_USER', 'api/v1/alarm/config/:id', 'allow', NULL, NULL, NULL),
	(300, 'p', 'SYS_ADMIN', 'api/v1/alarm/device/counts', 'allow', NULL, NULL, NULL),
	(301, 'p', 'TENANT_ADMIN', 'api/v1/alarm/device/counts', 'allow', NULL, NULL, NULL),
	(302, 'p', 'TENANT_USER', 'api/v1/alarm/device/counts', 'allow', NULL, NULL, NULL),
	(303, 'p', 'SYS_ADMIN', 'api/v1/alarm/info', 'allow', NULL, NULL, NULL),
	(304, 'p', 'TENANT_ADMIN', 'api/v1/alarm/info', 'allow', NULL, NULL, NULL),
	(305, 'p', 'TENANT_USER', 'api/v1/alarm/info', 'allow', NULL, NULL, NULL),
	(306, 'p', 'SYS_ADMIN', 'api/v1/alarm/info/batch', 'allow', NULL, NULL, NULL),
	(307, 'p', 'TENANT_ADMIN', 'api/v1/alarm/info/batch', 'allow', NULL, NULL, NULL),
	(308, 'p', 'TENANT_USER', 'api/v1/alarm/info/batch', 'allow', NULL, NULL, NULL),
	(309, 'p', 'SYS_ADMIN', 'api/v1/alarm/info/config/device', 'allow', NULL, NULL, NULL),
	(310, 'p', 'TENANT_ADMIN', 'api/v1/alarm/info/config/device', 'allow', NULL, NULL, NULL),
	(311, 'p', 'TENANT_USER', 'api/v1/alarm/info/config/device', 'allow', NULL, NULL, NULL),
	(312, 'p', 'SYS_ADMIN', 'api/v1/alarm/info/history', 'allow', NULL, NULL, NULL),
	(313, 'p', 'TENANT_ADMIN', 'api/v1/alarm/info/history', 'allow', NULL, NULL, NULL),
	(314, 'p', 'TENANT_USER', 'api/v1/alarm/info/history', 'allow', NULL, NULL, NULL),
	(315, 'p', 'SYS_ADMIN', 'api/v1/alarm/info/history/:id', 'allow', NULL, NULL, NULL),
	(316, 'p', 'TENANT_ADMIN', 'api/v1/alarm/info/history/:id', 'allow', NULL, NULL, NULL),
	(317, 'p', 'TENANT_USER', 'api/v1/alarm/info/history/:id', 'allow', NULL, NULL, NULL),
	(318, 'p', 'SYS_ADMIN', 'api/v1/alarm/info/history/:id/acknowledge', 'allow', NULL, NULL, NULL),
	(319, 'p', 'TENANT_ADMIN', 'api/v1/alarm/info/history/:id/acknowledge', 'allow', NULL, NULL, NULL),
	(320, 'p', 'TENANT_USER', 'api/v1/alarm/info/history/:id/acknowledge', 'allow', NULL, NULL, NULL),
	(321, 'p', 'SYS_ADMIN', 'api/v1/alarm/info/history/:id/reset', 'allow', NULL, NULL, NULL),
	(322, 'p', 'TENANT_ADMIN', 'api/v1/alarm/info/history/:id/reset', 'allow', NULL, NULL, NULL),
	(323, 'p', 'TENANT_USER', 'api/v1/alarm/info/history/:id/reset', 'allow', NULL, NULL, NULL),
	(324, 'p', 'SYS_ADMIN', 'api/v1/alarm/info/history/batch-action', 'allow', NULL, NULL, NULL),
	(325, 'p', 'TENANT_ADMIN', 'api/v1/alarm/info/history/batch-action', 'allow', NULL, NULL, NULL),
	(326, 'p', 'TENANT_USER', 'api/v1/alarm/info/history/batch-action', 'allow', NULL, NULL, NULL),
	(327, 'p', 'SYS_ADMIN', 'api/v1/alarm/info/history/device', 'allow', NULL, NULL, NULL),
	(328, 'p', 'TENANT_ADMIN', 'api/v1/alarm/info/history/device', 'allow', NULL, NULL, NULL),
	(329, 'p', 'TENANT_USER', 'api/v1/alarm/info/history/device', 'allow', NULL, NULL, NULL),
	(330, 'p', 'SYS_ADMIN', 'api/v1/alarm/info/history/monthly', 'allow', NULL, NULL, NULL),
	(331, 'p', 'TENANT_ADMIN', 'api/v1/alarm/info/history/monthly', 'allow', NULL, NULL, NULL),
	(332, 'p', 'TENANT_USER', 'api/v1/alarm/info/history/monthly', 'allow', NULL, NULL, NULL),
	(333, 'p', 'SYS_ADMIN', 'api/v1/asset', 'allow', NULL, NULL, NULL),
	(334, 'p', 'TENANT_ADMIN', 'api/v1/asset', 'allow', NULL, NULL, NULL),
	(335, 'p', 'TENANT_USER', 'api/v1/asset', 'allow', NULL, NULL, NULL),
	(336, 'p', 'SYS_ADMIN', 'api/v1/asset/:id', 'allow', NULL, NULL, NULL),
	(337, 'p', 'TENANT_ADMIN', 'api/v1/asset/:id', 'allow', NULL, NULL, NULL),
	(338, 'p', 'TENANT_USER', 'api/v1/asset/:id', 'allow', NULL, NULL, NULL),
	(339, 'p', 'SYS_ADMIN', 'api/v1/asset/list', 'allow', NULL, NULL, NULL),
	(340, 'p', 'TENANT_ADMIN', 'api/v1/asset/list', 'allow', NULL, NULL, NULL),
	(341, 'p', 'TENANT_USER', 'api/v1/asset/list', 'allow', NULL, NULL, NULL),
	(342, 'p', 'SYS_ADMIN', 'api/v1/asset/tree', 'allow', NULL, NULL, NULL),
	(343, 'p', 'TENANT_ADMIN', 'api/v1/asset/tree', 'allow', NULL, NULL, NULL),
	(344, 'p', 'TENANT_USER', 'api/v1/asset/tree', 'allow', NULL, NULL, NULL),
	(345, 'p', 'SYS_ADMIN', 'api/v1/attribute/datas/:id', 'allow', NULL, NULL, NULL),
	(346, 'p', 'TENANT_ADMIN', 'api/v1/attribute/datas/:id', 'allow', NULL, NULL, NULL),
	(347, 'p', 'TENANT_USER', 'api/v1/attribute/datas/:id', 'allow', NULL, NULL, NULL),
	(348, 'p', 'SYS_ADMIN', 'api/v1/attribute/datas/get', 'allow', NULL, NULL, NULL),
	(349, 'p', 'TENANT_ADMIN', 'api/v1/attribute/datas/get', 'allow', NULL, NULL, NULL),
	(350, 'p', 'TENANT_USER', 'api/v1/attribute/datas/get', 'allow', NULL, NULL, NULL),
	(351, 'p', 'SYS_ADMIN', 'api/v1/attribute/datas/key', 'allow', NULL, NULL, NULL),
	(352, 'p', 'TENANT_ADMIN', 'api/v1/attribute/datas/key', 'allow', NULL, NULL, NULL),
	(353, 'p', 'TENANT_USER', 'api/v1/attribute/datas/key', 'allow', NULL, NULL, NULL),
	(354, 'p', 'SYS_ADMIN', 'api/v1/attribute/datas/pub', 'allow', NULL, NULL, NULL),
	(355, 'p', 'TENANT_ADMIN', 'api/v1/attribute/datas/pub', 'allow', NULL, NULL, NULL),
	(356, 'p', 'TENANT_USER', 'api/v1/attribute/datas/pub', 'allow', NULL, NULL, NULL),
	(357, 'p', 'SYS_ADMIN', 'api/v1/attribute/datas/set/logs', 'allow', NULL, NULL, NULL),
	(358, 'p', 'TENANT_ADMIN', 'api/v1/attribute/datas/set/logs', 'allow', NULL, NULL, NULL),
	(359, 'p', 'TENANT_USER', 'api/v1/attribute/datas/set/logs', 'allow', NULL, NULL, NULL),
	(360, 'p', 'SYS_ADMIN', 'api/v1/board', 'allow', NULL, NULL, NULL),
	(361, 'p', 'TENANT_ADMIN', 'api/v1/board', 'allow', NULL, NULL, NULL),
	(362, 'p', 'TENANT_USER', 'api/v1/board', 'allow', NULL, NULL, NULL),
	(363, 'p', 'SYS_ADMIN', 'api/v1/board/:id', 'allow', NULL, NULL, NULL),
	(364, 'p', 'TENANT_ADMIN', 'api/v1/board/:id', 'allow', NULL, NULL, NULL),
	(365, 'p', 'TENANT_USER', 'api/v1/board/:id', 'allow', NULL, NULL, NULL),
	(366, 'p', 'SYS_ADMIN', 'api/v1/board/:id/publish', 'allow', NULL, NULL, NULL),
	(367, 'p', 'TENANT_ADMIN', 'api/v1/board/:id/publish', 'allow', NULL, NULL, NULL),
	(368, 'p', 'TENANT_USER', 'api/v1/board/:id/publish', 'allow', NULL, NULL, NULL),
	(369, 'p', 'SYS_ADMIN', 'api/v1/board/device', 'allow', NULL, NULL, NULL),
	(370, 'p', 'TENANT_ADMIN', 'api/v1/board/device', 'allow', NULL, NULL, NULL),
	(371, 'p', 'TENANT_USER', 'api/v1/board/device', 'allow', NULL, NULL, NULL),
	(372, 'p', 'SYS_ADMIN', 'api/v1/board/device/total', 'allow', NULL, NULL, NULL),
	(373, 'p', 'TENANT_ADMIN', 'api/v1/board/device/total', 'allow', NULL, NULL, NULL),
	(374, 'p', 'TENANT_USER', 'api/v1/board/device/total', 'allow', NULL, NULL, NULL),
	(375, 'p', 'SYS_ADMIN', 'api/v1/board/home', 'allow', NULL, NULL, NULL),
	(376, 'p', 'TENANT_ADMIN', 'api/v1/board/home', 'allow', NULL, NULL, NULL),
	(377, 'p', 'TENANT_USER', 'api/v1/board/home', 'allow', NULL, NULL, NULL),
	(378, 'p', 'SYS_ADMIN', 'api/v1/board/tenant', 'allow', NULL, NULL, NULL),
	(379, 'p', 'TENANT_ADMIN', 'api/v1/board/tenant', 'allow', NULL, NULL, NULL),
	(380, 'p', 'TENANT_USER', 'api/v1/board/tenant', 'allow', NULL, NULL, NULL),
	(381, 'p', 'SYS_ADMIN', 'api/v1/board/tenant/device/info', 'allow', NULL, NULL, NULL),
	(382, 'p', 'TENANT_ADMIN', 'api/v1/board/tenant/device/info', 'allow', NULL, NULL, NULL),
	(383, 'p', 'TENANT_USER', 'api/v1/board/tenant/device/info', 'allow', NULL, NULL, NULL),
	(384, 'p', 'SYS_ADMIN', 'api/v1/board/tenant/user/info', 'allow', NULL, NULL, NULL),
	(385, 'p', 'TENANT_ADMIN', 'api/v1/board/tenant/user/info', 'allow', NULL, NULL, NULL),
	(386, 'p', 'TENANT_USER', 'api/v1/board/tenant/user/info', 'allow', NULL, NULL, NULL),
	(387, 'p', 'SYS_ADMIN', 'api/v1/board/trend', 'allow', NULL, NULL, NULL),
	(388, 'p', 'TENANT_ADMIN', 'api/v1/board/trend', 'allow', NULL, NULL, NULL),
	(389, 'p', 'TENANT_USER', 'api/v1/board/trend', 'allow', NULL, NULL, NULL),
	(390, 'p', 'SYS_ADMIN', 'api/v1/board/user/info', 'allow', NULL, NULL, NULL),
	(391, 'p', 'TENANT_ADMIN', 'api/v1/board/user/info', 'allow', NULL, NULL, NULL),
	(392, 'p', 'TENANT_USER', 'api/v1/board/user/info', 'allow', NULL, NULL, NULL),
	(393, 'p', 'SYS_ADMIN', 'api/v1/board/user/update', 'allow', NULL, NULL, NULL),
	(394, 'p', 'TENANT_ADMIN', 'api/v1/board/user/update', 'allow', NULL, NULL, NULL),
	(395, 'p', 'TENANT_USER', 'api/v1/board/user/update', 'allow', NULL, NULL, NULL),
	(396, 'p', 'SYS_ADMIN', 'api/v1/board/user/update/password', 'allow', NULL, NULL, NULL),
	(397, 'p', 'TENANT_ADMIN', 'api/v1/board/user/update/password', 'allow', NULL, NULL, NULL),
	(398, 'p', 'TENANT_USER', 'api/v1/board/user/update/password', 'allow', NULL, NULL, NULL),
	(399, 'p', 'SYS_ADMIN', 'api/v1/calculated_fields', 'allow', NULL, NULL, NULL),
	(400, 'p', 'TENANT_ADMIN', 'api/v1/calculated_fields', 'allow', NULL, NULL, NULL);
INSERT INTO public.casbin_rule (id, ptype, v0, v1, v2, v3, v4, v5) VALUES
	(401, 'p', 'TENANT_USER', 'api/v1/calculated_fields', 'allow', NULL, NULL, NULL),
	(402, 'p', 'SYS_ADMIN', 'api/v1/calculated_fields/:id', 'allow', NULL, NULL, NULL),
	(403, 'p', 'TENANT_ADMIN', 'api/v1/calculated_fields/:id', 'allow', NULL, NULL, NULL),
	(404, 'p', 'TENANT_USER', 'api/v1/calculated_fields/:id', 'allow', NULL, NULL, NULL),
	(405, 'p', 'SYS_ADMIN', 'api/v1/calculated_fields/:id/toggle', 'allow', NULL, NULL, NULL),
	(406, 'p', 'TENANT_ADMIN', 'api/v1/calculated_fields/:id/toggle', 'allow', NULL, NULL, NULL),
	(407, 'p', 'TENANT_USER', 'api/v1/calculated_fields/:id/toggle', 'allow', NULL, NULL, NULL),
	(408, 'p', 'SYS_ADMIN', 'api/v1/casbin/function', 'allow', NULL, NULL, NULL),
	(409, 'p', 'TENANT_ADMIN', 'api/v1/casbin/function', 'allow', NULL, NULL, NULL),
	(411, 'p', 'SYS_ADMIN', 'api/v1/casbin/function/:id', 'allow', NULL, NULL, NULL),
	(412, 'p', 'TENANT_ADMIN', 'api/v1/casbin/function/:id', 'allow', NULL, NULL, NULL),
	(414, 'p', 'SYS_ADMIN', 'api/v1/casbin/user', 'allow', NULL, NULL, NULL),
	(415, 'p', 'TENANT_ADMIN', 'api/v1/casbin/user', 'allow', NULL, NULL, NULL),
	(417, 'p', 'SYS_ADMIN', 'api/v1/casbin/user/:id', 'allow', NULL, NULL, NULL),
	(418, 'p', 'TENANT_ADMIN', 'api/v1/casbin/user/:id', 'allow', NULL, NULL, NULL),
	(420, 'p', 'SYS_ADMIN', 'api/v1/command/datas/:id', 'allow', NULL, NULL, NULL),
	(421, 'p', 'TENANT_ADMIN', 'api/v1/command/datas/:id', 'allow', NULL, NULL, NULL),
	(422, 'p', 'TENANT_USER', 'api/v1/command/datas/:id', 'allow', NULL, NULL, NULL),
	(423, 'p', 'SYS_ADMIN', 'api/v1/command/datas/delivery/diagnostics/:device_id', 'allow', NULL, NULL, NULL),
	(424, 'p', 'TENANT_ADMIN', 'api/v1/command/datas/delivery/diagnostics/:device_id', 'allow', NULL, NULL, NULL),
	(425, 'p', 'TENANT_USER', 'api/v1/command/datas/delivery/diagnostics/:device_id', 'allow', NULL, NULL, NULL),
	(426, 'p', 'SYS_ADMIN', 'api/v1/command/datas/direct-method', 'allow', NULL, NULL, NULL),
	(427, 'p', 'TENANT_ADMIN', 'api/v1/command/datas/direct-method', 'allow', NULL, NULL, NULL),
	(428, 'p', 'TENANT_USER', 'api/v1/command/datas/direct-method', 'allow', NULL, NULL, NULL),
	(429, 'p', 'SYS_ADMIN', 'api/v1/command/datas/jobs', 'allow', NULL, NULL, NULL),
	(430, 'p', 'TENANT_ADMIN', 'api/v1/command/datas/jobs', 'allow', NULL, NULL, NULL),
	(431, 'p', 'TENANT_USER', 'api/v1/command/datas/jobs', 'allow', NULL, NULL, NULL),
	(432, 'p', 'SYS_ADMIN', 'api/v1/command/datas/jobs/:job_id', 'allow', NULL, NULL, NULL),
	(433, 'p', 'TENANT_ADMIN', 'api/v1/command/datas/jobs/:job_id', 'allow', NULL, NULL, NULL),
	(434, 'p', 'TENANT_USER', 'api/v1/command/datas/jobs/:job_id', 'allow', NULL, NULL, NULL),
	(435, 'p', 'SYS_ADMIN', 'api/v1/command/datas/jobs/:job_id/cancel', 'allow', NULL, NULL, NULL),
	(436, 'p', 'TENANT_ADMIN', 'api/v1/command/datas/jobs/:job_id/cancel', 'allow', NULL, NULL, NULL),
	(437, 'p', 'TENANT_USER', 'api/v1/command/datas/jobs/:job_id/cancel', 'allow', NULL, NULL, NULL),
	(438, 'p', 'SYS_ADMIN', 'api/v1/command/datas/jobs/:job_id/retry', 'allow', NULL, NULL, NULL),
	(439, 'p', 'TENANT_ADMIN', 'api/v1/command/datas/jobs/:job_id/retry', 'allow', NULL, NULL, NULL),
	(440, 'p', 'TENANT_USER', 'api/v1/command/datas/jobs/:job_id/retry', 'allow', NULL, NULL, NULL),
	(441, 'p', 'SYS_ADMIN', 'api/v1/command/datas/jobs/:job_id/rows', 'allow', NULL, NULL, NULL),
	(442, 'p', 'TENANT_ADMIN', 'api/v1/command/datas/jobs/:job_id/rows', 'allow', NULL, NULL, NULL),
	(443, 'p', 'TENANT_USER', 'api/v1/command/datas/jobs/:job_id/rows', 'allow', NULL, NULL, NULL),
	(444, 'p', 'SYS_ADMIN', 'api/v1/command/datas/jobs/:job_id/support-bundle', 'allow', NULL, NULL, NULL),
	(445, 'p', 'TENANT_ADMIN', 'api/v1/command/datas/jobs/:job_id/support-bundle', 'allow', NULL, NULL, NULL),
	(446, 'p', 'TENANT_USER', 'api/v1/command/datas/jobs/:job_id/support-bundle', 'allow', NULL, NULL, NULL),
	(447, 'p', 'SYS_ADMIN', 'api/v1/command/datas/jobs/preview', 'allow', NULL, NULL, NULL),
	(448, 'p', 'TENANT_ADMIN', 'api/v1/command/datas/jobs/preview', 'allow', NULL, NULL, NULL),
	(449, 'p', 'TENANT_USER', 'api/v1/command/datas/jobs/preview', 'allow', NULL, NULL, NULL),
	(450, 'p', 'SYS_ADMIN', 'api/v1/command/datas/jobs/submit', 'allow', NULL, NULL, NULL),
	(451, 'p', 'TENANT_ADMIN', 'api/v1/command/datas/jobs/submit', 'allow', NULL, NULL, NULL),
	(452, 'p', 'TENANT_USER', 'api/v1/command/datas/jobs/submit', 'allow', NULL, NULL, NULL),
	(453, 'p', 'SYS_ADMIN', 'api/v1/command/datas/pub', 'allow', NULL, NULL, NULL),
	(454, 'p', 'TENANT_ADMIN', 'api/v1/command/datas/pub', 'allow', NULL, NULL, NULL),
	(455, 'p', 'TENANT_USER', 'api/v1/command/datas/pub', 'allow', NULL, NULL, NULL),
	(456, 'p', 'SYS_ADMIN', 'api/v1/command/datas/saved-filters', 'allow', NULL, NULL, NULL),
	(457, 'p', 'TENANT_ADMIN', 'api/v1/command/datas/saved-filters', 'allow', NULL, NULL, NULL),
	(458, 'p', 'TENANT_USER', 'api/v1/command/datas/saved-filters', 'allow', NULL, NULL, NULL),
	(459, 'p', 'SYS_ADMIN', 'api/v1/command/datas/saved-filters/:filter_id', 'allow', NULL, NULL, NULL),
	(460, 'p', 'TENANT_ADMIN', 'api/v1/command/datas/saved-filters/:filter_id', 'allow', NULL, NULL, NULL),
	(461, 'p', 'TENANT_USER', 'api/v1/command/datas/saved-filters/:filter_id', 'allow', NULL, NULL, NULL),
	(462, 'p', 'SYS_ADMIN', 'api/v1/command/datas/set/logs', 'allow', NULL, NULL, NULL),
	(463, 'p', 'TENANT_ADMIN', 'api/v1/command/datas/set/logs', 'allow', NULL, NULL, NULL),
	(464, 'p', 'TENANT_USER', 'api/v1/command/datas/set/logs', 'allow', NULL, NULL, NULL),
	(465, 'p', 'SYS_ADMIN', 'api/v1/dashboard-menu/:dashboardId', 'allow', NULL, NULL, NULL),
	(466, 'p', 'TENANT_ADMIN', 'api/v1/dashboard-menu/:dashboardId', 'allow', NULL, NULL, NULL),
	(468, 'p', 'SYS_ADMIN', 'api/v1/dashboard-menu/batch', 'allow', NULL, NULL, NULL),
	(469, 'p', 'TENANT_ADMIN', 'api/v1/dashboard-menu/batch', 'allow', NULL, NULL, NULL),
	(471, 'p', 'SYS_ADMIN', 'api/v1/data_script', 'allow', NULL, NULL, NULL),
	(472, 'p', 'TENANT_ADMIN', 'api/v1/data_script', 'allow', NULL, NULL, NULL),
	(473, 'p', 'TENANT_USER', 'api/v1/data_script', 'allow', NULL, NULL, NULL),
	(474, 'p', 'SYS_ADMIN', 'api/v1/data_script/:id', 'allow', NULL, NULL, NULL),
	(475, 'p', 'TENANT_ADMIN', 'api/v1/data_script/:id', 'allow', NULL, NULL, NULL),
	(476, 'p', 'TENANT_USER', 'api/v1/data_script/:id', 'allow', NULL, NULL, NULL),
	(477, 'p', 'SYS_ADMIN', 'api/v1/data_script/enable', 'allow', NULL, NULL, NULL),
	(478, 'p', 'TENANT_ADMIN', 'api/v1/data_script/enable', 'allow', NULL, NULL, NULL),
	(479, 'p', 'TENANT_USER', 'api/v1/data_script/enable', 'allow', NULL, NULL, NULL),
	(480, 'p', 'SYS_ADMIN', 'api/v1/data_script/quiz', 'allow', NULL, NULL, NULL),
	(481, 'p', 'TENANT_ADMIN', 'api/v1/data_script/quiz', 'allow', NULL, NULL, NULL),
	(482, 'p', 'TENANT_USER', 'api/v1/data_script/quiz', 'allow', NULL, NULL, NULL),
	(483, 'p', 'SYS_ADMIN', 'api/v1/datapolicy', 'allow', NULL, NULL, NULL),
	(484, 'p', 'TENANT_ADMIN', 'api/v1/datapolicy', 'allow', NULL, NULL, NULL),
	(485, 'p', 'TENANT_USER', 'api/v1/datapolicy', 'allow', NULL, NULL, NULL),
	(486, 'p', 'SYS_ADMIN', 'api/v1/device', 'allow', NULL, NULL, NULL),
	(487, 'p', 'TENANT_ADMIN', 'api/v1/device', 'allow', NULL, NULL, NULL),
	(488, 'p', 'TENANT_USER', 'api/v1/device', 'allow', NULL, NULL, NULL),
	(489, 'p', 'SYS_ADMIN', 'api/v1/device/:id', 'allow', NULL, NULL, NULL),
	(490, 'p', 'TENANT_ADMIN', 'api/v1/device/:id', 'allow', NULL, NULL, NULL),
	(491, 'p', 'TENANT_USER', 'api/v1/device/:id', 'allow', NULL, NULL, NULL),
	(492, 'p', 'SYS_ADMIN', 'api/v1/device/:id/connection/diagnostics', 'allow', NULL, NULL, NULL),
	(493, 'p', 'TENANT_ADMIN', 'api/v1/device/:id/connection/diagnostics', 'allow', NULL, NULL, NULL),
	(494, 'p', 'TENANT_USER', 'api/v1/device/:id/connection/diagnostics', 'allow', NULL, NULL, NULL),
	(495, 'p', 'SYS_ADMIN', 'api/v1/device/:id/debug', 'allow', NULL, NULL, NULL),
	(496, 'p', 'TENANT_ADMIN', 'api/v1/device/:id/debug', 'allow', NULL, NULL, NULL),
	(497, 'p', 'TENANT_USER', 'api/v1/device/:id/debug', 'allow', NULL, NULL, NULL),
	(498, 'p', 'SYS_ADMIN', 'api/v1/device/:id/debug/logs', 'allow', NULL, NULL, NULL),
	(499, 'p', 'TENANT_ADMIN', 'api/v1/device/:id/debug/logs', 'allow', NULL, NULL, NULL),
	(500, 'p', 'TENANT_USER', 'api/v1/device/:id/debug/logs', 'allow', NULL, NULL, NULL),
	(501, 'p', 'SYS_ADMIN', 'api/v1/device/:id/debug/status', 'allow', NULL, NULL, NULL),
	(502, 'p', 'TENANT_ADMIN', 'api/v1/device/:id/debug/status', 'allow', NULL, NULL, NULL),
	(503, 'p', 'TENANT_USER', 'api/v1/device/:id/debug/status', 'allow', NULL, NULL, NULL),
	(504, 'p', 'SYS_ADMIN', 'api/v1/device/:id/mqtt-debug/session', 'allow', NULL, NULL, NULL),
	(505, 'p', 'TENANT_ADMIN', 'api/v1/device/:id/mqtt-debug/session', 'allow', NULL, NULL, NULL),
	(506, 'p', 'TENANT_USER', 'api/v1/device/:id/mqtt-debug/session', 'allow', NULL, NULL, NULL),
	(507, 'p', 'SYS_ADMIN', 'api/v1/device/:id/mqtt-debug/session/:session_id', 'allow', NULL, NULL, NULL),
	(508, 'p', 'TENANT_ADMIN', 'api/v1/device/:id/mqtt-debug/session/:session_id', 'allow', NULL, NULL, NULL),
	(509, 'p', 'TENANT_USER', 'api/v1/device/:id/mqtt-debug/session/:session_id', 'allow', NULL, NULL, NULL),
	(510, 'p', 'SYS_ADMIN', 'api/v1/device/:id/mqtt-debug/session/:session_id/command', 'allow', NULL, NULL, NULL),
	(511, 'p', 'TENANT_ADMIN', 'api/v1/device/:id/mqtt-debug/session/:session_id/command', 'allow', NULL, NULL, NULL),
	(512, 'p', 'TENANT_USER', 'api/v1/device/:id/mqtt-debug/session/:session_id/command', 'allow', NULL, NULL, NULL),
	(513, 'p', 'SYS_ADMIN', 'api/v1/device/:id/onboarding/connection-guide', 'allow', NULL, NULL, NULL),
	(514, 'p', 'TENANT_ADMIN', 'api/v1/device/:id/onboarding/connection-guide', 'allow', NULL, NULL, NULL),
	(515, 'p', 'TENANT_USER', 'api/v1/device/:id/onboarding/connection-guide', 'allow', NULL, NULL, NULL),
	(516, 'p', 'SYS_ADMIN', 'api/v1/device/active', 'allow', NULL, NULL, NULL),
	(517, 'p', 'TENANT_ADMIN', 'api/v1/device/active', 'allow', NULL, NULL, NULL),
	(518, 'p', 'TENANT_USER', 'api/v1/device/active', 'allow', NULL, NULL, NULL),
	(519, 'p', 'SYS_ADMIN', 'api/v1/device/check/:deviceNumber', 'allow', NULL, NULL, NULL),
	(520, 'p', 'TENANT_ADMIN', 'api/v1/device/check/:deviceNumber', 'allow', NULL, NULL, NULL),
	(521, 'p', 'TENANT_USER', 'api/v1/device/check/:deviceNumber', 'allow', NULL, NULL, NULL),
	(522, 'p', 'SYS_ADMIN', 'api/v1/device/connect/form', 'allow', NULL, NULL, NULL),
	(523, 'p', 'TENANT_ADMIN', 'api/v1/device/connect/form', 'allow', NULL, NULL, NULL),
	(524, 'p', 'TENANT_USER', 'api/v1/device/connect/form', 'allow', NULL, NULL, NULL),
	(525, 'p', 'SYS_ADMIN', 'api/v1/device/connect/info', 'allow', NULL, NULL, NULL),
	(526, 'p', 'TENANT_ADMIN', 'api/v1/device/connect/info', 'allow', NULL, NULL, NULL),
	(527, 'p', 'TENANT_USER', 'api/v1/device/connect/info', 'allow', NULL, NULL, NULL),
	(528, 'p', 'SYS_ADMIN', 'api/v1/device/detail/:id', 'allow', NULL, NULL, NULL),
	(529, 'p', 'TENANT_ADMIN', 'api/v1/device/detail/:id', 'allow', NULL, NULL, NULL),
	(530, 'p', 'TENANT_USER', 'api/v1/device/detail/:id', 'allow', NULL, NULL, NULL),
	(531, 'p', 'SYS_ADMIN', 'api/v1/device/group', 'allow', NULL, NULL, NULL),
	(532, 'p', 'TENANT_ADMIN', 'api/v1/device/group', 'allow', NULL, NULL, NULL),
	(533, 'p', 'TENANT_USER', 'api/v1/device/group', 'allow', NULL, NULL, NULL),
	(534, 'p', 'SYS_ADMIN', 'api/v1/device/group/:id', 'allow', NULL, NULL, NULL),
	(535, 'p', 'TENANT_ADMIN', 'api/v1/device/group/:id', 'allow', NULL, NULL, NULL),
	(536, 'p', 'TENANT_USER', 'api/v1/device/group/:id', 'allow', NULL, NULL, NULL),
	(537, 'p', 'SYS_ADMIN', 'api/v1/device/group/detail/:id', 'allow', NULL, NULL, NULL),
	(538, 'p', 'TENANT_ADMIN', 'api/v1/device/group/detail/:id', 'allow', NULL, NULL, NULL),
	(539, 'p', 'TENANT_USER', 'api/v1/device/group/detail/:id', 'allow', NULL, NULL, NULL),
	(540, 'p', 'SYS_ADMIN', 'api/v1/device/group/relation', 'allow', NULL, NULL, NULL),
	(541, 'p', 'TENANT_ADMIN', 'api/v1/device/group/relation', 'allow', NULL, NULL, NULL),
	(542, 'p', 'TENANT_USER', 'api/v1/device/group/relation', 'allow', NULL, NULL, NULL),
	(543, 'p', 'SYS_ADMIN', 'api/v1/device/group/relation/list', 'allow', NULL, NULL, NULL),
	(544, 'p', 'TENANT_ADMIN', 'api/v1/device/group/relation/list', 'allow', NULL, NULL, NULL),
	(545, 'p', 'TENANT_USER', 'api/v1/device/group/relation/list', 'allow', NULL, NULL, NULL),
	(546, 'p', 'SYS_ADMIN', 'api/v1/device/group/tree', 'allow', NULL, NULL, NULL),
	(547, 'p', 'TENANT_ADMIN', 'api/v1/device/group/tree', 'allow', NULL, NULL, NULL),
	(548, 'p', 'TENANT_USER', 'api/v1/device/group/tree', 'allow', NULL, NULL, NULL),
	(549, 'p', 'SYS_ADMIN', 'api/v1/device/list', 'allow', NULL, NULL, NULL),
	(550, 'p', 'TENANT_ADMIN', 'api/v1/device/list', 'allow', NULL, NULL, NULL),
	(551, 'p', 'TENANT_USER', 'api/v1/device/list', 'allow', NULL, NULL, NULL),
	(552, 'p', 'SYS_ADMIN', 'api/v1/device/map/telemetry/:id', 'allow', NULL, NULL, NULL),
	(553, 'p', 'TENANT_ADMIN', 'api/v1/device/map/telemetry/:id', 'allow', NULL, NULL, NULL),
	(554, 'p', 'TENANT_USER', 'api/v1/device/map/telemetry/:id', 'allow', NULL, NULL, NULL),
	(555, 'p', 'SYS_ADMIN', 'api/v1/device/metrics/:id', 'allow', NULL, NULL, NULL),
	(556, 'p', 'TENANT_ADMIN', 'api/v1/device/metrics/:id', 'allow', NULL, NULL, NULL),
	(557, 'p', 'TENANT_USER', 'api/v1/device/metrics/:id', 'allow', NULL, NULL, NULL),
	(558, 'p', 'SYS_ADMIN', 'api/v1/device/metrics/chart', 'allow', NULL, NULL, NULL),
	(559, 'p', 'TENANT_ADMIN', 'api/v1/device/metrics/chart', 'allow', NULL, NULL, NULL),
	(560, 'p', 'TENANT_USER', 'api/v1/device/metrics/chart', 'allow', NULL, NULL, NULL),
	(561, 'p', 'SYS_ADMIN', 'api/v1/device/metrics/condition/menu', 'allow', NULL, NULL, NULL),
	(562, 'p', 'TENANT_ADMIN', 'api/v1/device/metrics/condition/menu', 'allow', NULL, NULL, NULL),
	(563, 'p', 'TENANT_USER', 'api/v1/device/metrics/condition/menu', 'allow', NULL, NULL, NULL),
	(564, 'p', 'SYS_ADMIN', 'api/v1/device/metrics/menu', 'allow', NULL, NULL, NULL),
	(565, 'p', 'TENANT_ADMIN', 'api/v1/device/metrics/menu', 'allow', NULL, NULL, NULL),
	(566, 'p', 'TENANT_USER', 'api/v1/device/metrics/menu', 'allow', NULL, NULL, NULL),
	(567, 'p', 'SYS_ADMIN', 'api/v1/device/modbus/profile/:deviceId', 'allow', NULL, NULL, NULL),
	(568, 'p', 'TENANT_ADMIN', 'api/v1/device/modbus/profile/:deviceId', 'allow', NULL, NULL, NULL),
	(569, 'p', 'TENANT_USER', 'api/v1/device/modbus/profile/:deviceId', 'allow', NULL, NULL, NULL),
	(570, 'p', 'SYS_ADMIN', 'api/v1/device/modbus/profile/number/:deviceNumber', 'allow', NULL, NULL, NULL),
	(571, 'p', 'TENANT_ADMIN', 'api/v1/device/modbus/profile/number/:deviceNumber', 'allow', NULL, NULL, NULL),
	(572, 'p', 'TENANT_USER', 'api/v1/device/modbus/profile/number/:deviceNumber', 'allow', NULL, NULL, NULL),
	(573, 'p', 'SYS_ADMIN', 'api/v1/device/model/attributes', 'allow', NULL, NULL, NULL),
	(574, 'p', 'TENANT_ADMIN', 'api/v1/device/model/attributes', 'allow', NULL, NULL, NULL),
	(575, 'p', 'TENANT_USER', 'api/v1/device/model/attributes', 'allow', NULL, NULL, NULL),
	(576, 'p', 'SYS_ADMIN', 'api/v1/device/model/attributes/:id', 'allow', NULL, NULL, NULL),
	(577, 'p', 'TENANT_ADMIN', 'api/v1/device/model/attributes/:id', 'allow', NULL, NULL, NULL),
	(578, 'p', 'TENANT_USER', 'api/v1/device/model/attributes/:id', 'allow', NULL, NULL, NULL),
	(579, 'p', 'SYS_ADMIN', 'api/v1/device/model/commands', 'allow', NULL, NULL, NULL),
	(580, 'p', 'TENANT_ADMIN', 'api/v1/device/model/commands', 'allow', NULL, NULL, NULL),
	(581, 'p', 'TENANT_USER', 'api/v1/device/model/commands', 'allow', NULL, NULL, NULL),
	(582, 'p', 'SYS_ADMIN', 'api/v1/device/model/commands/:id', 'allow', NULL, NULL, NULL),
	(583, 'p', 'TENANT_ADMIN', 'api/v1/device/model/commands/:id', 'allow', NULL, NULL, NULL),
	(584, 'p', 'TENANT_USER', 'api/v1/device/model/commands/:id', 'allow', NULL, NULL, NULL),
	(585, 'p', 'SYS_ADMIN', 'api/v1/device/model/custom/commands', 'allow', NULL, NULL, NULL),
	(586, 'p', 'TENANT_ADMIN', 'api/v1/device/model/custom/commands', 'allow', NULL, NULL, NULL),
	(587, 'p', 'TENANT_USER', 'api/v1/device/model/custom/commands', 'allow', NULL, NULL, NULL),
	(588, 'p', 'SYS_ADMIN', 'api/v1/device/model/custom/commands/:deviceId', 'allow', NULL, NULL, NULL),
	(589, 'p', 'TENANT_ADMIN', 'api/v1/device/model/custom/commands/:deviceId', 'allow', NULL, NULL, NULL),
	(590, 'p', 'TENANT_USER', 'api/v1/device/model/custom/commands/:deviceId', 'allow', NULL, NULL, NULL),
	(591, 'p', 'SYS_ADMIN', 'api/v1/device/model/custom/commands/:id', 'allow', NULL, NULL, NULL),
	(592, 'p', 'TENANT_ADMIN', 'api/v1/device/model/custom/commands/:id', 'allow', NULL, NULL, NULL),
	(593, 'p', 'TENANT_USER', 'api/v1/device/model/custom/commands/:id', 'allow', NULL, NULL, NULL),
	(594, 'p', 'SYS_ADMIN', 'api/v1/device/model/custom/control', 'allow', NULL, NULL, NULL),
	(595, 'p', 'TENANT_ADMIN', 'api/v1/device/model/custom/control', 'allow', NULL, NULL, NULL),
	(596, 'p', 'TENANT_USER', 'api/v1/device/model/custom/control', 'allow', NULL, NULL, NULL),
	(597, 'p', 'SYS_ADMIN', 'api/v1/device/model/custom/control/:id', 'allow', NULL, NULL, NULL),
	(598, 'p', 'TENANT_ADMIN', 'api/v1/device/model/custom/control/:id', 'allow', NULL, NULL, NULL),
	(599, 'p', 'TENANT_USER', 'api/v1/device/model/custom/control/:id', 'allow', NULL, NULL, NULL),
	(600, 'p', 'SYS_ADMIN', 'api/v1/device/model/events', 'allow', NULL, NULL, NULL),
	(601, 'p', 'TENANT_ADMIN', 'api/v1/device/model/events', 'allow', NULL, NULL, NULL),
	(602, 'p', 'TENANT_USER', 'api/v1/device/model/events', 'allow', NULL, NULL, NULL),
	(603, 'p', 'SYS_ADMIN', 'api/v1/device/model/events/:id', 'allow', NULL, NULL, NULL),
	(604, 'p', 'TENANT_ADMIN', 'api/v1/device/model/events/:id', 'allow', NULL, NULL, NULL),
	(605, 'p', 'TENANT_USER', 'api/v1/device/model/events/:id', 'allow', NULL, NULL, NULL),
	(606, 'p', 'SYS_ADMIN', 'api/v1/device/model/source/at/list', 'allow', NULL, NULL, NULL);
INSERT INTO public.casbin_rule (id, ptype, v0, v1, v2, v3, v4, v5) VALUES
	(607, 'p', 'TENANT_ADMIN', 'api/v1/device/model/source/at/list', 'allow', NULL, NULL, NULL),
	(608, 'p', 'TENANT_USER', 'api/v1/device/model/source/at/list', 'allow', NULL, NULL, NULL),
	(609, 'p', 'SYS_ADMIN', 'api/v1/device/model/telemetry', 'allow', NULL, NULL, NULL),
	(610, 'p', 'TENANT_ADMIN', 'api/v1/device/model/telemetry', 'allow', NULL, NULL, NULL),
	(611, 'p', 'TENANT_USER', 'api/v1/device/model/telemetry', 'allow', NULL, NULL, NULL),
	(612, 'p', 'SYS_ADMIN', 'api/v1/device/model/telemetry/:id', 'allow', NULL, NULL, NULL),
	(613, 'p', 'TENANT_ADMIN', 'api/v1/device/model/telemetry/:id', 'allow', NULL, NULL, NULL),
	(614, 'p', 'TENANT_USER', 'api/v1/device/model/telemetry/:id', 'allow', NULL, NULL, NULL),
	(615, 'p', 'SYS_ADMIN', 'api/v1/device/online/status/:id', 'allow', NULL, NULL, NULL),
	(616, 'p', 'TENANT_ADMIN', 'api/v1/device/online/status/:id', 'allow', NULL, NULL, NULL),
	(617, 'p', 'TENANT_USER', 'api/v1/device/online/status/:id', 'allow', NULL, NULL, NULL),
	(618, 'p', 'SYS_ADMIN', 'api/v1/device/preRegister', 'allow', NULL, NULL, NULL),
	(619, 'p', 'TENANT_ADMIN', 'api/v1/device/preRegister', 'allow', NULL, NULL, NULL),
	(620, 'p', 'TENANT_USER', 'api/v1/device/preRegister', 'allow', NULL, NULL, NULL),
	(621, 'p', 'SYS_ADMIN', 'api/v1/device/preRegister/export', 'allow', NULL, NULL, NULL),
	(622, 'p', 'TENANT_ADMIN', 'api/v1/device/preRegister/export', 'allow', NULL, NULL, NULL),
	(623, 'p', 'TENANT_USER', 'api/v1/device/preRegister/export', 'allow', NULL, NULL, NULL),
	(624, 'p', 'SYS_ADMIN', 'api/v1/device/selector', 'allow', NULL, NULL, NULL),
	(625, 'p', 'TENANT_ADMIN', 'api/v1/device/selector', 'allow', NULL, NULL, NULL),
	(626, 'p', 'TENANT_USER', 'api/v1/device/selector', 'allow', NULL, NULL, NULL),
	(627, 'p', 'SYS_ADMIN', 'api/v1/device/service/access/batch', 'allow', NULL, NULL, NULL),
	(628, 'p', 'TENANT_ADMIN', 'api/v1/device/service/access/batch', 'allow', NULL, NULL, NULL),
	(629, 'p', 'TENANT_USER', 'api/v1/device/service/access/batch', 'allow', NULL, NULL, NULL),
	(630, 'p', 'SYS_ADMIN', 'api/v1/device/shadow/:deviceId', 'allow', NULL, NULL, NULL),
	(631, 'p', 'TENANT_ADMIN', 'api/v1/device/shadow/:deviceId', 'allow', NULL, NULL, NULL),
	(632, 'p', 'TENANT_USER', 'api/v1/device/shadow/:deviceId', 'allow', NULL, NULL, NULL),
	(633, 'p', 'SYS_ADMIN', 'api/v1/device/shadow/:deviceId/:msgId', 'allow', NULL, NULL, NULL),
	(634, 'p', 'TENANT_ADMIN', 'api/v1/device/shadow/:deviceId/:msgId', 'allow', NULL, NULL, NULL),
	(635, 'p', 'TENANT_USER', 'api/v1/device/shadow/:deviceId/:msgId', 'allow', NULL, NULL, NULL),
	(636, 'p', 'SYS_ADMIN', 'api/v1/device/son/add', 'allow', NULL, NULL, NULL),
	(637, 'p', 'TENANT_ADMIN', 'api/v1/device/son/add', 'allow', NULL, NULL, NULL),
	(638, 'p', 'TENANT_USER', 'api/v1/device/son/add', 'allow', NULL, NULL, NULL),
	(639, 'p', 'SYS_ADMIN', 'api/v1/device/status/history', 'allow', NULL, NULL, NULL),
	(640, 'p', 'TENANT_ADMIN', 'api/v1/device/status/history', 'allow', NULL, NULL, NULL),
	(641, 'p', 'TENANT_USER', 'api/v1/device/status/history', 'allow', NULL, NULL, NULL),
	(642, 'p', 'SYS_ADMIN', 'api/v1/device/sub-list/:id', 'allow', NULL, NULL, NULL),
	(643, 'p', 'TENANT_ADMIN', 'api/v1/device/sub-list/:id', 'allow', NULL, NULL, NULL),
	(644, 'p', 'TENANT_USER', 'api/v1/device/sub-list/:id', 'allow', NULL, NULL, NULL),
	(645, 'p', 'SYS_ADMIN', 'api/v1/device/sub-remove', 'allow', NULL, NULL, NULL),
	(646, 'p', 'TENANT_ADMIN', 'api/v1/device/sub-remove', 'allow', NULL, NULL, NULL),
	(647, 'p', 'TENANT_USER', 'api/v1/device/sub-remove', 'allow', NULL, NULL, NULL),
	(648, 'p', 'SYS_ADMIN', 'api/v1/device/telemetry/latest', 'allow', NULL, NULL, NULL),
	(649, 'p', 'TENANT_ADMIN', 'api/v1/device/telemetry/latest', 'allow', NULL, NULL, NULL),
	(650, 'p', 'TENANT_USER', 'api/v1/device/telemetry/latest', 'allow', NULL, NULL, NULL),
	(651, 'p', 'SYS_ADMIN', 'api/v1/device/template', 'allow', NULL, NULL, NULL),
	(652, 'p', 'TENANT_ADMIN', 'api/v1/device/template', 'allow', NULL, NULL, NULL),
	(653, 'p', 'TENANT_USER', 'api/v1/device/template', 'allow', NULL, NULL, NULL),
	(654, 'p', 'SYS_ADMIN', 'api/v1/device/template/:id', 'allow', NULL, NULL, NULL),
	(655, 'p', 'TENANT_ADMIN', 'api/v1/device/template/:id', 'allow', NULL, NULL, NULL),
	(656, 'p', 'TENANT_USER', 'api/v1/device/template/:id', 'allow', NULL, NULL, NULL),
	(657, 'p', 'SYS_ADMIN', 'api/v1/device/template/chart', 'allow', NULL, NULL, NULL),
	(658, 'p', 'TENANT_ADMIN', 'api/v1/device/template/chart', 'allow', NULL, NULL, NULL),
	(659, 'p', 'TENANT_USER', 'api/v1/device/template/chart', 'allow', NULL, NULL, NULL),
	(660, 'p', 'SYS_ADMIN', 'api/v1/device/template/chart/select', 'allow', NULL, NULL, NULL),
	(661, 'p', 'TENANT_ADMIN', 'api/v1/device/template/chart/select', 'allow', NULL, NULL, NULL),
	(662, 'p', 'TENANT_USER', 'api/v1/device/template/chart/select', 'allow', NULL, NULL, NULL),
	(663, 'p', 'SYS_ADMIN', 'api/v1/device/template/detail/:id', 'allow', NULL, NULL, NULL),
	(664, 'p', 'TENANT_ADMIN', 'api/v1/device/template/detail/:id', 'allow', NULL, NULL, NULL),
	(665, 'p', 'TENANT_USER', 'api/v1/device/template/detail/:id', 'allow', NULL, NULL, NULL),
	(666, 'p', 'SYS_ADMIN', 'api/v1/device/template/market/detail/:market_id', 'allow', NULL, NULL, NULL),
	(667, 'p', 'TENANT_ADMIN', 'api/v1/device/template/market/detail/:market_id', 'allow', NULL, NULL, NULL),
	(668, 'p', 'TENANT_USER', 'api/v1/device/template/market/detail/:market_id', 'allow', NULL, NULL, NULL),
	(669, 'p', 'SYS_ADMIN', 'api/v1/device/template/market/install', 'allow', NULL, NULL, NULL),
	(670, 'p', 'TENANT_ADMIN', 'api/v1/device/template/market/install', 'allow', NULL, NULL, NULL),
	(671, 'p', 'TENANT_USER', 'api/v1/device/template/market/install', 'allow', NULL, NULL, NULL),
	(672, 'p', 'SYS_ADMIN', 'api/v1/device/template/market/list', 'allow', NULL, NULL, NULL),
	(673, 'p', 'TENANT_ADMIN', 'api/v1/device/template/market/list', 'allow', NULL, NULL, NULL),
	(674, 'p', 'TENANT_USER', 'api/v1/device/template/market/list', 'allow', NULL, NULL, NULL),
	(675, 'p', 'SYS_ADMIN', 'api/v1/device/template/market/login', 'allow', NULL, NULL, NULL),
	(676, 'p', 'TENANT_ADMIN', 'api/v1/device/template/market/login', 'allow', NULL, NULL, NULL),
	(677, 'p', 'TENANT_USER', 'api/v1/device/template/market/login', 'allow', NULL, NULL, NULL),
	(678, 'p', 'SYS_ADMIN', 'api/v1/device/template/market/publish', 'allow', NULL, NULL, NULL),
	(679, 'p', 'TENANT_ADMIN', 'api/v1/device/template/market/publish', 'allow', NULL, NULL, NULL),
	(680, 'p', 'TENANT_USER', 'api/v1/device/template/market/publish', 'allow', NULL, NULL, NULL),
	(681, 'p', 'SYS_ADMIN', 'api/v1/device/template/menu', 'allow', NULL, NULL, NULL),
	(682, 'p', 'TENANT_ADMIN', 'api/v1/device/template/menu', 'allow', NULL, NULL, NULL),
	(683, 'p', 'TENANT_USER', 'api/v1/device/template/menu', 'allow', NULL, NULL, NULL),
	(684, 'p', 'SYS_ADMIN', 'api/v1/device/template/selector', 'allow', NULL, NULL, NULL),
	(685, 'p', 'TENANT_ADMIN', 'api/v1/device/template/selector', 'allow', NULL, NULL, NULL),
	(686, 'p', 'TENANT_USER', 'api/v1/device/template/selector', 'allow', NULL, NULL, NULL),
	(687, 'p', 'SYS_ADMIN', 'api/v1/device/template/stats', 'allow', NULL, NULL, NULL),
	(688, 'p', 'TENANT_ADMIN', 'api/v1/device/template/stats', 'allow', NULL, NULL, NULL),
	(689, 'p', 'TENANT_USER', 'api/v1/device/template/stats', 'allow', NULL, NULL, NULL),
	(690, 'p', 'SYS_ADMIN', 'api/v1/device/tenant/list', 'allow', NULL, NULL, NULL),
	(691, 'p', 'TENANT_ADMIN', 'api/v1/device/tenant/list', 'allow', NULL, NULL, NULL),
	(692, 'p', 'TENANT_USER', 'api/v1/device/tenant/list', 'allow', NULL, NULL, NULL),
	(693, 'p', 'SYS_ADMIN', 'api/v1/device/topic-mappings', 'allow', NULL, NULL, NULL),
	(694, 'p', 'TENANT_ADMIN', 'api/v1/device/topic-mappings', 'allow', NULL, NULL, NULL),
	(695, 'p', 'TENANT_USER', 'api/v1/device/topic-mappings', 'allow', NULL, NULL, NULL),
	(696, 'p', 'SYS_ADMIN', 'api/v1/device/topic-mappings/:id', 'allow', NULL, NULL, NULL),
	(697, 'p', 'TENANT_ADMIN', 'api/v1/device/topic-mappings/:id', 'allow', NULL, NULL, NULL),
	(698, 'p', 'TENANT_USER', 'api/v1/device/topic-mappings/:id', 'allow', NULL, NULL, NULL),
	(699, 'p', 'SYS_ADMIN', 'api/v1/device/topic-mappings/dry-run', 'allow', NULL, NULL, NULL),
	(700, 'p', 'TENANT_ADMIN', 'api/v1/device/topic-mappings/dry-run', 'allow', NULL, NULL, NULL),
	(701, 'p', 'TENANT_USER', 'api/v1/device/topic-mappings/dry-run', 'allow', NULL, NULL, NULL),
	(702, 'p', 'SYS_ADMIN', 'api/v1/device/twin-drift', 'allow', NULL, NULL, NULL),
	(703, 'p', 'TENANT_ADMIN', 'api/v1/device/twin-drift', 'allow', NULL, NULL, NULL),
	(704, 'p', 'TENANT_USER', 'api/v1/device/twin-drift', 'allow', NULL, NULL, NULL),
	(705, 'p', 'SYS_ADMIN', 'api/v1/device/twin/:id', 'allow', NULL, NULL, NULL),
	(706, 'p', 'TENANT_ADMIN', 'api/v1/device/twin/:id', 'allow', NULL, NULL, NULL),
	(707, 'p', 'TENANT_USER', 'api/v1/device/twin/:id', 'allow', NULL, NULL, NULL),
	(708, 'p', 'SYS_ADMIN', 'api/v1/device/twin/:id/desired', 'allow', NULL, NULL, NULL),
	(709, 'p', 'TENANT_ADMIN', 'api/v1/device/twin/:id/desired', 'allow', NULL, NULL, NULL),
	(710, 'p', 'TENANT_USER', 'api/v1/device/twin/:id/desired', 'allow', NULL, NULL, NULL),
	(711, 'p', 'SYS_ADMIN', 'api/v1/device/update/config', 'allow', NULL, NULL, NULL),
	(712, 'p', 'TENANT_ADMIN', 'api/v1/device/update/config', 'allow', NULL, NULL, NULL),
	(713, 'p', 'TENANT_USER', 'api/v1/device/update/config', 'allow', NULL, NULL, NULL),
	(714, 'p', 'SYS_ADMIN', 'api/v1/device/update/voucher', 'allow', NULL, NULL, NULL),
	(715, 'p', 'TENANT_ADMIN', 'api/v1/device/update/voucher', 'allow', NULL, NULL, NULL),
	(716, 'p', 'TENANT_USER', 'api/v1/device/update/voucher', 'allow', NULL, NULL, NULL),
	(717, 'p', 'SYS_ADMIN', 'api/v1/device_config', 'allow', NULL, NULL, NULL),
	(718, 'p', 'TENANT_ADMIN', 'api/v1/device_config', 'allow', NULL, NULL, NULL),
	(719, 'p', 'TENANT_USER', 'api/v1/device_config', 'allow', NULL, NULL, NULL),
	(720, 'p', 'SYS_ADMIN', 'api/v1/device_config/:id', 'allow', NULL, NULL, NULL),
	(721, 'p', 'TENANT_ADMIN', 'api/v1/device_config/:id', 'allow', NULL, NULL, NULL),
	(722, 'p', 'TENANT_USER', 'api/v1/device_config/:id', 'allow', NULL, NULL, NULL),
	(723, 'p', 'SYS_ADMIN', 'api/v1/device_config/batch', 'allow', NULL, NULL, NULL),
	(724, 'p', 'TENANT_ADMIN', 'api/v1/device_config/batch', 'allow', NULL, NULL, NULL),
	(725, 'p', 'TENANT_USER', 'api/v1/device_config/batch', 'allow', NULL, NULL, NULL),
	(726, 'p', 'SYS_ADMIN', 'api/v1/device_config/connect', 'allow', NULL, NULL, NULL),
	(727, 'p', 'TENANT_ADMIN', 'api/v1/device_config/connect', 'allow', NULL, NULL, NULL),
	(728, 'p', 'TENANT_USER', 'api/v1/device_config/connect', 'allow', NULL, NULL, NULL),
	(729, 'p', 'SYS_ADMIN', 'api/v1/device_config/menu', 'allow', NULL, NULL, NULL),
	(730, 'p', 'TENANT_ADMIN', 'api/v1/device_config/menu', 'allow', NULL, NULL, NULL),
	(731, 'p', 'TENANT_USER', 'api/v1/device_config/menu', 'allow', NULL, NULL, NULL),
	(732, 'p', 'SYS_ADMIN', 'api/v1/device_config/metrics/condition/menu', 'allow', NULL, NULL, NULL),
	(733, 'p', 'TENANT_ADMIN', 'api/v1/device_config/metrics/condition/menu', 'allow', NULL, NULL, NULL),
	(734, 'p', 'TENANT_USER', 'api/v1/device_config/metrics/condition/menu', 'allow', NULL, NULL, NULL),
	(735, 'p', 'SYS_ADMIN', 'api/v1/device_config/metrics/menu', 'allow', NULL, NULL, NULL),
	(736, 'p', 'TENANT_ADMIN', 'api/v1/device_config/metrics/menu', 'allow', NULL, NULL, NULL),
	(737, 'p', 'TENANT_USER', 'api/v1/device_config/metrics/menu', 'allow', NULL, NULL, NULL),
	(738, 'p', 'SYS_ADMIN', 'api/v1/device_config/voucher_type', 'allow', NULL, NULL, NULL),
	(739, 'p', 'TENANT_ADMIN', 'api/v1/device_config/voucher_type', 'allow', NULL, NULL, NULL),
	(740, 'p', 'TENANT_USER', 'api/v1/device_config/voucher_type', 'allow', NULL, NULL, NULL),
	(741, 'p', 'SYS_ADMIN', 'api/v1/dict', 'allow', NULL, NULL, NULL),
	(744, 'p', 'SYS_ADMIN', 'api/v1/dict/column', 'allow', NULL, NULL, NULL),
	(747, 'p', 'SYS_ADMIN', 'api/v1/dict/column/:id', 'allow', NULL, NULL, NULL),
	(750, 'p', 'SYS_ADMIN', 'api/v1/dict/enum', 'allow', NULL, NULL, NULL),
	(751, 'p', 'TENANT_ADMIN', 'api/v1/dict/enum', 'allow', NULL, NULL, NULL),
	(752, 'p', 'TENANT_USER', 'api/v1/dict/enum', 'allow', NULL, NULL, NULL),
	(753, 'p', 'SYS_ADMIN', 'api/v1/dict/language', 'allow', NULL, NULL, NULL),
	(756, 'p', 'SYS_ADMIN', 'api/v1/dict/language/:id', 'allow', NULL, NULL, NULL),
	(759, 'p', 'SYS_ADMIN', 'api/v1/dict/protocol/service', 'allow', NULL, NULL, NULL),
	(760, 'p', 'TENANT_ADMIN', 'api/v1/dict/protocol/service', 'allow', NULL, NULL, NULL),
	(761, 'p', 'TENANT_USER', 'api/v1/dict/protocol/service', 'allow', NULL, NULL, NULL),
	(762, 'p', 'SYS_ADMIN', 'api/v1/entity_versions', 'allow', NULL, NULL, NULL),
	(763, 'p', 'TENANT_ADMIN', 'api/v1/entity_versions', 'allow', NULL, NULL, NULL),
	(764, 'p', 'TENANT_USER', 'api/v1/entity_versions', 'allow', NULL, NULL, NULL),
	(765, 'p', 'SYS_ADMIN', 'api/v1/entity_versions/:id', 'allow', NULL, NULL, NULL),
	(766, 'p', 'TENANT_ADMIN', 'api/v1/entity_versions/:id', 'allow', NULL, NULL, NULL),
	(767, 'p', 'TENANT_USER', 'api/v1/entity_versions/:id', 'allow', NULL, NULL, NULL),
	(768, 'p', 'SYS_ADMIN', 'api/v1/entity_versions/:id/restore', 'allow', NULL, NULL, NULL),
	(769, 'p', 'TENANT_ADMIN', 'api/v1/entity_versions/:id/restore', 'allow', NULL, NULL, NULL),
	(770, 'p', 'TENANT_USER', 'api/v1/entity_versions/:id/restore', 'allow', NULL, NULL, NULL),
	(771, 'p', 'SYS_ADMIN', 'api/v1/event/datas', 'allow', NULL, NULL, NULL),
	(772, 'p', 'TENANT_ADMIN', 'api/v1/event/datas', 'allow', NULL, NULL, NULL),
	(773, 'p', 'TENANT_USER', 'api/v1/event/datas', 'allow', NULL, NULL, NULL),
	(774, 'p', 'SYS_ADMIN', 'api/v1/events', 'allow', NULL, NULL, NULL),
	(775, 'p', 'TENANT_ADMIN', 'api/v1/events', 'allow', NULL, NULL, NULL),
	(776, 'p', 'TENANT_USER', 'api/v1/events', 'allow', NULL, NULL, NULL),
	(777, 'p', 'SYS_ADMIN', 'api/v1/expected/data', 'allow', NULL, NULL, NULL),
	(778, 'p', 'TENANT_ADMIN', 'api/v1/expected/data', 'allow', NULL, NULL, NULL),
	(779, 'p', 'TENANT_USER', 'api/v1/expected/data', 'allow', NULL, NULL, NULL),
	(780, 'p', 'SYS_ADMIN', 'api/v1/expected/data/:id', 'allow', NULL, NULL, NULL),
	(781, 'p', 'TENANT_ADMIN', 'api/v1/expected/data/:id', 'allow', NULL, NULL, NULL),
	(782, 'p', 'TENANT_USER', 'api/v1/expected/data/:id', 'allow', NULL, NULL, NULL),
	(783, 'p', 'SYS_ADMIN', 'api/v1/expected/data/list', 'allow', NULL, NULL, NULL),
	(784, 'p', 'TENANT_ADMIN', 'api/v1/expected/data/list', 'allow', NULL, NULL, NULL),
	(785, 'p', 'TENANT_USER', 'api/v1/expected/data/list', 'allow', NULL, NULL, NULL),
	(786, 'p', 'SYS_ADMIN', 'api/v1/file/up', 'allow', NULL, NULL, NULL),
	(787, 'p', 'TENANT_ADMIN', 'api/v1/file/up', 'allow', NULL, NULL, NULL),
	(788, 'p', 'TENANT_USER', 'api/v1/file/up', 'allow', NULL, NULL, NULL),
	(789, 'p', 'SYS_ADMIN', 'api/v1/logo', 'allow', NULL, NULL, NULL),
	(790, 'p', 'TENANT_ADMIN', 'api/v1/logo', 'allow', NULL, NULL, NULL),
	(791, 'p', 'TENANT_USER', 'api/v1/logo', 'allow', NULL, NULL, NULL),
	(792, 'p', 'SYS_ADMIN', 'api/v1/message_push', 'allow', NULL, NULL, NULL),
	(793, 'p', 'TENANT_ADMIN', 'api/v1/message_push', 'allow', NULL, NULL, NULL),
	(794, 'p', 'TENANT_USER', 'api/v1/message_push', 'allow', NULL, NULL, NULL),
	(795, 'p', 'SYS_ADMIN', 'api/v1/message_push/config', 'allow', NULL, NULL, NULL),
	(796, 'p', 'TENANT_ADMIN', 'api/v1/message_push/config', 'allow', NULL, NULL, NULL),
	(797, 'p', 'TENANT_USER', 'api/v1/message_push/config', 'allow', NULL, NULL, NULL),
	(798, 'p', 'SYS_ADMIN', 'api/v1/message_push/logout', 'allow', NULL, NULL, NULL),
	(799, 'p', 'TENANT_ADMIN', 'api/v1/message_push/logout', 'allow', NULL, NULL, NULL),
	(800, 'p', 'TENANT_USER', 'api/v1/message_push/logout', 'allow', NULL, NULL, NULL),
	(801, 'p', 'SYS_ADMIN', 'api/v1/notification/e-mail/templates', 'allow', NULL, NULL, NULL),
	(802, 'p', 'TENANT_ADMIN', 'api/v1/notification/e-mail/templates', 'allow', NULL, NULL, NULL),
	(803, 'p', 'TENANT_USER', 'api/v1/notification/e-mail/templates', 'allow', NULL, NULL, NULL),
	(804, 'p', 'SYS_ADMIN', 'api/v1/notification/e-mail/templates/:id', 'allow', NULL, NULL, NULL),
	(805, 'p', 'TENANT_ADMIN', 'api/v1/notification/e-mail/templates/:id', 'allow', NULL, NULL, NULL),
	(806, 'p', 'TENANT_USER', 'api/v1/notification/e-mail/templates/:id', 'allow', NULL, NULL, NULL),
	(807, 'p', 'SYS_ADMIN', 'api/v1/notification/e-mail/templates/:id/default', 'allow', NULL, NULL, NULL),
	(808, 'p', 'TENANT_ADMIN', 'api/v1/notification/e-mail/templates/:id/default', 'allow', NULL, NULL, NULL),
	(809, 'p', 'TENANT_USER', 'api/v1/notification/e-mail/templates/:id/default', 'allow', NULL, NULL, NULL),
	(810, 'p', 'SYS_ADMIN', 'api/v1/notification/e-mail/templates/preview', 'allow', NULL, NULL, NULL),
	(811, 'p', 'TENANT_ADMIN', 'api/v1/notification/e-mail/templates/preview', 'allow', NULL, NULL, NULL),
	(812, 'p', 'TENANT_USER', 'api/v1/notification/e-mail/templates/preview', 'allow', NULL, NULL, NULL),
	(813, 'p', 'SYS_ADMIN', 'api/v1/notification/services/config', 'allow', NULL, NULL, NULL),
	(814, 'p', 'TENANT_ADMIN', 'api/v1/notification/services/config', 'allow', NULL, NULL, NULL),
	(815, 'p', 'TENANT_USER', 'api/v1/notification/services/config', 'allow', NULL, NULL, NULL),
	(816, 'p', 'SYS_ADMIN', 'api/v1/notification/services/config/:type', 'allow', NULL, NULL, NULL);
INSERT INTO public.casbin_rule (id, ptype, v0, v1, v2, v3, v4, v5) VALUES
	(817, 'p', 'TENANT_ADMIN', 'api/v1/notification/services/config/:type', 'allow', NULL, NULL, NULL),
	(818, 'p', 'TENANT_USER', 'api/v1/notification/services/config/:type', 'allow', NULL, NULL, NULL),
	(819, 'p', 'SYS_ADMIN', 'api/v1/notification/services/config/e-mail/test', 'allow', NULL, NULL, NULL),
	(820, 'p', 'TENANT_ADMIN', 'api/v1/notification/services/config/e-mail/test', 'allow', NULL, NULL, NULL),
	(821, 'p', 'TENANT_USER', 'api/v1/notification/services/config/e-mail/test', 'allow', NULL, NULL, NULL),
	(822, 'p', 'SYS_ADMIN', 'api/v1/notification_group', 'allow', NULL, NULL, NULL),
	(823, 'p', 'TENANT_ADMIN', 'api/v1/notification_group', 'allow', NULL, NULL, NULL),
	(824, 'p', 'TENANT_USER', 'api/v1/notification_group', 'allow', NULL, NULL, NULL),
	(825, 'p', 'SYS_ADMIN', 'api/v1/notification_group/:id', 'allow', NULL, NULL, NULL),
	(826, 'p', 'TENANT_ADMIN', 'api/v1/notification_group/:id', 'allow', NULL, NULL, NULL),
	(827, 'p', 'TENANT_USER', 'api/v1/notification_group/:id', 'allow', NULL, NULL, NULL),
	(828, 'p', 'SYS_ADMIN', 'api/v1/notification_group/list', 'allow', NULL, NULL, NULL),
	(829, 'p', 'TENANT_ADMIN', 'api/v1/notification_group/list', 'allow', NULL, NULL, NULL),
	(830, 'p', 'TENANT_USER', 'api/v1/notification_group/list', 'allow', NULL, NULL, NULL),
	(831, 'p', 'SYS_ADMIN', 'api/v1/notification_history/list', 'allow', NULL, NULL, NULL),
	(832, 'p', 'TENANT_ADMIN', 'api/v1/notification_history/list', 'allow', NULL, NULL, NULL),
	(833, 'p', 'TENANT_USER', 'api/v1/notification_history/list', 'allow', NULL, NULL, NULL),
	(834, 'p', 'SYS_ADMIN', 'api/v1/oidc/provider', 'allow', NULL, NULL, NULL),
	(835, 'p', 'TENANT_ADMIN', 'api/v1/oidc/provider', 'allow', NULL, NULL, NULL),
	(837, 'p', 'SYS_ADMIN', 'api/v1/oidc/provider/:id', 'allow', NULL, NULL, NULL),
	(838, 'p', 'TENANT_ADMIN', 'api/v1/oidc/provider/:id', 'allow', NULL, NULL, NULL),
	(840, 'p', 'SYS_ADMIN', 'api/v1/oidc/provider/list', 'allow', NULL, NULL, NULL),
	(841, 'p', 'TENANT_ADMIN', 'api/v1/oidc/provider/list', 'allow', NULL, NULL, NULL),
	(843, 'p', 'SYS_ADMIN', 'api/v1/open/keys', 'allow', NULL, NULL, NULL),
	(844, 'p', 'TENANT_ADMIN', 'api/v1/open/keys', 'allow', NULL, NULL, NULL),
	(846, 'p', 'SYS_ADMIN', 'api/v1/open/keys/:id', 'allow', NULL, NULL, NULL),
	(847, 'p', 'TENANT_ADMIN', 'api/v1/open/keys/:id', 'allow', NULL, NULL, NULL),
	(849, 'p', 'SYS_ADMIN', 'api/v1/operation_logs', 'allow', NULL, NULL, NULL),
	(850, 'p', 'TENANT_ADMIN', 'api/v1/operation_logs', 'allow', NULL, NULL, NULL),
	(852, 'p', 'SYS_ADMIN', 'api/v1/ota/package', 'allow', NULL, NULL, NULL),
	(853, 'p', 'TENANT_ADMIN', 'api/v1/ota/package', 'allow', NULL, NULL, NULL),
	(854, 'p', 'TENANT_USER', 'api/v1/ota/package', 'allow', NULL, NULL, NULL),
	(855, 'p', 'SYS_ADMIN', 'api/v1/ota/package/:id', 'allow', NULL, NULL, NULL),
	(856, 'p', 'TENANT_ADMIN', 'api/v1/ota/package/:id', 'allow', NULL, NULL, NULL),
	(857, 'p', 'TENANT_USER', 'api/v1/ota/package/:id', 'allow', NULL, NULL, NULL),
	(858, 'p', 'SYS_ADMIN', 'api/v1/ota/task', 'allow', NULL, NULL, NULL),
	(859, 'p', 'TENANT_ADMIN', 'api/v1/ota/task', 'allow', NULL, NULL, NULL),
	(860, 'p', 'TENANT_USER', 'api/v1/ota/task', 'allow', NULL, NULL, NULL),
	(861, 'p', 'SYS_ADMIN', 'api/v1/ota/task/:id', 'allow', NULL, NULL, NULL),
	(862, 'p', 'TENANT_ADMIN', 'api/v1/ota/task/:id', 'allow', NULL, NULL, NULL),
	(863, 'p', 'TENANT_USER', 'api/v1/ota/task/:id', 'allow', NULL, NULL, NULL),
	(864, 'p', 'SYS_ADMIN', 'api/v1/ota/task/:id/governance-preview', 'allow', NULL, NULL, NULL),
	(865, 'p', 'TENANT_ADMIN', 'api/v1/ota/task/:id/governance-preview', 'allow', NULL, NULL, NULL),
	(866, 'p', 'TENANT_USER', 'api/v1/ota/task/:id/governance-preview', 'allow', NULL, NULL, NULL),
	(867, 'p', 'SYS_ADMIN', 'api/v1/ota/task/:id/support-bundle', 'allow', NULL, NULL, NULL),
	(868, 'p', 'TENANT_ADMIN', 'api/v1/ota/task/:id/support-bundle', 'allow', NULL, NULL, NULL),
	(869, 'p', 'TENANT_USER', 'api/v1/ota/task/:id/support-bundle', 'allow', NULL, NULL, NULL),
	(870, 'p', 'SYS_ADMIN', 'api/v1/ota/task/detail', 'allow', NULL, NULL, NULL),
	(871, 'p', 'TENANT_ADMIN', 'api/v1/ota/task/detail', 'allow', NULL, NULL, NULL),
	(872, 'p', 'TENANT_USER', 'api/v1/ota/task/detail', 'allow', NULL, NULL, NULL),
	(873, 'p', 'SYS_ADMIN', 'api/v1/ota/task/preview', 'allow', NULL, NULL, NULL),
	(874, 'p', 'TENANT_ADMIN', 'api/v1/ota/task/preview', 'allow', NULL, NULL, NULL),
	(875, 'p', 'TENANT_USER', 'api/v1/ota/task/preview', 'allow', NULL, NULL, NULL),
	(876, 'p', 'SYS_ADMIN', 'api/v1/payload-schema', 'allow', NULL, NULL, NULL),
	(877, 'p', 'TENANT_ADMIN', 'api/v1/payload-schema', 'allow', NULL, NULL, NULL),
	(878, 'p', 'TENANT_USER', 'api/v1/payload-schema', 'allow', NULL, NULL, NULL),
	(879, 'p', 'SYS_ADMIN', 'api/v1/payload-schema/:schema_id', 'allow', NULL, NULL, NULL),
	(880, 'p', 'TENANT_ADMIN', 'api/v1/payload-schema/:schema_id', 'allow', NULL, NULL, NULL),
	(881, 'p', 'TENANT_USER', 'api/v1/payload-schema/:schema_id', 'allow', NULL, NULL, NULL),
	(882, 'p', 'SYS_ADMIN', 'api/v1/payload-schema/validate', 'allow', NULL, NULL, NULL),
	(883, 'p', 'TENANT_ADMIN', 'api/v1/payload-schema/validate', 'allow', NULL, NULL, NULL),
	(884, 'p', 'TENANT_USER', 'api/v1/payload-schema/validate', 'allow', NULL, NULL, NULL),
	(885, 'p', 'SYS_ADMIN', 'api/v1/product', 'allow', NULL, NULL, NULL),
	(886, 'p', 'TENANT_ADMIN', 'api/v1/product', 'allow', NULL, NULL, NULL),
	(887, 'p', 'TENANT_USER', 'api/v1/product', 'allow', NULL, NULL, NULL),
	(888, 'p', 'SYS_ADMIN', 'api/v1/protocol_plugin/config_form', 'allow', NULL, NULL, NULL),
	(889, 'p', 'TENANT_ADMIN', 'api/v1/protocol_plugin/config_form', 'allow', NULL, NULL, NULL),
	(890, 'p', 'TENANT_USER', 'api/v1/protocol_plugin/config_form', 'allow', NULL, NULL, NULL),
	(891, 'p', 'SYS_ADMIN', 'api/v1/rdi/devices/:device_id/commands', 'allow', NULL, NULL, NULL),
	(892, 'p', 'TENANT_ADMIN', 'api/v1/rdi/devices/:device_id/commands', 'allow', NULL, NULL, NULL),
	(893, 'p', 'TENANT_USER', 'api/v1/rdi/devices/:device_id/commands', 'allow', NULL, NULL, NULL),
	(894, 'p', 'SYS_ADMIN', 'api/v1/rdi/devices/:device_id/config', 'allow', NULL, NULL, NULL),
	(895, 'p', 'TENANT_ADMIN', 'api/v1/rdi/devices/:device_id/config', 'allow', NULL, NULL, NULL),
	(896, 'p', 'TENANT_USER', 'api/v1/rdi/devices/:device_id/config', 'allow', NULL, NULL, NULL),
	(897, 'p', 'SYS_ADMIN', 'api/v1/rdi/devices/:device_id/history', 'allow', NULL, NULL, NULL),
	(898, 'p', 'TENANT_ADMIN', 'api/v1/rdi/devices/:device_id/history', 'allow', NULL, NULL, NULL),
	(899, 'p', 'TENANT_USER', 'api/v1/rdi/devices/:device_id/history', 'allow', NULL, NULL, NULL),
	(900, 'p', 'SYS_ADMIN', 'api/v1/rdi/devices/:device_id/latest-firmware', 'allow', NULL, NULL, NULL),
	(901, 'p', 'TENANT_ADMIN', 'api/v1/rdi/devices/:device_id/latest-firmware', 'allow', NULL, NULL, NULL),
	(902, 'p', 'TENANT_USER', 'api/v1/rdi/devices/:device_id/latest-firmware', 'allow', NULL, NULL, NULL),
	(903, 'p', 'SYS_ADMIN', 'api/v1/rdi/devices/:device_id/share-recipients/:user_id', 'allow', NULL, NULL, NULL),
	(904, 'p', 'TENANT_ADMIN', 'api/v1/rdi/devices/:device_id/share-recipients/:user_id', 'allow', NULL, NULL, NULL),
	(905, 'p', 'TENANT_USER', 'api/v1/rdi/devices/:device_id/share-recipients/:user_id', 'allow', NULL, NULL, NULL),
	(906, 'p', 'SYS_ADMIN', 'api/v1/rdi/devices/:device_id/share-token', 'allow', NULL, NULL, NULL),
	(907, 'p', 'TENANT_ADMIN', 'api/v1/rdi/devices/:device_id/share-token', 'allow', NULL, NULL, NULL),
	(908, 'p', 'TENANT_USER', 'api/v1/rdi/devices/:device_id/share-token', 'allow', NULL, NULL, NULL),
	(909, 'p', 'SYS_ADMIN', 'api/v1/rdi/devices/:device_id/share-tokens/:token', 'allow', NULL, NULL, NULL),
	(910, 'p', 'TENANT_ADMIN', 'api/v1/rdi/devices/:device_id/share-tokens/:token', 'allow', NULL, NULL, NULL),
	(911, 'p', 'TENANT_USER', 'api/v1/rdi/devices/:device_id/share-tokens/:token', 'allow', NULL, NULL, NULL),
	(912, 'p', 'SYS_ADMIN', 'api/v1/rdi/devices/activate', 'allow', NULL, NULL, NULL),
	(913, 'p', 'TENANT_ADMIN', 'api/v1/rdi/devices/activate', 'allow', NULL, NULL, NULL),
	(914, 'p', 'TENANT_USER', 'api/v1/rdi/devices/activate', 'allow', NULL, NULL, NULL),
	(915, 'p', 'SYS_ADMIN', 'api/v1/rdi/share-tokens/:token/accept', 'allow', NULL, NULL, NULL),
	(916, 'p', 'TENANT_ADMIN', 'api/v1/rdi/share-tokens/:token/accept', 'allow', NULL, NULL, NULL),
	(917, 'p', 'TENANT_USER', 'api/v1/rdi/share-tokens/:token/accept', 'allow', NULL, NULL, NULL),
	(918, 'p', 'SYS_ADMIN', 'api/v1/rdi/shared-with-me/devices', 'allow', NULL, NULL, NULL),
	(919, 'p', 'TENANT_ADMIN', 'api/v1/rdi/shared-with-me/devices', 'allow', NULL, NULL, NULL),
	(920, 'p', 'TENANT_USER', 'api/v1/rdi/shared-with-me/devices', 'allow', NULL, NULL, NULL),
	(921, 'p', 'SYS_ADMIN', 'api/v1/rdi/thing-model', 'allow', NULL, NULL, NULL),
	(922, 'p', 'TENANT_ADMIN', 'api/v1/rdi/thing-model', 'allow', NULL, NULL, NULL),
	(923, 'p', 'TENANT_USER', 'api/v1/rdi/thing-model', 'allow', NULL, NULL, NULL),
	(924, 'p', 'SYS_ADMIN', 'api/v1/role', 'allow', NULL, NULL, NULL),
	(925, 'p', 'TENANT_ADMIN', 'api/v1/role', 'allow', NULL, NULL, NULL),
	(927, 'p', 'SYS_ADMIN', 'api/v1/role/:id', 'allow', NULL, NULL, NULL),
	(928, 'p', 'TENANT_ADMIN', 'api/v1/role/:id', 'allow', NULL, NULL, NULL),
	(930, 'p', 'SYS_ADMIN', 'api/v1/rule-chains', 'allow', NULL, NULL, NULL),
	(931, 'p', 'TENANT_ADMIN', 'api/v1/rule-chains', 'allow', NULL, NULL, NULL),
	(932, 'p', 'TENANT_USER', 'api/v1/rule-chains', 'allow', NULL, NULL, NULL),
	(933, 'p', 'SYS_ADMIN', 'api/v1/rule-chains/:id', 'allow', NULL, NULL, NULL),
	(934, 'p', 'TENANT_ADMIN', 'api/v1/rule-chains/:id', 'allow', NULL, NULL, NULL),
	(935, 'p', 'TENANT_USER', 'api/v1/rule-chains/:id', 'allow', NULL, NULL, NULL),
	(936, 'p', 'SYS_ADMIN', 'api/v1/rule-chains/list', 'allow', NULL, NULL, NULL),
	(937, 'p', 'TENANT_ADMIN', 'api/v1/rule-chains/list', 'allow', NULL, NULL, NULL),
	(938, 'p', 'TENANT_USER', 'api/v1/rule-chains/list', 'allow', NULL, NULL, NULL),
	(939, 'p', 'SYS_ADMIN', 'api/v1/scene', 'allow', NULL, NULL, NULL),
	(940, 'p', 'TENANT_ADMIN', 'api/v1/scene', 'allow', NULL, NULL, NULL),
	(941, 'p', 'TENANT_USER', 'api/v1/scene', 'allow', NULL, NULL, NULL),
	(942, 'p', 'SYS_ADMIN', 'api/v1/scene/:id', 'allow', NULL, NULL, NULL),
	(943, 'p', 'TENANT_ADMIN', 'api/v1/scene/:id', 'allow', NULL, NULL, NULL),
	(944, 'p', 'TENANT_USER', 'api/v1/scene/:id', 'allow', NULL, NULL, NULL),
	(945, 'p', 'SYS_ADMIN', 'api/v1/scene/active/:id', 'allow', NULL, NULL, NULL),
	(946, 'p', 'TENANT_ADMIN', 'api/v1/scene/active/:id', 'allow', NULL, NULL, NULL),
	(947, 'p', 'TENANT_USER', 'api/v1/scene/active/:id', 'allow', NULL, NULL, NULL),
	(948, 'p', 'SYS_ADMIN', 'api/v1/scene/detail/:id', 'allow', NULL, NULL, NULL),
	(949, 'p', 'TENANT_ADMIN', 'api/v1/scene/detail/:id', 'allow', NULL, NULL, NULL),
	(950, 'p', 'TENANT_USER', 'api/v1/scene/detail/:id', 'allow', NULL, NULL, NULL),
	(951, 'p', 'SYS_ADMIN', 'api/v1/scene/dry-run', 'allow', NULL, NULL, NULL),
	(952, 'p', 'TENANT_ADMIN', 'api/v1/scene/dry-run', 'allow', NULL, NULL, NULL),
	(953, 'p', 'TENANT_USER', 'api/v1/scene/dry-run', 'allow', NULL, NULL, NULL),
	(954, 'p', 'SYS_ADMIN', 'api/v1/scene/log', 'allow', NULL, NULL, NULL),
	(955, 'p', 'TENANT_ADMIN', 'api/v1/scene/log', 'allow', NULL, NULL, NULL),
	(956, 'p', 'TENANT_USER', 'api/v1/scene/log', 'allow', NULL, NULL, NULL),
	(957, 'p', 'SYS_ADMIN', 'api/v1/scene_automations', 'allow', NULL, NULL, NULL),
	(958, 'p', 'TENANT_ADMIN', 'api/v1/scene_automations', 'allow', NULL, NULL, NULL),
	(959, 'p', 'TENANT_USER', 'api/v1/scene_automations', 'allow', NULL, NULL, NULL),
	(960, 'p', 'SYS_ADMIN', 'api/v1/scene_automations/:id', 'allow', NULL, NULL, NULL),
	(961, 'p', 'TENANT_ADMIN', 'api/v1/scene_automations/:id', 'allow', NULL, NULL, NULL),
	(962, 'p', 'TENANT_USER', 'api/v1/scene_automations/:id', 'allow', NULL, NULL, NULL),
	(963, 'p', 'SYS_ADMIN', 'api/v1/scene_automations/alarm', 'allow', NULL, NULL, NULL),
	(964, 'p', 'TENANT_ADMIN', 'api/v1/scene_automations/alarm', 'allow', NULL, NULL, NULL),
	(965, 'p', 'TENANT_USER', 'api/v1/scene_automations/alarm', 'allow', NULL, NULL, NULL),
	(966, 'p', 'SYS_ADMIN', 'api/v1/scene_automations/detail/:id', 'allow', NULL, NULL, NULL),
	(967, 'p', 'TENANT_ADMIN', 'api/v1/scene_automations/detail/:id', 'allow', NULL, NULL, NULL),
	(968, 'p', 'TENANT_USER', 'api/v1/scene_automations/detail/:id', 'allow', NULL, NULL, NULL),
	(969, 'p', 'SYS_ADMIN', 'api/v1/scene_automations/dry-run', 'allow', NULL, NULL, NULL),
	(970, 'p', 'TENANT_ADMIN', 'api/v1/scene_automations/dry-run', 'allow', NULL, NULL, NULL),
	(971, 'p', 'TENANT_USER', 'api/v1/scene_automations/dry-run', 'allow', NULL, NULL, NULL),
	(972, 'p', 'SYS_ADMIN', 'api/v1/scene_automations/list', 'allow', NULL, NULL, NULL),
	(973, 'p', 'TENANT_ADMIN', 'api/v1/scene_automations/list', 'allow', NULL, NULL, NULL),
	(974, 'p', 'TENANT_USER', 'api/v1/scene_automations/list', 'allow', NULL, NULL, NULL),
	(975, 'p', 'SYS_ADMIN', 'api/v1/scene_automations/log', 'allow', NULL, NULL, NULL),
	(976, 'p', 'TENANT_ADMIN', 'api/v1/scene_automations/log', 'allow', NULL, NULL, NULL),
	(977, 'p', 'TENANT_USER', 'api/v1/scene_automations/log', 'allow', NULL, NULL, NULL),
	(978, 'p', 'SYS_ADMIN', 'api/v1/scene_automations/switch/:id', 'allow', NULL, NULL, NULL),
	(979, 'p', 'TENANT_ADMIN', 'api/v1/scene_automations/switch/:id', 'allow', NULL, NULL, NULL),
	(980, 'p', 'TENANT_USER', 'api/v1/scene_automations/switch/:id', 'allow', NULL, NULL, NULL),
	(981, 'p', 'SYS_ADMIN', 'api/v1/service', 'allow', NULL, NULL, NULL),
	(984, 'p', 'SYS_ADMIN', 'api/v1/service/:id', 'allow', NULL, NULL, NULL),
	(987, 'p', 'SYS_ADMIN', 'api/v1/service/access', 'allow', NULL, NULL, NULL),
	(990, 'p', 'SYS_ADMIN', 'api/v1/service/access/:id', 'allow', NULL, NULL, NULL),
	(993, 'p', 'SYS_ADMIN', 'api/v1/service/access/device/list', 'allow', NULL, NULL, NULL),
	(994, 'p', 'TENANT_ADMIN', 'api/v1/service/access/device/list', 'allow', NULL, NULL, NULL),
	(995, 'p', 'TENANT_USER', 'api/v1/service/access/device/list', 'allow', NULL, NULL, NULL),
	(996, 'p', 'SYS_ADMIN', 'api/v1/service/access/list', 'allow', NULL, NULL, NULL),
	(997, 'p', 'TENANT_ADMIN', 'api/v1/service/access/list', 'allow', NULL, NULL, NULL),
	(998, 'p', 'TENANT_USER', 'api/v1/service/access/list', 'allow', NULL, NULL, NULL),
	(999, 'p', 'SYS_ADMIN', 'api/v1/service/access/voucher/form', 'allow', NULL, NULL, NULL),
	(1000, 'p', 'TENANT_ADMIN', 'api/v1/service/access/voucher/form', 'allow', NULL, NULL, NULL),
	(1001, 'p', 'TENANT_USER', 'api/v1/service/access/voucher/form', 'allow', NULL, NULL, NULL),
	(1002, 'p', 'SYS_ADMIN', 'api/v1/service/detail/:id', 'allow', NULL, NULL, NULL),
	(1003, 'p', 'TENANT_ADMIN', 'api/v1/service/detail/:id', 'allow', NULL, NULL, NULL),
	(1004, 'p', 'TENANT_USER', 'api/v1/service/detail/:id', 'allow', NULL, NULL, NULL),
	(1005, 'p', 'SYS_ADMIN', 'api/v1/service/list', 'allow', NULL, NULL, NULL),
	(1006, 'p', 'TENANT_ADMIN', 'api/v1/service/list', 'allow', NULL, NULL, NULL),
	(1007, 'p', 'TENANT_USER', 'api/v1/service/list', 'allow', NULL, NULL, NULL),
	(1008, 'p', 'SYS_ADMIN', 'api/v1/service/plugin/info', 'allow', NULL, NULL, NULL),
	(1009, 'p', 'TENANT_ADMIN', 'api/v1/service/plugin/info', 'allow', NULL, NULL, NULL),
	(1010, 'p', 'TENANT_USER', 'api/v1/service/plugin/info', 'allow', NULL, NULL, NULL),
	(1011, 'p', 'SYS_ADMIN', 'api/v1/service/plugin/select', 'allow', NULL, NULL, NULL),
	(1012, 'p', 'TENANT_ADMIN', 'api/v1/service/plugin/select', 'allow', NULL, NULL, NULL),
	(1013, 'p', 'TENANT_USER', 'api/v1/service/plugin/select', 'allow', NULL, NULL, NULL),
	(1014, 'p', 'SYS_ADMIN', 'api/v1/sys_function/:id', 'allow', NULL, NULL, NULL),
	(1017, 'p', 'SYS_ADMIN', 'api/v1/system/metrics/current', 'allow', NULL, NULL, NULL),
	(1020, 'p', 'SYS_ADMIN', 'api/v1/system/metrics/history', 'allow', NULL, NULL, NULL),
	(1023, 'p', 'SYS_ADMIN', 'api/v1/telemetry/datas', 'allow', NULL, NULL, NULL),
	(1024, 'p', 'TENANT_ADMIN', 'api/v1/telemetry/datas', 'allow', NULL, NULL, NULL),
	(1025, 'p', 'TENANT_USER', 'api/v1/telemetry/datas', 'allow', NULL, NULL, NULL),
	(1026, 'p', 'SYS_ADMIN', 'api/v1/telemetry/datas/current/:id', 'allow', NULL, NULL, NULL),
	(1027, 'p', 'TENANT_ADMIN', 'api/v1/telemetry/datas/current/:id', 'allow', NULL, NULL, NULL),
	(1028, 'p', 'TENANT_USER', 'api/v1/telemetry/datas/current/:id', 'allow', NULL, NULL, NULL),
	(1029, 'p', 'SYS_ADMIN', 'api/v1/telemetry/datas/current/detail/:id', 'allow', NULL, NULL, NULL),
	(1030, 'p', 'TENANT_ADMIN', 'api/v1/telemetry/datas/current/detail/:id', 'allow', NULL, NULL, NULL),
	(1031, 'p', 'TENANT_USER', 'api/v1/telemetry/datas/current/detail/:id', 'allow', NULL, NULL, NULL),
	(1032, 'p', 'SYS_ADMIN', 'api/v1/telemetry/datas/current/keys', 'allow', NULL, NULL, NULL),
	(1033, 'p', 'TENANT_ADMIN', 'api/v1/telemetry/datas/current/keys', 'allow', NULL, NULL, NULL),
	(1034, 'p', 'TENANT_USER', 'api/v1/telemetry/datas/current/keys', 'allow', NULL, NULL, NULL),
	(1035, 'p', 'SYS_ADMIN', 'api/v1/telemetry/datas/dead-letters', 'allow', NULL, NULL, NULL),
	(1036, 'p', 'TENANT_ADMIN', 'api/v1/telemetry/datas/dead-letters', 'allow', NULL, NULL, NULL),
	(1037, 'p', 'TENANT_USER', 'api/v1/telemetry/datas/dead-letters', 'allow', NULL, NULL, NULL),
	(1038, 'p', 'SYS_ADMIN', 'api/v1/telemetry/datas/dead-letters/:id/status', 'allow', NULL, NULL, NULL);
INSERT INTO public.casbin_rule (id, ptype, v0, v1, v2, v3, v4, v5) VALUES
	(1039, 'p', 'TENANT_ADMIN', 'api/v1/telemetry/datas/dead-letters/:id/status', 'allow', NULL, NULL, NULL),
	(1040, 'p', 'TENANT_USER', 'api/v1/telemetry/datas/dead-letters/:id/status', 'allow', NULL, NULL, NULL),
	(1041, 'p', 'SYS_ADMIN', 'api/v1/telemetry/datas/dead-letters/drain', 'allow', NULL, NULL, NULL),
	(1042, 'p', 'TENANT_ADMIN', 'api/v1/telemetry/datas/dead-letters/drain', 'allow', NULL, NULL, NULL),
	(1043, 'p', 'TENANT_USER', 'api/v1/telemetry/datas/dead-letters/drain', 'allow', NULL, NULL, NULL),
	(1044, 'p', 'SYS_ADMIN', 'api/v1/telemetry/datas/history', 'allow', NULL, NULL, NULL),
	(1045, 'p', 'TENANT_ADMIN', 'api/v1/telemetry/datas/history', 'allow', NULL, NULL, NULL),
	(1046, 'p', 'TENANT_USER', 'api/v1/telemetry/datas/history', 'allow', NULL, NULL, NULL),
	(1047, 'p', 'SYS_ADMIN', 'api/v1/telemetry/datas/history/page', 'allow', NULL, NULL, NULL),
	(1048, 'p', 'TENANT_ADMIN', 'api/v1/telemetry/datas/history/page', 'allow', NULL, NULL, NULL),
	(1049, 'p', 'TENANT_USER', 'api/v1/telemetry/datas/history/page', 'allow', NULL, NULL, NULL),
	(1050, 'p', 'SYS_ADMIN', 'api/v1/telemetry/datas/history/pagination', 'allow', NULL, NULL, NULL),
	(1051, 'p', 'TENANT_ADMIN', 'api/v1/telemetry/datas/history/pagination', 'allow', NULL, NULL, NULL),
	(1052, 'p', 'TENANT_USER', 'api/v1/telemetry/datas/history/pagination', 'allow', NULL, NULL, NULL),
	(1053, 'p', 'SYS_ADMIN', 'api/v1/telemetry/datas/msg/count', 'allow', NULL, NULL, NULL),
	(1054, 'p', 'TENANT_ADMIN', 'api/v1/telemetry/datas/msg/count', 'allow', NULL, NULL, NULL),
	(1055, 'p', 'TENANT_USER', 'api/v1/telemetry/datas/msg/count', 'allow', NULL, NULL, NULL),
	(1056, 'p', 'SYS_ADMIN', 'api/v1/telemetry/datas/pub', 'allow', NULL, NULL, NULL),
	(1057, 'p', 'TENANT_ADMIN', 'api/v1/telemetry/datas/pub', 'allow', NULL, NULL, NULL),
	(1058, 'p', 'TENANT_USER', 'api/v1/telemetry/datas/pub', 'allow', NULL, NULL, NULL),
	(1059, 'p', 'SYS_ADMIN', 'api/v1/telemetry/datas/set/logs', 'allow', NULL, NULL, NULL),
	(1060, 'p', 'TENANT_ADMIN', 'api/v1/telemetry/datas/set/logs', 'allow', NULL, NULL, NULL),
	(1061, 'p', 'TENANT_USER', 'api/v1/telemetry/datas/set/logs', 'allow', NULL, NULL, NULL),
	(1062, 'p', 'SYS_ADMIN', 'api/v1/telemetry/datas/simulation', 'allow', NULL, NULL, NULL),
	(1063, 'p', 'TENANT_ADMIN', 'api/v1/telemetry/datas/simulation', 'allow', NULL, NULL, NULL),
	(1064, 'p', 'TENANT_USER', 'api/v1/telemetry/datas/simulation', 'allow', NULL, NULL, NULL),
	(1065, 'p', 'SYS_ADMIN', 'api/v1/telemetry/datas/simulation/init', 'allow', NULL, NULL, NULL),
	(1066, 'p', 'TENANT_ADMIN', 'api/v1/telemetry/datas/simulation/init', 'allow', NULL, NULL, NULL),
	(1067, 'p', 'TENANT_USER', 'api/v1/telemetry/datas/simulation/init', 'allow', NULL, NULL, NULL),
	(1068, 'p', 'SYS_ADMIN', 'api/v1/telemetry/datas/simulation/send', 'allow', NULL, NULL, NULL),
	(1069, 'p', 'TENANT_ADMIN', 'api/v1/telemetry/datas/simulation/send', 'allow', NULL, NULL, NULL),
	(1070, 'p', 'TENANT_USER', 'api/v1/telemetry/datas/simulation/send', 'allow', NULL, NULL, NULL),
	(1071, 'p', 'SYS_ADMIN', 'api/v1/telemetry/datas/statistic', 'allow', NULL, NULL, NULL),
	(1072, 'p', 'TENANT_ADMIN', 'api/v1/telemetry/datas/statistic', 'allow', NULL, NULL, NULL),
	(1073, 'p', 'TENANT_USER', 'api/v1/telemetry/datas/statistic', 'allow', NULL, NULL, NULL),
	(1074, 'p', 'SYS_ADMIN', 'api/v1/telemetry/datas/statistic/batch', 'allow', NULL, NULL, NULL),
	(1075, 'p', 'TENANT_ADMIN', 'api/v1/telemetry/datas/statistic/batch', 'allow', NULL, NULL, NULL),
	(1076, 'p', 'TENANT_USER', 'api/v1/telemetry/datas/statistic/batch', 'allow', NULL, NULL, NULL),
	(1077, 'p', 'SYS_ADMIN', 'api/v1/telemetry/datas/uplink-dead-letters', 'allow', NULL, NULL, NULL),
	(1078, 'p', 'TENANT_ADMIN', 'api/v1/telemetry/datas/uplink-dead-letters', 'allow', NULL, NULL, NULL),
	(1079, 'p', 'TENANT_USER', 'api/v1/telemetry/datas/uplink-dead-letters', 'allow', NULL, NULL, NULL),
	(1080, 'p', 'SYS_ADMIN', 'api/v1/telemetry/datas/uplink-dead-letters/:id/status', 'allow', NULL, NULL, NULL),
	(1081, 'p', 'TENANT_ADMIN', 'api/v1/telemetry/datas/uplink-dead-letters/:id/status', 'allow', NULL, NULL, NULL),
	(1082, 'p', 'TENANT_USER', 'api/v1/telemetry/datas/uplink-dead-letters/:id/status', 'allow', NULL, NULL, NULL),
	(1083, 'p', 'SYS_ADMIN', 'api/v1/telemetry/datas/uplink-dead-letters/drain', 'allow', NULL, NULL, NULL),
	(1084, 'p', 'TENANT_ADMIN', 'api/v1/telemetry/datas/uplink-dead-letters/drain', 'allow', NULL, NULL, NULL),
	(1085, 'p', 'TENANT_USER', 'api/v1/telemetry/datas/uplink-dead-letters/drain', 'allow', NULL, NULL, NULL),
	(1086, 'p', 'SYS_ADMIN', 'api/v1/ui_elements', 'allow', NULL, NULL, NULL),
	(1087, 'p', 'TENANT_ADMIN', 'api/v1/ui_elements', 'allow', NULL, NULL, NULL),
	(1089, 'p', 'SYS_ADMIN', 'api/v1/ui_elements/:id', 'allow', NULL, NULL, NULL),
	(1090, 'p', 'TENANT_ADMIN', 'api/v1/ui_elements/:id', 'allow', NULL, NULL, NULL),
	(1092, 'p', 'SYS_ADMIN', 'api/v1/ui_elements/menu', 'allow', NULL, NULL, NULL),
	(1093, 'p', 'TENANT_ADMIN', 'api/v1/ui_elements/menu', 'allow', NULL, NULL, NULL),
	(1094, 'p', 'TENANT_USER', 'api/v1/ui_elements/menu', 'allow', NULL, NULL, NULL),
	(1095, 'p', 'SYS_ADMIN', 'api/v1/ui_elements/select/form', 'allow', NULL, NULL, NULL),
	(1096, 'p', 'TENANT_ADMIN', 'api/v1/ui_elements/select/form', 'allow', NULL, NULL, NULL),
	(1097, 'p', 'TENANT_USER', 'api/v1/ui_elements/select/form', 'allow', NULL, NULL, NULL),
	(1098, 'p', 'SYS_ADMIN', 'api/v1/user', 'allow', NULL, NULL, NULL),
	(1099, 'p', 'TENANT_ADMIN', 'api/v1/user', 'allow', NULL, NULL, NULL),
	(1101, 'p', 'SYS_ADMIN', 'api/v1/user/:id', 'allow', NULL, NULL, NULL),
	(1102, 'p', 'TENANT_ADMIN', 'api/v1/user/:id', 'allow', NULL, NULL, NULL),
	(1104, 'p', 'SYS_ADMIN', 'api/v1/user/address/:id', 'allow', NULL, NULL, NULL),
	(1105, 'p', 'TENANT_ADMIN', 'api/v1/user/address/:id', 'allow', NULL, NULL, NULL),
	(1106, 'p', 'TENANT_USER', 'api/v1/user/address/:id', 'allow', NULL, NULL, NULL),
	(1107, 'p', 'SYS_ADMIN', 'api/v1/user/change-email', 'allow', NULL, NULL, NULL),
	(1108, 'p', 'TENANT_ADMIN', 'api/v1/user/change-email', 'allow', NULL, NULL, NULL),
	(1109, 'p', 'TENANT_USER', 'api/v1/user/change-email', 'allow', NULL, NULL, NULL),
	(1110, 'p', 'SYS_ADMIN', 'api/v1/user/detail', 'allow', NULL, NULL, NULL),
	(1111, 'p', 'TENANT_ADMIN', 'api/v1/user/detail', 'allow', NULL, NULL, NULL),
	(1112, 'p', 'TENANT_USER', 'api/v1/user/detail', 'allow', NULL, NULL, NULL),
	(1113, 'p', 'SYS_ADMIN', 'api/v1/user/logout', 'allow', NULL, NULL, NULL),
	(1114, 'p', 'TENANT_ADMIN', 'api/v1/user/logout', 'allow', NULL, NULL, NULL),
	(1115, 'p', 'TENANT_USER', 'api/v1/user/logout', 'allow', NULL, NULL, NULL),
	(1116, 'p', 'SYS_ADMIN', 'api/v1/user/prefer-lang', 'allow', NULL, NULL, NULL),
	(1117, 'p', 'TENANT_ADMIN', 'api/v1/user/prefer-lang', 'allow', NULL, NULL, NULL),
	(1118, 'p', 'TENANT_USER', 'api/v1/user/prefer-lang', 'allow', NULL, NULL, NULL),
	(1119, 'p', 'SYS_ADMIN', 'api/v1/user/refresh', 'allow', NULL, NULL, NULL),
	(1120, 'p', 'TENANT_ADMIN', 'api/v1/user/refresh', 'allow', NULL, NULL, NULL),
	(1121, 'p', 'TENANT_USER', 'api/v1/user/refresh', 'allow', NULL, NULL, NULL),
	(1122, 'p', 'SYS_ADMIN', 'api/v1/user/selector', 'allow', NULL, NULL, NULL),
	(1123, 'p', 'TENANT_ADMIN', 'api/v1/user/selector', 'allow', NULL, NULL, NULL),
	(1124, 'p', 'TENANT_USER', 'api/v1/user/selector', 'allow', NULL, NULL, NULL),
	(1125, 'p', 'SYS_ADMIN', 'api/v1/user/tenant/id', 'allow', NULL, NULL, NULL),
	(1126, 'p', 'TENANT_ADMIN', 'api/v1/user/tenant/id', 'allow', NULL, NULL, NULL),
	(1127, 'p', 'TENANT_USER', 'api/v1/user/tenant/id', 'allow', NULL, NULL, NULL),
	(1128, 'p', 'SYS_ADMIN', 'api/v1/user/totp/activate', 'allow', NULL, NULL, NULL),
	(1129, 'p', 'TENANT_ADMIN', 'api/v1/user/totp/activate', 'allow', NULL, NULL, NULL),
	(1130, 'p', 'TENANT_USER', 'api/v1/user/totp/activate', 'allow', NULL, NULL, NULL),
	(1131, 'p', 'SYS_ADMIN', 'api/v1/user/totp/disable', 'allow', NULL, NULL, NULL),
	(1132, 'p', 'TENANT_ADMIN', 'api/v1/user/totp/disable', 'allow', NULL, NULL, NULL),
	(1133, 'p', 'TENANT_USER', 'api/v1/user/totp/disable', 'allow', NULL, NULL, NULL),
	(1134, 'p', 'SYS_ADMIN', 'api/v1/user/totp/setup', 'allow', NULL, NULL, NULL),
	(1135, 'p', 'TENANT_ADMIN', 'api/v1/user/totp/setup', 'allow', NULL, NULL, NULL),
	(1136, 'p', 'TENANT_USER', 'api/v1/user/totp/setup', 'allow', NULL, NULL, NULL),
	(1137, 'p', 'SYS_ADMIN', 'api/v1/user/totp/status', 'allow', NULL, NULL, NULL),
	(1138, 'p', 'TENANT_ADMIN', 'api/v1/user/totp/status', 'allow', NULL, NULL, NULL),
	(1139, 'p', 'TENANT_USER', 'api/v1/user/totp/status', 'allow', NULL, NULL, NULL),
	(1140, 'p', 'SYS_ADMIN', 'api/v1/user/transform', 'allow', NULL, NULL, NULL),
	(1141, 'p', 'TENANT_ADMIN', 'api/v1/user/transform', 'allow', NULL, NULL, NULL),
	(1143, 'p', 'SYS_ADMIN', 'api/v1/user/update', 'allow', NULL, NULL, NULL),
	(1144, 'p', 'TENANT_ADMIN', 'api/v1/user/update', 'allow', NULL, NULL, NULL),
	(1145, 'p', 'TENANT_USER', 'api/v1/user/update', 'allow', NULL, NULL, NULL),
	(1146, 'p', 'SYS_ADMIN', 'api/v1/user/warning-email', 'allow', NULL, NULL, NULL),
	(1147, 'p', 'TENANT_ADMIN', 'api/v1/user/warning-email', 'allow', NULL, NULL, NULL),
	(1149, 'g', '11111111-4fe9-b409-67c3-111111111111', 'TENANT_ADMIN', NULL, NULL, NULL, NULL),
	(1150, 'g2', 'api/v1/device/template/export/:id', 'api/v1/device/template/export/:id', NULL, NULL, NULL, NULL),
	(1151, 'g2', 'api/v1/device/template/import', 'api/v1/device/template/import', NULL, NULL, NULL, NULL),
	(1152, 'p', 'SYS_ADMIN', 'api/v1/device/template/export/:id', 'allow', NULL, NULL, NULL),
	(1153, 'p', 'TENANT_ADMIN', 'api/v1/device/template/export/:id', 'allow', NULL, NULL, NULL),
	(1154, 'p', 'SYS_ADMIN', 'api/v1/device/template/import', 'allow', NULL, NULL, NULL),
	(1155, 'p', 'TENANT_ADMIN', 'api/v1/device/template/import', 'allow', NULL, NULL, NULL),
	(1156, 'g2', 'api/v1/rule-chains/:id/nodes/:nodeId/traces', 'api/v1/rule-chains/:id/nodes/:nodeId/traces', NULL, NULL, NULL, NULL),
	(1157, 'p', 'SYS_ADMIN', 'api/v1/rule-chains/:id/nodes/:nodeId/traces', 'allow', NULL, NULL, NULL),
	(1158, 'p', 'TENANT_ADMIN', 'api/v1/rule-chains/:id/nodes/:nodeId/traces', 'allow', NULL, NULL, NULL),
	(1159, 'g2', 'api/v1/plugins', 'api/v1/plugins', NULL, NULL, NULL, NULL),
	(1160, 'g2', 'api/v1/plugins/:id', 'api/v1/plugins/:id', NULL, NULL, NULL, NULL),
	(1161, 'g2', 'api/v1/plugins/:id/enable', 'api/v1/plugins/:id/enable', NULL, NULL, NULL, NULL),
	(1162, 'g2', 'api/v1/plugins/:id/disable', 'api/v1/plugins/:id/disable', NULL, NULL, NULL, NULL),
	(1163, 'g2', 'api/v1/plugins/:id/downlink', 'api/v1/plugins/:id/downlink', NULL, NULL, NULL, NULL),
	(1164, 'p', 'SYS_ADMIN', 'api/v1/plugins', 'allow', NULL, NULL, NULL),
	(1165, 'p', 'SYS_ADMIN', 'api/v1/plugins/:id', 'allow', NULL, NULL, NULL),
	(1166, 'p', 'SYS_ADMIN', 'api/v1/plugins/:id/enable', 'allow', NULL, NULL, NULL),
	(1167, 'p', 'SYS_ADMIN', 'api/v1/plugins/:id/disable', 'allow', NULL, NULL, NULL),
	(1168, 'p', 'SYS_ADMIN', 'api/v1/plugins/:id/downlink', 'allow', NULL, NULL, NULL),
	(1169, 'p', 'TENANT_ADMIN', 'api/v1/plugins', 'allow', NULL, NULL, NULL),
	(1170, 'p', 'TENANT_ADMIN', 'api/v1/plugins/:id', 'allow', NULL, NULL, NULL),
	(1171, 'p', 'TENANT_ADMIN', 'api/v1/plugins/:id/enable', 'allow', NULL, NULL, NULL),
	(1172, 'p', 'TENANT_ADMIN', 'api/v1/plugins/:id/disable', 'allow', NULL, NULL, NULL),
	(1173, 'p', 'TENANT_ADMIN', 'api/v1/plugins/:id/downlink', 'allow', NULL, NULL, NULL),
	(1174, 'g2', 'api/v1/device/template/market/bundle', 'api/v1/device/template/market/bundle', NULL, NULL, NULL, NULL),
	(1175, 'p', 'SYS_ADMIN', 'api/v1/device/template/market/bundle', 'allow', NULL, NULL, NULL),
	(1176, 'p', 'TENANT_ADMIN', 'api/v1/device/template/market/bundle', 'allow', NULL, NULL, NULL),
	(1183, 'g2', 'api/v1/calculated_fields/recompute', 'api/v1/calculated_fields/recompute', NULL, NULL, NULL, NULL),
	(1184, 'g2', 'api/v1/calculated_fields/recompute/:id', 'api/v1/calculated_fields/recompute/:id', NULL, NULL, NULL, NULL),
	(1185, 'g2', 'api/v1/device/template/market/catalog', 'api/v1/device/template/market/catalog', NULL, NULL, NULL, NULL),
	(1186, 'g2', 'api/v1/plugins/:id/token', 'api/v1/plugins/:id/token', NULL, NULL, NULL, NULL),
	(1187, 'p', 'SYS_ADMIN', 'api/v1/calculated_fields/recompute', 'allow', NULL, NULL, NULL),
	(1188, 'p', 'TENANT_ADMIN', 'api/v1/calculated_fields/recompute', 'allow', NULL, NULL, NULL),
	(1189, 'p', 'SYS_ADMIN', 'api/v1/calculated_fields/recompute/:id', 'allow', NULL, NULL, NULL),
	(1190, 'p', 'TENANT_ADMIN', 'api/v1/calculated_fields/recompute/:id', 'allow', NULL, NULL, NULL),
	(1191, 'p', 'SYS_ADMIN', 'api/v1/plugins/:id/token', 'allow', NULL, NULL, NULL),
	(1192, 'p', 'TENANT_ADMIN', 'api/v1/plugins/:id/token', 'allow', NULL, NULL, NULL),
	(1193, 'p', 'SYS_ADMIN', 'api/v1/device/template/market/catalog', 'allow', NULL, NULL, NULL),
	(1194, 'p', 'TENANT_ADMIN', 'api/v1/device/template/market/catalog', 'allow', NULL, NULL, NULL),
	(1195, 'p', 'TENANT_USER', 'api/v1/device/template/market/catalog', 'allow', NULL, NULL, NULL),
	(1196, 'g2', 'api/v1/report/schedules', 'api/v1/report/schedules', NULL, NULL, NULL, NULL),
	(1197, 'g2', 'api/v1/report/schedules/:id', 'api/v1/report/schedules/:id', NULL, NULL, NULL, NULL),
	(1198, 'g2', 'api/v1/report/schedules/:id/run', 'api/v1/report/schedules/:id/run', NULL, NULL, NULL, NULL),
	(1199, 'p', 'SYS_ADMIN', 'api/v1/report/schedules', 'allow', NULL, NULL, NULL),
	(1200, 'p', 'TENANT_ADMIN', 'api/v1/report/schedules', 'allow', NULL, NULL, NULL),
	(1201, 'p', 'SYS_ADMIN', 'api/v1/report/schedules/:id', 'allow', NULL, NULL, NULL),
	(1202, 'p', 'TENANT_ADMIN', 'api/v1/report/schedules/:id', 'allow', NULL, NULL, NULL),
	(1203, 'p', 'SYS_ADMIN', 'api/v1/report/schedules/:id/run', 'allow', NULL, NULL, NULL),
	(1204, 'p', 'TENANT_ADMIN', 'api/v1/report/schedules/:id/run', 'allow', NULL, NULL, NULL),
	(1205, 'g2', 'api/v1/device-certificates', 'api/v1/device-certificates', NULL, NULL, NULL, NULL),
	(1206, 'g2', 'api/v1/device-certificates/issue', 'api/v1/device-certificates/issue', NULL, NULL, NULL, NULL),
	(1207, 'g2', 'api/v1/device-certificates/verify', 'api/v1/device-certificates/verify', NULL, NULL, NULL, NULL),
	(1208, 'g2', 'api/v1/device-certificates/:id', 'api/v1/device-certificates/:id', NULL, NULL, NULL, NULL),
	(1209, 'g2', 'api/v1/device-certificates/:id/revoke', 'api/v1/device-certificates/:id/revoke', NULL, NULL, NULL, NULL),
	(1210, 'g2', 'api/v1/device-certificates/:id/renew', 'api/v1/device-certificates/:id/renew', NULL, NULL, NULL, NULL),
	(1211, 'p', 'SYS_ADMIN', 'api/v1/device-certificates', 'allow', NULL, NULL, NULL),
	(1212, 'p', 'TENANT_ADMIN', 'api/v1/device-certificates', 'allow', NULL, NULL, NULL),
	(1213, 'p', 'SYS_ADMIN', 'api/v1/device-certificates/issue', 'allow', NULL, NULL, NULL),
	(1214, 'p', 'TENANT_ADMIN', 'api/v1/device-certificates/issue', 'allow', NULL, NULL, NULL),
	(1215, 'p', 'SYS_ADMIN', 'api/v1/device-certificates/verify', 'allow', NULL, NULL, NULL),
	(1216, 'p', 'TENANT_ADMIN', 'api/v1/device-certificates/verify', 'allow', NULL, NULL, NULL),
	(1217, 'p', 'SYS_ADMIN', 'api/v1/device-certificates/:id', 'allow', NULL, NULL, NULL),
	(1218, 'p', 'TENANT_ADMIN', 'api/v1/device-certificates/:id', 'allow', NULL, NULL, NULL),
	(1219, 'p', 'SYS_ADMIN', 'api/v1/device-certificates/:id/revoke', 'allow', NULL, NULL, NULL),
	(1220, 'p', 'TENANT_ADMIN', 'api/v1/device-certificates/:id/revoke', 'allow', NULL, NULL, NULL),
	(1221, 'p', 'SYS_ADMIN', 'api/v1/device-certificates/:id/renew', 'allow', NULL, NULL, NULL),
	(1222, 'p', 'TENANT_ADMIN', 'api/v1/device-certificates/:id/renew', 'allow', NULL, NULL, NULL),
	(1223, 'g2', 'api/v1/edge/sync', 'api/v1/edge/sync', NULL, NULL, NULL, NULL),
	(1224, 'g2', 'api/v1/edge/sync/:id', 'api/v1/edge/sync/:id', NULL, NULL, NULL, NULL),
	(1225, 'g2', 'api/v1/edge/sync/:id/retry', 'api/v1/edge/sync/:id/retry', NULL, NULL, NULL, NULL),
	(1226, 'g2', 'api/v1/edge/ota/distribute', 'api/v1/edge/ota/distribute', NULL, NULL, NULL, NULL),
	(1227, 'p', 'SYS_ADMIN', 'api/v1/edge/sync', 'allow', NULL, NULL, NULL),
	(1228, 'p', 'TENANT_ADMIN', 'api/v1/edge/sync', 'allow', NULL, NULL, NULL),
	(1229, 'p', 'SYS_ADMIN', 'api/v1/edge/sync/:id', 'allow', NULL, NULL, NULL),
	(1230, 'p', 'TENANT_ADMIN', 'api/v1/edge/sync/:id', 'allow', NULL, NULL, NULL),
	(1231, 'p', 'SYS_ADMIN', 'api/v1/edge/sync/:id/retry', 'allow', NULL, NULL, NULL),
	(1232, 'p', 'TENANT_ADMIN', 'api/v1/edge/sync/:id/retry', 'allow', NULL, NULL, NULL),
	(1233, 'p', 'SYS_ADMIN', 'api/v1/edge/ota/distribute', 'allow', NULL, NULL, NULL),
	(1234, 'p', 'TENANT_ADMIN', 'api/v1/edge/ota/distribute', 'allow', NULL, NULL, NULL),
	(1235, 'g2', 'api/v1/ai/models', 'api/v1/ai/models', NULL, NULL, NULL, NULL),
	(1236, 'g2', 'api/v1/ai/models/:id', 'api/v1/ai/models/:id', NULL, NULL, NULL, NULL),
	(1237, 'g2', 'api/v1/ai/assistant/chat', 'api/v1/ai/assistant/chat', NULL, NULL, NULL, NULL),
	(1238, 'p', 'SYS_ADMIN', 'api/v1/ai/models', 'allow', NULL, NULL, NULL),
	(1239, 'p', 'TENANT_ADMIN', 'api/v1/ai/models', 'allow', NULL, NULL, NULL),
	(1240, 'p', 'SYS_ADMIN', 'api/v1/ai/models/:id', 'allow', NULL, NULL, NULL),
	(1241, 'p', 'TENANT_ADMIN', 'api/v1/ai/models/:id', 'allow', NULL, NULL, NULL),
	(1242, 'p', 'SYS_ADMIN', 'api/v1/ai/assistant/chat', 'allow', NULL, NULL, NULL),
	(1243, 'p', 'TENANT_ADMIN', 'api/v1/ai/assistant/chat', 'allow', NULL, NULL, NULL),
	(1244, 'p', 'TENANT_USER', 'api/v1/ai/assistant/chat', 'allow', NULL, NULL, NULL),
	(1245, 'g2', 'api/v1/report/schedules/:id/runs', 'api/v1/report/schedules/:id/runs', NULL, NULL, NULL, NULL),
	(1328, 'p', 'SYS_ADMIN', 'api/v1/mobile/devices/:id/shadow', 'allow', NULL, NULL, NULL),
	(1246, 'g2', 'api/v1/report/schedules/:id/runs/:run_id', 'api/v1/report/schedules/:id/runs/:run_id', NULL, NULL, NULL, NULL),
	(1247, 'g2', 'api/v1/report/schedules/:id/runs/:run_id/retry', 'api/v1/report/schedules/:id/runs/:run_id/retry', NULL, NULL, NULL, NULL),
	(1248, 'p', 'SYS_ADMIN', 'api/v1/report/schedules/:id/runs', 'allow', NULL, NULL, NULL),
	(1249, 'p', 'TENANT_ADMIN', 'api/v1/report/schedules/:id/runs', 'allow', NULL, NULL, NULL);
INSERT INTO public.casbin_rule (id, ptype, v0, v1, v2, v3, v4, v5) VALUES
	(1250, 'p', 'SYS_ADMIN', 'api/v1/report/schedules/:id/runs/:run_id', 'allow', NULL, NULL, NULL),
	(1251, 'p', 'TENANT_ADMIN', 'api/v1/report/schedules/:id/runs/:run_id', 'allow', NULL, NULL, NULL),
	(1252, 'p', 'SYS_ADMIN', 'api/v1/report/schedules/:id/runs/:run_id/retry', 'allow', NULL, NULL, NULL),
	(1253, 'p', 'TENANT_ADMIN', 'api/v1/report/schedules/:id/runs/:run_id/retry', 'allow', NULL, NULL, NULL),
	(1254, 'g2', 'api/v1/device/preRegister/cleanup', 'api/v1/device/preRegister/cleanup', NULL, NULL, NULL, NULL),
	(1255, 'p', 'SYS_ADMIN', 'api/v1/device/preRegister/cleanup', 'allow', NULL, NULL, NULL),
	(1256, 'p', 'TENANT_ADMIN', 'api/v1/device/preRegister/cleanup', 'allow', NULL, NULL, NULL),
	(1257, 'g2', 'api/v1/ota/task/:id/governance-apply', 'api/v1/ota/task/:id/governance-apply', NULL, NULL, NULL, NULL),
	(1258, 'g2', 'api/v1/entity-relations', 'api/v1/entity-relations', NULL, NULL, NULL, NULL),
	(1259, 'g2', 'api/v1/entity-relations/:id', 'api/v1/entity-relations/:id', NULL, NULL, NULL, NULL),
	(1260, 'g2', 'api/v1/entity-relations/by-entity', 'api/v1/entity-relations/by-entity', NULL, NULL, NULL, NULL),
	(1261, 'p', 'SYS_ADMIN', 'api/v1/ota/task/:id/governance-apply', 'allow', NULL, NULL, NULL),
	(1262, 'p', 'TENANT_ADMIN', 'api/v1/ota/task/:id/governance-apply', 'allow', NULL, NULL, NULL),
	(1263, 'p', 'TENANT_USER', 'api/v1/ota/task/:id/governance-apply', 'allow', NULL, NULL, NULL),
	(1264, 'p', 'SYS_ADMIN', 'api/v1/entity-relations', 'allow', NULL, NULL, NULL),
	(1265, 'p', 'TENANT_ADMIN', 'api/v1/entity-relations', 'allow', NULL, NULL, NULL),
	(1266, 'p', 'TENANT_USER', 'api/v1/entity-relations', 'allow', NULL, NULL, NULL),
	(1267, 'p', 'SYS_ADMIN', 'api/v1/entity-relations/:id', 'allow', NULL, NULL, NULL),
	(1268, 'p', 'TENANT_ADMIN', 'api/v1/entity-relations/:id', 'allow', NULL, NULL, NULL),
	(1269, 'p', 'TENANT_USER', 'api/v1/entity-relations/:id', 'allow', NULL, NULL, NULL),
	(1270, 'p', 'SYS_ADMIN', 'api/v1/entity-relations/by-entity', 'allow', NULL, NULL, NULL),
	(1271, 'p', 'TENANT_ADMIN', 'api/v1/entity-relations/by-entity', 'allow', NULL, NULL, NULL),
	(1272, 'p', 'TENANT_USER', 'api/v1/entity-relations/by-entity', 'allow', NULL, NULL, NULL),
	(1273, 'g2', 'api/v1/rule-chains/:id/versions', 'api/v1/rule-chains/:id/versions', NULL, NULL, NULL, NULL),
	(1274, 'g2', 'api/v1/rule-chains/versions/publish', 'api/v1/rule-chains/versions/publish', NULL, NULL, NULL, NULL),
	(1275, 'g2', 'api/v1/rule-chains/versions/rollback', 'api/v1/rule-chains/versions/rollback', NULL, NULL, NULL, NULL),
	(1276, 'p', 'SYS_ADMIN', 'api/v1/rule-chains/:id/versions', 'allow', NULL, NULL, NULL),
	(1277, 'p', 'TENANT_ADMIN', 'api/v1/rule-chains/:id/versions', 'allow', NULL, NULL, NULL),
	(1278, 'p', 'SYS_ADMIN', 'api/v1/rule-chains/versions/publish', 'allow', NULL, NULL, NULL),
	(1279, 'p', 'TENANT_ADMIN', 'api/v1/rule-chains/versions/publish', 'allow', NULL, NULL, NULL),
	(1280, 'p', 'SYS_ADMIN', 'api/v1/rule-chains/versions/rollback', 'allow', NULL, NULL, NULL),
	(1281, 'p', 'TENANT_ADMIN', 'api/v1/rule-chains/versions/rollback', 'allow', NULL, NULL, NULL),
	(1282, 'g2', 'api/v1/scada/projects', 'api/v1/scada/projects', NULL, NULL, NULL, NULL),
	(1283, 'g2', 'api/v1/scada/projects/:id', 'api/v1/scada/projects/:id', NULL, NULL, NULL, NULL),
	(1284, 'g2', 'api/v1/scada/projects/:id/documents', 'api/v1/scada/projects/:id/documents', NULL, NULL, NULL, NULL),
	(1285, 'g2', 'api/v1/scada/documents/:id', 'api/v1/scada/documents/:id', NULL, NULL, NULL, NULL),
	(1286, 'g2', 'api/v1/scada/documents/:id/publish', 'api/v1/scada/documents/:id/publish', NULL, NULL, NULL, NULL),
	(1287, 'g2', 'api/v1/scada/documents/:id/rollback', 'api/v1/scada/documents/:id/rollback', NULL, NULL, NULL, NULL),
	(1288, 'g2', 'api/v1/scada/documents/:id/archive', 'api/v1/scada/documents/:id/archive', NULL, NULL, NULL, NULL),
	(1289, 'g2', 'api/v1/scada/documents/:id/versions', 'api/v1/scada/documents/:id/versions', NULL, NULL, NULL, NULL),
	(1290, 'g2', 'api/v1/scada/documents/:id/audits', 'api/v1/scada/documents/:id/audits', NULL, NULL, NULL, NULL),
	(1291, 'g2', 'api/v1/scada/control/confirm', 'api/v1/scada/control/confirm', NULL, NULL, NULL, NULL),
	(1292, 'g2', 'api/v1/scada/control', 'api/v1/scada/control', NULL, NULL, NULL, NULL),
	(1293, 'g2', 'api/v1/mobile/capabilities', 'api/v1/mobile/capabilities', NULL, NULL, NULL, NULL),
	(1294, 'g2', 'api/v1/mobile/push/subscribe', 'api/v1/mobile/push/subscribe', NULL, NULL, NULL, NULL),
	(1295, 'g2', 'api/v1/mobile/push/:id', 'api/v1/mobile/push/:id', NULL, NULL, NULL, NULL),
	(1296, 'g2', 'api/v1/mobile/commands', 'api/v1/mobile/commands', NULL, NULL, NULL, NULL),
	(1297, 'g2', 'api/v1/mobile/devices', 'api/v1/mobile/devices', NULL, NULL, NULL, NULL),
	(1298, 'g2', 'api/v1/mobile/alarms', 'api/v1/mobile/alarms', NULL, NULL, NULL, NULL),
	(1299, 'g2', 'api/v1/mobile/alarms/:id/ack', 'api/v1/mobile/alarms/:id/ack', NULL, NULL, NULL, NULL),
	(1300, 'g2', 'api/v1/mobile/devices/:id/shadow', 'api/v1/mobile/devices/:id/shadow', NULL, NULL, NULL, NULL),
	(1301, 'g2', 'api/v1/mobile/devices/:id/ota', 'api/v1/mobile/devices/:id/ota', NULL, NULL, NULL, NULL),
	(1302, 'g2', 'api/v1/mobile/dashboards', 'api/v1/mobile/dashboards', NULL, NULL, NULL, NULL),
	(1303, 'g2', 'api/v1/device/shadow/:deviceId/:msgId/ack', 'api/v1/device/shadow/:deviceId/:msgId/ack', NULL, NULL, NULL, NULL),
	(1304, 'g2', 'api/v1/command/datas/jobs/:job_id/pause', 'api/v1/command/datas/jobs/:job_id/pause', NULL, NULL, NULL, NULL),
	(1305, 'g2', 'api/v1/command/datas/jobs/:job_id/resume', 'api/v1/command/datas/jobs/:job_id/resume', NULL, NULL, NULL, NULL),
	(1306, 'g2', 'api/v1/command/datas/jobs/:job_id/rollback', 'api/v1/command/datas/jobs/:job_id/rollback', NULL, NULL, NULL, NULL),
	(1307, 'g2', 'api/v1/command/datas/jobs/:job_id/progress', 'api/v1/command/datas/jobs/:job_id/progress', NULL, NULL, NULL, NULL),
	(1308, 'g2', 'api/v1/command/datas/jobs/:job_id/report', 'api/v1/command/datas/jobs/:job_id/report', NULL, NULL, NULL, NULL),
	(1309, 'p', 'TENANT_USER', 'api/v1/mobile/devices/:id/shadow', 'allow', NULL, NULL, NULL),
	(1310, 'p', 'TENANT_ADMIN', 'api/v1/scada/documents/:id', 'allow', NULL, NULL, NULL),
	(1311, 'p', 'SYS_ADMIN', 'api/v1/scada/projects/:id/documents', 'allow', NULL, NULL, NULL),
	(1312, 'p', 'SYS_ADMIN', 'api/v1/mobile/alarms/:id/ack', 'allow', NULL, NULL, NULL),
	(1313, 'p', 'TENANT_USER', 'api/v1/device/shadow/:deviceId/:msgId/ack', 'allow', NULL, NULL, NULL),
	(1314, 'p', 'SYS_ADMIN', 'api/v1/scada/documents/:id/publish', 'allow', NULL, NULL, NULL),
	(1315, 'p', 'SYS_ADMIN', 'api/v1/mobile/alarms', 'allow', NULL, NULL, NULL),
	(1316, 'p', 'SYS_ADMIN', 'api/v1/mobile/commands', 'allow', NULL, NULL, NULL),
	(1317, 'p', 'TENANT_USER', 'api/v1/mobile/dashboards', 'allow', NULL, NULL, NULL),
	(1318, 'p', 'TENANT_ADMIN', 'api/v1/mobile/devices/:id/ota', 'allow', NULL, NULL, NULL),
	(1319, 'p', 'SYS_ADMIN', 'api/v1/scada/documents/:id/archive', 'allow', NULL, NULL, NULL),
	(1320, 'p', 'TENANT_ADMIN', 'api/v1/command/datas/jobs/:job_id/report', 'allow', NULL, NULL, NULL),
	(1321, 'p', 'TENANT_ADMIN', 'api/v1/mobile/capabilities', 'allow', NULL, NULL, NULL),
	(1322, 'p', 'TENANT_ADMIN', 'api/v1/scada/documents/:id/rollback', 'allow', NULL, NULL, NULL),
	(1323, 'p', 'TENANT_ADMIN', 'api/v1/mobile/push/subscribe', 'allow', NULL, NULL, NULL),
	(1324, 'p', 'SYS_ADMIN', 'api/v1/device/shadow/:deviceId/:msgId/ack', 'allow', NULL, NULL, NULL),
	(1325, 'p', 'TENANT_ADMIN', 'api/v1/scada/documents/:id/audits', 'allow', NULL, NULL, NULL),
	(1326, 'p', 'TENANT_USER', 'api/v1/mobile/alarms/:id/ack', 'allow', NULL, NULL, NULL),
	(1327, 'p', 'TENANT_USER', 'api/v1/scada/projects/:id/documents', 'allow', NULL, NULL, NULL),
	(1329, 'p', 'TENANT_ADMIN', 'api/v1/mobile/devices', 'allow', NULL, NULL, NULL),
	(1330, 'p', 'TENANT_ADMIN', 'api/v1/scada/documents/:id/versions', 'allow', NULL, NULL, NULL),
	(1331, 'p', 'TENANT_USER', 'api/v1/scada/documents/:id/archive', 'allow', NULL, NULL, NULL),
	(1332, 'p', 'TENANT_ADMIN', 'api/v1/scada/projects', 'allow', NULL, NULL, NULL),
	(1333, 'p', 'TENANT_ADMIN', 'api/v1/scada/projects/:id', 'allow', NULL, NULL, NULL),
	(1334, 'p', 'SYS_ADMIN', 'api/v1/mobile/dashboards', 'allow', NULL, NULL, NULL),
	(1335, 'p', 'TENANT_ADMIN', 'api/v1/mobile/push/:id', 'allow', NULL, NULL, NULL),
	(1336, 'p', 'TENANT_USER', 'api/v1/mobile/alarms', 'allow', NULL, NULL, NULL),
	(1337, 'p', 'TENANT_USER', 'api/v1/mobile/commands', 'allow', NULL, NULL, NULL),
	(1338, 'p', 'TENANT_USER', 'api/v1/scada/documents/:id/publish', 'allow', NULL, NULL, NULL),
	(1339, 'p', 'SYS_ADMIN', 'api/v1/command/datas/jobs/:job_id/report', 'allow', NULL, NULL, NULL),
	(1340, 'p', 'TENANT_USER', 'api/v1/scada/projects', 'allow', NULL, NULL, NULL),
	(1341, 'p', 'TENANT_ADMIN', 'api/v1/scada/documents/:id/archive', 'allow', NULL, NULL, NULL),
	(1342, 'p', 'TENANT_USER', 'api/v1/scada/documents/:id/versions', 'allow', NULL, NULL, NULL),
	(1343, 'p', 'TENANT_ADMIN', 'api/v1/mobile/alarms', 'allow', NULL, NULL, NULL),
	(1344, 'p', 'TENANT_ADMIN', 'api/v1/mobile/commands', 'allow', NULL, NULL, NULL),
	(1345, 'p', 'TENANT_USER', 'api/v1/mobile/push/:id', 'allow', NULL, NULL, NULL),
	(1346, 'p', 'TENANT_USER', 'api/v1/scada/projects/:id', 'allow', NULL, NULL, NULL),
	(1347, 'p', 'SYS_ADMIN', 'api/v1/mobile/devices/:id/ota', 'allow', NULL, NULL, NULL),
	(1348, 'p', 'TENANT_ADMIN', 'api/v1/scada/documents/:id/publish', 'allow', NULL, NULL, NULL),
	(1349, 'p', 'TENANT_USER', 'api/v1/scada/documents/:id/rollback', 'allow', NULL, NULL, NULL),
	(1350, 'p', 'TENANT_USER', 'api/v1/mobile/capabilities', 'allow', NULL, NULL, NULL),
	(1351, 'p', 'TENANT_USER', 'api/v1/mobile/push/subscribe', 'allow', NULL, NULL, NULL),
	(1352, 'p', 'TENANT_ADMIN', 'api/v1/mobile/alarms/:id/ack', 'allow', NULL, NULL, NULL),
	(1353, 'p', 'TENANT_USER', 'api/v1/scada/documents/:id/audits', 'allow', NULL, NULL, NULL),
	(1354, 'p', 'SYS_ADMIN', 'api/v1/scada/documents/:id', 'allow', NULL, NULL, NULL),
	(1355, 'p', 'TENANT_USER', 'api/v1/mobile/devices', 'allow', NULL, NULL, NULL),
	(1356, 'p', 'TENANT_ADMIN', 'api/v1/scada/projects/:id/documents', 'allow', NULL, NULL, NULL),
	(1357, 'p', 'TENANT_USER', 'api/v1/mobile/devices/:id/ota', 'allow', NULL, NULL, NULL),
	(1358, 'p', 'SYS_ADMIN', 'api/v1/scada/projects/:id', 'allow', NULL, NULL, NULL),
	(1359, 'p', 'TENANT_ADMIN', 'api/v1/mobile/dashboards', 'allow', NULL, NULL, NULL),
	(1360, 'p', 'SYS_ADMIN', 'api/v1/mobile/push/:id', 'allow', NULL, NULL, NULL),
	(1361, 'p', 'SYS_ADMIN', 'api/v1/scada/documents/:id/versions', 'allow', NULL, NULL, NULL),
	(1362, 'p', 'SYS_ADMIN', 'api/v1/scada/projects', 'allow', NULL, NULL, NULL),
	(1363, 'p', 'TENANT_USER', 'api/v1/command/datas/jobs/:job_id/report', 'allow', NULL, NULL, NULL),
	(1364, 'p', 'TENANT_ADMIN', 'api/v1/mobile/devices/:id/shadow', 'allow', NULL, NULL, NULL),
	(1365, 'p', 'TENANT_USER', 'api/v1/scada/documents/:id', 'allow', NULL, NULL, NULL),
	(1366, 'p', 'SYS_ADMIN', 'api/v1/mobile/devices', 'allow', NULL, NULL, NULL),
	(1367, 'p', 'SYS_ADMIN', 'api/v1/scada/documents/:id/audits', 'allow', NULL, NULL, NULL),
	(1368, 'p', 'SYS_ADMIN', 'api/v1/mobile/push/subscribe', 'allow', NULL, NULL, NULL),
	(1369, 'p', 'TENANT_ADMIN', 'api/v1/device/shadow/:deviceId/:msgId/ack', 'allow', NULL, NULL, NULL),
	(1370, 'p', 'SYS_ADMIN', 'api/v1/mobile/capabilities', 'allow', NULL, NULL, NULL),
	(1371, 'p', 'SYS_ADMIN', 'api/v1/scada/documents/:id/rollback', 'allow', NULL, NULL, NULL),
	(1372, 'p', 'SYS_ADMIN', 'api/v1/scada/control/confirm', 'allow', NULL, NULL, NULL),
	(1373, 'p', 'TENANT_ADMIN', 'api/v1/scada/control/confirm', 'allow', NULL, NULL, NULL),
	(1374, 'p', 'SYS_ADMIN', 'api/v1/scada/control', 'allow', NULL, NULL, NULL),
	(1375, 'p', 'TENANT_ADMIN', 'api/v1/scada/control', 'allow', NULL, NULL, NULL),
	(1376, 'p', 'SYS_ADMIN', 'api/v1/command/datas/jobs/:job_id/pause', 'allow', NULL, NULL, NULL),
	(1377, 'p', 'TENANT_ADMIN', 'api/v1/command/datas/jobs/:job_id/pause', 'allow', NULL, NULL, NULL),
	(1378, 'p', 'SYS_ADMIN', 'api/v1/command/datas/jobs/:job_id/resume', 'allow', NULL, NULL, NULL),
	(1379, 'p', 'TENANT_ADMIN', 'api/v1/command/datas/jobs/:job_id/resume', 'allow', NULL, NULL, NULL),
	(1380, 'p', 'SYS_ADMIN', 'api/v1/command/datas/jobs/:job_id/rollback', 'allow', NULL, NULL, NULL),
	(1381, 'p', 'TENANT_ADMIN', 'api/v1/command/datas/jobs/:job_id/rollback', 'allow', NULL, NULL, NULL),
	(1382, 'p', 'SYS_ADMIN', 'api/v1/command/datas/jobs/:job_id/progress', 'allow', NULL, NULL, NULL),
	(1383, 'p', 'TENANT_ADMIN', 'api/v1/command/datas/jobs/:job_id/progress', 'allow', NULL, NULL, NULL),
	(1384, 'g2', 'api/v1/device/preRegister/credentials/grants', 'api/v1/device/preRegister/credentials/grants', NULL, NULL, NULL, NULL),
	(1385, 'g2', 'api/v1/device/preRegister/credentials/grants/:id/download', 'api/v1/device/preRegister/credentials/grants/:id/download', NULL, NULL, NULL, NULL),
	(1386, 'p', 'SYS_ADMIN', 'api/v1/device/preRegister/credentials/grants', 'allow', NULL, NULL, NULL),
	(1387, 'p', 'TENANT_ADMIN', 'api/v1/device/preRegister/credentials/grants', 'allow', NULL, NULL, NULL),
	(1388, 'p', 'SYS_ADMIN', 'api/v1/device/preRegister/credentials/grants/:id/download', 'allow', NULL, NULL, NULL),
	(1389, 'p', 'TENANT_ADMIN', 'api/v1/device/preRegister/credentials/grants/:id/download', 'allow', NULL, NULL, NULL),
	(1390, 'g2', 'api/v1/telemetry/analysis', 'api/v1/telemetry/analysis', NULL, NULL, NULL, NULL),
	(1391, 'g2', 'api/v1/telemetry/analysis/export', 'api/v1/telemetry/analysis/export', NULL, NULL, NULL, NULL),
	(1392, 'p', 'SYS_ADMIN', 'api/v1/telemetry/analysis', 'allow', NULL, NULL, NULL),
	(1393, 'p', 'SYS_ADMIN', 'api/v1/telemetry/analysis/export', 'allow', NULL, NULL, NULL),
	(1394, 'p', 'TENANT_ADMIN', 'api/v1/telemetry/analysis', 'allow', NULL, NULL, NULL),
	(1395, 'p', 'TENANT_ADMIN', 'api/v1/telemetry/analysis/export', 'allow', NULL, NULL, NULL),
	(1396, 'p', 'TENANT_USER', 'api/v1/telemetry/analysis', 'allow', NULL, NULL, NULL),
	(1397, 'p', 'TENANT_USER', 'api/v1/telemetry/analysis/export', 'allow', NULL, NULL, NULL),
	(1398, 'g2', 'api/v1/edge/nodes', 'api/v1/edge/nodes', NULL, NULL, NULL, NULL),
	(1399, 'g2', 'api/v1/edge/nodes/:node_id/heartbeat', 'api/v1/edge/nodes/:node_id/heartbeat', NULL, NULL, NULL, NULL),
	(1400, 'g2', 'api/v1/edge/nodes/:node_id/reconcile', 'api/v1/edge/nodes/:node_id/reconcile', NULL, NULL, NULL, NULL),
	(1401, 'g2', 'api/v1/device/template/market/bundle/import', 'api/v1/device/template/market/bundle/import', NULL, NULL, NULL, NULL),
	(1402, 'p', 'SYS_ADMIN', 'api/v1/edge/nodes', 'allow', NULL, NULL, NULL),
	(1403, 'p', 'TENANT_ADMIN', 'api/v1/edge/nodes', 'allow', NULL, NULL, NULL),
	(1404, 'p', 'SYS_ADMIN', 'api/v1/edge/nodes/:node_id/heartbeat', 'allow', NULL, NULL, NULL),
	(1405, 'p', 'TENANT_ADMIN', 'api/v1/edge/nodes/:node_id/heartbeat', 'allow', NULL, NULL, NULL),
	(1406, 'p', 'SYS_ADMIN', 'api/v1/edge/nodes/:node_id/reconcile', 'allow', NULL, NULL, NULL),
	(1407, 'p', 'TENANT_ADMIN', 'api/v1/edge/nodes/:node_id/reconcile', 'allow', NULL, NULL, NULL),
	(1408, 'p', 'SYS_ADMIN', 'api/v1/device/template/market/bundle/import', 'allow', NULL, NULL, NULL),
	(1409, 'p', 'TENANT_ADMIN', 'api/v1/device/template/market/bundle/import', 'allow', NULL, NULL, NULL),
	(1410, 'g2', 'api/v1/telemetry/analysis/anomaly', 'api/v1/telemetry/analysis/anomaly', NULL, NULL, NULL, NULL),
	(1411, 'p', 'SYS_ADMIN', 'api/v1/telemetry/analysis/anomaly', 'allow', NULL, NULL, NULL),
	(1412, 'p', 'TENANT_ADMIN', 'api/v1/telemetry/analysis/anomaly', 'allow', NULL, NULL, NULL),
	(1413, 'p', 'TENANT_USER', 'api/v1/telemetry/analysis/anomaly', 'allow', NULL, NULL, NULL),
	(1414, 'g2', 'api/v1/license/status', 'api/v1/license/status', NULL, NULL, NULL, NULL),
	(1415, 'p', 'SYS_ADMIN', 'api/v1/license/status', 'allow', NULL, NULL, NULL),
	(1416, 'g2', 'api/v1/board/projects', 'api/v1/board/projects', NULL, NULL, NULL, NULL),
	(1417, 'g2', 'api/v1/board/projects/:id', 'api/v1/board/projects/:id', NULL, NULL, NULL, NULL),
	(1418, 'g2', 'api/v1/board/projects/:id/boards/:board_id', 'api/v1/board/projects/:id/boards/:board_id', NULL, NULL, NULL, NULL),
	(1419, 'g2', 'api/v1/board/projects/member-of/:board_id', 'api/v1/board/projects/member-of/:board_id', NULL, NULL, NULL, NULL),
	(1420, 'p', 'SYS_ADMIN', 'api/v1/board/projects', 'allow', NULL, NULL, NULL),
	(1421, 'p', 'TENANT_ADMIN', 'api/v1/board/projects', 'allow', NULL, NULL, NULL),
	(1422, 'p', 'TENANT_USER', 'api/v1/board/projects', 'allow', NULL, NULL, NULL),
	(1423, 'p', 'SYS_ADMIN', 'api/v1/board/projects/member-of/:board_id', 'allow', NULL, NULL, NULL),
	(1424, 'p', 'TENANT_ADMIN', 'api/v1/board/projects/member-of/:board_id', 'allow', NULL, NULL, NULL),
	(1425, 'p', 'TENANT_USER', 'api/v1/board/projects/member-of/:board_id', 'allow', NULL, NULL, NULL),
	(1426, 'p', 'SYS_ADMIN', 'api/v1/board/projects/:id', 'allow', NULL, NULL, NULL),
	(1427, 'p', 'TENANT_ADMIN', 'api/v1/board/projects/:id', 'allow', NULL, NULL, NULL),
	(1428, 'p', 'TENANT_USER', 'api/v1/board/projects/:id', 'allow', NULL, NULL, NULL),
	(1429, 'p', 'SYS_ADMIN', 'api/v1/board/projects/:id/boards/:board_id', 'allow', NULL, NULL, NULL),
	(1430, 'p', 'TENANT_ADMIN', 'api/v1/board/projects/:id/boards/:board_id', 'allow', NULL, NULL, NULL),
	(1431, 'g2', 'api/v1/operation_logs/export', 'api/v1/operation_logs/export', NULL, NULL, NULL, NULL),
	(1432, 'p', 'SYS_ADMIN', 'api/v1/operation_logs/export', 'allow', NULL, NULL, NULL),
	(1433, 'p', 'TENANT_ADMIN', 'api/v1/operation_logs/export', 'allow', NULL, NULL, NULL),
	(1434, 'p', 'TENANT_USER', 'api/v1/operation_logs/export', 'allow', NULL, NULL, NULL),
	(1435, 'g2', 'api/v1/device/template/upgrade', 'api/v1/device/template/upgrade', NULL, NULL, NULL, NULL),
	(1436, 'g2', 'api/v1/device/template/upgrade/:history_id/rollback', 'api/v1/device/template/upgrade/:history_id/rollback', NULL, NULL, NULL, NULL),
	(1437, 'g2', 'api/v1/device/template/upgrade/history', 'api/v1/device/template/upgrade/history', NULL, NULL, NULL, NULL),
	(1438, 'p', 'SYS_ADMIN', 'api/v1/device/template/upgrade', 'allow', NULL, NULL, NULL),
	(1439, 'p', 'TENANT_ADMIN', 'api/v1/device/template/upgrade', 'allow', NULL, NULL, NULL),
	(1440, 'p', 'SYS_ADMIN', 'api/v1/device/template/upgrade/:history_id/rollback', 'allow', NULL, NULL, NULL),
	(1441, 'p', 'TENANT_ADMIN', 'api/v1/device/template/upgrade/:history_id/rollback', 'allow', NULL, NULL, NULL),
	(1442, 'p', 'SYS_ADMIN', 'api/v1/device/template/upgrade/history', 'allow', NULL, NULL, NULL),
	(1443, 'p', 'TENANT_ADMIN', 'api/v1/device/template/upgrade/history', 'allow', NULL, NULL, NULL),
	(1444, 'p', 'TENANT_USER', 'api/v1/device/template/upgrade/history', 'allow', NULL, NULL, NULL),
	(1445, 'g2', 'api/v1/alarm/info/history/:id/comment', 'api/v1/alarm/info/history/:id/comment', NULL, NULL, NULL, NULL),
	(1446, 'g2', 'api/v1/alarm/info/history/:id/comment/:comment_id', 'api/v1/alarm/info/history/:id/comment/:comment_id', NULL, NULL, NULL, NULL),
	(1447, 'p', 'SYS_ADMIN', 'api/v1/alarm/info/history/:id/comment', 'allow', NULL, NULL, NULL),
	(1448, 'p', 'TENANT_ADMIN', 'api/v1/alarm/info/history/:id/comment', 'allow', NULL, NULL, NULL),
	(1449, 'p', 'SYS_ADMIN', 'api/v1/alarm/info/history/:id/comment/:comment_id', 'allow', NULL, NULL, NULL),
	(1450, 'p', 'TENANT_ADMIN', 'api/v1/alarm/info/history/:id/comment/:comment_id', 'allow', NULL, NULL, NULL);
INSERT INTO public.casbin_rule (id, ptype, v0, v1, v2, v3, v4, v5) VALUES
	(1451, 'g2', 'api/v1/alarm/info/history/:id/assignment', 'api/v1/alarm/info/history/:id/assignment', NULL, NULL, NULL, NULL),
	(1452, 'p', 'SYS_ADMIN', 'api/v1/alarm/info/history/:id/assignment', 'allow', NULL, NULL, NULL),
	(1453, 'p', 'TENANT_ADMIN', 'api/v1/alarm/info/history/:id/assignment', 'allow', NULL, NULL, NULL),
	(1454, 'g2', 'api/v1/edge/nodes/:node_id/certificate', 'api/v1/edge/nodes/:node_id/certificate', NULL, NULL, NULL, NULL),
	(1455, 'g2', 'api/v1/edge/nodes/:node_id/upgrade', 'api/v1/edge/nodes/:node_id/upgrade', NULL, NULL, NULL, NULL),
	(1456, 'g2', 'api/v1/edge/nodes/:node_id/rollback', 'api/v1/edge/nodes/:node_id/rollback', NULL, NULL, NULL, NULL),
	(1457, 'g2', 'api/v1/edge/nodes/:node_id/upgrade/history', 'api/v1/edge/nodes/:node_id/upgrade/history', NULL, NULL, NULL, NULL),
	(1458, 'p', 'SYS_ADMIN', 'api/v1/edge/nodes/:node_id/certificate', 'allow', NULL, NULL, NULL),
	(1459, 'p', 'TENANT_ADMIN', 'api/v1/edge/nodes/:node_id/certificate', 'allow', NULL, NULL, NULL),
	(1460, 'p', 'SYS_ADMIN', 'api/v1/edge/nodes/:node_id/upgrade', 'allow', NULL, NULL, NULL),
	(1461, 'p', 'TENANT_ADMIN', 'api/v1/edge/nodes/:node_id/upgrade', 'allow', NULL, NULL, NULL),
	(1462, 'p', 'SYS_ADMIN', 'api/v1/edge/nodes/:node_id/rollback', 'allow', NULL, NULL, NULL),
	(1463, 'p', 'TENANT_ADMIN', 'api/v1/edge/nodes/:node_id/rollback', 'allow', NULL, NULL, NULL),
	(1464, 'p', 'SYS_ADMIN', 'api/v1/edge/nodes/:node_id/upgrade/history', 'allow', NULL, NULL, NULL),
	(1465, 'p', 'TENANT_ADMIN', 'api/v1/edge/nodes/:node_id/upgrade/history', 'allow', NULL, NULL, NULL),
	(1466, 'g2', 'api/v1/alarm/info/history/:id/clear', 'api/v1/alarm/info/history/:id/clear', NULL, NULL, NULL, NULL),
	(1467, 'p', 'SYS_ADMIN', 'api/v1/alarm/info/history/:id/clear', 'allow', NULL, NULL, NULL),
	(1468, 'p', 'TENANT_ADMIN', 'api/v1/alarm/info/history/:id/clear', 'allow', NULL, NULL, NULL),
	(1469, 'g2', 'api/v1/resource/center/catalog', 'api/v1/resource/center/catalog', NULL, NULL, NULL, NULL),
	(1470, 'g2', 'api/v1/resource/center/list', 'api/v1/resource/center/list', NULL, NULL, NULL, NULL),
	(1471, 'g2', 'api/v1/resource/center/detail/:type/:id', 'api/v1/resource/center/detail/:type/:id', NULL, NULL, NULL, NULL),
	(1472, 'g2', 'api/v1/resource/center/bundle', 'api/v1/resource/center/bundle', NULL, NULL, NULL, NULL),
	(1473, 'g2', 'api/v1/resource/center/bundle/import', 'api/v1/resource/center/bundle/import', NULL, NULL, NULL, NULL),
	(1474, 'g2', 'api/v1/resource/center/apply', 'api/v1/resource/center/apply', NULL, NULL, NULL, NULL),
	(1475, 'g2', 'api/v1/board/export/:id', 'api/v1/board/export/:id', NULL, NULL, NULL, NULL),
	(1476, 'g2', 'api/v1/board/import', 'api/v1/board/import', NULL, NULL, NULL, NULL),
	(1477, 'p', 'SYS_ADMIN', 'api/v1/resource/center/catalog', 'allow', NULL, NULL, NULL),
	(1478, 'p', 'SYS_ADMIN', 'api/v1/resource/center/list', 'allow', NULL, NULL, NULL),
	(1479, 'p', 'SYS_ADMIN', 'api/v1/resource/center/detail/:type/:id', 'allow', NULL, NULL, NULL),
	(1480, 'p', 'SYS_ADMIN', 'api/v1/resource/center/bundle', 'allow', NULL, NULL, NULL),
	(1481, 'p', 'SYS_ADMIN', 'api/v1/resource/center/bundle/import', 'allow', NULL, NULL, NULL),
	(1482, 'p', 'SYS_ADMIN', 'api/v1/resource/center/apply', 'allow', NULL, NULL, NULL),
	(1483, 'p', 'SYS_ADMIN', 'api/v1/board/export/:id', 'allow', NULL, NULL, NULL),
	(1484, 'p', 'SYS_ADMIN', 'api/v1/board/import', 'allow', NULL, NULL, NULL),
	(1485, 'p', 'TENANT_ADMIN', 'api/v1/resource/center/catalog', 'allow', NULL, NULL, NULL),
	(1486, 'p', 'TENANT_ADMIN', 'api/v1/resource/center/list', 'allow', NULL, NULL, NULL),
	(1487, 'p', 'TENANT_ADMIN', 'api/v1/resource/center/detail/:type/:id', 'allow', NULL, NULL, NULL),
	(1488, 'p', 'TENANT_ADMIN', 'api/v1/resource/center/bundle', 'allow', NULL, NULL, NULL),
	(1489, 'p', 'TENANT_ADMIN', 'api/v1/resource/center/bundle/import', 'allow', NULL, NULL, NULL),
	(1490, 'p', 'TENANT_ADMIN', 'api/v1/resource/center/apply', 'allow', NULL, NULL, NULL),
	(1491, 'p', 'TENANT_ADMIN', 'api/v1/board/export/:id', 'allow', NULL, NULL, NULL),
	(1492, 'p', 'TENANT_ADMIN', 'api/v1/board/import', 'allow', NULL, NULL, NULL),
	(1493, 'p', 'TENANT_USER', 'api/v1/resource/center/catalog', 'allow', NULL, NULL, NULL),
	(1494, 'p', 'TENANT_USER', 'api/v1/resource/center/list', 'allow', NULL, NULL, NULL),
	(1495, 'p', 'TENANT_USER', 'api/v1/resource/center/detail/:type/:id', 'allow', NULL, NULL, NULL),
	(1496, 'g2', 'api/v1/ratelimit/config', 'api/v1/ratelimit/config', NULL, NULL, NULL, NULL),
	(1497, 'g2', 'api/v1/ratelimit/metrics', 'api/v1/ratelimit/metrics', NULL, NULL, NULL, NULL),
	(1498, 'g2', 'api/v1/ratelimit/overrides', 'api/v1/ratelimit/overrides', NULL, NULL, NULL, NULL),
	(1499, 'g2', 'api/v1/ratelimit/override', 'api/v1/ratelimit/override', NULL, NULL, NULL, NULL),
	(1500, 'g2', 'api/v1/ratelimit/override/:target_type/:target_id', 'api/v1/ratelimit/override/:target_type/:target_id', NULL, NULL, NULL, NULL),
	(1501, 'g2', 'api/v1/queue/stats', 'api/v1/queue/stats', NULL, NULL, NULL, NULL),
	(1502, 'g2', 'api/v1/queue/config', 'api/v1/queue/config', NULL, NULL, NULL, NULL),
	(1503, 'p', 'SYS_ADMIN', 'api/v1/ratelimit/config', 'allow', NULL, NULL, NULL),
	(1504, 'p', 'SYS_ADMIN', 'api/v1/ratelimit/metrics', 'allow', NULL, NULL, NULL),
	(1505, 'p', 'SYS_ADMIN', 'api/v1/ratelimit/overrides', 'allow', NULL, NULL, NULL),
	(1506, 'p', 'SYS_ADMIN', 'api/v1/ratelimit/override', 'allow', NULL, NULL, NULL),
	(1507, 'p', 'SYS_ADMIN', 'api/v1/ratelimit/override/:target_type/:target_id', 'allow', NULL, NULL, NULL),
	(1508, 'p', 'SYS_ADMIN', 'api/v1/queue/stats', 'allow', NULL, NULL, NULL),
	(1509, 'p', 'SYS_ADMIN', 'api/v1/queue/config', 'allow', NULL, NULL, NULL),
	(1510, 'p', 'TENANT_ADMIN', 'api/v1/ratelimit/config', 'allow', NULL, NULL, NULL),
	(1511, 'p', 'TENANT_ADMIN', 'api/v1/ratelimit/metrics', 'allow', NULL, NULL, NULL),
	(1512, 'p', 'TENANT_ADMIN', 'api/v1/ratelimit/overrides', 'allow', NULL, NULL, NULL),
	(1513, 'p', 'TENANT_ADMIN', 'api/v1/ratelimit/override', 'allow', NULL, NULL, NULL),
	(1514, 'p', 'TENANT_ADMIN', 'api/v1/ratelimit/override/:target_type/:target_id', 'allow', NULL, NULL, NULL),
	(1515, 'p', 'TENANT_ADMIN', 'api/v1/queue/stats', 'allow', NULL, NULL, NULL),
	(1516, 'p', 'TENANT_ADMIN', 'api/v1/queue/config', 'allow', NULL, NULL, NULL),
	(1517, 'g2', 'api/v1/units/registry', 'api/v1/units/registry', NULL, NULL, NULL, NULL),
	(1518, 'g2', 'api/v1/units/convert', 'api/v1/units/convert', NULL, NULL, NULL, NULL),
	(1519, 'p', 'SYS_ADMIN', 'api/v1/units/registry', 'allow', NULL, NULL, NULL),
	(1520, 'p', 'SYS_ADMIN', 'api/v1/units/convert', 'allow', NULL, NULL, NULL),
	(1521, 'p', 'TENANT_ADMIN', 'api/v1/units/registry', 'allow', NULL, NULL, NULL),
	(1522, 'p', 'TENANT_ADMIN', 'api/v1/units/convert', 'allow', NULL, NULL, NULL),
	(1523, 'p', 'TENANT_USER', 'api/v1/units/registry', 'allow', NULL, NULL, NULL),
	(1524, 'p', 'TENANT_USER', 'api/v1/units/convert', 'allow', NULL, NULL, NULL),
	(1525, 'g2', 'api/v1/secrets', 'api/v1/secrets', NULL, NULL, NULL, NULL),
	(1526, 'g2', 'api/v1/secrets/:id', 'api/v1/secrets/:id', NULL, NULL, NULL, NULL),
	(1527, 'g2', 'api/v1/secrets/:id/reveal', 'api/v1/secrets/:id/reveal', NULL, NULL, NULL, NULL),
	(1528, 'g2', 'api/v1/secrets/:id/reseal', 'api/v1/secrets/:id/reseal', NULL, NULL, NULL, NULL),
	(1529, 'p', 'SYS_ADMIN', 'api/v1/secrets', 'allow', NULL, NULL, NULL),
	(1530, 'p', 'SYS_ADMIN', 'api/v1/secrets/:id', 'allow', NULL, NULL, NULL),
	(1531, 'p', 'SYS_ADMIN', 'api/v1/secrets/:id/reveal', 'allow', NULL, NULL, NULL),
	(1532, 'p', 'SYS_ADMIN', 'api/v1/secrets/:id/reseal', 'allow', NULL, NULL, NULL),
	(1533, 'p', 'TENANT_ADMIN', 'api/v1/secrets', 'allow', NULL, NULL, NULL),
	(1534, 'p', 'TENANT_ADMIN', 'api/v1/secrets/:id', 'allow', NULL, NULL, NULL),
	(1535, 'p', 'TENANT_ADMIN', 'api/v1/secrets/:id/reveal', 'allow', NULL, NULL, NULL),
	(1536, 'p', 'TENANT_ADMIN', 'api/v1/secrets/:id/reseal', 'allow', NULL, NULL, NULL),
	(1537, 'p', 'TENANT_USER', 'api/v1/secrets', 'allow', NULL, NULL, NULL),
	(1538, 'p', 'TENANT_USER', 'api/v1/secrets/:id', 'allow', NULL, NULL, NULL),
	(1539, 'g2', 'api/v1/product/:id', 'api/v1/product/:id', NULL, NULL, NULL, NULL),
	(1540, 'p', 'SYS_ADMIN', 'api/v1/product/:id', 'allow', NULL, NULL, NULL),
	(1541, 'p', 'TENANT_ADMIN', 'api/v1/product/:id', 'allow', NULL, NULL, NULL),
	(1542, 'p', 'TENANT_USER', 'api/v1/product/:id', 'allow', NULL, NULL, NULL),
	(1543, 'g2', 'api/v1/rule-chains/:id/dead-letters', 'api/v1/rule-chains/:id/dead-letters', NULL, NULL, NULL, NULL),
	(1544, 'g2', 'api/v1/rule-chains/dead-letters', 'api/v1/rule-chains/dead-letters', NULL, NULL, NULL, NULL),
	(1545, 'g2', 'api/v1/rule-chains/:id/executions/:execId/traces', 'api/v1/rule-chains/:id/executions/:execId/traces', NULL, NULL, NULL, NULL),
	(1546, 'g2', 'api/v1/rule-chains/executions/:execId/traces', 'api/v1/rule-chains/executions/:execId/traces', NULL, NULL, NULL, NULL),
	(1547, 'g2', 'api/v1/rule-chains/:id/executions/:execId/replay-records', 'api/v1/rule-chains/:id/executions/:execId/replay-records', NULL, NULL, NULL, NULL),
	(1548, 'g2', 'api/v1/rule-chains/:id/replay', 'api/v1/rule-chains/:id/replay', NULL, NULL, NULL, NULL),
	(1549, 'p', 'SYS_ADMIN', 'api/v1/rule-chains/:id/dead-letters', 'allow', NULL, NULL, NULL),
	(1550, 'p', 'TENANT_ADMIN', 'api/v1/rule-chains/:id/dead-letters', 'allow', NULL, NULL, NULL),
	(1551, 'p', 'TENANT_USER', 'api/v1/rule-chains/:id/dead-letters', 'allow', NULL, NULL, NULL),
	(1552, 'p', 'SYS_ADMIN', 'api/v1/rule-chains/dead-letters', 'allow', NULL, NULL, NULL),
	(1553, 'p', 'TENANT_ADMIN', 'api/v1/rule-chains/dead-letters', 'allow', NULL, NULL, NULL),
	(1554, 'p', 'TENANT_USER', 'api/v1/rule-chains/dead-letters', 'allow', NULL, NULL, NULL),
	(1555, 'p', 'SYS_ADMIN', 'api/v1/rule-chains/:id/executions/:execId/traces', 'allow', NULL, NULL, NULL),
	(1556, 'p', 'TENANT_ADMIN', 'api/v1/rule-chains/:id/executions/:execId/traces', 'allow', NULL, NULL, NULL),
	(1557, 'p', 'TENANT_USER', 'api/v1/rule-chains/:id/executions/:execId/traces', 'allow', NULL, NULL, NULL),
	(1558, 'p', 'SYS_ADMIN', 'api/v1/rule-chains/executions/:execId/traces', 'allow', NULL, NULL, NULL),
	(1559, 'p', 'TENANT_ADMIN', 'api/v1/rule-chains/executions/:execId/traces', 'allow', NULL, NULL, NULL),
	(1560, 'p', 'TENANT_USER', 'api/v1/rule-chains/executions/:execId/traces', 'allow', NULL, NULL, NULL),
	(1561, 'p', 'SYS_ADMIN', 'api/v1/rule-chains/:id/executions/:execId/replay-records', 'allow', NULL, NULL, NULL),
	(1562, 'p', 'TENANT_ADMIN', 'api/v1/rule-chains/:id/executions/:execId/replay-records', 'allow', NULL, NULL, NULL),
	(1563, 'p', 'TENANT_USER', 'api/v1/rule-chains/:id/executions/:execId/replay-records', 'allow', NULL, NULL, NULL),
	(1564, 'p', 'SYS_ADMIN', 'api/v1/rule-chains/:id/replay', 'allow', NULL, NULL, NULL),
	(1565, 'p', 'TENANT_ADMIN', 'api/v1/rule-chains/:id/replay', 'allow', NULL, NULL, NULL),
	(1566, 'g2', 'api/v1/device/claim-tokens', 'api/v1/device/claim-tokens', NULL, NULL, NULL, NULL),
	(1567, 'g2', 'api/v1/device/claim-tokens/:token_id', 'api/v1/device/claim-tokens/:token_id', NULL, NULL, NULL, NULL),
	(1568, 'g2', 'api/v1/device/claim-tokens/redeem', 'api/v1/device/claim-tokens/redeem', NULL, NULL, NULL, NULL),
	(1569, 'p', 'SYS_ADMIN', 'api/v1/device/claim-tokens', 'allow', NULL, NULL, NULL),
	(1570, 'p', 'TENANT_ADMIN', 'api/v1/device/claim-tokens', 'allow', NULL, NULL, NULL),
	(1571, 'p', 'SYS_ADMIN', 'api/v1/device/claim-tokens/:token_id', 'allow', NULL, NULL, NULL),
	(1572, 'p', 'TENANT_ADMIN', 'api/v1/device/claim-tokens/:token_id', 'allow', NULL, NULL, NULL),
	(1573, 'p', 'SYS_ADMIN', 'api/v1/device/claim-tokens/redeem', 'allow', NULL, NULL, NULL),
	(1574, 'p', 'TENANT_ADMIN', 'api/v1/device/claim-tokens/redeem', 'allow', NULL, NULL, NULL),
	(1575, 'g2', 'api/v1/solutions', 'api/v1/solutions', NULL, NULL, NULL, NULL),
	(1576, 'g2', 'api/v1/solutions/:id', 'api/v1/solutions/:id', NULL, NULL, NULL, NULL),
	(1577, 'g2', 'api/v1/solutions/:id/install', 'api/v1/solutions/:id/install', NULL, NULL, NULL, NULL),
	(1578, 'p', 'SYS_ADMIN', 'api/v1/solutions', 'allow', NULL, NULL, NULL),
	(1579, 'p', 'TENANT_ADMIN', 'api/v1/solutions', 'allow', NULL, NULL, NULL),
	(1580, 'p', 'SYS_ADMIN', 'api/v1/solutions/:id', 'allow', NULL, NULL, NULL),
	(1581, 'p', 'TENANT_ADMIN', 'api/v1/solutions/:id', 'allow', NULL, NULL, NULL),
	(1582, 'p', 'SYS_ADMIN', 'api/v1/solutions/:id/install', 'allow', NULL, NULL, NULL),
	(1583, 'p', 'TENANT_ADMIN', 'api/v1/solutions/:id/install', 'allow', NULL, NULL, NULL),
	(1584, 'g2', 'api/v1/tenants', 'api/v1/tenants', NULL, NULL, NULL, NULL),
	(1585, 'g2', 'api/v1/tenants/:id', 'api/v1/tenants/:id', NULL, NULL, NULL, NULL),
	(1586, 'p', 'SYS_ADMIN', 'api/v1/tenants', 'allow', NULL, NULL, NULL),
	(1587, 'p', 'TENANT_ADMIN', 'api/v1/tenants', 'allow', NULL, NULL, NULL),
	(1588, 'p', 'SYS_ADMIN', 'api/v1/tenants/:id', 'allow', NULL, NULL, NULL),
	(1589, 'p', 'TENANT_ADMIN', 'api/v1/tenants/:id', 'allow', NULL, NULL, NULL),
	(1590, 'g2', 'api/v1/billing/plans', 'api/v1/billing/plans', NULL, NULL, NULL, NULL),
	(1591, 'g2', 'api/v1/billing/usage', 'api/v1/billing/usage', NULL, NULL, NULL, NULL),
	(1592, 'g2', 'api/v1/billing/subscriptions', 'api/v1/billing/subscriptions', NULL, NULL, NULL, NULL),
	(1593, 'p', 'SYS_ADMIN', 'api/v1/billing/plans', 'allow', NULL, NULL, NULL),
	(1594, 'p', 'TENANT_ADMIN', 'api/v1/billing/plans', 'allow', NULL, NULL, NULL),
	(1595, 'p', 'TENANT_USER', 'api/v1/billing/plans', 'allow', NULL, NULL, NULL),
	(1596, 'p', 'SYS_ADMIN', 'api/v1/billing/usage', 'allow', NULL, NULL, NULL),
	(1597, 'p', 'TENANT_ADMIN', 'api/v1/billing/usage', 'allow', NULL, NULL, NULL),
	(1598, 'p', 'SYS_ADMIN', 'api/v1/billing/subscriptions', 'allow', NULL, NULL, NULL),
	(1599, 'p', 'TENANT_ADMIN', 'api/v1/billing/subscriptions', 'allow', NULL, NULL, NULL),
	(1600, 'g2', 'api/v1/permissions', 'api/v1/permissions', NULL, NULL, NULL, NULL),
	(1601, 'g2', 'api/v1/role/:id/permissions', 'api/v1/role/:id/permissions', NULL, NULL, NULL, NULL),
	(1602, 'g2', 'api/v1/role/:id/users', 'api/v1/role/:id/users', NULL, NULL, NULL, NULL),
	(1603, 'g2', 'api/v1/roles/:id/permissions', 'api/v1/roles/:id/permissions', NULL, NULL, NULL, NULL),
	(1604, 'g2', 'api/v1/roles/:id/users', 'api/v1/roles/:id/users', NULL, NULL, NULL, NULL),
	(1605, 'p', 'SYS_ADMIN', 'api/v1/permissions', 'allow', NULL, NULL, NULL),
	(1606, 'p', 'TENANT_ADMIN', 'api/v1/permissions', 'allow', NULL, NULL, NULL),
	(1607, 'p', 'SYS_ADMIN', 'api/v1/role/:id/permissions', 'allow', NULL, NULL, NULL),
	(1608, 'p', 'TENANT_ADMIN', 'api/v1/role/:id/permissions', 'allow', NULL, NULL, NULL),
	(1609, 'p', 'SYS_ADMIN', 'api/v1/role/:id/users', 'allow', NULL, NULL, NULL),
	(1610, 'p', 'TENANT_ADMIN', 'api/v1/role/:id/users', 'allow', NULL, NULL, NULL),
	(1611, 'p', 'SYS_ADMIN', 'api/v1/roles/:id/permissions', 'allow', NULL, NULL, NULL),
	(1612, 'p', 'TENANT_ADMIN', 'api/v1/roles/:id/permissions', 'allow', NULL, NULL, NULL),
	(1613, 'p', 'SYS_ADMIN', 'api/v1/roles/:id/users', 'allow', NULL, NULL, NULL),
	(1614, 'p', 'TENANT_ADMIN', 'api/v1/roles/:id/users', 'allow', NULL, NULL, NULL),
	(1615, 'g2', 'api/v1/devices/locations/latest', 'api/v1/devices/locations/latest', NULL, NULL, NULL, NULL),
	(1616, 'g2', 'api/v1/device/locations/latest', 'api/v1/device/locations/latest', NULL, NULL, NULL, NULL),
	(1617, 'g2', 'api/v1/devices/:device_id/location/history', 'api/v1/devices/:device_id/location/history', NULL, NULL, NULL, NULL),
	(1618, 'g2', 'api/v1/devices/:id/location/history', 'api/v1/devices/:id/location/history', NULL, NULL, NULL, NULL),
	(1619, 'g2', 'api/v1/device/:id/location/history', 'api/v1/device/:id/location/history', NULL, NULL, NULL, NULL),
	(1620, 'p', 'SYS_ADMIN', 'api/v1/devices/locations/latest', 'allow', NULL, NULL, NULL),
	(1621, 'p', 'SYS_ADMIN', 'api/v1/device/locations/latest', 'allow', NULL, NULL, NULL),
	(1622, 'p', 'TENANT_ADMIN', 'api/v1/devices/locations/latest', 'allow', NULL, NULL, NULL),
	(1623, 'p', 'TENANT_ADMIN', 'api/v1/device/locations/latest', 'allow', NULL, NULL, NULL),
	(1624, 'p', 'TENANT_USER', 'api/v1/devices/locations/latest', 'allow', NULL, NULL, NULL),
	(1625, 'p', 'TENANT_USER', 'api/v1/device/locations/latest', 'allow', NULL, NULL, NULL),
	(1626, 'p', 'SYS_ADMIN', 'api/v1/devices/:device_id/location/history', 'allow', NULL, NULL, NULL),
	(1627, 'p', 'SYS_ADMIN', 'api/v1/devices/:id/location/history', 'allow', NULL, NULL, NULL),
	(1628, 'p', 'SYS_ADMIN', 'api/v1/device/:id/location/history', 'allow', NULL, NULL, NULL),
	(1629, 'p', 'TENANT_ADMIN', 'api/v1/devices/:device_id/location/history', 'allow', NULL, NULL, NULL),
	(1630, 'p', 'TENANT_ADMIN', 'api/v1/devices/:id/location/history', 'allow', NULL, NULL, NULL),
	(1631, 'p', 'TENANT_ADMIN', 'api/v1/device/:id/location/history', 'allow', NULL, NULL, NULL),
	(1632, 'p', 'TENANT_USER', 'api/v1/devices/:device_id/location/history', 'allow', NULL, NULL, NULL),
	(1633, 'p', 'TENANT_USER', 'api/v1/devices/:id/location/history', 'allow', NULL, NULL, NULL),
	(1634, 'p', 'TENANT_USER', 'api/v1/device/:id/location/history', 'allow', NULL, NULL, NULL),
	(1635, 'g2', 'api/v1/converters', 'api/v1/converters', NULL, NULL, NULL, NULL),
	(1636, 'g2', 'api/v1/converters/:id', 'api/v1/converters/:id', NULL, NULL, NULL, NULL),
	(1637, 'g2', 'api/v1/converters/test', 'api/v1/converters/test', NULL, NULL, NULL, NULL),
	(1638, 'g2', 'api/v1/data-converters', 'api/v1/data-converters', NULL, NULL, NULL, NULL),
	(1639, 'g2', 'api/v1/data-converters/:id', 'api/v1/data-converters/:id', NULL, NULL, NULL, NULL),
	(1640, 'g2', 'api/v1/data-converters/test', 'api/v1/data-converters/test', NULL, NULL, NULL, NULL),
	(1641, 'p', 'SYS_ADMIN', 'api/v1/converters', 'allow', NULL, NULL, NULL),
	(1642, 'p', 'SYS_ADMIN', 'api/v1/converters/:id', 'allow', NULL, NULL, NULL),
	(1643, 'p', 'SYS_ADMIN', 'api/v1/converters/test', 'allow', NULL, NULL, NULL),
	(1644, 'p', 'SYS_ADMIN', 'api/v1/data-converters', 'allow', NULL, NULL, NULL),
	(1645, 'p', 'SYS_ADMIN', 'api/v1/data-converters/:id', 'allow', NULL, NULL, NULL),
	(1646, 'p', 'SYS_ADMIN', 'api/v1/data-converters/test', 'allow', NULL, NULL, NULL),
	(1647, 'p', 'TENANT_ADMIN', 'api/v1/converters', 'allow', NULL, NULL, NULL),
	(1648, 'p', 'TENANT_ADMIN', 'api/v1/converters/:id', 'allow', NULL, NULL, NULL),
	(1649, 'p', 'TENANT_ADMIN', 'api/v1/converters/test', 'allow', NULL, NULL, NULL),
	(1650, 'p', 'TENANT_ADMIN', 'api/v1/data-converters', 'allow', NULL, NULL, NULL);
INSERT INTO public.casbin_rule (id, ptype, v0, v1, v2, v3, v4, v5) VALUES
	(1651, 'p', 'TENANT_ADMIN', 'api/v1/data-converters/:id', 'allow', NULL, NULL, NULL),
	(1652, 'p', 'TENANT_ADMIN', 'api/v1/data-converters/test', 'allow', NULL, NULL, NULL),
	(1653, 'g2', 'api/v1/devices/health/summary', 'api/v1/devices/health/summary', NULL, NULL, NULL, NULL),
	(1654, 'g2', 'api/v1/devices/health/evaluate', 'api/v1/devices/health/evaluate', NULL, NULL, NULL, NULL),
	(1655, 'g2', 'api/v1/devices/:device_id/health', 'api/v1/devices/:device_id/health', NULL, NULL, NULL, NULL),
	(1656, 'g2', 'api/v1/devices/:device_id/health/evaluate', 'api/v1/devices/:device_id/health/evaluate', NULL, NULL, NULL, NULL),
	(1657, 'g2', 'api/v1/device/health/summary', 'api/v1/device/health/summary', NULL, NULL, NULL, NULL),
	(1658, 'g2', 'api/v1/device/health/evaluate', 'api/v1/device/health/evaluate', NULL, NULL, NULL, NULL),
	(1659, 'g2', 'api/v1/device/:id/health', 'api/v1/device/:id/health', NULL, NULL, NULL, NULL),
	(1660, 'g2', 'api/v1/device/:id/health/evaluate', 'api/v1/device/:id/health/evaluate', NULL, NULL, NULL, NULL),
	(1661, 'p', 'SYS_ADMIN', 'api/v1/devices/health/summary', 'allow', NULL, NULL, NULL),
	(1662, 'p', 'SYS_ADMIN', 'api/v1/devices/health/evaluate', 'allow', NULL, NULL, NULL),
	(1663, 'p', 'SYS_ADMIN', 'api/v1/devices/:device_id/health', 'allow', NULL, NULL, NULL),
	(1664, 'p', 'SYS_ADMIN', 'api/v1/devices/:device_id/health/evaluate', 'allow', NULL, NULL, NULL),
	(1665, 'p', 'SYS_ADMIN', 'api/v1/device/health/summary', 'allow', NULL, NULL, NULL),
	(1666, 'p', 'SYS_ADMIN', 'api/v1/device/health/evaluate', 'allow', NULL, NULL, NULL),
	(1667, 'p', 'SYS_ADMIN', 'api/v1/device/:id/health', 'allow', NULL, NULL, NULL),
	(1668, 'p', 'SYS_ADMIN', 'api/v1/device/:id/health/evaluate', 'allow', NULL, NULL, NULL),
	(1669, 'p', 'TENANT_ADMIN', 'api/v1/devices/health/summary', 'allow', NULL, NULL, NULL),
	(1670, 'p', 'TENANT_ADMIN', 'api/v1/devices/health/evaluate', 'allow', NULL, NULL, NULL),
	(1671, 'p', 'TENANT_ADMIN', 'api/v1/devices/:device_id/health', 'allow', NULL, NULL, NULL),
	(1672, 'p', 'TENANT_ADMIN', 'api/v1/devices/:device_id/health/evaluate', 'allow', NULL, NULL, NULL),
	(1673, 'p', 'TENANT_ADMIN', 'api/v1/device/health/summary', 'allow', NULL, NULL, NULL),
	(1674, 'p', 'TENANT_ADMIN', 'api/v1/device/health/evaluate', 'allow', NULL, NULL, NULL),
	(1675, 'p', 'TENANT_ADMIN', 'api/v1/device/:id/health', 'allow', NULL, NULL, NULL),
	(1676, 'p', 'TENANT_ADMIN', 'api/v1/device/:id/health/evaluate', 'allow', NULL, NULL, NULL),
	(1677, 'g2', 'api/v1/customer', 'api/v1/customer', NULL, NULL, NULL, NULL),
	(1678, 'g2', 'api/v1/customer/:id', 'api/v1/customer/:id', NULL, NULL, NULL, NULL),
	(1679, 'g2', 'api/v1/customers', 'api/v1/customers', NULL, NULL, NULL, NULL),
	(1680, 'g2', 'api/v1/customer/:id/devices', 'api/v1/customer/:id/devices', NULL, NULL, NULL, NULL),
	(1681, 'g2', 'api/v1/customer/:id/device/:device_id', 'api/v1/customer/:id/device/:device_id', NULL, NULL, NULL, NULL),
	(1682, 'p', 'SYS_ADMIN', 'api/v1/customer', 'allow', NULL, NULL, NULL),
	(1683, 'p', 'SYS_ADMIN', 'api/v1/customer/:id', 'allow', NULL, NULL, NULL),
	(1684, 'p', 'SYS_ADMIN', 'api/v1/customers', 'allow', NULL, NULL, NULL),
	(1685, 'p', 'SYS_ADMIN', 'api/v1/customer/:id/devices', 'allow', NULL, NULL, NULL),
	(1686, 'p', 'SYS_ADMIN', 'api/v1/customer/:id/device/:device_id', 'allow', NULL, NULL, NULL),
	(1687, 'p', 'TENANT_ADMIN', 'api/v1/customer', 'allow', NULL, NULL, NULL),
	(1688, 'p', 'TENANT_ADMIN', 'api/v1/customer/:id', 'allow', NULL, NULL, NULL),
	(1689, 'p', 'TENANT_ADMIN', 'api/v1/customers', 'allow', NULL, NULL, NULL),
	(1690, 'p', 'TENANT_ADMIN', 'api/v1/customer/:id/devices', 'allow', NULL, NULL, NULL),
	(1691, 'p', 'TENANT_ADMIN', 'api/v1/customer/:id/device/:device_id', 'allow', NULL, NULL, NULL),
	(1692, 'g2', 'api/v1/widget-bundles', 'api/v1/widget-bundles', NULL, NULL, NULL, NULL),
	(1693, 'g2', 'api/v1/widget-bundles/:id', 'api/v1/widget-bundles/:id', NULL, NULL, NULL, NULL),
	(1694, 'g2', 'api/v1/widget-bundles/builtin', 'api/v1/widget-bundles/builtin', NULL, NULL, NULL, NULL),
	(1695, 'g2', 'api/v1/widget-bundles/seed', 'api/v1/widget-bundles/seed', NULL, NULL, NULL, NULL),
	(1696, 'p', 'SYS_ADMIN', 'api/v1/widget-bundles', 'allow', NULL, NULL, NULL),
	(1697, 'p', 'SYS_ADMIN', 'api/v1/widget-bundles/:id', 'allow', NULL, NULL, NULL),
	(1698, 'p', 'SYS_ADMIN', 'api/v1/widget-bundles/builtin', 'allow', NULL, NULL, NULL),
	(1699, 'p', 'SYS_ADMIN', 'api/v1/widget-bundles/seed', 'allow', NULL, NULL, NULL),
	(1700, 'p', 'TENANT_ADMIN', 'api/v1/widget-bundles', 'allow', NULL, NULL, NULL),
	(1701, 'p', 'TENANT_ADMIN', 'api/v1/widget-bundles/:id', 'allow', NULL, NULL, NULL),
	(1702, 'p', 'TENANT_ADMIN', 'api/v1/widget-bundles/builtin', 'allow', NULL, NULL, NULL),
	(1703, 'p', 'TENANT_ADMIN', 'api/v1/widget-bundles/seed', 'allow', NULL, NULL, NULL),
	(1704, 'g2', 'api/v1/billing/api-quota', 'api/v1/billing/api-quota', NULL, NULL, NULL, NULL),
	(1705, 'p', 'SYS_ADMIN', 'api/v1/billing/api-quota', 'allow', NULL, NULL, NULL),
	(1706, 'p', 'TENANT_ADMIN', 'api/v1/billing/api-quota', 'allow', NULL, NULL, NULL),
	(1707, 'g2', 'api/v1/rule-chains/device-effective/:deviceId', 'api/v1/rule-chains/device-effective/:deviceId', NULL, NULL, NULL, NULL),
	(1708, 'p', 'SYS_ADMIN', 'api/v1/rule-chains/device-effective/:deviceId', 'allow', NULL, NULL, NULL),
	(1709, 'p', 'TENANT_ADMIN', 'api/v1/rule-chains/device-effective/:deviceId', 'allow', NULL, NULL, NULL),
	(1710, 'p', 'TENANT_USER', 'api/v1/rule-chains/device-effective/:deviceId', 'allow', NULL, NULL, NULL),
	(1711, 'g2', 'api/v1/entity_versions/:id/diff/:target_id', 'api/v1/entity_versions/:id/diff/:target_id', NULL, NULL, NULL, NULL),
	(1712, 'p', 'SYS_ADMIN', 'api/v1/entity_versions/:id/diff/:target_id', 'allow', NULL, NULL, NULL),
	(1713, 'p', 'TENANT_ADMIN', 'api/v1/entity_versions/:id/diff/:target_id', 'allow', NULL, NULL, NULL),
	(1714, 'p', 'TENANT_USER', 'api/v1/entity_versions/:id/diff/:target_id', 'allow', NULL, NULL, NULL),
	(1715, 'g2', 'api/v1/media/files', 'api/v1/media/files', NULL, NULL, NULL, NULL),
	(1716, 'g2', 'api/v1/media/files/:id', 'api/v1/media/files/:id', NULL, NULL, NULL, NULL),
	(1717, 'p', 'SYS_ADMIN', 'api/v1/media/files', 'allow', NULL, NULL, NULL),
	(1718, 'p', 'SYS_ADMIN', 'api/v1/media/files/:id', 'allow', NULL, NULL, NULL),
	(1719, 'p', 'TENANT_ADMIN', 'api/v1/media/files', 'allow', NULL, NULL, NULL),
	(1720, 'p', 'TENANT_ADMIN', 'api/v1/media/files/:id', 'allow', NULL, NULL, NULL),
	(1721, 'g2', 'api/v1/integrations', 'api/v1/integrations', NULL, NULL, NULL, NULL),
	(1722, 'g2', 'api/v1/integrations/:id', 'api/v1/integrations/:id', NULL, NULL, NULL, NULL),
	(1723, 'p', 'SYS_ADMIN', 'api/v1/integrations', 'allow', NULL, NULL, NULL),
	(1724, 'p', 'SYS_ADMIN', 'api/v1/integrations/:id', 'allow', NULL, NULL, NULL),
	(1725, 'p', 'TENANT_ADMIN', 'api/v1/integrations', 'allow', NULL, NULL, NULL),
	(1726, 'p', 'TENANT_ADMIN', 'api/v1/integrations/:id', 'allow', NULL, NULL, NULL),
	(1727, 'g2', 'api/v1/user_group', 'api/v1/user_group', NULL, NULL, NULL, NULL),
	(1728, 'g2', 'api/v1/user_group/:id', 'api/v1/user_group/:id', NULL, NULL, NULL, NULL),
	(1729, 'g2', 'api/v1/user_groups', 'api/v1/user_groups', NULL, NULL, NULL, NULL),
	(1730, 'g2', 'api/v1/user_group/:id/users', 'api/v1/user_group/:id/users', NULL, NULL, NULL, NULL),
	(1731, 'g2', 'api/v1/user_group/:id/permissions', 'api/v1/user_group/:id/permissions', NULL, NULL, NULL, NULL),
	(1732, 'p', 'SYS_ADMIN', 'api/v1/user_group', 'allow', NULL, NULL, NULL),
	(1733, 'p', 'SYS_ADMIN', 'api/v1/user_group/:id', 'allow', NULL, NULL, NULL),
	(1734, 'p', 'SYS_ADMIN', 'api/v1/user_groups', 'allow', NULL, NULL, NULL),
	(1735, 'p', 'SYS_ADMIN', 'api/v1/user_group/:id/users', 'allow', NULL, NULL, NULL),
	(1736, 'p', 'SYS_ADMIN', 'api/v1/user_group/:id/permissions', 'allow', NULL, NULL, NULL),
	(1737, 'p', 'TENANT_ADMIN', 'api/v1/user_group', 'allow', NULL, NULL, NULL),
	(1738, 'p', 'TENANT_ADMIN', 'api/v1/user_group/:id', 'allow', NULL, NULL, NULL),
	(1739, 'p', 'TENANT_ADMIN', 'api/v1/user_groups', 'allow', NULL, NULL, NULL),
	(1740, 'p', 'TENANT_ADMIN', 'api/v1/user_group/:id/users', 'allow', NULL, NULL, NULL),
	(1741, 'p', 'TENANT_ADMIN', 'api/v1/user_group/:id/permissions', 'allow', NULL, NULL, NULL),
	(1742, 'g2', 'api/v1/whitelabel/translations', 'api/v1/whitelabel/translations', NULL, NULL, NULL, NULL),
	(1743, 'g2', 'api/v1/whitelabel/custom-css', 'api/v1/whitelabel/custom-css', NULL, NULL, NULL, NULL),
	(1744, 'g2', 'api/v1/whitelabel/overrides', 'api/v1/whitelabel/overrides', NULL, NULL, NULL, NULL),
	(1745, 'p', 'SYS_ADMIN', 'api/v1/whitelabel/translations', 'allow', NULL, NULL, NULL),
	(1746, 'p', 'SYS_ADMIN', 'api/v1/whitelabel/custom-css', 'allow', NULL, NULL, NULL),
	(1747, 'p', 'SYS_ADMIN', 'api/v1/whitelabel/overrides', 'allow', NULL, NULL, NULL),
	(1748, 'p', 'TENANT_ADMIN', 'api/v1/whitelabel/translations', 'allow', NULL, NULL, NULL),
	(1749, 'p', 'TENANT_ADMIN', 'api/v1/whitelabel/custom-css', 'allow', NULL, NULL, NULL),
	(1750, 'p', 'TENANT_ADMIN', 'api/v1/whitelabel/overrides', 'allow', NULL, NULL, NULL),
	(1751, 'p', 'TENANT_USER', 'api/v1/whitelabel/overrides', 'allow', NULL, NULL, NULL),
	(1752, 'g2', 'api/v1/mobile/app_bundles', 'api/v1/mobile/app_bundles', NULL, NULL, NULL, NULL),
	(1753, 'g2', 'api/v1/mobile/app_bundles/upload', 'api/v1/mobile/app_bundles/upload', NULL, NULL, NULL, NULL),
	(1754, 'g2', 'api/v1/mobile/app_bundles/:id', 'api/v1/mobile/app_bundles/:id', NULL, NULL, NULL, NULL),
	(1755, 'g2', 'api/v1/mobile/app_bundles/:id/publish', 'api/v1/mobile/app_bundles/:id/publish', NULL, NULL, NULL, NULL),
	(1756, 'g2', 'api/v1/mobile/app_bundles/:id/archive', 'api/v1/mobile/app_bundles/:id/archive', NULL, NULL, NULL, NULL),
	(1757, 'g2', 'api/v1/mobile/app_bundles/:id/download', 'api/v1/mobile/app_bundles/:id/download', NULL, NULL, NULL, NULL),
	(1758, 'p', 'SYS_ADMIN', 'api/v1/mobile/app_bundles', 'allow', NULL, NULL, NULL),
	(1759, 'p', 'SYS_ADMIN', 'api/v1/mobile/app_bundles/upload', 'allow', NULL, NULL, NULL),
	(1760, 'p', 'SYS_ADMIN', 'api/v1/mobile/app_bundles/:id', 'allow', NULL, NULL, NULL),
	(1761, 'p', 'SYS_ADMIN', 'api/v1/mobile/app_bundles/:id/publish', 'allow', NULL, NULL, NULL),
	(1762, 'p', 'SYS_ADMIN', 'api/v1/mobile/app_bundles/:id/archive', 'allow', NULL, NULL, NULL),
	(1763, 'p', 'SYS_ADMIN', 'api/v1/mobile/app_bundles/:id/download', 'allow', NULL, NULL, NULL),
	(1764, 'p', 'TENANT_ADMIN', 'api/v1/mobile/app_bundles', 'allow', NULL, NULL, NULL),
	(1765, 'p', 'TENANT_ADMIN', 'api/v1/mobile/app_bundles/upload', 'allow', NULL, NULL, NULL),
	(1766, 'p', 'TENANT_ADMIN', 'api/v1/mobile/app_bundles/:id', 'allow', NULL, NULL, NULL),
	(1767, 'p', 'TENANT_ADMIN', 'api/v1/mobile/app_bundles/:id/publish', 'allow', NULL, NULL, NULL),
	(1768, 'p', 'TENANT_ADMIN', 'api/v1/mobile/app_bundles/:id/archive', 'allow', NULL, NULL, NULL),
	(1769, 'p', 'TENANT_ADMIN', 'api/v1/mobile/app_bundles/:id/download', 'allow', NULL, NULL, NULL),
	(1770, 'g2', 'api/v1/scheduler/events', 'api/v1/scheduler/events', NULL, NULL, NULL, NULL),
	(1771, 'g2', 'api/v1/scheduler/events/:id', 'api/v1/scheduler/events/:id', NULL, NULL, NULL, NULL),
	(1772, 'p', 'SYS_ADMIN', 'api/v1/scheduler/events', 'allow', NULL, NULL, NULL),
	(1773, 'p', 'SYS_ADMIN', 'api/v1/scheduler/events/:id', 'allow', NULL, NULL, NULL),
	(1774, 'p', 'TENANT_ADMIN', 'api/v1/scheduler/events', 'allow', NULL, NULL, NULL),
	(1775, 'p', 'TENANT_ADMIN', 'api/v1/scheduler/events/:id', 'allow', NULL, NULL, NULL),
	(1776, 'g2', 'api/v1/datapolicy/:id', 'api/v1/datapolicy/:id', NULL, NULL, NULL, NULL),
	(1777, 'p', 'SYS_ADMIN', 'api/v1/datapolicy/:id', 'allow', NULL, NULL, NULL),
	(1778, 'p', 'TENANT_ADMIN', 'api/v1/datapolicy/:id', 'allow', NULL, NULL, NULL);


--
-- Data for Name: command_job_details; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: command_job_dispatch_quotas; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: command_job_events; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: command_jobs; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: command_set_logs; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: customer_devices; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: customers; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: data_converters; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: data_policy; Type: TABLE DATA; Schema: public; Owner: -
--

INSERT INTO public.data_policy (id, data_type, retention_days, last_cleanup_time, last_cleanup_data_time, enabled, remark, tenant_id, device_config_id) VALUES
	('b', '2', 15, '2024-06-05 10:02:00.003+08', '2024-05-21 10:02:00.003+08', '1', '', NULL, NULL),
	('a', '1', 30, '2024-06-05 10:02:00.003+08', '2024-05-21 10:02:00.101+08', '1', '', NULL, NULL);


--
-- Data for Name: data_retention_registry; Type: TABLE DATA; Schema: public; Owner: -
--

INSERT INTO public.data_retention_registry (id, table_name, time_column, time_kind, retention_days, category, enabled, batch_size, resolved_only, last_cleanup_time, last_cleanup_data_time, remark, created_at, updated_at) VALUES
	('r-uplink-receipts', 'uplink_storage_receipts', 'created_at', 'timestamptz', 45, 'idempotency_receipt', '1', 10000, false, NULL, NULL, '上行幂等回执，默认开启；窗口必须显著长于最大重放租约', '2026-10-01 14:17:36.718298+08', '2026-10-01 14:17:36.718298+08'),
	('r-event-datas', 'event_datas', 'ts', 'timestamptz', 180, 'customer_data', '2', 10000, false, NULL, NULL, '设备事件上报（ts 为 timestamptz，与 telemetry_datas 的 UnixMilli 列不同）', '2026-10-01 14:17:36.718298+08', '2026-10-01 14:17:36.718298+08'),
	('r-telemetry-set-logs', 'telemetry_set_logs', 'created_at', 'timestamptz', 180, 'customer_data', '2', 10000, false, NULL, NULL, '遥测下发记录', '2026-10-01 14:17:36.718298+08', '2026-10-01 14:17:36.718298+08'),
	('r-attribute-set-logs', 'attribute_set_logs', 'created_at', 'timestamptz', 180, 'customer_data', '2', 10000, false, NULL, NULL, '属性下发记录', '2026-10-01 14:17:36.718298+08', '2026-10-01 14:17:36.718298+08'),
	('r-command-set-logs', 'command_set_logs', 'created_at', 'timestamptz', 180, 'customer_data', '2', 10000, false, NULL, NULL, '命令下发记录', '2026-10-01 14:17:36.718298+08', '2026-10-01 14:17:36.718298+08'),
	('r-device-status-hist', 'device_status_history', 'change_time', 'timestamptz', 180, 'customer_data', '2', 10000, false, NULL, NULL, '设备上下线历史', '2026-10-01 14:17:36.718298+08', '2026-10-01 14:17:36.718298+08'),
	('r-alarm-history', 'alarm_history', 'create_at', 'timestamptz', 365, 'customer_data', '2', 5000, false, NULL, NULL, '告警历史', '2026-10-01 14:17:36.718298+08', '2026-10-01 14:17:36.718298+08'),
	('r-alarm-info', 'alarm_info', 'alarm_time', 'timestamptz', 365, 'customer_data', '2', 5000, false, NULL, NULL, '告警信息', '2026-10-01 14:17:36.718298+08', '2026-10-01 14:17:36.718298+08'),
	('r-scene-log', 'scene_log', 'executed_at', 'timestamptz', 180, 'customer_data', '2', 10000, false, NULL, NULL, '场景执行日志', '2026-10-01 14:17:36.718298+08', '2026-10-01 14:17:36.718298+08'),
	('r-scene-auto-log', 'scene_automation_log', 'executed_at', 'timestamptz', 180, 'customer_data', '2', 10000, false, NULL, NULL, '场景联动执行日志', '2026-10-01 14:17:36.718298+08', '2026-10-01 14:17:36.718298+08'),
	('r-command-job-events', 'command_job_events', 'created_at', 'timestamptz', 180, 'customer_data', '2', 10000, false, NULL, NULL, '命令作业事件', '2026-10-01 14:17:36.718298+08', '2026-10-01 14:17:36.718298+08'),
	('r-command-job-det', 'command_job_details', 'created_at', 'timestamptz', 180, 'customer_data', '2', 10000, false, NULL, NULL, '命令作业明细（终态后不再变更）', '2026-10-01 14:17:36.718298+08', '2026-10-01 14:17:36.718298+08'),
	('r-device-user-logs', 'device_user_logs', 'created_at', 'timestamptz', 365, 'customer_data', '2', 10000, false, NULL, NULL, '设备用户操作日志', '2026-10-01 14:17:36.718298+08', '2026-10-01 14:17:36.718298+08'),
	('r-message-push-log', 'message_push_log', 'create_time', 'timestamptz', 180, 'customer_data', '2', 10000, false, NULL, NULL, '消息推送日志', '2026-10-01 14:17:36.718298+08', '2026-10-01 14:17:36.718298+08'),
	('r-rule-chain-traces', 'rule_chain_node_traces', 'created_at', 'timestamptz', 90, 'customer_data', '2', 10000, false, NULL, NULL, '规则链节点轨迹（调试用，可短保留）', '2026-10-01 14:17:36.718298+08', '2026-10-01 14:17:36.718298+08'),
	('r-operation-logs', 'operation_logs', 'created_at', 'timestamptz', 180, 'audit_log', '2', 10000, false, NULL, NULL, '操作审计日志；与 data_policy data_type=''2'' 并存，两者都开启时删除幂等', '2026-10-01 14:17:36.718298+08', '2026-10-01 14:17:36.718298+08'),
	('r-uplink-dl', 'uplink_storage_dead_letters', 'created_at', 'timestamptz', 30, 'dead_letter', '2', 5000, true, NULL, NULL, '上行死信，仅回收 resolved', '2026-10-01 14:17:36.718298+08', '2026-10-01 14:17:36.718298+08'),
	('r-telemetry-dl', 'telemetry_dead_letters', 'created_at', 'timestamptz', 30, 'dead_letter', '2', 5000, true, NULL, NULL, '遥测死信，仅回收 resolved', '2026-10-01 14:17:36.718298+08', '2026-10-01 14:17:36.718298+08'),
	('r-rule-chain-dl', 'rule_chain_dead_letters', 'created_at', 'timestamptz', 30, 'dead_letter', '2', 5000, false, NULL, NULL, '规则链死信（无可重放载荷，审计最小化）', '2026-10-01 14:17:36.718298+08', '2026-10-01 14:17:36.718298+08');


--
-- Data for Name: data_scripts; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: device_certificates; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: device_claim_tokens; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: device_configs; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: device_health_scores; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: device_modbus_profiles; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: device_model_attributes; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: device_model_commands; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: device_model_custom_commands; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: device_model_custom_control; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: device_model_events; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: device_model_telemetry; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: device_pre_register_credential_grants; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: device_shadow_messages; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: device_status_history; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: device_template_upgrade_history; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: device_templates; Type: TABLE DATA; Schema: public; Owner: -
--

INSERT INTO public.device_templates (id, name, author, version, description, tenant_id, created_at, updated_at, flag, label, web_chart_config, app_chart_config, remark, path, type_key, brand, model_number, download_count) VALUES
	('tpl-industrial-sensor', '工业传感器模板', 'AetherLink', '1.0.0', '面向工业现场的通用传感器接入模板：温湿度/振动/压力等点型遥测，配套 1 天分块与告警阈值示例。适用于 Modbus/SNMP/OPC UA 采集与 MQTT 直连设备。', 'd616bcbb', '2026-10-01 14:17:36.718298+08', '2026-10-01 14:17:36.718298+08', 1, '工业', '{"charts":[]}', NULL, '行业模板种子：复制后按现场点表改造', NULL, 'industrial', '', '', 0),
	('tpl-power-monitor', '电力监测模板', 'AetherLink', '1.0.0', '配电房/开关柜电力监测模板：三相电压电流、有功无功、功率因数、电能累计；支持越限告警与日/月电能曲线。', 'd616bcbb', '2026-10-01 14:17:36.718298+08', '2026-10-01 14:17:36.718298+08', 1, '电力', '{"charts":[]}', NULL, '行业模板种子：复制后按现场点表改造', NULL, 'power', '', '', 0),
	('tpl-smart-home', '智能家居模板', 'AetherLink', '1.0.0', '智能家居网关接入模板：温湿度、门窗、照明的状态与遥测，支持场景联动与移动端查看。', 'd616bcbb', '2026-10-01 14:17:36.718298+08', '2026-10-01 14:17:36.718298+08', 1, '智能家居', '{"charts":[]}', NULL, '行业模板种子：复制后按现场点表改造', NULL, 'smart-home', '', '', 0);


--
-- Data for Name: device_topic_mappings; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: device_trigger_condition; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: device_user_logs; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: devices; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: edge_node_certificates; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: edge_node_upgrade_history; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: edge_nodes; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: edge_sync_tasks; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: email_templates; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: entity_relations; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: entity_versions; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: event_datas; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: expected_datas; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: fleet_saved_filters; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: group_permissions; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: groups; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: industry_solution_installs; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: industry_solutions; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: integrations; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: logo; Type: TABLE DATA; Schema: public; Owner: -
--

INSERT INTO public.logo (id, system_name, logo_cache, logo_background, logo_loading, home_background, remark, tenant_id, theme_color, favicon) VALUES
	('a', 'AetherLink IoT', '', '', '', '', NULL, '', '', '');


--
-- Data for Name: media_files; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: message_push_config; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: message_push_log; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: message_push_manage; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: message_push_rule_log; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: mobile_app_bundles; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: mqtt_session_revocation_acks; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: mqtt_session_revocation_outbox; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: notification_groups; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: notification_histories; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: notification_history_devices; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: notification_services_config; Type: TABLE DATA; Schema: public; Owner: -
--

INSERT INTO public.notification_services_config (id, config, notice_type, status, remark) VALUES
	('286a116e-c25f-0f4c-890a-8a72128ef355', '{"host":"smtp.example.com","port":465,"from_password":"CHANGE_ME_SMTP_PASSWORD","from_email":"noreply@example.com","ssl":true}', 'EMAIL', 'OPEN', '');


--
-- Data for Name: one_time_tasks; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: open_api_keys; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: operation_logs; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: ota_upgrade_packages; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: ota_upgrade_task_details; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: ota_upgrade_tasks; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: payload_schemas; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: periodic_tasks; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: platform_cas; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: plugin_registries; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: products; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: protocol_plugins; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: push_deliveries; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: push_device_registrations; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: r_group_device; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: r_group_user; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: report_schedule_deliveries; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: report_schedule_runs; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: report_schedules; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: roles; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: rule_chain_checkpoints; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: rule_chain_dead_letters; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: rule_chain_edges; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: rule_chain_node_traces; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: rule_chain_nodes; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: rule_chain_replay_records; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: rule_chain_versions; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: rule_chains; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: scada_control_audits; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: scada_document_versions; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: scada_documents; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: scada_projects; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: scene_action_info; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: scene_automation_log; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: scene_automation_timers; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: scene_automations; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: scene_info; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: scene_log; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: scheduler_events; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: service_access; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: service_plugins; Type: TABLE DATA; Schema: public; Owner: -
--

INSERT INTO public.service_plugins (id, name, service_identifier, service_type, last_active_time, version, create_at, update_at, description, service_config, remark) VALUES
	('d073ba1d-445a-a07f-430b-cf6d154bc5e8', 'MODBUS-RTU', 'MODBUS_RTU', 1, '2024-12-25 10:30:43.678+08', 'v1.0.1', '2024-12-25 09:05:12.019+08', '2024-12-25 09:05:36.696+08', '', '{"http_address":"modbus_service:503","device_type":2,"sub_topic_prefix":"plugin/modbus/","access_address":":502"}', ''),
	('4bec425e-c7e3-476a-0303-ee8193ddf4ca', 'MODBUS-TCP	', 'MODBUS_TCP', 1, '2024-12-25 10:30:43.678+08', 'v1.0.1', '2024-12-25 09:03:25.401+08', '2024-12-25 09:10:02.208+08', '', '{"http_address":"modbus_service:503","device_type":2,"sub_topic_prefix":"plugin/modbus/","access_address":":502"}', ''),
	('a1c2d3e4-f5a6-4b7c-8d9e-0f1a2b3c45l5', 'HTTP', 'HTTP', 1, '2026-10-01 14:17:36.718298+08', 'v1.0.0', '2026-10-01 14:17:36.718298+08', '2026-10-01 14:17:36.718298+08', '官方标准 HTTP 协议接入组件', '{"http_address":"http_adapter:19091","device_type":1,"sub_topic_prefix":"plugin/http/","access_address":":19090"}', '');


--
-- Data for Name: subscription_plans; Type: TABLE DATA; Schema: public; Owner: -
--

INSERT INTO public.subscription_plans (id, code, name, description, price_monthly, currency, max_devices, max_tenants, max_users, max_telemetry_per_day, max_api_calls_per_day, features, enabled, created_at, updated_at) VALUES
	('plan-free', 'free', 'Community / Free', 'Free community edition plan for testing and personal projects', 0.00, 'USD', 10, 1, 3, 10000, 5000, '["basic_telemetry", "rule_chains", "dashboards"]', 1, '2026-10-01 14:17:36.718298+08', '2026-10-01 14:17:36.718298+08'),
	('plan-pro', 'pro', 'Professional', 'Professional tier for growing IoT deployments and SMBs', 99.00, 'USD', 500, 10, 25, 500000, 100000, '["basic_telemetry", "rule_chains", "dashboards", "alarm_advanced", "units_conversion", "sparkplug_b", "secrets_storage"]', 1, '2026-10-01 14:17:36.718298+08', '2026-10-01 14:17:36.718298+08'),
	('plan-enterprise', 'enterprise', 'Enterprise', 'Full enterprise tier with unlimited capabilities, SCADA, and SLA support', 499.00, 'USD', 10000, 100, 200, 10000000, 5000000, '["basic_telemetry", "rule_chains", "dashboards", "alarm_advanced", "units_conversion", "sparkplug_b", "secrets_storage", "scada_canvas", "cloud_rule_nodes", "sla_support"]', 1, '2026-10-01 14:17:36.718298+08', '2026-10-01 14:17:36.718298+08');


--
-- Data for Name: sys_config; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: sys_dict; Type: TABLE DATA; Schema: public; Owner: -
--

INSERT INTO public.sys_dict (id, dict_code, dict_value, created_at, remark) VALUES
	('0013fb9e-e3be-95d4-9c96-f18d1f9ddfcd', 'GATEWAY_PROTOCOL', 'MQTT', '2024-01-18 15:39:38.469+08', NULL),
	('7162fb9e-e3be-95d4-9c96-f18d1f9ddfcd', 'DRIECT_ATTACHED_PROTOCOL', 'MQTT', '2024-01-18 15:39:38.469+08', NULL);


--
-- Data for Name: sys_dict_language; Type: TABLE DATA; Schema: public; Owner: -
--

INSERT INTO public.sys_dict_language (id, dict_id, language_code, translation) VALUES
	('001c3960-3067-536d-5c97-7645351a687c', '7162fb9e-e3be-95d4-9c96-f18d1f9ddfcd', 'zh_CN', 'MQTT协议'),
	('002c3960-3067-536d-5c97-7645351a687b', '0013fb9e-e3be-95d4-9c96-f18d1f9ddfcd', 'zh_CN', 'MQTT协议(网关)'),
	('7162fb9e-e3be-95d4-9c96-f18d1f9ddfss', '7162fb9e-e3be-95d4-9c96-f18d1f9ddfcd', 'en_US', 'MQTT Protocol'),
	('7162fb9e-e3be-95d4-9c96-f18d1f9ddfff', '0013fb9e-e3be-95d4-9c96-f18d1f9ddfcd', 'en_US', 'MQTT Protocol(Gateway)');


--
-- Data for Name: sys_function; Type: TABLE DATA; Schema: public; Owner: -
--

INSERT INTO public.sys_function (id, name, enable_flag, description, remark) VALUES
	('function_1', 'use_captcha', 'disable', '验证码登陆', NULL),
	('function_2', 'enable_reg', 'disable', '租户注册', NULL),
	('function_3', 'frontend_res', 'disable', '前端RSA加密', NULL),
	('function_4', 'shared_account', 'enable', '共享账号', NULL);


--
-- Data for Name: sys_permissions; Type: TABLE DATA; Schema: public; Owner: -
--

INSERT INTO public.sys_permissions (code, name, module, description, api_patterns, created_at) VALUES
	('device:read', '查看设备与模板', 'device', '允许浏览设备列表、设备详情、产品物模型配置与分组树', '["api/v1/device", "api/v1/device/:id", "api/v1/device/list", "api/v1/device/group", "api/v1/device/group/tree", "api/v1/device/config", "api/v1/device/config/:id"]', '2026-10-01 14:17:36.718298+08'),
	('device:write', '管理设备与模板', 'device', '允许创建、更新、删除设备与产品配置，导入预注册', '["api/v1/device", "api/v1/device/batch", "api/v1/device/preRegister", "api/v1/device/config", "api/v1/product"]', '2026-10-01 14:17:36.718298+08'),
	('device:control', '下发设备命令与影子控制', 'device', '允许执行设备下行控制、RPC 指令下发与设备影子 ACK', '["api/v1/device/command", "api/v1/device/command/batch", "api/v1/device/shadow/:deviceId/:msgId/ack", "api/v1/device/shadow/messages"]', '2026-10-01 14:17:36.718298+08'),
	('telemetry:read', '读取遥测与数据分析', 'telemetry', '允许读取实时遥测、历史聚合数据、同比环比与分析导出', '["api/v1/telemetry/datas/current", "api/v1/telemetry/datas/statistic", "api/v1/telemetry/analysis/query", "api/v1/telemetry/analysis/export", "api/v1/telemetry/analysis/anomaly"]', '2026-10-01 14:17:36.718298+08'),
	('alarm:read', '查看告警历史与配置', 'alarm', '允许读取告警记录、告警配置与统计数据', '["api/v1/alarm/history", "api/v1/alarm/history/:id", "api/v1/alarm/config", "api/v1/alarm_config"]', '2026-10-01 14:17:36.718298+08'),
	('alarm:operate', '告警操作与处置', 'alarm', '允许确认告警、清除告警、指派责任人与添加评论', '["api/v1/alarm/history/:id/ack", "api/v1/alarm/history/:id/clear", "api/v1/alarm/history/:id/assign", "api/v1/alarm/history/:id/comments"]', '2026-10-01 14:17:36.718298+08'),
	('rule:read', '查看规则链与版本', 'rule_chain', '允许浏览规则链图谱、版本快照、死信队列与 Trace 追踪', '["api/v1/rule/chains", "api/v1/rule/chain/:id", "api/v1/rule/chain/versions", "api/v1/rule/chain/dead_letters", "api/v1/rule/chain/traces"]', '2026-10-01 14:17:36.718298+08'),
	('rule:write', '编排与发布规则链', 'rule_chain', '允许编辑规则链、发布新版本、回滚与重放', '["api/v1/rule/chain", "api/v1/rule/chain/publish", "api/v1/rule/chain/rollback", "api/v1/rule/chain/replay"]', '2026-10-01 14:17:36.718298+08'),
	('report:read', '查看报表与历史', 'report', '允许浏览报表调度计划、运行历史与交付记录', '["api/v1/report_schedules", "api/v1/report_schedules/:id/runs"]', '2026-10-01 14:17:36.718298+08'),
	('report:write', '管理与触发报表', 'report', '允许创建、更新、删除报表任务及手动即时执行', '["api/v1/report_schedules", "api/v1/report_schedules/:id/run"]', '2026-10-01 14:17:36.718298+08'),
	('scada:read', '查看 SCADA 画布与符号', 'scada', '允许浏览组态工程、画布文档与工业符号库', '["api/v1/scada/projects", "api/v1/scada/documents", "api/v1/scada/documents/:id"]', '2026-10-01 14:17:36.718298+08'),
	('scada:control', 'SCADA 工业控制下发', 'scada', '允许通过 HMAC 二次确认下发工业组态控制命令', '["api/v1/scada/controls/dispatch", "api/v1/scada/controls/confirm"]', '2026-10-01 14:17:36.718298+08'),
	('ota:read', '查看固件包与升级任务', 'ota', '允许查看 OTA 固件包列表与升级批次明细', '["api/v1/ota/packages", "api/v1/ota/tasks", "api/v1/ota/tasks/:id/report"]', '2026-10-01 14:17:36.718298+08'),
	('ota:write', '创建与管理 OTA 升级', 'ota', '允许上传固件包、创建升级任务、暂停与回滚批次', '["api/v1/ota/packages", "api/v1/ota/tasks", "api/v1/ota/tasks/:id/pause", "api/v1/ota/tasks/:id/resume", "api/v1/ota/tasks/:id/rollback"]', '2026-10-01 14:17:36.718298+08');


--
-- Data for Name: sys_role_permissions; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: sys_secrets; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: sys_ui_elements; Type: TABLE DATA; Schema: public; Owner: -
--

INSERT INTO public.sys_ui_elements (id, parent_id, element_code, element_type, orders, param1, param2, param3, authority, description, created_at, remark, multilingual, route_path) VALUES
	('f9bd5f79-291e-26d2-1553-473c04b15ce4', 'e1ebd134-53df-3105-35f4-489fc674d173', 'management_setting', 3, 42, '/management/setting', 'uil:brightness-plus', 'self', '["SYS_ADMIN"]', '系统设置', '2024-02-18 17:52:08.236+08', '', 'route.management_setting', 'view.management_setting'),
	('faf7e607-00ae-3483-40a1-b74f9245b100', 'e1ebd134-53df-3105-35f4-489fc674d173', 'management_auth', 3, 43, '/management/auth', 'ic:baseline-security', 'self', '["SYS_ADMIN"]', '菜单管理', '2024-02-18 17:49:31.209+08', '', 'route.management_auth', 'view.management_auth'),
	('a2c53126-029f-7138-4d7a-f45491f396da', '0', 'apply', 1, 3, '/apply', 'mdi:apps-box', '0', '["SYS_ADMIN"]', '应用管理', '2024-02-18 17:59:31.642+08', '', 'route.apply', 'layout.base'),
	('36c4f5ce-3279-55f2-ede2-81b4a0bae24b', 'e1ebd134-53df-3105-35f4-489fc674d173', 'management_user', 3, 41, '/management/user', 'ic:round-manage-accounts', 'self', '["SYS_ADMIN"]', '用户管理', '2024-02-18 17:50:48.999+08', '', 'route.management_user', 'view.management_user'),
	('fec91838-d30d-7d66-6715-0912f1b171d8', 'e1ebd134-53df-3105-35f4-489fc674d173', 'management_notification', 3, 44, '/management/notification', 'mdi:alert', 'self', '["SYS_ADMIN"]', '通知配置', '2024-03-15 19:50:07.495+08', '', 'route.management_notification', 'view.management_notification'),
	('29a684f9-c2bb-1a6f-6045-314944bef580', 'a2c53126-029f-7138-4d7a-f45491f396da', 'plug_in', 3, 32, '/apply/plugin', 'mdi:emoticon', '0', '["SYS_ADMIN"]', '插件管理', '2024-06-29 01:04:51.301+08', '', 'route.apply_in', ''),
	('51381989-1160-93cd-182e-d44a1c4ab89b', '676e8f33-875a-0473-e9ca-c82fd09fef57', 'automation_scene-manage', 3, 1142, '/automation/scene-manage', 'uil:brightness-plus', 'self', '["TENANT_ADMIN", "SYS_ADMIN", "TENANT_USER"]', '场景管理', '2024-03-07 21:44:11.106+08', '', 'route.automation_scene-manage', 'view.automation_scene-manage'),
	('95e2a961-382b-f4a6-87b3-1898123c95bc', '0', 'visualization', 1, 113, '/visualization', 'mdi:database-cog-outline', 'self', '["TENANT_ADMIN", "SYS_ADMIN", "TENANT_USER"]', '可视化', '2024-03-07 21:37:16.042+08', '', 'route.visualization', 'layout.base'),
	('676e8f33-875a-0473-e9ca-c82fd09fef57', '0', 'automation', 1, 114, '/automation', 'material-symbols:device-hub', 'self', '["TENANT_ADMIN", "TENANT_USER"]', '自动化', '2024-03-07 21:41:17.921+08', '', 'route.automation', 'layout.base'),
	('e1ebd134-53df-3105-35f4-489fc674d173', '0', 'management', 1, 120, '/management', 'carbon:cloud-service-management', 'self', '["SYS_ADMIN", "TENANT_ADMIN", "TENANT_USER"]', '系统管理', '2024-02-18 17:48:45.265+08', '', 'route.management', 'layout.base'),
	('96aa2fac-90b2-aca1-1ce0-51b5060f4081', '676e8f33-875a-0473-e9ca-c82fd09fef57', 'automation_linkage-edit', 3, 1143, '/automation/linkage-edit', '', '1', '["TENANT_ADMIN", "SYS_ADMIN", "TENANT_USER"]', '场景联动编辑', '2024-03-15 01:36:03.938+08', '', 'route.automation_linkage-edit', 'view.automation_linkage-edit'),
	('01dab674-9556-cdd7-b800-78bcb366adb4', '676e8f33-875a-0473-e9ca-c82fd09fef57', 'automation_scene-linkage', 3, 1141, '/automation/scene-linkage', 'mdi:airplane-edit', 'self', '["TENANT_ADMIN", "SYS_ADMIN", "TENANT_USER"]', '场景联动', '2024-03-07 21:43:33.92+08', '', 'route.automation_scene-linkage', 'view.automation_scene-linkage'),
	('c078182f-bf4b-b560-da97-02926fa98f78', '650bc444-7672-1123-1e41-7e37365b0186', 'alarm_notification-record', 3, 1, '/alarm/notification-record', 'mdi:pencil-box-outline', 'self', '["TENANT_ADMIN", "TENANT_USER"]', '通知记录', '2024-03-20 10:04:34.927+08', '', 'route.alarm_notification-record', 'view.alarm_notification-record'),
	('485c2a20-ebc5-2216-4871-26453470d290', '650bc444-7672-1123-1e41-7e37365b0186', 'alarm_warning-message', 3, 999, '/alarm/warning-message', 'mdi:airballoon', 'self', '["TENANT_ADMIN", "TENANT_USER"]', '警告信息', '2024-03-17 15:27:40.378+08', '', 'route.alarm_warning-message', 'view.alarm_warning-message'),
	('2f3ffd60-efec-aafb-a866-f1cb79f88390', 'e1ebd134-53df-3105-35f4-489fc674d173', 'system-management-user_system-log', 3, 1171, '/system-management-user/system-log', 'mdi:monitor-dashboard', 'basic', '["TENANT_ADMIN", "TENANT_USER"]', '系统日志', '2024-03-07 22:23:08.576+08', '', 'route.system-management-user_system-log', 'view.system-management-user_system-log'),
	('e186a671-8e24-143a-5a2c-27a1f5f38bf3', '5373a6a2-1861-af35-eb4c-adfd5ca55ecd', 'device_config-edit', 3, 1128, '/device/config-edit', '', '1', '["TENANT_ADMIN", "TENANT_USER"]', '设备配置编辑', '2024-03-11 21:49:34.952+08', '', 'route.device_config-edit', 'view.device_config-edit'),
	('49857e46-2176-610e-98fc-892b4fde50f9', '5373a6a2-1861-af35-eb4c-adfd5ca55ecd', 'device_details', 3, 1124, '/device/details', 'mdi:monitor-dashboard', '1', '["TENANT_ADMIN", "TENANT_USER"]', '设备详情', '2024-03-05 17:52:21.434+08', '', 'route.device_details', 'view.device_details'),
	('75785418-a5af-d790-0783-e4ee4e42521e', '5373a6a2-1861-af35-eb4c-adfd5ca55ecd', 'device_grouping', 3, 1122, '/device/grouping', 'material-symbols:grid-on-outline-sharp', '0', '["TENANT_ADMIN", "TENANT_USER"]', '设备分组', '2024-03-05 17:53:25.004+08', '', 'route.device_grouping', 'view.device_grouping'),
	('8de46003-170c-a24d-6baf-84d1c7298aa3', '5373a6a2-1861-af35-eb4c-adfd5ca55ecd', 'device_grouping-details', 3, 1123, '/device/grouping-details', '', '1', '["TENANT_ADMIN", "TENANT_USER"]', '分组详情', '2024-03-05 17:54:23.158+08', '', 'route.device_grouping-details', 'view.device_grouping-details'),
	('5373a6a2-1861-af35-eb4c-adfd5ca55ecd', '0', 'device', 1, 112, '/device', 'mdi:view-dashboard-outline', '0', '["TENANT_ADMIN", "TENANT_USER"]', '设备接入', '2024-03-05 17:51:19.298+08', '', 'route.device', 'layout.base'),
	('7419e37e-c167-f12b-7ace-76e479144181', '5373a6a2-1861-af35-eb4c-adfd5ca55ecd', 'device_template', 3, 1127, '/device/template', 'simple-icons:apacheecharts', 'self', '["TENANT_ADMIN", "TENANT_USER"]', '功能模板', '2024-03-05 18:01:29.826+08', '定义物模型和显示图表', 'route.device_template', 'view.device_template'),
	('774a716d-9861-bac9-857f-acaa25e7659f', '5373a6a2-1861-af35-eb4c-adfd5ca55ecd', 'device_config', 3, 1126, '/device/config', 'clarity:plugin-line', 'self', '["TENANT_ADMIN", "TENANT_USER"]', '配置模板', '2024-03-05 22:06:53.842+08', '设备的协议和其他参数等所有配置', 'route.device_config', 'view.device_config'),
	('c4dff952-3bf4-8102-6882-e9d3f3cffbda', '5373a6a2-1861-af35-eb4c-adfd5ca55ecd', 'device_manage', 3, 1121, '/device/manage', 'mdi:chart-box-outline', '0', '["TENANT_ADMIN", "TENANT_USER"]', '设备管理', '2024-03-05 17:55:08.17+08', '', 'route.device_manage', 'view.device_manage'),
	('82c46beb-9ec4-8a3d-c6e4-04ba426e525a', '650bc444-7672-1123-1e41-7e37365b0186', 'alarm_notification-group', 3, 1, '/alarm/notification-group', 'ic:round-supervisor-account', 'basic', '["TENANT_ADMIN", "TENANT_USER"]', '通知组', '2024-03-20 10:03:19.955+08', '', 'route.alarm_notification-group', 'view.alarm_notification-group'),
	('650bc444-7672-1123-1e41-7e37365b0186', '0', 'alarm', 1, 115, '/alarm', 'mdi:alert', 'self', '["TENANT_ADMIN", "TENANT_USER"]', '告警', '2024-03-17 09:01:52.183+08', '', 'route.alarm', 'layout.base'),
	('76bfc16e-ed22-bcc0-c688-d462666e8a8d', '0', 'personal-center', 3, 999, '/personal-center', 'carbon:user-role', '1', '["TENANT_ADMIN", "SYS_ADMIN", "TENANT_USER"]', '个人中心', '2024-03-17 09:27:01.048+08', '', 'route.personal_center', 'layout.base$view.personal-center'),
	('975c9550-5db9-7b4c-5dea-7a4c326a37ff', '676e8f33-875a-0473-e9ca-c82fd09fef57', 'automation_scene-edit', 3, 1, '/automation/scene-edit', 'mdi:apps-box', '1', '["TENANT_ADMIN", "TENANT_USER"]', '新增场景', '2024-04-04 10:50:43.219+08', '', 'route.automation_scene-edit', 'view.automation_scene-edit'),
	('f960c45c-6d5b-e67a-c4ff-1f0e869c1625', '5373a6a2-1861-af35-eb4c-adfd5ca55ecd', 'device_service-details', 3, 1130, '/device/service-details', 'ph:align-bottom', '1', '["TENANT_ADMIN", "TENANT_USER"]', '服务详情', '2024-07-01 23:16:56.668+08', '', 'route.device_service_details', ''),
	('86cb08fa-8b08-3d99-4b3a-d6132ee93a0f', '5373a6a2-1861-af35-eb4c-adfd5ca55ecd', 'device_config-detail', 3, 1127, '/device/config-detail', 'mdi:database-cog-outline', '1', '["TENANT_ADMIN", "TENANT_USER"]', '设备配置详情', '2024-03-10 11:13:25.253+08', '', 'route.device_config-detail', 'view.device_config-detail'),
	('075d9f19-5618-bb9b-6ccd-f382bfd3292b', '5373a6a2-1861-af35-eb4c-adfd5ca55ecd', 'device_service-access', 3, 1129, '/device/service-access', 'mdi:ab-testing', '0', '["TENANT_ADMIN", "TENANT_USER"]', '服务接入点管理', '2024-07-01 21:52:09.402+08', '', 'route.device_service_access', ''),
	('59612e2f-e297-acb7-fcf4-143bf6e66109', '5373a6a2-1861-af35-eb4c-adfd5ca55ecd', 'device_details-child', 3, 1124, '/device/details-child', '', '1', '["TENANT_ADMIN", "TENANT_USER"]', '子设备详情', '2024-05-10 20:33:34.869+08', '', 'route.device_details-child', 'view.device_details-child'),
	('a190f7a5-1501-3814-9dd1-f3e1fbe7265e', '0', 'home', 3, 0, '/home', 'mdi:alpha-f-box-outline', 'self', '["SYS_ADMIN", "TENANT_ADMIN", "TENANT_USER"]', '首页', '2024-02-26 16:07:20.202+08', 'home', 'route.home', 'layout.base$view.home'),
	('cf168132-3cde-a0e2-6772-e3a28fca4a59', 'e1ebd134-53df-3105-35f4-489fc674d173', 'management_api', 3, 1999, '/management/api', 'mdi:pencil-box-outline', '0', '["TENANT_ADMIN", "TENANT_USER"]', 'API key', '2025-02-14 18:38:42.007+08', '', 'route.management_api', ''),
	('a0848997-3a3c-7ffa-1ca2-0380c564d5d0', '95e2a961-382b-f4a6-87b3-1898123c95bc', 'visualization_thingsvis', 3, 1, '/visualization/thingsvis', 'mdi:chart-box-outline', '0', '["SYS_ADMIN", "TENANT_ADMIN", "TENANT_USER"]', '新看板', '2026-02-06 16:40:03.021+08', '', 'route.visualization-thingsvis', ''),
	('3733d118-fe8a-d02e-3f89-85a754586f75', '95e2a961-382b-f4a6-87b3-1898123c95bc', 'visualization_thingsvis-dashboards', 3, 1, '/visualization/thingsvis-dashboards', 'mdi:view-dashboard-outline', '1', '["TENANT_ADMIN", "SYS_ADMIN", "TENANT_USER"]', 'visualization_thingsvis-dashboards', '2026-02-06 18:43:38.941+08', '', 'route.visualization-thingsvis-dashboards', ''),
	('e84d450c-ebc8-eae2-da17-f2f1269f6732', '95e2a961-382b-f4a6-87b3-1898123c95bc', 'visualization_thingsvis-editor', 3, 2, '/visualization/thingsvis-editor', 'mdi:view-dashboard-outline', '1', '["SYS_ADMIN", "TENANT_ADMIN", "TENANT_USER"]', 'thingsvis-editor', '2026-02-06 18:46:31.554+08', '', 'route.visualization-thingsvis-editor', ''),
	('b8f6bb70-5a38-4e6d-9d85-6acb3e65e5f2', '650bc444-7672-1123-1e41-7e37365b0186', 'alarm_rdi-overview', 3, 998, '/alarm/rdi-overview', 'mdi:chart-bell-curve', 'self', '["TENANT_ADMIN", "TENANT_USER"]', 'RDI 告警概览', '2026-10-01 14:17:36.718298+08', '', 'route.alarm_rdi-overview', 'view.alarm_rdi-overview'),
	('d7dd2c9e-9b64-4c7b-a66a-8a9f0bbf45a8', '5373a6a2-1861-af35-eb4c-adfd5ca55ecd', 'device_command-center', 3, 1125, '/device/command-center', 'mdi:send-circle-outline', '1', '["TENANT_ADMIN", "SYS_ADMIN", "TENANT_USER"]', 'Command Center', '2026-10-01 14:17:36.718298+08', 'Hidden handoff route for selected-device command jobs', 'route.device_command-center', 'view.device_command-center'),
	('d43c06b8-0c55-4f0f-8a29-f1df239fd083', '95e2a961-382b-f4a6-87b3-1898123c95bc', 'visualization_report', 3, 3, '/visualization/report', 'mdi:file-chart-outline', '0', '["SYS_ADMIN","TENANT_ADMIN"]', '定时报表', '2026-10-01 14:17:36.718298+08', 'Durable report scheduling and delivery history', 'route.visualization-report', 'view.visualization_report'),
	('a7f3c1d2-5e84-4b19-9c6a-2d8f0e3b7a41', '95e2a961-382b-f4a6-87b3-1898123c95bc', 'visualization_anomaly', 3, 4, '/visualization/anomaly', 'mdi:chart-line-variant', '0', '["SYS_ADMIN","TENANT_ADMIN"]', '异常检测', '2026-10-01 14:17:36.718298+08', 'Telemetry anomaly detection workbench (P2.2)', 'route.visualization-anomaly', 'view.visualization_anomaly'),
	('b8e4d2f3-6f95-4c2a-8d7b-3e9a1f4c8b52', '0', 'market', 1, 116, '/market', 'mdi:storefront-outline', 'self', '["SYS_ADMIN","TENANT_ADMIN"]', '模板市场', '2026-10-01 14:17:36.718298+08', 'Device template market (P1.6)', 'route.market', 'layout.base'),
	('c9f5e3a4-7a06-4d3b-9e8c-4fab2a5d9c63', 'b8e4d2f3-6f95-4c2a-8d7b-3e9a1f4c8b52', 'market_browse', 3, 1, '/market/browse', 'mdi:package-variant-closed', '0', '["SYS_ADMIN","TENANT_ADMIN"]', '模板市场浏览', '2026-10-01 14:17:36.718298+08', 'Market browse with signed bundle preview / overwrite gate (P1.6)', 'route.market_browse', 'view.market_browse'),
	('d1a6f4b5-8b17-4e4c-af9d-5abc3b6ead74', 'e1ebd134-53df-3105-35f4-489fc674d173', 'management_edge-nodes', 3, 45, '/management/edge-nodes', 'mdi:lan-connect', 'self', '["SYS_ADMIN","TENANT_ADMIN"]', '边缘节点', '2026-10-01 14:17:36.718298+08', 'Edge node registry, heartbeat and reconcile console (P1.5)', 'route.management_edge-nodes', 'view.management_edge-nodes'),
	('e2b7a5c6-9c28-4f5d-b0ae-6bcd4c7fbe85', 'e1ebd134-53df-3105-35f4-489fc674d173', 'management_license', 3, 46, '/management/license', 'mdi:license', 'self', '["SYS_ADMIN"]', '许可证', '2026-10-01 14:17:36.718298+08', 'Offline Ed25519 license status view (P3)', 'route.management_license', 'view.management_license'),
	('b1e1a0c2-3d4e-4f50-8162-7a8b9c0d1e04', '0', 'dashboard', 1, 110, '/dashboard', 'mdi:view-dashboard-outline', '1', '["SYS_ADMIN","TENANT_ADMIN"]', '仪表盘', '2026-10-01 14:17:36.718298+08', 'Dashboard parent (hidden until pages are verified)', 'route.dashboard', 'layout.base'),
	('b1e1a0c2-3d4e-4f50-8162-7a8b9c0d1e0c', '0', 'product', 1, 111, '/product', 'mdi:package-variant-closed', '1', '["SYS_ADMIN","TENANT_ADMIN"]', '产品', '2026-10-01 14:17:36.718298+08', 'Product parent (hidden until pages are verified)', 'route.product', 'layout.base'),
	('b1e1a0c2-3d4e-4f50-8162-7a8b9c0d1e01', 'a2c53126-029f-7138-4d7a-f45491f396da', 'apply_service', 3, 33, '/apply/service', 'mdi:application-cog', '1', '["SYS_ADMIN","TENANT_ADMIN"]', '服务', '2026-10-01 14:17:36.718298+08', 'Service management under apply', 'route.apply-service', 'view.apply_service'),
	('b1e1a0c2-3d4e-4f50-8162-7a8b9c0d1e02', '676e8f33-875a-0473-e9ca-c82fd09fef57', 'automation_rule-chain', 3, 1144, '/automation/rule-chain', 'mdi:graph-outline', '1', '["SYS_ADMIN","TENANT_ADMIN"]', '规则链', '2026-10-01 14:17:36.718298+08', 'Rule chain console', 'route.automation-rule-chain', 'view.automation_rule-chain'),
	('b1e1a0c2-3d4e-4f50-8162-7a8b9c0d1e03', '676e8f33-875a-0473-e9ca-c82fd09fef57', 'automation_rule-chain-edit', 3, 1145, '/automation/rule-chain/edit', 'mdi:graph-outline', '1', '["SYS_ADMIN","TENANT_ADMIN"]', '规则链编辑', '2026-10-01 14:17:36.718298+08', 'Rule chain editor', 'route.automation-rule-chain-edit', 'view.automation_rule-chain-edit'),
	('b1e1a0c2-3d4e-4f50-8162-7a8b9c0d1e05', 'b1e1a0c2-3d4e-4f50-8162-7a8b9c0d1e04', 'dashboard_rdi-overview', 3, 1, '/dashboard/rdi-overview', 'mdi:view-dashboard', '1', '["SYS_ADMIN","TENANT_ADMIN"]', 'RDI 总览', '2026-10-01 14:17:36.718298+08', 'RDI dashboard overview', 'route.dashboard-rdi-overview', 'view.dashboard_rdi-overview'),
	('b1e1a0c2-3d4e-4f50-8162-7a8b9c0d1e06', 'b1e1a0c2-3d4e-4f50-8162-7a8b9c0d1e04', 'dashboard_workbench', 3, 2, '/dashboard/workbench', 'mdi:view-dashboard', '1', '["SYS_ADMIN","TENANT_ADMIN"]', '工作台', '2026-10-01 14:17:36.718298+08', 'Dashboard workbench', 'route.dashboard-workbench', 'view.dashboard_workbench'),
	('b1e1a0c2-3d4e-4f50-8162-7a8b9c0d1e07', 'b1e1a0c2-3d4e-4f50-8162-7a8b9c0d1e04', 'dashboard_workspace', 3, 3, '/dashboard/workspace', 'mdi:view-dashboard', '1', '["SYS_ADMIN","TENANT_ADMIN"]', '工作区', '2026-10-01 14:17:36.718298+08', 'Dashboard workspace', 'route.dashboard-workspace', 'view.dashboard_workspace'),
	('b1e1a0c2-3d4e-4f50-8162-7a8b9c0d1e08', '5373a6a2-1861-af35-eb4c-adfd5ca55ecd', 'device_asset', 3, 1180, '/device/asset', 'mdi:domain', '1', '["SYS_ADMIN","TENANT_ADMIN"]', '资产', '2026-10-01 14:17:36.718298+08', 'Device asset management', 'route.device-asset', 'view.device_asset'),
	('b1e1a0c2-3d4e-4f50-8162-7a8b9c0d1e09', '5373a6a2-1861-af35-eb4c-adfd5ca55ecd', 'device_entity-relation', 3, 1181, '/device/entity-relation', 'mdi:relation-many-to-many', '1', '["SYS_ADMIN","TENANT_ADMIN"]', '实体关系', '2026-10-01 14:17:36.718298+08', 'Entity relation management', 'route.device-entity-relation', 'view.device_entity-relation'),
	('b1e1a0c2-3d4e-4f50-8162-7a8b9c0d1e0a', 'e1ebd134-53df-3105-35f4-489fc674d173', 'management_entity-version', 3, 1180, '/management/entity-version', 'mdi:source-branch', '1', '["SYS_ADMIN","TENANT_ADMIN"]', '实体版本', '2026-10-01 14:17:36.718298+08', 'Entity version management', 'route.management-entity-version', 'view.management_entity-version'),
	('b1e1a0c2-3d4e-4f50-8162-7a8b9c0d1e0b', 'e1ebd134-53df-3105-35f4-489fc674d173', 'management_role', 3, 1181, '/management/role', 'mdi:account-key', '1', '["SYS_ADMIN","TENANT_ADMIN"]', '角色', '2026-10-01 14:17:36.718298+08', 'Role management', 'route.management-role', 'view.management_role'),
	('b1e1a0c2-3d4e-4f50-8162-7a8b9c0d1e0d', 'b1e1a0c2-3d4e-4f50-8162-7a8b9c0d1e0c', 'product_pre-register', 3, 1, '/product/pre-register', 'mdi:barcode', '1', '["SYS_ADMIN","TENANT_ADMIN"]', '预注册', '2026-10-01 14:17:36.718298+08', 'Product pre-registration', 'route.product-pre-register', 'view.product_pre-register'),
	('b1e1a0c2-3d4e-4f50-8162-7a8b9c0d1e0e', 'b1e1a0c2-3d4e-4f50-8162-7a8b9c0d1e0c', 'product_update-ota', 3, 2, '/product/update-ota', 'mdi:update', '1', '["SYS_ADMIN","TENANT_ADMIN"]', 'OTA 升级', '2026-10-01 14:17:36.718298+08', 'OTA update management', 'route.product-update-ota', 'view.product_update-ota'),
	('b1e1a0c2-3d4e-4f50-8162-7a8b9c0d1e0f', 'b1e1a0c2-3d4e-4f50-8162-7a8b9c0d1e0c', 'product_update-package', 3, 3, '/product/update-package', 'mdi:package-up', '1', '["SYS_ADMIN","TENANT_ADMIN"]', '升级包', '2026-10-01 14:17:36.718298+08', 'OTA update package management', 'route.product-update-package', 'view.product_update-package'),
	('b1e1a0c2-3d4e-4f50-8162-7a8b9c0d1e10', 'e1ebd134-53df-3105-35f4-489fc674d173', 'system-management-user_equipment-map', 3, 1172, '/system-management-user/equipment-map', 'mdi:map-marker-radius', '1', '["SYS_ADMIN","TENANT_ADMIN"]', '设备地图', '2026-10-01 14:17:36.718298+08', 'Equipment map', 'route.system-management-user-equipment-map', 'view.system-management-user_equipment-map'),
	('b1e1a0c2-3d4e-4f50-8162-7a8b9c0d1e11', '95e2a961-382b-f4a6-87b3-1898123c95bc', 'visualization_scada', 3, 5, '/visualization/scada', 'mdi:monitor-dashboard', '1', '["SYS_ADMIN","TENANT_ADMIN"]', 'SCADA', '2026-10-01 14:17:36.718298+08', 'SCADA board list', 'route.visualization-scada', 'view.visualization_scada'),
	('b1e1a0c2-3d4e-4f50-8162-7a8b9c0d1e12', '95e2a961-382b-f4a6-87b3-1898123c95bc', 'visualization_scada-editor', 3, 6, '/visualization/scada-editor', 'mdi:monitor-edit', '1', '["SYS_ADMIN","TENANT_ADMIN"]', 'SCADA 编辑器', '2026-10-01 14:17:36.718298+08', 'SCADA editor (symbol library + drag and drop)', 'route.visualization-scada-editor', 'view.visualization_scada-editor'),
	('99e1c4a0-5b12-4cf3-911e-8e4f1a2b3c4d', 'e1ebd134-53df-3105-35f4-489fc674d173', 'management_secrets', 3, 47, '/management/secrets', 'mdi:key-variant', 'self', '["SYS_ADMIN", "TENANT_ADMIN"]', 'Universal Secrets Storage (TB-18)', '2026-10-01 14:17:36.718298+08', '', 'route.management_secrets', 'view.management_secrets'),
	('a1b2c3d4-11a1-4bb2-9cc3-114d4e5f6a7b', 'e1ebd134-53df-3105-35f4-489fc674d173', 'management_solutions', 3, 48, '/management/solutions', 'mdi:package-variant-closed', 'self', '["SYS_ADMIN", "TENANT_ADMIN"]', 'Industry Solution Templates (TB-19)', '2026-10-01 14:17:36.718298+08', '', 'route.management_solutions', 'view.management_solutions'),
	('f3a8b6c7-2d14-4e5a-9f80-6b1c2d3e4f50', '0', 'customer', 1, 117, '/customer', 'mdi:card-account-details-outline', 'self', '["SYS_ADMIN","TENANT_ADMIN"]', '客户管理', '2026-10-01 14:17:36.718298+08', 'Customer management (ThingsBoard customers parity)', 'route.customer', 'layout.base'),
	('a4b7c8d9-3e25-4f6b-8a91-7c2d3e4f5a61', 'f3a8b6c7-2d14-4e5a-9f80-6b1c2d3e4f50', 'customer_list', 3, 1, '/customer/list', 'mdi:format-list-bulleted', '0', '["SYS_ADMIN","TENANT_ADMIN"]', '客户列表', '2026-10-01 14:17:36.718298+08', 'Customer list with device assignment workbench', 'route.customer_list', 'view.customer_list'),
	('d1e6f7a8-4b37-5c48-9ba2-8d3e4f5a6b72', '95e2a961-382b-f4a6-87b3-1898123c95bc', 'visualization_widget-bundles', 3, 5, '/visualization/widget-bundles', 'mdi:view-dashboard-edit-outline', '0', '["SYS_ADMIN","TENANT_ADMIN"]', '部件库', '2026-10-01 14:17:36.718298+08', 'Widget bundle library with builtin seed import (TB-04)', 'route.visualization-widget-bundles', 'view.visualization_widget-bundles'),
	('b3f9d2e1-7a46-4c58-9b0d-8e2f1a3c5d60', '0', 'billing', 1, 118, '/billing', 'mdi:chart-box-outline', 'self', '["SYS_ADMIN","TENANT_ADMIN"]', '配额计费', '2026-10-01 14:17:36.718298+08', 'Billing & API quota console (TB-17, ThingsBoard PE per-tenant quotas parity)', 'route.billing', 'layout.base'),
	('c4a8e3f2-8b57-4d69-9c1e-9f3a2b4d6e71', 'b3f9d2e1-7a46-4c58-9b0d-8e2f1a3c5d60', 'billing_api-quota', 3, 1, '/billing/api-quota', 'mdi:gauge', '0', '["SYS_ADMIN","TENANT_ADMIN"]', 'API 配额', '2026-10-01 14:17:36.718298+08', 'Per-tenant daily API quota usage: calls today / limit / remaining (TB-17)', 'route.billing_api-quota', 'view.billing_api-quota'),
	('b7a1c8d2-5e39-4f60-9ab4-1c6d7e8f9a04', '0', 'media', 1, 118, '/media', 'mdi:image-multiple-outline', 'self', '["SYS_ADMIN","TENANT_ADMIN"]', '媒体库', '2026-10-01 14:17:36.718298+08', 'Media library over ./files uploads (TB-41)', 'route.media', 'layout.base'),
	('c2d9e0f1-6a48-5b59-8cb3-9e4f5a6b7c83', 'b7a1c8d2-5e39-4f60-9ab4-1c6d7e8f9a04', 'media_library', 3, 1, '/media/library', 'mdi:image-multiple-outline', '0', '["SYS_ADMIN","TENANT_ADMIN"]', '媒体库', '2026-10-01 14:17:36.718298+08', 'Grid preview, upload and reference-aware delete (TB-41)', 'route.media_library', 'view.media_library'),
	('d4e5f6a7-8b29-4c30-9d41-2e3f4a5b6c72', '0', 'integration', 1, 121, '/integration', 'mdi:plug-socket', 'self', '["SYS_ADMIN","TENANT_ADMIN"]', '统一集成', '2026-10-01 14:17:36.718298+08', 'Unified integrations management (TB-45)', 'route.integration', 'layout.base'),
	('e5f6a7b8-9c3a-4d41-ae52-3f4a5b6c7d83', 'd4e5f6a7-8b29-4c30-9d41-2e3f4a5b6c72', 'integration_list', 3, 1, '/integration/list', 'mdi:plug-socket', '0', '["SYS_ADMIN","TENANT_ADMIN"]', '集成管理', '2026-10-01 14:17:36.718298+08', 'Integration instances with converter binding (TB-45)', 'route.integration_list', 'view.integration_list'),
	('d3a1b4c8-6e27-4f59-8a70-9b2c3d4e5f61', 'e1ebd134-53df-3105-35f4-489fc674d173', 'management_user-group', 3, 1182, '/management/user-group', 'mdi:account-group', '1', '["SYS_ADMIN","TENANT_ADMIN"]', '用户组', '2026-10-01 14:17:36.718298+08', 'User groups and group permission elements (TB-46 GPE v1)', 'route.management_user-group', 'view.management_user-group'),
	('e5f6a7b8-9c01-4d23-8e45-6f7a8b9c0d11', '0', 'mobile-app', 1, 119, '/mobile-app', 'mdi:cellphone-arrow-down', 'self', '["SYS_ADMIN","TENANT_ADMIN"]', '移动应用中心', '2026-10-01 14:17:36.718298+08', 'Mobile app bundles: version list, upload, publish and archive (TB-23)', 'route.mobile-app', 'layout.base'),
	('e5f6a7b8-9c01-4d23-8e45-6f7a8b9c0d12', 'e5f6a7b8-9c01-4d23-8e45-6f7a8b9c0d11', 'mobile-app_app-center', 3, 1, '/mobile-app/app-center', 'mdi:cellphone-arrow-down', '0', '["SYS_ADMIN","TENANT_ADMIN"]', '应用中心', '2026-10-01 14:17:36.718298+08', 'Bundle version list with upload/publish/archive state machine (TB-23)', 'route.mobile-app_app-center', 'view.mobile-app_app-center'),
	('f0a1b2c3-9d04-4e56-9f67-8a9b0c1d2e33', '0', 'scheduler', 1, 121, '/scheduler', 'mdi:calendar-clock', 'self', '["SYS_ADMIN","TENANT_ADMIN"]', '统一调度', '2026-10-01 14:17:36.718298+08', 'Unified scheduler calendar over scene timers, report schedules and fleet command jobs (TB-48)', 'route.scheduler', 'layout.base'),
	('f0a1b2c3-9d04-4e56-9f67-8a9b0c1d2e34', 'f0a1b2c3-9d04-4e56-9f67-8a9b0c1d2e33', 'scheduler_calendar', 3, 1, '/scheduler/calendar', 'mdi:calendar-clock', '0', '["SYS_ADMIN","TENANT_ADMIN"]', '调度日历', '2026-10-01 14:17:36.718298+08', 'Month-grid calendar aggregating scheduled events by source type (TB-48)', 'route.scheduler_calendar', 'view.scheduler_calendar');


--
-- Data for Name: telemetry_current_datas; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: telemetry_datas; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: telemetry_dead_letters; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: telemetry_rollups; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: telemetry_set_logs; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: tenant_custom_css; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: tenant_dashboard_menus; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: tenant_oidc_providers; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: tenant_rate_limits; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: tenant_subscriptions; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: tenant_translations; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: tenants; Type: TABLE DATA; Schema: public; Owner: -
--

INSERT INTO public.tenants (id, name, parent_tenant_id, created_at, updated_at) VALUES
	('d616bcbb', 'Disabled Tenant Sample', '', '2026-10-01 14:17:36.718298+08', '2026-10-01 14:17:36.718298+08');


--
-- Data for Name: uplink_storage_dead_letters; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: uplink_storage_receipts; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: user_address; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: user_groups; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: user_totp; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: user_totp_recovery_codes; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: users; Type: TABLE DATA; Schema: public; Owner: -
--

INSERT INTO public.users (id, name, phone_number, email, status, authority, password, tenant_id, remark, additional_info, created_at, updated_at, password_last_updated, last_visit_time, last_visit_ip, last_visit_device, organization, timezone, default_language, password_fail_count, avatar_url) VALUES
	('11111111-4fe9-b409-67c3-111111111111', 'Disabled Tenant Sample', '+86 13166666666', 'disabled.tenant@example.invalid', 'F', 'TENANT_ADMIN', 'DISABLED_PUBLIC_SEED_ACCOUNT_NO_LOGIN', 'd616bcbb', 'disabled public seed sample', '{}', '2024-06-05 16:48:11.097+08', '2024-06-05 16:48:11.097+08', NULL, NULL, NULL, NULL, NULL, NULL, NULL, 0, NULL);


--
-- Data for Name: vis_dashboard; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: vis_files; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: vis_plugin; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: widget_bundles; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Name: casbin_rule_id_seq; Type: SEQUENCE SET; Schema: public; Owner: -
--

SELECT pg_catalog.setval('public.casbin_rule_id_seq', 1778, true);


--
-- Name: device_status_history_id_seq; Type: SEQUENCE SET; Schema: public; Owner: -
--

SELECT pg_catalog.setval('public.device_status_history_id_seq', 1, false);


--
-- Name: device_topic_mappings_id_seq; Type: SEQUENCE SET; Schema: public; Owner: -
--

SELECT pg_catalog.setval('public.device_topic_mappings_id_seq', 1, false);


--
-- Name: user_address_id_seq; Type: SEQUENCE SET; Schema: public; Owner: -
--

SELECT pg_catalog.setval('public.user_address_id_seq', 1, false);


--
-- Name: action_info action_info_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.action_info
    ADD CONSTRAINT action_info_pkey PRIMARY KEY (id);


--
-- Name: ai_models ai_models_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.ai_models
    ADD CONSTRAINT ai_models_pkey PRIMARY KEY (id);


--
-- Name: alarm_assignment alarm_assignment_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.alarm_assignment
    ADD CONSTRAINT alarm_assignment_pkey PRIMARY KEY (id);


--
-- Name: alarm_comment alarm_comment_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.alarm_comment
    ADD CONSTRAINT alarm_comment_pkey PRIMARY KEY (id);


--
-- Name: alarm_config alarm_config_pk; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.alarm_config
    ADD CONSTRAINT alarm_config_pk PRIMARY KEY (id);


--
-- Name: alarm_history_devices alarm_history_devices_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.alarm_history_devices
    ADD CONSTRAINT alarm_history_devices_pkey PRIMARY KEY (alarm_history_id, device_id);


--
-- Name: alarm_history alarm_history_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.alarm_history
    ADD CONSTRAINT alarm_history_pkey PRIMARY KEY (id);


--
-- Name: alarm_info alarm_info_pk; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.alarm_info
    ADD CONSTRAINT alarm_info_pk PRIMARY KEY (id);


--
-- Name: assets assets_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.assets
    ADD CONSTRAINT assets_pkey PRIMARY KEY (id);


--
-- Name: attribute_datas attribute_datas_device_id_key_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.attribute_datas
    ADD CONSTRAINT attribute_datas_device_id_key_key UNIQUE (device_id, key);


--
-- Name: attribute_set_logs attribute_set_logs_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.attribute_set_logs
    ADD CONSTRAINT attribute_set_logs_pkey PRIMARY KEY (id);


--
-- Name: board_project_members board_project_members_pk; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.board_project_members
    ADD CONSTRAINT board_project_members_pk PRIMARY KEY (project_id, board_id);


--
-- Name: board_projects board_projects_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.board_projects
    ADD CONSTRAINT board_projects_pkey PRIMARY KEY (id);


--
-- Name: board_projects board_projects_tenant_name_uk; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.board_projects
    ADD CONSTRAINT board_projects_tenant_name_uk UNIQUE (tenant_id, name);


--
-- Name: boards boards_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.boards
    ADD CONSTRAINT boards_pkey PRIMARY KEY (id);


--
-- Name: calcfield_recompute_tasks calcfield_recompute_tasks_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.calcfield_recompute_tasks
    ADD CONSTRAINT calcfield_recompute_tasks_pkey PRIMARY KEY (id);


--
-- Name: calculated_fields calculated_fields_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.calculated_fields
    ADD CONSTRAINT calculated_fields_pkey PRIMARY KEY (id);


--
-- Name: casbin_rule casbin_rule_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.casbin_rule
    ADD CONSTRAINT casbin_rule_pkey PRIMARY KEY (id);


--
-- Name: command_job_details command_job_details_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.command_job_details
    ADD CONSTRAINT command_job_details_pkey PRIMARY KEY (id);


--
-- Name: command_job_dispatch_quotas command_job_dispatch_quotas_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.command_job_dispatch_quotas
    ADD CONSTRAINT command_job_dispatch_quotas_pkey PRIMARY KEY (scope_type, scope_id);


--
-- Name: command_job_events command_job_events_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.command_job_events
    ADD CONSTRAINT command_job_events_pkey PRIMARY KEY (id);


--
-- Name: command_jobs command_jobs_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.command_jobs
    ADD CONSTRAINT command_jobs_pkey PRIMARY KEY (id);


--
-- Name: command_set_logs command_set_logs_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.command_set_logs
    ADD CONSTRAINT command_set_logs_pkey PRIMARY KEY (id);


--
-- Name: customer_devices customer_devices_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.customer_devices
    ADD CONSTRAINT customer_devices_pkey PRIMARY KEY (id);


--
-- Name: customers customers_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.customers
    ADD CONSTRAINT customers_pkey PRIMARY KEY (id);


--
-- Name: data_converters data_converters_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.data_converters
    ADD CONSTRAINT data_converters_pkey PRIMARY KEY (id);


--
-- Name: data_policy data_policy_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.data_policy
    ADD CONSTRAINT data_policy_pkey PRIMARY KEY (id);


--
-- Name: data_retention_registry data_retention_registry_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.data_retention_registry
    ADD CONSTRAINT data_retention_registry_pkey PRIMARY KEY (id);


--
-- Name: data_retention_registry data_retention_registry_table_uq; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.data_retention_registry
    ADD CONSTRAINT data_retention_registry_table_uq UNIQUE (table_name);


--
-- Name: data_scripts data_scripts_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.data_scripts
    ADD CONSTRAINT data_scripts_pkey PRIMARY KEY (id);


--
-- Name: device_certificates device_certificates_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.device_certificates
    ADD CONSTRAINT device_certificates_pkey PRIMARY KEY (id);


--
-- Name: device_claim_tokens device_claim_tokens_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.device_claim_tokens
    ADD CONSTRAINT device_claim_tokens_pkey PRIMARY KEY (id);


--
-- Name: device_configs device_configs_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.device_configs
    ADD CONSTRAINT device_configs_pkey PRIMARY KEY (id);


--
-- Name: device_health_scores device_health_scores_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.device_health_scores
    ADD CONSTRAINT device_health_scores_pkey PRIMARY KEY (id);


--
-- Name: device_modbus_profiles device_modbus_profiles_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.device_modbus_profiles
    ADD CONSTRAINT device_modbus_profiles_pkey PRIMARY KEY (device_id);


--
-- Name: device_model_attributes device_model_attributes_unique; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.device_model_attributes
    ADD CONSTRAINT device_model_attributes_unique UNIQUE (device_template_id, data_identifier);


--
-- Name: device_model_commands device_model_commands_unique; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.device_model_commands
    ADD CONSTRAINT device_model_commands_unique UNIQUE (data_identifier, device_template_id);


--
-- Name: device_model_custom_commands device_model_custom_commands_pk; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.device_model_custom_commands
    ADD CONSTRAINT device_model_custom_commands_pk PRIMARY KEY (id);


--
-- Name: device_model_custom_control device_model_custom_control_pk; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.device_model_custom_control
    ADD CONSTRAINT device_model_custom_control_pk PRIMARY KEY (id);


--
-- Name: device_model_events device_model_events_unique; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.device_model_events
    ADD CONSTRAINT device_model_events_unique UNIQUE (device_template_id, data_identifier);


--
-- Name: device_model_attributes device_model_telemetry_copy1_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.device_model_attributes
    ADD CONSTRAINT device_model_telemetry_copy1_pkey PRIMARY KEY (id);


--
-- Name: device_model_events device_model_telemetry_copy1_pkey1; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.device_model_events
    ADD CONSTRAINT device_model_telemetry_copy1_pkey1 PRIMARY KEY (id);


--
-- Name: device_model_commands device_model_telemetry_copy1_pkey2; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.device_model_commands
    ADD CONSTRAINT device_model_telemetry_copy1_pkey2 PRIMARY KEY (id);


--
-- Name: device_model_telemetry device_model_telemetry_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.device_model_telemetry
    ADD CONSTRAINT device_model_telemetry_pkey PRIMARY KEY (id);


--
-- Name: device_model_telemetry device_model_telemetry_unique; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.device_model_telemetry
    ADD CONSTRAINT device_model_telemetry_unique UNIQUE (device_template_id, data_identifier);


--
-- Name: devices device_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.devices
    ADD CONSTRAINT device_pkey PRIMARY KEY (id);


--
-- Name: device_pre_register_credential_grants device_pre_register_credential_grants_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.device_pre_register_credential_grants
    ADD CONSTRAINT device_pre_register_credential_grants_pkey PRIMARY KEY (id);


--
-- Name: device_shadow_messages device_shadow_messages_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.device_shadow_messages
    ADD CONSTRAINT device_shadow_messages_pkey PRIMARY KEY (id);


--
-- Name: device_status_history device_status_history_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.device_status_history
    ADD CONSTRAINT device_status_history_pkey PRIMARY KEY (id);


--
-- Name: device_template_upgrade_history device_template_upgrade_history_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.device_template_upgrade_history
    ADD CONSTRAINT device_template_upgrade_history_pkey PRIMARY KEY (id);


--
-- Name: device_templates device_templates_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.device_templates
    ADD CONSTRAINT device_templates_pkey PRIMARY KEY (id);


--
-- Name: device_topic_mappings device_topic_mappings_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.device_topic_mappings
    ADD CONSTRAINT device_topic_mappings_pkey PRIMARY KEY (id);


--
-- Name: device_trigger_condition device_trigger_condition_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.device_trigger_condition
    ADD CONSTRAINT device_trigger_condition_pkey PRIMARY KEY (id);


--
-- Name: device_user_logs device_user_logs_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.device_user_logs
    ADD CONSTRAINT device_user_logs_pkey PRIMARY KEY (id);


--
-- Name: devices devices_unique; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.devices
    ADD CONSTRAINT devices_unique UNIQUE (device_number);


--
-- Name: edge_node_certificates edge_node_certificates_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.edge_node_certificates
    ADD CONSTRAINT edge_node_certificates_pkey PRIMARY KEY (id);


--
-- Name: edge_node_upgrade_history edge_node_upgrade_history_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.edge_node_upgrade_history
    ADD CONSTRAINT edge_node_upgrade_history_pkey PRIMARY KEY (id);


--
-- Name: edge_nodes edge_nodes_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.edge_nodes
    ADD CONSTRAINT edge_nodes_pkey PRIMARY KEY (id);


--
-- Name: edge_sync_tasks edge_sync_tasks_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.edge_sync_tasks
    ADD CONSTRAINT edge_sync_tasks_pkey PRIMARY KEY (id);


--
-- Name: email_templates email_templates_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.email_templates
    ADD CONSTRAINT email_templates_pkey PRIMARY KEY (id);


--
-- Name: entity_relations entity_relations_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.entity_relations
    ADD CONSTRAINT entity_relations_pkey PRIMARY KEY (id);


--
-- Name: entity_relations entity_relations_unique_edge; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.entity_relations
    ADD CONSTRAINT entity_relations_unique_edge UNIQUE (tenant_id, from_type, from_id, relation_type, to_type, to_id);


--
-- Name: entity_versions entity_versions_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.entity_versions
    ADD CONSTRAINT entity_versions_pkey PRIMARY KEY (id);


--
-- Name: event_datas event_datas_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.event_datas
    ADD CONSTRAINT event_datas_pkey PRIMARY KEY (id);


--
-- Name: expected_datas expected_datas_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.expected_datas
    ADD CONSTRAINT expected_datas_pkey PRIMARY KEY (id);


--
-- Name: fleet_saved_filters fleet_saved_filters_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.fleet_saved_filters
    ADD CONSTRAINT fleet_saved_filters_pkey PRIMARY KEY (id);


--
-- Name: fleet_saved_filters fleet_saved_filters_user_name_unique; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.fleet_saved_filters
    ADD CONSTRAINT fleet_saved_filters_user_name_unique UNIQUE (tenant_id, user_id, name);


--
-- Name: group_permissions group_permissions_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.group_permissions
    ADD CONSTRAINT group_permissions_pkey PRIMARY KEY (id);


--
-- Name: groups groups_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.groups
    ADD CONSTRAINT groups_pkey PRIMARY KEY (id);


--
-- Name: industry_solution_installs industry_solution_installs_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.industry_solution_installs
    ADD CONSTRAINT industry_solution_installs_pkey PRIMARY KEY (id);


--
-- Name: industry_solutions industry_solutions_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.industry_solutions
    ADD CONSTRAINT industry_solutions_pkey PRIMARY KEY (id);


--
-- Name: integrations integrations_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.integrations
    ADD CONSTRAINT integrations_pkey PRIMARY KEY (id);


--
-- Name: logo logo_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.logo
    ADD CONSTRAINT logo_pkey PRIMARY KEY (id);


--
-- Name: media_files media_files_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.media_files
    ADD CONSTRAINT media_files_pkey PRIMARY KEY (id);


--
-- Name: message_push_rule_log message_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.message_push_rule_log
    ADD CONSTRAINT message_pkey PRIMARY KEY (id);


--
-- Name: message_push_config message_push_config_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.message_push_config
    ADD CONSTRAINT message_push_config_pkey PRIMARY KEY (id);


--
-- Name: message_push_log message_push_log_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.message_push_log
    ADD CONSTRAINT message_push_log_pkey PRIMARY KEY (id);


--
-- Name: message_push_manage message_push_manage_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.message_push_manage
    ADD CONSTRAINT message_push_manage_pkey PRIMARY KEY (id);


--
-- Name: mobile_app_bundles mobile_app_bundles_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.mobile_app_bundles
    ADD CONSTRAINT mobile_app_bundles_pkey PRIMARY KEY (id);


--
-- Name: mqtt_session_revocation_acks mqtt_session_revocation_acks_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.mqtt_session_revocation_acks
    ADD CONSTRAINT mqtt_session_revocation_acks_pkey PRIMARY KEY (event_id, broker_id);


--
-- Name: mqtt_session_revocation_outbox mqtt_session_revocation_outbox_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.mqtt_session_revocation_outbox
    ADD CONSTRAINT mqtt_session_revocation_outbox_pkey PRIMARY KEY (id);


--
-- Name: notification_groups notification_groups_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.notification_groups
    ADD CONSTRAINT notification_groups_pkey PRIMARY KEY (id);


--
-- Name: notification_histories notification_histories_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.notification_histories
    ADD CONSTRAINT notification_histories_pkey PRIMARY KEY (id);


--
-- Name: notification_history_devices notification_history_devices_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.notification_history_devices
    ADD CONSTRAINT notification_history_devices_pkey PRIMARY KEY (notification_history_id, device_id);


--
-- Name: notification_services_config notification_services_config_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.notification_services_config
    ADD CONSTRAINT notification_services_config_pkey PRIMARY KEY (id);


--
-- Name: one_time_tasks one_time_tasks_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.one_time_tasks
    ADD CONSTRAINT one_time_tasks_pkey PRIMARY KEY (id);


--
-- Name: open_api_keys open_api_keys_app_key_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.open_api_keys
    ADD CONSTRAINT open_api_keys_app_key_key UNIQUE (api_key);


--
-- Name: open_api_keys open_api_keys_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.open_api_keys
    ADD CONSTRAINT open_api_keys_pkey PRIMARY KEY (id);


--
-- Name: operation_logs operation_logs_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.operation_logs
    ADD CONSTRAINT operation_logs_pkey PRIMARY KEY (id);


--
-- Name: ota_upgrade_packages ota_upgrade_packages_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.ota_upgrade_packages
    ADD CONSTRAINT ota_upgrade_packages_pkey PRIMARY KEY (id);


--
-- Name: ota_upgrade_task_details ota_upgrade_task_details_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.ota_upgrade_task_details
    ADD CONSTRAINT ota_upgrade_task_details_pkey PRIMARY KEY (id);


--
-- Name: ota_upgrade_tasks ota_upgrade_tasks_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.ota_upgrade_tasks
    ADD CONSTRAINT ota_upgrade_tasks_pkey PRIMARY KEY (id);


--
-- Name: payload_schemas payload_schemas_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.payload_schemas
    ADD CONSTRAINT payload_schemas_pkey PRIMARY KEY (id);


--
-- Name: payload_schemas payload_schemas_tenant_name_unique; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.payload_schemas
    ADD CONSTRAINT payload_schemas_tenant_name_unique UNIQUE (tenant_id, name);


--
-- Name: periodic_tasks periodic_tasks_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.periodic_tasks
    ADD CONSTRAINT periodic_tasks_pkey PRIMARY KEY (id);


--
-- Name: platform_cas platform_cas_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.platform_cas
    ADD CONSTRAINT platform_cas_pkey PRIMARY KEY (id);


--
-- Name: plugin_registries plugin_registries_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.plugin_registries
    ADD CONSTRAINT plugin_registries_pkey PRIMARY KEY (id);


--
-- Name: products products_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.products
    ADD CONSTRAINT products_pkey PRIMARY KEY (id);


--
-- Name: protocol_plugins protocol_plugins_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.protocol_plugins
    ADD CONSTRAINT protocol_plugins_pkey PRIMARY KEY (id);


--
-- Name: push_deliveries push_deliveries_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.push_deliveries
    ADD CONSTRAINT push_deliveries_pkey PRIMARY KEY (id);


--
-- Name: push_device_registrations push_device_registrations_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.push_device_registrations
    ADD CONSTRAINT push_device_registrations_pkey PRIMARY KEY (id);


--
-- Name: push_device_registrations push_device_registrations_unique; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.push_device_registrations
    ADD CONSTRAINT push_device_registrations_unique UNIQUE (tenant_id, user_id, platform, token);


--
-- Name: r_group_device r_group_device_group_id_device_id_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.r_group_device
    ADD CONSTRAINT r_group_device_group_id_device_id_key UNIQUE (group_id, device_id);


--
-- Name: report_schedule_deliveries report_schedule_deliveries_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.report_schedule_deliveries
    ADD CONSTRAINT report_schedule_deliveries_pkey PRIMARY KEY (run_id);


--
-- Name: report_schedule_runs report_schedule_runs_identity_unique; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.report_schedule_runs
    ADD CONSTRAINT report_schedule_runs_identity_unique UNIQUE (id, tenant_id, schedule_id);


--
-- Name: report_schedule_runs report_schedule_runs_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.report_schedule_runs
    ADD CONSTRAINT report_schedule_runs_pkey PRIMARY KEY (id);


--
-- Name: report_schedule_runs report_schedule_runs_tenant_identity_unique; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.report_schedule_runs
    ADD CONSTRAINT report_schedule_runs_tenant_identity_unique UNIQUE (id, tenant_id);


--
-- Name: report_schedules report_schedules_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.report_schedules
    ADD CONSTRAINT report_schedules_pkey PRIMARY KEY (id);


--
-- Name: roles roles_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.roles
    ADD CONSTRAINT roles_pkey PRIMARY KEY (id);


--
-- Name: rule_chain_checkpoints rule_chain_checkpoints_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.rule_chain_checkpoints
    ADD CONSTRAINT rule_chain_checkpoints_pkey PRIMARY KEY (id);


--
-- Name: rule_chain_dead_letters rule_chain_dead_letters_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.rule_chain_dead_letters
    ADD CONSTRAINT rule_chain_dead_letters_pkey PRIMARY KEY (id);


--
-- Name: rule_chain_edges rule_chain_edges_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.rule_chain_edges
    ADD CONSTRAINT rule_chain_edges_pkey PRIMARY KEY (id);


--
-- Name: rule_chain_node_traces rule_chain_node_traces_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.rule_chain_node_traces
    ADD CONSTRAINT rule_chain_node_traces_pkey PRIMARY KEY (id);


--
-- Name: rule_chain_nodes rule_chain_nodes_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.rule_chain_nodes
    ADD CONSTRAINT rule_chain_nodes_pkey PRIMARY KEY (id);


--
-- Name: rule_chain_replay_records rule_chain_replay_records_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.rule_chain_replay_records
    ADD CONSTRAINT rule_chain_replay_records_pkey PRIMARY KEY (id);


--
-- Name: rule_chain_versions rule_chain_versions_chain_version_unique; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.rule_chain_versions
    ADD CONSTRAINT rule_chain_versions_chain_version_unique UNIQUE (chain_id, version);


--
-- Name: rule_chain_versions rule_chain_versions_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.rule_chain_versions
    ADD CONSTRAINT rule_chain_versions_pkey PRIMARY KEY (id);


--
-- Name: rule_chains rule_chains_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.rule_chains
    ADD CONSTRAINT rule_chains_pkey PRIMARY KEY (id);


--
-- Name: scada_control_audits scada_control_audits_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.scada_control_audits
    ADD CONSTRAINT scada_control_audits_pkey PRIMARY KEY (id);


--
-- Name: scada_document_versions scada_document_versions_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.scada_document_versions
    ADD CONSTRAINT scada_document_versions_pkey PRIMARY KEY (id);


--
-- Name: scada_document_versions scada_document_versions_unique; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.scada_document_versions
    ADD CONSTRAINT scada_document_versions_unique UNIQUE (tenant_id, document_id, version);


--
-- Name: scada_documents scada_documents_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.scada_documents
    ADD CONSTRAINT scada_documents_pkey PRIMARY KEY (id);


--
-- Name: scada_documents scada_documents_project_name_unique; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.scada_documents
    ADD CONSTRAINT scada_documents_project_name_unique UNIQUE (tenant_id, project_id, name);


--
-- Name: scada_projects scada_projects_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.scada_projects
    ADD CONSTRAINT scada_projects_pkey PRIMARY KEY (id);


--
-- Name: scada_projects scada_projects_tenant_name_unique; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.scada_projects
    ADD CONSTRAINT scada_projects_tenant_name_unique UNIQUE (tenant_id, name);


--
-- Name: scene_action_info scene_action_info_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.scene_action_info
    ADD CONSTRAINT scene_action_info_pkey PRIMARY KEY (id);


--
-- Name: scene_automation_timers scene_automation_timers_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.scene_automation_timers
    ADD CONSTRAINT scene_automation_timers_pkey PRIMARY KEY (id);


--
-- Name: scene_automations scene_automations_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.scene_automations
    ADD CONSTRAINT scene_automations_pkey PRIMARY KEY (id);


--
-- Name: scene_info scene_info_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.scene_info
    ADD CONSTRAINT scene_info_pkey PRIMARY KEY (id);


--
-- Name: scene_log scene_log_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.scene_log
    ADD CONSTRAINT scene_log_pkey PRIMARY KEY (id);


--
-- Name: scheduler_events scheduler_events_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.scheduler_events
    ADD CONSTRAINT scheduler_events_pkey PRIMARY KEY (id);


--
-- Name: service_access service_access_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.service_access
    ADD CONSTRAINT service_access_pkey PRIMARY KEY (id);


--
-- Name: service_plugins service_plugins_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.service_plugins
    ADD CONSTRAINT service_plugins_pkey PRIMARY KEY (id);


--
-- Name: subscription_plans subscription_plans_code_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.subscription_plans
    ADD CONSTRAINT subscription_plans_code_key UNIQUE (code);


--
-- Name: subscription_plans subscription_plans_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.subscription_plans
    ADD CONSTRAINT subscription_plans_pkey PRIMARY KEY (id);


--
-- Name: sys_config sys_config_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.sys_config
    ADD CONSTRAINT sys_config_pkey PRIMARY KEY (config_key);


--
-- Name: sys_dict sys_dict_dict_code_dict_value_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.sys_dict
    ADD CONSTRAINT sys_dict_dict_code_dict_value_key UNIQUE (dict_code, dict_value);


--
-- Name: CONSTRAINT sys_dict_dict_code_dict_value_key ON sys_dict; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON CONSTRAINT sys_dict_dict_code_dict_value_key ON public.sys_dict IS 'dict_code和dict_value唯一';


--
-- Name: sys_dict_language sys_dict_language_dict_id_language_code_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.sys_dict_language
    ADD CONSTRAINT sys_dict_language_dict_id_language_code_key UNIQUE (dict_id, language_code);


--
-- Name: CONSTRAINT sys_dict_language_dict_id_language_code_key ON sys_dict_language; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON CONSTRAINT sys_dict_language_dict_id_language_code_key ON public.sys_dict_language IS 'dict_id和language_code唯一';


--
-- Name: sys_dict_language sys_dict_language_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.sys_dict_language
    ADD CONSTRAINT sys_dict_language_pkey PRIMARY KEY (id);


--
-- Name: sys_dict sys_dict_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.sys_dict
    ADD CONSTRAINT sys_dict_pkey PRIMARY KEY (id);


--
-- Name: sys_function sys_function_pk; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.sys_function
    ADD CONSTRAINT sys_function_pk PRIMARY KEY (id);


--
-- Name: sys_permissions sys_permissions_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.sys_permissions
    ADD CONSTRAINT sys_permissions_pkey PRIMARY KEY (code);


--
-- Name: sys_role_permissions sys_role_permissions_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.sys_role_permissions
    ADD CONSTRAINT sys_role_permissions_pkey PRIMARY KEY (id);


--
-- Name: sys_secrets sys_secrets_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.sys_secrets
    ADD CONSTRAINT sys_secrets_pkey PRIMARY KEY (id);


--
-- Name: sys_ui_elements sys_ui_elements_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.sys_ui_elements
    ADD CONSTRAINT sys_ui_elements_pkey PRIMARY KEY (id);


--
-- Name: telemetry_current_datas telemetry_current_datas_unique; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.telemetry_current_datas
    ADD CONSTRAINT telemetry_current_datas_unique UNIQUE (device_id, key);


--
-- Name: telemetry_datas telemetry_datas_device_id_key_ts_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.telemetry_datas
    ADD CONSTRAINT telemetry_datas_device_id_key_ts_key UNIQUE (device_id, key, ts);


--
-- Name: telemetry_dead_letters telemetry_dead_letters_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.telemetry_dead_letters
    ADD CONSTRAINT telemetry_dead_letters_pkey PRIMARY KEY (id);


--
-- Name: telemetry_rollups telemetry_rollups_pk; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.telemetry_rollups
    ADD CONSTRAINT telemetry_rollups_pk PRIMARY KEY (device_id, key, bucket_ms, bucket_start);


--
-- Name: telemetry_set_logs telemetry_set_logs_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.telemetry_set_logs
    ADD CONSTRAINT telemetry_set_logs_pkey PRIMARY KEY (id);


--
-- Name: tenant_custom_css tenant_custom_css_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.tenant_custom_css
    ADD CONSTRAINT tenant_custom_css_pkey PRIMARY KEY (tenant_id);


--
-- Name: tenant_dashboard_menus tenant_dashboard_menus_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.tenant_dashboard_menus
    ADD CONSTRAINT tenant_dashboard_menus_pkey PRIMARY KEY (id);


--
-- Name: tenant_oidc_providers tenant_oidc_providers_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.tenant_oidc_providers
    ADD CONSTRAINT tenant_oidc_providers_pkey PRIMARY KEY (id);


--
-- Name: tenant_rate_limits tenant_rate_limits_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.tenant_rate_limits
    ADD CONSTRAINT tenant_rate_limits_pkey PRIMARY KEY (id);


--
-- Name: tenant_subscriptions tenant_subscriptions_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.tenant_subscriptions
    ADD CONSTRAINT tenant_subscriptions_pkey PRIMARY KEY (id);


--
-- Name: tenant_subscriptions tenant_subscriptions_tenant_id_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.tenant_subscriptions
    ADD CONSTRAINT tenant_subscriptions_tenant_id_key UNIQUE (tenant_id);


--
-- Name: tenant_translations tenant_translations_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.tenant_translations
    ADD CONSTRAINT tenant_translations_pkey PRIMARY KEY (id);


--
-- Name: tenants tenants_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.tenants
    ADD CONSTRAINT tenants_pkey PRIMARY KEY (id);


--
-- Name: vis_files tp_vis_files_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.vis_files
    ADD CONSTRAINT tp_vis_files_pkey PRIMARY KEY (id);


--
-- Name: vis_plugin tp_vis_plugin_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.vis_plugin
    ADD CONSTRAINT tp_vis_plugin_pkey PRIMARY KEY (id);


--
-- Name: api_usage_daily uk_api_usage_daily_tenant_date; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.api_usage_daily
    ADD CONSTRAINT uk_api_usage_daily_tenant_date UNIQUE (tenant_id, usage_date);


--
-- Name: customer_devices uk_customer_devices_tenant_device; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.customer_devices
    ADD CONSTRAINT uk_customer_devices_tenant_device UNIQUE (tenant_id, device_id);


--
-- Name: customers uk_customers_tenant_name; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.customers
    ADD CONSTRAINT uk_customers_tenant_name UNIQUE (tenant_id, name);


--
-- Name: group_permissions uk_group_permissions_group_element; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.group_permissions
    ADD CONSTRAINT uk_group_permissions_group_element UNIQUE (group_id, element_code);


--
-- Name: media_files uk_media_files_tenant_path; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.media_files
    ADD CONSTRAINT uk_media_files_tenant_path UNIQUE (tenant_id, file_path);


--
-- Name: mobile_app_bundles uk_mobile_app_bundles_tenant_platform_version; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.mobile_app_bundles
    ADD CONSTRAINT uk_mobile_app_bundles_tenant_platform_version UNIQUE (tenant_id, platform, version);


--
-- Name: r_group_user uk_r_group_user_group_user; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.r_group_user
    ADD CONSTRAINT uk_r_group_user_group_user UNIQUE (group_id, user_id);


--
-- Name: sys_role_permissions uk_role_permission; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.sys_role_permissions
    ADD CONSTRAINT uk_role_permission UNIQUE (role_id, permission_code);


--
-- Name: tenant_translations uk_tenant_translations_tenant_lang_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.tenant_translations
    ADD CONSTRAINT uk_tenant_translations_tenant_lang_key UNIQUE (tenant_id, lang, key);


--
-- Name: user_groups uk_user_groups_tenant_name; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.user_groups
    ADD CONSTRAINT uk_user_groups_tenant_name UNIQUE (tenant_id, name);


--
-- Name: widget_bundles uk_widget_bundles_tenant_name; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.widget_bundles
    ADD CONSTRAINT uk_widget_bundles_tenant_name UNIQUE (tenant_id, name);


--
-- Name: service_plugins unique_name; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.service_plugins
    ADD CONSTRAINT unique_name UNIQUE (name);


--
-- Name: service_plugins unique_service_identifier; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.service_plugins
    ADD CONSTRAINT unique_service_identifier UNIQUE (service_identifier);


--
-- Name: uplink_storage_dead_letters uplink_storage_dead_letters_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.uplink_storage_dead_letters
    ADD CONSTRAINT uplink_storage_dead_letters_pkey PRIMARY KEY (id);


--
-- Name: uplink_storage_receipts uplink_storage_receipts_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.uplink_storage_receipts
    ADD CONSTRAINT uplink_storage_receipts_pkey PRIMARY KEY (id);


--
-- Name: device_health_scores uq_device_health_scores_tenant_device; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.device_health_scores
    ADD CONSTRAINT uq_device_health_scores_tenant_device UNIQUE (tenant_id, device_id);


--
-- Name: sys_secrets uq_sys_secrets_tenant_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.sys_secrets
    ADD CONSTRAINT uq_sys_secrets_tenant_key UNIQUE (tenant_id, key);


--
-- Name: tenant_rate_limits uq_tenant_rate_limit; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.tenant_rate_limits
    ADD CONSTRAINT uq_tenant_rate_limit UNIQUE (tenant_id, target_type, target_id, limit_type);


--
-- Name: user_address user_address_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.user_address
    ADD CONSTRAINT user_address_pkey PRIMARY KEY (id);


--
-- Name: user_groups user_groups_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.user_groups
    ADD CONSTRAINT user_groups_pkey PRIMARY KEY (id);


--
-- Name: user_totp user_totp_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.user_totp
    ADD CONSTRAINT user_totp_pkey PRIMARY KEY (user_id);


--
-- Name: user_totp_recovery_codes user_totp_recovery_codes_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.user_totp_recovery_codes
    ADD CONSTRAINT user_totp_recovery_codes_pkey PRIMARY KEY (id);


--
-- Name: users users_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.users
    ADD CONSTRAINT users_pkey PRIMARY KEY (id);


--
-- Name: users users_un; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.users
    ADD CONSTRAINT users_un UNIQUE (email);


--
-- Name: vis_dashboard vis_dashboard_pk; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.vis_dashboard
    ADD CONSTRAINT vis_dashboard_pk PRIMARY KEY (id);


--
-- Name: widget_bundles widget_bundles_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.widget_bundles
    ADD CONSTRAINT widget_bundles_pkey PRIMARY KEY (id);


--
-- Name: alarm_assignment_alarm_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX alarm_assignment_alarm_idx ON public.alarm_assignment USING btree (tenant_id, alarm_history_id, created_at DESC);


--
-- Name: alarm_comment_alarm_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX alarm_comment_alarm_idx ON public.alarm_comment USING btree (tenant_id, alarm_history_id, created_at DESC);


--
-- Name: board_project_members_board_uk; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX board_project_members_board_uk ON public.board_project_members USING btree (tenant_id, board_id);


--
-- Name: device_claim_tokens_device_status_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX device_claim_tokens_device_status_idx ON public.device_claim_tokens USING btree (device_id, status);


--
-- Name: device_claim_tokens_one_active_per_device; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX device_claim_tokens_one_active_per_device ON public.device_claim_tokens USING btree (device_id) WHERE ((status)::text = 'active'::text);


--
-- Name: device_claim_tokens_tenant_device_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX device_claim_tokens_tenant_device_idx ON public.device_claim_tokens USING btree (tenant_id, device_id, created_at DESC);


--
-- Name: device_pre_register_credential_grants_one_pending_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX device_pre_register_credential_grants_one_pending_idx ON public.device_pre_register_credential_grants USING btree (tenant_id, batch_number) WHERE ((status)::text = 'pending'::text);


--
-- Name: device_pre_register_credential_grants_tenant_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX device_pre_register_credential_grants_tenant_idx ON public.device_pre_register_credential_grants USING btree (tenant_id, created_at DESC);


--
-- Name: device_template_upgrade_history_name_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX device_template_upgrade_history_name_idx ON public.device_template_upgrade_history USING btree (tenant_id, template_name, created_at DESC);


--
-- Name: edge_node_certificates_tenant_node_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX edge_node_certificates_tenant_node_idx ON public.edge_node_certificates USING btree (tenant_id, node_id, status);


--
-- Name: edge_node_upgrade_tenant_node_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX edge_node_upgrade_tenant_node_idx ON public.edge_node_upgrade_history USING btree (tenant_id, node_id, created_at DESC);


--
-- Name: edge_nodes_tenant_seen_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX edge_nodes_tenant_seen_idx ON public.edge_nodes USING btree (tenant_id, last_seen_at DESC);


--
-- Name: idx_action_info_scene_automation_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_action_info_scene_automation_id ON public.action_info USING btree (scene_automation_id);


--
-- Name: idx_action_info_target_type; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_action_info_target_type ON public.action_info USING btree (action_target, action_type) WHERE (action_target IS NOT NULL);


--
-- Name: idx_ai_models_purpose; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_ai_models_purpose ON public.ai_models USING btree (purpose);


--
-- Name: idx_ai_models_tenant; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_ai_models_tenant ON public.ai_models USING btree (tenant_id);


--
-- Name: idx_alarm_history_devices_device_tenant; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_alarm_history_devices_device_tenant ON public.alarm_history_devices USING btree (device_id, tenant_id);


--
-- Name: idx_alarm_history_devices_tenant_device; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_alarm_history_devices_tenant_device ON public.alarm_history_devices USING btree (tenant_id, device_id);


--
-- Name: idx_alarm_history_sla_due_at; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_alarm_history_sla_due_at ON public.alarm_history USING btree (sla_due_at) WHERE (sla_due_at IS NOT NULL);


--
-- Name: idx_alarm_history_tenant_create_at; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_alarm_history_tenant_create_at ON public.alarm_history USING btree (tenant_id, create_at DESC);


--
-- Name: idx_alarm_info_tenant_alarm_time; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_alarm_info_tenant_alarm_time ON public.alarm_info USING btree (tenant_id, alarm_time DESC);


--
-- Name: idx_assets_tenant_parent; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_assets_tenant_parent ON public.assets USING btree (tenant_id, parent_id);


--
-- Name: idx_attribute_set_logs_device_created_at; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_attribute_set_logs_device_created_at ON public.attribute_set_logs USING btree (device_id, created_at DESC);


--
-- Name: idx_attribute_set_logs_message_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_attribute_set_logs_message_id ON public.attribute_set_logs USING btree (message_id) WHERE (message_id IS NOT NULL);


--
-- Name: idx_boards_share_token; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX idx_boards_share_token ON public.boards USING btree (share_token) WHERE (share_token IS NOT NULL);


--
-- Name: idx_calculated_fields_template_enabled; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_calculated_fields_template_enabled ON public.calculated_fields USING btree (device_template_id, enabled);


--
-- Name: idx_calculated_fields_tenant; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_calculated_fields_tenant ON public.calculated_fields USING btree (tenant_id);


--
-- Name: idx_casbin_rule; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX idx_casbin_rule ON public.casbin_rule USING btree (ptype, v0, v1, v2, v3, v4, v5);


--
-- Name: idx_cfrt_tenant_status; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_cfrt_tenant_status ON public.calcfield_recompute_tasks USING btree (tenant_id, status, created_at DESC);


--
-- Name: idx_command_job_details_device; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_command_job_details_device ON public.command_job_details USING btree (tenant_id, device_id, updated_at DESC);


--
-- Name: idx_command_job_details_device_message; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_command_job_details_device_message ON public.command_job_details USING btree (device_id, message_id);


--
-- Name: idx_command_job_details_dispatch_lease; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_command_job_details_dispatch_lease ON public.command_job_details USING btree (tenant_id, status, dispatch_lease_until);


--
-- Name: idx_command_job_details_global_dispatching_lease; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_command_job_details_global_dispatching_lease ON public.command_job_details USING btree (dispatch_lease_until) WHERE (((status)::text = 'dispatching'::text) AND (eligible = true));


--
-- Name: idx_command_job_details_job_status; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_command_job_details_job_status ON public.command_job_details USING btree (tenant_id, command_job_id, status);


--
-- Name: idx_command_job_details_retry_after; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_command_job_details_retry_after ON public.command_job_details USING btree (tenant_id, command_job_id, status, can_retry, next_retry_after);


--
-- Name: idx_command_job_events_job_created; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_command_job_events_job_created ON public.command_job_events USING btree (tenant_id, command_job_id, created_at);


--
-- Name: idx_command_jobs_next_dispatch_due; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_command_jobs_next_dispatch_due ON public.command_jobs USING btree (next_dispatch_at, updated_at) WHERE (((status)::text = 'running'::text) AND (next_dispatch_at IS NOT NULL));


--
-- Name: idx_command_jobs_scheduled_due; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_command_jobs_scheduled_due ON public.command_jobs USING btree (scheduled_at, updated_at) WHERE ((status)::text = 'scheduled'::text);


--
-- Name: idx_command_jobs_tenant_created; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_command_jobs_tenant_created ON public.command_jobs USING btree (tenant_id, created_at DESC);


--
-- Name: idx_command_jobs_tenant_status; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_command_jobs_tenant_status ON public.command_jobs USING btree (tenant_id, status, updated_at DESC);


--
-- Name: idx_command_set_logs_device_created_at; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_command_set_logs_device_created_at ON public.command_set_logs USING btree (device_id, created_at DESC);


--
-- Name: idx_command_set_logs_message_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_command_set_logs_message_id ON public.command_set_logs USING btree (message_id) WHERE (message_id IS NOT NULL);


--
-- Name: idx_customer_devices_customer_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_customer_devices_customer_id ON public.customer_devices USING btree (customer_id);


--
-- Name: idx_customer_devices_device_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_customer_devices_device_id ON public.customer_devices USING btree (device_id);


--
-- Name: idx_customers_name; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_customers_name ON public.customers USING btree (name);


--
-- Name: idx_customers_tenant_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_customers_tenant_id ON public.customers USING btree (tenant_id);


--
-- Name: idx_data_converters_tenant_type; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_data_converters_tenant_type ON public.data_converters USING btree (tenant_id, type);


--
-- Name: idx_data_retention_registry_enabled; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_data_retention_registry_enabled ON public.data_retention_registry USING btree (enabled, table_name);


--
-- Name: idx_device_certs_device; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_device_certs_device ON public.device_certificates USING btree (device_id);


--
-- Name: idx_device_certs_serial; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_device_certs_serial ON public.device_certificates USING btree (serial_number);


--
-- Name: idx_device_certs_status; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_device_certs_status ON public.device_certificates USING btree (status);


--
-- Name: idx_device_certs_tenant; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_device_certs_tenant ON public.device_certificates USING btree (tenant_id);


--
-- Name: idx_device_configs_default_rule_chain_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_device_configs_default_rule_chain_id ON public.device_configs USING btree (default_rule_chain_id);


--
-- Name: idx_device_configs_payload_schema_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_device_configs_payload_schema_id ON public.device_configs USING btree (payload_schema_id) WHERE (payload_schema_id IS NOT NULL);


--
-- Name: idx_device_health_scores_device; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_device_health_scores_device ON public.device_health_scores USING btree (device_id);


--
-- Name: idx_device_health_scores_tenant_status; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_device_health_scores_tenant_status ON public.device_health_scores USING btree (tenant_id, health_status);


--
-- Name: idx_device_shadow_messages_retry; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_device_shadow_messages_retry ON public.device_shadow_messages USING btree (status, next_attempt_at) WHERE ((status)::text = 'sent'::text);


--
-- Name: idx_device_topic_mapping_lookup; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_device_topic_mapping_lookup ON public.device_topic_mappings USING btree (device_config_id, direction, enabled, priority);


--
-- Name: idx_device_trigger_condition_scene_automation_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_device_trigger_condition_scene_automation_id ON public.device_trigger_condition USING btree (scene_automation_id);


--
-- Name: idx_device_trigger_condition_source_type; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_device_trigger_condition_source_type ON public.device_trigger_condition USING btree (trigger_source, trigger_condition_type) WHERE (trigger_source IS NOT NULL);


--
-- Name: idx_devices_device_config_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_devices_device_config_id ON public.devices USING btree (device_config_id) WHERE (device_config_id IS NOT NULL);


--
-- Name: idx_devices_parent_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_devices_parent_id ON public.devices USING btree (parent_id) WHERE (parent_id IS NOT NULL);


--
-- Name: idx_devices_parent_sub_addr; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_devices_parent_sub_addr ON public.devices USING btree (parent_id, sub_device_addr) WHERE (parent_id IS NOT NULL);


--
-- Name: idx_devices_tenant_active_created; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_devices_tenant_active_created ON public.devices USING btree (tenant_id, activate_flag, created_at DESC);


--
-- Name: idx_devices_tenant_owner_active; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_devices_tenant_owner_active ON public.devices USING btree (tenant_id, owner_user_id, activate_flag);


--
-- Name: idx_devices_voucher_hash; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX idx_devices_voucher_hash ON public.devices USING btree (voucher_hash);


--
-- Name: idx_dsm_device_pending; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_dsm_device_pending ON public.device_shadow_messages USING btree (device_id, created_at) WHERE ((status)::text = 'pending'::text);


--
-- Name: idx_dsm_expiry_sweep; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_dsm_expiry_sweep ON public.device_shadow_messages USING btree (expires_at) WHERE ((status)::text = 'pending'::text);


--
-- Name: idx_edge_sync_gateway; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_edge_sync_gateway ON public.edge_sync_tasks USING btree (gateway_device_id);


--
-- Name: idx_edge_sync_resource; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_edge_sync_resource ON public.edge_sync_tasks USING btree (resource_type);


--
-- Name: idx_edge_sync_status; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_edge_sync_status ON public.edge_sync_tasks USING btree (status);


--
-- Name: idx_edge_sync_tenant; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_edge_sync_tenant ON public.edge_sync_tasks USING btree (tenant_id);


--
-- Name: idx_email_templates_scope; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_email_templates_scope ON public.email_templates USING btree (tenant_id, purpose, updated_at DESC);


--
-- Name: idx_entity_relations_from; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_entity_relations_from ON public.entity_relations USING btree (tenant_id, from_type, from_id, relation_type);


--
-- Name: idx_entity_relations_to; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_entity_relations_to ON public.entity_relations USING btree (tenant_id, to_type, to_id, relation_type);


--
-- Name: idx_entity_versions_entity_created; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_entity_versions_entity_created ON public.entity_versions USING btree (tenant_id, entity_type, entity_id, created_at DESC);


--
-- Name: idx_entity_versions_entity_version; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX idx_entity_versions_entity_version ON public.entity_versions USING btree (tenant_id, entity_type, entity_id, version_number);


--
-- Name: idx_event_datas_device_identify_ts; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_event_datas_device_identify_ts ON public.event_datas USING btree (device_id, identify, ts DESC);


--
-- Name: idx_fleet_saved_filters_tenant_shared_updated; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_fleet_saved_filters_tenant_shared_updated ON public.fleet_saved_filters USING btree (tenant_id, shared, updated_at DESC);


--
-- Name: idx_fleet_saved_filters_user_updated; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_fleet_saved_filters_user_updated ON public.fleet_saved_filters USING btree (tenant_id, user_id, updated_at DESC);


--
-- Name: idx_group_permissions_element_code; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_group_permissions_element_code ON public.group_permissions USING btree (element_code);


--
-- Name: idx_group_permissions_group_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_group_permissions_group_id ON public.group_permissions USING btree (group_id);


--
-- Name: idx_group_permissions_tenant_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_group_permissions_tenant_id ON public.group_permissions USING btree (tenant_id);


--
-- Name: idx_groups_tenant_owner; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_groups_tenant_owner ON public.groups USING btree (tenant_id, owner_user_id);


--
-- Name: idx_integrations_converter_downlink; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_integrations_converter_downlink ON public.integrations USING btree (converter_downlink_id);


--
-- Name: idx_integrations_converter_uplink; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_integrations_converter_uplink ON public.integrations USING btree (converter_uplink_id);


--
-- Name: idx_integrations_tenant_connector; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_integrations_tenant_connector ON public.integrations USING btree (tenant_id, connector_type);


--
-- Name: idx_lower_device_number; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_lower_device_number ON public.devices USING btree (lower((device_number)::text));


--
-- Name: idx_lower_name; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_lower_name ON public.devices USING btree (lower((name)::text));


--
-- Name: idx_media_files_tenant_created; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_media_files_tenant_created ON public.media_files USING btree (tenant_id, created_at DESC);


--
-- Name: idx_mobile_app_bundles_tenant_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_mobile_app_bundles_tenant_id ON public.mobile_app_bundles USING btree (tenant_id);


--
-- Name: idx_mobile_app_bundles_tenant_status; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_mobile_app_bundles_tenant_status ON public.mobile_app_bundles USING btree (tenant_id, status);


--
-- Name: idx_notification_histories_tenant_send_time; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_notification_histories_tenant_send_time ON public.notification_histories USING btree (tenant_id, send_time DESC);


--
-- Name: idx_notification_history_devices_device_tenant; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_notification_history_devices_device_tenant ON public.notification_history_devices USING btree (device_id, tenant_id);


--
-- Name: idx_notification_history_devices_history_tenant; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_notification_history_devices_history_tenant ON public.notification_history_devices USING btree (notification_history_id, tenant_id);


--
-- Name: idx_oidc_providers_tenant; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_oidc_providers_tenant ON public.tenant_oidc_providers USING btree (tenant_id);


--
-- Name: idx_operation_logs_created_at; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_operation_logs_created_at ON public.operation_logs USING btree (created_at);


--
-- Name: idx_operation_logs_tenant_created_at; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_operation_logs_tenant_created_at ON public.operation_logs USING btree (tenant_id, created_at DESC);


--
-- Name: idx_operation_logs_tenant_entity; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_operation_logs_tenant_entity ON public.operation_logs USING btree (tenant_id, entity_type, entity_id) WHERE (entity_type IS NOT NULL);


--
-- Name: idx_ota_upgrade_task_details_device_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_ota_upgrade_task_details_device_id ON public.ota_upgrade_task_details USING btree (device_id);


--
-- Name: idx_ota_upgrade_task_details_dispatch_claim; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_ota_upgrade_task_details_dispatch_claim ON public.ota_upgrade_task_details USING btree (ota_upgrade_task_id, updated_at, id) WHERE (status = 1);


--
-- Name: idx_ota_upgrade_task_details_dispatch_lease; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_ota_upgrade_task_details_dispatch_lease ON public.ota_upgrade_task_details USING btree (dispatch_lease_until) WHERE ((status = 1) AND (dispatch_lease_token IS NOT NULL));


--
-- Name: idx_ota_upgrade_task_details_task_status; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_ota_upgrade_task_details_task_status ON public.ota_upgrade_task_details USING btree (ota_upgrade_task_id, status);


--
-- Name: idx_ota_upgrade_tasks_package_created_at; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_ota_upgrade_tasks_package_created_at ON public.ota_upgrade_tasks USING btree (ota_upgrade_package_id, created_at DESC);


--
-- Name: idx_ota_upgrade_tasks_rollout_due; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_ota_upgrade_tasks_rollout_due ON public.ota_upgrade_tasks USING btree (next_dispatch_at, scheduled_at, created_at) WHERE ((status)::text = ANY ((ARRAY['scheduled'::character varying, 'running'::character varying])::text[]));


--
-- Name: idx_ota_upgrade_tasks_timeout_due; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_ota_upgrade_tasks_timeout_due ON public.ota_upgrade_tasks USING btree (timeout_at) WHERE (((status)::text = 'running'::text) AND (timeout_at IS NOT NULL));


--
-- Name: idx_plugin_registries_name; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX idx_plugin_registries_name ON public.plugin_registries USING btree (name);


--
-- Name: idx_push_deliveries_retry; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_push_deliveries_retry ON public.push_deliveries USING btree (status, next_attempt_at) WHERE ((status)::text = ANY ((ARRAY['pending'::character varying, 'failed'::character varying])::text[]));


--
-- Name: idx_push_device_registrations_user; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_push_device_registrations_user ON public.push_device_registrations USING btree (tenant_id, user_id, enabled);


--
-- Name: idx_r_group_device_device_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_r_group_device_device_id ON public.r_group_device USING btree (device_id);


--
-- Name: idx_r_group_user_tenant_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_r_group_user_tenant_id ON public.r_group_user USING btree (tenant_id);


--
-- Name: idx_r_group_user_user_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_r_group_user_user_id ON public.r_group_user USING btree (user_id);


--
-- Name: idx_rc_tenant; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_rc_tenant ON public.rule_chains USING btree (tenant_id);


--
-- Name: idx_rcc_chain_node_time; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_rcc_chain_node_time ON public.rule_chain_checkpoints USING btree (chain_id, node_id, created_at DESC);


--
-- Name: idx_rce_chain; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_rce_chain ON public.rule_chain_edges USING btree (chain_id);


--
-- Name: idx_rcn_chain; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_rcn_chain ON public.rule_chain_nodes USING btree (chain_id);


--
-- Name: idx_rcnt_chain_node_time; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_rcnt_chain_node_time ON public.rule_chain_node_traces USING btree (chain_id, node_id, created_at DESC);


--
-- Name: idx_rcnt_tenant_time; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_rcnt_tenant_time ON public.rule_chain_node_traces USING btree (tenant_id, created_at DESC);


--
-- Name: idx_report_schedule_deliveries_claim_recovery; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_report_schedule_deliveries_claim_recovery ON public.report_schedule_deliveries USING btree (lease_until, run_id) WHERE ((status)::text = 'processing'::text);


--
-- Name: idx_report_schedule_deliveries_due; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_report_schedule_deliveries_due ON public.report_schedule_deliveries USING btree (next_attempt_at, created_at, run_id) WHERE ((status)::text = ANY ((ARRAY['pending'::character varying, 'retrying'::character varying])::text[]));


--
-- Name: idx_report_schedule_deliveries_history; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_report_schedule_deliveries_history ON public.report_schedule_deliveries USING btree (tenant_id, created_at DESC, run_id);


--
-- Name: idx_report_schedule_runs_claim_recovery; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_report_schedule_runs_claim_recovery ON public.report_schedule_runs USING btree (lease_until, id) WHERE ((generation_status)::text = 'processing'::text);


--
-- Name: idx_report_schedule_runs_due; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_report_schedule_runs_due ON public.report_schedule_runs USING btree (next_attempt_at, created_at, id) WHERE ((generation_status)::text = ANY ((ARRAY['pending'::character varying, 'retrying'::character varying])::text[]));


--
-- Name: idx_report_schedule_runs_history; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_report_schedule_runs_history ON public.report_schedule_runs USING btree (tenant_id, schedule_id, created_at DESC, id DESC);


--
-- Name: idx_report_schedule_runs_retry_parent; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_report_schedule_runs_retry_parent ON public.report_schedule_runs USING btree (retry_parent_run_id, created_at DESC) WHERE (retry_parent_run_id IS NOT NULL);


--
-- Name: idx_report_schedules_due; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_report_schedules_due ON public.report_schedules USING btree (next_run_at, id) WHERE (enabled AND (deleted_at IS NULL) AND (next_run_at IS NOT NULL));


--
-- Name: idx_report_schedules_enabled; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_report_schedules_enabled ON public.report_schedules USING btree (enabled);


--
-- Name: idx_report_schedules_tenant; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_report_schedules_tenant ON public.report_schedules USING btree (tenant_id);


--
-- Name: idx_report_schedules_tenant_history; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_report_schedules_tenant_history ON public.report_schedules USING btree (tenant_id, created_at DESC, id) WHERE (deleted_at IS NULL);


--
-- Name: idx_rule_chain_dead_letters_exec; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_rule_chain_dead_letters_exec ON public.rule_chain_dead_letters USING btree (tenant_id, exec_id);


--
-- Name: idx_rule_chain_dead_letters_tenant_chain; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_rule_chain_dead_letters_tenant_chain ON public.rule_chain_dead_letters USING btree (tenant_id, chain_id, created_at DESC);


--
-- Name: idx_rule_chain_replay_exec; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_rule_chain_replay_exec ON public.rule_chain_replay_records USING btree (tenant_id, execution_id, recorded_at);


--
-- Name: idx_rule_chains_tenant_enabled; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_rule_chains_tenant_enabled ON public.rule_chains USING btree (tenant_id, enabled);


--
-- Name: idx_scada_control_audits_doc; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_scada_control_audits_doc ON public.scada_control_audits USING btree (tenant_id, document_id, created_at DESC);


--
-- Name: idx_scada_document_versions_doc; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_scada_document_versions_doc ON public.scada_document_versions USING btree (tenant_id, document_id, version DESC);


--
-- Name: idx_scada_documents_project; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_scada_documents_project ON public.scada_documents USING btree (tenant_id, project_id);


--
-- Name: idx_scada_projects_tenant; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_scada_projects_tenant ON public.scada_projects USING btree (tenant_id);


--
-- Name: idx_scene_action_info_scene_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_scene_action_info_scene_id ON public.scene_action_info USING btree (scene_id);


--
-- Name: idx_scene_automation_log_automation_executed_at; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_scene_automation_log_automation_executed_at ON public.scene_automation_log USING btree (scene_automation_id, executed_at DESC);


--
-- Name: idx_scene_log_scene_executed_at; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_scene_log_scene_executed_at ON public.scene_log USING btree (scene_id, executed_at DESC);


--
-- Name: idx_scene_timers_due; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_scene_timers_due ON public.scene_automation_timers USING btree (next_run_at) WHERE (enabled = true);


--
-- Name: idx_scheduler_events_tenant_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_scheduler_events_tenant_id ON public.scheduler_events USING btree (tenant_id);


--
-- Name: idx_scheduler_events_tenant_next_run; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_scheduler_events_tenant_next_run ON public.scheduler_events USING btree (tenant_id, next_run_at);


--
-- Name: idx_scheduler_events_tenant_type; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_scheduler_events_tenant_type ON public.scheduler_events USING btree (tenant_id, event_type);


--
-- Name: idx_sys_permissions_module; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_sys_permissions_module ON public.sys_permissions USING btree (module);


--
-- Name: idx_sys_role_permissions_role_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_sys_role_permissions_role_id ON public.sys_role_permissions USING btree (role_id);


--
-- Name: idx_sys_role_permissions_tenant_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_sys_role_permissions_tenant_id ON public.sys_role_permissions USING btree (tenant_id);


--
-- Name: idx_sys_secrets_tenant; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_sys_secrets_tenant ON public.sys_secrets USING btree (tenant_id);


--
-- Name: idx_sys_secrets_type; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_sys_secrets_type ON public.sys_secrets USING btree (tenant_id, secret_type);


--
-- Name: idx_telemetry_current_datas_device_ts; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_telemetry_current_datas_device_ts ON public.telemetry_current_datas USING btree (device_id, ts DESC);


--
-- Name: INDEX idx_telemetry_current_datas_device_ts; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON INDEX public.idx_telemetry_current_datas_device_ts IS '服务 GetCurrentTelemetrDetailData / getCurrentTelemetryReadinessFromDB 的 device_id 等值 + ts DESC 取一';


--
-- Name: idx_telemetry_set_logs_device_created_at; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_telemetry_set_logs_device_created_at ON public.telemetry_set_logs USING btree (device_id, created_at DESC);


--
-- Name: idx_tenant_dashboard_menu_unique; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX idx_tenant_dashboard_menu_unique ON public.tenant_dashboard_menus USING btree (tenant_id, dashboard_id);


--
-- Name: idx_tenant_device_time; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_tenant_device_time ON public.device_status_history USING btree (tenant_id, device_id, change_time);


--
-- Name: idx_tenant_rate_limits_target; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_tenant_rate_limits_target ON public.tenant_rate_limits USING btree (target_type, target_id);


--
-- Name: idx_tenant_rate_limits_tenant; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_tenant_rate_limits_tenant ON public.tenant_rate_limits USING btree (tenant_id);


--
-- Name: idx_tenant_subscriptions_tenant_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_tenant_subscriptions_tenant_id ON public.tenant_subscriptions USING btree (tenant_id);


--
-- Name: idx_totp_recovery_user; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_totp_recovery_user ON public.user_totp_recovery_codes USING btree (user_id);


--
-- Name: idx_user_groups_tenant_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_user_groups_tenant_id ON public.user_groups USING btree (tenant_id);


--
-- Name: idx_widget_bundles_tenant_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_widget_bundles_tenant_id ON public.widget_bundles USING btree (tenant_id);


--
-- Name: idx_widget_bundles_type_key; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_widget_bundles_type_key ON public.widget_bundles USING btree (type_key);


--
-- Name: index_user_push; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX index_user_push ON public.message_push_manage USING btree (user_id, push_id);


--
-- Name: industry_solution_installs_solution_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX industry_solution_installs_solution_idx ON public.industry_solution_installs USING btree (tenant_id, solution_id, created_at DESC);


--
-- Name: industry_solutions_tenant_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX industry_solutions_tenant_idx ON public.industry_solutions USING btree (tenant_id, status, created_at DESC);


--
-- Name: industry_solutions_tenant_name_uniq; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX industry_solutions_tenant_name_uniq ON public.industry_solutions USING btree (tenant_id, name);


--
-- Name: mqtt_session_revocation_acks_device_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX mqtt_session_revocation_acks_device_idx ON public.mqtt_session_revocation_acks USING btree (device_id, revoked_at DESC);


--
-- Name: mqtt_session_revocation_outbox_device_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX mqtt_session_revocation_outbox_device_idx ON public.mqtt_session_revocation_outbox USING btree (device_id, revoked_at DESC);


--
-- Name: mqtt_session_revocation_outbox_retry_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX mqtt_session_revocation_outbox_retry_idx ON public.mqtt_session_revocation_outbox USING btree (status, next_retry_at, created_at);


--
-- Name: payload_schemas_tenant_updated_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX payload_schemas_tenant_updated_idx ON public.payload_schemas USING btree (tenant_id, updated_at DESC, id);


--
-- Name: rule_chain_versions_single_published_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX rule_chain_versions_single_published_idx ON public.rule_chain_versions USING btree (chain_id) WHERE ((status)::text = 'published'::text);


--
-- Name: rule_chain_versions_tenant_chain_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX rule_chain_versions_tenant_chain_idx ON public.rule_chain_versions USING btree (tenant_id, chain_id, status);


--
-- Name: telemetry_datas_ts_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX telemetry_datas_ts_idx ON public.telemetry_datas USING btree (ts DESC);


--
-- Name: telemetry_dead_letters_device_ts_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX telemetry_dead_letters_device_ts_idx ON public.telemetry_dead_letters USING btree (device_id, ts DESC);


--
-- Name: telemetry_dead_letters_status_retry_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX telemetry_dead_letters_status_retry_idx ON public.telemetry_dead_letters USING btree (status, next_retry_at);


--
-- Name: telemetry_rollups_window_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX telemetry_rollups_window_idx ON public.telemetry_rollups USING btree (device_id, key, bucket_start);


--
-- Name: uk_user_address_user_id; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX uk_user_address_user_id ON public.user_address USING btree (user_id);


--
-- Name: uplink_storage_dead_letters_claim_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX uplink_storage_dead_letters_claim_idx ON public.uplink_storage_dead_letters USING btree (status, next_retry_at, lease_until, created_at, id);


--
-- Name: uplink_storage_dead_letters_device_ts_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX uplink_storage_dead_letters_device_ts_idx ON public.uplink_storage_dead_letters USING btree (device_id, ts DESC);


--
-- Name: uplink_storage_dead_letters_replay_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX uplink_storage_dead_letters_replay_idx ON public.uplink_storage_dead_letters USING btree (status, next_retry_at, created_at, id);


--
-- Name: uplink_storage_receipts_device_ts_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX uplink_storage_receipts_device_ts_idx ON public.uplink_storage_receipts USING btree (device_id, ts DESC);


--
-- Name: uq_data_policy_row_level; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX uq_data_policy_row_level ON public.data_policy USING btree (tenant_id, COALESCE(device_config_id, ''::character varying)) WHERE (tenant_id IS NOT NULL);


--
-- Name: uq_email_templates_default_scope; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX uq_email_templates_default_scope ON public.email_templates USING btree (tenant_id, purpose) WHERE ((enabled = true) AND (is_default = true));


--
-- Name: uq_report_schedule_deliveries_message_id; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX uq_report_schedule_deliveries_message_id ON public.report_schedule_deliveries USING btree (message_id);


--
-- Name: uq_report_schedule_runs_idempotency; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX uq_report_schedule_runs_idempotency ON public.report_schedule_runs USING btree (tenant_id, schedule_id, trigger, idempotency_key_hash) WHERE ((idempotency_key_hash IS NOT NULL) AND ((trigger)::text = ANY ((ARRAY['manual'::character varying, 'retry'::character varying])::text[])));


--
-- Name: uq_report_schedule_runs_scheduled_slot; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX uq_report_schedule_runs_scheduled_slot ON public.report_schedule_runs USING btree (schedule_id, scheduled_slot) WHERE ((trigger)::text = 'scheduled'::text);


--
-- Name: uq_report_schedules_identity; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX uq_report_schedules_identity ON public.report_schedules USING btree (id, tenant_id);


--
-- Name: uq_rule_chain_replay_node; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX uq_rule_chain_replay_node ON public.rule_chain_replay_records USING btree (execution_id, node_id);


--
-- Name: uq_scene_timers_automation; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX uq_scene_timers_automation ON public.scene_automation_timers USING btree (scene_automation_id) WHERE (enabled = true);


--
-- Name: uq_totp_recovery_hash; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX uq_totp_recovery_hash ON public.user_totp_recovery_codes USING btree (user_id, code_hash);


--
-- Name: ux_device_topic_mapping_unique; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX ux_device_topic_mapping_unique ON public.device_topic_mappings USING btree (device_config_id, direction, source_topic, target_topic);


--
-- Name: alarm_history trg_alarm_history_devices_sync; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER trg_alarm_history_devices_sync AFTER INSERT OR UPDATE OF alarm_device_list ON public.alarm_history FOR EACH ROW EXECUTE FUNCTION public.alarm_history_devices_sync();


--
-- Name: action_info action_info_scene_automations_fk; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.action_info
    ADD CONSTRAINT action_info_scene_automations_fk FOREIGN KEY (scene_automation_id) REFERENCES public.scene_automations(id) ON DELETE CASCADE;


--
-- Name: alarm_history_devices alarm_history_devices_history_fk; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.alarm_history_devices
    ADD CONSTRAINT alarm_history_devices_history_fk FOREIGN KEY (alarm_history_id) REFERENCES public.alarm_history(id) ON DELETE CASCADE;


--
-- Name: alarm_info alarm_info_fk; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.alarm_info
    ADD CONSTRAINT alarm_info_fk FOREIGN KEY (alarm_config_id) REFERENCES public.alarm_config(id) ON DELETE CASCADE;


--
-- Name: attribute_datas attribute_datas_device_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.attribute_datas
    ADD CONSTRAINT attribute_datas_device_id_fkey FOREIGN KEY (device_id) REFERENCES public.devices(id) ON DELETE RESTRICT;


--
-- Name: attribute_set_logs attribute_set_logs_device_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.attribute_set_logs
    ADD CONSTRAINT attribute_set_logs_device_id_fkey FOREIGN KEY (device_id) REFERENCES public.devices(id) ON DELETE CASCADE;


--
-- Name: command_job_details command_job_details_job_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.command_job_details
    ADD CONSTRAINT command_job_details_job_fkey FOREIGN KEY (command_job_id) REFERENCES public.command_jobs(id) ON DELETE CASCADE;


--
-- Name: command_job_events command_job_events_job_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.command_job_events
    ADD CONSTRAINT command_job_events_job_fkey FOREIGN KEY (command_job_id) REFERENCES public.command_jobs(id) ON DELETE CASCADE;


--
-- Name: command_set_logs command_set_logs_device_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.command_set_logs
    ADD CONSTRAINT command_set_logs_device_id_fkey FOREIGN KEY (device_id) REFERENCES public.devices(id) ON DELETE CASCADE;


--
-- Name: data_scripts data_scripts_device_configs_fk; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.data_scripts
    ADD CONSTRAINT data_scripts_device_configs_fk FOREIGN KEY (device_config_id) REFERENCES public.device_configs(id) ON DELETE CASCADE;


--
-- Name: device_configs device_configs_default_rule_chain_fk; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.device_configs
    ADD CONSTRAINT device_configs_default_rule_chain_fk FOREIGN KEY (default_rule_chain_id) REFERENCES public.rule_chains(id) ON DELETE RESTRICT;


--
-- Name: device_configs device_configs_device_templates_fk; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.device_configs
    ADD CONSTRAINT device_configs_device_templates_fk FOREIGN KEY (device_template_id) REFERENCES public.device_templates(id) ON DELETE RESTRICT;


--
-- Name: device_model_attributes device_model_attributes_device_templates_fk; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.device_model_attributes
    ADD CONSTRAINT device_model_attributes_device_templates_fk FOREIGN KEY (device_template_id) REFERENCES public.device_templates(id) ON DELETE CASCADE;


--
-- Name: device_model_commands device_model_commands_device_templates_fk; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.device_model_commands
    ADD CONSTRAINT device_model_commands_device_templates_fk FOREIGN KEY (device_template_id) REFERENCES public.device_templates(id) ON DELETE CASCADE;


--
-- Name: device_model_custom_control device_model_custom_control_device_templates_fk; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.device_model_custom_control
    ADD CONSTRAINT device_model_custom_control_device_templates_fk FOREIGN KEY (device_template_id) REFERENCES public.device_templates(id) ON DELETE CASCADE;


--
-- Name: device_model_events device_model_events_device_templates_fk; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.device_model_events
    ADD CONSTRAINT device_model_events_device_templates_fk FOREIGN KEY (device_template_id) REFERENCES public.device_templates(id) ON DELETE CASCADE;


--
-- Name: device_model_telemetry device_model_telemetry_device_templates_fk; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.device_model_telemetry
    ADD CONSTRAINT device_model_telemetry_device_templates_fk FOREIGN KEY (device_template_id) REFERENCES public.device_templates(id) ON DELETE CASCADE;


--
-- Name: devices devices_service_access_fk; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.devices
    ADD CONSTRAINT devices_service_access_fk FOREIGN KEY (service_access_id) REFERENCES public.service_access(id) ON DELETE RESTRICT;


--
-- Name: event_datas event_datas_device_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.event_datas
    ADD CONSTRAINT event_datas_device_id_fkey FOREIGN KEY (device_id) REFERENCES public.devices(id) ON DELETE RESTRICT;


--
-- Name: expected_datas expected_datas_devices_fk; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.expected_datas
    ADD CONSTRAINT expected_datas_devices_fk FOREIGN KEY (device_id) REFERENCES public.devices(id) ON UPDATE CASCADE ON DELETE CASCADE;


--
-- Name: device_status_history fk_device; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.device_status_history
    ADD CONSTRAINT fk_device FOREIGN KEY (device_id) REFERENCES public.devices(id) ON DELETE CASCADE;


--
-- Name: devices fk_device_config_id; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.devices
    ADD CONSTRAINT fk_device_config_id FOREIGN KEY (device_config_id) REFERENCES public.device_configs(id) ON DELETE RESTRICT;


--
-- Name: device_configs fk_device_configs_payload_schema; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.device_configs
    ADD CONSTRAINT fk_device_configs_payload_schema FOREIGN KEY (payload_schema_id) REFERENCES public.payload_schemas(id) ON DELETE SET NULL;


--
-- Name: r_group_device fk_group_device; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.r_group_device
    ADD CONSTRAINT fk_group_device FOREIGN KEY (group_id) REFERENCES public.groups(id) ON DELETE CASCADE;


--
-- Name: r_group_device fk_group_device_2; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.r_group_device
    ADD CONSTRAINT fk_group_device_2 FOREIGN KEY (device_id) REFERENCES public.devices(id) ON DELETE CASCADE;


--
-- Name: group_permissions fk_group_permissions_group; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.group_permissions
    ADD CONSTRAINT fk_group_permissions_group FOREIGN KEY (group_id) REFERENCES public.user_groups(id) ON DELETE CASCADE;


--
-- Name: ota_upgrade_task_details fk_ota_upgrade_tasks; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.ota_upgrade_task_details
    ADD CONSTRAINT fk_ota_upgrade_tasks FOREIGN KEY (ota_upgrade_task_id) REFERENCES public.ota_upgrade_tasks(id) ON DELETE CASCADE;


--
-- Name: devices fk_product_id; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.devices
    ADD CONSTRAINT fk_product_id FOREIGN KEY (product_id) REFERENCES public.products(id) ON DELETE RESTRICT;


--
-- Name: r_group_user fk_r_group_user_group; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.r_group_user
    ADD CONSTRAINT fk_r_group_user_group FOREIGN KEY (group_id) REFERENCES public.user_groups(id) ON DELETE CASCADE;


--
-- Name: r_group_user fk_r_group_user_user; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.r_group_user
    ADD CONSTRAINT fk_r_group_user_user FOREIGN KEY (user_id) REFERENCES public.users(id) ON DELETE CASCADE;


--
-- Name: device_trigger_condition fk_scene_automation_id; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.device_trigger_condition
    ADD CONSTRAINT fk_scene_automation_id FOREIGN KEY (scene_automation_id) REFERENCES public.scene_automations(id) ON DELETE CASCADE;


--
-- Name: one_time_tasks fk_scene_automation_id; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.one_time_tasks
    ADD CONSTRAINT fk_scene_automation_id FOREIGN KEY (scene_automation_id) REFERENCES public.scene_automations(id) ON DELETE CASCADE;


--
-- Name: service_access fk_service_plugin; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.service_access
    ADD CONSTRAINT fk_service_plugin FOREIGN KEY (service_plugin_id) REFERENCES public.service_plugins(id) ON DELETE RESTRICT;


--
-- Name: user_address fk_user_address_user_id; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.user_address
    ADD CONSTRAINT fk_user_address_user_id FOREIGN KEY (user_id) REFERENCES public.users(id) ON DELETE CASCADE;


--
-- Name: integrations integrations_converter_downlink_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.integrations
    ADD CONSTRAINT integrations_converter_downlink_id_fkey FOREIGN KEY (converter_downlink_id) REFERENCES public.data_converters(id) ON DELETE SET NULL;


--
-- Name: integrations integrations_converter_uplink_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.integrations
    ADD CONSTRAINT integrations_converter_uplink_id_fkey FOREIGN KEY (converter_uplink_id) REFERENCES public.data_converters(id) ON DELETE SET NULL;


--
-- Name: mqtt_session_revocation_acks mqtt_session_revocation_acks_event_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.mqtt_session_revocation_acks
    ADD CONSTRAINT mqtt_session_revocation_acks_event_fkey FOREIGN KEY (event_id) REFERENCES public.mqtt_session_revocation_outbox(id) ON DELETE CASCADE;


--
-- Name: notification_history_devices notification_history_devices_history_fk; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.notification_history_devices
    ADD CONSTRAINT notification_history_devices_history_fk FOREIGN KEY (notification_history_id) REFERENCES public.notification_histories(id) ON DELETE CASCADE;


--
-- Name: ota_upgrade_task_details ota_upgrade_task_details_device_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.ota_upgrade_task_details
    ADD CONSTRAINT ota_upgrade_task_details_device_id_fkey FOREIGN KEY (device_id) REFERENCES public.devices(id) ON DELETE RESTRICT;


--
-- Name: products products_device_configs_fk; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.products
    ADD CONSTRAINT products_device_configs_fk FOREIGN KEY (device_config_id) REFERENCES public.device_configs(id) ON DELETE RESTRICT;


--
-- Name: push_deliveries push_deliveries_registration_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.push_deliveries
    ADD CONSTRAINT push_deliveries_registration_id_fkey FOREIGN KEY (registration_id) REFERENCES public.push_device_registrations(id) ON DELETE SET NULL;


--
-- Name: report_schedule_deliveries report_schedule_deliveries_run_fk; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.report_schedule_deliveries
    ADD CONSTRAINT report_schedule_deliveries_run_fk FOREIGN KEY (run_id, tenant_id) REFERENCES public.report_schedule_runs(id, tenant_id) ON DELETE RESTRICT;


--
-- Name: report_schedule_runs report_schedule_runs_retry_parent_fk; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.report_schedule_runs
    ADD CONSTRAINT report_schedule_runs_retry_parent_fk FOREIGN KEY (retry_parent_run_id, tenant_id, schedule_id) REFERENCES public.report_schedule_runs(id, tenant_id, schedule_id) ON DELETE RESTRICT;


--
-- Name: report_schedule_runs report_schedule_runs_schedule_fk; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.report_schedule_runs
    ADD CONSTRAINT report_schedule_runs_schedule_fk FOREIGN KEY (schedule_id, tenant_id) REFERENCES public.report_schedules(id, tenant_id) ON DELETE RESTRICT;


--
-- Name: report_schedules report_schedules_last_run_fk; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.report_schedules
    ADD CONSTRAINT report_schedules_last_run_fk FOREIGN KEY (last_run_id, tenant_id, id) REFERENCES public.report_schedule_runs(id, tenant_id, schedule_id) ON DELETE RESTRICT;


--
-- Name: rule_chain_edges rule_chain_edges_chain_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.rule_chain_edges
    ADD CONSTRAINT rule_chain_edges_chain_id_fkey FOREIGN KEY (chain_id) REFERENCES public.rule_chains(id) ON DELETE CASCADE;


--
-- Name: rule_chain_nodes rule_chain_nodes_chain_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.rule_chain_nodes
    ADD CONSTRAINT rule_chain_nodes_chain_id_fkey FOREIGN KEY (chain_id) REFERENCES public.rule_chains(id) ON DELETE CASCADE;


--
-- Name: scada_document_versions scada_document_versions_document_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.scada_document_versions
    ADD CONSTRAINT scada_document_versions_document_id_fkey FOREIGN KEY (document_id) REFERENCES public.scada_documents(id) ON DELETE CASCADE;


--
-- Name: scada_documents scada_documents_project_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.scada_documents
    ADD CONSTRAINT scada_documents_project_id_fkey FOREIGN KEY (project_id) REFERENCES public.scada_projects(id) ON DELETE CASCADE;


--
-- Name: scene_action_info scene_action_info_scene_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.scene_action_info
    ADD CONSTRAINT scene_action_info_scene_id_fkey FOREIGN KEY (scene_id) REFERENCES public.scene_info(id) ON DELETE CASCADE;


--
-- Name: periodic_tasks scene_automation_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.periodic_tasks
    ADD CONSTRAINT scene_automation_id_fkey FOREIGN KEY (scene_automation_id) REFERENCES public.scene_automations(id) ON DELETE CASCADE;


--
-- Name: scene_automation_log scene_automation_log_scene_automation_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.scene_automation_log
    ADD CONSTRAINT scene_automation_log_scene_automation_id_fkey FOREIGN KEY (scene_automation_id) REFERENCES public.scene_automations(id) ON DELETE CASCADE;


--
-- Name: scene_log scene_log_scene_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.scene_log
    ADD CONSTRAINT scene_log_scene_id_fkey FOREIGN KEY (scene_id) REFERENCES public.scene_info(id) ON DELETE CASCADE;


--
-- Name: sys_dict_language sys_dict_language_dict_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.sys_dict_language
    ADD CONSTRAINT sys_dict_language_dict_id_fkey FOREIGN KEY (dict_id) REFERENCES public.sys_dict(id) ON DELETE CASCADE;


--
-- Name: telemetry_set_logs telemetry_set_logs_device_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.telemetry_set_logs
    ADD CONSTRAINT telemetry_set_logs_device_id_fkey FOREIGN KEY (device_id) REFERENCES public.devices(id) ON DELETE CASCADE;


--
-- PostgreSQL database dump complete
--


SET LOCAL check_function_bodies = true;
