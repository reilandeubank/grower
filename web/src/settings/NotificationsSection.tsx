import { useState } from "react";
import { BellRing } from "lucide-react";
import { useCurrentUser } from "../auth/useCurrentUser";
import { Button, ErrorText, Switch, useToast } from "../design";
import { useUpdateSettings } from "./queries";
import {
  usePushState,
  useSendTestPush,
  useSetReminder,
  type PushState,
} from "./push";
import styles from "../routes/Settings.module.css";
import own from "./NotificationsSection.module.css";

const DESCRIPTION: Record<PushState, string> = {
  unsupported: "This browser can't show notifications.",
  denied:
    "Notifications are blocked for grower. Allow them in your browser's site settings.",
  off: "A note at your reminder time when plants need care.",
  on: "A note at your reminder time when plants need care.",
};

/** The daily reminder: on or off for this device, its time, and a test. */
export function NotificationsSection() {
  const state = usePushState();
  const setReminder = useSetReminder();
  const sendTest = useSendTestPush();
  const toast = useToast();
  const status = state.data;
  const on = status === "on";

  return (
    <div className={styles.card}>
      <Switch
        className={styles.row}
        label="Daily reminder"
        description={status ? DESCRIPTION[status] : "Checking this device…"}
        checked={on}
        disabled={status !== "on" && status !== "off"}
        busy={setReminder.isPending}
        onChange={(next) => setReminder.mutate(next)}
      />
      {setReminder.isError && (
        <ErrorText>
          {setReminder.variables
            ? "Couldn't turn on the reminder. Check your connection, then try again."
            : "Couldn't turn off the reminder. Check your connection, then try again."}
        </ErrorText>
      )}
      <TimeSetting
        field="digest_time"
        label="Reminder time"
        error="Couldn't save the reminder time. Check your connection, then try again."
      />
      <TimeSetting
        field="later_time"
        label="Remind me later at"
        hint="The reminder offers a button to send it again at this time."
        error="Couldn't save the later time. Check your connection, then try again."
      />
      {on && (
        <div className={styles.row}>
          <Button
            variant="secondary"
            icon={<BellRing size={20} strokeWidth={1.75} aria-hidden="true" />}
            loading={sendTest.isPending}
            onClick={() =>
              sendTest.mutate(undefined, {
                onSuccess: () => toast({ message: "Test notification sent." }),
              })
            }
          >
            Send test notification
          </Button>
        </div>
      )}
      {sendTest.isError && (
        <ErrorText>
          Couldn't send a test notification. Try again in a minute.
        </ErrorText>
      )}
      <p className={`t-caption ${styles.muted} ${own.note}`}>
        On Android, the reminder arrives even when grower is closed, as long as
        it's installed.
      </p>
    </div>
  );
}

/** A native time picker for one of the reminder times, saved as soon as it holds a whole time. */
function TimeSetting({
  field,
  label,
  hint,
  error,
}: {
  field: "digest_time" | "later_time";
  label: string;
  hint?: string;
  error: string;
}) {
  const me = useCurrentUser();
  const save = useUpdateSettings();
  const saved = me.settings[field];
  const [value, setValue] = useState(saved);

  return (
    <>
      <label className={styles.row}>
        <span>
          <span className="t-body">{label}</span>
          {hint && (
            <span className={`t-caption ${styles.muted} ${own.hint}`}>
              {hint}
            </span>
          )}
        </span>
        <input
          type="time"
          className={own.time}
          value={value}
          required
          onChange={(e) => {
            const next = e.target.value;
            setValue(next);
            if (/^\d\d:\d\d$/.test(next) && next !== saved)
              save.mutate({ [field]: next });
          }}
        />
      </label>
      {save.isError && <ErrorText>{error}</ErrorText>}
    </>
  );
}
