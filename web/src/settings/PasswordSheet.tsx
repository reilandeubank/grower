import { useState, type FormEvent } from "react";
import { ApiError, problemMessage } from "../api/client";
import { BottomSheet, Button, ErrorText, TextField, useToast } from "../design";
import { useChangePassword } from "./queries";
import styles from "./PasswordSheet.module.css";

const MIN_LENGTH = 10;

export interface PasswordValues {
  current: string;
  next: string;
  confirm: string;
}

type PasswordErrors = Partial<Record<keyof PasswordValues, string>>;

export function validatePassword({ current, next, confirm }: PasswordValues): PasswordErrors {
  const errors: PasswordErrors = {};
  if (!current) errors.current = "Enter your current password.";
  if (next.length < MIN_LENGTH) errors.next = `Use at least ${MIN_LENGTH} characters.`;
  else if (confirm !== next) errors.confirm = "The passwords don't match.";
  return errors;
}

/** The server's own words for a rejected change (wrong password, too many tries). */
function changeError(error: Error): string {
  if (error instanceof ApiError && error.problem?.detail && [400, 401, 429].includes(error.status)) {
    return error.problem.detail;
  }
  return problemMessage(error);
}

export interface PasswordSheetProps {
  open: boolean;
  /** For the password manager, so it updates the right saved login. */
  username: string;
  onClose: () => void;
}

export function PasswordSheet({ open, username, onClose }: PasswordSheetProps) {
  return (
    <BottomSheet open={open} onClose={onClose} title="Change password">
      <PasswordForm username={username} onDone={onClose} />
    </BottomSheet>
  );
}

// Mounted only while the sheet is open, so passwords never linger after it closes.
function PasswordForm({ username, onDone }: { username: string; onDone: () => void }) {
  const [values, setValues] = useState<PasswordValues>({ current: "", next: "", confirm: "" });
  const [errors, setErrors] = useState<PasswordErrors>({});
  const change = useChangePassword();
  const toast = useToast();

  const set = (key: keyof PasswordValues) => (e: { target: { value: string } }) =>
    setValues((v) => ({ ...v, [key]: e.target.value }));

  const onSubmit = (e: FormEvent<HTMLFormElement>) => {
    e.preventDefault();
    const found = validatePassword(values);
    setErrors(found);
    if (Object.keys(found).length > 0) return;
    change.mutate(
      { current_password: values.current, new_password: values.next },
      {
        onSuccess: () => {
          toast({ message: "Password changed. Other devices were signed out." });
          onDone();
        },
      },
    );
  };

  return (
    <form className={styles.form} onSubmit={onSubmit} noValidate>
      <input type="text" name="username" autoComplete="username" value={username} readOnly hidden />
      <TextField
        label="Current password"
        type="password"
        autoComplete="current-password"
        value={values.current}
        onChange={set("current")}
        error={errors.current}
      />
      <TextField
        label="New password"
        type="password"
        autoComplete="new-password"
        hint={`At least ${MIN_LENGTH} characters.`}
        value={values.next}
        onChange={set("next")}
        error={errors.next}
      />
      <TextField
        label="Confirm new password"
        type="password"
        autoComplete="new-password"
        value={values.confirm}
        onChange={set("confirm")}
        error={errors.confirm}
      />
      {change.error && <ErrorText>{changeError(change.error)}</ErrorText>}
      <Button type="submit" fullWidth loading={change.isPending}>
        Change password
      </Button>
    </form>
  );
}
