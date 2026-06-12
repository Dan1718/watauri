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
