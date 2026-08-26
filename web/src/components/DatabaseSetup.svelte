<script lang="ts">
  import { onDestroy, onMount } from 'svelte';

  export let onCompleted: () => void = () => {};

  type RequestState = 'idle' | 'working' | 'success' | 'error';
  type SchemaState = 'database_missing' | 'empty' | 'well_ambient' | 'unknown';
  type SetupOperation = 'create_database' | 'initialize_empty' | 'connect_existing' | 'blocked';
  type LegacyDecision = 'migrate' | 'skip';

  interface SetupInspection {
    server_version: string;
    schema_state: SchemaState;
    database_exists: boolean;
    can_create_database: boolean;
    required_operation: SetupOperation;
  }

  interface LegacySQLiteStatus {
    available: boolean;
    size_bytes?: number;
    table_count?: number;
    prompt_required: boolean;
    decision_recorded: boolean;
    decision?: LegacyDecision;
  }

  interface OperationStatus {
    id: string;
    state: 'queued' | 'running' | 'completed' | 'failed';
    stage: string;
    table?: string;
    tables_completed: number;
    tables_total: number;
    rows_copied: number;
    restart_required?: boolean;
    error?: { code?: string; message?: string };
  }

  interface SetupErrorPayload {
    error?: { code?: string; message?: string };
  }

  let step: 1 | 2 = 1;
  let host = 'postgres';
  let port = 5432;
  let database = 'well_ambient';
  let maintenanceDatabase = 'postgres';
  let username = 'well_ambient';
  let password = '';
  let sslMode = 'disable';
  let sslRootCert = '';
  let setupToken = '';
  let legacyDecision: LegacyDecision | '' = '';

  let testState: RequestState = 'idle';
  let applyState: RequestState = 'idle';
  let statusMessage = '尚未测试连接。';
  let inspection: SetupInspection | null = null;
  let legacySQLite: LegacySQLiteStatus = {
    available: false,
    prompt_required: false,
    decision_recorded: false
  };
  let operation: OperationStatus | null = null;
  let testedFingerprint = '';
  let fieldErrors: Record<string, string> = {};
  let restartTimedOut = false;
  let destroyed = false;

  $: currentFingerprint = JSON.stringify({
    host: host.trim(), port, database: database.trim(), maintenance_database: maintenanceDatabase.trim(),
    username: username.trim(), password, ssl_mode: sslMode, ssl_root_cert: sslRootCert.trim(), setup_token: setupToken
  });
  $: connectionIsCurrent = testState === 'success' && testedFingerprint === currentFingerprint;
  $: inspectionIsSafe = Boolean(
    inspection && inspection.required_operation !== 'blocked' &&
    (inspection.schema_state !== 'database_missing' || inspection.can_create_database)
  );
  $: legacyChoiceApplies = Boolean(
    inspection && (legacySQLite.available || legacySQLite.decision_recorded) &&
    (inspection.required_operation === 'create_database' || inspection.required_operation === 'initialize_empty')
  );
  $: legacyDecisionReady = !legacyChoiceApplies || legacySQLite.decision_recorded || Boolean(legacyDecision);
  $: canContinue = connectionIsCurrent && inspectionIsSafe && testState !== 'working';
  $: canApply = canContinue && legacyDecisionReady && applyState !== 'working';
  $: progressPercent = operation?.tables_total
    ? Math.round((operation.tables_completed / operation.tables_total) * 100)
    : operation?.stage === 'completed' ? 100 : 0;

  $: checklist = [
    {
      label: '连接信息',
      detail: host.trim() && database.trim() && username.trim() && password
        ? `${host.trim()}:${port} / ${database.trim()}` : '请填写全部必填字段',
      passed: Boolean(host.trim() && database.trim() && maintenanceDatabase.trim() && username.trim() && password && port > 0 && port <= 65535)
    },
    {
      label: '安装令牌',
      detail: setupToken.length >= 32 ? '已填写一次性令牌' : '至少 32 个字符',
      passed: setupToken.length >= 32
    },
    {
      label: '连接测试',
      detail: inspection ? `PostgreSQL ${inspection.server_version}，${schemaStateLabel(inspection.schema_state)}` : '需要使用当前信息测试',
      passed: connectionIsCurrent
    },
    {
      label: '安全操作',
      detail: inspection ? operationSummary(inspection) : '由服务器检查后确定',
      passed: connectionIsCurrent && inspectionIsSafe
    }
  ];

  function requestPayload() {
    return {
      host: host.trim(),
      port: Number(port),
      database: database.trim(),
      maintenance_database: maintenanceDatabase.trim(),
      username: username.trim(),
      password,
      ssl_mode: sslMode,
      ssl_root_cert: sslRootCert.trim(),
      mode: inspection?.required_operation || ''
    };
  }

  function markConnectionDirty() {
    if (testState === 'working' || applyState === 'working') return;
    if (testedFingerprint && testedFingerprint !== currentFingerprint) {
      testState = 'idle';
      inspection = null;
      step = 1;
      statusMessage = '连接信息已变化，请重新测试。';
    }
  }

  function validate(): boolean {
    const errors: Record<string, string> = {};
    if (!host.trim()) errors.host = '请输入 PostgreSQL 主机名或 IP 地址。';
    if (!Number.isInteger(Number(port)) || Number(port) < 1 || Number(port) > 65535) errors.port = '端口必须在 1 到 65535 之间。';
    if (!database.trim()) errors.database = '请输入目标数据库名。';
    if (new TextEncoder().encode(database.trim()).length > 63) errors.database = '数据库名的 UTF-8 编码不能超过 63 字节。';
    if (!maintenanceDatabase.trim()) errors.maintenance_database = '请输入维护数据库名。';
    if (new TextEncoder().encode(maintenanceDatabase.trim()).length > 63) errors.maintenance_database = '维护数据库名的 UTF-8 编码不能超过 63 字节。';
    if (!username.trim()) errors.username = '请输入数据库用户名。';
    if (!password) errors.password = '请输入数据库密码。';
    if (setupToken.length < 32) errors.setup_token = '安装令牌至少需要 32 个字符。';
    if ((sslMode === 'verify-ca' || sslMode === 'verify-full') && sslRootCert && !sslRootCert.startsWith('/')) {
      errors.ssl_root_cert = '根证书路径必须是服务器容器内的绝对路径。';
    }
    fieldErrors = errors;
    return Object.keys(errors).length === 0;
  }

  async function parseError(response: Response): Promise<string> {
    try {
      const payload = await response.json() as SetupErrorPayload;
      return payload.error?.message || `请求失败 (${response.status})`;
    } catch {
      return `请求失败 (${response.status})`;
    }
  }

  async function loadSetupStatus() {
    try {
      const response = await fetch('/api/setup/status', { cache: 'no-store' });
      if (!response.ok) return;
      const payload = await response.json() as { legacy_sqlite?: LegacySQLiteStatus };
      if (payload.legacy_sqlite) {
        legacySQLite = payload.legacy_sqlite;
        legacyDecision = payload.legacy_sqlite.decision || '';
      }
    } catch {
      // Connection testing remains available even if this optional probe fails.
    }
  }

  async function testConnection() {
    if (!validate()) {
      testState = 'error';
      statusMessage = '请先修正表单中的必填项。';
      return;
    }
    testState = 'working';
    applyState = 'idle';
    inspection = null;
    statusMessage = '正在连接维护数据库，并确认目标数据库是否存在。';
    const fingerprint = currentFingerprint;
    try {
      const response = await fetch('/api/setup/database/test', {
        method: 'POST',
        cache: 'no-store',
        headers: { 'Content-Type': 'application/json', 'X-Setup-Token': setupToken },
        body: JSON.stringify(requestPayload())
      });
      if (!response.ok) throw new Error(await parseError(response));
      const payload = await response.json() as { database: SetupInspection };
      inspection = payload.database;
      testedFingerprint = fingerprint;
      testState = 'success';
      statusMessage = `连接成功。${operationSummary(payload.database)}`;
      await loadSetupStatus();
    } catch (error) {
      testedFingerprint = '';
      testState = 'error';
      statusMessage = error instanceof Error ? error.message : '数据库连接测试失败。';
    }
  }

  async function continueToConfirmation() {
    if (!canContinue) {
      statusMessage = inspection?.schema_state === 'database_missing' && !inspection.can_create_database
        ? '目标数据库不存在，当前角色没有 CREATEDB 权限。'
        : '请先使用当前信息完成连接测试。';
      return;
    }
    await loadSetupStatus();
    step = 2;
  }

  async function recordLegacyDecisionIfNeeded(): Promise<void> {
    if (!legacyChoiceApplies || legacySQLite.decision_recorded) return;
    if (!legacyDecision) throw new Error('请选择迁移本地 SQLite 数据或跳过迁移。');
    const response = await fetch('/api/setup/legacy-sqlite/decision', {
      method: 'POST',
      cache: 'no-store',
      headers: { 'Content-Type': 'application/json', 'X-Setup-Token': setupToken },
      body: JSON.stringify({ decision: legacyDecision })
    });
    if (!response.ok) throw new Error(await parseError(response));
    const payload = await response.json() as { legacy_sqlite: LegacySQLiteStatus };
    legacySQLite = payload.legacy_sqlite;
  }

  async function applyDatabase(event: SubmitEvent) {
    event.preventDefault();
    if (step !== 2 || !validate() || !canApply || !inspection) {
      applyState = 'error';
      statusMessage = !connectionIsCurrent ? '请先使用当前信息完成连接测试。' : '请完成本地数据迁移选择。';
      return;
    }
    applyState = 'working';
    operation = null;
    statusMessage = '正在记录安装选择并启动数据库任务。';
    try {
      await recordLegacyDecisionIfNeeded();
      const response = await fetch('/api/setup/database/apply', {
        method: 'POST',
        cache: 'no-store',
        headers: { 'Content-Type': 'application/json', 'X-Setup-Token': setupToken },
        body: JSON.stringify(requestPayload())
      });
      if (!response.ok) throw new Error(await parseError(response));
      const payload = await response.json() as { operation: OperationStatus };
      operation = payload.operation;
      statusMessage = '数据库任务已启动，请保持页面打开。';
      await pollOperation();
    } catch (error) {
      applyState = 'error';
      statusMessage = error instanceof Error ? error.message : '数据库配置失败。';
    }
  }

  async function pollOperation() {
    while (!destroyed) {
      await new Promise((resolve) => window.setTimeout(resolve, 750));
      try {
        const response = await fetch('/api/setup/database/operation', {
          cache: 'no-store',
          headers: { 'X-Setup-Token': setupToken }
        });
        if (!response.ok) throw new Error(await parseError(response));
        const payload = await response.json() as { operation: OperationStatus };
        operation = payload.operation;
        statusMessage = operationStatusLabel(payload.operation);
        if (payload.operation.state === 'failed') {
          applyState = 'error';
          statusMessage = payload.operation.error?.message || '数据库安装任务失败。';
          return;
        }
        if (payload.operation.state === 'completed') {
          applyState = 'success';
          password = '';
          testedFingerprint = '';
          statusMessage = '数据库配置已安全保存，服务正在切换到正常模式。';
          await waitForNormalMode();
          return;
        }
      } catch {
        // The setup process exits after success, so an expected connection gap
        // is resolved by probing the public setup status without credentials.
        await waitForNormalMode();
        return;
      }
    }
  }

  async function waitForNormalMode() {
    for (let attempt = 0; attempt < 60 && !destroyed; attempt += 1) {
      await new Promise((resolve) => window.setTimeout(resolve, 1500));
      try {
        const response = await fetch('/api/setup/status', { cache: 'no-store' });
        if (response.status === 404) {
          setupToken = '';
          onCompleted();
          return;
        }
        if (response.ok) {
          const payload = await response.json() as { setup_required?: boolean };
          if (payload.setup_required === false) {
            setupToken = '';
            onCompleted();
            return;
          }
        }
      } catch {
        // A short connection gap is expected while the container restarts.
      }
    }
    restartTimedOut = true;
    statusMessage = '安装任务已结束，但服务重启时间超过预期。请刷新页面检查运行状态。';
  }

  function schemaStateLabel(state: SchemaState): string {
    if (state === 'database_missing') return '目标数据库不存在';
    if (state === 'empty') return '目标数据库存在且 schema 为空';
    if (state === 'well_ambient') return '已识别完整的 Well Ambient 结构';
    return '目标 schema 已有未知表';
  }

  function operationSummary(value: SetupInspection): string {
    if (value.schema_state === 'database_missing') {
      return value.can_create_database
        ? `目标数据库 ${database.trim()} 不存在，将由当前角色创建后初始化。`
        : `目标数据库 ${database.trim()} 不存在，当前角色没有 CREATEDB 权限。`;
    }
    if (value.schema_state === 'empty') return '目标数据库已存在且 schema 为空，将初始化当前结构。';
    if (value.schema_state === 'well_ambient') return '目标数据库结构完整，将直接接入，不改写历史数据。';
    return '目标 schema 已有未知表，自动安装已阻止。';
  }

  function operationStatusLabel(value: OperationStatus): string {
    const labels: Record<string, string> = {
      queued: '任务已排队。',
      checking_database: '正在重新确认目标数据库状态。',
      creating_database: '正在创建不存在的目标数据库。',
      preparing_schema: '正在创建 PostgreSQL 表与索引。',
      initializing_schema: '正在初始化 PostgreSQL 表、索引和基础权限。',
      copying_data: value.table
        ? `正在迁移 ${value.table}（${value.tables_completed}/${value.tables_total}，已复制 ${value.rows_copied} 行）。`
        : '正在迁移本地 SQLite 数据。',
      verifying_counts: `正在核对逐表行数，已复制 ${value.rows_copied} 行。`,
      analyzing: '正在更新 PostgreSQL 统计信息。',
      completed: '数据库任务已完成，正在切换服务模式。'
    };
    return labels[value.stage] || '数据库任务正在执行。';
  }

  function formatBytes(value = 0): string {
    if (value < 1024) return `${value} B`;
    if (value < 1024 * 1024) return `${(value / 1024).toFixed(1)} KiB`;
    if (value < 1024 * 1024 * 1024) return `${(value / 1024 / 1024).toFixed(1)} MiB`;
    return `${(value / 1024 / 1024 / 1024).toFixed(2)} GiB`;
  }

  onMount(() => {
    void loadSetupStatus();
  });

  onDestroy(() => {
    destroyed = true;
    password = '';
    setupToken = '';
  });
</script>

<main class="database-setup" aria-labelledby="setup-title">
  <section class="setup-intro">
    <div class="setup-brand">well-ambient</div>
    <div class="setup-intro-copy">
      <p class="setup-kicker">首次安装</p>
      <h1 id="setup-title">先确认 PostgreSQL，再迁移本地数据</h1>
      <p class="setup-summary">连接测试会明确区分“目标数据库不存在”“空 schema”和“已迁移结构”。安装完成后，数据库入口会关闭，登录与业务接口才会启用。</p>
    </div>
    <div class="setup-security-note">
      <strong>安全边界</strong>
      <p>数据库密码和安装令牌只提交给当前服务。SQLite 路径由服务器配置固定，页面无法选择任意文件。</p>
    </div>
  </section>

  <form class="setup-workspace" aria-busy={testState === 'working' || applyState === 'working'} on:submit={applyDatabase}>
    <nav class="setup-steps" aria-label="数据库安装步骤">
      <span class:active={step === 1} class:completed={step === 2}><b>1</b> 连接与测试</span>
      <span class:active={step === 2}><b>2</b> 数据与确认</span>
    </nav>

    <header class="setup-workspace-header">
      <div>
        <p class="setup-kicker">步骤 {step} / 2</p>
        <h2>{step === 1 ? '配置运行数据库' : '确认首次安装操作'}</h2>
        <p>{step === 1 ? '先连接维护数据库，再检查目标数据库是否存在及当前角色权限。' : '核对服务器检查结果，并仅在发现本地 SQLite 快照时确认是否迁移。'}</p>
      </div>
      <span class="setup-status-chip" class:is-ready={step === 1 ? canContinue : canApply}>
        {step === 1 ? (canContinue ? '检查通过' : '等待检查') : (canApply ? '可以安装' : '等待确认')}
      </span>
    </header>

    {#if step === 1}
      <fieldset class="setup-section" disabled={applyState === 'working' || applyState === 'success'}>
        <legend>连接信息</legend>
        <div class="setup-fields two-columns">
          <label>
            <span>主机名或 IP *</span>
            <input id="setup-host" bind:value={host} on:input={markConnectionDirty} aria-invalid={fieldErrors.host ? 'true' : undefined} autocomplete="off" />
            {#if fieldErrors.host}<small class="field-error">{fieldErrors.host}</small>{:else}<small>Compose 内置 PostgreSQL 使用 <code>postgres</code>。</small>{/if}
          </label>
          <label>
            <span>端口 *</span>
            <input id="setup-port" type="number" min="1" max="65535" bind:value={port} on:input={markConnectionDirty} aria-invalid={fieldErrors.port ? 'true' : undefined} />
            {#if fieldErrors.port}<small class="field-error">{fieldErrors.port}</small>{/if}
          </label>
          <label>
            <span>目标数据库名 *</span>
            <input id="setup-database" bind:value={database} on:input={markConnectionDirty} aria-invalid={fieldErrors.database ? 'true' : undefined} autocomplete="off" />
            {#if fieldErrors.database}<small class="field-error">{fieldErrors.database}</small>{:else}<small>允许尚不存在；测试后由服务判断是否创建。</small>{/if}
          </label>
          <label>
            <span>维护数据库名 *</span>
            <input id="setup-maintenance-database" bind:value={maintenanceDatabase} on:input={markConnectionDirty} aria-invalid={fieldErrors.maintenance_database ? 'true' : undefined} autocomplete="off" />
            {#if fieldErrors.maintenance_database}<small class="field-error">{fieldErrors.maintenance_database}</small>{:else}<small>通常为 <code>postgres</code>，用于检查和创建目标数据库。</small>{/if}
          </label>
          <label>
            <span>用户名 *</span>
            <input id="setup-username" bind:value={username} on:input={markConnectionDirty} aria-invalid={fieldErrors.username ? 'true' : undefined} autocomplete="off" />
            {#if fieldErrors.username}<small class="field-error">{fieldErrors.username}</small>{/if}
          </label>
          <label>
            <span>数据库密码 *</span>
            <input id="setup-password" type="password" bind:value={password} on:input={markConnectionDirty} aria-invalid={fieldErrors.password ? 'true' : undefined} autocomplete="new-password" />
            {#if fieldErrors.password}<small class="field-error">{fieldErrors.password}</small>{:else}<small>成功后从页面内存清除。</small>{/if}
          </label>
        </div>
      </fieldset>

      <fieldset class="setup-section" disabled={applyState === 'working' || applyState === 'success'}>
        <legend>传输安全与安装授权</legend>
        <div class="setup-fields two-columns">
          <label>
            <span>SSL 模式 *</span>
            <select id="setup-ssl-mode" bind:value={sslMode} on:change={markConnectionDirty}>
              <option value="disable">disable</option>
              <option value="prefer">prefer</option>
              <option value="require">require</option>
              <option value="verify-ca">verify-ca</option>
              <option value="verify-full">verify-full</option>
            </select>
            <small>跨主机连接建议使用 <code>verify-full</code>。</small>
          </label>
          <label>
            <span>根证书路径</span>
            <input id="setup-root-cert" bind:value={sslRootCert} on:input={markConnectionDirty} placeholder="/etc/well-ambient/ca.pem" aria-invalid={fieldErrors.ssl_root_cert ? 'true' : undefined} autocomplete="off" />
            {#if fieldErrors.ssl_root_cert}<small class="field-error">{fieldErrors.ssl_root_cert}</small>{:else}<small>仅 verify-ca / verify-full 使用。</small>{/if}
          </label>
          <label class="full-width">
            <span>一次性安装令牌 *</span>
            <input id="setup-token" type="password" bind:value={setupToken} on:input={markConnectionDirty} aria-invalid={fieldErrors.setup_token ? 'true' : undefined} autocomplete="new-password" />
            {#if fieldErrors.setup_token}<small class="field-error">{fieldErrors.setup_token}</small>{:else}<small>读取 <code>deploy/.env.production</code> 中的 <code>WELL_AMBIENT_SETUP_TOKEN</code>。</small>{/if}
          </label>
        </div>
      </fieldset>

      <section class="setup-check" aria-labelledby="setup-check-title">
        <div class="setup-check-header">
          <div>
            <h3 id="setup-check-title">连接检查</h3>
            <p>不会在此步骤创建数据库或写入配置。</p>
          </div>
          <button type="button" class="secondary-action" on:click={testConnection} disabled={testState === 'working' || applyState === 'working'}>
            {testState === 'working' ? '正在测试' : '测试连接'}
          </button>
        </div>
        <div class="check-list">
          {#each checklist as item}
            <div class="check-row" class:passed={item.passed}>
              <span class="check-symbol" aria-hidden="true">{item.passed ? '✓' : '!'}</span>
              <div><strong>{item.label}</strong><small>{item.detail}</small></div>
            </div>
          {/each}
        </div>
      </section>
    {:else}
      <section class="confirmation-section" aria-labelledby="operation-title">
        <div class="section-heading">
          <p class="setup-kicker">数据库操作</p>
          <h3 id="operation-title">{inspection ? schemaStateLabel(inspection.schema_state) : '等待检查结果'}</h3>
          <p>{inspection ? operationSummary(inspection) : '返回上一步重新测试连接。'}</p>
        </div>
        <dl class="setup-summary-list">
          <div><dt>PostgreSQL</dt><dd>{inspection?.server_version || '—'}</dd></div>
          <div><dt>目标</dt><dd>{host.trim()}:{port} / {database.trim()}</dd></div>
          <div><dt>维护库</dt><dd>{maintenanceDatabase.trim()}</dd></div>
          <div><dt>服务操作</dt><dd>{inspection?.required_operation || '—'}</dd></div>
        </dl>
      </section>

      <fieldset class="setup-section legacy-section" disabled={applyState === 'working' || applyState === 'success'}>
        <legend>本地 SQLite 数据</legend>
        {#if legacyChoiceApplies && legacySQLite.decision_recorded}
          <div class="decision-recorded">
            <strong>本次安装已记录：{legacySQLite.decision === 'migrate' ? '迁移本地数据' : '跳过本地数据'}</strong>
            <p>该选择只记录一次。若确需变更，请由管理员修改服务器运行配置后重新进入安装流程。</p>
          </div>
        {:else if legacyChoiceApplies}
          <p class="legacy-facts">服务器发现一个只读 SQLite 快照：{formatBytes(legacySQLite.size_bytes)}，约 {legacySQLite.table_count || 0} 张非系统表。请选择一次，提交后不可在页面修改。</p>
          <div class="mode-grid">
            <label class="mode-card" class:selected={legacyDecision === 'migrate'}>
              <input type="radio" name="legacy-decision" value="migrate" bind:group={legacyDecision} />
              <span>
                <strong>迁移本地数据</strong>
                <small>在同一事务中创建结构、按白名单分批复制、重置 sequence 并逐表核对行数。</small>
              </span>
            </label>
            <label class="mode-card" class:selected={legacyDecision === 'skip'}>
              <input type="radio" name="legacy-decision" value="skip" bind:group={legacyDecision} />
              <span>
                <strong>跳过本地数据</strong>
                <small>只初始化 PostgreSQL；SQLite 快照不会被删除或修改。</small>
              </span>
            </label>
          </div>
        {:else if legacySQLite.available && inspection?.required_operation === 'connect_existing'}
          <p class="legacy-facts">服务器发现 SQLite 快照，但目标 PostgreSQL 已是完整结构，本次只接入现有数据库，不重复迁移。</p>
        {:else}
          <p class="legacy-facts">服务器未发现配置路径下可读取且校验通过的 SQLite 快照，本次将按全新安装处理。</p>
        {/if}
      </fieldset>

      {#if operation}
        <section class="operation-progress" aria-labelledby="progress-title">
          <div class="progress-heading">
            <div>
              <h3 id="progress-title">安装进度</h3>
              <p>{operationStatusLabel(operation)}</p>
            </div>
            <strong>{progressPercent}%</strong>
          </div>
          <div class="progress-track" role="progressbar" aria-valuemin="0" aria-valuemax="100" aria-valuenow={progressPercent}>
            <span style={`width: ${progressPercent}%`}></span>
          </div>
          {#if operation.rows_copied > 0}
            <small>已复制 {operation.rows_copied} 行 · {operation.tables_completed}/{operation.tables_total} 张表</small>
          {/if}
        </section>
      {/if}
    {/if}

    <div class="setup-feedback tone-{applyState === 'success' ? 'success' : testState === 'error' || applyState === 'error' ? 'error' : testState === 'working' || applyState === 'working' ? 'working' : 'neutral'}" role="status" aria-live="polite">
      <strong>{applyState === 'success' ? '配置已保存' : testState === 'success' ? '检查结果' : '当前状态'}</strong>
      <span>{statusMessage}</span>
    </div>

    <footer class="setup-actions">
      <p>最终连接串会原子写入权限为 0600 的服务器运行配置；SQLite 快照只读，不会自动删除。</p>
      <div class="action-group">
        {#if step === 2 && applyState !== 'success'}
          <button type="button" class="secondary-action" on:click={() => step = 1} disabled={applyState === 'working'}>返回修改</button>
        {/if}
        {#if restartTimedOut}
          <button type="button" class="primary-action" on:click={() => window.location.reload()}>刷新页面</button>
        {:else if step === 1}
          <button type="button" class="primary-action" on:click={continueToConfirmation} disabled={!canContinue}>下一步</button>
        {:else}
          <button type="submit" class="primary-action" disabled={!canApply}>
            {applyState === 'working' ? '正在执行安装' : inspection?.required_operation === 'connect_existing' ? '确认接入并启动' : '开始安装并启动'}
          </button>
        {/if}
      </div>
    </footer>
  </form>
</main>

<style>
  .database-setup {
    min-height: 100dvh;
    display: grid;
    grid-template-columns: minmax(260px, 0.78fr) minmax(0, 1.7fr);
    gap: clamp(24px, 4vw, 64px);
    align-items: start;
    padding: clamp(24px, 5vw, 72px);
    background: var(--wa-bg-ambient), var(--wa-bg-page);
    color: var(--wa-text-main);
  }

  .setup-intro {
    min-width: 0;
    display: flex;
    flex-direction: column;
    gap: 48px;
    padding-top: 8px;
  }

  .setup-brand {
    width: fit-content;
    color: var(--wa-rail-text-strong);
    background: var(--wa-bg-rail);
    border-radius: var(--wa-radius-sm);
    padding: 10px 14px;
    font-family: var(--wa-font-display);
    font-size: 16px;
    font-weight: 760;
    letter-spacing: -0.02em;
  }

  .setup-kicker {
    margin: 0 0 10px;
    color: var(--wa-accent-strong);
    font-size: 12px;
    font-weight: 800;
    letter-spacing: 0.08em;
  }

  h1, h2, h3, p { margin-top: 0; }

  h1 {
    max-width: 12ch;
    margin-bottom: 18px;
    color: var(--wa-text-strong);
    font-family: var(--wa-font-display);
    font-size: clamp(34px, 4vw, 54px);
    line-height: 1.05;
    letter-spacing: -0.045em;
    text-wrap: balance;
    overflow-wrap: anywhere;
    min-width: 0;
  }

  .setup-summary {
    max-width: 38ch;
    margin-bottom: 0;
    color: var(--wa-text-muted);
    font-size: 15px;
    line-height: 1.75;
    text-wrap: pretty;
  }

  .setup-security-note {
    max-width: 390px;
    padding-top: 18px;
    border-top: 1px solid var(--wa-border-strong);
  }

  .setup-security-note strong { color: var(--wa-text-strong); font-size: 13px; }
  .setup-security-note p { margin: 8px 0 0; color: var(--wa-text-muted); font-size: 13px; line-height: 1.65; }

  code {
    font-family: var(--wa-font-mono);
    font-size: 0.92em;
    color: var(--wa-info-strong);
    overflow-wrap: anywhere;
  }

  .setup-workspace {
    width: 100%;
    max-width: 880px;
    min-width: 0;
    justify-self: end;
    background: var(--wa-glass-panel-strong);
    border: 1px solid var(--wa-glass-outline);
    border-top-color: var(--wa-glass-highlight);
    border-radius: var(--wa-radius-xl);
    box-shadow: var(--wa-shadow-panel);
    backdrop-filter: blur(18px) saturate(124%);
    padding: clamp(20px, 3vw, 34px);
  }

  .setup-steps {
    display: grid;
    grid-template-columns: minmax(0, 1fr) minmax(0, 1fr);
    gap: 1px;
    margin: -4px 0 24px;
    border: 1px solid var(--wa-border-divider);
    border-radius: var(--wa-radius-md);
    overflow: hidden;
    background: var(--wa-border-divider);
  }

  .setup-steps span {
    min-height: 44px;
    display: flex;
    align-items: center;
    gap: 9px;
    padding: 0 14px;
    background: var(--wa-surface-flat);
    color: var(--wa-text-muted);
    font-size: 12px;
    font-weight: 720;
  }

  .setup-steps b {
    width: 22px;
    height: 22px;
    display: grid;
    place-items: center;
    border-radius: var(--wa-radius-pill);
    background: var(--wa-neutral-soft);
    color: var(--wa-text-muted);
    font-size: 11px;
  }

  .setup-steps span.active { color: var(--wa-accent-strong); background: var(--wa-accent-soft); }
  .setup-steps span.active b, .setup-steps span.completed b { background: var(--wa-accent-fill); color: var(--wa-accent-fill-ink); }

  .setup-workspace-header {
    display: flex;
    justify-content: space-between;
    align-items: flex-start;
    gap: 24px;
    padding-bottom: 24px;
    border-bottom: 1px solid var(--wa-border-divider);
  }

  .setup-workspace-header h2 {
    margin-bottom: 8px;
    color: var(--wa-text-strong);
    font-size: 23px;
    letter-spacing: -0.025em;
  }

  .setup-workspace-header p:last-child { margin: 0; color: var(--wa-text-muted); font-size: 13px; line-height: 1.55; }

  .setup-status-chip {
    flex: 0 0 auto;
    min-height: 32px;
    display: inline-flex;
    align-items: center;
    padding: 0 11px;
    border-radius: var(--wa-radius-pill);
    background: var(--wa-neutral-soft);
    color: var(--wa-text-muted);
    font-size: 12px;
    font-weight: 760;
  }

  .setup-status-chip.is-ready { background: var(--wa-success-soft); color: var(--wa-success); }

  fieldset {
    min-width: 0;
    margin: 0;
    padding: 24px 0;
    border: 0;
    border-bottom: 1px solid var(--wa-border-divider);
  }

  legend {
    margin-bottom: 16px;
    padding: 0;
    color: var(--wa-text-strong);
    font-size: 15px;
    font-weight: 760;
  }

  .setup-fields { display: grid; gap: 18px; }
  .setup-fields.two-columns { grid-template-columns: minmax(0, 1fr) minmax(0, 1fr); }
  .full-width { grid-column: 1 / -1; }

  label { min-width: 0; display: flex; flex-direction: column; gap: 7px; color: var(--wa-text-main); font-size: 13px; font-weight: 680; }
  label small { min-height: 17px; color: var(--wa-text-muted); font-size: 11.5px; font-weight: 500; line-height: 1.45; }
  label small.field-error { color: var(--wa-danger); }

  input, select {
    width: 100%;
    min-width: 0;
    min-height: 44px;
    border: 1px solid var(--wa-border-soft);
    border-radius: var(--wa-radius-md);
    background: var(--wa-surface-flat);
    color: var(--wa-text-strong);
    padding: 0 12px;
    font: 600 14px var(--wa-font-sans);
    outline: none;
    transition: border-color var(--wa-duration-fast) var(--wa-ease), box-shadow var(--wa-duration-fast) var(--wa-ease), background var(--wa-duration-fast) var(--wa-ease);
  }

  input:hover, select:hover { border-color: var(--wa-border-strong); }
  input:focus-visible, select:focus-visible { border-color: var(--wa-border-focus); box-shadow: 0 0 0 3px var(--wa-accent-soft); }
  input[aria-invalid='true'] { border-color: var(--wa-danger-muted); box-shadow: 0 0 0 3px var(--wa-danger-soft); }
  input:disabled, select:disabled { cursor: not-allowed; background: var(--wa-surface-inset); color: var(--wa-text-subtle); }
  input::placeholder { color: var(--wa-text-subtle); opacity: 1; }

  .mode-grid { display: grid; grid-template-columns: minmax(0, 1fr) minmax(0, 1fr); gap: 12px; }
  .mode-card {
    min-height: 118px;
    display: grid;
    grid-template-columns: 20px minmax(0, 1fr);
    align-items: start;
    gap: 10px;
    padding: 16px;
    border: 1px solid var(--wa-border-soft);
    border-radius: var(--wa-radius-lg);
    background: var(--wa-surface-panel);
    cursor: pointer;
  }
  .mode-card:hover { border-color: var(--wa-border-strong); }
  .mode-card.selected { border-color: var(--wa-accent); background: var(--wa-accent-soft); }
  .mode-card input { width: 18px; min-height: 18px; margin: 2px 0 0; accent-color: var(--wa-accent); }
  .mode-card span { display: flex; flex-direction: column; gap: 8px; }
  .mode-card strong { color: var(--wa-text-strong); font-size: 14px; }
  .mode-card small { color: var(--wa-text-muted); font-size: 12px; line-height: 1.55; }

  .confirmation-section { padding: 26px 0 8px; border-bottom: 1px solid var(--wa-border-divider); }
  .section-heading h3 { margin-bottom: 8px; color: var(--wa-text-strong); font-size: 20px; letter-spacing: -0.02em; }
  .section-heading > p:last-child { max-width: 68ch; margin-bottom: 20px; color: var(--wa-text-muted); font-size: 13px; line-height: 1.65; }

  .setup-summary-list { margin: 0; display: grid; grid-template-columns: minmax(0, 1fr) minmax(0, 1fr); }
  .setup-summary-list div { min-width: 0; display: grid; grid-template-columns: 92px minmax(0, 1fr); gap: 10px; padding: 12px 0; border-top: 1px solid var(--wa-border-divider); }
  .setup-summary-list div:nth-child(odd) { padding-right: 18px; }
  .setup-summary-list dt { color: var(--wa-text-muted); font-size: 11.5px; }
  .setup-summary-list dd { min-width: 0; margin: 0; color: var(--wa-text-strong); font: 620 12px var(--wa-font-mono); overflow-wrap: anywhere; }

  .legacy-facts, .decision-recorded p { margin: 0 0 16px; color: var(--wa-text-muted); font-size: 12.5px; line-height: 1.65; }
  .decision-recorded { padding: 13px 14px; border: 1px solid var(--wa-border-divider); border-radius: var(--wa-radius-md); background: var(--wa-success-soft); }
  .decision-recorded strong { color: var(--wa-success); font-size: 13px; }
  .decision-recorded p { margin: 6px 0 0; }

  .operation-progress { padding: 22px 0; border-bottom: 1px solid var(--wa-border-divider); }
  .progress-heading { display: flex; align-items: flex-start; justify-content: space-between; gap: 20px; }
  .progress-heading h3 { margin-bottom: 5px; color: var(--wa-text-strong); font-size: 15px; }
  .progress-heading p { margin: 0; color: var(--wa-text-muted); font-size: 12px; line-height: 1.5; }
  .progress-heading > strong { color: var(--wa-accent-strong); font: 760 13px var(--wa-font-mono); }
  .progress-track { height: 7px; margin-top: 14px; border-radius: var(--wa-radius-pill); background: var(--wa-neutral-soft); overflow: hidden; }
  .progress-track span { display: block; height: 100%; border-radius: inherit; background: var(--wa-accent-fill); }
  .operation-progress > small { display: block; margin-top: 9px; color: var(--wa-text-muted); font-size: 11px; }

  .setup-check { padding: 24px 0; border-bottom: 1px solid var(--wa-border-divider); }
  .setup-check-header { display: flex; align-items: flex-start; justify-content: space-between; gap: 18px; margin-bottom: 16px; }
  .setup-check h3 { margin-bottom: 4px; color: var(--wa-text-strong); font-size: 15px; }
  .setup-check p { margin: 0; color: var(--wa-text-muted); font-size: 12px; }
  .check-list { display: grid; grid-template-columns: minmax(0, 1fr) minmax(0, 1fr); gap: 10px 18px; }
  .check-row { min-width: 0; display: grid; grid-template-columns: 26px minmax(0, 1fr); gap: 10px; align-items: start; padding: 10px 0; }
  .check-symbol { width: 24px; height: 24px; display: grid; place-items: center; border-radius: var(--wa-radius-pill); background: var(--wa-warning-soft); color: var(--wa-warning); font-weight: 850; }
  .check-row.passed .check-symbol { background: var(--wa-success-soft); color: var(--wa-success); }
  .check-row div { min-width: 0; display: flex; flex-direction: column; gap: 3px; }
  .check-row strong { color: var(--wa-text-strong); font-size: 12.5px; }
  .check-row small { color: var(--wa-text-muted); font-size: 11.5px; line-height: 1.4; overflow-wrap: anywhere; }

  .setup-feedback { display: grid; grid-template-columns: 112px minmax(0, 1fr); gap: 12px; margin-top: 20px; padding: 13px 14px; border-radius: var(--wa-radius-md); background: var(--wa-neutral-soft); color: var(--wa-text-main); font-size: 12.5px; line-height: 1.5; }
  .setup-feedback strong { color: var(--wa-text-strong); }
  .setup-feedback.tone-success { background: var(--wa-success-soft); color: var(--wa-success); }
  .setup-feedback.tone-error { background: var(--wa-danger-soft); color: var(--wa-danger); }
  .setup-feedback.tone-working { background: var(--wa-info-soft); color: var(--wa-info-strong); }

  .setup-actions { display: flex; align-items: center; justify-content: space-between; gap: 24px; padding-top: 22px; }
  .setup-actions p { max-width: 56ch; margin: 0; color: var(--wa-text-muted); font-size: 11.5px; line-height: 1.5; }
  .action-group { flex: 0 0 auto; display: flex; align-items: center; gap: 10px; }
  button { min-height: 44px; border-radius: var(--wa-radius-md); padding: 0 16px; border: 1px solid transparent; font: 760 13px var(--wa-font-sans); white-space: nowrap; cursor: pointer; outline: none; }
  button:focus-visible { box-shadow: 0 0 0 3px var(--wa-accent-soft); }
  button:active:not(:disabled) { transform: translateY(1px); }
  button:disabled { opacity: 0.5; cursor: not-allowed; }
  .primary-action { flex: 0 0 auto; min-width: 138px; background: var(--wa-accent-fill); border-color: var(--wa-accent-fill); color: var(--wa-accent-fill-ink); box-shadow: var(--wa-shadow-glow); }
  .primary-action:hover:not(:disabled) { background: var(--wa-accent-fill-hover); border-color: var(--wa-accent-fill-hover); }
  .secondary-action { background: var(--wa-surface-flat); border-color: var(--wa-border-strong); color: var(--wa-text-main); }
  .secondary-action:hover:not(:disabled) { border-color: var(--wa-accent); color: var(--wa-accent-strong); }

  @media (max-width: 900px) {
    .database-setup { grid-template-columns: minmax(0, 1fr); padding: 32px 24px; }
    .setup-intro { gap: 24px; }
    h1 { max-width: 18ch; }
    .setup-summary, .setup-security-note { max-width: 65ch; }
    .setup-workspace { max-width: none; justify-self: stretch; }
  }

  @media (max-width: 768px) {
    .setup-workspace-header, .setup-check-header, .setup-actions { flex-direction: column; align-items: stretch; }
    .setup-fields.two-columns, .mode-grid, .check-list { grid-template-columns: minmax(0, 1fr); }
    .full-width { grid-column: auto; }
    .setup-status-chip { align-self: flex-start; }
    .setup-feedback { grid-template-columns: minmax(0, 1fr); gap: 4px; }
    .setup-summary-list { grid-template-columns: minmax(0, 1fr); }
    .setup-summary-list div:nth-child(odd) { padding-right: 0; }
    .action-group { width: 100%; flex-direction: column-reverse; }
    .primary-action, .secondary-action { width: 100%; }
  }

  @media (max-width: 600px) {
    .database-setup { padding: 20px 14px; gap: 24px; }
    .setup-brand { font-size: 14px; }
    h1 { font-size: 32px; }
    .setup-workspace { padding: 18px 14px; border-radius: var(--wa-radius-lg); }
  }

  @media (prefers-reduced-motion: reduce) {
    input, select, button { transition: none; }
  }

  @media (prefers-reduced-transparency: reduce) {
    .setup-workspace { background: var(--wa-surface-panel); backdrop-filter: none; }
  }
</style>
