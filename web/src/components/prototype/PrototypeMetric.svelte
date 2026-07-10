<script lang="ts">
  type MetricTone = 'critical' | 'warning' | 'safe' | 'info' | 'neutral';

  export let label = '';
  export let value = '';
  export let helper = '';
  export let tone: MetricTone = 'neutral';
  export let meta = '';
</script>

<article class="prototype-metric tone-{tone}">
  <div class="metric-label-row">
    <span>{label}</span>
    {#if meta}
      <em>{meta}</em>
    {/if}
  </div>
  <strong>{value}</strong>
  <small>{helper}</small>
</article>

<style>
  .prototype-metric {
    position: relative;
    min-width: 0;
    min-height: 116px;
    overflow: hidden;
    padding: 15px;
    border: 1px solid rgba(184, 245, 255, 0.13);
    border-radius: 16px;
    background:
      linear-gradient(180deg, rgba(244, 251, 255, 0.058), rgba(244, 251, 255, 0.018)),
      rgba(5, 12, 17, 0.54);
    display: grid;
    align-content: space-between;
    gap: 10px;
    box-shadow: inset 0 1px 0 rgba(244, 251, 255, 0.055);
    transition:
      transform var(--wa-duration-fast) var(--wa-ease),
      border-color var(--wa-duration-fast) var(--wa-ease),
      background var(--wa-duration-fast) var(--wa-ease);
  }

  .prototype-metric::after {
    content: "";
    position: absolute;
    inset: auto 12px 10px auto;
    width: 58px;
    height: 58px;
    border: 1px solid rgba(184, 245, 255, 0.09);
    border-radius: 50%;
    background: radial-gradient(circle at center, rgba(38, 221, 255, 0.1), transparent 62%);
    pointer-events: none;
  }

  .prototype-metric:hover {
    transform: translateY(-2px);
    border-color: rgba(184, 245, 255, 0.24);
  }

  .metric-label-row {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 10px;
    min-width: 0;
  }

  .metric-label-row span,
  .prototype-metric small {
    min-width: 0;
    color: var(--wa-text-muted);
    font-size: 12px;
    line-height: 1.35;
  }

  .metric-label-row em {
    flex: 0 0 auto;
    color: var(--wa-text-subtle);
    font-family: var(--wa-font-mono);
    font-size: 11px;
    font-style: normal;
  }

  .prototype-metric strong {
    color: var(--wa-text-strong);
    font-family: var(--wa-font-mono);
    font-size: 34px;
    font-weight: 900;
    letter-spacing: -0.055em;
    line-height: 1;
  }

  .tone-critical {
    border-color: rgba(255, 97, 119, 0.34);
    background:
      radial-gradient(circle at 90% 0, rgba(255, 97, 119, 0.16), transparent 44%),
      rgba(255, 97, 119, 0.07);
  }

  .tone-warning {
    border-color: rgba(255, 197, 95, 0.3);
    background:
      radial-gradient(circle at 90% 0, rgba(255, 197, 95, 0.13), transparent 44%),
      rgba(255, 197, 95, 0.06);
  }

  .tone-safe {
    border-color: rgba(114, 230, 180, 0.26);
    background:
      radial-gradient(circle at 90% 0, rgba(114, 230, 180, 0.12), transparent 44%),
      rgba(114, 230, 180, 0.055);
  }

  .tone-info {
    border-color: rgba(38, 221, 255, 0.28);
    background:
      radial-gradient(circle at 90% 0, rgba(38, 221, 255, 0.14), transparent 44%),
      rgba(38, 221, 255, 0.06);
  }
</style>
