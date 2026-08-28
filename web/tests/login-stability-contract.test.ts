import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import test from 'node:test';

const app = readFileSync(new URL('../src/App.svelte', import.meta.url), 'utf8');

test('failed login keeps a persistent feedback slot and stable card geometry', () => {
  assert.match(app, /<div class="login-feedback-slot" aria-live="assertive" aria-atomic="true">/);
  assert.match(app, /class:visible=\{!!loginError\}/);
  assert.match(app, /\.login-feedback-slot\s*\{[\s\S]*?min-height:\s*58px;[\s\S]*?height:\s*58px;/);
  assert.match(app, /\.login-error-alert\s*\{[\s\S]*?visibility:\s*hidden;/);
  assert.match(app, /\.login-error-alert\s*\{[\s\S]*?height:\s*58px;[\s\S]*?overflow-y:\s*auto;/);
  assert.match(app, /\.login-error-alert\.visible\s*\{[\s\S]*?visibility:\s*visible;/);
});

test('login submission is single-flight and the submit button owns a fixed loading label', () => {
  assert.match(app, /async function handleLoginSubmit\(e: Event\)\s*\{[\s\S]*?if \(loggingIn\) return;/);
  assert.match(app, /<form on:submit=\{handleLoginSubmit\} class="login-form">/);
  assert.match(app, /<button type="submit" class="login-submit-btn" disabled=\{loggingIn\} aria-busy=\{loggingIn\}>/);
  assert.match(app, /<span class="login-submit-label">\{loggingIn \? '登录中…' : '立即登录'\}<\/span>/);
  assert.match(app, /\.login-submit-btn\s*\{[\s\S]*?min-height:/);
});
