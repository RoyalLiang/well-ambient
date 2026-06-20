<script lang="ts">
  export let currentUserName: string = '';
  export let currentUserEmail: string = '';
  export let currentUserAvatar: string = '';
  export let currentUserDepartment: string = '';
  export let currentUserRole: string = 'member';
  export let currentUserPermissions: string[] = [];
  export let onRefresh: () => Promise<void> = async () => {};
  export let compact: boolean = false;

  let refreshing = false;

  function getInitial() {
    const source = currentUserName || currentUserEmail || 'U';
    return source.charAt(0).toUpperCase();
  }

  function roleLabel(role: string) {
    if (role === 'super_admin') return '超级管理员';
    if (role === 'admin') return '系统管理员';
    return '成员';
  }

  async function handleRefresh() {
    refreshing = true;
    try {
      await onRefresh();
    } finally {
      refreshing = false;
    }
  }
</script>

<section class="profile-page {compact ? 'compact' : ''}">
  <div class="profile-header">
    <div>
      <span class="profile-eyebrow font-mono">OS PROFILE SNAPSHOT</span>
      <h2>个人信息</h2>
    </div>
    <button class="refresh-profile-btn font-mono" on:click={handleRefresh} disabled={refreshing}>
      {refreshing ? '同步中...' : '刷新资料'}
    </button>
  </div>

  <div class="profile-shell">
    <div class="profile-identity">
      <div class="profile-avatar">
        {#if currentUserAvatar}
          <img src={currentUserAvatar} alt="用户头像" />
        {:else}
          <span>{getInitial()}</span>
        {/if}
      </div>
      <div class="profile-title-block">
        <h3>{currentUserName || '未同步姓名'}</h3>
        <p class="font-mono">{currentUserEmail || '未同步账号'}</p>
      </div>
    </div>

    <div class="profile-grid">
      <div class="profile-field">
        <span class="field-label font-mono">姓名</span>
        <strong>{currentUserName || '未同步'}</strong>
      </div>
      <div class="profile-field">
        <span class="field-label font-mono">账号</span>
        <strong>{currentUserEmail || '未同步'}</strong>
      </div>
      <div class="profile-field department-field {currentUserDepartment ? '' : 'missing'}">
        <span class="field-label font-mono">部门</span>
        <strong>{currentUserDepartment || '未从 OS 同步'}</strong>
      </div>
      <div class="profile-field">
        <span class="field-label font-mono">角色</span>
        <strong>{roleLabel(currentUserRole)}</strong>
      </div>
      <div class="profile-field wide">
        <span class="field-label font-mono">权限数量</span>
        <strong>{currentUserPermissions.length}</strong>
      </div>
    </div>
  </div>
</section>

<style>
  .profile-page {
    display: flex;
    flex-direction: column;
    gap: 22px;
    color: #e2e8f0;
  }

  .profile-header {
    display: flex;
    align-items: flex-end;
    justify-content: space-between;
    gap: 16px;
    flex-wrap: wrap;
  }

  .profile-eyebrow {
    color: #64748b;
    font-size: 0.65rem;
    font-weight: 800;
    letter-spacing: 0.08em;
  }

  .profile-header h2 {
    margin: 6px 0 0;
    color: #f8fafc;
    font-size: 1.5rem;
    line-height: 1.2;
  }

  .refresh-profile-btn {
    background: rgba(14, 165, 233, 0.12);
    border: 1px solid rgba(56, 189, 248, 0.35);
    color: #7dd3fc;
    border-radius: 8px;
    padding: 8px 14px;
    font-size: 0.75rem;
    font-weight: 800;
    cursor: pointer;
    transition: background 0.18s ease, color 0.18s ease, transform 0.18s ease;
  }

  .refresh-profile-btn:hover:not(:disabled) {
    background: rgba(14, 165, 233, 0.22);
    color: #e0f2fe;
    transform: translateY(-1px);
  }

  .refresh-profile-btn:disabled {
    opacity: 0.55;
    cursor: not-allowed;
  }

  .profile-shell {
    border: 1px solid rgba(51, 65, 85, 0.5);
    background: rgba(8, 13, 28, 0.72);
    border-radius: 14px;
    padding: 22px;
    display: grid;
    grid-template-columns: minmax(240px, 0.75fr) minmax(320px, 1.25fr);
    gap: 22px;
  }

  .profile-identity {
    border-right: 1px solid rgba(51, 65, 85, 0.42);
    padding-right: 22px;
    display: flex;
    align-items: center;
    gap: 16px;
    min-width: 0;
  }

  .profile-avatar {
    width: 72px;
    height: 72px;
    border-radius: 18px;
    border: 1px solid rgba(56, 189, 248, 0.38);
    background: linear-gradient(135deg, rgba(14, 165, 233, 0.24), rgba(30, 41, 59, 0.9));
    display: flex;
    align-items: center;
    justify-content: center;
    color: #7dd3fc;
    font-size: 1.5rem;
    font-weight: 900;
    overflow: hidden;
    flex: 0 0 auto;
  }

  .profile-avatar img {
    width: 100%;
    height: 100%;
    object-fit: cover;
  }

  .profile-title-block {
    min-width: 0;
  }

  .profile-title-block h3 {
    margin: 0 0 6px;
    color: #f8fafc;
    font-size: 1.12rem;
  }

  .profile-title-block p {
    margin: 0;
    color: #94a3b8;
    font-size: 0.76rem;
    word-break: break-all;
  }

  .profile-grid {
    display: grid;
    grid-template-columns: repeat(2, minmax(0, 1fr));
    gap: 12px;
  }

  .profile-field {
    min-height: 78px;
    border: 1px solid rgba(51, 65, 85, 0.48);
    background: rgba(15, 23, 42, 0.58);
    border-radius: 10px;
    padding: 12px;
    display: flex;
    flex-direction: column;
    justify-content: space-between;
    gap: 12px;
  }

  .profile-field.wide {
    grid-column: 1 / -1;
  }

  .profile-field.department-field {
    border-color: rgba(34, 197, 94, 0.32);
  }

  .profile-field.department-field.missing {
    border-color: rgba(245, 158, 11, 0.34);
  }

  .field-label {
    color: #64748b;
    font-size: 0.66rem;
    font-weight: 800;
    letter-spacing: 0.05em;
  }

  .profile-field strong {
    color: #e2e8f0;
    font-size: 0.92rem;
    line-height: 1.35;
    word-break: break-word;
  }

  .department-field strong {
    color: #86efac;
  }

  .department-field.missing strong {
    color: #fbbf24;
  }

  .font-mono {
    font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
  }

  .profile-page.compact {
    gap: 12px;
    width: 100%;
  }

  .profile-page.compact .profile-header {
    align-items: center;
    padding: 0 2px 10px;
    border-bottom: 1px solid rgba(51, 65, 85, 0.42);
  }

  .profile-page.compact .profile-eyebrow {
    font-size: 0.58rem;
    letter-spacing: 0.06em;
  }

  .profile-page.compact .profile-header h2 {
    margin-top: 4px;
    font-size: 0.95rem;
  }

  .profile-page.compact .refresh-profile-btn {
    padding: 6px 9px;
    border-radius: 7px;
    font-size: 0.66rem;
    white-space: nowrap;
  }

  .profile-page.compact .profile-shell {
    border: none;
    background: transparent;
    border-radius: 0;
    padding: 0;
    grid-template-columns: 1fr;
    gap: 12px;
  }

  .profile-page.compact .profile-identity {
    border-right: none;
    border-bottom: 1px solid rgba(51, 65, 85, 0.34);
    padding: 0 2px 12px;
    gap: 10px;
  }

  .profile-page.compact .profile-avatar {
    width: 42px;
    height: 42px;
    border-radius: 12px;
    font-size: 1rem;
  }

  .profile-page.compact .profile-title-block h3 {
    font-size: 0.9rem;
    margin-bottom: 3px;
  }

  .profile-page.compact .profile-title-block p {
    font-size: 0.68rem;
  }

  .profile-page.compact .profile-grid {
    grid-template-columns: 1fr;
    gap: 8px;
  }

  .profile-page.compact .profile-field {
    min-height: 0;
    border-radius: 8px;
    padding: 9px 10px;
    gap: 6px;
  }

  .profile-page.compact .profile-field strong {
    font-size: 0.78rem;
  }

  .profile-page.compact .field-label {
    font-size: 0.58rem;
  }

  @media (max-width: 860px) {
    .profile-shell {
      grid-template-columns: 1fr;
    }

    .profile-identity {
      border-right: none;
      border-bottom: 1px solid rgba(51, 65, 85, 0.42);
      padding-right: 0;
      padding-bottom: 18px;
    }

    .profile-grid {
      grid-template-columns: 1fr;
    }
  }
</style>
