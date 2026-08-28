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
const dockerfile = read('../Dockerfile');
const dockerignore = read('../.dockerignore');
const serverMain = read('../cmd/server/main.go');
const setupTokenProvision = read('../internal/server/setup_token.go');
const localConfig = read('../config.example.yaml');

test('setup mode generates an owner-only temporary token when no explicit token is configured', () => {
  assert.match(serverMain, /server\.ProvisionSetupToken\(os\.Getenv\(server\.SetupTokenEnvironment\)\)/);
  assert.match(
    serverMain,
    /if setupToken\.Generated \{[\s\S]*?Generated one-time database setup token:[\s\S]*?setupToken\.FilePath/,
  );
  assert.match(serverMain, /setupToken\.Cleanup\(\)/);

  assert.match(setupTokenProvision, /crypto\/rand/);
  assert.match(setupTokenProvision, /os\.CreateTemp\(tempDir, "well-ambient-setup-token-\*\.txt"\)/);
  assert.match(setupTokenProvision, /file\.Chmod\(0o600\)/);
  assert.match(setupTokenProvision, /file\.Sync\(\)/);
  assert.match(setupTokenProvision, /func \(p SetupTokenProvision\) Cleanup\(\) error/);

  assert.match(compose, /WELL_AMBIENT_SETUP_TOKEN: \$\{WELL_AMBIENT_SETUP_TOKEN:-\}/);
  assert.match(productionEnv, /^WELL_AMBIENT_SETUP_TOKEN=$/m);
  assert.match(deployment, /\[\[ -n "\$setup_token" \]\][\s\S]*?\$\{#setup_token\} < 32/);
  assert.match(deployment, /logs --no-color --tail 100 server/);
});

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
  assert.match(setup, /can_migrate_legacy/);
  assert.match(setup, /inspection\.can_migrate_legacy/);
  assert.match(setup, /开始迁移/);
  assert.match(setup, /已包含业务数据，本地 SQLite 迁移已停用以避免覆盖/);
  assert.match(setup, /\/api\/setup\/legacy-sqlite\/decision/);
  assert.match(setup, /该选择只记录一次/);
  assert.doesNotMatch(setup, /type="file"/);
});

test('database setup polls migration progress and recovers across restart', () => {
  assert.match(setup, /\/api\/setup\/database\/operation/);
  assert.match(setup, /tables_completed/);
  assert.match(setup, /rows_copied/);
  assert.match(setup, /function operationProgressPercent/);
  assert.match(setup, /checking_database:\s*8/);
  assert.match(setup, /verifying_counts:\s*90/);
  assert.match(setup, /class="progress-stages"/);
  assert.match(setup, /当前表/);
  assert.match(setup, /response\.status === 404[\s\S]*?onCompleted\(\)/);
  assert.match(setup, /服务重启时间超过预期/);
});

test('database setup keeps the first screen concise and hides maintenance settings by default', () => {
  assert.doesNotMatch(setup, /class="setup-brand"|class="setup-security-note"|class="setup-summary"/);
  assert.doesNotMatch(setup, /class="setup-check"|class="check-list"/);
  assert.match(setup, /<details class="advanced-options">[\s\S]*?<summary>高级连接选项<\/summary>[\s\S]*?id="setup-maintenance-database"/);
  assert.doesNotMatch(setup, /<details class="advanced-options"\s+open/);
});

test('database setup keeps the native SSL select aligned with other form controls', () => {
  assert.match(setup, /<div class="select-control">[\s\S]*?<select id="setup-ssl-mode"[\s\S]*?<svg aria-hidden="true" class="select-chevron"[\s\S]*?<path d="M1\.5 1\.5 6 6l4\.5-4\.5"><\/path>[\s\S]*?<\/svg>[\s\S]*?<\/div>/);
  assert.match(setup, /\.select-control select\s*\{[\s\S]*?appearance:\s*none;[\s\S]*?padding-right:\s*40px;/);
  assert.match(setup, /\.select-chevron\s*\{[\s\S]*?pointer-events:\s*none;/);
});

test('local development defaults to the guarded PostgreSQL setup flow', () => {
  assert.match(localConfig, /^database:\n(?:.*\n)*?\s+driver:\s+setup\s*$/m);
  assert.doesNotMatch(localConfig, /^\s+driver:\s+sqlite\s*$/m);
  assert.doesNotMatch(localConfig, /^\s+dsn:\s+well-ambient\.db\s*$/m);
});

test('database setup previews a detected SQLite snapshot before the migration decision', () => {
  assert.match(setup, /legacySQLite\.available[\s\S]*?class="legacy-detected"/);
  assert.match(setup, /已检测到本地 SQLite/);
  assert.match(setup, /连接测试后可选择是否迁移/);
});

test('database setup uses one centered formal workflow at every breakpoint', () => {
  assert.match(setup, /\.database-setup\s*\{[\s\S]*?grid-template-columns:\s*minmax\(0,\s*1fr\);[\s\S]*?justify-items:\s*center;/);
  assert.match(setup, /\.setup-header\s*\{[\s\S]*?max-width:\s*760px;[\s\S]*?text-align:\s*center;/);
  assert.match(setup, /\.setup-workspace\s*\{[\s\S]*?max-width:\s*760px;[\s\S]*?justify-self:\s*center;/);
  assert.doesNotMatch(setup, /grid-template-columns:\s*minmax\(260px,\s*0\.78fr\)\s*minmax\(0,\s*1\.7fr\)/);
  assert.doesNotMatch(setup, /justify-self:\s*end/);
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
  assert.match(makefile, /^release-metadata:/m);
  assert.match(makefile, /^compose-bundle: release-metadata$/m);
  assert.match(makefile, /well-ambient-compose-\$\$WELL_AMBIENT_VERSION\.tar\.gz/);
  assert.match(makefile, /deploy\/generated\/release-notes\.txt/);
});

test('production image stages keep build toolchains out of both runtimes', () => {
  assert.match(dockerfile, /FROM [^\n]*debian:bookworm-slim AS server/);
  assert.match(dockerfile, /FROM [^\n]*nginx:1\.31\.0 AS web/);
  assert.doesNotMatch(dockerfile, /FROM (?:golang|node):[^\n]+ AS (?:server|web)$/m);

  const serverRuntime = dockerfile.match(/FROM [^\n]+ AS server\n([\s\S]*?)\nFROM [^\n]+ AS web/)?.[1] ?? '';
  assert.doesNotMatch(serverRuntime, /GOPROXY|go mod|pnpm|node_modules/);
  assert.match(
    serverRuntime,
    /COPY --from=server-build \/etc\/ssl\/certs\/ca-certificates\.crt \/etc\/ssl\/certs\/ca-certificates\.crt/,
  );
  assert.match(serverRuntime, /ENV SSL_CERT_FILE=\/etc\/ssl\/certs\/ca-certificates\.crt/);
  assert.match(dockerignore, /^\.git$/m);
  assert.match(dockerignore, /^well-ambient\.db-\*$/m);
  assert.match(dockerignore, /^web\/node_modules$/m);
  assert.match(dockerignore, /^web\/dist$/m);
});

test('the migration command synchronizes current settings before application rollout', () => {
  assert.match(serverMain, /if \*migrateOnly \{[\s\S]*?server\.BootstrapVersionedConfig\(cfg\)[\s\S]*?configuration synchronization completed successfully/);
});

test('external PostgreSQL upgrades fail closed until an upstream backup is referenced', () => {
  assert.match(deployment, /WELL_AMBIENT_EXTERNAL_BACKUP_REFERENCE/);
  assert.match(deployment, /external PostgreSQL requires a verified upstream snapshot before migration/);
  assert.match(deployment, /external_database_endpoint=%s\\nbackup_reference=%s/);
  assert.doesNotMatch(deployment, /exec -T postgres|\bpg_dump\b|\bpg_restore\b/);
  assert.doesNotMatch(deployment, /"\$\{compose\[@\]\}" up [^\n]*\bpostgres\b/);
});
