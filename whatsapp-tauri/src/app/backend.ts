const API_BASE = "http://localhost:8090";

export type BackendUser = {
  id: string;
  name: string;
  avatar: string;
  status: string;
};

export type BackendMessage = {
  id: string;
  senderId: string;
  text: string;
  timestamp: string;
  status: "sent" | "delivered" | "read";
};

export type BackendChat = {
  id: string;
  participants: BackendUser[];
  lastMessage?: BackendMessage;
  unreadCount: number;
  isGroup: boolean;
  name?: string;
  avatar?: string;
  isArchived: boolean;
  isStarred?: boolean;
  isCommunity?: boolean;
};

async function request<T>(path: string): Promise<T> {
  const response = await fetch(`${API_BASE}${path}`);
  if (!response.ok) throw new Error(`${path} failed: ${response.status}`);
  return response.json();
}

export const listBackendChats = () => request<BackendChat[]>("/api/chats");

export const listBackendMessages = (chatId: string) =>
  request<BackendMessage[]>(`/api/chats/${chatId}`);
