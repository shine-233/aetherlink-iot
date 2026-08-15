/*
 * 文件用途：定义 ThingsVis 集成所需的项目、字段、保存消息、平台数据和 SSO 类型。
 * 核心逻辑：集中声明 iframe 通信、平台字段和认证响应的数据结构。
 * 关键注意事项：类型变更会影响 SDK、URL 构建和嵌入页面契约。
 * 重构建议：后续可与后端 API schema 对齐，减少手写漂移。
 */
/**
 * ThingsVis 相关类型定义
 */

import type { ThingsVisCompatPlatform } from './constants'

/** 平台字段类型 */
export interface PlatformField {
  /** 字段唯一标识 */
  id: string
  /** 显示名称 */
  name: string
  /** 数据类型 */
  type: 'number' | 'string' | 'boolean' | 'json'
  /** 字段类型 */
  dataType: 'attribute' | 'telemetry' | 'command' | 'event'
  /** 单位(可选) */
  unit?: string
  /** 描述(可选) */
  description?: string
}

/** ThingsVis 项目配置 */
export interface ThingsVisProject {
  meta: {
    version: string
    id: string
    name: string
    createdAt?: number
    updatedAt?: number
  }
  canvas: {
    mode: string
    width: number
    height: number
    background: string
    gridEnabled?: boolean
  }
  nodes: any[]
  dataSources: any[]
  dataBindings: any[]
}

/** 保存请求消息 */
export interface SaveRequestMessage {
  type: 'tv:request-save'
  requestId: string
  payload: ThingsVisProject
}

/** 保存响应消息 */
export interface SaveResponseMessage {
  type: 'tv:save-response'
  requestId: string
  payload: {
    success: boolean
    data?: any
    error?: string
  }
}

/** 平台数据推送消息 */
export interface PlatformDataMessage {
  type: 'tv:platform-data'
  payload: {
    fieldId: string
    value: any
    timestamp: number
  }
}

/** SSO Token 交换请求 */
export interface SSOExchangeRequest {
  /** 平台标识 */
  platform: ThingsVisCompatPlatform
  /** AetherLink IoT JWT Token */
  platformToken: string
  /** 用户信息 */
  userInfo: {
    /** 用户 ID */
    id: string
    /** 用户邮箱 */
    email: string
    /** 用户名称 */
    name?: string
    /** 租户 ID */
    tenantId: string
  }
  /** ThingsVis 角色，用于注册时初始化默认看板。
   * SYS_ADMIN → SUPER_ADMIN, TENANT_ADMIN → TENANT_ADMIN, 其他 → EDITOR */
  role?: 'SUPER_ADMIN' | 'TENANT_ADMIN' | 'EDITOR' | 'VIEWER'
}

/** SSO Token 交换响应 */
export interface SSOExchangeResponse {
  /** ThingsVis JWT Access Token */
  accessToken: string
  /** 刷新 Token */
  refreshToken?: string
  /** Token 过期时间(秒) */
  expiresIn: number
  /** 用户信息 */
  user?: {
    id: string
    email: string
    name?: string
    role?: string
  }
}
