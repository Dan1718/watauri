const API_BASE = "http://localhost:8090";

export type BackendUser = {
  id: string;
  name?: string;
  avatar?: string;
  status?: string;
};

export type BackendProfile = {
  id: string;
  pushName: string;
};

export type BackendMessage = {
  id: string;
  senderId: string;
  text: string;
  timestamp: string;
  status: "pending" | "received" | "sent" | "delivered" | "read";
  mediaType?: string;
  isFromMe?: boolean;
};

export type MessagePage = {
  messages: BackendMessage[];
  nextCursor: string | null;
  latestCursor: string | null;
  hasMore: boolean;
};

export type MessagePageOptions = {
  before?: string;
  after?: string;
  limit?: number;
  signal?: AbortSignal;
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

export const listBackendChats = (signal?: AbortSignal) =>
  request<BackendChat[]>("/api/chats", { signal });

export const getBackendProfile = (signal?: AbortSignal) =>
  request<BackendProfile>("/api/profile", { signal });

export const listBackendMessages = (
  chatId: string,
  { before, after, limit, signal }: MessagePageOptions = {}
) => {
  const query = new URLSearchParams();
  if (before) query.set("before", before);
  if (after) query.set("after", after);
  if (limit !== undefined) query.set("limit", String(limit));
  const suffix = query.size ? `?${query}` : "";
  return request<MessagePage>(`/api/chats/${chatId}${suffix}`, { signal });
};

export const listBackendContacts = () => request<BackendUser[]>("/api/contacts");

export const sendBackendMessage = (chatId: string, text: string) =>
  request<BackendMessage>(`/api/chats/${chatId}/send`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ text }),
  });
