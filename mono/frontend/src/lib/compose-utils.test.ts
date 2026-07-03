import { describe, expect, it } from "vitest";
import {
  appendQuotedForwardHtml,
  buildTextCitation,
  formatRecipientAddress,
  parseCcAddresses,
  resolveComposeAccountId,
  resolveFromIdentityHeader,
  stripHtml,
} from "./compose-utils";
import type { Account, Email } from "@/hooks/useEmailTypes";

describe("compose-utils", () => {
  it("stripHtml converts breaks and removes tags", () => {
    expect(stripHtml("<p>Hi<br/>there</p>")).toBe("Hi\nthere\n");
  });

  it("formatRecipientAddress builds name and address", () => {
    const email = {
      sender_name: "Alice",
      sender_address: "alice@example.com",
    } as Email;
    expect(formatRecipientAddress(email)).toEqual([
      "Alice <alice@example.com>",
    ]);
  });

  it("parseCcAddresses splits comma-separated values", () => {
    expect(parseCcAddresses("a@x.com, b@x.com")).toEqual([
      "a@x.com",
      "b@x.com",
    ]);
  });

  it("resolveComposeAccountId prefers account identity prefix", () => {
    const accounts = [{ id: "acc-1", email: "a@x.com" }] as Account[];
    expect(
      resolveComposeAccountId({
        activeAccount: "unified",
        accounts,
        identity: "account:acc-2",
        isReplying: false,
        isForwarding: false,
      }),
    ).toBe("acc-2");
  });

  it("resolveFromIdentityHeader builds From header from account", () => {
    const accounts = [
      { id: "acc-1", email: "a@x.com", name: "Alice" },
    ] as Account[];
    expect(
      resolveFromIdentityHeader("account:acc-1", accounts),
    ).toEqual({
      accountId: "acc-1",
      fromHeader: "Alice <a@x.com>",
    });
  });

  it("buildTextCitation prefixes lines with >", () => {
    expect(buildTextCitation("one\ntwo")).toBe("> one<br>> two");
  });

  it("appendQuotedForwardHtml strips nested body tags", () => {
    const result = appendQuotedForwardHtml(
      "<p>Reply</p>",
      "<html><body><p>Original</p></body></html>",
      {
        from: "Alice",
        subject: "Hello",
        date: "Today",
        to: "Bob",
      },
      {
        begin: "Begin forwarded message:",
        from: "From:",
        subject: "Subject:",
        date: "Date:",
        to: "To:",
      },
    );
    expect(result).toContain("<p>Original</p>");
    expect(result).not.toContain("<body>");
  });
});
