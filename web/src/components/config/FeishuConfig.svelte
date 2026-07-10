<script lang="ts">
  import { createEventDispatcher } from 'svelte';
  import TextInput from '../shared/TextInput.svelte';
  import Switch from '../shared/Switch.svelte';
  import Button from '../shared/Button.svelte';
  import Alert from '../shared/Alert.svelte';
  import { resetSettingsWorkspaceScroll } from '../../lib/settings-ui';

  const dispatch = createEventDispatcher();

  export let config: {
    enabled?: boolean;
    app_id: string;
    app_secret: string;
    bot: { enabled: boolean; chat_group: string };
    bitable: {
      enabled: boolean;
      app_token: string;
      table_id: string;
      status_column: string;
      task_id_column: string;
    };
  } = {
    enabled: false,
    app_id: '',
    app_secret: '',
    bot: { enabled: false, chat_group: '' },
    bitable: {
      enabled: false,
      app_token: '',
      table_id: '',
      status_column: '任务状态',
      task_id_column: '任务ID'
    }
  };

  export let saving = false;
  export let saveError = '';
  export let saveSuccess = false;
  export let lastUpdated = '';

  let currentStep = 1;
  const steps = ['应用凭证', '机器人设置', '多维表格', '保存应用'];
  let editing = false;
  let showAppSecretEditor = !config.app_secret;
  let showBitableTokenEditor = !config.bitable?.app_token;

  // Step 1 states
  let enabled = config.enabled ?? false;
  let appID = config.app_id || '';
  let appSecret = config.app_secret || '';
  let testingCreds = false;
  let credsError = '';
  let credsSuccess = '';
  let credsDetails = '';

  // Step 2 states
  let botEnabled = config.bot?.enabled ?? false;
  let chatGroup = config.bot?.chat_group || '';

  // Step 3 states
  let bitableEnabled = config.bitable?.enabled ?? false;
  let appToken = config.bitable?.app_token || '';
  let tableID = config.bitable?.table_id || '';
  let statusCol = config.bitable?.status_column || '任务状态';
  let taskIDCol = config.bitable?.task_id_column || '任务ID';
  let testingBitable = false;
  let bitableError = '';
  let bitableSuccess = '';
  let bitableDetails = '';
  $: isConfigured = !!(appID || appSecret || botEnabled || bitableEnabled || chatGroup || appToken || tableID);
  let configurationIssues: string[] = [];
  let healthTone = 'unchecked';
  let healthLabel = '未检测';
  let healthMessage = '尚未执行健康检查。';

  $: configurationIssues = enabled
    ? [
        ...(!appID ? ['App ID'] : []),
        ...(!appSecret ? ['App Secret'] : []),
        ...(botEnabled && !chatGroup ? ['机器人群组 Chat ID'] : []),
        ...(bitableEnabled && !appToken ? ['Bitable App Token'] : []),
        ...(bitableEnabled && !tableID ? ['Bitable Table ID'] : [])
      ]
    : [];
  $: healthTone = saveSuccess
    ? 'success'
    : !isConfigured
      ? 'incomplete'
      : !enabled
        ? 'unchecked'
        : testingCreds || testingBitable
          ? 'checking'
          : credsError || bitableError
            ? 'error'
            : configurationIssues.length > 0
              ? 'incomplete'
              : credsSuccess || bitableSuccess
                ? 'success'
                : 'unchecked';
  $: healthLabel = saveSuccess
    ? '配置已保存'
    : !isConfigured
      ? '尚未配置'
      : !enabled
        ? '同步已停用'
        : testingCreds || testingBitable
          ? '检测中'
          : credsError || bitableError
            ? '检测失败'
            : configurationIssues.length > 0
              ? '配置不完整'
              : credsSuccess || bitableSuccess
                ? '检测通过'
                : '未检测';
  $: healthMessage = saveSuccess
    ? '飞书凭证、机器人和多维表格设置已写入新的配置版本。'
    : !isConfigured
      ? '尚未录入飞书应用凭证。进入编辑后完成基础连接配置。'
      : !enabled
        ? '配置已保留，飞书消息与表格同步当前不会运行。'
        : testingCreds || testingBitable
          ? '正在验证飞书接口与已启用的数据通道。'
          : credsError || bitableError
            ? credsError || bitableError
            : configurationIssues.length > 0
              ? `需要补齐：${configurationIssues.join('、')}。`
              : credsSuccess || bitableSuccess
                ? credsSuccess || bitableSuccess
                : '尚未执行健康检查，不默认判定为已就绪。';
  $: if (!editing && !saveSuccess) {
    enabled = config.enabled ?? false;
    appID = config.app_id || '';
    appSecret = config.app_secret || '';
    botEnabled = config.bot?.enabled ?? false;
    chatGroup = config.bot?.chat_group || '';
    bitableEnabled = config.bitable?.enabled ?? false;
    appToken = config.bitable?.app_token || '';
    tableID = config.bitable?.table_id || '';
    statusCol = config.bitable?.status_column || '任务状态';
    taskIDCol = config.bitable?.task_id_column || '任务ID';
    showAppSecretEditor = !config.app_secret;
    showBitableTokenEditor = !config.bitable?.app_token;
  }

  function formatUpdated(value: string) {
    if (!value) return '暂无版本记录';
    const date = new Date(value);
    if (Number.isNaN(date.getTime())) return value;
    return date.toLocaleString();
  }

  function openEditor() {
    editing = true;
    currentStep = 1;
    credsError = '';
    credsSuccess = '';
    credsDetails = '';
    bitableError = '';
    bitableSuccess = '';
    bitableDetails = '';
    resetSettingsWorkspaceScroll();
  }

  function finishClose() {
    editing = false;
    dispatch('close');
  }

  async function testCredentials() {
    if (!appID || !appSecret) {
      credsError = '请填写 App ID 和 App Secret';
      return;
    }

    testingCreds = true;
    credsError = '';
    credsSuccess = '';
    credsDetails = '';

    try {
      const res = await fetch('/api/config/test', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          type: 'feishu',
          feishu: {
            app_id: appID,
            app_secret: appSecret,
            bot: { enabled: false, chat_group: '' },
            bitable: { enabled: false, app_token: '', table_id: '', status_column: '', task_id_column: '' }
          }
        })
      });

      if (!res.ok) throw new Error(`HTTP 错误: ${res.status}`);
      const data = await res.json();

      if (data.success) {
        credsSuccess = '凭证校验通过，成功获取 Feishu 接口访问令牌。';
        credsDetails = data.details || '';
      } else {
        credsError = data.message;
        credsDetails = data.details || '';
      }
    } catch (e: any) {
      credsError = '连接测试请求失败，请检查网络或后端状态';
      credsDetails = e.message;
    } finally {
      testingCreds = false;
    }
  }

  async function testBitable() {
    if (!appToken || !tableID) {
      bitableError = '请填写多维表格的 App Token 和 Table ID';
      return;
    }

    testingBitable = true;
    bitableError = '';
    bitableSuccess = '';
    bitableDetails = '';

    try {
      const res = await fetch('/api/config/test', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          type: 'feishu',
          feishu: {
            app_id: appID,
            app_secret: appSecret,
            bot: { enabled: false, chat_group: '' },
            bitable: {
              enabled: true,
              app_token: appToken,
              table_id: tableID,
              status_column: statusCol,
              task_id_column: taskIDCol
            }
          }
        })
      });

      if (!res.ok) throw new Error(`HTTP 错误: ${res.status}`);
      const data = await res.json();

      if (data.success) {
        bitableSuccess = '多维表格鉴权及连通性测试成功！';
        bitableDetails = data.details || '';
      } else {
        bitableError = data.message;
        bitableDetails = data.details || '';
      }
    } catch (e: any) {
      bitableError = '连通性测试失败';
      bitableDetails = e.message;
    } finally {
      testingBitable = false;
    }
  }

  async function saveConfig(isToggle = false) {

    const updatedFeishu = {
      enabled: enabled,
      app_id: appID,
      app_secret: appSecret,
      bot: {
        enabled: botEnabled,
        chat_group: chatGroup
      },
      bitable: {
        enabled: bitableEnabled,
        app_token: appToken,
        table_id: tableID,
        status_column: statusCol,
        task_id_column: taskIDCol
      }
    };

    dispatch('save', {
      key: 'feishu',
      data: updatedFeishu,
      isToggle: isToggle
    });
  }

  function nextStep() {
    if (currentStep === 1) {
      if (!appID || !appSecret) {
        credsError = '请填写 App ID 和 App Secret';
        return;
      }
      currentStep = 2;
    } else if (currentStep === 2) {
      currentStep = 3;
    } else if (currentStep === 3) {
      currentStep = 4;
    }
  }

  function prevStep() {
    if (currentStep > 1) {
      currentStep -= 1;
    }
  }
</script>

<div class="scw-workbench">
  {#if saveSuccess}
    <Alert type="success" title="配置已保存" message="飞书凭证、机器人和多维表格设置已更新，版本审计会记录本次变更。" />
  {/if}

  {#if !editing && isConfigured}
    <section class="scw-overview" aria-label="飞书配置状态">
      <header class="scw-header">
        <div>
          <span class="scw-kicker">消息同步</span>
          <h4>飞书配置状态</h4>
          <p>只读摘要保留敏感字段脱敏展示，链路状态来自真实检查或明确的配置完整性判断。</p>
        </div>
        <div class="scw-toggle">
          <span>启用飞书同步</span>
          <Switch id="feishu-overview-toggle" label="启用飞书同步" bind:checked={enabled} on:change={() => saveConfig(true)} />
        </div>
      </header>

      <div class="scw-status tone-{healthTone}">
        <div class="scw-status-main">
          <span>链路健康</span>
          <strong>{healthLabel}</strong>
        </div>
        <p>{healthMessage}</p>
      </div>

      <div class="scw-read-grid">
        <div class="scw-read-item">
          <span>Feishu App ID</span>
          <strong class="font-mono">{appID || '-'}</strong>
        </div>
        <div class="scw-read-item">
          <span>App Secret 密钥</span>
          <strong>{appSecret ? '已配置，已脱敏' : '未配置'}</strong>
        </div>
        <div class="scw-read-item">
          <span>机器人通知</span>
          <strong>{botEnabled ? '已开启' : '未开启'} {chatGroup ? `(${chatGroup})` : ''}</strong>
        </div>
        <div class="scw-read-item">
          <span>多维表格同步</span>
          <strong>{bitableEnabled ? '已开启' : '未开启'}</strong>
        </div>
        {#if bitableEnabled}
          <div class="scw-read-item">
            <span>Bitable App Token</span>
            <strong class="font-mono">{appToken ? appToken.substring(0, 10) + '...' : '-'}</strong>
          </div>
          <div class="scw-read-item">
            <span>Table ID</span>
            <strong class="font-mono">{tableID || '-'}</strong>
          </div>
          <div class="scw-read-item">
            <span>任务 ID 列名 / 状态列名</span>
            <strong class="font-sans">{taskIDCol} / {statusCol}</strong>
          </div>
        {/if}
        <div class="scw-read-item">
          <span>最近更新时间</span>
          <strong>{formatUpdated(lastUpdated)}</strong>
        </div>
      </div>

      {#if credsDetails || bitableDetails}
        <pre class="scw-details scw-mono">{credsDetails || bitableDetails}</pre>
      {/if}

      <div class="scw-actions">
        <Button variant="secondary" loading={testingCreds} on:click={testCredentials}>健康检查</Button>
        {#if bitableEnabled}
          <Button variant="secondary" loading={testingBitable} on:click={testBitable}>Bitable 检查</Button>
        {/if}
        <Button variant="primary" on:click={openEditor}>编辑配置</Button>
      </div>
    </section>
  {:else}
    <section class="scw-editor" aria-label="编辑飞书配置">
      <div class="scw-stepper" aria-label="配置步骤">
        {#each steps as step, index}
          {@const stepNum = index + 1}
          <button
            type="button"
            class:active={currentStep === stepNum}
            class:completed={currentStep > stepNum}
            on:click={() => currentStep = stepNum}
          >
            <span class="scw-step-index">{stepNum}</span>
            {step}
          </button>
        {/each}
      </div>

    {#if currentStep === 1}
    <div class="scw-step-body">
      <div class="scw-info">
        <h4>飞书开放平台应用凭证</h4>
        <p>请输入在飞书开放平台 (open.feishu.cn) 申请的<strong>企业自建应用</strong>凭证，以授权 well-ambient 调用飞书机器人及多维表格 API。</p>
      </div>

      <TextInput
        id="feishu-appid"
        label="Feishu App ID"
        placeholder="例如: cli_a1b2c3d4e5f6g7h8"
        bind:value={appID}
        required
        error={credsError}
      />

      {#if showAppSecretEditor}
        <TextInput
          id="feishu-secret"
          label="Feishu App Secret"
          placeholder="请输入应用的 App Secret"
          type="password"
          bind:value={appSecret}
          required
        />
      {:else}
        <div class="scw-credential">
          <div class="scw-credential-copy">
            <span>Feishu App Secret</span>
            <strong>已配置，当前默认脱敏折叠</strong>
          </div>
          <button type="button" on:click={() => showAppSecretEditor = true}>编辑凭证/高级配置</button>
        </div>
      {/if}

      {#if credsSuccess}
        <Alert type="success" title="凭证校验通过" message={credsSuccess}>
          {#if credsDetails}
            <pre class="scw-details scw-mono">{credsDetails}</pre>
          {/if}
        </Alert>
      {/if}

      {#if credsError && credsDetails}
        <Alert type="error" title="校验失败" message={credsError}>
          <pre class="scw-details scw-mono">{credsDetails}</pre>
        </Alert>
      {/if}

      <div class="scw-actions">
        <Button variant="secondary" loading={testingCreds} on:click={testCredentials}>
          测试凭证有效性
        </Button>
        <Button variant="primary" on:click={nextStep}>
          下一步
        </Button>
      </div>
    </div>
  {:else if currentStep === 2}
    <div class="scw-step-body">
      <div class="scw-info">
        <h4>飞书机器人群组推送设置</h4>
        <p>启用群聊机器人后，well-ambient 系统将在捕获到 GitLab 代码状态变更（如 Merge Request 合并、分支推送）后，向群聊内无感推送同步交互卡片。</p>
      </div>

      <Switch
        id="bot-enabled"
        label="启用飞书机器人通知功能"
        bind:checked={botEnabled}
      />

      {#if botEnabled}
        <div class="scw-form-stack">
          <TextInput
            id="chat-group"
            label="默认通知群组 Chat ID"
            placeholder="例如: oc_chatgroupid12345"
            bind:value={chatGroup}
            required={botEnabled}
            helperText="请输入飞书群聊的 chat_id。可通过在飞书群聊中添加您的自建机器人并向其发送 @ 消息或通过飞书开发者工具获取该 ID。"
          />
          
          <div class="scw-empty">
            <span class="notice-title">注意事项:</span>
            <ul class="notice-list">
              <li>请在飞书群设置 -> 群机器人中，添加您当前绑定的自建应用。</li>
              <li>如果机器人未加入该群聊，即使配置了正确的 Chat ID，消息也会推送失败。</li>
            </ul>
          </div>
        </div>
      {/if}

      <div class="scw-actions">
        <Button variant="ghost" on:click={prevStep}>上一步</Button>
        <Button variant="primary" on:click={nextStep}>下一步</Button>
      </div>
    </div>
  {:else if currentStep === 3}
    <div class="scw-step-body">
      <div class="scw-info">
        <h4>飞书多维表格 (Bitable) 数据同步</h4>
        <p>启用此项，开发状态提交流转时，well-ambient 会自动更新映射的飞书多维数据表的列状态，实现多维看板双向无感式同步。</p>
      </div>

      <Switch
        id="bitable-enabled"
        label="启用多维表格双向同步"
        bind:checked={bitableEnabled}
      />

      {#if bitableEnabled}
        <div class="scw-form-stack">
          {#if showBitableTokenEditor}
            <TextInput
              id="bitable-token"
              label="多维表格 App Token"
              placeholder="例如: bascnYourBitableAppToken"
              bind:value={appToken}
              required={bitableEnabled}
              helperText="多维表格的 App Token (URL 中 /base/ 后面的一串字符)。"
            />
          {:else}
            <div class="scw-credential">
              <div class="scw-credential-copy">
                <span>多维表格 App Token</span>
                <strong>已配置，当前默认脱敏折叠</strong>
              </div>
              <button type="button" on:click={() => showBitableTokenEditor = true}>编辑凭证/高级配置</button>
            </div>
          {/if}

          <TextInput
            id="bitable-tableid"
            label="数据表 Table ID"
            placeholder="例如: tblYourTableID"
            bind:value={tableID}
            required={bitableEnabled}
            helperText="表格标签页内的 Table ID (URL 中 table= 后面的一串字符)。"
          />

          <div class="scw-form-grid">
            <TextInput
              id="col-status"
              label="状态列字段名 (Status Column)"
              placeholder="任务状态"
              bind:value={statusCol}
              required={bitableEnabled}
            />
            <TextInput
              id="col-taskid"
              label="任务 ID 列字段名 (Task ID Column)"
              placeholder="任务ID"
              bind:value={taskIDCol}
              required={bitableEnabled}
            />
          </div>

          {#if bitableSuccess}
            <Alert type="success" title="连通性测试通过" message={bitableSuccess}>
              {#if bitableDetails}
                <pre class="scw-details scw-mono">{bitableDetails}</pre>
              {/if}
            </Alert>
          {/if}

          {#if bitableError}
            <Alert type="error" title="测试失败" message={bitableError}>
              {#if bitableDetails}
                <pre class="scw-details scw-mono">{bitableDetails}</pre>
              {/if}
            </Alert>
          {/if}

          <div class="scw-section-actions">
            <Button variant="secondary" loading={testingBitable} on:click={testBitable}>
              测试多维表格连接
            </Button>
          </div>
        </div>
      {/if}

      <div class="scw-actions">
        <Button variant="ghost" on:click={prevStep}>上一步</Button>
        <Button variant="primary" on:click={nextStep}>下一步</Button>
      </div>
    </div>
  {:else if currentStep === 4}
    <div class="scw-step-body">
      <div class="scw-info">
        <h4>飞书集成配置摘要</h4>
        <p>确认无误后点击下方按钮保存并应用配置：</p>
      </div>

      <div class="scw-summary">
        <div class="scw-summary-row">
          <span class="summary-label">App ID:</span>
          <span class="summary-value font-mono">{appID}</span>
        </div>
        <div class="scw-summary-row">
          <span class="summary-label">飞书消息推送机器人:</span>
          <span class="summary-value">
            {#if botEnabled}
              <span class="text-success">已启用</span> (群 ID: <span class="font-mono">{chatGroup}</span>)
            {:else}
              <span class="text-muted">已禁用</span>
            {/if}
          </span>
        </div>
        <div class="scw-summary-row">
          <span class="summary-label">多维表格数据同步:</span>
          <span class="summary-value">
            {#if bitableEnabled}
              <span class="text-success">已启用</span> (Table: <span class="font-mono">{tableID}</span>)
            {:else}
              <span class="text-muted">已禁用</span>
            {/if}
          </span>
        </div>
      </div>

      {#if saveError}
        <Alert type="error" title="保存失败" message={saveError} />
      {/if}

      <div class="scw-actions">
        <Button variant="ghost" on:click={prevStep} disabled={saving}>上一步</Button>
        <Button variant="secondary" on:click={finishClose} disabled={saving}>取消</Button>
        <Button variant="primary" loading={saving} on:click={() => saveConfig(false)}>
          保存并应用
        </Button>
      </div>
    </div>
  {/if}
    </section>
  {/if}
</div>

<style>
  .notice-title {
    color: var(--wa-text-strong, #0d1722);
    font-size: 12px;
    font-weight: 780;
  }

  .notice-list {
    margin: 2px 0 0;
    padding-left: 18px;
    color: var(--wa-text-muted, #667789);
    font-size: 12px;
    line-height: 1.6;
  }

  .text-success {
    color: var(--wa-success, #04966f);
  }

  .text-muted {
    color: var(--wa-text-muted, #667789);
  }

  .font-mono {
    font-family: var(--wa-font-mono, ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace);
  }

  .font-sans {
    font-family: var(--wa-font-sans, -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif);
  }
</style>
