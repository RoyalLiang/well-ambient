<script lang="ts">
  export let title: string = '';
  export let status: 'online' | 'offline' | 'warning' = 'online';
  export let description: string = '';
  export let details: string = '';

  $: statusText = status.toUpperCase();
</script>

<div class="card hover-effect">
  <div class="card-header">
    <h3 class="card-title">{title}</h3>
    <span class="status-badge status-{status}">
      <span class="status-dot dot-{status}"></span>
      {statusText}
    </span>
  </div>
  
  <p class="card-description">{description}</p>
  
  <div class="card-footer">
    {#if details}
      <div class="card-details">
        {details}
      </div>
    {/if}
    <slot name="actions" />
  </div>
</div>

<style>
  .card {
    background: rgba(17, 24, 39, 0.6);
    border: 1px solid rgba(51, 65, 85, 0.4);
    border-radius: 12px;
    padding: 20px;
    backdrop-filter: blur(12px);
    -webkit-backdrop-filter: blur(12px);
    transition: all 0.3s cubic-bezier(0.4, 0, 0.2, 1);
    display: flex;
    flex-direction: column;
    justify-content: space-between;
    box-sizing: border-box;
    height: 100%;
  }

  .hover-effect:hover {
    border-color: rgba(99, 102, 241, 0.5);
    box-shadow: 0 10px 30px -10px rgba(99, 102, 241, 0.15);
    transform: translateY(-2px);
  }

  .card-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
    margin-bottom: 12px;
    gap: 8px;
  }

  .card-title {
    color: #f1f5f9;
    font-size: 1.05rem;
    font-weight: 600;
    margin: 0;
  }

  .status-badge {
    display: inline-flex;
    align-items: center;
    gap: 6px;
    padding: 3px 10px;
    border-radius: 9999px;
    font-size: 0.7rem;
    font-weight: 600;
    border: 1px solid transparent;
    letter-spacing: 0.05em;
  }

  .status-online {
    background: rgba(16, 185, 129, 0.15);
    color: #34d399;
    border-color: rgba(16, 185, 129, 0.2);
  }

  .status-offline {
    background: rgba(239, 68, 68, 0.15);
    color: #f87171;
    border-color: rgba(239, 68, 68, 0.2);
  }

  .status-warning {
    background: rgba(245, 158, 11, 0.15);
    color: #fbbf24;
    border-color: rgba(245, 158, 11, 0.2);
  }

  .status-dot {
    width: 6px;
    height: 6px;
    border-radius: 50%;
    display: inline-block;
  }

  .dot-online {
    background-color: #34d399;
    box-shadow: 0 0 8px #34d399;
  }

  .dot-offline {
    background-color: #f87171;
    box-shadow: 0 0 8px #f87171;
  }

  .dot-warning {
    background-color: #fbbf24;
    box-shadow: 0 0 8px #fbbf24;
  }

  .card-description {
    color: #94a3b8;
    font-size: 0.875rem;
    line-height: 1.5;
    margin: 0 0 16px 0;
    flex-grow: 1;
  }

  .card-footer {
    padding-top: 12px;
    border-top: 1px solid rgba(51, 65, 85, 0.3);
    display: flex;
    justify-content: space-between;
    align-items: center;
    gap: 8px;
    margin-top: auto;
  }

  .card-details {
    font-size: 0.725rem;
    font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
    color: rgba(129, 140, 248, 0.8);
    word-break: break-all;
  }
</style>
