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
