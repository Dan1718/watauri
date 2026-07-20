import { createContext, PropsWithChildren, useEffect, useState } from "react";
import { Chat, Message } from "./chats-provider";
import { useChats } from "../hooks/use-chats";
import { useContacts } from "../hooks/use-contacts";
import { Contact } from "./contacts-provider";
import { getDisplayNameFromJid } from "../utils";
import { BackendMessage, listBackendMessages, sendBackendMessage } from "../backend";

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
  isLoading: boolean;
  error: string | null;
  isSending: boolean;
};

export type CurrentChat = CurrentChatData & {
  loadCurrentChat: (chat: Partial<CurrentChatData>) => void;
  sendMessage: (text: string) => Promise<boolean>;
};

export const CurrentChatContext = createContext<undefined | CurrentChat>(
  undefined
);

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

export default function CurrentChatProvider({ children }: PropsWithChildren) {
  const [currentChat, setCurrentChat] = useState<CurrentChatData>({
    chatId: null,
    contact: null,
    messages: [],
    group: null,
    page: 0,
    isLoading: false,
    error: null,
    isSending: false,
  });
  const {
    chats: { complete },
  } = useChats();
  const { contacts } = useContacts();

  useEffect(() => {
    let active = true;

    const fetchMessages = async () => {
      const chatId = currentChat.chatId;
      if (!chatId) return;
      setCurrentChat((prev) => ({ ...prev, isLoading: prev.messages.length === 0 }));
      try {
        const data = await listBackendMessages(chatId);
        if (!active) return;
        const chat = complete.find((chat: Chat) => chat.id === chatId);
        const fallbackContactId = typeof chat?.contactId === "string" ? chat.contactId : chatId;
        setCurrentChat((prev) =>
          prev.chatId === chatId
            ? {
                ...prev,
                messages: data.map((message) => toMessage(message, fallbackContactId)),
                isLoading: false,
                error: null,
              }
            : prev
        );
      } catch (error) {
        if (!active) return;
        setCurrentChat((prev) =>
          prev.chatId === chatId
            ? {
                ...prev,
                isLoading: false,
                error: error instanceof Error ? error.message : "Failed to load messages",
              }
            : prev
        );
      }
    };

    void fetchMessages();
    const interval = setInterval(() => void fetchMessages(), 3000);

    return () => {
      active = false;
      clearInterval(interval);
    };
  }, [complete, currentChat.chatId, currentChat.page]);

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
        setCurrentChat((prev) => ({
          ...prev,
          contact,
          group: null,
        }));
      } else {
        const groupContacts: CurrentChatContacts = {};
        chat.contactId.forEach((groupContact: string) => {
          groupContacts[groupContact] = contacts.find(
            (contact: Contact) => contact.id === groupContact
          );
        });
        setCurrentChat((prev) => ({
          ...prev,
          contact: null,
          group: {
            name: chat.groupName ?? getDisplayNameFromJid(chat.id),
            avatar: chat.groupAvatar ?? "",
            contacts: groupContacts,
          },
        }));
      }
    }
  }, [complete, contacts, currentChat.chatId]);

  const loadCurrentChat = (chat: Partial<CurrentChatData>) => {
    setCurrentChat((prev) => {
      const isNewChat = chat.chatId !== undefined && chat.chatId !== prev.chatId;
      return {
        ...prev,
        ...(isNewChat ? { contact: null, group: null, messages: [] } : {}),
        ...chat,
      };
    });
  };

  const sendMessage = async (text: string) => {
    const trimmedText = text.trim();
    if (!currentChat.chatId || !trimmedText) return false;

    setCurrentChat((prev) => ({ ...prev, isSending: true, error: null }));
    try {
      const sentMessage = await sendBackendMessage(currentChat.chatId, trimmedText);
      const chat = complete.find((chat: Chat) => chat.id === currentChat.chatId);
      const fallbackContactId = typeof chat?.contactId === "string" ? chat.contactId : currentChat.chatId;
      setCurrentChat((prev) =>
        prev.chatId === currentChat.chatId
          ? {
              ...prev,
              messages: [...prev.messages, toMessage(sentMessage, fallbackContactId)],
              isSending: false,
              error: null,
            }
          : prev
      );
      return true;
    } catch (error) {
      setCurrentChat((prev) => ({
        ...prev,
        isSending: false,
        error: error instanceof Error ? error.message : "Failed to send message",
      }));
      return false;
    }
  };

  return (
    <CurrentChatContext.Provider
      value={{ ...currentChat, loadCurrentChat, sendMessage }}
    >
      {children}
    </CurrentChatContext.Provider>
  );
}
