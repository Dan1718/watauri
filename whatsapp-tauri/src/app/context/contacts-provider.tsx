import {
  createContext,
  PropsWithChildren,
  useCallback,
  useEffect,
  useMemo,
  useState,
} from "react";
import { BackendUser, listBackendContacts } from "../backend";
import { getDisplayNameFromJid, isPhonePlaceholder } from "../utils";

export type Contact = {
  id: string;
  displayName: string;
  contactAvatar: string;
  statusMessage: string;
  phone?: string;
  isSaved?: boolean;
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
  const phone = user.phone || (user.id.endsWith("@s.whatsapp.net") ? getDisplayNameFromJid(user.id) : undefined);
  return {
    id: user.id,
    displayName: user.name || (!isPhonePlaceholder(user.pushName) ? user.pushName : undefined) || getDisplayNameFromJid(user.id),
    contactAvatar: user.avatar ?? "",
    statusMessage: user.status ?? "",
    phone,
    isSaved: user.isSaved ?? Boolean(user.name),
  };
}

function generateDictionary(data: Contact[]): [string, Contact[]][] {
  const map = new Map<string, Contact[]>();
  [...data]
    .sort((a, b) => a.displayName.localeCompare(b.displayName))
    .forEach((contact) => {
      const firstLetter = contact.displayName.charAt(0).toUpperCase() || "#";
      const existing = map.get(firstLetter);
      if (existing) {
        existing.push(contact);
      } else {
        map.set(firstLetter, [contact]);
      }
    });
  return Array.from(map);
}

export default function ContactsProvider({ children }: PropsWithChildren) {
  const [contacts, setContacts] = useState<
    Omit<Contacts, "dictionary" | "filteredContacts">
  >({
    contacts: [],
    isLoading: false,
    error: null,
    search: "",
  });

  useEffect(() => {
    const fetchContacts = async () => {
      setContacts((prev) => ({ ...prev, isLoading: true }));
      try {
        const data = (await listBackendContacts()).map(toContact);

        setContacts((prev) => ({
          ...prev,
          contacts: data,
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

  const contactMap = useMemo(
    () => new Map(contacts.contacts.flatMap((contact) => [
      [contact.id, contact] as const,
      ...(contact.phone ? [[contact.phone, contact] as const] : []),
    ])),
    [contacts.contacts]
  );
  const filteredContacts = useMemo(() => {
    const normalizedSearch = contacts.search.toLowerCase();
    return contacts.contacts.filter((contact) =>
      contact.displayName.toLowerCase().includes(normalizedSearch)
    );
  }, [contacts.contacts, contacts.search]);
  const dictionary = useMemo(
    () => generateDictionary(filteredContacts),
    [filteredContacts]
  );

  const filterContacts = useCallback((search: string) => {
    setContacts((prev) => ({
      ...prev,
      search,
    }));
  }, []);

  const getContact = useCallback((id: string) => contactMap.get(id), [contactMap]);

  const setIsContactTyping = useCallback((id: string, typing: boolean) => {
    setContacts((prev) => {
      const contactIndex = prev.contacts.findIndex((contact) => contact.id === id);
      if (contactIndex === -1) return prev;
      return {
        ...prev,
        contacts: prev.contacts.map((contact, index) =>
          index === contactIndex ? { ...contact, typing } : contact
        ),
      };
    });
  }, []);

  const value = useMemo(
    () => ({
      ...contacts,
      dictionary,
      filteredContacts,
      filterContacts,
      getContact,
      setIsContactTyping,
    }),
    [contacts, dictionary, filteredContacts, filterContacts, getContact, setIsContactTyping]
  );

  return (
    <ContactsContext.Provider value={value}>
      {children}
    </ContactsContext.Provider>
  );
}
