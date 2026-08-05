import { describe, expect, test } from "bun:test";
import { splitMessageText } from "./chat-message";

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
});
