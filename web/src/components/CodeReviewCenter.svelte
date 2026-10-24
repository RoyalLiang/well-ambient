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
  import { attachmentFilename } from "../lib/code-review-download";
  export let currentUserPermissions: string[] = [];
  type Policy = {
    project_id: string;
    review_commits?:boolean; review_mrs?:boolean;
    sync_commits: boolean;
    sync_mrs: boolean;
    domain: string;
    scenario: string;
    knowledge_scope: string;
    rules: string;
  };
  type Repo = { project_id: string; name: string; namespace_name?:string; namespace_path?:string; policy: Policy };
  type CommentSettings = { sync_commits: boolean; sync_mrs: boolean };
  type RepoResponse = {
    repos: Repo[];
    default_rules: string[];
    comment_settings?: unknown;
  };
  type Run = {
    repository_source_json?:string;
 document?: {filename: string; sections: Record<ReviewPane, string>};
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
    priority?: string;
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
    review_method?: { name: string; capability_key: string; version_id: number; version: number; hash: string };
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
    { key: "project", label: "项目", width: "160px", priority: "essential" },
    { key: "repository", label: "仓库", width: "200px", priority: "essential" },
    { key: "ref", label: "MR / Commit", width: "140px", priority: "essential" },
    { key: "author", label: "提交人", width: "120px", priority: "essential" },
    { key: "change", label: "评审内容", width: "260px", priority: "essential" },
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
  let commentSettings: CommentSettings | null = null;
  let defaultRules: string[] = [];
  let ruleDrafts: Record<string, string> = {};
  let rulesOpen = false;
  $: ruleDraft = ruleDrafts[project] ?? repo?.policy.rules ?? "";
  $: rulesChanged = ruleDraft !== (repo?.policy.rules ?? "");
  $: rulesTooLong = ruleDraft.length > 8000;
  $: reviewedPolicy = parse<Policy>(selected?.policy_json);
  let runs: Run[] = [];
  let project = "";
  let filter = "";
  let searchQuery = "";
 let kind = "";
 let offset = 0;
 const pageSize = 50;
 let total = 0;
 let hasMore = false;
 let exporting = false;
 let exportError = "";
 let exportURL = "";
 let exportedID = 0;
  let exportFilename = "";
 $: activeSearch = Boolean(project || kind || searchQuery);
 function searchKey() { return JSON.stringify([project, kind, searchQuery, offset]); }
 function applySearch() { searchQuery = filter.trim(); offset = 0; listRequestVersion++; void refresh(false, "manual"); }
 function changeKind(value: string) { kind = value; applySearch(); }
 function resetSearch() { project = ""; kind = ""; filter = ""; closeDetail(); applySearch(); }
 function changePage(next: number) { offset = next; listRequestVersion++; void refresh(false, "manual"); }
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
  $: selectedGlobalSyncEnabled = Boolean(
    selected && commentSettings &&
      (selected.kind === "mr" ? commentSettings.sync_mrs : commentSettings.sync_commits),
  );
  $: selectedSyncEnabled = Boolean(
    selected &&
      selectedRepo &&
      selectedGlobalSyncEnabled &&
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
  $: manualSyncBlockedReason =
    canWrite && selected?.status === "completed" &&
    !["published", "publishing", "stale", "blocked"].includes(selected.publish_status)
      ? !commentSettings
        ? "全局评论设置未加载，暂不能同步评论"
        : !selectedGlobalSyncEnabled
          ? `全局 ${selected.kind === "mr" ? "MR" : "Commit"} 评论已暂停`
          : ""
      : "";
  $: visibleRuns = runs;
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
  function applyRepoResponse(data: RepoResponse) {
    repos = data.repos;
    defaultRules = data.default_rules || [];
    const settings = data.comment_settings as Partial<CommentSettings> | null;
    commentSettings = settings && typeof settings === "object" &&
      typeof settings.sync_commits === "boolean" && typeof settings.sync_mrs === "boolean"
        ? { sync_commits: settings.sync_commits, sync_mrs: settings.sync_mrs }
        : null;
  }
  async function refreshReposSilently() {
    const version = ++listRequestVersion;
    try {
      const data = await api<RepoResponse>("/api/code-reviews/repos");
      if (disposed || version !== listRequestVersion) return;
      applyRepoResponse(data);
    } catch (error) {
      if (!disposed && version === listRequestVersion) commentSettings = null;
      throw error;
    }
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
    const requestedProject = project;
 const requestedSearch = searchKey();
    const version = ++listRequestVersion;
    const visibleRefresh = mode !== "poll";
    const anchor = mode === "initial" ? null : captureListAnchor();
    if (visibleRefresh) {
      listError = "";
      if (mode === "initial" && runs.length === 0) loading = true;
      else refreshing = true;
    }
    try {
      if (initial) {
        let data: RepoResponse;
        try {
          data = await api<RepoResponse>("/api/code-reviews/repos");
        } catch (error) {
          if (!disposed && version === listRequestVersion) commentSettings = null;
          throw error;
        }
        if (disposed || version !== listRequestVersion) return false;
        applyRepoResponse(data);
      }
      const params = new URLSearchParams({limit: String(pageSize), offset: String(offset)});
 if (project) params.set("project_id", project);
 if (kind) params.set("kind", kind);
 if (searchQuery) params.set("q", searchQuery);
 const data = await api<{items: Run[]; total: number; has_more: boolean}>(`/api/code-reviews?${params}`);
 if (disposed || version !== listRequestVersion || requestedSearch !== searchKey()) return false;
 total = data.total;
 hasMore = data.has_more;
 if (offset > 0 && data.items.length === 0 && total > 0) { offset = Math.floor((total - 1) / pageSize) * pageSize; void refresh(false, "manual"); return false; }
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
      if (visibleRefresh && requestedSearch === searchKey() && version === listRequestVersion && !disposed) listError = (e as Error).message;
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
    if (action === "sync" && !canManualSync) {
      actionError = manualSyncBlockedReason || "当前仓库尚未开启此类评论同步。";
      return;
    }
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
                ? "已重新加入评审队列，完成后更新本条记录。"
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
        project: parse<{namespace_name?:string;namespace_path?:string}>(run.repository_source_json)?.namespace_name || parse<{namespace_path?:string}>(run.repository_source_json)?.namespace_path || '未归组',
        repository:run.repo,ref:run.kind==='mr'?`MR !${run.ref}`:`Commit ${run.ref.slice(0,12)}`,author:run.author||'未记录',
        change: run.title || "未获取变更标题",
        status: statusNames[run.status] || run.status,
        sync: syncNames[run.publish_status] || run.publish_status,
        received: formatReceived(run.created_at),
      },
      source: run,
    };
  }
  $: reviewMarkdown = selected?.document?.sections[pane] || "评审文档尚不可用；请刷新详情或核对结果记录。";
 async function exportMarkdown() {
  if (!selected || !canReadEvidence || !report || !["completed", "partial"].includes(selected.status) || exporting) return;
  const origin = selected.id;
  exporting = true; exportError = "";
  try {
   const response = await fetch(`/api/code-reviews/${origin}/export`);
   if (!response.ok) throw new Error((await response.text()).trim() || "导出失败，请重试。");
   const blob = await response.blob();
      const filename = attachmentFilename(response.headers.get("Content-Disposition"), selected?.document?.filename || "review.md");
   if (disposed || selected?.id !== origin) return;
   if (exportURL) URL.revokeObjectURL(exportURL);
   const url = URL.createObjectURL(blob);
   exportURL = url; exportedID = origin; exportFilename = filename;
   const link = document.createElement("a");
   link.href = url; link.download = filename;
   document.body.appendChild(link); link.click(); link.remove();
   if (selected?.id !== origin) { URL.revokeObjectURL(url); exportURL = ""; }
  } catch (error) { if (selected?.id === origin) exportError = (error as Error).message; }
  finally { exporting = false; }
 }
   async function changePane(nextPane: ReviewPane) {
    pane = nextPane;
    await tick();
    detailModal?.scrollToTop();
  }
  async function handlePaneKeydown(event: KeyboardEvent, currentPane: ReviewPane) {
    if (!["ArrowLeft", "ArrowRight", "Home", "End"].includes(event.key)) return;
    event.preventDefault();
    const available = selected?.document ? reviewPanes : ["review" as ReviewPane];
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
    exportError = "";
    if (exportURL) URL.revokeObjectURL(exportURL);
    exportURL = "";
    pane = "review";
  }
  function changeProject(value: string) {
    project = value;
 offset = 0; listRequestVersion++;
    closeDetail();
    policyError = "";
    pageNotice = "";
    policyFeedback = {};
    void refresh(false, "manual");
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
      if (exportURL) URL.revokeObjectURL(exportURL);
      requestVersion++;
      window.clearInterval(timer);
    };
  });
</script>

{#snippet renderReviewCell(row: ReviewRow, column: AdminTableColumn, value: AdminCellValue)}
  {@const run = row.source}
  {#if column.key === "project"}
    {@const source = parse<{namespace_name?:string;namespace_path?:string}>(run.repository_source_json)}
    <span>{source?.namespace_name || source?.namespace_path || repos.find(r=>r.project_id===run.project_id)?.namespace_name || repos.find(r=>r.project_id===run.project_id)?.namespace_path || '未归组'}</span>
  {:else if column.key === "repository"}<span class="cr-repository-name" title={run.repo}>{run.repo}</span>
  {:else if column.key === "ref"}<span title={run.head_sha||run.ref}>{run.kind==='mr'?`MR !${run.ref}`:`Commit ${run.ref.slice(0,12)}`}</span>
  {:else if column.key === "author"}<span>{run.author || (['queued','running'].includes(run.status)?'待提取':'未记录')}</span>
  {:else if column.key === "change"}
    <button
      type="button"
      class="cr-review-trigger"
      aria-haspopup="dialog"
      aria-expanded={selected?.id === run.id}
      aria-label={`查看评审：${run.title || run.ref}`}
      on:click={() => selectRun(run.id)}
    >
      <strong title={run.title}>{run.title || "未获取变更标题"}</strong>

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
        : activeSearch
          ? "没有匹配的评审"
          : "等待第一份评审"}
    </strong>
    <span>
      {repos.length === 0
        ? "在配置中心添加 GitLab 仓库与 Project ID。"
        : activeSearch
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
        <small>{loading ? "正在读取" : `共 ${total} 条`}</small>
        <span class="cr-sr" role="status" aria-live="polite">
          {refreshing ? "正在刷新评审记录，当前结果保留显示。" : ""}
        </span>
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
        <div class="cr-kind">
 <Select id="review-kind-filter" label="评审类型" value={kind} options={[{value:"",label:"全部类型"},{value:"mr",label:"MR"},{value:"commit",label:"Commit"}]} compact shadowless on:change={(e) => changeKind(e.detail)} />
 </div>
 <label class="cr-search">
          <span class="cr-sr">搜索评审记录</span>
          <input
            type="search"
            bind:value={filter}
            placeholder="搜索 !42、Commit SHA、标题或作者"
            maxlength={200}
            on:keydown={(event) => { if (event.key === "Enter") applySearch(); }}
          />
        </label>
        <button type="button" class="wa-admin-action secondary" on:click={applySearch}>搜索</button>
        <button
          type="button"
          class="wa-admin-action secondary cr-refresh-button"
          on:click={() => void refresh(true, "manual")}
          disabled={loading || refreshing || Boolean(policySaving) || Boolean(actionKind)}
          aria-busy={refreshing}
        >
          {refreshing ? "刷新中…" : "刷新"}
        </button>
      </div>
    {/snippet}
  </AdminListFilterBar>
 <div class="cr-result-navigation" aria-label="评审搜索结果分页">
 <span role="status" aria-live="polite">{total ? `第 ${offset + 1}–${offset + runs.length} 条，共 ${total} 条` : "0 条评审"}</span>
 {#if activeSearch}<button class="wa-admin-action secondary" on:click={resetSearch}>重置筛选</button>{/if}
 {#if offset > 0 || hasMore}
 <button class="wa-admin-action secondary" disabled={offset === 0 || refreshing} on:click={() => changePage(Math.max(0,offset-pageSize))}>上一页</button>
 <button class="wa-admin-action secondary" disabled={!hasMore || refreshing} on:click={() => changePage(offset+pageSize)}>下一页</button>
 {/if}
 </div>


    <AdminDataList
      columns={reviewColumns}

      rows={reviewRows}
      caption="代码评审记录"
      cell={renderReviewCell}
      empty={renderReviewEmpty}
      loading={loading}
      error={listError}
      onRetry={() => refresh(true, "manual")}
      skeletonRows={6}
      className="cr-review-list"
      scrollRegionId="code-review-list-content"
      tableMinWidth="1300px"
      compactTableMinWidth="0px"
      totalRowCount={reviewRows.length}
      selectedRowId={selected ? String(selected.id) : ""}
      resetKey={`${project}:${kind}:${filter}`}
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
          manualSyncBlockedReason ||
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
            <button
              type="button"
              class="wa-admin-action secondary cr-export"
              disabled={exporting || detailLoading || !canReadEvidence || !report || !["completed", "partial"].includes(selected.status)}
              aria-busy={exporting}
                  aria-label={exporting ? "正在导出 Markdown" : "下载 Markdown 报告"}
              title={!canReadEvidence ? "需要 AI 上下文预览权限" : !report ? "生成评审结果后可下载" : "下载本次保存的评审报告"}
              on:click={exportMarkdown}
            ><svg aria-hidden="true" width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"><path d="M12 3v12m-5-5 5 5 5-5M5 16v4h14v-4" /></svg></button>
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
                      disabled={!selected.document}
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

          {#if exporting}<p class="cr-export-feedback" role="status">正在准备 Markdown 报告…</p>{/if}
            {#if exportURL && exportedID === selected.id}<p class="cr-export-feedback" role="status">Markdown 报告已准备好。<a href={exportURL} download={exportFilename} aria-label="再次下载 Markdown 报告" title={exportFilename}><svg aria-hidden="true" width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"><path d="M12 3v12m-5-5 5 5 5-5M5 16v4h14v-4" /></svg></a></p>{/if}
          {#if exportError}<div class="cr-message cr-error" role="alert">{exportError}<button type="button" class="wa-admin-action secondary" on:click={exportMarkdown}>重试导出</button></div>{/if}
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
              {:else if manualSyncBlockedReason}
                <span>{manualSyncBlockedReason}</span>
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
              {:else if manualSyncBlockedReason}
                <button type="button" class="wa-admin-action primary" disabled title={manualSyncBlockedReason}>
                  同步评论
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
