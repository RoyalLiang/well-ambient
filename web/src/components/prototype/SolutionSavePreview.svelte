<script lang="ts">
  import { onDestroy } from 'svelte';
  import SolutionWorkspace from '../SolutionWorkspace.svelte';
  import ToastHost from '../shared/ToastHost.svelte';

  const outcome = new URLSearchParams(window.location.search).get('outcome') || 'success';
  const demand = { task_id: 'PREVIEW-1', title: '方案保存稳定性验证' };
  const permissions = ['solution:read', 'solution:write', 'solution:publish'];
  const originalFetch = window.fetch.bind(window);
  let revision = 1;
  let working = {
    id: 1,
    version: 1,
    status: 'draft',
    kind: 'human_draft',
    markdown: '# 验证方案\n\n' + Array.from({ length: 120 }, (_, index) => `${index + 1}. 保存时编辑器不应跳动。`).join('\n'),
    content_hash: 'preview-hash-1',
    content_bytes: 3200,
    stored_bytes: 3200,
    content_encoding: 'identity',
    authored_by: 'preview-user',
    created_at: '2026-08-26T07:00:00Z'
  };

  function workspace() {
    return {
      asset: { id: 1, demand_id: demand.task_id, revision },
      working,
      published: undefined,
      candidates: [],
      history: [working],
      sources: [],
      jobs: []
    };
  }

  window.fetch = async (input: RequestInfo | URL, init?: RequestInit) => {
    const path = typeof input === 'string' ? input : input instanceof URL ? input.pathname + input.search : input.url;
    if (path.startsWith('/api/solutions/workspace')) {
      return Response.json({ exists: true, workspace: workspace() });
    }
    if (path === '/api/solutions/draft' && init?.method === 'POST') {
      if (outcome === 'error') return new Response('模拟保存失败', { status: 500 });
      if (outcome === 'conflict') return new Response('模拟版本冲突', { status: 409 });
      const body = JSON.parse(String(init.body || '{}'));
      revision += 1;
      working = {
        ...working,
        id: working.id + 1,
        version: working.version + 1,
        markdown: body.markdown,
        content_hash: `preview-hash-${revision}`,
        content_bytes: body.markdown.length,
        stored_bytes: body.markdown.length,
        created_at: '2026-08-26T07:30:00Z'
      };
      return Response.json({ workspace: workspace() });
    }
    return originalFetch(input, init);
  };

  onDestroy(() => {
    window.fetch = originalFetch;
  });
</script>

<main class="solution-save-preview">
  <header>
    <strong>方案保存反馈验证</strong>
    <span>结果：{outcome}</span>
  </header>
  <section class="preview-workspace">
    <SolutionWorkspace {demand} currentUserPermissions={permissions} />
  </section>
</main>
<ToastHost />

<style>
  :global(body) { margin:0; min-width:320px; background:var(--wa-workspace-bg,#eef3f5); }
  .solution-save-preview { min-height:100dvh; padding:24px; box-sizing:border-box; font-family:var(--wa-font-sans,system-ui); }
  .solution-save-preview>header { height:48px; display:flex; align-items:center; justify-content:space-between; color:var(--wa-text-main,#293847); }
  .solution-save-preview>header span { color:var(--wa-text-muted,#667789); font-size:12px; }
  .preview-workspace { min-height:640px; display:flex; padding:18px; border-radius:12px; background:var(--wa-surface,#fbfdff); }
  @media (max-width:760px) {
    .solution-save-preview { padding:8px; }
    .preview-workspace { min-height:560px; padding:8px; }
  }
</style>
