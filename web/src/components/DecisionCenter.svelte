<script lang="ts">
  import DailyJiraAudit from './DailyJiraAudit.svelte';
  import DecisionDashboard from './DecisionDashboard.svelte';

  export let activeView: 'agenda' | 'daily_jira' = 'agenda';
  export let currentUser = '';
  export let currentUserPermissions: string[] = [];
</script>

<section class="decision-center" aria-label="决策中心">
  {#if activeView === 'agenda'}
    <DecisionDashboard {currentUser} />
  {:else}
    <DailyJiraAudit {currentUserPermissions} />
  {/if}
</section>

<style>
  .decision-center {
    width: 100%;
    height: 100%;
    min-height: 0;
    display: grid;
  }
</style>
