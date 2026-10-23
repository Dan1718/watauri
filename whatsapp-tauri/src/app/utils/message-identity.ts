import type { Contact } from "../context/contacts-provider";
import { isPhonePlaceholder, normalizeJid, resolveContactDisplayName } from ".";

type Identity = {
  id: string;
  name?: string;
  pushName?: string;
  displayName?: string;
  phoneNumber?: string;
  phone?: string;
  phoneJid?: string;
  lidJid?: string;
  isSaved?: boolean;
};

const usable = (name?: string) => name && name !== "Unknown contact" && !isPhonePlaceholder(name) ? name : undefined;

export function resolveMessageIdentity(
  identifier: string,
  participants: Identity[],
  getContact: (jid: string) => Contact | undefined,
  requireKnownIdentity = false,
) {
  const jids = identifier.includes("@")
    ? [normalizeJid(identifier)]
    : [`${identifier.replace(/^\+/, "")}@lid`, `${identifier.replace(/^\+/, "")}@s.whatsapp.net`];
  const participant = participants.find((candidate) =>
    [candidate.id, candidate.phoneJid, candidate.lidJid]
      .some((alias) => alias && jids.includes(normalizeJid(alias)))
  );
  const aliases = participant
    ? [participant.phoneNumber, participant.phone, participant.phoneJid, participant.lidJid, participant.id]
    : jids;
  const contacts = aliases
    .filter((alias): alias is string => Boolean(alias))
    .map(getContact)
    .filter((contact): contact is Contact => Boolean(contact));
  if (requireKnownIdentity && !participant && !contacts.length) return undefined;
  if (!identifier.includes("@") && !participant && new Set(contacts.map((contact) => contact.phoneJid ?? contact.id)).size > 1) return undefined;
  const saved = contacts.find((contact) => usable(contact.name));
  const named = contacts.find((contact) => usable(contact.pushName));
  const contact = saved ?? named ?? contacts[0];
  const jid = identifier.includes("@") ? identifier : participant?.id ?? contact?.phoneJid ?? contact?.id;
  if (!jid) return undefined;
  const name = resolveContactDisplayName({
    name: saved?.name ?? (participant?.isSaved ? usable(participant.name) : undefined),
    pushName: named?.pushName ?? usable(participant?.pushName) ?? usable(participant?.displayName),
    phoneNumber: contact?.phone ?? participant?.phoneNumber ?? participant?.phone,
    jid: contact?.phoneJid ?? participant?.phoneJid ?? jid,
  });
  return { jid, name: name === "Unknown contact" ? undefined : name };
}
