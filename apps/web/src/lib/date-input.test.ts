import { describe, expect, it } from "vitest";
import {
  isoToDisplay,
  isoToLocalDate,
  localDateToIso,
  maskDateTyping,
  parseDisplayDate,
} from "./date-input";

describe("isoToDisplay", () => {
  it("shows day first", () => {
    expect(isoToDisplay("2026-11-10")).toBe("10/11/2026");
  });
  it("gives an empty string for no date or a bad value", () => {
    expect(isoToDisplay("")).toBe("");
    expect(isoToDisplay("10/11/2026")).toBe("");
  });
});

describe("parseDisplayDate", () => {
  it("reads dd/mm/yyyy, never mm/dd", () => {
    expect(parseDisplayDate("10/11/2026")).toBe("2026-11-10");
    expect(parseDisplayDate("01/05/2026")).toBe("2026-05-01");
  });
  it("accepts single digits and other separators", () => {
    expect(parseDisplayDate("1/5/2026")).toBe("2026-05-01");
    expect(parseDisplayDate("1.5.2026")).toBe("2026-05-01");
    expect(parseDisplayDate("1-5-2026")).toBe("2026-05-01");
    expect(parseDisplayDate(" 1 5 2026 ")).toBe("2026-05-01");
  });
  it("accepts eight bare digits", () => {
    expect(parseDisplayDate("10112026")).toBe("2026-11-10");
  });
  it("rejects dates that do not exist", () => {
    expect(parseDisplayDate("31/02/2026")).toBeNull();
    expect(parseDisplayDate("29/02/2026")).toBeNull();
    expect(parseDisplayDate("29/02/2028")).toBe("2028-02-29");
    expect(parseDisplayDate("12/13/2026")).toBeNull();
    expect(parseDisplayDate("00/01/2026")).toBeNull();
  });
  it("rejects incomplete text and two-digit years", () => {
    expect(parseDisplayDate("")).toBeNull();
    expect(parseDisplayDate("10/11")).toBeNull();
    expect(parseDisplayDate("10/11/202")).toBeNull();
    expect(parseDisplayDate("10/11/26")).toBeNull();
  });
});

describe("maskDateTyping", () => {
  it("adds slashes to bare digits", () => {
    expect(maskDateTyping("1")).toBe("1");
    expect(maskDateTyping("10")).toBe("10");
    expect(maskDateTyping("101")).toBe("10/1");
    expect(maskDateTyping("1011")).toBe("10/11");
    expect(maskDateTyping("10112")).toBe("10/11/2");
    expect(maskDateTyping("101120261")).toBe("10/11/2026");
  });
  it("keeps masking after its own slash is in the text", () => {
    // Each keystroke arrives as the masked text plus one digit.
    expect(maskDateTyping("10/1")).toBe("10/1");
    expect(maskDateTyping("10/112")).toBe("10/11/2");
    expect(maskDateTyping("01/052026")).toBe("01/05/2026");
  });
  it("lets backspace remove the slash", () => {
    expect(maskDateTyping("10/11/")).toBe("10/11/");
    expect(maskDateTyping("10/11")).toBe("10/11");
    expect(maskDateTyping("10/")).toBe("10/");
  });
  it("takes the person's own separators and drops letters", () => {
    expect(maskDateTyping("1/5/2026")).toBe("1/5/2026");
    expect(maskDateTyping("1.5.2026")).toBe("1/5/2026");
    expect(maskDateTyping("1-5-2026x")).toBe("1/5/2026");
    expect(maskDateTyping("1//5")).toBe("1/5");
    expect(maskDateTyping("/1")).toBe("1");
    expect(maskDateTyping("ab")).toBe("");
  });
});

describe("local date round trip", () => {
  it("keeps the calendar day", () => {
    const d = isoToLocalDate("2026-11-10");
    expect(d?.getDate()).toBe(10);
    expect(d?.getMonth()).toBe(10);
    expect(localDateToIso(d as Date)).toBe("2026-11-10");
    expect(isoToLocalDate("")).toBeUndefined();
  });
});
