import type { Message } from "../context/chats-provider";

/** Keeps successful local reads across cached pages and overlapping polls. */
export class ReadBatches {
  private confirmed = new Map<string, Set<string>>();
  private inFlight = new Set<string>();

  reconcile(chatId: string, messages: Message[]): Message[] {
    const ids = this.confirmed.get(chatId);
    if (!ids) return messages;
    let changed = false;
    const next = messages.map((message) => {
      if (message.isSentFromUser || message.read || !ids.has(message.id)) return message;
      changed = true;
      return { ...message, read: true };
    });
    return changed ? next : messages;
  }

  async mark(
    chatId: string,
    messages: Message[],
    visibleIds: string[],
    isActive: () => boolean,
    send: (ids: string[]) => Promise<{ unreadCount: number }>,
    commit: (ids: string[], unreadCount: number) => void,
  ) {
    if (!isActive() || this.inFlight.has(chatId)) return;
    const visible = new Set(visibleIds);
    const confirmed = this.confirmed.get(chatId);
    const ids = [...new Set(messages.filter((message) =>
      visible.has(message.id) && !message.isSentFromUser && !message.read && !confirmed?.has(message.id)
    ).map(({ id }) => id))];
    if (ids.length === 0) return;

    this.inFlight.add(chatId);
    try {
      const { unreadCount } = await send(ids);
      const read = this.confirmed.get(chatId) ?? new Set<string>();
      ids.forEach((id) => read.add(id));
      this.confirmed.set(chatId, read);
      commit(ids, unreadCount);
    } finally {
      this.inFlight.delete(chatId);
    }
  }
}
