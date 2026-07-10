<script lang="ts">
  interface PrototypeLane {
    id: string;
    label: string;
    description: string;
    count: number;
    mark: string;
  }

  export let lanes: PrototypeLane[] = [];
  export let activeLane = '';
  export let onLaneSelect: (id: string) => void = () => {};
</script>

<section class="prototype-shell wa-grain" aria-label="well-ambient 管理台原型">
  <div class="ambient-plane" aria-hidden="true"></div>

  <aside class="prototype-rail wa-glass">
    <div class="brand-lockup">
      <div class="brand-mark" aria-hidden="true">WA</div>
      <div>
        <span>Well Ambient</span>
        <strong>Brain Console</strong>
      </div>
    </div>

    <nav class="lane-nav" aria-label="原型工作区">
      {#each lanes as lane}
        <button
          type="button"
          class="lane-button wa-focus-ring"
          class:active={activeLane === lane.id}
          on:click={() => onLaneSelect(lane.id)}
          aria-current={activeLane === lane.id ? 'page' : undefined}
        >
          <span class="lane-mark" aria-hidden="true">{lane.mark}</span>
          <span class="lane-copy">
            <strong>{lane.label}</strong>
            <small>{lane.description}</small>
          </span>
          <em>{lane.count}</em>
        </button>
      {/each}
    </nav>

    <div class="rail-footer">
      <span>Prototype v3</span>
      <strong>Beauty is justice</strong>
    </div>
  </aside>

  <main class="prototype-main">
    <header class="prototype-header wa-glass">
      <slot name="header" />
    </header>

    <section class="prototype-toolbar wa-glass" aria-label="命令栏">
      <slot name="toolbar" />
    </section>

    <slot />
  </main>
</section>

<style>
  .prototype-shell {
    position: relative;
    min-height: 780px;
    display: grid;
    grid-template-columns: var(--wa-rail-w) minmax(0, 1fr);
    gap: 14px;
    padding: 6px;
    color: var(--wa-text-main);
    font-family: var(--wa-font-sans);
    isolation: isolate;
  }

  .ambient-plane {
    position: absolute;
    inset: -32px;
    z-index: -2;
    pointer-events: none;
    background:
      radial-gradient(circle at 12% 8%, rgba(38, 221, 255, 0.16), transparent 28%),
      radial-gradient(circle at 86% 6%, rgba(255, 97, 119, 0.09), transparent 26%),
      radial-gradient(circle at 46% 92%, rgba(114, 230, 180, 0.08), transparent 24%),
      linear-gradient(180deg, #06090d, #05080b 66%, #070b10);
  }

  .ambient-plane::after {
    content: "";
    position: absolute;
    inset: 0;
    background:
      linear-gradient(90deg, transparent 0 24%, rgba(184, 245, 255, 0.045) 24% 24.1%, transparent 24.1% 100%),
      repeating-linear-gradient(90deg, rgba(184, 245, 255, 0.035) 0 1px, transparent 1px 88px);
    mask-image: radial-gradient(circle at 52% 30%, rgba(0, 0, 0, 0.8), transparent 76%);
  }

  .prototype-rail {
    min-width: 0;
    border-radius: 22px;
    padding: 16px;
    display: grid;
    grid-template-rows: auto 1fr auto;
    gap: 20px;
    background:
      linear-gradient(180deg, rgba(244, 251, 255, 0.06), rgba(244, 251, 255, 0.018)),
      rgba(6, 12, 17, 0.84);
  }

  .brand-lockup {
    display: flex;
    align-items: center;
    gap: 12px;
    min-height: 52px;
  }

  .brand-mark {
    width: 46px;
    height: 46px;
    border: 1px solid rgba(184, 245, 255, 0.32);
    border-radius: 14px;
    display: grid;
    place-items: center;
    background:
      radial-gradient(circle at 30% 18%, rgba(184, 245, 255, 0.24), transparent 34%),
      var(--wa-surface-inset);
    color: var(--wa-accent-strong);
    font-family: var(--wa-font-mono);
    font-size: 13px;
    font-weight: 800;
    box-shadow: inset 0 1px 0 rgba(244, 251, 255, 0.12), 0 16px 44px rgba(38, 221, 255, 0.11);
  }

  .brand-lockup span,
  .rail-footer span {
    display: block;
    color: var(--wa-text-subtle);
    font-size: 11px;
    font-weight: 700;
    line-height: 1.2;
    text-transform: uppercase;
  }

  .brand-lockup strong,
  .rail-footer strong {
    display: block;
    margin-top: 4px;
    color: var(--wa-text-strong);
    font-size: 15px;
    line-height: 1.2;
  }

  .lane-nav {
    display: grid;
    align-content: start;
    gap: 8px;
  }

  .lane-button {
    width: 100%;
    min-height: 72px;
    display: grid;
    grid-template-columns: 32px minmax(0, 1fr) auto;
    align-items: center;
    gap: 10px;
    padding: 10px;
    border: 1px solid transparent;
    border-radius: 16px;
    background: transparent;
    color: inherit;
    cursor: pointer;
    text-align: left;
    transition:
      background var(--wa-duration-fast) var(--wa-ease),
      border-color var(--wa-duration-fast) var(--wa-ease),
      transform var(--wa-duration-fast) var(--wa-ease);
  }

  .lane-button:hover,
  .lane-button.active {
    border-color: rgba(184, 245, 255, 0.2);
    background: rgba(244, 251, 255, 0.055);
  }

  .lane-button.active {
    background:
      linear-gradient(135deg, rgba(38, 221, 255, 0.13), rgba(244, 251, 255, 0.035)),
      var(--wa-row-active);
    box-shadow: inset 3px 0 0 var(--wa-accent), 0 14px 38px rgba(38, 221, 255, 0.08);
  }

  .lane-mark {
    width: 34px;
    height: 34px;
    border: 1px solid rgba(184, 245, 255, 0.18);
    border-radius: 12px;
    display: grid;
    place-items: center;
    background: rgba(244, 251, 255, 0.055);
    color: var(--wa-accent-strong);
    font-family: var(--wa-font-mono);
    font-size: 12px;
    font-weight: 800;
  }

  .lane-copy {
    min-width: 0;
  }

  .lane-copy strong {
    display: block;
    overflow: hidden;
    color: var(--wa-text-strong);
    font-size: 13px;
    line-height: 1.25;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .lane-copy small {
    display: block;
    margin-top: 4px;
    overflow: hidden;
    color: var(--wa-text-muted);
    font-size: 11px;
    line-height: 1.3;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .lane-button em {
    color: var(--wa-text-subtle);
    font-family: var(--wa-font-mono);
    font-size: 11px;
    font-style: normal;
  }

  .rail-footer {
    border-top: 1px solid var(--wa-border-soft);
    padding-top: 14px;
  }

  .prototype-main {
    min-width: 0;
    display: grid;
    grid-template-rows: auto auto auto minmax(0, 1fr);
    gap: 14px;
  }

  .prototype-header,
  .prototype-toolbar {
    min-width: 0;
    border-radius: 22px;
  }

  .prototype-header {
    min-height: 104px;
    padding: 20px;
  }

  .prototype-toolbar {
    min-height: 68px;
    padding: 12px;
  }

  @media (max-width: 1120px) {
    .prototype-shell {
      grid-template-columns: 1fr;
    }

    .prototype-rail {
      grid-template-rows: auto auto;
    }

    .lane-nav {
      grid-template-columns: repeat(2, minmax(0, 1fr));
    }

    .rail-footer {
      display: none;
    }
  }

  @media (max-width: 720px) {
    .prototype-shell {
      min-height: auto;
    }

    .lane-nav {
      grid-template-columns: 1fr;
    }
  }
</style>
