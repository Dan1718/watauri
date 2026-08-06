import { describe, expect, test } from "bun:test";
import { getMessageHref, splitMessageText } from "./chat-message";

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
