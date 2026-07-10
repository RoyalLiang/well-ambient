<script lang="ts">
  import PrototypeBadge from './PrototypeBadge.svelte';

  type RiskTone = 'critical' | 'warning' | 'safe' | 'info';
  type EvidenceState = 'done' | 'active' | 'missing';

  interface InspectorIssue {
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

  export let item: InspectorIssue | null = null;
  export let evidenceSteps: EvidenceStep[] = [];
  export let auditTrail: string[] = [];

  function riskLabel(risk: RiskTone): string {
    if (risk === 'critical') return '阻断';
    if (risk === 'warning') return '预警';
    if (risk === 'safe') return '健康';
    return '关注';
  }

  function stateLabel(state: EvidenceState): string {
    if (state === 'done') return '已确认';
    if (state === 'active') return '处理中';
    return '缺失';
  }

  function stateTone(state: EvidenceState): RiskTone {
    if (state === 'done') return 'safe';
    if (state === 'active') return 'info';
    return 'warning';
  }
</script>

<aside class="prototype-inspector wa-glass" aria-label="调停预检">
  {#if item}
    <div class="inspector-head">
      <div>
        <span class="eyebrow">调停预检</span>
        <h3>{item.key}</h3>
      </div>
      <PrototypeBadge tone={item.risk} label={riskLabel(item.risk)} />
    </div>

    <section class="inspector-section">
      <h4>{item.title}</h4>
      <dl class="fact-grid">
        <div>
          <dt>项目</dt>
          <dd>{item.project}</dd>
        </div>
        <div>
          <dt>负责人</dt>
          <dd>{item.owner}</dd>
        </div>
        <div>
          <dt>等待</dt>
          <dd>{item.wait}</dd>
        </div>
        <div>
          <dt>到期</dt>
          <dd>{item.due}</dd>
        </div>
      </dl>
    </section>

    <section class="inspector-section">
      <div class="section-title-row">
        <h4>处置判断</h4>
        <span>{item.type}</span>
      </div>
      <p>{item.reason}</p>
      <div class="close-box">
        <span>关闭条件</span>
        <strong>{item.closeCondition}</strong>
      </div>
    </section>

    <section class="inspector-section">
      <div class="section-title-row">
        <h4>证据链</h4>
        <span>{item.evidenceHealth}</span>
      </div>
      <ol class="evidence-list">
        {#each evidenceSteps as step}
          <li class="state-{step.state}">
            <span class="step-index" aria-hidden="true"></span>
            <div>
              <div class="step-row">
                <strong>{step.label}</strong>
                <PrototypeBadge tone={stateTone(step.state)} label={stateLabel(step.state)} compact />
              </div>
              <p>{step.detail}</p>
            </div>
          </li>
        {/each}
      </ol>
    </section>

    <section class="inspector-section">
      <div class="section-title-row">
        <h4>流转路径</h4>
        <span>{item.blockedBy}</span>
      </div>
      <div class="route-strip">
        {#each item.route as stop}
          <span>{stop}</span>
        {/each}
      </div>
    </section>

    <section class="inspector-section">
      <h4>审计记录</h4>
      <ul class="audit-list">
        {#each auditTrail as entry}
          <li>{entry}</li>
        {/each}
      </ul>
    </section>

    <div class="inspector-actions" aria-label="调停操作">
      <button type="button" class="secondary-action wa-focus-ring">生成议题</button>
      <button type="button" class="secondary-action wa-focus-ring">标记已处理</button>
      <button type="button" class="primary-action wa-focus-ring">发起调停</button>
    </div>
  {/if}
</aside>

<style>
  .prototype-inspector {
    position: sticky;
    top: 12px;
    min-width: 0;
    overflow: hidden;
    border-radius: 22px;
    padding: 16px;
    display: grid;
    align-content: start;
    gap: 13px;
    background:
      radial-gradient(circle at 78% 2%, rgba(38, 221, 255, 0.16), transparent 32%),
      linear-gradient(180deg, rgba(244, 251, 255, 0.07), rgba(244, 251, 255, 0.018)),
      rgba(7, 14, 20, 0.82);
  }

  .prototype-inspector::before {
    content: "";
    position: absolute;
    inset: 0;
    pointer-events: none;
    background:
      linear-gradient(90deg, rgba(38, 221, 255, 0.18), transparent 24%),
      repeating-linear-gradient(0deg, rgba(184, 245, 255, 0.045) 0 1px, transparent 1px 42px);
    opacity: 0.45;
    mask-image: linear-gradient(180deg, rgba(0, 0, 0, 0.75), transparent 60%);
  }

  .inspector-head,
  .section-title-row,
  .step-row {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 12px;
    min-width: 0;
  }

  .eyebrow {
    display: block;
    color: var(--wa-text-subtle);
    font-size: 11px;
    font-weight: 800;
    line-height: 1.2;
    text-transform: uppercase;
  }

  .inspector-head h3 {
    margin: 7px 0 0;
    color: var(--wa-text-strong);
    font-family: var(--wa-font-mono);
    font-size: 30px;
    font-weight: 900;
    letter-spacing: -0.055em;
    line-height: 1;
  }

  .inspector-section {
    position: relative;
    min-width: 0;
    padding: 13px;
    border: 1px solid rgba(184, 245, 255, 0.13);
    border-radius: 16px;
    background:
      linear-gradient(180deg, rgba(244, 251, 255, 0.045), rgba(244, 251, 255, 0.012)),
      rgba(4, 10, 14, 0.38);
    box-shadow: inset 0 1px 0 rgba(244, 251, 255, 0.045);
  }

  .inspector-section h4 {
    margin: 0;
    color: var(--wa-text-strong);
    font-size: 14px;
    font-weight: 850;
    line-height: 1.35;
  }

  .inspector-section p {
    margin: 10px 0 0;
    color: #a9bfca;
    font-size: 12px;
    line-height: 1.55;
  }

  .section-title-row span {
    flex: 0 0 auto;
    color: var(--wa-text-subtle);
    font-family: var(--wa-font-mono);
    font-size: 11px;
  }

  .fact-grid {
    margin: 12px 0 0;
    display: grid;
    grid-template-columns: repeat(2, minmax(0, 1fr));
    gap: 8px;
  }

  .fact-grid div {
    min-width: 0;
    padding: 9px;
    border: 1px solid rgba(184, 245, 255, 0.1);
    border-radius: 12px;
    background: rgba(244, 251, 255, 0.035);
  }

  .fact-grid dt {
    color: var(--wa-text-subtle);
    font-size: 11px;
  }

  .fact-grid dd {
    margin: 5px 0 0;
    overflow: hidden;
    color: var(--wa-text-strong);
    font-size: 12px;
    font-weight: 750;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .close-box {
    margin-top: 12px;
    padding: 12px;
    border: 1px solid rgba(38, 221, 255, 0.22);
    border-radius: 14px;
    background:
      radial-gradient(circle at 100% 0, rgba(38, 221, 255, 0.14), transparent 42%),
      rgba(38, 221, 255, 0.075);
  }

  .close-box span {
    display: block;
    color: var(--wa-text-subtle);
    font-size: 11px;
  }

  .close-box strong {
    display: block;
    margin-top: 5px;
    color: var(--wa-text-main);
    font-size: 12px;
    line-height: 1.45;
  }

  .evidence-list {
    margin: 12px 0 0;
    padding: 0;
    display: grid;
    gap: 9px;
    list-style: none;
  }

  .evidence-list li {
    display: grid;
    grid-template-columns: 14px minmax(0, 1fr);
    gap: 10px;
    min-width: 0;
  }

  .step-index {
    width: 10px;
    height: 10px;
    margin-top: 4px;
    border: 2px solid var(--wa-text-subtle);
    border-radius: 50%;
  }

  .state-done .step-index {
    border-color: var(--wa-success);
    background: var(--wa-success);
    box-shadow: 0 0 14px rgba(114, 230, 180, 0.42);
  }

  .state-active .step-index {
    border-color: var(--wa-accent);
    box-shadow: 0 0 14px rgba(38, 221, 255, 0.4);
  }

  .state-missing .step-index {
    border-color: var(--wa-warning);
    background: rgba(255, 197, 95, 0.18);
    box-shadow: 0 0 14px rgba(255, 197, 95, 0.36);
  }

  .step-row strong {
    overflow: hidden;
    color: var(--wa-text-strong);
    font-size: 12px;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .route-strip {
    margin-top: 12px;
    display: flex;
    flex-wrap: wrap;
    gap: 7px;
  }

  .route-strip span {
    min-height: 26px;
    padding: 6px 8px;
    border: 1px solid rgba(184, 245, 255, 0.12);
    border-radius: 999px;
    background: rgba(244, 251, 255, 0.045);
    color: var(--wa-text-muted);
    font-size: 11px;
    font-weight: 700;
  }

  .audit-list {
    margin: 10px 0 0;
    padding: 0;
    display: grid;
    gap: 7px;
    list-style: none;
  }

  .audit-list li {
    color: #a9bfca;
    font-size: 12px;
    line-height: 1.45;
  }

  .inspector-actions {
    display: grid;
    grid-template-columns: repeat(2, minmax(0, 1fr));
    gap: 8px;
  }

  .primary-action,
  .secondary-action {
    min-height: var(--wa-touch-h);
    border-radius: 999px;
    font-size: 12px;
    font-weight: 800;
    cursor: pointer;
    transition:
      transform var(--wa-duration-fast) var(--wa-ease),
      border-color var(--wa-duration-fast) var(--wa-ease),
      background var(--wa-duration-fast) var(--wa-ease);
  }

  .primary-action {
    grid-column: 1 / -1;
    border: 1px solid rgba(184, 245, 255, 0.5);
    background: var(--wa-accent);
    color: var(--wa-accent-ink);
    box-shadow: 0 16px 38px rgba(38, 221, 255, 0.2);
  }

  .secondary-action {
    border: 1px solid rgba(184, 245, 255, 0.16);
    background: rgba(244, 251, 255, 0.055);
    color: var(--wa-text-main);
  }

  .primary-action:hover,
  .secondary-action:hover {
    transform: translateY(-1px);
    border-color: rgba(184, 245, 255, 0.38);
  }

  @media (max-width: 720px) {
    .prototype-inspector {
      position: relative;
      top: auto;
    }

    .fact-grid {
      grid-template-columns: 1fr;
    }
  }
</style>
