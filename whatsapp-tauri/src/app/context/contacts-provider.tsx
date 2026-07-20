import { createContext, PropsWithChildren, useEffect, useState } from "react";
import { BackendUser, listBackendContacts } from "../backend";
import { getDisplayNameFromJid } from "../utils";

export type Contact = {
  id: string;
  displayName: string;
  contactAvatar: string;
  statusMessage: string;
  typing?: boolean;
};

export type Contacts = {
  contacts: Contact[];
  dictionary: [string, Contact[]][];
  isLoading: boolean;
  error: string | null;
  filteredContacts: Contact[];
  search: string;
};

export type ContactsContextType = Contacts & {
  filterContacts: (search: string) => void;
  getContact: (id: string) => Contact | undefined;
  setIsContactTyping: (id: string, typing: boolean) => void;
};

export const ContactsContext = createContext<ContactsContextType | undefined>(
  undefined
);

function toContact(user: BackendUser): Contact {
  return {
    id: user.id,
    displayName: user.name || getDisplayNameFromJid(user.id),
    contactAvatar: user.avatar ?? "",
    statusMessage: user.status ?? "",
  };
}

export default function ContactsProvider({ children }: PropsWithChildren) {
  const [contacts, setContacts] = useState<Contacts>({
    contacts: [],
    dictionary: [["", []]],
    isLoading: false,
    error: null,
    filteredContacts: [],
    search: "",
  });

  const generateDictionary = (
    data: Contacts["contacts"]
  ): [string, Contact[]][] => {
    const map: Map<string, Contact[]> = new Map();
    [...data]
      .sort((a: Contact, b: Contact) =>
        a.displayName.localeCompare(b.displayName)
      )
      .forEach((contact: Contact) => {
        const firstLetter = contact.displayName.charAt(0).toUpperCase() || "#";
        if (map.has(firstLetter)) {
          const existing = map.get(firstLetter);
          existing?.push(contact);
          if (existing) {
            map.set(firstLetter, existing);
          }
        } else {
          map.set(firstLetter, [contact]);
        }
      });
    return Array.from(map);
  };

  useEffect(() => {
    const fetchContacts = async () => {
      setContacts((prev) => ({ ...prev, isLoading: true }));
      try {
        const data = (await listBackendContacts()).map(toContact);
        const dictionary = generateDictionary(data);

        setContacts((prev) => ({
          ...prev,
          contacts: data,
          filteredContacts: data,
          dictionary,
          isLoading: false,
          error: null,
        }));
      } catch (error) {
        setContacts((prev) => ({
          ...prev,
          isLoading: false,
          error: error instanceof Error ? error.message : "Failed to load contacts",
        }));
      }
    };

    void fetchContacts();
  }, []);

  useEffect(() => {
    setContacts((prev) => {
      const contacts = prev.contacts;
      const search = prev.search;
      const normalizedSearch = search.toLowerCase();
      const filteredContacts = contacts.filter((contact: Contact) =>
        contact.displayName.toLowerCase().includes(normalizedSearch)
      );
      const dictionary = generateDictionary(filteredContacts);

      return {
        ...prev,
        filteredContacts,
        dictionary,
      };
    });
  }, [contacts.search]);

  const filterContacts = (search: string) => {
    setContacts((prev) => ({
      ...prev,
      search,
    }));
  };

  const getContact = (id: string) => {
    const contact = contacts.contacts.find((contact: Contact) => contact.id === id);
    return contact;
  };

  const setIsContactTyping = (id: string, typing: boolean) => {
    const contactIndex = contacts.contacts.findIndex(
      (contact: Contact) => contact.id === id
    );
    if (contactIndex !== -1) {
      const updatedContacts = [...contacts.contacts];
      updatedContacts[contactIndex].typing = typing;
      setContacts((prev) => ({
        ...prev,
        contacts: [...updatedContacts],
      }));
    }
  };

  return (
    <ContactsContext.Provider
      value={{ ...contacts, filterContacts, getContact, setIsContactTyping }}
    >
      {children}
    </ContactsContext.Provider>
  );
}
