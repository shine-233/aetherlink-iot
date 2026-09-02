/**
 * Static coverage-contract helpers for automation evidence classification.
 *
 * Coverage hits prove that endpoints or pages were exercised; they do not
 * replace business oracles, negative evidence, or runtime verification.
 */
const fs = require('fs');
const path = require('path');

const endpointCoverage = require('./endpoint_coverage');
const pageCoverage = require('./page_coverage');
const testMetadata = require('./test_metadata');
const { BUSINESS_CAPABILITIES, PARENT_ROUTES } = require('./coverage-contract/business-capabilities');
const readiness = require('./coverage-contract/readiness');

const PROJECT_ROOT = path.resolve(__dirname, '..', '..');
const FRONTEND_ROOT = path.join(PROJECT_ROOT, 'frontend');
const BACKEND_ROOT = path.join(PROJECT_ROOT, 'backend');
const GMQTT_ROOT = path.join(PROJECT_ROOT, 'mqtt-broker');
const GMQTT_AETHERLINK_PLUGIN_ROOT = path.join(GMQTT_ROOT, 'plugin', 'aetherlink');
const VALID_AUTOMATION_EVIDENCE_KINDS = new Set([
  'business',
  'boundary',
  'catalog',
  'config',
  'contract',
  'preflight',
  'page-coverage-only'
]);
const VALID_RUNNER_OUTCOMES = new Set([
  'passed',
  'failed',
  'partial-skip',
  'all-skipped'
]);
const AUTOMATION_EVIDENCE_KIND_DETAILS = {
  business: {
    businessClosureEvidence: true,
    note: 'seeded/status/body evidence may contribute to business closure'
  },
  boundary: {
    businessClosureEvidence: false,
    note: 'boundary/API-contract evidence only; does not prove business closure'
  },
  catalog: {
    businessClosureEvidence: false,
    note: 'catalog alignment evidence only; does not prove business closure'
  },
  config: {
    businessClosureEvidence: false,
    note: 'runtime configuration evidence only; does not prove business closure'
  },
  contract: {
    businessClosureEvidence: false,
    note: 'harness/source contract evidence only; does not prove business closure'
  },
  preflight: {
    businessClosureEvidence: false,
    note: 'environment preflight evidence only; does not prove business closure'
  },
  'page-coverage-only': {
    businessClosureEvidence: false,
    note: 'page visit evidence only; does not prove business closure'
  },
  unknown: {
    businessClosureEvidence: false,
    note: 'unknown evidence kind; must not prove business closure'
  }
};
const NON_BUSINESS_AUTOMATION_TEST_PATTERNS = [
  { pattern: /(?:^|\/)17_api_(?:boundary_smoke|coverage_closure)\.test\.js$/, evidenceKind: 'boundary' },
  { pattern: /(?:^|\/)00_endpoint_coverage\.test\.js$/, evidenceKind: 'catalog' },
  { pattern: /(?:^|\/)00_(?:coverage_contract|oracle_contract)\.test\.js$/, evidenceKind: 'contract' },
  { pattern: /(?:^|\/)00_runtime_config_env\.test\.js$/, evidenceKind: 'config' },
  { pattern: /(?:^|\/)00_preflight_api_e2e\.test\.js$/, evidenceKind: 'preflight' }
];
const AUTOMATION_EVIDENCE_DIRECTIVE_KIND = {
  boundary: 'boundary',
  catalog: 'catalog',
  config: 'config',
  contract: 'contract',
  preflight: 'preflight',
  business: 'business'
};
const BLOCKED_METADATA_CATEGORIES = new Set([
  'runtime-external',
  'seedable-local',
  'partial-skip',
  'all-skipped',
  'unknown'
]);

const FRONTEND_ELEGANT_ROUTE_ROOT = path.join(FRONTEND_ROOT, 'src', 'router', 'elegant');

function readText(filePath) {
  return fs.existsSync(filePath) ? fs.readFileSync(filePath, 'utf8') : '';
}

function getTopOfFileCommentBlock(text) {
  const normalized = String(text || '').replace(/^\uFEFF/, '');
  const lines = normalized.split(/\r?\n/);
  const commentLines = [];

  for (const line of lines) {
    const trimmed = line.trim();
    if (!trimmed) {
      if (commentLines.length > 0) {
        break;
      }
      continue;
    }
    if (/^(?:\/\/|\/\*|\*|\*\/)/.test(trimmed)) {
      commentLines.push(trimmed);
      continue;
    }
    break;
  }

  return commentLines.join('\n');
}

function parseTopOfFileAutomationEvidenceDirective(text) {
  const normalized = String(text || '').replace(/^\uFEFF/, '');
  const lines = normalized.split(/\r?\n/).slice(0, 80);
  const directiveSource = lines
    .map(line => line.trim())
    .filter(line => /^(?:\/\/|\/\*|\*|\*\/)/.test(line))
    .join('\n');
  const directiveMatch = directiveSource.match(/@file-([a-z0-9_-]+?)-evidence-only\b/i);
  if (!directiveMatch) {
    return null;
  }

  const directiveKind = directiveMatch[1].toLowerCase();
  return AUTOMATION_EVIDENCE_DIRECTIVE_KIND[directiveKind] || null;
}

function normalizeBooleanMetadataValue(value, fallback = false) {
  if (typeof value === 'boolean') {
    return value;
  }
  if (typeof value === 'string') {
    const normalized = value.trim().toLowerCase();
    if (normalized === 'true') {
      return true;
    }
    if (normalized === 'false') {
      return false;
    }
  }
  return fallback;
}

function normalizeBlockedMetadataCategory(category) {
  const normalized = typeof category === 'string' ? category.trim() : '';
  return BLOCKED_METADATA_CATEGORIES.has(normalized) ? normalized : null;
}

function extractObjectLiteralStringProperty(literal, propertyName) {
  const match = literal.match(new RegExp(propertyName + "\\s*:\\s*(['\"])((?:\\\\.|(?!\\1)[\\s\\S])*)\\1"));
  return match ? match[2] : null;
}

function extractObjectLiteralBooleanProperty(literal, propertyName) {
  const match = literal.match(new RegExp(propertyName + "\\s*:\\s*(true|false)\\b", 'i'));
  return match ? match[1].toLowerCase() === 'true' : null;
}

function extractStructuredSkipMetadataFromObjectLiteral(literal) {
  if (!literal || literal.indexOf('{') === -1) {
    return null;
  }

  const reason = extractObjectLiteralStringProperty(literal, 'reason');
  const category = normalizeBlockedMetadataCategory(extractObjectLiteralStringProperty(literal, 'category'));
  const seedable = extractObjectLiteralBooleanProperty(literal, 'seedable');
  const outcome = extractObjectLiteralStringProperty(literal, 'outcome');

  if (!reason && !category && seedable === null && !outcome) {
    return null;
  }

  return {
    reason: reason || '',
    category,
    seedable,
    outcome: outcome || ''
  };
}

function parseObjectLiteralAtIndex(text, braceIndex) {
  if (braceIndex < 0 || text[braceIndex] !== '{') {
    return null;
  }

  let depth = 0;
  let quote = null;
  let escapeNext = false;

  for (let index = braceIndex; index < text.length; index++) {
    const char = text[index];

    if (quote) {
      if (escapeNext) {
        escapeNext = false;
      } else if (char === '\\') {
        escapeNext = true;
      } else if (char === quote) {
        quote = null;
      }
      continue;
    }

    if (char === '"' || char === '\'' || char === '`') {
      quote = char;
      continue;
    }
    if (char === '{') {
      depth += 1;
      continue;
    }
    if (char === '}') {
      depth -= 1;
      if (depth === 0) {
        return text.slice(braceIndex, index + 1);
      }
    }
  }

  return null;
}

function getCallArgumentSlice(text, callStartIndex) {
  const openParenIndex = text.indexOf('(', callStartIndex);
  if (openParenIndex === -1) {
    return '';
  }

  let depth = 0;
  let quote = null;
  let escapeNext = false;

  for (let index = openParenIndex; index < text.length; index++) {
    const char = text[index];

    if (quote) {
      if (escapeNext) {
        escapeNext = false;
      } else if (char === '\\') {
        escapeNext = true;
      } else if (char === quote) {
        quote = null;
      }
      continue;
    }

    if (char === '"' || char === '\'' || char === '`') {
      quote = char;
      continue;
    }
    if (char === '(') {
      depth += 1;
      continue;
    }
    if (char === ')') {
      depth -= 1;
      if (depth === 0) {
        return text.slice(openParenIndex + 1, index);
      }
    }
  }

  return '';
}

function splitTopLevelArguments(argumentSlice) {
  const args = [];
  let current = '';
  let depthParen = 0;
  let depthBrace = 0;
  let depthBracket = 0;
  let quote = null;
  let escapeNext = false;

  for (const char of String(argumentSlice || '')) {
    if (quote) {
      current += char;
      if (escapeNext) {
        escapeNext = false;
      } else if (char === '\\') {
        escapeNext = true;
      } else if (char === quote) {
        quote = null;
      }
      continue;
    }

    if (char === '"' || char === '\'' || char === '`') {
      quote = char;
      current += char;
      continue;
    }
    if (char === '(') depthParen += 1;
    if (char === ')') depthParen -= 1;
    if (char === '{') depthBrace += 1;
    if (char === '}') depthBrace -= 1;
    if (char === '[') depthBracket += 1;
    if (char === ']') depthBracket -= 1;

    if (char === ',' && depthParen === 0 && depthBrace === 0 && depthBracket === 0) {
      args.push(current.trim());
      current = '';
      continue;
    }

    current += char;
  }

  if (current.trim()) {
    args.push(current.trim());
  }

  return args;
}

function getStructuredSkipMetadataFromCall(text, helperName, matchIndex) {
  const argumentSlice = getCallArgumentSlice(text, matchIndex);
  if (!argumentSlice) {
    return null;
  }

  const args = splitTopLevelArguments(argumentSlice);
  const metadataArgIndex = helperName === 'skipWhenBlocked' ? 2 : 1;
  const metadataArg = args[metadataArgIndex];
  if (!metadataArg) {
    return null;
  }

  if (/^['"`]/.test(metadataArg.trim())) {
    return null;
  }

  if (metadataArg.trim().startsWith('{')) {
    return extractStructuredSkipMetadataFromObjectLiteral(metadataArg.trim());
  }

  const inlineObjectStart = metadataArg.indexOf('{');
  if (inlineObjectStart === -1) {
    return null;
  }

  const literal = parseObjectLiteralAtIndex(metadataArg, inlineObjectStart);
  return extractStructuredSkipMetadataFromObjectLiteral(literal);
}

function walkFiles(dir, predicate, result = []) {
  if (!fs.existsSync(dir)) {
    return result;
  }
  for (const entry of fs.readdirSync(dir, { withFileTypes: true })) {
    const fullPath = path.join(dir, entry.name);
    if (entry.isDirectory()) {
      walkFiles(fullPath, predicate, result);
    } else if (!predicate || predicate(fullPath)) {
      result.push(fullPath);
    }
  }
  return result;
}

function getSourceLeafRoutes() {
  const routeFiles = walkFiles(
    FRONTEND_ELEGANT_ROUTE_ROOT,
    filePath => /\.ts$/.test(filePath) && !/\.test\.ts$/.test(filePath)
  );
  const text = routeFiles.map(readText).join('\n');
  const routes = Array.from(text.matchAll(/path:\s*'([^']+)'/g)).map(match => match[1]);
  return Array.from(new Set(routes.filter(route => !PARENT_ROUTES.has(route)))).sort();
}

function normalizeCatalogRoute(route) {
  if (route === '/login') return '/login/:module(pwd-login|code-login|register|register-email|register-super-admin|reset-pwd|bind-wechat)?';
  if (route.startsWith('/login/')) return '/login/:module(pwd-login|code-login|register|register-email|register-super-admin|reset-pwd|bind-wechat)?';
  return route;
}

function getPageCatalogRoutes() {
  // Parent layout routes are represented in the generated route tree but are
  // intentionally excluded from getSourceLeafRoutes(). Keep the comparison at
  // the same leaf-route level while retaining parent entries in page coverage
  // for explicit redirect/permission checks.
  return Array.from(new Set(
    pageCoverage.ALL_PAGES
      .map(page => normalizeCatalogRoute(page.route))
      .filter(route => !PARENT_ROUTES.has(route))
  )).sort();
}

function getEndpointCatalogKeys() {
  return endpointCoverage.ALL_ENDPOINTS.map(endpoint => `${endpoint.method} ${endpoint.path}`);
}

function normalizeRoutePart(part) {
  return String(part || '').replace(/^\/+|\/+$/g, '');
}

function joinRouteParts(...parts) {
  const joined = parts
    .map(normalizeRoutePart)
    .filter(Boolean)
    .join('/');
  return '/' + joined;
}

function stripGoComments(text) {
  return text
    .replace(/\/\*[\s\S]*?\*\//g, '')
    .split(/\r?\n/)
    .map(line => line.replace(/\/\/.*$/g, ''))
    .join('\n');
}

function getBackendSourceEndpointKeys() {
  const routerRoot = path.join(BACKEND_ROOT, 'router');
  const files = walkFiles(routerRoot, filePath => /\.go$/.test(filePath) && !/_test\.go$/.test(filePath));
  const endpoints = new Set();

  for (const filePath of files) {
    const text = stripGoComments(readText(filePath));
    const relPath = path.relative(routerRoot, filePath).replace(/\\/g, '/');

    if (relPath === 'router_init.go') {
      for (const match of text.matchAll(/\brouter\.(GET|POST|PUT|DELETE|PATCH|StaticFile)\(\s*"([^"]+)"/g)) {
        const method = match[1] === 'StaticFile' ? 'GET' : match[1];
        endpoints.add(`${method} ${match[2]}`);
      }
      for (const match of text.matchAll(/\bv1\.(GET|POST|PUT|DELETE|PATCH)\(\s*"([^"]+)"/g)) {
        endpoints.add(`${match[1]} ${joinRouteParts('/api/v1', match[2])}`);
      }
      // plugin 组（middleware.PluginAuth 保护）沿用 /api/v1 前缀。
      for (const match of text.matchAll(/\bplugin\.(GET|POST|PUT|DELETE|PATCH)\(\s*"([^"]+)"/g)) {
        endpoints.add(`${match[1]} ${joinRouteParts('/api/v1', match[2])}`);
      }
      continue;
    }

    collectRouterFileEndpoints(text).forEach(endpoint => endpoints.add(endpoint));
  }

  return Array.from(endpoints).sort();
}

function collectRouterFileEndpoints(text) {
  const functions = extractGoFunctions(text);
  const endpoints = new Set();

  const parseFunction = (fn, basePath, stack = []) => {
    if (!fn || stack.includes(fn.name)) {
      return;
    }

    const groupByVar = { Router: basePath, r: basePath };
    const lines = fn.body.split(/\r?\n/);

    for (const line of lines) {
      const groupMatch = line.match(/\b(\w+)\s*:=\s*(\w+)\.Group\(\s*"([^"]*)"\s*\)/);
      if (groupMatch && groupByVar[groupMatch[2]]) {
        groupByVar[groupMatch[1]] = joinRouteParts(groupByVar[groupMatch[2]], groupMatch[3]);
        continue;
      }

      const routeCall = line.match(/\b(\w+)\.(GET|POST|PUT|DELETE|PATCH)\(\s*"([^"]*)"/);
      if (routeCall && groupByVar[routeCall[1]]) {
        endpoints.add(`${routeCall[2]} ${joinRouteParts(groupByVar[routeCall[1]], routeCall[3])}`);
        continue;
      }

      const helperCall = line.match(/^\s*(\w+)\((\w+)\)\s*$/);
      if (helperCall && functions[helperCall[1]] && groupByVar[helperCall[2]]) {
        parseFunction(functions[helperCall[1]], groupByVar[helperCall[2]], [...stack, fn.name]);
      }
    }
  };

  Object.values(functions)
    .filter(fn => fn.isEntry)
    .forEach(fn => parseFunction(fn, '/api/v1'));

  return Array.from(endpoints);
}

function extractGoFunctions(text) {
  const functions = {};
  const fnPattern = /func\s+(?:\([^)]*\)\s*)?(\w+)\s*\([^)]*\)\s*\{/g;
  let match;

  while ((match = fnPattern.exec(text)) !== null) {
    const name = match[1];
    const bodyStart = fnPattern.lastIndex;
    let depth = 1;
    let index = bodyStart;

    while (index < text.length && depth > 0) {
      const char = text[index];
      if (char === '{') depth++;
      if (char === '}') depth--;
      index++;
    }

    const signature = match[0];
    const body = text.slice(bodyStart, index - 1);
    functions[name] = {
      name,
      body,
      isEntry: /\bInit\w*\s*\(/.test(signature) || /\bSSERouter\s*\(/.test(signature)
    };
    fnPattern.lastIndex = index;
  }

  return functions;
}

function compareEndpointCatalogToSource() {
  const sourceEndpoints = getBackendSourceEndpointKeys();
  const catalogEndpoints = Array.from(new Set(getEndpointCatalogKeys())).sort();
  // Gin does not require a parameter name to match the client/catalog name.
  // Compare route shapes separately so `:id` vs `:device_id` is diagnostic
  // information, not a false missing/extra endpoint.
  const normalizeEndpointShape = endpoint => String(endpoint).replace(/:[^/\s]+/g, ':param');
  const sourceShapes = new Map();
  const catalogShapes = new Map();
  const addShape = (map, endpoint) => {
    const shape = normalizeEndpointShape(endpoint);
    if (!map.has(shape)) map.set(shape, []);
    map.get(shape).push(endpoint);
  };
  sourceEndpoints.forEach(endpoint => addShape(sourceShapes, endpoint));
  catalogEndpoints.forEach(endpoint => addShape(catalogShapes, endpoint));

  const parameterNameMismatches = [];
  for (const [shape, sourceVariants] of sourceShapes.entries()) {
    const catalogVariants = catalogShapes.get(shape);
    if (!catalogVariants) continue;
    for (const source of sourceVariants) {
      for (const catalog of catalogVariants) {
        if (source !== catalog) {
          parameterNameMismatches.push({ shape, source, catalog });
        }
      }
    }
  }

  return {
    sourceEndpoints,
    catalogEndpoints,
    missingFromCatalog: sourceEndpoints.filter(endpoint => !catalogShapes.has(normalizeEndpointShape(endpoint))),
    extraInCatalog: catalogEndpoints.filter(endpoint => !sourceShapes.has(normalizeEndpointShape(endpoint))),
    parameterNameMismatches
  };
}

function comparePageCatalogToSource() {
  const sourceRoutes = getSourceLeafRoutes();
  const catalogRoutes = getPageCatalogRoutes();
  const catalogSet = new Set(catalogRoutes);
  const sourceSet = new Set(sourceRoutes);
  return {
    sourceRoutes,
    catalogRoutes,
    missingFromCatalog: sourceRoutes.filter(route => !catalogSet.has(route)),
    extraInCatalog: catalogRoutes.filter(route => !sourceSet.has(route) && !route.startsWith('/login/'))
  };
}

function compareEndpointCapabilityMap() {
  const endpointSet = new Set(getEndpointCatalogKeys());
  const missing = [];
  for (const capability of BUSINESS_CAPABILITIES) {
    for (const endpoint of capability.endpoints) {
      if (!endpointSet.has(endpoint)) {
        missing.push({ capability: capability.id, endpoint });
      }
    }
  }
  return missing;
}

function classifyEndpointCatalogItem(endpointKey) {
  const pathPart = String(endpointKey || '').replace(/^\w+\s+/, '');

  if (/^\/(?:health|ready|metrics|metrics-viewer|files\/|swagger\/)/.test(pathPart)) {
    return { endpoint: endpointKey, scope: 'infra', capability: 'system-deployment' };
  }
  if (/^\/api\/v1\/(?:login|user(?:\/|$)|role(?:\/|$)|casbin(?:\/|$)|tenant(?:\/|$)|reset\/password|verification\/code|dict(?:\/|$)|ui_elements(?:\/|$))/.test(pathPart)) {
    return { endpoint: endpointKey, scope: 'P0/P1', capability: 'permission-tenancy' };
  }
  if (/^\/api\/v1\/command\/datas\/(?:jobs|delivery\/diagnostics|saved-filters)(?:\/|$)/.test(pathPart) ||
      /^\/api\/v1\/payload-schema(?:\/|$)/.test(pathPart)) {
    return { endpoint: endpointKey, scope: 'P0/P1', capability: 'command-jobs' };
  }
  if (/^\/api\/v1\/ai(?:\/|$)/.test(pathPart)) {
    return { endpoint: endpointKey, scope: 'P0/P1', capability: 'device-telemetry' };
  }
  // 计算字段（遥测派生指标）归属设备遥测能力，必须在通用 device 分支之前匹配。
  if (/^\/api\/v1\/calculated_fields(?:\/|$)/.test(pathPart)) {
    return { endpoint: endpointKey, scope: 'P0/P1', capability: 'device-telemetry' };
  }
  if (/^\/api\/v1\/(?:device(?:\/|$)|devices(?:\/|$)|telemetry(?:\/|$)|attribute(?:\/|$)|event(?:\/|$)|events(?:\/|$)|command(?:\/|$)|expected(?:\/|$)|datapolicy(?:\/|$)|device_config(?:\/|$))/.test(pathPart)) {
    return { endpoint: endpointKey, scope: 'P0/P1', capability: 'device-telemetry' };
  }
  if (/^\/api\/v1\/rdi(?:\/|$)/.test(pathPart)) {
    return { endpoint: endpointKey, scope: 'P0/P1', capability: 'rdi' };
  }
  if (/^\/api\/v1\/(?:alarm(?:\/|$)|notification(?:\/|$)|notification_group(?:\/|$)|notification_history(?:\/|$)|message_push(?:\/|$))/.test(pathPart)) {
    return { endpoint: endpointKey, scope: 'P0/P1', capability: 'alarm-notification' };
  }
  if (/^\/api\/v1\/scene(?:_automations)?(?:\/|$)/.test(pathPart)) {
    return { endpoint: endpointKey, scope: 'P0/P1', capability: 'automation-scene' };
  }
  // 规则链（ROADMAP B2）归自动化场景能力，必须在通用 device 分支之前匹配。
  if (/^\/api\/v1\/rule-chains(?:\/|$)/.test(pathPart)) {
    return { endpoint: endpointKey, scope: 'P0/P1', capability: 'automation-scene' };
  }
  // 产品选择列表（预注册建档数据源）归设备遥测能力。
  if (/^\/api\/v1\/product(?:\/|$)/.test(pathPart)) {
    return { endpoint: endpointKey, scope: 'P0/P1', capability: 'device-telemetry' };
  }
  if (/^\/api\/v1\/(?:board(?:\/|$)|dashboard-menu(?:\/|$))/.test(pathPart)) {
    return { endpoint: endpointKey, scope: 'P0/P1', capability: 'visualization' };
  }
  if (/^\/api\/v1\/(?:ota(?:\/|$)|data_script(?:\/|$)|open(?:\/|$)|service(?:\/|$)|plugin(?:\/|$)|protocol_plugin(?:\/|$)|file\/up)/.test(pathPart)) {
    return { endpoint: endpointKey, scope: 'P0/P1', capability: 'ota-script-openapi-service' };
  }
  if (/^\/deployment\/health$/.test(pathPart) || /^\/api\/v1\/(?:deployment\/health|sys_|systime|logo|operation_logs|system\/metrics)/.test(pathPart)) {
    return { endpoint: endpointKey, scope: 'P0/P1', capability: 'system-deployment' };
  }

  return { endpoint: endpointKey, scope: 'unknown', capability: null };
}

function classifyPageCatalogRoute(route) {
  if (/^\/login/.test(route) || /^\/(?:manage|management|personal-center)(?:\/|$)/.test(route)) {
    return { route, scope: 'P0/P1', capability: 'permission-tenancy' };
  }
  if (route === '/device/command-center') {
    return { route, scope: 'P0/P1', capability: 'command-jobs' };
  }
  if (/^\/device(?:\/|$)|^\/device-details-app$/.test(route)) {
    return { route, scope: 'P0/P1', capability: 'device-telemetry' };
  }
  if (/^\/alarm(?:\/|$)/.test(route)) {
    return { route, scope: 'P0/P1', capability: 'alarm-notification' };
  }
  if (/^\/automation(?:\/|$)/.test(route)) {
    return { route, scope: 'P0/P1', capability: 'automation-scene' };
  }
  if (/^\/(?:visualization|dashboard)(?:\/|$)/.test(route)) {
    return { route, scope: 'P0/P1', capability: 'visualization' };
  }
  if (/^\/(?:product|apply)(?:\/|$)/.test(route)) {
    // 预注册建档属设备能力；其余 product 页（update-ota/update-package）保持 OTA 归属。
    if (route === '/product/pre-register') {
      return { route, scope: 'P0/P1', capability: 'device-telemetry' };
    }
    return { route, scope: 'P0/P1', capability: 'ota-script-openapi-service' };
  }
  if (/^\/(?:home|system-management-user)(?:\/|$)|^\/(?:403|404|500)$/.test(route)) {
    return { route, scope: 'P0/P1', capability: 'system-deployment' };
  }

  return { route, scope: 'unknown', capability: null };
}

function summarizeClassifications(items) {
  return items.reduce((acc, item) => {
    const key = item.capability || item.scope;
    acc[key] = (acc[key] || 0) + 1;
    return acc;
  }, {});
}

function getCatalogClassificationAudit() {
  const endpointClassifications = getEndpointCatalogKeys().map(classifyEndpointCatalogItem);
  const routeClassifications = getPageCatalogRoutes().map(classifyPageCatalogRoute);

  return {
    endpointClassifications,
    routeClassifications,
    endpointClassSummary: summarizeClassifications(endpointClassifications),
    routeClassSummary: summarizeClassifications(routeClassifications),
    unclassifiedEndpoints: endpointClassifications.filter(item => item.scope === 'unknown'),
    unclassifiedRoutes: routeClassifications.filter(item => item.scope === 'unknown')
  };
}

function getExplicitBusinessInventoryAudit(catalogClassificationAudit = getCatalogClassificationAudit()) {
  const endpointSetsByCapability = new Map(
    BUSINESS_CAPABILITIES.map(capability => [capability.id, new Set(capability.endpoints || [])])
  );
  const routeSetsByCapability = new Map(
    BUSINESS_CAPABILITIES.map(capability => [
      capability.id,
      new Set((capability.frontendRoutes || []).map(normalizeCatalogRoute))
    ])
  );

  const missingEndpoints = catalogClassificationAudit.endpointClassifications.filter(item => {
    if (item.scope !== 'P0/P1' || !item.capability) {
      return false;
    }
    return !(endpointSetsByCapability.get(item.capability) || new Set()).has(item.endpoint);
  });
  const missingRoutes = catalogClassificationAudit.routeClassifications.filter(item => {
    if (item.scope !== 'P0/P1' || !item.capability) {
      return false;
    }
    return !(routeSetsByCapability.get(item.capability) || new Set()).has(item.route);
  });

  return {
    missingEndpoints,
    missingRoutes,
    missingEndpointCount: missingEndpoints.length,
    missingRouteCount: missingRoutes.length
  };
}

function summarizeExplicitInventoryGaps(explicitBusinessInventoryAudit = getExplicitBusinessInventoryAudit()) {
  const byCapability = {};
  const ensure = capability => {
    if (!byCapability[capability]) {
      byCapability[capability] = {
        capability,
        endpointCount: 0,
        routeCount: 0,
        endpoints: [],
        routes: []
      };
    }
    return byCapability[capability];
  };

  explicitBusinessInventoryAudit.missingEndpoints.forEach(item => {
    const entry = ensure(item.capability);
    entry.endpointCount += 1;
    entry.endpoints.push(item.endpoint);
  });
  explicitBusinessInventoryAudit.missingRoutes.forEach(item => {
    const entry = ensure(item.capability);
    entry.routeCount += 1;
    entry.routes.push(item.route);
  });

  return Object.values(byCapability).sort((left, right) => {
    const rightTotal = right.endpointCount + right.routeCount;
    const leftTotal = left.endpointCount + left.routeCount;
    if (rightTotal !== leftTotal) {
      return rightTotal - leftTotal;
    }
    return left.capability.localeCompare(right.capability);
  });
}

function getEndpointReferenceNeedles(endpoint) {
  const pathPart = String(endpoint || '').replace(/^\w+\s+/, '');
  const withoutApiPrefix = pathPart.replace(/^\/api\/v1/, '') || pathPart;
  const needles = new Set([pathPart, withoutApiPrefix]);
  [
    '/device/model/',
    '/device/template/market/',
    '/device/template/',
    '/device/group/',
    '/device/topic-mappings',
    '/telemetry/datas/',
    '/attribute/datas/',
    '/event/datas/',
    '/command/datas/',
    '/expected/data',
    '/service/access',
    '/scene_automations'
  ].forEach(prefix => {
    if (withoutApiPrefix.startsWith(prefix)) {
      needles.add(prefix);
    }
  });
  [pathPart, withoutApiPrefix].forEach(candidate => {
    const paramIndex = candidate.search(/\/:[^/]+/);
    if (paramIndex >= 0) {
      needles.add(candidate.slice(0, paramIndex + 1));
    }
  });
  return Array.from(needles).filter(needle => needle && needle !== '/');
}

function getRouteReferenceNeedles(route) {
  const normalizedRoute = normalizeCatalogRoute(route);
  const needles = new Set([normalizedRoute]);
  const paramIndex = normalizedRoute.search(/\/:[^/]+/);
  if (paramIndex >= 0) {
    needles.add(normalizedRoute.slice(0, paramIndex + 1));
  }
  return Array.from(needles).filter(needle => needle && needle !== '/');
}

function getAutomationEvidenceFiles() {
  const automationRoot = path.join(PROJECT_ROOT, 'automation_tests');
  const roots = [
    path.join(automationRoot, 'tests'),
    path.join(automationRoot, 'e2e'),
    path.join(automationRoot, 'lib')
  ];
  // Exclude harness self-auditing helpers so evidence traceability reflects authored suites,
  // not the contract/catalog utilities that inspect them.
  const excluded = new Set([
    'lib/coverage_contract.js',
    'lib/oracle_contract.js',
    'lib/endpoint_coverage.js',
    'lib/page_coverage.js'
  ]);

  return roots
    .flatMap(root => walkFiles(root, filePath => /\.(js|cjs|mjs)$/.test(filePath)))
    .map(filePath => ({
      file: path.relative(automationRoot, filePath).replace(/\\/g, '/'),
      text: readText(filePath)
    }))
    .filter(item => !excluded.has(item.file));
}

function findDirectReferences(needles, files = getAutomationEvidenceFiles()) {
  return files
    .filter(file => needles.some(needle => file.text.includes(needle)))
    .map(file => file.file)
    .sort();
}

function getExplicitBusinessInventoryGapReport(explicitBusinessInventoryAudit = getExplicitBusinessInventoryAudit()) {
  const evidenceFiles = getAutomationEvidenceFiles();
  const endpointReferences = explicitBusinessInventoryAudit.missingEndpoints.map(item => {
    const needles = getEndpointReferenceNeedles(item.endpoint);
    const directReferences = findDirectReferences(needles, evidenceFiles);
    return { ...item, needles, directReferences };
  });
  const routeReferences = explicitBusinessInventoryAudit.missingRoutes.map(item => {
    const needles = getRouteReferenceNeedles(item.route);
    const directReferences = findDirectReferences(needles, evidenceFiles);
    return { ...item, needles, directReferences };
  });
  const withReferences = {
    ...explicitBusinessInventoryAudit,
    missingEndpoints: endpointReferences,
    missingRoutes: routeReferences
  };
  const byCapability = summarizeExplicitInventoryGaps(withReferences).map(item => {
    const endpointItems = endpointReferences.filter(endpoint => endpoint.capability === item.capability);
    const routeItems = routeReferences.filter(route => route.capability === item.capability);
    const unreferencedEndpoints = endpointItems.filter(endpoint => endpoint.directReferences.length === 0);
    const unreferencedRoutes = routeItems.filter(route => route.directReferences.length === 0);
    return {
      ...item,
      directlyReferencedEndpointCount: endpointItems.length - unreferencedEndpoints.length,
      directlyReferencedRouteCount: routeItems.length - unreferencedRoutes.length,
      unreferencedEndpointCount: unreferencedEndpoints.length,
      unreferencedRouteCount: unreferencedRoutes.length,
      unreferencedEndpointSamples: unreferencedEndpoints.slice(0, 10).map(endpoint => endpoint.endpoint),
      unreferencedRouteSamples: unreferencedRoutes.slice(0, 10).map(route => route.route)
    };
  });
  const p0Capabilities = new Set(BUSINESS_CAPABILITIES
    .filter(capability => capability.priority === 'P0')
    .map(capability => capability.id));

  return {
    generatedAt: new Date().toISOString(),
    totalMissingEndpoints: explicitBusinessInventoryAudit.missingEndpointCount,
    totalMissingRoutes: explicitBusinessInventoryAudit.missingRouteCount,
    byCapability,
    missingEndpoints: endpointReferences,
    missingRoutes: routeReferences,
    nextCapability: byCapability.find(item => p0Capabilities.has(item.capability)) || byCapability[0] || null
  };
}

function getAutomationEvidenceForTraceability(automationRoot, testEntry) {
  const testPath = getAutomationTestPath(testEntry);
  const capabilityId = typeof testEntry === 'object' && testEntry !== null
    ? testEntry.capabilityId || testEntry.capability
    : null;
  const evidenceMetadata = getAutomationEvidenceMetadata(testEntry);
  const evidenceKind = evidenceMetadata.evidenceKind;
  const evidenceDetails = getAutomationEvidenceKindDetails(evidenceKind);
  const text = readText(path.join(automationRoot, testPath));
  const rawOracleCases = getAutomationOracleCases(text, testPath).filter(item => {
    if (!capabilityId || !Array.isArray(item.capabilityIds) || item.capabilityIds.length === 0) {
      return true;
    }
    return item.capabilityIds.includes(capabilityId);
  });
  const oracleCases = evidenceDetails.businessClosureEvidence
    ? rawOracleCases.filter(isBusinessClosureOracleCase)
    : [];

  return {
    file: testPath,
    evidenceKind,
    evidenceSource: evidenceMetadata.evidenceSource,
    evidenceNote: evidenceDetails.note,
    businessClosureEvidence: evidenceDetails.businessClosureEvidence,
    rawOracleCases,
    oracleCases,
    rawHasExactStatusAssertion: rawOracleCases.some(item => item.hasExactStatusAssertion),
    rawHasBodyAssertion: rawOracleCases.some(item => item.hasBodyAssertion),
    rawHasMutationOrSeedAction: rawOracleCases.some(item => item.hasMutationOrSeedAction),
    rawHasNegativeAssertion: rawOracleCases.some(item => item.hasNegativeAssertion),
    rawHasStatusBodyCase: rawOracleCases.some(item => item.hasStatusBodyCase),
    rawHasStatefulStatusBodyCase: rawOracleCases.some(item => item.hasStatefulStatusBodyCase),
    rawHasNegativeStatusCase: rawOracleCases.some(item => item.hasNegativeStatusCase),
    hasExactStatusAssertion: oracleCases.some(item => item.hasExactStatusAssertion),
    hasBodyAssertion: oracleCases.some(item => item.hasBodyAssertion),
    hasMutationOrSeedAction: oracleCases.some(item => item.hasMutationOrSeedAction),
    hasNegativeAssertion: oracleCases.some(item => item.hasNegativeAssertion),
    hasStatusBodyCase: oracleCases.some(item => item.hasStatusBodyCase),
    hasStatefulStatusBodyCase: oracleCases.some(item => item.hasStatefulStatusBodyCase),
    hasNegativeStatusCase: oracleCases.some(item => item.hasNegativeStatusCase)
  };
}

function getE2EEvidenceForTraceability(automationRoot, testPath, capabilityId = null) {
  const text = readText(path.join(automationRoot, testPath));
  const businessCases = getE2EBusinessCases(text, testPath, capabilityId);
  const coverageTag = getCoverageTagMetadata(text, null, null, testPath);

  return {
    file: testPath,
    hasUserAction: /\.click\(|\.fill\(|\.selectOption\(|\.check\(|\.press\(/.test(text),
    hasBusinessAssertion: /toHaveURL|toHaveText|toContainText|toHaveValue|toHaveAttribute|objectContaining|share-success|toast|table|Save|Create|Search|Reset/.test(text),
    hasSeedOrApiSetup: /seedData|api\.|ensureDevice|createTenantAdminAccount/.test(text),
    hasFallbackRisk: /expectRouteSmoke|FORBIDDEN_OR_PAGE_TEXT|currently blocked|currently 403|current .*shell|current local fallback|Back to Home|返回首页|403|Forbidden|No Permission/i.test(text),
    pageCoverageOnly: coverageTag.pageCoverageOnly,
    pageCoverageOnlySource: coverageTag.source,
    businessCases
  };
}

function hasTrueAutomationEvidence(automationEvidence) {
  return automationEvidence.length > 0 && automationEvidence.some(item =>
    item.evidenceKind === 'business' &&
    item.hasStatusBodyCase &&
    (item.hasStatefulStatusBodyCase || item.hasNegativeStatusCase)
  );
}

function hasTrueE2EEvidence(capability, e2eEvidence) {
  return capability.e2eTests.length > 0 && e2eEvidence.some(item =>
    item.businessCases.some(testCase => testCase.hasBrowserUserFlow !== false)
  );
}

function getBusinessTraceability() {
  const automationRoot = path.join(PROJECT_ROOT, 'automation_tests');
  return BUSINESS_CAPABILITIES.map(capability => {
    const automationEvidence = capability.automationTests.map(testEntry =>
      getAutomationEvidenceForTraceability(
        automationRoot,
        typeof testEntry === 'string'
          ? { file: testEntry, capabilityId: capability.id }
          : { ...testEntry, capabilityId: capability.id }
      )
    );
    const e2eEvidence = capability.e2eTests.map(testPath =>
      getE2EEvidenceForTraceability(automationRoot, testPath, capability.id)
    );
    const backendEvidence = capability.backendTests.map(testPath => getMappedTestFileStatus(capability.id, 'backend', testPath));
    const gmqttEvidence = capability.gmqttTests.map(testPath => getMappedTestFileStatus(capability.id, 'gmqtt', testPath));
    const hasTrueAutomation = hasTrueAutomationEvidence(automationEvidence);
    const hasTrueE2E = hasTrueE2EEvidence(capability, e2eEvidence);

    return {
      ...capability,
      automationEvidence,
      e2eEvidence,
      backendEvidence,
      gmqttEvidence,
      hasFrontendRoute: capability.frontendRoutes.length > 0,
      hasEndpoint: capability.endpoints.length > 0,
      hasAutomation: capability.automationTests.length > 0,
      hasTrueAutomation,
      hasE2E: capability.e2eTests.length > 0,
      hasTrueE2E,
      hasBackend: backendEvidence.some(item => item.exists && item.hasTestFunction),
      hasGMQTT: capability.priority === 'P0' && capability.id === 'mqtt-broker-pipeline'
        ? gmqttEvidence.some(item => item.exists && item.hasTestFunction)
        : true
    };
  });
}

function getAutomationOracleCases(text, testPath = null) {
  const metadata = testMetadata.getTestMetadata(testPath);
  if (metadata && Array.isArray(metadata.cases) && metadata.cases.length > 0) {
    return metadata.cases.map(item => ({
      title: item.title,
      capabilityIds: Array.isArray(item.capabilityIds) ? item.capabilityIds : [],
      hasExactStatusAssertion: Boolean(item.hasExactStatusAssertion),
      hasBodyAssertion: Boolean(item.hasBodyAssertion),
      hasMutationOrSeedAction: Boolean(item.hasMutationOrSeedAction),
      hasNegativeAssertion: Boolean(item.hasNegativeAssertion),
      hasStatusBodyCase: Boolean(item.hasExactStatusAssertion && item.hasBodyAssertion),
      hasStatefulStatusBodyCase: Boolean(item.hasExactStatusAssertion && item.hasBodyAssertion && item.hasMutationOrSeedAction),
      hasNegativeStatusCase: Boolean(item.hasExactStatusAssertion && item.hasNegativeAssertion),
      evidenceKind: item.evidenceKind || metadata.evidenceKind || 'business',
      businessClosureEvidence: Boolean(item.businessClosureEvidence)
    }));
  }
  return getTestBlocks(text)
    .map(block => {
      const hasExactStatusAssertion = /expect(?:Ok|Code|Success|BusinessError|ValidationError|Unauthorized|Forbidden|RecordNotFound|SqlRecordNotFound|PagedPayload|ArrayPayload|ObjectPayload)|expect\([^)]*(?:status|code)[^)]*\)[\s\S]{0,80}(?:equal|be|eql|toBe|toEqual)\(?\s*(?:200|201|400|401|403|201001|101001|100000)/.test(block.text);
      const hasBodyAssertion = /objectContaining|property\(|to\.have\.property|toEqual\(|toMatchObject|expect\([^)]*(?:data|list|total|id|token)|expect(?:PagedPayload|PagedObject|ArrayPayload|ObjectPayload|NullablePagedList|NullableArray|SqlRecordNotFound|ValidationFieldOneOf)/.test(block.text);
      const hasMutationOrSeedAction = /seedData|ensureDevice|ensureScene|ensureNotificationGroup|ensureOpenApiKey|api\.(?:post|put|delete|patch)|apiClient\.(?:post|put|delete|patch)|client\.(?:post|put|delete|patch)/.test(block.text);
      const hasNegativeAssertion = /401|403|400|201001|100000|100002|101001|unauthorized|forbidden|invalid|incomplete|missing|non-existent|tenant mismatch|no permission|permission denied|record not found|expect(?:Code|BusinessError|ValidationError|Unauthorized|Forbidden|RecordNotFound|SqlRecordNotFound|ValidationFieldOneOf)/i.test(block.text);

      return {
        title: block.title,
        hasExactStatusAssertion,
        hasBodyAssertion,
        hasMutationOrSeedAction,
        hasNegativeAssertion,
        hasStatusBodyCase: hasExactStatusAssertion && hasBodyAssertion,
        hasStatefulStatusBodyCase: hasExactStatusAssertion && hasBodyAssertion && hasMutationOrSeedAction,
        hasNegativeStatusCase: hasExactStatusAssertion && hasNegativeAssertion
      };
  });
}

function isBusinessClosureOracleCase(item) {
  if (item && Object.prototype.hasOwnProperty.call(item, 'businessClosureEvidence')) {
    return item.businessClosureEvidence === true;
  }
  const evidenceKind = item?.evidenceKind || 'business';
  return getAutomationEvidenceKindDetails(evidenceKind).businessClosureEvidence;
}

function getAutomationTestPath(testEntry) {
  return typeof testEntry === 'string' ? testEntry : testEntry.file;
}

function getAutomationEvidenceMetadata(testEntry) {
  if (testEntry && typeof testEntry === 'object' && testEntry.evidenceKind) {
    return {
      evidenceKind: VALID_AUTOMATION_EVIDENCE_KINDS.has(testEntry.evidenceKind)
        ? testEntry.evidenceKind
        : 'unknown',
      evidenceSource: 'explicit-entry'
    };
  }
  const metadata = testMetadata.getTestMetadata(testEntry);
  if (metadata && metadata.evidenceKind) {
    return {
      evidenceKind: VALID_AUTOMATION_EVIDENCE_KINDS.has(metadata.evidenceKind)
        ? metadata.evidenceKind
        : 'unknown',
      evidenceSource: 'test-metadata'
    };
  }
  const testPath = getAutomationTestPath(testEntry).replace(/\\/g, '/');
  const directiveEvidenceKind = parseTopOfFileAutomationEvidenceDirective(
    readText(path.join(PROJECT_ROOT, 'automation_tests', testPath))
  );
  if (directiveEvidenceKind) {
    return {
      evidenceKind: directiveEvidenceKind,
      evidenceSource: 'file-directive'
    };
  }
  const nonBusinessMatch = NON_BUSINESS_AUTOMATION_TEST_PATTERNS.find(item => item.pattern.test(testPath));
  if (nonBusinessMatch) {
    return {
      evidenceKind: nonBusinessMatch.evidenceKind,
      evidenceSource: 'path-pattern'
    };
  }
  return {
    evidenceKind: 'business',
    evidenceSource: 'default-business'
  };
}

function getAutomationEvidenceKind(testEntry) {
  return getAutomationEvidenceMetadata(testEntry).evidenceKind;
}

function getAutomationEvidenceKindDetails(evidenceKind) {
  return AUTOMATION_EVIDENCE_KIND_DETAILS[evidenceKind] || AUTOMATION_EVIDENCE_KIND_DETAILS.unknown;
}

function getCoverageTagMetadata(text, lines, index, testPath = null) {
  const normalizedText = String(text || '');
  const normalizedLines = Array.isArray(lines) ? lines : normalizedText.split(/\r?\n/);
  const metadata = testMetadata.getTestMetadata(testPath);
  if (metadata && metadata.fileFlags && metadata.fileFlags.pageCoverageOnly) {
    return {
      pageCoverageOnly: true,
      marker: 'metadata:fileFlags.pageCoverageOnly',
      source: 'test-metadata'
    };
  }
  if (normalizedText.includes('@file-page-coverage-only')) {
    return {
      pageCoverageOnly: true,
      marker: '@file-page-coverage-only',
      source: 'file-marker'
    };
  }
  if (typeof index === 'number' && hasNearbyCoverageMarker(normalizedLines, index, '@page-coverage-only')) {
    return {
      pageCoverageOnly: true,
      marker: '@page-coverage-only',
      source: 'block-marker'
    };
  }
  return {
    pageCoverageOnly: false,
    marker: null,
    source: 'none'
  };
}

function normalizeRunnerOutcome(outcome) {
  const normalized = typeof outcome === 'string' ? outcome.trim() : '';
  const recognized = VALID_RUNNER_OUTCOMES.has(normalized);
  return {
    outcome: recognized ? normalized : 'unknown',
    recognized,
    partialSkip: normalized === 'partial-skip',
    allSkipped: normalized === 'all-skipped',
    failed: normalized === 'failed',
    passed: normalized === 'passed'
  };
}

function getMappedTestFileStatus(capability, layer, testPath) {
  const rootByLayer = {
    automation: path.join(PROJECT_ROOT, 'automation_tests'),
    e2e: path.join(PROJECT_ROOT, 'automation_tests'),
    backend: BACKEND_ROOT,
    gmqtt: GMQTT_ROOT
  };
  const filePath = path.join(rootByLayer[layer], testPath);
  const text = readText(filePath);
  const hasTestFunction = layer === 'backend' || layer === 'gmqtt'
    ? /\bfunc\s+Test\w+\s*\(/.test(text)
    : /\b(?:it|test)\s*\(/.test(text);

  return {
    capability,
    layer,
    file: testPath,
    exists: fs.existsSync(filePath),
    hasTestFunction
  };
}

function getMappedTestFileAudit(traceability = getBusinessTraceability()) {
  return traceability.flatMap(capability => [
    ...capability.automationTests.map(testEntry =>
      getMappedTestFileStatus(capability.id, 'automation', getAutomationTestPath(testEntry))
    ),
    ...capability.e2eTests.map(testPath => getMappedTestFileStatus(capability.id, 'e2e', testPath)),
    ...capability.backendEvidence,
    ...capability.gmqttEvidence
  ]).filter(item => !item.exists || !item.hasTestFunction);
}

function getTestBlocks(text) {
  const blocks = [];
  const pattern = /\b(?:it|test)\(\s*(['"`])([\s\S]*?)\1\s*,/g;
  let match;

  while ((match = pattern.exec(text)) !== null) {
    const title = match[2];
    const start = match.index;
    const arrowIndex = text.indexOf('=>', pattern.lastIndex);
    const functionIndex = text.indexOf('function', pattern.lastIndex);
    const functionBodyStart = functionIndex === -1 ? -1 : text.indexOf('{', functionIndex);
    const arrowBodyStart = arrowIndex === -1 ? -1 : text.indexOf('{', arrowIndex);
    let bodyStart = -1;

    if (arrowBodyStart !== -1 && (functionBodyStart === -1 || arrowIndex < functionIndex)) {
      bodyStart = arrowBodyStart;
    } else if (functionBodyStart !== -1) {
      bodyStart = functionBodyStart;
    }
    if (bodyStart === -1) {
      continue;
    }
    let depth = 1;
    let index = bodyStart + 1;
    while (index < text.length && depth > 0) {
      const char = text[index];
      if (char === '{') depth++;
      if (char === '}') depth--;
      index++;
    }
    blocks.push({ title, text: text.slice(start, index), startLine: text.slice(0, start).split(/\r?\n/).length });
    pattern.lastIndex = index;
  }

  return blocks;
}

function getNamedFunctionBlocks(text) {
  const functions = new Map();
  const pattern = /(?:async\s+)?function\s+([A-Za-z_$][\w$]*)\s*\([^)]*\)\s*\{|(?:const|let)\s+([A-Za-z_$][\w$]*)\s*=\s*(?:async\s*)?\([^)]*\)\s*=>\s*\{/g;
  let match;

  while ((match = pattern.exec(text)) !== null) {
    const name = match[1] || match[2];
    const bodyStart = text.indexOf('{', match.index);
    let depth = 1;
    let index = bodyStart + 1;
    while (index < text.length && depth > 0) {
      const char = text[index];
      if (char === '{') depth++;
      if (char === '}') depth--;
      index++;
    }
    functions.set(name, text.slice(bodyStart, index));
    pattern.lastIndex = index;
  }

  return functions;
}

function getReachableE2ETestText(text, block) {
  const namedFunctions = getNamedFunctionBlocks(text);
  const expandedFunctions = new Set();
  let reachableText = block.text;

  for (let depth = 0; depth < 3; depth++) {
    let addition = '';
    for (const [name, functionText] of namedFunctions.entries()) {
      if (
        !expandedFunctions.has(name) &&
        new RegExp(`\\b${escapeRegex(name)}\\s*\\(`).test(reachableText)
      ) {
        expandedFunctions.add(name);
        addition += `\n${functionText}`;
      }
    }
    if (!addition) break;
    reachableText += addition;
  }

  return reachableText;
}

function getE2EBlockEvidence(text, block) {
  const reachableText = getReachableE2ETestText(text, block);
  return {
    hasStatePreparation: /\b(?:seed\w*|api\.(?:get|post|put|delete)|create\w*|ensure\w*|getAccount|storageState)\s*\(/.test(reachableText),
    hasUserInteraction: /\.(?:click|fill|selectOption|check|uncheck|press|setInputFiles|dragTo)\s*\(/.test(reachableText),
    hasObservableBusinessResult: /\.(?:toBeVisible|toBeHidden|toHaveText|toContainText|toHaveValue|toHaveAttribute|toHaveCount|toEqual|toMatchObject|toBe)\s*\(/.test(reachableText)
  };
}

function getE2EMetadataSourceAudit(text, testPath) {
  const metadata = testMetadata.getTestMetadata(testPath);
  if (!metadata || metadata.type !== 'e2e' || !Array.isArray(metadata.cases)) {
    return [];
  }

  const blocksByTitle = new Map(getTestBlocks(text).map(block => [block.title, block]));
  const findings = [];
  for (const item of metadata.cases.filter(isMetadataE2EBusinessClosure)) {
    const block = blocksByTitle.get(item.title);
    if (!block) {
      findings.push({
        file: testPath,
        title: item.title,
        gap: 'missing-test-block'
      });
      continue;
    }

    const evidence = getE2EBlockEvidence(text, block);
    for (const [field, gap] of [
      ['hasStatePreparation', 'missing-state-preparation'],
      ['hasUserInteraction', 'missing-user-interaction'],
      ['hasObservableBusinessResult', 'missing-observable-business-result']
    ]) {
      if (!evidence[field]) {
        findings.push({
          file: testPath,
          line: block.startLine,
          title: item.title,
          gap
        });
      }
    }
  }

  return findings;
}

function hasNearbyCoverageMarker(lines, index, marker) {
  const start = Math.max(0, index - 20);
  return lines.slice(start, index + 1).some(line => line.includes(marker));
}

function isPageCoverageOnlyContext(text, lines, index, testPath = null) {
  return getCoverageTagMetadata(text, lines, index, testPath).pageCoverageOnly;
}

function escapeRegex(text) {
  return text.replace(/[.*+?^${}()|[\]\\]/g, '\\$&');
}

function hasBareObjectDataFollowup(lines, index, baseExpression) {
  const windowText = lines.slice(index + 1, Math.min(lines.length, index + 8)).join('\n');
  const base = escapeRegex(baseExpression.trim());
  return new RegExp(base + "\\.data(?:\\.|\\[)").test(windowText) ||
    new RegExp(base + "\\.data\\)\\.to\\.(?:have\\.property|include\\.keys)").test(windowText) ||
    /\.(?:to\.)?(?:have\.property|include\.keys)\(/.test(windowText) ||
    new RegExp("(?:pickId|getEntityId)\\(\\s*" + base + "\\s*\\)").test(windowText) ||
    /pickId\([^)]*\.data\)|getEntityId\([^)]*\.data\)/.test(windowText);
}

function getWeakAutomationAssertionFindings(text, relPath = 'inline') {
  const weakExistenceAssertions = [];
  const weakFlexibleShapeAssertions = [];
  const weakObjectOnlyAssertions = [];
  const weakBareObjectAssertions = [];
  const weakConditionalEmptyAssertions = [];
  const weakNullableHelperAssertions = [];
  const lines = text.split(/\r?\n/);

  lines.forEach((line, index) => {
    if (/\.to\.exist\b|\.to\.be\.ok\b|assert\.ok\(/.test(line)) {
      weakExistenceAssertions.push({ file: relPath, line: index + 1, text: line.trim() });
    }
    if (/expect\([^)]*===\s*null[^)]*(?:typeof\s+[^)]*===\s*['"]object['"]|Array\.isArray\([^)]*\))[^)]*\)\.to\.equal\(true\)/.test(line)) {
      weakFlexibleShapeAssertions.push({ file: relPath, line: index + 1, text: line.trim() });
    }
    if (/\bexpectObjectPayload\(\s*[^,\n)]+\s*\)/.test(line)) {
      weakObjectOnlyAssertions.push({ file: relPath, line: index + 1, text: line.trim() });
    }
    const bareObjectMatch = line.match(/^\s*expect\(\s*([A-Za-z_$][\w$]*)\.data\s*\)\.to\.be\.an\('object'\)/);
    if (bareObjectMatch && !hasBareObjectDataFollowup(lines, index, bareObjectMatch[1])) {
      weakBareObjectAssertions.push({ file: relPath, line: index + 1, text: line.trim() });
    }
    if (/if\s*\([^)]*(?:\.length\s*>\s*0|\.length\))/.test(line)) {
      weakConditionalEmptyAssertions.push({ file: relPath, line: index + 1, text: line.trim() });
    }
    if (/\bexpect(?:NullablePagedList|NullableArray|NullableCountList)\(/.test(line)) {
      weakNullableHelperAssertions.push({ file: relPath, line: index + 1, text: line.trim() });
    }
  });

  return {
    weakExistenceAssertions,
    weakFlexibleShapeAssertions,
    weakObjectOnlyAssertions,
    weakBareObjectAssertions,
    weakConditionalEmptyAssertions,
    weakNullableHelperAssertions
  };
}

function metadataE2ECaseMatchesCapability(item, capabilityId) {
  if (!capabilityId) {
    return true;
  }
  if (!Array.isArray(item.capabilityIds) || item.capabilityIds.length === 0) {
    return true;
  }
  return item.capabilityIds.includes(capabilityId);
}

function isMetadataE2EBusinessClosure(item) {
  return (
    item.provesBusinessFlow &&
    item.businessClosureEvidence === true &&
    item.hasBrowserUserFlow !== false &&
    item.evidenceLayer === 'browser-e2e-with-api-setup'
  );
}

function getE2EBusinessCases(text, testPath = null, capabilityId = null) {
  const metadata = testMetadata.getTestMetadata(testPath);
  if (metadata && Array.isArray(metadata.cases)) {
    const blocksByTitle = new Map(getTestBlocks(text).map(block => [block.title, block]));
    const sourceGapTitles = new Set(
      getE2EMetadataSourceAudit(text, testPath).map(item => item.title)
    );
    return metadata.cases
      .filter(item =>
        isMetadataE2EBusinessClosure(item) &&
        item.evidenceLayer !== 'api-via-e2e-fixture' &&
        metadataE2ECaseMatchesCapability(item, capabilityId) &&
        !sourceGapTitles.has(item.title)
      )
      .map(item => {
        const evidence = getE2EBlockEvidence(text, blocksByTitle.get(item.title));
        return {
          title: item.title,
          businessClosureEvidence: item.businessClosureEvidence === true,
          hasUserAction: evidence.hasUserInteraction,
          hasBusinessAssertion: evidence.hasObservableBusinessResult,
          hasSeedOrApiSetup: evidence.hasStatePreparation,
          hasFallbackRisk: false,
          pageCoverageOnly: false,
          pageCoverageOnlySource: 'test-metadata+source-audit',
          evidenceLayer: item.evidenceLayer || 'browser-e2e',
          capabilityIds: Array.isArray(item.capabilityIds) ? item.capabilityIds : [],
          hasBrowserUserFlow: item.hasBrowserUserFlow !== false,
          firstDeviceOnboarding: item.firstDeviceOnboarding === true,
          readyCheckDiagnosticsBundle: item.readyCheckDiagnosticsBundle === true,
          otaSupportArchive: item.otaSupportArchive === true,
          requiresSeededDevice: item.requiresSeededDevice === true,
          requiresSeededOtaTask: item.requiresSeededOtaTask === true,
          runtimeEvidenceRequired: item.runtimeEvidenceRequired === true,
          provesBusinessFlow: true
        };
      });
  }
  const lines = text.split(/\r?\n/);

  return getTestBlocks(text)
    .map(block => {
      let coverageTag = getCoverageTagMetadata(text, lines, Math.max(0, block.startLine - 1), testPath);
      if (!coverageTag.pageCoverageOnly && block.text.includes('@page-coverage-only')) {
        coverageTag = {
          pageCoverageOnly: true,
          marker: '@page-coverage-only',
          source: 'block-body-marker'
        };
      }
      const pageCoverageOnly = coverageTag.pageCoverageOnly;
      const hasUserAction = /\.click\(|\.fill\(|\.selectOption\(|\.check\(|\.press\(/.test(block.text);
      const hasBusinessAssertion = /toHaveURL|toHaveText|toContainText|toHaveValue|toHaveAttribute|objectContaining|share-success|toast|table|Save|Create|Search|Reset|data-testid/.test(block.text);
      const hasSeedOrApiSetup = /seedData|api\.|ensureDevice|createTenantAdminAccount/.test(block.text);
      const hasFallbackRisk = /expectRouteSmoke|FORBIDDEN_OR_PAGE_TEXT|currently blocked|currently 403|current .*shell|current local fallback|Back to Home|返回首页|403|Forbidden|No Permission/i.test(block.text);
      const provesBusinessFlow = (hasUserAction || hasSeedOrApiSetup) && hasBusinessAssertion && !hasFallbackRisk && !pageCoverageOnly;
      return {
        title: block.title,
        hasUserAction,
        hasBusinessAssertion,
        hasSeedOrApiSetup,
        hasFallbackRisk,
        pageCoverageOnly,
        pageCoverageOnlySource: coverageTag.source,
        provesBusinessFlow
      };
    })
    .filter(block => block.provesBusinessFlow);
}

function getSkipAudit() {
  const automationRoot = path.join(PROJECT_ROOT, 'automation_tests');
  const auditRoots = [
    path.join(automationRoot, 'tests'),
    path.join(automationRoot, 'e2e'),
    path.join(automationRoot, 'lib'),
    path.join(automationRoot, 'run_tests.js')
  ];
  const files = auditRoots.flatMap(auditRoot => {
    if (fs.existsSync(auditRoot) && fs.statSync(auditRoot).isFile()) {
      return [auditRoot];
    }
    return walkFiles(auditRoot, filePath => /\.(js|cjs|mjs)$/.test(filePath));
  });
  const rawMochaSkips = [];
  const rawPlaywrightSkips = [];
  let explicitBlockedHelpers = 0;
  const structuredBlockedHelpers = [];

  for (const filePath of files) {
    const text = readText(filePath);
    const relPath = path.relative(automationRoot, filePath).replace(/\\/g, '/');
    if (relPath === 'lib/coverage_contract.js') {
      continue;
    }

    for (const match of text.matchAll(/this\.skip\(/g)) {
      rawMochaSkips.push({ file: relPath, index: match.index });
    }
    for (const match of text.matchAll(/\btest\.skip\(/g)) {
      rawPlaywrightSkips.push({ file: relPath, index: match.index });
    }
    if (relPath.startsWith('tests/') || relPath.startsWith('e2e/')) {
      explicitBlockedHelpers += (text.match(/\bskipIfBlocked\(/g) || []).length;
      explicitBlockedHelpers += (text.match(/\bskipWhenBlocked\(/g) || []).length;
      for (const helperName of ['skipIfBlocked', 'skipWhenBlocked']) {
        for (const match of text.matchAll(new RegExp("\\b" + helperName + "\\s*\\(", 'g'))) {
          const metadata = getStructuredSkipMetadataFromCall(text, helperName, match.index);
          if (metadata) {
            structuredBlockedHelpers.push({
              file: relPath,
              helper: helperName,
              ...metadata
            });
          }
        }
      }
    }
  }

  return {
    rawMochaSkips,
    rawPlaywrightSkips,
    explicitBlockedHelpers,
    structuredBlockedHelpers
  };
}

function getBusinessAssertionAuditFiles(automationRoot) {
  const auditRoots = [
    path.join(automationRoot, 'tests'),
    path.join(automationRoot, 'e2e')
  ];
  return auditRoots.flatMap(auditRoot => walkFiles(
    auditRoot,
    filePath =>
      /\.(js|cjs|mjs)$/.test(filePath) &&
      path.basename(filePath) !== '00_coverage_contract.test.js'
  ));
}

function getBusinessE2EFileSet() {
  return new Set(
    BUSINESS_CAPABILITIES
      .filter(capability => capability.priority === 'P0' || capability.priority === 'P1')
      .flatMap(capability => capability.e2eTests)
  );
}

function createBusinessAssertionAudit() {
  return {
    broadNon200Assertions: [],
    businessE2EFallbackAssertions: [],
    e2eCurrentStateAssertions: [],
    e2eMetadataSourceGaps: [],
    e2eRouteSmokeAssertions: [],
    genericBlockedReasons: [],
    prohibitedCoverageMarkerTitles: [],
    seedBlockedReturns: [],
    weakBareObjectAssertions: [],
    weakConditionalEmptyAssertions: [],
    weakFlexibleShapeAssertions: [],
    weakExistenceAssertions: [],
    weakNullableHelperAssertions: [],
    weakObjectOnlyAssertions: [],
    weakBodyAssertions: []
  };
}

function appendWeakAutomationAssertionFindings(audit, weakAutomationAssertions) {
  audit.weakExistenceAssertions.push(...weakAutomationAssertions.weakExistenceAssertions);
  audit.weakFlexibleShapeAssertions.push(...weakAutomationAssertions.weakFlexibleShapeAssertions);
  audit.weakObjectOnlyAssertions.push(...weakAutomationAssertions.weakObjectOnlyAssertions);
  audit.weakBareObjectAssertions.push(...weakAutomationAssertions.weakBareObjectAssertions);
  audit.weakConditionalEmptyAssertions.push(...weakAutomationAssertions.weakConditionalEmptyAssertions);
  audit.weakNullableHelperAssertions.push(...weakAutomationAssertions.weakNullableHelperAssertions);
}

function appendBusinessAssertionLineFindings({
  audit,
  businessE2EFiles,
  line,
  index,
  lines,
  relPath,
  text
}) {
  const pageCoverageOnly = isPageCoverageOnlyContext(text, lines, index);
  const fileEvidenceKind = testMetadata.getTestMetadata(relPath)?.evidenceKind || null;
  const explicitNonBusinessFile = fileEvidenceKind && fileEvidenceKind !== 'business';
  const finding = { file: relPath, line: index + 1, text: line.trim() };

  if (line.includes('prohibited coverage assertion') || line.includes('prohibited coverage group')) {
    audit.prohibitedCoverageMarkerTitles.push(finding);
  }
  if (line.includes('integration prerequisite unavailable; see guarded setup or fixture condition in this test')) {
    audit.genericBlockedReasons.push(finding);
  }
  if (line.includes("locator('body')") && /toBeVisible|toContainText/.test(line)) {
    audit.weakBodyAssertions.push(finding);
  }
  if (/if\s*\(!expectBlockedOrSeeded\(/.test(line)) {
    audit.seedBlockedReturns.push(finding);
  }
  if (/to\.not\.equal\(\s*200\s*\)/.test(line)) {
    audit.broadNon200Assertions.push(finding);
  }
  if (
    !pageCoverageOnly &&
    !explicitNonBusinessFile &&
    businessE2EFiles.has(relPath) &&
    ((/toBeVisible/.test(line) && /(403|Forbidden|Back to Home|返回首页|No Permission)/i.test(line)) ||
      /\.or\([^)]*errorMessage/.test(line))
  ) {
    audit.businessE2EFallbackAssertions.push(finding);
  }
  if (!pageCoverageOnly && relPath.startsWith('e2e/') && /expectRouteSmoke|route smoke|FORBIDDEN_OR_PAGE_TEXT|show the current local fallback|renders or show the current|current permission boundary/i.test(line)) {
    audit.e2eRouteSmokeAssertions.push(finding);
  }
  if (!pageCoverageOnly && relPath.startsWith('e2e/') && /currently blocked|currently 403|current .*shell|currently renders|current local fallback|without a selected device id/i.test(line)) {
    audit.e2eCurrentStateAssertions.push(finding);
  }
}

function appendBusinessAssertionFileFindings({
  audit,
  automationRoot,
  businessE2EFiles,
  filePath
}) {
  const text = readText(filePath);
  const relPath = path.relative(automationRoot, filePath).replace(/\\/g, '/');
  const lines = text.split(/\r?\n/);
  appendWeakAutomationAssertionFindings(audit, getWeakAutomationAssertionFindings(text, relPath));
  if (relPath.startsWith('e2e/')) {
    audit.e2eMetadataSourceGaps.push(...getE2EMetadataSourceAudit(text, relPath));
  }

  lines.forEach((line, index) => appendBusinessAssertionLineFindings({
    audit,
    businessE2EFiles,
    line,
    index,
    lines,
    relPath,
    text
  }));
}

function getBusinessAssertionAudit() {
  const automationRoot = path.join(PROJECT_ROOT, 'automation_tests');
  const audit = createBusinessAssertionAudit();
  const businessE2EFiles = getBusinessE2EFileSet();

  for (const filePath of getBusinessAssertionAuditFiles(automationRoot)) {
    appendBusinessAssertionFileFindings({
      audit,
      automationRoot,
      businessE2EFiles,
      filePath
    });
  }

  return audit;
}
function getGoSourceStringContractAudit() {
  const roots = [
    path.join(BACKEND_ROOT, 'internal', 'api'),
    path.join(BACKEND_ROOT, 'router'),
    path.join(BACKEND_ROOT, 'mqtt'),
    path.join(BACKEND_ROOT, 'internal', 'processor'),
    path.join(BACKEND_ROOT, 'third_party', 'others', 'http_client'),
    path.join(GMQTT_ROOT, 'cmd', 'gmqttd'),
    GMQTT_AETHERLINK_PLUGIN_ROOT,
    path.join(GMQTT_ROOT, 'plugin', 'prometheus'),
    path.join(GMQTT_ROOT, 'pkg', 'codes')
  ];
  const files = roots.flatMap(root => walkFiles(root, filePath => /_test\.go$/.test(filePath)));

  return files.flatMap(filePath => {
    const text = readText(filePath);
    const hasSourceRead = /\bos\.ReadFile\(/.test(text);
    const hasSourceContains = /\bstrings\.Contains\(/.test(text) && /source missing|contract missing|missing %q|source/.test(text);
    if (!hasSourceRead || !hasSourceContains) {
      return [];
    }

    const relRoot = filePath.startsWith(BACKEND_ROOT) ? BACKEND_ROOT : GMQTT_ROOT;
    return [{
      file: path.relative(relRoot, filePath).replace(/\\/g, '/'),
      reason: 'source-string contract test reads .go files instead of executing business behavior'
    }];
  });
}

function getFrontendWeakAssertionAudit() {
  const targetRoots = [path.join(FRONTEND_ROOT, 'src')];
  const files = targetRoots.flatMap(root => walkFiles(root, filePath => /\.(test|spec)\.(ts|tsx)$/.test(filePath)));
  const weakAssertions = [];

  for (const filePath of files) {
    const text = readText(filePath);
    const relPath = path.relative(FRONTEND_ROOT, filePath).replace(/\\/g, '/');
    const lines = text.split(/\r?\n/);

    lines.forEach((line, index) => {
      if (/toBeTruthy\(\)|toMatchSnapshot\(|toBeGreaterThanOrEqual\(0\)|expect\([^)]*\.exists\(\)[^)]*\)\.toBe\(true\)|expect\([^)]*\.exists\(\)[^)]*\)\.toBeTruthy\(\)/.test(line)) {
        weakAssertions.push({ file: relPath, line: index + 1, text: line.trim() });
      }
      if (/readFile\(|fs\/promises|source-level|source contract/.test(line)) {
        weakAssertions.push({
          file: relPath,
          line: index + 1,
          text: line.trim(),
          reason: 'frontend source-string contract does not execute UI or guard behavior'
        });
      }
    });
  }

  return weakAssertions;
}

function getFrontendSourceContractAudit() {
  const targetRoots = [path.join(FRONTEND_ROOT, 'src')];
  const files = targetRoots.flatMap(root => walkFiles(root, filePath => /\.(test|spec)\.(ts|tsx)$/.test(filePath)));

  return files.flatMap(filePath => {
    const lines = readText(filePath).split(/\r?\n/);
    const sourceContractLine = lines.findIndex(line =>
      /\b(?:readFile|readFileSync)\s*\(/.test(line) ||
      /fs\/promises|source-level|source contract/.test(line)
    );
    if (sourceContractLine < 0) {
      return [];
    }

    const relPath = path.relative(FRONTEND_ROOT, filePath).replace(/\\/g, '/');
    return [{
      file: relPath,
      line: sourceContractLine + 1,
      text: lines[sourceContractLine].trim(),
      category: 'source-contract',
      reason: 'frontend source-string contract does not execute UI or guard behavior'
    }];
  });
}

function classifyBlockedReason(reason) {
  if (reason && typeof reason === 'object') {
    const normalizedReason = typeof reason.reason === 'string' ? reason.reason.trim() : '';
    const normalizedCategory = normalizeBlockedMetadataCategory(reason.category);
    const normalizedSeedable = normalizeBooleanMetadataValue(
      reason.seedable,
      normalizedCategory === 'seedable-local'
    );

    if (normalizedCategory) {
      return {
        reason: normalizedReason,
        category: normalizedCategory,
        seedable: normalizedSeedable
      };
    }
  }

  const normalized = typeof reason === 'string' ? reason.trim() : '';
  if (!normalized) {
    return {
      reason: normalized,
      category: 'unknown',
      seedable: false
    };
  }
  if (normalized.startsWith('requires runtime fixture or external dependency')) {
    return {
      reason: normalized,
      category: 'runtime-external',
      seedable: false
    };
  }
  return {
    reason: normalized,
    category: 'seedable-local',
    seedable: true
  };
}

function getBlockedReasonMetadata(reason, annotation) {
  const classified = classifyBlockedReason(annotation ? { reason, ...annotation } : reason);
  if (!annotation || !annotation.category) {
    return {
      ...classified,
      classificationSource: 'heuristic'
    };
  }
  return {
    reason: classified.reason,
    category: annotation.category,
    seedable: annotation.seedable === undefined ? classified.seedable : annotation.seedable,
    classificationSource: 'annotation'
  };
}

function getNearbyBlockedReasonAnnotation(lines, index) {
  const start = Math.max(0, index - 3);
  for (let cursor = index; cursor >= start; cursor -= 1) {
    const match = String(lines[cursor] || '').match(/@blocked-reason\s+([a-z-]+)(?:\s+seedable=(true|false))?/i);
    if (match) {
      return {
        category: match[1],
        seedable: match[2] === undefined ? undefined : match[2] === 'true'
      };
    }
  }
  return null;
}

function getBlockedReasonAudit() {
  const automationRoot = path.join(PROJECT_ROOT, 'automation_tests');
  const files = [
    ...walkFiles(path.join(automationRoot, 'tests'), filePath => /\.(js|cjs|mjs)$/.test(filePath)),
    ...walkFiles(path.join(automationRoot, 'e2e'), filePath => /\.(js|cjs|mjs)$/.test(filePath))
  ];
  const reasons = [];

  for (const filePath of files) {
    const text = readText(filePath);
    const relPath = path.relative(automationRoot, filePath).replace(/\\/g, '/');
    const lines = text.split(/\r?\n/);

    lines.forEach((line, index) => {
      const helperMatch = line.match(/\b(skipIfBlocked|skipWhenBlocked)\(/);
      if (!helperMatch) {
        return;
      }

      const metadata = getStructuredSkipMetadataFromCall(text, helperMatch[1], text.indexOf(helperMatch[0], text.indexOf(line)));
      if (metadata && metadata.reason) {
        reasons.push({
          file: relPath,
          line: index + 1,
          reason: metadata.reason,
          category: metadata.category || undefined,
          seedable: typeof metadata.seedable === 'boolean' ? metadata.seedable : undefined,
          outcome: metadata.outcome || undefined,
          annotation: getNearbyBlockedReasonAnnotation(lines, index)
        });
        return;
      }

      const match = line.match(/\bskip(?:If|When)Blocked\([^,]+,\s*(?:[^,]+,\s*)?'([^']+)'/);
      if (match) {
        reasons.push({
          file: relPath,
          line: index + 1,
          reason: match[1],
          annotation: getNearbyBlockedReasonAnnotation(lines, index)
        });
      }
    });
  }

  const summary = reasons.reduce((acc, item) => {
    acc[item.reason] = (acc[item.reason] || 0) + 1;
    return acc;
  }, {});
  const classifiedReasons = reasons.map(item => ({
    ...item,
    ...getBlockedReasonMetadata(item.reason, item.annotation || {
      category: item.category,
      seedable: item.seedable
    })
  }));
  const runtimeReasons = classifiedReasons.filter(item => item.category === 'runtime-external');
  const seedableReasons = classifiedReasons.filter(item => item.seedable);

  return {
    reasons,
    classifiedReasons,
    runtimeReasons,
    seedableReasons,
    summary
  };
}

function getSourceReviewLineItems(docs) {
  return docs.flatMap(doc => String(doc.text || '').split(/\r?\n/).map((text, index) => ({
    file: doc.file,
    line: index + 1,
    text
  })));
}

function hasNonBusinessClosureNegation(text) {
  return /(not|without|avoid|prevent|must not|does not|only|non-business)/i.test(text);
}

function getUnsafeSourceReviewClosureClaimsFromDocs(docs) {
  const lines = getSourceReviewLineItems(docs);
  return lines.filter(item => {
    const text = item.text.toLowerCase();
    if (/evidence into business closure/i.test(item.text)) {
      const previousLine = lines.find(candidate => candidate.file === item.file && candidate.line === item.line - 1);
      if (previousLine && /not upgrade/i.test(previousLine.text)) {
        return false;
      }
    }
    const nonBusinessEvidence =
      '(?:\\bboundary\\b|\\bsmoke\\b|\\bcatalog\\b|source[- ]?(?:inventory|evidence|string)|\\bast\\b|request[- ]?wrapper|api[- ]?wrapper)';
    const mentionsNonBusinessEvidenceAndClosure = new RegExp(
      `${nonBusinessEvidence}.*(?:business|closure)|(?:business|closure).*${nonBusinessEvidence}`
    ).test(text);

    if (!mentionsNonBusinessEvidenceAndClosure) {
      return false;
    }

    return !hasNonBusinessClosureNegation(item.text);
  });
}

function getSourceReviewBoundaryAudit() {
  const docs = [
    {
      file: 'references/source-quality-review.md',
      text: readText(path.join(PROJECT_ROOT, 'references', 'source-quality-review.md'))
    }
  ];
  const combined = docs.map(doc => doc.text).join('\n');
  const lines = getSourceReviewLineItems(docs);
  const unsafeClosureClaims = lines.filter(item => {
    const text = item.text.toLowerCase();
    if (/evidence into business closure/i.test(item.text)) {
      const previousLine = lines.find(candidate => candidate.file === item.file && candidate.line === item.line - 1);
      if (previousLine && /not upgrade/i.test(previousLine.text)) {
        return false;
      }
    }
    const nonBusinessEvidence =
      '(?:\\bboundary\\b|\\bsmoke\\b|\\bcatalog\\b|source[- ]?(?:inventory|evidence|string)|\\bast\\b|request[- ]?wrapper|api[- ]?wrapper)';
    const mentionsNonBusinessEvidenceAndClosure = new RegExp(
      `${nonBusinessEvidence}.*(?:business|closure)|(?:business|closure).*${nonBusinessEvidence}`
    ).test(text);

    if (!mentionsNonBusinessEvidenceAndClosure) {
      return false;
    }

    return !/(not|without|avoid|prevent|must not|does not|only|non-business|不是|不能|不得)/i.test(item.text);
  });

  return {
    docs: docs.map(doc => doc.file),
    priorityScopeDifferenceExplained: /1808 priority scope is not a refreshed full table/i.test(combined) &&
      /1516-file\s+batch plus \+12 same-scope additions, -5 same-scope removals, and \+285\s+expanded-scope files/i.test(combined) &&
      /expanded-scope files from `mqtt-broker`, `automation_tests`, and\s+`references`/i.test(combined),
    sourceInventoryDeclaresStaticBoundary: /evidence does not upgrade boundary, catalog, or page smoke checks into\s+business closure/i.test(combined),
    qualityReviewRejectsRequestWrapperClosure: /Not business closure by itself/i.test(combined) &&
      /frontend API wrapper tests that only assert endpoint\/method wiring/i.test(combined) &&
      /request wrapper\/interceptor tests that only prove request construction,\s+token\/header plumbing, or error-normalization branches/m.test(combined),
    qualityReviewRejectsSourceInventoryClosure: /source inventory\/quality-review wording was found that promotes\s+frontend request-wrapper checks, boundary API smoke, page smoke, catalog\s+checks, source inventory, or source-string\/AST checks into standalone\s+business closure/i.test(combined) &&
      /source inventory rows and source-quality notes/i.test(combined),
    qualityReviewRejectsSmokeClosure: /Not business closure by itself/i.test(combined) &&
      /route\/page smoke specs marked `@page-coverage-only` or\s+`@file-page-coverage-only`/m.test(combined),
    qualityReviewKeepsReleaseGateOpen: /Release gate still open/i.test(combined) &&
      /Fresh release evidence still requires a local backend\/database/i.test(combined),
    unsafeClosureClaims
  };
}

const {
  getMissingTraceability,
  hasCompleteExplicitBusinessInventory,
  hasNoBusinessAssertionGaps,
  hasNoCatalogInventoryOrMappingGaps,
  hasNoSourceReviewBoundaryGaps
} = readiness;

function collectSelfCheckAudits() {
  const traceability = getBusinessTraceability();
  const catalogClassificationAudit = getCatalogClassificationAudit();
  const explicitBusinessInventoryAudit = getExplicitBusinessInventoryAudit(catalogClassificationAudit);

  return {
    routeComparison: comparePageCatalogToSource(),
    endpointComparison: compareEndpointCatalogToSource(),
    missingCapabilityEndpoints: compareEndpointCapabilityMap(),
    traceability,
    catalogClassificationAudit,
    explicitBusinessInventoryAudit,
    explicitBusinessInventoryGapReport: getExplicitBusinessInventoryGapReport(explicitBusinessInventoryAudit),
    mappedTestFileAudit: getMappedTestFileAudit(traceability),
    skipAudit: getSkipAudit(),
    businessAssertionAudit: getBusinessAssertionAudit(),
    goSourceStringContractAudit: getGoSourceStringContractAudit(),
    frontendWeakAssertionAudit: getFrontendWeakAssertionAudit(),
    frontendSourceContractAudit: getFrontendSourceContractAudit(),
    sourceReviewBoundaryAudit: getSourceReviewBoundaryAudit(),
    blockedReasonAudit: getBlockedReasonAudit()
  };
}

function getSelfCheckReadiness(audits) {
  return readiness.getSelfCheckReadiness(audits);
}

function selfCheck() {
  const audits = collectSelfCheckAudits();
  return {
    ...audits,
    ...getSelfCheckReadiness(audits)
  };
}

module.exports = {
  BACKEND_ROOT,
  BUSINESS_CAPABILITIES,
  FRONTEND_ELEGANT_ROUTE_ROOT,
  FRONTEND_ROOT,
  GMQTT_ROOT,
  compareEndpointCapabilityMap,
  compareEndpointCatalogToSource,
  comparePageCatalogToSource,
  classifyEndpointCatalogItem,
  classifyPageCatalogRoute,
  getCatalogClassificationAudit,
  getExplicitBusinessInventoryAudit,
  getExplicitBusinessInventoryGapReport,
  getEndpointReferenceNeedles,
  getRouteReferenceNeedles,
  findDirectReferences,
  summarizeExplicitInventoryGaps,
  getAutomationOracleCases,
  getAutomationEvidenceMetadata,
  getAutomationEvidenceKind,
  getAutomationEvidenceKindDetails,
  getBusinessAssertionAudit,
  getE2EMetadataSourceAudit,
  getE2EBusinessCases,
  getCoverageTagMetadata,
  getWeakAutomationAssertionFindings,
  getMappedTestFileAudit,
  getMappedTestFileStatus,
  getFrontendWeakAssertionAudit,
  getFrontendSourceContractAudit,
  classifyBlockedReason,
  getBlockedReasonMetadata,
  getBlockedReasonAudit,
  normalizeRunnerOutcome,
  getSourceReviewBoundaryAudit,
  getUnsafeSourceReviewClosureClaimsFromDocs,
  getBusinessTraceability,
  getGoSourceStringContractAudit,
  getEndpointCatalogKeys,
  getBackendSourceEndpointKeys,
  getPageCatalogRoutes,
  getSkipAudit,
  getSourceLeafRoutes,
  selfCheck
};
