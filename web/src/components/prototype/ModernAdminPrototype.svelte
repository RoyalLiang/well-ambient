<script lang="ts">
  import PrototypeBadge from './PrototypeBadge.svelte';
  import PrototypeInspector from './PrototypeInspector.svelte';
  import PrototypeMetric from './PrototypeMetric.svelte';
  import PrototypeShell from './PrototypeShell.svelte';
  import PrototypeTable from './PrototypeTable.svelte';
  import PrototypeTheatre from './PrototypeTheatre.svelte';

  type LaneId = 'command' | 'schedule' | 'evidence' | 'meeting';
  type RiskTone = 'critical' | 'warning' | 'safe' | 'info';
  type EvidenceState = 'done' | 'active' | 'missing';

  interface Lane {
    id: LaneId;
    label: string;
    description: string;
    count: number;
    mark: string;
  }

  interface Issue {
    key: string;
    title: string;
    type: string;
    project: string;
    owner: string;
    risk: RiskTone;
    stage: string;
    due: string;
    wait: string;
    evidenceHealth: string;
    source: string;
    nextAction: string;
    reason: string;
    closeCondition: string;
    blockedBy: string;
    route: string[];
  }

  interface EvidenceStep {
    label: string;
    state: EvidenceState;
    detail: string;
  }

  interface DecisionItem {
    key: string;
    title: string;
    owner: string;
    window: string;
    decision: string;
  }

  const lanes: Lane[] = [
    { id: 'command', label: '异常处置', description: '只保留需要人介入的偏差', count: 7, mark: 'EX' },
    { id: 'schedule', label: '排期治理', description: '截止日、容量、停滞统一审计', count: 11, mark: 'DL' },
    { id: 'evidence', label: '证据追踪', description: '从需求到代码的事实链路', count: 18, mark: 'EV' },
    { id: 'meeting', label: '周会拍板', description: '把状态会压缩成决策队列', count: 4, mark: 'MT' }
  ];

  const issues: Issue[] = [
    {
      key: 'FZ-2220',
      title: 'Jira Task 已完成但本地负责人仍显示旧执行人',
      type: '事实冲突',
      project: 'FMS-Malaysia',
      owner: '梁志远',
      risk: 'critical',
      stage: '待关闭',
      due: '今天 18:00',
      wait: '2 天',
      evidenceHealth: 'Jira 已确认，本地覆盖待关闭',
      source: 'Jira / Override',
      nextAction: '确认主线事实并撤销覆盖',
      reason: 'Jira 主线状态已经完成，执行子任务仍保留历史负责人，继续展示会误导周会排期和负责人归因。',
      closeCondition: '本地覆盖记录关闭，负责人回写以 Jira 主线事实为准，并保留一条审计记录。',
      blockedBy: '覆盖记录',
      route: ['Jira', 'Sync Worker', 'Override', 'Decision Log']
    },
    {
      key: 'FZ-2198',
      title: '排期后没有新的 MR 或提交证据',
      type: '交付停滞',
      project: 'TOS Core',
      owner: '朱家聪',
      risk: 'warning',
      stage: '推进中',
      due: '明天 12:00',
      wait: '4 天',
      evidenceHealth: '代码证据缺口',
      source: 'GitLab / Schedule',
      nextAction: '发起调停或重排截止日',
      reason: '截止日和分支已经绑定，但 MR、commit 与 CI 记录没有更新，当前承诺缺少可验证进展。',
      closeCondition: '补齐 MR 或 commit 链接；如无法补齐，改写截止日并进入周会拍板队列。',
      blockedBy: 'MR 缺失',
      route: ['Jira', 'Schedule', 'GitLab', 'CI']
    },
    {
      key: 'FZ-2207',
      title: '需求缺少验收口径，AI 解构置信度偏低',
      type: '上下文缺口',
      project: 'Remote OPS',
      owner: '白凌云',
      risk: 'warning',
      stage: '待补充',
      due: '周三 10:00',
      wait: '1 天',
      evidenceHealth: '验收样例缺失',
      source: 'Context Pack',
      nextAction: '补充验收标准后重新解构',
      reason: '当前描述缺少边界、依赖和验收样例，系统可以推断任务结构，但无法给出可靠拆解。',
      closeCondition: '补齐验收标准、依赖方、非目标范围，并重新生成 context pack。',
      blockedBy: '验收标准',
      route: ['Jira', 'AI Deconstructor', 'Context Pack', 'Review']
    },
    {
      key: 'FZ-2212',
      title: 'Context Pack 已归档但周会没有引用证据',
      type: '决策证据',
      project: 'FMS-Archive',
      owner: '岳颖颖',
      risk: 'info',
      stage: '评审中',
      due: '周五',
      wait: '今天',
      evidenceHealth: '证据完整，入口弱',
      source: 'Archive / Meeting',
      nextAction: '把证据卡加入周会队列',
      reason: 'archive 与 context pack 已经绑定，但周会队列仍使用纯文本描述，拍板时需要额外追溯。',
      closeCondition: '周会卡片引用 archive、context pack 与负责人确认记录。',
      blockedBy: '会议入口',
      route: ['Archive', 'Context Pack', 'Meeting', 'Decision Log']
    },
    {
      key: 'FZ-2186',
      title: '核心成员权限边界变更需要回放',
      type: '权限审计',
      project: 'Admin Core',
      owner: 'Eddie',
      risk: 'safe',
      stage: '已闭环',
      due: '已完成',
      wait: '无',
      evidenceHealth: '证据完整',
      source: 'API / Build',
      nextAction: '保留为审计样本',
      reason: '权限边界已通过 API filter 与前端构建验证，可以作为后续治理规则的样本。',
      closeCondition: '无需追加动作，保留 build 记录和 API 过滤说明。',
      blockedBy: '无',
      route: ['API', 'Permission', 'Build', 'Audit']
    }
  ];

  const evidenceMap: Record<string, EvidenceStep[]> = {
    'FZ-2220': [
      { label: 'Jira 主线事实', state: 'done', detail: 'Task 状态已同步为 Done，父级事实可信。' },
      { label: '执行子任务', state: 'done', detail: '子任务进度存在历史负责人字段。' },
      { label: '本地覆盖', state: 'active', detail: '覆盖窗口尚未关闭，需要人工确认。' },
      { label: '审计记录', state: 'missing', detail: '缺少关闭覆盖后的决策日志。' }
    ],
    'FZ-2198': [
      { label: '排期事实', state: 'done', detail: '截止日和负责人已经绑定。' },
      { label: '分支线索', state: 'active', detail: '分支存在，但没有新 MR。' },
      { label: '代码提交', state: 'missing', detail: '最近没有可用于证明进展的提交记录。' },
      { label: 'CI 记录', state: 'missing', detail: '没有新的构建或测试结果。' }
    ],
    'FZ-2207': [
      { label: '需求描述', state: 'active', detail: '已有背景，但缺少验收边界。' },
      { label: 'AI 解构', state: 'active', detail: '可生成结构，但不建议进入排期承诺。' },
      { label: '验收样例', state: 'missing', detail: '需要 PM 补充可判断样例。' },
      { label: '依赖方', state: 'missing', detail: '接口与联调窗口尚未确认。' }
    ],
    'FZ-2212': [
      { label: '归档记录', state: 'done', detail: 'archive 已存在。' },
      { label: 'Context Pack', state: 'done', detail: '已关联需求与拆解结果。' },
      { label: '周会议题', state: 'active', detail: '会议队列引用不完整。' },
      { label: '拍板记录', state: 'missing', detail: '等待周会后写入决策日志。' }
    ],
    'FZ-2186': [
      { label: '权限策略', state: 'done', detail: '核心成员过滤规则已生效。' },
      { label: 'API 校验', state: 'done', detail: '后端接口按权限返回。' },
      { label: '前端构建', state: 'done', detail: '构建通过，页面可加载。' },
      { label: '审计样本', state: 'done', detail: '可作为权限治理示例。' }
    ]
  };

  const decisionQueue: DecisionItem[] = [
    {
      key: 'MT-04',
      title: 'FMS-Malaysia 交付窗口是否拆分',
      owner: '研发负责人',
      window: '周三前',
      decision: '高风险需求集中在同一负责人，需要确认拆分还是增补 reviewer。'
    },
    {
      key: 'MT-05',
      title: '缺陷容量是否挤占需求承诺',
      owner: 'PM',
      window: '周会前',
      decision: '缺陷修复占用本周容量，需要确认是否调整需求截止日。'
    },
    {
      key: 'MT-06',
      title: '无证据停滞项是否升级',
      owner: '技术负责人',
      window: '今天',
      decision: 'FZ-2198 若明天仍无 MR，自动进入版本风险队列。'
    }
  ];

  const auditTrail = [
    '09:42 同步 Jira 主线事实',
    '10:16 识别本地覆盖冲突',
    '10:28 进入人工调停队列'
  ];

  let activeLane: LaneId = 'command';
  let selectedKey = issues[0].key;
  let searchTerm = '';
  let onlyBlocking = true;
  let density: 'compact' | 'comfortable' = 'compact';

  $: activeLaneMeta = lanes.find((lane) => lane.id === activeLane) || lanes[0];
  $: selectedIssue = issues.find((item) => item.key === selectedKey) || issues[0];
  $: selectedEvidence = evidenceMap[selectedIssue.key] || [];
  $: commandRows = issues.filter((item) => isInActiveLane(item));
  $: visibleRows = commandRows.filter((item) => matchesFilters(item));
  $: blockedCount = issues.filter((item) => item.risk === 'critical').length;
  $: warningCount = issues.filter((item) => item.risk === 'warning').length;
  $: evidenceGapCount = issues.filter((item) => item.evidenceHealth !== '证据完整').length;
  $: meetingCount = decisionQueue.length;

  function isInActiveLane(item: Issue): boolean {
    if (activeLane === 'schedule') {
      return item.source.includes('Schedule') || item.stage.includes('推进') || item.due !== '已完成';
    }
    if (activeLane === 'evidence') {
      return item.evidenceHealth !== '证据完整';
    }
    if (activeLane === 'meeting') {
      return item.risk === 'critical' || item.risk === 'warning' || item.source.includes('Meeting');
    }
    return true;
  }

  function matchesFilters(item: Issue): boolean {
    const keyword = searchTerm.trim().toLowerCase();
    if (onlyBlocking && item.risk !== 'critical' && item.risk !== 'warning') return false;
    if (!keyword) return true;
    return [
      item.key,
      item.title,
      item.project,
      item.owner,
      item.stage,
      item.nextAction,
      item.source
    ].some((value) => value.toLowerCase().includes(keyword));
  }

  function selectIssue(key: string) {
    selectedKey = key;
  }

  function selectLane(id: string) {
    activeLane = id as LaneId;
  }

  function clearSearch() {
    searchTerm = '';
  }
</script>

<PrototypeShell lanes={lanes} activeLane={activeLane} onLaneSelect={selectLane}>
    <div class="header-content" slot="header">
      <div class="header-copy">
      <span>Phase 39 / Beauty Pass</span>
      <h2>well-ambient 管理台</h2>
      <p>{activeLaneMeta.description}。这版原型把异常处置、证据链、调停预检和周会拍板压进一个更有视觉主张的工作台。</p>
    </div>
    <div class="header-state">
      <PrototypeBadge tone="critical" label={`${blockedCount} 个阻断`} />
      <PrototypeBadge tone="warning" label={`${warningCount} 个预警`} />
      <PrototypeBadge tone="info" label="今日窗口" />
    </div>
  </div>

  <div class="toolbar-content" slot="toolbar">
    <label class="search-field">
      <span>Search</span>
      <input
        class="wa-focus-ring"
        type="search"
        bind:value={searchTerm}
        placeholder="搜索事项、负责人、项目"
      />
    </label>

    <div class="toolbar-group" aria-label="筛选">
      <button
        type="button"
        class="toggle-button wa-focus-ring"
        class:active={onlyBlocking}
        on:click={() => onlyBlocking = !onlyBlocking}
      >
        只看阻断
      </button>
      <button type="button" class="toggle-button wa-focus-ring" on:click={clearSearch}>清空搜索</button>
    </div>

    <div class="toolbar-group" aria-label="密度">
      <button
        type="button"
        class="toggle-button wa-focus-ring"
        class:active={density === 'compact'}
        on:click={() => density = 'compact'}
      >
        紧凑
      </button>
      <button
        type="button"
        class="toggle-button wa-focus-ring"
        class:active={density === 'comfortable'}
        on:click={() => density = 'comfortable'}
      >
        舒适
      </button>
    </div>

    <button type="button" class="command-button wa-focus-ring">导出议题</button>
  </div>

  <PrototypeTheatre
    item={selectedIssue}
    laneLabel={activeLaneMeta.label}
    blockedCount={blockedCount}
    warningCount={warningCount}
    evidenceGapCount={evidenceGapCount}
  />

  <div class="workspace-grid density-{density}">
    <section class="command-surface wa-panel">
      <div class="metric-strip" aria-label="今日摘要">
        <PrototypeMetric label="必须介入" value={`${blockedCount + warningCount}`} helper="阻断与预警合并进入处置台" tone="critical" meta="TODAY" />
        <PrototypeMetric label="证据缺口" value={`${evidenceGapCount}`} helper="MR、CI、验收样例是主要缺口" tone="warning" meta="EV" />
        <PrototypeMetric label="周会拍板" value={`${meetingCount}`} helper="资源、拆分、升级三类问题" tone="info" meta="MT" />
        <PrototypeMetric label="已闭环样本" value="1" helper="可复用为权限治理审计样本" tone="safe" meta="OK" />
      </div>

      <div class="surface-title-row">
        <div>
          <span>Command Table</span>
          <h3>{activeLaneMeta.label}列表</h3>
        </div>
        <div class="surface-counters">
          <PrototypeBadge tone="neutral" label={`${visibleRows.length} 条显示`} compact />
          <PrototypeBadge tone="info" label={selectedIssue.key} compact />
        </div>
      </div>

      <PrototypeTable rows={visibleRows} selectedKey={selectedKey} onSelect={selectIssue} />

      <div class="lower-grid">
        <section class="decision-queue">
          <div class="surface-title-row tight">
            <div>
              <span>Meeting Queue</span>
              <h3>周会拍板队列</h3>
            </div>
            <PrototypeBadge tone="warning" label="需要明确结论" compact />
          </div>
          <div class="decision-list">
            {#each decisionQueue as item}
              <article>
                <div>
                  <span>{item.key}</span>
                  <strong>{item.title}</strong>
                </div>
                <p>{item.decision}</p>
                <footer>
                  <span>{item.owner}</span>
                  <em>{item.window}</em>
                </footer>
              </article>
            {/each}
          </div>
        </section>

        <section class="workflow-map">
          <div class="surface-title-row tight">
            <div>
              <span>Control Path</span>
              <h3>当前事项流转路径</h3>
            </div>
            <PrototypeBadge tone={selectedIssue.risk} label={selectedIssue.blockedBy} compact />
          </div>
          <div class="path-grid">
            {#each selectedIssue.route as stop, index}
              <div class="path-node">
                <span>{String(index + 1).padStart(2, '0')}</span>
                <strong>{stop}</strong>
              </div>
            {/each}
          </div>
        </section>
      </div>
    </section>

    <PrototypeInspector item={selectedIssue} evidenceSteps={selectedEvidence} auditTrail={auditTrail} />
  </div>
</PrototypeShell>

<style>
  :global(body:has(.prototype-shell)) {
    background: var(--wa-bg-base);
  }

  .header-content {
    min-width: 0;
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 18px;
  }

  .header-copy {
    min-width: 0;
  }

  .header-copy span,
  .surface-title-row span {
    display: block;
    color: var(--wa-text-subtle);
    font-family: var(--wa-font-mono);
    font-size: 11px;
    font-weight: 800;
    line-height: 1.2;
    text-transform: uppercase;
  }

  .header-copy h2 {
    margin: 7px 0 0;
    color: var(--wa-text-strong);
    font-family: var(--wa-font-display);
    font-size: 34px;
    font-weight: 900;
    letter-spacing: -0.035em;
    line-height: 0.95;
    text-wrap: balance;
  }

  .header-copy p {
    max-width: 860px;
    margin: 9px 0 0;
    color: var(--wa-text-muted);
    font-size: 14px;
    line-height: 1.5;
    text-wrap: pretty;
  }

  .header-state {
    flex: 0 0 auto;
    display: flex;
    flex-wrap: wrap;
    justify-content: flex-end;
    gap: 8px;
  }

  .toolbar-content {
    display: grid;
    grid-template-columns: minmax(260px, 1fr) auto auto auto;
    align-items: center;
    gap: 10px;
  }

  .search-field {
    min-width: 0;
    height: var(--wa-touch-h);
    display: grid;
    grid-template-columns: auto minmax(0, 1fr);
    align-items: center;
    gap: 10px;
    padding: 0 12px;
    border: 1px solid var(--wa-border-soft);
    border-radius: var(--wa-radius-sm);
    background:
      linear-gradient(180deg, rgba(244, 251, 255, 0.055), rgba(244, 251, 255, 0.02)),
      rgba(4, 12, 17, 0.42);
  }

  .search-field span {
    color: var(--wa-text-subtle);
    font-family: var(--wa-font-mono);
    font-size: 11px;
    font-weight: 800;
    text-transform: uppercase;
  }

  .search-field input {
    min-width: 0;
    width: 100%;
    border: 0;
    outline: 0;
    background: transparent;
    color: var(--wa-text-strong);
    font-size: 13px;
  }

  .search-field input::placeholder {
    color: var(--wa-text-subtle);
  }

  .toolbar-group {
    display: inline-grid;
    grid-auto-flow: column;
    gap: 6px;
    min-width: 0;
    padding: 4px;
    border: 1px solid var(--wa-border-soft);
    border-radius: var(--wa-radius-sm);
    background: rgba(4, 12, 17, 0.34);
  }

  .toggle-button,
  .command-button {
    min-height: 36px;
    border-radius: var(--wa-radius-sm);
    font-size: 12px;
    font-weight: 800;
    cursor: pointer;
    white-space: nowrap;
  }

  .toggle-button {
    border: 1px solid transparent;
    background: transparent;
    color: var(--wa-text-muted);
    padding: 0 12px;
  }

  .toggle-button:hover,
  .toggle-button.active {
    border-color: var(--wa-border-soft);
    background: rgba(38, 221, 255, 0.1);
    color: var(--wa-text-strong);
  }

  .command-button {
    min-height: var(--wa-touch-h);
    border: 1px solid rgba(184, 245, 255, 0.52);
    background: var(--wa-accent);
    color: var(--wa-accent-ink);
    padding: 0 16px;
    box-shadow: 0 12px 34px rgba(38, 221, 255, 0.18);
  }

  .workspace-grid {
    min-width: 0;
    display: grid;
    grid-template-columns: minmax(0, 1fr) var(--wa-inspector-w);
    gap: 14px;
    align-items: start;
  }

  .command-surface {
    min-width: 0;
    padding: 16px;
    display: grid;
    gap: 16px;
    border-radius: 20px;
  }

  .metric-strip {
    display: grid;
    grid-template-columns: repeat(4, minmax(0, 1fr));
    gap: 10px;
  }

  .surface-title-row {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 14px;
    min-width: 0;
  }

  .surface-title-row.tight {
    align-items: flex-start;
  }

  .surface-title-row h3 {
    margin: 5px 0 0;
    color: var(--wa-text-strong);
    font-size: 15px;
    font-weight: 780;
    line-height: 1.2;
  }

  .surface-counters {
    display: flex;
    flex-wrap: wrap;
    justify-content: flex-end;
    gap: 6px;
  }

  .lower-grid {
    display: grid;
    grid-template-columns: 1.1fr 0.9fr;
    gap: 12px;
  }

  .decision-queue,
  .workflow-map {
    min-width: 0;
    padding: 14px;
    border: 1px solid var(--wa-border-soft);
    border-radius: 16px;
    background:
      linear-gradient(180deg, rgba(244, 251, 255, 0.045), rgba(244, 251, 255, 0.015)),
      rgba(5, 12, 17, 0.52);
  }

  .decision-list {
    margin-top: 12px;
    display: grid;
    gap: 8px;
  }

  .decision-list article {
    min-width: 0;
    padding: 12px;
    border: 1px solid rgba(184, 245, 255, 0.13);
    border-radius: 12px;
    background: rgba(244, 251, 255, 0.035);
    transition:
      transform var(--wa-duration-fast) var(--wa-ease),
      border-color var(--wa-duration-fast) var(--wa-ease),
      background var(--wa-duration-fast) var(--wa-ease);
  }

  .decision-list article:hover {
    transform: translateY(-1px);
    border-color: rgba(184, 245, 255, 0.26);
    background: rgba(38, 221, 255, 0.055);
  }

  .decision-list article div {
    display: flex;
    align-items: center;
    gap: 8px;
    min-width: 0;
  }

  .decision-list article div span {
    flex: 0 0 auto;
    color: var(--wa-text-subtle);
    font-family: var(--wa-font-mono);
    font-size: 11px;
  }

  .decision-list article strong {
    overflow: hidden;
    color: var(--wa-text-strong);
    font-size: 12px;
    line-height: 1.35;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .decision-list article p {
    margin: 8px 0 0;
    color: var(--wa-text-muted);
    font-size: 12px;
    line-height: 1.45;
  }

  .decision-list article footer {
    margin-top: 9px;
    display: flex;
    justify-content: space-between;
    gap: 10px;
    color: var(--wa-text-subtle);
    font-size: 11px;
  }

  .decision-list article footer em {
    font-style: normal;
    font-family: var(--wa-font-mono);
  }

  .path-grid {
    margin-top: 12px;
    display: grid;
    grid-template-columns: repeat(2, minmax(0, 1fr));
    gap: 8px;
  }

  .path-node {
    min-width: 0;
    min-height: 66px;
    padding: 12px;
    border: 1px solid rgba(184, 245, 255, 0.13);
    border-radius: 14px;
    background:
      radial-gradient(circle at 100% 0, rgba(38, 221, 255, 0.1), transparent 40%),
      rgba(244, 251, 255, 0.035);
    display: grid;
    align-content: space-between;
    gap: 8px;
  }

  .path-node span {
    color: var(--wa-text-subtle);
    font-family: var(--wa-font-mono);
    font-size: 11px;
  }

  .path-node strong {
    overflow: hidden;
    color: var(--wa-text-main);
    font-size: 12px;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .density-comfortable :global(.prototype-table th),
  .density-comfortable :global(.prototype-table td) {
    padding-top: 14px;
    padding-bottom: 14px;
  }

  @media (max-width: 1320px) {
    .workspace-grid {
      grid-template-columns: 1fr;
    }
  }

  @media (max-width: 1060px) {
    .toolbar-content,
    .metric-strip,
    .lower-grid {
      grid-template-columns: 1fr;
    }

    .toolbar-group {
      grid-auto-flow: row;
      grid-template-columns: repeat(2, minmax(0, 1fr));
    }
  }

  @media (max-width: 720px) {
    .header-content,
    .surface-title-row {
      align-items: flex-start;
      flex-direction: column;
    }

    .header-state,
    .surface-counters {
      justify-content: flex-start;
    }

    .toolbar-content {
      gap: 8px;
    }

    .search-field {
      grid-template-columns: 1fr;
      height: auto;
      min-height: var(--wa-touch-h);
      padding: 8px 10px;
      gap: 6px;
    }

    .path-grid {
      grid-template-columns: 1fr;
    }
  }
</style>
