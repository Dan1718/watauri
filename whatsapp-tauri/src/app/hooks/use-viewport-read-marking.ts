import { useCallback, useEffect, useState } from "react";
import { useChatPollingActive } from "./use-chat-polling-active";
import { canReadConversation, visibleUnreadIds } from "../read-marking/viewport";

export function useViewportReadMarking(
  chatId: string,
  markVisibleMessagesRead: (chatId: string, ids: string[]) => void,
) {
  const [scroller, setScroller] = useState<HTMLElement | null>(null);
  const active = useChatPollingActive();

  useEffect(() => {
    if (!scroller || !active) return;
    let timer: ReturnType<typeof setTimeout> | undefined;
    const flush = () => {
      timer = undefined;
      if (!canReadConversation(active, document.visibilityState, document.hasFocus())) return;
      const ids = visibleUnreadIds(scroller, window.innerWidth, window.innerHeight);
      markVisibleMessagesRead(chatId, ids);
    };
    const schedule = () => {
      if (timer === undefined) timer = setTimeout(flush, 200);
    };
    const observer = new MutationObserver(schedule);
    observer.observe(scroller, { childList: true, subtree: true, attributes: true, attributeFilter: ["data-unread-message-id", "style"] });
    const resize = new ResizeObserver(schedule);
    resize.observe(scroller);
    scroller.addEventListener("scroll", schedule, { passive: true });
    window.addEventListener("focus", schedule);
    window.addEventListener("resize", schedule);
    document.addEventListener("visibilitychange", schedule);
    // Retry failures and rows that became visible while a previous batch was in flight.
    const retry = setInterval(schedule, 1000);
    schedule();
    return () => {
      clearTimeout(timer);
      clearInterval(retry);
      observer.disconnect();
      resize.disconnect();
      scroller.removeEventListener("scroll", schedule);
      window.removeEventListener("focus", schedule);
      window.removeEventListener("resize", schedule);
      document.removeEventListener("visibilitychange", schedule);
    };
  }, [active, chatId, markVisibleMessagesRead, scroller]);

  return useCallback((element: HTMLElement | Window | null) => {
    setScroller(element instanceof HTMLElement ? element : null);
  }, []);
}
