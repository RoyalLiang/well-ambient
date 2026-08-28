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
    can_migrate_legacy: boolean;
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

  interface ProgressStep {
    key: string;
    label: string;
    state: 'pending' | 'active' | 'completed';
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
    inspection.can_migrate_legacy
  );
  $: legacyDecisionReady = !legacyChoiceApplies || legacySQLite.decision_recorded || Boolean(legacyDecision);
  $: canContinue = connectionIsCurrent && inspectionIsSafe && testState !== 'working';
  $: canApply = canContinue && legacyDecisionReady && applyState !== 'working';
  $: migrationSelected = legacySQLite.decision === 'migrate' || legacyDecision === 'migrate';
  $: progressPercent = operation ? operationProgressPercent(operation) : 0;
  $: progressSteps = operation ? buildProgressSteps(operation, migrationSelected) : [];

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
    if (value.schema_state === 'well_ambient' && value.can_migrate_legacy && legacySQLite.available) {
      return '目标数据库仅含安装初始化数据，可以迁移本地 SQLite。';
    }
    if (value.schema_state === 'well_ambient') return '目标数据库结构完整，将直接接入，不改写已有数据。';
    return '目标 schema 已有未知表，自动安装已阻止。';
  }

  function operationStatusLabel(value: OperationStatus): string {
    if (value.state === 'failed') {
      if (value.stage === 'copying_data') {
        return value.table
          ? `迁移 ${value.table} 时停止（${value.tables_completed}/${value.tables_total}，已复制 ${value.rows_copied} 行）。`
          : '迁移本地 SQLite 数据时停止。';
      }
      const failedLabels: Record<string, string> = {
        queued: '任务启动失败。',
        checking_database: '检查目标数据库时停止。',
        creating_database: '创建目标数据库时停止。',
        preparing_schema: '创建 PostgreSQL 结构时停止。',
        initializing_schema: '初始化 PostgreSQL 结构时停止。',
        verifying_counts: '核对迁移数据时停止。',
        analyzing: '更新 PostgreSQL 统计信息时停止。'
      };
      return failedLabels[value.stage] || '数据库安装任务已停止。';
    }
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

  function operationProgressPercent(value: OperationStatus): number {
    if (value.stage === 'copying_data') {
      const copiedRatio = value.tables_total > 0
        ? value.tables_completed / value.tables_total
        : 0;
      return Math.min(80, 35 + Math.round(copiedRatio * 45));
    }
    if (value.stage === 'failed') {
      const copiedRatio = value.tables_total > 0
        ? value.tables_completed / value.tables_total
        : 0;
      return value.tables_total > 0 ? Math.min(80, 35 + Math.round(copiedRatio * 45)) : 8;
    }
    const stageProgress: Record<string, number> = {
      queued: 2,
      checking_database: 8,
      creating_database: 18,
      preparing_schema: 28,
      initializing_schema: 34,
      verifying_counts: 90,
      analyzing: 96,
      completed: 100
    };
    return stageProgress[value.stage] ?? 2;
  }

  function buildProgressSteps(value: OperationStatus, includeMigration: boolean): ProgressStep[] {
    const definitions = [
      { key: 'database', label: '检查目标库' },
      { key: 'schema', label: '初始化结构' },
      ...(includeMigration ? [{ key: 'migration', label: '迁移 SQLite' }] : []),
      { key: 'verification', label: '核对数据' },
      { key: 'finish', label: '启动服务' }
    ];
    const stageKey: Record<string, string> = {
      queued: 'database',
      checking_database: 'database',
      creating_database: 'database',
      preparing_schema: 'schema',
      initializing_schema: 'schema',
      copying_data: includeMigration ? 'migration' : 'schema',
      verifying_counts: 'verification',
      analyzing: 'verification',
      completed: 'finish'
    };
    const activeKey = stageKey[value.stage] || (value.state === 'failed' && value.table && includeMigration ? 'migration' : 'database');
    const activeIndex = definitions.findIndex((item) => item.key === activeKey);
    return definitions.map((item, index) => ({
      ...item,
      state: value.stage === 'completed' || index < activeIndex
        ? 'completed'
        : index === activeIndex ? 'active' : 'pending'
    }));
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
  <header class="setup-header">
    <h1 id="setup-title">配置运行数据库</h1>
    <p>连接 PostgreSQL，完成后进入登录。</p>
  </header>

  <form class="setup-workspace" aria-busy={testState === 'working' || applyState === 'working'} on:submit={applyDatabase}>
    <nav class="setup-steps" aria-label="数据库安装步骤">
      <span class:active={step === 1} class:completed={step === 2}><b>1</b> 连接</span>
      <span class:active={step === 2}><b>2</b> 确认与迁移</span>
    </nav>

    {#if step === 1}
      <fieldset class="setup-section connection-section" disabled={applyState === 'working' || applyState === 'success'}>
        <legend>PostgreSQL 连接</legend>
        <div class="setup-fields two-columns">
          <label>
            <span>主机名或 IP *</span>
            <input id="setup-host" bind:value={host} on:input={markConnectionDirty} aria-invalid={fieldErrors.host ? 'true' : undefined} autocomplete="off" />
            {#if fieldErrors.host}<small class="field-error">{fieldErrors.host}</small>{/if}
          </label>
          <label>
            <span>端口 *</span>
            <input id="setup-port" type="number" min="1" max="65535" bind:value={port} on:input={markConnectionDirty} aria-invalid={fieldErrors.port ? 'true' : undefined} />
            {#if fieldErrors.port}<small class="field-error">{fieldErrors.port}</small>{/if}
          </label>
          <label>
            <span>目标数据库 *</span>
            <input id="setup-database" bind:value={database} on:input={markConnectionDirty} aria-invalid={fieldErrors.database ? 'true' : undefined} autocomplete="off" />
            {#if fieldErrors.database}<small class="field-error">{fieldErrors.database}</small>{/if}
          </label>
          <label>
            <span>用户名 *</span>
            <input id="setup-username" bind:value={username} on:input={markConnectionDirty} aria-invalid={fieldErrors.username ? 'true' : undefined} autocomplete="off" />
            {#if fieldErrors.username}<small class="field-error">{fieldErrors.username}</small>{/if}
          </label>
          <label>
            <span>数据库密码 *</span>
            <input id="setup-password" type="password" bind:value={password} on:input={markConnectionDirty} aria-invalid={fieldErrors.password ? 'true' : undefined} autocomplete="new-password" />
            {#if fieldErrors.password}<small class="field-error">{fieldErrors.password}</small>{/if}
          </label>
          <label>
            <span>安装令牌 *</span>
            <input id="setup-token" type="password" bind:value={setupToken} on:input={markConnectionDirty} aria-invalid={fieldErrors.setup_token ? 'true' : undefined} autocomplete="new-password" />
            {#if fieldErrors.setup_token}<small class="field-error">{fieldErrors.setup_token}</small>{/if}
          </label>
        </div>

        <details class="advanced-options">
          <summary>高级连接选项</summary>
          <div class="setup-fields two-columns advanced-fields">
            <label>
              <span>维护数据库</span>
              <input id="setup-maintenance-database" bind:value={maintenanceDatabase} on:input={markConnectionDirty} aria-invalid={fieldErrors.maintenance_database ? 'true' : undefined} autocomplete="off" />
              {#if fieldErrors.maintenance_database}<small class="field-error">{fieldErrors.maintenance_database}</small>{/if}
            </label>
            <label>
              <span>SSL 模式</span>
              <div class="select-control">
                <select id="setup-ssl-mode" bind:value={sslMode} on:change={markConnectionDirty}>
                  <option value="disable">disable</option>
                  <option value="prefer">prefer</option>
                  <option value="require">require</option>
                  <option value="verify-ca">verify-ca</option>
                  <option value="verify-full">verify-full</option>
                </select>
                <svg aria-hidden="true" class="select-chevron" viewBox="0 0 12 8" focusable="false">
                  <path d="M1.5 1.5 6 6l4.5-4.5"></path>
                </svg>
              </div>
            </label>
            <label class="full-width">
              <span>根证书路径</span>
              <input id="setup-root-cert" bind:value={sslRootCert} on:input={markConnectionDirty} placeholder="/etc/well-ambient/ca.pem" aria-invalid={fieldErrors.ssl_root_cert ? 'true' : undefined} autocomplete="off" />
              {#if fieldErrors.ssl_root_cert}<small class="field-error">{fieldErrors.ssl_root_cert}</small>{/if}
            </label>
          </div>
        </details>

        {#if legacySQLite.available}
          <div class="legacy-detected" role="note">
            <span aria-hidden="true">✓</span>
            <p><strong>已检测到本地 SQLite</strong><small>{formatBytes(legacySQLite.size_bytes)} · {legacySQLite.table_count || 0} 张表，连接测试后可选择是否迁移。</small></p>
          </div>
        {/if}
      </fieldset>
    {:else if operation}
      <section class="operation-progress" class:is-failed={operation.state === 'failed'} aria-labelledby="progress-title">
        <div class="progress-heading">
          <div>
            <h2 id="progress-title">{operation.state === 'failed' ? '安装已停止' : operation.state === 'completed' ? '安装完成' : '正在安装'}</h2>
            <p>{operationStatusLabel(operation)}</p>
          </div>
          <strong>{operation.state === 'failed' ? '失败' : `${progressPercent}%`}</strong>
        </div>
        <div class="progress-track" role="progressbar" aria-valuemin="0" aria-valuemax="100" aria-valuenow={progressPercent}>
          <span style={`transform: scaleX(${progressPercent / 100})`}></span>
        </div>
        <ol class="progress-stages" aria-label="安装阶段">
          {#each progressSteps as item}
            <li class:active={item.state === 'active'} class:completed={item.state === 'completed'} aria-current={item.state === 'active' ? 'step' : undefined}>
              <span aria-hidden="true">{item.state === 'completed' ? '✓' : ''}</span>
              {item.label}
            </li>
          {/each}
        </ol>
        <div class="progress-metrics">
          {#if operation.table}<span>当前表 <strong>{operation.table}</strong></span>{/if}
          {#if operation.tables_total > 0}<span>表进度 <strong>{operation.tables_completed}/{operation.tables_total}</strong></span>{/if}
          {#if operation.rows_copied > 0}<span>已复制 <strong>{operation.rows_copied} 行</strong></span>{/if}
        </div>
      </section>
    {:else}
      <section class="confirmation-section" aria-labelledby="operation-title">
        <h2 id="operation-title">{inspection ? schemaStateLabel(inspection.schema_state) : '确认安装'}</h2>
        <p>{inspection ? operationSummary(inspection) : '返回上一步重新测试连接。'}</p>
        <dl class="setup-summary-list">
          <div><dt>目标</dt><dd>{host.trim()}:{port} / {database.trim()}</dd></div>
          <div><dt>版本</dt><dd>PostgreSQL {inspection?.server_version || '—'}</dd></div>
        </dl>
      </section>

      {#if legacyChoiceApplies}
        <fieldset class="setup-section legacy-section" disabled={applyState === 'working' || applyState === 'success'}>
          <legend>本地 SQLite</legend>
          {#if legacySQLite.decision_recorded}
            <div class="decision-recorded">
              已选择：{legacySQLite.decision === 'migrate' ? '迁移本地数据' : '不迁移本地数据'}
            </div>
          {:else}
            <p class="legacy-facts">{formatBytes(legacySQLite.size_bytes)} · {legacySQLite.table_count || 0} 张表 · 该选择只记录一次</p>
            <div class="mode-grid">
              <label class="mode-card" class:selected={legacyDecision === 'migrate'}>
                <input type="radio" name="legacy-decision" value="migrate" bind:group={legacyDecision} />
                <span><strong>迁移本地数据</strong><small>复制到 PostgreSQL 并核对行数</small></span>
              </label>
              <label class="mode-card" class:selected={legacyDecision === 'skip'}>
                <input type="radio" name="legacy-decision" value="skip" bind:group={legacyDecision} />
                <span><strong>不迁移</strong><small>保留当前 PostgreSQL 数据</small></span>
              </label>
            </div>
          {/if}
        </fieldset>
      {:else if legacySQLite.available && inspection?.required_operation === 'connect_existing'}
        <p class="legacy-inline-note">目标 PostgreSQL 已包含业务数据，本地 SQLite 迁移已停用以避免覆盖。</p>
      {/if}
    {/if}

    <div class="setup-feedback tone-{applyState === 'success' ? 'success' : testState === 'error' || applyState === 'error' ? 'error' : testState === 'working' || applyState === 'working' ? 'working' : 'neutral'}" role="status" aria-live="polite">
      <span>{statusMessage}</span>
    </div>

    <footer class="setup-actions">
      {#if restartTimedOut}
        <button type="button" class="primary-action" on:click={() => window.location.reload()}>刷新页面</button>
      {:else if step === 1}
        <button type="button" class="secondary-action" on:click={testConnection} disabled={testState === 'working' || applyState === 'working'}>
          {testState === 'working' ? '正在测试' : '测试连接'}
        </button>
        <button type="button" class="primary-action" on:click={continueToConfirmation} disabled={!canContinue}>下一步</button>
      {:else if !operation}
        <button type="button" class="secondary-action" on:click={() => step = 1} disabled={applyState === 'working'}>返回</button>
        <button type="submit" class="primary-action" disabled={!canApply}>
          {migrationSelected ? '开始迁移' : inspection?.required_operation === 'connect_existing' ? '确认接入' : '开始安装'}
        </button>
      {/if}
    </footer>
  </form>
</main>

<style>
  /* finesse · register=product · shell=centered-setup-workflow · SOUL=4 SPECTACLE=1 DENSITY=7 */
  .database-setup {
    min-height: 100dvh;
    display: grid;
    grid-template-columns: minmax(0, 1fr);
    justify-items: center;
    align-content: start;
    gap: 20px;
    padding: clamp(28px, 4vw, 52px) clamp(20px, 4vw, 48px);
    background: var(--wa-bg-ambient), var(--wa-bg-page);
    color: var(--wa-text-main);
  }

  .setup-header {
    width: 100%;
    max-width: 760px;
    min-width: 0;
    text-align: center;
  }

  h1, h2, p { margin-top: 0; }

  h1 {
    max-width: 24ch;
    margin: 0 auto 7px;
    color: var(--wa-text-strong);
    font-family: var(--wa-font-display);
    font-size: 30px;
    line-height: 1.18;
    letter-spacing: -0.035em;
    text-wrap: balance;
    overflow-wrap: anywhere;
    min-width: 0;
  }

  .setup-header p {
    margin: 0 auto;
    color: var(--wa-text-muted);
    font-size: 13px;
    line-height: 1.5;
  }

  .setup-workspace {
    width: 100%;
    max-width: 760px;
    min-width: 0;
    justify-self: center;
    background: var(--wa-glass-panel-strong);
    border: 1px solid var(--wa-glass-outline);
    border-top-color: var(--wa-glass-highlight);
    border-radius: var(--wa-radius-xl);
    box-shadow: var(--wa-shadow-panel);
    backdrop-filter: blur(18px) saturate(124%);
    padding: clamp(20px, 3vw, 30px);
  }

  .setup-steps {
    display: grid;
    grid-template-columns: minmax(0, 1fr) minmax(0, 1fr);
    gap: 1px;
    margin: -4px 0 22px;
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

  fieldset {
    min-width: 0;
    margin: 0;
    padding: 2px 0 22px;
    border: 0;
    border-bottom: 1px solid var(--wa-border-divider);
  }

  legend {
    margin-bottom: 15px;
    padding: 0;
    color: var(--wa-text-strong);
    font-size: 15px;
    font-weight: 760;
  }

  .setup-fields { display: grid; gap: 16px; }
  .setup-fields.two-columns { grid-template-columns: minmax(0, 1fr) minmax(0, 1fr); }
  .full-width { grid-column: 1 / -1; }

  label { min-width: 0; display: flex; flex-direction: column; gap: 7px; color: var(--wa-text-main); font-size: 13px; font-weight: 680; }
  label small { color: var(--wa-text-muted); font-size: 11.5px; font-weight: 500; line-height: 1.45; }
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

  .select-control { position: relative; min-width: 0; }
  .select-control select {
    appearance: none;
    padding-right: 40px;
  }
  .select-chevron {
    position: absolute;
    top: 50%;
    right: 15px;
    width: 12px;
    height: 8px;
    color: var(--wa-text-muted);
    pointer-events: none;
    transform: translateY(-50%);
  }
  .select-chevron path { fill: none; stroke: currentColor; stroke-linecap: round; stroke-linejoin: round; stroke-width: 1.8; }
  .select-control:has(select:disabled) .select-chevron { color: var(--wa-text-subtle); }

  .advanced-options {
    margin-top: 18px;
    border-top: 1px solid var(--wa-border-divider);
  }

  .advanced-options summary {
    width: fit-content;
    margin-top: 15px;
    color: var(--wa-text-muted);
    font-size: 12.5px;
    font-weight: 700;
    cursor: pointer;
  }

  .advanced-options summary:hover { color: var(--wa-accent-strong); }
  .advanced-options summary:focus-visible { border-radius: var(--wa-radius-sm); outline: 3px solid var(--wa-accent-soft); outline-offset: 4px; }
  .advanced-fields { margin-top: 16px; }

  .legacy-detected {
    display: grid;
    grid-template-columns: 26px minmax(0, 1fr);
    gap: 10px;
    align-items: center;
    margin-top: 18px;
    padding: 12px 14px;
    border-radius: var(--wa-radius-md);
    background: var(--wa-info-soft);
    color: var(--wa-info-strong);
  }

  .legacy-detected > span {
    width: 24px;
    height: 24px;
    display: grid;
    place-items: center;
    border-radius: var(--wa-radius-pill);
    background: var(--wa-surface-flat);
    font-size: 12px;
    font-weight: 820;
  }

  .legacy-detected p { min-width: 0; margin: 0; display: flex; flex-direction: column; gap: 2px; }
  .legacy-detected strong { font-size: 12.5px; }
  .legacy-detected small { color: var(--wa-text-muted); font-size: 11.5px; line-height: 1.45; }

  .mode-grid { display: grid; grid-template-columns: minmax(0, 1fr) minmax(0, 1fr); gap: 12px; }
  .mode-card {
    min-height: 88px;
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

  .confirmation-section { padding: 2px 0 18px; border-bottom: 1px solid var(--wa-border-divider); }
  .confirmation-section h2 { margin-bottom: 7px; color: var(--wa-text-strong); font-size: 20px; letter-spacing: -0.02em; }
  .confirmation-section > p { margin-bottom: 17px; color: var(--wa-text-muted); font-size: 12.5px; line-height: 1.55; }

  .setup-summary-list { margin: 0; display: grid; grid-template-columns: minmax(0, 1fr) minmax(0, 1fr); }
  .setup-summary-list div { min-width: 0; display: grid; grid-template-columns: 54px minmax(0, 1fr); gap: 10px; padding: 11px 0; border-top: 1px solid var(--wa-border-divider); }
  .setup-summary-list div:nth-child(odd) { padding-right: 18px; }
  .setup-summary-list dt { color: var(--wa-text-muted); font-size: 11.5px; }
  .setup-summary-list dd { min-width: 0; margin: 0; color: var(--wa-text-strong); font: 620 12px var(--wa-font-mono); overflow-wrap: anywhere; }

  .legacy-section { padding-top: 20px; }
  .legacy-facts { margin: -5px 0 14px; color: var(--wa-text-muted); font: 600 12px var(--wa-font-mono); }
  .decision-recorded { padding: 12px 14px; border-radius: var(--wa-radius-md); background: var(--wa-success-soft); color: var(--wa-success); font-size: 12.5px; font-weight: 700; }
  .legacy-inline-note { margin: 18px 0 0; padding: 12px 14px; border-radius: var(--wa-radius-md); background: var(--wa-neutral-soft); color: var(--wa-text-muted); font-size: 12px; }

  .operation-progress { padding: 4px 0 20px; border-bottom: 1px solid var(--wa-border-divider); }
  .progress-heading { display: flex; align-items: flex-start; justify-content: space-between; gap: 20px; }
  .progress-heading h2 { margin-bottom: 6px; color: var(--wa-text-strong); font-size: 21px; letter-spacing: -0.02em; }
  .progress-heading p { margin: 0; color: var(--wa-text-muted); font-size: 12.5px; line-height: 1.5; }
  .progress-heading > strong { color: var(--wa-accent-strong); font: 760 14px var(--wa-font-mono); }
  .progress-track { height: 8px; margin-top: 18px; border-radius: var(--wa-radius-pill); background: var(--wa-neutral-soft); overflow: hidden; }
  .progress-track span { display: block; width: 100%; height: 100%; transform-origin: left center; border-radius: inherit; background: var(--wa-accent-fill); transition: transform var(--wa-duration-normal) var(--wa-ease); }
  .operation-progress.is-failed .progress-heading > strong { color: var(--wa-danger); }
  .operation-progress.is-failed .progress-track span { background: var(--wa-danger); }

  .progress-stages {
    display: grid;
    grid-template-columns: repeat(auto-fit, minmax(96px, 1fr));
    gap: 8px;
    margin: 18px 0 0;
    padding: 0;
    list-style: none;
  }

  .progress-stages li {
    min-width: 0;
    display: flex;
    align-items: center;
    gap: 7px;
    color: var(--wa-text-subtle);
    font-size: 11.5px;
    font-weight: 650;
    white-space: nowrap;
  }

  .progress-stages li > span {
    width: 18px;
    height: 18px;
    flex: 0 0 auto;
    display: grid;
    place-items: center;
    border: 1px solid var(--wa-border-soft);
    border-radius: var(--wa-radius-pill);
    background: var(--wa-surface-flat);
    font-size: 10px;
  }

  .progress-stages li.active { color: var(--wa-accent-strong); }
  .progress-stages li.active > span { border-color: var(--wa-accent); box-shadow: inset 0 0 0 4px var(--wa-accent-soft); }
  .progress-stages li.completed { color: var(--wa-text-main); }
  .progress-stages li.completed > span { border-color: var(--wa-accent-fill); background: var(--wa-accent-fill); color: var(--wa-accent-fill-ink); }

  .progress-metrics { display: flex; flex-wrap: wrap; gap: 8px 18px; min-height: 18px; margin-top: 16px; color: var(--wa-text-muted); font-size: 11.5px; }
  .progress-metrics strong { color: var(--wa-text-main); font-family: var(--wa-font-mono); font-weight: 650; overflow-wrap: anywhere; }

  .setup-feedback { margin-top: 18px; padding: 11px 13px; border-radius: var(--wa-radius-md); background: var(--wa-neutral-soft); color: var(--wa-text-main); font-size: 12.5px; line-height: 1.5; }
  .setup-feedback.tone-success { background: var(--wa-success-soft); color: var(--wa-success); }
  .setup-feedback.tone-error { background: var(--wa-danger-soft); color: var(--wa-danger); }
  .setup-feedback.tone-working { background: var(--wa-info-soft); color: var(--wa-info-strong); }

  .setup-actions { display: flex; align-items: center; justify-content: flex-end; gap: 10px; padding-top: 18px; }
  button { min-height: 44px; border-radius: var(--wa-radius-md); padding: 0 16px; border: 1px solid transparent; font: 760 13px var(--wa-font-sans); white-space: nowrap; cursor: pointer; outline: none; }
  button:focus-visible { box-shadow: 0 0 0 3px var(--wa-accent-soft); }
  button:active:not(:disabled) { transform: translateY(1px); }
  button:disabled { opacity: 0.5; cursor: not-allowed; }
  .primary-action { flex: 0 0 auto; min-width: 138px; background: var(--wa-accent-fill); border-color: var(--wa-accent-fill); color: var(--wa-accent-fill-ink); box-shadow: var(--wa-shadow-glow); }
  .primary-action:hover:not(:disabled) { background: var(--wa-accent-fill-hover); border-color: var(--wa-accent-fill-hover); }
  .secondary-action { background: var(--wa-surface-flat); border-color: var(--wa-border-strong); color: var(--wa-text-main); }
  .secondary-action:hover:not(:disabled) { border-color: var(--wa-accent); color: var(--wa-accent-strong); }

  @media (max-width: 900px) {
    .database-setup { padding: 32px 24px; }
  }

  @media (max-width: 768px) {
    .setup-fields.two-columns, .mode-grid { grid-template-columns: minmax(0, 1fr); }
    .full-width { grid-column: auto; }
    .setup-summary-list { grid-template-columns: minmax(0, 1fr); }
    .setup-summary-list div:nth-child(odd) { padding-right: 0; }
    .progress-stages { grid-template-columns: minmax(0, 1fr) minmax(0, 1fr); }
  }

  @media (max-width: 600px) {
    .database-setup { padding: 20px 14px; gap: 18px; }
    h1 { font-size: 26px; }
    .setup-workspace { padding: 18px 14px; border-radius: var(--wa-radius-lg); }
    .setup-steps span { padding: 0 10px; }
    .setup-actions { display: grid; grid-template-columns: minmax(0, 1fr) minmax(0, 1fr); }
    .setup-actions > :only-child { grid-column: 1 / -1; }
    .primary-action, .secondary-action { width: 100%; min-width: 0; }
  }

  @media (prefers-reduced-motion: reduce) {
    input, select, button, .progress-track span { transition: none; }
  }

  @media (prefers-reduced-transparency: reduce) {
    .setup-workspace { background: var(--wa-surface-panel); backdrop-filter: none; }
  }
</style>
