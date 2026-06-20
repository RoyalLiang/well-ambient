# Phase 4 动态周会议程与决策大屏设计文档

本文档详细记录了 `well-ambient` 系统中关于 Phase 4：动态周会议程与决策大屏的核心架构、数据流程、API 契约及前端 UI 实现方案。

---

## 1. 系统架构

```
                     ┌───────────────────────┐
                     │  GitLab Push/MR Hook  │
                     └───────────┬───────────┘
                                 │ Webhook (Git telemetry)
                                 ▼
                     ┌───────────────────────┐
                     │   internal/telemetry  │
                     └───────────┬───────────┘
                                 │
                        Write to │ GORM SQLite
                                 ▼
                     ┌───────────────────────┐
                     │    sqlite.db (Task)   │
                     └───────────┬───────────┘
                                 │ Read tasks state
                                 ▼
                     ┌───────────────────────┐
                     │    internal/agenda    │
                     │ (Red-Zone Diagnostics)│
                     └───────────┬───────────┘
                                 ├────────────────────────┐
                                 │ SSE / REST API         │ Lark Bot Message
                                 ▼                        ▼
                     ┌───────────────────────┐   ┌───────────────────┐
                     │   Svelte Dashboard    │   │  Lark Chat Group  │
                     │  (Cyberpunk War Room) │   │ (Interactive Card)│
                     └───────────────────────┘   └───────────────────┘
```

### 1.1 后端核心分析模块 (`internal/agenda`)
该模块周期性或在 API 请求时评估所有未完成任务的状态。
*   **诊断模型**：
    1.  **Block/No-Commit**: `status != 'done'`，但最后提交时间 `last_update` 超过 48 小时。
    2.  **Overdue**: 相比于 `due_date` 已经延期，或者自创建日期 `task_created_at` 超过 7 天尚未完成。
    3.  **Conflict Risk**: 多人在相同仓库的非主干分支上进行开发，或存在 MR 冲突标识。

### 1.2 前端决策大屏 (`web/src/components/DecisionDashboard.svelte`)
大屏采用**赛博朋克深色/极简质感美学**进行投屏展示，分为三大面板：
1.  **红区诊断盘**：直观高亮处于危急状态的任务与责任人，伴随轻微呼吸灯微动画。
2.  **交互式议程路由器**：点击某项议题时展示其详细的 Git commit 时序轨迹与 AI 诊断建议，提供快速动作面板（转派、挂起、排期调整）。
3.  **决策跟踪流**：记录并滚动展示本次会议已达成的决策历史，提供一键同步到飞书多维表格（Bitable）和 Jira 的状态。

---

## 2. 数据库结构扩展 (`internal/db/db.go`)

在现有的 `TaskTelemetry` 结构中，新增 `due_date`（截止时间）与 `decision_logs`（决策日志文本），以便支持超期计算和会议操作记录：

```go
type TaskTelemetry struct {
	TaskID        string     `gorm:"primaryKey;column:task_id" json:"task_id"`
	Title         string     `json:"title"`
	Repo          string     `json:"repo"`
	Assignee      string     `json:"assignee"`
	Branch        string     `json:"branch"`
	LastCommit    string     `json:"last_commit"`
	Status        string     `json:"status"` // backlog, progress, review, done
	IssueType     string     `json:"issue_type"`
	TaskCreatedAt time.Time  `json:"task_created_at"`
	LastUpdate    time.Time  `json:"last_update"`
	DueDate       *time.Time `json:"due_date"`      // 任务截止时间
	DecisionLogs  string     `json:"decision_logs"` // 会议决策历史，存储为 JSON 字符串
}
```

---

## 3. 接口协议设计

### 3.1 获取周会议程脱水简报
*   **路径**：`GET /api/agenda/summary`
*   **请求**：无
*   **响应 (JSON)**：
```json
{
  "total_active_tasks": 12,
  "red_zone_count": 2,
  "agenda_items": [
    {
      "task_id": "AB-3456",
      "title": "【对位故障】AT010 alignment not correct",
      "assignee": "朱家聪",
      "status": "progress",
      "risk_level": "critical",
      "risk_type": "no_commit_48h",
      "desc": "该任务处于“开发中”，但已连续 72 小时无任何 Git Commit，疑似遇到阻塞或卡点。",
      "telemetry_snippet": {
        "branch": "feat-fms-auth-with-5.1.1-20260612",
        "last_commit": "Merge remote-tracking branch...",
        "last_update": "2026-06-12T10:00:00Z"
      }
    }
  ]
}
```

### 3.2 提交会议决策
*   **路径**：`POST /api/agenda/decision`
*   **请求主体 (JSON)**：
```json
{
  "task_id": "AB-3456",
  "action": "reassign", // 可选值: reassign (转派), suspend (挂起), reschedule (调整截止时间)
  "payload": {
    "assignee": "白凌云",
    "due_date": "2026-06-19T18:00:00Z",
    "note": "转派白凌云协助快速收尾"
  },
  "operator": "李明"
}
```
*   **响应 (JSON)**：
```json
{
  "success": true,
  "message": "Decision logged and synced successfully",
  "updated_task": {
    "task_id": "AB-3456",
    "assignee": "白凌云",
    "status": "progress"
  }
}
```

---

## 4. UI/UX 视觉与动效交互设计

为了确保大屏有 WOW 级的观感，将采用以下前端设计规范：
*   **色彩体系**：背景采用 Slate-950 (#020617)，卡片采用带有 1px 细边框的毛玻璃透明效果（`backdrop-filter: blur(12px)`）。红区边框加重为霓虹红，安全区为极光绿。
*   **动画**：
    *   **呼吸灯效果**：红区警告图标伴随 `animation: pulse 2s infinite`，引起会议主持人的强烈警觉。
    *   **切换动效**：议程列表切换时，使用 CSS View Transitions 或 Svelte 的 `fade/fly` 动画，实现平滑推拉切换效果。
    *   **一键决策回馈**：点击“转派”等决策按钮时，按钮收缩轻微回弹（`transform: scale(0.95)`），并在卡片右下角显示绿色小对勾渐显。
