import { afterEach, beforeEach, expect, test } from "bun:test";
import { Window } from "happy-dom";
import { act, useContext } from "react";
import { createRoot, type Root } from "react-dom/client";
import ChatsProvider, { ChatsContext, type Chats } from "../context/chats-provider";
import CurrentChatProvider, { type CurrentChat } from "../context/current-chat-provider";
import RealtimeProvider from "../context/realtime-provider";
import { ContactsContext } from "../context/contacts-provider";
import { ProfileContext } from "../context/profile-provider";
import { TabContext } from "../context/tab-provider";
import { useCurrentChat } from "../hooks/use-current-chat";
import { useViewportReadMarking } from "../hooks/use-viewport-read-marking";
import { markBackendChatRead } from "../backend";

let root: Root;
let dom: Window;
let current: CurrentChat;
let chats: Chats;
let focused = true;
let visibility = "visible";
let selectedTab = "chats";
let receipts = true;
let requests: { chatId: string; sendReceipt: boolean; messageIds: string[] }[];
let deferChats = false;
let resolveChats: ((response: Response) => void) | undefined;
let serverUnreadCount = 3;
let readResponses: ((response: Response) => void)[];
const originalFetch = globalThis.fetch;
const browserGlobals = ["window", "document", "HTMLElement", "MutationObserver", "ResizeObserver", "IS_REACT_ACT_ENVIRONMENT"] as const;
const originalGlobals = new Map(browserGlobals.map((key) => [key, Object.getOwnPropertyDescriptor(globalThis, key)]));

function backendChat(id: string) {
  return { id, unreadCount: serverUnreadCount, isGroup: false, isArchived: false, isPinned: false, isMuted: false, canSend: true };
}

function Probe() {
  current = useCurrentChat();
  chats = useContext(ChatsContext)!.chats;
  return null;
}

function Harness() {
  return (
    <TabContext.Provider value={{ selectedTab, selectTab: () => {}, topTabs: ["chats", "status"] }}>
      <ProfileContext.Provider value={{ profile: { id: "me", name: "Me", blueTickEnabled: true, readReceiptsEnabled: receipts, avatarUrl: "" }, isLoading: false, setReadReceiptsEnabled: () => {} }}>
        <ContactsContext.Provider value={{ contacts: [], dictionary: [], filteredContacts: [], search: "", error: null, isLoading: false, getContact: () => undefined, filterContacts: () => {}, setIsContactTyping: () => {} }}>
          <RealtimeProvider><ChatsProvider><CurrentChatProvider><Probe /></CurrentChatProvider></ChatsProvider></RealtimeProvider>
        </ContactsContext.Provider>
      </ProfileContext.Provider>
    </TabContext.Provider>
  );
}

beforeEach(() => {
  dom = new Window({ url: "http://localhost" });
  Object.assign(globalThis, {
    window: dom, document: dom.document, HTMLElement: dom.HTMLElement,
    MutationObserver: dom.MutationObserver, ResizeObserver: dom.ResizeObserver,
    IS_REACT_ACT_ENVIRONMENT: true,
  });
  focused = true;
  visibility = "visible";
  selectedTab = "chats";
  receipts = true;
  deferChats = false;
  resolveChats = undefined;
  serverUnreadCount = 3;
  requests = [];
  readResponses = [];
  Object.defineProperty(dom.document, "visibilityState", { get: () => visibility });
  dom.document.hasFocus = () => focused;
  globalThis.fetch = (async (input: string | URL | Request, init?: RequestInit) => {
    const url = String(input);
    if (url.endsWith("/api/chats")) {
      if (deferChats) return new Promise<Response>((resolve) => { resolveChats = resolve; });
      return Response.json([backendChat("a"), backendChat("b")]);
    }
    const chatId = url.split("/api/chats/")[1].split(/[/?]/)[0];
    if (url.endsWith("/read")) {
      requests.push({ chatId, ...JSON.parse(String(init?.body)) });
      return new Promise<Response>((resolve) => { readResponses.push(resolve); });
    }
    return Response.json({
      messages: ["1", "2", "3"].map((id) => ({ id: `${chatId}-${id}`, senderId: "sender", text: id, timestamp: `2026-01-01T00:00:0${id}Z`, status: "received", isFromMe: id === "3" })),
      nextCursor: null, latestCursor: "cursor", hasMore: false,
    });
  }) as typeof fetch;
  root = createRoot(dom.document.createElement("div") as unknown as HTMLElement);
});

afterEach(async () => {
  await act(async () => root.unmount());
  await dom.happyDOM.close();
  globalThis.fetch = originalFetch;
  for (const key of browserGlobals) {
    const descriptor = originalGlobals.get(key);
    if (descriptor) Object.defineProperty(globalThis, key, descriptor);
    else Reflect.deleteProperty(globalThis, key);
  }
});

async function render() { await act(async () => root.render(<Harness />)); }
async function open(chatId: string) {
  await act(async () => current.loadCurrentChat({ chatId, unreadCount: 3 }));
}
async function mark(chatId: string, ids: string[]) {
  await act(async () => current.markVisibleMessagesRead(chatId, ids));
}
async function respond(index: number, unreadCount: number) {
  await act(async () => readResponses[index](Response.json({ unreadCount })));
}

test("opening never reads a chat; explicit reads honor privacy, deduplicate, and reconcile cache", async () => {
  await render();
  await open("a");
  expect(requests).toHaveLength(0);
  receipts = false;
  await render();
  await mark("a", ["a-1", "a-2", "a-3"]);
  expect(requests).toEqual([{ chatId: "a", sendReceipt: false, messageIds: ["a-1", "a-2"] }]);
  await mark("a", ["a-1"]);
  expect(requests).toHaveLength(1);
  await respond(0, 1);
  expect(current.unreadCount).toBe(1);
  expect(current.messages.map(({ read }) => read)).toEqual([true, true, false]);
  expect(chats.complete.find(({ id }) => id === "a")!.unreadCount).toBe(1);
  await open("b");
  await open("a");
  expect(current.messages[0].read).toBe(true);
  await mark("a", ["a-1", "a-2"]);
  expect(requests).toHaveLength(1);
});

test("a late read response only changes its own chat and cache after switching chats", async () => {
  await render();
  await open("a");
  await mark("a", ["a-1"]);
  await open("b");
  await mark("a", ["a-2"]);
  expect(requests).toHaveLength(1);
  await respond(0, 2);
  expect(current.chatId).toBe("b");
  expect(current.unreadCount).toBe(3);
  expect(current.messages.every(({ read }) => !read)).toBe(true);
  expect(chats.complete.find(({ id }) => id === "a")!.unreadCount).toBe(2);
  await open("a");
  expect(current.messages[0].read).toBe(true);
});

test("dispatch is blocked when unfocused, hidden, or on another app tab", async () => {
  await render();
  await open("a");
  focused = false;
  await mark("a", ["a-1"]);
  focused = true;
  visibility = "hidden";
  await mark("a", ["a-1"]);
  visibility = "visible";
  selectedTab = "status";
  await render();
  await mark("a", ["a-1"]);
  expect(requests).toHaveLength(0);
  selectedTab = "chats";
  await render();
  await mark("a", ["a-1"]);
  expect(requests[0].sendReceipt).toBe(true);
  await respond(0, 2);
});

test("API refuses an empty ID batch before fetch", async () => {
  await expect(markBackendChatRead("a", true, [])).rejects.toThrow("require message IDs");
  expect(requests).toHaveLength(0);
});

test("viewport batch remeasures after scrolling, excludes overscan, and checks focus at flush", async () => {
  const batches: string[][] = [];
  const scroller = dom.document.createElement("div");
  const row = dom.document.createElement("div");
  row.dataset.unreadMessageId = "1";
  scroller.append(row);
  let top = 310;
  scroller.getBoundingClientRect = () => new dom.DOMRect(0, 50, 500, 250);
  row.getBoundingClientRect = () => new dom.DOMRect(0, top, 100, 50);
  const markVisible = (_chatId: string, ids: string[]) => {
    if (ids.length) batches.push(ids);
  };
  function ViewportProbe() {
    const setScroller = useViewportReadMarking("a", markVisible);
    return <div ref={(node) => { if (node) setScroller(scroller as unknown as HTMLElement); }} />;
  }
  await act(async () => root.render(
    <TabContext.Provider value={{ selectedTab: "chats", selectTab: () => {}, topTabs: [] }}><ViewportProbe /></TabContext.Provider>
  ));
  await act(async () => { await Bun.sleep(230); });
  expect(batches).toHaveLength(0);
  top = 100;
  scroller.dispatchEvent(new dom.Event("scroll"));
  focused = false;
  await act(async () => { await Bun.sleep(230); });
  expect(batches).toHaveLength(0);
  focused = true;
  dom.dispatchEvent(new dom.Event("focus"));
  await act(async () => { await Bun.sleep(230); });
  expect(batches).toEqual([["1"]]);
});


test("a chat poll started before a read cannot restore its old count; fresh polls still update counts", async () => {
  await render();
  await open("a");
  selectedTab = "status";
  await render();
  deferChats = true;
  selectedTab = "chats";
  await render();
  expect(resolveChats).toBeDefined();
  await mark("a", ["a-1"]);
  await respond(0, 2);
  await act(async () => resolveChats!(Response.json([backendChat("a"), backendChat("b")])));
  expect(current.unreadCount).toBe(2);
  expect(chats.complete.find(({ id }) => id === "a")!.unreadCount).toBe(2);
  expect(current.messages[0].read).toBe(true);
  selectedTab = "status";
  await render();
  deferChats = false;
  serverUnreadCount = 4;
  selectedTab = "chats";
  await render();
  expect(current.unreadCount).toBe(4);
  expect(current.messages[0].read).toBe(true);
});

test("switching away and back while a read is in flight reconciles the reopened chat", async () => {
  await render();
  await open("a");
  await mark("a", ["a-1"]);
  await open("b");
  await open("a");
  await respond(0, 2);
  expect(current.chatId).toBe("a");
  expect(current.unreadCount).toBe(2);
  expect(current.messages[0].read).toBe(true);
  await mark("a", ["a-1"]);
  expect(requests).toHaveLength(1);
});

test("failed reads keep their optimistic marker, retry, and do not set errors on another chat", async () => {
  await render();
  await open("a");
  await mark("a", ["a-1"]);
  await open("b");
  await act(async () => readResponses[0](new Response(null, { status: 500 })));
  expect(current.error).toBeNull();
  await open("a");
  await mark("a", ["a-1"]);
  await open("b");
  await open("a");
  await act(async () => readResponses[1](new Response(null, { status: 500 })));
  expect(current.error).toBeNull();
  // Failed requests stay optimistically marked while the viewport retries them.
  expect(current.messages[0].read).toBe(true);
  await mark("a", ["a-1"]);
  expect(requests).toHaveLength(3);
  await respond(2, 2);
  expect(current.messages[0].read).toBe(true);
});
