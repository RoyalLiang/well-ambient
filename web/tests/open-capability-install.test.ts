import assert from 'node:assert/strict';
import test from 'node:test';
import { buildOpenCapabilityInstall, OPEN_API_KEY_PLACEHOLDER } from '../src/lib/openCapabilityInstall.ts';

test('installation script URL contains no credentials or query secrets', () => {
  const instructions = buildOpenCapabilityInstall(
    'https://user:password@ambient.example/base?token=wa_live_test_secret#secret',
    'https://user:password@ambient.example/base/mcp?api_key=wa_live_test_secret'
  );
  assert.equal(instructions.installCommand, "curl -fsSL 'https://ambient.example/base/open/v1/install.sh' | bash");
  assert.equal(instructions.mcpURL, 'https://ambient.example/base/mcp');
  assert.doesNotMatch(JSON.stringify(instructions), /wa_live_test_secret|password|user:|token=/);
});

test('Cursor uses the documented scheme and only a placeholder Authorization', () => {
  const { cursorDeepLink } = buildOpenCapabilityInstall('https://ambient.example', 'https://ambient.example/mcp');
  const link = new URL(cursorDeepLink);
  assert.equal(link.hostname, 'anysphere.cursor-deeplink');
  assert.equal(link.pathname, '/mcp/install');
  assert.equal(link.searchParams.get('name'), 'well-ambient');
  const config = JSON.parse(atob(link.searchParams.get('config')!));
  assert.deepEqual(config, {
    url: 'https://ambient.example/mcp', headers: { Authorization: `Bearer ${OPEN_API_KEY_PLACEHOLDER}` }
  });
  assert.equal(link.searchParams.has('headers'), false);
});

test('invalid or executable URLs cannot create an install link or command', () => {
  const instructions = buildOpenCapabilityInstall('javascript:alert(1)', 'file:///private/key');
  assert.deepEqual(instructions, { mcpURL: '', installCommand: '', cursorDeepLink: '' });
});

test('shell syntax in an allowed URL remains inside a single quoted argument', () => {
  const { installCommand } = buildOpenCapabilityInstall("https://ambient.example/a/$(id)/'", '');
  assert.equal(installCommand, "curl -fsSL 'https://ambient.example/a/$(id)/'\"'\"'/open/v1/install.sh' | bash");
});
