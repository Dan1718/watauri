import { MOCK_CHATS } from "../../constants";
import type { Chat } from "../../types";

const LATENCY_MS = 120;

function wait<T>(value: T): Promise<T> {
  return new Promise((resolve) => {
    setTimeout(() => resolve(value), LATENCY_MS);
  });
}

export function listChats(): Promise<Chat[]> {
  return wait(MOCK_CHATS);
}
