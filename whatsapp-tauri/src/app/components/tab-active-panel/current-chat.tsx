import { FormEvent, memo, useCallback, useLayoutEffect, useMemo, useRef, useState } from "react";
import { ArrowDownIcon } from "@phosphor-icons/react";
import dayjs from "dayjs";
import { Virtuoso, type VirtuosoHandle } from "react-virtuoso";
import { Message } from "@/app/context/chats-provider";
import { CurrentChatContacts } from "@/app/context/current-chat-provider";
import { useCurrentChat } from "@/app/hooks/use-current-chat";
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
  isSending,
  sendMessage,
  onSent,
}: {
  isSending: boolean;
  sendMessage: (text: string) => Promise<boolean>;
  onSent: () => void;
}) {
  const [messageText, setMessageText] = useState("");

  const handleSubmit = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    const text = messageText.trim();
    if (!text || isSending) return;
    if (await sendMessage(text)) {
      setMessageText("");
      onSent();
    }
  };

  return (
    <form className="z-30 h-auto w-full px-4 pb-2 pt-1" onSubmit={handleSubmit}>
      <input
        aria-label="Message"
        className="w-full rounded-full bg-[#242626] p-3 px-4 text-sm text-white caret-green-400 outline-none placeholder:text-white/60"
        disabled={isSending}
        placeholder={isSending ? "Sending..." : "Type a message"}
        value={messageText}
        onChange={(event) => setMessageText(event.target.value)}
      />
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
    isSending,
    hasMoreMessages,
    unreadCount,
    sendMessage,
    loadOlderMessages,
  } = useCurrentChat();
  const { profile: { blueTickEnabled, id: userId } } = useProfile();
  if (!chatId) {
    return (
      <section className="flex h-full w-full items-center justify-center text-white">
        Please select a chat to see messages
      </section>
    );
  }

  return (
    <section className="flex h-full min-h-0 w-full overflow-hidden">
      <div className="flex min-w-0 flex-1 flex-col">
        <ContactHeader
          infoOpen={infoOpen}
          onToggleInfo={() => setInfoOpen((open) => !open)}
        />
        <div className="relative flex min-h-0 w-full flex-1 flex-col items-center justify-end bg-[#161717] bg-[url('/background.webp')] bg-repeat">
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
          <Composer
            isSending={isSending}
            sendMessage={sendMessage}
            onSent={() => setScrollToBottomRequest((request) => request + 1)}
          />
        </div>
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
    </section>
  );
}
