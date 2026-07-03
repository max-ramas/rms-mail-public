import { describe, expect, it } from "vitest";
import { formatAddresses } from "./email-address-utils";

describe("formatAddresses", () => {
  it("returns empty string for empty input", () => {
    expect(formatAddresses("")).toBe("");
  });

  it("extracts display names from angle-bracket addresses", () => {
    expect(formatAddresses("Alice <alice@example.com>, Bob <bob@example.com>")).toBe(
      "Alice, Bob",
    );
  });

  it("keeps plain email addresses as-is", () => {
    expect(formatAddresses("plain@example.com")).toBe("plain@example.com");
  });
});
