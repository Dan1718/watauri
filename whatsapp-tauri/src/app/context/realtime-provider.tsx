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
          // A malformed event should not take down the stream or prevent fallback polling.
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

  const value = useMemo(() => ({ connected, subscribe }), [connected, subscribe]);
  return <RealtimeContext.Provider value={value}>{children}</RealtimeContext.Provider>;
}
