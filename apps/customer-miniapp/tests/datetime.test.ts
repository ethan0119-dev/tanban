import { describe, expect, it } from "vitest";
import { beijingDateKey, beijingEpoch, formatBeijingDate, formatBeijingDateTime } from "../miniprogram/utils/datetime";

describe("Beijing datetime contract", () => {
  it("formats wall clocks and legacy instants consistently", () => {
    expect(formatBeijingDateTime("2026-07-21 14:00:00")).toBe("2026-07-21 14:00:00");
    expect(formatBeijingDateTime("2026-07-21T06:00:00Z")).toBe("2026-07-21 14:00:00");
    expect(formatBeijingDateTime("2026-07-21T14:00:00+08:00")).toBe("2026-07-21 14:00:00");
    expect(formatBeijingDate("2026-07-21T16:30:00Z")).toBe("2026-07-22");
  });

  it("calculates daily keys with the Beijing date boundary", () => {
    expect(beijingDateKey(new Date("2026-07-21T16:30:00Z"))).toBe("2026-07-22");
    expect(beijingEpoch("2026-07-21 14:00:00")).toBe(Date.parse("2026-07-21T14:00:00+08:00"));
  });
});
