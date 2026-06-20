import type { Chat, Message } from "../types";

const BASE = "http://localhost:8090";

export interface BackendHealth {
  name: string;
  status: string;
  mode: string;
}

export async function backendHealth(): Promise<BackendHealth> {
  const res = await fetch(`${BASE}/health`);
  if (!res.ok) throw new Error(`health check failed: ${res.status}`);
  return res.json();
}

export async function listChats(): Promise<Chat[]> {
  const res = await fetch(`${BASE}/api/chats`);
  if (!res.ok) throw new Error(`listChats failed: ${res.status}`);
  return res.json();
}

export async function listMessages(chatId: string): Promise<Message[]> {
  const res = await fetch(`${BASE}/api/chats/${chatId}`);
  if (!res.ok) throw new Error(`listMessages failed: ${res.status}`);
  return res.json();
}

export interface AuthStatusResponse {
  status: string;
  qr: string;
}

export async function getAuthStatus(): Promise<AuthStatusResponse> {
  const res = await fetch(`${BASE}/api/auth/status`);
  if (!res.ok) throw new Error(`getAuthStatus failed: ${res.status}`);
  return res.json();
}

export async function startAuth(): Promise<AuthStatusResponse> {
  const res = await fetch(`${BASE}/api/auth/start`, { method: "POST" });
  if (!res.ok) throw new Error(`startAuth failed: ${res.status}`);
  return res.json();
}

export async function logout(): Promise<void> {
  const res = await fetch(`${BASE}/api/auth/logout`, { method: "POST" });
  if (!res.ok) throw new Error(`logout failed: ${res.status}`);
}

export function subscribeToEvents(
  onEvent: (type: string, data: unknown) => void,
  onDisconnect?: () => void,
): EventSource {
  const es = new EventSource(`${BASE}/api/events`);

  es.addEventListener("connected", (event) => onEvent("connected", JSON.parse(event.data)));
  es.addEventListener("message", (event) => onEvent("message", JSON.parse(event.data)));
  es.addEventListener("receipt", (event) => onEvent("receipt", JSON.parse(event.data)));
  es.addEventListener("contact", (event) => onEvent("contact", JSON.parse(event.data)));
  es.addEventListener("presence", (event) => onEvent("presence", JSON.parse(event.data)));
  es.addEventListener("logged_out", (event) => onEvent("logged_out", JSON.parse(event.data)));

  es.onerror = () => {
    es.close();
    onDisconnect?.();
  };

  return es;
}
