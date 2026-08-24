/**
 * 文件用途：声明式可选外部能力清单，供严格集成门禁豁免已声明的 optional/external blocked。
 * 核心逻辑：匹配 blockedReason 序列化文本；全部命中清单才可豁免，混合或未知原因一律视为阻断。
 * 关键注意事项：新增可选外部能力必须先在项目边界文档中声明，再扩充本清单。
 */

// 声明式可选外部能力清单：匹配 blockedReason 序列化文本（大小写不敏感）。
const OPTIONAL_EXTERNAL_PATTERNS = [/thingsvis/i];

function isOptionalExternalBlockedReason(reason) {
  if (reason == null) {
    return false;
  }
  let serialized;
  try {
    serialized = JSON.stringify(reason);
  } catch (err) {
    return false;
  }
  if (!serialized) {
    return false;
  }
  return OPTIONAL_EXTERNAL_PATTERNS.some(pattern => pattern.test(serialized));
}

function partitionBlockedReasons(blockedReasons) {
  const optionalExternal = [];
  const blocking = [];
  (Array.isArray(blockedReasons) ? blockedReasons : []).forEach(reason => {
    if (isOptionalExternalBlockedReason(reason)) {
      optionalExternal.push(reason);
    } else {
      blocking.push(reason);
    }
  });
  return { optionalExternal, blocking };
}

module.exports = {
  OPTIONAL_EXTERNAL_PATTERNS,
  isOptionalExternalBlockedReason,
  partitionBlockedReasons
};
