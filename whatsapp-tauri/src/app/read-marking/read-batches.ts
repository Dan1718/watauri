import type { Message } from "../context/chats-provider";

/** Keeps successful local reads across cached pages and overlapping polls. */
export class ReadBatches {
  private confirmed = new Map<string, Set<string>>();
  private optimistic = new Map<string, Set<string>>();
  private retry = new Map<string, Set<string>>();
  private retryAll = new Set<string>();
  private inFlight = new Set<string>();
  private pendingAll = new Map<string, () => Promise<void>>();

  reconcile(chatId: string, messages: Message[]): Message[] {
    const ids = new Set([...(this.confirmed.get(chatId) ?? []), ...(this.optimistic.get(chatId) ?? [])]);
    if (!ids.size) return messages;
    let changed = false;
    const next = messages.map((message) => {
      if (message.isSentFromUser || message.read || !ids.has(message.id)) return message;
      changed = true;
      return { ...message, read: true };
    });
    return changed ? next : messages;
  }

  beginOptimistic(chatId: string, ids: string[]) {
    const optimistic = this.optimistic.get(chatId) ?? new Set<string>();
    ids.forEach((id) => optimistic.add(id));
    this.optimistic.set(chatId, optimistic);
  }

  async mark(
    chatId: string,
    messages: Message[],
    visibleIds: string[],
    isActive: () => boolean,
    send: (ids: string[]) => Promise<{ unreadCount: number }>,
    commit: (ids: string[], unreadCount: number) => void,
    optimisticallyMark: (ids: string[]) => void = () => {},
  ) {
    if (!isActive() || this.inFlight.has(chatId)) return;
    if (this.retryAll.has(chatId)) {
      return this.markAll(chatId, messages, isActive, () => send([]), commit, optimisticallyMark);
    }
    const visible = new Set(visibleIds);
    const confirmed = this.confirmed.get(chatId);
    const retryIds = this.retry.get(chatId) ?? new Set<string>();
    const ids = [...new Set([
      ...messages.filter((message) =>
      visible.has(message.id) && !message.isSentFromUser && !message.read && !confirmed?.has(message.id)
      ).map(({ id }) => id),
      ...retryIds,
    ])];
    if (ids.length === 0) return;

    optimisticallyMark(ids);
    this.inFlight.add(chatId);
    try {
      const { unreadCount } = await send(ids);
      const read = this.confirmed.get(chatId) ?? new Set<string>();
      ids.forEach((id) => read.add(id));
      this.confirmed.set(chatId, read);
      ids.forEach((id) => this.optimistic.get(chatId)?.delete(id));
      if (this.optimistic.get(chatId)?.size === 0) this.optimistic.delete(chatId);
      ids.forEach((id) => retryIds.delete(id));
      if (retryIds.size) this.retry.set(chatId, retryIds);
      else this.retry.delete(chatId);
      commit(ids, unreadCount);
    } catch (error) {
      ids.forEach((id) => retryIds.add(id));
      this.retry.set(chatId, retryIds);
      throw error;
    } finally {
      this.inFlight.delete(chatId);
      this.runPendingAll(chatId);
    }
  }

  async markAll(
    chatId: string,
    messages: Message[],
    isActive: () => boolean,
    send: () => Promise<{ unreadCount: number }>,
    commit: (ids: string[], unreadCount: number) => void,
    optimisticallyMark: (ids: string[]) => void = () => {},
  ) {
    if (!isActive()) return;
    if (this.inFlight.has(chatId)) {
      this.pendingAll.set(chatId, () => this.markAll(chatId, messages, isActive, send, commit, optimisticallyMark));
      return;
    }
    const optimistic = this.optimistic.get(chatId);
    const ids = messages
      .filter((message) => !message.isSentFromUser && (!message.read || optimistic?.has(message.id)))
      .map(({ id }) => id);

    optimisticallyMark(ids);
    this.inFlight.add(chatId);
    try {
      const { unreadCount } = await send();
      const confirmed = this.confirmed.get(chatId) ?? new Set<string>();
      ids.forEach((id) => confirmed.add(id));
      this.confirmed.set(chatId, confirmed);
      ids.forEach((id) => this.optimistic.get(chatId)?.delete(id));
      if (this.optimistic.get(chatId)?.size === 0) this.optimistic.delete(chatId);
      this.retryAll.delete(chatId);
      this.retry.delete(chatId);
      commit(ids, unreadCount);
    } catch (error) {
      this.retryAll.add(chatId);
      throw error;
    } finally {
      this.inFlight.delete(chatId);
      this.runPendingAll(chatId);
    }
  }

  private runPendingAll(chatId: string) {
    const pending = this.pendingAll.get(chatId);
    if (!pending) return;
    this.pendingAll.delete(chatId);
    queueMicrotask(() => void pending().catch(() => {}));
  }
}
