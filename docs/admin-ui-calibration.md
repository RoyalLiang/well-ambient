# Phase 41 Admin UI Calibration

This document is the shared contract for the `well-ambient` admin UI refactor. It exists so page agents can migrate business pages against the same visual baseline and data shape instead of inventing local dark-glass systems.

## Scope

- Applies to the Phase 41 functional admin console refactor.
- Primary visual reference: `output/functional-admin-console-reference.png`.
- Shared implementation anchors:
  - `DESIGN.md`
  - `web/src/styles/modern-admin-tokens.css`
  - `web/src/components/prototype/FunctionalAdminShell.svelte`
  - `web/src/components/prototype/FunctionalWorkspace.svelte`
  - `web/src/lib/admin-console/contract.ts`
- Page agents still own business pages. This contract does not authorize edits to `DecisionDashboard.svelte`, `DemandKanban.svelte`, `TaskKanban.svelte`, `ProjectHealthTelemetry.svelte`, `KPIKanban.svelte`, `SettingsPanel.svelte`, or `Deconstructor.svelte`.

## Drift Audit

### `DESIGN.md`

- Before Phase 41 it described a global dark tech cockpit with slate-950 backgrounds, neon accent glow, bento cells, and active scale feedback.
- That conflicts with the new reference, which is a light admin workbench with a dark rail, white frosted cards, a dense table, and a right inspector.
- The old cockpit language is now superseded except for explicit future dark-screen requests.

### `modern-admin-tokens.css`

- Previous tokens made `--wa-bg-base`, surfaces, table rows, panels, and text all dark by default.
- Shared classes were too generic for page agents: `.wa-glass`, `.wa-panel`, and `.wa-control` did not define table, metric, inspector, or toolbar behavior.
- Phase 41 needs light page and surface tokens plus dark-rail tokens, with table-first classes that can be reused in business pages.

### `FunctionalAdminShell.svelte`

- The shell made both rail and main area dark, with radial glow, dark topbar, dark search, and a dark workspace frame.
- The reference requires the rail to remain dark while the topbar and work area become light, calm, and table-oriented.
- Brand must stay `well-ambient`; do not carry over old phase naming or cockpit-stage copy.

### `FunctionalWorkspace.svelte`

- The workspace hero used a large dark glass panel with glow and module color wash.
- Page agents should use compact module headers, metric strips, and table sections rather than a hero-first or theatre-first composition.
- Workspace wrappers may provide context, but the main page content should remain the table/inspector workflow.
- Production pages should leave `showContextBar` off by default so the topbar is followed directly by real business content.

### Prototype Components

- `PrototypeShell`, `PrototypeTheatre`, `PrototypeMetric`, `PrototypeTable`, and `PrototypeInspector` are Phase 38/39 dark prototype artifacts.
- They can be mined for behavior and naming, but they are not the current visual source of truth.
- New production refactor work should use the Phase 41 token/classes below or create light equivalents under `web/src/components/admin-console/*`.

## Visual Contract

- Layout: fixed dark left rail, light main canvas, sticky light topbar, content grid with optional right inspector.
- Primary page grammar: metrics row -> compact segment/status row -> main table -> right detail/inspector.
- Surfaces: white or near-white frosted cards with 1px cool-gray border and soft shadow. No nested cards inside cards.
- Tables: table is the primary comparison surface; rows must be stable height, scannable, and selectable.
- Inspector: right side panel contains selected record facts, description, evidence/checklist, progress, and actions.
- Motion: use color, border, and shadow feedback. Do not use active scale transforms for admin controls.
- Radius: use 6-12px for cards and controls. Avoid oversized rounded glass panels.
- Palette: light neutral canvas, dark navy rail, teal primary action, green success, amber warning, red danger, blue info.

## Shared CSS Contract

Page agents should prefer these classes from `web/src/styles/modern-admin-tokens.css`:

| Class | Purpose |
| --- | --- |
| `.wa-admin-card` | White frosted surface for one repeated item, metric, toolbar, or inspector panel. |
| `.wa-admin-metric` | Metric card with label, strong numeric value, helper text, and optional tone. |
| `.wa-admin-section` | Full-width content section wrapper; use for table modules, not nested in another card. |
| `.wa-admin-toolbar` | Filter/search/action row above tables. |
| `.wa-admin-table-shell` | Scroll container and border treatment for dense tables. |
| `.wa-admin-table` | Shared table typography, sticky header, selected-row and hover behavior. |
| `.wa-admin-inspector` | Right detail panel, sticky on desktop and normal flow on smaller viewports. |
| `.wa-admin-pill` | Semantic tags and status chips. Add `tone-success`, `tone-warning`, `tone-danger`, `tone-info`, or `tone-neutral`. |
| `.wa-admin-action` | Button/action control. Add `.primary`, `.secondary`, or `.danger`. |
| `.wa-admin-progress` | Evidence/completion bar with `--progress` percentage. |

## Component Contract

Until a dedicated `web/src/components/admin-console/*` library is expanded, page agents should use:

- Shell: `FunctionalAdminShell.svelte` for the dark rail, topbar, profile, alerts, and workspace frame.
- Workspace: `FunctionalWorkspace.svelte` as a pass-through content frame by default. Use its context bar only for exceptional page-level status, not as a standard module hero.
- Nested navigation: pages with multiple configuration or security subareas should render grouped submenus in `FunctionalAdminShell`'s global rail/menu area. Do not place these navigation controls inside the page's main content frame.
- Content frame: the right-side work area should be one unified frame with a breadcrumb bar above the actual page content. Do not mix a category index table, status hero, and inspector as sibling top-level regions unless the page's primary workflow is explicitly table-plus-inspector.
- Metrics: `.wa-admin-metric` markup inside the page adapter output.
- Table: native `table` with `.wa-admin-table-shell` and `.wa-admin-table`; do not replace the main workflow with card grids.
- Inspector: `aside.wa-admin-inspector` fed by the selected table row's adapter record.

## Data Adapter Contract

Business pages should import the shared shapes from `web/src/lib/admin-console/contract.ts` and map real API responses into those presentation records before rendering. Shared UI should not fetch, mock, or infer backend visibility rules.

```ts
type AdminTone = "neutral" | "info" | "success" | "warning" | "danger";

interface AdminMetric {
  label: string;
  value: string | number;
  helper?: string;
  delta?: string;
  tone?: AdminTone;
}

interface AdminTableColumn {
  key: string;
  label: string;
  width?: string;
  align?: "left" | "center" | "right";
}

interface AdminTableRow {
  id: string;
  title: string;
  status: string;
  tone?: AdminTone;
  owner?: string;
  dueDate?: string;
  priority?: string;
  risk?: string;
  cells: Record<string, string | number | boolean | null | undefined>;
}

interface AdminInspectorRecord {
  id: string;
  title: string;
  status: string;
  tone?: AdminTone;
  facts: Array<{ label: string; value: string | number }>;
  sections: Array<{ title: string; body?: string; items?: string[] }>;
  actions?: Array<{ label: string; kind: "primary" | "secondary" | "danger" }>;
}
```

Adapter rules:

- Keep the backend/API boundary authoritative. Do not re-add hidden non-core data in UI adapters.
- Keep source IDs stable: Jira key, demand ID, task ID, or config key must survive selection and inspector mapping.
- Map status/risk to a small tone set only at the adapter edge.
- Preserve empty states honestly. Do not synthesize fake table rows to make a layout look full.
- Every production page must use real API data before it is considered Phase 41 aligned.

## Page-Agent Checklist

- Read this document and `DESIGN.md` before editing a page.
- Do not use the old dark cockpit or Phase 39 theatre components as visual target.
- Start with the existing API response, write a small adapter, then render metrics/table/inspector from that adapter.
- Keep page ownership disjoint. Do not edit another page agent's component.
- Validate with at least `pnpm build` from `web/`, or record the narrower check and any pre-existing blockers.
