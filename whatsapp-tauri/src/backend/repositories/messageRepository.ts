import { MOCK_MESSAGES } from "../../constants";
import type { Message } from "../../types";

const LATENCY_MS = 120;

function wait<T>(value: T): Promise<T> {
  return new Promise((resolve) => {
    setTimeout(() => resolve(value), LATENCY_MS);
  });
}

export function listMessages(chatId: string): Promise<Message[]> {
  return wait(MOCK_MESSAGES[chatId] ?? []);
}
