import assert from 'node:assert/strict';
import { after, test } from 'node:test';
import { fileURLToPath } from 'node:url';
import { createServer } from 'vite';

const server = await createServer({
  root: fileURLToPath(new URL('../', import.meta.url)),
  server: { middlewareMode: true, hmr: false },
  appType: 'custom'
});
after(() => server.close());
const { default: AdminDataList } = await server.ssrLoadModule('/src/components/admin-console/AdminDataList.svelte');
const { render } = await server.ssrLoadModule('svelte/server');
const props = {
  columns: [{ key: 'title', label: '需求' }],
  rows: [{ id: 'SCHEDULE-1', cells: { title: '保留排期数据' } }],
  caption: '排期治理需求列表',
  loading: true
};

test('quiet refresh retains table rows and busy semantics without an overlay notice', () => {
  const { body } = render(AdminDataList, { props: { ...props, showRefreshStatus: false } });
  assert.match(body, /保留排期数据/);
  assert.match(body, /aria-busy="true"/);
  assert.doesNotMatch(body, /loading-status|正在更新，当前结果保留显示/);
});

test('other lists keep the existing refresh notice by default', () => {
  const { body } = render(AdminDataList, { props });
  assert.match(body, /正在更新，当前结果保留显示/);
});

test('quiet refresh preserves initial skeletons and error recovery', () => {
  const initial = render(AdminDataList, { props: { ...props, rows: [], showRefreshStatus: false } }).body;
  assert.match(initial, /skeleton/);
  const failed = render(AdminDataList, { props: { ...props, loading: false, error: '服务暂不可用', onRetry() {}, showRefreshStatus: false } }).body;
  assert.match(failed, /role="alert"/);
  assert.match(failed, /服务暂不可用/);
  assert.match(failed, /重新加载/);
  assert.match(failed, /保留排期数据/);
});
