import { FormEvent, memo, useCallback, useEffect, useLayoutEffect, useMemo, useRef, useState } from "react";
import { ArrowDownIcon } from "@phosphor-icons/react";
import dayjs from "dayjs";
import { Virtuoso, type VirtuosoHandle } from "react-virtuoso";
import { Message } from "@/app/context/chats-provider";
import { CurrentChatContacts } from "@/app/context/current-chat-provider";
import { useCurrentChat } from "@/app/hooks/use-current-chat";
import { useChats } from "@/app/hooks/use-chats";
import { useProfile } from "@/app/hooks/use-profile";
import { getDisplayNameFromJid } from "@/app/utils";
import Reaction from "../message/reaction";
import ChatInfoPanel from "./chat-info-panel";
import ContactHeader from "./contact-header";
import ChatMessage from "./chat-message";
import MessageReactions from "./message-reactions";

const INITIAL_ITEM_INDEX = 1_000_000;

function messageDay(timestamp: Message["timestamp"]) {
  return typeof timestamp === "number" ? dayjs.unix(timestamp) : dayjs(timestamp);
}

function isSameMessageDay(a: Message["timestamp"], b: Message["timestamp"]) {
  return messageDay(a).format("YYYY-MM-DD") === messageDay(b).format("YYYY-MM-DD");
}

function formatMessageDay(timestamp: Message["timestamp"]) {
  const date = messageDay(timestamp);
  const daysAgo = dayjs().startOf("day").diff(date.startOf("day"), "day");

  if (daysAgo === 0) return "Today";
  if (daysAgo === 1) return "Yesterday";
  if (daysAgo > 1 && daysAgo < 7) return date.format("dddd");
  return date.format("ddd, MMM D");
}

function DatePill({ timestamp, floating = false }: {
  timestamp: Message["timestamp"];
  floating?: boolean;
}) {
  return (
    <div className={floating ? undefined : "flex w-full justify-center px-4 py-3"}>
      <p className="rounded-full border border-white/10 bg-[#182229]/95 px-2.5 py-1 text-[11px] font-medium text-white/60 shadow-sm backdrop-blur-sm">
        {formatMessageDay(timestamp)}
      </p>
    </div>
  );
}

type MessageRowProps = {
  message: Message;
  isGroup: boolean;
  senderName?: string;
  senderAvatar?: string;
  showSender: boolean;
  compact: boolean;
  isLast: boolean;
  blueTickEnabled: boolean;
  reactionMenuOpen: boolean;
  onToggleReactionMenu: (messageId: string) => void;
};

const MessageRow = memo(function MessageRow({
  message,
  isGroup,
  senderName,
  senderAvatar,
  showSender,
  compact,
  isLast,
  blueTickEnabled,
  reactionMenuOpen,
  onToggleReactionMenu,
}: MessageRowProps) {
  const hasReactions = Boolean(message.reactions?.length);

  return (
    <div
      className={`group/message flex w-full items-center ${
        message.isSentFromUser ? "justify-end" : "justify-start"
      } px-4 ${isLast ? "pb-0" : compact && !hasReactions ? "pb-0.5" : "pb-4"}`}
    >
      <div
        className={`relative flex min-w-0 items-center gap-2 ${
          isGroup ? "w-[60%]" : "max-w-full"
        } ${message.isSentFromUser ? "justify-end" : "justify-start"}`}
      >
        {message.isSentFromUser ? (
          <Reaction
            isSentFromUser
            isOpen={reactionMenuOpen}
            messageId={message.id}
            onToggle={onToggleReactionMenu}
          />
        ) : null}
        <ChatMessage
          message={message}
          isGroup={isGroup}
          senderName={senderName}
          senderAvatar={senderAvatar}
          showSender={showSender}
          blueTickEnabled={blueTickEnabled}
        />
        {!message.isSentFromUser ? (
          <Reaction
            isSentFromUser={false}
            isOpen={reactionMenuOpen}
            messageId={message.id}
            onToggle={onToggleReactionMenu}
          />
        ) : null}
        {hasReactions ? (
          <MessageReactions
            reactions={message.reactions!}
            isSentFromUser={message.isSentFromUser}
          />
        ) : null}
      </div>
    </div>
  );
});

const MessageList = memo(function MessageList({
  chatId,
  messages,
  contacts,
  isGroup,
  blueTickEnabled,
  isLoading,
  error,
  hasMoreMessages,
  loadOlderMessages,
  unreadCount,
  scrollToBottomRequest,
}: {
  chatId: string;
  messages: Message[];
  contacts?: CurrentChatContacts;
  isGroup: boolean;
  blueTickEnabled: boolean;
  isLoading: boolean;
  error: string | null;
  hasMoreMessages: boolean;
  loadOlderMessages: () => Promise<void>;
  unreadCount: number;
  scrollToBottomRequest: number;
}) {
  const [activeReactionId, setActiveReactionId] = useState<string | null>(null);
  const [isAtBottom, setIsAtBottom] = useState(true);
  const [isScrolling, setIsScrolling] = useState(false);
  const [visibleTimestamp, setVisibleTimestamp] = useState<Message["timestamp"] | null>(null);
  const messageIndexes = useMemo(
    () => new Map(messages.map((message, index) => [message.id, index])),
    [messages]
  );
  const previousMessages = useRef(messages);
  const committedFirstItemIndex = useRef(INITIAL_ITEM_INDEX);
  const positionedAtUnread = useRef(false);
  const virtuosoRef = useRef<VirtuosoHandle>(null);
  let firstItemIndex = committedFirstItemIndex.current;
  const oldestUnreadIndex = unreadCount <= messages.length || !hasMoreMessages
    ? Math.max(0, messages.length - unreadCount)
    : -1;

  if (previousMessages.current !== messages && previousMessages.current[0]) {
    const previousFirstIndex = messages.findIndex(
      (message) => message.id === previousMessages.current[0].id
    );
    if (previousFirstIndex > 0) firstItemIndex -= previousFirstIndex;
  }

  useLayoutEffect(() => {
    previousMessages.current = messages;
    committedFirstItemIndex.current = firstItemIndex;
  }, [firstItemIndex, messages]);

  useLayoutEffect(() => {
    if (scrollToBottomRequest) {
      virtuosoRef.current?.scrollToIndex({ index: "LAST", align: "end", behavior: "smooth" });
    }
  }, [scrollToBottomRequest]);

  useLayoutEffect(() => {
    if (positionedAtUnread.current || unreadCount === 0 || messages.length === 0 || isLoading) return;
    if (unreadCount > messages.length && hasMoreMessages) {
      void loadOlderMessages();
      return;
    }

    positionedAtUnread.current = true;
    virtuosoRef.current?.scrollToIndex({
      index: firstItemIndex + Math.max(0, messages.length - unreadCount),
      align: "center",
    });
  }, [firstItemIndex, hasMoreMessages, isLoading, loadOlderMessages, messages.length, unreadCount]);

  const toggleReactionMenu = useCallback((messageId: string) => {
    setActiveReactionId((current) => current === messageId ? null : messageId);
  }, []);

  return (
    <div className="relative min-h-0 w-full flex-1">
      {error ? (
        <div className="absolute inset-x-0 top-2 z-30 text-center text-sm text-red-300">
          {error}
        </div>
      ) : null}
      {isLoading && messages.length === 0 ? (
        <div className="absolute inset-0 z-30 flex items-center justify-center text-white">
          Loading...
        </div>
      ) : null}
      <div
        aria-hidden="true"
        className={`pointer-events-none absolute left-1/2 top-3 z-20 -translate-x-1/2 transition-opacity duration-200 ${
          isScrolling && visibleTimestamp !== null ? "opacity-100" : "opacity-0 delay-1000"
        }`}
      >
        {visibleTimestamp !== null ? <DatePill timestamp={visibleTimestamp} floating /> : null}
      </div>
      <Virtuoso
        ref={virtuosoRef}
        key={chatId}
        className="h-full w-full"
        data={messages}
        firstItemIndex={firstItemIndex}
        initialTopMostItemIndex={Math.max(0, messages.length - 1)}
        alignToBottom
        followOutput={(isAtBottom) => isAtBottom ? "auto" : false}
        atBottomStateChange={setIsAtBottom}
        increaseViewportBy={{ top: 0, bottom: 200 }}
        computeItemKey={(_index, message) => message.id}
        isScrolling={setIsScrolling}
        rangeChanged={({ startIndex }) => {
          const message = messages[startIndex - firstItemIndex];
          if (message) setVisibleTimestamp(message.timestamp);
        }}
        startReached={() => {
          if (hasMoreMessages && !isLoading) void loadOlderMessages();
        }}
        itemContent={(_index, message) => {
          const index = messageIndexes.get(message.id)!;
          const previous = messages[index - 1];
          const next = messages[index + 1];
          const startsNewDay = !previous || !isSameMessageDay(previous.timestamp, message.timestamp);
          const endsDay = !next || !isSameMessageDay(message.timestamp, next.timestamp);
          const repeatedIncomingSender = isGroup && !message.isSentFromUser &&
            !startsNewDay && !previous?.isSentFromUser && previous?.contactId === message.contactId;
          const compact = next?.isSentFromUser === message.isSentFromUser &&
            next?.contactId === message.contactId && !endsDay;
          const contact = contacts?.[message.contactId];
          const isOldestUnread = unreadCount > 0 && index === oldestUnreadIndex;

          return (
            <>
              {startsNewDay ? <DatePill timestamp={message.timestamp} /> : null}
              {isOldestUnread ? (
                <div className="flex w-full items-center gap-2 px-4 py-3 text-[11px] font-medium text-[#00a884]">
                  <span className="h-px flex-1 bg-[#00a884]/50" />
                  <span>New</span>
                  <span className="h-px flex-1 bg-[#00a884]/50" />
                </div>
              ) : null}
              <MessageRow
                message={message}
                isGroup={isGroup}
                senderName={contact?.displayName ?? getDisplayNameFromJid(message.contactId)}
                senderAvatar={contact?.contactAvatar}
                showSender={!repeatedIncomingSender}
                compact={compact}
                isLast={index === messages.length - 1}
                blueTickEnabled={blueTickEnabled}
                reactionMenuOpen={activeReactionId === message.id}
                onToggleReactionMenu={toggleReactionMenu}
              />
            </>
          );
        }}
      />
      {!isAtBottom && unreadCount > 0 ? (
        <button
          type="button"
          aria-label={`Jump to ${unreadCount} new ${unreadCount === 1 ? "message" : "messages"}`}
          className="absolute bottom-3 left-1/2 z-20 flex -translate-x-1/2 items-center gap-2 rounded-full border border-white/10 bg-[#111b21]/95 px-2 py-1.5 text-xs font-medium text-[#00a884] shadow-lg backdrop-blur-sm transition hover:bg-[#202c33] focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-[#00a884]"
          onClick={() => virtuosoRef.current?.scrollToIndex({
            index: "LAST",
            align: "end",
            behavior: "smooth",
          })}
        >
          <span className="rounded-full bg-[#00a884] px-1.5 py-0.5 text-[10px] font-semibold text-[#0b141a]">
            {unreadCount}
          </span>
          <span>New {unreadCount === 1 ? "Message" : "Messages"}</span>
          <ArrowDownIcon aria-hidden="true" size={14} weight="bold" />
        </button>
      ) : null}
    </div>
  );
});

function Composer({
  chatId,
  sendMessage,
  onSent,
}: {
  chatId: string;
  sendMessage: (text: string) => boolean;
  onSent: () => void;
}) {
  const [messageText, setMessageText] = useState("");
  const [recordingState, setRecordingState] = useState<"idle" | "recording" | "paused">("idle");
  const [recordingSeconds, setRecordingSeconds] = useState(0);
  const [isPlaying, setIsPlaying] = useState(false);
  const [viewOnce, setViewOnce] = useState(false);
  const [isDrawerOpen, setIsDrawerOpen] = useState(false);
  const [isDrawerReady, setIsDrawerReady] = useState(false);
  const [isAttachmentDrawerOpen, setIsAttachmentDrawerOpen] = useState(false);
  const [isMediaDrawerOpen, setIsMediaDrawerOpen] = useState(false);
  const inputRef = useRef<HTMLInputElement>(null);
  const fileInputRef = useRef<HTMLInputElement>(null);

  useLayoutEffect(() => {
    if (recordingState === "idle") inputRef.current?.focus();
  }, [chatId, recordingState]);

  useEffect(() => {
    if (recordingState !== "recording") return;
    const timer = window.setInterval(() => setRecordingSeconds((seconds) => seconds + 1), 1000);
    return () => window.clearInterval(timer);
  }, [recordingState]);

  useEffect(() => {
    setIsAttachmentDrawerOpen(false);
    setIsMediaDrawerOpen(false);
    setIsDrawerReady(false);
    if (!isDrawerOpen) return;
    const timer = window.setTimeout(() => setIsDrawerReady(true), 250);
    return () => window.clearTimeout(timer);
  }, [isDrawerOpen]);

  const discardRecording = () => {
    setRecordingState("idle");
    setRecordingSeconds(0);
    setIsPlaying(false);
    setViewOnce(false);
  };

  const formattedTime = `${Math.floor(recordingSeconds / 60)}:${String(recordingSeconds % 60).padStart(2, "0")}`;
  const hasText = messageText.length > 0;
  const isRecording = recordingState !== "idle";

  const handleSubmit = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    const text = messageText.trim();
    if (!text) return;
    if (sendMessage(text)) {
      setMessageText("");
      onSent();
    }
  };

  const paused = recordingState === "paused";

  return (
    <form className="z-30 flex h-auto w-full gap-2 px-4 pb-2 pt-2" onSubmit={handleSubmit}>
      <input ref={fileInputRef} type="file" hidden />
      {!isRecording ? (
        <div
          className="relative h-11 shrink-0 motion-reduce:transition-none"
          style={{ width: isDrawerOpen ? 140 : 44, zIndex: 1, transition: "width 250ms ease-out" }}
        >
          <button
            type="button"
            aria-label={isDrawerOpen ? "Close attachments" : "Open attachments"}
            aria-expanded={isDrawerOpen}
            className="composer-action-button absolute left-0 top-0 grid size-11 cursor-pointer place-items-center overflow-hidden rounded-full bg-[#242626] text-white"
            style={{ zIndex: 1 }}
            onClick={() => setIsDrawerOpen((open) => !open)}
          >
            <span
              aria-hidden="true"
              className="material-symbols-outlined grid size-6 place-items-center !text-[24px] transition-transform duration-200 ease-out will-change-transform motion-reduce:transition-none"
              style={{ color: "#8a8a92", transform: `rotate(${isDrawerOpen ? 45 : 0}deg)` }}
            >
              add
            </span>
          </button>
          <div
            className={`absolute z-20 size-6 motion-reduce:transition-none ${isDrawerOpen ? "opacity-100" : "opacity-0"} ${isDrawerReady ? "" : "pointer-events-none"}`}
            style={{ left: 52, top: "50%", transform: `translate(${isDrawerOpen ? 0 : -42}px, -50%)`, transition: "transform 250ms ease-out, opacity 150ms ease-out" }}
            onMouseEnter={() => {
              if (isDrawerReady) setIsAttachmentDrawerOpen(true);
            }}
            onMouseLeave={() => setIsAttachmentDrawerOpen(false)}
            onFocus={() => {
              if (isDrawerReady) setIsAttachmentDrawerOpen(true);
            }}
            onBlur={(event) => {
              if (!event.currentTarget.contains(event.relatedTarget)) setIsAttachmentDrawerOpen(false);
            }}
          >
            <span
              aria-hidden="true"
              className={`absolute ${isAttachmentDrawerOpen ? "pointer-events-auto" : "pointer-events-none"}`}
              style={{ top: -90, left: -10, width: 44, height: 82, borderRadius: 999, backgroundColor: "#242626", opacity: isAttachmentDrawerOpen ? 1 : 0, transform: `scaleY(${isAttachmentDrawerOpen ? 1 : 0})`, transformOrigin: "bottom", transition: "transform 150ms ease-out, opacity 75ms ease-out" }}
            />
            <span
              aria-hidden="true"
              className={`absolute ${isAttachmentDrawerOpen ? "pointer-events-auto" : "pointer-events-none"}`}
              style={{ top: -8, left: -10, width: 44, height: 12 }}
            />
            <span
              aria-hidden="true"
              className={`absolute ${isDrawerReady ? "pointer-events-auto" : "pointer-events-none"}`}
              style={{ top: -18, left: -17, width: 22, height: 24, borderRadius: 999 }}
            />
            <button type="button" aria-label="Attach file" aria-expanded={isAttachmentDrawerOpen} tabIndex={isDrawerReady ? 0 : -1} className="absolute z-20 grid size-6 cursor-pointer place-items-center bg-transparent" style={{ backgroundColor: "transparent" }} onClick={() => fileInputRef.current?.click()}>
              <span aria-hidden="true" className="material-symbols-outlined !text-[24px]" style={{ color: "#8a8a92" }}>attach_file</span>
            </button>
            <button
              type="button"
              aria-label="Attach contact"
              tabIndex={isDrawerOpen ? 0 : -1}
              className={`absolute z-10 grid h-6 cursor-pointer place-items-center bg-transparent ${isAttachmentDrawerOpen ? "pointer-events-auto" : "pointer-events-none"}`}
              style={{ top: -77, left: -10, width: 44, backgroundColor: "transparent", opacity: isAttachmentDrawerOpen ? 1 : 0, transform: `translateY(${isAttachmentDrawerOpen ? 0 : 8}px)`, transition: "transform 250ms ease-out, opacity 150ms ease-out" }}
            >
              <span aria-hidden="true" className="material-symbols-outlined !text-[18px]" style={{ color: "#8a8a92" }}>person</span>
            </button>
            <button
              type="button"
              aria-label="Attach photo"
              tabIndex={isDrawerOpen ? 0 : -1}
              className={`absolute z-10 grid h-6 cursor-pointer place-items-center bg-transparent ${isAttachmentDrawerOpen ? "pointer-events-auto" : "pointer-events-none"}`}
              style={{ top: -45, left: -10, width: 44, backgroundColor: "transparent", opacity: isAttachmentDrawerOpen ? 1 : 0, transform: `translateY(${isAttachmentDrawerOpen ? 0 : 8}px)`, transition: "transform 250ms ease-out, opacity 150ms ease-out" }}
            >
              <span aria-hidden="true" className="material-symbols-outlined !text-[18px]" style={{ color: "#8a8a92" }}>photo_camera</span>
            </button>
          </div>
          <div
            className={`absolute z-20 size-6 motion-reduce:transition-none ${isDrawerOpen ? "opacity-100" : "opacity-0"} ${isDrawerReady ? "" : "pointer-events-none"}`}
            style={{ left: 84, top: "50%", transform: `translate(${isDrawerOpen ? 0 : -42}px, -50%)`, transition: "transform 250ms ease-out, opacity 150ms ease-out" }}
            onMouseEnter={() => {
              if (isDrawerReady) setIsMediaDrawerOpen(true);
            }}
            onMouseLeave={() => setIsMediaDrawerOpen(false)}
            onFocus={() => {
              if (isDrawerReady) setIsMediaDrawerOpen(true);
            }}
            onBlur={(event) => {
              if (!event.currentTarget.contains(event.relatedTarget)) setIsMediaDrawerOpen(false);
            }}
          >
            <span
              aria-hidden="true"
              className={`absolute ${isMediaDrawerOpen ? "pointer-events-auto" : "pointer-events-none"}`}
              style={{ top: -90, left: -10, width: 44, height: 82, borderRadius: 999, backgroundColor: "#242626", opacity: isMediaDrawerOpen ? 1 : 0, transform: `scaleY(${isMediaDrawerOpen ? 1 : 0})`, transformOrigin: "bottom", transition: "transform 150ms ease-out, opacity 75ms ease-out" }}
            />
            <span
              aria-hidden="true"
              className={`absolute ${isMediaDrawerOpen ? "pointer-events-auto" : "pointer-events-none"}`}
              style={{ top: -8, left: -10, width: 44, height: 12 }}
            />
            <span
              aria-hidden="true"
              className={`absolute ${isDrawerReady ? "pointer-events-auto" : "pointer-events-none"}`}
              style={{ top: -18, left: -15, width: 22, height: 24, borderRadius: 999 }}
            />
            <button type="button" aria-label="Add media" aria-expanded={isMediaDrawerOpen} tabIndex={isDrawerReady ? 0 : -1} className="absolute z-20 grid size-6 cursor-pointer place-items-center bg-transparent" style={{ backgroundColor: "transparent" }}>
              <span aria-hidden="true" className="material-symbols-outlined !text-[24px]" style={{ color: "#8a8a92" }}>add_circle</span>
            </button>
            <button
              type="button"
              aria-label="Create poll"
              tabIndex={isDrawerOpen ? 0 : -1}
              className={`absolute z-10 grid h-6 cursor-pointer place-items-center bg-transparent ${isMediaDrawerOpen ? "pointer-events-auto" : "pointer-events-none"}`}
              style={{ top: -77, left: -10, width: 44, backgroundColor: "transparent", opacity: isMediaDrawerOpen ? 1 : 0, transform: `translateY(${isMediaDrawerOpen ? 0 : 8}px)`, transition: "transform 250ms ease-out, opacity 150ms ease-out" }}
            >
              <svg aria-hidden="true" width="24" height="24" viewBox="0 0 960 960" fill="#8a8a92">
                <rect x="160" y="640" width="320" height="160" rx="80" />
                <rect x="160" y="400" width="640" height="160" rx="80" />
                <rect x="160" y="160" width="440" height="160" rx="80" />
              </svg>
            </button>
            <button
              type="button"
              aria-label="Add event"
              tabIndex={isDrawerOpen ? 0 : -1}
              className={`absolute z-10 grid h-6 cursor-pointer place-items-center bg-transparent ${isMediaDrawerOpen ? "pointer-events-auto" : "pointer-events-none"}`}
              style={{ top: -45, left: -10, width: 44, backgroundColor: "transparent", opacity: isMediaDrawerOpen ? 1 : 0, transform: `translateY(${isMediaDrawerOpen ? 0 : 8}px)`, transition: "transform 250ms ease-out, opacity 150ms ease-out" }}
            >
              <span aria-hidden="true" className="material-symbols-outlined !text-[18px]" style={{ color: "#8a8a92" }}>calendar_month</span>
            </button>
          </div>
          <button
            type="button"
            aria-label="Send later"
            tabIndex={isDrawerOpen ? 0 : -1}
            className={`absolute grid size-6 cursor-pointer place-items-center motion-reduce:transition-none ${isDrawerOpen ? "opacity-100" : "pointer-events-none opacity-0"}`}
            style={{ left: 116, top: "50%", backgroundColor: "transparent", transform: `translate(${isDrawerOpen ? 0 : -42}px, -50%)`, transition: "transform 250ms ease-out, opacity 150ms ease-out" }}
          >
            <svg aria-hidden="true" width="24" height="24" viewBox="0 -960 960 960" fill="#8a8a92">
              <path transform="translate(960 0) scale(-1 1)" d="M480-120q-138 0-240.5-91.5T122-440h82q14 104 92.5 172T480-200q117 0 198.5-81.5T760-480q0-117-81.5-198.5T480-760q-69 0-129 32t-101 88h110v80H120v-240h80v94q51-64 124.5-99T480-840q75 0 140.5 28.5t114 77q48.5 48.5 77 114T840-480q0 75-28.5 140.5t-77 114q-48.5 48.5-114 77T480-120Zm112-192L440-464v-216h80v184l128 128-56 56Z" />
            </svg>
          </button>
        </div>
      ) : null}
      {isRecording ? (
        <div className={`voice-recording-pill flex h-11 min-w-0 flex-1 items-center gap-2 rounded-full border-2 bg-[#202223] px-2 text-white sm:gap-3 ${paused ? "is-paused" : "is-recording"}`}>
          {paused ? (
            <button type="button" aria-label={isPlaying ? "Pause playback" : "Play recording"} className="grid size-8 shrink-0 place-items-center rounded-full bg-transparent" onClick={() => setIsPlaying((playing) => !playing)}>
              <span aria-hidden="true" className="material-symbols-outlined">{isPlaying ? "pause" : "play_arrow"}</span>
            </button>
          ) : (
            <span className="grid size-8 shrink-0 place-items-center" aria-hidden="true">
              <span className="size-2.5 rounded-full bg-[#ff7f96]" />
            </span>
          )}
          {paused ? (
            <div className="flex min-w-10 flex-1 items-center gap-1.5" aria-hidden="true">
              <span className="size-3 shrink-0 rounded-full bg-[#00a884]" />
              <span className={`voice-playback-track h-0.5 flex-1 overflow-hidden rounded-full bg-white/15 ${isPlaying ? "is-playing" : ""}`} />
            </div>
          ) : (
            <span className="voice-wave min-w-10 flex-1" aria-hidden="true" />
          )}
          <span className="shrink-0 text-base tabular-nums">{formattedTime}</span>

          <button
            type="button"
            aria-label={paused ? "Resume recording" : "Pause recording"}
            className="grid size-9 shrink-0 place-items-center rounded-full bg-transparent text-[#ff7f96]"
            onClick={() => {
              setRecordingState(paused ? "recording" : "paused");
              setIsPlaying(false);
            }}
          >
            <span aria-hidden="true" className="material-symbols-outlined">{paused ? "radio_button_checked" : "pause"}</span>
          </button>
          <button
            type="button"
            aria-label={viewOnce ? "Disable view once" : "Enable view once"}
            aria-pressed={viewOnce}
            className="grid size-9 shrink-0 place-items-center rounded-full bg-transparent"
            onClick={() => setViewOnce((enabled) => !enabled)}
          >
            <span
              aria-hidden="true"
              className="size-8 bg-contain bg-center bg-no-repeat"
              style={{ backgroundImage: `url('${viewOnce ? "/view-once-on-nobg.svg" : "/view-once-off-nobg.svg"}')` }}
            />
          </button>
          <button type="button" aria-label="Discard recording" className="voice-discard-button grid size-9 shrink-0 place-items-center overflow-hidden rounded-full bg-transparent" onClick={discardRecording}>
            <span aria-hidden="true" className="material-symbols-outlined">delete</span>
          </button>
        </div>
      ) : (
        <div className="relative min-w-0 flex-1">
          <input
            ref={inputRef}
            aria-label="Message"
            className="h-11 w-full rounded-full bg-[#242626] py-3 pl-4 pr-12 text-sm text-white caret-green-400 outline-none placeholder:text-white/60"
            placeholder="Type a message"
            value={messageText}
            onChange={(event) => setMessageText(event.target.value)}
          />
          <span
            aria-hidden="true"
            className="material-symbols-outlined absolute top-1/2 z-10 -translate-y-1/2 cursor-pointer !text-[24px] transition-colors"
            style={{ right: 12, color: "#8a8a92", clipPath: "circle(50%)" }}
            onMouseEnter={(event) => { event.currentTarget.style.color = "#00a884"; }}
            onMouseLeave={(event) => { event.currentTarget.style.color = "#8a8a92"; }}
          >mood</span>
        </div>
      )}
      <button
        type={hasText ? "submit" : "button"}
        aria-label={isRecording ? "Send voice message" : hasText ? "Send message" : "Record voice message"}
        className={`composer-action-button relative grid size-11 shrink-0 place-items-center overflow-hidden rounded-full transition-colors duration-200 ${isRecording || hasText ? "is-send" : "bg-[#242626] text-white"}`}
        onClick={!isRecording && !hasText ? () => {
          setRecordingSeconds(0);
          setRecordingState("recording");
        } : undefined}
      >
        <span aria-hidden="true" className={`material-symbols-outlined absolute !text-[25px] transition-all duration-200 ${isRecording || hasText ? "-translate-y-2 scale-50 opacity-0" : "translate-y-0 scale-100 opacity-100"}`} style={{ color: "#8a8a92" }}>mic</span>
        <span aria-hidden="true" className={`material-symbols-outlined absolute !text-[25px] text-[#081c15] transition-all duration-200 ${hasText ? "translate-y-0 scale-100 opacity-100" : "translate-y-2 scale-50 opacity-0"}`}>arrow_upward</span>
        <span aria-hidden="true" className={`material-symbols-outlined absolute !text-[25px] text-[#081c15] transition-all duration-200 ${isRecording ? "translate-y-0 scale-100 opacity-100" : "translate-y-2 scale-50 opacity-0"}`}>arrow_upward</span>
      </button>
    </form>
  );
}

export default function CurrentChat() {
  const [scrollToBottomRequest, setScrollToBottomRequest] = useState(0);
  const [infoOpen, setInfoOpen] = useState(false);
  const {
    chatId,
    contact,
    group,
    messages,
    isLoading,
    error,
    hasMoreMessages,
    unreadCount,
    sendMessage,
    loadOlderMessages,
  } = useCurrentChat();
  const { chats: { complete } } = useChats();
  const { profile: { blueTickEnabled, id: userId } } = useProfile();
  if (!chatId) {
    return (
      <section className="flex h-full w-full items-center justify-center bg-[#1d1f1f] text-white">
        Please select a chat to see messages
      </section>
    );
  }
  const cannotSend = complete.some((chat) => chat.id === chatId && chat.group && !chat.canSend);

  return (
    <section className="flex h-full min-h-0 w-full flex-col overflow-hidden">
      <ContactHeader
        infoOpen={infoOpen}
        onToggleInfo={() => setInfoOpen((open) => !open)}
      />
      <div className="flex min-h-0 flex-1 overflow-hidden">
        <div className="relative flex min-h-0 min-w-0 flex-1 flex-col items-center justify-end bg-[#161717] bg-[url('/background.webp')] bg-repeat">
          <MessageList
            key={chatId}
            chatId={chatId}
            messages={messages}
            contacts={group?.contacts}
            isGroup={Boolean(group)}
            blueTickEnabled={blueTickEnabled}
            isLoading={isLoading}
            error={error}
            hasMoreMessages={hasMoreMessages}
            loadOlderMessages={loadOlderMessages}
            unreadCount={unreadCount}
            scrollToBottomRequest={scrollToBottomRequest}
          />
          {cannotSend ? (
            <div className="z-30 w-full px-4 pb-2 pt-2">
              <p className="rounded-full bg-[#242626] px-4 py-3 text-center text-sm text-white/55" role="status">You don&apos;t have permission to send a message</p>
            </div>
          ) : (
            <Composer
              chatId={chatId}
              sendMessage={sendMessage}
              onSent={() => setScrollToBottomRequest((request) => request + 1)}
            />
          )}
        </div>
        {infoOpen ? (
          <ChatInfoPanel
            key={chatId}
            chatId={chatId}
            contact={contact}
            group={group}
            messages={messages}
            userId={userId}
          />
        ) : null}
      </div>
    </section>
  );
}
