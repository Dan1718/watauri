export type ViewportRect = Pick<DOMRectReadOnly, "top" | "bottom" | "left" | "right">;

export function canReadConversation(active: boolean, visibility: string, focused: boolean) {
  return active && visibility === "visible" && focused;
}

// Positive intersection, clipped to both the scroller and the browser window.
// Touching an edge or being mounted in Virtuoso overscan does not count.
export function intersectsViewport(row: ViewportRect, viewport: ViewportRect, width: number, height: number) {
  return Math.min(row.bottom, viewport.bottom, height) > Math.max(row.top, viewport.top, 0) &&
    Math.min(row.right, viewport.right, width) > Math.max(row.left, viewport.left, 0);
}

export function visibleUnreadIds(scroller: HTMLElement, width: number, height: number) {
  const viewport = scroller.getBoundingClientRect();
  return Array.from(scroller.querySelectorAll<HTMLElement>("[data-unread-message-id]"))
    .filter((row) => intersectsViewport(row.getBoundingClientRect(), viewport, width, height))
    .map((row) => row.dataset.unreadMessageId!)
    .filter(Boolean);
}
