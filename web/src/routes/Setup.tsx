import { useState, type FormEvent } from "react";
import { Link } from "react-router";
import { useQueryClient } from "@tanstack/react-query";
import { CircleAlert } from "lucide-react";
import { ApiError } from "../api/client";
import { setupDone, setupKey, useCompleteSetup, useSetupStatus } from "../auth/hooks";
import { Button, Logo, TextField } from "../design";
import { deviceTimeZone, zoneLabel } from "../settings/timeZones";
import loginStyles from "./Login.module.css";
import styles from "./Setup.module.css";

const MIN_PASSWORD_LENGTH = 10;

export interface SetupFormValues {
  username: string;
  password: string;
  confirm: string;
}

type SetupErrors = Partial<Record<keyof SetupFormValues, string>>;

export function validateSetup({ username, password, confirm }: SetupFormValues): SetupErrors {
  const errors: SetupErrors = {};
  if (!username.trim()) errors.username = "Choose a username.";
  else if (/\s/.test(username.trim())) errors.username = "Use a username with no spaces.";
  if (password.length < MIN_PASSWORD_LENGTH) errors.password = `Use at least ${MIN_PASSWORD_LENGTH} characters.`;
  else if (confirm !== password) errors.confirm = "The passwords don't match.";
  return errors;
}

/** Maps a failed setup to the copy we show; the server's words where it has them. */
export function setupErrorMessage(error: Error): string {
  if (error instanceof ApiError) {
    if ([400, 403, 409, 429].includes(error.status) && error.problem?.detail) return error.problem.detail;
    if (error.status === 429) return "Too many attempts. Wait a minute and try again.";
    return "Something went wrong on the server. Try again in a moment.";
  }
  return "Can't reach grower. Check your connection, then try again.";
}

export function Setup() {
  const status = useSetupStatus(true);
  const open = status.data?.open ?? false;

  return (
    <div className={loginStyles.page}>
      <div className={loginStyles.panel}>
        <div className={loginStyles.hero}>
          <Logo size={64} wordmark layout="stacked" />
          <h1 className="t-title">Create your account</h1>
          <p className={`t-body ${loginStyles.tagline}`}>
            {open
              ? "Choose the username and password you'll log in with."
              : "Setup closed 10 minutes after grower started. Restart grower, then reload this page."}
          </p>
        </div>
        {open && <SetupForm />}
      </div>
    </div>
  );
}

function SetupForm() {
  const setup = useCompleteSetup();
  const queryClient = useQueryClient();
  const [values, setValues] = useState<SetupFormValues>({ username: "", password: "", confirm: "" });
  const [errors, setErrors] = useState<SetupErrors>({});
  const timezone = deviceTimeZone();

  const set = (key: keyof SetupFormValues) => (e: { target: { value: string } }) =>
    setValues((v) => ({ ...v, [key]: e.target.value }));

  // On success /me is filled and <RequireSetupNeeded> takes the new user on to Today.
  const onSubmit = (e: FormEvent<HTMLFormElement>) => {
    e.preventDefault();
    const found = validateSetup(values);
    setErrors(found);
    if (Object.keys(found).length > 0) return;
    setup.mutate({ username: values.username.trim(), password: values.password, timezone });
  };

  const alreadySetUp = setup.error instanceof ApiError && setup.error.status === 409;

  return (
    <form className={loginStyles.form} onSubmit={onSubmit} noValidate aria-label="Create your account">
      <TextField
        label="Username"
        name="username"
        autoComplete="username"
        autoCapitalize="none"
        autoCorrect="off"
        spellCheck={false}
        enterKeyHint="next"
        value={values.username}
        onChange={set("username")}
        error={errors.username}
      />
      <TextField
        label="Password"
        name="new-password"
        type="password"
        hint={`At least ${MIN_PASSWORD_LENGTH} characters.`}
        autoComplete="new-password"
        enterKeyHint="next"
        value={values.password}
        onChange={set("password")}
        error={errors.password}
      />
      <TextField
        label="Confirm password"
        name="confirm-password"
        type="password"
        autoComplete="new-password"
        enterKeyHint="go"
        value={values.confirm}
        onChange={set("confirm")}
        error={errors.confirm}
      />
      {timezone && (
        <p className={`t-caption ${styles.note}`}>
          Time zone: {zoneLabel(timezone)}. You can change it later in Settings.
        </p>
      )}
      <div className={loginStyles.alertSlot} aria-live="assertive">
        {setup.isError && !setup.isPending && (
          <p className={`t-body ${loginStyles.alert}`} role="alert">
            <CircleAlert size={20} strokeWidth={1.75} aria-hidden="true" />
            <span>
              {setupErrorMessage(setup.error)}
              {alreadySetUp && (
                <>
                  {" "}
                  {/* Someone else finished setup: record that, or the guards send us back here. */}
                  <Link
                    to="/login"
                    className={`t-body-strong ${styles.link}`}
                    onClick={() => queryClient.setQueryData(setupKey, setupDone)}
                  >
                    Go to log in
                  </Link>
                </>
              )}
            </span>
          </p>
        )}
      </div>
      <Button type="submit" fullWidth loading={setup.isPending}>
        Create account
      </Button>
    </form>
  );
}
