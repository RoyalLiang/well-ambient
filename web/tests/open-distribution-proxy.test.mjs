import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { createServer as createHTTPServer } from 'node:http';
import test from 'node:test';
import { fileURLToPath } from 'node:url';
import { createServer as createViteServer } from 'vite';

test('development gateway preserves public distribution and MCP responses', async (t) => {
  let streamResponse;
  const backend = createHTTPServer((request, response) => {
    const url = new URL(request.url, 'http://fixture');
    const origin = `http://${request.headers.host}`;
    if (url.pathname === '/api/ai-governance/open-capabilities/overview') {
      response.setHeader('Content-Type', 'application/json');
      response.end(JSON.stringify({ base_url: origin, mcp: { endpoint_url: `${origin}/mcp` } }));
      return;
    }
    if (url.pathname.startsWith('/open/')) {
      const types = {
        '/open/v1/skills': 'application/json',
        '/open/v1/skills/jira-analysis/skill.md': 'text/markdown',
        '/open/v1/skills/jira-analysis/manifest': 'application/yaml',
        '/open/v1/skills/jira-analysis/archive': 'application/gzip',
        '/open/v1/install.sh': 'text/x-shellscript',
      };
      response.setHeader('Content-Type', types[url.pathname] || 'application/json');
      response.end(JSON.stringify({ path: request.url, base_url: origin }));
      return;
    }
    if (url.pathname === '/mcp') {
      if (url.searchParams.get('probe') === 'stream') {
        streamResponse = response;
        response.setHeader('Content-Type', 'text/event-stream');
        response.flushHeaders();
        response.write('event: message\ndata: {"ready":true}\n\n');
        return;
      }
      response.statusCode = 401;
      response.setHeader('Content-Type', 'application/json');
      response.setHeader('WWW-Authenticate', 'Bearer');
      response.setHeader('Mcp-Session-Id', 'test-session');
      response.end(JSON.stringify({
        method: request.method,
        path: request.url,
        authorization: request.headers.authorization,
        accept: request.headers.accept,
        session: request.headers['mcp-session-id'],
        protocol: request.headers['mcp-protocol-version'],
      }));
      return;
    }
    response.statusCode = 404;
    response.end('fixture route not found');
  });
  await new Promise((resolve) => backend.listen(0, '127.0.0.1', resolve));
  const previousTarget = process.env.VITE_API_PROXY_TARGET;
  process.env.VITE_API_PROXY_TARGET = `http://127.0.0.1:${backend.address().port}`;
  let vite;
  t.after(async () => {
    await vite?.close();
    backend.closeAllConnections();
    await new Promise((resolve) => backend.close(resolve));
    if (previousTarget === undefined) delete process.env.VITE_API_PROXY_TARGET;
    else process.env.VITE_API_PROXY_TARGET = previousTarget;
  });
  const portReservation = createHTTPServer();
  await new Promise((resolve) => portReservation.listen(0, '127.0.0.1', resolve));
  const gatewayPort = portReservation.address().port;
  await new Promise((resolve) => portReservation.close(resolve));
  vite = await createViteServer({
    root: fileURLToPath(new URL('../', import.meta.url)),
    configFile: fileURLToPath(new URL('../vite.config.ts', import.meta.url)),
    logLevel: 'silent',
    server: { host: '127.0.0.1', port: gatewayPort, strictPort: true, hmr: false },
  });
  await vite.listen();
  const origin = `http://127.0.0.1:${vite.httpServer.address().port}`;
  const get = (path, init) => fetch(`${origin}${path}`, {
    ...init, signal: AbortSignal.timeout(5000),
  });

  await t.test('catalog, documentation, manifest, package and installer bypass SPA fallback', async () => {
    for (const path of [
      '/open/v1/skills?probe=route',
      '/open/v1/skills/jira-analysis/skill.md',
      '/open/v1/skills/jira-analysis/manifest',
      '/open/v1/skills/jira-analysis/archive',
      '/open/v1/install.sh',
    ]) {
      const response = await get(path);
      assert.equal(response.status, 200, path);
      assert.doesNotMatch(response.headers.get('Content-Type') || '', /text\/html/, path);
      const payload = await response.json();
      assert.equal(payload.path, path);
      assert.equal(payload.base_url, origin, 'distribution links must retain the visible gateway origin');
    }
  });
  await t.test('overview fallback uses the visible gateway host and port', async () => {
    const response = await get('/api/ai-governance/open-capabilities/overview');
    assert.equal(response.status, 200);
    assert.deepEqual(await response.json(), { base_url: origin, mcp: { endpoint_url: `${origin}/mcp` } });
  });
  await t.test('MCP GET and POST retain authentication, protocol headers and query', async () => {
    for (const method of ['GET', 'POST']) {
      const response = await get('/mcp?probe=route', {
        method,
        headers: {
          Authorization: 'Bearer test-only-placeholder',
          Accept: 'application/json, text/event-stream',
          'Mcp-Session-Id': 'client-test-session',
          'MCP-Protocol-Version': '2025-11-25',
          ...(method === 'POST' ? { 'Content-Type': 'application/json' } : {}),
        },
        ...(method === 'POST' ? { body: JSON.stringify({ jsonrpc: '2.0', id: 1, method: 'tools/list' }) } : {}),
      });
      assert.equal(response.status, 401, 'gateway must preserve the backend authorization result');
      assert.equal(response.headers.get('WWW-Authenticate'), 'Bearer');
      assert.equal(response.headers.get('Mcp-Session-Id'), 'test-session');
      assert.deepEqual(await response.json(), {
        method, path: '/mcp?probe=route', authorization: 'Bearer test-only-placeholder',
        accept: 'application/json, text/event-stream', session: 'client-test-session', protocol: '2025-11-25',
      });
    }
  });
  await t.test('ordinary page navigation remains a SPA response', async () => {
    const response = await get('/');
    assert.equal(response.status, 200);
    assert.match(response.headers.get('Content-Type'), /text\/html/);
  });
  await t.test('similarly named paths remain frontend routes', async () => {
    const response = await get('/mcp-extra', { headers: { Accept: 'text/html' } });
    assert.equal(response.status, 200);
    assert.match(response.headers.get('Content-Type'), /text\/html/);
  });
  await t.test('MCP first event arrives before the backend closes its stream', async () => {
    let reader;
    try {
      const response = await get('/mcp?probe=stream', { headers: { Accept: 'text/event-stream' } });
      assert.equal(response.status, 200);
      assert.match(response.headers.get('Content-Type'), /text\/event-stream/);
      reader = response.body.getReader();
      const frame = await reader.read();
      assert.equal(frame.done, false);
      assert.equal(new TextDecoder().decode(frame.value), 'event: message\ndata: {"ready":true}\n\n');
      assert.equal(streamResponse.writableEnded, false);
    } finally {
      streamResponse?.end();
      await reader?.cancel();
    }
  });
});

test('production gateway forwards distribution and unbuffered MCP before SPA fallback', () => {
  const config = readFileSync(new URL('../../deploy/nginx.conf', import.meta.url), 'utf8');
  for (const route of ['= /mcp', '/open/', '/api/']) {
    const escaped = route.replace(/[.*+?^${}()|[\]\\]/g, '\\$&');
    const block = config.match(new RegExp(`location\\s+${escaped}\\s*\\{([^}]+)\\}`))?.[1];
    assert.ok(block, `${route} must have a backend location`);
    assert.match(block, /proxy_pass\s+http:\/\/server:8080\s*;/, 'original path must reach the backend');
    assert.match(block, /proxy_set_header\s+Host\s+\$http_host\s*;/, 'external non-default port must survive');
    assert.match(block, /proxy_set_header\s+X-Forwarded-Proto\s+\$scheme\s*;/);
    if (route === '= /mcp') {
      assert.match(block, /proxy_http_version\s+1\.1\s*;/);
      assert.match(block, /proxy_buffering\s+off\s*;/);
      assert.match(block, /proxy_cache\s+off\s*;/);
    }
  }
});
