"use client";

import { createContext, PropsWithChildren, useCallback, useContext, useEffect, useMemo, useRef, useState } from "react";

const API_BASE = "http://localhost:8090";

export type RealtimeNotification = {
  type: "message" | "receipt" | "chat" | "contact" | "sync" | "resync" | "connected";
  chatId?: string;
  messageIds?: string[];
  clientSendId?: string;
  progress?: number;
  syncType?: string;
};

type RealtimeContextValue = {
  connected: boolean;
  historySync: { active: boolean; progress: number; label: string };
  subscribe: (listener: (event: RealtimeNotification) => void) => () => void;
};

export const RealtimeContext = createContext<RealtimeContextValue | undefined>(undefined);

export function useRealtime() {
  const value = useContext(RealtimeContext);
  if (!value) throw new Error("useRealtime must be used within a RealtimeProvider");
  return value;
}

export default function RealtimeProvider({ children }: PropsWithChildren) {
  const [connected, setConnected] = useState(false);
  const [historySyncs, setHistorySyncs] = useState<Record<string, number>>({});
  const listenersRef = useRef(new Set<(event: RealtimeNotification) => void>());

  const subscribe = useCallback((listener: (event: RealtimeNotification) => void) => {
    listenersRef.current.add(listener);
    return () => {
      listenersRef.current.delete(listener);
    };
  }, []);

  useEffect(() => {
    if (typeof EventSource === "undefined") return;
    const source = new EventSource(`${API_BASE}/api/events`);
    const notify = (event: RealtimeNotification) => {
      if (event.type === "sync" && event.syncType && event.progress !== undefined) {
        setHistorySyncs((current) => {
          if (event.progress! >= 100) {
            if (!(event.syncType! in current)) return current;
            const next = { ...current };
            delete next[event.syncType!];
            return next;
          }
          return { ...current, [event.syncType!]: Math.max(0, Math.min(100, event.progress!)) };
        });
      }
      for (const listener of listenersRef.current) listener(event);
    };

    source.onopen = () => {
      setConnected(true);
      notify({ type: "connected" });
    };
    source.onerror = () => setConnected(false);

    const eventTypes: RealtimeNotification["type"][] = ["message", "receipt", "chat", "contact", "sync", "resync"];
    const handlers = eventTypes.map((type) => {
      const handler = (message: Event) => {
        try {
          const payload = JSON.parse((message as MessageEvent<string>).data) as RealtimeNotification;
          notify({ ...payload, type });
        } catch {
          // Ignore malformed events without taking down the stream.
        }
      };
      source.addEventListener(type, handler);
      return [type, handler] as const;
    });

    return () => {
      source.close();
      for (const [type, handler] of handlers) source.removeEventListener(type, handler);
    };
  }, []);

  const syncProgress = Object.values(historySyncs);
  const historySync = useMemo(() => ({
    active: syncProgress.length > 0,
    progress: syncProgress.length
      ? Math.round(syncProgress.reduce((sum, progress) => sum + progress, 0) / syncProgress.length)
      : 0,
    label: Object.keys(historySyncs).length === 1
      ? `Syncing ${Object.keys(historySyncs)[0].toLowerCase().replaceAll("_", " ")}`
      : "Syncing history",
  }), [historySyncs]);
  const value = useMemo(() => ({ connected, historySync, subscribe }), [connected, historySync, subscribe]);
  return <RealtimeContext.Provider value={value}>{children}</RealtimeContext.Provider>;
}
