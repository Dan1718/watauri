import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { AnimatePresence, motion } from "motion/react";
import { listChats, listContacts, listMessages } from "./backend/client";
import LoginScreen from "./components/LoginScreen";
import MessageArea from "./components/MessageArea";
import NavigationRail from "./components/NavigationRail";
import SettingsScreen from "./components/SettingsScreen";
import Sidebar from "./components/Sidebar";
import TitleBar from "./components/TitleBar";
import type { AppScreen, Chat, Contact, Message } from "./types";

export default function App() {
  const [screen, setScreen] = useState<AppScreen>("login");
  const [activeChatId, setActiveChatId] = useState<string | null>(null);
  const [chats, setChats] = useState<Chat[]>([]);
  const PAGE_SIZE = 50;
  const MAX_CACHED_CHATS = 20;

  interface ChatPageState {
    messages: Message[];
    buffer: Message[] | null;
    hasMore: boolean;
    loading: boolean;
  }

  const pageStates = useRef<Map<string, ChatPageState>>(new Map());
  function getPageState(chatId: string): ChatPageState | undefined {
    return pageStates.current.get(chatId);
  }
  function setPageState(chatId: string, state: ChatPageState) {
    const cache = pageStates.current;
    cache.set(chatId, state);
    if (cache.size > MAX_CACHED_CHATS) {
      const oldest = cache.keys().next().value;
      if (oldest !== undefined) cache.delete(oldest);
    }
  }

  const [dbMessages, setDbMessages] = useState<Message[]>([]);
  const [loadingMessages, setLoadingMessages] = useState(false);
  const [loadingOlder, setLoadingOlder] = useState(false);
  const [hasMoreOlder, setHasMoreOlder] = useState(true);
  const [contactsMap, setContactsMap] = useState<Record<string, Contact>>({});
  const lastLog = useRef("");

  if (screen !== "login" || activeChatId) {
    const msg = `[app] screen=${screen} chat=${activeChatId || "null"} chats=${chats.length} msgs=${dbMessages.length}`;
    if (msg !== lastLog.current) {
      console.log(msg);
      lastLog.current = msg;
    }
  }

  const refreshChats = useCallback(() => {
    void listChats().then((nextChats) => {
      setChats((prev) => {
        if (prev.length === nextChats.length && prev.every((c, i) => c.id === nextChats[i].id && c.unreadCount === nextChats[i].unreadCount)) {
          return prev;
        }
        return nextChats;
      });
    });
  }, []);

  useEffect(() => {
    console.log("[app] ⚡ Initial chat list fetch");
    refreshChats();
  }, [refreshChats]);

  useEffect(() => {
    if (screen !== "chats") return;
    console.log("[app] 📇 Fetching contacts...");
    void listContacts().then((contacts) => {
      const map: Record<string, Contact> = {};
      for (const c of contacts) map[c.id] = c;
      console.log(`[app] 📇 Contacts loaded: ${contacts.length}`);
      setContactsMap(map);
    });
  }, [screen]);

  useEffect(() => {
    if (!activeChatId) {
      setDbMessages([]);
      setLoadingMessages(false);
      return;
    }

    const existing = getPageState(activeChatId);
    if (existing) {
      console.log(`[app] 💬 Cache hit: ${activeChatId} (${existing.messages.length} messages, hasMore=${existing.hasMore})`);
      setDbMessages(existing.messages);
      setHasMoreOlder(existing.hasMore);
      setLoadingMessages(false);
      return;
    }

    console.log(`[app] 💬 Cache miss: ${activeChatId}, fetching first page...`);
    setDbMessages([]);
    setLoadingMessages(true);

    let active = true;
    const start = performance.now();
    void listMessages(activeChatId, { limit: PAGE_SIZE }).then((all) => {
      if (!active) return;
      const elapsed = (performance.now() - start).toFixed(1);
      const msgs = all.length > PAGE_SIZE ? all.slice(-PAGE_SIZE) : all;
      const buffer = all.length > PAGE_SIZE ? all.slice(0, -PAGE_SIZE) : null;
      const hasMore = buffer !== null;
      console.log(`[app] 💬 Fetched ${activeChatId}: ${all.length} total, showing ${msgs.length} (${elapsed}ms, buffer=${buffer?.length || 0})`);
      setPageState(activeChatId, { messages: msgs, buffer, hasMore, loading: false });
      setDbMessages(msgs);
      setHasMoreOlder(hasMore);
      setLoadingMessages(false);
    });

    return () => { active = false; };
  }, [activeChatId]);

  const loadOlderMessages = useCallback(() => {
    if (!activeChatId) return;
    const state = getPageState(activeChatId);
    if (!state || state.loading || !state.hasMore) return;

    state.loading = true;
    setLoadingOlder(true);

    if (state.buffer && state.buffer.length > 0) {
      const page = state.buffer.length > PAGE_SIZE
        ? state.buffer.slice(-PAGE_SIZE)
        : state.buffer;
      const newBuffer = state.buffer.length > page.length
        ? state.buffer.slice(0, -page.length)
        : null;
      const newMessages = [...page, ...state.messages];
      const newHasMore = newBuffer !== null;
      console.log(`[app] 💬 Load older ${activeChatId}: ${page.length} from buffer, ${newBuffer?.length || 0} remaining`);
      setPageState(activeChatId, { messages: newMessages, buffer: newBuffer, hasMore: newHasMore, loading: false });
      setDbMessages(newMessages);
      setHasMoreOlder(newHasMore);
      setLoadingOlder(false);
      return;
    }

    const oldest = state.messages[0]?.timestamp;
    if (!oldest) { state.loading = false; setLoadingOlder(false); return; }

    void listMessages(activeChatId, { limit: PAGE_SIZE, before: oldest }).then((older) => {
      const state = getPageState(activeChatId);
      if (!state) return;
      const newMessages = [...older, ...state.messages];
      const hasMore = older.length >= PAGE_SIZE;
      setPageState(activeChatId, { messages: newMessages, buffer: null, hasMore, loading: false });
      setDbMessages(newMessages);
      setHasMoreOlder(hasMore);
      setLoadingOlder(false);
    });
  }, [activeChatId]);

  const enrichedChats = useMemo(() => {
    return chats.map((chat) => {
      if (chat.isGroup || (chat.participants || []).length > 0 || chat.name) return chat;
      const contact = contactsMap[chat.id];
      if (!contact) return chat;
      return {
        ...chat,
        name: contact.name,
        participants: [{
          id: chat.id,
          name: contact.name,
          avatar: contact.avatar || "",
          status: "online" as const,
        }],
        avatar: contact.avatar || undefined,
      };
    });
  }, [chats, contactsMap]);

  const sortedChats = useMemo(() => {
    return [...enrichedChats].sort((a, b) => {
      const ta = a.lastMessage?.timestamp ? new Date(a.lastMessage.timestamp).getTime() : 0;
      const tb = b.lastMessage?.timestamp ? new Date(b.lastMessage.timestamp).getTime() : 0;
      return tb - ta;
    });
  }, [enrichedChats]);

  const activeChat = sortedChats.find((chat) => chat.id === activeChatId) || null;

  const handleLogout = () => {
    console.log("[app] 🚪 Logging out, clearing all state");
    pageStates.current.clear();
    setContactsMap({});
    setDbMessages([]);
    setActiveChatId(null);
    setScreen("login");
  };

  const handleLogin = () => {
    console.log("[app] 🚪 Login complete, switching to chats screen");
    setScreen("chats");
  };

  const handleSelectChat = (id: string) => {
    console.log(`[app] 💬 Chat selected: ${id}`);
    setActiveChatId(id);
  };

  const handleUnarchive = (id: string) => {
    console.log(`[app] 📂 Unarchiving chat ${id}`);
    setChats((previousChats) => {
      const chat = previousChats.find((c) => c.id === id);
      console.log(`[app] 📂 Unarchive: ${id} name=${chat?.name || (chat?.participants || [])[0]?.name || "unknown"}`);
      return previousChats.map((chat) =>
        chat.id === id ? { ...chat, isArchived: false } : chat,
      );
    });
  };

  if (screen === "login") {
    return <LoginScreen onLogin={handleLogin} />;
  }

  return (
    <div className="flex h-screen w-full flex-col overflow-hidden bg-surface">
      <TitleBar />

      <div className="flex min-h-0 flex-1 overflow-hidden">
        <NavigationRail
          currentScreen={screen}
          setScreen={(nextScreen) => {
            setScreen(nextScreen);
            if (screen !== nextScreen) {
              setActiveChatId(null);
            }
          }}
        />

        <main className="flex min-h-0 flex-1 overflow-hidden">
          <Sidebar
            chats={enrichedChats}
            screen={screen}
            activeChatId={activeChatId}
            onSelectChat={handleSelectChat}
            onBack={() => setScreen("chats")}
          />

          <MessageArea
            screen={screen}
            chat={activeChat}
            dbMessages={dbMessages}
            loadingMessages={loadingMessages}
            loadingOlder={loadingOlder}
            hasMoreOlder={hasMoreOlder}
            onLoadOlder={loadOlderMessages}
            onUnarchive={handleUnarchive}
          />
        </main>

        <AnimatePresence>
          {screen === "chats" && !activeChatId ? (
            <motion.div
              initial={{ opacity: 0, y: 50, scale: 0.9 }}
              animate={{ opacity: 1, y: 0, scale: 1 }}
              exit={{ opacity: 0, scale: 0.9 }}
              className="fixed bottom-8 right-8 z-[100] hidden max-w-sm items-center gap-4 rounded-lg border border-outline-variant/30 bg-surface-bright p-4 shadow-ambient md:flex"
            >
              <div className="flex h-10 w-10 items-center justify-center rounded-full bg-primary-container text-on-primary-container">
                <LaptopIcon />
              </div>
              <div>
                <h4 className="text-sm font-bold text-on-surface">
                  WhatsApp Desktop is ready
                </h4>
                <p className="text-xs text-on-surface-variant">
                  Get notified of new messages even when your phone is offline.
                </p>
              </div>
              <button className="p-1 text-outline hover:text-on-surface" type="button">
                <svg
                  xmlns="http://www.w3.org/2000/svg"
                  width="18"
                  height="18"
                  viewBox="0 0 24 24"
                  fill="none"
                  stroke="currentColor"
                  strokeWidth="2"
                  strokeLinecap="round"
                  strokeLinejoin="round"
                >
                  <path d="M18 6 6 18" />
                  <path d="m6 6 12 12" />
                </svg>
              </button>
            </motion.div>
          ) : null}
        </AnimatePresence>
      </div>
    </div>
  );
}

function LaptopIcon() {
  return (
    <svg
      xmlns="http://www.w3.org/2000/svg"
      width="20"
      height="20"
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      strokeWidth="2"
      strokeLinecap="round"
      strokeLinejoin="round"
    >
      <rect width="18" height="12" x="3" y="4" rx="2" ry="2" />
      <line x1="2" x2="22" y1="20" y2="20" />
    </svg>
  );
}
