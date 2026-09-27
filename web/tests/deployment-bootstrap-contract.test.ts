import assert from 'node:assert/strict';
import {
  existsSync,
  mkdtempSync,
  mkdirSync,
  readFileSync,
  readdirSync,
  rmSync,
  statSync,
  writeFileSync,
} from 'node:fs';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { spawnSync } from 'node:child_process';
import test from 'node:test';

function repoFile(relativePath: string): string {
  return readFileSync(new URL(`../../${relativePath}`, import.meta.url), 'utf8');
}

function mode(filePath: string): number {
  return statSync(filePath).mode & 0o777;
}

function shellFunctionBody(source: string, name: string): string {
  const match = source.match(new RegExp(`${name}\\(\\) \\{([\\s\\S]*?)\\n\\}`));
  assert.ok(match, `${name} function must exist`);
  return match[1];
}

const helperURL = new URL('../../deploy/prepare-runtime-config.sh', import.meta.url);
const deployment = repoFile('deploy/deploy.sh');
const runtimePreparation = repoFile('deploy/prepare-runtime-config.sh');
const deploymentDoc = repoFile('docs/deployment-linux-postgres.md');
const makefile = repoFile('Makefile');

test('deploy backfills the mounted legacy SQLite path with owner-only atomic files', (t) => {
  const fixture = mkdtempSync(join(tmpdir(), 'well-ambient-runtime-config-'));
  const runtimeConfig = join(fixture, 'config.yaml');
  const templateConfig = join(fixture, 'template.yaml');
  const dataDir = join(fixture, 'data');
  const legacyDir = join(dataDir, 'legacy');
  const legacyFile = join(legacyDir, 'well-ambient.db');
  t.after(() => rmSync(fixture, { recursive: true, force: true }));

  writeFileSync(runtimeConfig, 'database:\n  driver: setup\n  auto_migrate: false\nserver:\n  port: 8080\n');
  writeFileSync(templateConfig, 'database:\n  driver: setup\n');
  mkdirSync(legacyDir, { recursive: true });
  writeFileSync(legacyFile, Buffer.from('SQLite format 3\0fixture'));

  const result = spawnSync('bash', [helperURL.pathname, runtimeConfig, templateConfig, legacyFile], {
    encoding: 'utf8',
  });

  assert.equal(result.status, 0, result.stderr);
  const prepared = readFileSync(runtimeConfig, 'utf8');
  assert.match(prepared, /^\s{2}legacy_sqlite_path:\s+\/var\/lib\/well-ambient\/legacy\/well-ambient\.db$/m);
  assert.equal((prepared.match(/^\s{2}legacy_sqlite_path:/gm) || []).length, 1);
  assert.equal(mode(fixture), 0o700);
  assert.equal(mode(dataDir), 0o700);
  assert.equal(mode(legacyDir), 0o700);
  assert.equal(mode(runtimeConfig), 0o600);
  assert.equal(mode(legacyFile), 0o600);
  assert.deepEqual(readdirSync(fixture).filter((name) => name.startsWith('.config.yaml.tmp.')), []);
  assert.match(deployment, /prepare-runtime-config\.sh/);
});

test('runtime preparation preserves an explicitly configured legacy SQLite path and modes', (t) => {
  const fixture = mkdtempSync(join(tmpdir(), 'well-ambient-runtime-custom-'));
  const runtimeConfig = join(fixture, 'config.yaml');
  const templateConfig = join(fixture, 'template.yaml');
  const dataDir = join(fixture, 'data');
  const legacyDir = join(dataDir, 'legacy');
  const legacyFile = join(legacyDir, 'well-ambient.db');
  t.after(() => rmSync(fixture, { recursive: true, force: true }));

  writeFileSync(runtimeConfig, 'database:\n  driver: setup\n  legacy_sqlite_path: /custom/archive.db\n');
  writeFileSync(templateConfig, 'database:\n  driver: setup\n');
  mkdirSync(legacyDir, { recursive: true });
  writeFileSync(legacyFile, Buffer.from('SQLite format 3\0fixture'));

  const result = spawnSync('bash', [helperURL.pathname, runtimeConfig, templateConfig, legacyFile], {
    encoding: 'utf8',
  });

  assert.equal(result.status, 0, result.stderr);
  assert.match(readFileSync(runtimeConfig, 'utf8'), /legacy_sqlite_path: \/custom\/archive\.db/);
  assert.equal(mode(fixture), 0o700);
  assert.equal(mode(dataDir), 0o700);
  assert.equal(mode(legacyDir), 0o700);
  assert.equal(mode(runtimeConfig), 0o600);
  assert.equal(mode(legacyFile), 0o600);
});

test('runtime preparation rejects invalid SQLite input without deleting or rewriting it', (t) => {
  const fixture = mkdtempSync(join(tmpdir(), 'well-ambient-runtime-invalid-'));
  const runtimeConfig = join(fixture, 'config.yaml');
  const templateConfig = join(fixture, 'template.yaml');
  const dataDir = join(fixture, 'data');
  const legacyDir = join(dataDir, 'legacy');
  const legacyFile = join(legacyDir, 'well-ambient.db');
  const invalidContents = 'incomplete upload';
  t.after(() => rmSync(fixture, { recursive: true, force: true }));

  writeFileSync(templateConfig, 'database:\n  driver: setup\n');
  mkdirSync(legacyDir, { recursive: true });
  writeFileSync(legacyFile, invalidContents);
  const result = spawnSync('bash', [helperURL.pathname, runtimeConfig, templateConfig, legacyFile], {
    encoding: 'utf8',
  });

  assert.notEqual(result.status, 0);
  assert.match(result.stderr, /not a valid SQLite snapshot/);
  assert.equal(existsSync(legacyFile), true);
  assert.equal(readFileSync(legacyFile, 'utf8'), invalidContents);
  assert.equal(mode(fixture), 0o700);
  assert.equal(mode(dataDir), 0o700);
  assert.equal(mode(legacyDir), 0o700);
  assert.equal(mode(runtimeConfig), 0o600);
  assert.equal(mode(legacyFile), 0o600);
  assert.deepEqual(readdirSync(fixture).filter((name) => name.startsWith('.config.yaml.tmp.')), []);
});

test('deployment scripts keep runtime directories and generated files owner-only', () => {
  for (const script of [deployment, runtimePreparation]) {
    assert.doesNotMatch(script, /\bchmod\s+0?(?:755|644|777)\b/);
    assert.match(script, /chmod 0700/);
    assert.match(script, /chmod 0600/);
  }

  assert.match(runtimePreparation, /mktemp "\$runtime_config_dir\/\.\$\{runtime_config_base\}\.tmp\.XXXXXX"/);
  assert.match(runtimePreparation, /mv "\$temporary_config" "\$runtime_config"/);
  assert.match(deployment, /find "\$runtime_dir" "\$legacy_dir" "\$state_dir" "\$backup_dir"[^\n]+chmod 0600/);
  assert.match(deployment, /chmod 0600 "\$state_dir\/current-version"/);
  assert.match(deployment, /chmod 0600 "\$backup_path"/);
});

test('invalid canonical snapshots are quarantined without deletion or clobbering', () => {
  assert.match(deployment, /chmod 0600 "\$source_path"[\s\S]*date -u \+%Y%m%dT%H%M%SZ/);
  assert.match(deployment, /quarantine_base="\$\{source_path\}\.invalid-\$\{timestamp\}"/);
  assert.match(deployment, /quarantine_path="\$\{quarantine_base\}\.\$\{suffix\}"/);
  assert.match(deployment, /mv -n "\$source_path" "\$quarantine_path"/);
  assert.doesNotMatch(deployment, /rm -f "\$legacy_snapshot"/);
});

test('auto discovery is sorted, NUL-safe, diagnostic, and never interactive', () => {
  const cleanup = shellFunctionBody(deployment, 'cleanup_legacy_temp');
  const manifestCreation = shellFunctionBody(deployment, 'create_candidate_manifest');
  const candidateCase = deployment.match(
    /case \$\{#valid_legacy_candidates\[@\]\} in([\s\S]*?)\n  esac/,
  );
  assert.ok(candidateCase, 'valid candidate count case must exist');
  const multipleCandidateBranch = candidateCase[1].match(/\n    \*\)([\s\S]*?)\n      ;;/);
  assert.ok(multipleCandidateBranch, 'multi-candidate branch must exist');

  assert.match(deployment, /find "\$legacy_dir"[^\n]+-print0/);
  assert.match(deployment, /printf '%s\\0' "\$project_root\/well-ambient\.db"/);
  assert.match(deployment, /printf '%s\\0' "\$project_root\/data\.db"/);
  assert.match(deployment, /LC_ALL=C sort -zu/);
  assert.match(deployment, /自动候选有效/);
  assert.match(deployment, /自动候选无效/);
  assert.match(deployment, /case \$\{#valid_legacy_candidates\[@\]\} in/);
  assert.match(deployment, /发现多个有效的 SQLite 自动迁移候选/);
  assert.match(deployment, /--legacy-sqlite PATH or WELL_AMBIENT_LEGACY_SQLITE=PATH/);
  assert.match(multipleCandidateBranch[1], /\bexit 2\b/);
  assert.match(manifestCreation, /mktemp "\$runtime_dir\/\.legacy-candidates\.XXXXXX"/);
  assert.match(manifestCreation, /chmod 0600 "\$candidate_manifest"/);
  assert.match(cleanup, /rm -f "\$candidate_manifest"/);
  assert.match(deployment, /trap cleanup_legacy_temp EXIT/);
  const discoveryFailure = deployment.match(
    /if ! discover_auto_legacy_candidates >"\$candidate_manifest"; then([\s\S]*?)\n  fi/,
  );
  assert.ok(discoveryFailure, 'candidate discovery failure branch must exist');
  assert.match(discoveryFailure[1], /find 或 sort 返回错误/);
  assert.match(discoveryFailure[1], /\bexit 2\b/);
  assert.match(
    deployment,
    /done <"\$candidate_manifest"\n  rm -f "\$candidate_manifest"\n  candidate_manifest=""/,
  );
  assert.doesNotMatch(deployment, /done < <\(discover_auto_legacy_candidates\)/);
  assert.doesNotMatch(deployment, /read -r -p|\/dev\/tty|\[\[ -t 0 \]\]/);
});

test('explicit snapshots copy atomically while auto candidates require sqlite3 backup', () => {
  const explicitImport = shellFunctionBody(deployment, 'install_explicit_legacy_sqlite_snapshot');
  const autoImport = shellFunctionBody(deployment, 'install_auto_legacy_sqlite_snapshot');
  const completedInstall = shellFunctionBody(deployment, 'install_completed_legacy_temp');

  assert.match(deployment, /--legacy-sqlite/);
  assert.match(deployment, /WELL_AMBIENT_LEGACY_SQLITE/);
  assert.match(explicitImport, /cp "\$source_path" "\$legacy_temp"/);
  assert.match(explicitImport, /install_completed_legacy_temp/);
  assert.match(autoImport, /command -v sqlite3/);
  assert.match(autoImport, /sqlite3 "\$source_path" "\.backup '\$temp_name'"/);
  assert.doesNotMatch(autoImport, /\bcp\b/);
  assert.match(completedInstall, /mv "\$legacy_temp" "\$legacy_snapshot"/);
  assert.doesNotMatch(deployment, /回退至文件复制|cp -f "\$source_path" "\$target_path"/);
  assert.match(makefile, /LEGACY_SQLITE \?=/);
  assert.match(makefile, /SQLITE_PATH \?=/);
});

test('deployment documentation states the owner-only and fail-closed migration contract', () => {
  assert.match(deploymentDoc, /运行目录和文件仅所有者可访问/);
  assert.match(deploymentDoc, /internal\/config\.SaveConfig/);
  assert.match(deploymentDoc, /0 个有效候选/);
  assert.match(deploymentDoc, /1 个有效候选/);
  assert.match(deploymentDoc, /多于 1 个有效候选/);
  assert.match(deploymentDoc, /sqlite3 \.backup/);
  assert.match(deploymentDoc, /well-ambient\.db\.invalid-YYYYMMDDTHHMMSSZ/);
  assert.match(deploymentDoc, /无效文件不会被删除/);
  assert.match(deploymentDoc, /--legacy-sqlite/);
  assert.match(deploymentDoc, /WELL_AMBIENT_LEGACY_SQLITE/);
});

test('deploy verifies the mounted snapshot through the real setup status endpoint', () => {
  assert.match(deployment, /legacy_snapshot="\$runtime_dir\/data\/legacy\/well-ambient\.db"/);
  assert.match(
    deployment,
    /if \[\[ "\$database_driver" == "setup" \]\]; then[\s\S]*?up -d --force-recreate --wait --wait-timeout 240 server web/,
  );
  assert.match(deployment, /\/api\/setup\/status/);
  assert.match(deployment, /legacy_sqlite[^\n]+available/);
  assert.match(deployment, /expected container identity/);
  assert.match(deployment, /sudo chown \$container_app_uid:\$container_app_gid/);
  assert.match(deployment, /sudo chmod 0700/);
  assert.match(deployment, /sudo chmod 0600/);
});

test('one-command local test entrypoint reuses the isolated setup service lifecycle', () => {
  const localScriptURL = new URL('../../scripts/dev-setup.sh', import.meta.url);
  const localScript = readFileSync(localScriptURL, 'utf8');

  assert.ok((statSync(localScriptURL).mode & 0o111) !== 0, 'scripts/dev-setup.sh must be executable');
  assert.match(localScript, /run_backend/);
  assert.match(makefile, /^dev-setup:\n\t\.\/scripts\/dev-setup\.sh$/m);
});
