<script lang="ts">
  /* finesse · register=product · queue-inspector · SOUL=4 SPECTACLE=1 DENSITY=7 */
  import { onMount, tick } from "svelte";
  import AdminDataList from "./admin-console/AdminDataList.svelte";
  import AdminListFilterBar from "./admin-console/AdminListFilterBar.svelte";
  import type {
    AdminCellValue,
    AdminTableColumn,
    AdminTableRow,
    AdminTone,
  } from "../lib/admin-console/contract";
  import Switch from "./shared/Switch.svelte";
  import Modal from "./shared/Modal.svelte";
  import MarkdownWorkbench from "./shared/MarkdownWorkbench.svelte";
  import Select from "./shared/Select.svelte";
  import "../styles/code-review.css";
  export let currentUserPermissions: string[] = [];
  type Policy = {
    project_id: string;
    sync_commits: boolean;
    sync_mrs: boolean;
    domain: string;
    scenario: string;
    knowledge_scope: string;
    rules: string;
  };
  type Repo = { project_id: string; name: string; policy: Policy };
  type Run = {
    id: number;
    project_id: string;
    repo: string;
    author: string;
    updated_at: string;
    kind: string;
    ref: string;
    head_sha: string;
    base_sha: string;
    title: string;
    url: string;
    status: string;
    phase: string;
    error: string;
    publish_status: string;
    publish_error: string;
    report_json: string;
    policy_json: string;
    snapshot_json: string;
    model: string;
    prompt_version: string;
    skill_version_id: number;
    skill_version: number;
    skill_scope_type: string;
    skill_scope_id: string;
    skill_name: string;
    skill_hash: string;
    retry_of_id: number;
    created_at: string;
  };
  type Finding = {
    dimension: string;
    severity: string;
    title: string;
    file: string;
    line: number;
    evidence: string;
    impact: string;
    suggestion: string;
    verification: string;
    knowledge_ids: number[];
  };
  type Report = {
    summary: string;
    scenario: string;
    validation: string;
    findings: Finding[];
    assessments: { dimension: string; analysis: string }[];
    questions: string[];
  };
  type Snapshot = {
    complete: boolean;
    gaps: string[];
    files: { new_path: string; diff: string; source: string }[];
    knowledge: {
      id: number;
      version: number;
      scope: string;
      source: string;
      summary: string;
      content: string;
    }[];
  };
  type ReviewPane = "review" | "facts" | "code" | "knowledge" | "rules";
  type RefreshMode = "initial" | "manual" | "poll";
  type ReviewAction = "cancel" | "sync" | "retry";
  type PolicyField = keyof Policy | "policy";
  type ReviewRow = AdminTableRow & { source: Run };
  type DetailModalHandle = {
    scrollToTop: (behavior?: ScrollBehavior) => void;
    focusDialog: () => void;
  };
  type PolicyFeedback = { message: string; error: boolean };
  type ListAnchor = {
    rowID: string;
    offset: number;
    owner: "list" | "page";
    scrollTop: number;
  };
  type QueuedRefresh = {
    initial: boolean;
    mode: "initial" | "manual";
    resolvers: Array<(value: boolean) => void>;
  };
  const reviewColumns: AdminTableColumn[] = [
    { key: "change", label: "变更", width: "54%", priority: "essential" },
    { key: "status", label: "评审状态", width: "120px", priority: "essential" },
    { key: "sync", label: "评论同步", width: "142px", priority: "secondary" },
    { key: "received", label: "接收时间", width: "136px", priority: "secondary" },
  ];
  const paneLabels: Record<ReviewPane, string> = {
    review: "评审正文",
    facts: "事实依据",
    code: "代码依据",
    knowledge: "知识依据",
    rules: "规则依据",
  };
  const reviewPanes: ReviewPane[] = [
    "review",
    "facts",
    "code",
    "knowledge",
    "rules",
  ];
  const dimensions: Record<string, string> = {
    business: "业务与需求",
    robustness: "健壮性",
    reusability: "可复用性",
    abstraction: "抽象设计",
    encapsulation: "封装与职责",
    concurrency: "并发一致性",
    security: "安全权限",
    performance: "性能与资源",
    testing: "测试与可测性",
    delivery: "交付与运维",
  };
  const statusNames: Record<string, string> = {
    queued: "等待评审",
    running: "评审中",
    completed: "评审完成",
    partial: "部分评审",
    failed: "评审失败",
    cancelled: "已取消",
  };
  const syncNames: Record<string, string> = {
    off: "已关闭",
    waiting_review: "评审后同步",
    pending: "待同步",
    publishing: "同步中",
    published: "已同步",
    stale: "版本已变化",
    blocked: "同步受阻",
    unknown: "待核对",
    sync_failed: "同步失败",
  };
  let repos: Repo[] = [];
  let defaultRules: string[] = [];
  let ruleDrafts: Record<string, string> = {};
  let rulesOpen = false;
  $: ruleDraft = ruleDrafts[project] ?? repo?.policy.rules ?? "";
  $: rulesChanged = ruleDraft !== (repo?.policy.rules ?? "");
  $: reviewedPolicy = parse<Policy>(selected?.policy_json);
  let scenarios: Record<string, string> = {};
  let runs: Run[] = [];
  let project = "";
  let filter = "";
  $: normalizedFilter = filter.trim().toLowerCase();
  let policySaving: PolicyField | "" = "";
  let policyFeedback: Record<string, PolicyFeedback> = {};
  let actionKind: ReviewAction | "" = "";
  let loading = true;
  let refreshing = false;
  let listError = "";
  let policyError = "";
  let pageNotice = "";
  let selected: Run | null = null;
  let detailLoading = false;
  let detailError = "";
  let actionError = "";
  let actionNotice = "";
  let detailModal: DetailModalHandle | null = null;
  let pane: ReviewPane = "review";
  let requestVersion = 0;
  let listRequestVersion = 0;
  let actionGeneration = 0;
  let pollBusy = false;
  let queuedRefresh: QueuedRefresh | null = null;
  let disposed = false;
  $: canReadEvidence =
    currentUserPermissions.includes("ai_context:preview") ||
    currentUserPermissions.includes("*");
  $: canWrite =
    currentUserPermissions.includes("config:write") ||
    currentUserPermissions.includes("*");
  $: repo = repos.find((r) => r.project_id === project);
  $: selectedRepo = selected
    ? repos.find((r) => r.project_id === selected?.project_id)
    : undefined;
  $: selectedSyncEnabled = Boolean(
    selected &&
      selectedRepo &&
      (selected.kind === "mr"
        ? selectedRepo.policy.sync_mrs
        : selectedRepo.policy.sync_commits),
  );
  $: canManualSync = Boolean(
    canWrite &&
      selected?.status === "completed" &&
      selectedSyncEnabled &&
      !["published", "publishing", "stale", "blocked"].includes(
        selected?.publish_status || "",
      ),
  );
  $: visibleRuns = runs.filter(
    (r) =>
      (!project || r.project_id === project) &&
      (!normalizedFilter ||
        `${r.title} ${r.ref} ${r.repo} ${r.author} ${r.head_sha} ${r.base_sha}`
          .toLowerCase()
          .includes(normalizedFilter)),
  );
  $: reviewRows = visibleRuns.map(toReviewRow);
  $: report = parse<Report>(selected?.report_json);
  $: snapshot = parse<Snapshot>(selected?.snapshot_json);
  function parse<T>(s?: string): T | null {
    try {
      return s ? (JSON.parse(s) as T) : null;
    } catch {
      return null;
    }
  }
  function safeURL(s: string) {
    try {
      const u = new URL(s);
      return ["https:", "http:"].includes(u.protocol) ? u.href : "";
    } catch {
      return "";
    }
  }
  async function api<T>(path: string, init: RequestInit = {}): Promise<T> {
    const response = await fetch(path, init);
    if (!response.ok)
      throw new Error(
        (await response.text()).trim() || `请求失败 (${response.status})`,
      );
    if (response.status === 204) return undefined as T;
    return response.json();
  }
  async function refreshReposSilently() {
    const version = ++listRequestVersion;
    const data = await api<{
      repos: Repo[];
      scenarios: Record<string, string>;
      default_rules: string[];
    }>("/api/code-reviews/repos");
    if (disposed || version !== listRequestVersion) return;
    repos = data.repos;
    scenarios = data.scenarios;
    defaultRules = data.default_rules || [];
  }
  function detailNeedsRefresh(previous: Run, current: Run) {
    const mutable = ["queued", "running"];
    return (
      mutable.includes(previous.status) ||
      mutable.includes(current.status) ||
      (["completed", "partial"].includes(current.status) &&
        !previous.report_json)
    );
  }
  function runSummaryChanged(previous: Run, current: Run) {
    return [
      "project_id",
      "repo",
      "author",
      "updated_at",
      "kind",
      "ref",
      "head_sha",
      "base_sha",
      "title",
      "url",
      "status",
      "phase",
      "error",
      "publish_status",
      "publish_error",
    ].some((key) => previous[key as keyof Run] !== current[key as keyof Run]);
  }
  function runListChanged(previous: Run[], current: Run[]) {
    return (
      previous.length !== current.length ||
      current.some((run, index) => !previous[index] || runSummaryChanged(previous[index], run))
    );
  }
  function captureListAnchor(): ListAnchor | null {
    const list = document.getElementById("code-review-list-content");
    const page = document.querySelector<HTMLElement>(".cr-workbench");
    if (!list || !page) return null;
    const owner = list.scrollHeight > list.clientHeight + 1 ? list : page;
    if (owner.scrollTop <= 0) return null;
    const ownerTop = owner.getBoundingClientRect().top;
    const row = Array.from(list.querySelectorAll<HTMLElement>("tr.data-row")).find(
      (item) => item.getBoundingClientRect().bottom > ownerTop,
    );
    if (!row?.dataset.rowId) return null;
    return {
      rowID: row.dataset.rowId,
      offset: row.getBoundingClientRect().top - ownerTop,
      owner: owner === list ? "list" : "page",
      scrollTop: owner.scrollTop,
    };
  }
  async function restoreListAnchor(anchor: ListAnchor | null) {
    if (!anchor) return;
    await tick();
    const list = document.getElementById("code-review-list-content");
    const page = document.querySelector<HTMLElement>(".cr-workbench");
    const row = Array.from(
      list?.querySelectorAll<HTMLElement>("tr.data-row") || [],
    ).find((item) => item.dataset.rowId === anchor.rowID);
    const owner = anchor.owner === "list" ? list : page;
    if (!owner || !row) return;
    if (Math.abs(owner.scrollTop - anchor.scrollTop) > 1) return;
    owner.scrollTop += row.getBoundingClientRect().top -
      owner.getBoundingClientRect().top -
      anchor.offset;
  }
  async function refresh(
    initial = false,
    mode: RefreshMode = initial ? "initial" : "manual",
  ) {
    if (pollBusy) {
      if (mode === "poll") return false;
      listError = "";
      if (mode === "initial" && runs.length === 0) loading = true;
      else refreshing = true;
      return new Promise<boolean>((resolve) => {
        if (queuedRefresh) {
          queuedRefresh.initial = queuedRefresh.initial || initial;
          if (mode === "initial") queuedRefresh.mode = "initial";
          queuedRefresh.resolvers.push(resolve);
        } else {
          queuedRefresh = {
            initial,
            mode,
            resolvers: [resolve],
          };
        }
      });
    }
    pollBusy = true;
    const version = ++listRequestVersion;
    const visibleRefresh = mode !== "poll";
    const anchor = mode === "poll" ? captureListAnchor() : null;
    if (visibleRefresh) {
      listError = "";
      if (mode === "initial" && runs.length === 0) loading = true;
      else refreshing = true;
    }
    try {
      if (initial) {
        const data = await api<{
          repos: Repo[];
          scenarios: Record<string, string>;
          default_rules: string[];
        }>("/api/code-reviews/repos");
        if (disposed || version !== listRequestVersion) return false;
        repos = data.repos;
        scenarios = data.scenarios;
        defaultRules = data.default_rules || [];
      }
      const data = await api<{ items: Run[] }>("/api/code-reviews");
      if (disposed || version !== listRequestVersion) return false;
      const previousSelected = selected;
      const nextRuns = data.items;
      if (runListChanged(runs, nextRuns)) {
        runs = nextRuns;
        await restoreListAnchor(anchor);
      }
      if (previousSelected && selected?.id === previousSelected.id) {
        const summary = nextRuns.find((run) => run.id === previousSelected.id);
        if (summary) {
          const shouldRefreshDetail = detailNeedsRefresh(previousSelected, summary);
          if (shouldRefreshDetail && canReadEvidence) {
            try {
              const detail = await api<Run>(`/api/code-reviews/${summary.id}`);
              if (
                !disposed &&
                version === listRequestVersion &&
                selected?.id === previousSelected.id
              ) {
                selected = detail;
                detailError = "";
              }
            } catch {
              // Silent list polling must retain the last complete detail surface.
            }
          } else if (runSummaryChanged(previousSelected, summary)) {
            selected = {
              ...previousSelected,
              ...summary,
              report_json: previousSelected.report_json,
              policy_json: previousSelected.policy_json,
              snapshot_json: previousSelected.snapshot_json,
            };
          }
        }
      }
      return true;
    } catch (e) {
      if (visibleRefresh) listError = (e as Error).message;
      return false;
    } finally {
      if (visibleRefresh) {
        loading = false;
        refreshing = false;
      }
      pollBusy = false;
      const queued = queuedRefresh;
      queuedRefresh = null;
      if (queued && !disposed) {
        void refresh(queued.initial, queued.mode).then((value) => {
          queued.resolvers.forEach((resolve) => resolve(value));
        });
      }
    }
  }
  async function selectRun(id: number, showLoading = true, resetView = true) {
    const summary = runs.find((r) => r.id === id);
    if (resetView) {
      actionGeneration++;
      actionKind = "";
      selected = summary || selected;
      pane = "review";
      detailError = "";
      actionError = "";
      actionNotice = "";
      await tick();
      detailModal?.scrollToTop();
    }
    if (!canReadEvidence) {
      detailLoading = false;
      return;
    }
    const version = ++requestVersion;
    if (showLoading) {
      detailLoading = true;
      detailError = "";
    }
    try {
      const data = await api<Run>(`/api/code-reviews/${id}`);
      if (!disposed && version === requestVersion) {
        selected = data;
        detailError = "";
      }
    } catch (e) {
      if (version === requestVersion && showLoading) {
        detailError = (e as Error).message;
      }
    } finally {
      if (version === requestVersion) detailLoading = false;
    }
  }
  async function savePolicy(change: Partial<Policy>) {
    if (!repo || !canWrite || policySaving) return;
    // A policy mutation supersedes any repository snapshot already in flight.
    listRequestVersion++;
    const activeRepo = repo;
    const previousPolicy = { ...activeRepo.policy };
    const nextPolicy = { ...previousPolicy, ...change };
    const saveField = (Object.keys(change)[0] as keyof Policy | undefined) || "policy";
    policySaving = saveField;
    policyError = "";
    pageNotice = "";
    policyFeedback = { ...policyFeedback, [saveField]: { message: "", error: false } };
    repos = repos.map((item) =>
      item.project_id === activeRepo.project_id
        ? { ...item, policy: nextPolicy }
        : item,
    );
    try {
      const p = await api<Policy>("/api/code-reviews/policy", {
        method: "PUT",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          project_id: activeRepo.project_id,
          ...change,
        }),
      });
      repos = repos.map((r) =>
        r.project_id === p.project_id ? { ...r, policy: p } : r,
      );
      if (change.rules !== undefined) {
        ruleDrafts = { ...ruleDrafts, [p.project_id]: p.rules || "" };
        pageNotice = "评审规则已保存，将用于后续新事件；历史报告保留原规则。";
      } else if (change.sync_commits !== undefined) {
        policyFeedback = {
          ...policyFeedback,
          sync_commits: {
            message: `Commit 自动同步已${p.sync_commits ? "开启" : "关闭"}`,
            error: false,
          },
        };
      } else if (change.sync_mrs !== undefined) {
        policyFeedback = {
          ...policyFeedback,
          sync_mrs: {
            message: `MR 自动同步已${p.sync_mrs ? "开启" : "关闭"}`,
            error: false,
          },
        };
      } else {
        pageNotice = "仓库评审策略已保存。";
      }
    } catch (e) {
      const message = (e as Error).message;
      if (saveField === "sync_commits" || saveField === "sync_mrs") {
        policyFeedback = {
          ...policyFeedback,
          [saveField]: { message, error: true },
        };
      } else {
        policyError = message;
      }
      repos = repos.map((item) =>
        item.project_id === activeRepo.project_id
          ? { ...item, policy: previousPolicy }
          : item,
      );
    } finally {
      policySaving = "";
    }
  }
  async function runAction(action: ReviewAction) {
    if (!selected || actionKind) return;
    const origin = selected;
    const generation = ++actionGeneration;
    actionKind = action;
    actionError = "";
    actionNotice = "";
    try {
      const updated = await api<Run>(`/api/code-reviews/${origin.id}/${action}`, {
        method: "POST",
      });
      listRequestVersion++;
      if (action === "retry") {
        runs = [updated, ...runs.filter((run) => run.id !== updated.id)];
      } else {
        runs = runs.map((run) => (run.id === updated.id ? updated : run));
      }
      if (generation === actionGeneration && selected?.id === origin.id) {
        requestVersion++;
        selected =
          action === "retry"
            ? updated
            : {
                ...origin,
                ...updated,
                report_json: origin.report_json,
                policy_json: origin.policy_json,
                snapshot_json: origin.snapshot_json,
              };
        pane = "review";
        detailError = "";
        actionNotice =
          action === "retry"
            ? "已重新加入评审队列；原失败记录保留为历史。"
            : action === "cancel"
            ? "已取消评审；不会发布评论。"
            : "评论同步状态已更新。";
        await tick();
        detailModal?.scrollToTop();
        detailModal?.focusDialog();
      }
    } catch (e) {
      if (action === "sync") {
        try {
          await refreshReposSilently();
        } catch {
          // Keep the action error authoritative if policy readback also fails.
        }
      }
      if (generation === actionGeneration && selected?.id === origin.id) {
        actionError = (e as Error).message;
      }
    } finally {
      if (generation === actionGeneration) actionKind = "";
    }
  }
  function codeBlock(value: string, language = "") {
    const fence = "`".repeat(Math.max(3, ...Array.from(value.matchAll(/`+/g), m => m[0].length + 1)));
    return `${fence}${language}\n${value}\n${fence}`;
  }
  function plainText(value: string) { return value.replace(/[\\`*_{}[\]<>#|]/g, "\\$&").replace(/\r?\n/g, " "); }
  function conciseReviewError(value: string) {
    return value.replace(/[；;，,]?\s*请重新评审[。.!！]?$/, "").trim();
  }
  function markdownURL(value: string) {
    return safeURL(value).replace(/\)/g, "%29");
  }
  function statusTone(status: string): AdminTone {
    if (status === "completed") return "success";
    if (status === "running") return "info";
    if (status === "queued" || status === "partial") return "warning";
    if (status === "failed") return "danger";
    return "neutral";
  }
  function syncTone(status: string): AdminTone {
    if (status === "published") return "success";
    if (["publishing", "pending", "waiting_review"].includes(status)) return "info";
    if (["blocked", "stale", "unknown"].includes(status)) return "warning";
    if (status === "sync_failed") return "danger";
    return "neutral";
  }
  function toneClass(tone: AdminTone) {
    return `tone-${tone}`;
  }
  function formatReceived(value: string) {
    const date = new Date(value);
    if (Number.isNaN(date.getTime())) return value || "未记录";
    return date.toLocaleString("zh-CN", {
      month: "2-digit",
      day: "2-digit",
      hour: "2-digit",
      minute: "2-digit",
    });
  }
  function toReviewRow(run: Run): ReviewRow {
    return {
      id: String(run.id),
      title: run.title || "未获取变更标题",
      status: statusNames[run.status] || run.status,
      tone: statusTone(run.status),
      cells: {
        change: run.title || "未获取变更标题",
        status: statusNames[run.status] || run.status,
        sync: syncNames[run.publish_status] || run.publish_status,
        received: formatReceived(run.created_at),
      },
      source: run,
    };
  }
  function markdownContent(report: Report | null, snapshot: Snapshot | null, pane: ReviewPane, reviewedPolicy: Policy | null, selected: Run | null, defaultRules: string[]) {
    if (pane === "facts" && selected) {
      const sourceURL = markdownURL(selected.url);
      return [
        "## 事实依据",
        `- **评审对象**：${selected.kind === "mr" ? `MR !${plainText(selected.ref)}` : `Commit ${plainText(selected.ref.slice(0, 12))}`}`,
        `- **仓库**：${plainText(selected.repo)}（Project ${plainText(selected.project_id)}）`,
        `- **提交人 / MR 作者**：${plainText(selected.author || "未记录")}`,
        `- **接收时间**：${plainText(formatReceived(selected.created_at))}`,
        selected.base_sha ? `- **基线版本**：\`${plainText(selected.base_sha.slice(0, 12))}\`` : "",
        selected.head_sha ? `- **目标版本**：\`${plainText(selected.head_sha.slice(0, 12))}\`` : "",
        sourceURL ? "" : "- **原始变更**：未记录可用链接",
        report?.scenario ? `- **评审场景**：${plainText(report.scenario)}` : "",
        selected.skill_version_id
          ? `- **在线技能**：${plainText(selected.skill_name || "代码评审技能")} v${selected.skill_version}（${plainText(selected.prompt_version)}）`
          : `- **在线技能**：兼容运行时（${plainText(selected.prompt_version || "legacy")}）`,
        report?.validation ? "## 覆盖与限制\n\n范围：已读取的变更文件；未执行测试、构建或跨服务验证。" : "",
        ...(snapshot?.gaps?.length ? ["## 待补证据", ...snapshot.gaps.map(g => `- ${plainText(g)}`)] : []),
      ].filter(Boolean).join("\n\n");
    }
    if (pane === "code") return ["## 代码依据", ...(report?.findings || []).filter(f => f.evidence).map(f => `### ${plainText(f.file)}:${f.line}\n\n${codeBlock(f.evidence)}`), ...(snapshot?.files || []).map(f => `### ${plainText(f.new_path)}\n\n${codeBlock(f.diff || "未返回文本差异。", "diff")}`)].join("\n\n");
    if (pane === "knowledge") return ["## 知识依据", ...(snapshot?.knowledge.length ? snapshot.knowledge.map(k => `### #${k.id} ${plainText(k.summary)}\n\n版本 ${k.version} · ${plainText(k.scope)} · ${plainText(k.source)}\n\n${k.content}`) : ["本次评审未引用知识库内容。"])].join("\n\n");
    if (pane === "rules") return ["## 通用评审规则", ...defaultRules.map(rule => `- ${plainText(rule)}`), "## 本次补充规则", reviewedPolicy?.rules || "本次未设置补充规则。"].join("\n\n");
    if (!report) return "";
    return ["## 评审结论", report.summary, ...(report.findings.length ? ["## 问题与建议", ...report.findings.map(f => `### ${plainText(({high:"高风险",medium:"中风险",low:"建议"} as Record<string,string>)[f.severity] || f.severity)} · ${plainText(f.title)}\n\n${plainText(dimensions[f.dimension] || f.dimension)} · ${plainText(f.file)}:${f.line}\n\n**影响**\n\n${f.impact}\n\n**修改建议**\n\n${f.suggestion}\n\n**验证建议**\n\n${f.verification}`)] : [selected?.status === "partial" ? "当前可评审范围内未发现明确问题。" : "未发现需要修改的明确问题。"]), ...(report.assessments.length ? ["## 工程质量", ...report.assessments.map(a => `### ${plainText(dimensions[a.dimension] || a.dimension)}\n\n${a.analysis}`)] : []), ...(report.questions.length ? ["## 待确认事项", ...report.questions.map(q => `- ${plainText(q)}`)] : [])].join("\n\n");
  }
  $: reviewMarkdown = markdownContent(report, snapshot, pane, reviewedPolicy, selected, defaultRules);
  async function changePane(nextPane: ReviewPane) {
    pane = nextPane;
    await tick();
    detailModal?.scrollToTop();
  }
  async function handlePaneKeydown(event: KeyboardEvent, currentPane: ReviewPane) {
    if (!["ArrowLeft", "ArrowRight", "Home", "End"].includes(event.key)) return;
    event.preventDefault();
    const available = reviewPanes.filter(
      (item) => !((item === "code" || item === "knowledge") && !snapshot),
    );
    const currentIndex = Math.max(0, available.indexOf(currentPane));
    let nextIndex = currentIndex;
    if (event.key === "Home") nextIndex = 0;
    if (event.key === "End") nextIndex = available.length - 1;
    if (event.key === "ArrowRight") nextIndex = (currentIndex + 1) % available.length;
    if (event.key === "ArrowLeft") {
      nextIndex = (currentIndex - 1 + available.length) % available.length;
    }
    const nextPane = available[nextIndex];
    await changePane(nextPane);
    document.getElementById(`code-review-tab-${nextPane}`)?.focus();
  }
  function closeDetail() {
    actionGeneration++;
    actionKind = "";
    selected = null;
    requestVersion++;
    detailLoading = false;
    detailError = "";
    actionError = "";
    actionNotice = "";
    pane = "review";
  }
  function changeProject(value: string) {
    project = value;
    closeDetail();
    policyError = "";
    pageNotice = "";
    policyFeedback = {};
  }
  onMount(() => {
    void refresh(true, "initial");
    const timer = window.setInterval(() => {
      if (document.visibilityState === "visible") {
        void refresh(false, "poll");
      }
    }, 4000);
    return () => {
      disposed = true;
      requestVersion++;
      window.clearInterval(timer);
    };
  });
</script>

{#snippet renderReviewCell(row: ReviewRow, column: AdminTableColumn, value: AdminCellValue)}
  {@const run = row.source}
  {#if column.key === "change"}
    <button
      type="button"
      class="cr-review-trigger"
      aria-haspopup="dialog"
      aria-expanded={selected?.id === run.id}
      aria-label={`查看评审：${run.title || run.ref}`}
      on:click={() => selectRun(run.id)}
    >
      <span class="cr-review-object">
        {run.kind === "mr" ? `MR !${run.ref}` : `Commit ${run.ref.slice(0, 8)}`}
      </span>
      <strong title={run.title}>{run.title || "未获取变更标题"}</strong>
      <small>
        <span class="cr-mobile-sync">
          {syncNames[run.publish_status] || run.publish_status} ·
        </span>
        <span class="cr-review-source">
          {run.repo} · {run.author || (["queued", "running"].includes(run.status) ? "待提取作者" : "未记录作者")}
        </span>
      </small>
    </button>
  {:else if column.key === "status"}
    <span class="wa-admin-pill {toneClass(statusTone(run.status))}">
      {statusNames[run.status] || run.status}
    </span>
  {:else if column.key === "sync"}
    <span class="wa-admin-pill {toneClass(syncTone(run.publish_status))}">
      {syncNames[run.publish_status] || run.publish_status}
    </span>
  {:else if column.key === "received"}
    <span class="cr-received">{formatReceived(run.created_at)}</span>
  {:else}
    <span>{String(value ?? "-")}</span>
  {/if}
{/snippet}

{#snippet renderReviewEmpty()}
  <div class="cr-list-empty">
    <strong>
      {repos.length === 0
        ? "先接入一个代码仓库"
        : filter
          ? "没有匹配的评审"
          : "等待第一份评审"}
    </strong>
    <span>
      {repos.length === 0
        ? "在配置中心添加 GitLab 仓库与 Project ID。"
        : filter
          ? "调整搜索词或仓库筛选后重试。"
          : "收到新的 Commit / MR 事件后，评审会自动出现在这里。"}
    </span>
  </div>
{/snippet}

<section class="cr-workbench" aria-label="代码评审中心">
  <header class="cr-heading">
    <h1>代码评审</h1>
  </header>

  {#if policyError}<div class="cr-message cr-error" role="alert">{policyError}</div>{/if}
  {#if pageNotice}<div class="cr-message" role="status">{pageNotice}</div>{/if}

  <AdminListFilterBar label="代码评审列表筛选" className="cr-filter-bar">
    {#snippet leading()}
      <div class="cr-list-summary">
        <strong>评审记录</strong>
        <small>
          {loading
            ? "正在读取"
            : refreshing
              ? `正在刷新 · ${visibleRuns.length} 条`
              : `${visibleRuns.length} 条`}
        </small>
      </div>
    {/snippet}

    {#snippet controls()}
      <div class="cr-filter-controls">
      <div class="cr-repo">
        <Select
          label="代码仓库"
          value={project}
          options={[
            { value: "", label: "全部仓库" },
            ...repos.map((item) => ({
              value: item.project_id,
              label: item.name,
              meta: `Project ${item.project_id}`,
            })),
          ]}
          compact
          shadowless
          disabled={repos.length === 0}
          on:change={(e) => changeProject(e.detail)}
        />
      </div>
        <label class="cr-search">
          <span class="cr-sr">搜索评审记录</span>
          <input
            type="search"
            bind:value={filter}
            placeholder="搜索标题、仓库、作者或 SHA"
          />
        </label>
        <button
          type="button"
          class="wa-admin-action secondary"
          on:click={() => void refresh(true, "manual")}
          disabled={loading || refreshing || Boolean(policySaving) || Boolean(actionKind)}
        >
          {refreshing ? "刷新中…" : "刷新"}
        </button>
      </div>
    {/snippet}
  </AdminListFilterBar>

    {#if repos.length > 0 && !repo}
      <p class="cr-policy-guide">
        自动同步评论按仓库独立设置；请先在上方选择一个具体仓库。
      </p>
    {/if}

    {#if repo}
      <details class="cr-policy">
        <summary
          >仓库评审策略 <span
            >{repo.policy.domain === "fms"
              ? scenarios[repo.policy.scenario]
              : "通用工程评审"} · {repo.policy.sync_commits ||
            repo.policy.sync_mrs
              ? "评论同步已配置"
              : "评论同步关闭"}</span
          ></summary
        >
        <div class="cr-policy-content">
          <div class="cr-switches">
            <div>
              <Switch
                id="code-review-sync-commits"
                checked={repo.policy.sync_commits}
                label="自动同步 Commit 评论"
                saving={policySaving === "sync_commits"}
                expandedHitArea
                disabled={!canWrite || Boolean(policySaving)}
                on:change={(e) => savePolicy({ sync_commits: e.detail })}
              />
              <p>完整评审后同步到 Commit。</p>
              {#if policyFeedback.sync_commits?.message}
                <span
                  class="cr-policy-feedback"
                  class:is-error={policyFeedback.sync_commits.error}
                  role={policyFeedback.sync_commits.error ? "alert" : "status"}
                >
                  {policyFeedback.sync_commits.message}
                </span>
              {/if}
            </div>
            <div>
              <Switch
                id="code-review-sync-mrs"
                checked={repo.policy.sync_mrs}
                label="自动同步 MR 评论"
                saving={policySaving === "sync_mrs"}
                expandedHitArea
                disabled={!canWrite || Boolean(policySaving)}
                on:change={(e) => savePolicy({ sync_mrs: e.detail })}
              />
              <p>同步前校验 MR 版本。</p>
              {#if policyFeedback.sync_mrs?.message}
                <span
                  class="cr-policy-feedback"
                  class:is-error={policyFeedback.sync_mrs.error}
                  role={policyFeedback.sync_mrs.error ? "alert" : "status"}
                >
                  {policyFeedback.sync_mrs.message}
                </span>
              {/if}
            </div>
          </div>
          <div class="cr-context-fields">
            <Select
              label="业务领域"
              value={repo.policy.domain}
              options={[
                { value: "general", label: "通用软件工程" },
                { value: "fms", label: "FMS 车队管理系统" },
              ]}
              searchable={false}
              compact
              shadowless
              disabled={!canWrite || Boolean(policySaving)}
              on:change={(e) =>
                savePolicy({
                  domain: e.detail,
                  scenario: e.detail === "fms" ? "dispatch" : "general",
                })}
            />
            <Select
              label="评审场景"
              value={repo.policy.scenario}
              options={Object.entries(scenarios)
                .filter(
                  ([key]) => repo?.policy.domain === "fms" || key === "general",
                )
                .map(([value, label]) => ({ value, label }))}
              searchable={false}
              compact
              shadowless
              disabled={!canWrite || Boolean(policySaving) || repo.policy.domain !== "fms"}
              on:change={(e) => savePolicy({ scenario: e.detail })}
            />
            <label class="cr-field cr-scope"
              >项目知识范围<input
                value={repo.policy.knowledge_scope}
                placeholder="知识库中 project 范围的标识"
                disabled={!canWrite || Boolean(policySaving)}
                on:change={(e) =>
                  savePolicy({ knowledge_scope: e.currentTarget.value.trim() })}
              /></label
            >
          </div>
          <p class="cr-hint">
            只引用已启用的全局、当前仓库及指定项目知识。FMS
            缺少项目或仓库知识时标记部分评审，暂停评论同步。切换策略不会补发历史报告。
          </p>
          {#if !canWrite}<p class="cr-hint">
              当前为只读模式；修改策略需要配置写入权限。
            </p>{/if}
        </div>
      </details>
    {/if}
    {#if repo}
      <details class="cr-rules" bind:open={rulesOpen}>
        <summary
          >评审规则 <span
            >10 项默认工程规则 · {repo.policy.rules
              ? "已设置仓库补充规则"
              : "尚未设置补充规则"}{rulesChanged ? " · 有未保存修改" : ""}</span
          ></summary
        >
        <div class="cr-rules-body">
          <section aria-label="默认评审规则">
            <h3>默认工程检查项</h3>
            <p class="cr-hint">
              始终覆盖以下维度；每条问题都需要代码证据，业务结论需要知识依据。
            </p>
            <ul class="cr-rule-list">
              {#each defaultRules as rule}<li>{rule}</li>{/each}
            </ul>
          </section>
          <section aria-label="仓库补充评审规则">
            <label class="cr-field" for="cr-rules-input"
              >仓库补充规则<textarea
                id="cr-rules-input"
                rows="9"
                maxlength="8000"
                value={ruleDraft}
                disabled={!canWrite || Boolean(policySaving)}
                on:input={(e) =>
                  (ruleDrafts = {
                    ...ruleDrafts,
                    [project]: e.currentTarget.value,
                  })}
                placeholder="每行写一条可验证的要求。例如：任务完成必须有匹配的任务和车辆反馈；重试不得重复释放资源。"
              ></textarea></label
            >
            <p class="cr-hint">
              补充规则与默认规则、FMS
              场景及知识库一起进入两轮评审。不能替代业务证据或取消证据校验。最多
              8000 字符。
            </p>
            <div class="cr-rule-actions">
              <span class="cr-hint"
                >{rulesChanged ? "有未保存修改" : "已与仓库配置一致"}</span
              ><button
                class="cr-button"
                disabled={!canWrite || Boolean(policySaving) || !rulesChanged}
                on:click={() =>
                  (ruleDrafts = {
                    ...ruleDrafts,
                    [project]: repo?.policy.rules || "",
                  })}>撤销修改</button
              ><button
                class="cr-button"
                disabled={!canWrite || Boolean(policySaving) || !rulesChanged}
                on:click={() => savePolicy({ rules: ruleDraft })}
                >保存评审规则</button
              >
            </div>
            {#if !canWrite}<p class="cr-hint">
                当前为只读模式；保存规则需要配置写入权限。
              </p>{/if}
          </section>
        </div>
      </details>
    {/if}
    <AdminDataList
      columns={reviewColumns}
      rows={reviewRows}
      caption="代码评审记录"
      cell={renderReviewCell}
      empty={renderReviewEmpty}
      loading={loading || refreshing}
      error={listError}
      onRetry={() => refresh(true, "manual")}
      skeletonRows={6}
      className="cr-review-list"
      scrollRegionId="code-review-list-content"
      tableMinWidth="720px"
      compactTableMinWidth="0px"
      totalRowCount={reviewRows.length}
      selectedRowId={selected ? String(selected.id) : ""}
      resetKey={`${project}:${filter}`}
    />

    <Modal
      bind:this={detailModal}
      show={selected !== null}
      title={selected?.title || "评审详情"}
      size="wide"
      shadowless
      stableHeight
      footerVisible={Boolean(
        selected &&
          (actionError ||
            actionNotice ||
            selected.publish_error ||
            (selected.status === "partial" && selected.publish_status === "blocked") ||
            (canWrite &&
              (canManualSync ||
                selected.status === "failed" ||
                ["queued", "running"].includes(selected.status)))),
      )}
      closeLabel="关闭评审详情"
      on:close={closeDetail}
    >
      {#if selected}
        <section class="cr-reading" aria-label="评审详情" aria-busy={detailLoading}>
          <div class="cr-reading-toolbar">
            <div class="cr-reading-meta">
              <span>
                {selected.kind === "mr"
                  ? `MR !${selected.ref}`
                  : `Commit ${selected.ref.slice(0, 8)}`}
              </span>
              <span class="wa-admin-pill {toneClass(statusTone(selected.status))}">
                {statusNames[selected.status] || selected.status}
              </span>
            </div>
            {#if canReadEvidence}
              <nav class="cr-evidence-links" aria-label="评审内容与依据">
                <div class="cr-pane-tabs" role="tablist" aria-label="评审详情视图">
                  {#each reviewPanes as item}
                    <button
                      id={`code-review-tab-${item}`}
                      type="button"
                      role="tab"
                      class:active={pane === item}
                      aria-selected={pane === item}
                      tabindex={pane === item ? 0 : -1}
                      aria-controls="code-review-detail-panel"
                      disabled={(item === "code" || item === "knowledge") && !snapshot}
                      on:click={() => changePane(item)}
                      on:keydown={(event) => handlePaneKeydown(event, item)}
                    >
                      {paneLabels[item]}{item === "knowledge" && snapshot
                        ? ` ${snapshot.knowledge.length}`
                        : ""}
                    </button>
                  {/each}
                </div>
                {#if safeURL(selected.url)}
                  <a
                    href={safeURL(selected.url)}
                    target="_blank"
                    rel="noopener noreferrer"
                    title="在 GitLab 新窗口查看原始变更"
                  >
                    GitLab ↗
                  </a>
                {/if}
              </nav>
            {/if}
          </div>

          <div
            id="code-review-detail-panel"
            class="cr-reading-panel"
            role="tabpanel"
            aria-labelledby={canReadEvidence ? `code-review-tab-${pane}` : undefined}
          >
            {#if detailLoading}
              <div class="cr-reading-state" role="status">
                <strong>正在加载评审…</strong>
              </div>
            {:else if !canReadEvidence}
              <div class="cr-reading-state">
                <strong>当前账户无法查看评审内容</strong>
                <span>查看源码、知识与报告详情需要 AI 上下文预览权限。</span>
              </div>
            {:else if detailError}
              <div class="cr-reading-state cr-reading-error" role="alert">
                <strong>评审详情暂时无法加载</strong>
                <span>{detailError}</span>
                <button
                  type="button"
                  class="wa-admin-action secondary"
                  on:click={() => selected && selectRun(selected.id, true, false)}
                >
                  重新加载
                </button>
              </div>
            {:else if selected.error && !report && pane === "review"}
              <div class="cr-reading-state cr-reading-error" role="alert">
                <strong>评审未完成</strong>
                <span>{conciseReviewError(selected.error)}</span>
              </div>
            {:else if !report && pane === "review"}
              <div class="cr-reading-state" role="status">
                <strong>
                  {selected.status === "queued"
                    ? "排队中，完成后自动更新。"
                    : selected.status === "running"
                      ? "评审中，完成后自动更新。"
                      : "暂无评审结果"}
                </strong>
              </div>
            {:else}
              {#key `${selected.id}:${pane}`}
                <MarkdownWorkbench
                  value={reviewMarkdown}
                  mode="preview"
                  availableModes={["preview"]}
                  readonly
                  showToolbar={false}
                  showDocumentMeta={false}
                  embedded
                  fullWidthPreview
                  autoHeight
                  minHeight={180}
                  label={paneLabels[pane]}
                  description=""
                />
              {/key}
            {/if}
          </div>
        </section>

      {/if}

      <svelte:fragment slot="footer">
        {#if selected}
          <div class="cr-modal-footer">
            <div class="cr-modal-feedback" aria-live="polite">
              {#if actionError}
                <span class="is-error" role="alert">{actionError}</span>
              {:else if actionNotice}
                <span class="is-success">{actionNotice}</span>
              {:else if selected.publish_error}
                <span class="is-error" role="alert">评论同步：{selected.publish_error}</span>
              {:else}
                <span>{syncNames[selected.publish_status] || selected.publish_status}</span>
              {/if}
            </div>
            <div class="cr-modal-actions">
              {#if canManualSync}
                <button
                  type="button"
                  class="wa-admin-action primary"
                  disabled={Boolean(actionKind)}
                  on:click={() => runAction("sync")}
                >
                  {actionKind === "sync"
                    ? "处理中…"
                    : selected.publish_status === "unknown"
                      ? "核对远端评论"
                      : "同步评论"}
                </button>
              {:else if canWrite && selected.status === "failed"}
                <button
                  type="button"
                  class="wa-admin-action primary"
                  disabled={Boolean(actionKind)}
                  on:click={() => runAction("retry")}
                >
                  {actionKind === "retry" ? "重新评审中…" : "重新评审"}
                </button>
              {:else if canWrite && ["queued", "running"].includes(selected.status)}
                <button
                  type="button"
                  class="wa-admin-action secondary"
                  title="只停止本地评审，不会发布评论"
                  disabled={Boolean(actionKind)}
                  on:click={() => runAction("cancel")}
                >
                  {actionKind === "cancel" ? "取消中…" : "取消评审"}
                </button>
              {/if}
            </div>
          </div>
        {/if}
      </svelte:fragment>
    </Modal>
</section>
