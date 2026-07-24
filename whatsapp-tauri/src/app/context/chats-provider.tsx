import {
  createContext,
  PropsWithChildren,
  useCallback,
  useEffect,
  useMemo,
  useRef,
  useState,
} from "react";
import { BackendChat, BackendMessage, BackendUser, listBackendChats } from "../backend";
import { getDisplayNameFromJid } from "../utils";
import { useChatPollingActive } from "../hooks/use-chat-polling-active";

export enum Filters {
  ALL = "all",
  UNREAD = "unread",
  FAVORITES = "favorites",
  GROUPS = "groups",
}

export type ReactionType = {
  emoji: string;
  count: number;
};

export type Message = {
  id: string;
  contactId: string;
  message: string;
  timestamp: number | string;
  isSentFromUser: boolean;
  read?: boolean;
  sent?: boolean;
  delivered?: boolean;
  pending?: boolean;
  reactions?: ReactionType[];
  mediaType?: string;
};

export type Chat = {
  id: string;
  contactId: string | string[];
  groupName?: string;
  groupAvatar?: string;
  participants?: BackendUser[];
  unreadCount: number;
  read: boolean;
  group: boolean;
  favorite: boolean;
  messages: Message[];
};

export type Chats = {
  complete: Chat[];
  filtered: Chat[];
  isLoading: boolean;
  error: string | null;
};

export const ChatsContext = createContext<
  | undefined
  | {
      filter: string;
      updateFilter: (filter: string) => void;
      search: string;
      updateSearch: (query: string) => void;
      chats: Chats;
    }
>(undefined);

function getDirectContactId(chat: BackendChat) {
  return chat.participants?.find((participant) => participant.id !== "me")?.id ?? chat.id;
}

function toMessage(message: BackendMessage, fallbackContactId: string): Message {
  const isFromMe = Boolean(message.isFromMe);
  return {
    id: message.id,
    contactId: isFromMe ? fallbackContactId : message.senderId,
    message: message.text,
    timestamp: message.timestamp,
    isSentFromUser: isFromMe,
    sent: message.status !== "pending",
    delivered: message.status === "delivered" || message.status === "read",
    read: message.status === "read",
    pending: message.status === "pending",
    mediaType: message.mediaType,
  };
}

function sameMessage(a: Message, b: Message) {
  return a.id === b.id && a.contactId === b.contactId && a.message === b.message &&
    a.timestamp === b.timestamp && a.isSentFromUser === b.isSentFromUser &&
    a.read === b.read && a.sent === b.sent && a.delivered === b.delivered &&
    a.pending === b.pending &&
    a.mediaType === b.mediaType;
}

function sameChat(a: Chat, b: Chat) {
  const contactsEqual = typeof a.contactId === "string"
    ? a.contactId === b.contactId
    : Array.isArray(b.contactId) && a.contactId.length === b.contactId.length &&
      a.contactId.every((id, index) => id === b.contactId[index]);
  const participantsEqual = a.participants?.length === b.participants?.length &&
    a.participants?.every((participant, index) => {
      const other = b.participants?.[index];
      return participant.id === other?.id && participant.name === other?.name &&
        participant.avatar === other?.avatar && participant.status === other?.status &&
        participant.phone === other?.phone && participant.isSaved === other?.isSaved;
    });
  return a.id === b.id && contactsEqual && participantsEqual && a.groupName === b.groupName &&
    a.groupAvatar === b.groupAvatar && a.unreadCount === b.unreadCount &&
    a.read === b.read && a.group === b.group &&
    a.favorite === b.favorite && a.messages.length === b.messages.length &&
    a.messages.every((message, index) => sameMessage(message, b.messages[index]));
}

export function mergeChats(previous: Chat[], incoming: Chat[]) {
  const previousById = new Map(previous.map((chat) => [chat.id, chat]));
  const merged = incoming.map((chat) => {
    const existing = previousById.get(chat.id);
    return existing && sameChat(existing, chat) ? existing : chat;
  });
  return merged.length === previous.length && merged.every((chat, index) => chat === previous[index])
    ? previous
    : merged;
}

function toChat(chat: BackendChat): Chat {
  const directContactId = getDirectContactId(chat);
  const participants = chat.participants ?? [];
  const contactId = chat.isGroup
    ? participants.map((participant) => participant.id)
    : directContactId;

  return {
    id: chat.id,
    contactId,
    groupName: chat.name || (chat.isGroup ? getDisplayNameFromJid(chat.id) : undefined),
    groupAvatar: chat.avatar,
    participants,
    unreadCount: chat.unreadCount,
    read: chat.unreadCount === 0,
    group: chat.isGroup,
    favorite: Boolean(chat.isStarred),
    messages: chat.lastMessage
      ? [toMessage(chat.lastMessage, chat.isGroup ? "me" : directContactId)]
      : [],
  };
}

export default function ChatsProvider({ children }: PropsWithChildren) {
  const [filter, setFilter] = useState<Filters>(Filters.ALL);
  const [search, setSearch] = useState<string>("");
  const [chatState, setChatState] = useState<Omit<Chats, "filtered">>({
    complete: [],
    isLoading: false,
    error: null,
  });
  const pollingActive = useChatPollingActive();
  const loadedRef = useRef(false);

  const updateFilter = useCallback((filter: string) => {
    setFilter(filter as Filters);
  }, []);

  const updateSearch = useCallback((query: string) => {
    setSearch(query);
  }, []);

  useEffect(() => {
    if (!pollingActive) return;
    const controller = new AbortController();
    let inFlight = false;

    const fetchChats = async () => {
      if (inFlight) return;
      inFlight = true;
      if (!loadedRef.current) {
        setChatState((prev) => ({ ...prev, isLoading: true }));
      }
      try {
        const data = (await listBackendChats(controller.signal)).map(toChat);
        loadedRef.current = true;
        setChatState((prev) => {
          const complete = mergeChats(prev.complete, data);
          if (complete === prev.complete && !prev.isLoading && prev.error === null) return prev;
          return { complete, isLoading: false, error: null };
        });
      } catch (error) {
        if (controller.signal.aborted) return;
        setChatState((prev) => ({
          ...prev,
          isLoading: false,
          error: error instanceof Error ? error.message : "Failed to load chats",
        }));
      } finally {
        inFlight = false;
      }
    };

    void fetchChats();
    const interval = setInterval(() => void fetchChats(), 5000);

    return () => {
      controller.abort();
      clearInterval(interval);
    };
  }, [pollingActive]);

  const filtered = useMemo(() => chatState.complete.filter((chat) => {
    if (filter === Filters.UNREAD && chat.read) return false;
    if (filter === Filters.FAVORITES && !chat.favorite) return false;
    if (filter === Filters.GROUPS && !chat.group) return false;
    return true;
  }), [filter, chatState.complete]);
  const chats = useMemo(() => ({ ...chatState, filtered }), [chatState, filtered]);
  const value = useMemo(
    () => ({ chats, filter, search, updateFilter, updateSearch }),
    [chats, filter, search, updateFilter, updateSearch]
  );

  return (
    <ChatsContext.Provider
      value={value}
    >
      {children}
    </ChatsContext.Provider>
  );
}
