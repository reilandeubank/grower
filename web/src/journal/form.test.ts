import { describe, expect, it } from "vitest";
import { EMPTY_MESSAGE, newDraft, parseInches, readDraft } from "./form";

describe("parseInches", () => {
  it("reads decimals, blanks and nonsense", () => {
    expect(parseInches("12")).toBe(12);
    expect(parseInches(" 12.5 ")).toBe(12.5);
    expect(parseInches("12,5")).toBe(12.5);
    expect(parseInches(".5")).toBe(0.5);
    expect(parseInches("")).toBeNull();
    expect(parseInches("0")).toBeUndefined();
    expect(parseInches("601")).toBeUndefined();
    expect(parseInches("12in")).toBeUndefined();
    expect(parseInches("-3")).toBeUndefined();
  });
});

describe("readDraft", () => {
  it("needs at least one thing for a measurement", () => {
    expect(readDraft(newDraft(null, "2026-10-21"))).toEqual({ body: null, errors: { empty: EMPTY_MESSAGE } });
    expect(readDraft({ ...newDraft(null), note: "   " }).errors.empty).toBe(EMPTY_MESSAGE);
    expect(readDraft({ ...newDraft(null), note: "New leaf" }).body).toEqual({
      height_in: null,
      width_in: null,
      stage: null,
      note: "New leaf",
    });
    expect(readDraft(newDraft("young")).body?.stage).toBe("young");
  });

  it("lets an empty check-in through, and doesn't count a prefilled stage", () => {
    expect(readDraft(newDraft("young"), { prefilledStage: "young", allowEmpty: true })).toEqual({ body: null, errors: {} });
    const changed = readDraft({ ...newDraft("young"), stage: "mature" }, { prefilledStage: "young", allowEmpty: true });
    expect(changed.body).toMatchObject({ stage: "mature" });
    // With a size, the prefilled stage rides along.
    const sized = readDraft({ ...newDraft("young"), height: "12" }, { prefilledStage: "young", allowEmpty: true });
    expect(sized.body).toEqual({ height_in: 12, width_in: null, stage: "young", note: "" });
  });

  it("counts a photo as something", () => {
    const file = new File(["x"], "a.jpg");
    expect(readDraft({ ...newDraft(null), photo: { kind: "new", file, preview: undefined } }).body).not.toBeNull();
  });

  it("flags sizes and future dates", () => {
    const { body, errors } = readDraft(
      { ...newDraft(null, "2026-10-22"), height: "abc", width: "0" },
      { today: "2026-10-21" },
    );
    expect(body).toBeNull();
    expect(errors).toEqual({
      height: "Enter inches from 0.1 to 600, like 12.5.",
      width: "Enter inches from 0.1 to 600, like 12.5.",
      measuredOn: "Pick today or an earlier day.",
    });
    expect(readDraft({ ...newDraft(null, "2026-10-21"), height: "12" }, { today: "2026-10-21" }).body).toMatchObject({
      measured_on: "2026-10-21",
      height_in: 12,
    });
  });
});
