<script lang="ts">
  import { createEventDispatcher, onDestroy, onMount, tick } from 'svelte';
  import EmailReportPreview from './EmailReportPreview.svelte';
  import EmailTemplateGallery from './EmailTemplateGallery.svelte';
  import TextInput from '../shared/TextInput.svelte';
  import Switch from '../shared/Switch.svelte';
  import Button from '../shared/Button.svelte';
  import Alert from '../shared/Alert.svelte';
  import MultiSelect from '../shared/MultiSelect.svelte';
  import { watchProjectCatalog, type ProjectCatalogOption } from '../../lib/project-catalog';
  import { defaultSMTP, defaultDailyEmail, defaultConfluence, emailRequest, emailTemplateStyleName, emailTemplateSignature, type SMTPConfig, type DailyJiraEmailConfig, type EmailTemplate, type EmailTemplateEntry, type DailyJiraProjectGroup, type ConfluenceConfig, type DailyJiraEmailRun } from '../../lib/email-config';

  interface EditableProjectGroup extends DailyJiraProjectGroup {
    draft_id: string;
  }
  type EditableDailyConfig = Omit<DailyJiraEmailConfig, 'project_groups' | 'confluence'> & { project_groups: EditableProjectGroup[]; confluence: ConfluenceConfig };

  export let smtp: SMTPConfig = defaultSMTP();
  export let daily: DailyJiraEmailConfig = defaultDailyEmail();
  export let canWrite = false;
  export let coreMembers: string[] = [];
  export let jiraProjects: string[] = [];
  export let jqlFallback = false;
  export let saving = false;
  export let saveError = '';
  export let saveSuccess = false;
  export let saveSuccessKey: string | null = null;
  export let currentUserEmail = '';
  export let lastUpdated = '';
  let generatorOpen = false;
  const dispatch = createEventDispatcher();
  const tabs = ['发信服务', 'Jira 早报', '邮件模板'];
  let tab = 0;
  let smtpDraft = defaultSMTP();
  let dailyDraft: EditableDailyConfig = { ...defaultDailyEmail(), project_groups: [], confluence: defaultConfluence() };
  let projectGroupSequence = 0;
  const nextProjectGroupID = () => `email-project-group-${++projectGroupSequence}`;
  const editableGroups = (groups: DailyJiraProjectGroup[] = []): EditableProjectGroup[] => groups.map(group => ({
    draft_id: nextProjectGroupID(),
    name: group.name || '',
    owners: [...(group.owners || [])],
    projects: [...(group.projects || [])]
  }));
  const editableDaily = (value: DailyJiraEmailConfig): EditableDailyConfig => ({
    ...defaultDailyEmail(),
    ...value,
    template: { ...defaultDailyEmail().template, ...value.template },
    confluence: { ...defaultConfluence(), ...value.confluence },
    project_groups: editableGroups(value.project_groups || [])
  });
  let recipientsText = '';
  let smtpDirty = false;
  let dailyDirty = false;
  let loadedSMTP: SMTPConfig | undefined;
  let loadedDaily: DailyJiraEmailConfig | undefined;
  let passwordEditing = false;
  let confluenceTokenEditing = false;
  let confluenceTesting = false;
  let confluenceValidation = false;
  let confluenceRevision = 0;
  let confluenceResult: { success: boolean; message: string; revision: number } | null = null;
  let confluenceController: AbortController | undefined;
  $: savedConfluenceParent = (daily.confluence?.parent_page_url ?? defaultConfluence().parent_page_url).trim();
  $: confluenceParentChanged = dailyDraft.confluence.parent_page_url.trim() !== savedConfluenceParent;
  $: confluenceNeedsToken = confluenceParentChanged && dailyDraft.confluence.token === '__configured__';
  $: confluenceErrors = confluenceValidation
    ? validateConfluence(dailyDraft.confluence, savedConfluenceParent)
    : { parent: '', token: '' };
  $: confluenceTestStale = !!confluenceResult && confluenceResult.revision !== confluenceRevision;
  let recipient = currentUserEmail;
  let requirements = '';
  let templates: EmailTemplateEntry[] = [];
  let templateLoading = false;
  let templateError = '';
  let deletingTemplateId = '';
  let viewedTemplate: EmailTemplateEntry | null = null;
  let viewIntent = 0;
  let submittedViewIntent = 0;
  let templateUndo: EmailTemplate | null = null;
  $: appliedTemplateSignature = emailTemplateSignature(dailyDraft.template);
  $: templateLibraryFull = templates.filter(entry => !entry.builtin).length >= 30;
  let portNotice = '';
  function setHandshake(mode: SMTPConfig['tls_mode']) {
    const ports = { starttls: 587, tls: 465, none: 25 };
    const previous = smtpDraft.tls_mode;
    const autoPort = !smtpDraft.port || smtpDraft.port === ports[previous];
    smtpDraft = { ...smtpDraft, tls_mode: mode, port: autoPort ? ports[mode] : smtpDraft.port };
    portNotice = autoPort ? `端口已匹配为 ${ports[mode]}，可按服务商要求修改。` : `保留自定义端口 ${smtpDraft.port}，请确认与握手类型匹配。`;
    dirtySMTP();
  }
  async function loadTemplates() {
    if (templateLoading) return;
    templateLoading = true; templateError = '';
    try { const result = await emailRequest<{templates: EmailTemplateEntry[]}>('/api/daily-jira-email/templates'); templates = result.templates; }
    catch (e) { templateError = e instanceof Error ? e.message : '模板图库加载失败'; }
    finally { templateLoading = false; }
  }
  async function viewTemplate(entry: EmailTemplateEntry) {
    viewIntent++; viewedTemplate = entry;
    await tick();
    const panel = document.getElementById('email-template-preview');
    panel?.scrollIntoView({ block: 'start', behavior: matchMedia('(prefers-reduced-motion: reduce)').matches ? 'auto' : 'smooth' });
    panel?.focus({ preventScroll: true });
  }
  function useTemplate(entry: EmailTemplateEntry) {
    if (!canWrite || saving || generating || deletingTemplateId || templateLoading) return;
    viewIntent++;
    if (emailTemplateSignature(entry.template) === appliedTemplateSignature) { viewedTemplate = entry; return; }
    templateUndo = { ...dailyDraft.template };
    dailyDraft.template = { ...entry.template };
    viewedTemplate = entry; dirtyDaily();
    notice = `已将「${entry.name}」应用到草稿。保存后用于发信，可撤销本次应用。`;
  }
  function undoTemplate() {
    if (!templateUndo || saving) return;
    viewIntent++; dailyDraft.template = templateUndo; templateUndo = null; viewedTemplate = null; dirtyDaily();
    const signature = (value: DailyJiraEmailConfig) => JSON.stringify([value.enabled, value.recipients || [], value.timezone, value.send_time, value.include_commits, emailTemplateSignature(value.template), (value.project_groups || []).map(({name, owners, projects}) => ({name, owners, projects})), { ...defaultConfluence(), ...value.confluence }]);
    const restored = { ...dailyDraft, recipients: [...(dailyDraft.recipients || [])] };
    dailyDirty = signature(restored) !== signature(daily);
    notice = '已恢复应用前的模板草稿。';
  }
  async function removeTemplate(entry: EmailTemplateEntry): Promise<boolean> {
    if (!canWrite || entry.builtin || deletingTemplateId || generating || templateLoading) return false;
    deletingTemplateId = entry.id; templateError = '';
    try {
      await emailRequest(`/api/daily-jira-email/templates/${encodeURIComponent(entry.id)}`, undefined, undefined, 'DELETE');
      templates = templates.filter(item => item.id !== entry.id);
      if (viewedTemplate?.id === entry.id) { viewIntent++; viewedTemplate = null; }
      notice = `已删除候选「${entry.name}」。`; return true;
    } catch (e) { templateError = e instanceof Error ? e.message : '删除失败，请重试'; return false; }
    finally { deletingTemplateId = ''; }
  }
  async function proposeDefaultTemplate() {
    if (!templates.length) await loadTemplates();
    const entry = templates.find(item => item.id === 'builtin-brief');
    if (entry) { await viewTemplate(entry); notice = '正在查看默认模板，点击使用后才替换草稿。'; error = ''; }
  }
  let generating = false;
  let controller: AbortController | undefined;
  let testing = false;
  let previewing = false;
  let sending = false;
  let error = '';
  let notice = '';
  let smtpResult = '';
  let smtpTestSnapshot = '';
  let preview: { subject: string; html: string; date: string; timezone: string; warnings: string[] } | null = null;
  let previewSnapshot = '';
  let reportDate = '';
  let runs: DailyJiraEmailRun[] = [];
  let retryingDate = '';
  let runsError = '';
  let historyOpen = false;
  let submitted: 'smtp' | 'daily_jira_email' | null = null;
  let pendingSMTP: SMTPConfig | null = null;

  let candidateOwners: string[] = [];
  let catalogProjects: ProjectCatalogOption[] = [];
  let catalogError = '';
  let catalogLoading = true;
  let refreshCatalog: () => void = () => {};
  $: allOwnerOptions = Array.from(new Set([
    ...coreMembers,
    ...candidateOwners,
    ...dailyDraft.project_groups.flatMap(group => group.owners || [])
  ])).filter(Boolean).sort().map(name => ({ value: name, label: name }));
  $: projectNames = new Map(catalogProjects.map(project => [project.project_key, project.project_name]));
  $: allProjectOptions = Array.from(new Set([
    ...catalogProjects.map(project => project.project_key),
    ...jiraProjects,
    ...dailyDraft.project_groups.flatMap(group => group.projects || [])
  ].map(project => project.trim().toUpperCase()))).filter(Boolean).sort().map(project => ({
    value: project,
    label: project,
    meta: projectNames.get(project) && projectNames.get(project) !== project ? projectNames.get(project) : ''
  }));
  function projectFullName(key: string) {
    const name = projectNames.get(key.trim().toUpperCase());
    return name && name !== key.trim().toUpperCase() ? name : '待从 Jira 同步';
  }

  // --- Schedule Time & Timezone controls ---
  const commonTimezones = [
    { value: 'Asia/Shanghai', label: 'Asia/Shanghai (UTC+08:00 · 北京 / 上海)' },
    { value: 'Asia/Hong_Kong', label: 'Asia/Hong_Kong (UTC+08:00 · 香港)' },
    { value: 'Asia/Tokyo', label: 'Asia/Tokyo (UTC+09:00 · 东京)' },
    { value: 'Asia/Singapore', label: 'Asia/Singapore (UTC+08:00 · 新加坡)' },
    { value: 'UTC', label: 'UTC (UTC+00:00 · 协调世界时)' },
    { value: 'Europe/London', label: 'Europe/London (UTC+00:00 · 伦敦)' },
    { value: 'Europe/Berlin', label: 'Europe/Berlin (UTC+01:00 · 柏林)' },
    { value: 'America/New_York', label: 'America/New_York (UTC-05:00 · 纽约)' },
    { value: 'America/Los_Angeles', label: 'America/Los_Angeles (UTC-08:00 · 洛杉矶)' }
  ];

  const commonTimePresets = ['09:00', '09:30', '10:00', '18:00'];
  const SEND_TIME_PATTERN = /^([01]\d|2[0-3]):[0-5]\d$/;

  let customTimezoneMode = false;
  let sendTimeError = '';
  $: {
    if (dailyDraft.timezone && !commonTimezones.some(tz => tz.value === dailyDraft.timezone)) {
      customTimezoneMode = true;
    }
  }

  function handleTimezoneSelect(e: Event) {
    const val = (e.target as HTMLSelectElement).value;
    if (val === '__custom__') {
      customTimezoneMode = true;
    } else {
      customTimezoneMode = false;
      dailyDraft.timezone = val;
      dirtyDaily();
    }
  }

  function setTimePreset(preset: string) {
    if (!canWrite || saving) return;
    dailyDraft.send_time = preset;
    sendTimeError = '';
    dirtyDaily();
  }

  function handleSendTimeInput(event: Event) {
    const input = event.target as HTMLInputElement;
    const inputEvent = event as InputEvent;
    const raw = input.value.replace(/[^\d:]/g, '').slice(0, 5);

    // Only auto-insert while digits are being typed; editing/backspace keeps the caret stable.
    if (inputEvent.inputType === 'insertText' && /^\d+$/.test(input.value) && input.value.length >= 3) {
      const caret = input.selectionStart ?? input.value.length;
      input.value = `${input.value.slice(0, 2)}:${input.value.slice(2)}`;
      input.setSelectionRange(caret + 1, caret + 1);
    } else {
      input.value = raw;
    }

    dailyDraft.send_time = input.value;
    sendTimeError = '';
    dirtyDaily();
  }

  function normalizeSendTime(event: Event) {
    const input = event.target as HTMLInputElement;
    const raw = input.value.trim();
    let hourText = '';
    let minuteText = '';

    if (raw.includes(':')) {
      const [first, second = '0'] = raw.split(':');
      hourText = first;
      minuteText = second;
    } else if (/^\d{1,2}$/.test(raw)) {
      hourText = raw;
      minuteText = '0';
    } else if (/^\d{3,4}$/.test(raw)) {
      hourText = raw.slice(0, 2);
      minuteText = raw.slice(2);
    }

    if (!/^\d{1,2}$/.test(hourText) || !/^\d{1,2}$/.test(minuteText)) {
      sendTimeError = '时间需为 00:00–23:59';
      return;
    }

    const hour = Number(hourText);
    const minute = Number(minuteText);
    if (hour > 23 || minute > 59) {
      sendTimeError = '时间需为 00:00–23:59';
      return;
    }

    const value = `${String(hour).padStart(2, '0')}:${String(minute).padStart(2, '0')}`;
    input.value = value;
    dailyDraft.send_time = value;
    sendTimeError = '';
    dirtyDaily();
  }

  // --- Recipient Tag Input controls ---
  let newRecipientInput = '';
  let recipientInputError = '';

  function addRecipient(email: string): boolean {
    const trimmed = email.trim();
    if (!trimmed) return false;
    const emailRegex = /^[^\s@]+@[^\s@]+\.[^\s@]+$/;
    if (!emailRegex.test(trimmed)) {
      recipientInputError = `"${trimmed}" 邮箱格式不正确`;
      return false;
    }
    recipientInputError = '';
    const current = dailyDraft.recipients || [];
    if (current.includes(trimmed)) {
      recipientInputError = '该收件人已存在';
      return false;
    }
    if (current.length >= 100) {
      recipientInputError = '收件人最多添加 100 个';
      return false;
    }
    dailyDraft.recipients = [...current, trimmed];
    recipientsText = dailyDraft.recipients.join('\n');
    dirtyDaily();
    return true;
  }

  function removeRecipient(index: number) {
    if (!canWrite || saving) return;
    const current = [...(dailyDraft.recipients || [])];
    current.splice(index, 1);
    dailyDraft.recipients = current;
    recipientsText = current.join('\n');
    recipientInputError = '';
    dirtyDaily();
  }

  function clearAllRecipients() {
    if (!canWrite || saving) return;
    dailyDraft.recipients = [];
    recipientsText = '';
    recipientInputError = '';
    dirtyDaily();
  }

  function handleRecipientInputKeydown(event: KeyboardEvent) {
    if (event.isComposing) return;
    if (['Enter', ',', ';', '，', '；'].includes(event.key)) {
      event.preventDefault();
      commitRecipientInput();
    } else if (event.key === 'Backspace' && !newRecipientInput && (dailyDraft.recipients || []).length > 0) {
      removeRecipient((dailyDraft.recipients || []).length - 1);
    }
  }

  function handleRecipientInput(event: Event) {
    if (/[,;，；\s]/.test(newRecipientInput)) {
      commitRecipientInput();
    }
  }

  function commitRecipientInput() {
    if (!newRecipientInput.trim()) return;
    const parts = newRecipientInput.split(/[\s,;，；]+/);
    const failedParts: string[] = [];
    let failureMessage = '';
    for (const part of parts) {
      if (part.trim()) {
        if (!addRecipient(part)) {
          failedParts.push(part.trim());
          if (recipientInputError) failureMessage = recipientInputError;
        }
      }
    }
    if (failedParts.length) {
      newRecipientInput = failedParts.join(' ');
      recipientInputError = failureMessage || '部分收件人未添加，请检查格式或重复项。';
    } else {
      newRecipientInput = '';
    }
  }

  function handleRecipientPaste(event: ClipboardEvent) {
    const text = event.clipboardData?.getData('text');
    if (!text) return;
    const emails = text.split(/[\s,;，；\n\r]+/);
    if (emails.length > 1 || emails[0].includes('@')) {
      event.preventDefault();
      newRecipientInput = text;
      commitRecipientInput();
    }
  }
  let hoveredGroupTooltip: EditableProjectGroup | null = null;
  let tooltipTriggerRect: DOMRect | null = null;
  let tooltipHideTimeout: ReturnType<typeof setTimeout> | undefined;

  function showGroupTooltip(group: EditableProjectGroup, el: HTMLElement) {
    if (tooltipHideTimeout) {
      clearTimeout(tooltipHideTimeout);
      tooltipHideTimeout = undefined;
    }
    tooltipTriggerRect = el.getBoundingClientRect();
    hoveredGroupTooltip = group;
  }

  function scheduleHideGroupTooltip() {
    if (tooltipHideTimeout) clearTimeout(tooltipHideTimeout);
    tooltipHideTimeout = setTimeout(() => {
      hoveredGroupTooltip = null;
      tooltipTriggerRect = null;
    }, 120);
  }

  function keepGroupTooltip() {
    if (tooltipHideTimeout) {
      clearTimeout(tooltipHideTimeout);
      tooltipHideTimeout = undefined;
    }
  }

  function handleTooltipKeydown(event: KeyboardEvent) {
    if (event.key === 'Escape' && hoveredGroupTooltip) {
      hoveredGroupTooltip = null;
      tooltipTriggerRect = null;
    }
  }

  $: tooltipPos = (() => {
    if (!tooltipTriggerRect) return { top: 0, left: 0, isAbove: false };
    const r = tooltipTriggerRect;
    const spaceBelow = window.innerHeight - r.bottom;
    const isAbove = spaceBelow < 160 && r.top > spaceBelow;
    const top = isAbove ? Math.max(10, r.top - 8) : r.bottom + 6;
    const left = Math.max(12, Math.min(window.innerWidth - 320, r.left - 8));
    return { top, left, isAbove };
  })();

  $: projectGroupErrors = dailyDraft.project_groups.map(group => ({
    name: group.name.trim() ? '' : '请填写分组名称。',
    scope: group.owners.length || group.projects.length ? '' : '至少选择一个项目或负责人。'
  }));

  async function loadCandidateOwners() {
    try {
      const result = await emailRequest<{ owners: string[] }>('/api/daily-jira-email/candidate-owners');
      if (result && Array.isArray(result.owners)) {
        candidateOwners = result.owners;
      }
    } catch {
      // Non-blocking fallback
    }
  }

  function addProjectGroup() {
    if (!canWrite) return;
    const groups = dailyDraft.project_groups ? [...dailyDraft.project_groups] : [];
    groups.push({
      draft_id: nextProjectGroupID(),
      name: `项目分组 ${groups.length + 1}`,
      owners: [],
      projects: []
    });
    dailyDraft = { ...dailyDraft, project_groups: groups };
    dirtyDaily();
  }

  function removeProjectGroup(index: number) {
    if (!canWrite) return;
    const groups = [...(dailyDraft.project_groups || [])];
    groups.splice(index, 1);
    dailyDraft = { ...dailyDraft, project_groups: groups };
    dirtyDaily();
  }

  function moveProjectGroup(index: number, direction: -1 | 1) {
    if (!canWrite) return;
    const groups = [...(dailyDraft.project_groups || [])];
    const target = index + direction;
    if (target < 0 || target >= groups.length) return;
    const temp = groups[index];
    groups[index] = groups[target];
    groups[target] = temp;
    dailyDraft = { ...dailyDraft, project_groups: groups };
    dirtyDaily();
  }

  $: if (smtp && smtp !== loadedSMTP) {
    if (!smtpDirty) smtpDraft = { ...defaultSMTP(), ...smtp };
    loadedSMTP = smtp;
  }
  $: if (daily && daily !== loadedDaily) {
    if (!dailyDirty) {
      dailyDraft = editableDaily(daily);
      recipientsText = (daily.recipients || []).join('\n');
      confluenceTokenEditing = false;
      confluenceValidation = false;
      confluenceRevision++;
    }
    loadedDaily = daily;
  }
  $: if (!saving && submitted) {
    if (saveSuccess && saveSuccessKey === submitted) {
      if (submitted === 'smtp' && pendingSMTP) { smtpDraft = pendingSMTP; smtpDirty = false; passwordEditing = false; }
      if (submitted === 'daily_jira_email') {
        dailyDraft = editableDaily(daily);
        dailyDirty = false;
        confluenceTokenEditing = false;
        confluenceValidation = false;
        confluenceRevision++;
        templateUndo = null;
        if (submittedViewIntent === viewIntent) { viewIntent++; viewedTemplate = null; }
      }
      notice = '配置已保存。发信链路状态请通过测试邮件单独确认。';
    }
    pendingSMTP = null;
    submitted = null;
  }
  $: draftSettings = {
    ...dailyDraft,
    recipients: (dailyDraft.recipients || []).map(s => s.trim()).filter(Boolean),
    project_groups: (dailyDraft.project_groups || []).map(g => ({
      name: g.name.trim(),
      owners: (g.owners || []).map(o => o.trim()).filter(Boolean),
      projects: (g.projects || []).map((p: string) => p.trim()).filter(Boolean)
    })).filter(g => g.name || g.owners.length > 0 || g.projects.length > 0)
  };
  $: previewStale = !!preview && previewSnapshot !== previewSignature(reportDate, draftSettings, confluenceRevision);
  $: smtpTestStale = !!smtpResult && smtpTestSnapshot !== JSON.stringify(smtpDraft);

  function dirtySMTP() { smtpDirty = true; notice = ''; error = ''; dispatch('close'); }
  function dirtyDaily() { dailyDirty = true; notice = ''; error = ''; dispatch('close'); }
  function dirtyConfluence() {
    confluenceRevision++;
    if (!dailyDraft.confluence.enabled) confluenceValidation = false;
    dirtyDaily();
  }
  function confluenceLink(value?: string): string {
    if (!value) return '';
    try {
      const url = new URL(value);
      return ['http:', 'https:'].includes(url.protocol) && !url.username && !url.password ? url.href : '';
    } catch { return ''; }
  }
  function validateConfluence(value: ConfluenceConfig, savedParent: string) {
    return {
      parent: !value.parent_page_url.trim() ? '请填写父页面 URL。' : !confluenceLink(value.parent_page_url.trim()) ? '请输入完整的 http:// 或 https:// 父页面 URL。' : '',
      token: !value.token.trim() ? '请填写 PAT Token。' : value.token === '__configured__' && value.parent_page_url.trim() !== savedParent ? '父页面已更改，请替换 Token 后再继续。' : ''
    };
  }
  async function focusConfluenceError(errors: { parent: string; token: string }) {
    if (tab !== 1) selectTab(1);
    await tick();
    document.getElementById(errors.parent ? 'email-confluence-parent' : dailyDraft.confluence.token === '__configured__' && !confluenceTokenEditing ? 'email-confluence-credential' : 'email-confluence-token')?.focus();
  }
  async function replaceConfluenceToken() {
    if (!canWrite || saving) return;
    confluenceTokenEditing = true;
    dailyDraft.confluence.token = '';
    dirtyConfluence();
    await tick();
    document.getElementById('email-confluence-token')?.focus();
  }
  // Keep credentials out of retained preview snapshots; edits use a revision.
  function previewSignature(date: string, settings: DailyJiraEmailConfig, revision: number) {
    return JSON.stringify({ date, settings: { ...settings, confluence: { ...settings.confluence, token: undefined } }, revision });
  }
  async function testConfluence() {
    if (!canWrite || saving || confluenceTesting) return;
    await tick();
    confluenceValidation = true;
    const errors = validateConfluence(dailyDraft.confluence, savedConfluenceParent);
    if (errors.parent || errors.token) {
      await focusConfluenceError(errors);
      return;
    }
    const revision = confluenceRevision;
    const confluence = { ...dailyDraft.confluence, parent_page_url: dailyDraft.confluence.parent_page_url.trim() };
    confluenceTesting = true;
    confluenceResult = null;
    confluenceController = new AbortController();
    try {
      const result = await emailRequest<{ success: boolean; message: string }>('/api/config/test', { type: 'confluence', confluence }, confluenceController.signal);
      confluenceResult = { success: true, message: result.message || '父页面可访问。', revision };
    } catch (e) {
      if (!(e instanceof Error && e.name === 'AbortError')) confluenceResult = { success: false, message: e instanceof Error ? e.message : '连接测试失败，请重试。', revision };
    } finally {
      confluenceTesting = false;
      confluenceController = undefined;
    }
  }
  function runRecipients(raw?: string) { try { const values = JSON.parse(raw || "[]"); return Array.isArray(values) && values.length ? values.join("、") : "历史记录未保存"; } catch { return "历史记录未保存"; } }
  function runStatusLabel(status: string) {
    return ({
      sent: 'SMTP 已接受',
      sending: '发送中或结果待确认',
      syncing_confluence: '正在同步 Confluence，尚未发送邮件',
      confluence_failed: 'Confluence 同步失败，邮件未发送',
      failed_or_unknown: '发送失败或结果未知'
    } as Record<string, string>)[status] || '结果未知';
  }
  function selectTab(index: number) { viewIntent++; tab = index; if (index !== 2) viewedTemplate = null; error = ''; notice = ''; }
  async function tabKey(event: KeyboardEvent) {
    let next = tab;
    if (event.key === 'ArrowRight') next = (tab + 1) % 3;
    else if (event.key === 'ArrowLeft') next = (tab + 2) % 3;
    else if (event.key === 'Home') next = 0;
    else if (event.key === 'End') next = 2;
    else return;
    event.preventDefault(); selectTab(next); await tick(); document.getElementById(`email-tab-${next}`)?.focus();
  }
  function saveSMTP() {
    if (!canWrite || saving) return;
    submitted = 'smtp'; pendingSMTP = { ...smtpDraft, password: smtpDraft.password ? '__configured__' : '' };
    dispatch('save', { key: 'smtp', data: { ...smtpDraft } });
  }
  async function saveDaily() {
    if (!canWrite || saving) return;
    // A custom multi-select value commits on focus-out immediately before submit.
    // Let parent bindings and draftSettings settle before validating/sending.
    await tick();
    confluenceValidation = dailyDraft.confluence.enabled;
    if (dailyDraft.confluence.enabled) {
      const errors = validateConfluence(dailyDraft.confluence, savedConfluenceParent);
      if (errors.parent || errors.token) {
        await focusConfluenceError(errors);
        return;
      }
    }
    const invalidGroup = projectGroupErrors.findIndex(groupError => groupError.name || groupError.scope);
    if (invalidGroup >= 0) {
      error = projectGroupErrors[invalidGroup].name || projectGroupErrors[invalidGroup].scope;
      if (tab !== 1) selectTab(1);
      await tick();
      document.getElementById(projectGroupErrors[invalidGroup].name ? `group-name-${invalidGroup}` : `group-projects-${invalidGroup}`)?.focus();
      return;
    }
    if (sendTimeError || !SEND_TIME_PATTERN.test(dailyDraft.send_time)) {
      sendTimeError = sendTimeError || '时间需为 00:00–23:59';
      error = sendTimeError;
      if (tab !== 1) selectTab(1);
      await tick();
      document.getElementById('email-time')?.focus();
      return;
    }
    submittedViewIntent = viewIntent;
    submitted = 'daily_jira_email';
    dispatch('save', { key: 'daily_jira_email', data: draftSettings });
  }
  async function testSMTP() {
    if (!canWrite || testing) return;
    if (!recipient.trim()) { error = '请填写测试收件人。'; return; }
    const snapshot = JSON.stringify(smtpDraft);
    testing = true; error = ''; smtpResult = '';
    try {
      await emailRequest('/api/config/test', { type: 'smtp', smtp: smtpDraft, recipient: recipient.trim() });
      smtpTestSnapshot = snapshot;
      smtpResult = `SMTP 已接受发往 ${recipient.trim()} 的测试邮件（${new Date().toLocaleTimeString()}）。请在收件箱或垃圾邮件中确认。`;
    } catch (e) { error = e instanceof Error ? e.message : '测试发送失败'; }
    finally { testing = false; }
  }
  async function generate() {
    if (!canWrite || generating || templateLoading || deletingTemplateId || templateLibraryFull || !requirements.trim()) return;
    generating = true; error = ''; notice = '';
    const generationIntent = viewIntent;
    controller = new AbortController();
    try {
      const result = await emailRequest<{candidates: {name: string; template: EmailTemplate}[]}>('/api/daily-jira-email/template', { requirements }, controller.signal);
      const saved = await emailRequest<{templates: EmailTemplateEntry[]}>('/api/daily-jira-email/templates', { templates: result.candidates }, controller.signal);
      templates = [...templates, ...saved.templates];
      const showGenerated = generationIntent === viewIntent;
      if (showGenerated) { viewIntent++; viewedTemplate = saved.templates[0] || null; }
      notice = '已生成并保存 1 个自定义整版模板候选。请在图库查看并选择使用。';
      await tick();
      if (showGenerated && saved.templates[0]) document.getElementById(`email-template-view-${saved.templates[0].id}`)?.scrollIntoView({ block: 'nearest', inline: 'center' });
    } catch (e) { if (e instanceof Error && e.name === 'AbortError') notice = '已停止等待，未生成任何候选，原模板保持不变。'; else error = e instanceof Error ? e.message : '模板生成失败'; }
    finally { generating = false; controller = undefined; }
  }
  async function makePreview() {
    if (!canWrite || previewing) return;
    previewing = true; error = '';
    const body = { date: reportDate, settings: draftSettings };
    const snapshot = previewSignature(reportDate, draftSettings, confluenceRevision);
    const intent = ++viewIntent;
    try { preview = await emailRequest('/api/daily-jira-email/preview', body); previewSnapshot = snapshot; if (intent === viewIntent) viewedTemplate = null; }
    catch (e) { error = e instanceof Error ? e.message : '预览失败'; }
    finally { previewing = false; }
  }
  async function loadRuns() {
    try { runs = await emailRequest('/api/daily-jira-email/runs'); runsError = ''; }
    catch (e) { runsError = e instanceof Error ? e.message : '发送记录加载失败'; }
  }
  async function sendReportForDate(date: string) {
    if (!canWrite || saving || sending || smtpDirty || dailyDirty || !smtp.enabled || !daily.recipients?.length) return;
    sending = true; error = ''; notice = '';
    try {
      const result = await emailRequest<{message: string}>('/api/daily-jira-email/send', { date }); notice = result.message;
    } catch (e) { error = e instanceof Error ? e.message : '早报发送失败'; }
    finally { sending = false; await loadRuns(); }
  }
  async function sendReport() { await sendReportForDate(reportDate); }
  async function retryReport(run: DailyJiraEmailRun) {
    if (!(run.status === 'confluence_failed' || (run.status === 'sent' && run.confluence_status === 'failed')) || !canWrite || saving || sending || smtpDirty || dailyDirty || !smtp.enabled || !daily.recipients?.length) return;
    retryingDate = run.date;
    try { await sendReportForDate(run.date); }
    finally { retryingDate = ''; }
  }
  function resetDraft() {
    if (tab === 0) { smtpDraft = { ...defaultSMTP(), ...smtp }; smtpDirty = false; passwordEditing = false; }
    else {
      dailyDraft = editableDaily(daily);
      recipientsText = (daily.recipients || []).join('\n');
      dailyDirty = false;
      confluenceTokenEditing = false;
      confluenceValidation = false;
      confluenceRevision++;
    }
    viewIntent++; templateUndo = null; viewedTemplate = null; error = ''; notice = ''; dispatch('close');
  }
  onMount(() => {
    void loadRuns(); void loadTemplates(); void loadCandidateOwners();
    const catalog = watchProjectCatalog(
      projects => { catalogProjects = projects; catalogError = ''; catalogLoading = false; },
      message => { catalogError = message; catalogLoading = false; }
    );
    refreshCatalog = () => { catalogLoading = true; void catalog.refresh(); };
    return catalog.destroy;
  });
  onDestroy(() => {
    controller?.abort();
    confluenceController?.abort();
    if (tooltipHideTimeout) clearTimeout(tooltipHideTimeout);
  });
</script>

<svelte:window on:keydown={handleTooltipKeydown} />

<div class="email-config scw-workbench">
  <div class="scw-context-toolbar email-toolbar">
    <div class="scw-task-tabs" role="tablist" aria-label="邮件服务设置">
      {#each tabs as title, index}
        <button id={`email-tab-${index}`} role="tab" aria-selected={tab === index} aria-controls={`email-panel-${index}`} tabindex={tab === index ? 0 : -1} class:active={tab === index} on:click={() => selectTab(index)} on:keydown={tabKey}>{title}</button>
      {/each}
    </div>
    {#if smtpDirty || dailyDirty}<span class="email-save-state dirty">有未保存更改</span>{/if}
  </div>

  {#if error || saveError || notice}
    <div aria-live="polite" class="email-feedback">
      {#if error || saveError}<Alert type="error">{error || saveError}</Alert>{/if}
      {#if notice}<Alert type="success">{notice}</Alert>{/if}
    </div>
  {/if}
  {#if !canWrite}<p class="scw-help">当前为只读模式，需要配置写入权限才能保存、测试、生成或发送邮件。</p>{/if}

  <div id={`email-panel-${tab}`} class="scw-tab-panel" role="tabpanel" aria-labelledby={`email-tab-${tab}`} tabindex="0">
    {#if tab === 2}
      <EmailTemplateGallery {templates} loading={templateLoading} error={templateError} {canWrite} busy={generating || saving} deletingId={deletingTemplateId} viewedId={viewedTemplate?.id || ''} appliedSignature={appliedTemplateSignature} onView={viewTemplate} onUse={useTemplate} onDelete={removeTemplate} onRetry={loadTemplates} />
    {/if}
    <div class="email-layout">
      <div class="email-main email-pane">
        {#if tab === 0}
          <form class="scw-form-stack" on:submit|preventDefault={saveSMTP} on:input={dirtySMTP}>
            <fieldset disabled={!canWrite || saving || testing}>
              <legend class="email-sr-only">SMTP 发信服务</legend>
              <div class="scw-section-head email-pane-heading">
                <div class="scw-section-copy"><h4>发信连接</h4><p>配置邮件服务器与发件身份。</p></div>
                <div class="scw-toggle"><span>{smtpDraft.enabled ? '已启用' : '已停用'}</span><Switch id="email-service-enabled" label="启用邮件服务" bind:checked={smtpDraft.enabled} on:change={dirtySMTP} /></div>
              </div>
              <div class="scw-form-grid email-server-fields">
                <TextInput id="smtp-host" label="SMTP 主机" bind:value={smtpDraft.host} placeholder="smtp.example.com" required />
                <div class="scw-native-field"><label class="scw-native-label" for="smtp-port">端口 <span class="required">*</span></label><input id="smtp-port" class="scw-native-input" type="number" min="1" max="65535" bind:value={smtpDraft.port} required /></div>
              </div>
              <fieldset class="scw-section email-security">
                <legend class="email-sr-only">SMTP 握手类型</legend>
                <div class="scw-section-head"><div class="scw-section-copy"><h4>SMTP 握手类型</h4><p>按邮件服务商要求选择，默认验证服务器证书。</p></div></div>
                <div class="email-handshakes">
                  <label class="email-radio" class:selected={smtpDraft.tls_mode === 'starttls'}><input type="radio" name="smtp-handshake" value="starttls" checked={smtpDraft.tls_mode === 'starttls'} on:change={() => setHandshake('starttls')} /><span><strong>STARTTLS · 升级握手</strong><small>连接后升级加密 · 587</small></span></label>
                  <label class="email-radio" class:selected={smtpDraft.tls_mode === 'tls'}><input type="radio" name="smtp-handshake" value="tls" checked={smtpDraft.tls_mode === 'tls'} on:change={() => setHandshake('tls')} /><span><strong>SSL/TLS · 隐式握手</strong><small>连接即加密 · 465</small></span></label>
                  <label class="email-radio" class:selected={smtpDraft.tls_mode === 'none'}><input type="radio" name="smtp-handshake" value="none" checked={smtpDraft.tls_mode === 'none'} on:change={() => setHandshake('none')} /><span><strong>无加密 · 仅本机测试</strong><small>本机无认证接收器 · 25</small></span></label>
                </div>
                {#if portNotice}<p class="scw-help" aria-live="polite">{portNotice}</p>{/if}
                <details class="email-disclosure"><summary>连接安全说明</summary><p>隐式 SSL/TLS 实际使用 TLS 1.2 或更高版本，不启用旧 SSL。无加密只允许 localhost / loopback 且不带认证。切换时保留自定义端口。</p></details>
              </fieldset>
              <section class="scw-section">
                <div class="scw-section-head"><div class="scw-section-copy"><h4>认证与发件人</h4><p>使用 SMTP 授权码，已保存凭证不返回原文。</p></div></div>
                <div class="scw-form-grid">
                  <TextInput id="smtp-user" label="认证用户名" bind:value={smtpDraft.username} placeholder="通常为邮箱地址" />
                  <div class="scw-native-field">
                    {#if smtpDraft.password === '__configured__' && !passwordEditing}
                      <span class="scw-native-label">认证密码 / 授权码</span><div class="email-credential"><span>凭证已配置</span><Button size="small" variant="secondary" on:click={() => { passwordEditing = true; smtpDraft.password = ''; dirtySMTP(); }}>替换凭证</Button></div>
                    {:else}<TextInput id="smtp-password" label="认证密码 / 授权码" type="password" bind:value={smtpDraft.password} />{/if}
                  </div>
                  <div class="wide"><TextInput id="smtp-from" label="发件人邮箱" type="email" bind:value={smtpDraft.from} placeholder="reports@example.com" required /></div>
                </div>
                <p class="scw-help">使用服务商提供的 SMTP 授权码；本机测试接收器可留空认证信息。</p>
              </section>
            </fieldset>
            {#if canWrite}<div class="scw-actions email-actions"><span class="email-action-note">保存后应用于早报发送</span>{#if smtpDirty}<Button variant="secondary" on:click={resetDraft}>放弃更改</Button>{/if}<Button type="submit" loading={saving} disabled={testing}>保存发信服务</Button></div>{/if}
          </form>
        {:else if tab === 1}
          <form class="scw-form-stack" on:submit|preventDefault={saveDaily} on:input={dirtyDaily}>
            <fieldset disabled={!canWrite || saving}>
              <legend class="email-sr-only">每日 Jira 早报</legend>
              <div class="scw-section-head email-pane-heading">
                <div class="scw-section-copy"><h4>每日发送计划</h4><p>按指定时区发送，服务重启后可补发当天早报。</p></div>
                <div class="scw-toggle"><span>{dailyDraft.enabled ? '已启用' : '已停用'}</span><Switch id="email-daily-enabled" label="每天定时发送" bind:checked={dailyDraft.enabled} on:change={dirtyDaily} /></div>
              </div>
              <div class="scw-form-grid email-schedule-grid">
                <div class="scw-native-field email-time-field">
                  <div class="email-field-head">
                    <label class="scw-native-label" for="email-time">发送时间 <span class="required">*</span></label>
                  </div>
                  <div class="email-field-shell email-time-wrapper" class:has-error={!!sendTimeError}>
                    <svg class="email-time-icon" width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">
                      <circle cx="12" cy="12" r="10" />
                      <polyline points="12 6 12 12 16 14" />
                    </svg>
                    <input
                      id="email-time"
                      class="email-time-input"
                      type="text"
                      value={dailyDraft.send_time}
                      placeholder="HH:MM"
                      inputmode="numeric"
                      maxlength="5"
                      autocomplete="off"
                      spellcheck="false"
                      title="24 小时制 HH:MM"
                      aria-invalid={sendTimeError ? 'true' : undefined}
                      aria-describedby={sendTimeError ? 'email-time-error' : 'email-time-help'}
                      disabled={!canWrite || saving}
                      on:input={handleSendTimeInput}
                      on:blur={normalizeSendTime}
                      required
                    />
                  </div>
                  <div class="email-field-foot">
                    <div class="email-time-presets" role="group" aria-label="快捷发送时间">
                      {#each commonTimePresets as preset}
                        <button
                          type="button"
                          class="email-time-preset-pill"
                          class:is-active={dailyDraft.send_time === preset}
                          disabled={!canWrite || saving}
                          on:click={() => setTimePreset(preset)}
                        >{preset}</button>
                      {/each}
                    </div>
                    {#if sendTimeError}
                      <p id="email-time-error" class="email-control-error" role="alert">{sendTimeError}</p>
                    {:else}
                      <span id="email-time-help" class="email-field-hint">24 小时制</span>
                    {/if}
                  </div>
                </div>
                <div class="scw-native-field email-timezone-field">
                  <div class="email-field-head">
                    <label class="scw-native-label" for={customTimezoneMode ? 'email-timezone' : 'email-timezone-select'}>时区 <span class="required">*</span></label>
                  </div>
                  {#if !customTimezoneMode}
                    <div class="email-field-shell email-timezone-select-wrapper">
                      <svg class="email-timezone-icon" width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">
                        <circle cx="12" cy="12" r="10" />
                        <line x1="2" y1="12" x2="22" y2="12" />
                        <path d="M12 2a15.3 15.3 0 0 1 4 10 15.3 15.3 0 0 1-4 10 15.3 15.3 0 0 1-4-10 15.3 15.3 0 0 1 4-10z" />
                      </svg>
                      <select
                        id="email-timezone-select"
                        class="email-timezone-select"
                        value={dailyDraft.timezone}
                        disabled={!canWrite || saving}
                        on:change={handleTimezoneSelect}
                        aria-describedby="email-timezone-help"
                      >
                        {#each commonTimezones as tz}
                          <option value={tz.value}>{tz.label}</option>
                        {/each}
                        <option value="__custom__">其他自定义时区…</option>
                      </select>
                      <svg class="email-select-chevron" width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">
                        <polyline points="6 9 12 15 18 9" />
                      </svg>
                    </div>
                  {:else}
                    <div class="email-field-shell email-timezone-select-wrapper email-timezone-custom-row">
                      <svg class="email-timezone-icon" width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">
                        <circle cx="12" cy="12" r="10" />
                        <line x1="2" y1="12" x2="22" y2="12" />
                        <path d="M12 2a15.3 15.3 0 0 1 4 10 15.3 15.3 0 0 1-4 10 15.3 15.3 0 0 1-4-10 15.3 15.3 0 0 1 4-10z" />
                      </svg>
                      <input
                        id="email-timezone"
                        class="email-timezone-custom-input"
                        type="text"
                        bind:value={dailyDraft.timezone}
                        placeholder="例如 Asia/Shanghai 或 UTC"
                        autocomplete="off"
                        spellcheck="false"
                        disabled={!canWrite || saving}
                        on:input={dirtyDaily}
                        aria-describedby="email-timezone-help"
                        required
                      />
                    </div>
                  {/if}
                  <div class="email-field-foot">
                    <span id="email-timezone-help" class="email-field-hint">按此时区计算当天发送窗口。</span>
                    {#if customTimezoneMode}
                      <button type="button" class="email-timezone-toggle-btn" on:click={() => { customTimezoneMode = false; dailyDraft.timezone = 'Asia/Shanghai'; dirtyDaily(); }}>选择推荐时区</button>
                    {/if}
                  </div>
                </div>
                <div class="scw-native-field wide email-recipients-field">
                  <div class="email-field-head">
                    <label class="scw-native-label" for="email-recipient-input">
                      早报收件人
                    </label>
                    <div class="email-field-actions">
                      <span class="email-field-meta" aria-live="polite">已添加 {(dailyDraft.recipients || []).length} 位</span>
                      {#if (dailyDraft.recipients || []).length > 0 && canWrite && !saving}
                        <button type="button" class="email-recipients-clear-btn" on:click={clearAllRecipients}>清空收件人</button>
                      {/if}
                    </div>
                  </div>
                  <div
                    class="email-recipients-box"
                    class:has-error={!!recipientInputError}
                    class:is-disabled={!canWrite || saving}
                    role="group"
                    aria-label="早报收件人列表"
                  >
                    {#if (dailyDraft.recipients || []).length}
                      <ul class="email-recipient-list" aria-label="已添加收件人">
                        {#each (dailyDraft.recipients || []) as email, idx (email)}
                          <li class="email-recipient-chip" title={email}>
                            <span class="email-chip-text">{email}</span>
                            {#if canWrite && !saving}
                              <button type="button" aria-label={`移除 ${email}`} on:click={() => removeRecipient(idx)}>×</button>
                            {/if}
                          </li>
                        {/each}
                      </ul>
                    {/if}
                    {#if canWrite && !saving}
                      <div class="email-recipient-entry">
                        <input
                          id="email-recipient-input"
                          type="text"
                          class="email-recipient-inline-input"
                          bind:value={newRecipientInput}
                          placeholder="输入邮箱，回车或点击 + 添加"
                          autocomplete="off"
                          spellcheck="false"
                          aria-invalid={recipientInputError ? 'true' : undefined}
                          aria-describedby={recipientInputError ? 'email-recipients-error' : 'email-recipients-help'}
                          on:input={handleRecipientInput}
                          on:keydown={handleRecipientInputKeydown}
                          on:blur={commitRecipientInput}
                          on:paste={handleRecipientPaste}
                        />
                        <button type="button" class="email-recipient-add-btn" aria-label="添加收件人" title="添加收件人" disabled={!newRecipientInput.trim()} on:mousedown={(event) => event.preventDefault()} on:click={commitRecipientInput}>
                          <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.4" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">
                            <line x1="12" y1="5" x2="12" y2="19" />
                            <line x1="5" y1="12" x2="19" y2="12" />
                          </svg>
                        </button>
                      </div>
                    {:else}
                      <div class="email-recipient-entry is-disabled">
                        <input
                          id="email-recipient-input"
                          type="text"
                          class="email-recipient-inline-input"
                          value=""
                          placeholder="当前角色不可编辑收件人"
                          disabled
                        />
                      </div>
                    {/if}
                  </div>
                  <div class="email-field-foot">
                    {#if recipientInputError}
                      <p id="email-recipients-error" class="email-control-error" role="alert">{recipientInputError}</p>
                    {:else}
                      <span id="email-recipients-help" class="email-field-hint">支持回车、逗号、分号或批量粘贴；最多 100 个。</span>
                    {/if}
                  </div>
                </div>
              </div>
              <section class="scw-section email-confluence" aria-labelledby="email-confluence-title" on:input={dirtyConfluence}>
                <div class="scw-section-head">
                  <div class="scw-section-copy"><h4 id="email-confluence-title">Confluence 同步</h4><p>发送前按报告日期写入父页面下的子页，同日重试更新同一子页。</p></div>
                  <div class="scw-toggle"><span>{dailyDraft.confluence.enabled ? '已启用' : '已停用'}</span><Switch id="email-confluence-enabled" label="同步早报到 Confluence" bind:checked={dailyDraft.confluence.enabled} disabled={!canWrite || saving} on:change={dirtyConfluence} /></div>
                </div>
                <div class="scw-form-grid">
                  <div class="wide"><TextInput id="email-confluence-parent" label="父页面 URL" bind:value={dailyDraft.confluence.parent_page_url} disabled={!canWrite || saving} required={dailyDraft.confluence.enabled} error={confluenceErrors.parent} helperText="早报按报告日期保存在此页面下。更换父页面后，需要重新填写 Token。" /></div>
                  <div class="wide">
                    {#if dailyDraft.confluence.token === '__configured__' && !confluenceTokenEditing}
                      <span class="scw-native-label">PAT Token {#if dailyDraft.confluence.enabled}<span class="required">*</span>{/if}</span>
                      <div id="email-confluence-credential" class="email-credential" role="group" aria-label="已保存的 Confluence Token" aria-describedby={confluenceErrors.token || confluenceNeedsToken ? 'email-confluence-token-message' : undefined} tabindex="-1">
                        <span>凭证已配置</span>
                        {#if canWrite}<Button size="small" variant="secondary" disabled={saving} on:click={replaceConfluenceToken}>替换 Token</Button>{/if}
                      </div>
                      {#if confluenceErrors.token || confluenceNeedsToken}<p id="email-confluence-token-message" class="email-confluence-error">{confluenceErrors.token || '父页面已更改，请替换 Token 后再测试或启用同步。'}</p>{/if}
                    {:else}
                      <TextInput id="email-confluence-token" label="PAT Token" type="password" bind:value={dailyDraft.confluence.token} disabled={!canWrite || saving} required={dailyDraft.confluence.enabled} error={confluenceErrors.token} helperText="使用有权访问父页面的个人访问令牌，已保存凭证不返回原文。" />
                    {/if}
                  </div>
                </div>
                <p class="scw-help">{dailyDraft.confluence.enabled ? '同步失败不影响邮件发送。修复配置并保存后，可从发送记录单独重试文档同步。' : '关闭同步后，仍会保留父页面和凭证配置。'}</p>
                <div class="scw-section-actions email-confluence-actions">
                  {#if canWrite}<Button variant="secondary" loading={confluenceTesting} disabled={saving} on:click={testConfluence}>{confluenceTesting ? '检查连接中…' : '测试连接'}</Button>{/if}
                </div>
                <p class="scw-help">测试仅检查父页面是否可访问，不创建页面、不验证写入权限，也不会自动保存配置。</p>
                <div role="status" aria-live="polite" aria-atomic="true">
                  {#if confluenceResult}
                    <div class="email-confluence-result" class:stale={confluenceTestStale} class:failed={!confluenceTestStale && !confluenceResult.success}>
                      <strong>{confluenceTestStale ? '配置已更改，请重新测试' : confluenceResult.success ? '父页面连接检查通过' : '连接检查失败'}</strong>
                      <p>{confluenceTestStale ? '此前结果：' : ''}{confluenceResult.message}</p>
                      {#if confluenceResult.success && !confluenceTestStale}<p>只读检查通过，实际写入权限仍需同步时确认。</p>{/if}
                    </div>
                  {/if}
                </div>
              </section>
              <section class="scw-section">
                <div class="scw-section-head"><div class="scw-section-copy"><h4>早报内容</h4><p>Jira 图表、事项明细与 Coremember 负责人始终包含。</p></div></div>
                <div class="email-content-row"><div><strong>附加 commit 统计分析</strong><p>在邮件下方展示去重提交数与作者分布。</p></div><Switch id="email-include-commits" label="下方附加昨日 commit 统计分析" bind:checked={dailyDraft.include_commits} on:change={dirtyDaily} /></div>
                {#if dailyDraft.include_commits}<p class="email-advisory">按本地采集日期统计，不等同于原始提交日期；提交数量不用于判断个人绩效。</p>{/if}
                {#if !coreMembers.length && !jqlFallback}<p class="email-advisory">尚未设置核心成员，早报将显示范围未配置，不会扩大为所有人员。</p>{/if}
                <details class="email-disclosure"><summary>统计范围与核心成员<span>{coreMembers.length ? `${coreMembers.length} 位成员` : jqlFallback ? '由 JQL 解析' : '尚未配置'}</span></summary><ul><li>昨日 Jira：昨日 00:00 至今日 00:00 更新的事项，包含当前已解决事项。</li><li>近 3 天未解决：前三个完整自然日内创建、当前仍未解决，不含今日新建。</li><li>Coremember：复用 Jira 设置中的成员与身份映射。</li></ul>{#if coreMembers.length}<p>{coreMembers.join('、')}</p>{/if}</details>
              </section>
              <section class="scw-section email-project-groups">
                <div class="scw-section-head">
                  <div class="scw-section-copy">
                    <h4>项目与负责人分组</h4>
                    <p>按顺序匹配项目或负责人；未命中的事项归入“其他项目”。</p>
                  </div>
                  {#if canWrite}
                    <Button size="small" variant="secondary" disabled={saving} on:click={addProjectGroup}>添加分组</Button>
                  {/if}
                </div>
                {#if catalogError}
                  <div class="email-catalog-feedback" role="status"><span>{catalogError}，已选项目仍保留。</span><Button size="small" variant="secondary" on:click={refreshCatalog}>重试项目目录</Button></div>
                {:else}
                  <p class="scw-help" role="status">{catalogLoading ? '正在读取项目目录…' : `可选 ${allProjectOptions.length} 个项目，由 Jira 同步自动收录。`}</p>
                {/if}
                {#if dailyDraft.project_groups.length === 0}
                  <div class="email-empty-groups">
                    <strong>尚未设置分组</strong>
                    <span>早报将按原顺序展示事项。</span>
                  </div>
                {:else}
                  <!-- svelte-ignore a11y_no_noninteractive_tabindex (The horizontal scroll region must be reachable by keyboard.) -->
                  <div class="email-groups-scroll" role="region" aria-label="项目分组表格，可横向滚动" tabindex="0">
                    <table class="email-groups-table">
                      <caption class="email-sr-only">早报项目与负责人分组，按表格顺序匹配</caption>
                      <colgroup><col class="group-name-column" /><col class="group-projects-column" /><col class="group-owners-column" /><col class="group-actions-column" /></colgroup>
                      <thead><tr><th scope="col">分组名称</th><th scope="col">项目简称</th><th scope="col">负责人</th><th scope="col">操作</th></tr></thead>
                      <tbody>
                        {#each dailyDraft.project_groups as group, i (group.draft_id)}
                          <tr class="email-group-row" aria-describedby={projectGroupErrors[i]?.scope ? `email-group-error-${group.draft_id}` : undefined}>
                            <td>
                              <TextInput id={`group-name-${i}`} label={`分组 ${i + 1} 名称`} bind:value={group.name} placeholder="分组名称" error={projectGroupErrors[i]?.name || ''} disabled={!canWrite || saving} on:input={dirtyDaily} required />
                              {#if projectGroupErrors[i]?.scope}<p id={`email-group-error-${group.draft_id}`} class="email-group-error" role="status">{projectGroupErrors[i].scope}</p>{/if}
                            </td>
                            <td class="email-group-projects">
                              <MultiSelect
                                id={`group-projects-${i}`}
                                ariaLabel={`分组 ${i + 1} 负责项目`}
                                options={allProjectOptions}
                                bind:values={group.projects}
                                allowCustom={true}
                                appearance="settings"
                                separated={true}
                                showClear={true}
                                shadowless={true}
                                overlay={true}
                                disabled={!canWrite || saving}
                                placeholder="选择项目简称"
                                searchPlaceholder="搜索简称或全名"
                                emptyText="没有匹配项目，回车添加当前输入"
                                on:change={dirtyDaily}
                              >
                                <svelte:fragment slot="after-chips">
                                  {#if group.projects.length}
                                    <button
                                      type="button"
                                      class="email-name-hint"
                                      aria-label={`查看分组 ${group.name || i + 1} 项目全名`}
                                      title={group.projects.map(key => `${key}：${projectFullName(key)}`).join('\n')}
                                      on:pointerenter={(e) => showGroupTooltip(group, e.currentTarget)}
                                      on:pointerleave={scheduleHideGroupTooltip}
                                      on:focus={(e) => showGroupTooltip(group, e.currentTarget)}
                                      on:blur={scheduleHideGroupTooltip}
                                    >?</button>
                                  {/if}
                                </svelte:fragment>
                              </MultiSelect>
                            </td>
                            <td class="email-group-owners">
                              <MultiSelect
                                id={`group-owners-${i}`}
                                ariaLabel={`分组 ${i + 1} 负责人`}
                                options={allOwnerOptions}
                                bind:values={group.owners}
                                allowCustom={true}
                                appearance="settings"
                                separated={true}
                                showClear={true}
                                shadowless={true}
                                overlay={true}
                                disabled={!canWrite || saving}
                                placeholder="选择负责人"
                                searchPlaceholder="搜索姓名"
                                emptyText="没有匹配成员，回车添加当前输入"
                                on:change={dirtyDaily}
                              />
                            </td>
                            <td>
                              {#if canWrite}
                                <div class="email-group-actions" role="group" aria-label={`${group.name || '分组 ' + (i + 1)}排序与删除`}>
                                  <Button size="small" variant="ghost" disabled={saving || i === 0} on:click={() => moveProjectGroup(i, -1)}><svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" aria-hidden="true"><path d="m6 12 6-6 6 6M12 6v13" /></svg><span class="email-sr-only">上移</span></Button>
                                  <Button size="small" variant="ghost" disabled={saving || i === dailyDraft.project_groups.length - 1} on:click={() => moveProjectGroup(i, 1)}><svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" aria-hidden="true"><path d="m6 12 6 6 6-6M12 18V5" /></svg><span class="email-sr-only">下移</span></Button>
                                  <Button size="small" variant="ghost" disabled={saving} on:click={() => removeProjectGroup(i)}><svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" aria-hidden="true"><path d="M4 7h16M9 7V4h6v3M6 7l1 13h10l1-13M10 10v7M14 10v7" /></svg><span class="email-sr-only">删除</span></Button>
                                </div>
                              {:else}<span class="scw-help">只读</span>{/if}
                            </td>
                          </tr>
                        {/each}
                      </tbody>
                    </table>
                  </div>
                  <p class="scw-help">项目默认显示简称；悬停“?”图标可查看 Jira 自动收录的项目全名。</p>
                {/if}
              </section>
            </fieldset>
            {#if canWrite}<div class="scw-actions email-actions"><span class="email-action-note">早报与模板共享保存配置</span>{#if dailyDirty}<Button variant="secondary" on:click={resetDraft}>放弃更改</Button>{/if}<Button type="submit" loading={saving}>保存早报设置</Button></div>{/if}
          </form>
          <details class="email-disclosure email-history" open={historyOpen || !!runsError} on:toggle={(event) => historyOpen = event.currentTarget.open}>
            <summary>近期发送记录<span class:email-history-warning={runs.length > 0 && runs[0].status !== 'sent'}>{runs.length ? `${runs.length} 次 · 最近${runStatusLabel(runs[0].status)}` : '暂无记录'}</span></summary>
            <p>邮件与 Confluence 分别记录结果。已发送的邮件仅可重试文档同步；发送中或邮件结果未知时不会自动重发。</p>
            {#if runsError}
              <Alert type="error">{runsError}</Alert><Button variant="secondary" on:click={loadRuns}>重试加载</Button>
            {:else if !runs.length}
              <p>保存配置后，可先预览再手动发送。</p>
            {:else}
              {#if canWrite && runs.some(run => (run.status === 'confluence_failed' || (run.status === 'sent' && run.confluence_status === 'failed')))}
                <p class="scw-help">{smtpDirty || dailyDirty ? '请先保存或放弃更改，再重试该日期的早报。' : !smtp.enabled || !daily.recipients?.length ? '重试前需保存并启用发信服务，且至少配置一个早报收件人。' : '重试使用已保存配置；已发送的邮件只重试文档同步，不会再次发送。'}</p>
              {/if}
              <ul class="email-runs">
                {#each runs as run}
                  <li>
                    <div class="email-run-copy"><span>{run.date} · {run.trigger === 'manual' ? '手动' : '定时'}</span><strong>{runStatusLabel(run.status)}{run.confluence_status === "failed" ? " · Confluence 同步失败" : run.confluence_status === "synced" ? " · Confluence 已同步" : ""}</strong><span>收件人：{runRecipients(run.recipients_json)}</span></div>
                    <div class="email-run-actions">
                      {#if confluenceLink(run.confluence_url)}<a href={confluenceLink(run.confluence_url)} target="_blank" rel="noopener noreferrer">查看 Confluence 页面<span class="email-sr-only">，{run.date}，在新窗口打开</span></a>{/if}
                      {#if canWrite && (run.status === 'confluence_failed' || (run.status === 'sent' && run.confluence_status === 'failed'))}<Button size="small" variant="secondary" loading={retryingDate === run.date} disabled={saving || sending || smtpDirty || dailyDirty || !smtp.enabled || !daily.recipients?.length} on:click={() => retryReport(run)}>{retryingDate === run.date ? '重试中…' : run.status === 'sent' ? '重试同步' : '重试发送'}<span class="email-sr-only"> {run.date} 的早报</span></Button>{/if}
                    </div>
                  </li>
                {/each}
              </ul>
            {/if}
          </details>
        {:else}
          <div class="scw-section-head email-pane-heading">
            <div class="scw-section-copy"><h4>当前模板草稿</h4><p>已应用样式：{emailTemplateStyleName(dailyDraft.template.style)}</p></div>
            {#if canWrite}<Button size="small" variant="secondary" disabled={generating || saving || templateLoading} on:click={proposeDefaultTemplate}>恢复默认模板</Button>{/if}
          </div>
          {#if templateUndo}<div class="template-undo"><span>已应用新的模板样式</span><Button variant="ghost" size="small" disabled={saving} on:click={undoTemplate}>撤销应用</Button></div>{/if}
          <details class="email-disclosure email-agent" open={generating || generatorOpen} on:toggle={(event) => generatorOpen = event.currentTarget.open}>
            <summary>使用 Agent 定制整版模板<span>描述要求，生成自定义候选</span></summary>
            <section aria-busy={generating} class="email-agent-body">
              <div class="scw-native-field"><label class="scw-native-label" for="email-requirements">模板需求</label><textarea id="email-requirements" class="scw-native-textarea" rows="3" maxlength="4000" bind:value={requirements} disabled={!canWrite || generating} placeholder="例如：面向研发负责人，简洁中文，重点关注未解决事项。"></textarea></div>
              <p class="scw-help">Agent 会替换整版样式：生成一份完整的自定义 HTML 邮件模板，并重写主题、开场与结尾；服务端会清洗为邮件安全子集，并注入日期、时区、开场、结尾与各报告区块。生成后保存到上方图库，不自动应用或发送。</p>
              {#if templateLibraryFull}<p class="email-advisory">模板库已满，请先删除不需要的候选。</p>{/if}
              {#if canWrite}<div class="scw-section-actions email-local-actions"><Button variant="secondary" loading={generating} disabled={!requirements.trim() || templateLoading || !!deletingTemplateId || templateLibraryFull} on:click={generate}>生成自定义整版模板</Button>{#if generating}<Button variant="ghost" on:click={() => controller?.abort()}>停止等待</Button>{/if}</div>{/if}
            </section>
          </details>
          <form class="scw-form-stack email-template-form" on:submit|preventDefault={saveDaily} on:input={() => { templateUndo = null; dirtyDaily(); }}>
            <fieldset class="scw-form-stack" disabled={!canWrite || saving}>
              <legend class="email-sr-only">邮件模板草稿</legend>
              <TextInput id="email-subject" label="邮件主题" bind:value={dailyDraft.template.subject} required />
              <div class="scw-native-field"><label class="scw-native-label" for="email-introduction">开场说明</label><textarea id="email-introduction" class="scw-native-textarea" rows="4" bind:value={dailyDraft.template.introduction}></textarea></div>
              <div class="email-inserted"><span class="wa-icon icon-checklist" aria-hidden="true"></span><div><strong>图表与事项明细由系统填充</strong><span>状态分布 · 未解决事项 · 负责人 · 可选 commit 分析</span></div></div>
              <div class="scw-native-field"><label class="scw-native-label" for="email-closing">结尾说明</label><textarea id="email-closing" class="scw-native-textarea" rows="3" bind:value={dailyDraft.template.closing}></textarea></div>
              <p class="scw-help">可用变量：<code>{'{{date}}'}</code> 报告日期、<code>{'{{timezone}}'}</code> 时区。正文按纯文本处理。</p>
            </fieldset>
            {#if canWrite}<div class="scw-actions email-actions"><span class="email-action-note">保存前可先预览草稿</span>{#if dailyDirty}<Button variant="secondary" on:click={resetDraft}>放弃更改</Button>{/if}<Button type="submit" loading={saving}>保存模板</Button></div>{/if}
          </form>
        {/if}
      </div>
      <div class="email-aside email-pane" id="email-template-preview" tabindex="-1">
        {#if tab === 0}
          <aside class="email-verification" aria-label="发信验证">
            <div class="scw-section-head email-pane-heading"><div class="scw-section-copy"><h4>发送测试邮件</h4><p>验证当前填写的连接，测试不自动保存。</p></div><span class="email-save-state">{testing ? '发送中' : smtpResult && !smtpTestStale ? '已接受' : '待测试'}</span></div>
            <form class="scw-form-stack" on:submit|preventDefault={testSMTP}>
              <TextInput id="smtp-test-recipient" label="测试收件人" type="email" bind:value={recipient} disabled={!canWrite || testing} placeholder="用于确认收信的邮箱" required />
              {#if canWrite}<Button type="submit" variant="secondary" loading={testing} disabled={saving}>发送测试邮件</Button>{/if}
              <p class="scw-help">SMTP 接受后，请检查收件箱或垃圾邮件。</p>
            </form>
            <div aria-live="polite">{#if smtpResult}<div class="email-test-result"><strong>测试结果</strong><p>{smtpResult}</p>{#if smtpTestStale}<p class="email-advisory">配置已更改，请重新测试。</p>{/if}</div>{/if}</div>
            <section class="scw-section"><div class="scw-section-head"><div class="scw-section-copy"><h4>当前连接草稿</h4></div></div><dl class="email-receipt"><div><dt>服务器</dt><dd>{smtpDraft.host || '尚未填写'}</dd></div><div><dt>端口 / 握手</dt><dd>{smtpDraft.port} / {smtpDraft.tls_mode === 'starttls' ? 'STARTTLS' : smtpDraft.tls_mode === 'tls' ? 'SSL/TLS' : '本机无加密'}</dd></div><div><dt>发件人</dt><dd>{smtpDraft.from || '尚未填写'}</dd></div><div><dt>认证凭证</dt><dd>{smtpDraft.password ? '已填写' : '尚未填写'}</dd></div></dl></section>
          </aside>
        {:else}
          <EmailReportPreview {preview} bind:reportDate stale={previewStale} {previewing} {sending} {saving} dirty={smtpDirty || dailyDirty} {canWrite} smtpEnabled={smtp.enabled} recipientCount={(daily.recipients || []).length} onPreview={makePreview} onSend={sendReport} candidate={tab === 2 ? viewedTemplate : null} candidateApplied={!!viewedTemplate && emailTemplateSignature(viewedTemplate.template) === appliedTemplateSignature} candidateBusy={generating || !!deletingTemplateId || templateLoading} onUseCandidate={() => { if (viewedTemplate) useTemplate(viewedTemplate); }} onCloseCandidate={() => { viewIntent++; viewedTemplate = null; }} />
        {/if}
      </div>
    </div>
  </div>
  <div class="email-meta"><span>{canWrite ? '可编辑配置' : '只读配置'}</span><span>最近保存：{lastUpdated ? new Date(lastUpdated).toLocaleString() : '暂无记录'}</span></div>

  {#if hoveredGroupTooltip && tooltipTriggerRect}
    <!-- svelte-ignore a11y_no_noninteractive_element_interactions -->
    <div
      class="email-project-tooltip-panel"
      class:is-above={tooltipPos.isAbove}
      style={`top: ${tooltipPos.top}px; left: ${tooltipPos.left}px;`}
      role="tooltip"
      on:pointerenter={keepGroupTooltip}
      on:pointerleave={scheduleHideGroupTooltip}
    >
      <div class="email-project-tooltip-title">项目全名对照</div>
      <ul class="email-project-tooltip-items">
        {#each hoveredGroupTooltip.projects as key}
          <li>
            <span class="tooltip-key">{key}</span>
            <span class="tooltip-sep">：</span>
            <span class="tooltip-name" class:is-missing={projectFullName(key) === '待从 Jira 同步'}>
              {projectFullName(key)}
            </span>
          </li>
        {/each}
      </ul>
    </div>
  {/if}
</div>

<style>
  /* finesse · register=product · shell=split-email-panes+group-table */
  .email-config { --config-muted: var(--wa-text-muted); container: email-workbench / inline-size; min-width: 0; padding: 0; }
  .email-toolbar { gap: var(--wa-space-3); flex-wrap: wrap; }
  .email-save-state { flex: none; color: var(--scw-muted); background: var(--wa-neutral-soft); border-radius: var(--wa-radius-sm); padding: 4px 8px; font-size: 11px; font-weight: 700; line-height: 1.5; }
  .email-save-state.dirty { color: var(--wa-warning); background: var(--wa-warning-soft); }
  .email-feedback { display: grid; gap: var(--wa-space-2); }
  .email-catalog-feedback { display: flex; flex-wrap: wrap; align-items: center; gap: var(--wa-space-2); color: var(--wa-danger); font-size: 12px; line-height: 1.6; }
  .email-layout { display: grid; grid-template-columns: minmax(0, 1fr); gap: var(--wa-space-4); align-items: start; }
  .email-pane {
    min-width: 0;
    box-sizing: border-box;
    padding: var(--wa-space-4);
    border: 1px solid var(--wa-glass-outline);
    border-radius: var(--wa-radius-lg);
    background: var(--wa-glass-panel);
    box-shadow: inset 0 1px 0 var(--wa-glass-highlight);
    -webkit-backdrop-filter: blur(18px) saturate(124%);
    backdrop-filter: blur(18px) saturate(124%);
  }
  .email-pane:focus-visible { outline: 2px solid var(--wa-border-focus); outline-offset: 2px; }
  .email-main { container: email-fields / inline-size; min-width: 0; width: 100%; }
  .email-aside { min-width: 0; }
  .email-pane :global(.email-pane-heading) { min-height: 52px; border-bottom: 1px solid var(--scw-line); padding-bottom: var(--wa-space-3); margin-bottom: var(--wa-space-4); }
  .template-undo { display: flex; align-items: center; justify-content: space-between; gap: var(--wa-space-2); color: var(--scw-accent-strong); font-size: 12px; margin: 0 0 var(--wa-space-3); }
  fieldset:not(.scw-section) { border: 0; padding: 0; margin: 0; min-width: 0; }
  .email-sr-only { position: absolute; width: 1px; height: 1px; padding: 0; margin: -1px; overflow: hidden; clip-path: inset(50%); white-space: nowrap; }
  .required { color: var(--wa-danger); }
  .email-server-fields { grid-template-columns: minmax(0, 1fr) 120px; }
  .email-security { margin-top: var(--wa-space-5); }
  .email-handshakes { display: grid; grid-template-columns: repeat(3, minmax(0, 1fr)); gap: var(--wa-space-2); }
  .email-radio { display: flex; align-items: flex-start; gap: var(--wa-space-2); padding: var(--wa-space-3); border: 1px solid var(--scw-line-strong); border-radius: var(--wa-radius-sm); cursor: pointer; min-width: 0; background: var(--wa-surface-panel); }
  .email-radio.selected { border-color: var(--scw-accent-strong); background: var(--scw-accent-soft); }
  .email-radio:focus-within { outline: 2px solid var(--wa-border-focus); outline-offset: 2px; }
  .email-radio input { flex: none; width: 15px; height: 15px; margin: 2px 0 0; accent-color: var(--scw-accent-strong); }
  .email-radio span { display: grid; gap: var(--wa-space-1); min-width: 0; }
  .email-radio strong { font-size: 12px; line-height: 1.5; color: var(--scw-text); }
  .email-radio small { font-size: 11px; line-height: 1.6; color: var(--scw-muted); }
  .email-credential { min-height: 36px; display: flex; align-items: center; justify-content: space-between; gap: var(--wa-space-2); font-size: 12px; color: var(--scw-muted); }
  .email-credential:focus-visible { outline: 2px solid var(--wa-border-focus); outline-offset: 2px; }
  .email-confluence-actions { justify-content: flex-start; margin-block: var(--wa-space-3); }
  .email-confluence-error { color: var(--wa-danger); font-size: 12px; line-height: 1.6; margin: var(--wa-space-2) 0 0; overflow-wrap: anywhere; }
  .email-confluence-result { padding-top: var(--wa-space-3); color: var(--wa-success); font-size: 12px; line-height: 1.6; overflow-wrap: anywhere; }
  .email-confluence-result.stale { color: var(--wa-warning); }
  .email-confluence-result.failed { color: var(--wa-danger); }
  .email-confluence-result p { margin: var(--wa-space-1) 0 0; }
  .email-actions { align-items: center; }
  .email-action-note { margin-right: auto; font-size: 11px; line-height: 1.5; color: var(--scw-muted); }
  .email-disclosure { border-top: 1px solid var(--scw-line); margin-top: var(--wa-space-4); padding-top: var(--wa-space-3); color: var(--scw-muted); font-size: 12px; }
  .email-disclosure summary { min-height: 32px; cursor: pointer; color: var(--scw-text); font-weight: 700; line-height: 1.6; }
  .email-disclosure summary span { margin-left: var(--wa-space-2); font-size: 11px; color: var(--scw-muted); font-weight: 400; }
  .email-disclosure summary .email-history-warning { color: var(--wa-warning); font-weight: 600; }
  .email-disclosure summary:focus-visible { outline: 2px solid var(--wa-border-focus); outline-offset: 2px; }
  .email-disclosure p, .email-disclosure li { line-height: 1.7; overflow-wrap: anywhere; }
  .email-disclosure ul { padding-left: var(--wa-space-5); }
  .email-agent { margin-top: 0; }
  .email-agent-body { display: grid; gap: var(--wa-space-3); padding: var(--wa-space-3) 0 var(--wa-space-4); }
  .email-local-actions { justify-content: flex-start; }
  .email-template-form { margin-top: var(--wa-space-4); }
  .email-content-row { display: flex; align-items: center; justify-content: space-between; gap: var(--wa-space-4); padding: var(--wa-space-3) 0; }
  .email-content-row strong { font-size: 13px; color: var(--scw-text); }
  .email-content-row p { color: var(--scw-muted); font-size: 12px; line-height: 1.6; margin: var(--wa-space-1) 0 0; }
  .email-content-row :global(.switch-label) { position: absolute; width: 1px; height: 1px; overflow: hidden; clip-path: inset(50%); }
  .email-advisory { color: var(--wa-warning); font-size: 12px; line-height: 1.7; margin: var(--wa-space-2) 0; }
  .email-runs { padding: 0 !important; list-style: none; }
  .email-runs li { display: flex; flex-wrap: wrap; justify-content: space-between; gap: var(--wa-space-2); padding: var(--wa-space-2) 0; border-bottom: 1px solid var(--scw-line); }
  .email-runs strong { font-size: 12px; font-weight: 600; }
  .email-run-copy { min-width: 0; display: grid; gap: var(--wa-space-1); overflow-wrap: anywhere; }
  .email-run-actions { display: flex; flex-wrap: wrap; align-items: center; gap: var(--wa-space-3); min-width: 0; }
  .email-run-actions a { color: var(--scw-accent-strong); line-height: 1.6; overflow-wrap: anywhere; }
  .email-run-actions a:focus-visible { outline: 2px solid var(--wa-border-focus); outline-offset: 3px; }
  .email-inserted { display: flex; align-items: center; gap: var(--wa-space-3); padding: var(--wa-space-3) 0; border-block: 1px solid var(--scw-line); }
  .email-inserted > .wa-icon { color: var(--scw-accent-strong); }
  .email-inserted > div { display: grid; gap: var(--wa-space-1); }
  .email-inserted strong { font-size: 12px; color: var(--scw-text); }
  .email-inserted span { font-size: 11px; line-height: 1.6; color: var(--scw-muted); }
  .email-verification { display: grid; gap: var(--wa-space-4); }
  .email-verification .scw-section-head { margin-bottom: 0; }
  .email-test-result { border-top: 1px solid var(--scw-line); padding-top: var(--wa-space-3); font-size: 12px; color: var(--scw-text); }
  .email-test-result p { margin: var(--wa-space-2) 0 0; line-height: 1.7; overflow-wrap: anywhere; }
  .email-receipt { margin: 0; }
  .email-receipt > div { display: grid; grid-template-columns: 80px minmax(0,1fr); gap: var(--wa-space-3); padding: var(--wa-space-3) 0; border-bottom: 1px solid var(--scw-line); }
  .email-receipt dt { color: var(--scw-muted); font-size: 12px; }
  .email-receipt dd { color: var(--scw-text); font-size: 12px; font-weight: 600; margin: 0; text-align: right; overflow-wrap: anywhere; }
  .email-meta { display: flex; flex-wrap: wrap; gap: var(--wa-space-3); padding-top: var(--wa-space-3); border-top: 1px solid var(--scw-line); color: var(--scw-muted); font-size: 11px; }
  .email-project-groups { min-width: 0; display: grid; gap: var(--wa-space-3); }
  .email-empty-groups { display: grid; gap: var(--wa-space-1); padding: var(--wa-space-4) 0; border-block: 1px solid var(--scw-line); color: var(--scw-muted); }
  .email-empty-groups strong { color: var(--scw-text); font-size: 13px; font-weight: 600; }
  .email-empty-groups span { font-size: 12px; line-height: 1.6; }
  .email-groups-scroll { min-width: 0; width: 100%; max-width: 100%; overflow-x: auto; overscroll-behavior-x: contain; border-block: 1px solid var(--scw-line); }
  .email-groups-scroll:focus-visible, .email-name-hint:focus-visible { outline: 2px solid var(--wa-border-focus); outline-offset: 2px; }
  .email-groups-table { width: 100%; min-width: 680px; table-layout: fixed; border-collapse: collapse; color: var(--scw-text); font-size: 12px; }
  .group-name-column { width: 22%; }
  .group-projects-column { width: 36%; }
  .group-owners-column { width: 28%; }
  .group-actions-column { width: 14%; }
  :global(.settings-unified) .email-groups-table th,
  :global(.settings-unified) .email-groups-table td,
  .email-groups-table th,
  .email-groups-table td {
    padding: 10px 8px;
    border-top: 1px solid var(--scw-line);
    vertical-align: top !important;
  }
  .email-groups-table th { background: var(--wa-surface-inset); color: var(--scw-text); text-align: left; font-weight: 600; }
  .email-groups-table td { box-sizing: border-box; }
  .email-groups-table :global(.input-group), .email-groups-table :global(.multi-select-group) { margin-bottom: 0; }
  .email-groups-table :global(.input-label) { position: absolute; width: 1px; height: 1px; padding: 0; margin: -1px; overflow: hidden; clip-path: inset(50%); white-space: nowrap; }
  .email-groups-table :global(.text-input) { height: 36px; min-height: 36px; box-sizing: border-box; }
  .email-groups-table :global(.text-input:focus-visible) { outline: 2px solid var(--wa-accent-strong); outline-offset: 2px; }
  .email-group-actions { display: flex; flex-wrap: nowrap; gap: var(--wa-space-1); align-items: center; min-height: 36px; height: 36px; }
  .email-group-actions :global(.btn) { width: 28px; min-width: 28px; height: 28px; padding: 0; }
  .email-group-error { margin: var(--wa-space-2) 0 0; color: var(--wa-danger); font-size: 12px; line-height: 1.6; }
  .email-name-hint {
    display: inline-flex;
    justify-content: center;
    align-items: center;
    width: 22px;
    height: 22px;
    padding: 0;
    margin-left: 2px;
    border: 1px solid var(--wa-border-strong);
    border-radius: 50%;
    background: var(--wa-surface-panel);
    color: var(--wa-text-muted);
    font-size: 11px;
    font-weight: 700;
    line-height: 1;
    cursor: help;
    transition: all var(--wa-duration-fast, 140ms) var(--wa-ease, ease);
  }
  .email-name-hint:hover,
  .email-name-hint:focus-visible {
    outline: none;
    border-color: var(--wa-accent-strong);
    color: var(--wa-accent-strong);
    background: var(--wa-accent-soft);
    box-shadow: 0 0 0 2px rgba(0, 143, 150, 0.15);
  }

  /* Schedule Time & Timezone controls */
  .email-schedule-grid {
    align-items: start;
    grid-template-columns: minmax(0, 0.9fr) minmax(0, 1.1fr);
  }
  .email-schedule-grid .scw-native-field { gap: 6px; }
  .email-field-head {
    min-height: 24px;
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 8px;
  }
  .email-field-head > .scw-native-label {
    display: flex;
    align-items: center;
    height: 18px;
    line-height: 18px;
  }
  .email-field-foot {
    min-height: 24px;
    display: flex;
    align-items: center;
    gap: 8px;
    flex-wrap: wrap;
  }
  .email-field-hint {
    color: var(--wa-text-muted);
    font-size: 12px;
    line-height: 18px;
  }
  .email-time-field .email-field-hint {
    margin-left: auto;
  }
  .email-field-actions {
    display: flex;
    align-items: center;
    gap: 8px;
  }
  .email-field-meta {
    color: var(--wa-text-muted);
    font-size: 12px;
    line-height: 18px;
    font-variant-numeric: tabular-nums;
  }
  .email-field-shell {
    position: relative;
    width: 100%;
    max-width: 100%;
    height: 36px;
    min-height: 36px;
    display: flex;
    align-items: center;
    gap: 8px;
    padding: 0 12px;
    border: 1px solid var(--wa-border-strong);
    border-radius: var(--wa-radius-sm);
    background: var(--wa-surface-panel);
    box-sizing: border-box;
    transition: border-color var(--wa-duration-fast, 140ms) var(--wa-ease, ease), box-shadow var(--wa-duration-fast, 140ms) var(--wa-ease, ease);
  }
  .email-field-shell:focus-within {
    border-color: var(--wa-border-focus, var(--wa-accent-strong));
    box-shadow: 0 0 0 2px rgba(0, 143, 150, 0.15);
    outline: none;
  }
  .email-field-shell.has-error {
    border-color: var(--wa-danger);
  }
  .email-time-icon {
    flex: none;
    color: var(--wa-text-muted);
  }
  .email-time-input,
  .email-timezone-custom-input {
    flex: 1 1 auto;
    min-width: 72px;
    height: 100%;
    border: 0;
    background: transparent;
    background-image: none;
    color: var(--wa-text-main);
    font-family: inherit;
    font-size: 13px;
    line-height: 20px;
    font-variant-numeric: tabular-nums;
    outline: none;
    padding: 0;
    margin: 0;
  }
  .email-time-input::placeholder,
  .email-timezone-custom-input::placeholder { color: var(--wa-text-muted); }
  .email-field-shell input.email-time-input,
  .email-field-shell input.email-timezone-custom-input,
  .email-field-shell select.email-timezone-select {
    min-height: 0 !important;
    height: 100% !important;
    border: 0 !important;
    border-radius: 0 !important;
    background: transparent !important;
    color: var(--wa-text-main) !important;
    box-shadow: none !important;
  }
  .email-field-shell input.email-time-input:focus,
  .email-field-shell input.email-timezone-custom-input:focus,
  .email-field-shell select.email-timezone-select:focus {
    background: transparent !important;
    border: 0 !important;
    box-shadow: none !important;
  }
  .email-field-shell input.email-time-input::placeholder,
  .email-field-shell input.email-timezone-custom-input::placeholder {
    font-size: 13px !important;
    color: var(--wa-text-muted) !important;
  }
  .email-time-presets {
    display: flex;
    gap: 4px;
    align-items: center;
  }
  .email-time-preset-pill {
    border: 1px solid var(--wa-border-soft);
    border-radius: var(--wa-radius-xs);
    background: var(--wa-surface-inset);
    color: var(--wa-text-muted);
    font-size: 11px;
    font-weight: 600;
    min-height: 24px;
    padding: 2px 8px;
    cursor: pointer;
    transition: all 140ms;
    font-variant-numeric: tabular-nums;
  }
  .email-time-preset-pill:hover {
    background: var(--wa-surface-panel);
    color: var(--wa-text-main);
    border-color: var(--wa-border-strong);
  }
  .email-time-preset-pill.is-active {
    background: var(--wa-accent-soft);
    color: var(--wa-accent-strong);
    border-color: var(--wa-accent-strong);
  }
  .email-timezone-toggle-btn {
    border: 0;
    background: transparent;
    color: var(--wa-accent-strong);
    font-size: 11px;
    font-weight: 600;
    cursor: pointer;
    padding: 0;
  }
  .email-timezone-toggle-btn { margin-left: auto; }
  .email-timezone-toggle-btn:focus-visible {
    outline: 2px solid var(--wa-accent-strong);
    outline-offset: 2px;
  }
  .email-timezone-toggle-btn:hover {
    text-decoration: underline;
  }
  .email-timezone-icon {
    flex: none;
    color: var(--wa-text-muted);
  }
  .email-timezone-select {
    flex: 1 1 auto;
    min-width: 0;
    height: 100%;
    border: 0;
    background: transparent;
    padding: 0 28px 0 0;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    color-scheme: light;
    text-align: left;
    font-family: inherit;
    font-size: 13px;
    line-height: 20px;
    color: var(--wa-text-main);
    outline: none;
    -webkit-appearance: none;
    appearance: none;
    cursor: pointer;
  }
  .email-timezone-select:focus-visible { outline: none; }
  .email-select-chevron {
    position: absolute;
    right: 10px;
    color: var(--wa-text-muted);
    pointer-events: none;
  }
  .email-timezone-custom-row {
    margin-bottom: 0;
  }
  .email-control-error {
    margin: 0;
    color: var(--wa-danger);
    font-size: 12px;
    line-height: 1.4;
  }

  /* Email Recipients Tag Input */
  .email-recipients-clear-btn {
    border: 0;
    background: transparent;
    color: var(--wa-danger);
    font-size: 11px;
    font-weight: 600;
    cursor: pointer;
    min-height: 24px;
    padding: 0 2px;
  }
  .email-recipients-clear-btn:focus-visible { outline: 2px solid var(--wa-accent-strong); outline-offset: 2px; }
  .email-recipients-clear-btn:hover {
    text-decoration: underline;
  }
  .email-recipients-box {
    display: flex;
    flex-direction: column;
    gap: 6px;
    min-height: 44px;
    max-height: 140px;
    padding: 8px 12px;
    border: 1px solid var(--wa-border-strong);
    border-radius: var(--wa-radius-sm);
    background: var(--wa-surface-panel);
    box-sizing: border-box;
    transition: border-color 140ms, box-shadow 140ms;
  }
  .email-recipients-box:focus-within {
    border-color: var(--wa-border-focus, var(--wa-accent-strong));
    box-shadow: 0 0 0 2px rgba(0, 143, 150, 0.15);
  }
  .email-recipients-box.has-error {
    border-color: var(--wa-danger);
  }
  .email-recipients-box.is-disabled {
    background: var(--wa-surface-inset);
    opacity: 0.7;
    cursor: not-allowed;
  }
  .email-recipient-list {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: 6px;
    margin: 0;
    padding: 0;
    max-height: 84px;
    overflow-y: auto;
    list-style: none;
    scrollbar-width: thin;
  }
  .email-recipient-list + .email-recipient-entry { border-top: 1px solid var(--wa-border-soft); }
  .email-recipient-chip {
    height: 28px;
    padding: 0 8px;
    gap: 5px;
    border: 1px solid var(--wa-border-soft);
    border-radius: var(--wa-radius-xs);
    background: var(--wa-surface-inset);
    color: var(--wa-text-main);
    font-size: 12px;
    display: inline-flex;
    align-items: center;
    box-sizing: border-box;
  }
  .email-chip-text {
    min-width: 0;
    max-width: 260px;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .email-recipient-chip button {
    flex: none;
    width: 20px;
    height: 20px;
    border: 0;
    background: transparent;
    color: var(--wa-text-muted);
    font-size: 14px;
    line-height: 1;
    cursor: pointer;
    border-radius: 50%;
    display: grid;
    place-items: center;
    padding: 0;
    transition: color 140ms, background 140ms;
  }
  .email-recipient-chip button:focus-visible { outline: 2px solid var(--wa-accent-strong); outline-offset: -2px; }
  .email-recipient-chip button:hover {
    color: var(--wa-danger);
    background: var(--wa-danger-soft, rgba(200, 66, 54, 0.1));
  }
  .email-recipient-entry {
    display: flex;
    align-items: center;
    gap: 8px;
    padding: 0;
  }
  .email-recipient-inline-input {
    flex: 1 1 auto;
    min-width: 160px;
    height: 32px !important;
    min-height: 32px !important;
    border: 0;
    background: transparent;
    color: var(--wa-text-main);
    font-family: inherit;
    font-size: 13px;
    line-height: 20px;
    outline: none;
    padding: 0;
    margin: 0;
  }
  .email-recipient-inline-input::placeholder {
    color: var(--wa-text-muted);
  }
  .email-recipient-entry input.email-recipient-inline-input {
    min-height: 32px !important;
    height: 32px !important;
    border: 0 !important;
    border-radius: 0 !important;
    background: transparent !important;
    color: var(--wa-text-main) !important;
    box-shadow: none !important;
  }
  .email-recipient-entry input.email-recipient-inline-input:focus {
    background: transparent !important;
    border: 0 !important;
    box-shadow: none !important;
  }
  .email-recipient-entry input.email-recipient-inline-input::placeholder {
    font-size: 13px !important;
    color: var(--wa-text-muted) !important;
  }
  .email-recipient-add-btn {
    flex: none;
    width: 32px;
    height: 32px;
    display: grid;
    place-items: center;
    border: 1px solid var(--wa-border-strong);
    border-radius: var(--wa-radius-xs);
    background: var(--wa-surface-inset);
    color: var(--wa-accent-strong);
    cursor: pointer;
    padding: 0;
    transition: background 140ms, border-color 140ms, color 140ms;
  }
  .email-recipient-add-btn:hover:not(:disabled) {
    background: var(--wa-accent-soft);
    border-color: var(--wa-accent-strong);
  }
  .email-recipient-add-btn:focus-visible { outline: 2px solid var(--wa-accent-strong); outline-offset: -2px; }
  .email-recipient-add-btn:disabled { opacity: 0.5; cursor: not-allowed; }
  .email-project-tooltip-panel {
    position: fixed;
    z-index: 1300;
    min-width: 200px;
    max-width: 340px;
    padding: 8px 12px;
    border: 1px solid var(--wa-border-strong);
    border-radius: var(--wa-radius-sm);
    background: var(--wa-surface-overlay);
    color: var(--wa-text-main);
    box-shadow: var(--wa-shadow-md);
    font-size: 12px;
    line-height: 1.5;
    pointer-events: auto;
    animation: emailTooltipFadeIn 120ms ease;
  }
  .email-project-tooltip-panel.is-above {
    transform: translateY(-100%);
    animation: emailTooltipFadeInAbove 120ms ease;
  }
  @keyframes emailTooltipFadeIn {
    from { opacity: 0; transform: translateY(3px); }
    to { opacity: 1; transform: translateY(0); }
  }
  @keyframes emailTooltipFadeInAbove {
    from { opacity: 0; transform: translateY(calc(-100% - 3px)); }
    to { opacity: 1; transform: translateY(-100%); }
  }
  .email-project-tooltip-title {
    font-weight: 700;
    font-size: 11px;
    color: var(--wa-text-muted);
    margin-bottom: 6px;
    padding-bottom: 4px;
    border-bottom: 1px solid var(--wa-border-soft);
  }
  .email-project-tooltip-items {
    list-style: none;
    margin: 0;
    padding: 0;
    display: flex;
    flex-direction: column;
    gap: 5px;
  }
  .email-project-tooltip-items li {
    display: flex;
    align-items: baseline;
    gap: 2px;
    line-height: 1.4;
    font-size: 12px;
  }
  .tooltip-key {
    font-weight: 700;
    color: var(--wa-text-strong);
    flex: none;
    font-family: var(--wa-font-mono, monospace);
    font-size: 11px;
  }
  .tooltip-sep {
    color: var(--wa-text-muted);
    flex: none;
  }
  .tooltip-name {
    color: var(--wa-text-main);
    word-break: break-all;
  }
  .tooltip-name.is-missing {
    color: var(--wa-text-muted);
    font-style: italic;
  }
  @container email-workbench (min-width: 800px) {
    .email-layout { grid-template-columns: minmax(0, 1.8fr) minmax(340px, 1fr); }
  }
  @container email-fields (max-width: 560px) {
    .email-handshakes { grid-template-columns: minmax(0,1fr); }
    .email-radio { align-items: center; min-height: 44px; box-sizing: border-box; }
    .email-radio span { grid-template-columns: minmax(0,1fr) auto; flex: 1; align-items: center; gap: var(--wa-space-2); }
  }
  @container email-fields (max-width: 420px) {
    .email-server-fields, .email-main .scw-form-grid { grid-template-columns: minmax(0,1fr); }
    .email-radio span { grid-template-columns: minmax(0,1fr); }
    .email-action-note { flex-basis: 100%; }
  }
  @media (prefers-reduced-transparency: reduce) {
    .email-config { background: transparent; }
    .email-pane { background: var(--wa-surface-panel); -webkit-backdrop-filter: none; backdrop-filter: none; }
  }
  @supports not ((backdrop-filter: blur(1px)) or (-webkit-backdrop-filter: blur(1px))) {
    .email-pane { background: var(--wa-surface-panel); }
  }
  @media (max-width: 760px), (pointer: coarse) {
    .email-field-shell { height: 44px; min-height: 44px; }
    .email-time-input,
    .email-timezone-custom-input { min-width: 96px; }
    .email-time-presets { flex-wrap: wrap; }
    .email-time-preset-pill { min-height: 44px; padding: 0 12px; }
    .email-timezone-toggle-btn,
    .email-recipients-clear-btn { min-height: 44px; display: inline-flex; align-items: center; }
    .email-recipient-list { max-height: 76px; }
    .email-recipient-chip {
      position: relative;
      height: 44px;
      padding: 0 0 0 10px;
      background: transparent;
      border: 0;
    }
    .email-recipient-chip::before {
      content: '';
      position: absolute;
      inset: 8px 0;
      border: 1px solid var(--wa-border-soft);
      border-radius: var(--wa-radius-xs);
      background: var(--wa-surface-inset);
      pointer-events: none;
    }
    .email-chip-text,
    .email-recipient-chip button { position: relative; }
    .email-recipient-chip button { width: 44px; height: 44px; }
    .email-recipient-inline-input,
    .email-recipient-add-btn { height: 44px !important; min-height: 44px !important; }
    .email-recipient-entry input.email-recipient-inline-input { height: 44px !important; min-height: 44px !important; }
    .email-recipient-inline-input { min-width: 120px; }
    .email-recipient-add-btn { width: 44px; }
  }
  @media (max-width: 760px) {
    .email-config :global(.btn), .email-toolbar button, .email-config :global(.text-input), .email-config .scw-native-input { min-height: 44px; }
    .email-actions { align-items: stretch; }
    .email-actions :global(.btn) { flex: 1 1 140px; }
    .email-group-actions { min-height: 44px; height: 44px; }
    .email-group-actions :global(.btn) { width: 44px; min-width: 44px; height: 44px; }
    .email-name-hint { width: 32px; height: 32px; font-size: 13px; }
    .email-run-actions { width: 100%; }
    .email-run-actions a { display: inline-flex; align-items: center; min-height: 44px; }
    .email-run-actions :global(.btn) { width: 100%; }
    .email-action-note { margin: 0; flex-basis: 100%; }
    .email-layout { gap: var(--wa-space-5); }
    .email-disclosure summary { min-height: 44px; }
    .email-radio span { grid-template-columns: minmax(0,1fr); }
    .email-toolbar .scw-task-tabs { box-sizing: border-box; width: 100%; }
    .email-toolbar button { flex: 1 0 auto; padding-inline: var(--wa-space-2); }
  }
</style>
