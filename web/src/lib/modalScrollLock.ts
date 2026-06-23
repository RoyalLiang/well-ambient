let lockCount = 0;
let previousBodyOverflow = '';
let previousBodyPosition = '';
let previousBodyTop = '';
let previousBodyWidth = '';
let previousHtmlOverscroll = '';
let lockedScrollY = 0;

export function lockBodyScroll() {
  if (typeof document === 'undefined') return;

  if (lockCount === 0) {
    lockedScrollY = window.scrollY || document.documentElement.scrollTop || 0;
    previousBodyOverflow = document.body.style.overflow;
    previousBodyPosition = document.body.style.position;
    previousBodyTop = document.body.style.top;
    previousBodyWidth = document.body.style.width;
    previousHtmlOverscroll = document.documentElement.style.overscrollBehavior;
    document.body.style.overflow = 'hidden';
    document.body.style.position = 'fixed';
    document.body.style.top = `-${lockedScrollY}px`;
    document.body.style.width = '100%';
    document.documentElement.style.overscrollBehavior = 'none';
  }

  lockCount += 1;
}

export function unlockBodyScroll() {
  if (typeof document === 'undefined' || lockCount === 0) return;

  lockCount -= 1;
  if (lockCount === 0) {
    document.body.style.overflow = previousBodyOverflow;
    document.body.style.position = previousBodyPosition;
    document.body.style.top = previousBodyTop;
    document.body.style.width = previousBodyWidth;
    document.documentElement.style.overscrollBehavior = previousHtmlOverscroll;
    window.scrollTo(0, lockedScrollY);
    previousBodyOverflow = '';
    previousBodyPosition = '';
    previousBodyTop = '';
    previousBodyWidth = '';
    previousHtmlOverscroll = '';
    lockedScrollY = 0;
  }
}
