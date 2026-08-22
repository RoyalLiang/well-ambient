import assert from 'node:assert/strict';
import test from 'node:test';

import { dailyJiraSnapshotFingerprint } from '../src/lib/daily-jira-snapshot.ts';

function snapshot(generatedAt: string, title = 'Stable item') {
  return {
    generated_at: generatedAt,
    summary: { total: 1, today: 0, three_day: 0, seven_day: 1 },
    buckets: [{
      key: 'seven_day',
      label: '7 日及以上',
      description: '创建至少 7 天仍未解决',
      count: 1,
      items: [{ task_id: 'WA-900', title, assignee: 'Alice', status: 'progress' }]
    }],
    assignees: ['Alice'],
    recent_watch_count: 0,
    unclassified_count: 0
  };
}

test('poll timestamps alone do not invalidate a Daily Jira business snapshot', () => {
  assert.equal(
    dailyJiraSnapshotFingerprint(snapshot('2026-08-21T10:00:00+08:00')),
    dailyJiraSnapshotFingerprint(snapshot('2026-08-21T10:00:30+08:00'))
  );
});

test('material Daily Jira changes invalidate the snapshot', () => {
  assert.notEqual(
    dailyJiraSnapshotFingerprint(snapshot('2026-08-21T10:00:00+08:00')),
    dailyJiraSnapshotFingerprint(snapshot('2026-08-21T10:00:30+08:00', 'Changed item'))
  );
});
