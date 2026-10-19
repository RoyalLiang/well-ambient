export const OPEN_API_KEY_PLACEHOLDER = '<YOUR_API_KEY>';

function publicConnectionURL(value: string): string {
  if (!value) return '';
  try {
    const url = new URL(value);
    if (url.protocol !== 'https:' && url.protocol !== 'http:') return '';
    url.username = '';
    url.password = '';
    url.search = '';
    url.hash = '';
    return url.toString().replace(/\/$/, '');
  } catch {
    return '';
  }
}

// Install URLs describe public configuration. Credentials belong in the local client only.
export function buildCodexMcpConfig(endpointURL: string, token = OPEN_API_KEY_PLACEHOLDER): string {
  const url = publicConnectionURL(endpointURL);
  if (!url) return '';
  return `[mcp_servers."well-ambient"]\nurl = ${JSON.stringify(url)}\nhttp_headers = { Authorization = ${JSON.stringify(`Bearer ${token}`)} }`;
}

export function buildOpenCapabilityInstall(baseURL: string, endpointURL: string) {
  const publicBaseURL = publicConnectionURL(baseURL);
  const mcpURL = publicConnectionURL(endpointURL || `${publicBaseURL}/mcp`);
  const config = { url: mcpURL, headers: { Authorization: `Bearer ${OPEN_API_KEY_PLACEHOLDER}` } };
  const bytes = new TextEncoder().encode(JSON.stringify(config));
  const encoded = btoa(Array.from(bytes, byte => String.fromCharCode(byte)).join(''));
  return {
    mcpURL,
    installCommand: publicBaseURL
      ? `curl -fsSL '${`${publicBaseURL}/open/v1/install.sh`.replaceAll("'", "'\"'\"'")}' | bash`
      : '',
    cursorDeepLink: mcpURL
      ? `cursor://anysphere.cursor-deeplink/mcp/install?name=well-ambient&config=${encodeURIComponent(encoded)}`
      : ''
  };
}

export type SkillInstallTarget = 'codex' | 'claude' | 'choose';

export function skillInstallCommand(baseURL: string, name: string, target: SkillInstallTarget): string {
  if (!/^[a-z][a-z0-9-]{0,95}$/.test(name)) return '';
  const { installCommand } = buildOpenCapabilityInstall(baseURL, '');
  if (!installCommand) return '';
  const root = target === 'codex' ? ' --root "$HOME/.codex"' : target === 'claude' ? ' --root "$HOME/.claude"' : '';
  return `${installCommand} -s --${root} --skill ${name} --skip-mcp`;
}
