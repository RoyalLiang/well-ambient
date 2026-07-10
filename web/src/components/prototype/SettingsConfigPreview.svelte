<script lang="ts">
  import SettingsPanel from '../SettingsPanel.svelte';

  type PreviewSection = 'gitlab' | 'feishu' | 'jira' | 'projects' | 'ai' | 'ai_context';

  const pages: Array<{ id: PreviewSection; label: string }> = [
    { id: 'gitlab', label: 'GitLab 仓库' },
    { id: 'feishu', label: '飞书消息同步' },
    { id: 'jira', label: 'Jira 服务关联' },
    { id: 'projects', label: '项目优先级' },
    { id: 'ai', label: 'AI 引擎配置' },
    { id: 'ai_context', label: '系统设计语料库' }
  ];

  const previewPermissions = [
    'config:read',
    'config:write',
    'ai_context:read',
    'ai_context:write',
    'kpi:read',
    'users:read',
    'users:write',
    'policies:read',
    'policies:write',
    'authorization_audit:read'
  ];

  let activeSection: PreviewSection = 'gitlab';

  function selectSection(section: PreviewSection) {
    activeSection = section;
  }

  function handleSectionChange(section: string) {
    if (pages.some((page) => page.id === section)) {
      activeSection = section as PreviewSection;
    }
  }
</script>

<main class="preview-page">
  <header class="preview-topbar">
    <div>
      <span>well-ambient / 配置中心</span>
      <h1>配置自适应验收</h1>
    </div>
    <strong>Phase 50</strong>
  </header>

  <nav class="preview-tabs" aria-label="配置页面切换">
    {#each pages as page}
      <button
        type="button"
        class:active={activeSection === page.id}
        on:click={() => selectSection(page.id)}
      >
        {page.label}
      </button>
    {/each}
  </nav>

  <section class="preview-settings-host">
    {#key activeSection}
      <SettingsPanel
        currentUserEmail="preview@well-ambient.local"
        currentUserPermissions={previewPermissions}
        activeSettingsSection={activeSection}
        onSectionChange={handleSectionChange}
      />
    {/key}
  </section>
</main>

<style>
  :global(*) {
    box-sizing: border-box;
  }

  :global(html),
  :global(body) {
    margin: 0;
    min-width: 320px;
    min-height: 100%;
    background: #edf4f7;
    color: var(--wa-text-main, #293847);
    font-family: var(--wa-font-sans, -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif);
  }

  .preview-page {
    width: min(1760px, 100%);
    min-height: 100vh;
    margin: 0 auto;
    padding: 14px 18px 32px;
  }

  .preview-topbar {
    min-height: 58px;
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 16px;
    border-bottom: 1px solid rgba(106, 126, 145, 0.16);
  }

  .preview-topbar span {
    color: var(--wa-text-muted, #667789);
    font-size: 11px;
    font-weight: 700;
  }

  .preview-topbar h1 {
    margin: 3px 0 0;
    color: var(--wa-text-strong, #0d1722);
    font-size: 20px;
    letter-spacing: 0;
  }

  .preview-topbar strong {
    color: var(--wa-accent-strong, #006f76);
    font-size: 12px;
  }

  .preview-tabs {
    display: flex;
    gap: 4px;
    overflow-x: auto;
    padding: 10px 0 12px;
  }

  .preview-tabs button {
    min-height: 36px;
    flex: 0 0 auto;
    border: 1px solid rgba(106, 126, 145, 0.16);
    border-radius: 7px;
    background: rgba(251, 253, 254, 0.78);
    color: var(--wa-text-muted, #667789);
    padding: 0 12px;
    font: inherit;
    font-size: 12px;
    font-weight: 740;
    cursor: pointer;
  }

  .preview-tabs button:hover,
  .preview-tabs button.active {
    border-color: rgba(0, 143, 150, 0.24);
    background: rgba(0, 143, 150, 0.1);
    color: var(--wa-accent-strong, #006f76);
  }

  .preview-settings-host {
    min-width: 0;
  }

  .preview-settings-host :global(.settings-container) {
    max-width: none;
    min-height: 0;
  }

  @media (max-width: 700px) {
    .preview-page {
      padding: 10px;
    }

    .preview-tabs button {
      min-height: 44px;
    }
  }
</style>
