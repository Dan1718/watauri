import { createContext, PropsWithChildren, useEffect, useState } from "react";
import { BackendChat, BackendMessage, listBackendChats } from "../backend";

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
  return chat.participants.find((participant) => participant.id !== "me")?.id ?? chat.id;
}

function toMessage(message: BackendMessage, fallbackContactId: string): Message {
  return {
    contactId: message.senderId === "me" ? fallbackContactId : message.senderId,
    message: message.text,
    timestamp: message.timestamp,
    isSentFromUser: message.senderId === "me",
    sent: true,
    delivered: message.status === "delivered" || message.status === "read",
    read: message.status === "read",
  };
}

function toChat(chat: BackendChat): Chat {
  const directContactId = getDirectContactId(chat);
  const contactId = chat.isGroup
    ? chat.participants.map((participant) => participant.id)
    : directContactId;

  return {
    id: chat.id,
    contactId,
    groupName: chat.name,
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
  });

  const updateFilter = (filter: string) => {
    setFilter(filter as Filters);
  };

  const updateSearch = (query: string) => {
    setSearch(query);
  };

  useEffect(() => {
    const fetchChats = async () => {
      setChats((prev) => ({ ...prev, isLoading: true }));
      const data = (await listBackendChats()).map(toChat);
      setChats((prev) => ({
        ...prev,
        complete: data,
        filtered: data,
        isLoading: false,
      }));
    };

    fetchChats();
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
  }, [filter]);

  return (
    <ChatsContext.Provider
      value={{ chats, filter, search, updateFilter, updateSearch }}
    >
      {children}
    </ChatsContext.Provider>
  );
}
