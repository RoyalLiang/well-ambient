<script lang="ts">
  import PrototypeBadge from './PrototypeBadge.svelte';

  type RiskTone = 'critical' | 'warning' | 'safe' | 'info';

  interface TheatreIssue {
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
    nextAction: string;
    blockedBy: string;
  }

  export let item: TheatreIssue;
  export let laneLabel = '';
  export let blockedCount = 0;
  export let warningCount = 0;
  export let evidenceGapCount = 0;

  function riskLabel(risk: RiskTone): string {
    if (risk === 'critical') return '阻断';
    if (risk === 'warning') return '预警';
    if (risk === 'safe') return '健康';
    return '关注';
  }
</script>

<section class="prototype-theatre" aria-label="指挥室状态剧场">
  <div class="theatre-field" aria-hidden="true">
    <span class="field-line line-a"></span>
    <span class="field-line line-b"></span>
    <span class="field-pulse pulse-a"></span>
    <span class="field-pulse pulse-b"></span>
  </div>

  <div class="theatre-copy">
    <div class="theatre-meta">
      <PrototypeBadge tone={item.risk} label={riskLabel(item.risk)} />
      <span>{laneLabel}</span>
      <span>{item.project}</span>
    </div>

    <h1>{item.key}</h1>
    <p>{item.title}</p>

    <div class="theatre-actions" aria-label="关键动作">
      <button type="button" class="primary-action wa-focus-ring">启动调停</button>
      <button type="button" class="secondary-action wa-focus-ring">打开证据链</button>
    </div>
  </div>

  <div class="theatre-console" aria-label="当前事项状态">
    <div class="console-core">
      <span>Focus</span>
      <strong>{item.blockedBy}</strong>
      <small>{item.nextAction}</small>
    </div>

    <div class="console-grid">
      <div>
        <span>Owner</span>
        <strong>{item.owner}</strong>
      </div>
      <div>
        <span>Due</span>
        <strong>{item.due}</strong>
      </div>
      <div>
        <span>Wait</span>
        <strong>{item.wait}</strong>
      </div>
      <div>
        <span>Stage</span>
        <strong>{item.stage}</strong>
      </div>
    </div>

    <div class="signal-row" aria-label="风险信号">
      <div>
        <span>{blockedCount}</span>
        <small>阻断</small>
      </div>
      <div>
        <span>{warningCount}</span>
        <small>预警</small>
      </div>
      <div>
        <span>{evidenceGapCount}</span>
        <small>证据缺口</small>
      </div>
    </div>
  </div>
</section>

<style>
  .prototype-theatre {
    position: relative;
    min-height: 308px;
    overflow: hidden;
    border: 1px solid rgba(184, 245, 255, 0.2);
    border-radius: 24px;
    background:
      radial-gradient(circle at 18% 20%, rgba(38, 221, 255, 0.25), transparent 34%),
      radial-gradient(circle at 78% 16%, rgba(255, 97, 119, 0.12), transparent 30%),
      linear-gradient(135deg, rgba(244, 251, 255, 0.105), rgba(244, 251, 255, 0.025) 38%, rgba(38, 221, 255, 0.075)),
      #071017;
    box-shadow:
      inset 0 1px 0 rgba(244, 251, 255, 0.13),
      inset 0 -1px 0 rgba(38, 221, 255, 0.09),
      0 34px 100px rgba(1, 9, 14, 0.48),
      0 0 80px rgba(38, 221, 255, 0.08);
    display: grid;
    grid-template-columns: minmax(0, 1.25fr) minmax(320px, 0.75fr);
    gap: 22px;
    padding: 28px;
    isolation: isolate;
  }

  .prototype-theatre::before {
    content: "";
    position: absolute;
    inset: 0;
    z-index: -2;
    background:
      linear-gradient(115deg, transparent 0 22%, rgba(184, 245, 255, 0.08) 22% 23%, transparent 23% 100%),
      repeating-linear-gradient(0deg, rgba(184, 245, 255, 0.055) 0 1px, transparent 1px 34px);
    mask-image: linear-gradient(90deg, rgba(0, 0, 0, 0.95), transparent 75%);
  }

  .prototype-theatre::after {
    content: "";
    position: absolute;
    inset: auto -18% -46% 38%;
    height: 72%;
    z-index: -1;
    background: radial-gradient(ellipse at center, rgba(38, 221, 255, 0.22), transparent 62%);
    filter: blur(30px);
  }

  .theatre-field {
    position: absolute;
    inset: 0;
    z-index: -1;
    pointer-events: none;
  }

  .field-line {
    position: absolute;
    border: 1px solid rgba(184, 245, 255, 0.14);
    border-radius: 50%;
  }

  .line-a {
    width: 440px;
    height: 440px;
    right: -120px;
    top: -160px;
  }

  .line-b {
    width: 540px;
    height: 220px;
    right: -180px;
    bottom: -84px;
    transform: rotate(-18deg);
  }

  .field-pulse {
    position: absolute;
    width: 9px;
    height: 9px;
    border-radius: 50%;
    background: var(--wa-accent);
    box-shadow: 0 0 22px rgba(38, 221, 255, 0.85);
    animation: floatPulse 4.8s var(--wa-ease) infinite;
  }

  .pulse-a {
    right: 25%;
    top: 26%;
  }

  .pulse-b {
    right: 12%;
    bottom: 28%;
    animation-delay: 900ms;
  }

  .theatre-copy,
  .theatre-console {
    min-width: 0;
  }

  .theatre-copy {
    display: grid;
    align-content: center;
    gap: 18px;
  }

  .theatre-meta {
    display: flex;
    align-items: center;
    flex-wrap: wrap;
    gap: 8px;
  }

  .theatre-meta > span {
    min-height: 24px;
    display: inline-flex;
    align-items: center;
    padding: 0 9px;
    border: 1px solid rgba(184, 245, 255, 0.13);
    border-radius: 999px;
    background: rgba(244, 251, 255, 0.055);
    color: var(--wa-text-muted);
    font-size: 12px;
    font-weight: 800;
  }

  h1 {
    margin: 0;
    color: var(--wa-text-strong);
    font-family: var(--wa-font-display);
    font-size: 66px;
    font-weight: 900;
    letter-spacing: -0.04em;
    line-height: 0.9;
    text-wrap: balance;
  }

  p {
    max-width: 780px;
    margin: 0;
    color: #dff0f5;
    font-size: 18px;
    font-weight: 650;
    line-height: 1.36;
    text-wrap: balance;
  }

  .theatre-actions {
    display: flex;
    flex-wrap: wrap;
    gap: 10px;
  }

  .primary-action,
  .secondary-action {
    min-height: var(--wa-touch-h);
    border-radius: 999px;
    padding: 0 18px;
    font-size: 13px;
    font-weight: 850;
    cursor: pointer;
    transition:
      transform var(--wa-duration-fast) var(--wa-ease),
      border-color var(--wa-duration-fast) var(--wa-ease),
      background var(--wa-duration-fast) var(--wa-ease);
  }

  .primary-action {
    border: 1px solid rgba(184, 245, 255, 0.52);
    background: var(--wa-accent);
    color: var(--wa-accent-ink);
    box-shadow: 0 14px 38px rgba(38, 221, 255, 0.22);
  }

  .secondary-action {
    border: 1px solid rgba(184, 245, 255, 0.2);
    background: rgba(244, 251, 255, 0.065);
    color: var(--wa-text-main);
  }

  .primary-action:hover,
  .secondary-action:hover {
    transform: translateY(-1px);
    border-color: rgba(184, 245, 255, 0.42);
  }

  .theatre-console {
    position: relative;
    align-self: stretch;
    min-height: 250px;
    padding: 16px;
    border: 1px solid rgba(184, 245, 255, 0.18);
    border-radius: 20px;
    background: rgba(5, 12, 17, 0.54);
    box-shadow: inset 0 1px 0 rgba(244, 251, 255, 0.08);
    backdrop-filter: blur(20px) saturate(130%);
    -webkit-backdrop-filter: blur(20px) saturate(130%);
    display: grid;
    align-content: space-between;
    gap: 14px;
  }

  .console-core {
    min-height: 112px;
    padding: 16px;
    border: 1px solid rgba(38, 221, 255, 0.18);
    border-radius: 18px;
    background:
      linear-gradient(160deg, rgba(38, 221, 255, 0.13), rgba(244, 251, 255, 0.035)),
      rgba(4, 12, 17, 0.62);
    display: grid;
    align-content: center;
    gap: 8px;
  }

  .console-core span,
  .console-grid span,
  .signal-row small {
    color: var(--wa-text-subtle);
    font-family: var(--wa-font-mono);
    font-size: 11px;
    font-weight: 800;
    text-transform: uppercase;
  }

  .console-core strong {
    color: var(--wa-text-strong);
    font-size: 24px;
    font-weight: 880;
    line-height: 1.05;
  }

  .console-core small {
    color: var(--wa-text-muted);
    font-size: 12px;
    line-height: 1.42;
  }

  .console-grid,
  .signal-row {
    display: grid;
    gap: 8px;
  }

  .console-grid {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }

  .console-grid div,
  .signal-row div {
    min-width: 0;
    border: 1px solid rgba(184, 245, 255, 0.12);
    border-radius: 12px;
    background: rgba(244, 251, 255, 0.04);
  }

  .console-grid div {
    padding: 10px;
  }

  .console-grid strong {
    display: block;
    margin-top: 5px;
    overflow: hidden;
    color: var(--wa-text-main);
    font-size: 13px;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .signal-row {
    grid-template-columns: repeat(3, minmax(0, 1fr));
  }

  .signal-row div {
    min-height: 56px;
    padding: 9px;
    display: grid;
    align-content: center;
    justify-items: start;
    gap: 3px;
  }

  .signal-row span {
    color: var(--wa-accent-strong);
    font-family: var(--wa-font-mono);
    font-size: 22px;
    font-weight: 850;
    line-height: 1;
  }

  @keyframes floatPulse {
    0%,
    100% {
      opacity: 0.5;
      transform: translate3d(0, 0, 0) scale(1);
    }

    50% {
      opacity: 1;
      transform: translate3d(-8px, 10px, 0) scale(1.45);
    }
  }

  @media (max-width: 1180px) {
    .prototype-theatre {
      grid-template-columns: 1fr;
    }
  }

  @media (max-width: 720px) {
    .prototype-theatre {
      min-height: auto;
      padding: 18px;
      border-radius: 18px;
    }

    h1 {
      font-size: 42px;
    }

    p {
      font-size: 15px;
    }

    .console-grid,
    .signal-row {
      grid-template-columns: 1fr;
    }
  }

  @media (prefers-reduced-motion: reduce) {
    .field-pulse {
      animation: none;
    }
  }
</style>
