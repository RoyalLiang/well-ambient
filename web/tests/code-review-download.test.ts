import assert from 'node:assert/strict';
import test from 'node:test';
import { attachmentFilename } from '../src/lib/code-review-download.ts';
test('download uses UTF-8 server filenames and safe fallbacks',()=>{
 assert.equal(attachmentFilename("attachment; filename*=UTF-8''gui_server_feature-FZ-2661_mr1_2026-10-24.md",'fallback.md'),'gui_server_feature-FZ-2661_mr1_2026-10-24.md');
 assert.equal(attachmentFilename("attachment; filename*=utf-8''%E7%A6%8F%E5%B7%9E_dev_mr1_2026-10-24.md",'fallback.md'),'福州_dev_mr1_2026-10-24.md');
 assert.equal(attachmentFilename('attachment; filename="repo_dev_mr1_2026-10-24.md"','fallback.md'),'repo_dev_mr1_2026-10-24.md');
 assert.equal(attachmentFilename("attachment; filename*=UTF-8''%ZZ",'fallback.md'),'fallback.md');
 assert.equal(attachmentFilename(null,'fallback.md'),'fallback.md');
 assert.equal(attachmentFilename('attachment; filename="../evil.md"','fallback.md'),'..-evil.md');
});
