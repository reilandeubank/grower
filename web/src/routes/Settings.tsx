import { LogOut } from "lucide-react";
import { useNavigate } from "react-router";
import { useLogout } from "../auth/hooks";
import { useCurrentUser } from "../auth/useCurrentUser";
import { AppShell, Button, TopBar } from "../design";
import styles from "./Settings.module.css";

export function Settings() {
  const me = useCurrentUser();
  const logout = useLogout();
  const navigate = useNavigate();

  const onLogout = () =>
    logout.mutate(undefined, { onSuccess: () => navigate("/login", { replace: true }) });

  return (
    <AppShell topBar={<TopBar title="Settings" />}>
      <div className={styles.sections}>
        <section aria-labelledby="settings-account" className={styles.section}>
          <h2 id="settings-account" className="t-heading">
            Account
          </h2>
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
          </div>
        </section>

        <section aria-labelledby="settings-about" className={styles.section}>
          <h2 id="settings-about" className="t-heading">
            About
          </h2>
          <dl className={styles.card}>
            <div className={styles.row}>
              <dt className="t-body">Version</dt>
              <dd className={`t-caption ${styles.muted}`}>{me.version}</dd>
            </div>
            <div className={styles.row}>
              <dt className="t-body">Time zone</dt>
              <dd className={`t-caption ${styles.muted}`}>{me.settings.timezone}</dd>
            </div>
          </dl>
        </section>
      </div>
    </AppShell>
  );
}
