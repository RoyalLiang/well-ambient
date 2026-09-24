export interface SMTPConfig {
  enabled: boolean; host: string; port: number; username: string; password: string; from: string; tls_mode: 'starttls' | 'tls' | 'none';
}
export type EmailTemplateStyle = 'brief' | 'focus' | 'ledger' | 'hyperframe' | 'custom';
export interface EmailTemplate { subject: string; introduction: string; closing: string; style?: EmailTemplateStyle; html?: string }
export interface EmailTemplateEntry {
  id: string; name: string; builtin: boolean; template: EmailTemplate; preview_html: string; created_at?: string;
}
export const emailTemplateStyleName = (style?: EmailTemplateStyle): string => ({
  brief: '标准简报',
  focus: '重点跟进',
  ledger: '紧凑清单',
  hyperframe: 'HyperFrame 全景看板',
  custom: '自定义整版'
})[style || 'brief'];
export interface DailyJiraProjectGroup {
  name: string;
  owners: string[];
  projects: string[];
}
export interface ConfluenceConfig {
  enabled: boolean;
  parent_page_url: string;
  token: string;
}
export interface DailyJiraEmailRun {
 recipients_json?: string; confluence_status?: string; confluence_error?: string;
  date: string;
  trigger: string;
  status: string;
  confluence_url?: string;
}
export const defaultConfluence = (): ConfluenceConfig => ({
  enabled: false,
  parent_page_url: 'https://confluence.westwell-lab.com/display/~zhiyuan_liang/well-infra',
  token: ''
});
export const emailTemplateSignature = (template: EmailTemplate): string => JSON.stringify([template.style || 'brief', template.subject, template.introduction, template.closing, template.html || '']);
export interface DailyJiraEmailConfig {
  enabled: boolean; recipients: string[]; timezone: string; send_time: string; include_commits: boolean; template: EmailTemplate; project_groups?: DailyJiraProjectGroup[]; confluence?: ConfluenceConfig;
}
export const defaultSMTP = (): SMTPConfig => ({ enabled: false, host: '', port: 587, username: '', password: '', from: '', tls_mode: 'starttls' });
export const defaultEmailTemplate = (): EmailTemplate => ({
  style: 'brief',
  subject: '研发每日早报 · {{date}}',
  introduction: '各位早上好，{{date}} 研发早报（{{timezone}}）：昨日重点更新与待办事项汇总如下。',
  closing: '请各负责人及时推进待办事项，并在 Jira 更新最新状态。'
});
export const defaultDailyEmail = (): DailyJiraEmailConfig => ({ enabled: false, recipients: [], timezone: 'Asia/Shanghai', send_time: '09:00', include_commits: false, template: defaultEmailTemplate(), project_groups: [], confluence: defaultConfluence() });
export async function emailRequest<T>(path: string, body?: unknown, signal?: AbortSignal, method?: 'DELETE'): Promise<T> {
  const response = await fetch(path, method === 'DELETE' ? { method, signal } : body === undefined ? { signal } : { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(body), signal });
  const text = await response.text();
  let data: any;
  try { data = JSON.parse(text); } catch { throw new Error(text || `请求失败（${response.status}）`); }
  if (!response.ok || data.success === false) throw new Error(data.message || `请求失败（${response.status}）`);
  return data as T;
}
