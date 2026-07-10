export function resetSettingsWorkspaceScroll() {
  requestAnimationFrame(() => {
    document.querySelector<HTMLElement>('.workspace-frame')?.scrollTo({
      top: 0,
      behavior: 'auto'
    });
  });
}
