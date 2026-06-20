<script lang="ts">
  import { onMount } from 'svelte';

  let activePeriod = 'week'; // 'week' | 'month' | 'year'
  let loading = false;
  let errorMsg = '';

  interface KPISummary {
    total_completed: number;
    tasks_completed: number;
    bugs_completed: number;
    demands_completed: number;
  }

  interface UserKPI {
    username: string;
    name: string;
    avatar: string;
    department: string;
    tasks_completed: number;
    bugs_completed: number;
    demands_completed: number;
    total_completed: number;
  }

  interface DeptKPI {
    department: string;
    tasks_completed: number;
    bugs_completed: number;
    demands_completed: number;
    total_completed: number;
  }

  let summary: KPISummary = {
    total_completed: 0,
    tasks_completed: 0,
    bugs_completed: 0,
    demands_completed: 0,
  };

  let userKPIList: UserKPI[] = [];
  let deptKPIList: DeptKPI[] = [];

  $: maxUserTotal = userKPIList.length > 0 ? Math.max(...userKPIList.map(u => u.total_completed), 1) : 1;
  $: maxDeptTotal = deptKPIList.length > 0 ? Math.max(...deptKPIList.map(d => d.total_completed), 1) : 1;

  async function fetchKPIData() {
    loading = true;
    errorMsg = '';
    const token = localStorage.getItem('jwt_token');

    try {
      const res = await fetch(`/api/kpi/performance?period=${activePeriod}`, {
        headers: {
          'Authorization': `Bearer ${token}`
        }
      });

      if (!res.ok) {
        if (res.status === 403) {
          throw new Error('权限不足，只有管理员组有权查看 KPI 绩效看板');
        }
        throw new Error(`加载绩效数据失败: ${res.statusText}`);
      }

      const data = await res.json();
      summary = data.summary || { total_completed: 0, tasks_completed: 0, bugs_completed: 0, demands_completed: 0 };
      userKPIList = data.user_kpi || [];
      deptKPIList = data.department_kpi || [];
    } catch (err: any) {
      errorMsg = err.message || '获取绩效数据请求失败';
      console.error('KPI Fetch Error:', err);
    } finally {
      loading = false;
    }
  }

  function changePeriod(p: string) {
    activePeriod = p;
    fetchKPIData();
  }

  onMount(() => {
    fetchKPIData();
  });
</script>

<div class="kpi-dashboard font-sans">
  <div class="kpi-header">
    <div>
      <span class="eyebrow">TEAM PERFORMANCE METRICS</span>
      <h2>📈 KPI 研发绩效大盘</h2>
    </div>

    <!-- Period Selector -->
    <div class="period-selector font-mono">
      <button class="period-btn {activePeriod === 'week' ? 'active' : ''}" on:click={() => changePeriod('week')}>
        近 7 天
      </button>
      <button class="period-btn {activePeriod === 'month' ? 'active' : ''}" on:click={() => changePeriod('month')}>
        近 30 天
      </button>
      <button class="period-btn {activePeriod === 'year' ? 'active' : ''}" on:click={() => changePeriod('year')}>
        近一年
      </button>
    </div>
  </div>

  {#if loading}
    <div class="state-msg">正在实时统计团队绩效遥测指标...</div>
  {:else if errorMsg}
    <div class="state-msg error-msg font-mono">❌ {errorMsg}</div>
  {:else}
    <!-- Summary Cards -->
    <div class="summary-cards">
      <div class="summary-card total-glow">
        <span class="card-icon">🏆</span>
        <div class="card-info">
          <span class="card-label">总计已交付</span>
          <span class="card-val text-indigo font-mono">{summary.total_completed}</span>
        </div>
      </div>
      <div class="summary-card task-glow">
        <span class="card-icon">🚀</span>
        <div class="card-info">
          <span class="card-label">需求已交付</span>
          <span class="card-val text-blue font-mono">{summary.tasks_completed}</span>
        </div>
      </div>
      <div class="summary-card demand-glow">
        <span class="card-icon">📋</span>
        <div class="card-info">
          <span class="card-label">大需求已交付</span>
          <span class="card-val text-purple font-mono">{summary.demands_completed}</span>
        </div>
      </div>
      <div class="summary-card bug-glow">
        <span class="card-icon">🪲</span>
        <div class="card-info">
          <span class="card-label">Bug已解决</span>
          <span class="card-val text-rose font-mono">{summary.bugs_completed}</span>
        </div>
      </div>
    </div>

    <!-- Grid: Users Ranking & Department Ranking -->
    <div class="kpi-grid">
      <!-- 1. User Ranking Board -->
      <div class="kpi-panel glass-panel">
        <div class="panel-header">
          <h3>👤 研发成员个人效能排行</h3>
          <span class="panel-subtitle">按周期内已交付总量统计</span>
        </div>

        <div class="ranking-list">
          {#if userKPIList.length === 0}
            <div class="empty-msg font-mono">该周期内暂无已交付的任务或 Bug</div>
          {:else}
            {#each userKPIList as item, index}
              <div class="ranking-item">
                <div class="rank-badge font-mono" class:rank-1={index === 0} class:rank-2={index === 1} class:rank-3={index === 2}>
                  {index + 1}
                </div>
                
                <div class="user-avatar-wrapper">
                  {#if item.avatar}
                    <img src={item.avatar} alt={item.name} class="user-avatar" />
                  {:else}
                    <div class="avatar-placeholder font-mono">{item.name.slice(0, 1).toUpperCase()}</div>
                  {/if}
                </div>

                <div class="item-meta">
                  <div class="meta-row">
                    <span class="name">{item.name}</span>
                    <span class="dept-tag font-mono">{item.department || '未分配'}</span>
                  </div>

                  <!-- Details and Progress bar -->
                  <div class="progress-container">
                    <div class="kpi-progress-bar">
                      <div class="kpi-fill" style="width: {(item.total_completed / maxUserTotal) * 100}%"></div>
                    </div>
                    
                    <div class="breakdown font-mono">
                      <span>需求: {item.tasks_completed}</span>
                      <span>大需求: {item.demands_completed}</span>
                      <span>故障: {item.bugs_completed}</span>
                      <strong class="total-score text-indigo">总计: {item.total_completed}</strong>
                    </div>
                  </div>
                </div>
              </div>
            {/each}
          {/if}
        </div>
      </div>

      <!-- 2. Department Ranking Board -->
      <div class="kpi-panel glass-panel">
        <div class="panel-header">
          <h3>🏢 部门完成指标汇总</h3>
          <span class="panel-subtitle">部门整体研发吞吐量度量</span>
        </div>

        <div class="ranking-list">
          {#if deptKPIList.length === 0}
            <div class="empty-msg font-mono">该周期内暂无已交付的部门数据</div>
          {:else}
            {#each deptKPIList as item, index}
              <div class="ranking-item">
                <div class="rank-badge font-mono" class:rank-1={index === 0} class:rank-2={index === 1} class:rank-3={index === 2}>
                  {index + 1}
                </div>

                <div class="item-meta">
                  <div class="meta-row">
                    <span class="dept-title font-semibold">{item.department}</span>
                  </div>

                  <!-- Progress bar -->
                  <div class="progress-container">
                    <div class="kpi-progress-bar dept-bar">
                      <div class="kpi-fill dept-fill" style="width: {(item.total_completed / maxDeptTotal) * 100}%"></div>
                    </div>

                    <div class="breakdown font-mono">
                      <span>需求: {item.tasks_completed}</span>
                      <span>大需求: {item.demands_completed}</span>
                      <span>故障: {item.bugs_completed}</span>
                      <strong class="total-score text-teal">总计: {item.total_completed}</strong>
                    </div>
                  </div>
                </div>
              </div>
            {/each}
          {/if}
        </div>
      </div>
    </div>
  {/if}
</div>

<style>
  .kpi-dashboard {
    display: flex;
    flex-direction: column;
    gap: 24px;
    margin-top: 10px;
    color: #e2e8f0;
  }

  .kpi-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
    flex-wrap: wrap;
    gap: 16px;
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

  .kpi-header h2 {
    margin: 0;
    font-size: 1.5rem;
    font-weight: 800;
    background: linear-gradient(135deg, #ffffff 0%, #cbd5e1 100%);
    -webkit-background-clip: text;
    -webkit-text-fill-color: transparent;
  }

  /* Period selector styling */
  .period-selector {
    display: inline-flex;
    background: rgba(15, 23, 42, 0.6);
    border: 1px solid rgba(129, 140, 248, 0.2);
    border-radius: 8px;
    padding: 3px;
    gap: 2px;
  }

  .period-btn {
    background: transparent;
    border: none;
    color: #94a3b8;
    padding: 6px 16px;
    font-size: 0.75rem;
    font-weight: 600;
    cursor: pointer;
    border-radius: 6px;
    transition: all 0.2s cubic-bezier(0.16, 1, 0.3, 1);
  }

  .period-btn:hover {
    color: #ffffff;
    background: rgba(99, 102, 241, 0.1);
  }

  .period-btn.active {
    background: #6366f1;
    color: #ffffff;
    box-shadow: 0 4px 12px rgba(99, 102, 241, 0.3);
  }

  /* Summary Cards */
  .summary-cards {
    display: grid;
    grid-template-columns: repeat(auto-fit, minmax(220px, 1fr));
    gap: 20px;
  }

  .summary-card {
    background: rgba(10, 15, 30, 0.7);
    border: 1px solid rgba(51, 65, 85, 0.35);
    border-radius: 16px;
    backdrop-filter: blur(12px);
    -webkit-backdrop-filter: blur(12px);
    padding: 20px;
    display: flex;
    align-items: center;
    gap: 16px;
    transition: all 0.3s;
    box-shadow: 0 8px 32px 0 rgba(0, 0, 0, 0.4);
  }

  .summary-card:hover {
    transform: translateY(-2px);
  }

  .total-glow:hover { border-color: rgba(99, 102, 241, 0.4); box-shadow: 0 8px 24px rgba(99, 102, 241, 0.15); }
  .task-glow:hover { border-color: rgba(59, 130, 246, 0.4); box-shadow: 0 8px 24px rgba(59, 130, 246, 0.15); }
  .demand-glow:hover { border-color: rgba(168, 85, 247, 0.4); box-shadow: 0 8px 24px rgba(168, 85, 247, 0.15); }
  .bug-glow:hover { border-color: rgba(244, 63, 94, 0.4); box-shadow: 0 8px 24px rgba(244, 63, 94, 0.15); }

  .card-icon {
    font-size: 2rem;
  }

  .card-info {
    display: flex;
    flex-direction: column;
    gap: 4px;
  }

  .card-label {
    font-size: 0.75rem;
    color: #64748b;
    font-weight: 700;
  }

  .card-val {
    font-size: 1.8rem;
    font-weight: 800;
    line-height: 1;
  }

  .text-indigo { color: #818cf8; }
  .text-blue { color: #60a5fa; }
  .text-purple { color: #c084fc; }
  .text-rose { color: #f43f5e; }
  .text-teal { color: #2dd4bf; }

  /* KPI Grid */
  .kpi-grid {
    display: grid;
    grid-template-columns: repeat(auto-fit, minmax(450px, 1fr));
    gap: 24px;
  }

  .glass-panel {
    background: rgba(10, 15, 30, 0.7);
    border: 1px solid rgba(51, 65, 85, 0.35);
    border-radius: 16px;
    backdrop-filter: blur(12px);
    -webkit-backdrop-filter: blur(12px);
    padding: 24px;
    box-shadow: 0 8px 32px 0 rgba(0, 0, 0, 0.5);
  }

  .kpi-panel {
    display: flex;
    flex-direction: column;
    gap: 20px;
  }

  .panel-header {
    border-bottom: 1px solid rgba(51, 65, 85, 0.2);
    padding-bottom: 12px;
  }

  .panel-header h3 {
    margin: 0 0 4px 0;
    font-size: 1.1rem;
    font-weight: 700;
    color: #f1f5f9;
  }

  .panel-subtitle {
    font-size: 0.7rem;
    color: #64748b;
  }

  .ranking-list {
    display: flex;
    flex-direction: column;
    gap: 16px;
    max-height: 480px;
    overflow-y: auto;
    padding-right: 6px;
  }

  .ranking-list::-webkit-scrollbar {
    width: 4px;
  }
  .ranking-list::-webkit-scrollbar-thumb {
    background: rgba(99, 102, 241, 0.25);
    border-radius: 2px;
  }

  .ranking-item {
    display: flex;
    align-items: center;
    gap: 16px;
    background: rgba(30, 41, 59, 0.2);
    border: 1px solid rgba(51, 65, 85, 0.2);
    border-radius: 12px;
    padding: 12px 16px;
    transition: all 0.25s cubic-bezier(0.16, 1, 0.3, 1);
  }

  .ranking-item:hover {
    background: rgba(51, 65, 85, 0.25);
    border-color: rgba(99, 102, 241, 0.25);
    transform: translateX(4px);
  }

  .rank-badge {
    width: 24px;
    height: 24px;
    border-radius: 50%;
    background: rgba(51, 65, 85, 0.5);
    display: flex;
    align-items: center;
    justify-content: center;
    font-size: 0.75rem;
    font-weight: 700;
    color: #94a3b8;
    flex-shrink: 0;
  }

  .rank-1 { background: #eab308; color: #020617; box-shadow: 0 0 10px rgba(234, 179, 8, 0.4); }
  .rank-2 { background: #94a3b8; color: #020617; }
  .rank-3 { background: #b45309; color: #ffffff; }

  .user-avatar-wrapper {
    flex-shrink: 0;
  }

  .user-avatar {
    width: 38px;
    height: 38px;
    border-radius: 50%;
    border: 1px solid rgba(99, 102, 241, 0.3);
    object-fit: cover;
  }

  .avatar-placeholder {
    width: 38px;
    height: 38px;
    border-radius: 50%;
    background: linear-gradient(135deg, #3b82f6 0%, #8b5cf6 100%);
    color: white;
    display: flex;
    align-items: center;
    justify-content: center;
    font-weight: 700;
    font-size: 1rem;
    border: 1px solid rgba(255, 255, 255, 0.2);
  }

  .item-meta {
    flex-grow: 1;
    display: flex;
    flex-direction: column;
    gap: 8px;
  }

  .meta-row {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 8px;
  }

  .name {
    font-size: 0.85rem;
    font-weight: 600;
    color: #f8fafc;
  }

  .dept-tag {
    font-size: 0.65rem;
    background: rgba(99, 102, 241, 0.15);
    color: #818cf8;
    padding: 2px 8px;
    border-radius: 4px;
    border: 1px solid rgba(99, 102, 241, 0.2);
  }

  .dept-title {
    font-size: 0.85rem;
    color: #f1f5f9;
  }

  .progress-container {
    display: flex;
    flex-direction: column;
    gap: 6px;
  }

  .kpi-progress-bar {
    height: 6px;
    background: rgba(15, 23, 42, 0.6);
    border-radius: 999px;
    overflow: hidden;
  }

  .kpi-fill {
    height: 100%;
    background: linear-gradient(90deg, #6366f1 0%, #a855f7 100%);
    border-radius: 999px;
    transition: width 0.6s cubic-bezier(0.16, 1, 0.3, 1);
  }

  .dept-fill {
    background: linear-gradient(90deg, #14b8a6 0%, #06b6d4 100%);
  }

  .breakdown {
    display: flex;
    gap: 12px;
    font-size: 0.65rem;
    color: #64748b;
    flex-wrap: wrap;
    align-items: center;
  }

  .total-score {
    margin-left: auto;
    font-size: 0.75rem;
  }

  .state-msg {
    padding: 80px 0;
    text-align: center;
    color: #64748b;
    font-size: 0.85rem;
  }

  .empty-msg {
    padding: 40px 0;
    text-align: center;
    color: #475569;
    font-size: 0.75rem;
  }

  .error-msg {
    color: #f87171;
  }
</style>
