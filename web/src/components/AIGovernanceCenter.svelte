<script lang="ts">
  import { onDestroy, onMount, tick } from 'svelte';
  import { marked } from 'marked';
  import DOMPurify from 'dompurify';
  import Select from './shared/Select.svelte';
  import Button from './shared/Button.svelte';
  import Switch from './shared/Switch.svelte';
  import TextInput from './shared/TextInput.svelte';
  import Textarea from './shared/Textarea.svelte';
  import MarkdownWorkbench from './shared/MarkdownWorkbench.svelte';
  import CapabilityUpgradeModal from './config/CapabilityUpgradeModal.svelte';
  import AdminListFilterBar from './admin-console/AdminListFilterBar.svelte';
  import AdminDataList from './admin-console/AdminDataList.svelte';
  import type { AdminCellValue, AdminTableColumn, AdminTableRow } from '../lib/admin-console/contract';
  import { resolveGovernanceCapabilityHierarchy, filterGovernanceCapabilities, countIncludedGovernanceComponents } from '../lib/governance-capability-list';
  import { normalizeReviewRepoPolicies, type ReviewRepoPolicy } from '../lib/review-repo-policy';
  import { lockBodyScroll, unlockBodyScroll } from '../lib/modalScrollLock';
  import SolutionPromptConfig from './config/SolutionPromptConfig.svelte';
  import AIConfig from './config/AIConfig.svelte';
  import OpenSkillRegistry from './config/OpenSkillRegistry.svelte';
  import OpenCapabilitiesConfig from './config/OpenCapabilitiesConfig.svelte';
  import {
    AI_GOVERNANCE_SECTION_DEFINITIONS,
    canAccessAIGovernanceSection,
    canManageAIGovernanceSkills,
    canManageOpenCapabilities,
    canReadSolutionPublicURL,
    canWriteAIGovernanceRules,
    canWriteSolutionPublicURL,
    firstAccessibleAIGovernanceSection,
    type AIGovernanceSection
  } from '../lib/ai-governance-sections';

  export let currentUserPermissions: string[] = [];
  export let currentUserRole = 'member';
  export let activeSection: AIGovernanceSection = 'skills';
  export let onSectionChange: (section: string) => void = () => {};

  let mounted = false;
  let loadedSections = new Set<AIGovernanceSection>();
  let loadingSections = new Set<AIGovernanceSection>();
  let lastAuthorizationSignature = '';
  $: accessibleSections = AI_GOVERNANCE_SECTION_DEFINITIONS.filter((section) =>
    canAccessAIGovernanceSection(section.id, currentUserPermissions, currentUserRole)
  );
  $: canManageSkills = canManageAIGovernanceSkills(currentUserPermissions, currentUserRole);
  $: canWriteRules = canWriteAIGovernanceRules(currentUserPermissions);
  $: canReadPublicURL = canReadSolutionPublicURL(currentUserPermissions, currentUserRole);
  $: canWritePublicURL = canWriteSolutionPublicURL(currentUserPermissions, currentUserRole);
  $: canManageOpenCaps = canManageOpenCapabilities(currentUserPermissions, currentUserRole);
  $: authorizationSignature = `${currentUserRole}:${[...currentUserPermissions].sort().join(',')}`;
  $: if (authorizationSignature !== lastAuthorizationSignature) {
    lastAuthorizationSignature = authorizationSignature;
    loadedSections = new Set();
    loadingSections = new Set();
    if (!canReadPublicURL) {
      solutionPublicURL = '';
      solutionPublicURLVersion = 0;
    }
  }

  function switchSection(sec: AIGovernanceSection) {
    if (!canAccessAIGovernanceSection(sec, currentUserPermissions, currentUserRole)) return;
    activeSection = sec;
    onSectionChange(sec);
  }

  // Capability types
  interface CapabilityVersionResource {
    id: number;
    capability_version_id: number;
    resource_key: string;
    content_kind: string;
    content_hash: string;
    blob_ref: string;
    token_estimate: number;
    load_level: string;
    content: string;
    created_at: string;
  }

  interface CapabilityVersionDependency {
    id: number;
    capability_version_id: number;
    dependency_key: string;
    version_constraint: string;
    is_optional: boolean;
  }

  interface CapabilityVersionItem {
    id: number;
    capability_id: number;
    version: number;
    scope_type: string;
    scope_id: string;
    manifest_json: string;
    content_digest: string;
    signature?: string;
    validation_status: string;
    status: string;
    activated_at?: string;
    retired_at?: string;
    created_at: string;
    resources?: CapabilityVersionResource[];
    dependencies?: CapabilityVersionDependency[];
    disk_size_bytes?: number;
    disk_size_formatted?: string;
  }

  function installationMethodLabel(method?: string) {
    return ({manifest_import:'清单导入',remote_manifest:'远程清单安装',manifest_upgrade:'清单升级'} as Record<string,string>)[method || ''] || '仅注册记录';
  }

  interface InstallationEvidence {
    location: string;
    method: string;
    source: string;
    evidence_source: string;
    installed_at: string;
    artifact_sha256: string;
  }
  interface PluginRuntime {
    version: string; mode: string; default_enabled: boolean; state: string;
    state_source: string; worker_path: string; calls: number; compressed: number;
    fallbacks: number; tokens_saved: number; last_reason?: string;
  }
  interface InventoryMetadata {
    origin?: string;
    install_state?: string;
    status_source?: string;
    client_installation?: string;
    builtin_location?: string;
    installation?: InstallationEvidence;
    plugin_runtime?: PluginRuntime;
  }
  interface IncludedComponent {
    key: string;
    kind: string;
    name: string;
    description: string;
    tools?: string[];
    origin?: string;
    source_label?: string;
  }

  interface CapabilityItem extends InventoryMetadata {
    id: number;
    capability_key: string;
    kind: string;
    status: 'active' | 'disabled' | 'retired' | 'uninstalled' | 'archived';
    owner: string;
    sensitivity: string;
    description: string;
    disk_size_bytes: number;
    disk_size_formatted: string;
    bindings_count?: number;
    is_archived?: boolean;
    is_top_level_skill?: boolean;
    parent_skill_key?: string;
    included_components?: IncludedComponent[];
    latest_version: number;
    versions: CapabilityVersionItem[];
    created_at: string;
    updated_at: string;
  }

  interface CapabilityDetailData extends InventoryMetadata {
    capability: CapabilityItem;
    versions: CapabilityVersionItem[];
    disk_size_bytes: number;
    disk_size_formatted: string;
    bindings_count?: number;
    is_archived?: boolean;
    is_top_level_skill?: boolean;
    parent_skill_key?: string;
    included_components?: IncludedComponent[];
    skill_markdown?: string;
  }

  // State
  let capabilities: CapabilityItem[] = [];
  let totalDiskSizeBytes = 0;
  let totalDiskSizeFormatted = '0 B';
  let loadingCapabilities = false;
  let capabilityError = '';
  let capabilityNotice = '';
  let capabilityRequestID = 0;

  // Filter
  let searchKeyword = '';
  let filterKind = 'all';
  let inventoryScope: 'installed' | 'system' | 'registry' = 'installed';
  let checkingHeadroom = false;
  const inventoryOptions = [
    { value: 'installed', label: '已安装 Skill' },
    { value: 'system', label: '系统内置' },
    { value: 'registry', label: '注册记录（含待核验）' }
  ];
  let filterStatus = 'all';

  const kindOptions = [
    { value: 'all', label: '全部技能与独立能力' },
    { value: 'skills_only', label: '⭐ 仅看业务技能 (Skills)' },
    { value: 'skill', label: 'Skill 业务技能' },
    { value: 'plugin', label: 'Plugin 底层插件' },
    { value: 'context_provider', label: 'Context 语料源' },
    { value: 'mcp', label: 'MCP 扩展工具' },
    { value: 'policy', label: 'Policy 治理策略' }
  ];

  const statusOptions = [
    { value: 'all', label: '全状态' },
    { value: 'active', label: '已启用 (Active)' },
    { value: 'disabled', label: '已禁用 (Disabled)' },
    { value: 'archived', label: '已冷归档 (Archived)' },
    { value: 'uninstalled', label: '已卸载 (Uninstalled)' }
  ];

  function portalToConsole(node: HTMLElement) {
    const target = document.querySelector<HTMLElement>('.functional-console') || document.body;
    target.appendChild(node);
    return {
      destroy() {
        node.remove();
      }
    };
  }

  // Resolve ownership from the full registry before applying display filters.
  $: capabilityHierarchy = resolveGovernanceCapabilityHierarchy(capabilities);
  $: displayCapabilities = filterGovernanceCapabilities(capabilityHierarchy.records, {
    search: searchKeyword,
    kind: filterKind,
    scope: inventoryScope,
    status: filterStatus
  });
  $: displayedComponentCount = countIncludedGovernanceComponents(displayCapabilities);

  // Detail Drawer state
  let selectedCapability: CapabilityItem | null = null;
  let capabilityDetail: CapabilityDetailData | null = null;
  let loadingDetail = false;
  let detailDrawerOpen = false;
  let detailRequestID = 0;
  let activeVersionTab = 0;
  let drawerActiveView: 'skill_doc' | 'slices' = 'skill_doc';
  let copiedSkillDoc = false;

  function renderMarkdown(src: string): string {
    if (!src) return '';
    try {
      const rawHtml = marked.parse(String(src), { async: false, gfm: true, breaks: false }) as string;
      return DOMPurify.sanitize(rawHtml);
    } catch {
      return `<pre class="raw-fallback">${src}</pre>`;
    }
  }

  async function copySkillDoc() {
    if (!capabilityDetail?.skill_markdown) return;
    try {
      await navigator.clipboard.writeText(capabilityDetail.skill_markdown);
      copiedSkillDoc = true;
      setTimeout(() => copiedSkillDoc = false, 2500);
    } catch {
      // Fallback
    }
  }

  // Import Modal state
  let importModalOpen = false;
  let importTab: 'manifest' | 'remote' = 'manifest';
  let importManifestText = '';
  let remoteInstallURL = '';
  let remoteAutoActivate = true;
  let importing = false;
  let importError = '';

  // Upgrade Modal state
  let upgradeModalOpen = false;
  let upgradeTarget: CapabilityItem | null = null;
  let upgradeManifestText = '';
  let upgradeRemoteURL = '';
  let upgradeMode: 'manifest' | 'remote' = 'manifest';
  let upgrading = false;
  let upgradeError = '';

  // Delete / Archive Confirm Modal
  let deleteModalOpen = false;
  let deleteTarget: CapabilityItem | null = null;
  let deleteMode: 'cold_archive' | 'purge' = 'cold_archive';
  let deleting = false;
  let deleteError = '';

  // Action status state
  let togglingStatusKey = '';
  let reinstallingKey = '';

  // --- Prompts and scoped configuration state ---
  const governanceAIConfig = {
    enabled: false,
    provider: 'openai',
    base_url: '',
    endpoint_type: 'responses',
    api_token: '',
    model: '',
    project_architecture: '',
    delivery_workflow: '',
    implemented_features: '',
    estimation_guidelines: '',
    default_work_hours_per_day: 8
  };

  let solutionPublicURL = '';
  let solutionPublicURLVersion = 0;
  let solutionPublicURLLoading = false;
  let solutionPublicURLError = '';
  let contextAIReady: boolean | null = null;
  let contextAIStatus = 'unknown';
  let contextReadinessLoading = false;
  let contextReadinessError = '';

  // --- Rules Section State ---
  interface RepoPolicyRow extends AdminTableRow { repo: ReviewRepoPolicy; }
  const repoPolicyColumns: AdminTableColumn[] = [
    { key: 'name', label: '名称', width: '72%' },
    { key: 'rules', label: '规则', width: '28%' }
  ];
  let repoPolicies: ReviewRepoPolicy[] = [];
  type CommentSettings = Pick<ReviewRepoPolicy, 'sync_commits' | 'sync_mrs'>;
  type CommentSetting = keyof CommentSettings;
  let commentSettings: CommentSettings | null = null;
  let savingCommentSetting: CommentSetting | null = null;
  let savingRepoSetting: CommentSetting | null = null;
  let commentSettingsNotice = '';
  let commentSettingsError = '';
  let policySearch = '';
  let policyDrafts: Record<string, string> = {};
  let loadingPolicies = false;
  let selectedPolicyRepo = '';
  let editingPolicy: ReviewRepoPolicy | null = null;
  let savingPolicy = false;
  let policyNotice = '';
  let policyError = '';
  $: visibleRepoPolicies = repoPolicies.filter((repo) =>
    `${repo.repo_name} ${repo.project_id}`.toLowerCase().includes(policySearch.trim().toLowerCase())
  );
  $: policyRulesDirty = !!editingPolicy && editingPolicy.rules !==
    (repoPolicies.find((repo) => repo.project_id === editingPolicy?.project_id)?.rules ?? '');
  $: draftRepoIDs = new Set(repoPolicies.filter((repo) =>
    (editingPolicy?.project_id === repo.project_id ? editingPolicy.rules : policyDrafts[repo.project_id] ?? repo.rules) !== repo.rules
  ).map((repo) => repo.project_id));

  $: repoPolicyRows = visibleRepoPolicies.map((repo): RepoPolicyRow => ({
    id: repo.project_id,
    title: repo.repo_name || `仓库 #${repo.project_id}`,
    status: draftRepoIDs.has(repo.project_id) ? '未保存' : repo.rules.trim() ? '已配置' : '未配置',
    cells: { name: repo.repo_name, rules: repo.rules },
    repo
  }));

  // Manifest Sample Template
  const sampleManifest = `schema: capability-manifest/v1
id: custom.quality.auditor
kind: skill
version: 1
owner: quality-engineering
sensitivity: internal
description: "自主代码质量与合规性扫描技能，检查函数圈复杂度与规范标准"
runtime:
  min_kernel: "1.0.0"
scope:
  type: global
triggers:
  intents:
    - "code_quality_check"
provides:
  - "quality.report"
permissions:
  - "source.read"
resources:
  - key: quality-rubric
    load_level: L1
    content_kind: text
    token_estimate: 800
    content: |
      你是一个严谨的代码质量评审专家。
      请检查代码中是否存在超过50行的单函数、缺少异常捕获或硬编码密钥等问题。
budgets:
  instruction_tokens: 1500
  evidence_tokens: 4000
  tool_calls: 10
  wall_time_seconds: 120
validation:
  suite: "quality-smoke"
  required:
    - "parse-success"
activation:
  mode: human_approved
`;

  class GovernanceRequestError extends Error {
    status: number;
    code: string;

    constructor(status: number, code: string, message: string) {
      super(message);
      this.name = 'GovernanceRequestError';
      this.status = status;
      this.code = code;
    }
  }

  // API helper
  async function api<T = any>(path: string, init: RequestInit = {}): Promise<T> {
    const res = await fetch(path, init);
    const text = await res.text();
    let data: any = {};
    try {
      data = text ? JSON.parse(text) : {};
    } catch {
      data = { message: text };
    }
    if (!res.ok) {
      throw new GovernanceRequestError(
        res.status,
        typeof data.error === 'string' ? data.error : '',
        data.message || data.error || text || `HTTP ${res.status}`
      );
    }
    return data;
  }

  // --- Capability Functions ---
  async function checkHeadroom() {
    if (!canManageSkills || checkingHeadroom) return;
    checkingHeadroom = true;
    try {
      const result = await api<{runtime: PluginRuntime}>('/api/agent-runtime/system-plugins/headroom/check', {method: 'POST'});
      if (capabilityDetail) capabilityDetail = {...capabilityDetail, plugin_runtime: result.runtime};
      capabilityNotice = result.runtime.state === 'ready' ? 'Headroom 本地无损检查通过，未调用模型服务。' : 'Headroom 当前不可用，已保留原文；请查看详情中的回退原因。';
    } catch (err) { capabilityError = err instanceof Error ? err.message : '插件检查失败'; }
    finally { checkingHeadroom = false; }
  }

  async function loadCapabilities(): Promise<boolean> {
    const requestID = ++capabilityRequestID;
    loadingCapabilities = true;
    capabilityError = '';
    try {
      const resp = await api('/api/agent-runtime/capabilities');
      if (requestID !== capabilityRequestID) return false;
      capabilities = resp.capabilities || [];
      totalDiskSizeBytes = resp.total_disk_size_bytes || 0;
      totalDiskSizeFormatted = resp.total_disk_size_formatted || '0 B';
      return true;
    } catch (err: any) {
      if (requestID === capabilityRequestID) {
        capabilityError = err.message || '加载技能列表失败';
      }
      return false;
    } finally {
      if (requestID === capabilityRequestID) {
        loadingCapabilities = false;
      }
    }
  }

  async function openDetail(cap: CapabilityItem) {
    const requestID = ++detailRequestID;
    selectedCapability = cap;
    detailDrawerOpen = true;
    lockBodyScroll();
    loadingDetail = true;
    capabilityDetail = null;
    activeVersionTab = 0;
    drawerActiveView = 'skill_doc';
    copiedSkillDoc = false;
    try {
      const detail = await api<CapabilityDetailData>(`/api/agent-runtime/capabilities/${cap.capability_key}`);
      if (
        requestID !== detailRequestID ||
        !detailDrawerOpen ||
        selectedCapability?.capability_key !== cap.capability_key
      ) return;
      capabilityDetail = detail;
    } catch (err: any) {
      if (requestID === detailRequestID) {
        capabilityError = err.message || '加载技能详情失败';
      }
    } finally {
      if (requestID === detailRequestID) {
        loadingDetail = false;
      }
    }
  }

  function closeDetailDrawer() {
    detailRequestID += 1;
    detailDrawerOpen = false;
    loadingDetail = false;
    capabilityDetail = null;
    unlockBodyScroll();
  }

  function handleWindowKeydown(e: KeyboardEvent) {
    if (e.key !== 'Escape' || upgradeModalOpen) return;
    if (deleteModalOpen) {
      if (!deleting) deleteModalOpen = false;
    } else if (upgradeModalOpen) {
      if (!upgrading) upgradeModalOpen = false;
    } else if (importModalOpen) {
      if (!importing) importModalOpen = false;
    } else if (detailDrawerOpen) {
      closeDetailDrawer();
    } else return;
    e.preventDefault();
  }

  function manageDialogFocus(node: HTMLElement) {
    const returnFocus = document.activeElement instanceof HTMLElement ? document.activeElement : null;
    const focusable = () => Array.from(node.querySelectorAll<HTMLElement>(
      'a[href], button:not([disabled]), input:not([disabled]), textarea:not([disabled]), select:not([disabled]), [tabindex]:not([tabindex="-1"])'
    )).filter(element => element.getClientRects().length > 0);
    void tick().then(() => {
      if (node.isConnected) (focusable()[0] || node).focus();
    });
    const trapFocus = (event: KeyboardEvent) => {
      if (event.key !== 'Tab') return;
      const elements = focusable();
      const first = elements[0];
      const last = elements.at(-1);
      if (!first || !last) {
        event.preventDefault();
        node.focus();
      } else if (event.shiftKey && document.activeElement === first) {
        event.preventDefault();
        last.focus();
      } else if (!event.shiftKey && document.activeElement === last) {
        event.preventDefault();
        first.focus();
      }
    };
    node.addEventListener('keydown', trapFocus);
    return { destroy() {
      node.removeEventListener('keydown', trapFocus);
      void tick().then(() => { if (returnFocus?.isConnected) returnFocus.focus(); });
    } };
  }

  onDestroy(() => {
    if (detailDrawerOpen) {
      unlockBodyScroll();
    }
  });

  function requireSkillMutationAccess() {
    if (!canManageSkills) throw new Error('需要全局超级管理员和 solution_prompt:manage 权限');
  }

  async function toggleCapabilityStatus(cap: CapabilityItem) {
    try {
      requireSkillMutationAccess();
    } catch (err: any) {
      capabilityError = err.message;
      return;
    }
    const nextStatus = cap.status === 'active' ? 'disabled' : 'active';
    togglingStatusKey = cap.capability_key;
    capabilityNotice = '';
    capabilityError = '';
    try {
      await api(`/api/agent-runtime/capabilities/${cap.capability_key}/status`, {
        method: 'PATCH',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ status: nextStatus })
      });
      cap.status = nextStatus;
      capabilities = [...capabilities];
      capabilityNotice = `技能 ${cap.capability_key} 已${nextStatus === 'active' ? '启用' : '禁用'}`;
      if (selectedCapability?.capability_key === cap.capability_key) {
        selectedCapability.status = nextStatus;
      }
    } catch (err: any) {
      capabilityError = err.message || '切换技能状态失败';
    } finally {
      togglingStatusKey = '';
    }
  }

  function openDeleteConfirm(cap: CapabilityItem) {
    if (!canManageSkills) return;
    deleteTarget = cap;
    deleteError = '';
    // 如果存在历史运行绑定，默认推荐冷归档；否则默认彻底清除
    deleteMode = (cap.bindings_count && cap.bindings_count > 0) ? 'cold_archive' : 'purge';
    deleteModalOpen = true;
  }

  async function confirmUninstall() {
    if (!canManageSkills || !deleteTarget) return;
    deleting = true;
    deleteError = '';
    try {
      await api(`/api/agent-runtime/capabilities/${deleteTarget.capability_key}?mode=${deleteMode}`, {
        method: 'DELETE'
      });
      deleteModalOpen = false;
      if (deleteMode === 'cold_archive') {
        capabilityNotice = `技能 ${deleteTarget.capability_key} 已成功冷归档，已保留运行审计凭据与重放能力。`;
      } else {
        capabilityNotice = `技能 ${deleteTarget.capability_key} 已成功卸载并清理注册资源大小，已保留安装记录供回溯与恢复。`;
      }
      if (detailDrawerOpen && selectedCapability?.capability_key === deleteTarget.capability_key) {
        closeDetailDrawer();
      }
      await loadCapabilities();
    } catch (err: any) {
      deleteError = err.message || '操作失败';
    } finally {
      deleting = false;
    }
  }

  async function handleReinstall(cap: CapabilityItem) {
    if (!canManageSkills) return;
    reinstallingKey = cap.capability_key;
    capabilityError = '';
    capabilityNotice = '';
    try {
      await api(`/api/agent-runtime/capabilities/${cap.capability_key}/reinstall`, {
        method: 'POST'
      });
      capabilityNotice = `技能 ${cap.capability_key} 已从历史安装记录成功重新安装并激活！`;
      await loadCapabilities();
      if (detailDrawerOpen && selectedCapability?.capability_key === cap.capability_key) {
        await openDetail(cap);
      }
    } catch (err: any) {
      capabilityError = err.message || '重新安装失败';
    } finally {
      reinstallingKey = '';
    }
  }

  function openImportModal() {
    if (!canManageSkills) return;
    importManifestText = sampleManifest;
    remoteInstallURL = '';
    remoteAutoActivate = true;
    importError = '';
    importTab = 'manifest';
    importModalOpen = true;
  }

  async function submitImport() {
    if (!canManageSkills) return;
    importing = true;
    importError = '';
    try {
      if (importTab === 'manifest') {
        if (!importManifestText.trim()) throw new Error('请输入 Capability Manifest 内容');
        await api('/api/agent-runtime/capabilities', {
          method: 'POST',
          headers: { 'Content-Type': 'application/yaml' },
          body: importManifestText.trim()
        });
        capabilityNotice = '新技能已成功通过通用 Manifest 规范注册！';
      } else {
        if (!remoteInstallURL.trim()) throw new Error('请输入远程 Manifest URL');
        await api('/api/agent-runtime/capabilities/remote-install', {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({ url: remoteInstallURL.trim(), auto_activate: remoteAutoActivate })
        });
        capabilityNotice = '技能已成功从远程仓库/市场下载、校验并安装入库！';
      }
      importModalOpen = false;
      await loadCapabilities();
    } catch (err: any) {
      importError = err.message || '安装/导入技能失败';
    } finally {
      importing = false;
    }
  }

  function openUpgradeModal(cap: CapabilityItem) {
    if (!canManageSkills) return;
    upgradeTarget = cap;
    upgradeManifestText = '';
    upgradeRemoteURL = '';
    upgradeError = '';
    upgradeMode = 'manifest';
    upgradeModalOpen = true;
  }

  async function submitUpgrade() {
    if (!canManageSkills || !upgradeTarget || upgrading) return;
    upgrading = true;
    upgradeError = '';
    try {
      let bodyData: any;
      let headers: Record<string, string> = {};
      if (upgradeMode === 'manifest') {
        if (!upgradeManifestText.trim()) throw new Error('请输入升级后的 Manifest 内容');
        headers['Content-Type'] = 'application/yaml';
        bodyData = upgradeManifestText.trim();
      } else {
        if (!upgradeRemoteURL.trim()) throw new Error('请输入远程升级 Manifest URL');
        headers['Content-Type'] = 'application/json';
        bodyData = JSON.stringify({ url: upgradeRemoteURL.trim() });
      }
      await api(`/api/agent-runtime/capabilities/${upgradeTarget.capability_key}/upgrade`, {
        method: 'POST',
        headers,
        body: bodyData
      });
      capabilityNotice = `技能 ${upgradeTarget.capability_key} 已成功升级至新版本并自动生效！旧版本已安全退役。`;
      upgradeModalOpen = false;
      await loadCapabilities();
      if (detailDrawerOpen && selectedCapability?.capability_key === upgradeTarget.capability_key) {
        await openDetail(upgradeTarget);
      }
    } catch (err: any) {
      upgradeError = err.message || '技能升级失败';
    } finally {
      upgrading = false;
    }
  }

  async function activateSpecificVersion(capKey: string, version: number, scopeType: string) {
    if (!canManageSkills) return;
    capabilityError = '';
    try {
      await api(`/api/agent-runtime/capabilities/${capKey}/activate`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ version, scope_type: scopeType })
      });
      capabilityNotice = `技能 ${capKey} 已成功切换并激活 v${version}！`;
      if (selectedCapability) {
        await openDetail(selectedCapability);
      }
      await loadCapabilities();
    } catch (err: any) {
      capabilityError = err.message || '激活版本失败';
    }
  }

  // --- Prompts and scoped configuration functions ---
  async function loadSolutionPublicURL(): Promise<boolean> {
    if (!canReadPublicURL) return true;
    solutionPublicURLLoading = true;
    solutionPublicURLError = '';
    try {
      const response = await api<{ public_url: string; version: number }>('/api/solution-prompts/public-url');
      solutionPublicURL = response.public_url || '';
      solutionPublicURLVersion = response.version;
      return true;
    } catch (err: any) {
      solutionPublicURLError = err.message || '方案公开地址加载失败';
      return false;
    } finally {
      solutionPublicURLLoading = false;
    }
  }

  async function saveSolutionPublicURL(value: string, expectedVersion: number) {
    if (!canWritePublicURL) {
      throw new Error('需要 config:read 与 config:write 权限才能保存方案公开地址');
    }
    try {
      const response = await api<{ public_url: string; version: number }>('/api/solution-prompts/public-url', {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ public_url: value, expected_version: expectedVersion })
      });
      solutionPublicURL = response.public_url || '';
      solutionPublicURLVersion = response.version;
      solutionPublicURLError = '';
    } catch (err: any) {
      if (err instanceof GovernanceRequestError && err.status === 409) {
        const reloaded = await loadSolutionPublicURL();
        throw new Error(
          reloaded
            ? '配置版本已更新，已重新读取当前公开地址。你的输入仍保留，请确认后再次保存。'
            : '配置版本已更新，但重新读取当前公开地址失败。请稍后重试。'
        );
      }
      throw err;
    }
  }

  async function loadContextReadiness(): Promise<boolean> {
    contextReadinessLoading = true;
    contextReadinessError = '';
    try {
      const response = await api<{ ready: boolean; status: string }>('/api/ai/context-readiness');
      contextAIReady = response.ready;
      contextAIStatus = response.status || (response.ready ? 'ready' : 'unknown');
      return true;
    } catch (err: any) {
      contextAIReady = null;
      contextAIStatus = 'unknown';
      contextReadinessError = err.message || 'AI 就绪状态加载失败';
      return false;
    } finally {
      contextReadinessLoading = false;
    }
  }

  // --- Rules Functions ---
  async function loadRepoPolicies(): Promise<boolean> {
    loadingPolicies = true;
    policyError = '';
    try {
      const [repos, settings] = await Promise.all([
        api('/api/code-reviews/repos'),
        api('/api/code-reviews/comment-settings')
      ]);
      if (editingPolicy && policyRulesDirty) policyDrafts = { ...policyDrafts, [editingPolicy.project_id]: editingPolicy.rules };
      repoPolicies = normalizeReviewRepoPolicies(repos.repos);
      commentSettings = readCommentSettings(settings);
      const selected = repoPolicies.find((repo) => repo.project_id === selectedPolicyRepo) ?? repoPolicies[0];
      selectedPolicyRepo = selected?.project_id ?? '';
      editingPolicy = selected ? { ...selected, rules: policyDrafts[selected.project_id] ?? selected.rules } : null;
      return true;
    } catch (err: any) {
      policyError = err.message || '加载评审工程规则失败';
      return false;
    } finally {
      loadingPolicies = false;
    }
  }

  function selectRepoPolicy(repo: ReviewRepoPolicy) {
    if (savingPolicy || savingRepoSetting) return;
    if (editingPolicy) policyDrafts = { ...policyDrafts, [editingPolicy.project_id]: editingPolicy.rules };
    policyNotice = '';
    policyError = '';
    selectedPolicyRepo = repo.project_id;
    editingPolicy = { ...repo, rules: policyDrafts[repo.project_id] ?? repo.rules };
  }

  function readCommentSettings(value: any): CommentSettings {
    if (typeof value?.sync_commits !== 'boolean' || typeof value?.sync_mrs !== 'boolean') {
      throw new Error('评论同步设置响应不完整，请重试。');
    }
    return { sync_commits: value.sync_commits, sync_mrs: value.sync_mrs };
  }

  async function saveCommentSetting(field: CommentSetting, checked: boolean) {
    if (!canWriteRules || !commentSettings || savingCommentSetting) return;
    savingCommentSetting = field;
    commentSettingsNotice = '';
    commentSettingsError = '';
    try {
      commentSettings = readCommentSettings(await api('/api/code-reviews/comment-settings', {
        method: 'PATCH', headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ [field]: checked })
      }));
      commentSettingsNotice = `全局 ${field === 'sync_mrs' ? 'MR' : 'Commit'} 评论${checked ? '已允许' : '已暂停'}。`;
    } catch (err: any) {
      commentSettingsError = err.message || '保存全局评论设置失败';
    } finally {
      savingCommentSetting = null;
    }
  }

  async function saveRepoSetting(field: CommentSetting, checked: boolean) {
    if (!canWriteRules || !editingPolicy || savingRepoSetting || savingPolicy) return;
    const projectID = editingPolicy.project_id;
    savingRepoSetting = field;
    policyNotice = '';
    policyError = '';
    try {
      await api('/api/code-reviews/policy', {
        method: 'PUT', headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ project_id: projectID, [field]: checked })
      });
      repoPolicies = repoPolicies.map((repo) => repo.project_id === projectID ? { ...repo, [field]: checked } : repo);
      if (editingPolicy?.project_id === projectID) editingPolicy = { ...editingPolicy, [field]: checked };
      policyNotice = `仓库 ${field === 'sync_mrs' ? 'MR' : 'Commit'} 评论设置已保存。`;
    } catch (err: any) {
      policyError = err.message || '保存仓库评论设置失败';
    } finally {
      savingRepoSetting = null;
    }
  }

  async function saveRepoPolicy() {
    if (!canWriteRules || !editingPolicy || savingPolicy || savingRepoSetting) return;
    const savedPolicy = { ...editingPolicy };
    savingPolicy = true;
    policyNotice = '';
    policyError = '';
    try {
      await api('/api/code-reviews/policy', {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          project_id: savedPolicy.project_id,
          rules: savedPolicy.rules
        })
      });
      repoPolicies = repoPolicies.map((repo) => repo.project_id === savedPolicy.project_id ? { ...repo, rules: savedPolicy.rules } : repo);
      policyDrafts = { ...policyDrafts, [savedPolicy.project_id]: savedPolicy.rules };
      policyNotice = '规则已保存，将用于后续代码评审。';
    } catch (err: any) {
      policyError = err.message || '更新规则失败';
    } finally {
      savingPolicy = false;
    }
  }

  // File drag & upload helper for Manifest
  function handleFileUpload(e: Event) {
    const target = e.target as HTMLInputElement;
    if (target.files && target.files[0]) {
      const file = target.files[0];
      const reader = new FileReader();
      reader.onload = ev => {
        importManifestText = ev.target?.result as string;
      };
      reader.readAsText(file);
    }
  }

  async function loadAuthorizedSection(section: AIGovernanceSection) {
    if (
      !canAccessAIGovernanceSection(section, currentUserPermissions, currentUserRole) ||
      loadedSections.has(section) ||
      loadingSections.has(section)
    ) return;

    loadingSections = new Set([...loadingSections, section]);
    let loaded = false;
    try {
      if (section === 'skills') loaded = await loadCapabilities();
      if (section === 'prompts') loaded = await loadSolutionPublicURL();
      if (section === 'rules') loaded = await loadRepoPolicies();
      if (section === 'context') loaded = await loadContextReadiness();
      if (section === 'open_capabilities' || section === 'skill_registry') loaded = true;
      if (loaded) loadedSections = new Set([...loadedSections, section]);
    } finally {
      loadingSections = new Set([...loadingSections].filter((item) => item !== section));
    }
  }

  function retrySection(section: AIGovernanceSection) {
    loadedSections = new Set([...loadedSections].filter((item) => item !== section));
    void loadAuthorizedSection(section);
  }

  function handleSectionTabKeydown(event: KeyboardEvent, index: number) {
    if (!['ArrowLeft', 'ArrowRight', 'Home', 'End'].includes(event.key)) return;
    event.preventDefault();
    const lastIndex = accessibleSections.length - 1;
    const nextIndex = event.key === 'Home'
      ? 0
      : event.key === 'End'
        ? lastIndex
        : event.key === 'ArrowRight'
          ? (index + 1) % accessibleSections.length
          : (index - 1 + accessibleSections.length) % accessibleSections.length;
    const nextSection = accessibleSections[nextIndex];
    if (!nextSection) return;
    switchSection(nextSection.id);
    const buttons = Array.from(
      (event.currentTarget as HTMLElement).parentElement?.querySelectorAll<HTMLButtonElement>('[role="tab"]') || []
    );
    buttons[nextIndex]?.focus();
  }

  $: resolvedActiveSection = canAccessAIGovernanceSection(activeSection, currentUserPermissions, currentUserRole)
    ? activeSection
    : firstAccessibleAIGovernanceSection(currentUserPermissions, currentUserRole);
  $: if (mounted && resolvedActiveSection && resolvedActiveSection !== activeSection) {
    onSectionChange(resolvedActiveSection);
  }
  $: if (mounted && resolvedActiveSection && authorizationSignature) {
    void loadAuthorizedSection(resolvedActiveSection);
  }

  onMount(() => {
    mounted = true;
  });
</script>

<svelte:window on:keydown={handleWindowKeydown} />

<div class="gov-workbench" data-layout-contract="admin-viewport-workbench/v1">
  <!-- Navigation owns module selection; each pane owns its content and actions. -->
  <header class="gov-header">
    {#if accessibleSections.length > 0}
      <nav class="gov-nav-tabs" role="tablist" aria-label="AI 治理分区">
        {#each accessibleSections as section, index}
          <button
            id={`ai-governance-tab-${section.id}`}
            type="button"
            role="tab"
            aria-selected={resolvedActiveSection === section.id}
            aria-controls={`ai-governance-panel-${section.id}`}
            tabindex={resolvedActiveSection === section.id ? 0 : -1}
            class="gov-tab"
            class:active={resolvedActiveSection === section.id}
            on:click={() => switchSection(section.id)}
            on:keydown={(event) => handleSectionTabKeydown(event, index)}
          >
            <span class="wa-icon tab-icon" class:icon-sparkles={section.id === 'skills'} class:icon-checklist={section.id === 'prompts' || section.id === 'rules'} class:icon-network={section.id === 'open_capabilities'} class:icon-library={section.id === 'context' || section.id === 'skill_registry'} aria-hidden="true"></span>
            {section.label}
            {#if section.id === 'skills'}<span class="tab-count">{capabilities.filter(c => c.kind === 'skill' && c.origin === 'installed' && c.install_state === 'installed').length}</span>{/if}
          </button>
        {/each}
      </nav>
    {/if}
  </header>

  {#if accessibleSections.length === 0}
    <section class="gov-empty-state gov-no-access" role="status" aria-labelledby="ai-governance-no-access-title">
      <div class="empty-icon" aria-hidden="true">🔒</div>
      <h2 id="ai-governance-no-access-title">暂无可访问的 AI 治理分区</h2>
      <p>当前账号没有技能、提示词、规则或上下文分区的读取权限。请联系安全管理员分配对应权限。</p>
    </section>
  {/if}

  <!-- Global Alerts -->
  {#if resolvedActiveSection === 'skills' && capabilityNotice}
    <div class="gov-notice" role="status">
      <span>{capabilityNotice}</span>
      <button type="button" class="close-btn" aria-label="关闭技能操作提示" on:click={() => capabilityNotice = ''}>×</button>
    </div>
  {/if}
  {#if resolvedActiveSection === 'skills' && capabilityError}
    <div class="gov-notice error" role="alert">
      <span>{capabilityError}</span>
      <div class="notice-actions">
        <button type="button" class="btn btn-sm btn-ghost" on:click={() => retrySection('skills')}>重试</button>
        <button type="button" class="close-btn" aria-label="关闭技能错误提示" on:click={() => capabilityError = ''}>×</button>
      </div>
    </div>
  {/if}

  <!-- TAB 1: 技能治理中心 (Skills) -->
  {#if resolvedActiveSection === 'skills'}
    <section
      id="ai-governance-panel-skills"
      class="gov-pane"
      role="tabpanel"
      aria-labelledby="ai-governance-tab-skills"
      aria-busy={loadingCapabilities}
    >
      <div class="gov-title-row">
        <div class="gov-title-group">
          <h2>技能治理中心</h2>
          <p class="gov-subtitle">按安装回执查看 Skill，系统内置能力与注册记录分别治理。</p>
        </div>
      {#if resolvedActiveSection === 'skills'}
        <div class="gov-header-actions">
          {#if canManageSkills}
            <button type="button" class="btn btn-secondary" on:click={openImportModal}>
              <svg width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" aria-hidden="true">
                <path d="M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4M7 10l5 5 5-5M12 15V3"/>
              </svg>
              导入 / 远程安装技能
            </button>
          {/if}
          <button type="button" class="btn btn-ghost icon-action" on:click={loadCapabilities} aria-label="刷新技能与磁盘占用" title="刷新技能与磁盘占用">
            <svg width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" class:spin={loadingCapabilities} aria-hidden="true">
              <path d="M23 4v6h-6M1 20v-6h6M3.51 9a9 9 0 0 1 14.85-3.36L23 10M1 14l4.64 4.36A9 9 0 0 0 20.49 15"/>
            </svg>
          </button>
        </div>
      {/if}
      </div>
      <div class="gov-metrics-strip" aria-label="当前可治理能力摘要">
        <div class="gov-metric-item">
          <span class="gov-metric-label">{inventoryScope === 'installed' ? '已核验安装 Skill' : inventoryScope === 'system' ? '系统内置能力' : '注册记录'}</span>
          <div class="gov-metric-val"><strong>{capabilityError && capabilities.length === 0 ? '—' : displayCapabilities.length}</strong><span class="gov-metric-sub">{capabilityError && capabilities.length === 0 ? '数据暂不可用' : `项 · ${displayCapabilities.filter(c => c.status === 'active').length} 项已启用`}</span></div>
        </div>
        <div class="gov-metric-divider"></div>
        <div class="gov-metric-item">
          <span class="gov-metric-label">所属内置组件</span>
          <div class="gov-metric-val"><strong>{displayedComponentCount}</strong><span class="gov-metric-sub">项 · 由所属技能统一治理</span></div>
        </div>
        <div class="gov-metric-divider"></div>
        <div class="gov-metric-item">
          <span class="gov-metric-label">完整注册表资源大小</span>
          <div class="gov-metric-val"><strong class="highlight-cyan">{totalDiskSizeFormatted}</strong><span class="gov-metric-sub">{totalDiskSizeBytes.toLocaleString()} 字节</span></div>
        </div>
      </div>
      {#if !canManageSkills}
        <div class="gov-notice" role="status">技能清单为只读。启用、升级、导入和卸载需要全局超级管理员与 solution_prompt:manage 权限。</div>
      {/if}

      {#if capabilityHierarchy.orphans.length > 0}
        <div class="gov-notice" role="status">
          <span>检测到 {capabilityHierarchy.orphans.length} 项组件归属异常：{capabilityHierarchy.orphans.map(component => `${component.key} → ${component.parentKeys.join('、')}`).join('；')}。所属技能不在注册表中，暂不提供独立生命周期操作。</span>
        </div>
      {/if}
      <div class="inventory-note">{inventoryScope === 'installed' ? '仅展示服务端安装回执与文件校验一致的 Skill。Codex / Claude 客户端安装状态未上报。' : inventoryScope === 'system' ? '随系统版本提供，不计入用户安装。开放能力组件在 well-ambient 主技能详情中查看。' : '注册记录用于审计；没有安装回执或文件核验不通过时，不视为已安装。'}</div>
      <!-- Filter Bar -->
      <AdminListFilterBar label="技能筛选">
        {#snippet leading()}
        <div class="search-box">
          <svg width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
            <circle cx="11" cy="11" r="8"/><path d="m21 21-4.3-4.3"/>
          </svg>
          <input
            type="text"
            aria-label="搜索技能与内置组件"
            placeholder="搜索技能、内置组件、负责人或描述…"
            bind:value={searchKeyword}
          />
        </div>

        {/snippet}
        {#snippet controls()}
        <div class="filter-controls">
          <div class="filter-field"><span class="filter-field-label">查看:</span><div class="filter-select-wrap"><Select id="gov-inventory-scope" ariaLabel="查看范围" value={inventoryScope} options={inventoryOptions} searchable={false} compact shadowless on:change={(e) => { inventoryScope = e.detail as typeof inventoryScope; filterKind = 'all'; }} /></div></div>
          {#if inventoryScope !== 'installed'}
          <div class="filter-field">
            <span class="filter-field-label">类别:</span>
            <div class="filter-select-wrap">
              <Select
                id="gov-filter-kind"
                ariaLabel="类别"
                value={filterKind}
                options={kindOptions}
                searchable={false}
                compact
                shadowless
                on:change={(e) => {
                  filterKind = e.detail;
                }}
              />
            </div>
          </div>
          {/if}

          <div class="filter-field">
            <span class="filter-field-label">状态:</span>
            <div class="filter-select-wrap">
              <Select
                id="gov-filter-status"
                ariaLabel="状态"
                value={filterStatus}
                options={statusOptions}
                searchable={false}
                compact
                shadowless
                on:change={(e) => {
                  filterStatus = e.detail;
                }}
              />
            </div>
          </div>
        </div>
        {/snippet}
      </AdminListFilterBar>

      <!-- Skills Table / Card List -->
      {#if loadingCapabilities && capabilities.length === 0}
        <div class="gov-loading-state" role="status" aria-live="polite">
          <div class="spinner" aria-hidden="true"></div>
          <span>正在检索微内核能力注册表与磁盘统计...</span>
        </div>
      {:else if capabilityError && capabilities.length === 0}
        <div class="gov-empty-state" role="status"><h3>当前无法核验安装记录</h3><p>读取失败不代表没有安装。请重试获取服务端回执与文件核验结果。</p></div>
      {:else if displayCapabilities.length === 0}
        <div class="gov-empty-state">
          <div class="empty-icon">📦</div>
          <h3>{inventoryScope === 'installed' ? '暂无已核验安装的 Skill' : '没有匹配的能力记录'}</h3>
          <p>{inventoryScope === 'installed' ? '安装回执与文件校验一致后，Skill 才会显示在此处。内置能力可切换到“系统内置”查看。' : '当前筛选条件下没有匹配记录。'}</p>
          {#if canManageSkills}<button type="button" class="btn btn-primary" on:click={openImportModal}>立即导入技能</button>{/if}
        </div>
      {:else}
        <!-- svelte-ignore a11y_no_noninteractive_tabindex (Keyboard users scroll this named table region.) -->
        <div class="gov-table-container" role="region" aria-label="技能列表，可横向滚动" tabindex="0" data-surface-contract="admin-data-list-surface/v1">
          <table class="gov-table">
            <colgroup>
              <col class="cap-column-identity" />
            <col class="cap-column-kind" />
              <col class="cap-column-status" />
              <col class="cap-column-version" />
              <col class="cap-column-bindings" />
              <col class="cap-column-disk" />
              <col class="cap-column-description" />
              <col class="cap-column-actions" />
            </colgroup>
            <thead>
              <tr>
                <th>名称</th>
                <th>类别</th>
                <th>状态</th>
                <th>版本</th>
                <th>关联运行</th>
                <th>资源大小</th>
                <th>责任方 / 描述</th>
                <th class="cap-actions-heading">生命周期操作</th>
              </tr>
            </thead>
            <tbody>
              {#each displayCapabilities as cap (cap.id)}
                <tr class:row-uninstalled={cap.status === 'uninstalled'} class:row-archived={cap.status === 'archived'}>
                  <!-- Key & Title with Hierarchy -->
                  <td>
                    <div class="cap-identity">
                      <div class="cap-title-row">
                        <strong class="cap-key">{cap.capability_key}</strong>
                        {#if cap.origin === 'system_builtin'}<span class="wa-admin-pill">系统内置</span>{/if}
                        {#if cap.sensitivity === 'restricted'}
                          <span class="badge badge-restricted">安全限制</span>
                        {/if}
                      </div>


                      {#if cap.installation}
                        <div class="cap-install-meta"><span>安装位置 <code title={cap.installation.location}>{cap.installation.location}</code></span><span>{cap.installation.method === 'remote_manifest' ? '远程安装' : '清单导入'} · {cap.install_state === 'installed' ? '文件已核验' : '安装文件待核验'}</span><span>状态来源：{cap.status_source}</span></div>
                      {:else if cap.origin !== 'system_builtin'}<div class="cap-install-meta">仅有注册记录，尚无已核验安装回执。</div>{/if}
                      <!-- Included Components (Sub-plugins / Providers) -->
                      {#if cap.included_components && cap.included_components.length > 0}
                        <div class="cap-included-block" title="本技能内聚包含的能力切片与插件">
                          <span class="inc-title">包含组件:</span>
                          {#each cap.included_components as comp}
                            <span class="inc-badge {comp.kind}" title="{comp.description}">
                              {comp.kind === 'plugin' ? '🧩' : '📚'} {comp.key}
                            </span>
                          {/each}
                        </div>
                      {/if}
                    </div>
                  </td>


                  <td><span class="cap-kind-badge {cap.kind}">{cap.kind}</span></td>

                  <!-- Switch Status -->
                  <td>
                    {#if cap.status === 'uninstalled'}
                      <span class="status-tag uninstalled" title="可通过重新安装恢复">已卸载</span>
                    {:else if cap.status === 'archived'}
                      <span class="status-tag archived" title="保留历史审计，可重新安装">已冷归档</span>
                    {:else}
                      <div class="switch-container">
                        <button
                          type="button"
                          role="switch"
                          aria-label={`切换 ${cap.capability_key} 启用状态`}
                          aria-checked={cap.status === 'active'}
                          class="gov-switch"
                          class:checked={cap.status === 'active'}
                          disabled={!canManageSkills || togglingStatusKey === cap.capability_key}
                          on:click={() => toggleCapabilityStatus(cap)}
                          title={!canManageSkills ? '需要全局超级管理员与 solution_prompt:manage 权限' : cap.status === 'active' ? '点击禁用该技能' : '点击启用该技能'}
                        >
                          <span class="gov-switch-handle"></span>
                        </button>
                        <span class="switch-text">{cap.status === 'active' ? '已启用' : '已禁用'}</span>
                      </div>
                    {/if}
                  </td>

                  <!-- Version -->
                  <td>
                    <span class="ver-pill">v{cap.latest_version || 1}</span>
                  </td>

                  <!-- Run Bindings -->
                  <td>
                    <div class="bindings-col">
                      <span class="bindings-badge" class:has-bindings={(cap.bindings_count || 0) > 0}>
                        {cap.bindings_count || 0} 次
                      </span>
                    </div>
                  </td>

                  <!-- Disk Usage -->
                  <td>
                    <div class="disk-usage-col">
                      <span class="disk-text">{cap.disk_size_formatted || '0 B'}</span>
                    </div>
                  </td>

                  <!-- Description -->
                  <td>
                    <div class="cap-desc-block">
                      <span class="cap-owner" title={cap.owner || 'system'}>{cap.owner || 'system'}</span>
                      <p class="cap-desc" title={cap.description}>{cap.description || '暂无详细描述'}</p>
                    </div>
                  </td>

                  <!-- Actions (Strict No-Wrap) -->
                  <td style="text-align: right; white-space: nowrap;">
                    <div class="action-btn-group">
                      <button type="button" class="btn btn-sm btn-ghost" title="查看详情与 SKILL.md" aria-label={`查看 ${cap.capability_key} 详情与 SKILL.md`} on:click={() => openDetail(cap)}>
                        详情
                      </button>

                      {#if canManageSkills && cap.origin !== 'system_builtin'}
                        {#if cap.status === 'uninstalled' || cap.status === 'archived'}
                          <button
                            type="button"
                            class="btn btn-sm btn-primary"
                            aria-label={`重新安装 ${cap.capability_key}`}
                            disabled={reinstallingKey === cap.capability_key}
                            on:click={() => handleReinstall(cap)}
                          >
                            {reinstallingKey === cap.capability_key ? '恢复中…' : '重新安装'}
                          </button>
                        {:else}
                          <button type="button" class="btn btn-sm btn-ghost" aria-label={`升级 ${cap.capability_key}`} on:click={() => openUpgradeModal(cap)}>
                            升级
                          </button>
                          <button type="button" class="btn btn-sm btn-danger-ghost" aria-label={`卸载 ${cap.capability_key}`} on:click={() => openDeleteConfirm(cap)}>
                            卸载
                          </button>
                        {/if}
                      {/if}
                    </div>
                  </td>
                </tr>
              {/each}
            </tbody>
          </table>
        </div>
      {/if}
    </section>
  {/if}

  <!-- TAB 2: 提示词管理 (Prompts) -->
  {#if resolvedActiveSection === 'prompts'}
    <section
      id="ai-governance-panel-prompts"
      class="gov-pane"
      role="tabpanel"
      aria-labelledby="ai-governance-tab-prompts"
      aria-busy={solutionPublicURLLoading}
    >
      {#if solutionPublicURLError}
        <div class="gov-notice error" role="alert">
          <span>{solutionPublicURLError}</span>
          <button type="button" class="btn btn-sm btn-ghost" on:click={() => retrySection('prompts')}>重试公开地址</button>
        </div>
      {/if}
      <SolutionPromptConfig
        publicURL={solutionPublicURL}
        publicURLVersion={solutionPublicURLVersion}
        publicURLLoading={solutionPublicURLLoading}
        {canReadPublicURL}
        {canWritePublicURL}
        onSavePublicURL={saveSolutionPublicURL}
      />
    </section>
  {/if}

    {#snippet renderRepoPolicyCell(row: RepoPolicyRow, column: AdminTableColumn, _value: AdminCellValue)}
      {#if column.key === 'name'}
        <button type="button" class="repo-row-select" aria-pressed={selectedPolicyRepo === row.id}
          disabled={savingPolicy || !!savingRepoSetting} aria-label={`选择 ${row.title} 的规则`}
          title={row.title} on:click={() => selectRepoPolicy(row.repo)}>
          <span class="repo-name">{row.title}</span>
          <span class="repo-meta">项目 ID {row.id}</span>
        </button>
      {:else if column.key === 'rules'}
        <span class="repo-rule-state" class:is-draft={draftRepoIDs.has(row.id)}>{row.status}</span>
      {/if}
    {/snippet}
    {#snippet renderRepoPolicyEmpty()}
      <div class="rules-empty">{repoPolicies.length === 0 ? '暂无已配置的 GitLab 仓库' : '没有匹配的仓库'}</div>
    {/snippet}

    <!-- TAB 3: 规则与工程标准 (Rules) -->
  {#if resolvedActiveSection === 'rules'}
    <section id="ai-governance-panel-rules" class="gov-pane rules-pane" role="tabpanel"
      aria-labelledby="ai-governance-tab-rules" aria-busy={loadingPolicies}>
      {#if !canWriteRules}
        <div class="gov-notice" role="status">当前为只读模式。保存规则需要 config:write 权限。</div>
      {/if}
      {#if policyError}
        <div class="gov-notice error" role="alert">
          <span>{policyError}</span>
          {#if !editingPolicy}<Button variant="secondary" size="small" on:click={() => retrySection('rules')}>重试</Button>{/if}
        </div>
      {/if}
      <div class="rules-workbench">
        <header class="rules-context-header">
          <div class="rules-heading"><h2>规则与标准</h2><p>管理仓库工程规则与评审评论。</p></div>
          <section class="global-comment-settings" aria-label="全局评论设置">
            <div class="comment-switch-row">
              <Switch optimistic={false} id="global-policy-sync-commits" label="全局 Commit 评论" expandedHitArea
                checked={commentSettings?.sync_commits ?? false} saving={savingCommentSetting === 'sync_commits'}
                disabled={!canWriteRules || !commentSettings || !!savingCommentSetting || loadingPolicies}
                on:change={(event) => saveCommentSetting('sync_commits', event.detail)} />
              <Switch optimistic={false} id="global-policy-sync-mrs" label="全局 MR 评论" expandedHitArea
                checked={commentSettings?.sync_mrs ?? false} saving={savingCommentSetting === 'sync_mrs'}
                disabled={!canWriteRules || !commentSettings || !!savingCommentSetting || loadingPolicies}
                on:change={(event) => saveCommentSetting('sync_mrs', event.detail)} />
            </div>
            <p>全局允许后，按各仓库设置发布。</p>
            {#if commentSettingsError}<p class="policy-feedback is-error" role="alert">{commentSettingsError}</p>
            {:else if commentSettingsNotice}<p class="policy-feedback" role="status">{commentSettingsNotice}</p>{/if}
          </section>
        </header>
        <div class="rules-workbench-body">
          <aside class="wa-admin-card rules-repo-list" aria-label="规则所属仓库">
              <AdminListFilterBar label="仓库规则列表筛选" className="rules-repo-filter">
                {#snippet leading()}
                  <div class="repo-directory-header"><h3>项目仓库</h3><span>{visibleRepoPolicies.length} / {repoPolicies.length}</span></div>
                {/snippet}
                {#snippet controls()}
                  <TextInput id="rules-repo-search" label="搜索仓库" bind:value={policySearch} placeholder="搜索仓库名称或 ID" />
                {/snippet}
              </AdminListFilterBar>
              <AdminDataList
                columns={repoPolicyColumns} rows={repoPolicyRows} caption="仓库规则列表"
                cell={renderRepoPolicyCell} empty={renderRepoPolicyEmpty}
                loading={loadingPolicies && repoPolicies.length === 0} skeletonRows={4}
                disabled={savingPolicy || !!savingRepoSetting} selectedRowId={selectedPolicyRepo}
                resetKey={policySearch} surface="embedded" className="rules-repo-table"
                scrollRegionId="rules-repo-table-content" tableMinWidth="0px" compactTableMinWidth="0px"
              />
            </aside>
            <section class="wa-admin-card rules-editor-pane" aria-label="仓库规则编辑">
            {#if editingPolicy}
              <header class="policy-editor-header">
                <div class="policy-editor-title">
                  <h3>{editingPolicy.repo_name || `仓库 #${editingPolicy.project_id}`}</h3>
                  <p>项目 ID {editingPolicy.project_id}</p>
                </div>
                <div class="repo-comment-settings" aria-label="当前仓库评论设置">
                  <div class="comment-switch-row">
                    <Switch optimistic={false} id="policy-sync-commits" label="Commit 评论" expandedHitArea
                      checked={editingPolicy.sync_commits} saving={savingRepoSetting === 'sync_commits'}
                      disabled={!canWriteRules || savingPolicy || !!savingRepoSetting || loadingPolicies}
                      on:change={(event) => saveRepoSetting('sync_commits', event.detail)} />
                    <Switch optimistic={false} id="policy-sync-mrs" label="MR 评论" expandedHitArea
                      checked={editingPolicy.sync_mrs} saving={savingRepoSetting === 'sync_mrs'}
                      disabled={!canWriteRules || savingPolicy || !!savingRepoSetting || loadingPolicies}
                      on:change={(event) => saveRepoSetting('sync_mrs', event.detail)} />
                  </div>
                  {#if commentSettings && (!commentSettings.sync_commits || !commentSettings.sync_mrs)}
                    <p class="policy-gate-note">全局已暂停{!commentSettings.sync_commits && !commentSettings.sync_mrs ? ' Commit 与 MR ' : !commentSettings.sync_commits ? ' Commit ' : ' MR '}评论发布。</p>
                  {/if}
                </div>
              </header>
              <div class="policy-document-heading">
                <div><h4>自定义工程规则</h4><p>{policyRulesDirty ? '有未保存的修改' : '保存后用于后续代码评审'}</p></div>
                <Button variant="primary" size="small" loading={savingPolicy}
                  disabled={!canWriteRules || savingPolicy || !!savingRepoSetting || !policyRulesDirty || editingPolicy.rules.length > 8000}
                  on:click={saveRepoPolicy}>保存规则</Button>
              </div>
              {#if policyNotice}<p class="policy-feedback" role="status">{policyNotice}</p>{/if}
              {#if editingPolicy.rules.length > 8000}<p class="policy-feedback is-error" role="alert">规则内容超过 8000 字符，请精简后保存。</p>{/if}
              <div class="policy-primary-editor">
                {#key editingPolicy.project_id}
                  <MarkdownWorkbench bind:value={editingPolicy.rules} mode="live" availableModes={['live']}
                    label="自定义工程规则" description="编辑即排版，支持 Markdown" embedded minHeight={360}
                    readonly={!canWriteRules || savingPolicy || !!savingRepoSetting} saving={savingPolicy}
                    placeholder={'编写本仓库的工程规则，例如：\n\n## 评审重点\n- 核对接口约定与异常处理\n- 补充关键业务场景的验证依据'}
                    on:save={() => { if (policyRulesDirty && editingPolicy && editingPolicy.rules.length <= 8000) void saveRepoPolicy(); }} />
                {/key}
              </div>
            {:else}
              <div class="rules-empty-editor" role="status">
                <h3>{loadingPolicies ? '正在加载仓库规则…' : '选择一个仓库'}</h3>
                <p>{loadingPolicies ? '规则加载完成后即可查看。' : '选择仓库以查看和编辑工程规则。'}</p>
              </div>
            {/if}
          </section>
        </div>
      </div>
    </section>
  {/if}

  <!-- TAB 4: 上下文与语料库 (Context) -->
  {#if resolvedActiveSection === 'context'}
    <section
      id="ai-governance-panel-context"
      class="gov-pane"
      role="tabpanel"
      aria-labelledby="ai-governance-tab-context"
      aria-busy={contextReadinessLoading}
    >
      {#if contextReadinessLoading}
        <div class="gov-notice" role="status" aria-live="polite">正在读取 AI 上下文就绪状态...</div>
      {:else if contextReadinessError}
        <div class="gov-notice error" role="alert">
          <span>{contextReadinessError}</span>
          <button type="button" class="btn btn-sm btn-ghost" on:click={() => retrySection('context')}>重试</button>
        </div>
      {:else if contextAIReady === false}
        <div class="gov-notice" role="status">AI 引擎当前{contextAIStatus === 'disabled' ? '未启用' : '配置不完整'}；资料仍可浏览，导入时由后端继续执行权威校验。</div>
      {/if}
      <AIConfig
        view="context"
        config={governanceAIConfig}
        {currentUserPermissions}
        {contextAIReady}
      />
    </section>
  {/if}

  {#if resolvedActiveSection === 'skill_registry'}
    <div id="ai-governance-panel-skill_registry" class="gov-pane" role="tabpanel" tabindex="0" aria-labelledby="ai-governance-tab-skill_registry">
      <OpenSkillRegistry canWrite={canManageOpenCaps} canObserve={currentUserRole === 'super_admin'} canManageKnowledge={currentUserRole === 'super_admin' && (currentUserPermissions.includes('ai_context:write') || currentUserPermissions.includes('*'))} onOpenCapabilities={() => switchSection('open_capabilities')} />
    </div>
  {/if}

  <!-- TAB 5: 开放能力与 MCP 集成 (Open Capabilities) -->
  {#if resolvedActiveSection === 'open_capabilities'}
    <section
      id="ai-governance-panel-open_capabilities"
      class="gov-pane"
      role="tabpanel"
      aria-labelledby="ai-governance-tab-open_capabilities"
    >
      <OpenCapabilitiesConfig
        canWrite={canManageOpenCaps}
        onOpenRegistry={() => switchSection('skill_registry')}
      />
    </section>
  {/if}
</div>

<!-- ================= MODALS & DRAWERS ================= -->

<!-- 1. Capability Detail Drawer -->
{#if detailDrawerOpen}
  <div use:portalToConsole class="drawer-portal-layer">
    <div class="drawer-backdrop" role="presentation" on:click={closeDetailDrawer}></div>
    <aside use:manageDialogFocus tabindex="-1" class="gov-drawer" role="dialog" aria-modal="true" aria-label="技能详细规约">
    <div class="drawer-header">
      <div>
        <span class="drawer-kicker">{selectedCapability?.kind?.toUpperCase()} 规约详情</span>
        <h2>{selectedCapability?.capability_key}</h2>
      </div>
      <button type="button" class="close-btn" aria-label="关闭技能详情" on:click={closeDetailDrawer}>×</button>
    </div>

    <div class="drawer-body">
      {#if loadingDetail}
        <div class="gov-loading-state">
          <div class="spinner"></div>
          <span>读取微内核资源切片与依赖拓扑...</span>
        </div>
      {:else if capabilityDetail}
        <!-- Summary Cards -->
        <div class="detail-summary-grid">
          <div class="summary-box">
            <span class="label">注册资源大小</span>
            <strong>{capabilityDetail.disk_size_formatted}</strong>
            <span class="sub">{capabilityDetail.disk_size_bytes.toLocaleString()} bytes</span>
          </div>
          <div class="summary-box">
            <span class="label">关联运行绑定</span>
            <strong>{capabilityDetail.bindings_count || 0} 次</strong>
            <span class="sub">{capabilityDetail.bindings_count ? '已产生实际运行审计' : '未关联历史任务'}</span>
          </div>
          <div class="summary-box">
            <span class="label">责任方 / Owner</span>
            <strong>{capabilityDetail.capability.owner || 'system'}</strong>
            <span class="sub">敏感度: {capabilityDetail.capability.sensitivity}</span>
          </div>
          <div class="summary-box">
            <span class="label">当前运行状态</span>
            <span class="status-tag {capabilityDetail.capability.status}">
              {capabilityDetail.capability.status === 'archived' ? '已冷归档 (保留审计)' : capabilityDetail.capability.status === 'uninstalled' ? '已卸载 (可恢复)' : capabilityDetail.capability.status === 'active' ? '活跃生效中' : '已停用'}
            </span>
          </div>
        </div>

        <section class="installation-evidence" aria-label="安装位置与状态来源">
          <h3>{capabilityDetail.origin === 'system_builtin' ? '系统内置' : capabilityDetail.install_state === 'installed' ? '服务端安装已核验' : '安装尚未核验'}</h3>
          <dl>
            <div><dt>安装位置</dt><dd><code>{capabilityDetail.installation?.location || capabilityDetail.builtin_location || '未找到服务端安装凭据'}</code></dd></div>
            <div><dt>安装方式</dt><dd>{capabilityDetail.origin === 'system_builtin' ? '随服务镜像提供' : installationMethodLabel(capabilityDetail.installation?.method)}</dd></div>
            <div><dt>核验来源</dt><dd>{capabilityDetail.installation?.evidence_source || (capabilityDetail.origin === 'system_builtin' ? '服务端内置清单；运行可用性以检查结果为准' : '未上报')}</dd></div>
            <div><dt>状态来源</dt><dd>{capabilityDetail.status_source || '运行时注册表'}</dd></div>
            <div><dt>客户端安装</dt><dd>{capabilityDetail.client_installation || '未上报'} · Codex / Claude</dd></div>
            {#if capabilityDetail.installation}<div><dt>来源 / 时间</dt><dd>{capabilityDetail.installation.source} · {new Date(capabilityDetail.installation.installed_at).toLocaleString()}</dd></div>{/if}
          </dl>
          {#if capabilityDetail.install_state === 'missing'}<p role="status">安装文件缺失或校验不一致，已从“已安装 Skill”列表移除。</p>{/if}
        </section>
        {#if capabilityDetail.plugin_runtime}
          {@const runtime = capabilityDetail.plugin_runtime}
          <section class="installation-evidence" aria-label="Headroom 运行观测">
            <div class="plugin-runtime-heading"><h3>Headroom {runtime.version}</h3>{#if canManageSkills}<Button variant="secondary" size="small" disabled={checkingHeadroom} on:click={checkHeadroom}>{checkingHeadroom ? '检查中…' : '运行本地检查'}</Button>{/if}</div>
            <p>Agent 默认启用 · 本地无损压缩 · 遵循 AI 配置与插件启用状态。失败时保留原文。</p>
            <dl>
              <div><dt>当前进程</dt><dd>{runtime.state === 'ready' ? '运行正常' : runtime.state === 'degraded' ? '已回退原文' : '尚未运行检查'} · {runtime.mode}</dd></div>
              <div><dt>统计来源</dt><dd>{runtime.state_source}</dd></div>
              <div><dt>处理 / 压缩 / 回退</dt><dd>{runtime.calls} / {runtime.compressed} / {runtime.fallbacks} 次</dd></div>
              <div><dt>节省 Token</dt><dd>{runtime.tokens_saved.toLocaleString()} · 最近结果：{runtime.last_reason || '尚无记录'}</dd></div>
            </dl>
          </section>
        {/if}

        <!-- Dual View Tabs -->
        <div class="drawer-view-tabs">
          <button
            type="button"
            class="drawer-tab-btn"
            class:active={drawerActiveView === 'skill_doc'}
            on:click={() => drawerActiveView = 'skill_doc'}
          >
            📄 技能说明与规约 (SKILL.md)
          </button>
          <button
            type="button"
            class="drawer-tab-btn"
            class:active={drawerActiveView === 'slices'}
            on:click={() => drawerActiveView = 'slices'}
          >
            🧩 微内核切片与版本 ({capabilityDetail.versions.length})
          </button>
        </div>

        {#if drawerActiveView === 'skill_doc'}
          <!-- 1. SKILL.md Document View -->
          <div class="skill-doc-container">
            <!-- Included Components Highlight Box -->
            {#if capabilityDetail.included_components && capabilityDetail.included_components.length > 0}
              <div class="doc-included-box">
                <div class="doc-inc-header">
                  <span class="inc-icon">📦</span>
                  <strong>所属组件与插件 ({capabilityDetail.included_components.length})</strong>
                </div>
                <div class="doc-inc-grid">
                  {#each capabilityDetail.included_components as comp}
                    <div class="doc-inc-card">
                      <div class="doc-inc-card-head">
                        <span class="inc-badge {comp.kind}">{comp.kind === 'skill' ? '子技能' : comp.kind === 'mcp' ? 'MCP' : comp.kind === 'plugin' ? '插件' : '上下文提供方'}</span>
                        <code>{comp.key}</code>
                        {#if comp.source_label}<span class="component-source">{comp.source_label}</span>{/if}
                      </div>
                      <p class="doc-inc-desc">{comp.description}</p>
                      {#if comp.tools && comp.tools.length > 0}
                        <div class="doc-inc-tools">
                          <span>提供工具:</span>
                          {#each comp.tools as tool}
                            <code>{tool}</code>
                          {/each}
                        </div>
                      {/if}
                    </div>
                  {/each}
                </div>
              </div>
            {:else if capabilityDetail.parent_skill_key}
              <div class="doc-parent-box">
                <span>🏷️ 本组件为底层能力切片，内聚在顶层业务技能 <strong>{capabilityDetail.parent_skill_key}</strong> 中统一调用。</span>
              </div>
            {/if}

            <div class="skill-doc-actions-bar">
              <span class="doc-meta-tip">通用 Agent 技能规范 Canonical SKILL.md</span>
              <button type="button" class="btn btn-sm btn-secondary" on:click={copySkillDoc}>
                {copiedSkillDoc ? '✓ 已复制到剪贴板' : '📋 复制 SKILL.md 原文'}
              </button>
            </div>

            <!-- Rendered Markdown Content -->
            <div class="skill-doc-prose">
              {@html renderMarkdown(capabilityDetail.skill_markdown || '# 暂无说明文档\n此技能尚未配置 SKILL.md')}
            </div>
          </div>
        {:else}
          <!-- 2. Versions & Resources -->
          <div class="versions-section">
            <h3>已归档版本 ({capabilityDetail.versions.length})</h3>

            <div class="version-tabs">
              {#each capabilityDetail.versions as v, idx}
                <button
                  type="button"
                  class="ver-tab-btn"
                  class:active={activeVersionTab === idx}
                  on:click={() => activeVersionTab = idx}
                >
                  v{v.version} ({v.status})
                </button>
              {/each}
            </div>

            {#if capabilityDetail.versions[activeVersionTab]}
              {@const curVer = capabilityDetail.versions[activeVersionTab]}
              <div class="version-content-box">
                <div class="ver-meta-strip">
                  <div>
                    <strong>Version {curVer.version}</strong>
                    <span class="ver-digest font-mono">Digest: {curVer.content_digest?.slice(0, 16)}...</span>
                  </div>

                  {#if curVer.status !== 'active' && canManageSkills}
                    <button
                      type="button"
                      class="btn btn-sm btn-secondary"
                      on:click={() => {
                        const capKey = capabilityDetail?.capability?.capability_key || selectedCapability?.capability_key || '';
                        if (capKey) activateSpecificVersion(capKey, curVer.version, curVer.scope_type);
                      }}
                    >
                      激活此版本为线上生效
                    </button>
                  {:else if curVer.status === 'active'}
                    <span class="status-tag active">当前生效中</span>
                  {/if}
                </div>

                <!-- Sliced Resources -->
                <h4>微内核资源切片 (Resource Slices)</h4>
                <div class="resources-list">
                  {#if curVer.resources && curVer.resources.length > 0}
                    {#each curVer.resources as res}
                      <div class="resource-card">
                        <div class="res-header">
                          <span class="res-level">{res.load_level}</span>
                          <strong>{res.resource_key}</strong>
                          <span class="res-kind">{res.content_kind}</span>
                          <span class="res-tokens">~{res.token_estimate || 0} tokens</span>
                          <span class="res-size">{res.content ? `${new Blob([res.content]).size} B` : '0 B (已清理)'}</span>
                        </div>
                        <pre class="res-code">{res.content || '(内容已清理释放，元数据已保留)'}</pre>
                      </div>
                    {/each}
                  {:else}
                    <p class="muted-text">无独立资源切片</p>
                  {/if}
                </div>

                <!-- Dependencies -->
                <h4>依赖拓扑 (Dependencies)</h4>
                {#if curVer.dependencies && curVer.dependencies.length > 0}
                  <ul class="dep-list">
                    {#each curVer.dependencies as dep}
                      <li>
                        <code>{dep.dependency_key}</code>
                        <span class="dep-constraint">{dep.version_constraint}</span>
                        {#if dep.is_optional}<span class="badge">可选</span>{/if}
                      </li>
                    {/each}
                  </ul>
                {:else}
                  <p class="muted-text">无外部声明依赖</p>
                {/if}

                <!-- Manifest Code View -->
                <h4>Canonical Manifest (JSON/YAML)</h4>
                <pre class="manifest-raw-code">{curVer.manifest_json}</pre>
              </div>
            {/if}
          </div>
        {/if}
      {/if}
    </div>
  </aside>
  </div>
{/if}

<!-- 2. Import / Remote Install Modal -->
{#if canManageSkills && importModalOpen}
  <div class="modal-backdrop" role="presentation" on:click={() => importModalOpen = false}></div>
  <div use:manageDialogFocus tabindex="-1" class="gov-modal" role="dialog" aria-modal="true" aria-label="安装或导入能力">
    <div class="modal-header">
      <h2>新增 / 导入 Agent 微内核技能</h2>
      <button type="button" class="close-btn" aria-label="关闭技能导入对话框" on:click={() => importModalOpen = false}>×</button>
    </div>

    <!-- Modal Mode Tab -->
    <div class="modal-tabs">
      <button
        type="button"
        class="modal-tab-btn"
        class:active={importTab === 'manifest'}
        on:click={() => importTab = 'manifest'}
      >
        通用 Manifest (YAML/JSON)
      </button>
      <button
        type="button"
        class="modal-tab-btn"
        class:active={importTab === 'remote'}
        on:click={() => importTab = 'remote'}
      >
        远程网络安装 (Remote Install)
      </button>
    </div>

    <div class="modal-body">
      {#if importError}
        <div class="gov-notice error">{importError}</div>
      {/if}

      {#if importTab === 'manifest'}
        <p class="modal-hint">遵循通用 Capability Manifest 标准规约，支持本地文件导入或在线编写：</p>
        <div class="file-upload-row">
          <label class="btn btn-sm btn-ghost file-label">
            选择本地 Manifest 文件 (.yaml/.json)
            <input type="file" accept=".yaml,.yml,.json" on:change={handleFileUpload} />
          </label>
          <button type="button" class="btn btn-sm btn-ghost" on:click={() => importManifestText = sampleManifest}>
            重置为标准模板
          </button>
        </div>

        <textarea
          rows="14"
          class="code-editor"
          bind:value={importManifestText}
          placeholder="在此输入符合 capability-manifest/v1 规范的 YAML 或 JSON 内容..."
        ></textarea>
      {:else}
        <p class="modal-hint">支持从远程 GitHub、GitLab、私有制品库或技能市场 URL 直接下载并安装微内核能力：</p>
        <div class="form-group">
          <label for="remote-url-input">远程 Manifest URL (HTTP/HTTPS):</label>
          <input
            id="remote-url-input"
            type="url"
            bind:value={remoteInstallURL}
            placeholder="https://raw.githubusercontent.com/org/repo/main/capabilities/fms-dispatch.yaml"
          />
        </div>

        <div class="form-group">
          <label class="checkbox-label">
            <input type="checkbox" bind:checked={remoteAutoActivate} />
            安装后自动校验并激活为可用状态
          </label>
        </div>
      {/if}
    </div>

    <div class="modal-footer">
      <button type="button" class="btn btn-ghost" on:click={() => importModalOpen = false}>取消</button>
      <button type="button" class="btn btn-primary" disabled={importing} on:click={submitImport}>
        {importing ? '正在安装校验…' : (importTab === 'manifest' ? '解析并注册' : '下载并安装')}
      </button>
    </div>
  </div>
{/if}

<!-- 3. Upgrade Modal -->
  {#if canManageSkills && upgradeTarget}
  <CapabilityUpgradeModal
    show={upgradeModalOpen}
    capabilityKey={upgradeTarget.capability_key}
    latestVersion={upgradeTarget.latest_version || 1}
    busy={upgrading}
    error={upgradeError}
    bind:mode={upgradeMode}
    bind:manifestText={upgradeManifestText}
    bind:remoteURL={upgradeRemoteURL}
    on:modechange={() => upgradeError = ''}
    on:close={() => { if (!upgrading) upgradeModalOpen = false; }}
    on:submit={submitUpgrade}
  />
{/if}

<!-- 4. Delete & Cleanup / Cold Archive Confirmation Modal -->
{#if canManageSkills && deleteModalOpen && deleteTarget}
  <div class="modal-backdrop" role="presentation" on:click={() => deleteModalOpen = false}></div>
  <div use:manageDialogFocus tabindex="-1" class="gov-modal modal-danger" role="dialog" aria-modal="true" aria-label="卸载或冷归档技能">
    <div class="modal-header">
      <h2>确认{deleteMode === 'cold_archive' ? '冷归档' : '卸载清理'}技能？</h2>
      <button type="button" class="close-btn" aria-label="关闭技能清理对话框" on:click={() => deleteModalOpen = false}>×</button>
    </div>

    <div class="modal-body">
      {#if deleteError}
        <div class="gov-notice error">{deleteError}</div>
      {/if}

      <p>目标能力：<strong>{deleteTarget.capability_key}</strong> (当前物理磁盘：{deleteTarget.disk_size_formatted})</p>
      
      {#if (deleteTarget.bindings_count || 0) > 0}
        <div class="audit-warning-box">
          <div class="warning-title">⚠️ 运行审计与重放关联提醒</div>
          <p>
            检测到该技能已被 <strong>{deleteTarget.bindings_count}</strong> 次 Agent 运行任务绑定。
            为确保历史运行的证据链不断裂并支持离线重放（Replay），系统推荐将其执行【冷归档】。
          </p>
        </div>

        <div class="mode-select-group">
          <label class="mode-select-card" class:active={deleteMode === 'cold_archive'}>
            <input type="radio" bind:group={deleteMode} value="cold_archive" />
            <div class="mode-info">
              <div class="mode-head">
                <strong>冷归档 (Cold Archive)</strong>
                <span class="badge badge-recommended">推荐模式</span>
              </div>
              <p>从调度中下线隔离，新任务不再使用，但保留代码资源与校验指纹，历史 Run 完全可重现与审计追溯。</p>
            </div>
          </label>

          <label class="mode-select-card" class:active={deleteMode === 'purge'}>
            <input type="radio" bind:group={deleteMode} value="purge" />
            <div class="mode-info">
              <div class="mode-head">
                <strong>强力清除 (Purge)</strong>
              </div>
              <p>彻底物理清空本地磁盘中的代码资源内容，释放 {deleteTarget.disk_size_formatted} 空间。历史任务重放将降级为只读摘要。</p>
            </div>
          </label>
        </div>
      {:else}
        <div class="cleanup-alert-box">
          <strong>✨ 自动磁盘清理与回溯机制：</strong>
          <ul>
            <li>该技能未关联任何历史 Run 执行，将立即彻底清空占用的 <strong>{deleteTarget.disk_size_formatted}</strong> 本地磁盘资源内容，释放存储空间；</li>
            <li>将技能状态标记为 <code>uninstalled</code>，历史版本标记为 <code>retired</code>；</li>
            <li><strong>系统将保留安装审计记录与元数据</strong>，您可随时在列表中查阅回溯，并在需要时点击“重新安装”一键复原！</li>
          </ul>
        </div>
      {/if}
    </div>

    <div class="modal-footer">
      <button type="button" class="btn btn-ghost" on:click={() => deleteModalOpen = false}>取消</button>
      <button type="button" class="btn btn-danger" disabled={deleting} on:click={confirmUninstall}>
        {deleting ? '正在处理中…' : deleteMode === 'cold_archive' ? '确认冷归档' : '确认卸载与彻底清理'}
      </button>
    </div>
  </div>
{/if}

<style>
  /* finesse · register=product · ai-governance-center · SOUL=4 SPECTACLE=1 DENSITY=7 */
  .gov-workbench {
    box-sizing: border-box;
    width: 100%;
    min-width: 0;
    max-width: 100%;
    overflow-x: clip;
    display: flex;
    flex-direction: column;
    gap: var(--wa-space-4);
    padding: 0;
    height: 100%;
    min-height: 0;
    overflow: hidden;
    color: var(--wa-text-main, #293847);
    font-size: 14px;
    background: transparent;
  }

  /* Header */
  .gov-header {
    display: flex;
    flex-direction: column;
    gap: 16px;
    min-width: 0;
    flex-shrink: 0;
  }

  .gov-title-row {
    display: flex;
    align-items: flex-start;
    justify-content: space-between;
    gap: 16px;
  }

  .gov-title-group h2 {
    margin: 0;
    font-size: 18px;
    font-weight: 700;
    color: var(--wa-text-strong);
    letter-spacing: -0.02em;
    text-wrap: balance;
  }

  .gov-subtitle {
    margin: 6px 0 0 0;
    color: var(--wa-text-muted, #718096);
    font-size: 13.5px;
    line-height: 1.5;
  }

  .gov-header-actions {
    display: flex;
    align-items: center;
    gap: var(--wa-space-2);
  }

  /* Metrics Strip */
  .gov-metrics-strip {
    display: flex;
    align-items: center;
    overflow-x: auto;
    background: #ffffff;
    border: 1px solid var(--wa-border-soft, #e2e8f0);
    border-radius: 8px;
    padding: 12px 20px;
    gap: 24px;
  }

  .gov-metric-item {
    display: flex;
    flex-direction: column;
    gap: 2px;
  }

  .gov-metric-label {
    font-size: 11.5px;
    font-weight: 600;
    color: var(--wa-text-muted, #718096);
    text-transform: uppercase;
    letter-spacing: 0.04em;
  }

  .gov-metric-val {
    display: flex;
    align-items: baseline;
    gap: 6px;
  }

  .gov-metric-val strong {
    font-size: 18px;
    font-weight: 720;
    color: var(--wa-text-strong, #1a202c);
    font-variant-numeric: tabular-nums;
  }

  .highlight-cyan {
    color: var(--wa-accent-strong, #008f96) !important;
  }

  .gov-metric-sub {
    font-size: 12px;
    color: var(--wa-text-muted, #a0aec0);
  }

  .gov-metric-divider {
    width: 1px;
    height: 32px;
    background: var(--wa-border-soft, #edf2f7);
  }

  .gov-nav-tabs {
    display: flex;
    gap: 8px;
    margin-top: 0;
    padding: var(--wa-space-2);
    overflow-x: auto;
    scrollbar-width: thin;
  }

  .gov-tab {
    display: flex;
    align-items: center;
    gap: 8px;
    padding: 9px 16px;
    border-radius: 6px;
    border: none;
    background: transparent;
    color: var(--wa-text-muted, #4a5568);
    font-size: 13.5px;
    font-weight: 600;
    cursor: pointer;
    transition: all 0.15s ease;
    flex: 0 0 auto;
    white-space: nowrap;
  }

  .gov-tab:hover {
    background: #edf2f7;
    color: var(--wa-text-strong, #1a202c);
  }

  .gov-tab.active {
    background: #ffffff;
    color: var(--wa-accent-strong, #008f96);
    box-shadow: 0 1px 3px rgba(0,0,0,0.06), 0 1px 2px rgba(0,0,0,0.04);
  }

  .tab-count {
    padding: 1px 6px;
    font-size: 11px;
    font-weight: 700;
    background: #edf2f7;
    border-radius: 10px;
    color: var(--wa-text-muted, #718096);
  }

  .gov-tab.active .tab-count {
    background: rgba(0, 143, 150, 0.1);
    color: var(--wa-accent-strong, #008f96);
  }

  /* Alerts */
  .gov-notice {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: 10px 16px;
    background: #ebf8fa;
    border: 1px solid #b2e3e8;
    color: #0c666c;
    border-radius: 6px;
    font-size: 13.5px;
  }

  .gov-notice.error {
    background: #fff5f5;
    border-color: #feb2b2;
    color: #c53030;
  }

  /* Pane */
  .gov-pane {
    flex: 1;
    min-width: 0;
    min-height: 0;
    display: flex;
    flex-direction: column;
    gap: var(--wa-space-4);
    overflow-x: hidden;
    overflow-y: auto;
    scrollbar-gutter: stable;
  }
  .gov-pane > :global(*) { flex-shrink: 0; }
  .gov-workbench > .gov-notice { flex-shrink: 0; }

  /* Filter bar */
  .search-box {
    display: flex;
    align-items: center;
    gap: 8px;
    background: #ffffff;
    border: 1px solid var(--wa-border-soft, #cbd5e0);
    border-radius: 6px;
    padding: 8px 12px;
    flex: 1;
    max-width: 420px;
  }

  .search-box input {
    border: none;
    outline: none;
    font-size: 13.5px;
    width: 100%;
    color: var(--wa-text-main, #2d3748);
  }

  .filter-controls {
    display: flex;
    align-items: center;
    gap: 16px;
    flex-wrap: wrap;
  }

  .filter-field {
    display: flex;
    align-items: center;
    gap: 8px;
  }

  .filter-field-label {
    font-size: 13px;
    color: var(--wa-text-muted, #4a5568);
    font-weight: 600;
    white-space: nowrap;
  }

  .filter-select-wrap {
    min-width: 175px;
  }

  .filter-select-wrap :global(.select-group) {
    margin-bottom: 0;
  }

  .filter-select-wrap :global(.select-trigger) {
    min-height: 32px;
    height: 32px;
    padding: 0 10px;
    font-size: 13px;
    border-radius: 6px;
    background: #ffffff;
    border-color: var(--wa-border-soft, #cbd5e0);
  }

  .filter-select-wrap :global(.select-trigger:hover) {
    border-color: #a0aec0;
    background: #fcfdfe;
  }

  .filter-select-wrap :global(.select-trigger:focus-within) {
    border-color: var(--wa-accent-strong, #008f96);
    box-shadow: 0 0 0 3px rgba(0, 143, 150, 0.15);
  }

  /* Table */
  .gov-table-container {
    width: 100%;
    min-width: 0;
    max-width: 100%;
    background: #ffffff;
    border: 1px solid var(--wa-border-soft, #e2e8f0);
    border-radius: 8px;
    overflow-x: auto;
    overflow-y: hidden;
    box-shadow: 0 1px 2px rgba(0,0,0,0.02);
  }

  .gov-table {
    width: 100%;
    min-width: 960px;
    table-layout: fixed;
    border-collapse: collapse;
    font-size: 13.5px;
  }

  .gov-table th {
    text-align: left;
    padding: var(--wa-space-3);
    background: var(--wa-surface-inset);
    border-bottom: 1px solid var(--wa-border-soft, #e2e8f0);
    font-size: 12px;
    font-weight: 650;
    color: var(--wa-text-muted, #718096);
    letter-spacing: 0.03em;
  }

  .gov-table td {
    padding: var(--wa-space-3);
    border-bottom: 1px solid var(--wa-border-soft, #edf2f7);
    vertical-align: middle;
  }

  .gov-table tbody tr:hover {
    background: var(--wa-row-hover);
  }

  .cap-column-identity { width: 208px; }
  .cap-column-kind { width: 96px; }
  .cap-column-status { width: 108px; }
  .cap-column-version { width: 56px; }
  .cap-column-bindings { width: 68px; }
  .cap-column-disk { width: 80px; }
  .cap-column-description { width: 140px; }
  .cap-column-actions { width: 204px; }
  .cap-actions-heading { text-align: right !important; }

  .row-uninstalled {
    opacity: 0.7;
    background: #fdfdfd;
  }

  .cap-identity {
    min-width: 0;
    display: flex;
    flex-direction: column;
    gap: 6px;
  }

  .cap-title-row {
    display: flex;
    align-items: center;
    gap: 6px;
    flex-wrap: wrap;
  }

  .cap-key {
    min-width: 0;
    overflow-wrap: anywhere;
    white-space: normal;
    color: var(--wa-text-strong, #1a202c);
    font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
    font-size: 13px;
  }


  .cap-included-block {
    display: flex;
    align-items: center;
    gap: 5px;
    flex-wrap: wrap;
    font-size: 11px;
    min-width: 0;
    padding-top: var(--wa-space-1);
    border-top: 1px solid var(--wa-border-divider);
  }

  .inc-title {
    color: var(--wa-text-muted, #718096);
    font-weight: 600;
    white-space: nowrap;
  }

  .inc-badge {
    max-width: 100%;
    overflow-wrap: anywhere;
    padding: 1px 6px;
    border-radius: 3px;
    font-size: 11px;
    font-weight: 550;
    font-family: ui-monospace, SFMono-Regular, monospace;
    white-space: normal;
  }

  .inc-badge.plugin {
    background: #e6fffa;
    color: #234e52;
    border: 1px solid #b2f5ea;
  }

  .inc-badge.context_provider {
    background: #ebf8ff;
    color: #2b6cb0;
    border: 1px solid #bee3f8;
  }

  .cap-kind-badge {
    align-self: flex-start;
    padding: 3px 8px;
    border-radius: 4px;
    font-size: 11px;
    font-weight: 650;
    text-transform: uppercase;
    letter-spacing: 0.04em;
    white-space: nowrap;
  }

  .cap-kind-badge.skill { background: #e6fffa; color: #234e52; }
  .cap-kind-badge.plugin { background: #ebf8ff; color: #2a4365; }
  .cap-kind-badge.mcp { background: #faf5ff; color: #44337a; }
  .cap-kind-badge.policy { background: #fffaf0; color: #744210; }

  /* Switch */
  .switch-container {
    display: flex;
    align-items: center;
    gap: 8px;
  }

  .gov-switch {
    width: 36px;
    height: 20px;
    background: #cbd5e0;
    border-radius: 20px;
    border: none;
    position: relative;
    cursor: pointer;
    transition: background 0.2s cubic-bezier(0.4, 0, 0.2, 1);
    padding: 0;
  }

  .gov-switch.checked {
    background: var(--wa-accent-strong, #008f96);
  }

  .gov-switch:disabled {
    opacity: 0.5;
    cursor: not-allowed;
  }

  .gov-switch-handle {
    display: block;
    width: 16px;
    height: 16px;
    background: #ffffff;
    border-radius: 50%;
    position: absolute;
    top: 2px;
    left: 2px;
    transition: transform 0.2s cubic-bezier(0.4, 0, 0.2, 1);
    box-shadow: 0 1px 2px rgba(0,0,0,0.2);
  }

  .gov-switch.checked .gov-switch-handle {
    transform: translateX(16px);
  }

  .switch-text {
    font-size: 12.5px;
    font-weight: 500;
    color: var(--wa-text-main, #4a5568);
  }

  .status-tag {
    font-size: 11.5px;
    padding: 2px 7px;
    border-radius: 4px;
    font-weight: 600;
    white-space: nowrap;
  }

  .status-tag.active { background: #def7ec; color: #03543f; }
  .status-tag.disabled { background: #f3f4f6; color: #4b5563; }
  .status-tag.uninstalled { background: #feecdc; color: #9c4221; }

  .ver-pill {
    padding: 2px 7px;
    border-radius: 4px;
    background: #edf2f7;
    font-size: 12px;
    font-weight: 600;
    color: #4a5568;
    font-variant-numeric: tabular-nums;
    white-space: nowrap;
  }

  .disk-usage-col {
    display: flex;
    flex-direction: column;
  }

  .disk-text {
    font-weight: 650;
    color: var(--wa-text-strong, #1a202c);
    font-variant-numeric: tabular-nums;
    white-space: nowrap;
  }

  .cap-desc-block {
    min-width: 0;
    display: flex;
    flex-direction: column;
    gap: 2px;
  }

  .cap-owner {
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    font-size: 11.5px;
    font-weight: 600;
    color: var(--wa-text-muted, #718096);
  }

  .cap-desc {
    margin: 0;
    color: var(--wa-text-main, #4a5568);
    font-size: 12.5px;
    line-height: 1.4;
    max-width: 100%;
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }

  .action-btn-group {
    display: inline-flex;
    align-items: center;
    justify-content: flex-end;
    gap: 6px;
    white-space: nowrap !important;
    flex-wrap: nowrap !important;
  }

  /* Buttons */
  .btn {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    gap: 6px;
    padding: 8px 14px;
    border-radius: 6px;
    font-size: 13px;
    font-weight: 600;
    cursor: pointer;
    border: none;
    transition: all 0.15s ease;
    white-space: nowrap !important;
    word-break: keep-all !important;
    flex-shrink: 0 !important;
  }

  .btn-sm {
    padding: 5px 10px;
    font-size: 12px;
    white-space: nowrap !important;
    word-break: keep-all !important;
    flex-shrink: 0 !important;
  }

  .btn-primary {
    background: var(--wa-accent-strong, #008f96);
    color: #ffffff;
  }

  .btn-primary:hover {
    background: #007a80;
  }

  .btn-secondary {
    background: #ffffff;
    color: var(--wa-text-strong, #2d3748);
    border: 1px solid var(--wa-border-soft, #cbd5e0);
  }

  .btn-secondary:hover {
    background: #f7fafc;
    border-color: #a0aec0;
  }

  .btn-ghost {
    background: transparent;
    color: var(--wa-text-main, #4a5568);
    border: 1px solid var(--wa-border-soft, #e2e8f0);
  }

  .btn-ghost:hover {
    background: #edf2f7;
    color: var(--wa-text-strong, #1a202c);
  }

  .btn-danger-ghost {
    background: transparent;
    color: #e53e3e;
    border: 1px solid #fed7d7;
  }

  .btn-danger-ghost:hover {
    background: #fff5f5;
    border-color: #feb2b2;
  }

  .btn-danger {
    background: #e53e3e;
    color: #ffffff;
  }

  .btn-danger:hover {
    background: #c53030;
  }

  .close-btn {
    min-width: 40px;
    min-height: 40px;
    display: inline-flex;
    align-items: center;
    justify-content: center;
    border: none;
    background: transparent;
    font-size: 18px;
    color: var(--wa-text-muted);
    cursor: pointer;
  }

  .notice-actions {
    display: flex;
    align-items: center;
    gap: 8px;
  }

  .icon-action {
    min-width: 40px;
    padding-inline: 10px;
  }

  .gov-no-access h2,
  .gov-no-access p {
    margin: 0;
    text-align: center;
  }

  .gov-no-access p {
    max-width: 56ch;
  }

  /* Jira-style peer panels: each panel owns its surface and scroll region. */
  .gov-pane.rules-pane { overflow: hidden; scrollbar-gutter: auto; }
  .gov-pane > .rules-workbench { flex: 1 1 0; }
  .rules-workbench { min-width: 0; min-height: 0; display: flex; flex-direction: column; gap: var(--wa-space-3); }
  .rules-context-header { flex: 0 0 auto; display: flex; align-items: center; justify-content: space-between; gap: var(--wa-space-6); padding: var(--wa-space-3) var(--wa-space-4); }
  .rules-heading { min-width: 0; }
  .rules-context-header h2 { margin: 0; color: var(--wa-text-strong); font-size: 16px; line-height: 1.4; font-weight: 700; }
  .rules-context-header p { margin: var(--wa-space-1) 0 0; color: var(--wa-text-muted); font-size: 12px; line-height: 1.5; }
  .global-comment-settings { min-width: 0; }
  .comment-switch-row { display: flex; flex-wrap: wrap; align-items: center; gap: var(--wa-space-2) var(--wa-space-5); }
  .comment-switch-row :global(.switch-label) { font-size: 12px; white-space: nowrap; }
  .comment-switch-row :global(.switch-container) { gap: var(--wa-space-2); }
  .rules-workbench-body { flex: 1 1 0; display: grid; min-width: 0; min-height: 0; grid-template-columns: minmax(260px, 32%) minmax(0, 1fr); grid-template-rows: minmax(0, 1fr); gap: var(--wa-space-3); align-items: stretch; }
  .rules-repo-list, .rules-editor-pane { min-width: 0; min-height: 0; height: 100%; box-sizing: border-box; overflow: hidden; }
  .rules-repo-list { display: flex; flex-direction: column; padding: 0; }
  .rules-repo-list :global(.rules-repo-filter) { flex: 0 0 auto; display: flex; flex-direction: column; align-items: stretch; gap: var(--wa-space-3); border: 0; border-radius: 0; background: transparent; box-shadow: none; backdrop-filter: none; }
  .rules-repo-list :global(.filter-controls) { width: 100%; }
  .repo-directory-header { display: flex; align-items: center; justify-content: space-between; gap: var(--wa-space-2); }
  .repo-directory-header h3 { margin: 0; font-size: 13px; color: var(--wa-text-main); }
  .repo-directory-header span { font-size: 12px; color: var(--wa-text-muted); font-variant-numeric: tabular-nums; }
  .rules-repo-list :global(.input-group) { margin-bottom: 0; }
  .rules-repo-list :global(.input-label) { position: absolute; width: 1px; height: 1px; padding: 0; overflow: hidden; clip-path: inset(50%); white-space: nowrap; }
  .rules-repo-list :global(.text-input) { min-height: 38px; font-size: 12px; }
  .rules-repo-list :global(.rules-repo-table) { flex: 1 1 0; }
  .rules-repo-list :global(.admin-table) { table-layout: fixed; }
  .rules-repo-list :global(.admin-table td) { white-space: normal; }
  .repo-row-select { display: flex; flex-direction: column; gap: var(--wa-space-1); width: 100%; min-width: 0; min-height: 44px; padding: var(--wa-space-1) 0; border: 0; border-radius: 0; background: transparent; text-align: left; cursor: pointer; }
  .repo-row-select:disabled { cursor: not-allowed; opacity: 0.6; }
  .repo-name { max-width: 100%; overflow: hidden; display: -webkit-box; -webkit-box-orient: vertical; -webkit-line-clamp: 2; line-clamp: 2; overflow-wrap: anywhere; color: var(--wa-text-strong); font-size: 13px; line-height: 1.5; font-weight: 650; }
  .repo-meta { color: var(--wa-text-muted); font-size: 11px; line-height: 1.5; }
  .repo-rule-state { color: var(--wa-text-muted); font-size: 11px; line-height: 1.5; white-space: nowrap; }
  .repo-rule-state.is-draft { color: var(--wa-accent-strong); }
  .rules-editor-pane { display: flex; flex-direction: column; padding: var(--wa-space-4) var(--wa-space-5); }
  .policy-editor-header { flex: 0 0 auto; display: flex; flex-wrap: wrap; align-items: center; justify-content: space-between; gap: var(--wa-space-3); padding-bottom: var(--wa-space-4); border-bottom: 1px solid var(--wa-border-divider); }
  .policy-editor-title { flex: 1 1 160px; min-width: 0; }
  .policy-editor-header h3 { margin: 0; font-size: 16px; line-height: 1.4; overflow-wrap: anywhere; color: var(--wa-text-strong); }
  .policy-editor-header p { margin: var(--wa-space-1) 0 0; color: var(--wa-text-muted); font-size: 12px; line-height: 1.5; }
  .repo-comment-settings { min-width: 0; }
  .policy-gate-note { text-align: right; }
  .policy-document-heading { flex: 0 0 auto; display: flex; align-items: center; justify-content: space-between; gap: var(--wa-space-3); margin: var(--wa-space-4) 0 var(--wa-space-3); }
  .policy-document-heading h4 { margin: 0; color: var(--wa-text-main); font-size: 13px; font-weight: 650; }
  .policy-document-heading p { margin: var(--wa-space-1) 0 0; color: var(--wa-text-muted); font-size: 12px; line-height: 1.5; }
  .policy-document-heading :global(.btn) { flex-shrink: 0; min-width: 90px; }
  .policy-primary-editor { flex: 1 1 0; min-width: 0; min-height: 0; }
  .policy-primary-editor :global(.markdown-workbench) { height: 100%; }
  .policy-feedback { flex: 0 0 auto; margin: 0 0 var(--wa-space-3); color: var(--wa-accent-strong); font-size: 12px; line-height: 1.5; overflow-wrap: anywhere; }
  .rules-context-header .policy-feedback { color: var(--wa-accent-strong); }
  .policy-feedback.is-error, .rules-context-header .policy-feedback.is-error { color: var(--wa-danger, #b42318); }
  .rules-empty { color: var(--wa-text-muted); font-size: 13px; line-height: 1.6; }
  .rules-empty-editor { flex: 1; display: grid; align-content: center; min-height: 0; text-align: center; padding: var(--wa-space-5); }
  .rules-empty-editor h3 { margin: 0; color: var(--wa-text-strong); font-size: 15px; }
  .rules-empty-editor p { margin: var(--wa-space-2) 0 0; color: var(--wa-text-muted); font-size: 13px; line-height: 1.6; }

  @media (min-width: 761px) and (max-width: 1100px) {
    .rules-workbench-body { grid-template-columns: minmax(230px, 32%) minmax(0, 1fr); }
    .rules-context-header { align-items: flex-start; gap: var(--wa-space-3); }
    .rules-editor-pane { padding: var(--wa-space-4); }
    .comment-switch-row { gap: var(--wa-space-1) var(--wa-space-3); }
  }
  @media (max-width: 760px) {
    .gov-pane.rules-pane { overflow-y: auto; }
    .gov-pane > .rules-workbench { flex: 0 0 auto; }
    .rules-context-header { align-items: stretch; flex-direction: column; gap: var(--wa-space-2); padding: var(--wa-space-4); }
    .rules-workbench-body { flex: 0 0 auto; grid-template-columns: minmax(0, 1fr); grid-template-rows: 260px minmax(600px, auto); }
    .rules-repo-list :global(.text-input) { min-height: 44px; }
    .rules-editor-pane { height: auto; min-height: 600px; padding: var(--wa-space-4); }
    .policy-editor-header { align-items: flex-start; }
    .policy-editor-title { flex-basis: 100%; }
    .policy-gate-note { text-align: left; }
    .policy-primary-editor { flex: 0 0 auto; height: 420px; }
  }

  .form-group {
    display: flex;
    flex-direction: column;
    gap: 6px;
  }

  .form-group label {
    font-size: 12.5px;
    font-weight: 600;
    color: #4a5568;
  }

  .checkbox-label {
    display: inline-flex;
    align-items: center;
    gap: 8px;
    font-size: 13px;
    font-weight: 500;
    cursor: pointer;
  }

  textarea.code-editor {
    width: 100%; box-sizing: border-box; border: 1px solid var(--wa-border-soft);
    border-radius: var(--wa-radius-sm); padding: var(--wa-space-3); background: var(--wa-surface-lift);
    color: var(--wa-text-main); font: 12.5px/1.5 var(--wa-font-mono); resize: vertical;
  }

  /* Drawer */
  .drawer-portal-layer {
    position: fixed;
    inset: 0;
    z-index: 2200;
    pointer-events: none;
  }

  .drawer-backdrop {
    position: fixed;
    inset: 0;
    background: rgba(15, 23, 42, 0.45);
    z-index: 2201;
    backdrop-filter: blur(4px);
    -webkit-backdrop-filter: blur(4px);
    pointer-events: auto;
    animation: fadeIn 0.2s cubic-bezier(0.16, 1, 0.3, 1);
  }

  .gov-drawer {
    position: fixed;
    top: 0;
    right: 0;
    bottom: 0;
    height: 100vh;
    height: 100dvh;
    width: 680px;
    max-width: 90vw;
    background: #ffffff;
    z-index: 2202;
    box-shadow: -8px 0 32px rgba(15, 23, 42, 0.18);
    display: flex;
    flex-direction: column;
    overflow: hidden;
    pointer-events: auto;
    animation: drawerSlideIn 0.25s cubic-bezier(0.16, 1, 0.3, 1);
  }

  @keyframes drawerSlideIn {
    from {
      transform: translateX(100%);
    }
    to {
      transform: translateX(0);
    }
  }

  .drawer-header {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: 20px 24px;
    border-bottom: 1px solid var(--wa-border-soft, #e2e8f0);
  }

  .drawer-kicker {
    font-size: 11px;
    font-weight: 700;
    letter-spacing: 0.06em;
    color: var(--wa-accent-strong, #008f96);
  }

  .drawer-header h2 {
    margin: 4px 0 0 0;
    font-size: 20px;
    font-weight: 720;
    font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
  }

  .drawer-body {
    padding: 24px;
    overflow-y: auto;
    display: flex;
    flex-direction: column;
    gap: 20px;
  }

  .detail-summary-grid {
    display: grid;
    grid-template-columns: repeat(3, 1fr);
    gap: 12px;
  }

  .summary-box {
    background: #f8fafc;
    border: 1px solid var(--wa-border-soft, #e2e8f0);
    border-radius: 6px;
    padding: 12px;
    display: flex;
    flex-direction: column;
    gap: 2px;
  }

  .summary-box .label {
    font-size: 11px;
    font-weight: 600;
    color: #718096;
    text-transform: uppercase;
  }

  .summary-box strong {
    font-size: 16px;
    color: #1a202c;
  }

  .summary-box .sub {
    font-size: 11px;
    color: var(--wa-text-muted);
  }

  /* Dual View Tabs in Drawer */
  .drawer-view-tabs {
    display: flex;
    gap: 8px;
    padding: 4px;
    background: #f1f5f9;
    border-radius: 8px;
    border: 1px solid var(--wa-border-soft, #e2e8f0);
  }

  .drawer-tab-btn {
    flex: 1;
    display: inline-flex;
    align-items: center;
    justify-content: center;
    gap: 6px;
    padding: 8px 12px;
    border-radius: 6px;
    border: none;
    background: transparent;
    font-size: 13px;
    font-weight: 600;
    color: var(--wa-text-muted, #64748b);
    cursor: pointer;
    transition: all 0.15s ease;
    white-space: nowrap;
  }

  .drawer-tab-btn:hover {
    color: var(--wa-text-strong, #1e293b);
  }

  .drawer-tab-btn.active {
    background: #ffffff;
    color: var(--wa-accent-strong, #008f96);
    box-shadow: 0 1px 3px rgba(0,0,0,0.06), 0 1px 2px rgba(0,0,0,0.04);
  }

  /* SKILL.md Document View */
  .skill-doc-container {
    display: flex;
    flex-direction: column;
    gap: 16px;
  }

  .doc-included-box {
    background: #f8fafc;
    border: 1px solid #e2e8f0;
    border-radius: 8px;
    padding: 14px 16px;
    display: flex;
    flex-direction: column;
    gap: 10px;
  }

  .doc-inc-header {
    display: flex;
    align-items: center;
    gap: 6px;
    font-size: 13px;
    font-weight: 650;
    color: var(--wa-text-strong, #1e293b);
  }

  .doc-inc-grid {
    display: grid;
    grid-template-columns: repeat(auto-fit, minmax(260px, 1fr));
    gap: 8px;
  }

  .doc-inc-card {
    background: #ffffff;
    border: 1px solid #e2e8f0;
    border-radius: 6px;
    padding: 10px 12px;
    display: flex;
    flex-direction: column;
    gap: 6px;
  }

  .doc-inc-card-head {
    display: flex;
    align-items: center;
    gap: 6px;
  }

  .doc-inc-card-head code {
    font-weight: 650;
    color: #1e293b;
    font-size: 12.5px;
  }

  .doc-inc-desc {
    margin: 0;
    font-size: 11.5px;
    color: var(--wa-text-muted, #64748b);
    line-height: 1.45;
  }

  .doc-inc-tools {
    display: flex;
    align-items: center;
    gap: 4px;
    flex-wrap: wrap;
    font-size: 11px;
    color: #94a3b8;
  }

  .doc-inc-tools code {
    background: #f1f5f9;
    padding: 1px 5px;
    border-radius: 3px;
    color: #475569;
    font-size: 11px;
  }

  .doc-parent-box {
    background: #eff6ff;
    border: 1px solid #bfdbfe;
    color: #1e40af;
    padding: 10px 14px;
    border-radius: 6px;
    font-size: 13px;
    line-height: 1.45;
  }

  .skill-doc-actions-bar {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding-bottom: 8px;
    border-bottom: 1px solid var(--wa-border-soft, #e2e8f0);
  }

  .doc-meta-tip {
    font-size: 12px;
    color: var(--wa-text-muted, #94a3b8);
  }

  /* Markdown prose rendering */
  .skill-doc-prose {
    font-size: 13.5px;
    line-height: 1.65;
    color: var(--wa-text-main, #334155);
    background: #ffffff;
    border: 1px solid var(--wa-border-soft, #e2e8f0);
    border-radius: 8px;
    padding: 18px 20px;
  }

  .skill-doc-prose :global(h1) {
    font-size: 18px;
    font-weight: 700;
    color: var(--wa-text-strong, #0f172a);
    margin: 14px 0 8px;
    padding-bottom: 6px;
    border-bottom: 1px solid #e2e8f0;
  }

  .skill-doc-prose :global(h2) {
    font-size: 15px;
    font-weight: 650;
    color: var(--wa-text-strong, #1e293b);
    margin: 16px 0 8px;
  }

  .skill-doc-prose :global(h3) {
    font-size: 13.5px;
    font-weight: 650;
    color: #334155;
    margin: 12px 0 6px;
  }

  .skill-doc-prose :global(p) {
    margin: 0 0 10px;
  }

  .skill-doc-prose :global(blockquote) {
    margin: 8px 0 12px;
    padding: 10px 14px;
    background: #f8fafc;
    border: 1px solid var(--wa-border-soft, #e2e8f0);
    color: #475569;
    border-radius: 6px;
    font-size: 13px;
  }

  .skill-doc-prose :global(ul), .skill-doc-prose :global(ol) {
    margin: 0 0 10px 18px;
    padding: 0;
  }

  .skill-doc-prose :global(li) {
    margin-bottom: 4px;
  }

  .skill-doc-prose :global(code) {
    font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
    font-size: 12px;
    background: #f1f5f9;
    color: #0f172a;
    padding: 2px 5px;
    border-radius: 4px;
  }

  .skill-doc-prose :global(pre) {
    background: #0f172a;
    color: #e2e8f0;
    padding: 12px 14px;
    border-radius: 6px;
    overflow-x: auto;
    font-size: 12px;
    margin: 8px 0 12px;
  }

  .skill-doc-prose :global(pre code) {
    background: transparent;
    color: inherit;
    padding: 0;
    border-radius: 0;
  }

  .version-tabs {
    display: flex;
    gap: 8px;
    border-bottom: 1px solid var(--wa-border-soft, #e2e8f0);
    padding-bottom: 8px;
    margin-bottom: 14px;
  }

  .ver-tab-btn {
    border: 1px solid var(--wa-border-soft, #e2e8f0);
    background: #f8fafc;
    padding: 6px 12px;
    border-radius: 6px;
    font-size: 12px;
    font-weight: 600;
    cursor: pointer;
  }

  .ver-tab-btn.active {
    background: var(--wa-accent-strong, #008f96);
    color: #ffffff;
    border-color: var(--wa-accent-strong, #008f96);
  }

  .ver-meta-strip {
    display: flex;
    align-items: center;
    justify-content: space-between;
    background: #f8fafc;
    padding: 10px 14px;
    border-radius: 6px;
    margin-bottom: 14px;
  }

  .ver-digest {
    margin-left: 12px;
    font-size: 11px;
    color: #718096;
  }

  .resources-list {
    display: flex;
    flex-direction: column;
    gap: 12px;
  }

  .resource-card {
    border: 1px solid var(--wa-border-soft, #e2e8f0);
    border-radius: 6px;
    background: #ffffff;
    overflow: hidden;
  }

  .res-header {
    display: flex;
    align-items: center;
    gap: 10px;
    background: #f8fafc;
    padding: 8px 12px;
    border-bottom: 1px solid #edf2f7;
    font-size: 12px;
  }

  .res-level {
    font-weight: 700;
    color: var(--wa-accent-strong, #008f96);
  }

  .res-kind, .res-tokens, .res-size {
    font-size: 11px;
    color: #718096;
  }

  .res-code {
    margin: 0;
    padding: 10px 12px;
    font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
    font-size: 11.5px;
    background: #ffffff;
    line-height: 1.45;
    max-height: 200px;
    overflow-y: auto;
  }

  .manifest-raw-code {
    margin: 0;
    padding: 12px;
    font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
    font-size: 11px;
    background: #1a202c;
    color: #e2e8f0;
    border-radius: 6px;
    max-height: 260px;
    overflow-y: auto;
  }

  /* Modals */
  .modal-backdrop {
    position: fixed;
    inset: 0;
    background: rgba(0,0,0,0.4);
    z-index: 1050;
    backdrop-filter: blur(2px);
  }

  .gov-modal {
    position: fixed;
    top: 50%;
    left: 50%;
    transform: translate(-50%, -50%);
    width: 600px;
    max-width: 92vw;
    background: #ffffff;
    border-radius: 8px;
    box-shadow: 0 10px 25px rgba(0,0,0,0.15);
    z-index: 1051;
    display: flex;
    flex-direction: column;
    overflow: hidden;
  }

  .modal-header {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: 16px 20px;
    border-bottom: 1px solid var(--wa-border-soft, #e2e8f0);
  }

  .modal-header h2 {
    margin: 0;
    font-size: 17px;
    font-weight: 700;
  }

  .modal-tabs {
    display: flex;
    border-bottom: 1px solid var(--wa-border-soft, #e2e8f0);
    background: #f8fafc;
  }

  .modal-tab-btn {
    flex: 1;
    padding: 10px;
    border: none;
    background: transparent;
    font-size: 13px;
    font-weight: 600;
    color: #4a5568;
    cursor: pointer;
  }

  .modal-tab-btn.active {
    background: #ffffff;
    color: var(--wa-accent-strong, #008f96);
    border-bottom: 2px solid var(--wa-accent-strong, #008f96);
  }

  .modal-body {
    padding: 20px;
    display: flex;
    flex-direction: column;
    gap: 12px;
    max-height: 70vh;
    overflow-y: auto;
  }

  .modal-hint {
    margin: 0 0 6px 0;
    font-size: 13px;
    color: #718096;
  }

  .file-upload-row {
    display: flex;
    align-items: center;
    gap: 10px;
  }

  .file-label {
    position: relative;
    cursor: pointer;
  }

  .file-label input[type="file"] {
    position: absolute;
    inset: 0;
    opacity: 0;
    cursor: pointer;
  }

  .modal-footer {
    display: flex;
    align-items: center;
    justify-content: flex-end;
    gap: 10px;
    padding: 14px 20px;
    border-top: 1px solid var(--wa-border-soft, #e2e8f0);
    background: #f8fafc;
  }

  .cleanup-alert-box {
    background: #fffaf0;
    border: 1px solid #feebc8;
    border-radius: 6px;
    padding: 12px 16px;
    font-size: 13px;
    color: #744210;
  }

  .cleanup-alert-box ul {
    margin: 8px 0 0 0;
    padding-left: 18px;
    display: flex;
    flex-direction: column;
    gap: 6px;
  }

  .info-banner {
    background: #ebf8fa;
    border: 1px solid #b2e3e8;
    padding: 10px 14px;
    border-radius: 6px;
    font-size: 13px;
    color: #0c666c;
  }

  /* Utilities */
  .spin {
    animation: rotate 1s linear infinite;
  }
  @keyframes rotate {
    from { transform: rotate(0deg); }
    to { transform: rotate(360deg); }
  }

  .gov-loading-state, .gov-empty-state {
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    padding: 48px 24px;
    gap: 12px;
    background: #ffffff;
    border: 1px solid var(--wa-border-soft, #e2e8f0);
    border-radius: 8px;
    color: #718096;
  }

  .spinner {
    width: 28px;
    height: 28px;
    border: 3px solid #edf2f7;
    border-top-color: var(--wa-accent-strong, #008f96);
    border-radius: 50%;
    animation: rotate 0.8s linear infinite;
  }

  @media (max-width: 1024px) {
    .gov-workbench {
      padding: 0;
    }

    .gov-title-row,
    .filter-controls {
      align-items: stretch;
      flex-direction: column;
    }

    .gov-header-actions,
    .filter-controls {
      width: 100%;
      flex-wrap: wrap;
    }

    .search-box {
      max-width: none;
    }

  }

  @media (max-width: 760px) {
    .gov-workbench {
      gap: 14px;
      padding: 16px;
    }

    .gov-title-group h2 {
      font-size: 20px;
    }

    .gov-header-actions .btn:not(.icon-action),
    .filter-controls {
      width: 100%;
    }

    .gov-header-actions .btn,
    .gov-tab,
    .close-btn {
      min-height: 44px;
    }

    .gov-metrics-strip {
      gap: 16px;
      padding: 12px 14px;
    }

    .gov-metric-item {
      min-width: 132px;
    }

    .gov-notice {
      align-items: flex-start;
      gap: 10px;
    }

    .notice-actions {
      flex: none;
    }


    .gov-drawer {
      width: 100%;
      max-width: 100vw;
    }
  }

  @media (prefers-reduced-motion: reduce) {
    .spin,
    .spinner {
      animation: none;
    }
  }

  .empty-icon {
    font-size: 36px;
  }

  /* Run Bindings and Cold Archive Styles */
  .bindings-col {
    display: inline-flex;
    align-items: center;
  }
  .bindings-badge {
    display: inline-block;
    padding: 2px 7px;
    font-size: 11px;
    font-weight: 500;
    border-radius: 4px;
    background: #edf2f7;
    color: #4a5568;
    border: 1px solid #e2e8f0;
  }
  .bindings-badge.has-bindings {
    background: #e6fffa;
    color: #234e52;
    border-color: #b2f5ea;
    font-weight: 600;
  }
  .row-archived {
    background: #fafaf9;
    opacity: 0.85;
  }
  .status-tag.archived {
    background: #f7fafc;
    color: #4a5568;
    border: 1px solid #cbd5e0;
  }
  .audit-warning-box {
    background: #fffaf0;
    border: 1px solid #fbd38d;
    border-radius: 6px;
    padding: 10px 14px;
    font-size: 13px;
    color: #744210;
    margin-bottom: 12px;
  }
  .audit-warning-box .warning-title {
    font-weight: 600;
    margin-bottom: 4px;
  }
  .audit-warning-box p {
    margin: 0;
    line-height: 1.4;
  }
  .mode-select-group {
    display: flex;
    flex-direction: column;
    gap: 10px;
    margin-bottom: 12px;
  }
  .mode-select-card {
    display: flex;
    align-items: flex-start;
    gap: 12px;
    padding: 12px 14px;
    border: 1px solid var(--wa-border-soft, #e2e8f0);
    border-radius: 6px;
    background: #ffffff;
    cursor: pointer;
    transition: all 0.15s ease;
  }
  .mode-select-card:hover {
    border-color: #cbd5e0;
    background: #fcfcfc;
  }
  .mode-select-card.active {
    border-color: var(--wa-accent-strong, #008f96);
    background: #f0fdfa;
  }
  .mode-select-card input[type="radio"] {
    margin-top: 3px;
    cursor: pointer;
  }
  .mode-info {
    display: flex;
    flex-direction: column;
    gap: 4px;
    flex: 1;
  }
  .mode-head {
    display: flex;
    align-items: center;
    gap: 8px;
    font-size: 13px;
    color: #2d3748;
  }
  .mode-info p {
    margin: 0;
    font-size: 12px;
    color: #718096;
    line-height: 1.4;
  }
  .badge-recommended {
    background: #319795;
    color: #ffffff;
    font-size: 10px;
    font-weight: 600;
    padding: 1px 5px;
    border-radius: 3px;
  }

  /* Peer surfaces share the admin material; the workspace itself owns no elevation. */
  .gov-nav-tabs,
  .gov-metrics-strip,
  .gov-table-container,
  .rules-context-header,
  .rules-repo-list,
  .rules-editor-pane,
  .gov-loading-state,
  .gov-empty-state {
    border: 1px solid var(--wa-admin-list-surface-border);
    border-top-color: var(--wa-admin-list-surface-border-top);
    border-radius: var(--wa-admin-list-surface-radius);
    background: var(--wa-admin-list-surface-background);
    box-shadow: var(--wa-admin-list-surface-shadow);
    -webkit-backdrop-filter: var(--wa-admin-list-surface-filter);
    backdrop-filter: var(--wa-admin-list-surface-filter);
  }

  .gov-metrics-strip { padding: var(--wa-space-3) var(--wa-space-4); gap: var(--wa-space-6); }
  .gov-metric-label { text-transform: none; letter-spacing: normal; }
  .gov-metric-val strong { font-size: 18px; }
  .gov-title-row { align-items: flex-start; text-align: left; }
  .gov-title-group { min-width: 0; text-align: left; }
  .gov-tab { min-height: 36px; border-radius: var(--wa-radius-sm); }
  .gov-tab:hover { background: var(--wa-row-hover); }
  .gov-tab.active { background: var(--wa-accent-soft); color: var(--wa-accent-strong); box-shadow: none; }
  .gov-tab:focus-visible,
  .gov-table-container:focus-visible,
  .repo-row-select:focus-visible,
  .search-box:focus-within { outline: 2px solid var(--wa-border-focus); outline-offset: 2px; }
  .tab-icon { width: 16px; height: 16px; }
  .search-box { min-width: 0; width: 360px; max-width: 100%; box-sizing: border-box; background: var(--wa-surface-lift); }
  .search-box input { min-width: 0; background: transparent; }
  .search-box input::placeholder { color: var(--wa-text-muted); }
  .filter-controls { gap: var(--wa-space-3); }
  .gov-table th, .gov-table td { white-space: nowrap; }
  .gov-switch { flex: 0 0 36px; }
  .gov-drawer, .gov-modal {
    background: var(--wa-glass-panel-strong);
    -webkit-backdrop-filter: blur(20px) saturate(128%);
    backdrop-filter: blur(20px) saturate(128%);
  }
  .gov-loading-state, .gov-empty-state { color: var(--wa-text-muted); }
  .gov-notice > span { min-width: 0; overflow-wrap: anywhere; }

  @media (max-width: 760px) {
    .gov-workbench { padding: 0; }
    .gov-title-group h2 { font-size: 18px; }
    .gov-title-row { align-items: stretch; flex-direction: column; }
    .gov-metrics-strip { flex-wrap: wrap; gap: var(--wa-space-3); }
    .gov-metric-divider { display: none; }
    .filter-controls, .filter-field { width: 100%; }
    .filter-field { justify-content: space-between; }
    .filter-select-wrap { min-width: 0; flex: 1; }
    .filter-select-wrap :global(.select-trigger), .btn, .gov-tab { min-height: var(--wa-touch-h); }
    .search-box { width: 100%; max-width: none; min-height: var(--wa-touch-h); }
  }

  @media (prefers-reduced-transparency: reduce) {
    .gov-nav-tabs, .gov-metrics-strip, .gov-table-container, .rules-context-header, .rules-repo-list, .rules-editor-pane,
    .gov-loading-state, .gov-empty-state, .gov-drawer, .gov-modal {
      background: var(--wa-admin-list-surface-solid-background);
      -webkit-backdrop-filter: none;
      backdrop-filter: none;
    }
  }

  @supports not (backdrop-filter: blur(1px)) {
    .gov-nav-tabs, .gov-metrics-strip, .gov-table-container, .rules-context-header, .rules-repo-list, .rules-editor-pane,
    .gov-loading-state, .gov-empty-state, .gov-drawer, .gov-modal { background: var(--wa-admin-list-surface-solid-background); }
  }

  @media (prefers-reduced-motion: reduce) {
    .gov-drawer, .drawer-backdrop, .gov-modal, .gov-switch-handle { animation: none; transition: none; }
  }


  .inventory-note { margin: 16px 0; color: var(--wa-text-muted); font-size: 12px; line-height: 1.7; }
  .cap-install-meta { margin-top: 5px; max-width: 100%; white-space: normal; color: var(--wa-text-muted); font-size: 11px; line-height: 1.6; overflow-wrap: anywhere; }
  .cap-install-meta > span { display: block; }
  .cap-install-meta code { font-size: inherit; display: -webkit-box; -webkit-line-clamp: 3; line-clamp: 3; -webkit-box-orient: vertical; overflow: hidden; }
  .installation-evidence { padding: 18px 0; border-bottom: 1px solid var(--wa-border-divider); }
  .installation-evidence h3 { margin: 0; font-size: 14px; font-weight: 600; }
  .installation-evidence p { margin: 10px 0 0; color: var(--wa-text-muted); font-size: 12px; line-height: 1.7; }
  .installation-evidence dl { margin: 14px 0 0; display: grid; gap: 10px; font-size: 12px; }
  .installation-evidence dl > div { display: grid; grid-template-columns: 110px minmax(0, 1fr); gap: 12px; }
  .installation-evidence dt { color: var(--wa-text-muted); }
  .installation-evidence dd { margin: 0; overflow-wrap: anywhere; line-height: 1.6; }
  .installation-evidence code { font-size: 11px; }
  .plugin-runtime-heading { display: flex; align-items: center; justify-content: space-between; gap: 12px; }
  .component-source { color: var(--wa-text-muted); font-size: 11px; white-space: nowrap; }
  .doc-inc-card-head { flex-wrap: wrap; }
  .doc-inc-card-head .inc-badge { flex: none; white-space: nowrap; }
  .doc-inc-card-head code { min-width: 120px; flex: 1; overflow-wrap: anywhere; }
  @media (max-width: 480px) { .doc-inc-grid { grid-template-columns: 1fr; } .installation-evidence dl > div { grid-template-columns: 1fr; gap: 3px; } }

</style>
