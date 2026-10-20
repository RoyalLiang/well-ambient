<script lang="ts">
  import { onMount, onDestroy } from 'svelte';
  import Button from '../shared/Button.svelte';
  import Modal from '../shared/Modal.svelte';
  import Select from '../shared/Select.svelte';
  import Switch from '../shared/Switch.svelte';
  import TextInput from '../shared/TextInput.svelte';
  import { copyToClipboard } from '../../lib/clipboard';
  import { showToast } from '../../lib/toast';
  import { filterSkills, publicationDisabledSkills, type OpenSkillItem, type OpenSkillsObservation } from '../../lib/openSkillRegistry';
  import { skillInstallCommand, type SkillInstallTarget } from '../../lib/openCapabilityInstall';

  export let canWrite = false;
  export let canObserve = false;
  export let canManageKnowledge = false;
  export let onOpenCapabilities: () => void = () => {};

  let bundle: OpenSkillItem | null = null;
  let observation: OpenSkillsObservation | null = null;
  let loading = true;
  let refreshing = false;
  let error = '';
  let observationError = '';
  let saveError = '';
  let query = '';
  let savingSkill = '';
  let knowledgeBusy = false;
  let selected: OpenSkillItem | null = null;
  let dialog: 'install' | 'doc' | null = null;
  let target: SkillInstallTarget = 'codex';
  let manualCopy = false;
  let doc = '';
  let docLoading = false;
  let docError = '';
  let controller: AbortController | null = null;
  let documentController: AbortController | null = null;
  let generation = 0;
  let lastLoad = 0;
  let baseURL = '';
  const targets = [
    { value: 'codex', label: 'Codex · ~/.codex/skills' },
    { value: 'claude', label: 'Claude Code · ~/.claude/skills' },
    { value: 'choose', label: '运行时选择安装根目录' },
  ];
  $: metricsByID = new Map((canObserve ? observation?.subskills || [] : []).map((item) => [item.id, item]));
  $: children = bundle?.children || [];
  $: filtered = filterSkills(children, query);
  $: published = children.filter((child) => child.enabled !== false).length;
  $: command = bundle ? skillInstallCommand(baseURL, bundle.name, target) : '';
  $: knowledge = canObserve ? observation?.knowledge : null;
  $: nextRefresh = knowledge?.last_checked_at ? new Date(new Date(knowledge.last_checked_at).getTime() + knowledge.interval_minutes * 60000).toISOString() : '';

  function date(value?: string | null) {
    if (!value) return '暂无记录';
    const parsed = new Date(value);
    return Number.isNaN(parsed.getTime()) ? '时间未知' : parsed.toLocaleString('zh-CN', { hour12: false });
  }
  function rate(value?: number | null) { return value == null ? '—' : `${(value * 100).toFixed(1)}%`; }
  function childTitle(id: string) { return children.find((item) => item.canonical_name === id)?.title || '未归类调用'; }
  function outcome(value: string) { return ({ succeeded: '成功', failed: '失败', started: '待回写' } as Record<string, string>)[value] || '结果未知'; }

  async function load() {
    controller?.abort(); controller = new AbortController();
    const signal = controller.signal; const request = ++generation;
    loading = !bundle; refreshing = !!bundle; error = ''; observationError = '';
    const catalogTask = (async () => {
      try {
        const response = await fetch('/open/v1/skills', { cache: 'no-store', signal });
        if (!response.ok) throw new Error(`技能读取失败（HTTP ${response.status}）`);
        const catalog: OpenSkillItem[] = await response.json();
        const parent = catalog.find((item) => item.name === 'well-ambient' && Array.isArray(item.children));
        if (!parent) throw new Error('服务端尚未提供 well-ambient 主技能包，请部署匹配的服务版本。');
        if (request !== generation) return;
        bundle = parent; baseURL = parent.archive_url.split('/open/v1/skills/')[0] || window.location.origin;
      } catch (cause) { if (!signal.aborted && request === generation) error = cause instanceof Error ? cause.message : '暂时无法读取技能'; }
    })();
    const observationTask = canObserve ? (async () => {
      try {
        const response = await fetch('/api/ai-governance/open-skills/overview', { cache: 'no-store', signal });
        if (!response.ok) throw new Error(response.status === 403 ? '当前身份没有全局调用观测权限。' : `调用观测读取失败（HTTP ${response.status}）`);
        const value: OpenSkillsObservation = await response.json(); if (request === generation) observation = value;
      } catch (cause) { if (!signal.aborted && request === generation) observationError = cause instanceof Error ? cause.message : '暂时无法读取调用观测'; }
    })() : Promise.resolve();
    await Promise.allSettled([catalogTask, observationTask]);
    if (request === generation) { loading = false; refreshing = false; lastLoad = Date.now(); }
  }
  async function setPublished(skill: OpenSkillItem, enabled: boolean) {
    if (!canWrite || savingSkill) return; savingSkill = skill.canonical_name; saveError = '';
    try {
      const current = await fetch('/api/ai-governance/open-capabilities/overview', { cache: 'no-store' });
      if (!current.ok) throw new Error(`配置读取失败（HTTP ${current.status}）`);
      const snapshot = await current.json();
      const disabled = publicationDisabledSkills(snapshot.config.disabled_skills || [], children.map((item) => item.canonical_name), skill.canonical_name, enabled);
      const response = await fetch('/api/ai-governance/open-capabilities/config', {
        method: 'PUT', headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ ...snapshot.config, name_prefix: 'well-ambient', disabled_skills: disabled, expected_version: snapshot.version }),
      });
      if (!response.ok) throw new Error(response.status === 409 ? '配置已被更新，请刷新后重试。' : `发布设置失败（HTTP ${response.status}）`);
      const saved = await response.json(); const excluded: string[] = saved.config.disabled_skills || [];
      if (bundle) {
        const updated = children.map((item) => ({ ...item, enabled: !excluded.includes(item.canonical_name) }));
        bundle = { ...bundle, children: updated, enabled: updated.some((item) => item.enabled), tools: updated.filter((item) => item.enabled).flatMap((item) => item.tools) };
      }
      showToast(`${skill.title} 已${enabled ? '发布' : '停止分发'}`, { type: 'success' }); await load();
    } catch (cause) { saveError = cause instanceof Error ? cause.message : '暂时无法更新发布状态'; }
    finally { savingSkill = ''; }
  }
  function closeDialog() { documentController?.abort(); dialog = null; selected = null; manualCopy = false; }
  function install() { selected = bundle; target = 'codex'; manualCopy = false; dialog = 'install'; }
  async function showDoc(skill: OpenSkillItem) {
    documentController?.abort(); documentController = new AbortController(); const signal = documentController.signal;
    selected = skill; dialog = 'doc'; doc = ''; docError = ''; docLoading = true;
    try {
      const path = skill.parent ? `/open/v1/skills/well-ambient/subskills/${encodeURIComponent(skill.canonical_name)}/skill.md` : '/open/v1/skills/well-ambient/skill.md';
      const response = await fetch(path, { cache: 'no-store', signal }); if (!response.ok) throw new Error(`文档读取失败（HTTP ${response.status}）`);
      const value = await response.text(); if (!signal.aborted) doc = value;
    } catch (cause) { if (!signal.aborted) docError = cause instanceof Error ? cause.message : '暂时无法读取文档'; }
    finally { if (!signal.aborted) docLoading = false; }
  }
  async function copyCommand() {
    try { if (await copyToClipboard(command)) { showToast('安装命令已复制', { type: 'success' }); return; } } catch { /* manual copy remains available */ }
    manualCopy = true;
  }
  async function refreshKnowledge() {
    if (!canManageKnowledge || knowledgeBusy) return; knowledgeBusy = true; saveError = '';
    try {
      const response = await fetch('/api/agent-runtime/knowledge/refresh', { method: 'POST' });
      if (!response.ok) throw new Error(`知识整理失败（HTTP ${response.status}）`);
      showToast('知识快照已整理，正在更新观测', { type: 'success' }); await load();
    } catch (cause) { saveError = cause instanceof Error ? cause.message : '知识整理失败'; }
    finally { knowledgeBusy = false; }
  }
  onMount(() => {
    void load(); const onFocus = () => { if (document.visibilityState === 'visible' && Date.now() - lastLoad > 10000) void load(); };
    window.addEventListener('focus', onFocus); return () => window.removeEventListener('focus', onFocus);
  });
  onDestroy(() => { generation++; controller?.abort(); documentController?.abort(); });
</script>

<section class="skill-center" aria-labelledby="skill-center-title">
  <header class="skill-header">
    <div><h2 id="skill-center-title">Skill 中心</h2><p class="description">管理 well-ambient 主技能、业务子技能与调用观测。</p></div>
    <div class="actions"><Button variant="secondary" disabled={loading || refreshing} loading={refreshing} on:click={load}>刷新观测</Button><Button variant="secondary" on:click={onOpenCapabilities}>连接与权限配置</Button></div>
  </header>
  {#if error}<div class="notice error" role="alert"><p>{error}{bundle ? ' 已保留上次读取的技能列表。' : ''}</p><Button variant="secondary" size="small" on:click={load}>重试</Button></div>{/if}
  {#if saveError}<div class="notice error" role="alert">{saveError}</div>{/if}
  {#if loading}<div class="skill-panel empty" role="status">正在读取 well-ambient 技能包与调用观测…</div>
  {:else if bundle}
    <section class="skill-panel package-panel" aria-label="well-ambient 统一技能包" data-surface-contract="admin-data-list-surface/v1">
      <div class="package-bar">
        <div class="package-title"><h3>well-ambient <span class="version">v{bundle.version || '2.0.0'}</span></h3><p>一个主技能，按需加载业务子技能。</p></div>
        <div class="actions"><Button variant="secondary" size="small" on:click={() => bundle && showDoc(bundle)}>主技能文档</Button>{#if bundle.enabled !== false}<a class="wa-admin-action download" href="/open/v1/skills/well-ambient/archive">下载统一包</a>{/if}<Button variant="primary" disabled={bundle.enabled === false} on:click={install}>安装 well-ambient</Button></div>
      </div>
      <div class="package-facts"><span class="wa-admin-pill" class:muted={bundle.enabled === false}>{bundle.enabled === false ? '暂停分发' : '可安装'}</span><span>{published} / {children.length} 个子技能已发布</span><span>{bundle.tools.length} 个对外工具</span></div>
    <p class="migration">原独立技能已归入 <code>well-ambient</code>。重新安装后加载客户端工具列表；安装器将可识别的旧目录移到 skill-backups，保留原有修改。发布开关仅控制技能分发，工具访问仍由阶段与策略控制。</p>

    {#if canObserve && observation}<div class="metrics"><div><span>近 24 小时调用</span><strong>{observation.summary.calls}</strong><small>按请求去重，按工具归类</small></div><div><span>已回写成功率</span><strong>{rate(observation.summary.success_rate)}</strong><small>{observation.summary.succeeded} 成功 / {observation.summary.failed} 失败</small></div><div><span>待回写终态</span><strong>{observation.summary.in_progress}</strong><small>不能据此判断仍在运行</small></div><div><span>当前发布策略</span><strong>{observation.policy_version ? `v${observation.policy_version}` : '未配置'}</strong><small>每次调用仍逐项校验授权</small></div></div>{/if}
    </section>
    {#if observationError && canObserve}<div class="notice error" role="alert">{observationError}{observation ? ` 保留截至 ${date(observation.generated_at)} 的上次观测。` : ''}</div>{/if}

    <section class="skill-panel catalog-panel" aria-label="业务子技能" data-surface-contract="admin-data-list-surface/v1">
    <div class="section-heading catalog-heading"><div><h3>业务子技能</h3><p>随主技能统一安装，使用时按需读取。</p></div><div class="search"><label class="sr-only" for="skill-center-search">搜索子技能</label><TextInput id="skill-center-search" type="search" bind:value={query} placeholder="搜索用途、子技能或工具" /></div></div>
    {#if filtered.length === 0}<div class="empty"><p>没有匹配的子技能。</p><Button variant="secondary" size="small" on:click={() => query = ''}>清除搜索</Button></div>
    {:else}<table class="skills-table"><thead><tr><th scope="col">子技能与用途</th><th scope="col">工具</th><th scope="col">对外发布</th><th scope="col">近 24 小时</th><th scope="col">操作</th></tr></thead><tbody>
      {#each filtered as skill (skill.canonical_name)}{@const stats = metricsByID.get(skill.canonical_name)}
        <tr class:unpublished={skill.enabled === false}><td><strong>{skill.title}</strong><code class="skill-id">{skill.name}</code><p>{skill.description}</p></td><td><span class="mobile-label">工具</span><details><summary>{skill.tools.length} 个工具</summary><ul class="tool-list">{#each skill.tools as tool}<li><code>{tool}</code></li>{/each}</ul></details></td><td><span class="mobile-label">对外发布</span><div class="publish-control"><Switch checked={skill.enabled !== false} label={`发布 ${skill.title}`} optimistic={false} expandedHitArea saving={savingSkill === skill.canonical_name} disabled={!canWrite || !!savingSkill} on:change={(event) => setPublished(skill, event.detail)} /><span>{skill.enabled === false ? '未发布' : '已发布'}</span></div></td><td><span class="mobile-label">近 24 小时</span>{#if stats}<strong class="number">{stats.calls} 次</strong><small>{stats.calls ? `${stats.failed} 次失败 · ${stats.in_progress} 条待回写` : '暂无调用观测'}</small>{:else}<span class="quiet">{canObserve ? '观测未就绪' : '需要观测权限'}</span>{/if}</td><td><Button variant="secondary" size="small" on:click={() => showDoc(skill)}>说明与观测</Button></td></tr>
      {/each}
    </tbody></table>{/if}

    </section>
    <section class="skill-panel observations" data-surface-contract="admin-data-list-surface/v1" aria-labelledby="skill-observations-title"><div class="section-heading"><div><h3 id="skill-observations-title">调用观测</h3><p>{observation && canObserve ? `按工具归类，直接调用同样计入 · 最近 24 小时 · 截至 ${date(observation.generated_at)}` : '来自 HTTP 与 MCP 的真实审计记录'}</p></div></div>
      {#if !canObserve}<div class="empty">当前为技能浏览模式。全局超级管理员可查看来源、调用结果与知识刷新状态。</div>
      {:else if observation}
        <div class="phase-line"><span>访问阶段</span><span class="wa-admin-pill" class:muted={!observation.phases.Read}>只读 {observation.phases.Read ? '启用' : '关闭'}</span><span class="wa-admin-pill" class:muted={!observation.phases.Prepare}>预备 {observation.phases.Prepare ? '启用' : '关闭'}</span><span class="wa-admin-pill" class:muted={!observation.phases.Execute}>执行 {observation.phases.Execute ? '启用' : '关闭'}</span></div>
        <section class="knowledge" aria-labelledby="knowledge-title"><div class="section-heading"><div><h4 id="knowledge-title">运行时知识整理</h4><p>自动整理已同步的资料；不代表上游刚刚完成采集。</p></div>{#if canManageKnowledge}<Button variant="secondary" size="small" disabled={knowledgeBusy} loading={knowledgeBusy} on:click={refreshKnowledge}>立即整理</Button>{/if}</div>
          {#if knowledge}<dl class="knowledge-values"><div><dt>定时任务</dt><dd>{knowledge.enabled ? `每 ${knowledge.interval_minutes / 60} 小时` : '已停用'}</dd></div><div><dt>最近成功检查</dt><dd>{date(knowledge.last_checked_at)}</dd></div><div><dt>下次到期</dt><dd>{knowledge.enabled && nextRefresh ? date(nextRefresh) : '—'}</dd></div><div><dt>当前快照</dt><dd>{knowledge.generation_id ? `#${knowledge.generation_id} · ${observation.knowledge_generation?.item_count ?? '—'} 个证据段` : '尚未生成'}</dd></div></dl>{#if knowledge.last_error}<p class="inline-error" role="status">最近一次整理失败；保留上次成功快照。请检查服务日志与来源完整性。</p>{/if}
          {:else}<p class="quiet">尚无知识调度记录。服务启动后建立快照，也可由有权限的管理员手动整理。</p>{/if}
          {#if observation.gaps.length}<p class="inline-error">部分知识观测不可用：{observation.gaps.join('、')}</p>{/if}
        </section>
        <div class="section-heading"><div><h4>最近调用</h4><p>最多 {observation.recent_limit} 条；不展示凭证值或请求正文。</p></div></div>
        {#if observation.recent_calls.length === 0}<div class="empty">当前窗口内暂无调用记录。安装或发布技能本身不会生成成功调用。</div>
        {:else}<table class="calls-table"><thead><tr><th scope="col">时间 / 来源</th><th scope="col">子技能与工具</th><th scope="col">结果</th><th scope="col">耗时</th><th scope="col">审计引用</th></tr></thead><tbody>{#each observation.recent_calls as call (call.request_id)}<tr><td><time datetime={call.created_at}>{date(call.created_at)}</time><small>{call.source_id} · {call.transport}</small></td><td><strong>{childTitle(call.subskill)}</strong><code>{call.tool}</code></td><td><span class="wa-admin-pill" class:failed={call.outcome === 'failed'} class:muted={call.outcome !== 'failed' && call.outcome !== 'succeeded'}>{outcome(call.outcome)}</span>{#if call.error_code}<small class="inline-error">{call.error_code}</small>{/if}</td><td>{call.outcome === 'started' ? '待回写' : `${call.duration_ms} ms`}</td><td><code>{call.request_id}</code><small>策略 v{call.policy_version}</small></td></tr>{/each}</tbody></table>{/if}
      {:else if !observationError}<div class="empty" role="status">正在读取调用观测…</div>{/if}
    </section>
  {/if}
</section>

<Modal show={dialog !== null} title={dialog === 'install' ? '安装 well-ambient' : `${selected?.title || 'well-ambient'} · 说明与观测`} size="wide" solid footerVisible={false} on:close={closeDialog}>
  {#if dialog === 'install'}<div class="dialog-content"><p>只安装一个 <code>well-ambient</code> 主技能包，已发布子技能随包提供。</p><Select id="skill-install-target" label="安装到" value={target} options={targets} searchable={false} on:change={(event) => { target = event.detail as SkillInstallTarget; manualCopy = false; }} /><pre class="command"><code>{command}</code></pre><Button variant="primary" on:click={copyCommand}>复制安装命令</Button>{#if manualCopy}<label for="skill-manual-copy">浏览器未允许自动复制，请选中后复制。</label><textarea id="skill-manual-copy" readonly value={command} on:focus={(event) => event.currentTarget.select()}></textarea>{/if}<p class="quiet">首次接入还需在“连接与权限配置”完成 MCP 和凭证配置。命令不包含密钥；可识别的旧目录迁移到 skill-backups，保留修改并可恢复。</p></div>
  {:else if dialog === 'doc' && selected}<div class="dialog-content">
    {#if selected.parent && metricsByID.get(selected.canonical_name)}{@const stats = metricsByID.get(selected.canonical_name)!}<div class="detail-observation"><h3>近 24 小时观测</h3><p>{stats.calls} 次调用 · {stats.failed} 次失败 · 成功率 {rate(stats.success_rate)} · 平均耗时 {stats.average_ms == null ? '暂无完成观测' : `${stats.average_ms.toFixed(0)} ms`}</p><ul>{#each stats.conditions as condition}<li>{condition}</li>{/each}</ul>{#if Object.keys(stats.error_codes).length}<p class="inline-error">失败类型：{Object.entries(stats.error_codes).map(([code, count]) => `${code} × ${count}`).join('、')}</p>{/if}</div>{/if}
    {#if docLoading}<p role="status">正在读取技能文档…</p>{:else if docError}<div class="notice error" role="alert"><p>{docError}</p><Button variant="secondary" on:click={() => selected && showDoc(selected)}>重试文档</Button></div>{:else}<pre class="document">{doc}</pre>{/if}
  </div>{/if}
</Modal>

<style>
  /* finesse · component: skill-center · register=product
   * ownership: package discovery, child publication and observation layout
   * palette, typography and surfaces: inherited Phase 41 admin contract */
  .skill-center { display: flex; flex-direction: column; min-width: 0; gap: var(--wa-space-4); color: var(--wa-text-main); }
  h2, h3, h4, p { margin: 0; }
  h2 { font-size: 18px; font-weight: 700; line-height: 1.5; letter-spacing: -.02em; color: var(--wa-text-strong); }
  h3 { font-size: 15px; font-weight: 700; line-height: 1.5; color: var(--wa-text-strong); }
  h4 { font-size: 13.5px; font-weight: 700; line-height: 1.5; }
  .skill-header, .package-bar, .section-heading { display: flex; align-items: center; justify-content: space-between; gap: var(--wa-space-4); min-width: 0; }
  .skill-header { align-items: flex-start; }
  .skill-header > div:first-child, .package-title, .section-heading > div { min-width: 0; }
  .description { margin-top: var(--wa-space-1); }
  .description, .section-heading p, .package-title p, .migration, .quiet { color: var(--wa-text-muted); font-size: 13px; line-height: 1.65; }
  .section-heading p, .package-title p { margin-top: var(--wa-space-1); }
  .actions { display: flex; align-items: center; gap: var(--wa-space-2); flex-wrap: wrap; flex-shrink: 0; }
  .skill-panel {
    min-width: 0;
    box-sizing: border-box;
    border: 1px solid var(--wa-admin-list-surface-border);
    border-top-color: var(--wa-admin-list-surface-border-top);
    border-radius: var(--wa-admin-list-surface-radius);
    background: var(--wa-admin-list-surface-background);
    box-shadow: var(--wa-admin-list-surface-shadow);
    -webkit-backdrop-filter: var(--wa-admin-list-surface-filter);
    backdrop-filter: var(--wa-admin-list-surface-filter);
  }
  .package-panel, .observations { padding: var(--wa-space-4) var(--wa-space-5); }
  .package-panel { display: flex; flex-direction: column; gap: var(--wa-space-3); }
  .package-title h3 { display: flex; align-items: center; gap: var(--wa-space-2); font-size: 16px; }
  .version { color: var(--wa-text-muted); font-size: 12px; font-weight: 500; white-space: nowrap; letter-spacing: 0; }
  .package-facts { display: flex; align-items: center; flex-wrap: wrap; gap: var(--wa-space-2) var(--wa-space-4); font-size: 12px; color: var(--wa-text-muted); }
  .download { display: inline-flex; align-items: center; justify-content: center; min-height: var(--wa-control-h); padding: 0 var(--wa-space-3); border-radius: var(--wa-radius-md); font-size: 13px; font-weight: 700; text-decoration: none; white-space: nowrap; }
  .migration { padding-top: var(--wa-space-3); border-top: 1px solid var(--wa-border-divider); font-size: 12px; }
  .metrics { display: grid; grid-template-columns: repeat(4, minmax(0, 1fr)); padding-top: var(--wa-space-3); border-top: 1px solid var(--wa-border-divider); }
  .metrics > div { min-width: 0; padding-inline: var(--wa-space-4); border-left: 1px solid var(--wa-border-divider); }
  .metrics > div:first-child { padding-left: 0; border-left: 0; }
  .metrics > div:last-child { padding-right: 0; }
  .metrics span { font-size: 12px; color: var(--wa-text-muted); }
  .metrics strong { display: block; margin-top: var(--wa-space-1); font-size: 22px; line-height: 1.4; font-weight: 700; color: var(--wa-text-strong); }
  .metrics small { margin-top: var(--wa-space-1); font-size: 11px; line-height: 1.5; }
  .catalog-panel { overflow: hidden; }
  .catalog-heading { padding: var(--wa-space-4) var(--wa-space-5); }
  .search { width: min(320px, 45%); }
  .search :global(.input-group) { margin-bottom: 0; }
  code { font-family: var(--wa-font-mono, monospace); font-size: 12px; overflow-wrap: anywhere; }
  textarea { width: 100%; box-sizing: border-box; border: 1px solid var(--wa-border-divider); border-radius: var(--wa-radius-sm); padding: var(--wa-space-2) var(--wa-space-3); background: var(--wa-surface-panel); color: var(--wa-text-main); font: inherit; }
  textarea:focus-visible, a:focus-visible, summary:focus-visible { outline: 2px solid var(--wa-accent-fill); outline-offset: 3px; }
  table { width: 100%; border-collapse: collapse; table-layout: fixed; font-size: 13px; }
  th { padding: var(--wa-space-3); text-align: left; color: var(--wa-text-muted); font-size: 12px; font-weight: 600; background: var(--wa-surface-inset); border-block: 1px solid var(--wa-border-divider); }
  td { padding: var(--wa-space-4) var(--wa-space-3); border-bottom: 1px solid var(--wa-border-divider); vertical-align: top; overflow-wrap: anywhere; }
  th:first-child, td:first-child { padding-left: var(--wa-space-5); }
  th:last-child, td:last-child { padding-right: var(--wa-space-5); }
  tbody tr:last-child td { border-bottom: 0; }
  .skills-table th:first-child { width: 36%; }
  .skills-table th:nth-child(2) { width: 14%; }
  .skills-table th:nth-child(3) { width: 17%; }
  .skills-table th:nth-child(4) { width: 18%; }
  td strong, td code, small { display: block; }
  .skill-id { margin-top: var(--wa-space-1); color: var(--wa-text-muted); }
  td p { margin-top: var(--wa-space-2); line-height: 1.6; color: var(--wa-text-muted); }
  small { color: var(--wa-text-muted); }
  td small { margin-top: var(--wa-space-1); font-size: 11px; line-height: 1.55; }
  .number, .metrics strong { font-variant-numeric: tabular-nums; }
  .publish-control :global(.switch-label) { position: absolute; width: 1px; height: 1px; padding: 0; margin: -1px; overflow: hidden; clip-path: inset(50%); white-space: nowrap; border: 0; }
  .publish-control { display: flex; align-items: center; flex-wrap: wrap; gap: var(--wa-space-2); }
  .unpublished td:first-child strong { color: var(--wa-text-muted); }
  summary { cursor: pointer; padding-block: var(--wa-space-1); }
  .tool-list { list-style: none; padding: 0; margin: var(--wa-space-2) 0 0; display: grid; gap: var(--wa-space-2); }
  .notice { padding: var(--wa-space-3) var(--wa-space-4); border: 1px solid var(--wa-border-divider); border-radius: var(--wa-radius-md); display: flex; align-items: center; justify-content: space-between; gap: var(--wa-space-3); font-size: 13px; line-height: 1.65; }
  .notice.error { background: var(--wa-danger-soft); }
  .inline-error { color: var(--wa-danger); font-size: 13px; line-height: 1.65; }
  .empty { padding: var(--wa-space-6) var(--wa-space-4); text-align: center; color: var(--wa-text-muted); font-size: 13px; line-height: 1.7; }
  .empty :global(button) { margin-top: var(--wa-space-2); }
  .observations { display: flex; flex-direction: column; gap: var(--wa-space-4); }
  .phase-line { display: flex; gap: var(--wa-space-2); align-items: center; flex-wrap: wrap; font-size: 12px; color: var(--wa-text-muted); }
  .wa-admin-pill.muted { background: var(--wa-surface-inset); color: var(--wa-text-muted); }
  .wa-admin-pill.failed { background: var(--wa-danger-soft); color: var(--wa-danger); }
  .knowledge { border-block: 1px solid var(--wa-border-divider); padding-block: var(--wa-space-4); display: grid; gap: var(--wa-space-3); }
  .knowledge-values { display: grid; grid-template-columns: repeat(4, minmax(0, 1fr)); gap: var(--wa-space-4); margin: 0; font-size: 13px; }
  .knowledge-values > div { min-width: 0; }
  dt { color: var(--wa-text-muted); font-size: 12px; margin-bottom: var(--wa-space-1); }
  dd { margin: 0; line-height: 1.6; overflow-wrap: anywhere; font-variant-numeric: tabular-nums; }
  .calls-table th:nth-child(1) { width: 22%; }
  .calls-table th:nth-child(2) { width: 30%; }
  .calls-table th:nth-child(3) { width: 14%; }
  .calls-table th:nth-child(4) { width: 11%; }
  .calls-table th:first-child, .calls-table td:first-child { padding-left: 0; }
  .calls-table th:last-child, .calls-table td:last-child { padding-right: 0; }
  .dialog-content { padding: var(--wa-space-5); box-sizing: border-box; display: grid; gap: var(--wa-space-4); min-width: 0; font-size: 14px; line-height: 1.7; }
  .command, .document { white-space: pre-wrap; overflow-wrap: anywhere; margin: 0; padding: var(--wa-space-4); border: 1px solid var(--wa-border-divider); border-radius: var(--wa-radius-sm); background: var(--wa-surface-inset); font-size: 13px; line-height: 1.85; font-family: var(--wa-font-mono, monospace); }
  .detail-observation { border-bottom: 1px solid var(--wa-border-divider); padding-bottom: var(--wa-space-3); }
  .detail-observation p { margin-top: var(--wa-space-2); }
  .detail-observation ul { padding-left: var(--wa-space-4); color: var(--wa-text-muted); }
  textarea { min-height: 120px; font-family: var(--wa-font-mono, monospace); }
  .mobile-label { display: none; }
  .sr-only { position: absolute; width: 1px; height: 1px; padding: 0; overflow: hidden; clip-path: inset(50%); white-space: nowrap; border: 0; }
  @media (max-width: 1100px) {
    .package-bar { align-items: flex-start; flex-wrap: wrap; }
    .knowledge-values { grid-template-columns: repeat(2, minmax(0, 1fr)); }
  }
  @media (max-width: 760px) {
    .skill-header, .section-heading, .package-bar { align-items: stretch; flex-direction: column; }
    .package-panel, .observations { padding: var(--wa-space-4); }
    .catalog-heading { padding: var(--wa-space-4); }
    .search { width: 100%; }
    .actions { gap: var(--wa-space-2); }
    .actions :global(button), .actions .download { flex: 1 1 auto; min-height: 44px; }
    .search :global(input) { min-height: 44px; }
    .metrics { grid-template-columns: repeat(2, minmax(0, 1fr)); gap: var(--wa-space-4) 0; }
    .metrics > div:nth-child(3) { border-left: 0; padding-left: 0; }
    .skills-table, .calls-table, tbody, tr, td { display: block; width: auto; }
    thead { display: none; }
    tr { padding: var(--wa-space-3) var(--wa-space-4); border-bottom: 1px solid var(--wa-border-divider); }
    tr:last-child { border-bottom: 0; }
    td, th:first-child, td:first-child, td:last-child { border: 0; padding: var(--wa-space-2) 0; }
    .skills-table td:nth-child(2), .skills-table td:nth-child(3), .skills-table td:nth-child(4) { display: grid; grid-template-columns: 90px minmax(0, 1fr); align-items: start; gap: var(--wa-space-2); }
    .mobile-label { display: inline; color: var(--wa-text-muted); font-size: 12px; }
    .skills-table td small { grid-column: 2; margin-top: 0; }
    .skills-table td:last-child :global(button) { min-height: 44px; width: 100%; }
    .calls-table tr { padding-inline: 0; }
    .calls-table td { padding-block: var(--wa-space-1); }
    .calls-table td:nth-child(3), .calls-table td:nth-child(4) { display: inline-block; margin-right: var(--wa-space-3); }
    .dialog-content { padding: var(--wa-space-4); font-size: 13px; }
    .knowledge-values { gap: var(--wa-space-3); }
    .notice { align-items: flex-start; flex-wrap: wrap; }
  }
  @media (prefers-reduced-transparency: reduce) {
    .skill-panel { background: var(--wa-surface-panel); -webkit-backdrop-filter: none; backdrop-filter: none; }
  }
  @supports not (backdrop-filter: blur(1px)) {
    .skill-panel { background: var(--wa-surface-panel); }
  }
</style>
