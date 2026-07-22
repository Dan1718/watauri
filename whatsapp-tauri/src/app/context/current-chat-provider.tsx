import {
  createContext,
  PropsWithChildren,
  useCallback,
  useEffect,
  useMemo,
  useRef,
  useState,
} from "react";
import { Chat, Message } from "./chats-provider";
import { useChats } from "../hooks/use-chats";
import { useContacts } from "../hooks/use-contacts";
import { Contact } from "./contacts-provider";
import { getDisplayNameFromJid } from "../utils";
import { BackendMessage, listBackendMessages, sendBackendMessage } from "../backend";
import { useChatPollingActive } from "../hooks/use-chat-polling-active";

export type CurrentChatContacts = {
  [contactId: string]: Contact | undefined;
};

export type CurrentChatContactsGroup = {
  name: string;
  avatar: string;
  contacts: CurrentChatContacts;
};

export type CurrentChatData = {
  chatId: string | null;
  contact: Contact | null;
  messages: Message[];
  group: CurrentChatContactsGroup | null;
  page: number;
  unreadCount: number;
  isLoading: boolean;
  error: string | null;
  isSending: boolean;
  hasMoreMessages: boolean;
};

export type CurrentChat = CurrentChatData & {
  loadCurrentChat: (chat: Partial<CurrentChatData>) => void;
  sendMessage: (text: string) => Promise<boolean>;
  loadOlderMessages: () => Promise<void>;
};

export const CurrentChatContext = createContext<undefined | CurrentChat>(
  undefined
);

function toMessage(message: BackendMessage, fallbackContactId: string): Message {
  const isFromMe = Boolean(message.isFromMe);
  return {
    id: message.id,
    contactId: isFromMe ? fallbackContactId : message.senderId,
    message: message.text,
    timestamp: message.timestamp,
    isSentFromUser: isFromMe,
    sent: true,
    delivered: message.status === "delivered" || message.status === "read",
    read: message.status === "read",
    mediaType: message.mediaType,
  };
}

function sameMessage(a: Message, b: Message) {
  return a.id === b.id && a.contactId === b.contactId && a.message === b.message &&
    a.timestamp === b.timestamp && a.isSentFromUser === b.isSentFromUser &&
    a.read === b.read && a.sent === b.sent && a.delivered === b.delivered &&
    a.mediaType === b.mediaType &&
    (a.reactions === b.reactions || Boolean(a.reactions && b.reactions &&
      a.reactions.length === b.reactions.length && a.reactions.every((reaction, index) =>
        reaction.emoji === b.reactions![index].emoji && reaction.count === b.reactions![index].count)));
}

export function mergeMessages(previous: Message[], incoming: Message[], prepend = false) {
  if (incoming.length === 0) return previous;
  const previousById = new Map(previous.map((message) => [message.id, message]));
  const previousIds = new Set(previousById.keys());
  const incomingById = new Map(incoming.map((message) => [message.id, message]));
  incomingById.forEach((message, id) => {
    const existing = previousById.get(message.id);
    previousById.set(id, existing && sameMessage(existing, message) ? existing : message);
  });
  const compare = (a: Message, b: Message) => {
    const time = (typeof a.timestamp === "number" ? a.timestamp : Date.parse(a.timestamp)) -
      (typeof b.timestamp === "number" ? b.timestamp : Date.parse(b.timestamp));
    return time || (a.id < b.id ? -1 : a.id > b.id ? 1 : 0);
  };
  const merged = prepend
    ? [
        ...[...incomingById.values()].filter((message) => !previousIds.has(message.id)).sort(compare),
        ...[...previousById.values()].filter((message) => previousIds.has(message.id)),
      ]
    : [...previousById.values()].sort(compare);
  return merged.length === previous.length && merged.every((message, index) => message === previous[index])
    ? previous
    : merged;
}

type CachedMessages = {
  messages: Message[];
  nextCursor: string | null;
  latestCursor: string | null;
  hasMore: boolean;
  initialized: boolean;
};

type ActiveRequest = {
  chatId: string;
  controller: AbortController;
  done: Promise<void>;
};

function sameContact(a: Contact | undefined | null, b: Contact | undefined | null) {
  return a === b || Boolean(a && b && a.id === b.id && a.displayName === b.displayName &&
    a.contactAvatar === b.contactAvatar && a.statusMessage === b.statusMessage && a.typing === b.typing);
}

export default function CurrentChatProvider({ children }: PropsWithChildren) {
  const [currentChat, setCurrentChat] = useState<CurrentChatData>({
    chatId: null,
    contact: null,
    messages: [],
    group: null,
    page: 0,
    unreadCount: 0,
    isLoading: false,
    error: null,
    isSending: false,
    hasMoreMessages: false,
  });
  const {
    chats: { complete },
  } = useChats();
  const { contacts } = useContacts();
  const pollingActive = useChatPollingActive();
  const cacheRef = useRef(new Map<string, CachedMessages>());
  const requestRef = useRef<ActiveRequest | null>(null);
  const olderRequestRef = useRef<{ chatId: string; promise: Promise<void> } | null>(null);
  const chatsRef = useRef(complete);
  const currentChatRef = useRef(currentChat);
  chatsRef.current = complete;
  currentChatRef.current = currentChat;

  const requestPage = useCallback(async (chatId: string, direction: "initial" | "newer" | "older") => {
    while (requestRef.current) {
      if (direction === "newer") return;
      await requestRef.current.done;
    }
    const cached = cacheRef.current.get(chatId);
    if (direction === "older" && (!cached?.hasMore || !cached.nextCursor)) return;

    const controller = new AbortController();
    let finishRequest!: () => void;
    const done = new Promise<void>((resolve) => { finishRequest = resolve; });
    requestRef.current = { chatId, controller, done };
    if (direction === "initial") {
      setCurrentChat((prev) => prev.chatId === chatId ? { ...prev, isLoading: true, error: null } : prev);
    }

    try {
      const page = await listBackendMessages(chatId, {
        limit: 100,
        before: direction === "older" ? cached?.nextCursor ?? undefined : undefined,
        after: direction === "newer" ? cached?.latestCursor ?? undefined : undefined,
        signal: controller.signal,
      });
      if (controller.signal.aborted) return;

      const chat = chatsRef.current.find((item: Chat) => item.id === chatId);
      const fallbackContactId = typeof chat?.contactId === "string" ? chat.contactId : chatId;
      const incoming = page.messages.map((message) => toMessage(message, fallbackContactId));
      const current = cacheRef.current.get(chatId);
      const candidate: CachedMessages = direction === "initial" || !current
        ? {
            messages: mergeMessages(incoming, current?.messages ?? []),
            nextCursor: page.nextCursor,
            latestCursor: page.latestCursor,
            hasMore: page.hasMore,
            initialized: true,
          }
        : direction === "older"
          ? {
              ...current,
              messages: mergeMessages(current.messages, incoming, true),
              nextCursor: page.nextCursor,
              hasMore: page.hasMore,
            }
          : {
              ...current,
              messages: mergeMessages(current.messages, incoming),
              latestCursor: page.latestCursor ?? current.latestCursor,
            };
      const next = current && candidate.messages === current.messages &&
        candidate.nextCursor === current.nextCursor &&
        candidate.latestCursor === current.latestCursor &&
        candidate.hasMore === current.hasMore &&
        candidate.initialized === current.initialized
        ? current
        : candidate;
      cacheRef.current.set(chatId, next);
      setCurrentChat((prev) => {
        if (prev.chatId !== chatId) return prev;
        if (prev.messages === next.messages && prev.hasMoreMessages === next.hasMore &&
          !prev.isLoading && prev.error === null) return prev;
        return {
          ...prev,
          messages: next.messages,
          hasMoreMessages: next.hasMore,
          isLoading: false,
          error: null,
        };
      });
    } catch (error) {
      if (controller.signal.aborted) return;
      setCurrentChat((prev) => prev.chatId === chatId
        ? {
            ...prev,
            isLoading: false,
            error: error instanceof Error ? error.message : "Failed to load messages",
          }
        : prev);
    } finally {
      if (requestRef.current?.controller === controller) requestRef.current = null;
      finishRequest();
    }
  }, []);

  useEffect(() => {
    const chatId = currentChat.chatId;
    if (!chatId || !pollingActive) return;

    void requestPage(chatId, cacheRef.current.get(chatId)?.initialized ? "newer" : "initial");
    const interval = setInterval(() => void requestPage(chatId, "newer"), 3000);
    return () => {
      clearInterval(interval);
      if (requestRef.current?.chatId === chatId) {
        requestRef.current.controller.abort();
      }
    };
  }, [currentChat.chatId, pollingActive, requestPage]);

  useEffect(() => {
    const chat = complete.find((chat: Chat) => chat.id === currentChat.chatId);
    if (chat) {
      if (typeof chat.contactId == "string") {
        const contactId = chat.contactId;
        const contact = contacts.find(
          (contact: Contact) => contact.id === contactId
        ) ?? {
          id: contactId,
          displayName: getDisplayNameFromJid(contactId),
          contactAvatar: "",
          statusMessage: "",
        };
        setCurrentChat((prev) => sameContact(prev.contact, contact) && prev.group === null
          ? prev
          : { ...prev, contact, group: null });
      } else {
        const groupContacts: CurrentChatContacts = {};
        chat.contactId.forEach((groupContact: string) => {
          groupContacts[groupContact] = contacts.find(
            (contact: Contact) => contact.id === groupContact
          );
        });
        const group = {
          name: chat.groupName ?? getDisplayNameFromJid(chat.id),
          avatar: chat.groupAvatar ?? "",
          contacts: groupContacts,
        };
        setCurrentChat((prev) => {
          const previousContacts = prev.group?.contacts;
          const contactIds = Object.keys(groupContacts);
          if (prev.contact === null && prev.group?.name === group.name &&
            prev.group.avatar === group.avatar && previousContacts &&
            Object.keys(previousContacts).length === contactIds.length &&
            contactIds.every((id) => sameContact(previousContacts[id], groupContacts[id]))) return prev;
          return { ...prev, contact: null, group };
        });
      }
    }
  }, [complete, contacts, currentChat.chatId]);

  const loadCurrentChat = useCallback((chat: Partial<CurrentChatData>) => {
    setCurrentChat((prev) => {
      const isNewChat = chat.chatId !== undefined && chat.chatId !== prev.chatId;
      const cached = isNewChat && chat.chatId ? cacheRef.current.get(chat.chatId) : undefined;
      return {
        ...prev,
        ...(isNewChat ? {
          contact: null,
          group: null,
          messages: cached?.messages ?? [],
          hasMoreMessages: cached?.hasMore ?? false,
          isLoading: !cached?.initialized,
          error: null,
          isSending: false,
        } : {}),
        ...chat,
      };
    });
  }, []);

  const loadOlderMessages = useCallback(async () => {
    const chatId = currentChatRef.current.chatId;
    if (!chatId) return;
    if (olderRequestRef.current?.chatId === chatId) return olderRequestRef.current.promise;
    const promise = requestPage(chatId, "older").finally(() => {
      if (olderRequestRef.current?.promise === promise) olderRequestRef.current = null;
    });
    olderRequestRef.current = { chatId, promise };
    await promise;
  }, [requestPage]);

  const sendMessage = useCallback(async (text: string) => {
    const trimmedText = text.trim();
    const chatId = currentChatRef.current.chatId;
    if (!chatId || !trimmedText) return false;

    setCurrentChat((prev) => ({ ...prev, isSending: true, error: null }));
    try {
      const sentMessage = await sendBackendMessage(chatId, trimmedText);
      const chat = chatsRef.current.find((item: Chat) => item.id === chatId);
      const fallbackContactId = typeof chat?.contactId === "string" ? chat.contactId : chatId;
      const message = toMessage(sentMessage, fallbackContactId);
      const cached = cacheRef.current.get(chatId);
      if (cached) {
        cacheRef.current.set(chatId, { ...cached, messages: mergeMessages(cached.messages, [message]) });
      } else {
        cacheRef.current.set(chatId, {
          messages: [message], nextCursor: null, latestCursor: null, hasMore: false, initialized: false,
        });
      }
      setCurrentChat((prev) =>
        prev.chatId === chatId
          ? {
              ...prev,
              messages: mergeMessages(prev.messages, [message]),
              isSending: false,
              error: null,
            }
          : prev
      );
      return currentChatRef.current.chatId === chatId;
    } catch (error) {
      setCurrentChat((prev) => prev.chatId === chatId ? {
        ...prev,
        isSending: false,
        error: error instanceof Error ? error.message : "Failed to send message",
      } : prev);
      return false;
    }
  }, []);

  const value = useMemo(
    () => ({ ...currentChat, loadCurrentChat, sendMessage, loadOlderMessages }),
    [currentChat, loadCurrentChat, sendMessage, loadOlderMessages]
  );

  return (
    <CurrentChatContext.Provider
      value={value}
    >
      {children}
    </CurrentChatContext.Provider>
  );
}
