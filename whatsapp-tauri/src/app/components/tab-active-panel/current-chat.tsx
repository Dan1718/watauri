import { FormEvent, memo, useCallback, useLayoutEffect, useMemo, useRef, useState } from "react";
import { Virtuoso, type VirtuosoHandle } from "react-virtuoso";
import { Message } from "@/app/context/chats-provider";
import { CurrentChatContacts } from "@/app/context/current-chat-provider";
import { useCurrentChat } from "@/app/hooks/use-current-chat";
import { useProfile } from "@/app/hooks/use-profile";
import { getDisplayNameFromJid } from "@/app/utils";
import Reaction from "../message/reaction";
import ContactHeader from "./contact-header";
import ChatMessage from "./chat-message";
import MessageReactions from "./message-reactions";

const INITIAL_ITEM_INDEX = 1_000_000;

function MessageListHeader() {
  return (
    <div className="flex w-full justify-center px-4 pb-4 pt-4">
      <p className="rounded-full bg-[#202c33] px-2 py-1 text-xs text-white/55">
        Today
      </p>
    </div>
  );
}

const virtuosoComponents = { Header: MessageListHeader };

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
  scrollToBottomRequest: number;
}) {
  const [activeReactionId, setActiveReactionId] = useState<string | null>(null);
  const messageIndexes = useMemo(
    () => new Map(messages.map((message, index) => [message.id, index])),
    [messages]
  );
  const previousMessages = useRef(messages);
  const committedFirstItemIndex = useRef(INITIAL_ITEM_INDEX);
  const virtuosoRef = useRef<VirtuosoHandle>(null);
  let firstItemIndex = committedFirstItemIndex.current;

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
      <Virtuoso
        ref={virtuosoRef}
        key={chatId}
        className="h-full w-full"
        data={messages}
        firstItemIndex={firstItemIndex}
        initialTopMostItemIndex={Math.max(0, messages.length - 1)}
        alignToBottom
        followOutput={(isAtBottom) => isAtBottom ? "auto" : false}
        increaseViewportBy={{ top: 400, bottom: 200 }}
        components={virtuosoComponents}
        computeItemKey={(_index, message) => message.id}
        startReached={() => {
          if (hasMoreMessages && !isLoading) void loadOlderMessages();
        }}
        itemContent={(_index, message) => {
          const index = messageIndexes.get(message.id)!;
          const previous = messages[index - 1];
          const next = messages[index + 1];
          const repeatedIncomingSender = isGroup && !message.isSentFromUser &&
            !previous?.isSentFromUser && previous?.contactId === message.contactId;
          const compact = next?.isSentFromUser === message.isSentFromUser &&
            next?.contactId === message.contactId;
          const contact = contacts?.[message.contactId];

          return (
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
          );
        }}
      />
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
    <form className="z-30 h-auto w-full p-4" onSubmit={handleSubmit}>
      <input
        aria-label="Message"
        className="w-full rounded-full bg-white/15 p-3 px-4 text-sm text-white caret-green-400 outline-none placeholder:text-white/60"
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
  const {
    chatId,
    group,
    messages,
    isLoading,
    error,
    isSending,
    hasMoreMessages,
    sendMessage,
    loadOlderMessages,
  } = useCurrentChat();
  const { profile: { blueTickEnabled } } = useProfile();

  if (!chatId) {
    return (
      <section className="flex h-full w-full items-center justify-center text-white">
        Please select a chat to see messages
      </section>
    );
  }

  return (
    <section className="flex h-full min-h-0 w-full flex-col">
      <ContactHeader />
      <div
        className="relative flex min-h-0 w-full flex-1 flex-col items-center justify-end bg-[#161717] bg-[url('/background.webp')] bg-repeat bg-blend-soft-light"
      >
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
          scrollToBottomRequest={scrollToBottomRequest}
        />
        <Composer
          isSending={isSending}
          sendMessage={sendMessage}
          onSent={() => setScrollToBottomRequest((request) => request + 1)}
        />
      </div>
    </section>
  );
}
