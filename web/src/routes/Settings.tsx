import { useState, type ReactNode } from "react";
import { ChevronRight, LogOut, Monitor, Moon, Sun } from "lucide-react";
import { Link, useNavigate } from "react-router";
import { useLogout } from "../auth/hooks";
import { useCurrentUser } from "../auth/useCurrentUser";
import { AppShell, Button, SegmentedControl, TopBar } from "../design";
import { readTheme, saveTheme, type Theme } from "../lib/theme";
import { DataSection } from "../settings/DataSection";
import { PasswordSheet } from "../settings/PasswordSheet";
import { TimeZoneSheet } from "../settings/TimeZoneSheet";
import { zoneLabel } from "../settings/timeZones";
import styles from "./Settings.module.css";

const icon = (Icon: typeof Sun) => <Icon size={16} strokeWidth={1.75} aria-hidden="true" />;
const THEMES: { value: Theme; label: string; icon: ReactNode }[] = [
  { value: "system", label: "System", icon: icon(Monitor) },
  { value: "light", label: "Light", icon: icon(Sun) },
  { value: "dark", label: "Dark", icon: icon(Moon) },
];

export function Settings() {
  const me = useCurrentUser();
  const logout = useLogout();
  const navigate = useNavigate();
  const [theme, setTheme] = useState(readTheme);
  const [sheet, setSheet] = useState<"timezone" | "password" | null>(null);
  const closeSheet = () => setSheet(null);

  const onLogout = () => logout.mutate(undefined, { onSuccess: () => navigate("/login", { replace: true }) });

  return (
    <AppShell topBar={<TopBar title="Settings" />}>
      <div className={styles.sections}>
        <Section id="settings-appearance" title="Appearance">
          <div className={`${styles.card} ${styles.padded}`}>
            <SegmentedControl
              label="Theme"
              options={THEMES}
              value={theme}
              onChange={(t) => {
                setTheme(t);
                saveTheme(t);
              }}
            />
          </div>
        </Section>

        <Section id="settings-general" title="General">
          <div className={styles.card}>
            <RowButton label="Time zone" value={zoneLabel(me.settings.timezone)} onClick={() => setSheet("timezone")} />
            <Link to="/settings/locations" className={`${styles.row} ${styles.action}`}>
              <span className="t-body">Locations</span>
              <ChevronRight size={20} strokeWidth={1.75} aria-hidden="true" className={styles.muted} />
            </Link>
          </div>
        </Section>

        <Section id="settings-account" title="Account">
          <div className={styles.card}>
            <div className={styles.row}>
              <div className={styles.rowText}>
                <p className={`t-caption ${styles.muted}`}>Signed in as</p>
                <p className={`t-body-strong ${styles.value}`}>{me.user.username}</p>
              </div>
              <Button
                variant="secondary"
                icon={<LogOut size={20} strokeWidth={1.75} aria-hidden="true" />}
                loading={logout.isPending}
                onClick={onLogout}
              >
                Log out
              </Button>
            </div>
            {logout.isError && (
              <p className={`t-caption ${styles.error}`} role="alert">
                Couldn't log out. Check your connection, then try again.
              </p>
            )}
            <RowButton label="Change password" onClick={() => setSheet("password")} />
          </div>
        </Section>

        <Section id="settings-data" title="Data">
          <DataSection />
        </Section>

        <Section id="settings-about" title="About">
          <dl className={styles.card}>
            <div className={styles.row}>
              <dt className="t-body">Version</dt>
              <dd className={`t-caption ${styles.muted}`}>{me.version}</dd>
            </div>
          </dl>
        </Section>
      </div>

      <TimeZoneSheet open={sheet === "timezone"} current={me.settings.timezone} onClose={closeSheet} />
      <PasswordSheet open={sheet === "password"} username={me.user.username} onClose={closeSheet} />
    </AppShell>
  );
}

function Section({ id, title, children }: { id: string; title: string; children: ReactNode }) {
  return (
    <section aria-labelledby={id} className={styles.section}>
      <h2 id={id} className="t-heading">
        {title}
      </h2>
      {children}
    </section>
  );
}

/** A row that opens a sheet: label, the current value, and a chevron. */
function RowButton({ label, value, onClick }: { label: string; value?: string; onClick: () => void }) {
  return (
    <button type="button" className={`${styles.row} ${styles.action}`} aria-haspopup="dialog" onClick={onClick}>
      <span className="t-body">{label}</span>
      {/* Flex drops the space visually; it keeps the accessible name "Time zone America/Chicago". */}{" "}
      <span className={styles.trailing}>
        {value && <span className={`t-caption ${styles.muted}`}>{value}</span>}
        <ChevronRight size={20} strokeWidth={1.75} aria-hidden="true" className={styles.muted} />
      </span>
    </button>
  );
}
