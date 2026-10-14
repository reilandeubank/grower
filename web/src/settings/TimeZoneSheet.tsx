import { useMemo, useState } from "react";
import { Check, Smartphone } from "lucide-react";
import { problemMessage } from "../api/client";
import { BottomSheet, Button, ErrorText, TextField, useToast } from "../design";
import { useUpdateSettings } from "./queries";
import { allTimeZones, deviceTimeZone, searchZones, zoneLabel } from "./timeZones";
import styles from "./TimeZoneSheet.module.css";

export interface TimeZoneSheetProps {
  open: boolean;
  /** The saved zone. */
  current: string;
  onClose: () => void;
}

/** Pick the zone that decides when a new day starts, by search or from this device. */
export function TimeZoneSheet({ open, current, onClose }: TimeZoneSheetProps) {
  return (
    <BottomSheet open={open} onClose={onClose} title="Time zone">
      <ZonePicker current={current} onDone={onClose} />
    </BottomSheet>
  );
}

// Mounted only while the sheet is open, so each opening starts with an empty search.
function ZonePicker({ current, onDone }: { current: string; onDone: () => void }) {
  const [query, setQuery] = useState("");
  // The saved zone leads the list, so it's in view without searching.
  const zones = useMemo(() => [current, ...allTimeZones().filter((z) => z !== current)], [current]);
  const matches = useMemo(() => searchZones(zones, query), [zones, query]);
  const device = deviceTimeZone();
  const save = useUpdateSettings();
  const toast = useToast();

  const choose = (timezone: string) => {
    if (timezone === current) return onDone();
    save.mutate(
      { timezone },
      {
        onSuccess: () => {
          toast({ message: `Time zone set to ${zoneLabel(timezone)}.` });
          onDone();
        },
      },
    );
  };

  return (
    <>
      <TextField
        label="Search"
        type="search"
        placeholder="City or region"
        value={query}
        onChange={(e) => setQuery(e.target.value)}
        autoComplete="off"
        enterKeyHint="search"
      />
      {device && device !== current && (
        <Button
          variant="secondary"
          fullWidth
          className={styles.device}
          icon={<Smartphone size={20} strokeWidth={1.75} aria-hidden="true" />}
          disabled={save.isPending}
          onClick={() => choose(device)}
        >
          Use this device's time zone ({zoneLabel(device)})
        </Button>
      )}
      {save.error && <ErrorText>{problemMessage(save.error)}</ErrorText>}
      {matches.length === 0 ? (
        <p className={`t-body ${styles.muted}`}>No time zones match "{query}".</p>
      ) : (
        <ul className={styles.list} aria-label="Time zones">
          {matches.map((zone) => (
            <li key={zone}>
              <button
                type="button"
                className={`t-body ${styles.zone}`}
                aria-current={zone === current || undefined}
                disabled={save.isPending}
                onClick={() => choose(zone)}
              >
                {zoneLabel(zone)}
                {zone === current && <Check size={20} strokeWidth={1.75} aria-hidden="true" className={styles.check} />}
              </button>
            </li>
          ))}
        </ul>
      )}
    </>
  );
}
