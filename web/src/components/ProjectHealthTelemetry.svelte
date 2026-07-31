<script lang="ts">
  import { onMount, onDestroy } from 'svelte';
  import type { AdminInspectorRecord, AdminTableColumn, AdminTableRow, AdminTone } from '../lib/admin-console/contract';
  import { ADMIN_TONE_CLASS, formatAdminDate } from '../lib/admin-console/contract';

  interface ProjectScore {
    id?: number;
    project_key: string;
    project_name: string;
    schedule_health_score: number;
    engineering_quality: number;
    collaboration_effic: number;
    stability_index: number;
    compound_score: number;
    diagnostic: string;
    snapshot_date: string;
  }

  type HealthLevel = 'red' | 'yellow' | 'green';
  type HealthFilter = 'all' | HealthLevel;
  type DimensionTone = 'schedule' | 'engineering' | 'collaboration' | 'stability';

  interface DimensionInsight {
    key: DimensionTone;
    code: string;
    label: string;
    score: number;
    status: string;
    tone: 'safe' | 'warn' | 'danger';
    action: string;
    evidence: string;
  }

  interface ProjectDiagnosis {
    level: HealthLevel;
    label: string;
    urgency: string;
    summary: string;
    weakest: DimensionInsight;
    dimensions: DimensionInsight[];
    manualDirection: string;
    evidenceGap: string;
    decisionQuestion: string;
  }

  const projectHealthColumns: AdminTableColumn[] = [
    { key: 'project', label: '项目', width: '22%' },
    { key: 'score', label: '健康度', width: '10%', align: 'center' },
    { key: 'status', label: '状态', width: '11%', align: 'center' },
    { key: 'dimensions', label: '四维证据', width: '24%' },
    { key: 'weakest', label: '最弱维度', width: '16%' },
    { key: 'action', label: '介入动作', width: '17%' }
  ];

  let scores: ProjectScore[] = [];
  let projectConfigs: any[] = [];
  $: projectConfigMap = new Map<string, any>(projectConfigs.map(c => [c.project_key.toUpperCase(), c]));
  let jiraProjectMap: {[key: string]: string} = {};
  let loading = false;
  let errorMsg = '';
  let searchQuery = '';
  let healthFilter: HealthFilter = 'all';
  let isPanelCollapsed = false;
  let activeProjectKey = '';
  let activeProjectScore: ProjectScore | null = null;
  let activeProjectRecord: AdminInspectorRecord | null = null;

  let showDetailsModal = false;
  let selectedProjectScore: ProjectScore | null = null;
  $: selectedConfig = selectedProjectScore ? projectConfigMap.get(selectedProjectScore.project_key.toUpperCase()) : null;

  $: if (typeof document !== 'undefined') {
    if (showDetailsModal) {
      const scrollbarWidth = window.innerWidth - document.documentElement.clientWidth;
      document.body.style.overflow = 'hidden';
      if (scrollbarWidth > 0) {
        document.body.style.paddingRight = `${scrollbarWidth}px`;
      }
    } else {
      document.body.style.overflow = '';
      document.body.style.paddingRight = '';
    }
  }

  onDestroy(() => {
    if (typeof document !== 'undefined') {
      document.body.style.overflow = '';
      document.body.style.paddingRight = '';
    }
  });

  function showProjectDetails(score: ProjectScore) {
    selectedProjectScore = score;
    showDetailsModal = true;
  }

  function closeDetailsModal() {
    showDetailsModal = false;
    selectedProjectScore = null;
  }

  function handleModalBackdropClick(event: MouseEvent) {
    if (event.target === event.currentTarget) {
      closeDetailsModal();
    }
  }

  function handleModalBackdropKeydown(event: KeyboardEvent) {
    if (event.key === 'Escape') {
      closeDetailsModal();
    }
  }

  async function fetchScoresAndConfigs() {
    loading = true;
    errorMsg = '';
    const token = localStorage.getItem('jwt_token');
    
    try {
      // 1. Fetch scores
      const resScores = await fetch('/api/projects/scores', {
        headers: { 'Authorization': `Bearer ${token}` }
      });
      if (!resScores.ok) throw new Error('获取项目健康度打分失败');
      const scorePayload = await resScores.json();
      scores = Array.isArray(scorePayload) ? scorePayload : [];

      // 2. Fetch configs for robust names matching
      const resConfigs = await fetch('/api/projects/config', {
        headers: { 'Authorization': `Bearer ${token}` }
      });
      if (resConfigs.ok) {
        const configPayload = await resConfigs.json();
        projectConfigs = Array.isArray(configPayload) ? configPayload : [];
      }

      // 3. Fetch agenda summary for project name matching
      const resSummary = await fetch('/api/agenda/summary', {
        headers: { 'Authorization': `Bearer ${token}` }
      });
      if (resSummary.ok) {
        const summaryData = (await resSummary.json()) || {};
        const items = summaryData.agenda_items || [];
        const newMap: {[key: string]: string} = summaryData.project_map || {};
        
        if (Object.keys(newMap).length === 0) {
          items.forEach((item: any) => {
            if (item.repo) {
              const repoStr = item.repo.trim();
              const lastOpenParen = repoStr.lastIndexOf('(');
              const lastCloseParen = repoStr.lastIndexOf(')');
              if (lastOpenParen > 0 && lastCloseParen > lastOpenParen) {
                const key = repoStr.substring(lastOpenParen + 1, lastCloseParen).trim().toUpperCase();
                const name = repoStr.substring(0, lastOpenParen).trim();
                if (key && name) {
                  newMap[key] = name;
                }
              }
            }
          });
        }
        jiraProjectMap = newMap;
      }
    } catch (err: any) {
      errorMsg = err.message || '加载项目健康度遥测失败';
    } finally {
      loading = false;
    }
  }

  onMount(async () => {
    await fetchScoresAndConfigs();
  });

  function cleanProjectName(name: string): string {
    if (!name) return '';
    const lastOpenParen = name.lastIndexOf('(');
    const lastCloseParen = name.lastIndexOf(')');
    if (lastOpenParen > 0 && lastCloseParen > lastOpenParen) {
      const potentialKey = name.substring(lastOpenParen + 1, lastCloseParen).trim();
      if (potentialKey && /^[A-Z0-9]{2,10}$/i.test(potentialKey)) {
        return name.substring(0, lastOpenParen).trim();
      }
    }
    return name;
  }

  function getDisplayName(score: ProjectScore): string {
    const keyUpper = score.project_key.toUpperCase();
    if (jiraProjectMap[keyUpper]) {
      return cleanProjectName(jiraProjectMap[keyUpper]);
    }
    const conf = projectConfigMap.get(keyUpper);
    if (conf && conf.project_name && conf.project_name !== `${score.project_key}项目`) {
      return cleanProjectName(conf.project_name);
    }
    return cleanProjectName(score.project_name) || score.project_key;
  }

  $: searchMatchedScores = scores.filter(s => {
    const displayName = getDisplayName(s);
    if (!searchQuery) return true;
    const q = searchQuery.toLowerCase();
    return s.project_key.toLowerCase().includes(q) || 
           displayName.toLowerCase().includes(q);
  });
  $: filteredScores = searchMatchedScores.filter(score =>
    healthFilter === 'all' || getProjectHealthLevel(score) === healthFilter
  );
  $: rankedFilteredScores = [...filteredScores].sort((a, b) => {
    const levelDelta = getHealthRank(getProjectHealthLevel(b)) - getHealthRank(getProjectHealthLevel(a));
    if (levelDelta !== 0) return levelDelta;
    return a.compound_score - b.compound_score;
  });
  $: redProjectCount = searchMatchedScores.filter(s => getProjectHealthLevel(s) === 'red').length;
  $: yellowProjectCount = searchMatchedScores.filter(s => getProjectHealthLevel(s) === 'yellow').length;
  $: greenProjectCount = searchMatchedScores.filter(s => getProjectHealthLevel(s) === 'green').length;
  $: averageScore = searchMatchedScores.length
    ? Math.round(searchMatchedScores.reduce((sum, s) => sum + s.compound_score, 0) / searchMatchedScores.length)
    : 0;
  $: priorityScores = rankedFilteredScores.filter(s => getProjectHealthLevel(s) !== 'green').slice(0, 3);
  $: topPriorityScore = priorityScores[0] || rankedFilteredScores[0] || null;
  $: if (!activeProjectKey && topPriorityScore) {
    activeProjectKey = topPriorityScore.project_key;
  }
  $: activeProjectScore = rankedFilteredScores.find(s => s.project_key === activeProjectKey) || topPriorityScore || rankedFilteredScores[0] || null;
  $: activeProjectRecord = buildProjectInspectorRecord(activeProjectScore);
  $: projectHealthRows = rankedFilteredScores.map(buildProjectHealthRow) satisfies AdminTableRow[];

  function getScoreColorClass(score: number): string {
    if (score >= 85) return 'score-green';
    if (score >= 70) return 'score-yellow';
    return 'score-red';
  }

  function getHealthLevel(score: number): HealthLevel {
    if (score >= 85) return 'green';
    if (score >= 70) return 'yellow';
    return 'red';
  }

  function getProjectHealthLevel(score: ProjectScore): HealthLevel {
    const weakestDimension = Math.min(
      score.schedule_health_score,
      score.engineering_quality,
      score.collaboration_effic,
      score.stability_index
    );
    if (score.compound_score < 70 || weakestDimension < 70) return 'red';
    if (score.compound_score < 85 || weakestDimension < 85) return 'yellow';
    return 'green';
  }

  function getHealthRank(level: HealthLevel): number {
    if (level === 'red') return 3;
    if (level === 'yellow') return 2;
    return 1;
  }

  function getHealthLabel(score: ProjectScore): string {
    const level = getProjectHealthLevel(score);
    if (level === 'red') return '红区介入';
    if (level === 'yellow') return '黄区观察';
    return '稳定运行';
  }

  function healthTone(score: ProjectScore): AdminTone {
    const level = getProjectHealthLevel(score);
    if (level === 'red') return 'danger';
    if (level === 'yellow') return 'warning';
    return 'success';
  }

  function dimensionTone(tone: 'safe' | 'warn' | 'danger'): AdminTone {
    if (tone === 'safe') return 'success';
    if (tone === 'warn') return 'warning';
    return 'danger';
  }

  function getDimensionInsights(score: ProjectScore): DimensionInsight[] {
    return [
      {
        key: 'schedule',
        code: 'SH',
        label: '排期可信度',
        score: score.schedule_health_score,
        status: score.schedule_health_score >= 85 ? '排期可信' : (score.schedule_health_score >= 70 ? '排期波动' : '排期失真'),
        tone: score.schedule_health_score >= 85 ? 'safe' : (score.schedule_health_score >= 70 ? 'warn' : 'danger'),
        action: score.schedule_health_score >= 85 ? '保持节奏' : '人工确认截止日、范围和延期原因',
        evidence: score.schedule_health_score >= 85 ? '排期证据完整' : '需补截止日、二次延期或停滞说明'
      },
      {
        key: 'engineering',
        code: 'EQ',
        label: '工程证据',
        score: score.engineering_quality,
        status: score.engineering_quality >= 85 ? '证据闭环' : (score.engineering_quality >= 70 ? '证据偏弱' : '证据断链'),
        tone: score.engineering_quality >= 85 ? 'safe' : (score.engineering_quality >= 70 ? 'warn' : 'danger'),
        action: score.engineering_quality >= 85 ? '保持关联' : '人工补齐需求到分支、MR、commit 的证据链',
        evidence: score.engineering_quality >= 85 ? '代码链路可追溯' : '需补 Jira issue、分支、MR 或 commit 关联'
      },
      {
        key: 'collaboration',
        code: 'CE',
        label: '协同负载',
        score: score.collaboration_effic,
        status: score.collaboration_effic >= 85 ? '负载均衡' : (score.collaboration_effic >= 70 ? '局部偏载' : '协同阻塞'),
        tone: score.collaboration_effic >= 85 ? 'safe' : (score.collaboration_effic >= 70 ? 'warn' : 'danger'),
        action: score.collaboration_effic >= 85 ? '保持分工' : '人工评估转派、结对协作或负责人保护窗口',
        evidence: score.collaboration_effic >= 85 ? '负责人分布正常' : '需确认负责人负载、转派记录和阻塞归因'
      },
      {
        key: 'stability',
        code: 'SI',
        label: '质量稳定性',
        score: score.stability_index,
        status: score.stability_index >= 85 ? '质量稳定' : (score.stability_index >= 70 ? '缺陷升温' : '质量高危'),
        tone: score.stability_index >= 85 ? 'safe' : (score.stability_index >= 70 ? 'warn' : 'danger'),
        action: score.stability_index >= 85 ? '保持观测' : '人工组织缺陷分诊、回归窗口和发布风险确认',
        evidence: score.stability_index >= 85 ? '缺陷压力可控' : '需补缺陷根因、回归负责人和验收结论'
      }
    ];
  }

  function getProjectDiagnosis(score: ProjectScore): ProjectDiagnosis {
    const dimensions = getDimensionInsights(score);
    const weakest = [...dimensions].sort((a, b) => a.score - b.score)[0];
    const level = getProjectHealthLevel(score);
    const label = getHealthLabel(score);
    const urgency = level === 'red' ? '今天必须介入' : (level === 'yellow' ? '本周补证据' : '无需打扰');
    const summary = level === 'green'
      ? '当前项目指标稳定，系统继续观测即可。'
      : `${weakest.label}是当前最低维度，需要把状态判断转成人工可执行动作。`;
    return {
      level,
      label,
      urgency,
      summary,
      weakest,
      dimensions,
      manualDirection: weakest.action,
      evidenceGap: weakest.evidence,
      decisionQuestion: level === 'green'
        ? '是否继续保持当前节奏，不触发人工打扰？'
        : `是否现在介入处理${weakest.label}，并指定责任人补齐证据？`
    };
  }

  function getDimensionToneClass(tone: 'safe' | 'warn' | 'danger'): string {
    if (tone === 'safe') return 'tone-safe';
    if (tone === 'warn') return 'tone-warn';
    return 'tone-danger';
  }

  function buildProjectHealthRow(score: ProjectScore): AdminTableRow {
    const diagnosis = getProjectDiagnosis(score);
    const config = projectConfigMap.get(score.project_key.toUpperCase());
    return {
      id: score.project_key,
      title: getDisplayName(score),
      status: diagnosis.label,
      tone: healthTone(score),
      owner: diagnosis.weakest.label,
      dueDate: formatAdminDate(score.snapshot_date),
      priority: config?.base_priority || '',
      risk: diagnosis.weakest.status,
      cells: {
        project: getDisplayName(score),
        key: score.project_key,
        score: score.compound_score,
        status: diagnosis.label,
        weakest: `${diagnosis.weakest.label} ${diagnosis.weakest.score}%`,
        action: diagnosis.manualDirection,
        snapshot: formatAdminDate(score.snapshot_date)
      }
    };
  }

  function buildProjectInspectorRecord(score: ProjectScore | null): AdminInspectorRecord | null {
    if (!score) return null;
    const diagnosis = getProjectDiagnosis(score);
    const config = projectConfigMap.get(score.project_key.toUpperCase());
    return {
      id: score.project_key,
      title: getDisplayName(score),
      status: diagnosis.label,
      tone: healthTone(score),
      facts: [
        { label: '项目编号', value: score.project_key },
        { label: '综合健康度', value: `${score.compound_score}%` },
        { label: '最低维度', value: `${diagnosis.weakest.label} ${diagnosis.weakest.score}%` },
        { label: '快照日期', value: formatAdminDate(score.snapshot_date) },
        { label: '项目阶段', value: config?.project_phase || '交付' },
        { label: '基准优先级', value: config?.base_priority || 'P1' }
      ],
      sections: [
        {
          title: '介入判断',
          body: diagnosis.decisionQuestion,
          items: [diagnosis.manualDirection, diagnosis.evidenceGap]
        },
        {
          title: '证据维度',
          items: diagnosis.dimensions.map((dim) => `${dim.label}: ${Math.round(dim.score)}% · ${dim.status} · ${dim.evidence}`)
        },
        {
          title: '诊断摘要',
          body: score.diagnostic || diagnosis.summary
        }
      ],
      actions: [
        { label: '查看计算细节', kind: 'primary' }
      ]
    };
  }

  function selectProject(score: ProjectScore) {
    activeProjectKey = score.project_key;
  }

  function portal(node: HTMLElement) {
    document.body.appendChild(node);
    return {
      destroy() {
        if (node.parentNode) {
          node.parentNode.removeChild(node);
        }
      }
    };
  }
</script>

<div class="project-health-workbench">
  {#if !isPanelCollapsed}
    {#if loading && scores.length === 0}
      <div class="project-health-state wa-admin-card font-mono">正在加载项目证据健康数据</div>
    {:else if errorMsg}
      <div class="project-health-state is-error wa-admin-card font-mono">加载失败 · {errorMsg}</div>
    {:else if searchMatchedScores.length === 0}
      <div class="project-health-state wa-admin-card font-mono">没有匹配的项目证据数据</div>
    {:else}
      <div class="health-decision-strip wa-admin-card" aria-label="证据健康异常筛选">
        <div class="health-decision-copy">
          <span class="eyebrow">Evidence triage</span>
          <strong>{redProjectCount > 0 ? `${redProjectCount} 个红区项目需要今天介入` : yellowProjectCount > 0 ? `${yellowProjectCount} 个黄区项目需要本周补证据` : '当前项目证据健康稳定'}</strong>
          <small>{searchMatchedScores.length} 个可见项目 · 平均健康度 {averageScore}% · 按最低证据维度排序</small>
        </div>
        <div class="health-filter-group" role="group" aria-label="按健康状态筛选">
          <button type="button" class:active={healthFilter === 'all'} on:click={() => healthFilter = 'all'}>
            <span>全部</span><strong>{searchMatchedScores.length}</strong>
          </button>
          <button type="button" class:active={healthFilter === 'red'} class:tone-danger={redProjectCount > 0} on:click={() => healthFilter = 'red'}>
            <span>红区介入</span><strong>{redProjectCount}</strong>
          </button>
          <button type="button" class:active={healthFilter === 'yellow'} class:tone-warning={yellowProjectCount > 0} on:click={() => healthFilter = 'yellow'}>
            <span>黄区补证</span><strong>{yellowProjectCount}</strong>
          </button>
          <button type="button" class:active={healthFilter === 'green'} class:tone-success={greenProjectCount > 0} on:click={() => healthFilter = 'green'}>
            <span>稳定运行</span><strong>{greenProjectCount}</strong>
          </button>
        </div>
      </div>

      <div class="project-health-main-grid">
        <section class="project-health-table-stack wa-admin-section" aria-label="项目健康证据表">
          <div class="project-health-table-card wa-admin-card">
            <div class="project-health-table-head">
              <div>
                <span class="eyebrow">项目列表</span>
                <h3>证据健康总表</h3>
              </div>
              <div class="project-health-actions">
                <div class="project-health-search">
                  <span class="search-icon" aria-hidden="true"></span>
                  <input type="text" placeholder="搜索项目、编号" bind:value={searchQuery} class="telemetry-search-input" />
                </div>
                <span class="project-health-count font-mono">{projectHealthRows.length} / {scores.length}</span>
                <button class="wa-admin-action secondary font-mono" on:click={fetchScoresAndConfigs} disabled={loading}>{loading ? '同步中' : '刷新'}</button>
              </div>
            </div>

            <div class="wa-admin-table-shell">
              <table class="wa-admin-table project-health-table">
                <colgroup>
                  {#each projectHealthColumns as column}
                    <col style="width: {column.width || 'auto'}" />
                  {/each}
                  <col style="width: 128px" />
                </colgroup>
                <thead>
                  <tr>
                    {#each projectHealthColumns as column}
                      <th class:align-center={column.align === 'center'}>{column.label}</th>
                    {/each}
                    <th class="align-center">操作</th>
                  </tr>
                </thead>
                <tbody>
                  {#if rankedFilteredScores.length === 0}
                    <tr>
                      <td colspan={projectHealthColumns.length + 1} class="project-health-empty-row">当前状态筛选下暂无项目</td>
                    </tr>
                  {:else}
                  {#each rankedFilteredScores as score}
                    {@const row = buildProjectHealthRow(score)}
                    {@const diagnosis = getProjectDiagnosis(score)}
                    <tr
                      class:is-selected={activeProjectScore?.project_key === score.project_key}
                      on:click={() => selectProject(score)}
                    >
                      <td>
                        <div class="project-info-cell">
                          <span class="project-avatar font-mono {getScoreColorClass(score.compound_score)}">{score.project_key.slice(0, 2).toUpperCase()}</span>
                          <div class="project-meta">
                            <strong>{row.title}</strong>
                            <span class="font-mono text-muted">{row.id}</span>
                          </div>
                        </div>
                      </td>
                      <td class="align-center">
                        <span class="score-badge font-mono {ADMIN_TONE_CLASS[row.tone || 'neutral']}">{row.cells.score}%</span>
                      </td>
                      <td class="align-center">
                        <span class="wa-admin-pill {ADMIN_TONE_CLASS[row.tone || 'neutral']}">{row.status}</span>
                      </td>
                      <td>
                        <div class="metric-micro-grid">
                          {#each diagnosis.dimensions as dim}
                            <div class="metric-micro-item {ADMIN_TONE_CLASS[dimensionTone(dim.tone)]}">
                              <span>{dim.label}</span>
                              <strong class="font-mono">{Math.round(dim.score)}%</strong>
                              <i style="width: {dim.score}%"></i>
                            </div>
                          {/each}
                        </div>
                      </td>
                      <td>
                        <div class="weak-dimension-cell {ADMIN_TONE_CLASS[dimensionTone(diagnosis.weakest.tone)]}">
                          <span>{diagnosis.weakest.label}</span>
                          <strong class="font-mono">{diagnosis.weakest.score}% · {diagnosis.weakest.status}</strong>
                        </div>
                      </td>
                      <td>
                        <div class="intervention-cell">
                          <strong>{diagnosis.manualDirection}</strong>
                          <span>{diagnosis.evidenceGap}</span>
                        </div>
                      </td>
                      <td class="align-center">
                        <button class="wa-admin-action secondary" type="button" on:click|stopPropagation={() => showProjectDetails(score)}>
                          详情
                        </button>
                      </td>
                    </tr>
                  {/each}
                  {/if}
                </tbody>
              </table>
            </div>
          </div>
        </section>

        <aside class="project-health-inspector wa-admin-card wa-admin-inspector" aria-label="项目详情与检查项">
          {#if activeProjectRecord && activeProjectScore}
            <div class="inspector-title-row">
              <span class="wa-admin-pill {ADMIN_TONE_CLASS[activeProjectRecord.tone || 'neutral']}">{activeProjectRecord.status}</span>
              <span class="font-mono">{activeProjectRecord.id}</span>
            </div>
            <h3>{activeProjectRecord.title}</h3>
            <p>{getProjectDiagnosis(activeProjectScore).summary}</p>

            <div class="project-health-facts">
              {#each activeProjectRecord.facts as fact}
                <div>
                  <span>{fact.label}</span>
                  <strong>{fact.value}</strong>
                </div>
              {/each}
            </div>

            <div class="project-health-progress">
              <div>
                <span>证据健康度</span>
                <strong class="font-mono">{activeProjectScore.compound_score}%</strong>
              </div>
              <div class="wa-admin-progress" style="--progress: {activeProjectScore.compound_score}%" aria-hidden="true"></div>
            </div>

            {#each activeProjectRecord.sections as section}
              <div class="project-health-inspector-section">
                <h4>{section.title}</h4>
                {#if section.body}
                  <p>{section.body}</p>
                {/if}
                {#if section.items && section.items.length > 0}
                  <ul>
                    {#each section.items as item}
                      <li>{item}</li>
                    {/each}
                  </ul>
                {/if}
              </div>
            {/each}

            <div class="project-health-inspector-actions">
              <button class="wa-admin-action primary" type="button" on:click={() => showProjectDetails(activeProjectScore)}>
                查看计算细节
              </button>
            </div>
          {:else}
            <div class="project-health-inspector-empty">
              <strong>暂无项目证据</strong>
              <span>等待项目分数生成后显示检查项。</span>
            </div>
          {/if}
        </aside>
      </div>
    {/if}
  {/if}

  {#if showDetailsModal && selectedProjectScore}
    {@const selectedDiagnosis = getProjectDiagnosis(selectedProjectScore)}
    <div
      class="modal-backdrop"
      use:portal
      role="presentation"
      on:click={handleModalBackdropClick}
      on:keydown={handleModalBackdropKeydown}
    >
      <div class="modal-content glass-panel health-diagnosis-modal" role="dialog" aria-modal="true" aria-label="{getDisplayName(selectedProjectScore)} 健康诊断与介入方案">
        <div class="modal-header">
          <div class="modal-title-block">
            <span>项目健康诊断</span>
            <h3>{getDisplayName(selectedProjectScore)} 健康诊断与介入方案</h3>
          </div>
          <button class="close-btn" on:click={closeDetailsModal} aria-label="关闭健康诊断弹窗">&times;</button>
        </div>
        
        <div class="modal-body">
          <section class="intervention-brief-card {getScoreColorClass(selectedProjectScore.compound_score)}">
            <div class="intervention-brief-head">
              <div>
                <span>{selectedDiagnosis.urgency}</span>
                <h4>{selectedDiagnosis.decisionQuestion}</h4>
              </div>
              <div class="intervention-brief-score">
                <span>综合健康度</span>
                <strong class="font-mono">{selectedProjectScore.compound_score}</strong>
              </div>
            </div>
            <div class="intervention-brief-grid">
              <div>
                <span>最低维度</span>
                <strong>{selectedDiagnosis.weakest.label} · {selectedDiagnosis.weakest.score}%</strong>
              </div>
              <div>
                <span>证据缺口</span>
                <strong>{selectedDiagnosis.evidenceGap}</strong>
              </div>
              <div>
                <span>人工动作</span>
                <strong>{selectedDiagnosis.manualDirection}</strong>
              </div>
            </div>
          </section>

          <div class="dashboard-columns">
            <!-- 左栏：诊断决策链与大分分析 -->
            <div class="column-left">
              {#if selectedProjectScore.diagnostic}
                <div class="detail-section">
                  <h4>诊断意见</h4>
                  <div class="diagnostic-panel-modal {getScoreColorClass(selectedProjectScore.compound_score)} font-mono">
                    <div class="diag-header-row">
                      <span class="pulse-diagnostic-dot {getScoreColorClass(selectedProjectScore.compound_score)}"></span>
                      <span>DIAGNOSIS EVIDENCE</span>
                    </div>
                    <p class="diagnostic-copy">{selectedProjectScore.diagnostic}</p>
                  </div>
                </div>
              {/if}

              <div class="detail-section">
                <h4>PHDI 综合健康度分析</h4>
                <div class="phdi-display">
                  <div class="phdi-score-row">
                    <span class="big-score {getScoreColorClass(selectedProjectScore.compound_score)}">
                      {selectedProjectScore.compound_score} <span class="unit">分</span>
                    </span>
                    <span class="health-badge {getScoreColorClass(selectedProjectScore.compound_score)} font-mono">
                      {selectedDiagnosis.label}
                    </span>
                  </div>
                  
                  <div class="formula-breakdown">
                    <h5>计算公式权重剖析</h5>
                    {#if selectedConfig?.base_score_weight !== undefined}
                      {@const w = selectedConfig.base_score_weight}
                      {@const s = selectedConfig.base_score}
                      {@const metricsScore = selectedProjectScore.schedule_health_score * 0.30 + selectedProjectScore.engineering_quality * 0.25 + selectedProjectScore.collaboration_effic * 0.25 + selectedProjectScore.stability_index * 0.20}
                      
                      <div class="formula-schematic">
                        <div class="schematic-node input-node">
                          <div class="node-label">基础设定</div>
                          <div class="node-value">{s}分 &times; {Math.round(w * 100)}%</div>
                        </div>
                        <div class="schematic-connector">+</div>
                        <div class="schematic-node input-node">
                          <div class="node-label">过程遥测</div>
                          <div class="node-value">{Math.round(metricsScore * 100)/100}分 &times; {Math.round((1 - w) * 100)}%</div>
                        </div>
                        <div class="schematic-connector">=</div>
                        <div class="schematic-node result-node {getScoreColorClass(selectedProjectScore.compound_score)}">
                          <div class="node-label">PHDI 综合</div>
                          <div class="node-value">{selectedProjectScore.compound_score}分</div>
                        </div>
                      </div>
                    {:else}
                      <div class="formula-schematic">
                        <div class="schematic-node input-node">
                          <div class="node-label">默认基础</div>
                          <div class="node-value">60分 &times; 10%</div>
                        </div>
                        <div class="schematic-connector">+</div>
                        <div class="schematic-node input-node">
                          <div class="node-label">度量过程</div>
                          <div class="node-value">{Math.round((selectedProjectScore.compound_score - 6) / 0.9 * 100)/100}分 &times; 90%</div>
                        </div>
                        <div class="schematic-connector">=</div>
                        <div class="schematic-node result-node {getScoreColorClass(selectedProjectScore.compound_score)}">
                          <div class="node-label">PHDI 综合</div>
                          <div class="node-value">{selectedProjectScore.compound_score}分</div>
                        </div>
                      </div>
                    {/if}
                  </div>
                </div>
              </div>
            </div>

            <!-- 右栏：基础属性与遥测细目、行动建议 -->
            <div class="column-right">
              <div class="detail-section">
                <h4>基础属性</h4>
                <div class="detail-grid">
                  <div class="detail-item">
                    <span class="label">项目标识:</span>
                    <span class="value highlight">{selectedProjectScore.project_key}</span>
                  </div>
                  <div class="detail-item">
                    <span class="label">当前所处阶段:</span>
                    <span class="value badge">{
                      selectedConfig?.project_phase || '交付'
                    }</span>
                  </div>
                  <div class="detail-item">
                    <span class="label">基准优先级:</span>
                    <span class="value">{
                      selectedConfig?.base_priority || 'P1'
                    }</span>
                  </div>
                  <div class="detail-item">
                    <span class="label">快照周期:</span>
                    <span class="value">{selectedProjectScore.snapshot_date || '当前周期'}</span>
                  </div>
                </div>
              </div>

              <div class="detail-section">
                <h4>核心维度明细</h4>
                <div class="metrics-grid-vertical">
                  <!-- SH -->
                  <div class="metric-row-card {selectedProjectScore.schedule_health_score >= 85 ? 'metric-tone-safe' : (selectedProjectScore.schedule_health_score >= 70 ? 'metric-tone-warn' : 'metric-tone-danger')}">
                    <div class="row-header">
                      <span class="row-title">进度排期健康 [SH]</span>
                      <div class="row-status">
                        <span class="row-status-tag font-mono {selectedProjectScore.schedule_health_score >= 85 ? 'text-emerald' : (selectedProjectScore.schedule_health_score >= 70 ? 'text-amber' : 'text-rose')}">
                          {selectedProjectScore.schedule_health_score >= 85 ? '正常' : (selectedProjectScore.schedule_health_score >= 70 ? '风险' : '滞后')}
                        </span>
                        <span class="row-score text-blue">{selectedProjectScore.schedule_health_score}%</span>
                      </div>
                    </div>
                    <div class="row-progress">
                      <span class="bg-blue" style="width: {selectedProjectScore.schedule_health_score}%"></span>
                    </div>
                  </div>
                  <!-- EQ -->
                  <div class="metric-row-card {selectedProjectScore.engineering_quality >= 85 ? 'metric-tone-safe' : (selectedProjectScore.engineering_quality >= 70 ? 'metric-tone-warn' : 'metric-tone-danger')}">
                    <div class="row-header">
                      <span class="row-title">工程质量 [EQ]</span>
                      <div class="row-status">
                        <span class="row-status-tag font-mono {selectedProjectScore.engineering_quality >= 85 ? 'text-emerald' : (selectedProjectScore.engineering_quality >= 70 ? 'text-amber' : 'text-rose')}">
                          {selectedProjectScore.engineering_quality >= 85 ? '正常' : (selectedProjectScore.engineering_quality >= 70 ? '待绑' : '无提交')}
                        </span>
                        <span class="row-score text-emerald">{selectedProjectScore.engineering_quality}%</span>
                      </div>
                    </div>
                    <div class="row-progress">
                      <span class="bg-emerald" style="width: {selectedProjectScore.engineering_quality}%"></span>
                    </div>
                  </div>
                  <!-- CE -->
                  <div class="metric-row-card {selectedProjectScore.collaboration_effic >= 85 ? 'metric-tone-safe' : (selectedProjectScore.collaboration_effic >= 70 ? 'metric-tone-warn' : 'metric-tone-danger')}">
                    <div class="row-header">
                      <span class="row-title">指派协同效率 [CE]</span>
                      <div class="row-status">
                        <span class="row-status-tag font-mono {selectedProjectScore.collaboration_effic >= 85 ? 'text-emerald' : (selectedProjectScore.collaboration_effic >= 70 ? 'text-amber' : 'text-rose')}">
                          {selectedProjectScore.collaboration_effic >= 85 ? '均衡' : (selectedProjectScore.collaboration_effic >= 70 ? '偏载' : '严重不均')}
                        </span>
                        <span class="row-score text-amber">{selectedProjectScore.collaboration_effic}%</span>
                      </div>
                    </div>
                    <div class="row-progress">
                      <span class="bg-amber" style="width: {selectedProjectScore.collaboration_effic}%"></span>
                    </div>
                  </div>
                  <!-- SI -->
                  <div class="metric-row-card {selectedProjectScore.stability_index >= 85 ? 'metric-tone-safe' : (selectedProjectScore.stability_index >= 70 ? 'metric-tone-warn' : 'metric-tone-danger')}">
                    <div class="row-header">
                      <span class="row-title">缺陷与稳定性 [SI]</span>
                      <div class="row-status">
                        <span class="row-status-tag font-mono {selectedProjectScore.stability_index >= 85 ? 'text-emerald' : (selectedProjectScore.stability_index >= 70 ? 'text-amber' : 'text-rose')}">
                          {selectedProjectScore.stability_index >= 85 ? '稳定' : (selectedProjectScore.stability_index >= 70 ? '新增' : '高危')}
                        </span>
                        <span class="row-score text-rose">{selectedProjectScore.stability_index}%</span>
                      </div>
                    </div>
                    <div class="row-progress">
                      <span class="bg-rose" style="width: {selectedProjectScore.stability_index}%"></span>
                    </div>
                  </div>
                </div>
              </div>

              <!-- well-ambient 改进建议 -->
              <div class="detail-section">
                <h4>人工介入方向</h4>
                <div class="action-recommendations">
                  {#each selectedDiagnosis.dimensions.filter(d => d.tone !== 'safe') as dim}
                    <div class="recommendation-item {getDimensionToneClass(dim.tone)}">
                      <span class="rec-bullet">{dim.code}</span>
                      <p><b>{dim.label}</b>：{dim.action}。{dim.evidence}。</p>
                    </div>
                  {/each}
                  {#if selectedDiagnosis.dimensions.every(d => d.tone === 'safe')}
                    <div class="recommendation-item success">
                      <span class="rec-bullet">OK</span>
                      <p>所有维度稳定，暂不触发人工介入，继续后台观测。</p>
                    </div>
                  {/if}
                  <div class="recommendation-route">
                    <span>建议入口</span>
                    <strong>{selectedDiagnosis.level === 'green' ? '保持观测' : '排期治理台 / 红区卡点诊断盘 / 负责人调停记录'}</strong>
                  </div>
                </div>
              </div>

            </div>
          </div>
        </div>
      </div>
    </div>
  {/if}
</div>

<style>
  /* Header indicator dots */
  .pulse-status-dot {
    display: inline-block;
    width: 8px;
    height: 8px;
    border-radius: 50%;
    margin-right: 6px;
    vertical-align: middle;
  }
  .pulse-status-dot.online {
    background-color: #10b981;
    box-shadow: 0 0 10px #10b981;
    animation: statusPulse 2s infinite ease-in-out;
  }

  .collapse-chevron {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    color: #6366f1;
    margin-left: 8px;
    transition: transform 0.25s cubic-bezier(0.4, 0, 0.2, 1);
  }
  .collapse-chevron.is-collapsed {
    transform: rotate(-90deg);
  }

  /* Modal Styles */
  .modal-backdrop {
    position: fixed;
    top: 0;
    left: 0;
    width: 100vw;
    height: 100vh;
    background: rgba(4, 6, 12, 0.75);
    backdrop-filter: blur(20px);
    -webkit-backdrop-filter: blur(20px);
    z-index: 1000;
    display: flex;
    align-items: center;
    justify-content: center;
    animation: fadeIn 0.2s ease-out;
  }

  .modal-content {
    width: 840px;
    max-width: 95vw;
    max-height: 90vh;
    background: rgba(10, 16, 32, 0.95);
    border: 1px solid rgba(99, 102, 241, 0.35);
    border-radius: 16px;
    padding: 26px;
    display: flex;
    flex-direction: column;
    box-shadow: 0 25px 60px rgba(0, 0, 0, 0.8), inset 0 1px 0 rgba(255, 255, 255, 0.05);
    color: #f1f5f9;
    overflow-y: auto;
    transition: border-color 0.3s;
  }
  .modal-content:hover {
    border-color: rgba(99, 102, 241, 0.5);
  }

  /* Custom scrollbar for modal-content */
  .modal-content::-webkit-scrollbar {
    width: 6px;
  }
  .modal-content::-webkit-scrollbar-track {
    background: rgba(15, 23, 42, 0.3);
    border-radius: 3px;
  }
  .modal-content::-webkit-scrollbar-thumb {
    background: rgba(99, 102, 241, 0.4);
    border-radius: 3px;
    transition: background 0.2s;
  }
  .modal-content::-webkit-scrollbar-thumb:hover {
    background: rgba(99, 102, 241, 0.7);
  }

  /* 大分健康度标签 */
  .health-badge {
    padding: 3px 10px;
    border-radius: 6px;
    font-size: 0.72rem;
    font-weight: 800;
  }
  .health-badge.score-green {
    background: rgba(16, 185, 129, 0.12);
    color: #34d399;
    border: 1px solid rgba(16, 185, 129, 0.3);
  }
  .health-badge.score-yellow {
    background: rgba(245, 158, 11, 0.1);
    color: #fbbf24;
    border: 1px solid rgba(245, 158, 11, 0.25);
  }
  .health-badge.score-red {
    background: rgba(244, 63, 94, 0.12);
    color: #f43f5e;
    border: 1px solid rgba(244, 63, 94, 0.3);
  }

  /* 核心指标行状态标签 */
  .row-status-tag {
    font-size: 0.65rem;
    font-weight: 800;
    padding: 1px 6px;
    border-radius: 4px;
    background: rgba(255, 255, 255, 0.03);
    border: 1px solid rgba(255, 255, 255, 0.05);
  }
  .row-status-tag.text-emerald {
    color: #34d399;
    background: rgba(16, 185, 129, 0.06);
    border-color: rgba(16, 185, 129, 0.15);
  }
  .row-status-tag.text-amber {
    color: #fbbf24;
    background: rgba(245, 158, 11, 0.05);
    border-color: rgba(245, 158, 11, 0.15);
  }
  .row-status-tag.text-rose {
    color: #f43f5e;
    background: rgba(244, 63, 94, 0.06);
    border-color: rgba(244, 63, 94, 0.15);
  }

  /* 智能行动建议 */
  .action-recommendations {
    display: flex;
    flex-direction: column;
    gap: 10px;
    background: rgba(15, 23, 42, 0.4);
    border: 1px solid rgba(255, 255, 255, 0.05);
    border-radius: 8px;
    padding: 14px;
  }

  .recommendation-item {
    display: flex;
    gap: 8px;
    align-items: flex-start;
    font-size: 0.72rem;
    line-height: 1.4;
    border: 1px solid rgba(71, 85, 105, 0.32);
    background: rgba(2, 6, 23, 0.22);
    border-radius: 8px;
    padding: 9px 10px;
  }

  .recommendation-item p {
    margin: 0;
    color: #94a3b8;
  }

  .recommendation-item b {
    color: #e2e8f0;
  }

  .rec-bullet {
    min-width: 30px;
    height: 24px;
    border-radius: 999px;
    display: inline-flex;
    align-items: center;
    justify-content: center;
    font-size: 0.64rem;
    font-weight: 900;
    background: rgba(99, 102, 241, 0.15);
    color: #c7d2fe;
    flex-shrink: 0;
  }

  .recommendation-item.tone-danger {
    border-color: rgba(244, 63, 94, 0.32);
    background: rgba(127, 29, 29, 0.08);
  }

  .recommendation-item.tone-warn {
    border-color: rgba(245, 158, 11, 0.28);
    background: rgba(120, 53, 15, 0.08);
  }

  .recommendation-item.tone-safe {
    border-color: rgba(16, 185, 129, 0.24);
    background: rgba(6, 78, 59, 0.08);
  }

  .rec-link {
    color: #6366f1;
    text-decoration: none;
    font-weight: 700;
    border-bottom: 1px dashed rgba(99, 102, 241, 0.4);
    transition: color 0.15s, border-color 0.15s;
  }

  .rec-link:hover {
    color: #a5b4fc;
    border-bottom-color: #a5b4fc;
  }

  .recommendation-item.success p {
    color: #34d399;
  }

  .recommendation-route {
    display: flex;
    justify-content: space-between;
    gap: 12px;
    padding-top: 10px;
    border-top: 1px dashed rgba(148, 163, 184, 0.16);
    color: #64748b;
    font-size: 0.7rem;
  }

  .recommendation-route strong {
    color: #cbd5e1;
    text-align: right;
  }

  /* Double column dashboard grid */
  .dashboard-columns {
    display: grid;
    grid-template-columns: 1.2fr 1fr;
    gap: 24px;
    align-items: start;
  }

  .column-left, .column-right {
    display: flex;
    flex-direction: column;
    gap: 20px;
  }

  @media (max-width: 768px) {
    .dashboard-columns {
      grid-template-columns: 1fr;
      gap: 20px;
    }
  }

  /* 垂直核心指标明细 */
  .metrics-grid-vertical {
    display: flex;
    flex-direction: column;
    gap: 12px;
  }

  .metric-row-card {
    background: rgba(15, 23, 42, 0.4);
    border: 1px solid rgba(255, 255, 255, 0.05);
    border-radius: 8px;
    padding: 12px 14px;
    display: flex;
    flex-direction: column;
    gap: 8px;
  }

  .metric-row-card .row-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
  }

  .metric-row-card .row-title {
    font-size: 0.72rem;
    color: #94a3b8;
    font-weight: 700;
  }

  .metric-row-card .row-score {
    font-size: 0.85rem;
    font-weight: 800;
  }

  .metric-row-card .row-progress {
    height: 6px;
    background: rgba(255, 255, 255, 0.05);
    border-radius: 3px;
    overflow: hidden;
    position: relative;
  }

  .metric-row-card .row-progress span {
    display: block;
    height: 100%;
    border-radius: 3px;
  }

  .metric-row-card .row-progress span.bg-blue { background: #6366f1; box-shadow: 0 0 8px rgba(99, 102, 241, 0.6); }
  .metric-row-card .row-progress span.bg-emerald { background: #10b981; box-shadow: 0 0 8px rgba(16, 185, 129, 0.6); }
  .metric-row-card .row-progress span.bg-amber { background: #f59e0b; box-shadow: 0 0 8px rgba(245, 158, 11, 0.6); }
  .metric-row-card .row-progress span.bg-rose { background: #f43f5e; box-shadow: 0 0 8px rgba(244, 63, 94, 0.6); }

  .modal-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
    border-bottom: 1px solid rgba(255, 255, 255, 0.08);
    padding-bottom: 16px;
    margin-bottom: 22px;
  }

  .modal-header h3 {
    margin: 0;
    font-size: 1.15rem;
    font-weight: 800;
    color: #f1f5f9;
    letter-spacing: 0;
  }

  .close-btn {
    background: transparent;
    border: none;
    color: #64748b;
    font-size: 1.5rem;
    cursor: pointer;
    transition: color 0.2s, transform 0.2s;
  }
  .close-btn:hover {
    color: #f8fafc;
    transform: scale(1.1);
  }

  .modal-body {
    flex: 1;
    overflow-y: auto;
    display: flex;
    flex-direction: column;
    gap: 24px;
    padding-right: 4px;
  }

  .intervention-brief-card {
    border: 1px solid rgba(71, 85, 105, 0.42);
    border-radius: 12px;
    background: rgba(2, 6, 23, 0.34);
    padding: 16px;
    display: flex;
    flex-direction: column;
    gap: 14px;
  }

  .intervention-brief-card.score-red {
    border-color: rgba(244, 63, 94, 0.36);
    background: rgba(127, 29, 29, 0.1);
  }

  .intervention-brief-card.score-yellow {
    border-color: rgba(245, 158, 11, 0.32);
    background: rgba(120, 53, 15, 0.08);
  }

  .intervention-brief-card.score-green {
    border-color: rgba(16, 185, 129, 0.28);
    background: rgba(6, 78, 59, 0.08);
  }

  .intervention-brief-head {
    display: flex;
    justify-content: space-between;
    align-items: flex-start;
    gap: 16px;
  }

  .intervention-brief-head span {
    color: #94a3b8;
    font-size: 0.68rem;
    font-weight: 900;
    letter-spacing: 0.08em;
  }

  .intervention-brief-head h4 {
    margin: 5px 0 0;
    color: #f8fafc;
    font-size: 0.98rem;
    line-height: 1.35;
    text-wrap: balance;
  }

  .intervention-brief-head > strong {
    color: #f8fafc;
    font-size: 1.8rem;
    line-height: 1;
    font-variant-numeric: tabular-nums;
  }

  .intervention-brief-grid {
    display: grid;
    grid-template-columns: repeat(3, minmax(0, 1fr));
    gap: 10px;
  }

  .intervention-brief-grid div {
    min-width: 0;
    border: 1px solid rgba(71, 85, 105, 0.34);
    background: rgba(15, 23, 42, 0.48);
    border-radius: 8px;
    padding: 10px;
  }

  .intervention-brief-grid span {
    display: block;
    color: #64748b;
    font-size: 0.66rem;
    font-weight: 800;
    margin-bottom: 5px;
  }

  .intervention-brief-grid strong {
    color: #dbeafe;
    font-size: 0.74rem;
    line-height: 1.35;
  }

  .detail-section h4 {
    margin: 0 0 12px 0;
    font-size: 0.82rem;
    color: #818cf8;
    text-transform: uppercase;
    letter-spacing: 0.08em;
    border-bottom: 1px solid rgba(129, 140, 248, 0.16);
    padding-bottom: 8px;
  }

  .detail-grid {
    display: grid;
    grid-template-columns: repeat(2, 1fr);
    gap: 14px;
    background: rgba(15, 23, 42, 0.4);
    border: 1px solid rgba(255, 255, 255, 0.06);
    border-radius: 8px;
    padding: 16px;
  }

  .detail-item {
    display: flex;
    justify-content: space-between;
    font-size: 0.76rem;
  }

  .detail-item .label {
    color: #64748b;
  }

  .detail-item .value {
    color: #cbd5e1;
    font-weight: 700;
  }

  .detail-item .value.highlight {
    color: #38bdf8;
    text-shadow: 0 0 8px rgba(56, 189, 248, 0.3);
  }

  .detail-item .value.badge {
    background: rgba(99, 102, 241, 0.15);
    border: 1px solid rgba(99, 102, 241, 0.3);
    padding: 1px 8px;
    border-radius: 4px;
    color: #a5b4fc;
  }

  .phdi-display {
    display: flex;
    align-items: center;
    gap: 24px;
    background: rgba(15, 23, 42, 0.4);
    border: 1px solid rgba(255, 255, 255, 0.06);
    border-radius: 8px;
    padding: 20px;
    flex-wrap: wrap;
  }

  .big-score {
    font-size: 2.6rem;
    font-weight: 900;
    display: flex;
    align-items: baseline;
    gap: 4px;
  }
  .big-score.score-green {
    color: #10b981;
    text-shadow: 0 0 15px rgba(16, 185, 129, 0.4);
  }
  .big-score.score-yellow {
    color: #fbbf24;
    text-shadow: 0 0 15px rgba(251, 191, 36, 0.4);
  }
  .big-score.score-red {
    color: #f43f5e;
    text-shadow: 0 0 15px rgba(244, 63, 94, 0.4);
  }

  .big-score .unit {
    font-size: 0.95rem;
    font-weight: 600;
    color: #64748b;
    text-shadow: none;
  }

  .formula-breakdown {
    flex: 1;
    min-width: 280px;
  }

  .formula-breakdown h5 {
    margin: 0 0 10px 0;
    color: #94a3b8;
    font-size: 0.74rem;
    letter-spacing: 0.02em;
  }

  /* Hardware Schematic Formula style */
  .formula-schematic {
    display: flex;
    align-items: center;
    gap: 12px;
    font-size: 0.74rem;
    background: rgba(8, 12, 24, 0.6);
    border: 1px solid rgba(99, 102, 241, 0.15);
    border-radius: 8px;
    padding: 12px 14px;
    width: 100%;
    box-sizing: border-box;
  }

  .schematic-node {
    flex: 1;
    display: flex;
    flex-direction: column;
    align-items: center;
    padding: 6px;
    border-radius: 4px;
    background: rgba(255, 255, 255, 0.02);
    border: 1px dashed rgba(255, 255, 255, 0.08);
  }

  .schematic-node .node-label {
    font-size: 0.62rem;
    color: #64748b;
    margin-bottom: 4px;
    text-transform: uppercase;
  }

  .schematic-node .node-value {
    font-weight: 700;
    color: #e2e8f0;
  }

  .schematic-node.result-node {
    background: rgba(99, 102, 241, 0.08);
    border: 1px solid rgba(99, 102, 241, 0.3);
  }
  .schematic-node.result-node.score-green { color: #34d399; border-color: rgba(16, 185, 129, 0.4); background: rgba(16, 185, 129, 0.06); }
  .schematic-node.result-node.score-yellow { color: #fbbf24; border-color: rgba(251, 191, 36, 0.3); background: rgba(251, 191, 36, 0.05); }
  .schematic-node.result-node.score-red { color: #f43f5e; border-color: rgba(244, 63, 94, 0.4); background: rgba(244, 63, 94, 0.06); }
  .schematic-node.result-node.score-green .node-value { color: #34d399; }
  .schematic-node.result-node.score-yellow .node-value { color: #fbbf24; }
  .schematic-node.result-node.score-red .node-value { color: #f43f5e; }

  .schematic-connector {
    font-size: 0.9rem;
    font-weight: 800;
    color: #475569;
  }

  /* Metrics Grid in modal */
  .metrics-grid {
    display: grid;
    grid-template-columns: repeat(2, 1fr);
    gap: 16px;
  }

  .metric-card {
    background: rgba(15, 23, 42, 0.3);
    border: 1px solid rgba(255, 255, 255, 0.05);
    border-radius: 8px;
    padding: 14px 16px;
    display: flex;
    flex-direction: column;
    gap: 8px;
    transition: transform 0.2s, border-color 0.2s;
  }
  .metric-card:hover {
    transform: translateY(-2px);
    border-color: rgba(99, 102, 241, 0.25);
  }

  .metric-card .m-title {
     font-size: 0.72rem;
     color: #64748b;
     font-weight: 800;
     letter-spacing: 0.02em;
  }

  .m-score-row {
    display: flex;
    justify-content: space-between;
    align-items: center;
    gap: 12px;
  }

  .metric-card .m-score {
     font-size: 1.25rem;
     font-weight: 800;
  }

  .m-mini-bar {
    flex: 1;
    height: 4px;
    background: rgba(255, 255, 255, 0.05);
    border-radius: 2px;
    overflow: hidden;
  }
  .m-mini-bar span {
    display: block;
    height: 100%;
    border-radius: 2px;
  }
  .m-mini-bar span.bg-blue { background: #6366f1; box-shadow: 0 0 6px #6366f1; }
  .m-mini-bar span.bg-emerald { background: #10b981; box-shadow: 0 0 6px #10b981; }
  .m-mini-bar span.bg-amber { background: #f59e0b; box-shadow: 0 0 6px #f59e0b; }
  .m-mini-bar span.bg-rose { background: #f43f5e; box-shadow: 0 0 6px #f43f5e; }

  .text-blue { color: #818cf8; }
  .text-emerald { color: #34d399; }
  .text-amber { color: #fbbf24; }
  .text-rose { color: #f43f5e; }

  .metric-card .m-desc {
    margin: 0;
    font-size: 0.68rem;
    color: #64748b;
    line-height: 1.4;
  }

  .diagnostic-panel-modal {
    border-radius: 8px;
    padding: 16px;
    font-size: 0.74rem;
    border: 1px solid transparent;
  }
  .diag-header-row {
    display: flex;
    align-items: center;
    gap: 6px;
    font-size: 0.62rem;
    font-weight: 800;
    letter-spacing: 0.08em;
    color: #64748b;
    margin-bottom: 10px;
    border-bottom: 1px dashed rgba(255, 255, 255, 0.05);
    padding-bottom: 6px;
  }

  .diagnostic-panel-modal.score-green {
    background: rgba(16, 185, 129, 0.03);
    border-color: rgba(16, 185, 129, 0.15);
    color: #34d399;
  }

  .diagnostic-panel-modal.score-yellow {
    background: rgba(245, 158, 11, 0.03);
    border-color: rgba(245, 158, 11, 0.15);
    color: #fbbf24;
  }

  .diagnostic-panel-modal.score-red {
    background: rgba(244, 63, 94, 0.03);
    border-color: rgba(244, 63, 94, 0.15);
    color: #f43f5e;
  }

  .modal-footer {
    border-top: 1px solid rgba(255, 255, 255, 0.08);
    padding-top: 16px;
    margin-top: 22px;
    display: flex;
    justify-content: flex-end;
  }

  .telemetry-panel {
    margin-bottom: 24px;
    box-sizing: border-box;
    overflow: hidden;
    background:
      linear-gradient(145deg, rgba(244, 251, 255, 0.072), rgba(244, 251, 255, 0.018) 42%, rgba(38, 221, 255, 0.042)),
      rgba(8, 12, 17, 0.74);
    border: 1px solid rgba(203, 234, 244, 0.13);
    border-radius: 12px;
    backdrop-filter: blur(24px) saturate(132%);
    -webkit-backdrop-filter: blur(24px) saturate(132%);
    padding: 20px;
    box-shadow:
      0 22px 58px rgba(1, 9, 14, 0.38),
      inset 0 1px 0 rgba(244, 251, 255, 0.07);
  }

  .flex-header {
    display: flex;
    justify-content: space-between;
    align-items: flex-start;
    gap: 16px;
    flex-wrap: wrap;
    padding-bottom: 16px;
    border-bottom: 1px solid rgba(203, 234, 244, 0.1);
  }

  .eyebrow {
    font-size: 0.65rem;
    font-weight: 800;
    color: #90a8b5;
    letter-spacing: 0;
    display: block;
    margin-bottom: 6px;
    text-transform: uppercase;
  }

  .title-toggle {
    appearance: none;
    border: 0;
    padding: 0;
    background: transparent;
    text-align: left;
    cursor: pointer;
    font: inherit;
    color: inherit;
  }

  .title-toggle:focus-visible {
    outline: 2px solid rgba(38, 221, 255, 0.78);
    outline-offset: 6px;
    border-radius: 8px;
  }

  .filter-controls {
    display: flex;
    align-items: center;
    gap: 12px;
    min-height: 38px;
  }

  .search-input-wrapper {
    position: relative;
    width: min(300px, 46vw);
  }

  .search-icon {
    position: absolute;
    left: 13px;
    top: 50%;
    width: 12px;
    height: 12px;
    border: 1.6px solid rgba(144, 168, 181, 0.86);
    border-radius: 999px;
    transform: translateY(-54%);
    pointer-events: none;
  }

  .search-icon::after {
    content: '';
    position: absolute;
    right: -4px;
    bottom: -3px;
    width: 6px;
    height: 1.6px;
    border-radius: 999px;
    background: rgba(144, 168, 181, 0.86);
    transform: rotate(45deg);
    transform-origin: left center;
  }

  .telemetry-search-input {
    width: 100%;
    min-height: 38px;
    box-sizing: border-box;
    background: rgba(244, 251, 255, 0.045);
    border: 1px solid rgba(203, 234, 244, 0.13);
    border-radius: 8px;
    padding: 0 12px 0 38px;
    color: #f4fbff;
    font-size: 0.78rem;
    outline: none;
    transition: border-color 0.16s ease, background 0.16s ease, box-shadow 0.16s ease;
  }

  .telemetry-search-input:focus {
    border-color: rgba(38, 221, 255, 0.58);
    background: rgba(244, 251, 255, 0.07);
    box-shadow: 0 0 0 3px rgba(38, 221, 255, 0.11);
  }

  .telemetry-search-input::placeholder {
    color: #5f7582;
  }

  .refresh-btn {
    min-height: 38px;
    background: rgba(38, 221, 255, 0.1);
    border: 1px solid rgba(38, 221, 255, 0.24);
    color: #b8f5ff;
    border-radius: 8px;
    padding: 7px 14px;
    font-size: 0.74rem;
    font-weight: 800;
    cursor: pointer;
    transition: transform 0.16s ease, border-color 0.16s ease, background 0.16s ease, box-shadow 0.16s ease;
  }

  .refresh-btn:hover {
    transform: translateY(-1px);
    background: rgba(38, 221, 255, 0.15);
    border-color: rgba(38, 221, 255, 0.42);
    color: #f8fafc;
    box-shadow: 0 14px 30px rgba(1, 9, 14, 0.22);
  }

  .refresh-btn:disabled {
    cursor: wait;
    opacity: 0.58;
    transform: none;
    box-shadow: none;
  }

  .health-command-board {
    margin-top: 18px;
    display: grid;
    grid-template-columns: minmax(0, 1.45fr) minmax(300px, 0.9fr);
    gap: 14px;
  }

  .health-command-primary {
    border: 1px solid rgba(71, 85, 105, 0.42);
    border-radius: 12px;
    background:
      linear-gradient(180deg, rgba(15, 23, 42, 0.8), rgba(2, 6, 23, 0.6)),
      radial-gradient(circle at 12% 0%, rgba(56, 189, 248, 0.1), transparent 36%);
    padding: 16px;
    display: flex;
    flex-direction: column;
    gap: 14px;
    min-width: 0;
  }

  .health-command-primary.score-red {
    border-color: rgba(244, 63, 94, 0.34);
    background:
      linear-gradient(180deg, rgba(30, 10, 18, 0.82), rgba(2, 6, 23, 0.62)),
      radial-gradient(circle at 12% 0%, rgba(244, 63, 94, 0.16), transparent 38%);
  }

  .health-command-primary.score-yellow {
    border-color: rgba(245, 158, 11, 0.3);
    background:
      linear-gradient(180deg, rgba(28, 18, 8, 0.84), rgba(2, 6, 23, 0.62)),
      radial-gradient(circle at 12% 0%, rgba(245, 158, 11, 0.16), transparent 38%);
  }

  .command-kicker {
    color: #94a3b8;
    font-size: 0.68rem;
    font-weight: 900;
    letter-spacing: 0.08em;
  }

  .command-title-row {
    display: flex;
    align-items: flex-start;
    justify-content: space-between;
    gap: 18px;
  }

  .command-title-row p {
    margin: 7px 0 0;
    color: #94a3b8;
    font-size: 0.78rem;
    line-height: 1.45;
  }

  .command-score {
    flex: none;
    color: #f8fafc;
    font-size: 2rem;
    font-weight: 900;
    line-height: 1;
    font-variant-numeric: tabular-nums;
  }

  .command-evidence-grid {
    display: grid;
    grid-template-columns: repeat(3, minmax(0, 1fr));
    gap: 8px;
  }

  .command-evidence-grid div {
    border: 1px solid rgba(71, 85, 105, 0.34);
    border-radius: 8px;
    background: rgba(2, 6, 23, 0.32);
    padding: 9px;
    min-width: 0;
  }

  .command-evidence-grid span {
    display: block;
    color: #64748b;
    font-size: 0.64rem;
    font-weight: 800;
    margin-bottom: 5px;
  }

  .command-evidence-grid strong {
    display: block;
    color: #cbd5e1;
    font-size: 0.72rem;
    line-height: 1.35;
  }

  .command-action-row {
    display: flex;
    align-items: center;
    gap: 8px;
    flex-wrap: wrap;
  }

  .health-status-matrix {
    display: grid;
    grid-template-columns: repeat(2, minmax(0, 1fr));
    gap: 10px;
  }

  .health-status-card {
    min-height: 92px;
    border: 1px solid rgba(71, 85, 105, 0.36);
    border-radius: 10px;
    background: rgba(2, 6, 23, 0.28);
    padding: 12px;
    display: flex;
    flex-direction: column;
    justify-content: space-between;
  }

  .health-status-card span,
  .health-status-card em {
    color: #64748b;
    font-size: 0.68rem;
    font-style: normal;
    font-weight: 800;
  }

  .health-status-card strong {
    color: #f8fafc;
    font-size: 1.6rem;
    line-height: 1;
    font-variant-numeric: tabular-nums;
  }

  .health-status-card.is-red strong { color: #fb7185; }
  .health-status-card.is-yellow strong { color: #fbbf24; }
  .health-status-card.is-green strong { color: #34d399; }

  .priority-diagnostic-strip {
    margin-top: 14px;
    border: 1px solid rgba(71, 85, 105, 0.38);
    border-radius: 12px;
    background: rgba(2, 6, 23, 0.28);
    padding: 12px;
  }

  .strip-header {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 12px;
    margin-bottom: 10px;
  }

  .strip-header strong {
    color: #e2e8f0;
    font-size: 0.8rem;
  }

  .strip-header span {
    color: #64748b;
    font-size: 0.68rem;
  }

  .priority-card-grid {
    display: grid;
    grid-template-columns: repeat(3, minmax(0, 1fr));
    gap: 10px;
  }

  .priority-card {
    text-align: left;
    border: 1px solid rgba(71, 85, 105, 0.36);
    border-radius: 10px;
    background: rgba(15, 23, 42, 0.44);
    padding: 12px;
    color: inherit;
    cursor: pointer;
    display: flex;
    flex-direction: column;
    gap: 10px;
    min-width: 0;
    transition: transform 0.16s ease, border-color 0.16s ease, background 0.16s ease;
  }

  .priority-card:hover,
  .priority-card:focus-visible {
    transform: translateY(-1px);
    border-color: rgba(129, 140, 248, 0.45);
    background: rgba(30, 41, 59, 0.58);
    outline: none;
  }

  .priority-card.score-red {
    border-color: rgba(244, 63, 94, 0.32);
  }

  .priority-card.score-yellow {
    border-color: rgba(245, 158, 11, 0.28);
  }

  .priority-card-head {
    display: grid;
    grid-template-columns: auto minmax(0, 1fr) auto;
    align-items: center;
    gap: 9px;
  }

  .priority-card-head strong {
    display: block;
    color: #f8fafc;
    font-size: 0.78rem;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .priority-card-head span {
    display: block;
    color: #64748b;
    font-size: 0.66rem;
    margin-top: 2px;
  }

  .priority-card-head em {
    color: #e2e8f0;
    font-style: normal;
    font-weight: 900;
    font-size: 0.95rem;
  }

  .priority-card p {
    margin: 0;
    color: #cbd5e1;
    font-size: 0.72rem;
    line-height: 1.42;
  }

  .priority-card-footer {
    display: flex;
    gap: 8px;
    flex-wrap: wrap;
  }

  .priority-card-footer span {
    color: #94a3b8;
    background: rgba(2, 6, 23, 0.34);
    border: 1px solid rgba(71, 85, 105, 0.34);
    border-radius: 999px;
    padding: 4px 7px;
    font-size: 0.64rem;
    font-weight: 800;
  }

  .stable-state {
    border: 1px solid rgba(114, 230, 180, 0.22);
    background:
      linear-gradient(135deg, rgba(114, 230, 180, 0.1), rgba(244, 251, 255, 0.025)),
      rgba(6, 78, 59, 0.08);
    border-radius: 8px;
    padding: 14px;
    display: flex;
    justify-content: space-between;
    gap: 12px;
    color: #90a8b5;
    font-size: 0.74rem;
  }

  .stable-state strong {
    color: #34d399;
  }

  .refresh-btn:active {
    transform: scale(0.97);
  }

  .loading-state, .error-state, .empty-state {
    margin-top: 16px;
    padding: 34px;
    text-align: left;
    border: 1px solid rgba(203, 234, 244, 0.12);
    border-radius: 8px;
    color: #90a8b5;
    font-size: 0.82rem;
    background:
      linear-gradient(135deg, rgba(244, 251, 255, 0.05), rgba(244, 251, 255, 0.014)),
      rgba(8, 12, 24, 0.34);
    box-shadow: inset 0 1px 0 rgba(244, 251, 255, 0.045);
  }

  .error-state {
    color: #ffd3da;
    background:
      linear-gradient(135deg, rgba(255, 97, 119, 0.12), rgba(244, 251, 255, 0.014)),
      rgba(42, 14, 21, 0.28);
    border-color: rgba(255, 97, 119, 0.22);
  }

  .table-responsive {
    overflow-x: auto;
    border-radius: 8px;
    border: 1px solid rgba(203, 234, 244, 0.11);
    background: rgba(6, 10, 20, 0.42);
    margin-top: 16px;
    box-shadow: inset 0 1px 0 rgba(244, 251, 255, 0.035);
  }

  .telemetry-table {
    width: 100%;
    border-collapse: collapse;
    text-align: left;
    font-size: 0.8rem;
  }

  .project-info-cell {
    display: flex;
    align-items: center;
    gap: 12px;
  }

  .project-avatar {
    width: 32px;
    height: 32px;
    border-radius: 8px;
    color: #e2e8f0;
    display: flex;
    align-items: center;
    justify-content: center;
    font-weight: 800;
    font-size: 0.85rem;
    border: 1px solid rgba(255, 255, 255, 0.08);
    background: rgba(15, 23, 42, 0.72);
    flex-shrink: 0;
  }

  .project-avatar.score-red {
    color: #fecdd3;
    border-color: rgba(244, 63, 94, 0.42);
    background: rgba(127, 29, 29, 0.24);
  }

  .project-avatar.score-yellow {
    color: #fde68a;
    border-color: rgba(245, 158, 11, 0.42);
    background: rgba(120, 53, 15, 0.22);
  }

  .project-avatar.score-green {
    color: #bbf7d0;
    border-color: rgba(16, 185, 129, 0.34);
    background: rgba(6, 78, 59, 0.2);
  }

  .project-meta {
    display: flex;
    flex-direction: column;
    gap: 2px;
    min-width: 0;
  }

  .project-meta strong {
    color: #e2e8f0;
    font-size: 0.82rem;
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }

  .text-muted {
    color: #64748b;
    font-size: 0.72rem;
  }

  .metric-micro-grid {
    display: grid;
    grid-template-columns: repeat(4, minmax(0, 1fr));
    gap: 6px;
  }

  .metric-micro-item {
    position: relative;
    overflow: hidden;
    min-height: 40px;
    border: 1px solid rgba(71, 85, 105, 0.34);
    border-radius: 8px;
    background: rgba(2, 6, 23, 0.26);
    padding: 6px;
    display: flex;
    flex-direction: column;
    justify-content: space-between;
  }

  .metric-micro-item span,
  .metric-micro-item strong {
    position: relative;
    z-index: 1;
  }

  .metric-micro-item span {
    color: #64748b;
    font-size: 0.62rem;
    font-weight: 900;
  }

  .metric-micro-item strong {
    color: #e2e8f0;
    font-size: 0.78rem;
    font-variant-numeric: tabular-nums;
  }

  .metric-micro-item i {
    position: absolute;
    inset: auto 0 0;
    height: 3px;
    background: #818cf8;
    opacity: 0.82;
  }

  .metric-micro-item.tone-safe i { background: #34d399; }
  .metric-micro-item.tone-warn i { background: #fbbf24; }
  .metric-micro-item.tone-danger i { background: #fb7185; }

  .weak-dimension-cell,
  .intervention-cell {
    display: flex;
    flex-direction: column;
    gap: 4px;
    min-width: 0;
  }

  .weak-dimension-cell span,
  .intervention-cell span {
    color: #64748b;
    font-size: 0.68rem;
    line-height: 1.3;
  }

  .weak-dimension-cell strong,
  .intervention-cell strong {
    color: #cbd5e1;
    font-size: 0.72rem;
    line-height: 1.35;
  }

  .weak-dimension-cell.tone-danger strong { color: #fb7185; }
  .weak-dimension-cell.tone-warn strong { color: #fbbf24; }
  .weak-dimension-cell.tone-safe strong { color: #34d399; }

  .diagnostic-action-line {
    display: flex;
    gap: 8px;
    align-items: flex-start;
    border-top: 1px dashed rgba(148, 163, 184, 0.14);
    padding-top: 10px;
    margin-top: 10px;
    color: #94a3b8;
    font-size: 0.72rem;
    line-height: 1.4;
  }

  .diagnostic-action-line strong {
    color: #f8fafc;
    flex: none;
  }

  /* Score Badges */
  .score-badge {
    display: inline-block;
    padding: 4px 10px;
    border-radius: 6px;
    font-size: 0.76rem;
    font-weight: 800;
    text-shadow: 0 2px 4px rgba(0, 0, 0, 0.4);
  }

  .score-badge.score-green {
    background: rgba(16, 185, 129, 0.12);
    color: #34d399;
    border: 1px solid rgba(16, 185, 129, 0.3);
    box-shadow: 0 0 8px rgba(16, 185, 129, 0.15);
  }

  .score-badge.score-yellow {
    background: rgba(245, 158, 11, 0.1);
    color: #fbbf24;
    border: 1px solid rgba(245, 158, 11, 0.25);
    box-shadow: 0 0 8px rgba(251, 191, 36, 0.1);
  }

  .score-badge.score-red {
    background: rgba(244, 63, 94, 0.12);
    color: #f43f5e;
    border: 1px solid rgba(244, 63, 94, 0.3);
    box-shadow: 0 0 8px rgba(244, 63, 94, 0.15);
  }

  /* Hardware Style Progress bars */
  .metric-progress-wrapper {
    display: flex;
    flex-direction: column;
    gap: 5px;
  }

  .metric-val-row {
    display: flex;
    justify-content: space-between;
    font-size: 0.65rem;
    color: #64748b;
    font-weight: 700;
    letter-spacing: 0.02em;
  }

  .progress-bar-bg {
    height: 4px;
    background: rgba(255, 255, 255, 0.03);
    border: 1px solid rgba(255, 255, 255, 0.04);
    border-radius: 2px;
    position: relative;
  }

  .progress-bar-fill {
    display: block;
    height: 100%;
    border-radius: 2px;
    position: relative;
  }

  .progress-bar-fill.is-blue { background: #6366f1; box-shadow: 0 0 8px rgba(99, 102, 241, 0.6); }
  .progress-bar-fill.is-emerald { background: #10b981; box-shadow: 0 0 8px rgba(16, 185, 129, 0.6); }
  .progress-bar-fill.is-amber { background: #f59e0b; box-shadow: 0 0 8px rgba(245, 158, 11, 0.6); }
  .progress-bar-fill.is-rose { background: #f43f5e; box-shadow: 0 0 8px rgba(244, 63, 94, 0.6); }

  .glow-point {
    position: absolute;
    right: -2px;
    top: 50%;
    transform: translateY(-50%);
    width: 4px;
    height: 4px;
    border-radius: 50%;
    background: #f8fafc;
    box-shadow: 0 0 8px currentColor;
  }

  /* Diagnostic Panel */
  .diagnostic-expand-row td {
    padding: 0 16px 14px 16px !important;
    border-bottom: 1px solid rgba(255, 255, 255, 0.04) !important;
    background: rgba(99, 102, 241, 0.02) !important;
  }

  .diagnostic-panel {
    border-radius: 8px;
    padding: 16px 20px;
    animation: slideDown 0.25s cubic-bezier(0.16, 1, 0.3, 1);
    border: 1px solid transparent;
  }

  .diagnostic-panel.score-green {
    background: rgba(16, 185, 129, 0.02);
    border-color: rgba(16, 185, 129, 0.15);
  }

  .diagnostic-panel.score-yellow {
    background: rgba(245, 158, 11, 0.02);
    border-color: rgba(245, 158, 11, 0.15);
  }

  .diagnostic-panel.score-red {
    background: rgba(244, 63, 94, 0.02);
    border-color: rgba(244, 63, 94, 0.15);
  }

  .diagnostic-title {
    font-size: 0.74rem;
    font-weight: 800;
    margin-bottom: 8px;
    display: flex;
    align-items: center;
  }

  .pulse-diagnostic-dot {
    display: inline-block;
    width: 6px;
    height: 6px;
    border-radius: 50%;
    margin-right: 8px;
    animation: statusPulse 1.5s infinite ease-in-out;
  }
  .pulse-diagnostic-dot.score-green { background: #34d399; box-shadow: 0 0 8px #34d399; }
  .pulse-diagnostic-dot.score-yellow { background: #fbbf24; box-shadow: 0 0 8px #fbbf24; }
  .pulse-diagnostic-dot.score-red { background: #f43f5e; box-shadow: 0 0 8px #f43f5e; }

  .diagnostic-content {
    margin: 0 0 12px 0;
    font-size: 0.74rem;
    line-height: 1.5;
    color: #cbd5e1;
  }

  .diagnostic-footer {
    font-size: 0.65rem;
    color: #475569;
    border-top: 1px dashed rgba(255, 255, 255, 0.05);
    padding-top: 8px;
  }

  @media (max-width: 1180px) {
    .health-command-board {
      grid-template-columns: 1fr;
    }

    .priority-card-grid {
      grid-template-columns: 1fr;
    }
  }

  @media (max-width: 820px) {
    .telemetry-panel {
      padding: 16px;
    }

    .filter-controls {
      width: 100%;
      align-items: stretch;
      flex-direction: column;
    }

    .search-input-wrapper {
      width: 100%;
    }

    .health-status-matrix,
    .command-evidence-grid,
    .intervention-brief-grid {
      grid-template-columns: 1fr;
    }

    .command-title-row,
    .intervention-brief-head,
    .stable-state,
    .recommendation-route {
      flex-direction: column;
      align-items: flex-start;
    }

    .dashboard-columns {
      grid-template-columns: 1fr;
    }

    .detail-grid {
      grid-template-columns: 1fr;
    }
  }

  .project-health-workbench {
    display: grid;
    gap: var(--wa-space-4);
    margin-bottom: var(--wa-space-6);
    color: var(--wa-text-main);
    font-family: var(--wa-font-sans);
  }

  .project-health-header {
    min-width: 0;
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: var(--wa-space-4);
    min-height: 58px;
    padding: 10px 12px;
    border: 1px solid var(--wa-border-soft);
    border-radius: var(--wa-radius-xl);
    background: rgba(255, 255, 255, 0.82);
    box-shadow: var(--wa-shadow-sm);
    backdrop-filter: blur(16px) saturate(124%);
    -webkit-backdrop-filter: blur(16px) saturate(124%);
  }

  .project-health-header h2 {
    margin: 3px 0 0;
    color: var(--wa-text-strong);
    font-size: 18px;
    line-height: 1.18;
    font-weight: 840;
  }

  .project-health-header p {
    display: none;
  }

  .project-health-workbench .eyebrow {
    display: block;
    margin: 0;
    color: var(--wa-text-muted);
    font-size: 11px;
    line-height: 1.2;
    font-weight: 780;
    letter-spacing: 0;
    text-transform: none;
  }

  .project-health-actions {
    display: flex;
    align-items: center;
    justify-content: flex-end;
    gap: var(--wa-space-2);
    flex-wrap: wrap;
  }

  .project-health-search {
    position: relative;
    width: min(320px, 42vw);
    min-width: 220px;
  }

  .project-health-search .search-icon {
    border-color: var(--wa-text-subtle);
  }

  .project-health-search .search-icon::after {
    background: var(--wa-text-subtle);
  }

  .telemetry-search-input {
    width: 100%;
    min-height: var(--wa-control-h);
    box-sizing: border-box;
    border: 1px solid var(--wa-border-soft);
    border-radius: var(--wa-radius-md);
    background: rgba(255, 255, 255, 0.84);
    color: var(--wa-text-main);
    padding: 0 12px 0 38px;
    font-size: 13px;
    outline: none;
    transition:
      border-color var(--wa-duration-fast) var(--wa-ease),
      box-shadow var(--wa-duration-fast) var(--wa-ease),
      background var(--wa-duration-fast) var(--wa-ease);
  }

  .telemetry-search-input:focus {
    border-color: var(--wa-border-focus);
    background: #ffffff;
    box-shadow: 0 0 0 3px var(--wa-accent-soft);
  }

  .telemetry-search-input::placeholder {
    color: var(--wa-text-subtle);
  }

  .project-health-state {
    padding: var(--wa-space-5);
    color: var(--wa-text-muted);
    font-size: 13px;
  }

  .project-health-state.is-error {
    border-color: rgba(221, 75, 62, 0.24);
    background: var(--wa-danger-soft);
    color: var(--wa-danger);
  }

  .project-health-metrics {
    display: grid;
    grid-template-columns: repeat(4, minmax(0, 1fr));
    gap: var(--wa-space-3);
  }

  .project-health-metric {
    min-height: 112px;
  }

  .project-health-metric em,
  .project-health-segment em {
    color: var(--wa-text-muted);
    font-size: 12px;
    line-height: 1.35;
    font-style: normal;
  }

  .project-health-segments {
    display: grid;
    grid-template-columns: repeat(4, minmax(0, 1fr));
    gap: var(--wa-space-3);
  }

  .project-health-segment {
    min-width: 0;
    min-height: 72px;
    padding: var(--wa-space-3) var(--wa-space-4);
    display: grid;
    align-content: space-between;
    gap: var(--wa-space-1);
    text-align: left;
    color: var(--wa-text-main);
    cursor: pointer;
    font: inherit;
  }

  .project-health-segment:hover,
  .project-health-segment:focus-visible {
    border-color: var(--wa-border-strong);
    background: #ffffff;
    outline: none;
  }

  .project-health-segment span {
    color: var(--wa-text-muted);
    font-size: 12px;
    font-weight: 740;
  }

  .project-health-segment strong {
    color: var(--wa-text-strong);
    font-size: 22px;
    line-height: 1;
  }

  .project-health-main-grid {
    min-width: 0;
    display: grid;
    grid-template-columns: minmax(0, 1fr) minmax(320px, 380px);
    gap: var(--wa-space-4);
    align-items: stretch;
  }

  .project-health-table-card {
    min-width: 0;
    padding: var(--wa-space-4);
    display: grid;
    gap: var(--wa-space-3);
  }

  .project-health-table-head {
    display: flex;
    align-items: flex-start;
    justify-content: space-between;
    gap: var(--wa-space-3);
  }

  .project-health-table-head h3,
  .project-health-inspector h3 {
    margin: 3px 0 0;
    color: var(--wa-text-strong);
    font-size: 16px;
    line-height: 1.25;
    font-weight: 820;
  }

  .project-health-count {
    color: var(--wa-text-muted);
    font-size: 12px;
  }

  .project-health-table {
    min-width: 1020px;
  }

  .project-health-table .align-center,
  .project-health-table th.align-center {
    text-align: center;
  }

  .project-info-cell {
    display: flex;
    align-items: center;
    gap: var(--wa-space-3);
    min-width: 0;
  }

  .project-avatar {
    width: 32px;
    height: 32px;
    border-radius: var(--wa-radius-md);
    display: inline-flex;
    align-items: center;
    justify-content: center;
    flex: none;
    border: 1px solid var(--wa-border-soft);
    background: var(--wa-surface-inset);
    color: var(--wa-text-main);
    font-weight: 820;
    font-size: 12px;
  }

  .project-avatar.score-red {
    border-color: rgba(221, 75, 62, 0.24);
    background: var(--wa-danger-soft);
    color: var(--wa-danger);
  }

  .project-avatar.score-yellow {
    border-color: rgba(216, 135, 0, 0.24);
    background: var(--wa-warning-soft);
    color: var(--wa-warning);
  }

  .project-avatar.score-green {
    border-color: rgba(4, 150, 111, 0.22);
    background: var(--wa-success-soft);
    color: var(--wa-success);
  }

  .project-meta {
    min-width: 0;
    display: grid;
    gap: 2px;
  }

  .project-meta strong {
    color: var(--wa-text-strong);
    font-size: 13px;
    line-height: 1.25;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .text-muted {
    color: var(--wa-text-muted);
    font-size: 12px;
  }

  .score-badge {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    min-width: 54px;
    min-height: 24px;
    padding: 0 8px;
    border: 1px solid var(--wa-border-soft);
    border-radius: var(--wa-radius-sm);
    background: var(--wa-neutral-soft);
    color: var(--wa-text-main);
    font-size: 12px;
    line-height: 1;
    font-weight: 800;
    text-shadow: none;
    box-shadow: none;
  }

  .metric-micro-grid {
    display: grid;
    grid-template-columns: repeat(4, minmax(0, 1fr));
    gap: 6px;
  }

  .metric-micro-item {
    position: relative;
    overflow: hidden;
    min-height: 38px;
    border: 1px solid var(--wa-border-soft);
    border-radius: var(--wa-radius-sm);
    background: var(--wa-surface-inset);
    padding: 6px;
    display: grid;
    align-content: space-between;
  }

  .metric-micro-item span {
    color: var(--wa-text-muted);
    font-size: 11px;
    font-weight: 800;
  }

  .metric-micro-item strong {
    color: var(--wa-text-strong);
    font-size: 13px;
    font-variant-numeric: tabular-nums;
  }

  .metric-micro-item i {
    position: absolute;
    inset: auto 0 0;
    height: 3px;
    background: currentColor;
    opacity: 0.75;
  }

  .weak-dimension-cell,
  .intervention-cell {
    min-width: 0;
    display: grid;
    gap: 4px;
  }

  .weak-dimension-cell span,
  .intervention-cell span {
    color: var(--wa-text-muted);
    font-size: 12px;
    line-height: 1.35;
  }

  .weak-dimension-cell strong,
  .intervention-cell strong {
    color: var(--wa-text-main);
    font-size: 12px;
    line-height: 1.35;
  }

  .diagnostic-expand-row td {
    padding: 0 14px 14px !important;
    background: var(--wa-row-active) !important;
  }

  .diagnostic-panel {
    border: 1px solid var(--wa-border-soft);
    border-radius: var(--wa-radius-md);
    background: rgba(255, 255, 255, 0.72);
    padding: var(--wa-space-3);
  }

  .diagnostic-title {
    display: flex;
    align-items: center;
    color: var(--wa-text-strong);
    font-size: 12px;
    font-weight: 800;
    margin-bottom: 8px;
  }

  .diagnostic-content {
    margin: 0 0 10px;
    color: var(--wa-text-main);
    font-size: 13px;
    line-height: 1.5;
  }

  .diagnostic-action-line,
  .diagnostic-footer {
    color: var(--wa-text-muted);
    font-size: 12px;
  }

  .diagnostic-action-line strong {
    color: var(--wa-text-strong);
  }

  .project-health-inspector {
    position: static;
    top: auto;
    height: 100%;
    max-height: none;
    overflow: visible;
  }

  .inspector-title-row {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: var(--wa-space-2);
  }

  .project-health-inspector > p {
    margin: 0;
    color: var(--wa-text-muted);
    font-size: 13px;
    line-height: 1.5;
  }

  .project-health-facts {
    display: grid;
    grid-template-columns: repeat(2, minmax(0, 1fr));
    gap: var(--wa-space-2);
  }

  .project-health-facts div {
    min-width: 0;
    border: 1px solid var(--wa-border-soft);
    border-radius: var(--wa-radius-md);
    background: var(--wa-surface-inset);
    padding: var(--wa-space-2);
  }

  .project-health-facts span,
  .project-health-progress span,
  .project-health-inspector-section h4 {
    display: block;
    color: var(--wa-text-muted);
    font-size: 12px;
    line-height: 1.3;
    font-weight: 760;
  }

  .project-health-facts strong {
    display: block;
    margin-top: 4px;
    color: var(--wa-text-strong);
    font-size: 13px;
    line-height: 1.35;
    overflow-wrap: anywhere;
  }

  .project-health-progress {
    display: grid;
    gap: var(--wa-space-2);
  }

  .project-health-progress > div:first-child {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: var(--wa-space-2);
  }

  .project-health-progress strong {
    color: var(--wa-text-strong);
  }

  .project-health-inspector-section {
    display: grid;
    gap: var(--wa-space-2);
    padding-top: var(--wa-space-3);
    border-top: 1px solid var(--wa-border-soft);
  }

  .project-health-inspector-section h4 {
    margin: 0;
    color: var(--wa-text-strong);
    font-size: 13px;
  }

  .project-health-inspector-section p,
  .project-health-inspector-section li {
    color: var(--wa-text-main);
    font-size: 13px;
    line-height: 1.5;
  }

  .project-health-inspector-section p {
    margin: 0;
  }

  .project-health-inspector-section ul {
    margin: 0;
    padding-left: 18px;
  }

  .project-health-inspector-section li + li {
    margin-top: 6px;
  }

  .project-health-inspector-actions {
    display: flex;
    gap: var(--wa-space-2);
    flex-wrap: wrap;
  }

  .project-health-inspector-empty {
    display: grid;
    gap: var(--wa-space-2);
    color: var(--wa-text-muted);
  }

  .project-health-inspector-empty strong {
    color: var(--wa-text-strong);
  }

  .modal-backdrop {
    background: rgba(13, 23, 34, 0.28);
    backdrop-filter: blur(12px);
    -webkit-backdrop-filter: blur(12px);
  }

  .modal-content {
    background: rgba(255, 255, 255, 0.96);
    border: 1px solid var(--wa-border-soft);
    border-radius: var(--wa-radius-xl);
    box-shadow: var(--wa-shadow-md);
    color: var(--wa-text-main);
  }

  .modal-header {
    border-bottom-color: var(--wa-border-soft);
  }

  .modal-header h3,
  .intervention-brief-head h4,
  .detail-item .value,
  .schematic-node .node-value,
  .recommendation-item b,
  .recommendation-route strong {
    color: var(--wa-text-strong);
  }

  .close-btn {
    color: var(--wa-text-muted);
  }

  .close-btn:hover {
    color: var(--wa-text-strong);
    transform: none;
  }

  .intervention-brief-card,
  .detail-grid,
  .phdi-display,
  .formula-schematic,
  .metric-row-card,
  .action-recommendations,
  .recommendation-item {
    background: var(--wa-surface-inset);
    border-color: var(--wa-border-soft);
    box-shadow: none;
  }

  .intervention-brief-head span,
  .intervention-brief-grid span,
  .detail-section h4,
  .detail-item .label,
  .formula-breakdown h5,
  .schematic-node .node-label,
  .metric-row-card .row-title,
  .recommendation-item p,
  .recommendation-route,
  .diagnostic-panel-modal,
  .diag-header-row {
    color: var(--wa-text-muted);
  }

  .intervention-brief-grid div,
  .schematic-node {
    background: rgba(255, 255, 255, 0.72);
    border-color: var(--wa-border-soft);
  }

  .detail-section h4 {
    border-bottom-color: var(--wa-border-soft);
    text-transform: none;
    letter-spacing: 0;
  }

  .formula-schematic {
    color: var(--wa-text-main);
  }

  .modal-body {
    color: var(--wa-text-main);
  }

  .modal-content::-webkit-scrollbar-track {
    background: rgba(121, 139, 159, 0.12);
  }

  .modal-content::-webkit-scrollbar-thumb {
    background: rgba(121, 139, 159, 0.34);
  }

  /* Health diagnosis modal: calm light workbench contract */
  .health-diagnosis-modal {
    width: min(920px, calc(100vw - 40px));
    max-width: none;
    max-height: min(860px, calc(100dvh - 40px));
    padding: 0;
    overflow: hidden;
    border: 1px solid rgba(123, 143, 160, 0.22);
    border-radius: 18px;
    background: #fbfdfe;
    box-shadow: 0 26px 70px rgba(30, 46, 64, 0.16), 0 2px 8px rgba(30, 46, 64, 0.05);
    font-family: var(--wa-font-sans);
  }

  .health-diagnosis-modal:hover {
    border-color: rgba(123, 143, 160, 0.22);
  }

  .health-diagnosis-modal .modal-header {
    flex: none;
    min-height: 68px;
    box-sizing: border-box;
    margin: 0;
    padding: 14px 18px 13px 20px;
    border-bottom: 1px solid var(--wa-border-soft);
    background: rgba(255, 255, 255, 0.9);
  }

  .health-diagnosis-modal .modal-title-block {
    min-width: 0;
    display: grid;
    gap: 3px;
  }

  .health-diagnosis-modal .modal-title-block > span {
    color: var(--wa-text-subtle);
    font-size: 10px;
    line-height: 1.2;
    font-weight: 760;
    letter-spacing: 0.04em;
  }

  .health-diagnosis-modal .modal-header h3 {
    overflow: hidden;
    margin: 0;
    color: var(--wa-text-strong);
    font-size: 15px;
    line-height: 1.3;
    font-weight: 780;
    letter-spacing: -0.01em;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .health-diagnosis-modal .close-btn {
    width: 36px;
    height: 36px;
    display: inline-grid;
    place-items: center;
    flex: none;
    border: 1px solid transparent;
    border-radius: 10px;
    color: var(--wa-text-muted);
    font-size: 21px;
    line-height: 1;
  }

  .health-diagnosis-modal .close-btn:hover {
    border-color: var(--wa-border-soft);
    background: var(--wa-surface-inset);
    color: var(--wa-text-strong);
  }

  .health-diagnosis-modal .close-btn:focus-visible {
    outline: 3px solid var(--wa-accent-soft);
    outline-offset: 1px;
  }

  .health-diagnosis-modal .modal-body {
    min-height: 0;
    padding: 16px 18px 20px 20px;
    gap: 18px;
    overflow-y: auto;
    scrollbar-color: rgba(102, 119, 137, 0.34) transparent;
  }

  .health-diagnosis-modal .modal-body::-webkit-scrollbar {
    width: 8px;
  }

  .health-diagnosis-modal .modal-body::-webkit-scrollbar-track {
    background: transparent;
  }

  .health-diagnosis-modal .modal-body::-webkit-scrollbar-thumb {
    border: 2px solid #fbfdfe;
    border-radius: 999px;
    background: rgba(102, 119, 137, 0.32);
  }

  .health-diagnosis-modal .intervention-brief-card {
    position: relative;
    gap: 12px;
    padding: 14px 16px 13px;
    border: 1px solid var(--wa-border-soft);
    border-radius: 12px;
    background: #f5f8fa;
  }

  .health-diagnosis-modal .intervention-brief-card.score-red {
    border-color: rgba(221, 75, 62, 0.18);
    background: color-mix(in srgb, var(--wa-danger-soft) 34%, #f7f9fb);
  }

  .health-diagnosis-modal .intervention-brief-card.score-yellow {
    border-color: rgba(216, 135, 0, 0.18);
    background: color-mix(in srgb, var(--wa-warning-soft) 30%, #f7f9fb);
  }

  .health-diagnosis-modal .intervention-brief-card.score-green {
    border-color: rgba(4, 150, 111, 0.16);
    background: color-mix(in srgb, var(--wa-success-soft) 28%, #f7f9fb);
  }

  .health-diagnosis-modal .intervention-brief-head {
    align-items: center;
  }

  .health-diagnosis-modal .intervention-brief-head > div:first-child {
    min-width: 0;
  }

  .health-diagnosis-modal .intervention-brief-head > div:first-child > span {
    color: var(--wa-text-muted);
    font-size: 10px;
    font-weight: 760;
    letter-spacing: 0.03em;
  }

  .health-diagnosis-modal .intervention-brief-card.score-red .intervention-brief-head > div:first-child > span {
    color: #a8453c;
  }

  .health-diagnosis-modal .intervention-brief-head h4 {
    margin-top: 4px;
    color: var(--wa-text-strong);
    font-size: 14px;
    line-height: 1.45;
    font-weight: 720;
    text-wrap: pretty;
  }

  .health-diagnosis-modal .intervention-brief-score {
    display: grid;
    justify-items: end;
    gap: 2px;
    flex: none;
  }

  .health-diagnosis-modal .intervention-brief-score span {
    color: var(--wa-text-subtle);
    font-size: 9px;
    font-weight: 720;
    letter-spacing: 0;
  }

  .health-diagnosis-modal .intervention-brief-score strong {
    color: var(--wa-text-main);
    font-size: 24px;
    line-height: 1;
    font-variant-numeric: tabular-nums;
  }

  .health-diagnosis-modal .intervention-brief-grid {
    gap: 0;
    overflow: hidden;
    border: 1px solid var(--wa-border-soft);
    border-radius: 10px;
    background: rgba(255, 255, 255, 0.76);
  }

  .health-diagnosis-modal .intervention-brief-grid div {
    min-height: 58px;
    padding: 9px 11px;
    border: 0;
    border-left: 1px solid var(--wa-border-soft);
    border-radius: 0;
    background: transparent;
  }

  .health-diagnosis-modal .intervention-brief-grid div:first-child {
    border-left: 0;
  }

  .health-diagnosis-modal .intervention-brief-grid span {
    margin-bottom: 4px;
    color: var(--wa-text-subtle);
    font-size: 10px;
    font-weight: 720;
  }

  .health-diagnosis-modal .intervention-brief-grid strong {
    display: block;
    color: var(--wa-text-main);
    font-size: 11px;
    line-height: 1.45;
    font-weight: 650;
  }

  .health-diagnosis-modal .dashboard-columns {
    grid-template-columns: minmax(0, 1.07fr) minmax(0, 0.93fr);
    gap: 18px;
  }

  .health-diagnosis-modal .column-left,
  .health-diagnosis-modal .column-right {
    gap: 18px;
  }

  .health-diagnosis-modal .detail-section {
    min-width: 0;
  }

  .health-diagnosis-modal .detail-section h4 {
    margin: 0 0 9px;
    padding: 0 0 7px;
    border-bottom: 1px solid var(--wa-border-soft);
    color: var(--wa-text-muted);
    font-size: 11px;
    line-height: 1.3;
    font-weight: 760;
    letter-spacing: 0.01em;
  }

  .health-diagnosis-modal .diagnostic-panel-modal {
    padding: 12px 13px;
    border: 1px solid var(--wa-border-soft);
    border-radius: 10px;
    background: #f8fafc;
    color: var(--wa-text-main);
    font-family: var(--wa-font-sans);
    font-size: 11px;
    line-height: 1.65;
  }

  .health-diagnosis-modal .diagnostic-copy {
    margin: 0;
    white-space: pre-wrap;
    line-height: 1.65;
  }

  .health-diagnosis-modal .diagnostic-panel-modal.score-red {
    border-color: rgba(221, 75, 62, 0.16);
    background: color-mix(in srgb, var(--wa-danger-soft) 24%, #fafcfd);
    color: var(--wa-text-main);
  }

  .health-diagnosis-modal .diagnostic-panel-modal.score-yellow {
    border-color: rgba(216, 135, 0, 0.16);
    background: color-mix(in srgb, var(--wa-warning-soft) 22%, #fafcfd);
    color: var(--wa-text-main);
  }

  .health-diagnosis-modal .diagnostic-panel-modal.score-green {
    border-color: rgba(4, 150, 111, 0.14);
    border-left-color: rgba(4, 126, 94, 0.62);
    background: color-mix(in srgb, var(--wa-success-soft) 20%, #fafcfd);
    color: var(--wa-text-main);
  }

  .health-diagnosis-modal .diag-header-row {
    margin-bottom: 8px;
    padding-bottom: 7px;
    border-bottom: 1px solid var(--wa-border-soft);
    color: var(--wa-text-muted);
    font-family: var(--wa-font-mono);
    font-size: 9px;
    letter-spacing: 0.06em;
  }

  .health-diagnosis-modal .pulse-diagnostic-dot {
    width: 5px;
    height: 5px;
    animation: none;
    box-shadow: none;
  }

  .health-diagnosis-modal .phdi-display {
    display: grid;
    gap: 12px;
    padding: 14px;
    border: 1px solid var(--wa-border-soft);
    border-radius: 11px;
    background: var(--wa-surface-inset);
  }

  .health-diagnosis-modal .phdi-score-row {
    width: 100%;
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 12px;
    padding-bottom: 10px;
    border-bottom: 1px solid var(--wa-border-soft);
  }

  .health-diagnosis-modal .big-score {
    color: var(--wa-text-main);
    font-size: 32px;
    line-height: 1;
    font-weight: 780;
    text-shadow: none;
    font-variant-numeric: tabular-nums;
  }

  .health-diagnosis-modal .big-score.score-red {
    color: #b9473c;
    text-shadow: none;
  }

  .health-diagnosis-modal .big-score.score-yellow {
    color: #a56b12;
    text-shadow: none;
  }

  .health-diagnosis-modal .big-score.score-green {
    color: #087b5d;
    text-shadow: none;
  }

  .health-diagnosis-modal .big-score .unit {
    color: var(--wa-text-muted);
    font-size: 11px;
  }

  .health-diagnosis-modal .health-badge {
    border-radius: 6px;
    font-size: 10px;
    font-weight: 720;
    box-shadow: none;
  }

  .health-diagnosis-modal .health-badge.score-red {
    border-color: rgba(221, 75, 62, 0.18);
    background: var(--wa-danger-soft);
    color: #a8453c;
  }

  .health-diagnosis-modal .health-badge.score-yellow {
    border-color: rgba(216, 135, 0, 0.18);
    background: var(--wa-warning-soft);
    color: #99620d;
  }

  .health-diagnosis-modal .health-badge.score-green {
    border-color: rgba(4, 150, 111, 0.16);
    background: var(--wa-success-soft);
    color: #087558;
  }

  .health-diagnosis-modal .formula-breakdown {
    width: 100%;
    min-width: 0;
  }

  .health-diagnosis-modal .formula-breakdown h5 {
    margin: 0 0 8px;
    color: var(--wa-text-muted);
    font-size: 10px;
    font-weight: 720;
  }

  .health-diagnosis-modal .formula-schematic {
    gap: 8px;
    padding: 9px;
    border: 1px solid var(--wa-border-soft);
    border-radius: 9px;
    background: rgba(255, 255, 255, 0.72);
    color: var(--wa-text-main);
    font-size: 10px;
  }

  .health-diagnosis-modal .schematic-node {
    min-height: 44px;
    justify-content: center;
    padding: 5px 7px;
    border: 1px solid var(--wa-border-soft);
    border-radius: 7px;
    background: #fff;
  }

  .health-diagnosis-modal .schematic-node .node-label {
    margin-bottom: 3px;
    color: var(--wa-text-subtle);
    font-size: 9px;
    text-transform: none;
  }

  .health-diagnosis-modal .schematic-node .node-value {
    color: var(--wa-text-main);
    font-size: 10px;
  }

  .health-diagnosis-modal .schematic-node.result-node {
    border-style: solid;
    box-shadow: none;
  }

  .health-diagnosis-modal .schematic-node.result-node.score-red {
    border-color: rgba(221, 75, 62, 0.18);
    background: var(--wa-danger-soft);
  }

  .health-diagnosis-modal .schematic-node.result-node.score-red .node-value {
    color: #a8453c;
  }

  .health-diagnosis-modal .schematic-node.result-node.score-yellow {
    border-color: rgba(216, 135, 0, 0.18);
    background: var(--wa-warning-soft);
  }

  .health-diagnosis-modal .schematic-node.result-node.score-yellow .node-value {
    color: #99620d;
  }

  .health-diagnosis-modal .schematic-node.result-node.score-green {
    border-color: rgba(4, 150, 111, 0.16);
    background: var(--wa-success-soft);
  }

  .health-diagnosis-modal .schematic-node.result-node.score-green .node-value {
    color: #087558;
  }

  .health-diagnosis-modal .schematic-connector {
    color: var(--wa-text-subtle);
    font-size: 12px;
  }

  .health-diagnosis-modal .detail-grid {
    grid-template-columns: repeat(2, minmax(0, 1fr));
    gap: 7px;
    padding: 0;
    border: 0;
    background: transparent;
  }

  .health-diagnosis-modal .detail-item {
    min-width: 0;
    min-height: 36px;
    align-items: center;
    gap: 8px;
    padding: 0 9px;
    border: 1px solid var(--wa-border-soft);
    border-radius: 8px;
    background: var(--wa-surface-inset);
    font-size: 10px;
  }

  .health-diagnosis-modal .detail-item .label {
    color: var(--wa-text-muted);
  }

  .health-diagnosis-modal .detail-item .value {
    overflow: hidden;
    color: var(--wa-text-strong);
    font-weight: 720;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .health-diagnosis-modal .detail-item .value.highlight {
    color: var(--wa-accent-strong);
    text-shadow: none;
  }

  .health-diagnosis-modal .detail-item .value.badge {
    padding: 2px 6px;
    border: 1px solid rgba(0, 143, 150, 0.14);
    border-radius: 5px;
    background: var(--wa-accent-soft);
    color: var(--wa-accent-strong);
  }

  .health-diagnosis-modal .metrics-grid-vertical {
    gap: 7px;
  }

  .health-diagnosis-modal .metric-row-card {
    gap: 7px;
    padding: 9px 11px;
    border: 1px solid var(--wa-border-soft);
    border-radius: 9px;
    background: var(--wa-surface-inset);
  }

  .health-diagnosis-modal .metric-row-card .row-header,
  .health-diagnosis-modal .row-status {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 8px;
  }

  .health-diagnosis-modal .metric-row-card .row-title {
    color: var(--wa-text-muted);
    font-size: 10px;
    font-weight: 680;
  }

  .health-diagnosis-modal .metric-row-card .row-score {
    color: var(--wa-text-main);
    font-size: 11px;
    font-weight: 760;
    font-variant-numeric: tabular-nums;
  }

  .health-diagnosis-modal .row-status-tag {
    padding: 2px 5px;
    border-radius: 5px;
    font-size: 9px;
    font-weight: 700;
  }

  .health-diagnosis-modal .row-status-tag.text-emerald {
    border-color: rgba(4, 150, 111, 0.14);
    background: var(--wa-success-soft);
    color: #087558;
  }

  .health-diagnosis-modal .row-status-tag.text-amber {
    border-color: rgba(216, 135, 0, 0.16);
    background: var(--wa-warning-soft);
    color: #99620d;
  }

  .health-diagnosis-modal .row-status-tag.text-rose {
    border-color: rgba(221, 75, 62, 0.16);
    background: var(--wa-danger-soft);
    color: #a8453c;
  }

  .health-diagnosis-modal .metric-row-card .row-progress {
    height: 4px;
    border-radius: 999px;
    background: rgba(102, 119, 137, 0.12);
  }

  .health-diagnosis-modal .metric-row-card .row-progress span,
  .health-diagnosis-modal .metric-row-card .row-progress span.bg-blue,
  .health-diagnosis-modal .metric-row-card .row-progress span.bg-emerald,
  .health-diagnosis-modal .metric-row-card .row-progress span.bg-amber,
  .health-diagnosis-modal .metric-row-card .row-progress span.bg-rose {
    border-radius: 999px;
    background: rgba(0, 143, 150, 0.64);
    box-shadow: none;
  }

  .health-diagnosis-modal .metric-row-card.metric-tone-warn .row-progress span {
    background: rgba(184, 117, 14, 0.66);
  }

  .health-diagnosis-modal .metric-row-card.metric-tone-danger .row-progress span {
    background: rgba(185, 71, 60, 0.66);
  }

  .health-diagnosis-modal .action-recommendations {
    gap: 7px;
    padding: 0;
    border: 0;
    background: transparent;
  }

  .health-diagnosis-modal .recommendation-item {
    gap: 9px;
    padding: 9px 10px 9px 8px;
    border: 1px solid var(--wa-border-soft);
    border-radius: 9px;
    background: var(--wa-surface-inset);
    font-size: 10px;
    line-height: 1.55;
  }

  .health-diagnosis-modal .recommendation-item.tone-danger {
    border-color: rgba(221, 75, 62, 0.14);
    background: var(--wa-surface-inset);
  }

  .health-diagnosis-modal .recommendation-item.tone-warn {
    border-color: rgba(216, 135, 0, 0.14);
    background: var(--wa-surface-inset);
  }

  .health-diagnosis-modal .recommendation-item.tone-safe {
    border-color: rgba(4, 150, 111, 0.12);
    background: var(--wa-surface-inset);
  }

  .health-diagnosis-modal .recommendation-item p {
    color: var(--wa-text-main);
  }

  .health-diagnosis-modal .recommendation-item b {
    color: var(--wa-text-strong);
    font-weight: 720;
  }

  .health-diagnosis-modal .rec-bullet {
    min-width: 28px;
    height: 22px;
    border-radius: 6px;
    background: var(--wa-neutral-soft);
    color: var(--wa-text-muted);
    font-size: 9px;
  }

  .health-diagnosis-modal .recommendation-route {
    align-items: center;
    padding: 9px 10px;
    border: 1px solid var(--wa-border-soft);
    border-radius: 9px;
    background: rgba(0, 143, 150, 0.055);
    color: var(--wa-text-muted);
    font-size: 10px;
  }

  .health-diagnosis-modal .recommendation-route strong {
    max-width: 72%;
    color: var(--wa-accent-strong);
    font-weight: 700;
  }

  @media (max-width: 860px) {
    .health-diagnosis-modal {
      width: min(720px, calc(100vw - 24px));
      max-height: calc(100dvh - 24px);
    }

    .health-diagnosis-modal .dashboard-columns {
      grid-template-columns: 1fr;
    }
  }

  @media (max-width: 620px) {
    .health-diagnosis-modal .modal-header {
      min-height: 62px;
      padding: 12px 12px 11px 14px;
    }

    .health-diagnosis-modal .modal-body {
      padding: 12px 10px 16px 12px;
    }

    .health-diagnosis-modal .intervention-brief-head,
    .health-diagnosis-modal .recommendation-route {
      align-items: flex-start;
    }

    .health-diagnosis-modal .intervention-brief-score {
      justify-items: start;
    }

    .health-diagnosis-modal .intervention-brief-grid,
    .health-diagnosis-modal .detail-grid {
      grid-template-columns: 1fr;
    }

    .health-diagnosis-modal .intervention-brief-grid div {
      min-height: 0;
      border-top: 1px solid var(--wa-border-soft);
      border-left: 0;
    }

    .health-diagnosis-modal .intervention-brief-grid div:first-child {
      border-top: 0;
    }

    .health-diagnosis-modal .formula-schematic {
      align-items: stretch;
      flex-direction: column;
    }

    .health-diagnosis-modal .schematic-connector {
      align-self: center;
    }

    .health-diagnosis-modal .recommendation-route strong {
      max-width: none;
      text-align: left;
    }
  }

  @media (max-width: 1180px) {
    .project-health-main-grid {
      grid-template-columns: 1fr;
    }

    .project-health-inspector {
      position: static;
    }
  }

  @media (max-width: 860px) {
    .project-health-header {
      flex-direction: column;
    }

    .project-health-actions,
    .project-health-search {
      width: 100%;
    }

    .project-health-metrics,
    .project-health-segments,
    .project-health-facts {
      grid-template-columns: 1fr;
    }
  }

  /* Viewport-bounded health workbench: data rows scroll inside the table shell. */
  @media (min-width: 1181px) {
    .project-health-workbench {
      height: 100%;
      min-height: 0;
      grid-template-rows: auto auto auto;
      overflow-y: auto;
      overscroll-behavior: contain;
      margin-bottom: 0;
    }

    .project-health-main-grid,
    .project-health-table-stack,
    .project-health-table-card {
      height: 100%;
      min-height: 0;
    }

    .project-health-table-stack,
    .project-health-table-card {
      overflow: hidden;
    }

    .project-health-main-grid {
      height: auto;
      min-height: 450px;
      overflow: visible;
    }

    .project-health-table-card {
      grid-template-rows: auto minmax(0, 1fr);
    }

    .project-health-table-card > .wa-admin-table-shell {
      min-height: 0;
      height: 100%;
      overflow: auto;
      overscroll-behavior: contain;
    }

    .project-health-inspector {
      height: auto;
      min-height: 0;
      max-height: none;
      overflow-y: auto;
      overscroll-behavior: contain;
      scrollbar-width: none;
    }

    .project-health-inspector::-webkit-scrollbar {
      width: 0;
      height: 0;
    }
  }

  @media (min-width: 861px) and (max-width: 1180px) {
    .project-health-workbench {
      height: 100%;
      min-height: 0;
      overflow-y: auto;
      overscroll-behavior: contain;
      margin-bottom: 0;
    }

    .project-health-table-card {
      height: clamp(420px, 58dvh, 560px);
      min-height: 0;
      grid-template-rows: auto minmax(0, 1fr);
      overflow: hidden;
    }

    .project-health-table-card > .wa-admin-table-shell {
      min-height: 0;
      height: 100%;
      overflow: auto;
      overscroll-behavior: contain;
    }
  }

  @media (max-width: 860px) {
    .project-health-table-card {
      height: min(560px, 62dvh);
      min-height: 0;
      grid-template-rows: auto minmax(0, 1fr);
      overflow: hidden;
    }

    .project-health-table-card > .wa-admin-table-shell {
      min-height: 0;
      height: 100%;
      overflow: auto;
      overscroll-behavior: contain;
    }
  }

  /* Keyframe Animations */
  @keyframes slideDown {
    from { opacity: 0; transform: translateY(-6px); }
    to { opacity: 1; transform: translateY(0); }
  }

  @keyframes fadeIn {
    from { opacity: 0; }
    to { opacity: 1; }
  }

  @keyframes statusPulse {
    0%, 100% { opacity: 0.6; transform: scale(1); }
    50% { opacity: 1; transform: scale(1.2); }
  }

  /* Project health keeps the data table as the only desktop scroll owner. */
  .project-health-header,
  .health-decision-strip,
  .project-health-table-card,
  .project-health-inspector {
    border-color: var(--wa-glass-outline, rgba(72, 98, 118, 0.18));
    border-top-color: var(--wa-glass-highlight, rgba(255, 255, 255, 0.82));
    border-radius: var(--wa-radius-lg, 14px);
    background: var(--wa-glass-panel, rgba(250, 253, 255, 0.76));
    box-shadow: var(--wa-shadow-glass, inset 0 1px 0 rgba(255, 255, 255, 0.86), 0 6px 14px rgba(30, 52, 68, 0.085));
    -webkit-backdrop-filter: blur(16px) saturate(128%);
    backdrop-filter: blur(16px) saturate(128%);
  }

  .telemetry-search-input,
  .project-health-actions .wa-admin-action {
    border-radius: var(--wa-radius-pill, 999px);
  }

  .project-health-table-card > .wa-admin-table-shell {
    border: 0;
    border-top: 1px solid var(--wa-border-divider, rgba(123, 143, 160, 0.18));
    border-radius: 0;
    background: transparent;
    box-shadow: none;
  }

  .health-decision-strip {
    min-width: 0;
    display: grid;
    grid-template-columns: minmax(260px, 1fr) auto;
    align-items: center;
    gap: var(--wa-space-4);
    padding: 10px 12px;
  }

  .health-decision-copy {
    min-width: 0;
    display: grid;
    gap: 3px;
  }

  .health-decision-copy strong {
    color: var(--wa-text-strong);
    font-size: 14px;
    line-height: 1.35;
  }

  .health-decision-copy small {
    color: var(--wa-text-muted);
    font-size: 11px;
    line-height: 1.35;
  }

  .health-filter-group {
    display: flex;
    align-items: stretch;
    overflow: hidden;
    border: 1px solid var(--wa-border-divider);
    border-radius: var(--wa-radius-md);
    background: rgba(255, 255, 255, 0.58);
  }

  .health-filter-group button {
    min-width: 88px;
    min-height: 44px;
    display: grid;
    grid-template-columns: 1fr auto;
    align-items: center;
    gap: 8px;
    border: 0;
    border-left: 1px solid var(--wa-border-divider);
    padding: 7px 10px;
    background: transparent;
    color: var(--wa-text-muted);
    font: inherit;
    font-size: 11px;
    cursor: pointer;
  }

  .health-filter-group button:first-child {
    border-left: 0;
  }

  .health-filter-group button strong {
    color: var(--wa-text-strong);
    font-size: 16px;
    font-variant-numeric: tabular-nums;
  }

  .health-filter-group button:hover,
  .health-filter-group button:focus-visible {
    background: var(--wa-surface-inset);
    color: var(--wa-text-main);
    outline: none;
  }

  .health-filter-group button.active {
    background: var(--wa-row-active);
    box-shadow: inset 0 -2px 0 var(--wa-accent);
    color: var(--wa-text-strong);
  }

  .health-filter-group button.tone-danger strong {
    color: var(--wa-danger);
  }

  .health-filter-group button.tone-warning strong {
    color: var(--wa-warning);
  }

  .health-filter-group button.tone-success strong {
    color: var(--wa-success);
  }

  .project-health-empty-row {
    height: 160px;
    color: var(--wa-text-muted);
    text-align: center;
  }

  .project-health-table .metric-micro-grid {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }

  .project-health-table .metric-micro-item {
    min-height: 34px;
    grid-template-columns: minmax(0, 1fr) auto;
    align-items: center;
    gap: 6px;
  }

  .project-health-table .metric-micro-item span {
    overflow: hidden;
    font-weight: 720;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .project-health-facts {
    gap: 0 var(--wa-space-3);
  }

  .project-health-facts div {
    border: 0;
    border-bottom: 1px solid var(--wa-border-divider, rgba(123, 143, 160, 0.18));
    border-radius: 0;
    background: transparent;
    padding: 8px 0;
  }

  @media (min-width: 1181px) {
    .project-health-workbench {
      height: 100%;
      min-height: 0;
      grid-template-rows: auto minmax(0, 1fr);
      overflow: hidden;
      margin-bottom: 0;
    }

    .project-health-main-grid,
    .project-health-table-stack,
    .project-health-table-card {
      height: 100%;
      min-height: 0;
      overflow: hidden;
    }

    .project-health-table-card > .wa-admin-table-shell {
      height: 100%;
      min-height: 0;
      overflow: auto;
    }

    .project-health-inspector {
      height: 100%;
      min-height: 0;
      max-height: none;
      gap: 12px;
      overflow: hidden;
    }

    .project-health-inspector-section p {
      display: -webkit-box;
      overflow: hidden;
      line-clamp: 3;
      -webkit-box-orient: vertical;
      -webkit-line-clamp: 3;
    }

    .project-health-inspector-section li:nth-child(n + 3) {
      display: none;
    }
  }

  @media (max-width: 860px) {
    .health-decision-strip {
      grid-template-columns: 1fr;
    }

    .health-filter-group {
      width: 100%;
      overflow-x: auto;
    }

    .health-filter-group button {
      flex: 1 0 92px;
    }
  }

  @media (prefers-reduced-motion: reduce) {
    .status-pulse,
    .health-diagnosis-modal,
    .project-health-state {
      animation: none !important;
    }
  }
</style>
