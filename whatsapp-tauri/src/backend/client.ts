import { invoke } from "@tauri-apps/api/core";
import { listChats as listMockChats } from "./repositories/chatRepository";
import { listMessages as listMockMessages } from "./repositories/messageRepository";
import type { Chat, Message } from "../types";

export interface BackendHealth {
  name: string;
  status: string;
  mode: string;
}

export async function backendHealth(): Promise<BackendHealth> {
  try {
    return await invoke<BackendHealth>("backend_health");
  } catch {
    return {
      name: "whatsapp-tauri",
      status: "ok",
      mode: "mock",
    };
  }
}

export async function listChats(): Promise<Chat[]> {
  return listMockChats();
}

export async function listMessages(chatId: string): Promise<Message[]> {
  return listMockMessages(chatId);
}
