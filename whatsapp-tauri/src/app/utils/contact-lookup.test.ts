import { describe, expect, test } from "bun:test";
import type { Contact } from "../context/contacts-provider";
import { contactAliasKey, createContactLookup } from "./contact-lookup";
import { resolveMessageIdentity } from "./message-identity";

function contact(fields: Partial<Contact> & Pick<Contact, "id">): Contact {
  return { displayName: "Unknown contact", contactAvatar: "", statusMessage: "", ...fields };
}

describe("contact identity lookup", () => {
  test("a duplicate LID placeholder cannot hide a saved phone contact", () => {
    const lookup = createContactLookup([
      contact({ id: "15550001@s.whatsapp.net", name: "Saved Hansika", phone: "15550001", lidJid: "900001@lid" }),
      contact({ id: "900001@lid", pushName: "+1••••01", phone: "15550001", phoneJid: "15550001@s.whatsapp.net" }),
    ]);
    const getContact = (id: string) => lookup.get(contactAliasKey(id));
    for (const alias of ["15550001@s.whatsapp.net", "900001:27@lid", "+15550001"]) {
      expect(getContact(alias)?.displayName).toBe("Saved Hansika");
    }
    expect(resolveMessageIdentity("900001:27@lid", [], getContact)?.name).toBe("Saved Hansika");
  });

  test("keeps a usable push name when another alias has a masked placeholder", () => {
    const lookup = createContactLookup([
      contact({ id: "15550002@s.whatsapp.net", pushName: "Akshat", phone: "15550002", lidJid: "900002@lid" }),
      contact({ id: "900002@lid", pushName: "+1••••02", phone: "15550002", phoneJid: "15550002@s.whatsapp.net" }),
    ]);
    const getContact = (id: string) => lookup.get(contactAliasKey(id));
    expect(resolveMessageIdentity("900002:7@lid", [], getContact)?.name).toBe("Akshat");
    expect(resolveMessageIdentity("900002", [], getContact, true)).toEqual({ jid: "15550002@s.whatsapp.net", name: "Akshat" });
    expect(resolveMessageIdentity("999999", [], getContact, true)).toBeUndefined();
  });
});
