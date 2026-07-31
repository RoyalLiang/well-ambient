<script lang="ts">
  type WorkspaceTone = 'cyan' | 'green' | 'amber' | 'rose' | 'violet' | 'slate';
  type SignalTone = 'neutral' | 'good' | 'warn' | 'danger' | 'info';

  interface WorkspaceSignal {
    label: string;
    value: string;
    tone?: SignalTone;
  }

  interface WorkspaceAction {
    label: string;
    route: string;
    kind?: 'primary' | 'secondary';
  }

  interface WorkspaceDecisionEventSummary {
    kind: 'automatic' | 'manual';
    kindLabel: string;
    taskId: string;
    title: string;
    message: string;
    dateLabel: string;
    timeLabel: string;
    dateTime: string;
  }

  export let kicker = '';
  export let title = '';
  export let summary = '';
  export let statusLabel = '';
  export let tone: WorkspaceTone = 'cyan';
  export let signals: WorkspaceSignal[] = [];
  export let actions: WorkspaceAction[] = [];
  export let showContextBar = false;
  export let showBreadcrumbBar = true;
  export let breadcrumbs: string[] = [];
  export let contextStatus = '';
  export let contextMeta = '';
  export let latestEvent: WorkspaceDecisionEventSummary | null = null;
  export let onLatestEventClick: () => void = () => {};
  export let onNavigate: (route: string) => void = () => {};
</script>

<section class="functional-workspace tone-{tone}" aria-label={title || '工作区'}>
  {#if showBreadcrumbBar && breadcrumbs.length > 0}
    <nav class="workspace-breadcrumb-bar" class:has-latest-event={latestEvent !== null} aria-label="面包屑导航">
      <ol>
        {#each breadcrumbs as crumb, index}
          <li class:current={index === breadcrumbs.length - 1}><span>{crumb}</span></li>
        {/each}
      </ol>
      {#if latestEvent}
        <button
          type="button"
          class="workspace-latest-event event-{latestEvent.kind}"
          aria-label={`查看最新事件详情：${latestEvent.taskId} ${latestEvent.message || latestEvent.title}`}
          on:click={onLatestEventClick}
        >
          <i class="latest-event-dot" aria-hidden="true"></i>
          <span class="latest-event-kind">{latestEvent.kindLabel}</span>
          <strong>{latestEvent.taskId}</strong>
          <span class="latest-event-title">{latestEvent.message || latestEvent.title}</span>
          <time datetime={latestEvent.dateTime}>{latestEvent.dateLabel} {latestEvent.timeLabel}</time>
          <span class="latest-event-action">详情 <i aria-hidden="true">→</i></span>
        </button>
      {/if}
      <div class="workspace-breadcrumb-meta">
        {#if contextStatus}<span class="context-status">{contextStatus}</span>{/if}
        {#if contextMeta}<span>{contextMeta}</span>{/if}
      </div>
    </nav>
  {/if}

  {#if showContextBar}
    <header class="workspace-hero">
      <div class="workspace-copy">
        {#if kicker}
          <span class="workspace-kicker">{kicker}</span>
        {/if}
        <h2>{title}</h2>
        {#if summary}
          <p>{summary}</p>
        {/if}
      </div>

      <div class="workspace-command">
        {#if statusLabel}
          <span class="status-chip">{statusLabel}</span>
        {/if}

        {#if actions.length > 0}
          <div class="workspace-actions" aria-label="模块快捷动作">
            {#each actions as action}
              <button
                type="button"
                class:primary={action.kind === 'primary'}
                on:click={() => onNavigate(action.route)}
              >
                {action.label}
              </button>
            {/each}
          </div>
        {/if}
      </div>

      {#if signals.length > 0}
        <div class="signal-strip" aria-label="当前模块状态">
          {#each signals as signal}
            <div class="signal-card signal-{signal.tone || 'neutral'}">
              <span>{signal.label}</span>
              <strong>{signal.value}</strong>
            </div>
          {/each}
        </div>
      {/if}
    </header>
  {/if}

  <div class="workspace-content">
    <slot />
  </div>
</section>

<style>
  .functional-workspace {
    --module-accent: var(--wa-accent, #26ddff);
    --module-accent-soft: rgba(38, 221, 255, 0.13);
    min-width: 0;
    min-height: 100%;
    padding: 18px;
    background:
      radial-gradient(circle at 0% 0%, var(--module-accent-soft), transparent 28rem),
      linear-gradient(180deg, rgba(244, 251, 255, 0.035), rgba(244, 251, 255, 0.012));
  }

  .functional-workspace.tone-green {
    --module-accent: #72e6b4;
    --module-accent-soft: rgba(114, 230, 180, 0.13);
  }

  .functional-workspace.tone-amber {
    --module-accent: #ffc55f;
    --module-accent-soft: rgba(255, 197, 95, 0.13);
  }

  .functional-workspace.tone-rose {
    --module-accent: #ff6177;
    --module-accent-soft: rgba(255, 97, 119, 0.13);
  }

  .functional-workspace.tone-violet {
    --module-accent: #b8a7ff;
    --module-accent-soft: rgba(184, 167, 255, 0.13);
  }

  .functional-workspace.tone-slate {
    --module-accent: #d8e5ec;
    --module-accent-soft: rgba(216, 229, 236, 0.1);
  }

  .workspace-hero {
    position: relative;
    overflow: hidden;
    display: grid;
    grid-template-columns: minmax(0, 1fr) auto;
    gap: 16px;
    align-items: start;
    padding: 20px;
    border: 1px solid var(--wa-border-soft, rgba(203, 234, 244, 0.14));
    border-radius: var(--wa-radius-xl, 18px);
    background:
      linear-gradient(135deg, rgba(244, 251, 255, 0.075), rgba(244, 251, 255, 0.025) 48%, rgba(38, 221, 255, 0.04)),
      rgba(12, 18, 24, 0.76);
    box-shadow: var(--wa-shadow-panel, inset 0 1px 0 rgba(244, 251, 255, 0.065), 0 24px 68px rgba(1, 9, 14, 0.42));
    backdrop-filter: blur(24px) saturate(138%);
    -webkit-backdrop-filter: blur(24px) saturate(138%);
  }

  .workspace-hero::before {
    content: "";
    position: absolute;
    inset: 0;
    pointer-events: none;
    background:
      linear-gradient(90deg, var(--module-accent), transparent 34%),
      radial-gradient(circle at 88% 12%, var(--module-accent-soft), transparent 18rem);
    opacity: 0.46;
    mix-blend-mode: screen;
  }

  .workspace-copy,
  .workspace-command,
  .signal-strip {
    position: relative;
    z-index: 1;
  }

  .workspace-copy {
    min-width: 0;
  }

  .workspace-kicker {
    display: inline-flex;
    align-items: center;
    min-height: 22px;
    color: var(--module-accent);
    font-family: var(--wa-font-mono, ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace);
    font-size: 0.68rem;
    font-weight: 760;
    letter-spacing: 0.12em;
    text-transform: uppercase;
  }

  h2 {
    margin: 6px 0 0;
    color: var(--wa-text-strong, #f4fbff);
    font-size: 24px;
    line-height: 1.08;
    letter-spacing: 0;
    text-wrap: balance;
  }

  p {
    max-width: 760px;
    margin: 9px 0 0;
    color: var(--wa-text-muted, #90a8b5);
    font-size: 0.9rem;
    line-height: 1.65;
    text-wrap: pretty;
  }

  .workspace-command {
    display: grid;
    justify-items: end;
    gap: 10px;
  }

  .status-chip {
    display: inline-flex;
    align-items: center;
    min-height: 30px;
    padding: 0 12px;
    border: 1px solid rgba(203, 234, 244, 0.14);
    border-radius: 999px;
    background: rgba(244, 251, 255, 0.055);
    color: var(--wa-text-main, #d8e5ec);
    font-size: 0.75rem;
    font-weight: 720;
    white-space: nowrap;
  }

  .workspace-actions {
    display: flex;
    flex-wrap: wrap;
    justify-content: flex-end;
    gap: 8px;
  }

  .workspace-actions button {
    min-height: 36px;
    border: 1px solid rgba(203, 234, 244, 0.16);
    border-radius: var(--wa-radius-md, 8px);
    background: rgba(244, 251, 255, 0.055);
    color: var(--wa-text-main, #d8e5ec);
    padding: 0 12px;
    font-size: 0.78rem;
    font-weight: 760;
    cursor: pointer;
    transition: transform 160ms var(--wa-ease, cubic-bezier(0.16, 1, 0.3, 1)), border-color 160ms ease, background 160ms ease;
  }

  .workspace-actions button:hover {
    transform: translateY(-1px);
    border-color: rgba(203, 234, 244, 0.32);
    background: rgba(244, 251, 255, 0.09);
  }

  .workspace-actions button:focus-visible {
    outline: 2px solid var(--module-accent);
    outline-offset: 2px;
  }

  .workspace-actions button.primary {
    border-color: color-mix(in srgb, var(--module-accent) 58%, transparent);
    background: var(--module-accent);
    color: #021318;
  }

  .signal-strip {
    grid-column: 1 / -1;
    display: grid;
    grid-template-columns: repeat(3, minmax(0, 1fr));
    gap: 10px;
    margin-top: 2px;
  }

  .signal-card {
    min-height: 62px;
    display: grid;
    align-content: center;
    gap: 6px;
    padding: 12px;
    border-radius: var(--wa-radius-lg, 12px);
    border: 1px solid rgba(203, 234, 244, 0.12);
    background: rgba(7, 11, 16, 0.52);
  }

  .signal-card span {
    color: var(--wa-text-subtle, #5f7582);
    font-size: 0.7rem;
  }

  .signal-card strong {
    color: var(--wa-text-strong, #f4fbff);
    font-family: var(--wa-font-mono, ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace);
    font-size: 0.92rem;
    font-variant-numeric: tabular-nums;
    line-height: 1.1;
  }

  .signal-good strong {
    color: var(--wa-success, #72e6b4);
  }

  .signal-warn strong {
    color: var(--wa-warning, #ffc55f);
  }

  .signal-danger strong {
    color: var(--wa-danger, #ff6177);
  }

  .signal-info strong {
    color: var(--wa-info, #9bd4ff);
  }

  .workspace-content {
    min-width: 0;
    margin-top: 14px;
  }

  :global(.workspace-content > .demand-dashboard),
  :global(.workspace-content > .kanban-section),
  :global(.workspace-content > .kpi-dashboard),
  :global(.workspace-content > .settings-container),
  :global(.workspace-content > .telemetry-panel),
  :global(.workspace-content > .deconstructor-section) {
    margin: 0;
  }

  :global(.workspace-content > .settings-container) {
    min-height: min(760px, calc(100dvh - 110px));
  }

  @media (max-width: 980px) {
    .workspace-hero {
      grid-template-columns: 1fr;
    }

    .workspace-command {
      justify-items: start;
    }

    .workspace-actions {
      justify-content: flex-start;
    }
  }

  @media (max-width: 700px) {
    .functional-workspace {
      padding: 12px;
    }

    .workspace-hero {
      padding: 16px;
    }

    .signal-strip {
      grid-template-columns: 1fr;
    }
  }

  /* Phase 41: compact light module frame, table-first rather than hero-first. */
  .functional-workspace {
    min-height: 100%;
    padding: 0;
    background: transparent;
    color: var(--wa-text-main, #293847);
  }

  .workspace-content {
    width: 100%;
    max-width: var(--wa-content-max, 1720px);
    margin: 0 auto;
  }

  :global(.workspace-frame.flow-frame .functional-workspace) {
    display: block;
    height: auto;
    min-height: 0;
  }

  :global(.workspace-frame.flow-frame .workspace-content) {
    position: static;
    display: block;
    flex: none;
    height: auto;
    min-height: 0;
    overflow: visible;
  }

  :global(.workspace-frame.flow-frame .workspace-content > .demand-dashboard) {
    position: static;
    inset: auto;
    flex: none;
    height: auto;
    min-height: 0;
  }

  @media (max-width: 860px) {
    :global(.workspace-frame.flow-frame .workspace-content) {
      overflow: visible;
    }

    :global(.workspace-frame.flow-frame .workspace-content > .demand-dashboard) {
      position: static;
      inset: auto;
    }
  }

  .functional-workspace.tone-cyan,
  .functional-workspace.tone-green,
  .functional-workspace.tone-amber,
  .functional-workspace.tone-rose,
  .functional-workspace.tone-violet,
  .functional-workspace.tone-slate {
    --module-accent: var(--wa-accent, #008f96);
    --module-accent-soft: var(--wa-accent-soft, rgba(0, 143, 150, 0.12));
  }

  .workspace-hero {
    margin: 0 0 14px;
    padding: 14px 16px;
    border-color: rgba(121, 139, 159, 0.18);
    border-radius: var(--wa-radius-xl, 12px);
    background: rgba(255, 255, 255, 0.84);
    box-shadow: var(--wa-shadow-sm, 0 10px 24px rgba(26, 41, 58, 0.08));
    backdrop-filter: blur(18px) saturate(124%);
    -webkit-backdrop-filter: blur(18px) saturate(124%);
  }

  .workspace-hero::before {
    opacity: 0;
  }

  .workspace-kicker {
    color: var(--wa-text-muted, #667789);
    font-family: var(--wa-font-sans, -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif);
    font-size: 0.78rem;
    font-weight: 720;
    letter-spacing: 0;
    text-transform: none;
  }

  h2 {
    margin-top: 4px;
    color: var(--wa-text-strong, #0d1722);
    font-size: 20px;
    line-height: 1.2;
  }

  p {
    max-width: 820px;
    margin-top: 6px;
    color: var(--wa-text-muted, #667789);
    font-size: 0.86rem;
    line-height: 1.55;
  }

  .status-chip {
    border-color: rgba(0, 143, 150, 0.18);
    border-radius: var(--wa-radius-sm, 6px);
    background: var(--wa-accent-soft, rgba(0, 143, 150, 0.12));
    color: var(--wa-accent-strong, #006f76);
  }

  .workspace-actions button {
    border-color: rgba(121, 139, 159, 0.18);
    border-radius: var(--wa-radius-md, 8px);
    background: rgba(255, 255, 255, 0.82);
    color: var(--wa-text-main, #293847);
    transition:
      border-color var(--wa-duration-fast, 140ms) var(--wa-ease, cubic-bezier(0.16, 1, 0.3, 1)),
      background var(--wa-duration-fast, 140ms) var(--wa-ease, cubic-bezier(0.16, 1, 0.3, 1)),
      box-shadow var(--wa-duration-fast, 140ms) var(--wa-ease, cubic-bezier(0.16, 1, 0.3, 1));
  }

  .workspace-actions button:hover {
    transform: none;
    border-color: var(--wa-border-strong, rgba(85, 106, 128, 0.38));
    background: var(--wa-surface-flat, #fbfdff);
    box-shadow: 0 8px 20px rgba(26, 41, 58, 0.08);
  }

  .workspace-actions button.primary {
    border-color: var(--wa-accent, #008f96);
    background: var(--wa-accent, #008f96);
    color: var(--wa-accent-ink, #f6fbff);
  }

  .signal-strip {
    grid-template-columns: repeat(4, minmax(0, 1fr));
    gap: 10px;
    margin-top: 8px;
  }

  .signal-card {
    min-height: 58px;
    border-color: rgba(121, 139, 159, 0.16);
    border-radius: var(--wa-radius-lg, 10px);
    background: rgba(247, 250, 252, 0.76);
  }

  .signal-card span {
    color: var(--wa-text-muted, #667789);
  }

  .signal-card strong {
    color: var(--wa-text-strong, #0d1722);
  }

  .signal-good strong {
    color: var(--wa-success, #04966f);
  }

  .signal-warn strong {
    color: var(--wa-warning, #d88700);
  }

  .signal-danger strong {
    color: var(--wa-danger, #dd4b3e);
  }

  .signal-info strong {
    color: var(--wa-info, #256bd8);
  }

  .workspace-content {
    margin-top: 0;
  }

  @media (max-width: 980px) {
    .workspace-hero {
      grid-template-columns: 1fr;
    }

    .workspace-command {
      justify-items: start;
    }

    .signal-strip {
      grid-template-columns: repeat(2, minmax(0, 1fr));
    }
  }

  @media (max-width: 700px) {
    .functional-workspace {
      padding: 0;
    }

    .workspace-hero {
      padding: 14px;
    }

    .signal-strip {
      grid-template-columns: 1fr;
    }
  }

  /* Strongest-brain workspace contract: pages are render-only children. */
  .functional-workspace {
    width: 100%;
    min-width: 0;
    min-height: 0;
    padding: 0 !important;
    background: transparent !important;
    color: var(--wa-text-main, #293847);
  }

  .workspace-content {
    display: grid;
    align-content: start;
    width: 100%;
    max-width: var(--wa-content-max, 1720px);
    min-width: 0;
    min-height: 0;
    margin: 0 auto !important;
  }

  .workspace-breadcrumb-bar {
    width: 100%;
    max-width: var(--wa-content-max, 1720px);
    min-height: 44px;
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 14px;
    margin: 0 auto 12px;
    padding: 0 14px;
    border: 1px solid var(--wa-glass-outline, rgba(72, 98, 118, 0.18));
    border-top-color: var(--wa-glass-highlight, rgba(255, 255, 255, 0.82));
    border-radius: var(--wa-radius-lg, 14px);
    background: var(--wa-glass-panel-strong, rgba(252, 254, 255, 0.88));
    color: var(--wa-text-muted, #667789);
    box-shadow: var(--wa-shadow-glass, inset 0 1px 0 rgba(255, 255, 255, 0.86), 0 6px 14px rgba(30, 52, 68, 0.085));
    -webkit-backdrop-filter: blur(18px) saturate(128%);
    backdrop-filter: blur(18px) saturate(128%);
  }

  .workspace-breadcrumb-bar.has-latest-event {
    display: grid;
    grid-template-columns: minmax(220px, 1fr) minmax(420px, 720px) minmax(180px, 1fr);
  }

  .workspace-breadcrumb-bar.has-latest-event > ol {
    justify-self: start;
  }

  .workspace-breadcrumb-bar.has-latest-event > .workspace-breadcrumb-meta {
    justify-self: end;
  }

  .workspace-breadcrumb-bar ol,
  .workspace-breadcrumb-meta {
    min-width: 0;
    display: flex;
    align-items: center;
    gap: 8px;
    margin: 0;
    padding: 0;
    list-style: none;
  }

  .workspace-breadcrumb-bar li {
    min-width: 0;
    display: flex;
    align-items: center;
    gap: 8px;
    font-size: 12px;
    font-weight: 680;
    white-space: nowrap;
  }

  .workspace-breadcrumb-bar li + li::before {
    content: "/";
    color: var(--wa-text-subtle, #8a99aa);
  }

  .workspace-breadcrumb-bar li.current {
    color: var(--wa-text-strong, #0d1722);
    font-weight: 780;
  }

  .workspace-breadcrumb-meta {
    flex: 0 0 auto;
    color: var(--wa-text-main, #293847);
    font-size: 11px;
    font-weight: 680;
  }

  .workspace-latest-event {
    min-width: 0;
    min-height: 34px;
    display: grid;
    grid-template-columns: auto auto auto minmax(72px, 1fr) auto auto;
    align-items: center;
    gap: 8px;
    padding: 0 2px;
    border: 0;
    background: transparent;
    color: var(--wa-text-main, #293847);
    font: inherit;
    cursor: pointer;
  }

  .workspace-latest-event:hover {
    background: transparent;
  }

  .workspace-latest-event:focus-visible {
    outline: none;
  }

  .workspace-latest-event:focus-visible .latest-event-title,
  .workspace-latest-event:focus-visible .latest-event-action {
    color: var(--wa-accent-strong, #006f76);
  }

  .workspace-latest-event:focus-visible .latest-event-action {
    text-decoration: underline 2px;
    text-underline-offset: 4px;
  }

  .latest-event-kind,
  .latest-event-action {
    white-space: nowrap;
    font-size: 10px;
    font-weight: 760;
  }

  .latest-event-dot {
    width: 7px;
    height: 7px;
    border-radius: 999px;
    background: var(--wa-info, #256bd8);
  }

  .event-manual .latest-event-dot {
    background: var(--wa-accent, #008f96);
  }

  .latest-event-kind {
    color: var(--wa-info, #256bd8);
  }

  .event-manual .latest-event-kind {
    color: var(--wa-accent-strong, #006f76);
  }

  .workspace-latest-event strong {
    color: var(--wa-text-strong, #0d1722);
    font-family: var(--wa-font-mono, ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace);
    font-size: 11px;
    font-variant-numeric: tabular-nums;
    white-space: nowrap;
  }

  .latest-event-title {
    min-width: 0;
    overflow: hidden;
    color: var(--wa-text-main, #293847);
    font-size: 11px;
    font-weight: 680;
    text-align: left;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .workspace-latest-event time {
    color: var(--wa-text-muted, #667789);
    font-family: var(--wa-font-mono, ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace);
    font-size: 10px;
    font-variant-numeric: tabular-nums;
    white-space: nowrap;
  }

  .latest-event-action {
    display: inline-flex;
    align-items: center;
    gap: 3px;
    color: var(--wa-accent-strong, #006f76);
  }

  .latest-event-action i {
    display: inline-block;
    font-style: normal;
  }

  .workspace-latest-event:hover .latest-event-title,
  .workspace-latest-event:hover .latest-event-action {
    color: var(--wa-accent-strong, #006f76);
  }

  .workspace-latest-event:hover .latest-event-action i {
    transform: translateX(2px);
  }

  .context-status {
    min-height: 24px;
    display: inline-flex;
    align-items: center;
    padding: 0 8px;
    border: 1px solid rgba(216, 135, 0, 0.18);
    border-radius: 999px;
    background: rgba(216, 135, 0, 0.08);
    color: var(--wa-warning, #a36908);
  }

  :global(.workspace-content > .decision-admin),
  :global(.workspace-content > .daily-jira),
  :global(.workspace-content > .demand-dashboard),
  :global(.workspace-content > .kanban-section),
  :global(.workspace-content > .kpi-dashboard),
  :global(.workspace-content > .metrics-workspace),
  :global(.workspace-content > .settings-container),
  :global(.workspace-content > .telemetry-panel),
  :global(.workspace-content > .deconstructor-section),
  :global(.workspace-content > .console-functional-stack) {
    width: 100%;
    min-width: 0;
    max-width: none;
    margin: 0 !important;
    background: transparent;
  }

  :global(.workspace-content > .settings-container) {
    padding-bottom: 0 !important;
  }

  :global(.workspace-content > .console-functional-stack > :last-child) {
    margin-bottom: 0 !important;
  }

  @media (max-width: 700px) {
    .workspace-breadcrumb-bar {
      align-items: flex-start;
      flex-direction: column;
      padding: 10px 12px;
    }

    .workspace-breadcrumb-bar ol {
      width: 100%;
      overflow-x: auto;
    }

    .workspace-breadcrumb-meta {
      width: 100%;
      justify-content: space-between;
    }
  }

  @media (max-width: 1180px) {
    .workspace-breadcrumb-bar.has-latest-event {
      grid-template-columns: minmax(0, 1fr) auto;
      padding-block: 8px;
    }

    .workspace-breadcrumb-bar.has-latest-event .workspace-latest-event {
      grid-column: 1 / -1;
      grid-row: 2;
      width: min(100%, 720px);
      justify-self: center;
    }
  }

  @media (max-width: 700px) {
    .workspace-breadcrumb-bar.has-latest-event {
      display: grid;
      grid-template-columns: 1fr;
      gap: 8px;
    }

    .workspace-breadcrumb-bar.has-latest-event > ol,
    .workspace-breadcrumb-bar.has-latest-event > .workspace-breadcrumb-meta,
    .workspace-breadcrumb-bar.has-latest-event .workspace-latest-event {
      width: 100%;
      justify-self: stretch;
    }

    .workspace-breadcrumb-bar.has-latest-event > ol {
      grid-row: 1;
    }

    .workspace-breadcrumb-bar.has-latest-event .workspace-latest-event {
      grid-row: 2;
      min-height: 44px;
    }

    .workspace-breadcrumb-bar.has-latest-event > .workspace-breadcrumb-meta {
      grid-row: 3;
    }
  }

  @media (max-width: 520px) {
    .workspace-latest-event {
      grid-template-columns: auto auto minmax(0, 1fr) auto;
    }

    .latest-event-kind,
    .workspace-latest-event time {
      display: none;
    }
  }

  @media (prefers-reduced-motion: reduce) {
    .latest-event-action i {
      transform: none !important;
    }
  }

  :global(.workspace-content > .decision-admin),
  :global(.workspace-content > .daily-jira),
  :global(.workspace-content > .demand-dashboard) {
    min-height: 0;
  }

  :global(.workspace-frame.viewport-fit-frame .functional-workspace) {
    height: 100%;
    min-height: 0;
    display: grid;
    grid-template-rows: auto minmax(0, 1fr);
    overflow: hidden;
  }

  :global(.workspace-frame.viewport-fit-frame .workspace-content) {
    height: 100%;
    min-height: 0;
    align-content: stretch;
    overflow: hidden;
  }

  :global(.workspace-frame.viewport-fit-frame .workspace-content > .decision-admin),
  :global(.workspace-frame.viewport-fit-frame .workspace-content > .daily-jira),
  :global(.workspace-frame.viewport-fit-frame .workspace-content > .kanban-section),
  :global(.workspace-frame.viewport-fit-frame .workspace-content > .console-functional-stack) {
    height: 100%;
    min-height: 0;
    overflow: hidden;
  }

  :global(.workspace-frame.viewport-fit-frame .workspace-content > .console-functional-stack > :last-child) {
    height: 100%;
    min-height: 0;
  }

  @media (max-width: 860px) {
    :global(.workspace-frame.viewport-fit-frame .functional-workspace),
    :global(.workspace-frame.viewport-fit-frame .workspace-content),
    :global(.workspace-frame.viewport-fit-frame .workspace-content > .decision-admin),
    :global(.workspace-frame.viewport-fit-frame .workspace-content > .daily-jira),
    :global(.workspace-frame.viewport-fit-frame .workspace-content > .kanban-section),
    :global(.workspace-frame.viewport-fit-frame .workspace-content > .console-functional-stack),
    :global(.workspace-frame.viewport-fit-frame .workspace-content > .console-functional-stack > :last-child) {
      height: auto;
      overflow: visible;
    }
  }
</style>
