import assert from 'node:assert/strict';
import test from 'node:test';
import { normalizeReviewRepoPolicies } from '../src/lib/review-repo-policy.ts';

test('repository responses expose names and nested persisted policy fields to the editor', () => {
  const response = [{
    project_id: '1696',
    name: 'well-ambient',
    policy: {
      project_id: '1696',
      domain: 'fms',
      scenario: 'yardmove',
      knowledge_scope: 'fms:yardmove',
      sync_mrs: true,
      sync_commits: false,
      rules: '- 完成必须有证据\n- 不得以空字符串表示完成',
    },
  }];
  const before = structuredClone(response);

  assert.deepEqual(normalizeReviewRepoPolicies(response), [{
    project_id: '1696',
    review_commits:true, review_mrs:true, repo_name: 'well-ambient',
    domain: 'fms',
    scenario: 'yardmove',
    knowledge_scope: 'fms:yardmove',
    sync_mrs: true,
    sync_commits: false,
    rules: '- 完成必须有证据\n- 不得以空字符串表示完成',
  }]);
  assert.deepEqual(response, before);
});

test('explicit empty policy strings and false publication settings are not replaced by defaults', () => {
  assert.deepEqual(normalizeReviewRepoPolicies([{
    project_id: '1647',
    name: '',
    policy: {
      domain: '',
      scenario: '',
      knowledge_scope: '',
      sync_mrs: false,
      sync_commits: false,
      rules: '',
    },
  }]), [{
    project_id: '1647',
    review_commits:true, review_mrs:true, repo_name: '',
    domain: '',
    scenario: '',
    knowledge_scope: '',
    sync_mrs: false,
    sync_commits: false,
    rules: '',
  }]);
});

test('missing policy or name uses the same unset-policy values as the backend', () => {
  for (const repo of [{ project_id: '1716' }, { project_id: '1716', name: null, policy: null }]) {
    assert.deepEqual(normalizeReviewRepoPolicies([repo]), [{
      project_id: '1716',
      review_commits:true, review_mrs:true, repo_name: '',
      domain: 'general',
      scenario: 'general',
      knowledge_scope: '',
      sync_mrs: false,
      sync_commits: false,
      rules: '',
    }]);
  }
});

test('repository IDs remain strings without coercion or replacement by a nested policy ID', () => {
  const ids = ['0001696', '9007199254740993', 'group/project'];
  const normalized = normalizeReviewRepoPolicies(ids.map(project_id => ({
    project_id,
    policy: { project_id: 'another-repository' },
  })));

  assert.deepEqual(normalized.map(repo => repo.project_id), ids);
});

test('malformed responses do not create repositories or enable publication from truthy non-booleans', () => {
  for (const response of [undefined, null, {}, 'repos']) {
    assert.deepEqual(normalizeReviewRepoPolicies(response), []);
  }
  assert.deepEqual(normalizeReviewRepoPolicies([
    null, [], {}, { project_id: 1696 }, { project_id: '' }, { project_id: '  ' },
    {
      project_id: '2047',
      name: 2047,
      policy: { domain: null, scenario: 12, knowledge_scope: {}, rules: [], sync_mrs: 'false', sync_commits: 1 },
    },
  ]), [{
    project_id: '2047',
    review_commits:true, review_mrs:true, repo_name: '',
    domain: 'general',
    scenario: 'general',
    knowledge_scope: '',
    sync_mrs: false,
    sync_commits: false,
    rules: '',
  }]);
});

test('MR and commit review defaults preserve legacy policies and explicit independent gates',()=>{const rows=normalizeReviewRepoPolicies([{project_id:'a',policy:{review_mrs:false,review_commits:true}},{project_id:'b',policy:{review_mrs:true,review_commits:false}}]);assert.deepEqual(rows.map(r=>[r.review_mrs,r.review_commits]),[[false,true],[true,false]])});
