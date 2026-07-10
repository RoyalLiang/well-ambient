<script lang="ts">
  import { onMount } from 'svelte';
  import type {
    AdminInspectorRecord,
    AdminMetric,
    AdminTableColumn,
    AdminTableRow,
    AdminTone
  } from '../lib/admin-console/contract';
  import { ADMIN_TONE_CLASS } from '../lib/admin-console/contract';

  type ReportType = 'daily' | 'weekly';

  let activePeriod = 'week'; // 'day' | 'week' | 'month' | 'year'
  let reportType: ReportType = 'weekly';
  let selectedReportUser = '';
  let loading = false;
  let hasLoadedKPI = false;
  let reportLoading = false;
  let errorMsg = '';
  let reportErrorMsg = '';
  let showReportUserDropdown = false;
  let reportUserSelectEl: HTMLElement;
  let selectedMemberId = '';

  const reportModes: Array<{ key: ReportType; period: string; label: string; window: string; helper: string }> = [
    { key: 'daily', period: 'day', label: '日报', window: '近 24 小时', helper: '适合每日站会与当天风险复盘' },
    { key: 'weekly', period: 'week', label: '周报', window: '近 7 天', helper: '适合周会总结、绩效评估与资源调度' }
  ];

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
    overdue_completed: number;
    active_overdue: number;
    avg_cycle_days: number;
    review_count: number;
    mr_count: number;
    risk_notes: string[];
  }

  interface MemberScorecard extends UserKPI {
    kpi_score: number;
    score_label: string;
    score_tone: 'excellent' | 'good' | 'watch' | 'risk';
    delivery_score: number;
    evidence_score: number;
    flow_score: number;
    risk_health_score: number;
    risk_penalty: number;
    total_risk: number;
  }

  interface MemberAdminRow extends AdminTableRow {
    source: MemberScorecard;
    rank: number;
  }

  interface KPIInsight {
    label: string;
    value: string;
    helper: string;
    tone: 'strong' | 'good' | 'watch' | 'risk';
  }

  interface KPIDistributionItem {
    label: string;
    value: number;
    percent: number;
    tone: 'task' | 'demand' | 'bug' | 'excellent' | 'good' | 'watch' | 'risk';
  }

  interface DeptKPI {
    department: string;
    tasks_completed: number;
    bugs_completed: number;
    demands_completed: number;
    total_completed: number;
  }

  interface ReportOverview {
    title: string;
    period_label: string;
    summary: string;
    total_completed: number;
    active_overdue: number;
    overdue_completed: number;
    mr_count: number;
    personal_count: number;
    department_count: number;
  }

  interface ReportPersonalSection {
    user: UserKPI;
    highlights: string[];
    risk_notes: string[];
    evidence_ids: string[];
  }

  interface ReportDepartmentSection {
    department: string;
    total_completed: number;
    active_overdue: number;
    overdue_completed: number;
    review_count: number;
    mr_count: number;
    highlights: string[];
    evidence_ids: string[];
  }

  interface ReportRisk {
    level: string;
    owner: string;
    department: string;
    title: string;
    detail: string;
    evidence_ids: string[];
  }

  interface MeetingFocus {
    topic: string;
    owner: string;
    department: string;
    reason: string;
    evidence_ids: string[];
  }

  interface ReportEvidence {
    id: string;
    task_id: string;
    title: string;
    repo: string;
    assignee: string;
    department: string;
    status: string;
    issue_type: string;
    due_date?: string;
    completed_at?: string;
    last_update: string;
    mr_url?: string;
    task_group_id?: string;
  }

  interface KPIReportPreview {
    period: string;
    type: string;
    user?: string;
    generated_at: string;
    overview: ReportOverview;
    personal_sections: ReportPersonalSection[];
    department_sections: ReportDepartmentSection[];
    risks: ReportRisk[];
    meeting_focus: MeetingFocus[];
    evidence: ReportEvidence[];
  }

  const emptySummary: KPISummary = {
    total_completed: 0,
    tasks_completed: 0,
    bugs_completed: 0,
    demands_completed: 0,
  };

  let summary: KPISummary = { ...emptySummary };
  let userKPIList: UserKPI[] = [];
  let deptKPIList: DeptKPI[] = [];
  let reportPreview: KPIReportPreview | null = null;

  const memberColumns = [
    { key: 'member', label: '成员', width: '220px' },
    { key: 'score', label: 'KPI 分', width: '120px', align: 'right' },
    { key: 'delivery', label: '交付事实', width: '210px' },
    { key: 'flow', label: '证据链', width: '190px' },
    { key: 'risk', label: '风险', width: '140px' },
    { key: 'breakdown', label: '评分拆解', width: '260px' }
  ] satisfies AdminTableColumn[];

  $: maxUserTotal = userKPIList.length > 0 ? Math.max(...userKPIList.map(u => u.total_completed), 1) : 1;
  $: maxDeptTotal = deptKPIList.length > 0 ? Math.max(...deptKPIList.map(d => d.total_completed), 1) : 1;
  $: maxMRCount = userKPIList.length > 0 ? Math.max(...userKPIList.map(u => u.mr_count || 0), 1) : 1;
  $: totalActiveOverdue = userKPIList.reduce((sum, item) => sum + (item.active_overdue || 0), 0);
  $: totalOverdueCompleted = userKPIList.reduce((sum, item) => sum + (item.overdue_completed || 0), 0);
  $: totalMRCount = userKPIList.reduce((sum, item) => sum + (item.mr_count || 0), 0);
  $: totalReviewCount = userKPIList.reduce((sum, item) => sum + (item.review_count || 0), 0);
  $: memberScorecards = userKPIList
    .map(item => buildMemberScorecard(item, maxUserTotal, maxMRCount))
    .sort((a, b) => {
      if (b.kpi_score !== a.kpi_score) return b.kpi_score - a.kpi_score;
      if (b.total_completed !== a.total_completed) return b.total_completed - a.total_completed;
      return a.name.localeCompare(b.name);
    });
  $: averageMemberScore = memberScorecards.length
    ? Math.round(memberScorecards.reduce((sum, item) => sum + item.kpi_score, 0) / memberScorecards.length)
    : 0;
  $: topScorecard = memberScorecards[0] || null;
  $: lowestScorecard = memberScorecards[memberScorecards.length - 1] || null;
  $: riskMemberCount = memberScorecards.filter(item => item.total_risk > 0).length;
  $: totalRiskPoints = memberScorecards.reduce((sum, item) => sum + item.total_risk, 0);
  $: averageCycleDays = buildAverageCycleDays(memberScorecards);
  $: deliveryConcentration = topScorecard && summary.total_completed > 0
    ? percent(topScorecard.total_completed, summary.total_completed)
    : 0;
  $: evidenceCoverage = summary.total_completed > 0 ? Math.min(100, percent(totalMRCount, summary.total_completed)) : 0;
  $: bugShare = summary.total_completed > 0 ? percent(summary.bugs_completed, summary.total_completed) : 0;
  $: scoreSpread = topScorecard && lowestScorecard ? topScorecard.kpi_score - lowestScorecard.kpi_score : 0;
  $: reviewPressure = memberScorecards.length > 0 ? roundOneDecimal(totalReviewCount / memberScorecards.length) : 0;
  $: riskLoad = memberScorecards.length > 0 ? roundOneDecimal(totalRiskPoints / memberScorecards.length) : 0;
  $: kpiDepthInsights = buildKPIDepthInsights(
    topScorecard,
    lowestScorecard,
    deliveryConcentration,
    evidenceCoverage,
    totalMRCount,
    summary,
    riskMemberCount,
    totalRiskPoints,
    riskLoad,
    reviewPressure,
    totalReviewCount,
    bugShare,
    scoreSpread
  );
  $: issueDistribution = buildIssueDistribution(summary);
  $: scoreDistribution = buildScoreDistribution(memberScorecards);
  $: topDepartment = deptKPIList[0] || null;
  $: departmentBreadthLabel = deptKPIList.length > 0
    ? `${deptKPIList.length} 个部门 / TOP ${topDepartment?.department || '-'}`
    : '暂无部门数据';
  $: selectedReportMode = reportModes.find(mode => mode.key === reportType) || reportModes[1];
  $: visibleReportEvidence = reportPreview?.evidence?.slice(0, 8) || [];
  $: focusedPersonalSection = reportPreview?.personal_sections?.[0] || null;
  $: reportUserLabel = selectedReportUser || '团队视图';
  $: kpiAdminMetrics = buildKPIAdminMetrics(
    summary,
    averageMemberScore,
    memberScorecards.length,
    evidenceCoverage,
    totalMRCount,
    totalRiskPoints,
    totalActiveOverdue,
    totalOverdueCompleted
  );
  $: kpiAdminSegments = buildKPIAdminSegments(
    summary,
    issueDistribution,
    bugShare,
    totalReviewCount,
    reviewPressure,
    averageCycleDays,
    departmentBreadthLabel
  );
  $: memberTableRows = memberScorecards.map((item, index) => buildMemberTableRow(item, index));
  $: selectedMemberKey = selectedMemberId || memberTableRows[0]?.id || '';
  $: selectedMemberRow = memberTableRows.find(row => row.id === selectedMemberKey) || memberTableRows[0] || null;
  $: selectedMember = selectedMemberRow?.source || null;
  $: memberInspectorRecord = buildMemberInspectorRecord(selectedMember);

  function authHeaders() {
    const token = localStorage.getItem('jwt_token');
    return {
      'Authorization': `Bearer ${token || ''}`
    };
  }

  async function fetchKPIData() {
    loading = true;
    errorMsg = '';

    try {
      const res = await fetch(`/api/kpi/performance?period=${activePeriod}`, {
        headers: authHeaders()
      });

      if (!res.ok) {
        if (res.status === 403) {
          throw new Error('权限不足，只有管理员组有权查看 KPI 绩效看板');
        }
        throw new Error(`加载绩效数据失败: ${res.statusText}`);
      }

      const data = await res.json();
      summary = data.summary || { ...emptySummary };
      userKPIList = data.user_kpi || [];
      deptKPIList = data.department_kpi || [];
      await fetchReportPreview();
      hasLoadedKPI = true;
    } catch (err: any) {
      errorMsg = err.message || '获取绩效数据请求失败';
      console.error('KPI Fetch Error:', err);
    } finally {
      loading = false;
    }
  }

  async function fetchReportPreview() {
    reportLoading = true;
    reportErrorMsg = '';

    try {
      const params = new URLSearchParams({
        period: activePeriod,
        type: reportType
      });
      if (selectedReportUser) {
        params.set('user', selectedReportUser);
      }

      const res = await fetch(`/api/kpi/report-preview?${params.toString()}`, {
        headers: authHeaders()
      });

      if (!res.ok) {
        if (res.status === 403) {
          throw new Error('权限不足，无法生成 KPI 报告预览');
        }
        throw new Error(`加载报告预览失败: ${res.statusText}`);
      }

      reportPreview = await res.json();
    } catch (err: any) {
      reportErrorMsg = err.message || '获取报告预览请求失败';
      console.error('KPI Report Preview Fetch Error:', err);
    } finally {
      reportLoading = false;
    }
  }

  function changeReportType(t: ReportType) {
    if (loading || reportType === t) return;
    reportType = t;
    activePeriod = reportModes.find(mode => mode.key === t)?.period || 'week';
    fetchKPIData();
  }

  function changeReportUser(user: string) {
    selectedReportUser = user;
    showReportUserDropdown = false;
    fetchReportPreview();
  }

  function toggleReportUserDropdown() {
    showReportUserDropdown = !showReportUserDropdown;
  }

  function handleDocumentClick(e: MouseEvent) {
    const target = e.target as HTMLElement;
    if (showReportUserDropdown && reportUserSelectEl && !reportUserSelectEl.contains(target)) {
      showReportUserDropdown = false;
    }
  }

  function formatMetric(value: number | undefined, digits = 0) {
    const safeValue = value || 0;
    return digits > 0 ? safeValue.toFixed(digits) : String(safeValue);
  }

  function toneClass(tone?: AdminTone) {
    return ADMIN_TONE_CLASS[tone || 'neutral'];
  }

  function scoreToAdminTone(score: number): AdminTone {
    if (score >= 85) return 'success';
    if (score >= 70) return 'info';
    if (score >= 55) return 'warning';
    return 'danger';
  }

  function scoreToneToAdminTone(tone: MemberScorecard['score_tone']): AdminTone {
    if (tone === 'excellent') return 'success';
    if (tone === 'good') return 'info';
    if (tone === 'watch') return 'warning';
    return 'danger';
  }

  function memberId(item: UserKPI) {
    return item.username || item.name || 'unknown-member';
  }

  function riskNotesFor(item: UserKPI | null | undefined) {
    return Array.isArray(item?.risk_notes) ? item.risk_notes : [];
  }

  function buildKPIAdminMetrics(
    currentSummary: KPISummary,
    currentAverageScore: number,
    memberCount: number,
    currentEvidenceCoverage: number,
    currentMRCount: number,
    currentRiskPoints: number,
    currentActiveOverdue: number,
    currentOverdueCompleted: number
  ): AdminMetric[] {
    return [
      {
        label: '本周期交付',
        value: currentSummary.total_completed,
        helper: `任务 ${currentSummary.tasks_completed} / 需求 ${currentSummary.demands_completed} / Bug ${currentSummary.bugs_completed}`,
        tone: 'info'
      },
      {
        label: '团队均分',
        value: currentAverageScore || '-',
        helper: `${memberCount} 名成员纳入可解释评分`,
        tone: memberCount > 0 ? scoreToAdminTone(currentAverageScore) : 'neutral'
      },
      {
        label: '证据覆盖率',
        value: `${currentEvidenceCoverage}%`,
        helper: `MR ${currentMRCount} / 交付 ${currentSummary.total_completed}`,
        tone: currentEvidenceCoverage >= 80 ? 'success' : currentEvidenceCoverage >= 55 ? 'warning' : 'danger'
      },
      {
        label: '风险负荷',
        value: currentRiskPoints,
        helper: `当前超期 ${currentActiveOverdue} / 延期完成 ${currentOverdueCompleted}`,
        tone: currentRiskPoints > 0 ? 'warning' : 'success'
      }
    ];
  }

  function buildKPIAdminSegments(
    currentSummary: KPISummary,
    currentIssueDistribution: KPIDistributionItem[],
    currentBugShare: number,
    currentReviewCount: number,
    currentReviewPressure: number,
    currentAverageCycleDays: number,
    currentDepartmentBreadthLabel: string
  ): AdminMetric[] {
    return [
      {
        label: '任务完成',
        value: currentSummary.tasks_completed,
        helper: `${currentIssueDistribution[0]?.percent || 0}%`,
        tone: 'info'
      },
      {
        label: '需求交付',
        value: currentSummary.demands_completed,
        helper: `${currentIssueDistribution[1]?.percent || 0}%`,
        tone: 'success'
      },
      {
        label: 'Bug 关闭',
        value: currentSummary.bugs_completed,
        helper: `${currentIssueDistribution[2]?.percent || 0}%`,
        tone: currentBugShare > 25 ? 'warning' : 'neutral'
      },
      {
        label: '评审队列',
        value: currentReviewCount,
        helper: `人均 ${currentReviewPressure}`,
        tone: currentReviewPressure > 1 ? 'warning' : 'neutral'
      },
      {
        label: '平均周期',
        value: `${formatMetric(currentAverageCycleDays, 1)} 天`,
        helper: currentDepartmentBreadthLabel,
        tone: currentAverageCycleDays > 5 ? 'warning' : 'neutral'
      }
    ];
  }

  function buildMemberTableRow(item: MemberScorecard, index: number): MemberAdminRow {
    return {
      id: memberId(item),
      title: item.name,
      status: item.score_label,
      tone: scoreToneToAdminTone(item.score_tone),
      owner: item.name,
      priority: item.department || '未分配',
      risk: item.total_risk > 0 ? '需关注' : '稳定',
      source: item,
      rank: index + 1,
      cells: {
        member: item.name,
        department: item.department || '未分配',
        score: item.kpi_score,
        scoreLabel: item.score_label,
        delivery: item.total_completed,
        tasks: item.tasks_completed,
        demands: item.demands_completed,
        bugs: item.bugs_completed,
        cycle: formatMetric(item.avg_cycle_days, 1),
        mr: item.mr_count || 0,
        review: item.review_count || 0,
        risk: item.total_risk,
        activeOverdue: item.active_overdue || 0,
        overdueCompleted: item.overdue_completed || 0,
        breakdown: `产出 ${item.delivery_score} / 证据 ${item.evidence_score} / 效率 ${item.flow_score} / 风险 ${item.risk_health_score}`
      }
    };
  }

  function buildMemberInspectorRecord(item: MemberScorecard | null): AdminInspectorRecord | null {
    if (!item) return null;
    const notes = riskNotesFor(item);
    return {
      id: memberId(item),
      title: item.name,
      status: item.score_label,
      tone: scoreToneToAdminTone(item.score_tone),
      facts: [
        { label: '部门', value: item.department || '未分配' },
        { label: 'KPI 分', value: item.kpi_score },
        { label: '本周期交付', value: item.total_completed },
        { label: '平均周期', value: `${formatMetric(item.avg_cycle_days, 1)} 天` },
        { label: 'MR / 评审', value: `${item.mr_count || 0} / ${item.review_count || 0}` },
        { label: '当前风险', value: item.total_risk }
      ],
      sections: [
        {
          title: '可解释评分',
          items: [
            `产出贡献：${item.delivery_score} / 40`,
            `代码证据：${item.evidence_score} / 25`,
            `流转效率：${item.flow_score} / 20`,
            `风险健康：${item.risk_health_score} / 15`
          ]
        },
        {
          title: '交付结构',
          items: [
            `任务 ${item.tasks_completed}`,
            `需求 ${item.demands_completed}`,
            `Bug ${item.bugs_completed}`
          ]
        },
        {
          title: '风险说明',
          body: notes.length > 0 ? undefined : '当前成员暂无后端返回的风险备注。',
          items: notes.length > 0 ? notes : undefined
        }
      ],
      actions: [
        { label: '查看个人报告', kind: 'primary' },
        { label: '团队视图', kind: 'secondary' }
      ]
    };
  }

  function clampMetric(value: number, min = 0, max = 100) {
    return Math.max(min, Math.min(max, value));
  }

  function percent(value: number, total: number) {
    if (!total || total <= 0) return 0;
    return Math.round((value / total) * 100);
  }

  function roundOneDecimal(value: number) {
    return Math.round(value * 10) / 10;
  }

  function buildAverageCycleDays(items: MemberScorecard[]) {
    const cycleItems = items.filter(item => item.avg_cycle_days > 0);
    if (cycleItems.length === 0) return 0;
    return roundOneDecimal(cycleItems.reduce((sum, item) => sum + item.avg_cycle_days, 0) / cycleItems.length);
  }

  function insightTone(value: number, good: number, watch: number, risk: number, inverse = false): KPIInsight['tone'] {
    if (inverse) {
      if (value <= good) return 'strong';
      if (value <= watch) return 'good';
      if (value <= risk) return 'watch';
      return 'risk';
    }
    if (value >= good) return 'strong';
    if (value >= watch) return 'good';
    if (value >= risk) return 'watch';
    return 'risk';
  }

  function buildKPIDepthInsights(
    topMember: MemberScorecard | null,
    lowestMember: MemberScorecard | null,
    concentration: number,
    evidencePercent: number,
    mrCount: number,
    currentSummary: KPISummary,
    riskyMembers: number,
    riskPoints: number,
    averageRiskLoad: number,
    averageReviewPressure: number,
    reviewCount: number,
    currentBugShare: number,
    currentScoreSpread: number
  ): KPIInsight[] {
    return [
      {
        label: '交付集中度',
        value: topMember ? `${concentration}%` : '-',
        helper: topMember ? `${topMember.name} 贡献 ${topMember.total_completed} 项，观察是否过度依赖单点。` : '暂无成员交付数据。',
        tone: insightTone(concentration, 35, 50, 65, true)
      },
      {
        label: '证据覆盖率',
        value: `${evidencePercent}%`,
        helper: `MR ${mrCount} / 交付 ${currentSummary.total_completed}，衡量代码证据与交付事实的贴合度。`,
        tone: insightTone(evidencePercent, 80, 55, 30)
      },
      {
        label: '风险负荷',
        value: `${averageRiskLoad}`,
        helper: `${riskyMembers} 人存在超期/延期，累计风险点 ${riskPoints}。`,
        tone: insightTone(averageRiskLoad, 0.2, 0.8, 1.5, true)
      },
      {
        label: '评审压力',
        value: `${averageReviewPressure}`,
        helper: `当前评审 ${reviewCount} 项，人均评审排队 ${averageReviewPressure} 项。`,
        tone: insightTone(averageReviewPressure, 0.4, 1, 2, true)
      },
      {
        label: 'Bug 占比',
        value: `${currentBugShare}%`,
        helper: `Bug ${currentSummary.bugs_completed} / 总交付 ${currentSummary.total_completed}，用于观察质量偿债压力。`,
        tone: insightTone(currentBugShare, 12, 25, 40, true)
      },
      {
        label: '分数离散度',
        value: `${currentScoreSpread}`,
        helper: topMember && lowestMember ? `${topMember.name} 与 ${lowestMember.name} 相差 ${currentScoreSpread} 分。` : '暂无可比较成员。',
        tone: insightTone(currentScoreSpread, 15, 28, 42, true)
      }
    ];
  }

  function buildIssueDistribution(data: KPISummary): KPIDistributionItem[] {
    const total = data.total_completed || 0;
    return [
      { label: '任务', value: data.tasks_completed, percent: percent(data.tasks_completed, total), tone: 'task' },
      { label: '需求', value: data.demands_completed, percent: percent(data.demands_completed, total), tone: 'demand' },
      { label: 'Bug', value: data.bugs_completed, percent: percent(data.bugs_completed, total), tone: 'bug' }
    ];
  }

  function buildScoreDistribution(items: MemberScorecard[]): KPIDistributionItem[] {
    const total = items.length || 0;
    const excellent = items.filter(item => item.kpi_score >= 85).length;
    const good = items.filter(item => item.kpi_score >= 70 && item.kpi_score < 85).length;
    const watch = items.filter(item => item.kpi_score >= 55 && item.kpi_score < 70).length;
    const risk = items.filter(item => item.kpi_score < 55).length;
    return [
      { label: '高绩效', value: excellent, percent: percent(excellent, total), tone: 'excellent' },
      { label: '稳定', value: good, percent: percent(good, total), tone: 'good' },
      { label: '需关注', value: watch, percent: percent(watch, total), tone: 'watch' },
      { label: '需介入', value: risk, percent: percent(risk, total), tone: 'risk' }
    ];
  }

  function buildMemberScorecard(item: UserKPI, maxDelivery: number, maxMR: number): MemberScorecard {
    const deliveryScore = maxDelivery > 0 ? Math.round((item.total_completed / maxDelivery) * 40) : 0;
    const evidenceScore = clampMetric(Math.round(((item.mr_count || 0) / maxMR) * 18 + (item.review_count || 0) * 2), 0, 25);
    const flowScore = item.total_completed === 0
      ? 0
      : clampMetric(Math.round(20 - Math.max(item.avg_cycle_days || 0, 0) * 2), 4, 20);
    const riskPenalty = Math.min(15, (item.active_overdue || 0) * 6 + (item.overdue_completed || 0) * 4);
    const riskHealthScore = 15 - riskPenalty;
    const kpiScore = clampMetric(Math.round(deliveryScore + evidenceScore + flowScore + riskHealthScore), 0, 100);

    return {
      ...item,
      kpi_score: kpiScore,
      score_label: scoreLabel(kpiScore),
      score_tone: scoreTone(kpiScore),
      delivery_score: deliveryScore,
      evidence_score: evidenceScore,
      flow_score: flowScore,
      risk_health_score: riskHealthScore,
      risk_penalty: riskPenalty,
      total_risk: (item.active_overdue || 0) + (item.overdue_completed || 0)
    };
  }

  function scoreTone(score: number): MemberScorecard['score_tone'] {
    if (score >= 85) return 'excellent';
    if (score >= 70) return 'good';
    if (score >= 55) return 'watch';
    return 'risk';
  }

  function scoreLabel(score: number) {
    if (score >= 85) return '高绩效';
    if (score >= 70) return '稳定';
    if (score >= 55) return '需关注';
    return '需介入';
  }

  function formatDateTime(value?: string) {
    if (!value) return '未生成';
    const date = new Date(value);
    if (Number.isNaN(date.getTime())) return value;
    return date.toLocaleString('zh-CN', {
      month: '2-digit',
      day: '2-digit',
      hour: '2-digit',
      minute: '2-digit',
      hour12: false
    });
  }

  function issueTypeLabel(type: string) {
    const normalized = (type || '').toLowerCase();
    if (normalized === 'bug') return 'Bug';
    if (normalized === 'demand') return '需求';
    return '任务';
  }

  function riskTone(level: string) {
    if (level === 'high') return 'high';
    if (level === 'medium') return 'medium';
    return 'low';
  }

  onMount(() => {
    fetchKPIData();
    document.addEventListener('click', handleDocumentClick);
    return () => {
      document.removeEventListener('click', handleDocumentClick);
    };
  });
</script>

<div class="kpi-dashboard font-sans" class:is-refreshing={loading && hasLoadedKPI} aria-busy={loading || reportLoading}>
  <div class="kpi-header">
    <div class="kpi-title-block">
      <span class="eyebrow">PERFORMANCE REVIEW</span>
      <h2>KPI 研发绩效大盘</h2>
      <p>按日报/周报粒度沉淀事实，用任务、需求、Bug、MR、评审、周期和风险构成可解释评分。</p>
    </div>

    <div class="header-action-stack">
      <div class="report-mode-switch" aria-label="报告粒度">
        {#each reportModes as mode}
          <button
            type="button"
            class="report-mode-btn"
            class:is-active={reportType === mode.key}
            aria-pressed={reportType === mode.key}
            on:click={() => changeReportType(mode.key)}
          >
            <strong>{mode.label}</strong>
            <span>{mode.window}</span>
          </button>
        {/each}
      </div>
      <span
        class="sync-indicator font-mono"
        class:is-visible={loading && hasLoadedKPI}
        aria-live="polite"
        aria-hidden={!(loading && hasLoadedKPI)}
      >
        同步中
      </span>
    </div>
  </div>

  {#if loading && !hasLoadedKPI}
    <div class="state-msg">正在统计 {selectedReportMode.label} 研发绩效事实</div>
  {:else if errorMsg && !hasLoadedKPI}
    <div class="state-msg error-msg font-mono">加载失败 · {errorMsg}</div>
  {:else}
    {#if errorMsg && hasLoadedKPI}
      <div class="inline-error font-mono">{errorMsg}</div>
    {/if}

    <section class="kpi-command-layout">
      <div class="report-brief-panel glass-panel wa-admin-section">
        <div class="brief-header-row">
          <div>
            <span class="panel-kicker font-mono">{selectedReportMode.label} · {selectedReportMode.window}</span>
            <h3>{reportPreview?.overview.title || `${selectedReportMode.label}绩效评估`}</h3>
            <p>{reportPreview?.overview.summary || selectedReportMode.helper}</p>
          </div>
          <div class="brief-meta font-mono">
            <span>生成时间</span>
            <strong>{formatDateTime(reportPreview?.generated_at)}</strong>
          </div>
        </div>

        <div class="brief-metric-grid">
          {#each kpiAdminMetrics as metric}
            <article class="metric-tile wa-admin-card wa-admin-metric metric-{metric.tone || 'neutral'}">
              <span>{metric.label}</span>
              <strong>{metric.value}</strong>
              <em>{metric.helper}</em>
            </article>
          {/each}
        </div>
      </div>

      <div class="score-focus-panel glass-panel wa-admin-card">
        <div class="score-ring" class:tone-excellent={averageMemberScore >= 85} class:tone-good={averageMemberScore >= 70 && averageMemberScore < 85} class:tone-watch={averageMemberScore >= 55 && averageMemberScore < 70} class:tone-risk={averageMemberScore < 55}>
          <strong class="font-mono">{averageMemberScore}</strong>
          <span>团队均分</span>
        </div>
        <div class="score-focus-copy">
          <span class="panel-kicker font-mono">SCORE MODEL</span>
          <h3>绩效评分 = 产出 + 代码证据 + 流转效率 + 风险健康</h3>
          {#if topScorecard}
            <p>当前最高分：{topScorecard.name}，{topScorecard.kpi_score} 分，交付 {topScorecard.total_completed} 项。</p>
          {:else}
            <p>当前周期暂无可评估成员，等待任务或 Bug 进入完成/评审/超期证据链。</p>
          {/if}
          <div class="focus-stat-row font-mono">
            <span>成员 {memberScorecards.length}</span>
            <span>风险成员 {riskMemberCount}</span>
            <span>评审中 {totalReviewCount}</span>
          </div>
        </div>
      </div>
    </section>

    <section class="kpi-status-strip" aria-label="KPI 事实分段">
      {#each kpiAdminSegments as segment}
        <article class="status-segment wa-admin-card metric-{segment.tone || 'neutral'}">
          <span>{segment.label}</span>
          <strong>{segment.value}</strong>
          <small>{segment.helper}</small>
        </article>
      {/each}
    </section>

    <section class="kpi-analytics-layout">
      <div class="diagnostic-panel glass-panel wa-admin-card">
        <div class="panel-header">
          <div class="panel-title-row">
            <span class="panel-kicker font-mono">DEPTH DIAGNOSIS</span>
            <h3>绩效深度诊断</h3>
            <span class="panel-subtitle">从单点依赖、证据链、风险负荷和质量压力判断绩效是否可信</span>
          </div>
        </div>

        <div class="insight-grid">
          {#each kpiDepthInsights as insight}
            <div class="insight-card tone-{insight.tone}">
              <div class="insight-head">
                <span>{insight.label}</span>
                <strong class="font-mono">{insight.value}</strong>
              </div>
              <p>{insight.helper}</p>
            </div>
          {/each}
        </div>
      </div>

      <div class="breadth-panel glass-panel wa-admin-card">
        <div class="panel-header">
          <div class="panel-title-row">
            <span class="panel-kicker font-mono">BREADTH MAP</span>
            <h3>团队广度分布</h3>
            <span class="panel-subtitle">观察任务类型、评分段和部门覆盖，避免只看总分</span>
          </div>
        </div>

        <div class="distribution-stack">
          <div class="distribution-block">
            <div class="distribution-title">
              <span>任务类型构成</span>
              <strong class="font-mono">{summary.total_completed} 项</strong>
            </div>
            <div class="mix-meter" aria-label="任务类型构成">
              {#each issueDistribution as item}
                <span class="mix-segment tone-{item.tone}" style="width: {item.percent}%"></span>
              {/each}
            </div>
            <div class="distribution-legend font-mono">
              {#each issueDistribution as item}
                <span><i class="tone-{item.tone}"></i>{item.label} {item.value} / {item.percent}%</span>
              {/each}
            </div>
          </div>

          <div class="distribution-block">
            <div class="distribution-title">
              <span>评分段分布</span>
              <strong class="font-mono">{memberScorecards.length} 人</strong>
            </div>
            <div class="score-band-grid font-mono">
              {#each scoreDistribution as item}
                <div class="score-band tone-{item.tone}">
                  <span>{item.label}</span>
                  <strong>{item.value}</strong>
                  <em>{item.percent}%</em>
                </div>
              {/each}
            </div>
          </div>

          <div class="distribution-foot font-mono">
            <span>部门覆盖</span>
            <strong>{departmentBreadthLabel}</strong>
            <span>平均周期 {formatMetric(averageCycleDays, 1)} 天</span>
          </div>
        </div>
      </div>
    </section>

    <div class="report-panel glass-panel wa-admin-card wa-admin-inspector">
      <div class="report-header">
        <div class="panel-title-row">
          <span class="panel-kicker font-mono">REPORT PREVIEW</span>
          <h3>{selectedReportMode.label}事实预览</h3>
          <span class="panel-subtitle">{selectedReportMode.helper}</span>
        </div>

        <div class="report-controls">
          <div class="report-select-container font-mono" bind:this={reportUserSelectEl}>
            <button
              type="button"
              class="report-select-trigger"
              on:click={toggleReportUserDropdown}
              aria-label="报告视角筛选"
              aria-expanded={showReportUserDropdown}
            >
              <span>{reportUserLabel}</span>
              <span class="report-select-arrow">{showReportUserDropdown ? '▲' : '▼'}</span>
            </button>
            {#if showReportUserDropdown}
              <div class="report-select-options">
                <button
                  type="button"
                  class="report-select-option {selectedReportUser === '' ? 'active' : ''}"
                  on:click={() => changeReportUser('')}
                >
                  团队视图
                </button>
                {#each memberScorecards as user}
                  <button
                    type="button"
                    class="report-select-option {selectedReportUser === user.name ? 'active' : ''}"
                    on:click={() => changeReportUser(user.name)}
                  >
                    {user.name}
                  </button>
                {/each}
              </div>
            {/if}
          </div>

          <button class="refresh-btn wa-admin-action secondary" type="button" on:click={fetchReportPreview} disabled={reportLoading} aria-busy={reportLoading}>
            {reportLoading ? '生成中' : '刷新预览'}
          </button>
        </div>
      </div>

      {#if memberInspectorRecord}
        <section class="member-inspector-card" aria-label="成员评分详情">
          <div class="inspector-head">
            <div>
              <span class="inspector-kicker">成员详情</span>
              <h3>{memberInspectorRecord.title}</h3>
            </div>
            <span class="wa-admin-pill {toneClass(memberInspectorRecord.tone)}">{memberInspectorRecord.status}</span>
          </div>

          <dl class="fact-grid">
            {#each memberInspectorRecord.facts as fact}
              <div>
                <dt>{fact.label}</dt>
                <dd>{fact.value}</dd>
              </div>
            {/each}
          </dl>

          {#each memberInspectorRecord.sections as section}
            <section class="inspector-section">
              <h4>{section.title}</h4>
              {#if section.body}
                <p>{section.body}</p>
              {/if}
              {#if section.items}
                <ul>
                  {#each section.items as item}
                    <li>{item}</li>
                  {/each}
                </ul>
              {/if}
            </section>
          {/each}

          <div class="inspector-actions">
            <button class="wa-admin-action primary" type="button" on:click={() => selectedMember && changeReportUser(selectedMember.name)}>
              查看个人报告
            </button>
            <button class="wa-admin-action secondary" type="button" on:click={() => changeReportUser('')}>
              团队视图
            </button>
          </div>
        </section>
      {/if}

      {#if reportErrorMsg}
        <div class="inline-error font-mono">{reportErrorMsg}</div>
      {:else if reportLoading && !reportPreview}
        <div class="report-loading">
          <div class="skeleton-line wide"></div>
          <div class="skeleton-grid">
            <div class="skeleton-line"></div>
            <div class="skeleton-line"></div>
            <div class="skeleton-line"></div>
          </div>
        </div>
      {:else if reportPreview}
        <div class="report-overview-strip font-mono">
          <div>
            <span>范围</span>
            <strong>{reportPreview.overview.period_label}</strong>
          </div>
          <div>
            <span>交付</span>
            <strong>{reportPreview.overview.total_completed}</strong>
          </div>
          <div>
            <span>当前超期</span>
            <strong class="text-amber">{reportPreview.overview.active_overdue}</strong>
          </div>
          <div>
            <span>延期完成</span>
            <strong class="text-rose">{reportPreview.overview.overdue_completed}</strong>
          </div>
          <div>
            <span>MR</span>
            <strong class="text-teal">{reportPreview.overview.mr_count}</strong>
          </div>
        </div>

        <div class="report-summary">
          <strong>{reportPreview.overview.title}</strong>
          <span>{reportPreview.overview.summary}</span>
        </div>

        <div class="report-grid">
          <section class="report-block">
            <div class="report-block-header">
              <h4>个人绩效分析</h4>
              <span class="font-mono">{reportPreview.personal_sections.length} 人</span>
            </div>
            {#if focusedPersonalSection}
              <div class="personal-focus">
                <div class="focus-title">
                  <strong>{focusedPersonalSection.user.name}</strong>
                  <span>{focusedPersonalSection.user.department || '未分配'}</span>
                </div>
                {#each focusedPersonalSection.highlights as highlight}
                  <p>{highlight}</p>
                {/each}
                {#if focusedPersonalSection.risk_notes.length > 0}
                  <div class="risk-note-list">
                    {#each focusedPersonalSection.risk_notes as note}
                      <span>{note}</span>
                    {/each}
                  </div>
                {/if}
              </div>
            {:else}
              <div class="empty-msg font-mono">暂无个人报告数据</div>
            {/if}
          </section>

          <section class="report-block">
            <div class="report-block-header">
              <h4>风险点</h4>
              <span class="font-mono">{reportPreview.risks.length} 项</span>
            </div>
            {#if reportPreview.risks.length === 0}
              <div class="empty-msg font-mono">暂无超期或延期风险</div>
            {:else}
              <div class="compact-list">
                {#each reportPreview.risks.slice(0, 4) as risk}
                  <div class="compact-row">
                    <span class="risk-pill {riskTone(risk.level)} font-mono">{risk.level}</span>
                    <div>
                      <strong>{risk.title}</strong>
                      <p>{risk.detail}</p>
                      <span class="row-meta font-mono">{risk.owner} / {risk.department} / {risk.evidence_ids.join(', ')}</span>
                    </div>
                  </div>
                {/each}
              </div>
            {/if}
          </section>

          <section class="report-block">
            <div class="report-block-header">
              <h4>会议关注项</h4>
              <span class="font-mono">{reportPreview.meeting_focus.length} 项</span>
            </div>
            {#if reportPreview.meeting_focus.length === 0}
              <div class="empty-msg font-mono">暂无需要升级到会议的关注项</div>
            {:else}
              <div class="compact-list">
                {#each reportPreview.meeting_focus.slice(0, 4) as focus}
                  <div class="compact-row">
                    <span class="focus-tag font-mono">{focus.topic}</span>
                    <div>
                      <strong>{focus.owner}</strong>
                      <p>{focus.reason}</p>
                      <span class="row-meta font-mono">{focus.department} / {focus.evidence_ids.join(', ')}</span>
                    </div>
                  </div>
                {/each}
              </div>
            {/if}
          </section>

          <section class="report-block evidence-block">
            <div class="report-block-header">
              <h4>证据</h4>
              <span class="font-mono">{reportPreview.evidence.length} 条</span>
            </div>
            {#if visibleReportEvidence.length === 0}
              <div class="empty-msg font-mono">暂无可追溯证据</div>
            {:else}
              <div class="evidence-list">
                {#each visibleReportEvidence as item}
                  <div class="evidence-row">
                    <span class="evidence-id font-mono">{item.task_id}</span>
                    <span class="evidence-title">{item.title}</span>
                    <span class="row-meta font-mono">{issueTypeLabel(item.issue_type)} / {item.status} / {item.assignee}</span>
                  </div>
                {/each}
              </div>
            {/if}
          </section>
        </div>
      {/if}
    </div>

    <section class="member-matrix-panel glass-panel wa-admin-section">
      <div class="panel-header">
        <div class="panel-title-row">
          <span class="panel-kicker font-mono">MEMBER MATRIX</span>
          <h3>成员任务/Bug 与评分矩阵</h3>
          <span class="panel-subtitle">分数由产出贡献、代码证据、流转效率和风险健康组成</span>
        </div>
        <span class="panel-count font-mono">{memberScorecards.length} 人</span>
      </div>

      {#if memberScorecards.length === 0}
        <div class="empty-msg font-mono">该周期内暂无可评估的任务、Bug、MR 或风险证据</div>
      {:else}
        <div class="member-table-wrap wa-admin-table-shell">
          <table class="member-performance-table wa-admin-table">
            <colgroup>
              {#each memberColumns as column}
                <col style={column.width ? `width: ${column.width}` : undefined} />
              {/each}
            </colgroup>
            <thead>
              <tr>
                {#each memberColumns as column}
                  <th class:cell-right={column.align === 'right'}>{column.label}</th>
                {/each}
              </tr>
            </thead>
            <tbody>
              {#each memberTableRows as row}
                <tr class:is-selected={selectedMemberKey === row.id}>
                  <td>
                    <div class="member-cell">
                      <span class="rank-badge font-mono" class:rank-1={row.rank === 1} class:rank-2={row.rank === 2} class:rank-3={row.rank === 3}>{row.rank}</span>
                      {#if row.source.avatar}
                        <img src={row.source.avatar} alt={row.source.name} class="member-avatar" />
                      {:else}
                        <span class="avatar-placeholder font-mono">{row.source.name.slice(0, 1).toUpperCase()}</span>
                      {/if}
                      <div class="member-identity">
                        <button type="button" class="member-select-btn" on:click={() => selectedMemberId = row.id}>
                          {row.title}
                        </button>
                        <span>{row.cells.department}</span>
                      </div>
                    </div>
                  </td>
                  <td class="numeric">
                    <div class="score-cell">
                      <strong
                        class="score-chip wa-admin-pill font-mono {toneClass(row.tone)}"
                      >
                        {row.cells.score}
                      </strong>
                      <span>{row.status}</span>
                    </div>
                  </td>
                  <td>
                    <div class="issue-count-grid font-mono">
                      <span><b>{row.cells.tasks}</b>任务</span>
                      <span><b>{row.cells.demands}</b>需求</span>
                      <span><b>{row.cells.bugs}</b>Bug</span>
                      <strong>{row.cells.delivery} 项</strong>
                    </div>
                  </td>
                  <td>
                    <div class="flow-stat-stack font-mono">
                      <span>周期 {row.cells.cycle} 天</span>
                      <span>MR {row.cells.mr}</span>
                      <span>评审 {row.cells.review}</span>
                    </div>
                  </td>
                  <td>
                    <div class="risk-stack font-mono" class:is-risk={Number(row.cells.risk) > 0}>
                      <strong>{row.cells.risk}</strong>
                      <span>当前 {row.cells.activeOverdue} / 延期 {row.cells.overdueCompleted}</span>
                    </div>
                  </td>
                  <td>
                    <div class="score-breakdown-grid font-mono">
                      <span class="score-part">产出 {row.source.delivery_score}</span>
                      <span class="score-part">证据 {row.source.evidence_score}</span>
                      <span class="score-part">效率 {row.source.flow_score}</span>
                      <span class="score-part">风险 {row.source.risk_health_score}</span>
                    </div>
                  </td>
                </tr>
              {/each}
            </tbody>
          </table>
        </div>
      {/if}
    </section>

    <section class="department-panel glass-panel wa-admin-card">
      <div class="panel-header">
        <div class="panel-title-row">
          <span class="panel-kicker font-mono">DEPARTMENT MIX</span>
          <h3>部门完成指标汇总</h3>
          <span class="panel-subtitle">部门维度只展示完成事实，不参与个人评分扣分</span>
        </div>
      </div>

      <div class="dept-performance-list">
        {#if deptKPIList.length === 0}
          <div class="empty-msg font-mono">该周期内暂无已交付的部门数据</div>
        {:else}
          {#each deptKPIList as item, index}
            <div class="dept-performance-row">
              <div class="dept-row-head">
                <span class="rank-badge font-mono" class:rank-1={index === 0} class:rank-2={index === 1} class:rank-3={index === 2}>{index + 1}</span>
                <strong>{item.department}</strong>
                <em class="font-mono">{item.total_completed} 项</em>
              </div>
              <div class="dept-meter">
                <span style="width: {(item.total_completed / maxDeptTotal) * 100}%"></span>
              </div>
              <div class="dept-mix font-mono">
                <span>任务 {item.tasks_completed}</span>
                <span>需求 {item.demands_completed}</span>
                <span>Bug {item.bugs_completed}</span>
              </div>
            </div>
          {/each}
        {/if}
      </div>
    </section>

  {/if}
</div>

<style>
  .kpi-dashboard {
    display: flex;
    flex-direction: column;
    gap: 18px;
    margin-top: 10px;
    color: #e2e8f0;
    overflow-anchor: none;
  }

  .kpi-header,
  .report-header,
  .panel-header,
  .brief-header-row {
    display: flex;
    justify-content: space-between;
    align-items: flex-start;
    flex-wrap: wrap;
    gap: 16px;
  }

  .kpi-title-block {
    max-width: 760px;
  }

  .kpi-dashboard.is-refreshing {
    cursor: progress;
  }

  .header-action-stack {
    display: grid;
    grid-template-rows: auto 24px;
    justify-items: end;
    gap: 8px;
  }

  .sync-indicator {
    min-width: 58px;
    min-height: 24px;
    border: 1px solid rgba(84, 202, 182, 0.22);
    border-radius: 999px;
    padding: 5px 10px;
    color: #8ef5e4;
    background: rgba(17, 65, 72, 0.24);
    font-size: 0.66rem;
    opacity: 0;
    pointer-events: none;
    text-align: center;
    visibility: hidden;
    transition: opacity 0.18s ease, border-color 0.18s ease, background 0.18s ease;
  }

  .sync-indicator.is-visible {
    opacity: 1;
    visibility: visible;
  }

  .eyebrow,
  .panel-kicker {
    display: block;
    margin-bottom: 7px;
    color: #8290a6;
    font-size: 0.66rem;
    font-weight: 800;
    letter-spacing: 0;
    text-transform: uppercase;
  }

  .kpi-header h2,
  .brief-header-row h3,
  .score-focus-copy h3,
  .panel-title-row h3,
  .report-block-header h4 {
    margin: 0;
    color: #eef4ff;
    font-weight: 800;
    letter-spacing: 0;
  }

  .kpi-header h2 {
    font-size: 1.55rem;
    line-height: 1.2;
  }

  .kpi-title-block p,
  .brief-header-row p,
  .score-focus-copy p,
  .panel-subtitle {
    margin: 8px 0 0;
    color: #9aa8bb;
    font-size: 0.82rem;
    line-height: 1.65;
  }

  .brief-header-row p,
  .score-focus-copy p {
    min-height: 2.7em;
  }

  .panel-subtitle {
    display: block;
    min-height: 1.35em;
  }

  .report-mode-switch {
    display: grid;
    grid-template-columns: repeat(2, minmax(132px, 1fr));
    gap: 8px;
    padding: 4px;
    background: rgba(13, 19, 34, 0.78);
    border: 1px solid rgba(116, 129, 157, 0.22);
    border-radius: 10px;
  }

  .report-mode-btn {
    min-height: 54px;
    border: 1px solid transparent;
    border-radius: 8px;
    padding: 8px 14px;
    background: transparent;
    color: #95a2b7;
    text-align: left;
    cursor: pointer;
    transition: border-color 0.18s ease, background 0.18s ease, color 0.18s ease;
  }

  .report-mode-btn strong,
  .report-mode-btn span {
    display: block;
  }

  .report-mode-btn strong {
    color: inherit;
    font-size: 0.9rem;
    line-height: 1.15;
  }

  .report-mode-btn span {
    margin-top: 5px;
    color: #708094;
    font-size: 0.68rem;
  }

  .report-mode-btn:hover,
  .report-mode-btn.is-active {
    border-color: rgba(84, 202, 182, 0.34);
    background: rgba(23, 80, 87, 0.22);
    color: #dffcf5;
  }

  .report-mode-btn.is-active span {
    color: #7ddfd1;
  }

  .glass-panel {
    border: 1px solid rgba(83, 99, 130, 0.28);
    border-radius: 8px;
    background:
      linear-gradient(180deg, rgba(15, 23, 42, 0.88), rgba(8, 13, 25, 0.92)),
      rgba(12, 18, 31, 0.9);
    box-shadow: 0 18px 48px rgba(2, 8, 23, 0.36);
  }

  .kpi-command-layout {
    display: grid;
    grid-template-columns: minmax(0, 1.75fr) minmax(320px, 0.85fr);
    gap: 14px;
  }

  .kpi-analytics-layout {
    display: grid;
    grid-template-columns: minmax(0, 1.35fr) minmax(360px, 0.9fr);
    gap: 14px;
  }

  .report-brief-panel,
  .score-focus-panel,
  .report-panel,
  .member-matrix-panel,
  .department-panel,
  .diagnostic-panel,
  .breadth-panel {
    padding: 20px;
  }

  .report-panel,
  .member-matrix-panel,
  .department-panel,
  .report-brief-panel,
  .diagnostic-panel,
  .breadth-panel {
    display: flex;
    flex-direction: column;
    gap: 16px;
  }

  .report-panel {
    min-height: 520px;
  }

  .member-matrix-panel {
    min-height: 468px;
  }

  .brief-meta {
    min-width: 142px;
    border: 1px solid rgba(84, 202, 182, 0.18);
    border-radius: 8px;
    padding: 10px 12px;
    background: rgba(11, 31, 37, 0.52);
  }

  .brief-meta span,
  .metric-tile span,
  .report-overview-strip span,
  .row-meta {
    display: block;
    color: #758399;
    font-size: 0.66rem;
  }

  .brief-meta strong {
    display: block;
    margin-top: 5px;
    color: #dffcf5;
    font-size: 0.78rem;
  }

  .brief-metric-grid {
    display: grid;
    grid-template-columns: repeat(6, minmax(112px, 1fr));
    gap: 10px;
  }

  .metric-tile {
    min-height: 94px;
    border: 1px solid rgba(72, 86, 113, 0.32);
    border-radius: 8px;
    padding: 13px;
    background: rgba(11, 17, 30, 0.74);
  }

  .metric-tile strong {
    display: block;
    margin-top: 9px;
    color: #edf5ff;
    font-size: 1.55rem;
    line-height: 1;
  }

  .metric-tile em {
    display: block;
    margin-top: 9px;
    color: #7d8ba2;
    font-size: 0.66rem;
    font-style: normal;
  }

  .metric-tile.is-primary {
    border-color: rgba(84, 202, 182, 0.36);
    background: rgba(17, 65, 72, 0.32);
  }

  .metric-tile.is-primary strong {
    color: #8ef5e4;
  }

  .metric-tile.is-warn {
    border-color: rgba(239, 177, 74, 0.34);
    background: rgba(75, 48, 18, 0.28);
  }

  .metric-tile.is-warn strong {
    color: #f5c970;
  }

  .score-focus-panel {
    display: grid;
    grid-template-columns: 112px minmax(0, 1fr);
    gap: 16px;
    align-items: center;
  }

  .score-ring {
    position: relative;
    display: grid;
    place-items: center;
    width: 112px;
    height: 112px;
    border: 1px solid rgba(84, 202, 182, 0.34);
    border-radius: 50%;
    background:
      radial-gradient(circle at center, rgba(12, 18, 31, 0.96) 58%, transparent 60%),
      conic-gradient(from 215deg, #54cab6, #84d3ff, rgba(72, 86, 113, 0.38) 68%);
  }

  .score-ring strong {
    color: #eff8ff;
    font-size: 2rem;
    line-height: 1;
  }

  .score-ring span {
    margin-top: -22px;
    color: #8a98ae;
    font-size: 0.66rem;
  }

  .score-ring.tone-good {
    border-color: rgba(129, 140, 248, 0.34);
    background:
      radial-gradient(circle at center, rgba(12, 18, 31, 0.96) 58%, transparent 60%),
      conic-gradient(from 215deg, #8da2ff, #54cab6, rgba(72, 86, 113, 0.38) 68%);
  }

  .score-ring.tone-watch {
    border-color: rgba(239, 177, 74, 0.34);
    background:
      radial-gradient(circle at center, rgba(12, 18, 31, 0.96) 58%, transparent 60%),
      conic-gradient(from 215deg, #f5c970, #8da2ff, rgba(72, 86, 113, 0.38) 68%);
  }

  .score-ring.tone-risk {
    border-color: rgba(248, 113, 113, 0.34);
    background:
      radial-gradient(circle at center, rgba(12, 18, 31, 0.96) 58%, transparent 60%),
      conic-gradient(from 215deg, #f87171, #f5c970, rgba(72, 86, 113, 0.38) 68%);
  }

  .focus-stat-row {
    display: flex;
    flex-wrap: wrap;
    gap: 8px;
    margin-top: 12px;
  }

  .focus-stat-row span {
    border: 1px solid rgba(83, 99, 130, 0.28);
    border-radius: 6px;
    padding: 5px 8px;
    color: #b4c0d2;
    background: rgba(13, 21, 36, 0.68);
    font-size: 0.66rem;
  }

  .insight-grid {
    display: grid;
    grid-template-columns: repeat(3, minmax(0, 1fr));
    gap: 10px;
  }

  .insight-card {
    min-height: 118px;
    border: 1px solid rgba(72, 86, 113, 0.28);
    border-radius: 8px;
    padding: 12px;
    background: rgba(10, 16, 29, 0.56);
  }

  .insight-head {
    display: flex;
    align-items: flex-start;
    justify-content: space-between;
    gap: 10px;
  }

  .insight-head span {
    color: #9aa8bb;
    font-size: 0.72rem;
    font-weight: 800;
  }

  .insight-head strong {
    color: #edf5ff;
    font-size: 1.1rem;
    line-height: 1;
  }

  .insight-card p {
    margin: 12px 0 0;
    color: #8390a5;
    font-size: 0.72rem;
    line-height: 1.55;
    text-wrap: pretty;
  }

  .insight-card.tone-strong {
    border-color: rgba(84, 202, 182, 0.3);
    background: rgba(17, 65, 72, 0.2);
  }

  .insight-card.tone-good {
    border-color: rgba(129, 140, 248, 0.28);
    background: rgba(42, 50, 104, 0.18);
  }

  .insight-card.tone-watch {
    border-color: rgba(239, 177, 74, 0.28);
    background: rgba(75, 48, 18, 0.18);
  }

  .insight-card.tone-risk {
    border-color: rgba(248, 113, 113, 0.28);
    background: rgba(96, 28, 34, 0.18);
  }

  .distribution-stack,
  .distribution-block {
    display: grid;
    gap: 12px;
  }

  .distribution-block {
    border: 1px solid rgba(72, 86, 113, 0.28);
    border-radius: 8px;
    padding: 12px;
    background: rgba(10, 16, 29, 0.56);
  }

  .distribution-title,
  .distribution-foot {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 10px;
  }

  .distribution-title span,
  .distribution-foot span {
    color: #8390a5;
    font-size: 0.72rem;
  }

  .distribution-title strong,
  .distribution-foot strong {
    color: #edf5ff;
    font-size: 0.78rem;
  }

  .mix-meter {
    display: flex;
    height: 10px;
    overflow: hidden;
    border-radius: 999px;
    background: rgba(72, 86, 113, 0.24);
  }

  .mix-segment {
    min-width: 0;
    height: 100%;
  }

  .mix-segment.tone-task,
  .distribution-legend i.tone-task {
    background: #84d3ff;
  }

  .mix-segment.tone-demand,
  .distribution-legend i.tone-demand {
    background: #8ef5e4;
  }

  .mix-segment.tone-bug,
  .distribution-legend i.tone-bug {
    background: #f5c970;
  }

  .distribution-legend {
    display: flex;
    flex-wrap: wrap;
    gap: 8px;
  }

  .distribution-legend span {
    display: inline-flex;
    align-items: center;
    gap: 6px;
    color: #aeb9ca;
    font-size: 0.66rem;
  }

  .distribution-legend i {
    width: 7px;
    height: 7px;
    border-radius: 50%;
  }

  .score-band-grid {
    display: grid;
    grid-template-columns: repeat(4, minmax(0, 1fr));
    gap: 8px;
  }

  .score-band {
    min-height: 72px;
    border: 1px solid rgba(72, 86, 113, 0.24);
    border-radius: 7px;
    padding: 9px;
    background: rgba(12, 18, 31, 0.54);
  }

  .score-band span,
  .score-band em {
    display: block;
    color: #8390a5;
    font-size: 0.64rem;
    font-style: normal;
  }

  .score-band strong {
    display: block;
    margin: 5px 0;
    color: #edf5ff;
    font-size: 1.08rem;
    line-height: 1;
  }

  .score-band.tone-excellent strong {
    color: #8ef5e4;
  }

  .score-band.tone-good strong {
    color: #b9c4ff;
  }

  .score-band.tone-watch strong {
    color: #f5c970;
  }

  .score-band.tone-risk strong {
    color: #fca5a5;
  }

  .distribution-foot {
    border: 1px solid rgba(84, 202, 182, 0.18);
    border-radius: 8px;
    padding: 10px 12px;
    background: rgba(17, 65, 72, 0.18);
  }

  .panel-title-row {
    min-width: 240px;
  }

  .panel-title-row h3 {
    font-size: 1rem;
  }

  .panel-count {
    border: 1px solid rgba(84, 202, 182, 0.2);
    border-radius: 999px;
    padding: 6px 10px;
    color: #8ef5e4;
    background: rgba(12, 55, 60, 0.26);
    font-size: 0.68rem;
  }

  .report-controls {
    display: flex;
    align-items: center;
    gap: 10px;
    flex-wrap: wrap;
  }

  .refresh-btn,
  .report-select-trigger {
    min-height: 34px;
    border-radius: 7px;
    font-size: 0.72rem;
    font-weight: 700;
    cursor: pointer;
    transition: border-color 0.18s ease, background 0.18s ease, color 0.18s ease;
  }

  .refresh-btn {
    border: 1px solid rgba(84, 202, 182, 0.28);
    min-width: 86px;
    padding: 0 13px;
    color: #dffcf5;
    background: rgba(17, 65, 72, 0.52);
    text-align: center;
  }

  .refresh-btn:hover,
  .report-select-trigger:hover,
  .report-select-trigger:focus {
    border-color: rgba(84, 202, 182, 0.58);
    background: rgba(20, 76, 84, 0.58);
  }

  .refresh-btn:disabled {
    cursor: wait;
    opacity: 0.64;
  }

  .refresh-btn:active,
  .report-select-trigger:active,
  .report-mode-btn:active,
  .report-select-option:active {
    border-color: rgba(84, 202, 182, 0.68);
    background: rgba(20, 76, 84, 0.66);
  }

  .report-select-container {
    position: relative;
    width: 168px;
    z-index: 20;
  }

  .report-select-trigger {
    width: 100%;
    display: inline-flex;
    align-items: center;
    justify-content: space-between;
    gap: 10px;
    border: 1px solid rgba(116, 129, 157, 0.26);
    padding: 0 10px 0 12px;
    color: #cbd7e8;
    background: rgba(12, 18, 31, 0.76);
    outline: none;
  }

  .report-select-trigger span:first-child {
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    text-align: left;
  }

  .report-select-arrow {
    flex: 0 0 auto;
    color: #54cab6;
    font-size: 0.58rem;
    line-height: 1;
  }

  .report-select-options {
    position: absolute;
    top: calc(100% + 6px);
    right: 0;
    z-index: 120;
    display: flex;
    width: max-content;
    min-width: 168px;
    max-width: min(280px, 76vw);
    max-height: 232px;
    flex-direction: column;
    gap: 2px;
    overflow-y: auto;
    border: 1px solid rgba(84, 202, 182, 0.24);
    border-radius: 8px;
    padding: 4px;
    background: #0c1323;
    box-shadow: 0 18px 36px rgba(2, 8, 23, 0.62);
    scrollbar-width: thin;
    scrollbar-color: rgba(84, 202, 182, 0.35) transparent;
  }

  .report-select-option {
    width: 100%;
    border: 0;
    border-radius: 6px;
    padding: 8px 10px;
    color: #cbd7e8;
    background: transparent;
    font-family: inherit;
    font-size: 0.75rem;
    line-height: 1.35;
    text-align: left;
    cursor: pointer;
    white-space: normal;
    word-break: keep-all;
  }

  .report-select-option:hover,
  .report-select-option.active {
    color: #dffcf5;
    background: rgba(23, 80, 87, 0.54);
  }

  .inline-error {
    border: 1px solid rgba(248, 113, 113, 0.25);
    border-radius: 8px;
    padding: 10px 12px;
    color: #fca5a5;
    background: rgba(96, 28, 34, 0.2);
    font-size: 0.75rem;
  }

  .report-loading {
    display: grid;
    gap: 12px;
  }

  .skeleton-grid {
    display: grid;
    grid-template-columns: repeat(3, 1fr);
    gap: 10px;
  }

  .skeleton-line {
    height: 42px;
    border-radius: 8px;
    background: linear-gradient(90deg, rgba(30, 41, 59, 0.34), rgba(68, 84, 111, 0.54), rgba(30, 41, 59, 0.34));
    background-size: 180% 100%;
    animation: shimmer 1.4s ease infinite;
  }

  .skeleton-line.wide {
    height: 54px;
  }

  .report-overview-strip {
    display: grid;
    grid-template-columns: repeat(5, minmax(120px, 1fr));
    gap: 9px;
  }

  .report-overview-strip div {
    display: flex;
    min-height: 64px;
    flex-direction: column;
    justify-content: center;
    gap: 6px;
    border: 1px solid rgba(72, 86, 113, 0.32);
    border-radius: 8px;
    padding: 10px 12px;
    background: rgba(10, 16, 29, 0.62);
  }

  .report-overview-strip strong {
    color: #edf5ff;
    font-size: 0.94rem;
  }

  .report-summary {
    display: flex;
    flex-direction: column;
    gap: 6px;
    border-left: 3px solid rgba(84, 202, 182, 0.65);
    padding: 8px 0 8px 12px;
  }

  .report-summary strong {
    color: #edf5ff;
    font-size: 0.92rem;
  }

  .report-summary span,
  .compact-row p,
  .personal-focus p {
    margin: 0;
    color: #98a6ba;
    font-size: 0.78rem;
    line-height: 1.55;
  }

  .report-grid {
    display: grid;
    grid-template-columns: repeat(2, minmax(0, 1fr));
    gap: 12px;
  }

  .report-block {
    min-width: 0;
    border: 1px solid rgba(72, 86, 113, 0.32);
    border-radius: 8px;
    padding: 14px;
    background: rgba(10, 16, 29, 0.58);
  }

  .report-block-header {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 10px;
    margin-bottom: 12px;
    border-bottom: 1px solid rgba(72, 86, 113, 0.2);
    padding-bottom: 10px;
  }

  .report-block-header h4 {
    color: #e7eef9;
    font-size: 0.88rem;
  }

  .report-block-header span {
    color: #758399;
    font-size: 0.68rem;
  }

  .personal-focus,
  .compact-list,
  .evidence-list {
    display: flex;
    flex-direction: column;
    gap: 10px;
  }

  .focus-title {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 12px;
  }

  .focus-title strong {
    color: #edf5ff;
    font-size: 0.94rem;
  }

  .focus-title span {
    color: #7ddfd1;
    font-size: 0.72rem;
  }

  .risk-note-list {
    display: flex;
    flex-wrap: wrap;
    gap: 6px;
  }

  .risk-note-list span {
    border: 1px solid rgba(239, 177, 74, 0.25);
    border-radius: 6px;
    padding: 4px 8px;
    color: #f5c970;
    background: rgba(75, 48, 18, 0.2);
    font-size: 0.68rem;
  }

  .compact-row {
    display: grid;
    grid-template-columns: max-content minmax(0, 1fr);
    gap: 10px;
    align-items: start;
  }

  .compact-row strong {
    color: #e2eaf5;
    font-size: 0.82rem;
  }

  .risk-pill,
  .focus-tag,
  .evidence-id {
    border-radius: 6px;
    padding: 4px 7px;
    font-size: 0.62rem;
    white-space: nowrap;
  }

  .risk-pill.high {
    border: 1px solid rgba(248, 113, 113, 0.26);
    color: #fca5a5;
    background: rgba(96, 28, 34, 0.22);
  }

  .risk-pill.medium {
    border: 1px solid rgba(239, 177, 74, 0.26);
    color: #f5c970;
    background: rgba(75, 48, 18, 0.22);
  }

  .risk-pill.low,
  .focus-tag {
    border: 1px solid rgba(129, 140, 248, 0.25);
    color: #b9c4ff;
    background: rgba(42, 50, 104, 0.26);
  }

  .evidence-row {
    display: grid;
    grid-template-columns: minmax(96px, 0.72fr) minmax(0, 1.4fr) minmax(104px, 0.74fr);
    gap: 8px;
    align-items: center;
    border-bottom: 1px solid rgba(72, 86, 113, 0.16);
    padding-bottom: 8px;
  }

  .evidence-row:last-child {
    border-bottom: 0;
    padding-bottom: 0;
  }

  .evidence-id {
    overflow: hidden;
    border: 1px solid rgba(84, 202, 182, 0.22);
    color: #8ef5e4;
    background: rgba(17, 65, 72, 0.26);
    text-overflow: ellipsis;
  }

  .evidence-title {
    overflow: hidden;
    color: #cbd7e8;
    font-size: 0.74rem;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .member-table-wrap {
    min-height: 320px;
    max-height: 520px;
    overflow-x: auto;
    overflow-y: auto;
    border: 1px solid rgba(72, 86, 113, 0.26);
    border-radius: 8px;
    overflow-anchor: none;
  }

  .member-performance-table {
    width: 100%;
    min-width: 980px;
    border-collapse: collapse;
    table-layout: fixed;
  }

  .member-performance-table th {
    position: sticky;
    top: 0;
    z-index: 2;
    height: 42px;
    border-bottom: 1px solid rgba(72, 86, 113, 0.3);
    padding: 0 12px;
    color: #758399;
    background: rgba(11, 17, 30, 0.74);
    font-size: 0.68rem;
    font-weight: 800;
    text-align: left;
    text-transform: uppercase;
  }

  .member-performance-table td {
    border-bottom: 1px solid rgba(72, 86, 113, 0.18);
    padding: 12px;
    vertical-align: middle;
  }

  .member-performance-table tbody tr {
    background: rgba(10, 16, 29, 0.36);
    transition: background 0.18s ease;
  }

  .member-performance-table tbody tr:hover {
    background: rgba(22, 32, 52, 0.58);
  }

  .member-performance-table tbody tr:last-child td {
    border-bottom: 0;
  }

  .member-matrix-panel .empty-msg {
    min-height: 320px;
    display: grid;
    place-items: center;
  }

  .numeric {
    text-align: right;
  }

  .member-cell {
    display: flex;
    align-items: center;
    gap: 10px;
    min-width: 0;
  }

  .rank-badge {
    display: inline-flex;
    flex: 0 0 auto;
    align-items: center;
    justify-content: center;
    width: 24px;
    height: 24px;
    border: 1px solid rgba(116, 129, 157, 0.22);
    border-radius: 50%;
    color: #9aa8bb;
    background: rgba(23, 32, 50, 0.78);
    font-size: 0.72rem;
    font-weight: 800;
  }

  .rank-1 {
    border-color: rgba(245, 201, 112, 0.36);
    color: #3a2a0d;
    background: #f5c970;
  }

  .rank-2 {
    border-color: rgba(199, 211, 229, 0.34);
    color: #263246;
    background: #c7d3e5;
  }

  .rank-3 {
    border-color: rgba(201, 153, 101, 0.34);
    color: #2b1a0c;
    background: #c99965;
  }

  .member-avatar,
  .avatar-placeholder {
    width: 34px;
    height: 34px;
    border-radius: 50%;
  }

  .member-avatar {
    border: 1px solid rgba(84, 202, 182, 0.28);
    object-fit: cover;
  }

  .avatar-placeholder {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    border: 1px solid rgba(84, 202, 182, 0.28);
    color: #dffcf5;
    background: rgba(17, 65, 72, 0.48);
    font-weight: 800;
  }

  .member-identity {
    min-width: 0;
  }

  .member-identity strong {
    display: block;
    overflow: hidden;
    color: #edf5ff;
    font-size: 0.84rem;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .member-identity span {
    display: block;
    margin-top: 3px;
    overflow: hidden;
    color: #758399;
    font-size: 0.68rem;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .score-cell,
  .flow-stat-stack,
  .risk-stack,
  .issue-count-grid,
  .score-breakdown-grid {
    display: grid;
    gap: 6px;
  }

  .score-cell {
    justify-items: end;
  }

  .score-cell span {
    color: #8a98ae;
    font-size: 0.66rem;
  }

  .score-chip {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    min-width: 46px;
    border: 1px solid rgba(84, 202, 182, 0.28);
    border-radius: 999px;
    padding: 5px 9px;
    color: #8ef5e4;
    background: rgba(17, 65, 72, 0.3);
    font-size: 0.88rem;
  }

  .score-chip.tone-good {
    border-color: rgba(129, 140, 248, 0.3);
    color: #b9c4ff;
    background: rgba(42, 50, 104, 0.25);
  }

  .score-chip.tone-watch {
    border-color: rgba(239, 177, 74, 0.3);
    color: #f5c970;
    background: rgba(75, 48, 18, 0.23);
  }

  .score-chip.tone-risk {
    border-color: rgba(248, 113, 113, 0.28);
    color: #fca5a5;
    background: rgba(96, 28, 34, 0.22);
  }

  .issue-count-grid {
    grid-template-columns: repeat(3, minmax(0, 1fr));
    max-width: 250px;
  }

  .issue-count-grid span,
  .issue-count-grid strong,
  .flow-stat-stack span,
  .score-part {
    border: 1px solid rgba(72, 86, 113, 0.24);
    border-radius: 6px;
    padding: 5px 7px;
    color: #aeb9ca;
    background: rgba(12, 18, 31, 0.54);
    font-size: 0.66rem;
    white-space: nowrap;
  }

  .issue-count-grid span b {
    margin-right: 3px;
    color: #edf5ff;
  }

  .issue-count-grid strong {
    grid-column: 1 / -1;
    color: #8ef5e4;
    background: rgba(17, 65, 72, 0.24);
  }

  .flow-stat-stack {
    width: max-content;
  }

  .risk-stack {
    width: max-content;
    min-width: 92px;
    justify-items: start;
    border: 1px solid rgba(72, 86, 113, 0.28);
    border-radius: 7px;
    padding: 8px;
    background: rgba(12, 18, 31, 0.54);
  }

  .risk-stack strong {
    color: #aeb9ca;
    font-size: 1rem;
  }

  .risk-stack span {
    color: #7d8ba2;
    font-size: 0.64rem;
  }

  .risk-stack.is-risk {
    border-color: rgba(239, 177, 74, 0.28);
    background: rgba(75, 48, 18, 0.2);
  }

  .risk-stack.is-risk strong {
    color: #f5c970;
  }

  .score-breakdown-grid {
    grid-template-columns: repeat(2, max-content);
  }

  .score-part {
    color: #c5d0df;
  }

  .dept-performance-list {
    display: grid;
    grid-template-columns: repeat(auto-fit, minmax(240px, 1fr));
    gap: 12px;
  }

  .dept-performance-row {
    display: grid;
    gap: 12px;
    border: 1px solid rgba(72, 86, 113, 0.3);
    border-radius: 8px;
    padding: 13px;
    background: rgba(10, 16, 29, 0.56);
  }

  .dept-row-head {
    display: flex;
    align-items: center;
    gap: 10px;
  }

  .dept-row-head strong {
    min-width: 0;
    flex: 1;
    overflow: hidden;
    color: #edf5ff;
    font-size: 0.88rem;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .dept-row-head em {
    color: #8ef5e4;
    font-size: 0.72rem;
    font-style: normal;
  }

  .dept-meter {
    height: 7px;
    overflow: hidden;
    border-radius: 999px;
    background: rgba(72, 86, 113, 0.28);
  }

  .dept-meter span {
    display: block;
    height: 100%;
    border-radius: inherit;
    background: linear-gradient(90deg, #54cab6, #84d3ff);
  }

  .dept-mix {
    display: flex;
    flex-wrap: wrap;
    gap: 7px;
  }

  .dept-mix span {
    border: 1px solid rgba(72, 86, 113, 0.24);
    border-radius: 6px;
    padding: 5px 7px;
    color: #9aa8bb;
    background: rgba(12, 18, 31, 0.48);
    font-size: 0.66rem;
  }

  .state-msg {
    border: 1px solid rgba(83, 99, 130, 0.22);
    border-radius: 8px;
    padding: 60px 18px;
    color: #758399;
    background: rgba(10, 16, 29, 0.44);
    font-size: 0.85rem;
    text-align: center;
  }

  .empty-msg {
    border: 1px dashed rgba(83, 99, 130, 0.24);
    border-radius: 8px;
    padding: 24px 12px;
    color: #64748b;
    font-size: 0.75rem;
    text-align: center;
  }

  .error-msg {
    color: #fca5a5;
  }

  .text-rose {
    color: #f87171;
  }

  .text-teal {
    color: #54cab6;
  }

  .text-amber {
    color: #f5c970;
  }

  @keyframes shimmer {
    0% { background-position: 180% 0; }
    100% { background-position: -180% 0; }
  }

  @media (prefers-reduced-motion: reduce) {
    .report-mode-btn,
    .refresh-btn,
    .report-select-trigger,
    .member-performance-table tbody tr {
      transition: none;
    }

    .skeleton-line {
      animation: none;
    }
  }

  @media (max-width: 1180px) {
    .kpi-command-layout,
    .kpi-analytics-layout {
      grid-template-columns: 1fr;
    }

    .brief-metric-grid {
      grid-template-columns: repeat(3, minmax(0, 1fr));
    }
  }

  @media (max-width: 860px) {
    .kpi-header,
    .report-header,
    .panel-header,
    .brief-header-row {
      align-items: stretch;
    }

    .report-mode-switch,
    .report-controls,
    .header-action-stack {
      width: 100%;
    }

    .header-action-stack {
      justify-items: stretch;
    }

    .report-overview-strip,
    .report-grid,
    .skeleton-grid {
      grid-template-columns: 1fr;
    }

    .report-select-container,
    .refresh-btn {
      width: 100%;
    }

    .score-focus-panel {
      grid-template-columns: 1fr;
    }

    .score-ring {
      width: 104px;
      height: 104px;
    }

    .insight-grid {
      grid-template-columns: repeat(2, minmax(0, 1fr));
    }

    .distribution-foot {
      flex-wrap: wrap;
      justify-content: flex-start;
    }
  }

  @media (max-width: 640px) {
    .report-brief-panel,
    .score-focus-panel,
    .report-panel,
    .member-matrix-panel,
    .department-panel,
    .diagnostic-panel,
    .breadth-panel {
      padding: 16px;
    }

    .brief-metric-grid,
    .report-mode-switch,
    .dept-performance-list,
    .insight-grid {
      grid-template-columns: 1fr;
    }

    .score-band-grid {
      grid-template-columns: repeat(2, minmax(0, 1fr));
    }

    .metric-tile {
      min-height: 82px;
    }

    .evidence-row {
      grid-template-columns: 1fr;
    }
  }

  /* Phase 41 light admin console calibration */
  .kpi-dashboard {
    display: grid;
    grid-template-columns: minmax(0, 1fr) minmax(360px, 420px);
    grid-template-areas:
      "header header"
      "brief focus"
      "segments report"
      "matrix report"
      "diagnostic report"
      "breadth dept";
    gap: var(--wa-space-4);
    margin-top: 0;
    color: var(--wa-text-main);
    font-family: var(--wa-font-sans);
  }

  .kpi-command-layout,
  .kpi-analytics-layout {
    display: contents;
  }

  .kpi-header {
    grid-area: header;
    align-items: center;
    min-height: 58px;
    border: 1px solid var(--wa-border-soft);
    border-radius: var(--wa-radius-xl);
    padding: 10px 12px;
    background: rgba(255, 255, 255, 0.82);
    box-shadow: var(--wa-shadow-sm);
  }

  .report-brief-panel {
    grid-area: brief;
  }

  .score-focus-panel {
    grid-area: focus;
  }

  .kpi-status-strip {
    grid-area: segments;
  }

  .member-matrix-panel {
    grid-area: matrix;
  }

  .report-panel {
    grid-area: report;
  }

  .diagnostic-panel {
    grid-area: diagnostic;
  }

  .breadth-panel {
    grid-area: breadth;
  }

  .department-panel {
    grid-area: dept;
  }

  .kpi-title-block {
    max-width: 680px;
  }

  .eyebrow,
  .panel-kicker,
  .inspector-kicker {
    color: var(--wa-text-muted);
    font-size: 12px;
    font-weight: 760;
    letter-spacing: 0;
    text-transform: none;
  }

  .kpi-header h2,
  .brief-header-row h3,
  .score-focus-copy h3,
  .panel-title-row h3,
  .report-block-header h4,
  .member-inspector-card h3,
  .inspector-section h4 {
    color: var(--wa-text-strong);
  }

  .kpi-header h2 {
    margin: 3px 0 0;
    font-size: 18px;
    line-height: 1.15;
  }

  .kpi-title-block p {
    display: none;
  }

  .brief-header-row p,
  .score-focus-copy p,
  .panel-subtitle,
  .report-summary span,
  .compact-row p,
  .personal-focus p,
  .inspector-section p {
    color: var(--wa-text-muted);
  }

  .glass-panel {
    border: 1px solid var(--wa-border-soft);
    border-radius: var(--wa-radius-xl);
    background: var(--wa-chrome-1);
    box-shadow: var(--wa-shadow-panel);
    backdrop-filter: blur(16px) saturate(124%);
    -webkit-backdrop-filter: blur(16px) saturate(124%);
  }

  .report-brief-panel,
  .score-focus-panel,
  .report-panel,
  .member-matrix-panel,
  .department-panel,
  .diagnostic-panel,
  .breadth-panel {
    padding: var(--wa-space-4);
  }

  .report-brief-panel {
    background: transparent;
    border: 0;
    box-shadow: none;
    padding: 0;
  }

  .brief-header-row {
    margin-bottom: var(--wa-space-3);
  }

  .brief-meta {
    border-color: var(--wa-border-soft);
    background: var(--wa-surface-lift);
  }

  .brief-meta span,
  .metric-tile span,
  .report-overview-strip span,
  .row-meta {
    color: var(--wa-text-muted);
  }

  .brief-meta strong,
  .metric-tile strong,
  .report-overview-strip strong,
  .distribution-title strong,
  .distribution-foot strong,
  .focus-title strong,
  .compact-row strong,
  .report-summary strong {
    color: var(--wa-text-strong);
  }

  .brief-metric-grid {
    grid-template-columns: repeat(4, minmax(0, 1fr));
  }

  .metric-tile {
    min-height: 112px;
    border-color: var(--wa-border-soft);
    background: var(--wa-chrome-1);
  }

  .metric-tile strong {
    margin-top: 2px;
    font-size: 32px;
  }

  .metric-tile em {
    color: var(--wa-text-muted);
    line-height: 1.35;
  }

  .metric-info {
    box-shadow: inset 4px 0 0 var(--wa-info), var(--wa-shadow-panel);
  }

  .metric-success {
    box-shadow: inset 4px 0 0 var(--wa-success), var(--wa-shadow-panel);
  }

  .metric-warning {
    box-shadow: inset 4px 0 0 var(--wa-warning), var(--wa-shadow-panel);
  }

  .metric-danger {
    box-shadow: inset 4px 0 0 var(--wa-danger), var(--wa-shadow-panel);
  }

  .report-mode-switch {
    background: rgba(255, 255, 255, 0.72);
    border-color: var(--wa-border-soft);
    border-radius: var(--wa-radius-lg);
  }

  .report-mode-btn {
    border-radius: var(--wa-radius-md);
    color: var(--wa-text-main);
  }

  .report-mode-btn span {
    color: var(--wa-text-muted);
  }

  .report-mode-btn:hover,
  .report-mode-btn.is-active {
    border-color: var(--wa-border-focus);
    background: var(--wa-accent-soft);
    color: var(--wa-accent-strong);
  }

  .report-mode-btn.is-active span {
    color: var(--wa-accent-strong);
  }

  .sync-indicator {
    border-color: var(--wa-border-soft);
    color: var(--wa-accent-strong);
    background: var(--wa-accent-soft);
  }

  .score-focus-panel {
    grid-template-columns: 104px minmax(0, 1fr);
    min-height: 152px;
    align-items: center;
  }

  .score-ring,
  .score-ring.tone-good,
  .score-ring.tone-watch,
  .score-ring.tone-risk {
    width: 96px;
    height: 96px;
    border-color: var(--wa-border-soft);
    border-radius: var(--wa-radius-xl);
    background: var(--wa-surface-flat);
    box-shadow: inset 0 0 0 7px var(--wa-accent-soft);
  }

  .score-ring strong {
    color: var(--wa-text-strong);
    font-size: 30px;
  }

  .score-ring span {
    color: var(--wa-text-muted);
  }

  .focus-stat-row span {
    border-color: var(--wa-border-soft);
    color: var(--wa-text-main);
    background: var(--wa-surface-inset);
  }

  .kpi-status-strip {
    display: grid;
    grid-template-columns: repeat(5, minmax(0, 1fr));
    gap: var(--wa-space-3);
  }

  .status-segment {
    min-height: 78px;
    padding: var(--wa-space-3);
    display: grid;
    align-content: space-between;
    gap: 4px;
  }

  .status-segment span,
  .status-segment small {
    color: var(--wa-text-muted);
    font-size: 12px;
  }

  .status-segment strong {
    color: var(--wa-text-strong);
    font-size: 22px;
    line-height: 1;
    font-variant-numeric: tabular-nums;
  }

  .report-panel {
    position: sticky;
    top: var(--wa-space-4);
    max-height: calc(100vh - 32px);
    overflow: auto;
    align-content: start;
    scrollbar-width: thin;
    scrollbar-color: rgba(121, 139, 159, 0.32) transparent;
  }

  .report-header {
    gap: var(--wa-space-3);
  }

  .report-controls {
    width: 100%;
    justify-content: space-between;
  }

  .report-select-container {
    flex: 1 1 160px;
    width: auto;
  }

  .report-select-trigger {
    border-color: var(--wa-border-soft);
    color: var(--wa-text-main);
    background: rgba(255, 255, 255, 0.82);
  }

  .report-select-trigger:hover,
  .report-select-trigger:focus {
    border-color: var(--wa-border-focus);
    background: #ffffff;
  }

  .report-select-arrow {
    color: var(--wa-accent);
  }

  .report-select-options {
    border-color: var(--wa-border-soft);
    background: #ffffff;
    box-shadow: var(--wa-shadow-md);
  }

  .report-select-option {
    color: var(--wa-text-main);
  }

  .report-select-option:hover,
  .report-select-option.active {
    color: var(--wa-accent-strong);
    background: var(--wa-accent-soft);
  }

  .member-inspector-card {
    display: grid;
    gap: var(--wa-space-3);
    border-top: 1px solid var(--wa-border-soft);
    padding-top: var(--wa-space-4);
  }

  .inspector-head {
    display: flex;
    align-items: flex-start;
    justify-content: space-between;
    gap: var(--wa-space-3);
  }

  .inspector-head h3 {
    margin: 4px 0 0;
    font-size: 18px;
  }

  .fact-grid {
    display: grid;
    grid-template-columns: repeat(2, minmax(0, 1fr));
    gap: var(--wa-space-2);
    margin: 0;
  }

  .fact-grid div {
    min-width: 0;
    border: 1px solid var(--wa-border-soft);
    border-radius: var(--wa-radius-md);
    padding: 9px 10px;
    background: var(--wa-surface-inset);
  }

  .fact-grid dt {
    color: var(--wa-text-muted);
    font-size: 12px;
  }

  .fact-grid dd {
    margin: 4px 0 0;
    color: var(--wa-text-strong);
    font-size: 13px;
    font-weight: 760;
  }

  .inspector-section {
    border-top: 1px solid rgba(121, 139, 159, 0.16);
    padding-top: var(--wa-space-3);
  }

  .inspector-section h4 {
    margin: 0 0 8px;
    font-size: 14px;
  }

  .inspector-section ul {
    display: grid;
    gap: 7px;
    margin: 0;
    padding: 0;
    list-style: none;
  }

  .inspector-section li {
    position: relative;
    padding-left: 16px;
    color: var(--wa-text-main);
    font-size: 13px;
    line-height: 1.45;
  }

  .inspector-section li::before {
    content: "";
    position: absolute;
    left: 0;
    top: 0.62em;
    width: 6px;
    height: 6px;
    border-radius: 50%;
    background: var(--wa-accent);
  }

  .inspector-actions {
    display: flex;
    flex-wrap: wrap;
    gap: var(--wa-space-2);
  }

  .report-overview-strip {
    grid-template-columns: 1fr;
  }

  .report-overview-strip div,
  .report-block,
  .distribution-block,
  .insight-card,
  .dept-performance-row {
    border-color: var(--wa-border-soft);
    background: var(--wa-surface-inset);
  }

  .report-grid {
    grid-template-columns: 1fr;
  }

  .report-block-header {
    border-color: rgba(121, 139, 159, 0.18);
  }

  .focus-title span {
    color: var(--wa-accent-strong);
  }

  .risk-note-list span {
    border-color: rgba(216, 135, 0, 0.24);
    color: var(--wa-warning);
    background: var(--wa-warning-soft);
  }

  .risk-pill.high {
    border-color: rgba(221, 75, 62, 0.24);
    color: var(--wa-danger);
    background: var(--wa-danger-soft);
  }

  .risk-pill.medium {
    border-color: rgba(216, 135, 0, 0.24);
    color: var(--wa-warning);
    background: var(--wa-warning-soft);
  }

  .risk-pill.low,
  .focus-tag {
    border-color: rgba(37, 107, 216, 0.22);
    color: var(--wa-info);
    background: var(--wa-info-soft);
  }

  .evidence-row {
    grid-template-columns: minmax(86px, 0.72fr) minmax(0, 1.4fr);
    border-color: rgba(121, 139, 159, 0.16);
  }

  .evidence-id {
    border-color: rgba(0, 143, 150, 0.22);
    color: var(--wa-accent-strong);
    background: var(--wa-accent-soft);
  }

  .evidence-title {
    color: var(--wa-text-main);
  }

  .member-matrix-panel {
    min-height: 0;
    gap: var(--wa-space-3);
    background: transparent;
    border: 0;
    box-shadow: none;
    padding: 0;
  }

  .member-table-wrap {
    min-height: 420px;
    max-height: 620px;
    border-color: var(--wa-border-soft);
    background: rgba(255, 255, 255, 0.84);
    box-shadow: var(--wa-shadow-panel);
  }

  .member-performance-table th {
    height: 42px;
    border-color: rgba(121, 139, 159, 0.18);
    color: var(--wa-text-muted);
    background: rgba(248, 251, 254, 0.96);
    text-transform: none;
  }

  .member-performance-table td {
    border-color: rgba(121, 139, 159, 0.14);
  }

  .member-performance-table tbody tr {
    background: rgba(255, 255, 255, 0.66);
  }

  .member-performance-table tbody tr:hover {
    background: var(--wa-row-hover);
  }

  .member-performance-table tbody tr.is-selected {
    background: var(--wa-row-active);
    box-shadow: inset 3px 0 0 var(--wa-accent);
  }

  .cell-right,
  .numeric {
    text-align: right;
  }

  .member-select-btn {
    display: block;
    max-width: 146px;
    overflow: hidden;
    border: 0;
    padding: 0;
    color: var(--wa-text-strong);
    background: transparent;
    font: inherit;
    font-size: 13px;
    font-weight: 760;
    text-align: left;
    text-overflow: ellipsis;
    white-space: nowrap;
    cursor: pointer;
  }

  .member-select-btn:hover,
  .member-select-btn:focus-visible {
    color: var(--wa-accent-strong);
    outline: none;
  }

  .member-identity strong,
  .dept-row-head strong {
    color: var(--wa-text-strong);
  }

  .member-identity span,
  .dept-mix span,
  .score-cell span,
  .risk-stack span,
  .score-band span,
  .score-band em,
  .distribution-title span,
  .distribution-foot span {
    color: var(--wa-text-muted);
  }

  .member-avatar {
    border-color: rgba(0, 143, 150, 0.2);
  }

  .avatar-placeholder {
    border-color: rgba(0, 143, 150, 0.22);
    color: var(--wa-accent-strong);
    background: var(--wa-accent-soft);
  }

  .rank-badge {
    border-color: var(--wa-border-soft);
    color: var(--wa-text-muted);
    background: var(--wa-surface-inset);
  }

  .issue-count-grid span,
  .issue-count-grid strong,
  .flow-stat-stack span,
  .score-part,
  .risk-stack,
  .dept-mix span,
  .score-band {
    border-color: var(--wa-border-soft);
    color: var(--wa-text-main);
    background: var(--wa-surface-inset);
  }

  .issue-count-grid span b,
  .risk-stack strong,
  .score-band strong {
    color: var(--wa-text-strong);
  }

  .issue-count-grid strong {
    color: var(--wa-accent-strong);
    background: var(--wa-accent-soft);
  }

  .risk-stack.is-risk {
    border-color: rgba(216, 135, 0, 0.24);
    background: var(--wa-warning-soft);
  }

  .risk-stack.is-risk strong {
    color: var(--wa-warning);
  }

  .panel-count {
    border-color: rgba(0, 143, 150, 0.22);
    color: var(--wa-accent-strong);
    background: var(--wa-accent-soft);
  }

  .insight-head span,
  .insight-card p {
    color: var(--wa-text-muted);
  }

  .insight-head strong {
    color: var(--wa-text-strong);
  }

  .distribution-foot {
    border-color: rgba(0, 143, 150, 0.18);
    background: var(--wa-accent-soft);
  }

  .dept-meter {
    background: rgba(121, 139, 159, 0.18);
  }

  .dept-meter span {
    background: var(--wa-accent);
  }

  .state-msg,
  .empty-msg {
    border-color: var(--wa-border-soft);
    color: var(--wa-text-muted);
    background: var(--wa-surface-inset);
  }

  .inline-error {
    border-color: rgba(221, 75, 62, 0.24);
    color: var(--wa-danger);
    background: var(--wa-danger-soft);
  }

  .text-rose {
    color: var(--wa-danger);
  }

  .text-teal {
    color: var(--wa-accent-strong);
  }

  .text-amber {
    color: var(--wa-warning);
  }

  .skeleton-line {
    background: linear-gradient(90deg, rgba(226, 234, 243, 0.72), rgba(246, 249, 252, 0.98), rgba(226, 234, 243, 0.72));
    background-size: 180% 100%;
  }

  @media (max-width: 1280px) {
    .kpi-dashboard {
      grid-template-columns: 1fr;
      grid-template-areas:
        "header"
        "brief"
        "focus"
        "segments"
        "report"
        "matrix"
        "diagnostic"
        "breadth"
        "dept";
    }

    .report-panel {
      position: static;
      max-height: none;
    }
  }

  @media (max-width: 860px) {
    .brief-metric-grid,
    .kpi-status-strip {
      grid-template-columns: repeat(2, minmax(0, 1fr));
    }
  }

  @media (max-width: 640px) {
    .kpi-dashboard {
      gap: var(--wa-space-3);
    }

    .kpi-header,
    .score-focus-panel,
    .report-panel,
    .department-panel,
    .diagnostic-panel,
    .breadth-panel {
      padding: var(--wa-space-3);
    }

    .brief-metric-grid,
    .kpi-status-strip,
    .fact-grid {
      grid-template-columns: 1fr;
    }
  }
</style>
