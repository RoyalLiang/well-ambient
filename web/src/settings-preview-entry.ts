import { mount } from 'svelte'
import './styles/modern-admin-tokens.css'
import './styles/settings-config-workbench.css'
import SettingsConfigPreview from './components/prototype/SettingsConfigPreview.svelte'

const contextFacts = [
  {
    id: 1,
    type: 'architecture',
    scope: 'global',
    scope_id: '',
    source: 'doc',
    owner: 'platform-team',
    status: 'active',
    version: 4,
    summary: '管理台采用统一轻量工作台结构',
    content: '配置页使用只读概览、显式健康状态、分步编辑与版本审计。',
    token_count: 84,
    freshness: 0.95,
    confidence: 0.9
  },
  {
    id: 2,
    type: 'workflow',
    scope: 'module',
    scope_id: 'settings',
    source: 'manual',
    owner: 'product-ops',
    status: 'draft',
    version: 2,
    summary: '集成配置先检测再启用',
    content: '凭证录入后执行连接检查，检测结果与配置完整性分别呈现。',
    token_count: 66,
    freshness: 0.82,
    confidence: 0.86
  }
]

const projects = [
  { id: 1, project_name: '智能驾驶平台', project_key: 'IDP', base_priority: 'P1', project_phase: '交付', base_score: 82, base_score_weight: 0.2, git_repos_json: '[]' },
  { id: 2, project_name: '远程运营中心', project_key: 'ROC', base_priority: 'P2', project_phase: '运营', base_score: 76, base_score_weight: 0.15, git_repos_json: '[]' },
  { id: 3, project_name: '设备诊断服务', project_key: 'EDS', base_priority: 'P3', project_phase: '售后', base_score: 68, base_score_weight: 0.1, git_repos_json: '[]' }
]

const repositoryFixtures = Array.from({ length: 12 }, (_, index) => ({
  name: `ambient-service-${String(index + 1).padStart(2, '0')}`,
  path: `platform/ambient-service-${String(index + 1).padStart(2, '0')}`,
  project_id: String(101 + index)
}))

const previewConfig = {
  server: { host: '127.0.0.1', port: 8080 },
  gitlab: {
    enabled: true,
    base_url: 'https://devgit.westwell.cc',
    secret_token: 'preview-secret',
    api_token: 'preview-api-token',
    repos: repositoryFixtures
  },
  feishu: {
    enabled: true,
    app_id: 'cli_preview_ambient',
    app_secret: 'preview-secret',
    bot: { enabled: true, chat_group: 'oc_delivery_ops' },
    bitable: {
      enabled: true,
      app_token: 'bascnPreviewToken',
      table_id: 'tblDeliveryEvidence',
      status_column: '任务状态',
      task_id_column: '任务ID'
    }
  },
  jira: {
    enabled: true,
    base_url: 'https://jira.westwell-lab.com',
    username: 'integration@westwell-lab.com',
    api_token: 'preview-token',
    sync_projects: ['IDP', 'ROC', 'EDS', 'DMS'],
    sync_users: ['delivery-owner', 'platform-owner'],
    sync_statuses: ['To Do', 'In Progress', 'In Review', 'Done'],
    custom_jql: 'project in (IDP, ROC, EDS) AND statusCategory != Done'
  },
  ai: {
    enabled: true,
    provider: 'openai-compatible',
    base_url: 'https://api.example.com/v1/responses',
    endpoint_type: 'responses',
    api_token: 'preview-token',
    model: 'reasoning-large',
    default_work_hours_per_day: 8
  }
}

const versionSectionSequence = [
  'jira', 'jira', 'jira', 'jira', 'jira', 'jira', 'jira', 'jira',
  'gitlab', 'gitlab', 'gitlab', 'feishu', 'projects', 'ai', 'gitlab', 'jira'
]

const configVersions = versionSectionSequence.map((section, index) => {
  const version = versionSectionSequence.length + 3 - index
  const isRepositoryDiff = section === 'gitlab' && index % 2 === 0
  return {
    id: 1000 - index,
    version,
    actor_id: 'preview-user',
    actor_name: '预览用户',
    source: index % 3 === 0 ? 'manual-save' : 'settings-update',
    config: previewConfig,
    changed_sections: [section],
    diff: [
      {
        path: `${section}.${isRepositoryDiff ? 'repos' : 'enabled'}`,
        before: isRepositoryDiff ? repositoryFixtures.slice(0, 7) : index % 2 === 0,
        after: isRepositoryDiff ? repositoryFixtures : index % 2 !== 0
      }
    ],
    previous_version_id: 999 - index,
    rollback_from_version_id: 0,
    created_at: new Date(Date.UTC(2026, 6, 10, 1, 30 - index)).toISOString()
  }
})

const json = (value: unknown, status = 200) => new Response(JSON.stringify(value), {
  status,
  headers: { 'Content-Type': 'application/json' }
})

const nativeFetch = window.fetch.bind(window)

window.fetch = async (input: RequestInfo | URL, init?: RequestInit) => {
  const url = typeof input === 'string' ? input : input instanceof URL ? input.toString() : input.url
  const method = (init?.method || 'GET').toUpperCase()

  if (url.startsWith('/api/config/test')) {
    return json({ success: true, message: '预览环境连接检查通过', details: 'Preview fixture: 200 OK' })
  }
  if (url.startsWith('/api/config/versions/') && url.endsWith('/rollback')) {
    return json({ success: true })
  }
  if (url.startsWith('/api/config/versions')) {
    return json(configVersions)
  }
  if (url === '/api/config') {
    return method === 'GET' ? json(previewConfig) : json({ success: true })
  }
  if (url.startsWith('/api/status')) {
    return json({ status: 'online', telemetry: { active_hooks: 12 } })
  }
  if (url.startsWith('/api/users') || url.startsWith('/api/groups') || url.startsWith('/api/permissions') || url.startsWith('/api/audit-logs')) {
    return json([])
  }
  if (url.startsWith('/api/authz/policies') || url.startsWith('/api/authz/audit-logs')) {
    return json([])
  }
  if (url.startsWith('/api/projects/config')) {
    return method === 'GET' ? json(projects) : json({ success: true })
  }
  if (url.startsWith('/api/agenda/summary')) {
    return json({ project_map: { IDP: '智能驾驶平台', ROC: '远程运营中心', EDS: '设备诊断服务', DMS: '数据管理服务' }, agenda_items: [] })
  }
  if (url.startsWith('/api/context/facts')) {
    if (method === 'GET') return json({ items: contextFacts })
    return json({ ...contextFacts[0], id: 3, summary: '预览环境新建资料' })
  }
  if (url.startsWith('/api/context/pack/preview')) {
    return json({
      id: 'preview-pack-48',
      summary: '选择管理台架构与配置检查流程作为需求解构上下文。',
      token_count: 150,
      budget_tokens: 1200,
      cache_key: 'settings-preview-v48',
      items: contextFacts.map((fact, index) => ({ ...fact, score: 0.92 - index * 0.08, reason: '与配置中心工作流直接相关' }))
    })
  }
  if (url.startsWith('/api/gitlab/projects')) {
    return json([
      { id: 101, name: 'ambient-core', path: 'platform/ambient-core', path_with_namespace: 'platform/ambient-core' },
      { id: 102, name: 'ambient-web', path: 'platform/ambient-web', path_with_namespace: 'platform/ambient-web' }
    ])
  }
  if (url.startsWith('/api/gitlab/webhooks')) {
    return json({ success: true, results: [] })
  }

  return nativeFetch(input, init)
}

const target = document.getElementById('settings-preview')

if (!target) {
  throw new Error('Missing settings preview mount node')
}
const previewTarget = target

function showPreviewRuntimeError(error: unknown) {
  const message = error instanceof Error ? `${error.name}: ${error.message}\n${error.stack || ''}` : String(error)
  previewTarget.textContent = message
  previewTarget.setAttribute('data-preview-error', 'true')
}

window.addEventListener('error', (event) => showPreviewRuntimeError(event.error || event.message))
window.addEventListener('unhandledrejection', (event) => showPreviewRuntimeError(event.reason))

let app
try {
  app = mount(SettingsConfigPreview, { target: previewTarget })
} catch (error) {
  showPreviewRuntimeError(error)
  throw error
}

export default app
