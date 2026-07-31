export type SelectCloseReason = 'outside-pointer' | 'escape';

type SelectLifecycleOptions = {
  isOpen: () => boolean;
  close: (reason: SelectCloseReason) => void;
  ownsTarget?: (target: Node) => boolean;
};

export function registerSelectLifecycle(container: HTMLElement, options: SelectLifecycleOptions) {
  function handlePointerDown(event: PointerEvent) {
    const target = event.target as Node;
    if (options.isOpen() && !container.contains(target) && !options.ownsTarget?.(target)) {
      options.close('outside-pointer');
    }
  }

  function handleKeydown(event: KeyboardEvent) {
    if (!options.isOpen() || event.key !== 'Escape') return;
    event.preventDefault();
    options.close('escape');
  }

  window.addEventListener('pointerdown', handlePointerDown, true);
  window.addEventListener('keydown', handleKeydown);

  return () => {
    window.removeEventListener('pointerdown', handlePointerDown, true);
    window.removeEventListener('keydown', handleKeydown);
  };
}

export function focusLeftSelect(
  container: HTMLElement,
  event: FocusEvent,
  additionalContainers: Array<HTMLElement | undefined> = []
) {
  const nextTarget = event.relatedTarget as Node | null;
  return (
    !nextTarget ||
    (!container.contains(nextTarget) &&
      !additionalContainers.some((additionalContainer) => additionalContainer?.contains(nextTarget)))
  );
}
