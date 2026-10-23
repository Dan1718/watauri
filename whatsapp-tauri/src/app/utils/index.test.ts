import { describe, expect, test } from "bun:test";
import { resolveContactDisplayName } from "./index";

describe("resolveContactDisplayName", () => {
  test("prefers saved names over push names and phone numbers", () => {
    expect(resolveContactDisplayName({ name: "Alice Smith", pushName: "Ali", phoneNumber: "123", jid: "123@s.whatsapp.net" }))
      .toBe("Alice Smith");
  });

  test("uses a usable push name before the phone number", () => {
    expect(resolveContactDisplayName({ name: "123", pushName: "Ali", phoneNumber: "456", jid: "456@s.whatsapp.net" }))
      .toBe("Ali");
  });

  test("uses the mapped phone number when names are missing or phone placeholders", () => {
    expect(resolveContactDisplayName({ name: "123", pushName: "+456", phoneNumber: "789", jid: "opaque@lid" }))
      .toBe("+789");
  });

  test("derives a phone number from a phone JID and never displays a JID fallback", () => {
    expect(resolveContactDisplayName({ jid: "919876543210@s.whatsapp.net" })).toBe("+919876543210");
    expect(resolveContactDisplayName({ jid: "opaque@lid" })).toBe("Unknown contact");
  });
});
