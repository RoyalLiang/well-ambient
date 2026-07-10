<script lang="ts">
  type BadgeTone = 'critical' | 'warning' | 'safe' | 'info' | 'neutral';

  export let label = '';
  export let tone: BadgeTone = 'neutral';
  export let compact = false;
</script>

<span class="prototype-badge tone-{tone}" class:compact>
  <span class="badge-dot" aria-hidden="true"></span>
  {label}
  <slot />
</span>

<style>
  .prototype-badge {
    display: inline-flex;
    align-items: center;
    gap: 6px;
    min-height: 24px;
    width: fit-content;
    max-width: 100%;
    padding: 0 9px;
    border: 1px solid rgba(184, 245, 255, 0.16);
    border-radius: 999px;
    background:
      linear-gradient(180deg, rgba(244, 251, 255, 0.07), rgba(244, 251, 255, 0.025)),
      var(--wa-neutral-soft);
    color: var(--wa-text-main);
    font-size: 12px;
    font-weight: 700;
    line-height: 1;
    white-space: nowrap;
    box-shadow: inset 0 1px 0 rgba(244, 251, 255, 0.065);
  }

  .prototype-badge.compact {
    min-height: 20px;
    padding: 0 7px;
    font-size: 11px;
  }

  .badge-dot {
    width: 6px;
    height: 6px;
    border-radius: 50%;
    background: var(--wa-text-subtle);
  }

  .tone-critical {
    border-color: rgba(255, 97, 119, 0.42);
    background: var(--wa-danger-soft);
    color: #ffe0e5;
  }

  .tone-critical .badge-dot {
    background: var(--wa-danger);
    box-shadow: 0 0 14px rgba(255, 97, 119, 0.5);
  }

  .tone-warning {
    border-color: rgba(255, 197, 95, 0.4);
    background: var(--wa-warning-soft);
    color: #ffe7b6;
  }

  .tone-warning .badge-dot {
    background: var(--wa-warning);
    box-shadow: 0 0 14px rgba(255, 197, 95, 0.46);
  }

  .tone-safe {
    border-color: rgba(114, 230, 180, 0.36);
    background: var(--wa-success-soft);
    color: #d1fae5;
  }

  .tone-safe .badge-dot {
    background: var(--wa-success);
    box-shadow: 0 0 14px rgba(114, 230, 180, 0.42);
  }

  .tone-info {
    border-color: rgba(38, 221, 255, 0.34);
    background: var(--wa-info-soft);
    color: #d9f6ff;
  }

  .tone-info .badge-dot {
    background: var(--wa-accent);
    box-shadow: 0 0 14px rgba(38, 221, 255, 0.44);
  }

  .tone-neutral .badge-dot {
    background: var(--wa-text-muted);
  }
</style>
