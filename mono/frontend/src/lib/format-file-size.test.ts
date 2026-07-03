import { describe, expect, it } from "vitest";
import { formatFileSize } from "./format-file-size";

describe("formatFileSize", () => {
  it("formats zero bytes", () => {
    expect(formatFileSize(0)).toBe("0 B");
  });

  it("formats kilobytes with one decimal", () => {
    expect(formatFileSize(1536)).toBe("1.5 KB");
  });

  it("formats bytes without decimals", () => {
    expect(formatFileSize(512)).toBe("512 B");
  });
});
