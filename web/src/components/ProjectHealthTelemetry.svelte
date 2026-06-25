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
  let isPanelCollapsed = true; // 默认折叠

  // Details Modal states
  let showDetailsModal = false;
  let selectedProjectScore: ProjectScore | null = null;

  function showProjectDetails(score: ProjectScore) {
    selectedProjectScore = score;
    showDetailsModal = true;
  }

  function closeDetailsModal() {
    showDetailsModal = false;
    selectedProjectScore = null;
  }

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
        const newMap: {[key: string]: string} = summaryData.project_map || {};
        
        if (Object.keys(newMap).length === 0) {
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
        }
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

  function cleanProjectName(name: string): string {
    if (!name) return '';
    const lastOpenParen = name.lastIndexOf('(');
    const lastCloseParen = name.lastIndexOf(')');
    if (lastOpenParen > 0 && lastCloseParen > lastOpenParen) {
      const potentialKey = name.substring(lastOpenParen + 1, lastCloseParen).trim();
      if (potentialKey && /^[A-Z0-9]{2,10}$/i.test(potentialKey)) {
        return name.substring(0, lastOpenParen).trim();
      }
    }
    return name;
  }

  function getDisplayName(score: ProjectScore): string {
    const keyUpper = score.project_key.toUpperCase();
    if (jiraProjectMap[keyUpper]) {
      return cleanProjectName(jiraProjectMap[keyUpper]);
    }
    const conf = projectConfigs.find(c => c.project_key.toUpperCase() === keyUpper);
    if (conf && conf.project_name && conf.project_name !== `${score.project_key}项目`) {
      return cleanProjectName(conf.project_name);
    }
    return cleanProjectName(score.project_name) || score.project_key;
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

  function portal(node: HTMLElement) {
    document.body.appendChild(node);
    return {
      destroy() {
        if (node.parentNode) {
          node.parentNode.removeChild(node);
        }
      }
    };
  }
</script>

<div class="telemetry-panel glass-panel">
  <div class="panel-header flex-header">
    <div class="title-group" on:click={() => isPanelCollapsed = !isPanelCollapsed} style="cursor: pointer; user-select: none;">
      <span class="eyebrow">SYSTEM HEALTH METRICS</span>
      <h2>
        <span class="pulse-status-dot online"></span>
        🧠 项目健康遥测
        <span class="collapse-chevron" class:is-collapsed={isPanelCollapsed}>
          <svg viewBox="0 0 24 24" width="16" height="16" stroke="currentColor" stroke-width="2.5" fill="none" stroke-linecap="round" stroke-linejoin="round">
            <polyline points="6 9 12 15 18 9"></polyline>
          </svg>
        </span>
      </h2>
    </div>
    
    {#if !isPanelCollapsed}
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
    {/if}
  </div>

  {#if !isPanelCollapsed}
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
              <th style="width: 130px; text-align: center;">PHDI 综合健康度</th>
              <th style="width: 155px;">进度排期健康 (SH)</th>
              <th style="width: 155px;">工程质量 (EQ)</th>
              <th style="width: 155px;">指派协同效率 (CE)</th>
              <th style="width: 155px;">缺陷与稳定性 (SI)</th>
              <th style="text-align: right; width: 220px;">操作</th>
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
                    <div class="metric-val-row font-mono">
                      <span>SH</span>
                      <span>{s.schedule_health_score}%</span>
                    </div>
                    <div class="progress-bar-bg">
                      <span class="progress-bar-fill is-blue" style="width: {s.schedule_health_score}%"><span class="glow-point"></span></span>
                    </div>
                  </div>
                </td>
                <!-- EQ -->
                <td>
                  <div class="metric-progress-wrapper">
                    <div class="metric-val-row font-mono">
                      <span>EQ</span>
                      <span>{s.engineering_quality}%</span>
                    </div>
                    <div class="progress-bar-bg">
                      <span class="progress-bar-fill is-emerald" style="width: {s.engineering_quality}%"><span class="glow-point"></span></span>
                    </div>
                  </div>
                </td>
                <!-- CE -->
                <td>
                  <div class="metric-progress-wrapper">
                    <div class="metric-val-row font-mono">
                      <span>CE</span>
                      <span>{s.collaboration_effic}%</span>
                    </div>
                    <div class="progress-bar-bg">
                      <span class="progress-bar-fill is-amber" style="width: {s.collaboration_effic}%"><span class="glow-point"></span></span>
                    </div>
                  </div>
                </td>
                <!-- SI -->
                <td>
                  <div class="metric-progress-wrapper">
                    <div class="metric-val-row font-mono">
                      <span>SI</span>
                      <span>{s.stability_index}%</span>
                    </div>
                    <div class="progress-bar-bg">
                      <span class="progress-bar-fill is-rose" style="width: {s.stability_index}%"><span class="glow-point"></span></span>
                    </div>
                  </div>
                </td>
                <td style="text-align: right; white-space: nowrap;">
                  <Button size="small" variant="ghost" on:click={() => toggleDiagnostic(s.project_key)}>
                    {expandedProjectKey === s.project_key ? '收起诊断' : '💡 风控诊断'}
                  </Button>
                  <Button size="small" variant="primary" on:click={() => showProjectDetails(s)}>
                    📊 项目大盘
                  </Button>
                </td>
              </tr>
              {#if expandedProjectKey === s.project_key}
                <tr class="diagnostic-expand-row">
                  <td colspan="7">
                    <div class="diagnostic-panel font-mono {getScoreColorClass(s.compound_score)}">
                      <div class="diagnostic-title">
                        <span class="pulse-diagnostic-dot {getScoreColorClass(s.compound_score)}"></span>
                        🧠 大脑风控诊断意见 ({s.project_key})
                      </div>
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
  {/if}

  {#if showDetailsModal && selectedProjectScore}
    <div class="modal-backdrop" use:portal on:click={closeDetailsModal}>
      <div class="modal-content glass-panel" on:click|stopPropagation>
        <div class="modal-header">
          <h3>📊 {getDisplayName(selectedProjectScore)} 项目详情大盘</h3>
          <button class="close-btn" on:click={closeDetailsModal}>&times;</button>
        </div>
        
        <div class="modal-body font-mono">
          <div class="dashboard-columns">
            <!-- 左栏：诊断决策链与大分分析 -->
            <div class="column-left">
              {#if selectedProjectScore.diagnostic}
                <div class="detail-section">
                  <h4>💡 大脑诊断意见</h4>
                  <div class="diagnostic-panel-modal {getScoreColorClass(selectedProjectScore.compound_score)} font-mono">
                    <div class="diag-header-row">
                      <span class="pulse-diagnostic-dot {getScoreColorClass(selectedProjectScore.compound_score)}"></span>
                      <span>DECISION ENGINE OUTPUT</span>
                    </div>
                    <p style="margin: 0; white-space: pre-wrap; line-height: 1.6;">{selectedProjectScore.diagnostic}</p>
                  </div>
                </div>
              {/if}

              <div class="detail-section" style="margin-top: 10px;">
                <h4>📈 PHDI 综合健康度分析</h4>
                <div class="phdi-display">
                  <div class="phdi-score-row" style="display: flex; align-items: center; justify-content: space-between; width: 100%; margin-bottom: 12px; border-bottom: 1px dashed rgba(255, 255, 255, 0.05); padding-bottom: 12px;">
                    <span class="big-score {getScoreColorClass(selectedProjectScore.compound_score)}">
                      {selectedProjectScore.compound_score} <span class="unit">分</span>
                    </span>
                    <span class="health-badge {getScoreColorClass(selectedProjectScore.compound_score)} font-mono">
                      {selectedProjectScore.compound_score >= 85 ? '🟢 极佳交付 / 运行健康' : (selectedProjectScore.compound_score >= 70 ? '🟡 部分维度滞后' : '🔴 严重瓶颈高危')}
                    </span>
                  </div>
                  
                  <div class="formula-breakdown" style="width: 100%;">
                    <h5 style="margin-top: 0;">计算公式权重剖析:</h5>
                    {#if (projectConfigs.find(c => c.project_key.toUpperCase() === selectedProjectScore.project_key.toUpperCase())?.base_score_weight) !== undefined}
                      {@const config = projectConfigs.find(c => c.project_key.toUpperCase() === selectedProjectScore.project_key.toUpperCase())}
                      {@const w = config.base_score_weight}
                      {@const s = config.base_score}
                      {@const metricsScore = selectedProjectScore.schedule_health_score * 0.30 + selectedProjectScore.engineering_quality * 0.25 + selectedProjectScore.collaboration_effic * 0.25 + selectedProjectScore.stability_index * 0.20}
                      
                      <div class="formula-schematic">
                        <div class="schematic-node input-node">
                          <div class="node-label">基础设定</div>
                          <div class="node-value">{s}分 &times; {Math.round(w * 100)}%</div>
                        </div>
                        <div class="schematic-connector">+</div>
                        <div class="schematic-node input-node">
                          <div class="node-label">过程遥测</div>
                          <div class="node-value">{Math.round(metricsScore * 100)/100}分 &times; {Math.round((1 - w) * 100)}%</div>
                        </div>
                        <div class="schematic-connector">=</div>
                        <div class="schematic-node result-node {getScoreColorClass(selectedProjectScore.compound_score)}">
                          <div class="node-label">PHDI 综合</div>
                          <div class="node-value">{selectedProjectScore.compound_score}分</div>
                        </div>
                      </div>
                    {:else}
                      <div class="formula-schematic">
                        <div class="schematic-node input-node">
                          <div class="node-label">默认基础</div>
                          <div class="node-value">60分 &times; 10%</div>
                        </div>
                        <div class="schematic-connector">+</div>
                        <div class="schematic-node input-node">
                          <div class="node-label">度量过程</div>
                          <div class="node-value">{Math.round((selectedProjectScore.compound_score - 6) / 0.9 * 100)/100}分 &times; 90%</div>
                        </div>
                        <div class="schematic-connector">=</div>
                        <div class="schematic-node result-node {getScoreColorClass(selectedProjectScore.compound_score)}">
                          <div class="node-label">PHDI 综合</div>
                          <div class="node-value">{selectedProjectScore.compound_score}分</div>
                        </div>
                      </div>
                    {/if}
                  </div>
                </div>
              </div>
            </div>

            <!-- 右栏：基础属性与遥测细目、行动建议 -->
            <div class="column-right">
              <div class="detail-section">
                <h4>📌 基础属性</h4>
                <div class="detail-grid">
                  <div class="detail-item">
                    <span class="label">项目标识:</span>
                    <span class="value highlight">{selectedProjectScore.project_key}</span>
                  </div>
                  <div class="detail-item">
                    <span class="label">当前所处阶段:</span>
                    <span class="value badge">{
                      (projectConfigs.find(c => c.project_key.toUpperCase() === selectedProjectScore.project_key.toUpperCase())?.project_phase) || '交付'
                    }</span>
                  </div>
                  <div class="detail-item">
                    <span class="label">基准优先级:</span>
                    <span class="value">{
                      (projectConfigs.find(c => c.project_key.toUpperCase() === selectedProjectScore.project_key.toUpperCase())?.base_priority) || 'P1'
                    }</span>
                  </div>
                  <div class="detail-item">
                    <span class="label">快照周期:</span>
                    <span class="value">{selectedProjectScore.snapshot_date || '当前周期'}</span>
                  </div>
                </div>
              </div>

              <div class="detail-section" style="margin-top: 15px;">
                <h4>📊 核心维度明细</h4>
                <div class="metrics-grid-vertical">
                  <!-- SH -->
                  <div class="metric-row-card">
                    <div class="row-header" style="display: flex; justify-content: space-between; align-items: center;">
                      <span class="row-title">进度排期健康 [SH]</span>
                      <div style="display: flex; gap: 8px; align-items: center;">
                        <span class="row-status-tag font-mono {selectedProjectScore.schedule_health_score >= 85 ? 'text-emerald' : (selectedProjectScore.schedule_health_score >= 70 ? 'text-amber' : 'text-rose')}">
                          {selectedProjectScore.schedule_health_score >= 85 ? '🟢 正常' : (selectedProjectScore.schedule_health_score >= 70 ? '🟡 风险' : '🔴 滞后')}
                        </span>
                        <span class="row-score text-blue">{selectedProjectScore.schedule_health_score}%</span>
                      </div>
                    </div>
                    <div class="row-progress">
                      <span class="bg-blue" style="width: {selectedProjectScore.schedule_health_score}%"></span>
                    </div>
                  </div>
                  <!-- EQ -->
                  <div class="metric-row-card">
                    <div class="row-header" style="display: flex; justify-content: space-between; align-items: center;">
                      <span class="row-title">工程质量 [EQ]</span>
                      <div style="display: flex; gap: 8px; align-items: center;">
                        <span class="row-status-tag font-mono {selectedProjectScore.engineering_quality >= 85 ? 'text-emerald' : (selectedProjectScore.engineering_quality >= 70 ? 'text-amber' : 'text-rose')}">
                          {selectedProjectScore.engineering_quality >= 85 ? '🟢 正常' : (selectedProjectScore.engineering_quality >= 70 ? '🟡 待绑' : '🔴 无提交')}
                        </span>
                        <span class="row-score text-emerald">{selectedProjectScore.engineering_quality}%</span>
                      </div>
                    </div>
                    <div class="row-progress">
                      <span class="bg-emerald" style="width: {selectedProjectScore.engineering_quality}%"></span>
                    </div>
                  </div>
                  <!-- CE -->
                  <div class="metric-row-card">
                    <div class="row-header" style="display: flex; justify-content: space-between; align-items: center;">
                      <span class="row-title">指派协同效率 [CE]</span>
                      <div style="display: flex; gap: 8px; align-items: center;">
                        <span class="row-status-tag font-mono {selectedProjectScore.collaboration_effic >= 85 ? 'text-emerald' : (selectedProjectScore.collaboration_effic >= 70 ? 'text-amber' : 'text-rose')}">
                          {selectedProjectScore.collaboration_effic >= 85 ? '🟢 均衡' : (selectedProjectScore.collaboration_effic >= 70 ? '🟡 偏载' : '🔴 严重不均')}
                        </span>
                        <span class="row-score text-amber">{selectedProjectScore.collaboration_effic}%</span>
                      </div>
                    </div>
                    <div class="row-progress">
                      <span class="bg-amber" style="width: {selectedProjectScore.collaboration_effic}%"></span>
                    </div>
                  </div>
                  <!-- SI -->
                  <div class="metric-row-card">
                    <div class="row-header" style="display: flex; justify-content: space-between; align-items: center;">
                      <span class="row-title">缺陷与稳定性 [SI]</span>
                      <div style="display: flex; gap: 8px; align-items: center;">
                        <span class="row-status-tag font-mono {selectedProjectScore.stability_index >= 85 ? 'text-emerald' : (selectedProjectScore.stability_index >= 70 ? 'text-amber' : 'text-rose')}">
                          {selectedProjectScore.stability_index >= 85 ? '🟢 稳定' : (selectedProjectScore.stability_index >= 70 ? '🟡 新增' : '🔴 高危')}
                        </span>
                        <span class="row-score text-rose">{selectedProjectScore.stability_index}%</span>
                      </div>
                    </div>
                    <div class="row-progress">
                      <span class="bg-rose" style="width: {selectedProjectScore.stability_index}%"></span>
                    </div>
                  </div>
                </div>
              </div>

              <!-- 🧠 最强大脑改进建议 -->
              <div class="detail-section" style="margin-top: 15px;">
                <h4>🧠 大脑行动建议</h4>
                <div class="action-recommendations">
                  {#if selectedProjectScore.schedule_health_score < 85}
                    <div class="recommendation-item">
                      <span class="rec-bullet text-rose">⚡</span>
                      <p>【排期优化】进度维度异常，建议前往 <a href="#/" class="rec-link" on:click|preventDefault={() => { closeDetailsModal(); window.location.hash = '#/demands'; }}>需求看板</a> 补全开发任务截止日或重新排期。</p>
                    </div>
                  {/if}
                  {#if selectedProjectScore.engineering_quality < 85}
                    <div class="recommendation-item">
                      <span class="rec-bullet text-amber">⚡</span>
                      <p>【工程合规】工程关联率偏低，请指导开发人员创建包含 <code>Task ID</code> 的 Git 提交流转记录，或开启 <b>离线排期模式</b>。</p>
                    </div>
                  {/if}
                  {#if selectedProjectScore.collaboration_effic < 85}
                    <div class="recommendation-item">
                      <span class="rec-bullet text-blue">⚡</span>
                      <p>【协同负载】团队负荷指数异常，建议重新平衡分工，调配闲置人力（如外部协同）以解开瓶颈。</p>
                    </div>
                  {/if}
                  {#if selectedProjectScore.stability_index < 85}
                    <div class="recommendation-item">
                      <span class="rec-bullet text-rose">⚡</span>
                      <p>【缺陷治理】当前项目遗留缺陷故障时效过长，建议指派测试人员进入以执行缺陷集中流转推进。</p>
                    </div>
                  {/if}
                  {#if selectedProjectScore.schedule_health_score >= 85 && selectedProjectScore.engineering_quality >= 85 && selectedProjectScore.collaboration_effic >= 85 && selectedProjectScore.stability_index >= 85}
                    <div class="recommendation-item success">
                      <span class="rec-bullet text-emerald">✨</span>
                      <p>【完美交付】本项目全部遥测指标表现完美，团队效能极佳，请继续保持当前的敏捷流转节奏！</p>
                    </div>
                  {/if}
                </div>
              </div>

            </div>
          </div>
        </div>
        
        <div class="modal-footer">
          <Button variant="secondary" on:click={closeDetailsModal}>关闭</Button>
        </div>
      </div>
    </div>
  {/if}
</div>

<style>
  /* Header indicator dots */
  .pulse-status-dot {
    display: inline-block;
    width: 8px;
    height: 8px;
    border-radius: 50%;
    margin-right: 6px;
    vertical-align: middle;
  }
  .pulse-status-dot.online {
    background-color: #10b981;
    box-shadow: 0 0 10px #10b981;
    animation: statusPulse 2s infinite ease-in-out;
  }

  .collapse-chevron {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    color: #6366f1;
    margin-left: 8px;
    transition: transform 0.25s cubic-bezier(0.4, 0, 0.2, 1);
  }
  .collapse-chevron.is-collapsed {
    transform: rotate(-90deg);
  }

  /* Modal Styles */
  .modal-backdrop {
    position: fixed;
    top: 0;
    left: 0;
    width: 100vw;
    height: 100vh;
    background: rgba(4, 6, 12, 0.75);
    backdrop-filter: blur(20px);
    -webkit-backdrop-filter: blur(20px);
    z-index: 1000;
    display: flex;
    align-items: center;
    justify-content: center;
    animation: fadeIn 0.2s ease-out;
  }

  .modal-content {
    width: 840px;
    max-width: 95vw;
    max-height: 90vh;
    background: rgba(10, 16, 32, 0.95);
    border: 1px solid rgba(99, 102, 241, 0.35);
    border-radius: 16px;
    padding: 26px;
    display: flex;
    flex-direction: column;
    box-shadow: 0 25px 60px rgba(0, 0, 0, 0.8), inset 0 1px 0 rgba(255, 255, 255, 0.05);
    color: #f1f5f9;
    overflow-y: auto;
    transition: border-color 0.3s;
  }
  .modal-content:hover {
    border-color: rgba(99, 102, 241, 0.5);
  }

  /* 大分健康度标签 */
  .health-badge {
    padding: 3px 10px;
    border-radius: 6px;
    font-size: 0.72rem;
    font-weight: 800;
  }
  .health-badge.score-green {
    background: rgba(16, 185, 129, 0.12);
    color: #34d399;
    border: 1px solid rgba(16, 185, 129, 0.3);
  }
  .health-badge.score-yellow {
    background: rgba(245, 158, 11, 0.1);
    color: #fbbf24;
    border: 1px solid rgba(245, 158, 11, 0.25);
  }
  .health-badge.score-red {
    background: rgba(244, 63, 94, 0.12);
    color: #f43f5e;
    border: 1px solid rgba(244, 63, 94, 0.3);
  }

  /* 核心指标行状态标签 */
  .row-status-tag {
    font-size: 0.65rem;
    font-weight: 800;
    padding: 1px 6px;
    border-radius: 4px;
    background: rgba(255, 255, 255, 0.03);
    border: 1px solid rgba(255, 255, 255, 0.05);
  }
  .row-status-tag.text-emerald {
    color: #34d399;
    background: rgba(16, 185, 129, 0.06);
    border-color: rgba(16, 185, 129, 0.15);
  }
  .row-status-tag.text-amber {
    color: #fbbf24;
    background: rgba(245, 158, 11, 0.05);
    border-color: rgba(245, 158, 11, 0.15);
  }
  .row-status-tag.text-rose {
    color: #f43f5e;
    background: rgba(244, 63, 94, 0.06);
    border-color: rgba(244, 63, 94, 0.15);
  }

  /* 智能行动建议 */
  .action-recommendations {
    display: flex;
    flex-direction: column;
    gap: 10px;
    background: rgba(15, 23, 42, 0.4);
    border: 1px solid rgba(255, 255, 255, 0.05);
    border-radius: 8px;
    padding: 14px;
  }

  .recommendation-item {
    display: flex;
    gap: 8px;
    align-items: flex-start;
    font-size: 0.72rem;
    line-height: 1.4;
  }

  .recommendation-item p {
    margin: 0;
    color: #94a3b8;
  }

  .recommendation-item code {
    background: rgba(255, 255, 255, 0.06);
    padding: 1px 4px;
    border-radius: 4px;
    color: #f1f5f9;
  }

  .rec-bullet {
    font-size: 0.8rem;
    flex-shrink: 0;
  }

  .rec-link {
    color: #6366f1;
    text-decoration: none;
    font-weight: 700;
    border-bottom: 1px dashed rgba(99, 102, 241, 0.4);
    transition: color 0.15s, border-color 0.15s;
  }

  .rec-link:hover {
    color: #a5b4fc;
    border-bottom-color: #a5b4fc;
  }

  .recommendation-item.success p {
    color: #34d399;
  }

  /* Double column dashboard grid */
  .dashboard-columns {
    display: grid;
    grid-template-columns: 1.2fr 1fr;
    gap: 24px;
    align-items: start;
  }

  .column-left, .column-right {
    display: flex;
    flex-direction: column;
    gap: 20px;
  }

  @media (max-width: 768px) {
    .dashboard-columns {
      grid-template-columns: 1fr;
      gap: 20px;
    }
  }

  /* 垂直核心指标明细 */
  .metrics-grid-vertical {
    display: flex;
    flex-direction: column;
    gap: 12px;
  }

  .metric-row-card {
    background: rgba(15, 23, 42, 0.4);
    border: 1px solid rgba(255, 255, 255, 0.05);
    border-radius: 8px;
    padding: 12px 14px;
    display: flex;
    flex-direction: column;
    gap: 8px;
  }

  .metric-row-card .row-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
  }

  .metric-row-card .row-title {
    font-size: 0.72rem;
    color: #94a3b8;
    font-weight: 700;
  }

  .metric-row-card .row-score {
    font-size: 0.85rem;
    font-weight: 800;
  }

  .metric-row-card .row-progress {
    height: 6px;
    background: rgba(255, 255, 255, 0.05);
    border-radius: 3px;
    overflow: hidden;
    position: relative;
  }

  .metric-row-card .row-progress span {
    display: block;
    height: 100%;
    border-radius: 3px;
  }

  .metric-row-card .row-progress span.bg-blue { background: #6366f1; box-shadow: 0 0 8px rgba(99, 102, 241, 0.6); }
  .metric-row-card .row-progress span.bg-emerald { background: #10b981; box-shadow: 0 0 8px rgba(16, 185, 129, 0.6); }
  .metric-row-card .row-progress span.bg-amber { background: #f59e0b; box-shadow: 0 0 8px rgba(245, 158, 11, 0.6); }
  .metric-row-card .row-progress span.bg-rose { background: #f43f5e; box-shadow: 0 0 8px rgba(244, 63, 94, 0.6); }

  .modal-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
    border-bottom: 1px solid rgba(255, 255, 255, 0.08);
    padding-bottom: 16px;
    margin-bottom: 22px;
  }

  .modal-header h3 {
    margin: 0;
    font-size: 1.15rem;
    font-weight: 800;
    background: linear-gradient(135deg, #ffffff 0%, #a5b4fc 100%);
    -webkit-background-clip: text;
    -webkit-text-fill-color: transparent;
  }

  .close-btn {
    background: transparent;
    border: none;
    color: #64748b;
    font-size: 1.5rem;
    cursor: pointer;
    transition: color 0.2s, transform 0.2s;
  }
  .close-btn:hover {
    color: #ffffff;
    transform: scale(1.1);
  }

  .modal-body {
    flex: 1;
    overflow-y: auto;
    display: flex;
    flex-direction: column;
    gap: 24px;
    padding-right: 4px;
  }

  .detail-section h4 {
    margin: 0 0 12px 0;
    font-size: 0.82rem;
    color: #818cf8;
    text-transform: uppercase;
    letter-spacing: 0.08em;
    border-left: 3px solid #6366f1;
    padding-left: 8px;
  }

  .detail-grid {
    display: grid;
    grid-template-columns: repeat(2, 1fr);
    gap: 14px;
    background: rgba(15, 23, 42, 0.4);
    border: 1px solid rgba(255, 255, 255, 0.06);
    border-radius: 8px;
    padding: 16px;
  }

  .detail-item {
    display: flex;
    justify-content: space-between;
    font-size: 0.76rem;
  }

  .detail-item .label {
    color: #64748b;
  }

  .detail-item .value {
    color: #cbd5e1;
    font-weight: 700;
  }

  .detail-item .value.highlight {
    color: #38bdf8;
    text-shadow: 0 0 8px rgba(56, 189, 248, 0.3);
  }

  .detail-item .value.badge {
    background: rgba(99, 102, 241, 0.15);
    border: 1px solid rgba(99, 102, 241, 0.3);
    padding: 1px 8px;
    border-radius: 4px;
    color: #a5b4fc;
  }

  .phdi-display {
    display: flex;
    align-items: center;
    gap: 24px;
    background: rgba(15, 23, 42, 0.4);
    border: 1px solid rgba(255, 255, 255, 0.06);
    border-radius: 8px;
    padding: 20px;
    flex-wrap: wrap;
  }

  .big-score {
    font-size: 2.6rem;
    font-weight: 900;
    display: flex;
    align-items: baseline;
    gap: 4px;
  }
  .big-score.score-green {
    color: #10b981;
    text-shadow: 0 0 15px rgba(16, 185, 129, 0.4);
  }
  .big-score.score-yellow {
    color: #fbbf24;
    text-shadow: 0 0 15px rgba(251, 191, 36, 0.4);
  }
  .big-score.score-red {
    color: #f43f5e;
    text-shadow: 0 0 15px rgba(244, 63, 94, 0.4);
  }

  .big-score .unit {
    font-size: 0.95rem;
    font-weight: 600;
    color: #64748b;
    text-shadow: none;
  }

  .formula-breakdown {
    flex: 1;
    min-width: 280px;
  }

  .formula-breakdown h5 {
    margin: 0 0 10px 0;
    color: #94a3b8;
    font-size: 0.74rem;
    letter-spacing: 0.02em;
  }

  /* Hardware Schematic Formula style */
  .formula-schematic {
    display: flex;
    align-items: center;
    gap: 12px;
    font-size: 0.74rem;
    background: rgba(8, 12, 24, 0.6);
    border: 1px solid rgba(99, 102, 241, 0.15);
    border-radius: 8px;
    padding: 12px 14px;
    width: 100%;
    box-sizing: border-box;
  }

  .schematic-node {
    flex: 1;
    display: flex;
    flex-direction: column;
    align-items: center;
    padding: 6px;
    border-radius: 4px;
    background: rgba(255, 255, 255, 0.02);
    border: 1px dashed rgba(255, 255, 255, 0.08);
  }

  .schematic-node .node-label {
    font-size: 0.62rem;
    color: #64748b;
    margin-bottom: 4px;
    text-transform: uppercase;
  }

  .schematic-node .node-value {
    font-weight: 700;
    color: #e2e8f0;
  }

  .schematic-node.result-node {
    background: rgba(99, 102, 241, 0.08);
    border: 1px solid rgba(99, 102, 241, 0.3);
  }
  .schematic-node.result-node.score-green { color: #34d399; border-color: rgba(16, 185, 129, 0.4); background: rgba(16, 185, 129, 0.06); }
  .schematic-node.result-node.score-yellow { color: #fbbf24; border-color: rgba(251, 191, 36, 0.3); background: rgba(251, 191, 36, 0.05); }
  .schematic-node.result-node.score-red { color: #f43f5e; border-color: rgba(244, 63, 94, 0.4); background: rgba(244, 63, 94, 0.06); }
  .schematic-node.result-node.score-green .node-value { color: #34d399; }
  .schematic-node.result-node.score-yellow .node-value { color: #fbbf24; }
  .schematic-node.result-node.score-red .node-value { color: #f43f5e; }

  .schematic-connector {
    font-size: 0.9rem;
    font-weight: 800;
    color: #475569;
  }

  /* Metrics Grid in modal */
  .metrics-grid {
    display: grid;
    grid-template-columns: repeat(2, 1fr);
    gap: 16px;
  }

  .metric-card {
    background: rgba(15, 23, 42, 0.3);
    border: 1px solid rgba(255, 255, 255, 0.05);
    border-radius: 8px;
    padding: 14px 16px;
    display: flex;
    flex-direction: column;
    gap: 8px;
    transition: transform 0.2s, border-color 0.2s;
  }
  .metric-card:hover {
    transform: translateY(-2px);
    border-color: rgba(99, 102, 241, 0.25);
  }

  .metric-card .m-title {
     font-size: 0.72rem;
     color: #64748b;
     font-weight: 800;
     letter-spacing: 0.02em;
  }

  .m-score-row {
    display: flex;
    justify-content: space-between;
    align-items: center;
    gap: 12px;
  }

  .metric-card .m-score {
     font-size: 1.25rem;
     font-weight: 800;
  }

  .m-mini-bar {
    flex: 1;
    height: 4px;
    background: rgba(255, 255, 255, 0.05);
    border-radius: 2px;
    overflow: hidden;
  }
  .m-mini-bar span {
    display: block;
    height: 100%;
    border-radius: 2px;
  }
  .m-mini-bar span.bg-blue { background: #6366f1; box-shadow: 0 0 6px #6366f1; }
  .m-mini-bar span.bg-emerald { background: #10b981; box-shadow: 0 0 6px #10b981; }
  .m-mini-bar span.bg-amber { background: #f59e0b; box-shadow: 0 0 6px #f59e0b; }
  .m-mini-bar span.bg-rose { background: #f43f5e; box-shadow: 0 0 6px #f43f5e; }

  .text-blue { color: #818cf8; }
  .text-emerald { color: #34d399; }
  .text-amber { color: #fbbf24; }
  .text-rose { color: #f43f5e; }

  .metric-card .m-desc {
    margin: 0;
    font-size: 0.68rem;
    color: #64748b;
    line-height: 1.4;
  }

  .diagnostic-panel-modal {
    border-radius: 8px;
    padding: 16px;
    font-size: 0.74rem;
    border: 1px solid transparent;
  }
  .diag-header-row {
    display: flex;
    align-items: center;
    gap: 6px;
    font-size: 0.62rem;
    font-weight: 800;
    letter-spacing: 0.08em;
    color: #64748b;
    margin-bottom: 10px;
    border-bottom: 1px dashed rgba(255, 255, 255, 0.05);
    padding-bottom: 6px;
  }

  .diagnostic-panel-modal.score-green {
    background: rgba(16, 185, 129, 0.03);
    border-color: rgba(16, 185, 129, 0.15);
    color: #34d399;
  }

  .diagnostic-panel-modal.score-yellow {
    background: rgba(245, 158, 11, 0.03);
    border-color: rgba(245, 158, 11, 0.15);
    color: #fbbf24;
  }

  .diagnostic-panel-modal.score-red {
    background: rgba(244, 63, 94, 0.03);
    border-color: rgba(244, 63, 94, 0.15);
    color: #f43f5e;
  }

  .modal-footer {
    border-top: 1px solid rgba(255, 255, 255, 0.08);
    padding-top: 16px;
    margin-top: 22px;
    display: flex;
    justify-content: flex-end;
  }

  /* Telemetry Panel container */
  .telemetry-panel {
    margin-bottom: 24px;
    box-sizing: border-box;
    background: rgba(10, 16, 32, 0.7);
    border: 1px solid rgba(255, 255, 255, 0.06);
    border-radius: 16px;
    backdrop-filter: blur(16px);
    -webkit-backdrop-filter: blur(16px);
    padding: 24px;
    box-shadow: 0 12px 40px rgba(0, 0, 0, 0.6), inset 0 1px 0 rgba(255, 255, 255, 0.05);
  }

  .flex-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
    gap: 16px;
    flex-wrap: wrap;
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
    background: linear-gradient(135deg, #ffffff 0%, #c7d2fe 100%);
    -webkit-background-clip: text;
    -webkit-text-fill-color: transparent;
    display: flex;
    align-items: center;
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
    background: rgba(8, 12, 24, 0.6);
    border: 1px solid rgba(255, 255, 255, 0.08);
    border-radius: 6px;
    padding: 0 10px 0 32px;
    color: #f1f5f9;
    font-size: 0.78rem;
    outline: none;
    transition: all 0.2s ease;
  }

  .telemetry-search-input:focus {
    border-color: rgba(99, 102, 241, 0.5);
    box-shadow: 0 0 10px rgba(99, 102, 241, 0.2);
  }

  .refresh-btn {
    min-height: 32px;
    background: rgba(99, 102, 241, 0.1);
    border: 1px solid rgba(99, 102, 241, 0.25);
    color: #a5b4fc;
    border-radius: 6px;
    padding: 6px 14px;
    font-size: 0.74rem;
    font-weight: 700;
    cursor: pointer;
    transition: all 0.2s ease;
  }

  .refresh-btn:hover {
    background: rgba(99, 102, 241, 0.2);
    border-color: rgba(99, 102, 241, 0.45);
    color: #ffffff;
    box-shadow: 0 0 10px rgba(99, 102, 241, 0.15);
  }

  .refresh-btn:active {
    transform: scale(0.97);
  }

  .loading-state, .error-state, .empty-state {
    padding: 40px;
    text-align: center;
    border: 1px dashed rgba(255, 255, 255, 0.06);
    border-radius: 12px;
    color: #64748b;
    font-size: 0.82rem;
    background: rgba(8, 12, 24, 0.3);
  }

  .error-state {
    color: #fca5a5;
    background: rgba(127, 29, 29, 0.1);
    border-color: rgba(248, 113, 113, 0.2);
  }

  .table-responsive {
    overflow-x: auto;
    border-radius: 12px;
    border: 1px solid rgba(255, 255, 255, 0.06);
    background: rgba(6, 10, 20, 0.5);
    margin-top: 16px;
  }

  .telemetry-table {
    width: 100%;
    border-collapse: collapse;
    text-align: left;
    font-size: 0.8rem;
  }

  .telemetry-table th {
    background: rgba(8, 12, 24, 0.85);
    color: #64748b;
    font-size: 0.72rem;
    font-weight: 800;
    padding: 14px 16px;
    border-bottom: 1px solid rgba(255, 255, 255, 0.08);
    text-transform: uppercase;
    letter-spacing: 0.06em;
  }

  .telemetry-table td {
    padding: 14px 16px;
    border-bottom: 1px solid rgba(255, 255, 255, 0.04);
    vertical-align: middle;
  }

  .telemetry-table tr:last-child td {
    border-bottom: none;
  }

  .telemetry-table tr:hover td {
    background: rgba(255, 255, 255, 0.015);
  }

  .telemetry-table tr.has-expanded td {
    border-bottom-color: transparent;
    background: rgba(99, 102, 241, 0.02);
  }

  .project-info-cell {
    display: flex;
    align-items: center;
    gap: 12px;
  }

  .project-avatar {
    width: 32px;
    height: 32px;
    border-radius: 8px;
    background: linear-gradient(135deg, rgba(59, 130, 246, 0.25) 0%, rgba(139, 92, 246, 0.25) 100%);
    color: #e2e8f0;
    display: flex;
    align-items: center;
    justify-content: center;
    font-weight: 800;
    font-size: 0.85rem;
    border: 1px solid rgba(255, 255, 255, 0.08);
    box-shadow: inset 0 0 10px rgba(99, 102, 241, 0.15);
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
    font-size: 0.82rem;
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
    font-size: 0.76rem;
    font-weight: 800;
    text-shadow: 0 2px 4px rgba(0, 0, 0, 0.4);
  }

  .score-badge.score-green {
    background: rgba(16, 185, 129, 0.12);
    color: #34d399;
    border: 1px solid rgba(16, 185, 129, 0.3);
    box-shadow: 0 0 8px rgba(16, 185, 129, 0.15);
  }

  .score-badge.score-yellow {
    background: rgba(245, 158, 11, 0.1);
    color: #fbbf24;
    border: 1px solid rgba(245, 158, 11, 0.25);
    box-shadow: 0 0 8px rgba(251, 191, 36, 0.1);
  }

  .score-badge.score-red {
    background: rgba(244, 63, 94, 0.12);
    color: #f43f5e;
    border: 1px solid rgba(244, 63, 94, 0.3);
    box-shadow: 0 0 8px rgba(244, 63, 94, 0.15);
  }

  /* Hardware Style Progress bars */
  .metric-progress-wrapper {
    display: flex;
    flex-direction: column;
    gap: 5px;
  }

  .metric-val-row {
    display: flex;
    justify-content: space-between;
    font-size: 0.65rem;
    color: #64748b;
    font-weight: 700;
    letter-spacing: 0.02em;
  }

  .progress-bar-bg {
    height: 4px;
    background: rgba(255, 255, 255, 0.03);
    border: 1px solid rgba(255, 255, 255, 0.04);
    border-radius: 2px;
    position: relative;
  }

  .progress-bar-fill {
    display: block;
    height: 100%;
    border-radius: 2px;
    position: relative;
  }

  .progress-bar-fill.is-blue { background: #6366f1; box-shadow: 0 0 8px rgba(99, 102, 241, 0.6); }
  .progress-bar-fill.is-emerald { background: #10b981; box-shadow: 0 0 8px rgba(16, 185, 129, 0.6); }
  .progress-bar-fill.is-amber { background: #f59e0b; box-shadow: 0 0 8px rgba(245, 158, 11, 0.6); }
  .progress-bar-fill.is-rose { background: #f43f5e; box-shadow: 0 0 8px rgba(244, 63, 94, 0.6); }

  .glow-point {
    position: absolute;
    right: -2px;
    top: 50%;
    transform: translateY(-50%);
    width: 4px;
    height: 4px;
    border-radius: 50%;
    background: #ffffff;
    box-shadow: 0 0 8px currentColor;
  }

  /* Diagnostic Panel */
  .diagnostic-expand-row td {
    padding: 0 16px 14px 16px !important;
    border-bottom: 1px solid rgba(255, 255, 255, 0.04) !important;
    background: rgba(99, 102, 241, 0.02) !important;
  }

  .diagnostic-panel {
    border-radius: 8px;
    padding: 16px 20px;
    animation: slideDown 0.25s cubic-bezier(0.16, 1, 0.3, 1);
    border: 1px solid transparent;
  }

  .diagnostic-panel.score-green {
    background: rgba(16, 185, 129, 0.02);
    border-color: rgba(16, 185, 129, 0.15);
  }

  .diagnostic-panel.score-yellow {
    background: rgba(245, 158, 11, 0.02);
    border-color: rgba(245, 158, 11, 0.15);
  }

  .diagnostic-panel.score-red {
    background: rgba(244, 63, 94, 0.02);
    border-color: rgba(244, 63, 94, 0.15);
  }

  .diagnostic-title {
    font-size: 0.74rem;
    font-weight: 800;
    margin-bottom: 8px;
    display: flex;
    align-items: center;
  }

  .pulse-diagnostic-dot {
    display: inline-block;
    width: 6px;
    height: 6px;
    border-radius: 50%;
    margin-right: 8px;
    animation: statusPulse 1.5s infinite ease-in-out;
  }
  .pulse-diagnostic-dot.score-green { background: #34d399; box-shadow: 0 0 8px #34d399; }
  .pulse-diagnostic-dot.score-yellow { background: #fbbf24; box-shadow: 0 0 8px #fbbf24; }
  .pulse-diagnostic-dot.score-red { background: #f43f5e; box-shadow: 0 0 8px #f43f5e; }

  .diagnostic-content {
    margin: 0 0 12px 0;
    font-size: 0.74rem;
    line-height: 1.5;
    color: #cbd5e1;
  }

  .diagnostic-footer {
    font-size: 0.65rem;
    color: #475569;
    border-top: 1px dashed rgba(255, 255, 255, 0.05);
    padding-top: 8px;
  }

  /* Keyframe Animations */
  @keyframes slideDown {
    from { opacity: 0; transform: translateY(-6px); }
    to { opacity: 1; transform: translateY(0); }
  }

  @keyframes fadeIn {
    from { opacity: 0; }
    to { opacity: 1; }
  }

  @keyframes statusPulse {
    0%, 100% { opacity: 0.6; transform: scale(1); }
    50% { opacity: 1; transform: scale(1.2); }
  }
</style>
