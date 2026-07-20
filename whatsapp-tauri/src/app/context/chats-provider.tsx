import { createContext, PropsWithChildren, useEffect, useState } from "react";
import { BackendChat, BackendMessage, listBackendChats } from "../backend";
import { getDisplayNameFromJid } from "../utils";

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
  contactId: string;
  message: string;
  timestamp: number | string;
  isSentFromUser: boolean;
  read?: boolean;
  sent?: boolean;
  delivered?: boolean;
  reactions?: ReactionType[];
};

export type Chat = {
  id: string;
  contactId: string | string[];
  groupName?: string;
  groupAvatar?: string;
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
    contactId: isFromMe ? fallbackContactId : message.senderId,
    message: message.text,
    timestamp: message.timestamp,
    isSentFromUser: isFromMe,
    sent: true,
    delivered: message.status === "delivered" || message.status === "read",
    read: message.status === "read",
  };
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
  const [chats, setChats] = useState<Chats>({
    complete: [],
    filtered: [],
    isLoading: false,
    error: null,
  });

  const updateFilter = (filter: string) => {
    setFilter(filter as Filters);
  };

  const updateSearch = (query: string) => {
    setSearch(query);
  };

  useEffect(() => {
    let active = true;

    const fetchChats = async () => {
      setChats((prev) => ({ ...prev, isLoading: prev.complete.length === 0 }));
      try {
        const data = (await listBackendChats()).map(toChat);
        if (!active) return;
        setChats((prev) => ({
          ...prev,
          complete: data,
          filtered: data,
          isLoading: false,
          error: null,
        }));
      } catch (error) {
        if (!active) return;
        setChats((prev) => ({
          ...prev,
          isLoading: false,
          error: error instanceof Error ? error.message : "Failed to load chats",
        }));
      }
    };

    void fetchChats();
    const interval = setInterval(() => void fetchChats(), 5000);

    return () => {
      active = false;
      clearInterval(interval);
    };
  }, []);

  useEffect(() => {
    setChats((prev) => {
      const complete = prev.complete;
      const filtered = complete.filter((chat) => {
        if (filter === Filters.UNREAD && chat.read) return false;
        if (filter === Filters.FAVORITES && !chat.favorite) return false;
        if (filter === Filters.GROUPS && !chat.group) return false;
        return true;
      });

      return {
        ...prev,
        filtered,
      };
    });
  }, [filter, chats.complete]);

  return (
    <ChatsContext.Provider
      value={{ chats, filter, search, updateFilter, updateSearch }}
    >
      {children}
    </ChatsContext.Provider>
  );
}
