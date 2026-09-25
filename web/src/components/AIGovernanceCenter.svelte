<script lang="ts">
  import { onMount, tick } from 'svelte';
  import { marked } from 'marked';
  import DOMPurify from 'dompurify';
  import CorpusSourceLibrary from './config/CorpusSourceLibrary.svelte';
  import CorpusCandidateReview from './config/CorpusCandidateReview.svelte';

  export let currentUserPermissions: string[] = [];
  export let activeSection = 'skills'; // 'skills' | 'prompts' | 'rules' | 'context'
  export let onSectionChange: (section: string) => void = () => {};

  function switchSection(sec: string) {
    activeSection = sec;
    onSectionChange(sec);
  }

  $: canManagePrompts = currentUserPermissions.includes('solution_prompt:manage');
  $: canReadContext = currentUserPermissions.includes('ai_context:read');
  $: canWriteContext = currentUserPermissions.includes('ai_context:write');

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

  interface IncludedComponent {
    key: string;
    kind: string;
    name: string;
    description: string;
    tools?: string[];
  }

  interface CapabilityItem {
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

  interface CapabilityDetailData {
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

  // Filter
  let searchKeyword = '';
  let filterKind = 'all';
  let filterStatus = 'all';

  // Derived filtered capability list for skills_only view
  $: displayCapabilities = capabilities.filter(c => {
    if (filterKind === 'skills_only') {
      return c.kind === 'skill' || c.is_top_level_skill;
    }
    return true;
  });

  // Detail Drawer state
  let selectedCapability: CapabilityItem | null = null;
  let capabilityDetail: CapabilityDetailData | null = null;
  let loadingDetail = false;
  let detailDrawerOpen = false;
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

  // --- Prompts Section State ---
  type PromptPurpose = 'solution_polish' | 'solution_compare_requirement' | 'solution_compare_compatibility' | 'code_review';
  type PromptVersion = {
    purpose: PromptPurpose;
    id: number; scope_type: 'global' | 'project'; scope_id: string; version: number;
    status: 'draft' | 'active' | 'retired'; name: string; system_prompt: string;
    content_hash: string; validation_status: 'untested' | 'passed' | 'failed';
    validation_summary?: string; validated_by?: string; validated_at?: string;
    created_by: string; activated_by: string; created_at: string; activated_at?: string;
  };
  const purposeOptions = [
    { value: 'solution_polish', label: '方案润色', meta: '需求方案生成与整理' },
    { value: 'solution_compare_requirement', label: '需求等价性对比', meta: '两轮对比的第一轮' },
    { value: 'solution_compare_compatibility', label: '方案兼容性对比', meta: '两轮对比的第二轮' },
    { value: 'code_review', label: '代码评审', meta: '证据驱动的代码审查' }
  ];
  let prompts: PromptVersion[] = [];
  let loadingPrompts = false;
  let selectedPurpose: PromptPurpose = 'solution_polish';
  let promptScopeType: 'global' | 'project' = 'global';
  let promptScopeID = '';
  let promptDraftName = '';
  let promptDraftSystem = '';
  let promptActivateOnSave = true;
  let promptNotice = '';
  let promptError = '';
  let savingPrompt = false;

  // --- Rules Section State ---
  interface RepoPolicyItem {
    project_id: string;
    repo_name: string;
    domain: string;
    scenario: string;
    knowledge_scope: string;
    sync_mrs: boolean;
    sync_commits: boolean;
    rules: string;
  }
  let repoPolicies: RepoPolicyItem[] = [];
  let loadingPolicies = false;
  let selectedPolicyRepo = '';
  let editingPolicy: RepoPolicyItem | null = null;
  let savingPolicy = false;
  let policyNotice = '';
  let policyError = '';

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
      throw new Error(data.message || data.error || text || `HTTP ${res.status}`);
    }
    return data;
  }

  // --- Capability Functions ---
  async function loadCapabilities() {
    loadingCapabilities = true;
    capabilityError = '';
    try {
      const params = new URLSearchParams();
      if (searchKeyword.trim()) params.set('search', searchKeyword.trim());
      if (filterKind !== 'all') params.set('kind', filterKind);
      if (filterStatus !== 'all') params.set('status', filterStatus);

      const qs = params.toString() ? `?${params.toString()}` : '';
      const resp = await api(`/api/agent-runtime/capabilities${qs}`);
      capabilities = resp.capabilities || [];
      totalDiskSizeBytes = resp.total_disk_size_bytes || 0;
      totalDiskSizeFormatted = resp.total_disk_size_formatted || '0 B';
    } catch (err: any) {
      capabilityError = err.message || '加载技能列表失败';
    } finally {
      loadingCapabilities = false;
    }
  }

  async function openDetail(cap: CapabilityItem) {
    selectedCapability = cap;
    detailDrawerOpen = true;
    loadingDetail = true;
    capabilityDetail = null;
    activeVersionTab = 0;
    drawerActiveView = 'skill_doc';
    copiedSkillDoc = false;
    try {
      const detail = await api<CapabilityDetailData>(`/api/agent-runtime/capabilities/${cap.capability_key}`);
      capabilityDetail = detail;
    } catch (err: any) {
      capabilityError = err.message || '加载技能详情失败';
    } finally {
      loadingDetail = false;
    }
  }

  async function toggleCapabilityStatus(cap: CapabilityItem) {
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
    deleteTarget = cap;
    deleteError = '';
    // 如果存在历史运行绑定，默认推荐冷归档；否则默认彻底清除
    deleteMode = (cap.bindings_count && cap.bindings_count > 0) ? 'cold_archive' : 'purge';
    deleteModalOpen = true;
  }

  async function confirmUninstall() {
    if (!deleteTarget) return;
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
        capabilityNotice = `技能 ${deleteTarget.capability_key} 已成功卸载并清理物理磁盘占用，已保留安装记录供回溯与恢复。`;
      }
      if (detailDrawerOpen && selectedCapability?.capability_key === deleteTarget.capability_key) {
        detailDrawerOpen = false;
      }
      await loadCapabilities();
    } catch (err: any) {
      deleteError = err.message || '操作失败';
    } finally {
      deleting = false;
    }
  }

  async function handleReinstall(cap: CapabilityItem) {
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
    importManifestText = sampleManifest;
    remoteInstallURL = '';
    remoteAutoActivate = true;
    importError = '';
    importTab = 'manifest';
    importModalOpen = true;
  }

  async function submitImport() {
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
    upgradeTarget = cap;
    upgradeManifestText = '';
    upgradeRemoteURL = '';
    upgradeError = '';
    upgradeMode = 'manifest';
    upgradeModalOpen = true;
  }

  async function submitUpgrade() {
    if (!upgradeTarget) return;
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

  // --- Prompts Functions ---
  async function loadPrompts() {
    loadingPrompts = true;
    promptError = '';
    try {
      const resp = await api('/api/solution-prompts');
      prompts = resp.items || [];
      initPromptEditor();
    } catch (err: any) {
      promptError = err.message || '加载提示词失败';
    } finally {
      loadingPrompts = false;
    }
  }

  function initPromptEditor() {
    const active = prompts.find(p => p.purpose === selectedPurpose && p.scope_type === 'global' && p.status === 'active');
    if (active) {
      promptDraftName = active.name;
      promptDraftSystem = active.system_prompt;
      promptScopeType = active.scope_type;
      promptScopeID = active.scope_id || '';
    } else {
      promptDraftName = `${selectedPurpose} 系统指令`;
      promptDraftSystem = '';
    }
  }

  async function savePromptVersion() {
    savingPrompt = true;
    promptError = '';
    promptNotice = '';
    try {
      await api('/api/solution-prompts', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          purpose: selectedPurpose,
          scope_type: promptScopeType,
          scope_id: promptScopeID,
          name: promptDraftName,
          system_prompt: promptDraftSystem,
          activate_on_save: promptActivateOnSave
        })
      });
      promptNotice = '提示词新版本已成功创建并保存！';
      await loadPrompts();
    } catch (err: any) {
      promptError = err.message || '保存提示词失败';
    } finally {
      savingPrompt = false;
    }
  }

  // --- Rules Functions ---
  async function loadRepoPolicies() {
    loadingPolicies = true;
    policyError = '';
    try {
      const repos = await api('/api/code-reviews/repos');
      repoPolicies = repos.repos || [];
      if (repoPolicies.length > 0 && !selectedPolicyRepo) {
        selectedPolicyRepo = repoPolicies[0].project_id;
        editingPolicy = { ...repoPolicies[0] };
      }
    } catch (err: any) {
      policyError = err.message || '加载评审工程规则失败';
    } finally {
      loadingPolicies = false;
    }
  }

  function selectRepoPolicy(repo: RepoPolicyItem) {
    selectedPolicyRepo = repo.project_id;
    editingPolicy = { ...repo };
  }

  async function saveRepoPolicy() {
    if (!editingPolicy) return;
    savingPolicy = true;
    policyNotice = '';
    policyError = '';
    try {
      await api('/api/code-reviews/policy', {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          project_id: editingPolicy.project_id,
          domain: editingPolicy.domain,
          scenario: editingPolicy.scenario,
          knowledge_scope: editingPolicy.knowledge_scope,
          sync_mrs: editingPolicy.sync_mrs,
          sync_commits: editingPolicy.sync_commits,
          rules: editingPolicy.rules
        })
      });
      policyNotice = `项目 ${editingPolicy.repo_name} 的工程审查规则已成功更新！`;
      await loadRepoPolicies();
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

  onMount(() => {
    void loadCapabilities();
    void loadPrompts();
    void loadRepoPolicies();
  });
</script>

<div class="gov-workbench">
  <!-- Top Header & Metrics Strip -->
  <header class="gov-header">
    <div class="gov-title-row">
      <div class="gov-title-group">
        <div class="gov-badge-spine">AGENT RUNTIME 2.0</div>
        <h1>AI 治理中心</h1>
        <p class="gov-subtitle">基于通用微内核规约，统一治理 Agent 技能生命周期、场景提示词工程、合规审查规则与领域设计语料。</p>
      </div>

      <div class="gov-header-actions">
        <button type="button" class="btn btn-secondary" on:click={openImportModal}>
          <svg width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
            <path d="M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4M7 10l5 5 5-5M12 15V3"/>
          </svg>
          导入 / 远程安装技能
        </button>
        <button type="button" class="btn btn-ghost" on:click={loadCapabilities} title="刷新技能与磁盘占用">
          <svg width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" class:spin={loadingCapabilities}>
            <path d="M23 4v6h-6M1 20v-6h6M3.51 9a9 9 0 0 1 14.85-3.36L23 10M1 14l4.64 4.36A9 9 0 0 0 20.49 15"/>
          </svg>
        </button>
      </div>
    </div>

    <!-- Metrics Strip (Finesse UI Product Register) -->
    <div class="gov-metrics-strip" aria-label="AI 治理关键指标">
      <div class="gov-metric-item">
        <span class="gov-metric-label">已安装技能 (Active / Total)</span>
        <div class="gov-metric-val">
          <strong>{capabilities.filter(c => c.status === 'active').length}</strong>
          <span class="gov-metric-sub">/ {capabilities.length} 项</span>
        </div>
      </div>

      <div class="gov-metric-divider"></div>

      <div class="gov-metric-item">
        <span class="gov-metric-label">物理磁盘占用</span>
        <div class="gov-metric-val">
          <strong class="highlight-cyan">{totalDiskSizeFormatted}</strong>
          <span class="gov-metric-sub">{totalDiskSizeBytes.toLocaleString()} 字节</span>
        </div>
      </div>

      <div class="gov-metric-divider"></div>

      <div class="gov-metric-item">
        <span class="gov-metric-label">微内核规约</span>
        <div class="gov-metric-val">
          <strong>L0-L3 切片</strong>
          <span class="gov-metric-sub">DAG 动态规划</span>
        </div>
      </div>

      <div class="gov-metric-divider"></div>

      <div class="gov-metric-item">
        <span class="gov-metric-label">安全状态</span>
        <div class="gov-metric-val">
          <span class="status-indicator good"></span>
          <span class="gov-metric-text">沙箱隔离生效</span>
        </div>
      </div>
    </div>

    <!-- Tab Navigation -->
    <nav class="gov-nav-tabs" role="tablist">
      <button
        role="tab"
        aria-selected={activeSection === 'skills'}
        class="gov-tab"
        class:active={activeSection === 'skills'}
        on:click={() => switchSection('skills')}
      >
        <span class="tab-icon">⚡</span>
        技能治理中心
        <span class="tab-count">{capabilities.length}</span>
      </button>

      <button
        role="tab"
        aria-selected={activeSection === 'prompts'}
        class="gov-tab"
        class:active={activeSection === 'prompts'}
        on:click={() => switchSection('prompts')}
      >
        <span class="tab-icon">📝</span>
        提示词管理
        <span class="tab-count">{prompts.length}</span>
      </button>

      <button
        role="tab"
        aria-selected={activeSection === 'rules'}
        class="gov-tab"
        class:active={activeSection === 'rules'}
        on:click={() => switchSection('rules')}
      >
        <span class="tab-icon">⚖️</span>
        规则与工程标准
      </button>

      <button
        role="tab"
        aria-selected={activeSection === 'context'}
        class="gov-tab"
        class:active={activeSection === 'context'}
        on:click={() => switchSection('context')}
      >
        <span class="tab-icon">📚</span>
        上下文与语料库
      </button>
    </nav>
  </header>

  <!-- Global Alerts -->
  {#if capabilityNotice}
    <div class="gov-notice" role="status">
      <span>{capabilityNotice}</span>
      <button type="button" class="close-btn" on:click={() => capabilityNotice = ''}>×</button>
    </div>
  {/if}
  {#if capabilityError}
    <div class="gov-notice error" role="alert">
      <span>{capabilityError}</span>
      <button type="button" class="close-btn" on:click={() => capabilityError = ''}>×</button>
    </div>
  {/if}

  <!-- TAB 1: 技能治理中心 (Skills) -->
  {#if activeSection === 'skills'}
    <section class="gov-pane" aria-label="技能治理列表">
      <!-- Filter Bar -->
      <div class="gov-filter-bar">
        <div class="search-box">
          <svg width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
            <circle cx="11" cy="11" r="8"/><path d="m21 21-4.3-4.3"/>
          </svg>
          <input
            type="text"
            placeholder="搜索技能标识、Owner 或描述..."
            bind:value={searchKeyword}
            on:input={() => loadCapabilities()}
          />
        </div>

        <div class="filter-controls">
          <label class="filter-label">
            类别:
            <div class="custom-select-wrap">
              <select class="modern-select" bind:value={filterKind} on:change={() => loadCapabilities()}>
                <option value="all">全类别 (含内置组件)</option>
                <option value="skills_only">⭐ 仅看业务技能 (Skills)</option>
                <option value="skill">Skill 业务技能</option>
                <option value="plugin">Plugin 底层插件</option>
                <option value="context_provider">Context 语料源</option>
                <option value="mcp">MCP 扩展工具</option>
                <option value="policy">Policy 治理策略</option>
              </select>
              <svg class="select-chevron" width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5">
                <path d="m6 9 6 6 6-6"/>
              </svg>
            </div>
          </label>

          <label class="filter-label">
            状态:
            <div class="custom-select-wrap">
              <select class="modern-select" bind:value={filterStatus} on:change={() => loadCapabilities()}>
                <option value="all">全状态</option>
                <option value="active">已启用 (Active)</option>
                <option value="disabled">已禁用 (Disabled)</option>
                <option value="archived">已冷归档 (Archived)</option>
                <option value="uninstalled">已卸载 (Uninstalled)</option>
              </select>
              <svg class="select-chevron" width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5">
                <path d="m6 9 6 6 6-6"/>
              </svg>
            </div>
          </label>
        </div>
      </div>

      <!-- Skills Table / Card List -->
      {#if loadingCapabilities}
        <div class="gov-loading-state">
          <div class="spinner"></div>
          <span>正在检索微内核能力注册表与磁盘统计...</span>
        </div>
      {:else if displayCapabilities.length === 0}
        <div class="gov-empty-state">
          <div class="empty-icon">📦</div>
          <h3>未检索到匹配的微内核技能</h3>
          <p>当前过滤条件下没有已注册能力，您可以点击“导入 / 远程安装技能”快速部署通用标准 Capability。</p>
          <button type="button" class="btn btn-primary" on:click={openImportModal}>立即导入技能</button>
        </div>
      {:else}
        <div class="gov-table-container">
          <table class="gov-table">
            <thead>
              <tr>
                <th style="width: 250px;">技能与内置组件标识</th>
                <th style="width: 80px;">类型</th>
                <th style="width: 140px;">状态 / 启用滑块</th>
                <th style="width: 70px;">版本</th>
                <th style="width: 90px;">关联运行</th>
                <th style="width: 100px;">物理磁盘</th>
                <th>责任方 / 描述</th>
                <th style="min-width: 230px; width: 240px; text-align: right; white-space: nowrap;">生命周期操作</th>
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
                        {#if cap.kind === 'skill' || cap.is_top_level_skill}
                          <span class="badge badge-top-skill" title="面向特定业务目标的顶层智能技能">⭐ 业务技能</span>
                        {/if}
                        {#if cap.sensitivity === 'restricted'}
                          <span class="badge badge-restricted">安全限制</span>
                        {/if}
                      </div>

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
                      {:else if cap.parent_skill_key}
                        <div class="cap-parent-ref-block">
                          <span class="parent-ref-badge" title="本组件内聚在 {cap.parent_skill_key} 技能中">
                            🏷️ 包含于: <strong>{cap.parent_skill_key}</strong>
                          </span>
                        </div>
                      {/if}
                    </div>
                  </td>

                  <!-- Kind -->
                  <td>
                    <span class="cap-kind-badge {cap.kind}">{cap.kind}</span>
                  </td>

                  <!-- Switch Status -->
                  <td>
                    {#if cap.status === 'uninstalled'}
                      <span class="status-tag uninstalled">已卸载 (可恢复)</span>
                    {:else if cap.status === 'archived'}
                      <span class="status-tag archived">已冷归档 (保留审计)</span>
                    {:else}
                      <div class="switch-container">
                        <button
                          type="button"
                          role="switch"
                          aria-checked={cap.status === 'active'}
                          class="gov-switch"
                          class:checked={cap.status === 'active'}
                          disabled={togglingStatusKey === cap.capability_key}
                          on:click={() => toggleCapabilityStatus(cap)}
                          title={cap.status === 'active' ? '点击禁用该技能' : '点击启用该技能'}
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
                      <span class="cap-owner">{cap.owner || 'system'}</span>
                      <p class="cap-desc" title={cap.description}>{cap.description || '暂无详细描述'}</p>
                    </div>
                  </td>

                  <!-- Actions (Strict No-Wrap) -->
                  <td style="text-align: right; white-space: nowrap;">
                    <div class="action-btn-group">
                      <button type="button" class="btn btn-sm btn-ghost" on:click={() => openDetail(cap)}>
                        查看详情 / SKILL.md
                      </button>

                      {#if cap.status === 'uninstalled' || cap.status === 'archived'}
                        <button
                          type="button"
                          class="btn btn-sm btn-primary"
                          disabled={reinstallingKey === cap.capability_key}
                          on:click={() => handleReinstall(cap)}
                        >
                          {reinstallingKey === cap.capability_key ? '恢复中…' : '重新安装'}
                        </button>
                      {:else}
                        <button type="button" class="btn btn-sm btn-ghost" on:click={() => openUpgradeModal(cap)}>
                          升级
                        </button>
                        <button type="button" class="btn btn-sm btn-danger-ghost" on:click={() => openDeleteConfirm(cap)}>
                          卸载清理
                        </button>
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
  {#if activeSection === 'prompts'}
    <section class="gov-pane" aria-label="提示词治理">
      <div class="prompt-split-workbench">
        <!-- Sidebar Purpose Selector -->
        <aside class="prompt-purpose-nav">
          <h3>场景提示词分类</h3>
          <div class="purpose-nav-list">
            {#each purposeOptions as opt}
              <button
                type="button"
                class="purpose-nav-item"
                class:active={selectedPurpose === opt.value}
                on:click={() => { selectedPurpose = opt.value as PromptPurpose; initPromptEditor(); }}
              >
                <div class="item-title">{opt.label}</div>
                <div class="item-meta">{opt.meta}</div>
              </button>
            {/each}
          </div>
        </aside>

        <!-- Main Prompt Editor & Version List -->
        <div class="prompt-editor-area">
          <div class="pane-header">
            <div>
              <h2>{purposeOptions.find(o => o.value === selectedPurpose)?.label} 指令治理</h2>
              <p class="subtitle">管理系统初审、复核与比对阶段的核心 LLM System Prompt 版本。</p>
            </div>
          </div>

          {#if promptNotice}
            <div class="gov-notice">{promptNotice}</div>
          {/if}
          {#if promptError}
            <div class="gov-notice error">{promptError}</div>
          {/if}

          <!-- Editor Form -->
          <div class="prompt-form">
            <div class="form-row">
              <label>
                版本名称:
                <input type="text" bind:value={promptDraftName} placeholder="例如：v2.1 强化边界证据校验" />
              </label>

              <label>
                作用范围:
                <select bind:value={promptScopeType}>
                  <option value="global">全局默认 (Global)</option>
                  <option value="project">项目覆盖 (Project)</option>
                </select>
              </label>

              {#if promptScopeType === 'project'}
                <label>
                  项目标识:
                  <input type="text" bind:value={promptScopeID} placeholder="输入 ProjectID" />
                </label>
              {/if}
            </div>

            <div class="form-group">
              <label for="sys-prompt-editor">系统指令模板 (System Prompt):</label>
              <textarea
                id="sys-prompt-editor"
                rows="14"
                bind:value={promptDraftSystem}
                placeholder="在此输入或调整 System Prompt 提示词..."
              ></textarea>
            </div>

            <div class="form-actions-row">
              <label class="checkbox-label">
                <input type="checkbox" bind:checked={promptActivateOnSave} />
                保存后立即激活此版本为线上生效版本
              </label>

              <button
                type="button"
                class="btn btn-primary"
                disabled={savingPrompt || !promptDraftSystem.trim()}
                on:click={savePromptVersion}
              >
                {savingPrompt ? '保存中…' : '发布新版本'}
              </button>
            </div>
          </div>

          <!-- History Versions -->
          <div class="prompt-history-block">
            <h3>历史版本归档 ({prompts.filter(p => p.purpose === selectedPurpose).length})</h3>
            <div class="history-list">
              {#each prompts.filter(p => p.purpose === selectedPurpose) as ver}
                <div class="history-card" class:active-ver={ver.status === 'active'}>
                  <div class="history-card-header">
                    <span class="ver-badge">v{ver.version}</span>
                    <strong>{ver.name}</strong>
                    <span class="status-tag {ver.status}">{ver.status === 'active' ? '当前生效' : '历史版本'}</span>
                    <span class="hash-tag font-mono">{ver.content_hash.slice(0, 8)}</span>
                  </div>
                  <pre class="history-preview">{ver.system_prompt.slice(0, 180)}...</pre>
                </div>
              {/each}
            </div>
          </div>
        </div>
      </div>
    </section>
  {/if}

  <!-- TAB 3: 规则与工程标准 (Rules) -->
  {#if activeSection === 'rules'}
    <section class="gov-pane" aria-label="工程审查规则治理">
      <div class="rules-workbench">
        <!-- Repos List -->
        <aside class="rules-repo-list">
          <h3>项目仓库 ({repoPolicies.length})</h3>
          {#if loadingPolicies}
            <p>加载中...</p>
          {:else}
            {#each repoPolicies as r}
              <button
                type="button"
                class="repo-nav-item"
                class:active={selectedPolicyRepo === r.project_id}
                on:click={() => selectRepoPolicy(r)}
              >
                <div class="repo-name">{r.repo_name}</div>
                <div class="repo-meta">ID: {r.project_id} · {r.domain || '通用'}</div>
              </button>
            {/each}
          {/if}
        </aside>

        <!-- Policy Editor -->
        <div class="rules-editor-pane">
          {#if editingPolicy}
            <div class="pane-header">
              <h2>{editingPolicy.repo_name} 代码审查标准与策略</h2>
              <p class="subtitle">定义仓库级自动化评审的场景、知识库检索边界与自定义合规规则。</p>
            </div>

            {#if policyNotice}
              <div class="gov-notice">{policyNotice}</div>
            {/if}
            {#if policyError}
              <div class="gov-notice error">{policyError}</div>
            {/if}

            <div class="policy-form">
              <div class="form-row">
                <label>
                  领域分类:
                  <select bind:value={editingPolicy.domain}>
                    <option value="general">通用工程 (General)</option>
                    <option value="fms">FMS 车辆/调度业务 (FMS)</option>
                  </select>
                </label>

                <label>
                  场景分类:
                  <select bind:value={editingPolicy.scenario}>
                    <option value="general">通用场景</option>
                    <option value="dispatch">调度协同场景</option>
                    <option value="vehicle">车辆状态流转</option>
                    <option value="yard">堆场作业控制</option>
                  </select>
                </label>

                <label>
                  知识库检索作用域:
                  <input type="text" bind:value={editingPolicy.knowledge_scope} placeholder="例如：fms:dispatch" />
                </label>
              </div>

              <div class="form-row">
                <label class="checkbox-label">
                  <input type="checkbox" bind:checked={editingPolicy.sync_mrs} />
                  自动为 Merge Request 发布审查评论
                </label>
                <label class="checkbox-label">
                  <input type="checkbox" bind:checked={editingPolicy.sync_commits} />
                  自动为独立 Commit 发布审查评论
                </label>
              </div>

              <div class="form-group">
                <label for="repo-rules-input">仓库自定义工程规则 (Markdown 格式):</label>
                <textarea
                  id="repo-rules-input"
                  rows="8"
                  bind:value={editingPolicy.rules}
                  placeholder="- 必须校验车辆反馈完整证据&#10;- 禁止使用空字符串替代完成状态"
                ></textarea>
              </div>

              <div class="form-actions-row">
                <button
                  type="button"
                  class="btn btn-primary"
                  disabled={savingPolicy}
                  on:click={saveRepoPolicy}
                >
                  {savingPolicy ? '保存中…' : '保存规则策略'}
                </button>
              </div>
            </div>
          {:else}
            <p>请选择一个仓库以查看并配置规则。</p>
          {/if}
        </div>
      </div>
    </section>
  {/if}

  <!-- TAB 4: 上下文与语料库 (Context) -->
  {#if activeSection === 'context'}
    <section class="gov-pane" aria-label="设计语料库治理">
      <CorpusSourceLibrary {currentUserPermissions} aiReady={true} />
    </section>
  {/if}
</div>

<!-- ================= MODALS & DRAWERS ================= -->

<!-- 1. Capability Detail Drawer -->
{#if detailDrawerOpen}
  <div class="drawer-backdrop" role="presentation" on:click={() => detailDrawerOpen = false}></div>
  <aside class="gov-drawer" role="dialog" aria-modal="true" aria-label="技能详细规约">
    <div class="drawer-header">
      <div>
        <span class="drawer-kicker">{selectedCapability?.kind?.toUpperCase()} 规约详情</span>
        <h2>{selectedCapability?.capability_key}</h2>
      </div>
      <button type="button" class="close-btn" on:click={() => detailDrawerOpen = false}>×</button>
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
            <span class="label">物理磁盘占用</span>
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
                  <strong>本技能所内聚包含的组件与插件 ({capabilityDetail.included_components.length})</strong>
                </div>
                <div class="doc-inc-grid">
                  {#each capabilityDetail.included_components as comp}
                    <div class="doc-inc-card">
                      <div class="doc-inc-card-head">
                        <span class="inc-badge {comp.kind}">{comp.kind === 'plugin' ? '🧩 插件' : '📚 语料源'}</span>
                        <code>{comp.key}</code>
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

                  {#if curVer.status !== 'active'}
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
                  {:else}
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
{/if}

<!-- 2. Import / Remote Install Modal -->
{#if importModalOpen}
  <div class="modal-backdrop" role="presentation" on:click={() => importModalOpen = false}></div>
  <div class="gov-modal" role="dialog" aria-modal="true" aria-label="安装或导入能力">
    <div class="modal-header">
      <h2>新增 / 导入 Agent 微内核技能</h2>
      <button type="button" class="close-btn" on:click={() => importModalOpen = false}>×</button>
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
{#if upgradeModalOpen && upgradeTarget}
  <div class="modal-backdrop" role="presentation" on:click={() => upgradeModalOpen = false}></div>
  <div class="gov-modal" role="dialog" aria-modal="true" aria-label="技能版本升级">
    <div class="modal-header">
      <h2>升级技能：{upgradeTarget.capability_key}</h2>
      <button type="button" class="close-btn" on:click={() => upgradeModalOpen = false}>×</button>
    </div>

    <div class="modal-tabs">
      <button
        type="button"
        class="modal-tab-btn"
        class:active={upgradeMode === 'manifest'}
        on:click={() => upgradeMode = 'manifest'}
      >
        提供新版 Manifest
      </button>
      <button
        type="button"
        class="modal-tab-btn"
        class:active={upgradeMode === 'remote'}
        on:click={() => upgradeMode = 'remote'}
      >
        远程拉取升级
      </button>
    </div>

    <div class="modal-body">
      {#if upgradeError}
        <div class="gov-notice error">{upgradeError}</div>
      {/if}

      <div class="info-banner">
        当前最新版本：<strong>v{upgradeTarget.latest_version || 1}</strong>。升级后系统将自动生成新版本记录并原子切换激活，历史版本将安全标记为 retired。
      </div>

      {#if upgradeMode === 'manifest'}
        <label for="upgrade-manifest-editor">新版本 Manifest (版本号建议为 v{ (upgradeTarget.latest_version || 1) + 1 }):</label>
        <textarea
          id="upgrade-manifest-editor"
          rows="12"
          class="code-editor"
          bind:value={upgradeManifestText}
          placeholder="在此输入升级后的新版本 Manifest (YAML 或 JSON)..."
        ></textarea>
      {:else}
        <label for="upgrade-remote-url">远程升级 Manifest URL:</label>
        <input
          id="upgrade-remote-url"
          type="url"
          bind:value={upgradeRemoteURL}
          placeholder="https://example.com/capabilities/fms-review-v2.yaml"
        />
      {/if}
    </div>

    <div class="modal-footer">
      <button type="button" class="btn btn-ghost" on:click={() => upgradeModalOpen = false}>取消</button>
      <button type="button" class="btn btn-primary" disabled={upgrading} on:click={submitUpgrade}>
        {upgrading ? '正在升级部署…' : '确认升级'}
      </button>
    </div>
  </div>
{/if}

<!-- 4. Delete & Cleanup / Cold Archive Confirmation Modal -->
{#if deleteModalOpen && deleteTarget}
  <div class="modal-backdrop" role="presentation" on:click={() => deleteModalOpen = false}></div>
  <div class="gov-modal modal-danger" role="dialog" aria-modal="true" aria-label="卸载或冷归档技能">
    <div class="modal-header">
      <h2>确认{deleteMode === 'cold_archive' ? '冷归档' : '卸载清理'}技能？</h2>
      <button type="button" class="close-btn" on:click={() => deleteModalOpen = false}>×</button>
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
    display: flex;
    flex-direction: column;
    gap: 18px;
    padding: 24px 32px;
    min-height: 100%;
    color: var(--wa-text-main, #293847);
    font-size: 14px;
    background: #f7fafc;
  }

  /* Header */
  .gov-header {
    display: flex;
    flex-direction: column;
    gap: 16px;
    border-bottom: 1px solid var(--wa-border, #e2e8f0);
    padding-bottom: 14px;
  }

  .gov-title-row {
    display: flex;
    align-items: flex-start;
    justify-content: space-between;
    gap: 16px;
  }

  .gov-badge-spine {
    font-size: 11px;
    font-weight: 700;
    letter-spacing: 0.08em;
    color: var(--wa-focus-ring, #008f96);
    text-transform: uppercase;
    margin-bottom: 4px;
  }

  .gov-title-group h1 {
    margin: 0;
    font-size: 24px;
    font-weight: 750;
    color: var(--wa-text-strong, #1a202c);
    letter-spacing: -0.02em;
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
    gap: 10px;
  }

  /* Metrics Strip */
  .gov-metrics-strip {
    display: flex;
    align-items: center;
    background: #ffffff;
    border: 1px solid var(--wa-border, #e2e8f0);
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
    color: var(--wa-focus-ring, #008f96) !important;
  }

  .gov-metric-sub {
    font-size: 12px;
    color: var(--wa-text-muted, #a0aec0);
  }

  .gov-metric-divider {
    width: 1px;
    height: 32px;
    background: var(--wa-border, #edf2f7);
  }

  .status-indicator {
    width: 8px;
    height: 8px;
    border-radius: 50%;
    display: inline-block;
  }

  .status-indicator.good {
    background: #38a169;
    box-shadow: 0 0 0 2px rgba(56, 161, 105, 0.2);
  }

  /* Tab Navigation */
  .gov-nav-tabs {
    display: flex;
    gap: 8px;
    margin-top: 4px;
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
  }

  .gov-tab:hover {
    background: #edf2f7;
    color: var(--wa-text-strong, #1a202c);
  }

  .gov-tab.active {
    background: #ffffff;
    color: var(--wa-focus-ring, #008f96);
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
    color: var(--wa-focus-ring, #008f96);
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
    display: flex;
    flex-direction: column;
    gap: 14px;
  }

  /* Filter bar */
  .gov-filter-bar {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 16px;
  }

  .search-box {
    display: flex;
    align-items: center;
    gap: 8px;
    background: #ffffff;
    border: 1px solid var(--wa-border, #cbd5e0);
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
    gap: 14px;
  }

  .filter-label {
    display: flex;
    align-items: center;
    gap: 8px;
    font-size: 13px;
    color: var(--wa-text-muted, #4a5568);
    font-weight: 500;
  }

  .custom-select-wrap {
    position: relative;
    display: inline-flex;
    align-items: center;
  }

  .modern-select {
    appearance: none;
    -webkit-appearance: none;
    -moz-appearance: none;
    height: 32px;
    padding: 0 28px 0 10px;
    font-size: 13px;
    font-weight: 500;
    color: var(--wa-text-strong, #1a202c);
    background: #ffffff;
    border: 1px solid var(--wa-border, #cbd5e0);
    border-radius: 6px;
    cursor: pointer;
    line-height: 30px;
    transition: all 0.15s ease;
  }

  .modern-select:hover {
    border-color: #a0aec0;
    background: #fcfdfe;
  }

  .modern-select:focus {
    outline: none;
    border-color: var(--wa-focus-ring, #008f96);
    box-shadow: 0 0 0 3px rgba(0, 143, 150, 0.15);
  }

  .select-chevron {
    position: absolute;
    right: 9px;
    pointer-events: none;
    color: var(--wa-text-muted, #718096);
    transition: transform 0.15s ease;
  }

  /* Table */
  .gov-table-container {
    background: #ffffff;
    border: 1px solid var(--wa-border, #e2e8f0);
    border-radius: 8px;
    overflow: hidden;
    box-shadow: 0 1px 2px rgba(0,0,0,0.02);
  }

  .gov-table {
    width: 100%;
    border-collapse: collapse;
    font-size: 13.5px;
  }

  .gov-table th {
    text-align: left;
    padding: 12px 16px;
    background: #f8fafc;
    border-bottom: 1px solid var(--wa-border, #e2e8f0);
    font-size: 12px;
    font-weight: 650;
    color: var(--wa-text-muted, #718096);
    letter-spacing: 0.03em;
  }

  .gov-table td {
    padding: 14px 16px;
    border-bottom: 1px solid var(--wa-border, #edf2f7);
    vertical-align: middle;
  }

  .gov-table tbody tr:hover {
    background: #fafcff;
  }

  .row-uninstalled {
    opacity: 0.7;
    background: #fdfdfd;
  }

  .cap-identity {
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
    color: var(--wa-text-strong, #1a202c);
    font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
    font-size: 13px;
  }

  .badge-top-skill {
    background: #fefcbf;
    color: #744210;
    border: 1px solid #faf089;
    font-size: 11px;
    font-weight: 700;
    padding: 1px 6px;
    border-radius: 4px;
    letter-spacing: 0.02em;
    white-space: nowrap;
  }

  .cap-included-block {
    display: flex;
    align-items: center;
    gap: 5px;
    flex-wrap: wrap;
    font-size: 11px;
    background: #f8fafc;
    border: 1px dashed var(--wa-border, #e2e8f0);
    padding: 3px 8px;
    border-radius: 4px;
  }

  .inc-title {
    color: var(--wa-text-muted, #718096);
    font-weight: 600;
    white-space: nowrap;
  }

  .inc-badge {
    padding: 1px 6px;
    border-radius: 3px;
    font-size: 11px;
    font-weight: 550;
    font-family: ui-monospace, SFMono-Regular, monospace;
    white-space: nowrap;
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

  .cap-parent-ref-block {
    display: flex;
    align-items: center;
    font-size: 11px;
  }

  .parent-ref-badge {
    color: #4a5568;
    background: #edf2f7;
    padding: 2px 7px;
    border-radius: 4px;
    font-size: 11px;
    white-space: nowrap;
  }

  .parent-ref-badge strong {
    color: #2b6cb0;
  }

  .cap-kind-badge {
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
    background: var(--wa-focus-ring, #008f96);
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
    display: flex;
    flex-direction: column;
    gap: 2px;
  }

  .cap-owner {
    font-size: 11.5px;
    font-weight: 600;
    color: var(--wa-text-muted, #718096);
  }

  .cap-desc {
    margin: 0;
    color: var(--wa-text-main, #4a5568);
    font-size: 12.5px;
    line-height: 1.4;
    max-width: 380px;
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
    background: var(--wa-focus-ring, #008f96);
    color: #ffffff;
  }

  .btn-primary:hover {
    background: #007a80;
  }

  .btn-secondary {
    background: #ffffff;
    color: var(--wa-text-strong, #2d3748);
    border: 1px solid var(--wa-border, #cbd5e0);
  }

  .btn-secondary:hover {
    background: #f7fafc;
    border-color: #a0aec0;
  }

  .btn-ghost {
    background: transparent;
    color: var(--wa-text-main, #4a5568);
    border: 1px solid var(--wa-border, #e2e8f0);
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
    border: none;
    background: transparent;
    font-size: 18px;
    color: #a0aec0;
    cursor: pointer;
  }

  /* Prompts layout */
  .prompt-split-workbench {
    display: grid;
    grid-template-columns: 240px 1fr;
    gap: 20px;
    background: #ffffff;
    border: 1px solid var(--wa-border, #e2e8f0);
    border-radius: 8px;
    padding: 20px;
  }

  .prompt-purpose-nav h3 {
    margin: 0 0 12px 0;
    font-size: 13px;
    text-transform: uppercase;
    color: var(--wa-text-muted, #718096);
  }

  .purpose-nav-list {
    display: flex;
    flex-direction: column;
    gap: 6px;
  }

  .purpose-nav-item {
    text-align: left;
    padding: 10px 12px;
    border-radius: 6px;
    border: 1px solid transparent;
    background: transparent;
    cursor: pointer;
    transition: all 0.15s ease;
  }

  .purpose-nav-item:hover {
    background: #f7fafc;
  }

  .purpose-nav-item.active {
    background: #ebf8fa;
    border-color: #b2e3e8;
  }

  .purpose-nav-item .item-title {
    font-weight: 650;
    color: var(--wa-text-strong, #1a202c);
    font-size: 13.5px;
  }

  .purpose-nav-item .item-meta {
    font-size: 11.5px;
    color: var(--wa-text-muted, #718096);
    margin-top: 2px;
  }

  .prompt-editor-area {
    display: flex;
    flex-direction: column;
    gap: 16px;
  }

  .prompt-form {
    display: flex;
    flex-direction: column;
    gap: 14px;
  }

  .form-row {
    display: flex;
    gap: 14px;
    flex-wrap: wrap;
  }

  .form-row label {
    display: flex;
    flex-direction: column;
    gap: 4px;
    font-size: 12.5px;
    font-weight: 600;
    color: #4a5568;
    flex: 1;
  }

  .form-row input, .form-row select {
    border: 1px solid var(--wa-border, #cbd5e0);
    border-radius: 6px;
    padding: 8px 10px;
    font-size: 13.5px;
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

  .form-group textarea, textarea.code-editor {
    border: 1px solid var(--wa-border, #cbd5e0);
    border-radius: 6px;
    padding: 12px;
    font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
    font-size: 12.5px;
    line-height: 1.5;
    background: #fdfdfd;
    resize: vertical;
  }

  .form-actions-row {
    display: flex;
    align-items: center;
    justify-content: space-between;
  }

  .checkbox-label {
    display: inline-flex;
    align-items: center;
    gap: 8px;
    font-size: 13px;
    font-weight: 500;
    cursor: pointer;
  }

  .prompt-history-block h3 {
    margin: 20px 0 10px 0;
    font-size: 14px;
    font-weight: 700;
    color: var(--wa-text-strong, #1a202c);
  }

  .history-list {
    display: flex;
    flex-direction: column;
    gap: 10px;
  }

  .history-card {
    border: 1px solid var(--wa-border, #e2e8f0);
    border-radius: 6px;
    padding: 12px;
    background: #f8fafc;
  }

  .history-card.active-ver {
    border-color: #b2e3e8;
    background: #f4fdfe;
  }

  .history-card-header {
    display: flex;
    align-items: center;
    gap: 10px;
    margin-bottom: 6px;
  }

  .ver-badge {
    padding: 2px 6px;
    border-radius: 4px;
    background: #cbd5e0;
    font-weight: 700;
    font-size: 11px;
  }

  .hash-tag {
    font-size: 11px;
    color: #a0aec0;
  }

  .history-preview {
    margin: 0;
    font-size: 11.5px;
    color: #4a5568;
    white-space: pre-wrap;
    background: #ffffff;
    padding: 8px;
    border-radius: 4px;
    border: 1px solid #edf2f7;
  }

  /* Rules layout */
  .rules-workbench {
    display: grid;
    grid-template-columns: 240px 1fr;
    gap: 20px;
    background: #ffffff;
    border: 1px solid var(--wa-border, #e2e8f0);
    border-radius: 8px;
    padding: 20px;
  }

  .rules-repo-list h3 {
    margin: 0 0 12px 0;
    font-size: 13px;
    text-transform: uppercase;
    color: var(--wa-text-muted, #718096);
  }

  .repo-nav-item {
    text-align: left;
    width: 100%;
    padding: 10px 12px;
    border-radius: 6px;
    border: 1px solid transparent;
    background: transparent;
    cursor: pointer;
    margin-bottom: 4px;
  }

  .repo-nav-item.active {
    background: #ebf8fa;
    border-color: #b2e3e8;
  }

  .repo-name {
    font-weight: 650;
    color: var(--wa-text-strong, #1a202c);
  }

  .repo-meta {
    font-size: 11px;
    color: #a0aec0;
  }

  .policy-form {
    display: flex;
    flex-direction: column;
    gap: 14px;
  }

  /* Drawer */
  .drawer-backdrop {
    position: fixed;
    inset: 0;
    background: rgba(0,0,0,0.3);
    z-index: 1000;
    backdrop-filter: blur(1px);
  }

  .gov-drawer {
    position: fixed;
    top: 0;
    right: 0;
    bottom: 0;
    width: 680px;
    max-width: 90vw;
    background: #ffffff;
    z-index: 1001;
    box-shadow: -4px 0 24px rgba(0,0,0,0.12);
    display: flex;
    flex-direction: column;
    overflow: hidden;
  }

  .drawer-header {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: 20px 24px;
    border-bottom: 1px solid var(--wa-border, #e2e8f0);
  }

  .drawer-kicker {
    font-size: 11px;
    font-weight: 700;
    letter-spacing: 0.06em;
    color: var(--wa-focus-ring, #008f96);
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
    border: 1px solid var(--wa-border, #e2e8f0);
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
    color: #a0aec0;
  }

  /* Dual View Tabs in Drawer */
  .drawer-view-tabs {
    display: flex;
    gap: 8px;
    padding: 4px;
    background: #f1f5f9;
    border-radius: 8px;
    border: 1px solid var(--wa-border, #e2e8f0);
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
    color: var(--wa-focus-ring, #008f96);
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
    border-bottom: 1px solid var(--wa-border, #e2e8f0);
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
    border: 1px solid var(--wa-border, #e2e8f0);
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
    border: 1px solid var(--wa-border, #e2e8f0);
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
    border-bottom: 1px solid var(--wa-border, #e2e8f0);
    padding-bottom: 8px;
    margin-bottom: 14px;
  }

  .ver-tab-btn {
    border: 1px solid var(--wa-border, #e2e8f0);
    background: #f8fafc;
    padding: 6px 12px;
    border-radius: 6px;
    font-size: 12px;
    font-weight: 600;
    cursor: pointer;
  }

  .ver-tab-btn.active {
    background: var(--wa-focus-ring, #008f96);
    color: #ffffff;
    border-color: var(--wa-focus-ring, #008f96);
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
    border: 1px solid var(--wa-border, #e2e8f0);
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
    color: var(--wa-focus-ring, #008f96);
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
    border-bottom: 1px solid var(--wa-border, #e2e8f0);
  }

  .modal-header h2 {
    margin: 0;
    font-size: 17px;
    font-weight: 700;
  }

  .modal-tabs {
    display: flex;
    border-bottom: 1px solid var(--wa-border, #e2e8f0);
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
    color: var(--wa-focus-ring, #008f96);
    border-bottom: 2px solid var(--wa-focus-ring, #008f96);
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
    border-top: 1px solid var(--wa-border, #e2e8f0);
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
    border: 1px solid var(--wa-border, #e2e8f0);
    border-radius: 8px;
    color: #718096;
  }

  .spinner {
    width: 28px;
    height: 28px;
    border: 3px solid #edf2f7;
    border-top-color: var(--wa-focus-ring, #008f96);
    border-radius: 50%;
    animation: rotate 0.8s linear infinite;
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
    border: 1px solid var(--wa-border, #e2e8f0);
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
    border-color: var(--wa-focus-ring, #008f96);
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
</style>
