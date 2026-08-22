<script lang="ts">
  import KPIKanban from './KPIKanban.svelte';
  import ProjectHealthTelemetry from './ProjectHealthTelemetry.svelte';
	import PerformanceCalculationGuide from './PerformanceCalculationGuide.svelte';

	export let activeLens: 'health' | 'kpi' | 'performance' = 'health';
</script>

<section class="insights-workspace" aria-label="洞察工作台">
  {#if activeLens === 'health'}
    <ProjectHealthTelemetry />
	{:else if activeLens === 'kpi'}
    <KPIKanban />
	{:else}
		<PerformanceCalculationGuide />
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
