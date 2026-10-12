import { describe, expect, test } from "bun:test";
import type { Message } from "../context/chats-provider";
import { ReadBatches } from "./read-batches";
import { canReadConversation, intersectsViewport, visibleUnreadIds } from "./viewport";

const message = (id: string, extra: Partial<Message> = {}): Message => ({
  id, contactId: "sender", message: id, timestamp: 1, isSentFromUser: false, read: false, ...extra,
});

function deferred<T>() {
  let resolve!: (value: T) => void;
  const promise = new Promise<T>((done) => { resolve = done; });
  return { promise, resolve };
}

describe("read batches", () => {
  test("batches only visible unread incoming IDs and never sends empty batches", async () => {
    const batches = new ReadBatches();
    const requests: string[][] = [];
    const messages = [message("visible"), message("offscreen"), message("outgoing", { isSentFromUser: true }), message("read", { read: true })];
    const send = async (ids: string[]) => { requests.push(ids); return { unreadCount: 3 }; };
    let count = 0;
    await batches.mark("a", messages, ["visible", "visible", "outgoing", "read", "unknown"], () => true, send, (_ids, remaining) => { count = remaining; });
    expect(requests).toEqual([["visible"]]);
    expect(count).toBe(3);
    await batches.mark("a", messages, [], () => true, send, () => {});
    await batches.mark("a", messages, ["visible"], () => true, send, () => {});
    expect(requests).toHaveLength(1);
    const reconciled = batches.reconcile("a", messages);
    expect(reconciled[0].read).toBe(true);
    expect(reconciled[1]).toBe(messages[1]);
    expect(batches.reconcile("b", messages)).toBe(messages);
    expect(batches.reconcile("a", reconciled)).toBe(reconciled);
  });

  test("serializes per chat, deduplicates in-flight IDs, and lets the next viewport retry", async () => {
    const batches = new ReadBatches();
    const pending = deferred<{ unreadCount: number }>();
    const requests: string[][] = [];
    const messages = [message("1"), message("2")];
    const first = batches.mark("a", messages, ["1"], () => true, async (ids) => { requests.push(ids); return pending.promise; }, () => {});
    await batches.mark("a", messages, ["1", "2"], () => true, async (ids) => { requests.push(ids); return { unreadCount: 0 }; }, () => {});
    expect(requests).toEqual([["1"]]);
    pending.resolve({ unreadCount: 1 });
    await first;
    await batches.mark("a", messages, ["1", "2"], () => true, async (ids) => { requests.push(ids); return { unreadCount: 0 }; }, () => {});
    expect(requests).toEqual([["1"], ["2"]]);
  });

  test("failed requests leave messages unread and can retry", async () => {
    const batches = new ReadBatches();
    const messages = [message("1")];
    await expect(batches.mark("a", messages, ["1"], () => true, async () => { throw new Error("offline"); }, () => {})).rejects.toThrow("offline");
    expect(batches.reconcile("a", messages)).toBe(messages);
    let called = false;
    await batches.mark("a", messages, ["1"], () => true, async () => ({ unreadCount: 0 }), () => { called = true; });
    expect(called).toBe(true);
  });

  test("does not dispatch for inactive conversations", async () => {
    const batches = new ReadBatches();
    await batches.mark("a", [message("1")], ["1"], () => false, async () => { throw new Error("must not send"); }, () => {});
  });
});

describe("viewport visibility", () => {
  const viewport = { top: 50, bottom: 300, left: 0, right: 500 };
  const row = (top: number, bottom: number) => ({ top, bottom, left: 10, right: 100 });

  test("excludes overscan and edge contact, includes partially visible and tall messages", () => {
    expect(intersectsViewport(row(300, 400), viewport, 500, 500)).toBe(false);
    expect(intersectsViewport(row(10, 50), viewport, 500, 500)).toBe(false);
    expect(intersectsViewport(row(299, 400), viewport, 500, 500)).toBe(true);
    expect(intersectsViewport(row(0, 700), viewport, 500, 500)).toBe(true);
    expect(intersectsViewport(row(150, 200), viewport, 500, 100)).toBe(false);
    expect(intersectsViewport({ ...row(100, 200), left: 500, right: 600 }, viewport, 500, 500)).toBe(false);
    expect(intersectsViewport(row(100, 200), { ...viewport, bottom: 50 }, 500, 500)).toBe(false);
  });

  test("checks actual row geometry rather than Virtuoso mounted items or indices", () => {
    const scroller = {
      getBoundingClientRect: () => viewport,
      querySelectorAll: () => [
        { dataset: { unreadMessageId: "visible" }, getBoundingClientRect: () => row(100, 150) },
        { dataset: { unreadMessageId: "overscan" }, getBoundingClientRect: () => row(310, 360) },
      ],
    } as unknown as HTMLElement;
    expect(visibleUnreadIds(scroller, 500, 500)).toEqual(["visible"]);
  });

  test("requires chats tab, visible document, and window focus", () => {
    expect(canReadConversation(true, "visible", true)).toBe(true);
    expect(canReadConversation(false, "visible", true)).toBe(false);
    expect(canReadConversation(true, "hidden", true)).toBe(false);
    expect(canReadConversation(true, "visible", false)).toBe(false);
  });
});
