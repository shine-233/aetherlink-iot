// 文件用途：定义 device trigger condition 相关 HTTP 入参、出参和列表查询结构，承接 API 层与模型层的数据契约。
// 核心逻辑：使用 json/form/validate 标签描述请求校验、分页筛选和响应字段，保持 handler 与 service 的传参稳定。
// 关键注意事项：这里只维护传输结构和校验标签，不放入权限、事务或数据库访问等业务逻辑。
// 重构建议：接口字段变化时同步 OpenAPI/前端调用和服务层映射，公共分页或筛选结构可继续抽成复用类型。

package model

const (
	DEVICE_TRIGGER_CONDITION_TYPE_ONE      = "10" // 单个设备
	DEVICE_TRIGGER_CONDITION_TYPE_MULTIPLE = "11" // 单类设备
	DEVICE_TRIGGER_CONDITION_TYPE_TIME     = "22" // 时间范围
	// 条件类型
	TRIGGER_PARAM_TYPE_TEL        = "TEL"        // 遥测TEL
	TRIGGER_PARAM_TYPE_TELEMETRY  = "TELEMETRY"  // 遥测TEL
	TRIGGER_PARAM_TYPE_ATTR       = "ATTR"       // 属性ATTR
	TRIGGER_PARAM_TYPE_ATTRIBUTES = "ATTRIBUTES" // 属性ATTR
	TRIGGER_PARAM_TYPE_EVT        = "EVT"        // 事件EVT
	TRIGGER_PARAM_TYPE_EVENT      = "EVENT"      // 事件EVT
	TRIGGER_PARAM_TYPE_STATUS     = "STATUS"     // 状态STATUS

	// 运算符
	CONDITION_TRIGGER_OPERATOR_EQ      = "="       // 等于
	CONDITION_TRIGGER_OPERATOR_NEQ     = "!="      // 不等于
	CONDITION_TRIGGER_OPERATOR_GT      = ">"       // 大于
	CONDITION_TRIGGER_OPERATOR_LT      = "<"       // 大于
	CONDITION_TRIGGER_OPERATOR_GTE     = ">="      // 大于
	CONDITION_TRIGGER_OPERATOR_LTE     = "<="      // 大于
	CONDITION_TRIGGER_OPERATOR_BETWEEN = "between" // 大于
	CONDITION_TRIGGER_OPERATOR_IN      = "in"      // 大于

	// 动作类型
	AUTOMATE_ACTION_TYPE_ONE      = "10" // 单个设备
	AUTOMATE_ACTION_TYPE_MULTIPLE = "11" // 单类设备
	AUTOMATE_ACTION_TYPE_SCENE    = "20" // 激活场景
	AUTOMATE_ACTION_TYPE_ALARM    = "30" // 告警
	AUTOMATE_ACTION_TYPE_SERVICE  = "40" // 服务

)
