<script lang="ts">
  import { onMount } from 'svelte';

  export let demand: any;
  export let currentUserPermissions: string[] = [];
  export let currentUserName = '';
  export let currentUserEmail = '';

  type Spec = {
    id: number;
    demand_id: string;
    version: number;
    status: string;
    source_archive_id: number;
    context_pack_id: number;
    original_text: string;
    intent: string;
    intent_confidence: number;
    summary: string;
    user_goal: string;
    facts: string[];
    inferences: string[];
    missing_context: string[];
    business_rules: string[];
    main_flows: string[];
    exception_flows: string[];
    permission_rules: string[];
    data_impact: string[];
    api_impact: string[];
    ui_impact: string[];
    dependencies: string[];
    risks: string[];
    acceptance_criteria: string[];
    test_plan: string[];
    mapped_repos: string[];
    tasks: any[];
    readiness_score: number;
    authored_by: string;
    reviewed_by: string;
    frozen_by: string;
    frozen_at?: string;
  };

  type ReviewContract = {
    id: number;
    demand_spec_version_id: number;
    status: string;
    required_roles: string[];
    reviewer_candidates: string[];
    resolved_reviewers: string[];
    acceptance_owner: string;
    minimum_approvals: number;
    protected_path_rules: any[];
    segregation_rules: string[];
    review_sla_hours: number;
    escalation_owner: string;
    resolution_status: string;
    resolution_reason: string;
  };

  type FileAction = { action: 'create' | 'update'; path: string; content: string; encoding?: string };
  type GeneratedChangeSet = { summary: string; change_set: FileAction[]; test_commands: string[]; model?: string };
  type ExecutionRun = {
    id: number;
    status: string;
    repo: string;
    topic_branch: string;
    base_branch: string;
    pipeline_status: string;
    acceptance_state: string;
    resolved_reviewers_json: string;
    mr_url: string;
    mr_state: string;
    block_reason: string;
    actions?: any[];
  };

  let loadedDemandId = '';
  let loading = false;
  let actionLoading = '';
  let error = '';
  let notice = '';
  let specs: Spec[] = [];
  let spec: Spec | null = null;
  let contract: ReviewContract | null = null;
  let runs: ExecutionRun[] = [];
  let generated: GeneratedChangeSet | null = null;
  let sourcePathsText = '';
  let generationInstructions = '';

  let summaryText = '';
  let goalText = '';
  let acceptanceText = '';
  let testPlanText = '';
  let reposText = '';
  let missingContextText = '';
  let riskText = '';
  let reviewerCandidatesText = '';
  let requiredRolesText = '';
  let acceptanceOwnerText = '';
  let minimumApprovals = 1;
  let escalationOwnerText = '';

  $: latestRun = runs[0] || null;
  $: canRead = hasPermission('demand_spec:read');
  $: canWrite = hasPermission('demand_spec:write');
  $: canFreeze = hasPermission('demand_spec:freeze');
  $: canManageReview = hasPermission('review_contract:manage');
  $: canExecute = hasPermission('execution:start');
  $: canAccept = hasPermission('execution:accept');
  $: if (demand?.task_id && demand.task_id !== loadedDemandId) {
    loadedDemandId = demand.task_id;
    void loadDeliveryWorkspace();
  }

  onMount(() => {
    if (demand?.task_id && demand.task_id !== loadedDemandId) {
      loadedDemandId = demand.task_id;
      void loadDeliveryWorkspace();
    }
  });

  function hasPermission(permission: string) {
    return currentUserPermissions.includes(permission);
  }

  function lines(value: string) {
    return Array.from(new Set(value.split(/\r?\n|,/).map(item => item.trim()).filter(Boolean)));
  }

  function fillSpecForm(value: Spec) {
    summaryText = value.summary || '';
    goalText = value.user_goal || '';
    acceptanceText = (value.acceptance_criteria || []).join('\n');
    testPlanText = (value.test_plan || []).join('\n');
    reposText = (value.mapped_repos || []).join('\n');
    missingContextText = (value.missing_context || []).join('\n');
    riskText = (value.risks || []).join('\n');
  }

  function fillContractForm(value: ReviewContract) {
    reviewerCandidatesText = (value.reviewer_candidates || []).join('\n');
    requiredRolesText = (value.required_roles || []).join('\n');
    acceptanceOwnerText = value.acceptance_owner || '';
    minimumApprovals = value.minimum_approvals || 1;
    escalationOwnerText = value.escalation_owner || '';
  }

  async function api(path: string, init: RequestInit = {}) {
    const response = await fetch(path, init);
    if (!response.ok) {
      const contentType = response.headers.get('content-type') || '';
      if (contentType.includes('application/json')) {
        const body = await response.json();
        throw new Error(body.blockers?.join('；') || body.error || body.message || `HTTP ${response.status}`);
      }
      throw new Error((await response.text()) || `HTTP ${response.status}`);
    }
    return response.json();
  }

  async function loadDeliveryWorkspace() {
    if (!canRead || !demand?.task_id) return;
    loading = true;
    error = '';
    try {
      const [specResponse, runResponse] = await Promise.all([
        api(`/api/demand-specs?demand_id=${encodeURIComponent(demand.task_id)}`),
        api(`/api/execution/runs?demand_id=${encodeURIComponent(demand.task_id)}`)
      ]);
      specs = specResponse.items || [];
      runs = runResponse.items || [];
      spec = specs[0] || null;
      if (spec) {
        fillSpecForm(spec);
        const contractResponse = await api(`/api/review-contracts?demand_spec_version_id=${spec.id}`);
        contract = contractResponse.review_contract || null;
        if (contract) fillContractForm(contract);
      } else {
        contract = null;
      }
    } catch (err: any) {
      error = err.message || '加载受控交付状态失败';
    } finally {
      loading = false;
    }
  }

  async function createDraft(useAI: boolean) {
    if (!canWrite) return;
    actionLoading = useAI ? 'ai-draft' : 'manual-draft';
    error = '';
    notice = '';
    try {
      const demandText = [demand.title, demand.description].filter(Boolean).join('\n');
      let payload: any = {
        demand_id: demand.task_id,
        original_text: demandText,
        summary: demand.title,
        user_goal: demand.description || demand.title,
        mapped_repos: demand.repo && demand.repo !== '-' ? [demand.repo] : [],
        readiness_score: 40,
        acceptance_criteria: [],
        test_plan: []
      };
      if (useAI) {
        const intent = await api('/api/ai/intent', {
          method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ text: demandText })
        });
        const deconstruction = await api('/api/deconstruct', {
          method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ text: demandText })
        });
        payload = {
          ...payload,
          context_pack_id: deconstruction.context_pack_id || 0,
          intent: intent.intent,
          intent_confidence: intent.confidence,
          summary: intent.summary || demand.title,
          facts: intent.facts || [],
          inferences: intent.inferences || [],
          missing_context: [...(intent.missing_context || []), ...(deconstruction.analysis?.missing_info || [])],
          risks: deconstruction.analysis?.risks || [],
          dependencies: deconstruction.analysis?.dependencies || [],
          acceptance_criteria: deconstruction.analysis?.acceptance_criteria || [],
          test_plan: deconstruction.analysis?.acceptance_criteria || [],
          mapped_repos: deconstruction.mappedRepos || payload.mapped_repos,
          tasks: deconstruction.tasks || [],
          readiness_score: deconstruction.analysis?.completeness_score || 40
        };
      }
      const result = await api('/api/demand-specs', {
        method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(payload)
      });
      spec = result.spec;
      contract = result.review_contract;
      fillSpecForm(spec!);
      fillContractForm(contract!);
      notice = useAI ? 'AI 草案已生成，请人工修订后冻结。' : '人工草案已建立。';
      await loadDeliveryWorkspace();
    } catch (err: any) {
      error = err.message || '创建规格草案失败';
    } finally {
      actionLoading = '';
    }
  }

  function specPayload() {
    if (!spec) return null;
    return {
      ...spec,
      summary: summaryText.trim(),
      user_goal: goalText.trim(),
      acceptance_criteria: lines(acceptanceText),
      test_plan: lines(testPlanText),
      mapped_repos: lines(reposText),
      missing_context: lines(missingContextText),
      risks: lines(riskText)
    };
  }

  async function saveSpec() {
    const payload = specPayload();
    if (!payload || spec?.status !== 'draft') return false;
    actionLoading = 'save-spec'; error = ''; notice = '';
    try {
      const result = await api('/api/demand-specs', {
        method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(payload)
      });
      spec = result.spec;
      fillSpecForm(spec!);
      notice = '规格草案已保存。';
      return true;
    } catch (err: any) {
      error = err.message || '保存规格失败';
      return false;
    } finally { actionLoading = ''; }
  }

  async function saveContract() {
    if (!contract || contract.status === 'approved') return false;
    actionLoading = 'save-contract'; error = ''; notice = '';
    try {
      const result = await api('/api/review-contracts', {
        method: 'POST', headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          id: contract.id,
          demand_spec_version_id: contract.demand_spec_version_id,
          required_roles: lines(requiredRolesText),
          reviewer_candidates: lines(reviewerCandidatesText),
          acceptance_owner: acceptanceOwnerText.trim(),
          minimum_approvals: Number(minimumApprovals) || 1,
          protected_path_rules: contract.protected_path_rules || [],
          segregation_rules: ['author_cannot_self_approve'],
          review_sla_hours: contract.review_sla_hours || 24,
          escalation_owner: escalationOwnerText.trim()
        })
      });
      contract = result.review_contract;
      fillContractForm(contract!);
      notice = '审核契约已保存。';
      return true;
    } catch (err: any) {
      error = err.message || '保存审核契约失败';
      return false;
    } finally { actionLoading = ''; }
  }

  async function approveContract() {
    if (!contract) return;
    actionLoading = 'approve-contract'; error = ''; notice = '';
    try {
      if (!(await saveContract())) return;
      const result = await api(`/api/review-contracts/${contract.id}/approve`, {
        method: 'POST', headers: { 'Content-Type': 'application/json' }, body: '{}'
      });
      contract = result.review_contract;
      fillContractForm(contract!);
      notice = '审核契约已批准，可以冻结规格。';
    } catch (err: any) {
      error = err.message || '批准审核契约失败';
    } finally { actionLoading = ''; }
  }

  async function freezeSpec() {
    if (!spec) return;
    actionLoading = 'freeze'; error = ''; notice = '';
    try {
      if (!(await saveSpec())) return;
      const result = await api(`/api/demand-specs/${spec.id}/freeze`, {
        method: 'POST', headers: { 'Content-Type': 'application/json' }, body: '{}'
      });
      spec = result.spec;
      contract = result.review_contract;
      fillSpecForm(spec!);
      notice = '规格已冻结，后续执行将绑定此版本。';
    } catch (err: any) {
      error = err.message || '冻结规格失败';
    } finally { actionLoading = ''; }
  }

  async function generateChangeSet() {
    if (!spec || spec.status !== 'frozen') return;
    actionLoading = 'generate'; error = ''; notice = ''; generated = null;
    try {
      generated = await api('/api/execution/change-set/generate', {
        method: 'POST', headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          demand_spec_version_id: spec.id,
          repo: lines(reposText)[0],
          source_paths: lines(sourcePathsText),
          instructions: generationInstructions.trim()
        })
      });
      notice = `AI 已生成 ${generated?.change_set.length || 0} 个受限文件动作。`;
    } catch (err: any) {
      error = err.message || '生成代码变更失败';
    } finally { actionLoading = ''; }
  }

  async function createAndStartRun() {
    if (!spec || !generated) return;
    actionLoading = 'execute'; error = ''; notice = '';
    try {
      const created = await api('/api/execution/runs', {
        method: 'POST', headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          preflight: {
            demand_spec_version_id: spec.id,
            repo: lines(reposText)[0],
            author: currentUserName || currentUserEmail,
            change_set: generated.change_set,
            test_commands: generated.test_commands
          }
        })
      });
      const run = created.run;
      const started = await api(`/api/execution/runs/${run.id}/start`, {
        method: 'POST', headers: { 'Content-Type': 'application/json' }, body: '{}'
      });
      notice = started.run?.mr_url ? 'Draft MR 已创建，等待 CI 与人工审核。' : '执行运行已启动。';
      await loadDeliveryWorkspace();
    } catch (err: any) {
      error = err.message || '启动受控执行失败';
      await loadDeliveryWorkspace();
    } finally { actionLoading = ''; }
  }

  async function refreshRun() {
    if (!latestRun) return;
    actionLoading = 'refresh'; error = ''; notice = '';
    try {
      await api(`/api/execution/runs/${latestRun.id}/refresh`, {
        method: 'POST', headers: { 'Content-Type': 'application/json' }, body: '{}'
      });
      notice = 'CI 状态已刷新。';
      await loadDeliveryWorkspace();
    } catch (err: any) {
      error = err.message || '刷新运行失败';
    } finally { actionLoading = ''; }
  }

  async function verifyRun(decision: 'accepted' | 'rejected') {
    if (!latestRun) return;
    actionLoading = `verify-${decision}`; error = ''; notice = '';
    try {
      await api(`/api/execution/runs/${latestRun.id}/verify`, {
        method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ decision })
      });
      notice = decision === 'accepted' ? '人工验收已通过，等待 MR 合并证据。' : '本次交付已被人工拒绝。';
      await loadDeliveryWorkspace();
    } catch (err: any) {
      error = err.message || '提交人工验收失败';
    } finally { actionLoading = ''; }
  }

  async function cancelRun() {
    if (!latestRun) return;
    actionLoading = 'cancel'; error = ''; notice = '';
    try {
      await api(`/api/execution/runs/${latestRun.id}/cancel`, {
        method: 'POST', headers: { 'Content-Type': 'application/json' }, body: '{}'
      });
      notice = '执行运行已取消，审计记录保留。';
      await loadDeliveryWorkspace();
    } catch (err: any) {
      error = err.message || '取消运行失败';
    } finally { actionLoading = ''; }
  }

  function displayStatus(value: string) {
    const labels: Record<string, string> = {
      draft: '草案', frozen: '已冻结', approved: '已批准', preflight_ready: '预检通过',
      executing: '执行中', review_pending: '待审核', acceptance_pending: '待合并', delivered: '已交付',
      tests_failed: 'CI 失败', execution_failed: '执行失败', rejected: '已拒绝', cancelled: '已取消'
    };
    return labels[value] || value || '未开始';
  }

  function runTone(value: string) {
    if (['delivered', 'success', 'frozen', 'approved'].includes(value)) return 'success';
    if (['tests_failed', 'execution_failed', 'rejected'].includes(value)) return 'danger';
    if (['executing', 'review_pending', 'acceptance_pending', 'pending'].includes(value)) return 'info';
    return 'neutral';
  }
</script>

{#if canRead}
  <section class="delivery-control" aria-label="受控自治交付">
    <header class="delivery-control-head">
      <div>
        <span class="delivery-kicker">CONTROLLED DELIVERY</span>
        <h4>需求规格与自治执行</h4>
        <p>AI 生成草案与代码，人负责冻结规格、审核 MR 和最终验收。</p>
      </div>
      <div class="delivery-state-strip">
        <span class="state-pill {spec ? runTone(spec.status) : 'neutral'}">规格 {spec ? `v${spec.version} · ${displayStatus(spec.status)}` : '未建立'}</span>
        <span class="state-pill {contract ? runTone(contract.status) : 'neutral'}">契约 {contract ? displayStatus(contract.status) : '未建立'}</span>
        <span class="state-pill {latestRun ? runTone(latestRun.status) : 'neutral'}">执行 {latestRun ? displayStatus(latestRun.status) : '未开始'}</span>
      </div>
    </header>

    {#if loading}
      <div class="delivery-empty loading">正在读取规格、审核契约与执行证据…</div>
    {:else}
      {#if error}<div class="delivery-message error">{error}</div>{/if}
      {#if notice}<div class="delivery-message success">{notice}</div>{/if}

      {#if !spec}
        <div class="delivery-empty">
          <strong>尚未建立可执行需求规格</strong>
          <p>可以先让 AI 识别意图并解构，也可以直接建立人工草案。</p>
          {#if canWrite}
            <div class="delivery-actions">
              <button class="primary" disabled={!!actionLoading} on:click={() => createDraft(true)}>{actionLoading === 'ai-draft' ? 'AI 分析中…' : 'AI 生成规格草案'}</button>
              <button disabled={!!actionLoading} on:click={() => createDraft(false)}>建立人工草案</button>
            </div>
          {/if}
        </div>
      {:else}
        <div class="delivery-stage">
          <div class="stage-head"><span>01</span><div><strong>人工审核规格</strong><small>冻结后不可原地修改</small></div></div>
          <div class="delivery-grid two">
            <label><span>需求摘要</span><textarea rows="2" bind:value={summaryText} disabled={spec.status !== 'draft'}></textarea></label>
            <label><span>用户目标</span><textarea rows="2" bind:value={goalText} disabled={spec.status !== 'draft'}></textarea></label>
            <label><span>验收标准 · 每行一项</span><textarea rows="4" bind:value={acceptanceText} disabled={spec.status !== 'draft'}></textarea></label>
            <label><span>测试计划 · 每行一项</span><textarea rows="4" bind:value={testPlanText} disabled={spec.status !== 'draft'}></textarea></label>
            <label><span>目标仓库 · 每行一项</span><textarea rows="3" bind:value={reposText} disabled={spec.status !== 'draft'}></textarea></label>
            <label><span>仍缺上下文</span><textarea rows="3" bind:value={missingContextText} disabled={spec.status !== 'draft'}></textarea></label>
            <label class="span-two"><span>已识别风险</span><textarea rows="3" bind:value={riskText} disabled={spec.status !== 'draft'}></textarea></label>
          </div>
          <div class="stage-meta">
            <span>意图：{spec.intent || '需求解构'}</span><span>就绪度：{spec.readiness_score}%</span><span>Context Pack：#{spec.context_pack_id || '-'}</span>
          </div>
          {#if spec.status === 'draft' && canWrite}
            <div class="delivery-actions"><button disabled={!!actionLoading} on:click={saveSpec}>{actionLoading === 'save-spec' ? '保存中…' : '保存规格'}</button></div>
          {/if}
        </div>

        {#if contract}
          <div class="delivery-stage">
            <div class="stage-head"><span>02</span><div><strong>审核契约</strong><small>先确定审核规则，MR 时再按真实 Diff 解析人员</small></div></div>
            <div class="delivery-grid three">
              <label><span>候选 Reviewer</span><textarea rows="4" bind:value={reviewerCandidatesText} disabled={contract.status === 'approved'}></textarea></label>
              <label><span>必须审核角色</span><textarea rows="4" bind:value={requiredRolesText} disabled={contract.status === 'approved'}></textarea></label>
              <div class="delivery-field-stack">
                <label><span>业务验收人</span><input bind:value={acceptanceOwnerText} disabled={contract.status === 'approved'} /></label>
                <label><span>最少批准数</span><input type="number" min="1" bind:value={minimumApprovals} disabled={contract.status === 'approved'} /></label>
                <label><span>升级负责人</span><input bind:value={escalationOwnerText} disabled={contract.status === 'approved'} /></label>
              </div>
            </div>
            {#if contract.resolved_reviewers?.length}
              <div class="resolved-line">最终 Reviewer：{contract.resolved_reviewers.join('、')} · {contract.resolution_reason}</div>
            {/if}
            {#if contract.status !== 'approved' && canManageReview}
              <div class="delivery-actions">
                <button disabled={!!actionLoading} on:click={saveContract}>保存契约</button>
                <button class="primary" disabled={!!actionLoading} on:click={approveContract}>{actionLoading === 'approve-contract' ? '批准中…' : '批准审核契约'}</button>
              </div>
            {:else if contract.status === 'approved' && spec.status === 'draft' && canFreeze}
              <div class="delivery-actions"><button class="primary" disabled={!!actionLoading} on:click={freezeSpec}>{actionLoading === 'freeze' ? '冻结中…' : '冻结规格并开放执行'}</button></div>
            {/if}
          </div>
        {/if}

        {#if spec.status === 'frozen'}
          <div class="delivery-stage">
            <div class="stage-head"><span>03</span><div><strong>AI 生成与受控执行</strong><small>只允许结构化文件动作，测试由 GitLab Pipeline 执行</small></div></div>
            {#if !latestRun || ['cancelled', 'rejected', 'execution_failed', 'tests_failed'].includes(latestRun.status)}
              <div class="delivery-grid two">
                <label><span>允许读取/更新的源文件路径</span><textarea rows="4" bind:value={sourcePathsText} placeholder="internal/server/handler.go"></textarea></label>
                <label><span>补充生成指令</span><textarea rows="4" bind:value={generationInstructions} placeholder="说明要修改的行为，不填写 shell 命令"></textarea></label>
              </div>
              <div class="delivery-actions">
                <button disabled={!!actionLoading} on:click={generateChangeSet}>{actionLoading === 'generate' ? 'AI 生成中…' : 'AI 生成代码变更'}</button>
                {#if generated && canExecute}<button class="primary" disabled={!!actionLoading} on:click={createAndStartRun}>{actionLoading === 'execute' ? '创建中…' : '创建分支、提交与 Draft MR'}</button>{/if}
              </div>
              {#if generated}
                <div class="change-set-preview">
                  <strong>{generated.summary}</strong>
                  {#each generated.change_set as change}<div><span>{change.action}</span><code>{change.path}</code></div>{/each}
                  <small>CI：{generated.test_commands.join(' · ')}</small>
                </div>
              {/if}
            {/if}

            {#if latestRun}
              <div class="run-ledger">
                <div><span>运行状态</span><strong>{displayStatus(latestRun.status)}</strong></div>
                <div><span>Pipeline</span><strong>{displayStatus(latestRun.pipeline_status)}</strong></div>
                <div><span>人工验收</span><strong>{displayStatus(latestRun.acceptance_state)}</strong></div>
                <div><span>分支</span><code>{latestRun.topic_branch || '-'}</code></div>
              </div>
              {#if latestRun.block_reason}<div class="run-blocker">{latestRun.block_reason}</div>{/if}
              <div class="delivery-actions">
                {#if latestRun.mr_url}<a class="delivery-link" href={latestRun.mr_url} target="_blank" rel="noopener noreferrer">打开 Draft MR</a>{/if}
                {#if !['delivered', 'rejected', 'cancelled'].includes(latestRun.status)}<button disabled={!!actionLoading} on:click={refreshRun}>刷新 CI</button>{/if}
                {#if canAccept && latestRun.pipeline_status === 'success' && latestRun.acceptance_state !== 'accepted'}
                  <button class="primary" disabled={!!actionLoading} on:click={() => verifyRun('accepted')}>人工验收通过</button>
                  <button class="danger" disabled={!!actionLoading} on:click={() => verifyRun('rejected')}>拒绝交付</button>
                {/if}
                {#if hasPermission('execution:cancel') && !['delivered', 'rejected', 'cancelled'].includes(latestRun.status)}<button disabled={!!actionLoading} on:click={cancelRun}>取消运行</button>{/if}
              </div>
              <div class="audit-line">已记录 {latestRun.actions?.length || 0} 个执行动作；MR 合并后由 GitLab Webhook 完成交付闭环。</div>
            {/if}
          </div>
        {/if}
      {/if}
    {/if}
  </section>
{/if}

<style>
  .delivery-control { border-top: 1px solid var(--wa-border, #d9e0e7); padding-top: 20px; display: grid; gap: 14px; color: #17212b; }
  .delivery-control-head { display: flex; align-items: flex-start; justify-content: space-between; gap: 18px; }
  .delivery-kicker { display: block; margin-bottom: 5px; color: #21847a; font: 700 10px/1.2 ui-monospace, SFMono-Regular, Menlo, monospace; letter-spacing: .12em; }
  h4 { margin: 0; font-size: 17px; letter-spacing: -.02em; }
  .delivery-control-head p { margin: 5px 0 0; color: #687784; font-size: 12px; }
  .delivery-state-strip { display: flex; flex-wrap: wrap; justify-content: flex-end; gap: 6px; }
  .state-pill { border: 1px solid #dce3e8; border-radius: 999px; padding: 5px 9px; background: #f7f9fa; color: #586773; font-size: 11px; }
  .state-pill.success { border-color: #a9d9cf; background: #edf8f5; color: #167267; }
  .state-pill.info { border-color: #b8d7eb; background: #eef7fc; color: #236c94; }
  .state-pill.danger { border-color: #edc1c1; background: #fff3f2; color: #a94747; }
  .delivery-stage { border: 1px solid #dce3e8; background: rgba(255,255,255,.82); padding: 16px; display: grid; gap: 13px; }
  .stage-head { display: flex; gap: 10px; align-items: center; }
  .stage-head > span { display: grid; place-items: center; width: 28px; height: 28px; background: #e9f5f2; color: #19766c; font: 700 11px/1 ui-monospace, monospace; }
  .stage-head div { display: grid; gap: 2px; }
  .stage-head strong { font-size: 13px; }
  .stage-head small { color: #75828d; font-size: 11px; }
  .delivery-grid { display: grid; gap: 12px; }
  .delivery-grid.two { grid-template-columns: repeat(2, minmax(0, 1fr)); }
  .delivery-grid.three { grid-template-columns: repeat(3, minmax(0, 1fr)); }
  .span-two { grid-column: 1 / -1; }
  label { display: grid; gap: 6px; min-width: 0; }
  label > span { color: #667580; font-size: 11px; font-weight: 650; }
  textarea, input { width: 100%; box-sizing: border-box; border: 1px solid #ccd6dd; border-radius: 6px; background: #fff; color: #1f2a33; padding: 9px 10px; font: 12px/1.45 inherit; outline: none; resize: vertical; transition: border-color 150ms ease, box-shadow 150ms ease; }
  textarea:focus, input:focus { border-color: #55a99f; box-shadow: 0 0 0 3px rgba(38, 139, 127, .11); }
  textarea:disabled, input:disabled { background: #f4f6f7; color: #53616c; resize: none; }
  .delivery-field-stack { display: grid; gap: 9px; }
  .stage-meta, .resolved-line, .audit-line { display: flex; flex-wrap: wrap; gap: 12px; color: #687681; font-size: 11px; }
  .resolved-line { padding: 8px 10px; background: #eff8f6; color: #276f68; }
  .delivery-actions { display: flex; align-items: center; flex-wrap: wrap; gap: 8px; }
  button, .delivery-link { min-height: 34px; border: 1px solid #cbd5dc; border-radius: 6px; background: #fff; color: #34434e; padding: 0 12px; font: 650 11px/32px inherit; cursor: pointer; text-decoration: none; transition: background 150ms ease, border-color 150ms ease, transform 120ms ease; }
  button:hover, .delivery-link:hover { border-color: #8fa8b4; background: #f7fafb; }
  button:active, .delivery-link:active { transform: scale(.98); }
  button:focus-visible, .delivery-link:focus-visible { outline: 3px solid rgba(38,139,127,.18); outline-offset: 1px; }
  button:disabled { opacity: .56; cursor: wait; }
  button.primary { border-color: #197c71; background: #197c71; color: #fff; }
  button.primary:hover { background: #126d63; }
  button.danger { border-color: #e4b5b5; color: #a44242; background: #fff8f7; }
  .delivery-empty { border: 1px dashed #cbd6dc; padding: 20px; text-align: center; color: #6d7b86; background: #f9fbfb; }
  .delivery-empty strong { display: block; color: #2b3943; margin-bottom: 4px; }
  .delivery-empty p { margin: 0 0 12px; font-size: 12px; }
  .delivery-empty .delivery-actions { justify-content: center; }
  .delivery-empty.loading { animation: pulse 1.4s ease-in-out infinite; }
  .delivery-message { padding: 9px 11px; border-left: 3px solid; font-size: 12px; }
  .delivery-message.error { border-color: #c95a5a; background: #fff4f3; color: #943e3e; }
  .delivery-message.success { border-color: #279184; background: #eff9f6; color: #276e66; }
  .change-set-preview { display: grid; gap: 6px; padding: 11px; background: #f5f8f9; border: 1px solid #dde4e8; }
  .change-set-preview > div { display: grid; grid-template-columns: 58px minmax(0, 1fr); align-items: center; gap: 8px; }
  .change-set-preview span { color: #1b7f74; font: 700 10px/1 ui-monospace, monospace; text-transform: uppercase; }
  code { overflow-wrap: anywhere; color: #344852; font: 11px/1.4 ui-monospace, SFMono-Regular, Menlo, monospace; }
  .change-set-preview small { color: #71808a; }
  .run-ledger { display: grid; grid-template-columns: repeat(4, minmax(0, 1fr)); border: 1px solid #dde4e8; }
  .run-ledger > div { min-width: 0; padding: 10px; border-right: 1px solid #e3e8eb; display: grid; gap: 4px; }
  .run-ledger > div:last-child { border-right: 0; }
  .run-ledger span { color: #7a8791; font-size: 10px; }
  .run-ledger strong { font-size: 12px; }
  .run-blocker { padding: 9px 11px; background: #fff4f3; color: #944444; font-size: 11px; }
  @keyframes pulse { 50% { opacity: .6; } }
  @media (max-width: 900px) {
    .delivery-control-head { display: grid; }
    .delivery-state-strip { justify-content: flex-start; }
    .delivery-grid.two, .delivery-grid.three, .run-ledger { grid-template-columns: 1fr; }
    .run-ledger > div { border-right: 0; border-bottom: 1px solid #e3e8eb; }
  }
</style>
