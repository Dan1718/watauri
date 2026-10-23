import type { Contact } from "../context/contacts-provider";
import { isPhonePlaceholder, normalizeJid, resolveContactDisplayName } from ".";

const usableName = (name?: string) => Boolean(name && name !== "Unknown contact" && !isPhonePlaceholder(name));

export function contactAliasKey(value: string) {
  const normalized = normalizeJid(value).replace(/^\+/, "");
  return /^\d+$/.test(normalized) ? `${normalized}@s.whatsapp.net` : normalized;
}

// WhatsApp can return separate phone and LID records for the same person.
// Join their aliases before choosing a name, rather than letting the last row win.
export function createContactLookup(contacts: Contact[]) {
  const parents = contacts.map((_, index) => index);
  const root = (index: number): number => {
    if (parents[index] !== index) parents[index] = root(parents[index]);
    return parents[index];
  };
  const aliases = new Map<string, number>();
  contacts.forEach((contact, index) => {
    for (const alias of [contact.id, contact.phoneJid, contact.lidJid, contact.phone]) {
      if (!alias) continue;
      const key = contactAliasKey(alias);
      const previous = aliases.get(key);
      if (previous !== undefined) parents[root(index)] = root(previous);
      aliases.set(key, index);
    }
  });
  const groups = new Map<number, Contact[]>();
  contacts.forEach((contact, index) => {
    const key = root(index);
    groups.set(key, [...(groups.get(key) ?? []), contact]);
  });
  const identities = new Map<number, Contact>();
  for (const [key, records] of groups) {
    const saved = records.find((contact) => usableName(contact.name));
    const named = records.find((contact) => usableName(contact.pushName));
    const preferred = saved ?? named ?? records[0];
    const phone = records.find((contact) => contact.phone)?.phone;
    const phoneJid = records.find((contact) => contact.phoneJid)?.phoneJid;
    const identity = {
      ...preferred,
      name: saved?.name,
      pushName: named?.pushName,
      phone,
      phoneJid,
      lidJid: records.find((contact) => contact.lidJid)?.lidJid,
      contactAvatar: preferred.contactAvatar || records.find((contact) => contact.contactAvatar)?.contactAvatar || "",
      isSaved: Boolean(saved),
      displayName: resolveContactDisplayName({ name: saved?.name, pushName: named?.pushName, phoneNumber: phone, jid: phoneJid ?? preferred.id }),
    };
    identities.set(key, identity);
  }
  return new Map([...aliases].map(([alias, index]) => [alias, identities.get(root(index))!]));
}
