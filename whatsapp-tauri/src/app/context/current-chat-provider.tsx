import { createContext, PropsWithChildren, useEffect, useState } from "react";
import { Chat, Message } from "./chats-provider";
import { useChats } from "../hooks/use-chats";
import { useContacts } from "../hooks/use-contacts";
import { Contact } from "./contacts-provider";
import { getTimestamp } from "../utils";
import { BackendMessage, listBackendMessages } from "../backend";

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
};

export type CurrentChat = CurrentChatData & {
  loadCurrentChat: (chat: Partial<CurrentChatData>) => void;
  addNewMessage: () => void;
};

export const CurrentChatContext = createContext<undefined | CurrentChat>(
  undefined
);

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

export default function CurrentChatProvider({ children }: PropsWithChildren) {
  const [currentChat, setCurrentChat] = useState<CurrentChatData>({
    chatId: null,
    contact: null,
    messages: [],
    group: null,
    page: 0,
    isLoading: false,
  });
  const {
    chats: { complete },
  } = useChats();
  const { contacts, setIsContactTyping } = useContacts();

  useEffect(() => {
    const fetchMessages = async () => {
      const chatId = currentChat.chatId;
      if (!chatId) return;
      setCurrentChat((prev) => ({ ...prev, isLoading: true }));
      const data = await listBackendMessages(chatId);
      const chat = complete.find((chat: Chat) => chat.id === chatId);
      const fallbackContactId = typeof chat?.contactId === "string" ? chat.contactId : "me";
      setCurrentChat((prev) =>
        prev.chatId === chatId
          ? {
              ...prev,
              messages: data.map((message) => toMessage(message, fallbackContactId)),
              isLoading: false,
            }
          : prev
      );
    };

    fetchMessages();
  }, [complete, currentChat.chatId, currentChat.page]);

  useEffect(() => {
    const chat = complete.find((chat: Chat) => chat.id === currentChat.chatId);
    if (chat) {
      if (typeof chat.contactId == "string") {
        const contactId = chat.contactId;
        const contact = contacts.find(
          (contact: Contact) => contact.id === contactId
        );
        if (contact) {
          setCurrentChat((prev) => ({
            ...prev,
            contact,
            group: null,
          }));
        }
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
            name: chat.groupName ?? "",
            avatar: chat.groupAvatar ?? "",
            contacts: groupContacts,
          },
        }));
      }
    }
  }, [complete, contacts, currentChat.chatId]);

  const loadCurrentChat = (chat: Partial<CurrentChat>) => {
    setCurrentChat((prev) => {
      const isNewChat = chat.chatId !== undefined && chat.chatId !== prev.chatId;
      return {
        ...prev,
        ...(isNewChat ? { contact: null, group: null, messages: [] } : {}),
        ...chat,
      };
    });
  };

  const addNewMessage = () => {
    if (currentChat.contact) {
      const contactId = currentChat.contact?.id;

      setIsContactTyping(contactId, true);
      setCurrentChat((prev) => ({
        ...prev,
        messages: [
          ...prev.messages,
          {
            contactId,
            message: "Hey!",
            timestamp: getTimestamp(),
            isSentFromUser: false,
          },
        ],
      }));
      setIsContactTyping(contactId, false);
    }
  };

  return (
    <CurrentChatContext.Provider
      value={{ ...currentChat, loadCurrentChat, addNewMessage }}
    >
      {children}
    </CurrentChatContext.Provider>
  );
}
