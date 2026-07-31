<script lang="ts">
  import KPIKanban from './KPIKanban.svelte';
  import ProjectHealthTelemetry from './ProjectHealthTelemetry.svelte';

  export let activeLens: 'health' | 'kpi' = 'health';
</script>

<section class="insights-workspace" aria-label="洞察工作台">
  {#if activeLens === 'health'}
    <ProjectHealthTelemetry />
  {:else}
    <KPIKanban />
  {/if}
</section>

<style>
  .insights-workspace {
    width: 100%;
    height: 100%;
    min-height: 0;
    display: grid;
  }
</style>
