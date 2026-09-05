import { createServer } from 'node:http'
import { URL } from 'node:url'
import { readFileSync } from 'node:fs'

const host = '127.0.0.1'
const port = Number(process.env.LOCAL_DEMO_PORT || 4174)
const now = '2026-08-29T00:00:00.000Z'

function loadProductionSnapshot() {
  try {
    return JSON.parse(readFileSync(new URL('./production-snapshot.json', import.meta.url), 'utf8'))
  } catch {
    return null
  }
}

const productionSnapshot = loadProductionSnapshot()

const demoUser = {
  id: 1,
  username: 'local-admin',
  email: 'local-admin@example.invalid',
  role: 'admin',
  balance: 100,
  concurrency: 20,
  status: 'active',
  allowed_groups: null,
  balance_notify_enabled: false,
  balance_notify_threshold: null,
  balance_notify_extra_emails: [],
  created_at: now,
  updated_at: now,
}

const baseGroup = (id, name, platform, sortOrder) => ({
  id,
  name,
  description: 'Local demo data only',
  platform,
  rate_multiplier: 1,
  rpm_limit: 0,
  max_reasoning_effort: '',
  reasoning_effort_mappings: [],
  is_exclusive: false,
  status: 'active',
  subscription_type: 'standard',
  daily_limit_usd: null,
  weekly_limit_usd: null,
  monthly_limit_usd: null,
  long_context_pricing_enabled: true,
  allow_image_generation: false,
  allow_batch_image_generation: false,
  image_rate_independent: false,
  image_rate_multiplier: 1,
  batch_image_discount_multiplier: 0.5,
  batch_image_hold_multiplier: 0.6,
  image_price_1k: null,
  image_price_2k: null,
  image_price_4k: null,
  video_rate_independent: false,
  video_rate_multiplier: 1,
  video_price_480p: null,
  video_price_720p: null,
  video_price_1080p: null,
  video_model_prices: {},
  web_search_price_per_call: null,
  search_price_per_1k: null,
  audio_realtime_price_per_min: null,
  audio_tts_price_per_million_chars: null,
  audio_stt_price_per_hour: null,
  peak_rate_enabled: false,
  peak_start: '00:00',
  peak_end: '00:00',
  peak_rate_multiplier: 1,
  claude_code_only: false,
  fallback_group_id: null,
  fallback_group_id_on_invalid_request: null,
  allow_messages_dispatch: true,
  allow_live: false,
  default_mapped_model: '',
  messages_dispatch_model_config: {},
  require_oauth_only: false,
  require_privacy_set: false,
  model_pricing: [],
  model_routing: null,
  model_routing_enabled: false,
  mcp_xml_inject: false,
  supported_model_scopes: [],
  models_list_config: { enabled: false, models: [] },
  profit_control_enabled: false,
  profit_min_margin: 0,
  profit_safety_buffer: 0,
  account_count: 0,
  active_account_count: 0,
  rate_limited_account_count: 0,
  sort_order: sortOrder,
  created_at: now,
  updated_at: now,
})

const demoGroups = [
  baseGroup(1, 'OpenAI Primary', 'openai', 10),
  baseGroup(2, 'OpenAI Fallback', 'openai', 20),
  baseGroup(3, 'Anthropic Shared', 'anthropic', 30),
]

const groups = Array.isArray(productionSnapshot?.groups) && productionSnapshot.groups.length
  ? productionSnapshot.groups.map((group, index) => ({
    ...baseGroup(Number(group.id) || index + 1, group.name || `Imported group ${index + 1}`, group.platform || 'openai', Number(group.sort_order) || (index + 1) * 10),
    ...group,
    description: group.description || 'Imported production snapshot',
  }))
  : demoGroups

const snapshotProfiles = Array.isArray(productionSnapshot?.profiles)
  ? productionSnapshot.profiles
  : []

// The production snapshot may not include the separate profile settings export.
// Build a local-only fallback from the imported account URL and name prefix so
// the batch editor can demonstrate the real grouping relationship without
// contacting production or inventing account credentials.
const inferredProfiles = Array.from(
  new Map(
    (productionSnapshot?.accounts || [])
      .filter((account) => account?.base_url)
      .map((account) => {
        const name = String(account.name || '')
        const prefix = name.includes('-') ? `${name.split('-')[0]}-` : ''
        return [String(account.base_url), { base_url: String(account.base_url), name_prefix: prefix, platform: account.platform || 'openai' }]
      })
  ).entries()
).map(([baseURL, profile], index) => ({
  id: index + 1,
  name: profile.name_prefix ? `${profile.name_prefix}上游` : `上游中转站 ${index + 1}`,
  name_prefix: profile.name_prefix,
  base_url: baseURL,
  platform: profile.platform,
  enabled: true,
}))

const upstreamProfiles = snapshotProfiles.length ? snapshotProfiles : inferredProfiles
const profileByBaseURL = new Map(upstreamProfiles.map((profile) => [String(profile.base_url), Number(profile.id)]))

const channelMonitors = Array.isArray(productionSnapshot?.channel_monitors)
  ? productionSnapshot.channel_monitors.map((monitor) => ({
    api_key_masked: '[已隐藏]',
    primary_status: '',
    primary_latency_ms: null,
    availability_7d: 0,
    extra_models_status: [],
    extra_headers: {},
    body_override: null,
    ...monitor,
  }))
  : []

const monitorGroupNames = [...new Set(channelMonitors.map((monitor) => monitor.monitor_group_name || monitor.group_name || `${monitor.provider || 'other'} monitors`).filter(Boolean))]
const channelMonitorGroups = Array.isArray(productionSnapshot?.channel_monitor_groups) && productionSnapshot.channel_monitor_groups.length
  ? productionSnapshot.channel_monitor_groups.map((group, index) => ({
    id: Number(group.id) || index + 1,
    name: group.name || `Monitor group ${index + 1}`,
    sort_order: Number(group.sort_order) || index * 10,
    created_by: demoUser.id,
    created_at: now,
    updated_at: now,
    monitor_count: 0,
  }))
  : monitorGroupNames.map((name, index) => ({
    id: index + 1,
    name,
    sort_order: index * 10,
    created_by: demoUser.id,
    created_at: now,
    updated_at: now,
    monitor_count: 0,
  }))

// The original group_name is a user-facing aggregation label. The new
// monitor_group_id is deliberately independent and only controls the admin
// monitor organization shown in the local preview.
for (const monitor of channelMonitors) {
  const groupName = monitor.monitor_group_name || monitor.group_name || `${monitor.provider || 'other'} monitors`
  const matchingGroup = channelMonitorGroups.find((group) => group.name === groupName) || channelMonitorGroups[0]
  monitor.monitor_group_id = monitor.monitor_group_id == null ? (matchingGroup?.id ?? null) : Number(monitor.monitor_group_id)
  monitor.monitor_sort_order = Number.isFinite(Number(monitor.monitor_sort_order)) ? Number(monitor.monitor_sort_order) : Number(monitor.id || 0) * 10
}

function refreshMonitorGroupCounts() {
  for (const group of channelMonitorGroups) group.monitor_count = channelMonitors.filter((monitor) => monitor.monitor_group_id === group.id).length
}

function monitorResponse(monitor) {
  const group = channelMonitorGroups.find((item) => item.id === monitor.monitor_group_id)
  return { ...monitor, monitor_group_name: group?.name || null }
}

refreshMonitorGroupCounts()

const monitorV2Config = {
  version: 1,
  enabled: true,
  refresh_interval_seconds: 60,
  platforms: [...new Set(channelMonitors.map((monitor) => monitor.provider).filter(Boolean))].map((platform) => ({
    platform,
    enabled: true,
    models: [...new Set(channelMonitors.filter((monitor) => monitor.provider === platform).flatMap((monitor) => [monitor.primary_model, ...(monitor.extra_models || [])]).filter(Boolean))],
  })),
  group_ids: [],
  health_thresholds: {
    minimum_sample: 50,
    warning_error_rate: 0.05,
    critical_error_rate: 0.2,
    target_ttft_ms: 3000,
    warning_ttft_ms: 3000,
    critical_ttft_ms: 10000,
    warning_cache_rate: 0.85,
    critical_cache_rate: 0.6,
    error_weight: 0.6,
    ttft_weight: 0.2,
    cache_weight: 0.2,
  },
  ignored_error_categories: ['authentication', 'client_cancelled', 'content_policy', 'context_limit', 'group_access', 'model_unsupported', 'not_found', 'quota_or_balance'],
}

const demoAccounts = [
  {
    id: 1,
    name: 'OpenAI account A',
    platform: 'openai',
    type: 'oauth',
    priority: 1,
    status: 'active',
    group_ids: [1],
    base_url: 'https://api.openai.com/v1',
  },
  {
    id: 2,
    name: 'OpenAI account B (multi-group)',
    platform: 'openai',
    type: 'apikey',
    priority: 10,
    status: 'active',
    group_ids: [1, 2],
    upstream_provider_profile_id: 1,
    base_url: 'https://coco.example.invalid/v1',
  },
  {
    id: 3,
    name: 'OpenAI account C',
    platform: 'openai',
    type: 'oauth',
    priority: 2,
    status: 'inactive',
    group_ids: [2],
  },
  {
    id: 4,
    name: 'Anthropic account A',
    platform: 'anthropic',
    type: 'oauth',
    priority: 5,
    status: 'active',
    group_ids: [3],
  },
]

const accounts = Array.isArray(productionSnapshot?.accounts) && productionSnapshot.accounts.length
  ? productionSnapshot.accounts.map((account, index) => ({
    ...account,
    id: Number(account.id) || index + 1,
    name: account.name || `Imported account ${index + 1}`,
    platform: account.platform || 'openai',
    type: account.type || 'apikey',
    priority: Math.max(1, Math.round(Number(account.priority) || 1)),
    group_ids: Array.isArray(account.group_ids) ? account.group_ids.map(Number).filter(Boolean) : [],
    upstream_provider_profile_id: Number(account.upstream_provider_profile_id || account.extra?.upstream_provider_profile_id) || profileByBaseURL.get(String(account.base_url)) || undefined,
  }))
  : demoAccounts

function accountResponse(account) {
  return {
    ...account,
    notes: 'Local demo account',
    credentials: account.base_url ? { base_url: account.base_url } : {},
    credentials_status: {},
    proxy_id: null,
    concurrency: 10,
    load_factor: null,
    rate_multiplier: 1,
    error_message: null,
    last_used_at: null,
    expires_at: null,
    auto_pause_on_expired: false,
    created_at: now,
    updated_at: now,
    extra: account.upstream_provider_profile_id ? { upstream_provider_profile_id: account.upstream_provider_profile_id } : {},
    schedulable: account.status === 'active',
    rate_limited_at: null,
    rate_limit_reset_at: null,
    overload_until: null,
    temp_unschedulable_until: null,
    groups: account.group_ids.map((groupID) => groups.find((group) => group.id === groupID)).filter(Boolean),
  }
}

function refreshCounts() {
  for (const group of groups) {
    const members = accounts.filter((account) => account.group_ids.includes(group.id))
    group.account_count = members.length
    group.active_account_count = members.filter((account) => account.status === 'active').length
    group.rate_limited_account_count = 0
  }
}

function json(res, status, body) {
  res.writeHead(status, {
    'Content-Type': 'application/json; charset=utf-8',
    'Access-Control-Allow-Origin': '*',
    'Access-Control-Allow-Headers': 'Content-Type, Authorization, Accept-Language, X-Admin-UI-Request',
    'Access-Control-Allow-Methods': 'GET, POST, PUT, DELETE, OPTIONS',
  })
  res.end(JSON.stringify(body))
}

function api(res, data, status = 200) {
  json(res, status, { code: 0, message: 'ok', data })
}

function sse(res, events) {
  res.writeHead(200, {
    'Content-Type': 'text/event-stream; charset=utf-8',
    'Cache-Control': 'no-cache',
    Connection: 'keep-alive',
    'Access-Control-Allow-Origin': '*',
    'Access-Control-Allow-Headers': 'Content-Type, Authorization, Accept-Language, X-Admin-UI-Request',
    'Access-Control-Allow-Methods': 'GET, POST, PUT, DELETE, OPTIONS',
  })
  for (const event of events) res.write(`data: ${JSON.stringify(event)}\n\n`)
  res.end()
}

async function readBody(req) {
  const chunks = []
  for await (const chunk of req) chunks.push(chunk)
  if (!chunks.length) return {}
  try {
    return JSON.parse(Buffer.concat(chunks).toString('utf8'))
  } catch {
    return {}
  }
}

function paginated(items, url) {
  const page = Number(url.searchParams.get('page') || 1)
  const pageSize = Number(url.searchParams.get('page_size') || 20)
  const start = (page - 1) * pageSize
  return { items: items.slice(start, start + pageSize), total: items.length, page, page_size: pageSize, pages: Math.ceil(items.length / pageSize) || 1 }
}

function filterAccounts(url) {
  const groupID = Number(url.searchParams.get('group') || 0)
  const platform = url.searchParams.get('platform')
  const search = (url.searchParams.get('search') || '').toLowerCase()
  let result = accounts
  if (groupID) result = result.filter((account) => account.group_ids.includes(groupID))
  if (platform) result = result.filter((account) => account.platform === platform)
  if (search) result = result.filter((account) => account.name.toLowerCase().includes(search))
  result = result.slice().sort((a, b) => a.priority - b.priority || a.id - b.id)
  return result.map(accountResponse)
}

function getGroup(id) {
  refreshCounts()
  return groups.find((group) => group.id === id)
}

function roleFactor(url) {
  const role = url.searchParams.get('user_role')
  if (role === 'admin') return 0.35
  if (role === 'user') return 0.65
  return 1
}

function usageTotal() {
  return (productionSnapshot?.group_usage || []).reduce((sum, item) => sum + Number(item.total_cost || 0), 0)
}

function dashboardStats(url) {
  const factor = roleFactor(url)
  const totalCost = usageTotal() * factor
  const totalTokens = Math.round(totalCost * 1_000_000)
  const totalRequests = Math.max(1, Math.round(totalTokens / 4200))
  const todayCost = (productionSnapshot?.group_usage || []).reduce((sum, item) => sum + Number(item.today_cost || 0), 0) * factor
  const todayTokens = Math.round(todayCost * 1_000_000)
  const todayRequests = Math.max(1, Math.round(todayTokens / 4200))
  const activeAccounts = accounts.filter((account) => account.status === 'active').length
  return {
    total_users: roleFactor(url) === 0.35 ? 1 : 8,
    today_new_users: roleFactor(url) === 0.35 ? 0 : 1,
    active_users: roleFactor(url) === 0.35 ? 1 : 5,
    hourly_active_users: roleFactor(url) === 0.35 ? 1 : 3,
    stats_updated_at: now,
    stats_stale: false,
    total_api_keys: roleFactor(url) === 0.35 ? 3 : 12,
    active_api_keys: roleFactor(url) === 0.35 ? 3 : 10,
    total_accounts: accounts.length,
    normal_accounts: activeAccounts,
    error_accounts: accounts.filter((account) => account.status !== 'active').length,
    ratelimit_accounts: 0,
    overload_accounts: 0,
    total_requests: totalRequests,
    total_input_tokens: Math.round(totalTokens * 0.7),
    total_output_tokens: Math.round(totalTokens * 0.25),
    total_cache_creation_tokens: Math.round(totalTokens * 0.03),
    total_cache_read_tokens: Math.round(totalTokens * 0.02),
    total_tokens: totalTokens,
    total_cost: totalCost,
    total_actual_cost: totalCost,
    total_account_cost: totalCost * 0.72,
    today_requests: todayRequests,
    today_input_tokens: Math.round(todayTokens * 0.7),
    today_output_tokens: Math.round(todayTokens * 0.25),
    today_cache_creation_tokens: Math.round(todayTokens * 0.03),
    today_cache_read_tokens: Math.round(todayTokens * 0.02),
    today_tokens: todayTokens,
    today_cost: todayCost,
    today_actual_cost: todayCost,
    today_account_cost: todayCost * 0.72,
    average_duration_ms: 1450,
    uptime: 86400 * 30,
    rpm: Math.max(1, Math.round(todayRequests / 1440)),
    tpm: Math.max(1, Math.round(todayTokens / 1440)),
  }
}

function rangeFromURL(url) {
  const end = new Date(`${url.searchParams.get('end_date') || '2026-08-29'}T23:00:00.000Z`)
  const start = new Date(`${url.searchParams.get('start_date') || '2026-08-28'}T00:00:00.000Z`)
  return { start, end }
}

function trendRows(url) {
  const { start, end } = rangeFromURL(url)
  const hourly = url.searchParams.get('granularity') === 'hour'
  const count = hourly ? 8 : 7
  const total = dashboardStats(url)
  return Array.from({ length: count }, (_, index) => {
    const point = new Date(end.getTime() - (count - index - 1) * (hourly ? 3 : 1) * 3600000)
    if (!hourly) point.setTime(start.getTime() + index * Math.max(1, Math.round((end.getTime() - start.getTime()) / Math.max(1, count - 1))))
    const ratio = 0.65 + index * 0.05
    return {
      date: hourly ? point.toISOString() : point.toISOString().slice(0, 10),
      requests: Math.round(total.today_requests * ratio / count),
      input_tokens: Math.round(total.today_input_tokens * ratio / count),
      output_tokens: Math.round(total.today_output_tokens * ratio / count),
      cache_creation_tokens: Math.round(total.today_cache_creation_tokens * ratio / count),
      cache_read_tokens: Math.round(total.today_cache_read_tokens * ratio / count),
      total_tokens: Math.round(total.today_tokens * ratio / count),
      cost: total.today_cost * ratio / count,
      actual_cost: total.today_actual_cost * ratio / count,
    }
  })
}

function modelRows(url) {
  const factor = roleFactor(url)
  const base = Math.max(1, Math.round(usageTotal() * 1_000_000))
  const models = [...new Set(channelMonitors.map((monitor) => monitor.primary_model).filter(Boolean))].slice(0, 8)
  return (models.length ? models : ['gpt-5.5']).map((model, index) => {
    const tokens = Math.round(base * factor * (1 - index * 0.08) / Math.max(1, models.length))
    const requests = Math.max(1, Math.round(tokens / 4200))
    return { model, requests, input_tokens: Math.round(tokens * 0.7), output_tokens: Math.round(tokens * 0.25), cache_creation_tokens: Math.round(tokens * 0.03), cache_read_tokens: Math.round(tokens * 0.02), total_tokens: tokens, cost: tokens / 1_000_000, actual_cost: tokens / 1_000_000, account_cost: tokens / 1_400_000 }
  })
}

function groupRows(url) {
  const factor = roleFactor(url)
  return (productionSnapshot?.group_usage || []).map((item) => {
    const group = groups.find((candidate) => candidate.id === Number(item.group_id))
    const cost = Number(item.total_cost || 0) * factor
    const tokens = Math.round(cost * 1_000_000)
    return { group_id: Number(item.group_id), group_name: group?.name || `Group #${item.group_id}`, requests: Math.max(1, Math.round(tokens / 4200)), total_tokens: tokens, cost, actual_cost: cost, account_cost: cost * 0.72 }
  })
}

function userTrendRows(url) {
  const factor = roleFactor(url)
  const total = dashboardStats(url)
  return trendRows(url).map((point, index) => ({
    date: point.date,
    user_id: factor === 0.35 ? demoUser.id : 100 + (index % 3),
    email: factor === 0.35 ? demoUser.email : `user-${(index % 3) + 1}@example.invalid`,
    username: factor === 0.35 ? demoUser.username : `demo-user-${(index % 3) + 1}`,
    requests: point.requests,
    tokens: point.total_tokens,
    cost: point.cost,
    actual_cost: point.actual_cost,
  }))
}

function usageRows(url) {
  const factor = roleFactor(url)
  const groupItems = productionSnapshot?.group_usage || []
  const rows = (groupItems.length ? groupItems : groups).map((item, index) => {
    const groupID = Number(item.group_id || item.id || groups[index]?.id || 1)
    const group = groups.find((candidate) => candidate.id === groupID) || groups[index % Math.max(1, groups.length)]
    const cost = Number(item.total_cost || item.today_cost || 0.012) * factor
    const inputTokens = Math.max(100, Math.round(cost * 700000))
    const outputTokens = Math.max(40, Math.round(cost * 250000))
    return {
      id: index + 1,
      user_id: factor === 0.35 ? demoUser.id : 100 + (index % 3),
      api_key_id: factor === 0.35 ? 1 : 10 + (index % 3),
      account_id: accounts[index % Math.max(1, accounts.length)]?.id || null,
      request_id: `local-demo-${index + 1}`,
      model: channelMonitors[index % Math.max(1, channelMonitors.length)]?.primary_model || 'gpt-5.5',
      upstream_model: null,
      upstream_response_model: null,
      model_mapping_chain: null,
      inbound_endpoint: '/v1/chat/completions',
      upstream_endpoint: 'https://upstream.example.invalid/v1/chat/completions',
      group_id: group?.id || null,
      subscription_id: null,
      input_tokens: inputTokens,
      output_tokens: outputTokens,
      cache_creation_tokens: 0,
      cache_read_tokens: 0,
      cache_creation_5m_tokens: 0,
      cache_creation_1h_tokens: 0,
      input_cost: cost * 0.7,
      output_cost: cost * 0.3,
      cache_creation_cost: 0,
      cache_read_cost: 0,
      total_cost: cost,
      actual_cost: cost,
      account_rate_multiplier: 0.72,
      account_stats_cost: cost * 0.72,
      rate_multiplier: 1,
      long_context_billing_applied: false,
      billing_type: 0,
      billing_mode: 'tokens',
      request_type: 'stream',
      stream: true,
      duration_ms: 1450 + index * 90,
      first_token_ms: 480 + index * 30,
      image_count: 0,
      image_size: null,
      image_input_size: null,
      image_output_size: null,
      image_size_source: null,
      image_size_breakdown: null,
      image_input_tokens: 0,
      image_input_cost: 0,
      image_output_tokens: 0,
      image_output_cost: 0,
      user_agent: 'Sub2API Local Demo',
      ip_address: '127.0.0.1',
      cache_ttl_overridden: false,
      created_at: now,
      user: {
        id: factor === 0.35 ? demoUser.id : 100 + (index % 3),
        email: factor === 0.35 ? demoUser.email : `user-${(index % 3) + 1}@example.invalid`,
        username: factor === 0.35 ? demoUser.username : `demo-user-${(index % 3) + 1}`,
        role: factor === 0.35 ? 'admin' : 'user',
      },
      api_key: { id: factor === 0.35 ? 1 : 10 + (index % 3), name: factor === 0.35 ? 'Admin demo key' : `User demo key ${index + 1}`, user_id: factor === 0.35 ? demoUser.id : 100 + (index % 3) },
      account: accounts[index % Math.max(1, accounts.length)] ? { id: accounts[index % accounts.length].id, name: accounts[index % accounts.length].name } : null,
      group: group ? { id: group.id, name: group.name } : null,
    }
  })
  const requestedRole = url.searchParams.get('user_role')
  return requestedRole === 'admin' ? rows.filter((row) => row.user?.role === 'admin') : requestedRole === 'user' ? rows.filter((row) => row.user?.role === 'user') : rows
}

function monitorStatus(status) {
  return status || 'operational'
}

function userMonitorView(monitor) {
  const status = monitorStatus(monitor.primary_status)
  const monitorGroup = channelMonitorGroups.find((group) => group.id === monitor.monitor_group_id)
  return {
    id: monitor.id,
    name: monitor.name,
    provider: monitor.provider,
    // Keep the legacy business label for compatibility, but expose the shared
    // admin monitor organization separately so both UIs render the same groups.
    group_name: monitor.group_name || monitor.name,
    monitor_group_id: monitor.monitor_group_id == null ? null : Number(monitor.monitor_group_id),
    monitor_group_name: monitorGroup?.name || null,
    monitor_group_sort_order: monitorGroup?.sort_order ?? 0,
    monitor_sort_order: Number(monitor.monitor_sort_order || 0),
    primary_model: monitor.primary_model,
    primary_status: status,
    primary_latency_ms: monitor.primary_latency_ms ?? null,
    primary_ping_latency_ms: monitor.primary_latency_ms ?? null,
    availability_7d: Number(monitor.availability_7d || 0),
    extra_models: (monitor.extra_models || []).map((model) => ({ model, status: 'operational', latency_ms: monitor.primary_latency_ms ?? null })),
    timeline: Array.from({ length: 8 }, (_, index) => ({ status, latency_ms: monitor.primary_latency_ms ?? null, ping_latency_ms: monitor.primary_latency_ms ?? null, checked_at: new Date(Date.now() - (7 - index) * 3600000).toISOString() })),
  }
}

function monitorCoverage(url) {
  const { start, end } = rangeFromURL(url)
  return { requested_start: start.toISOString(), requested_end: end.toISOString(), coverage_start: start.toISOString(), data_through: now, computed_at: now, aggregation_lag_seconds: 0, coverage_complete: true, bucket_seconds: 3600, bootstrap: null }
}

function metricForMonitor(monitor) {
  const requests = 120 + Number(monitor.id || 0) * 17
  const errors = monitorStatus(monitor.primary_status) === 'operational' ? 4 : 24
  const success = requests - errors
  const latency = monitor.primary_latency_ms ?? 800
  const latencyMetric = { sample_count: requests, p50_ms: latency, p90_ms: Math.round(latency * 1.35), p95_ms: Math.round(latency * 1.6), avg_ms: latency }
  return { success_requests: success, error_requests: errors, request_count: requests, token_count: requests * 4200, rpm: Math.round(requests / 24), tpm: Math.round(requests * 4200 / 24), error_rate: errors / requests, cache_rate: 0.82, cache_rate_numerator: 82, cache_rate_denominator: 100, ttft: latencyMetric, duration: latencyMetric, upstream_affected_requests: errors, upstream_attempt_count: requests + errors }
}

function healthForMetric(metrics) {
  const overall = metrics.error_rate > 0.2 ? 'critical' : metrics.error_rate > 0.05 ? 'warning' : 'healthy'
  return { overall, error_rate: overall, ttft: metrics.ttft.p95_ms > 10000 ? 'critical' : metrics.ttft.p95_ms > 3000 ? 'warning' : 'healthy', cache: 'healthy', score: Math.round((1 - metrics.error_rate) * 100), error_rate_score: Math.round((1 - metrics.error_rate) * 100), ttft_score: 80, cache_score: 82, minimum_sample: 50, thresholds: monitorV2Config.health_thresholds }
}

function v2Snapshot(url) {
  const metrics = channelMonitors.reduce((total, monitor) => {
    const current = metricForMonitor(monitor)
    for (const key of ['success_requests', 'error_requests', 'request_count', 'token_count', 'upstream_affected_requests', 'upstream_attempt_count']) total[key] += current[key] || 0
    return total
  }, { success_requests: 0, error_requests: 0, request_count: 0, token_count: 0, rpm: 0, tpm: 0, error_rate: 0, cache_rate: 0.82, cache_rate_numerator: 82, cache_rate_denominator: 100, ttft: { sample_count: 0, p50_ms: 0, p90_ms: 0, p95_ms: 0, avg_ms: 0 }, duration: { sample_count: 0, p50_ms: 0, p90_ms: 0, p95_ms: 0, avg_ms: 0 } })
  metrics.error_rate = metrics.request_count ? metrics.error_requests / metrics.request_count : 0
  metrics.rpm = Math.round(metrics.request_count / 24)
  metrics.tpm = Math.round(metrics.token_count / 24)
  metrics.ttft = { sample_count: metrics.request_count, p50_ms: 820, p90_ms: 1120, p95_ms: 1450, avg_ms: 900 }
  metrics.duration = { sample_count: metrics.request_count, p50_ms: 1450, p90_ms: 2200, p95_ms: 3200, avg_ms: 1600 }
  const trend = Array.from({ length: 8 }, (_, index) => ({ bucket_start: new Date(Date.now() - (7 - index) * 3600000).toISOString(), metrics: { ...metrics, request_count: Math.round(metrics.request_count / 8), success_requests: Math.round(metrics.success_requests / 8), error_requests: Math.round(metrics.error_requests / 8), token_count: Math.round(metrics.token_count / 8) }, health: healthForMetric(metrics) }))
  return { config: monitorV2Config, coverage: monitorCoverage(url), metrics, health: healthForMetric(metrics), trend }
}

async function handle(req, res) {
  if (req.method === 'OPTIONS') return json(res, 204, {})
  const url = new URL(req.url, `http://${host}:${port}`)
  const path = url.pathname
  const body = ['POST', 'PUT', 'PATCH'].includes(req.method) ? await readBody(req) : {}

  if (path === '/setup/status') return json(res, 200, { data: { needs_setup: false, step: '' } })

  const apiPath = path.startsWith('/api/v1') ? path.slice('/api/v1'.length) || '/' : path
  if (apiPath === '/settings/public') {
    return api(res, {
      site_name: 'Sub2API Local Demo',
      site_logo: '',
      site_subtitle: 'Local-only administrator preview',
      api_base_url: '/api/v1',
      contact_info: '',
      doc_url: '',
      registration_enabled: false,
      email_verify_enabled: false,
      force_email_on_third_party_signup: false,
      payment_enabled: false,
      backend_mode_enabled: false,
      table_default_page_size: 20,
      table_page_size_options: [10, 20, 50, 100],
      custom_menu_items: [],
      model_plaza_enabled: false,
      model_plaza_require_auth: false,
      channel_monitor_enabled: true,
      channel_monitor_mode: 'v1',
      channel_monitor_default_interval_seconds: 60,
      channel_monitor_hide_throughput: false,
      channel_monitor_show_quota: false,
      available_channels_enabled: false,
      plugin_management_enabled: false,
      service_quota_enabled: false,
      affiliate_enabled: false,
      risk_control_enabled: false,
    })
  }
  if (apiPath === '/admin/settings/upstream-providers' && req.method === 'GET') {
    return api(res, upstreamProfiles)
  }
  if (apiPath === '/admin/settings/upstream-providers' && req.method === 'PUT') {
    upstreamProfiles.splice(0, upstreamProfiles.length, ...(Array.isArray(body.profiles) ? body.profiles : []))
    return api(res, upstreamProfiles)
  }

  if (apiPath === '/admin/usage' && req.method === 'GET') return api(res, paginated(usageRows(url), url))
  if (apiPath === '/admin/usage/stats' && req.method === 'GET') return api(res, dashboardStats(url))
  if (apiPath === '/admin/channel-monitor-v2/config' && req.method === 'GET') return api(res, monitorV2Config)
  if (apiPath === '/admin/channel-monitor-v2/config' && req.method === 'PUT') {
    Object.assign(monitorV2Config, body, { health_thresholds: { ...monitorV2Config.health_thresholds, ...(body.health_thresholds || {}) } })
    return api(res, monitorV2Config)
  }

  if (apiPath.startsWith('/admin/dashboard/')) {
    const { start, end } = rangeFromURL(url)
    const granularity = url.searchParams.get('granularity') || 'day'
    if (apiPath === '/admin/dashboard/stats') return api(res, dashboardStats(url))
    if (apiPath === '/admin/dashboard/realtime') return api(res, { active_requests: 0, requests_per_minute: dashboardStats(url).rpm, average_response_time: 1450, error_rate: 0.03 })
    if (apiPath === '/admin/dashboard/snapshot-v2') {
      const snapshot = v2Snapshot(url)
      return api(res, {
        generated_at: now,
        start_date: start.toISOString().slice(0, 10),
        end_date: end.toISOString().slice(0, 10),
        granularity,
        ...(url.searchParams.get('include_stats') !== 'false' ? { stats: { ...dashboardStats(url), uptime: 86400 * 30 } } : {}),
        ...(url.searchParams.get('include_trend') !== 'false' ? { trend: trendRows(url) } : {}),
        ...(url.searchParams.get('include_model_stats') !== 'false' ? { models: modelRows(url) } : {}),
        ...(url.searchParams.get('include_group_stats') === 'true' ? { groups: groupRows(url) } : {}),
        ...(url.searchParams.get('include_users_trend') === 'true' ? { users_trend: userTrendRows(url) } : {}),
        _monitor_snapshot: snapshot,
      })
    }
    if (apiPath === '/admin/dashboard/trend') return api(res, { trend: trendRows(url), start_date: start.toISOString().slice(0, 10), end_date: end.toISOString().slice(0, 10), granularity })
    if (apiPath === '/admin/dashboard/models') return api(res, { models: modelRows(url), start_date: start.toISOString().slice(0, 10), end_date: end.toISOString().slice(0, 10) })
    if (apiPath === '/admin/dashboard/groups') return api(res, { groups: groupRows(url), start_date: start.toISOString().slice(0, 10), end_date: end.toISOString().slice(0, 10) })
    if (apiPath === '/admin/dashboard/users-trend') return api(res, { trend: userTrendRows(url), start_date: start.toISOString().slice(0, 10), end_date: end.toISOString().slice(0, 10), granularity })
    if (apiPath === '/admin/dashboard/users-ranking') {
      const trend = userTrendRows(url)
      const ranking = [...new Map(trend.map((item) => [item.user_id, item])).values()].map((item) => ({ user_id: item.user_id, email: item.email, username: item.username, actual_cost: item.actual_cost * 7, requests: item.requests * 7, tokens: item.tokens * 7 })).sort((a, b) => b.actual_cost - a.actual_cost)
      return api(res, { ranking, total_actual_cost: ranking.reduce((sum, item) => sum + item.actual_cost, 0), total_requests: ranking.reduce((sum, item) => sum + item.requests, 0), total_tokens: ranking.reduce((sum, item) => sum + item.tokens, 0), start_date: start.toISOString().slice(0, 10), end_date: end.toISOString().slice(0, 10) })
    }
    if (apiPath === '/admin/dashboard/user-breakdown') {
      const trend = userTrendRows(url)
      return api(res, { users: [...new Map(trend.map((item) => [item.user_id, { user_id: item.user_id, email: item.email, requests: item.requests * 7, input_tokens: Math.round(item.tokens * 0.7 * 7), output_tokens: Math.round(item.tokens * 0.25 * 7), cache_tokens: Math.round(item.tokens * 0.05 * 7), total_tokens: item.tokens * 7, cost: item.cost * 7, actual_cost: item.actual_cost * 7, account_cost: item.actual_cost * 7 * 0.72 }])).values()], start_date: start.toISOString().slice(0, 10), end_date: end.toISOString().slice(0, 10) })
    }
    if (apiPath === '/admin/dashboard/api-keys-trend') return api(res, { trend: [], start_date: start.toISOString().slice(0, 10), end_date: end.toISOString().slice(0, 10), granularity })
    if (apiPath === '/admin/dashboard/users-usage' && req.method === 'POST') return api(res, { stats: {} })
    if (apiPath === '/admin/dashboard/api-keys-usage' && req.method === 'POST') return api(res, { stats: {} })
  }

  if (apiPath === '/channel-monitors' && req.method === 'GET') return api(res, { items: channelMonitors.filter((monitor) => monitor.enabled !== false).map(userMonitorView) })
  const userMonitorMatch = apiPath.match(/^\/channel-monitors\/(\d+)\/status$/)
  if (userMonitorMatch && req.method === 'GET') {
    const monitor = channelMonitors.find((item) => item.id === Number(userMonitorMatch[1]))
    if (!monitor) return api(res, { message: 'not found' }, 404)
    const view = userMonitorView(monitor)
    const models = [monitor.primary_model, ...(monitor.extra_models || [])].filter(Boolean).map((model) => ({ model, latest_status: view.primary_status, latest_latency_ms: view.primary_latency_ms, availability_7d: view.availability_7d, availability_15d: view.availability_7d, availability_30d: view.availability_7d, avg_latency_7d_ms: view.primary_latency_ms }))
    return api(res, { id: view.id, name: view.name, provider: view.provider, group_name: view.group_name, models })
  }

  for (const admin of [false, true]) {
    const prefix = admin ? '/admin/channel-monitor-v2' : '/channel-monitor-v2'
    if (!apiPath.startsWith(prefix + '/')) continue
    const suffix = apiPath.slice(prefix.length)
    const snapshot = v2Snapshot(url)
    if (suffix === '/dimensions') {
      return api(res, {
        platforms: [...new Set(channelMonitors.map((monitor) => monitor.provider).filter(Boolean))].map((platform) => ({ value: platform, label: platform, request_count: channelMonitors.filter((monitor) => monitor.provider === platform).reduce((sum, monitor) => sum + metricForMonitor(monitor).request_count, 0) })),
        groups: groups.map((group) => ({ id: group.id, name: group.name, platform: group.platform, request_count: group.account_count * 100 || 0 })),
        models: [...new Set(channelMonitors.flatMap((monitor) => [monitor.primary_model, ...(monitor.extra_models || [])]).filter(Boolean))].map((model) => ({ value: model, label: model, request_count: 100 })),
      })
    }
    if (suffix === '/snapshot') return api(res, snapshot)
    if (suffix === '/matrix') {
      const groupBy = url.searchParams.get('group_by') || 'platform_group_model'
      return api(res, { coverage: snapshot.coverage, group_by: groupBy, items: channelMonitors.map((monitor) => { const metrics = metricForMonitor(monitor); return { platform: monitor.provider, group_id: groups.find((group) => group.name === monitor.group_name)?.id, group_name: monitor.group_name || monitor.name, model: monitor.primary_model, metrics, health: healthForMetric(metrics), buckets: [{ bucket_start: now, metrics, health: healthForMetric(metrics) }] } }) })
    }
    if (suffix === '/models') return api(res, { coverage: snapshot.coverage, items: channelMonitors.map((monitor) => { const metrics = metricForMonitor(monitor); return { platform: monitor.provider, model: monitor.primary_model, metrics, health: healthForMetric(metrics) } }) })
    if (suffix === '/errors') return api(res, { coverage: snapshot.coverage, items: [{ category: 'upstream_5xx', count: snapshot.metrics.error_requests, rate: snapshot.metrics.error_rate, ignored: false, details: [] }] })
    if (suffix === '/users') return api(res, { coverage: snapshot.coverage, items: [{ user_id: demoUser.id, rank: 1, email: demoUser.email, username: demoUser.username, display_label: demoUser.username, is_self: true, can_drilldown: admin, metrics: snapshot.metrics }] })
  }

  if (apiPath === '/admin/channel-monitors' && req.method === 'GET') {
    const provider = url.searchParams.get('provider')
    const enabled = url.searchParams.get('enabled')
    const search = (url.searchParams.get('search') || '').toLowerCase()
    const result = channelMonitors
      .filter((monitor) => (!provider || monitor.provider === provider) && (enabled == null || String(monitor.enabled) === enabled) && (!search || monitor.name.toLowerCase().includes(search)))
      .slice()
      .sort((a, b) => {
        const groupA = channelMonitorGroups.find((group) => group.id === a.monitor_group_id)
        const groupB = channelMonitorGroups.find((group) => group.id === b.monitor_group_id)
        return (groupA?.sort_order ?? Number.MAX_SAFE_INTEGER) - (groupB?.sort_order ?? Number.MAX_SAFE_INTEGER)
          || a.monitor_sort_order - b.monitor_sort_order
          || a.id - b.id
      })
      .map(monitorResponse)
    return api(res, paginated(result, url))
  }
  if (apiPath === '/admin/channel-monitor-groups' && req.method === 'GET') {
    refreshMonitorGroupCounts()
    return api(res, { items: channelMonitorGroups.slice().sort((a, b) => a.sort_order - b.sort_order || a.id - b.id) })
  }
  if (apiPath === '/admin/channel-monitor-groups' && req.method === 'POST') {
    const nextID = Math.max(...channelMonitorGroups.map((group) => Number(group.id) || 0), 0) + 1
    const group = { id: nextID, name: String(body.name || `Monitor group ${nextID}`).trim(), sort_order: (channelMonitorGroups.length ? Math.max(...channelMonitorGroups.map((item) => item.sort_order)) + 10 : 0), monitor_count: 0, created_by: demoUser.id, created_at: now, updated_at: now }
    channelMonitorGroups.push(group)
    return api(res, group)
  }
  if (apiPath === '/admin/channel-monitor-groups/sort-order' && req.method === 'PUT') {
    for (const item of body.updates || []) {
      const group = channelMonitorGroups.find((candidate) => candidate.id === Number(item.id))
      if (group) group.sort_order = Number(item.sort_order) || 0
    }
    return api(res, { message: 'updated' })
  }
  const monitorGroupMatch = apiPath.match(/^\/admin\/channel-monitor-groups\/(\d+)$/)
  if (monitorGroupMatch) {
    const group = channelMonitorGroups.find((item) => item.id === Number(monitorGroupMatch[1]))
    if (!group) return api(res, { message: 'not found' }, 404)
    if (req.method === 'GET') return api(res, group)
    if (req.method === 'PUT') {
      if (body.name != null) group.name = String(body.name).trim() || group.name
      group.updated_at = now
      return api(res, group)
    }
    if (req.method === 'DELETE') {
      for (const monitor of channelMonitors) {
        if (monitor.monitor_group_id === group.id) {
          monitor.monitor_group_id = null
          monitor.monitor_sort_order = 0
        }
      }
      channelMonitorGroups.splice(channelMonitorGroups.indexOf(group), 1)
      refreshMonitorGroupCounts()
      return api(res, { message: 'deleted' })
    }
  }
  if (apiPath === '/admin/channel-monitors/sort-order' && req.method === 'PUT') {
    for (const item of body.updates || []) {
      const monitor = channelMonitors.find((candidate) => candidate.id === Number(item.id))
      if (!monitor) continue
      monitor.monitor_group_id = item.monitor_group_id == null ? null : Number(item.monitor_group_id)
      monitor.monitor_sort_order = Number(item.monitor_sort_order) || 0
      monitor.updated_at = now
    }
    refreshMonitorGroupCounts()
    return api(res, { message: 'updated' })
  }
  if (apiPath === '/admin/channel-monitors' && req.method === 'POST') {
    const nextID = Math.max(...channelMonitors.map((monitor) => Number(monitor.id) || 0), 0) + 1
    const monitor = {
      id: nextID,
      name: body.name || `Local monitor ${nextID}`,
      provider: body.provider || 'openai',
      api_mode: body.api_mode || 'chat_completions',
      endpoint: body.endpoint || '',
      api_key_masked: body.api_key ? `${String(body.api_key).slice(0, 4)}***` : '[已隐藏]',
      primary_model: body.primary_model || '',
      extra_models: Array.isArray(body.extra_models) ? body.extra_models : [],
      group_name: body.group_name || '',
      monitor_group_id: body.monitor_group_id == null ? null : Number(body.monitor_group_id),
      monitor_sort_order: Number(body.monitor_sort_order) || 0,
      enabled: body.enabled !== false,
      interval_seconds: Number(body.interval_seconds) || 60,
      jitter_seconds: Number(body.jitter_seconds) || 0,
      created_by: demoUser.id,
      created_at: now,
      updated_at: now,
      primary_status: 'operational',
      primary_latency_ms: 820,
      availability_7d: 100,
      extra_models_status: [],
      extra_headers: body.extra_headers || {},
      body_override: body.body_override || null,
      template_id: body.template_id || null,
      body_override_mode: body.body_override_mode || 'off',
      check_mode: body.check_mode || 'probe',
      account_id: body.account_id || null,
      last_checked_at: now,
    }
    channelMonitors.push(monitor)
    refreshMonitorGroupCounts()
    return api(res, monitorResponse(monitor))
  }
  const adminMonitorMatch = apiPath.match(/^\/admin\/channel-monitors\/(\d+)(.*)$/)
  if (adminMonitorMatch) {
    const monitor = channelMonitors.find((item) => item.id === Number(adminMonitorMatch[1]))
    if (!monitor) return api(res, { message: 'not found' }, 404)
    const suffix = adminMonitorMatch[2]
    if (suffix === '' && req.method === 'GET') return api(res, monitorResponse(monitor))
    if (suffix === '' && req.method === 'PUT') {
      Object.assign(monitor, body, { id: monitor.id, updated_at: now })
      if (body.api_key) monitor.api_key_masked = `${String(body.api_key).slice(0, 4)}***`
      refreshMonitorGroupCounts()
      return api(res, monitorResponse(monitor))
    }
    if (suffix === '' && req.method === 'DELETE') {
      channelMonitors.splice(channelMonitors.indexOf(monitor), 1)
      refreshMonitorGroupCounts()
      return api(res, { message: 'deleted' })
    }
    if (suffix === '/duplicate' && req.method === 'POST') {
      const duplicate = { ...monitor, id: Math.max(...channelMonitors.map((item) => item.id), 0) + 1, name: `${monitor.name} Copy`, created_at: now, updated_at: now }
      channelMonitors.push(duplicate)
      return api(res, duplicate)
    }
    if (suffix === '/run' && req.method === 'POST') {
      const result = { model: monitor.primary_model, status: monitor.primary_status || 'operational', latency_ms: monitor.primary_latency_ms || 820, ping_latency_ms: monitor.primary_latency_ms || 820, message: 'Local demo check', checked_at: now }
      return api(res, { results: [result, ...(monitor.extra_models || []).map((model) => ({ ...result, model }))] })
    }
    if (suffix === '/history' && req.method === 'GET') {
      return api(res, { items: Array.from({ length: Math.min(Number(url.searchParams.get('limit') || 20), 20) }, (_, index) => ({ id: index + 1, model: monitor.primary_model, status: monitor.primary_status || 'operational', latency_ms: monitor.primary_latency_ms || 820, ping_latency_ms: monitor.primary_latency_ms || 820, message: 'Local demo history', checked_at: new Date(Date.now() - index * 3600000).toISOString() })) })
    }
  }
  if (apiPath === '/auth/login' && req.method === 'POST') {
    return api(res, { access_token: 'local-demo-token', token_type: 'Bearer', expires_in: 86400, user: demoUser })
  }
  if (apiPath === '/auth/me') return api(res, demoUser)
  if (apiPath === '/admin/compliance') {
    return api(res, { required: false, version: 'local-demo', document_path_zh: '', document_path_en: '', document_url_zh: '', document_url_en: '', ack_phrase_zh: '', ack_phrase_en: '' })
  }
  if (apiPath === '/admin/groups/live-capability') return api(res, { supported: false, reason: 'Local demo mode' })
  if (apiPath === '/announcements') return api(res, [])
  if (apiPath === '/usage' && req.method === 'GET') return api(res, paginated([], url))
  if (apiPath.startsWith('/usage/')) return api(res, {})
  if (apiPath === '/admin/groups/usage-summary') {
    const usageByGroup = new Map((productionSnapshot?.group_usage || []).map((item) => [Number(item.group_id), item]))
    return api(res, groups.map((group) => usageByGroup.get(group.id) || ({ group_id: group.id, today_cost: 0, yesterday_cost: 0, total_cost: 0 })))
  }
  if (apiPath === '/admin/groups/capacity-summary') {
    refreshCounts()
    return api(res, groups.map((group) => ({ group_id: group.id, concurrency_used: group.active_account_count, concurrency_max: group.account_count * 10, sessions_used: 0, sessions_max: group.account_count * 10, rpm_used: 0, rpm_max: group.rpm_limit })))
  }
  if (apiPath === '/admin/groups/all') {
    refreshCounts()
    const includeInactive = url.searchParams.get('include_inactive') === 'true'
    const platform = url.searchParams.get('platform')
    return api(res, groups.filter((group) => (includeInactive || group.status === 'active') && (!platform || group.platform === platform)))
  }
  if (apiPath === '/admin/groups' && req.method === 'GET') {
    refreshCounts()
    const platform = url.searchParams.get('platform')
    const status = url.searchParams.get('status')
    const search = (url.searchParams.get('search') || '').toLowerCase()
    const result = groups.filter((group) => (!platform || group.platform === platform) && (!status || group.status === status) && (!search || group.name.toLowerCase().includes(search))).sort((a, b) => a.sort_order - b.sort_order)
    return api(res, paginated(result, url))
  }
  if (apiPath === '/admin/groups' && req.method === 'POST') {
    const nextID = Math.max(...groups.map((group) => group.id), 0) + 1
    const group = { ...baseGroup(nextID, body.name || `Local group ${nextID}`, body.platform || 'openai', nextID * 10), ...body, id: nextID, created_at: now, updated_at: now }
    groups.push(group)
    refreshCounts()
    return api(res, group)
  }
  if (apiPath === '/admin/groups/sort-order' && req.method === 'PUT') {
    for (const item of body.updates || []) {
      const group = getGroup(Number(item.id))
      if (group) group.sort_order = Number(item.sort_order) || group.sort_order
    }
    return api(res, { message: 'updated' })
  }
  const groupMatch = apiPath.match(/^\/admin\/groups\/(\d+)(.*)$/)
  if (groupMatch) {
    const groupID = Number(groupMatch[1])
    const suffix = groupMatch[2]
    if (suffix === '/models-list-candidates') return api(res, { models: ['gpt-4.1', 'gpt-4o', 'claude-sonnet-4-20250514'] })
    const group = getGroup(groupID)
    if (!group) return api(res, { message: 'not found' }, 404)
    if (suffix === '/composite-routes' && req.method === 'GET') return api(res, [])
    if (suffix === '/composite-routes/preview' && req.method === 'POST') return api(res, { matched: false, source: 'route', group_id: groupID, public_model: body.model || '', target_platform: '', upstream_model: '', endpoint: body.endpoint || 'any', reason: 'No local demo route' })
    if (suffix === '/api-keys') return api(res, paginated([], url))
    if (suffix === '/rate-multipliers') return api(res, [])
    if (suffix === '/stats') return api(res, { total_api_keys: 0, active_api_keys: 0, total_requests: 0, total_cost: 0 })
    if (suffix === '' && req.method === 'GET') return api(res, group)
    if (suffix === '' && req.method === 'PUT') {
      Object.assign(group, body, { id: group.id, updated_at: now })
      refreshCounts()
      return api(res, group)
    }
    if (suffix === '/duplicate' && req.method === 'POST') {
      const nextID = Math.max(...groups.map((item) => item.id), 0) + 1
      const duplicate = { ...group, ...body, id: nextID, name: `${group.name} Copy`, sort_order: nextID * 10, created_at: now, updated_at: now }
      groups.push(duplicate)
      return api(res, duplicate)
    }
    if (suffix === '' && req.method === 'DELETE') {
      const index = groups.findIndex((item) => item.id === groupID)
      groups.splice(index, 1)
      for (const account of accounts) account.group_ids = account.group_ids.filter((id) => id !== groupID)
      refreshCounts()
      return api(res, { message: 'deleted' })
    }
  }
  if (apiPath === '/admin/accounts' && req.method === 'GET') return api(res, paginated(filterAccounts(url), url))
  if (apiPath === '/admin/accounts' && req.method === 'POST') {
    const nextID = Math.max(...accounts.map((account) => account.id), 0) + 1
    const profileID = Number(body.extra?.upstream_provider_profile_id)
    const account = {
      id: nextID,
      name: body.name || `Local account ${nextID}`,
      platform: body.platform || 'openai',
      type: body.type || 'apikey',
      priority: Math.max(1, Math.round(Number(body.priority) || 1)),
      status: 'active',
      group_ids: Array.isArray(body.group_ids) ? body.group_ids.map(Number) : [],
      upstream_provider_profile_id: Number.isFinite(profileID) && profileID > 0 ? profileID : undefined,
      base_url: body.credentials?.base_url || '',
    }
    accounts.push(account)
    refreshCounts()
    return api(res, accountResponse(account))
  }
  const accountTestMatch = apiPath.match(/^\/admin\/accounts\/(\d+)\/(models|test)$/)
  if (accountTestMatch) {
    const account = accounts.find((item) => item.id === Number(accountTestMatch[1]))
    if (!account) return api(res, { message: 'not found' }, 404)
    const action = accountTestMatch[2]
    if (action === 'models' && req.method === 'GET') {
      const modelByPlatform = {
        openai: { id: 'gpt-4o-mini', display_name: 'GPT-4o mini' },
        anthropic: { id: 'claude-3-5-sonnet', display_name: 'Claude 3.5 Sonnet' },
        gemini: { id: 'gemini-2.5-flash', display_name: 'Gemini 2.5 Flash' },
        grok: { id: 'grok-4', display_name: 'Grok 4' },
      }
      return api(res, [modelByPlatform[account.platform] || { id: 'local-demo-model', display_name: 'Local demo model' }])
    }
    if (action === 'test' && req.method === 'POST') {
      const model = body.model_id || 'local-demo-model'
      return sse(res, [
        { type: 'test_start', model },
        { type: 'content', text: 'Local demo connection succeeded.' },
        { type: 'test_complete', success: true },
      ])
    }
  }

  const accountMatch = apiPath.match(/^\/admin\/accounts\/(\d+)$/)
  if (accountMatch) {
    const account = accounts.find((item) => item.id === Number(accountMatch[1]))
    if (!account) return api(res, { message: 'not found' }, 404)
    if (req.method === 'GET') return api(res, accountResponse(account))
    if (req.method === 'PUT') {
      Object.assign(account, body, { id: account.id })
      if (body.extra && Object.prototype.hasOwnProperty.call(body.extra, 'upstream_provider_profile_id')) {
        account.upstream_provider_profile_id = Number(body.extra.upstream_provider_profile_id) || undefined
      }
      if (body.credentials?.base_url) account.base_url = body.credentials.base_url
      if (Number.isFinite(Number(body.priority))) account.priority = Math.max(1, Math.round(Number(body.priority)))
      if (Array.isArray(body.group_ids)) account.group_ids = [...new Set(body.group_ids.map(Number).filter((id) => groups.some((group) => group.id === id)))]
      return api(res, accountResponse(account))
    }
  }
  if (apiPath === '/admin/proxies/all') return api(res, [])
  if (apiPath === '/admin/accounts/all') return api(res, accounts.map(accountResponse))
  if (apiPath === '/auth/logout' && req.method === 'POST') return api(res, { message: 'logged out' })
  return api(res, Array.isArray(body) ? [] : {})
}

refreshCounts()
const server = createServer((req, res) => {
  handle(req, res).catch((error) => {
    console.error('[local-demo] request failed:', error)
    json(res, 500, { code: 500, message: 'local demo mock error', data: null })
  })
})

server.listen(port, host, () => {
  console.log(`[local-demo] mock API listening at http://${host}:${port}`)
  console.log('[local-demo] data is in-memory and never connects to production')
})
