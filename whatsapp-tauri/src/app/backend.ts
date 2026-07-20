const API_BASE = "http://localhost:8090";

export type BackendUser = {
  id: string;
  name?: string;
  avatar?: string;
  status?: string;
};

export type BackendMessage = {
  id: string;
  senderId: string;
  text: string;
  timestamp: string;
  status: "received" | "sent" | "delivered" | "read";
  mediaType?: string;
  isFromMe?: boolean;
};

export type BackendChat = {
  id: string;
  participants?: BackendUser[] | null;
  lastMessage?: BackendMessage;
  unreadCount: number;
  isGroup: boolean;
  name?: string;
  avatar?: string;
  isArchived: boolean;
  isStarred?: boolean;
  isCommunity?: boolean;
};

async function request<T>(path: string, init?: RequestInit): Promise<T> {
  const response = await fetch(`${API_BASE}${path}`, init);
  if (!response.ok) throw new Error(`${path} failed: ${response.status}`);
  return response.json();
}

export const listBackendChats = () => request<BackendChat[]>("/api/chats");

export const listBackendMessages = (chatId: string) =>
  request<BackendMessage[]>(`/api/chats/${chatId}`);

export const listBackendContacts = () => request<BackendUser[]>("/api/contacts");

export const sendBackendMessage = (chatId: string, text: string) =>
  request<BackendMessage>(`/api/chats/${chatId}/send`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ text }),
  });
