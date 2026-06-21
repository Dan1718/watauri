import { useEffect, useMemo, useRef, useState } from "react";
import { useVirtualizer } from "@tanstack/react-virtual";
import { CheckCheck, Info, Loader2, Mic, MoreVertical, Paperclip, Phone, Search, Smile, Video } from "lucide-react";
import type { AppScreen, Chat, Message } from "../types";
import { chatName } from "../types";

function formatTime(ts: string): string {
  const d = new Date(ts);
  return d.toLocaleTimeString(undefined, { hour: "2-digit", minute: "2-digit" });
}

function dateLabel(ts: string): string {
  const d = new Date(ts);
  const now = new Date();
  const sameDay = (a: Date, b: Date) =>
    a.getFullYear() === b.getFullYear() && a.getMonth() === b.getMonth() && a.getDate() === b.getDate();
  if (sameDay(d, now)) return "Today";
  const yesterday = new Date(now);
  yesterday.setDate(yesterday.getDate() - 1);
  if (sameDay(d, yesterday)) return "Yesterday";
  return d.toLocaleDateString(undefined, { weekday: "long", month: "long", day: "numeric" });
}

function dateKey(ts: string): string {
  const d = new Date(ts);
  return `${d.getFullYear()}-${d.getMonth()}-${d.getDate()}`;
}

type VirtualRow =
  | { type: "date"; label: string }
  | { type: "message"; message: Message };

interface MessageAreaProps {
  screen: AppScreen;
  chat: Chat | null;
  dbMessages: Message[];
  liveMessages?: Message[];
  loadingMessages?: boolean;
  loadingOlder?: boolean;
  hasMoreOlder?: boolean;
  onLoadOlder?: () => void;
  onUnarchive?: (id: string) => void;
}

export default function MessageArea({ screen, chat, dbMessages, liveMessages = [], loadingMessages = false, loadingOlder = false, hasMoreOlder = false, onLoadOlder, onUnarchive }: MessageAreaProps) {
  const [inputText, setInputText] = useState("");
  const scrollRef = useRef<HTMLDivElement>(null);

  const mergedMessages = useMemo(() => {
    if (liveMessages.length === 0) return dbMessages;
    const seen = new Set(liveMessages.map((m) => m.id));
    const deduped = dbMessages.filter((m) => !seen.has(m.id));
    return [...deduped, ...liveMessages];
  }, [dbMessages, liveMessages]);

  const rows = useMemo(() => {
    const msgs = mergedMessages;
    const result: VirtualRow[] = [];
    let lastKey = "";
    for (const m of msgs) {
      const k = dateKey(m.timestamp);
      if (k !== lastKey) {
        result.push({ type: "date", label: dateLabel(m.timestamp) });
        lastKey = k;
      }
      result.push({ type: "message", message: m });
    }
    return result;
  }, [mergedMessages]);

  const loadOlderRef = useRef(hasMoreOlder && !loadingOlder ? onLoadOlder : undefined);
  loadOlderRef.current = hasMoreOlder && !loadingOlder ? onLoadOlder : undefined;

  const rowCount = rows.length;

  const virtualizer = useVirtualizer({
    count: rowCount,
    getScrollElement: () => scrollRef.current,
    estimateSize: (index) => (rows[index]?.type === "date" ? 40 : 72),
    overscan: 20,
  });

  useEffect(() => {
    const el = scrollRef.current;
    if (!el || !loadOlderRef.current) return;

    const handler = () => {
      if (!loadOlderRef.current) return;
      const items = virtualizer.getVirtualItems();
      if (items.length > 0 && items[0].index <= 5) {
        loadOlderRef.current();
      }
    };
    el.addEventListener("scroll", handler, { passive: true });
    return () => el.removeEventListener("scroll", handler);
  }, [virtualizer]);

  if (!chat) {
    const emptyState =
      screen === "communities"
        ? {
            title: "Communities",
            body: "Bring together announcement groups, shared spaces, and the conversations that belong together.",
          }
        : screen === "starred"
          ? {
              title: "Starred conversations",
              body: "Keep the chats you reopen all day in one place for quick access.",
            }
          : screen === "archived"
            ? {
                title: "Archived conversations",
                body: "Keep quieter threads out of the main inbox while still keeping them easy to reopen when needed.",
              }
            : {
                title: "WhatsApp for Windows",
                body: "Send and receive messages without keeping your phone online. Use WhatsApp on up to 4 linked devices and 1 phone at the same time.",
              };

    return (
      <div className="relative flex flex-1 flex-col items-center justify-center overflow-hidden bg-surface">
        <div className="z-10 max-w-md space-y-4 px-6 text-center">
          <div className="mx-auto mb-8 flex h-24 w-24 items-center justify-center rounded-full bg-surface-container-highest">
            <LaptopIcon size={48} className="text-outline" />
          </div>
          <h2 className="text-2xl font-bold tracking-tight text-on-surface">{emptyState.title}</h2>
          <p className="text-sm leading-relaxed text-on-surface-variant">{emptyState.body}</p>
          <div className="flex items-center justify-center gap-2 pt-8 text-xs text-outline">
            <ShieldCheckIcon size={14} />
            <span>End-to-end encrypted</span>
          </div>
        </div>
      </div>
    );
  }

  const participant = (chat.participants || []).find((item) => item.id !== "me");
  const name = chatName(chat);
  const avatar = chat.isGroup
    ? chat.avatar || "https://images.unsplash.com/photo-1522071823991-b96767a1c56f?q=80&w=100&auto=format&fit=crop"
    : participant?.avatar;

  return (
    <section className="relative flex flex-1 flex-col overflow-hidden bg-surface">
      <header className="sticky top-0 right-0 z-40 flex h-[60px] w-full items-center justify-between bg-surface-container-high px-4">
        <div className="flex cursor-pointer items-center gap-3 transition-opacity active:opacity-80">
          {avatar ? (
            <img className="h-10 w-10 rounded-full object-cover" src={avatar} alt={name} referrerPolicy="no-referrer" />
          ) : (
            <div className="flex h-10 w-10 items-center justify-center rounded-full bg-primary-container text-sm font-bold text-on-primary-container">
              {name[0]?.toUpperCase()}
            </div>
          )}
          <div className="flex flex-col">
            <span className="font-medium tracking-tight text-on-surface">{name}</span>
            <span className={`text-[11px] ${chat.isArchived ? "text-outline" : "text-primary"}`}>
              {chat.isArchived ? "Archived" : participant?.status || "online"}
            </span>
          </div>
        </div>
        <div className="flex items-center gap-4 text-primary">
          {!chat.isArchived ? (
            <>
              <button className="rounded-full p-2 transition-colors hover:bg-surface-container-highest" type="button">
                <Video size={20} />
              </button>
              <button className="rounded-full p-2 transition-colors hover:bg-surface-container-highest" type="button">
                <Phone size={20} />
              </button>
            </>
          ) : null}
          <div className="mx-2 h-6 w-px bg-outline-variant/30" />
          <button className="rounded-full p-2 transition-colors hover:bg-surface-container-highest" type="button">
            <Search size={20} />
          </button>
          <button className="rounded-full p-2 transition-colors hover:bg-surface-container-highest" type="button">
            <MoreVertical size={20} />
          </button>
        </div>
      </header>

      <div
        ref={scrollRef}
        className="relative flex-1 overflow-y-auto bg-surface px-[60px] py-6"
      >
        <div className="mb-4 flex justify-center">
          <span className="rounded-lg bg-surface-container-high px-3 py-1 text-[10px] font-bold uppercase tracking-widest text-outline">
            Today
          </span>
        </div>

        {loadingMessages && rowCount === 0 ? (
          <div className="flex flex-1 items-center justify-center">
            <Loader2 size={24} className="animate-spin text-outline" />
          </div>
        ) : null}

        {loadingOlder ? (
          <div className="flex justify-center py-2">
            <Loader2 size={16} className="animate-spin text-outline" />
          </div>
        ) : null}

        <div style={{ height: virtualizer.getTotalSize(), position: "relative" }}>
          {virtualizer.getVirtualItems().map((virtualItem) => {
            const row = rows[virtualItem.index];
            if (row.type === "date") {
              return (
                <div
                  key={row.label}
                  style={{
                    position: "absolute",
                    top: 0,
                    left: 0,
                    width: "100%",
                    transform: `translateY(${virtualItem.start}px)`,
                    height: `${virtualItem.size}px`,
                  }}
                  className="flex items-center justify-center"
                >
                  <span className="rounded-lg bg-surface-container-high px-3 py-1 text-[10px] font-bold uppercase tracking-widest text-outline">
                    {row.label}
                  </span>
                </div>
              );
            }
            const message = row.message;
            const isMe = message.isFromMe ?? message.senderId === "me";
            return (
              <div
                key={message.id}
                style={{
                  position: "absolute",
                  top: 0,
                  left: isMe ? "auto" : 0,
                  right: isMe ? 0 : "auto",
                  transform: `translateY(${virtualItem.start}px)`,
                }}
                className={`flex max-w-[70%] flex-col ${isMe ? "items-end" : "items-start"}`}
              >
                <div
                  className={`rounded-lg p-3 shadow-sm ${isMe ? "rounded-tr-none bg-primary-container text-on-primary-container" : "rounded-tl-none bg-surface-container-highest text-on-surface"}`}
                >
                  <p className="text-sm">{message.text}</p>
                  <div className={`mt-1 flex items-center justify-end gap-1 ${isMe ? "text-on-primary-container/70" : "text-outline"}`}>
                    <span className="text-[10px]">{formatTime(message.timestamp)}</span>
                    {isMe ? (
                      <CheckCheck
                        size={14}
                        className={message.status === "read" ? "text-on-primary-container" : "text-on-primary-container/50"}
                      />
                    ) : null}
                  </div>
                </div>
              </div>
            );
          })}
        </div>

        {chat.isArchived ? (
          <div className="mt-8 rounded-xl border-l-4 border-primary bg-surface-container-high p-4">
            <div className="flex items-start gap-3">
              <Info className="shrink-0 text-primary" size={20} />
              <div>
                <h4 className="text-sm font-bold text-on-surface">This chat is archived</h4>
                <p className="mt-1 text-xs text-on-surface-variant">
                  New messages in this chat will not be shown in your main chat list. Unarchive to move it back.
                </p>
                <button onClick={() => onUnarchive?.(chat.id)} className="mt-2 text-xs font-bold text-primary hover:underline" type="button">
                  Unarchive Chat
                </button>
              </div>
            </div>
          </div>
        ) : null}
      </div>

      <footer className="flex h-[62px] shrink-0 items-center gap-3 bg-surface-container-high px-4">
        <button className="rounded-full p-2 text-outline transition-colors hover:bg-surface-container-highest" type="button">
          <Smile size={24} />
        </button>
        <button className="rounded-full p-2 text-outline transition-colors hover:bg-surface-container-highest" type="button">
          <Paperclip size={24} />
        </button>
        <div className="flex flex-1 items-center rounded-lg bg-surface-container-lowest px-4 py-2">
          <input
            className="w-full border-none bg-transparent text-sm text-on-surface placeholder:text-outline focus:ring-0"
            placeholder={chat.isArchived ? "Unarchive to reply to this conversation" : "Type a message"}
            type="text"
            disabled={chat.isArchived}
            value={inputText}
            onChange={(event) => setInputText(event.target.value)}
          />
        </div>
        <button
          className={`rounded-full p-2 shadow-ambient transition-transform active:scale-95 ${inputText ? "bg-primary-container text-on-primary-container" : "text-outline hover:bg-surface-container-highest"}`}
          type="button"
        >
          {inputText ? (
            <svg viewBox="0 0 24 24" width="24" height="24" fill="currentColor">
              <path d="M2.01 21L23 12 2.01 3 2 10l15 2-15 2z" />
            </svg>
          ) : (
            <Mic size={24} />
          )}
        </button>
      </footer>
    </section>
  );
}

function LaptopIcon(props: { size?: number; className?: string }) {
  return (
    <svg
      {...props}
      xmlns="http://www.w3.org/2000/svg"
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

function ShieldCheckIcon(props: { size?: number; className?: string }) {
  return (
    <svg
      {...props}
      xmlns="http://www.w3.org/2000/svg"
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      strokeWidth="2"
      strokeLinecap="round"
      strokeLinejoin="round"
    >
      <path d="M12 22s8-4 8-10V5l-8-3-8 3v7c0 6 8 10 8 10z" />
      <path d="m9 12 2 2 4-4" />
    </svg>
  );
}
