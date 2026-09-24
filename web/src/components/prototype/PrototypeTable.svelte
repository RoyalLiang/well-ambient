<script lang="ts">
  import PrototypeBadge from './PrototypeBadge.svelte';

  type RiskTone = 'critical' | 'warning' | 'safe' | 'info';

  interface PrototypeIssue {
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
    blockedBy: string;
  }

  export let rows: PrototypeIssue[] = [];
  export let selectedKey = '';
  export let columns: string[] = [];
  export let ariaLabel = '事项列表';
  export let onSelect: (key: string) => void = () => {};

  function riskLabel(risk: RiskTone): string {
    if (risk === 'critical') return '阻断';
    if (risk === 'warning') return '预警';
    if (risk === 'safe') return '健康';
    return '关注';
  }
</script>

<div class="prototype-table-shell">
  <table class="prototype-table" aria-label={ariaLabel}>
    <thead>
      <tr>
        {#if columns.length}{#each columns as column}<th scope="col">{column}</th>{/each}{:else}
        <th scope="col">事项</th>
        <th scope="col">风险</th>
        <th scope="col">负责人</th>
        <th scope="col">阶段</th>
        <th scope="col">到期</th>
        <th scope="col">证据</th>
        <th scope="col">下一步</th>
        {/if}
      </tr>
    </thead>
    <tbody>
      {#if columns.length}<slot name="rows" />{:else}
      {#each rows as row}
        <tr class:active={row.key === selectedKey}>
          <td class="issue-cell">
            <button
              type="button"
              class="issue-button wa-focus-ring"
              on:click={() => onSelect(row.key)}
              aria-label={`查看 ${row.key} 调停预检`}
            >
              <span>{row.key}</span>
              <strong>{row.title}</strong>
              <small>{row.project} / {row.source}</small>
            </button>
          </td>
          <td>
            <PrototypeBadge tone={row.risk} label={riskLabel(row.risk)} compact />
          </td>
          <td class="person-cell">{row.owner}</td>
          <td>{row.stage}</td>
          <td class="mono-cell">{row.due}</td>
          <td>{row.evidenceHealth}</td>
          <td class="next-cell">{row.nextAction}</td>
        </tr>
      {/each}
      {/if}
    </tbody>
  </table>
</div>

<style>
  .prototype-table-shell {
    min-width: 0;
    overflow: auto;
    border: 1px solid rgba(184, 245, 255, 0.16);
    border-radius: 18px;
    background:
      linear-gradient(180deg, rgba(244, 251, 255, 0.025), rgba(244, 251, 255, 0)),
      var(--wa-surface-inset);
    box-shadow: inset 0 1px 0 rgba(244, 251, 255, 0.045);
  }

  .prototype-table {
    width: 100%;
    min-width: 980px;
    border-collapse: collapse;
    table-layout: fixed;
  }

  th,
  td {
    border-bottom: 1px solid rgba(184, 245, 255, 0.1);
    padding: 11px 12px;
    color: var(--wa-text-main);
    font-size: 12px;
    line-height: 1.35;
    text-align: left;
    vertical-align: middle;
    font-variant-numeric: tabular-nums;
  }

  th {
    position: sticky;
    top: 0;
    z-index: 1;
    background:
      linear-gradient(180deg, rgba(14, 24, 32, 0.98), rgba(9, 15, 21, 0.98));
    color: var(--wa-text-subtle);
    font-size: 11px;
    font-weight: 800;
    text-transform: uppercase;
    box-shadow: inset 0 -1px 0 rgba(184, 245, 255, 0.12);
  }

  th:nth-child(1),
  td:nth-child(1) {
    width: 31%;
  }

  th:nth-child(2),
  td:nth-child(2) {
    width: 8%;
  }

  th:nth-child(3),
  td:nth-child(3),
  th:nth-child(4),
  td:nth-child(4),
  th:nth-child(5),
  td:nth-child(5) {
    width: 10%;
  }

  th:nth-child(6),
  td:nth-child(6) {
    width: 13%;
  }

  th:nth-child(7),
  td:nth-child(7) {
    width: 18%;
  }

  tbody tr {
    transition:
      background var(--wa-duration-fast) var(--wa-ease),
      box-shadow var(--wa-duration-fast) var(--wa-ease);
  }

  tbody tr:hover,
  tbody tr.active {
    background:
      linear-gradient(90deg, rgba(38, 221, 255, 0.075), transparent 48%),
      var(--wa-row-hover);
  }

  tbody tr.active {
    box-shadow: inset 4px 0 0 var(--wa-accent), inset 0 0 0 999px rgba(38, 221, 255, 0.025);
  }

  tbody tr:last-child td {
    border-bottom: 0;
  }

  .issue-cell {
    padding: 0;
  }

  .issue-button {
    width: 100%;
    min-height: 64px;
    padding: 11px 12px 11px 14px;
    border: 0;
    background: transparent;
    color: inherit;
    cursor: pointer;
    display: grid;
    gap: 4px;
    text-align: left;
  }

  .issue-button span,
  .mono-cell {
    color: var(--wa-text-subtle);
    font-family: var(--wa-font-mono);
    font-size: 11px;
  }

  .issue-button strong {
    overflow: hidden;
    color: var(--wa-text-strong);
    font-size: 14px;
    font-weight: 820;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .issue-button small {
    overflow: hidden;
    color: var(--wa-text-muted);
    font-size: 11px;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .person-cell {
    color: var(--wa-text-strong);
    font-weight: 760;
  }

  .next-cell {
    color: #bdd1da;
  }
</style>
