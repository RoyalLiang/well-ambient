// HTTP intranet pages may not expose the secure-context Clipboard API.
export async function copyToClipboard(text: string): Promise<boolean> {
  if (typeof navigator !== 'undefined' && navigator.clipboard?.writeText) {
    try {
      await navigator.clipboard.writeText(text);
      return true;
    } catch {
      // A denied permission can still allow copying during a click gesture.
    }
  }
  if (typeof document === 'undefined') return false;
  const focused = document.activeElement instanceof HTMLElement ? document.activeElement : null;
  const textarea = document.createElement('textarea');
  textarea.value = text;
  textarea.readOnly = true;
  textarea.tabIndex = -1;
  textarea.style.cssText = 'position:fixed;inset:0 auto auto 0;opacity:0;pointer-events:none;';
  document.body.appendChild(textarea);
  try {
    textarea.focus({ preventScroll: true });
    textarea.select();
    return document.execCommand('copy');
  } catch {
    return false;
  } finally {
    textarea.remove();
    focused?.focus({ preventScroll: true });
  }
}
