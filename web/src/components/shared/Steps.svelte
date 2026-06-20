<script lang="ts">
  export let currentStep = 1;
  export let steps: string[] = [];
</script>

<div class="steps-container">
  {#each steps as step, index}
    {@const stepNum = index + 1}
    {@const isCompleted = stepNum < currentStep}
    {@const isActive = stepNum === currentStep}
    
    <div class="step-item" class:active={isActive} class:completed={isCompleted}>
      <div class="step-indicator">
        {#if isCompleted}
          <svg xmlns="http://www.w3.org/2000/svg" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="3" stroke-linecap="round" stroke-linejoin="round"><polyline points="20 6 9 17 4 12"></polyline></svg>
        {:else}
          {stepNum}
        {/if}
      </div>
      <span class="step-label">{step}</span>
      
      {#if index < steps.length - 1}
        <div class="step-line" class:completed-line={stepNum < currentStep}></div>
      {/if}
    </div>
  {/each}
</div>

<style>
  .steps-container {
    display: flex;
    align-items: center;
    justify-content: space-between;
    width: 100%;
    margin-bottom: 28px;
    position: relative;
    padding: 0 4px;
    box-sizing: border-box;
  }

  .step-item {
    display: flex;
    align-items: center;
    position: relative;
    flex: 1;
  }

  .step-item:last-child {
    flex: 0 0 auto;
  }

  .step-indicator {
    width: 32px;
    height: 32px;
    border-radius: 50%;
    background: #1e293b;
    border: 2px solid #334155;
    color: #94a3b8;
    display: flex;
    align-items: center;
    justify-content: center;
    font-size: 0.85rem;
    font-weight: 700;
    transition: all 0.3s cubic-bezier(0.4, 0, 0.2, 1);
    z-index: 2;
    flex-shrink: 0;
  }

  .step-label {
    margin-left: 12px;
    font-size: 0.85rem;
    font-weight: 600;
    color: #64748b;
    transition: all 0.3s ease;
    white-space: nowrap;
    z-index: 2;
    background: #0f172a;
    padding-right: 12px;
  }

  .step-line {
    position: absolute;
    left: 32px;
    right: 12px;
    top: 50%;
    height: 2px;
    background: #334155;
    transform: translateY(-50%);
    z-index: 1;
    transition: all 0.4s cubic-bezier(0.4, 0, 0.2, 1);
  }

  /* Active State */
  .step-item.active .step-indicator {
    background: rgba(99, 102, 241, 0.15);
    border-color: #6366f1;
    color: #818cf8;
    box-shadow: 0 0 12px rgba(99, 102, 241, 0.3);
  }

  .step-item.active .step-label {
    color: #f1f5f9;
  }

  /* Completed State */
  .step-item.completed .step-indicator {
    background: #4f46e5;
    border-color: #4f46e5;
    color: #ffffff;
  }

  .step-item.completed .step-label {
    color: #cbd5e1;
  }

  .step-item .step-line.completed-line {
    background: linear-gradient(90deg, #4f46e5 0%, #6366f1 100%);
  }

  @media (max-width: 520px) {
    .step-label {
      display: none;
    }
    .step-line {
      right: 0;
    }
  }
</style>
