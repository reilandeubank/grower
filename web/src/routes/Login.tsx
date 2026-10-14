import { useState, type FormEvent } from "react";
import { CircleAlert } from "lucide-react";
import { ApiError } from "../api/client";
import { useLogin } from "../auth/hooks";
import { Button, Logo, TextField } from "../design";
import styles from "./Login.module.css";

/** Maps a failed login to the copy we show. Exported for reuse in tests. */
export function loginErrorMessage(error: Error): string {
  if (error instanceof ApiError) {
    if (error.status === 401) return "That username and password don't match.";
    if (error.status === 429) return "Too many attempts. Wait a minute and try again.";
    if (error.status === 400) return "Check your username and password, then try again.";
    return "Something went wrong on the server. Try again in a moment.";
  }
  return "Can't reach grower. Check your connection, then try again.";
}

interface FieldErrors {
  username?: string;
  password?: string;
}

export function Login() {
  const login = useLogin();
  const [username, setUsername] = useState("");
  const [password, setPassword] = useState("");
  const [fieldErrors, setFieldErrors] = useState<FieldErrors>({});

  // On success the /me cache is filled and <RedirectIfSignedIn> takes the user on to ?next.
  const onSubmit = (e: FormEvent<HTMLFormElement>) => {
    e.preventDefault();
    const errors: FieldErrors = {};
    if (!username.trim()) errors.username = "Enter your username.";
    if (!password) errors.password = "Enter your password.";
    setFieldErrors(errors);
    if (errors.username || errors.password) return;
    login.mutate({ username: username.trim(), password });
  };

  return (
    <div className={styles.page}>
      <div className={styles.panel}>
        <div className={styles.hero}>
          <Logo size={64} wordmark layout="stacked" />
          <p className={`t-body ${styles.tagline}`}>Log in to see what your plants need today.</p>
        </div>

        <form className={styles.form} onSubmit={onSubmit} noValidate aria-label="Log in">
          <TextField
            label="Username"
            name="username"
            autoComplete="username"
            autoCapitalize="none"
            autoCorrect="off"
            spellCheck={false}
            enterKeyHint="next"
            value={username}
            onChange={(e) => setUsername(e.target.value)}
            error={fieldErrors.username}
          />
          <TextField
            label="Password"
            name="password"
            type="password"
            autoComplete="current-password"
            enterKeyHint="go"
            value={password}
            onChange={(e) => setPassword(e.target.value)}
            error={fieldErrors.password}
          />
          <div className={styles.alertSlot} aria-live="assertive">
            {login.isError && !login.isPending && (
              <p className={`t-body ${styles.alert}`} role="alert">
                <CircleAlert size={20} strokeWidth={1.75} aria-hidden="true" />
                {loginErrorMessage(login.error)}
              </p>
            )}
          </div>
          <Button type="submit" fullWidth loading={login.isPending}>
            Log in
          </Button>
        </form>
      </div>
    </div>
  );
}
