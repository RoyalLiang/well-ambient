import DOMPurify from 'dompurify';

// Both the gallery thumbnail and full preview render the server's email HTML.
// Block embedded network/executable content; only explicit HTTP(S) links may open a new tab.
export function safeEmailPreviewHTML(html: string): string {
  const clean = DOMPurify.sanitize(html, {
    WHOLE_DOCUMENT: true,
    USE_PROFILES: { html: true, svg: true, svgFilters: true },
    ADD_TAGS: [
      'style', 'defs', 'linearGradient', 'radialGradient', 'stop',
      'svg', 'circle', 'path', 'line', 'text', 'rect', 'tspan', 'g'
    ],
    ADD_ATTR: [
      'viewBox', 'xmlns', 'd', 'fill', 'fill-opacity', 'stroke', 'stroke-width',
      'stroke-dasharray', 'stroke-linecap', 'stroke-linejoin', 'stroke-opacity',
      'transform', 'text-anchor', 'font-size', 'font-weight', 'font-family',
      'x', 'y', 'x1', 'y1', 'x2', 'y2', 'cx', 'cy', 'r', 'rx', 'ry',
      'width', 'height', 'stop-color', 'stop-opacity', 'offset', 'role', 'aria-label'
    ]
  });
  const policy = `<meta http-equiv="Content-Security-Policy" content="default-src 'none'; style-src 'unsafe-inline'; img-src data:; base-uri 'none'; form-action 'none'">`;
  const head = /<head(?:\s[^>]*)?>/i;
  if (!head.test(clean)) return `<!doctype html><html><head>${policy}</head><body>预览内容无法安全显示，请重新加载。</body></html>`;
  const preview = new DOMParser().parseFromString(clean, 'text/html');
  for (const link of preview.querySelectorAll('a')) {
    const href = link.getAttribute('href') || '';
    try {
      const url = new URL(href);
      if (!['http:', 'https:'].includes(url.protocol) || !url.hostname || url.username || url.password) {
        throw new Error('Unsupported preview link');
      }
      link.setAttribute('href', url.href);
      link.setAttribute('target', '_blank');
      link.setAttribute('rel', 'noopener noreferrer');
    } catch {
      link.removeAttribute('href');
      link.removeAttribute('target');
    }
  }
  return `<!doctype html>${preview.documentElement.outerHTML}`.replace(head, match => `${match}${policy}`);
}
