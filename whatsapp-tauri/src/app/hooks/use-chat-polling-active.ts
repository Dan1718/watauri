import { useEffect, useState } from "react";
import { useTab } from "./use-tab";

export function useChatPollingActive() {
  const { selectedTab } = useTab();
  const [visible, setVisible] = useState(() =>
    typeof document === "undefined" || document.visibilityState === "visible"
  );

  useEffect(() => {
    const update = () => setVisible(document.visibilityState === "visible");
    document.addEventListener("visibilitychange", update);
    return () => document.removeEventListener("visibilitychange", update);
  }, []);

  return selectedTab === "chats" && visible;
}
