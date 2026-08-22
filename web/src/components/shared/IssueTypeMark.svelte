<script lang="ts">
  export let issueType = '';

  $: normalizedIssueType = issueType.trim().toLowerCase();
  $: isBug = ['bug', 'defect', '缺陷', '故障'].includes(normalizedIssueType);
  $: accessibleLabel = isBug ? 'Bug' : 'Task';
</script>

<span
  class="issue-type-mark"
  class:bug={isBug}
  role="img"
  aria-label={accessibleLabel}
  title={accessibleLabel}
>
  <span aria-hidden="true">{isBug ? 'B' : 'T'}</span>
</span>

<style>
  .issue-type-mark {
    width: 22px;
    height: 22px;
    display: inline-grid;
    flex: none;
    place-items: center;
    border: 1px solid rgba(43, 105, 179, 0.24);
    border-radius: 6px;
    background: rgba(43, 105, 179, 0.08);
    color: #2b69b3;
    font: 800 10px/1 var(--wa-font-mono, "SFMono-Regular", Consolas, monospace);
  }

  .issue-type-mark.bug {
    border-color: rgba(201, 71, 60, 0.25);
    background: rgba(201, 71, 60, 0.08);
    color: var(--wa-danger, #c9473c);
  }
</style>
