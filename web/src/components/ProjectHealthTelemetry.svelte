<script lang="ts">
  import { onMount } from 'svelte';
  import Button from './shared/Button.svelte';

  interface ProjectScore {
    id?: number;
    project_key: string;
    project_name: string;
    schedule_health_score: number;
    engineering_quality: number;
    collaboration_effic: number;
    stability_index: number;
    compound_score: number;
    diagnostic: string;
    snapshot_date: string;
  }

  let scores: ProjectScore[] = [];
  let projectConfigs: any[] = [];
  let jiraProjectMap: {[key: string]: string} = {};
  let loading = false;
  let errorMsg = '';
  let searchQuery = '';
  let expandedProjectKey: string | null = null; // 用于展开诊断行

  async function fetchScoresAndConfigs() {
    loading = true;
    errorMsg = '';
    const token = localStorage.getItem('jwt_token');
    
    try {
      // 1. Fetch scores
      const resScores = await fetch('/api/projects/scores', {
        headers: { 'Authorization': `Bearer ${token}` }
      });
      if (!resScores.ok) throw new Error('获取项目健康度打分失败');
      scores = await resScores.json();

      // 2. Fetch configs for robust names matching
      const resConfigs = await fetch('/api/projects/config', {
        headers: { 'Authorization': `Bearer ${token}` }
      });
      if (resConfigs.ok) {
        projectConfigs = await resConfigs.json();
      }

      // 3. Fetch agenda summary for project name matching
      const resSummary = await fetch('/api/agenda/summary', {
        headers: { 'Authorization': `Bearer ${token}` }
      });
      if (resSummary.ok) {
        const summaryData = await resSummary.json();
        const items = summaryData.agenda_items || [];
        const newMap: {[key: string]: string} = {};
        items.forEach((item: any) => {
          if (item.repo) {
            const repoStr = item.repo.trim();
            const lastOpenParen = repoStr.lastIndexOf('(');
            const lastCloseParen = repoStr.lastIndexOf(')');
            if (lastOpenParen > 0 && lastCloseParen > lastOpenParen) {
              const key = repoStr.substring(lastOpenParen + 1, lastCloseParen).trim().toUpperCase();
              const name = repoStr.substring(0, lastOpenParen).trim();
              if (key && name) {
                newMap[key] = name;
              }
            }
          }
        });
        jiraProjectMap = newMap;
      }
    } catch (err: any) {
      errorMsg = err.message || '加载项目健康度遥测失败';
    } finally {
      loading = false;
    }
  }

  onMount(async () => {
    await fetchScoresAndConfigs();
  });

  function getDisplayName(score: ProjectScore): string {
    const keyUpper = score.project_key.toUpperCase();
    if (jiraProjectMap[keyUpper]) {
      return jiraProjectMap[keyUpper];
    }
    const conf = projectConfigs.find(c => c.project_key.toUpperCase() === keyUpper);
    if (conf && conf.project_name && conf.project_name !== `${score.project_key}项目`) {
      return conf.project_name;
    }
    return score.project_name || score.project_key;
  }

  $: filteredScores = scores.filter(s => {
    const displayName = getDisplayName(s);
    if (!searchQuery) return true;
    const q = searchQuery.toLowerCase();
    return s.project_key.toLowerCase().includes(q) || 
           displayName.toLowerCase().includes(q);
  });

  function getScoreColorClass(score: number): string {
    if (score >= 85) return 'score-green';
    if (score >= 70) return 'score-yellow';
    return 'score-red';
  }

  function toggleDiagnostic(key: string) {
    if (expandedProjectKey === key) {
      expandedProjectKey = null;
    } else {
      expandedProjectKey = key;
    }
  }
</script>

<div class="telemetry-panel glass-panel">
  <div class="panel-header flex-header">
    <div class="title-group">
      <span class="eyebrow">SYSTEM HEALTH METRICS</span>
      <h2>🧠 大脑项目健康遥测与决策诊断</h2>
    </div>
    
    <div class="filter-controls font-mono">
      <div class="search-input-wrapper">
        <span class="search-icon">🔍</span>
        <input 
          type="text" 
          placeholder="过滤项目号/项目名称..." 
          bind:value={searchQuery}
          class="telemetry-search-input"
        />
      </div>
      <button class="refresh-btn font-mono" on:click={fetchScoresAndConfigs} disabled={loading}>
        {loading ? '正在同步...' : '🔄 刷新遥测'}
      </button>
    </div>
  </div>

  {#if loading && scores.length === 0}
    <div class="loading-state font-mono">正在实时分析各集成项目健康度指标...</div>
  {:else if errorMsg}
    <div class="error-state font-mono">❌ {errorMsg}</div>
  {:else if filteredScores.length === 0}
    <div class="empty-state font-mono">没有匹配的遥测项目数据</div>
  {:else}
    <div class="table-responsive">
      <table class="telemetry-table">
        <thead>
          <tr>
            <th>项目 (Jira Project)</th>
            <th style="width: 120px; text-align: center;">PHDI 综合健康度</th>
            <th style="width: 140px;">进度排期健康 (SH)</th>
            <th style="width: 140px;">工程质量 (EQ)</th>
            <th style="width: 140px;">指派协同效率 (CE)</th>
            <th style="width: 140px;">缺陷与稳定性 (SI)</th>
            <th style="text-align: right; width: 110px;">操作</th>
          </tr>
        </thead>
        <tbody>
          {#each filteredScores as s}
            <tr class:has-expanded={expandedProjectKey === s.project_key}>
              <td class="project-info-cell">
                <span class="project-avatar font-mono">{s.project_key.slice(0, 2).toUpperCase()}</span>
                <div class="project-meta">
                  <strong>{getDisplayName(s)}</strong>
                  <span class="font-mono text-muted">{s.project_key}</span>
                </div>
              </td>
              <td style="text-align: center;">
                <span class="score-badge font-mono {getScoreColorClass(s.compound_score)}">
                  {s.compound_score}分
                </span>
              </td>
              <!-- SH -->
              <td>
                <div class="metric-progress-wrapper">
                  <div class="metric-val font-mono">{s.schedule_health_score}%</div>
                  <div class="progress-bar-bg">
                    <span class="progress-bar-fill is-blue" style="width: {s.schedule_health_score}%"></span>
                  </div>
                </div>
              </td>
              <!-- EQ -->
              <td>
                <div class="metric-progress-wrapper">
                  <div class="metric-val font-mono">{s.engineering_quality}%</div>
                  <div class="progress-bar-bg">
                    <span class="progress-bar-fill is-emerald" style="width: {s.engineering_quality}%"></span>
                  </div>
                </div>
              </td>
              <!-- CE -->
              <td>
                <div class="metric-progress-wrapper">
                  <div class="metric-val font-mono">{s.collaboration_effic}%</div>
                  <div class="progress-bar-bg">
                    <span class="progress-bar-fill is-amber" style="width: {s.collaboration_effic}%"></span>
                  </div>
                </div>
              </td>
              <!-- SI -->
              <td>
                <div class="metric-progress-wrapper">
                  <div class="metric-val font-mono">{s.stability_index}%</div>
                  <div class="progress-bar-bg">
                    <span class="progress-bar-fill is-rose" style="width: {s.stability_index}%"></span>
                  </div>
                </div>
              </td>
              <td style="text-align: right;">
                <Button size="small" variant="ghost" on:click={() => toggleDiagnostic(s.project_key)}>
                  {expandedProjectKey === s.project_key ? '收起诊断' : '💡 风控诊断'}
                </Button>
              </td>
            </tr>
            {#if expandedProjectKey === s.project_key}
              <tr class="diagnostic-expand-row">
                <td colspan="7">
                  <div class="diagnostic-panel font-mono {getScoreColorClass(s.compound_score)}">
                    <div class="diagnostic-title">🧠 大脑风控诊断意见 ({s.project_key})</div>
                    <p class="diagnostic-content">{s.diagnostic || '当前项目运转状态良好，暂未扫描到风险点。'}</p>
                    <div class="diagnostic-footer">数据截止: {s.snapshot_date || '当前周期'} · 诊断模型 well-ambient v1.0</div>
                  </div>
                </td>
              </tr>
            {/if}
          {/each}
        </tbody>
      </table>
    </div>
  {/if}
</div>

<style>
  .telemetry-panel {
    margin-bottom: 24px;
    box-sizing: border-box;
    background: rgba(10, 15, 30, 0.7);
    border: 1px solid rgba(51, 65, 85, 0.35);
    border-radius: 16px;
    backdrop-filter: blur(12px);
    -webkit-backdrop-filter: blur(12px);
    padding: 22px;
    box-shadow: 0 8px 32px 0 rgba(0, 0, 0, 0.5);
  }

  .flex-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
    gap: 16px;
    flex-wrap: wrap;
    margin-bottom: 20px;
  }

  .eyebrow {
    font-size: 0.65rem;
    font-weight: 800;
    color: #64748b;
    letter-spacing: 0.1em;
    display: block;
    margin-bottom: 6px;
    text-transform: uppercase;
  }

  .title-group h2 {
    margin: 0;
    font-size: 1.25rem;
    font-weight: 800;
    background: linear-gradient(135deg, #ffffff 0%, #cbd5e1 100%);
    -webkit-background-clip: text;
    -webkit-text-fill-color: transparent;
  }

  .filter-controls {
    display: flex;
    align-items: center;
    gap: 12px;
  }

  .search-input-wrapper {
    position: relative;
    width: 240px;
  }

  .search-icon {
    position: absolute;
    left: 10px;
    top: 50%;
    transform: translateY(-50%);
    font-size: 0.8rem;
    color: #64748b;
  }

  .telemetry-search-input {
    width: 100%;
    height: 32px;
    box-sizing: border-box;
    background: rgba(15, 23, 42, 0.6);
    border: 1px solid rgba(71, 85, 105, 0.4);
    border-radius: 6px;
    padding: 0 10px 0 32px;
    color: #e2e8f0;
    font-size: 0.78rem;
    outline: none;
    transition: all 0.2s ease;
  }

  .telemetry-search-input:focus {
    border-color: rgba(99, 102, 241, 0.6);
    box-shadow: 0 0 0 2px rgba(99, 102, 241, 0.15);
  }

  .refresh-btn {
    min-height: 32px;
    background: rgba(99, 102, 241, 0.12);
    border: 1px solid rgba(129, 140, 248, 0.3);
    color: #c7d2fe;
    border-radius: 6px;
    padding: 6px 14px;
    font-size: 0.74rem;
    font-weight: 700;
    cursor: pointer;
    transition: all 0.2s ease;
  }

  .refresh-btn:hover {
    background: rgba(99, 102, 241, 0.22);
    border-color: rgba(129, 140, 248, 0.5);
    color: #ffffff;
  }

  .refresh-btn:active {
    transform: scale(0.97);
  }

  .loading-state, .error-state, .empty-state {
    padding: 40px;
    text-align: center;
    border: 1px dashed rgba(51, 65, 85, 0.4);
    border-radius: 12px;
    color: #64748b;
    font-size: 0.82rem;
    background: rgba(10, 15, 30, 0.3);
  }

  .error-state {
    color: #fca5a5;
    background: rgba(127, 29, 29, 0.1);
    border-color: rgba(248, 113, 113, 0.2);
  }

  .table-responsive {
    overflow-x: auto;
    border-radius: 12px;
    border: 1px solid rgba(51, 65, 85, 0.35);
    background: rgba(8, 13, 28, 0.4);
  }

  .telemetry-table {
    width: 100%;
    border-collapse: collapse;
    text-align: left;
    font-size: 0.8rem;
  }

  .telemetry-table th {
    background: rgba(11, 19, 41, 0.8);
    color: #94a3b8;
    font-size: 0.74rem;
    font-weight: 700;
    padding: 12px 16px;
    border-bottom: 1px solid rgba(51, 65, 85, 0.5);
    text-transform: uppercase;
    letter-spacing: 0.05em;
  }

  .telemetry-table td {
    padding: 14px 16px;
    border-bottom: 1px solid rgba(51, 65, 85, 0.25);
    vertical-align: middle;
  }

  .telemetry-table tr:last-child td {
    border-bottom: none;
  }

  .telemetry-table tr:hover td {
    background: rgba(30, 41, 59, 0.25);
  }

  .telemetry-table tr.has-expanded td {
    border-bottom-color: transparent;
    background: rgba(30, 41, 59, 0.15);
  }

  .project-info-cell {
    display: flex;
    align-items: center;
    gap: 12px;
  }

  .project-avatar {
    width: 32px;
    height: 32px;
    border-radius: 6px;
    background: linear-gradient(135deg, #3b82f6 0%, #8b5cf6 100%);
    color: #ffffff;
    display: flex;
    align-items: center;
    justify-content: center;
    font-weight: 800;
    font-size: 0.85rem;
    border: 1px solid rgba(255, 255, 255, 0.15);
    flex-shrink: 0;
  }

  .project-meta {
    display: flex;
    flex-direction: column;
    gap: 2px;
    min-width: 0;
  }

  .project-meta strong {
    color: #e2e8f0;
    font-size: 0.85rem;
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }

  .text-muted {
    color: #64748b;
    font-size: 0.72rem;
  }

  /* Score Badges */
  .score-badge {
    display: inline-block;
    padding: 4px 10px;
    border-radius: 6px;
    font-size: 0.78rem;
    font-weight: 800;
    text-shadow: 0 2px 4px rgba(0, 0, 0, 0.4);
  }

  .score-badge.score-green {
    background: rgba(16, 185, 129, 0.16);
    color: #34d399;
    border: 1px solid rgba(16, 185, 129, 0.35);
  }

  .score-badge.score-yellow {
    background: rgba(245, 158, 11, 0.14);
    color: #fbbf24;
    border: 1px solid rgba(245, 158, 11, 0.3);
  }

  .score-badge.score-red {
    background: rgba(244, 63, 94, 0.16);
    color: #f43f5e;
    border: 1px solid rgba(244, 63, 94, 0.35);
  }

  /* Progress bars inside table */
  .metric-progress-wrapper {
    display: flex;
    flex-direction: column;
    gap: 4px;
  }

  .metric-val {
    font-size: 0.72rem;
    color: #cbd5e1;
    font-weight: 700;
  }

  .progress-bar-bg {
    height: 5px;
    background: rgba(30, 41, 59, 0.6);
    border-radius: 3px;
    overflow: hidden;
    position: relative;
  }

  .progress-bar-fill {
    display: block;
    height: 100%;
    border-radius: 3px;
  }

  .progress-bar-fill.is-blue { background: #6366f1; }
  .progress-bar-fill.is-emerald { background: #10b981; }
  .progress-bar-fill.is-amber { background: #f59e0b; }
  .progress-bar-fill.is-rose { background: #f43f5e; }

  /* Diagnostic Panel */
  .diagnostic-expand-row td {
    padding: 0 16px 14px 16px !important;
    border-bottom: 1px solid rgba(51, 65, 85, 0.25) !important;
    background: rgba(30, 41, 59, 0.15) !important;
  }

  .diagnostic-panel {
    border-radius: 8px;
    padding: 14px 18px;
    animation: slideDown 0.25s ease-out;
  }

  .diagnostic-panel.score-green {
    background: rgba(16, 185, 129, 0.05);
    border: 1px solid rgba(16, 185, 129, 0.2);
  }

  .diagnostic-panel.score-yellow {
    background: rgba(245, 158, 11, 0.04);
    border: 1px solid rgba(245, 158, 11, 0.15);
  }

  .diagnostic-panel.score-red {
    background: rgba(244, 63, 94, 0.05);
    border: 1px solid rgba(244, 63, 94, 0.2);
  }

  .diagnostic-title {
    font-size: 0.76rem;
    font-weight: 800;
    margin-bottom: 6px;
  }

  .diagnostic-panel.score-green .diagnostic-title { color: #34d399; }
  .diagnostic-panel.score-yellow .diagnostic-title { color: #fbbf24; }
  .diagnostic-panel.score-red .diagnostic-title { color: #f43f5e; }

  .diagnostic-content {
    margin: 0 0 10px 0;
    font-size: 0.76rem;
    line-height: 1.5;
    color: #cbd5e1;
  }

  .diagnostic-footer {
    font-size: 0.65rem;
    color: #64748b;
    border-top: 1px dashed rgba(71, 85, 105, 0.25);
    padding-top: 6px;
  }

  @keyframes slideDown {
    from { opacity: 0; transform: translateY(-4px); }
    to { opacity: 1; transform: translateY(0); }
  }
</style>
