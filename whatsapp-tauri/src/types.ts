export type UserStatus = "online" | "offline" | "typing..." | string;

export interface User {
  id: string;
  name: string;
  avatar: string;
  status: UserStatus;
}

export interface Message {
  id: string;
  senderId: string;
  text: string;
  timestamp: string;
  status: "sent" | "delivered" | "read";
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

export type AppScreen = "login" | "chats" | "communities" | "starred" | "archived";
