import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import test from 'node:test';

function read(relativePath: string) {
  return readFileSync(new URL(`../${relativePath}`, import.meta.url), 'utf8');
}

const app = read('src/App.svelte');
const setup = read('src/components/DatabaseSetup.svelte');
const compose = read('../compose.yaml');
const deployment = read('../deploy/deploy.sh');
const makefile = read('../Makefile');
const productionEnv = read('../deploy/.env.production.example');

test('the application resolves database setup before rendering login or authenticated workspaces', () => {
  assert.match(app, /type SetupGateState = 'checking' \| 'required' \| 'configured' \| 'unavailable'/);
  assert.match(app, /originalFetch\('\/api\/setup\/status', \{ cache: 'no-store' \}\)/);
  assert.match(app, /response\.status === 404[\s\S]*?setupGateState = 'configured'[\s\S]*?initializeApplication\(\)/);
  assert.match(app, /payload\.setup_required === true[\s\S]*?setupGateState = 'required'/);
  assert.match(app, /\{#if setupGateState === 'checking'\}[\s\S]*?\{:else if setupGateState === 'required'\}[\s\S]*?<DatabaseSetup[\s\S]*?\{:else if !jwtToken\}/);
});

test('setup requests bypass JWT injection and stale-session logout handling', () => {
  assert.match(app, /url\.includes\('\/api\/login'\) \|\| url\.includes\('\/api\/setup\/'\)/);
  assert.match(app, /if \(isPublicBootstrapRequest\(urlStr\)\) \{[\s\S]*?return originalFetch\(resource, init\)/);
  assert.match(app, /response\.status === 401 && !isPublicBootstrapRequest\(urlStr\)/);
});

test('database setup requires a current connection test and never stores secrets in browser persistence', () => {
  assert.match(setup, /testedFingerprint === currentFingerprint/);
  assert.match(setup, /canApply = canContinue && legacyDecisionReady/);
  assert.match(setup, /'X-Setup-Token': setupToken/g);
  assert.match(setup, /maintenance_database: maintenanceDatabase\.trim\(\)/);
  assert.match(setup, /mode: inspection\?\.required_operation/);
  assert.match(setup, /password = ''[\s\S]*?setupToken = ''/);
  assert.doesNotMatch(setup, /localStorage|sessionStorage|indexedDB/);
  assert.doesNotMatch(setup, /console\.(?:log|debug|info|warn|error)/);
});

test('database setup explicitly distinguishes a missing target database from an empty schema', () => {
  assert.match(setup, /SchemaState = 'database_missing' \| 'empty' \| 'well_ambient' \| 'unknown'/);
  assert.match(setup, /目标数据库不存在/);
  assert.match(setup, /当前角色没有 CREATEDB 权限/);
  assert.match(setup, /目标数据库存在且 schema 为空/);
  assert.match(setup, /目标 schema 已有未知表，自动安装已阻止/);
});

test('the second step asks about a server-controlled SQLite snapshot only once', () => {
  assert.match(setup, /step: 1 \| 2/);
  assert.match(setup, /legacySQLite\.available/);
  assert.match(setup, /legacySQLite\.decision_recorded/);
  assert.match(setup, /\/api\/setup\/legacy-sqlite\/decision/);
  assert.match(setup, /该选择只记录一次/);
  assert.doesNotMatch(setup, /type="file"/);
});

test('database setup polls migration progress and recovers across restart', () => {
  assert.match(setup, /\/api\/setup\/database\/operation/);
  assert.match(setup, /tables_completed/);
  assert.match(setup, /rows_copied/);
  assert.match(setup, /response\.status === 404[\s\S]*?onCompleted\(\)/);
  assert.match(setup, /服务重启时间超过预期/);
});

test('server compose requires external PostgreSQL and prebuilt application images', () => {
  assert.doesNotMatch(compose, /^\s{2}postgres:\s*$/m);
  assert.doesNotMatch(compose, /postgres_data|postgres:5432|POSTGRES_PASSWORD|\bbuild:/);
  assert.match(compose, /WELL_AMBIENT_SERVER_IMAGE/);
  assert.match(compose, /WELL_AMBIENT_WEB_IMAGE/);
  assert.match(compose, /host\.docker\.internal:host-gateway/);
  assert.match(productionEnv, /^WELL_AMBIENT_SERVER_IMAGE=/m);
  assert.match(productionEnv, /^WELL_AMBIENT_WEB_IMAGE=/m);
  assert.doesNotMatch(productionEnv, /^POSTGRES_(?:DB|USER|PASSWORD)=/m);
  assert.doesNotMatch(deployment, /export APP_(?:UID|GID)=/);
  assert.match(makefile, /^compose-bundle:/m);
  assert.match(makefile, /well-ambient-compose-\$\(VERSION\)\.tar\.gz/);
});

test('external PostgreSQL upgrades fail closed until an upstream backup is referenced', () => {
  assert.match(deployment, /WELL_AMBIENT_EXTERNAL_BACKUP_REFERENCE/);
  assert.match(deployment, /external PostgreSQL requires a verified upstream snapshot before migration/);
  assert.match(deployment, /external_database_endpoint=%s\\nbackup_reference=%s/);
  assert.doesNotMatch(deployment, /exec -T postgres|\bpg_dump\b|\bpg_restore\b/);
  assert.doesNotMatch(deployment, /"\$\{compose\[@\]\}" up [^\n]*\bpostgres\b/);
});
