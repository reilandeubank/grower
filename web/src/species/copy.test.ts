import { describe, expect, it } from "vitest";
import { applyToast, careGuideLines, hasCareDefaults, lookupNotes, sourceLine } from "./copy";
import { monstera, waterTask } from "./fixtures";

const none = { water_interval_days: null, light: null, water_at: null };

describe("care guide copy", () => {
  it("says only what the species knows", () => {
    expect(careGuideLines(monstera).map((l) => l.text)).toEqual([
      "Water about every 7 days",
      "Bright, indirect light",
      "Water at 3 · Dry",
    ]);
    expect(careGuideLines(none)).toEqual([]);
    expect(careGuideLines({ ...none, water_interval_days: 1 }).map((l) => l.text)).toEqual(["Water about every day"]);
    expect(careGuideLines({ ...none, light: "direct", water_at: 5 }).map((l) => l.text)).toEqual([
      "Direct sun",
      "Water at 5 · Moist",
    ]);
  });

  it("names where the species came from", () => {
    expect(sourceLine(monstera)).toBe("From Perenual");
    expect(sourceLine({ source: "openplantbook", user_edited: false })).toBe("From Open Plantbook");
    expect(sourceLine({ source: "perenual", user_edited: true })).toBe("Edited by you");
    expect(sourceLine({ source: "manual", user_edited: true })).toBe("Added by you");
  });

  it("only offers to apply watering defaults", () => {
    expect(hasCareDefaults(monstera)).toBe(true);
    expect(hasCareDefaults({ ...none, water_at: 2 })).toBe(true);
    expect(hasCareDefaults(none)).toBe(false);
  });
});

describe("applyToast", () => {
  it("says a task was added when the plant had no water task", () => {
    expect(applyToast([], [waterTask])).toBe("Water task added.");
  });

  it("names the new interval or threshold of an existing task", () => {
    expect(applyToast([waterTask], [{ ...waterTask, interval_days: 5 }])).toBe("Watering set to every 5 days.");
    expect(applyToast([waterTask], [{ ...waterTask, interval_days: 1 }])).toBe("Watering set to every day.");
    expect(applyToast([waterTask], [{ ...waterTask, moisture_check: true, water_at: 2 }])).toBe(
      "Soil check set to water at 2 · Dry.",
    );
  });
});

describe("lookupNotes", () => {
  it("explains a missing setup, a quiet provider and no matches", () => {
    expect(lookupNotes({ providers: [], items: [] })).toEqual([
      "Online lookup isn't set up. Ask whoever runs grower to add API keys.",
    ]);
    expect(
      lookupNotes({
        providers: [
          { source: "openplantbook", ok: true },
          { source: "perenual", ok: false, detail: "Daily limit reached." },
        ],
        items: [],
      }),
    ).toEqual(["No matches online.", "Perenual didn't answer. Daily limit reached."]);
    expect(lookupNotes({ providers: [{ source: "perenual", ok: false }], items: [] })).toEqual([
      "Perenual didn't answer.",
    ]);
  });
});
