grower is a self-hosted plant care PWA used mostly one-handed on an Android phone. Its main screen answers one question: what needs care today? Every rule below serves that loop: open the app, see what's due, tap done.

## Content fundamentals

- Write like a calm, knowledgeable friend. Use short sentences and plain verbs: "Water", "Done", "Snooze 3 days".
- Address the user as "you", and use sentence case everywhere except `label`-style location tags, which are uppercase.
- Name actions by what happens. A button reads "Mark watered" and the toast then says "Watered Monstera. Undo".
- Dates are relative when close and absolute when far: "Today", "Tomorrow", "In 3 days", "Overdue 2 days", then "Oct 28".
- Intervals read as "Every 7 days", never "7d".
- Empty states say what goes there and how to add the first one: "No plants yet. Add your first plant to start a care schedule."
- Never guilt the user about missed care. Write "Overdue 4 days", not "You forgot!".
- No emoji in UI copy.

## Colour

- Paint screens on `surface`. Put task rows, cards, sheets and the tab bar on `surface-raised`, and text fields on `surface-sunken`.
- Set text in `ink`, and secondary text (species, dates, intervals) in `ink-muted`.
- `brand` is the one action colour: primary buttons, the done check, the active tab and links. Text on a brand fill uses `on-brand`, never literal white.
- Each care type has its own accent: `water`, `fertilize`, `mist`, `repot` and `custom` (custom also covers rotate). Use them only for the task-type icon and its small accent. Never use them as a row or card background.
- `overdue` is reserved for overdue state. It always comes with the clock icon and the word "Overdue" or a day count, because `brand` and `overdue` have similar lightness and would otherwise be told apart by hue alone. Overdue chips use `overdue` text on `overdue-soft`.
- Show selected chips and the moment a task is completed on `brand-soft`.
- `line` is decorative only. Control borders use `line-strong`.
- Focus is a 2px solid `focus` ring at a 2px offset on every interactive element, in both themes.
- Both themes are first-class. The app follows the system setting and offers a manual override in Settings.

## Typography

- Two families: Bricolage Grotesque (`display`) for character, and Atkinson Hyperlegible Next (`sans`) for everything you read at a glance on a phone.
- Use `display` once per screen, for the date on Today. Use `title` for screen titles and the plant name on detail.
- Use `heading` for section headers (Overdue, Due today, Coming up), `body` for prose and form values, and `body-strong` for plant names in rows and cards.
- Use `caption` for dates and intervals, with `font-variant-numeric: tabular-nums`. Use `label` for uppercase location tags.
- The app self-hosts both fonts (it runs with no external services). This reference loads them from Google Fonts.

## Spacing, shape and depth

- Use a 4px base. Screens have a `space-4` side gutter, and sections on Today are separated by `space-6`.
- Every interactive element is at least `target-min` (48px) in both dimensions, including the done check, whose visual circle is smaller.
- Task rows and plant cards use `radius-md`. Bottom sheets round their top corners with `radius-lg`. The done check and avatars are `radius-full`.
- Depth is quiet: `shadow-raised` lifts rows and cards slightly off `surface`, and `shadow-sheet` sits under sheets and the FAB. Nothing else casts a shadow.

## Layout and interaction

- Mobile first at 360–430px wide. Primary actions sit in the bottom third, within thumb reach: the FAB, the sheet buttons and the tab bar.
- Bottom tab bar: Today, Plants, Settings. It is `tab-bar` tall plus the safe-area inset.
- A task row shows the plant thumbnail (`thumb-row`), the plant name, the task-type icon and its due line, and a done check on the right. Tap the check to complete with an undo toast. Swipe or long-press the row for snooze and skip.
- Snooze and skip, and all create and edit forms, open in a bottom sheet over `scrim`, never a centred dialog.
- On laptop widths, content centres in a column up to 720px wide and the tab bar becomes a left rail.

## Motion

- Keep motion short: 120ms for presses, 200ms for sheets and toasts, with ease-out on enter and ease-in on exit.
- Completing a task flashes the row to `brand-soft`, then collapses it over 200ms.
- Under `prefers-reduced-motion`, rows disappear without collapsing and sheets fade instead of sliding.

## Iconography

- Lucide icons at 20px (24px in the tab bar), 1.75px stroke, coloured by the text or accent token they sit with.
- Care types: droplet (water), sprout (fertilize), spray-can (mist), shovel (repot), rotate-cw (rotate), sparkles (custom). Clock means overdue, check means done, and alarm-clock-off means snooze.
- The app name is set lowercase, "grower", in `display` type. The leaf mark is `assets/brand/leaf.svg`.

## Components (planned)

Component previews come next, in this order: Button, DoneCheck, TaskRow, Chip, BottomTabBar, TopBar, BottomSheet, TextField, Toast, PlantCard, EmptyState, PhotoTimelineItem.
