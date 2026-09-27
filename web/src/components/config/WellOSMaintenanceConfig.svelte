<script lang="ts">
  import { createEventDispatcher, onMount } from 'svelte';
  import Alert from '../shared/Alert.svelte';
  import Button from '../shared/Button.svelte';
  import Modal from '../shared/Modal.svelte';
  import Switch from '../shared/Switch.svelte';
  import { responseErrorMessage } from '../../lib/http-error';
  import { showToast } from '../../lib/toast';

  type MaintenanceStatus = {
    configured: boolean;
    effective: boolean;
    source: 'database' | 'environment' | 'cli';
    override_locked: boolean;
    override_name?: string;
    version: number;
    updated_at?: string;
  };

  const dispatch = createEventDispatcher();

  export let canWrite = false;

  let status: MaintenanceStatus | null = null;
  let loading = true;
  let saving = false;
  let error = '';
  let confirmOpen = false;
  let requestedValue = false;

  $: effectiveLabel = status?.effective ? '已开启' : '已关闭';
  $: configuredLabel = status?.configured ? '开启' : '关闭';
  $: sourceLabel = status?.source === 'cli'
    ? '启动参数'
    : status?.source === 'environment'
      ? '部署环境'
      : '数据库配置';

  function formatUpdated(value?: string) {
    if (!value) return '暂无版本记录';
    const date = new Date(value);
    return Number.isNaN(date.getTime()) ? value : date.toLocaleString();
  }

  async function loadStatus() {
    loading = true;
    error = '';
    try {
      const response = await fetch('/api/config/wellos-maintenance');
      if (!response.ok) throw new Error(await responseErrorMessage(response, `HTTP ${response.status}`));
      status = await response.json();
    } catch (cause: any) {
      error = cause?.message || '维护模式状态加载失败';
    } finally {
      loading = false;
    }
  }

  function requestChange(next: boolean) {
    if (!status || saving || !canWrite) return;
    requestedValue = next;
    confirmOpen = true;
  }

  function closeConfirm() {
    if (saving) return;
    confirmOpen = false;
  }

  async function confirmChange() {
    if (!status || saving) return;
    saving = true;
    error = '';
    try {
      const response = await fetch('/api/config/wellos-maintenance', {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          enabled: requestedValue,
          expected_version: status.version
        })
      });
      if (!response.ok) throw new Error(await responseErrorMessage(response, `HTTP ${response.status}`));
      status = await response.json();
      confirmOpen = false;
      dispatch('updated', status);
      showToast(
        requestedValue
          ? '维护模式已启用，新登录可使用本地降级认证。'
          : '维护模式已关闭；已有降级会话最多仍可使用 2 小时。',
        { title: '登录维护模式已更新' }
      );
    } catch (cause: any) {
      error = cause?.message || '维护模式更新失败';
      showToast(error, { type: 'error', title: '更新失败' });
      if (error.includes('配置已被其他操作更新')) {
        await loadStatus();
        confirmOpen = false;
      }
    } finally {
      saving = false;
    }
  }

  onMount(loadStatus);
</script>

<div class="scw-workbench maintenance-workbench">
  <section class="scw-overview" aria-label="WellOS 登录维护模式">
    <header class="scw-header maintenance-header">
      <div class="scw-header-copy">
        <span class="scw-kicker">认证连续性</span>
        <h4>WellOS 登录维护模式</h4>
        <p>在 WellOS 计划维护期间，为符合条件的本地已知用户提供缓存凭据验证登录。</p>
      </div>

      <div class="maintenance-control" aria-busy={saving}>
        <span class:active={status?.effective}>{loading ? '加载中' : effectiveLabel}</span>
        <Switch
          id="wellos-maintenance-enabled"
          label="允许 WellOS 维护期间的本地降级登录"
          checked={status?.configured ?? false}
          disabled={loading || saving || !canWrite}
          {saving}
          optimistic={false}
          expandedHitArea
          on:change={(event) => requestChange(event.detail)}
        />
      </div>
    </header>

    {#if error}
      <div class="maintenance-error" role="alert">{error}</div>
    {/if}

    {#if status?.override_locked}
      <Alert
        type="warning"
        title="当前由部署环境强制开启"
        message={`数据库配置为“${configuredLabel}”，但 ${status.override_name || '部署覆盖'} 仍使维护模式保持开启。可预先修改数据库值；实际状态要在移除覆盖并重启服务后切换。`}
      />
    {:else if !canWrite}
      <Alert
        type="info"
        title="当前为只读状态"
        message="仅全局超级管理员且具备 config:write 权限的账号可以修改维护模式。"
      />
    {/if}

    {#if loading}
      <div class="maintenance-loading" aria-label="正在加载维护模式状态">
        <span></span>
        <span></span>
        <span></span>
      </div>
    {:else if status}
      <dl class="maintenance-facts">
        <div>
          <dt>当前生效</dt>
          <dd class:active={status.effective}>{effectiveLabel}</dd>
        </div>
        <div>
          <dt>数据库配置</dt>
          <dd>{configuredLabel}</dd>
        </div>
        <div>
          <dt>生效来源</dt>
          <dd>{sourceLabel}</dd>
        </div>
        <div>
          <dt>配置版本</dt>
          <dd class="font-mono">v{status.version || 0}</dd>
        </div>
        <div>
          <dt>最近更新</dt>
          <dd>{formatUpdated(status.updated_at)}</dd>
        </div>
      </dl>
    {/if}

    <div class="maintenance-notes">
      <div>
        <strong>允许范围</strong>
        <p>仅允许已成功登录过、具有本地密码哈希和权限快照的账号。未知账号、密码不匹配或缺少权限快照仍会被拒绝。</p>
      </div>
      <div>
        <strong>影响范围</strong>
        <p>数据库开关只控制 WellOS 返回维护或账号禁用状态时的新登录。网络不可达时的既有本地降级策略不受影响。</p>
      </div>
      <div>
        <strong>会话说明</strong>
        <p>关闭后不再签发新的维护会话；已经签发的降级 JWT 不会立即撤销，最多继续有效 2 小时。</p>
      </div>
    </div>
  </section>
</div>

<Modal
  show={confirmOpen}
  title={requestedValue ? '启用 WellOS 登录维护模式？' : '关闭 WellOS 登录维护模式？'}
  closeLabel="关闭维护模式确认"
  closeDisabled={saving}
  layer="critical"
  on:close={closeConfirm}
>
  <div class="maintenance-confirm-copy">
    <p>
      {#if requestedValue}
        符合条件的本地已知用户将在 WellOS 返回维护或账号禁用状态时获得本地降级登录。此操作不会扩大用户权限。
      {:else}
        后续登录遇到 WellOS 维护或账号禁用状态时，将不再通过此模式尝试本地验证。
      {/if}
    </p>
    {#if status?.override_locked}
      <p class="session-warning">当前由 {status.override_name || '部署环境'} 强制开启；本次操作只更新数据库值，不会立即改变实际生效状态。</p>
    {/if}
    <p class="session-warning">现有会话不受立即撤销，包括维护模式签发的降级会话；已签发 JWT 最多仍可使用 2 小时。</p>
  </div>
  <svelte:fragment slot="footer">
    <div class="maintenance-confirm-actions">
      <Button variant="secondary" disabled={saving} on:click={closeConfirm}>取消</Button>
      <Button
        variant={requestedValue ? 'primary' : 'danger'}
        loading={saving}
        on:click={confirmChange}
      >
        {requestedValue ? '确认启用' : '确认关闭'}
      </Button>
    </div>
  </svelte:fragment>
</Modal>

<style>
  .maintenance-workbench {
    max-width: 980px;
  }

  .maintenance-header {
    align-items: center;
  }

  .maintenance-control {
    flex: none;
    min-height: 44px;
    display: flex;
    align-items: center;
    gap: 12px;
    color: var(--scw-muted);
    font-size: 12px;
    font-weight: 760;
    white-space: nowrap;
  }

  .maintenance-control > span.active,
  .maintenance-facts dd.active {
    color: var(--scw-warning);
  }

  .maintenance-error {
    padding: 12px 14px;
    border: 1px solid color-mix(in srgb, var(--scw-danger) 28%, transparent);
    border-radius: 8px;
    background: color-mix(in srgb, var(--scw-danger) 7%, transparent);
    color: var(--scw-danger);
    font-size: 12px;
    line-height: 1.55;
  }

  .maintenance-loading,
  .maintenance-facts {
    margin: 0;
    border-top: 1px solid var(--scw-line);
    border-bottom: 1px solid var(--scw-line);
  }

  .maintenance-loading {
    display: grid;
    gap: 1px;
    background: var(--scw-line);
  }

  .maintenance-loading span {
    min-height: 48px;
    background: linear-gradient(90deg, rgba(121, 139, 159, 0.08), rgba(255, 255, 255, 0.72), rgba(121, 139, 159, 0.08));
  }

  .maintenance-facts {
    display: grid;
    grid-template-columns: repeat(5, minmax(0, 1fr));
  }

  .maintenance-facts div {
    min-width: 0;
    padding: 13px 14px;
  }

  .maintenance-facts div + div {
    border-left: 1px solid var(--scw-line);
  }

  .maintenance-facts dt {
    color: var(--scw-muted);
    font-size: 11px;
    line-height: 1.35;
  }

  .maintenance-facts dd {
    margin: 5px 0 0;
    color: var(--scw-ink);
    font-size: 13px;
    font-weight: 760;
    line-height: 1.4;
    overflow-wrap: anywhere;
  }

  .maintenance-notes {
    display: grid;
    gap: 0;
  }

  .maintenance-notes > div {
    display: grid;
    grid-template-columns: minmax(110px, 0.2fr) minmax(0, 1fr);
    gap: 16px;
    padding: 13px 0;
    border-bottom: 1px solid var(--scw-line);
  }

  .maintenance-notes strong {
    color: var(--scw-ink);
    font-size: 12px;
  }

  .maintenance-notes p,
  .maintenance-confirm-copy p {
    max-width: 72ch;
    margin: 0;
    color: var(--scw-muted);
    font-size: 12px;
    line-height: 1.65;
    text-wrap: pretty;
  }

  .maintenance-confirm-copy {
    display: grid;
    gap: 14px;
  }

  .maintenance-confirm-copy .session-warning {
    padding-top: 14px;
    border-top: 1px solid var(--scw-line);
    color: var(--scw-warning);
  }

  .maintenance-confirm-actions {
    display: flex;
    justify-content: flex-end;
    gap: 10px;
  }

  @media (max-width: 760px) {
    .maintenance-header {
      display: grid;
    }

    .maintenance-control {
      width: 100%;
      justify-content: space-between;
    }

    .maintenance-facts {
      grid-template-columns: repeat(2, minmax(0, 1fr));
    }

    .maintenance-facts div + div {
      border-left: 0;
    }

    .maintenance-facts div:nth-child(even) {
      border-left: 1px solid var(--scw-line);
    }

    .maintenance-facts div:nth-child(n + 3) {
      border-top: 1px solid var(--scw-line);
    }

    .maintenance-notes > div {
      grid-template-columns: 1fr;
      gap: 5px;
    }
  }

  @media (max-width: 480px) {
    .maintenance-facts {
      grid-template-columns: 1fr;
    }

    .maintenance-facts div:nth-child(even) {
      border-left: 0;
    }

    .maintenance-facts div + div {
      border-top: 1px solid var(--scw-line);
    }

    .maintenance-confirm-actions {
      flex-direction: column;
    }

    .maintenance-confirm-actions :global(button) {
      width: 100%;
    }
  }

  @media (prefers-reduced-motion: reduce) {
    .maintenance-loading span {
      background: rgba(121, 139, 159, 0.08);
    }
  }
</style>
