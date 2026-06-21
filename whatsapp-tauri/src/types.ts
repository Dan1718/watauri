export type UserStatus = "online" | "offline" | "typing..." | string;

export interface User {
  id: string;
  name: string;
  avatar: string;
  status: UserStatus;
}

export interface Message {
  id: string;
  chatJid?: string;
  senderId: string;
  text: string;
  timestamp: string;
  status: "sent" | "delivered" | "read";
  mediaType?: string;
  isFromMe?: boolean;
}

export interface Chat {
  id: string;
  participants: User[];
  lastMessage?: Message;
  unreadCount: number;
  isGroup: boolean;
  name?: string;
  avatar?: string;
  isArchived: boolean;
  isStarred?: boolean;
  isCommunity?: boolean;
}

export interface Contact {
  id: string;
  name: string;
  avatar?: string;
}

export type AppScreen = "login" | "chats" | "communities" | "starred" | "archived" | "settings";

function stripJid(jid: string): string {
  return jid.split("@")[0] || jid;
}

export function chatName(chat: Chat): string {
  if (chat.name) return chat.name;
  const p = (chat.participants || []).find((u) => u.id !== "me");
  if (p?.name) return p.name;
  return stripJid(chat.id);
}
