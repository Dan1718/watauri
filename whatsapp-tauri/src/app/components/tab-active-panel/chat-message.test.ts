import { describe, expect, test } from "bun:test";
import { getMessageHref, splitMessageContent, splitMessageText } from "./chat-message";

describe("splitMessageText", () => {
  test("separates web URLs from message text without trailing punctuation", () => {
    expect(splitMessageText("See https://example.com/docs, then http://example.org."))
      .toEqual([
        "See ",
        "https://example.com/docs",
        ", then ",
        "http://example.org",
        ".",
      ]);
  });

  test("recognizes scheme-less domains with supported TLDs", () => {
    expect(splitMessageText("Open courses.iiit.ac.in, not courses.iiit.ac.invalid."))
      .toEqual([
        "Open ",
        "courses.iiit.ac.in",
        ", not ",
        "courses.iiit.ac.invalid",
        ".",
      ]);
    expect(getMessageHref("courses.iiit.ac.in")).toBe("https://courses.iiit.ac.in");
    expect(getMessageHref("courses.iiit.ac.invalid")).toBeNull();
  });

  test("does not detect domains inside email addresses", () => {
    expect(splitMessageText("Email user@example.com")).toEqual(["Email user@example.com"]);
  });
});

describe("splitMessageContent", () => {
  test("resolves known legacy IDs without metadata, preserving punctuation and unknown IDs", () => {
    const resolve = (id: string) => id === "900001" ? { jid: "900001@lid", name: "Hansika" } : undefined;
    const parts = splitMessageContent("Thanks @900001. @900001 and @999999; email a@900001", [], resolve);
    expect(parts.filter((part) => part.type === "mention").map((part) => part.target.name)).toEqual(["Hansika", "Hansika"]);
    expect(parts.map((part) => part.type === "mention" ? `@${part.target.name}` : part.text).join(""))
      .toBe("Thanks @Hansika. @Hansika and @999999; email a@900001");
  });

  test("resolves mention labels in JID order while preserving links and text", () => {
    expect(splitMessageContent("Hi @+91......80, then @919876543210 https://example.com", [
      { jid: "first@lid", name: "Alice" },
      { jid: "919876543210@s.whatsapp.net", name: "+919876543210" },
    ])).toEqual([
      { type: "text", text: "Hi " },
      { type: "mention", target: { jid: "first@lid", name: "Alice" }, fallbackText: "@+91......80" },
      { type: "text", text: ", then " },
      { type: "mention", target: { jid: "919876543210@s.whatsapp.net", name: "+919876543210" }, fallbackText: "@919876543210" },
      { type: "text", text: " " },
      { type: "text", text: "https://example.com" },
    ]);
  });

  test("keeps unmatched mentions as plain text and preserves unresolved mentioned tokens", () => {
    expect(splitMessageContent("@+91......80 and @opaque", [
      { jid: "unknown@lid" },
    ])).toEqual([
      { type: "mention", target: { jid: "unknown@lid" }, fallbackText: "@+91......80" },
      { type: "text", text: " and @opaque" },
    ]);
  });
});
