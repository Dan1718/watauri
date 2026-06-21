import type { Chat, Contact, Message } from "../types";

const BASE = "http://localhost:8090";

export interface BackendHealth {
  name: string;
  status: string;
  mode: string;
}

async function fetchJson<T>(url: string, options?: RequestInit): Promise<T> {
  const start = performance.now();
  const label = `${options?.method || "GET"} ${url}`;
  console.log(`[api] ➡️ ${label}`);

  try {
    const res = await fetch(url, options);
    const duration = (performance.now() - start).toFixed(1);
    const text = await res.text();

    if (!res.ok) {
      console.error(`[api] ❌ ${label} -> ${res.status} ${res.statusText} (${duration}ms) body=${text.slice(0, 300)}`);
      throw new Error(`${label} failed: ${res.status} ${res.statusText}`);
    }

    let parsed: T;
    try {
      parsed = JSON.parse(text) as T;
    } catch (parseErr) {
      console.error(`[api] ❌ ${label} -> ${res.status} JSON parse error:`, parseErr, `body=${text.slice(0, 300)}`);
      throw new Error(`JSON parse error for ${label}`);
    }

    const bodySize = text.length;
    console.log(`[api] ✅ ${label} -> ${res.status} (${duration}ms, ${bodySize}b)`);
    return parsed;
  } catch (err) {
    if (err instanceof TypeError && err.message === "Failed to fetch") {
      console.error(`[api] ❌ ${label} -> network error (backend unreachable)`);
      throw err;
    }
    throw err;
  }
}

export async function backendHealth(): Promise<BackendHealth> {
  return fetchJson<BackendHealth>(`${BASE}/health`);
}

export async function listChats(): Promise<Chat[]> {
  const chats = await fetchJson<Chat[]>(`${BASE}/api/chats`);
  console.log(`[api] listChats -> ${chats.length} chats${chats.length > 0 ? `, first=${chats[0].id}` : ""}`);
  return chats;
}

export async function listMessages(chatId: string, params?: { limit?: number; before?: string }): Promise<Message[]> {
  let url = `${BASE}/api/chats/${encodeURIComponent(chatId)}/messages`;
  if (params) {
    const qs: string[] = [];
    if (params.limit) qs.push(`limit=${params.limit}`);
    if (params.before) qs.push(`before=${encodeURIComponent(params.before)}`);
    if (qs.length) url += `?${qs.join("&")}`;
  }
  const msgs = await fetchJson<Message[]>(url);
  console.log(`[api] listMessages(${chatId}) -> ${msgs.length} messages${msgs.length > 0 ? `, first=${msgs[0].id} last=${msgs[msgs.length - 1].id}` : ""}`);
  return msgs;
}

export async function listContacts(): Promise<Contact[]> {
  try {
    return await fetchJson<Contact[]>(`${BASE}/api/contacts`);
  } catch {
    console.warn("[api] listContacts -> endpoint not ready, returning []");
    return [];
  }
}

export interface AuthStatusResponse {
  status: string;
  qr: string;
}

export async function getAuthStatus(): Promise<AuthStatusResponse> {
  const result = await fetchJson<AuthStatusResponse>(`${BASE}/api/auth/status`);
  console.log(`[api] getAuthStatus -> status=${result.status} qrLength=${result.qr?.length || 0}`);
  return result;
}

export async function startAuth(): Promise<AuthStatusResponse> {
  const result = await fetchJson<AuthStatusResponse>(`${BASE}/api/auth/start`, { method: "POST" });
  console.log(`[api] startAuth -> status=${result.status} qrLength=${result.qr?.length || 0}`);
  return result;
}

export async function logout(): Promise<void> {
  const start = performance.now();
  console.log("[api] ➡️ POST /api/auth/logout");
  try {
    const res = await fetch(`${BASE}/api/auth/logout`, { method: "POST" });
    const duration = (performance.now() - start).toFixed(1);
    if (!res.ok) {
      console.error(`[api] ❌ POST /api/auth/logout -> ${res.status} (${duration}ms)`);
      throw new Error(`logout failed: ${res.status}`);
    }
    console.log(`[api] ✅ POST /api/auth/logout -> ${res.status} (${duration}ms)`);
  } catch (err) {
    if (err instanceof TypeError && err.message === "Failed to fetch") {
      console.error(`[api] ❌ POST /api/auth/logout -> network error`);
    }
    throw err;
  }
}

export function subscribeToEvents(
  onEvent: (type: string, data: unknown) => void,
  onDisconnect?: () => void,
): EventSource {
  console.log("[sse] 🔌 Connecting to /api/events...");
  const es = new EventSource(`${BASE}/api/events`);

  es.addEventListener("open", () => {
    console.log("[sse] ✅ Connection opened");
  });

  es.addEventListener("connected", (event) => {
    console.log("[sse] 📩 event=connected data=" + event.data.slice(0, 200));
    onEvent("connected", JSON.parse(event.data));
  });

  es.addEventListener("message", (event) => {
    const dataPreview = event.data.slice(0, 300);
    console.log("[sse] 📩 event=message data=" + dataPreview);
    try {
      onEvent("message", JSON.parse(event.data));
    } catch (err) {
      console.error("[sse] ❌ JSON parse error for message event:", err);
    }
  });

  es.addEventListener("receipt", (event) => {
    console.log("[sse] 📩 event=receipt data=" + event.data.slice(0, 200));
    try {
      onEvent("receipt", JSON.parse(event.data));
    } catch (err) {
      console.error("[sse] ❌ JSON parse error for receipt event:", err);
    }
  });

  es.addEventListener("contact", (event) => {
    console.log("[sse] 📩 event=contact data=" + event.data.slice(0, 200));
    try {
      onEvent("contact", JSON.parse(event.data));
    } catch (err) {
      console.error("[sse] ❌ JSON parse error for contact event:", err);
    }
  });

  es.addEventListener("presence", (event) => {
    console.log("[sse] 📩 event=presence data=" + event.data.slice(0, 200));
    try {
      onEvent("presence", JSON.parse(event.data));
    } catch (err) {
      console.error("[sse] ❌ JSON parse error for presence event:", err);
    }
  });

  es.addEventListener("logged_out", (event) => {
    console.log("[sse] 📩 event=logged_out data=" + event.data.slice(0, 200));
    onEvent("logged_out", JSON.parse(event.data));
  });

  es.onerror = (event) => {
    console.error("[sse] ❌ Connection error:", (event as Event & { message?: string }).message || "unknown");
    console.log("[sse] 🔌 Closing and will reconnect on next mount");
    es.close();
    onDisconnect?.();
  };

  return es;
}
