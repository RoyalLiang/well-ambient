<script module lang="ts">
  export type MarkdownMode = 'live' | 'edit' | 'split' | 'preview';
</script>

<script lang="ts">
  import { defaultKeymap, history, historyKeymap, indentWithTab } from '@codemirror/commands';
  import { bracketMatching, defaultHighlightStyle, foldGutter, indentOnInput, syntaxHighlighting } from '@codemirror/language';
  import { markdown } from '@codemirror/lang-markdown';
  import { Compartment, EditorState, type Range } from '@codemirror/state';
  import {
    Decoration,
    drawSelection,
    dropCursor,
    EditorView,
    highlightActiveLine,
    highlightActiveLineGutter,
    highlightSpecialChars,
    keymap,
    lineNumbers,
    placeholder as editorPlaceholder,
    ViewPlugin,
    type DecorationSet,
    type ViewUpdate,
    WidgetType
  } from '@codemirror/view';
  import DOMPurify from 'dompurify';
  import { marked } from 'marked';
  import { createEventDispatcher, onMount } from 'svelte';

  export let value = '';
  export let mode: MarkdownMode = 'live';
  export let availableModes: readonly MarkdownMode[] = ['live'];
  export let readonly = false;
  export let label = 'Markdown 文档';
  export let description = '支持 GFM、表格、任务列表、引用与代码块';
  export let placeholder = '使用 Markdown 编写内容…';
  export let minHeight = 460;
  export let showToolbar = true;
  export let showDocumentMeta = false;
  export let embedded = false;
  export let fullWidthPreview = false;
  export let streaming = false;
  export let inlineEditing = false;
  export let saving = false;
  export let autoHeight = false;

  const dispatch = createEventDispatcher<{ change: string; save: string; commit: string }>();
  const readonlyCompartment = new Compartment();
  const liveModeCompartment = new Compartment();
  let editorHost: HTMLDivElement;
  let editorView: EditorView | null = null;
  let workbenchRoot: HTMLElement;
  let rendered = '';
  let editBaseline = '';

  const modeLabels: Record<MarkdownMode, string> = {
    live: '即时排版',
    edit: '源码',
    split: '分屏',
    preview: '阅读'
  };

  $: rendered = renderMarkdown(value);
  $: normalizedAvailableModes = Array.from(new Set<MarkdownMode>(availableModes.length ? availableModes : ['live']));
  $: toolbarModes = normalizedAvailableModes.includes(mode) ? normalizedAvailableModes : [mode];
  $: showInlineEditStatus = inlineEditing && mode === 'edit';
  $: showWorkbenchToolbar = showToolbar && (showDocumentMeta || toolbarModes.length > 1);
  $: lineCount = value ? value.split('\n').length : 1;
  $: if (editorView && value !== editorView.state.doc.toString()) {
    editorView.dispatch({ changes: { from: 0, to: editorView.state.doc.length, insert: value } });
  }
  $: if (editorView) {
    editorView.dispatch({ effects: readonlyCompartment.reconfigure(readonlyExtensions(readonly)) });
  }
  $: if (editorView) {
    editorView.dispatch({ effects: liveModeCompartment.reconfigure(mode === 'live' ? liveMarkdownExtension : []) });
  }

  onMount(() => {
    const handleOutsidePointer = (event: PointerEvent) => {
      if (!inlineEditing || readonly || saving || mode !== 'edit' || !workbenchRoot) return;
      if (event.target instanceof Node && workbenchRoot.contains(event.target)) return;
      if (value === editBaseline) {
        setMode('preview');
        return;
      }
      dispatch('commit', value);
    };
    document.addEventListener('pointerdown', handleOutsidePointer, true);
    editorView = new EditorView({
      parent: editorHost,
      state: EditorState.create({
        doc: value,
        extensions: [
          lineNumbers(),
          highlightActiveLineGutter(),
          highlightSpecialChars(),
          history(),
          foldGutter(),
          drawSelection(),
          dropCursor(),
          indentOnInput(),
          bracketMatching(),
          syntaxHighlighting(defaultHighlightStyle, { fallback: true }),
          markdown(),
          editorPlaceholder(placeholder),
          EditorView.lineWrapping,
          highlightActiveLine(),
          keymap.of([
            { key: 'Mod-s', preventDefault: true, run: () => { dispatch('save', value); return true; } },
            ...defaultKeymap,
            ...historyKeymap,
            indentWithTab
          ]),
          EditorView.contentAttributes.of({ 'aria-label': `${label}编辑器`, spellcheck: 'true' }),
          EditorView.updateListener.of(update => {
            if (!update.docChanged) return;
            value = update.state.doc.toString();
            dispatch('change', value);
          }),
          readonlyCompartment.of(readonlyExtensions(readonly)),
          liveModeCompartment.of(mode === 'live' ? liveMarkdownExtension : []),
          editorTheme
        ]
      })
    });
    return () => {
      document.removeEventListener('pointerdown', handleOutsidePointer, true);
      editorView?.destroy();
      editorView = null;
    };
  });

  function readonlyExtensions(value: boolean) {
    return [EditorState.readOnly.of(value), EditorView.editable.of(!value)];
  }

  class MarkdownBulletWidget extends WidgetType {
    toDOM() {
      const marker = document.createElement('span');
      marker.className = 'cm-live-bullet-widget';
      marker.textContent = '•';
      marker.setAttribute('aria-hidden', 'true');
      return marker;
    }
  }

  class MarkdownQuoteWidget extends WidgetType {
    toDOM() {
      const marker = document.createElement('span');
      marker.className = 'cm-live-quote-widget';
      marker.textContent = '“';
      marker.setAttribute('aria-hidden', 'true');
      return marker;
    }
  }

  class MarkdownTaskWidget extends WidgetType {
    checked: boolean;

    constructor(checked: boolean) {
      super();
      this.checked = checked;
    }

    eq(other: MarkdownTaskWidget) {
      return this.checked === other.checked;
    }

    toDOM() {
      const marker = document.createElement('span');
      marker.className = `cm-live-task-widget${this.checked ? ' checked' : ''}`;
      marker.textContent = this.checked ? '✓' : '';
      marker.setAttribute('aria-hidden', 'true');
      return marker;
    }
  }

  function addInlineDecorations(
    lineFrom: number,
    text: string,
    decorations: Range<Decoration>[],
    occupied: Array<[number, number]>
  ) {
    const available = (from: number, to: number) => !occupied.some(([start, end]) => from < end && to > start);
    const reserve = (from: number, to: number) => occupied.push([from, to]);
    const hide = (from: number, to: number) => {
      if (to > from) decorations.push(Decoration.replace({}).range(lineFrom + from, lineFrom + to));
    };
    const mark = (from: number, to: number, className: string) => {
      if (to > from) decorations.push(Decoration.mark({ class: className }).range(lineFrom + from, lineFrom + to));
    };

    for (const match of text.matchAll(/(`+)([^`\n]+?)\1/g)) {
      const start = match.index ?? 0;
      const end = start + match[0].length;
      if (!available(start, end)) continue;
      const markerLength = match[1].length;
      hide(start, start + markerLength);
      mark(start + markerLength, end - markerLength, 'cm-live-inline-code');
      hide(end - markerLength, end);
      reserve(start, end);
    }

    for (const match of text.matchAll(/\[([^\]\n]+)\]\(([^)\n]+)\)/g)) {
      const start = match.index ?? 0;
      const end = start + match[0].length;
      if (!available(start, end)) continue;
      const labelStart = start + 1;
      const labelEnd = labelStart + match[1].length;
      hide(start, labelStart);
      mark(labelStart, labelEnd, 'cm-live-link');
      hide(labelEnd, end);
      reserve(start, end);
    }

    for (const match of text.matchAll(/(\*\*|__)(.+?)\1/g)) {
      const start = match.index ?? 0;
      const end = start + match[0].length;
      if (!available(start, end)) continue;
      const markerLength = match[1].length;
      hide(start, start + markerLength);
      mark(start + markerLength, end - markerLength, 'cm-live-strong');
      hide(end - markerLength, end);
      reserve(start, end);
    }

    for (const match of text.matchAll(/~~([^~\n]+)~~/g)) {
      const start = match.index ?? 0;
      const end = start + match[0].length;
      if (!available(start, end)) continue;
      hide(start, start + 2);
      mark(start + 2, end - 2, 'cm-live-strike');
      hide(end - 2, end);
      reserve(start, end);
    }

    for (const match of text.matchAll(/(^|[\s(])([*_])([^*_\n]+)\2(?=$|[\s).,!?，。！？])/g)) {
      const matchStart = match.index ?? 0;
      const prefixLength = match[1].length;
      const start = matchStart + prefixLength;
      const end = matchStart + match[0].length;
      if (!available(start, end)) continue;
      hide(start, start + 1);
      mark(start + 1, end - 1, 'cm-live-emphasis');
      hide(end - 1, end);
      reserve(start, end);
    }
  }

  function buildLiveMarkdownDecorations(view: EditorView): DecorationSet {
    const decorations: Range<Decoration>[] = [];
    const activeLine = view.state.doc.lineAt(view.state.selection.main.head).number;

    for (const visibleRange of view.visibleRanges) {
      let line = view.state.doc.lineAt(visibleRange.from);
      let inFence = false;
      for (let number = 1; number < line.number; number += 1) {
        if (/^\s*(```|~~~)/.test(view.state.doc.line(number).text)) inFence = !inFence;
      }

      while (line.from <= visibleRange.to) {
        const text = line.text;
        const isFence = /^\s*(```|~~~)/.test(text);
        const currentLineInFence = inFence || isFence;
        if (isFence) inFence = !inFence;

        if (!currentLineInFence && text) {
          const isActive = line.number === activeLine;
          const heading = /^(#{1,6})\s+/.exec(text);
          const quote = /^(\s*>\s?)/.exec(text);
          const task = /^(\s*)[-*+]\s+\[([ xX])\]\s+/.exec(text);
          const bullet = task ? null : /^(\s*)[-*+]\s+/.exec(text);

          if (heading) {
            decorations.push(Decoration.line({ class: `cm-live-heading cm-live-h${heading[1].length}` }).range(line.from));
            if (!isActive) decorations.push(Decoration.replace({}).range(line.from, line.from + heading[0].length));
          } else if (quote) {
            decorations.push(Decoration.line({ class: 'cm-live-quote-line' }).range(line.from));
            if (!isActive) {
              decorations.push(Decoration.replace({ widget: new MarkdownQuoteWidget() }).range(line.from, line.from + quote[0].length));
            }
          } else if (task) {
            decorations.push(Decoration.line({ class: 'cm-live-list-line cm-live-task-line' }).range(line.from));
            if (!isActive) {
              decorations.push(Decoration.replace({ widget: new MarkdownTaskWidget(task[2].toLowerCase() === 'x') }).range(line.from, line.from + task[0].length));
            }
          } else if (bullet) {
            decorations.push(Decoration.line({ class: 'cm-live-list-line' }).range(line.from));
            if (!isActive) {
              decorations.push(Decoration.replace({ widget: new MarkdownBulletWidget() }).range(line.from, line.from + bullet[0].length));
            }
          }

          if (!isActive) addInlineDecorations(line.from, text, decorations, []);
        }

        if (line.to >= visibleRange.to || line.number >= view.state.doc.lines) break;
        line = view.state.doc.line(line.number + 1);
      }
    }

    return Decoration.set(decorations, true);
  }

  const liveMarkdownExtension = ViewPlugin.fromClass(class {
    decorations: DecorationSet;

    constructor(view: EditorView) {
      this.decorations = buildLiveMarkdownDecorations(view);
    }

    update(update: ViewUpdate) {
      if (update.docChanged || update.viewportChanged || update.selectionSet) {
        this.decorations = buildLiveMarkdownDecorations(update.view);
      }
    }
  }, {
    decorations: plugin => plugin.decorations
  });

  function renderMarkdown(source: string) {
    const html = marked.parse(String(source || ''), { async: false, gfm: true, breaks: false }) as string;
    return DOMPurify.sanitize(html, {
      USE_PROFILES: { html: true },
      ADD_ATTR: ['target', 'rel']
    });
  }

  function setMode(nextMode: MarkdownMode) {
    if (nextMode === 'edit') editBaseline = value;
    mode = nextMode;
    if (nextMode !== 'preview') requestAnimationFrame(() => editorView?.focus());
  }

  function beginInlineEdit() {
    if (!inlineEditing || readonly || saving || mode !== 'preview') return;
    setMode('edit');
  }

  function previewActivation(node: HTMLElement) {
    const handleDoubleClick = () => beginInlineEdit();
    node.addEventListener('dblclick', handleDoubleClick);
    return {
      destroy() {
        node.removeEventListener('dblclick', handleDoubleClick);
      }
    };
  }

  const editorTheme = EditorView.theme({
    '&': {
      height: '100%',
      backgroundColor: 'transparent',
      color: 'var(--wa-text-main, #293847)',
      fontSize: '13px'
    },
    '&.cm-focused': { outline: 'none' },
    '.cm-scroller': {
      fontFamily: 'var(--wa-font-mono, ui-monospace, SFMono-Regular, Menlo, monospace)',
      lineHeight: '1.68',
      padding: '18px 0'
    },
    '.cm-content': { padding: '0 20px', caretColor: 'var(--wa-accent, #008f96)' },
    '.cm-line': { padding: '0 2px' },
    '.cm-live-heading': {
      color: 'var(--wa-text-strong, #0d1722)',
      fontFamily: 'var(--wa-font-sans, ui-sans-serif, system-ui, sans-serif)',
      fontWeight: '780',
      letterSpacing: '-0.015em'
    },
    '.cm-live-h1': { paddingTop: '8px', paddingBottom: '8px', fontSize: '26px', lineHeight: '1.25', letterSpacing: '-0.03em' },
    '.cm-live-h2': { paddingTop: '7px', paddingBottom: '5px', fontSize: '19px', lineHeight: '1.35' },
    '.cm-live-h3': { paddingTop: '5px', paddingBottom: '3px', fontSize: '16px', lineHeight: '1.4' },
    '.cm-live-h4, .cm-live-h5, .cm-live-h6': { paddingTop: '4px', paddingBottom: '2px', fontSize: '14px', lineHeight: '1.45' },
    '.cm-live-strong': { color: 'var(--wa-text-strong, #0d1722)', fontWeight: '760' },
    '.cm-live-emphasis': { fontStyle: 'italic' },
    '.cm-live-strike': { color: 'var(--wa-text-muted, #667789)', textDecoration: 'line-through' },
    '.cm-live-inline-code': {
      borderRadius: '4px',
      padding: '2px 5px',
      backgroundColor: '#edf3f4',
      color: '#176b63',
      fontFamily: 'var(--wa-font-mono, ui-monospace, SFMono-Regular, Menlo, monospace)',
      fontSize: '.92em'
    },
    '.cm-live-link': { color: 'var(--wa-accent-strong, #006f76)', textDecoration: 'underline', textUnderlineOffset: '3px' },
    '.cm-live-quote-line': {
      borderRadius: '5px',
      paddingLeft: '10px',
      backgroundColor: 'rgba(0, 143, 150, .045)',
      color: '#49636a',
      fontFamily: 'var(--wa-font-sans, ui-sans-serif, system-ui, sans-serif)',
      fontStyle: 'italic'
    },
    '.cm-live-quote-widget': {
      display: 'inline-block',
      width: '20px',
      color: 'var(--wa-accent, #008f96)',
      fontFamily: 'Georgia, serif',
      fontSize: '20px',
      fontStyle: 'normal',
      fontWeight: '700',
      lineHeight: '1'
    },
    '.cm-live-list-line': { paddingLeft: '8px', fontFamily: 'var(--wa-font-sans, ui-sans-serif, system-ui, sans-serif)' },
    '.cm-live-bullet-widget': { display: 'inline-block', width: '18px', color: 'var(--wa-accent, #008f96)', fontWeight: '800', textAlign: 'center' },
    '.cm-live-task-widget': {
      width: '14px',
      height: '14px',
      display: 'inline-grid',
      placeItems: 'center',
      margin: '0 7px 0 2px',
      border: '1px solid rgba(69, 95, 118, .38)',
      borderRadius: '4px',
      backgroundColor: '#fbfdfe',
      color: '#fbfdfe',
      fontSize: '10px',
      fontWeight: '800',
      lineHeight: '1'
    },
    '.cm-live-task-widget.checked': { borderColor: 'var(--wa-accent, #008f96)', backgroundColor: 'var(--wa-accent, #008f96)' },
    '.cm-gutters': {
      backgroundColor: 'transparent',
      color: 'var(--wa-text-subtle, #8a99aa)',
      border: '0',
      paddingLeft: '8px'
    },
    '.cm-activeLine, .cm-activeLineGutter': { backgroundColor: 'rgba(0, 143, 150, 0.055)' },
    '.cm-selectionBackground, &.cm-focused .cm-selectionBackground': { backgroundColor: 'rgba(0, 143, 150, 0.16)' },
    '.cm-cursor': { borderLeftColor: 'var(--wa-accent, #008f96)', borderLeftWidth: '2px' },
    '.cm-placeholder': { color: 'var(--wa-text-muted, #667789)', fontStyle: 'normal' }
  });
</script>

<section
  class:streaming
  class:readonly
  class:toolbar-hidden={!showToolbar}
  class:workbench-header-hidden={!showInlineEditStatus && !showWorkbenchToolbar}
  class:embedded
  class:full-width-preview={fullWidthPreview}
  class:inline-editable={inlineEditing && !readonly}
  class:editing-inline={inlineEditing && mode === 'edit'}
  class:auto-height={autoHeight}
  class="markdown-workbench mode-{mode}"
  style:--workbench-height="{minHeight}px"
  aria-label={label}
  aria-busy={saving}
  bind:this={workbenchRoot}
>
  {#if showInlineEditStatus}
    <header class="inline-edit-status" aria-live="polite">
      <div>
        <strong>{saving ? '正在保存 Markdown…' : '正在编辑 Markdown'}</strong>
        <span>点击文档外的空白区域即可临时保存并退出</span>
      </div>
      <span>{lineCount} 行 · {value.length} 字符</span>
    </header>
  {:else if showWorkbenchToolbar}
    <header class:meta-hidden={!showDocumentMeta} class="workbench-toolbar">
      {#if showDocumentMeta}
        <div class="document-identity">
          <strong>{label}</strong>
          <span>{description}</span>
        </div>
      {/if}
      {#if toolbarModes.length > 1}
        <div class="mode-switch" aria-label="Markdown 显示模式">
          {#each toolbarModes as toolbarMode}
            <button
              type="button"
              class:active={mode === toolbarMode}
              aria-pressed={mode === toolbarMode}
              on:click={() => setMode(toolbarMode)}
            >{modeLabels[toolbarMode]}</button>
          {/each}
        </div>
      {/if}
    </header>
  {/if}

  <div class="workbench-body">
    {#if inlineEditing && mode === 'preview' && !readonly}
      <button class="preview-edit-access" type="button" on:click={beginInlineEdit}>进入 Markdown 编辑</button>
    {/if}
    <div class="editor-pane" aria-hidden={mode === 'preview'}>
      <div class="editor-host" bind:this={editorHost}></div>
    </div>
    <article
      class="preview-pane markdown-body"
      aria-label={`${label}预览`}
      title={inlineEditing && !readonly ? '双击编辑 Markdown' : undefined}
      use:previewActivation
    >
      {#if value.trim()}
        {@html rendered}
      {:else}
        <p class="empty-document">暂无 Markdown 内容</p>
      {/if}
    </article>
  </div>

  {#if showToolbar && mode !== 'preview'}
    <footer class="workbench-status" aria-live="polite">
      <span>{mode === 'live' ? '即时排版 · 当前行显示语法' : 'Markdown'}</span>
      <span>{lineCount} 行 · {value.length} 字符</span>
      {#if !readonly}<span>⌘S 保存</span>{/if}
    </footer>
  {/if}
</section>

<style>
  .markdown-workbench {
    min-width: 0;
    height: var(--workbench-height);
    display: grid;
    grid-template-rows: auto minmax(0, 1fr) auto;
    overflow: hidden;
    border: 1px solid var(--wa-border-soft, rgba(123, 143, 160, .18));
    border-radius: var(--wa-radius-md, 10px);
    background: var(--wa-surface-flat, #fbfdfe);
    color: var(--wa-text-main, #293847);
  }
  .markdown-workbench.embedded {
    border: 0;
    border-radius: 0;
    background: transparent;
  }
  .workbench-toolbar {
    min-height: 54px;
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 16px;
    padding: 8px 10px 8px 16px;
    border-bottom: 1px solid var(--wa-border-soft, rgba(123, 143, 160, .18));
    background: var(--wa-surface-inset, #f5f8fb);
  }
  .workbench-toolbar.meta-hidden { justify-content: flex-end; }
  .inline-edit-status {
    min-height: 48px;
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 16px;
    padding: 8px 16px;
    border-bottom: 1px solid var(--wa-border-soft, rgba(123, 143, 160, .18));
    background: var(--wa-surface-inset, #f5f8fb);
    color: var(--wa-text-muted, #667789);
    font-size: 10px;
  }
  .inline-edit-status > div { min-width: 0; display: grid; gap: 4px; }
  .inline-edit-status strong { color: var(--wa-text-strong, #0d1722); font-size: 12px; }
  .inline-edit-status > span { flex: 0 0 auto; font-family: var(--wa-font-mono, monospace); }
  .document-identity { min-width: 0; display: grid; gap: 2px; }
  .document-identity strong { color: var(--wa-text-strong, #0d1722); font-size: 12px; line-height: 1.35; }
  .document-identity span { overflow: hidden; color: var(--wa-text-muted, #667789); font-size: 10px; line-height: 1.35; text-overflow: ellipsis; white-space: nowrap; }
  .mode-switch { display: inline-flex; align-items: center; gap: 2px; padding: 2px; border: 1px solid var(--wa-border-soft, rgba(123, 143, 160, .18)); border-radius: 8px; background: #fbfdfe; }
  .mode-switch button { min-height: 30px; border: 0; border-radius: 6px; padding: 0 10px; background: transparent; color: var(--wa-text-muted, #667789); font: 700 11px/30px var(--wa-font-sans, sans-serif); cursor: pointer; }
  .mode-switch button:hover { color: var(--wa-text-main, #293847); background: var(--wa-row-hover, #f2f8fb); }
  .mode-switch button.active { color: var(--wa-accent-strong, #006f76); background: var(--wa-accent-soft, rgba(0, 143, 150, .12)); }
  .mode-switch button:focus-visible { outline: 2px solid rgba(0, 143, 150, .24); outline-offset: 1px; }
  .workbench-body { position: relative; min-height: 0; display: grid; grid-template-columns: minmax(0, 1fr) minmax(0, 1fr); }
  .preview-edit-access {
    position: absolute;
    width: 1px;
    height: 1px;
    margin: -1px;
    padding: 0;
    overflow: hidden;
    clip: rect(0 0 0 0);
    clip-path: inset(50%);
    border: 0;
    white-space: nowrap;
  }
  .preview-edit-access:focus-visible {
    z-index: 1;
    top: 12px;
    right: 12px;
    width: auto;
    height: 36px;
    margin: 0;
    padding: 0 12px;
    clip: auto;
    clip-path: none;
    border: 1px solid var(--wa-border-focus, rgba(0, 143, 150, .86));
    border-radius: 8px;
    background: #fbfdfe;
    color: var(--wa-accent-strong, #006f76);
    font: 700 11px/34px var(--wa-font-sans, sans-serif);
    outline: 2px solid rgba(0, 143, 150, .2);
    outline-offset: 2px;
  }
  .editor-pane, .preview-pane { min-width: 0; min-height: 0; overflow: auto; }
  .editor-pane { border-right: 1px solid var(--wa-border-soft, rgba(123, 143, 160, .18)); background: #fbfcfd; }
  .embedded .editor-pane, .embedded .preview-pane { background: transparent; }
  .editor-host { height: 100%; min-height: inherit; }
  .mode-live .workbench-body, .mode-edit .workbench-body, .mode-preview .workbench-body { grid-template-columns: minmax(0, 1fr); }
  .mode-live .preview-pane, .mode-edit .preview-pane, .mode-preview .editor-pane { display: none; }
  .mode-live .editor-pane, .mode-edit .editor-pane { border-right: 0; }
  .inline-editable.mode-preview .preview-pane { cursor: text; }
  .preview-pane { box-sizing: border-box; padding: clamp(24px, 4vw, 48px); background: #fbfdfe; overflow-wrap: anywhere; }
  .markdown-body { color: #33434d; font-size: 13px; line-height: 1.72; }
  .markdown-body :global(> :first-child) { margin-top: 0; }
  .markdown-body :global(> :last-child) { margin-bottom: 0; }
  .markdown-body :global(h1) { margin: 0 0 24px; padding-bottom: 14px; border-bottom: 1px solid #e5eaec; color: #17252e; font-size: 28px; line-height: 1.2; letter-spacing: -.03em; text-wrap: balance; }
  .markdown-body :global(h2) { margin: 30px 0 11px; color: #1f333c; font-size: 17px; line-height: 1.35; letter-spacing: -.015em; text-wrap: balance; }
  .markdown-body :global(h3) { margin: 23px 0 8px; color: #2c414a; font-size: 14px; line-height: 1.4; }
  .markdown-body :global(p) { max-width: 75ch; margin: 0 0 13px; text-wrap: pretty; }
  .markdown-body :global(a) { color: var(--wa-accent-strong, #006f76); text-decoration-thickness: 1px; text-underline-offset: 3px; }
  .markdown-body :global(ul), .markdown-body :global(ol) { max-width: 75ch; margin: 8px 0 17px; padding-left: 23px; }
  .markdown-body :global(li) { margin: 5px 0; padding-left: 3px; }
  .markdown-body :global(li::marker) { color: var(--wa-accent, #008f96); font-weight: 700; }
  .markdown-body :global(blockquote) { position: relative; max-width: 72ch; margin: 0 0 20px; padding: 4px 0 4px 24px; color: #49636a; }
  .markdown-body :global(blockquote::before) { content: '“'; position: absolute; left: 0; top: -4px; color: var(--wa-accent, #008f96); font: 700 24px/1 Georgia, serif; }
  .markdown-body :global(code) { border-radius: 4px; padding: 2px 5px; background: #edf3f4; color: #176b63; font: .9em/1.5 var(--wa-font-mono, monospace); }
  .markdown-body :global(pre) { max-width: 100%; margin: 14px 0 20px; padding: 16px; border: 1px solid #dce4e7; border-radius: 8px; background: #f5f8f9; overflow-x: auto; }
  .markdown-body :global(pre code) { padding: 0; background: transparent; color: #314750; }
  .markdown-body :global(table) { width: max-content; min-width: 100%; margin: 14px 0 20px; border-collapse: collapse; font-size: 12px; }
  .markdown-body :global(th), .markdown-body :global(td) { padding: 9px 11px; border: 1px solid #dfe6e9; text-align: left; vertical-align: top; }
  .markdown-body :global(th) { background: #f3f7f7; color: #31454e; font-weight: 750; }
  .markdown-body :global(hr) { height: 1px; margin: 26px 0; border: 0; background: #e3e8eb; }
  .markdown-body :global(input[type='checkbox']) { margin-right: 7px; accent-color: var(--wa-accent, #008f96); }
  .full-width-preview .markdown-body :global(p),
  .full-width-preview .markdown-body :global(ul),
  .full-width-preview .markdown-body :global(ol),
  .full-width-preview .markdown-body :global(blockquote) { max-width: none; }
  .empty-document { color: var(--wa-text-muted, #667789); }
  .workbench-status { min-height: 28px; display: flex; align-items: center; justify-content: flex-end; gap: 14px; padding: 0 12px; border-top: 1px solid var(--wa-border-soft, rgba(123, 143, 160, .18)); background: var(--wa-surface-inset, #f5f8fb); color: var(--wa-text-muted, #667789); font: 10px/1.2 var(--wa-font-mono, monospace); }
  .streaming .preview-pane::after { content: ''; display: inline-block; width: 2px; height: 1.05em; margin-left: 3px; border-radius: 2px; background: var(--wa-accent, #008f96); vertical-align: -.15em; animation: caret-blink .8s steps(1) infinite; }
  .toolbar-hidden { grid-template-rows: minmax(0, 1fr); }
  .toolbar-hidden.editing-inline { grid-template-rows: auto minmax(0, 1fr); }
  .workbench-header-hidden { grid-template-rows: minmax(0, 1fr) auto; }
  .toolbar-hidden.workbench-header-hidden { grid-template-rows: minmax(0, 1fr); }
  .auto-height.mode-preview {
    height: auto;
    min-height: var(--workbench-height);
    grid-template-rows: auto auto auto;
    overflow: visible;
  }
  .auto-height.mode-preview.toolbar-hidden { grid-template-rows: auto; }
  .auto-height.mode-preview.workbench-header-hidden { grid-template-rows: auto; }
  .auto-height.mode-preview .workbench-body,
  .auto-height.mode-preview .preview-pane {
    min-height: var(--workbench-height);
  }
  .auto-height.mode-preview .preview-pane { overflow: visible; }
  @keyframes caret-blink { 50% { opacity: 0; } }
  @media (max-width: 820px) {
    .workbench-toolbar { align-items: stretch; flex-direction: column; padding: 11px 12px; }
    .document-identity span { white-space: normal; }
    .mode-switch { align-self: flex-start; }
    .mode-switch button { min-height: 44px; padding: 0 8px; line-height: 44px; }
    .mode-split { height: calc(var(--workbench-height) + 240px); }
    .mode-split .workbench-body { grid-template-columns: minmax(0, 1fr); grid-template-rows: repeat(2, minmax(0, 1fr)); }
    .mode-split .editor-pane { border-right: 0; border-bottom: 1px solid var(--wa-border-soft, rgba(123, 143, 160, .18)); }
    .preview-pane { padding: 22px 18px; }
    .inline-edit-status { align-items: flex-start; flex-direction: column; gap: 4px; padding: 8px 12px; }
    .markdown-body :global(h1) { font-size: 23px; }
  }
  @media (max-width: 480px) {
    .mode-switch { width: 100%; }
    .mode-switch button { min-width: 0; flex: 1 1 0; padding: 0 5px; }
  }
  @media (prefers-reduced-motion: reduce) {
    .streaming .preview-pane::after { animation: none; }
  }
</style>
