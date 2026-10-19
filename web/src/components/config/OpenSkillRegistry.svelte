<script lang="ts">
  import OpenSkillCenter from './OpenSkillCenter.svelte';
  export let canWrite = false;
  export let canObserve = false;
  export let canManageKnowledge = false;
  export let onOpenCapabilities: () => void = () => {};
</script>

<OpenSkillCenter {canWrite} {canObserve} {canManageKnowledge} {onOpenCapabilities} />
