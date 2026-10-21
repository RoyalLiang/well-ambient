<script lang="ts">
  import Button from '../shared/Button.svelte';
  type Membership = { group_name: string; group_display_name: string; scope: string; scope_id: string };
  type Member = { username: string; name: string; email: string; avatar: string; memberships: Membership[] };
  export let users: Member[] = [];
  export let groups: { name: string; displayName: string }[] = [];
  export let canWrite = false;
  export let canTransfer = false;
  export let currentUserEmail = '';
  export let loading = false;
  export let error = '';
  export let onRefresh: () => void;
  export let onAssign: (username: string) => void;
  export let onRemove: (username: string, membership: Membership) => void;
  export let onTransfer: (username: string) => void;
  let search = '';
  let role = '';
  let page = 1;
  let previousFilter = '';
  let expanded = '';
  $: filterKey = search + '\n' + role;
  $: if (filterKey !== previousFilter) { previousFilter = filterKey; page = 1; expanded = ''; }
  $: filtered = users.filter(user => (!search.trim() || `${user.name} ${user.username} ${user.email}`.toLowerCase().includes(search.trim().toLowerCase())) && (!role || (role === 'unassigned' ? !(user.memberships || []).length : (user.memberships || []).some(m => m.group_name === role))));
  $: pages = Math.max(1, Math.ceil(filtered.length / 25));
  $: if (page > pages) page = pages;
  $: visible = filtered.slice((page - 1) * 25, page * 25);
</script>

<section class="member-directory" aria-label="成员角色目录" aria-busy={loading}>
  <header><div><h2>成员与作用域</h2><p>按成员查找角色，区分全局授权与仓库授权。</p></div><Button variant="secondary" loading={loading} on:click={onRefresh}>刷新成员</Button></header>
  <div class="filters">
    <label><span>查找成员</span><input type="search" bind:value={search} placeholder="姓名、邮箱或账号" /></label>
    <label><span>角色</span><select bind:value={role}><option value="">全部角色</option><option value="unassigned">尚未分配</option>{#each groups as group}<option value={group.name}>{group.displayName}</option>{/each}</select></label>
    <span class="count" role="status">{filtered.length} / {users.length} 位成员 · {canWrite ? '可管理' : '只读'}</span>
  </div>
  {#if error}<div class="error" role="alert">{error}<Button variant="secondary" on:click={onRefresh}>重试</Button></div>{/if}
  {#if loading && !users.length}<p role="status">正在加载成员目录…</p>
  {:else if !visible.length}<div class="empty"><h3>{users.length ? '没有匹配的成员' : '暂无成员'}</h3><p>{users.length ? '调整姓名或角色筛选后重试。' : '完成用户同步后，成员会显示在这里。'}</p></div>
  {:else}
    <div class="table-scroll"><table><thead><tr><th>成员</th><th>角色与作用域</th><th>操作</th></tr></thead><tbody>
      {#each visible as user (user.username)}
        <tr><td><div class="identity"><span class="avatar">{#if user.avatar}<img src={user.avatar} alt="" loading="lazy" />{:else}{(user.name || user.username).slice(0,1)}{/if}</span><div><strong>{user.name || user.username}</strong><small>{user.email || user.username}</small>{#if user.email && user.username !== user.email}<small>{user.username}</small>{/if}</div></div></td>
          <td><div class="memberships">{#each user.memberships || [] as membership}<div class="membership"><div><strong>{membership.group_display_name || membership.group_name}</strong><small>{membership.scope === 'global' ? '全局' : `仓库 · ${membership.scope_id}`}</small></div>{#if canWrite && membership.group_name !== 'super_admin'}<Button size="small" variant="ghost" ariaLabel={`移除 ${user.name || user.username} 的 ${membership.group_display_name} 身份`} on:click={() => onRemove(user.username,membership)}>移除</Button>{/if}</div>{:else}<span class="muted">尚未分配角色</span>{/each}</div></td>
          <td><div class="member-actions">{#if canWrite}<Button size="small" variant="secondary" on:click={() => onAssign(user.username)}>分配角色</Button>{/if}{#if canTransfer && user.username !== currentUserEmail}<button class="more" aria-expanded={expanded === user.username} aria-label={`${user.name || user.username} 的更多操作`} on:click={() => expanded = expanded === user.username ? '' : user.username}>更多</button>{/if}{#if !canWrite && !canTransfer}<span class="muted">只读</span>{/if}</div></td></tr>
        {#if expanded === user.username}<tr><td colspan="3"><div class="sensitive"><p>将超级管理员转让给 {user.name || user.username}，会变更你当前的管理员身份。</p><Button variant="danger" size="small" on:click={() => onTransfer(user.username)}>转让超级管理员</Button><Button variant="ghost" size="small" on:click={() => expanded = ''}>收起</Button></div></td></tr>{/if}
      {/each}
    </tbody></table></div>
    <footer><span>第 {page} / {pages} 页 · 每页 25 位</span><div class="member-actions"><Button variant="secondary" disabled={page <= 1} on:click={() => page--}>上一页</Button><Button variant="secondary" disabled={page >= pages} on:click={() => page++}>下一页</Button></div></footer>
  {/if}
</section>

<style>
  .member-directory{color:var(--wa-text-main);display:grid;gap:var(--wa-space-4,16px);min-width:0}header,footer{display:flex;align-items:center;justify-content:space-between;gap:16px}h2{color:var(--wa-text-strong);font-size:18px;margin:0 0 6px}h3{color:var(--wa-text-strong);font-size:16px;margin:0}p{margin:0;color:var(--wa-text-muted);font-size:13px}.filters{display:grid;grid-template-columns:minmax(180px,1fr) minmax(160px,240px) auto;align-items:end;gap:16px;padding-block:16px;border-block:1px solid var(--wa-border-divider)}label{display:grid;gap:6px;font-size:12px;font-weight:650}input,select{width:100%;min-height:38px;border:1px solid var(--wa-border-panel);border-radius:var(--wa-radius-md);background:var(--wa-surface-panel);color:var(--wa-text-main);padding:8px 12px;font:inherit}.count,.muted,footer{font-size:12px;color:var(--wa-text-muted)}.table-scroll{overflow:auto;max-height:650px}table{width:100%;border-collapse:collapse;text-align:left}th{font-size:12px;color:var(--wa-text-muted);background:var(--wa-surface-inset);position:sticky;top:0;z-index:1}th,td{padding:14px 12px;border-bottom:1px solid var(--wa-border-divider);vertical-align:middle}th:last-child{width:180px}td{font-size:13px}.identity{display:flex;gap:12px;align-items:center}.identity>div{min-width:0}.identity strong{display:block;color:var(--wa-text-strong)}.identity small,.membership small{display:block;color:var(--wa-text-muted);font-size:12px;overflow-wrap:anywhere}.avatar{display:grid;place-items:center;flex:0 0 32px;height:32px;border-radius:50%;background:var(--wa-surface-inset);color:var(--wa-text-main)}.avatar img{width:32px;height:32px;border-radius:50%;object-fit:cover}.memberships{display:flex;flex-wrap:wrap;gap:8px}.membership{display:flex;align-items:center;gap:12px;padding:6px 10px;background:var(--wa-surface-inset);border-radius:var(--wa-radius-md)}.membership strong{font-size:12px;color:var(--wa-text-strong)}.member-actions{display:flex;gap:8px;align-items:center;flex-wrap:wrap}.more{border:0;background:transparent;color:var(--wa-accent-strong);padding:8px;cursor:pointer;font:inherit}.more:hover{background:var(--wa-surface-inset)}:is(input,select,.more):focus-visible{outline:2px solid var(--wa-accent-strong);outline-offset:2px}.sensitive{display:flex;gap:12px;align-items:center;flex-wrap:wrap}.sensitive p{flex:1}.empty{padding:32px 16px;text-align:center;display:grid;gap:8px}.error{display:flex;gap:12px;align-items:center;color:var(--wa-danger)}
  input::placeholder{color:var(--wa-text-muted);opacity:1}input,select{color-scheme:light}.more:active{background:var(--wa-accent-soft)}
  @media(max-width:760px){header,footer{align-items:stretch;flex-direction:column}.filters{grid-template-columns:1fr}.count{padding-block:4px}input,select,.more{min-height:44px}.table-scroll{max-height:none}table,tbody,tr,td{display:block}thead{display:none}tr{padding-block:8px;border-bottom:1px solid var(--wa-border-divider)}td{border:0;padding:8px 0}.identity{align-items:flex-start}.member-actions{width:100%}.sensitive{align-items:stretch;flex-direction:column}.sensitive p{flex:auto}.membership{width:100%;justify-content:space-between}.memberships{display:grid}.table-scroll{overflow:visible}}
</style>
