import { useState, type FormEvent } from "react";
import { Copy } from "lucide-react";
import { problemMessage } from "../api/client";
import { Button, ErrorText, TextField, useToast } from "../design";
import { HOME_ASSISTANT_PATH, useCreateApiToken } from "./tokens";
import own from "./ApiTokensSection.module.css";

const MAX_LABEL = 64;

/**
 * Names a new token, then shows its secret once. The sheet unmounts this when it closes,
 * which drops the secret from the page and (see useCreateApiToken) from the query cache.
 */
export function CreateTokenForm({ onDone }: { onDone: () => void }) {
  const [label, setLabel] = useState("Home Assistant");
  const [error, setError] = useState<string>();
  const create = useCreateApiToken();

  const onSubmit = (e: FormEvent<HTMLFormElement>) => {
    e.preventDefault();
    const name = label.trim();
    if (!name) return setError("Name the token, for example after the app that will use it.");
    if (name.length > MAX_LABEL) return setError(`Use a name of at most ${MAX_LABEL} characters.`);
    setError(undefined);
    create.mutate(name);
  };

  if (create.data) return <Reveal secret={create.data.secret} onDone={onDone} />;

  return (
    <form className={own.form} onSubmit={onSubmit} noValidate>
      <TextField
        label="Name"
        hint="What will use it, so you can tell tokens apart."
        value={label}
        maxLength={MAX_LABEL}
        onChange={(e) => setLabel(e.target.value)}
        error={error}
      />
      {create.error && <ErrorText>{problemMessage(create.error)}</ErrorText>}
      <Button type="submit" fullWidth loading={create.isPending}>
        Create token
      </Button>
    </form>
  );
}

function Reveal({ secret, onDone }: { secret: string; onDone: () => void }) {
  const toast = useToast();
  const address = `${window.location.origin}${HOME_ASSISTANT_PATH}`;
  const copy = async () => {
    try {
      await navigator.clipboard.writeText(secret);
      toast({ message: "Token copied." });
    } catch {
      toast({ message: "Couldn't copy. Select the token and copy it yourself." });
    }
  };

  return (
    <div className={own.form}>
      <p className="t-body">Copy the token now. grower keeps only a fingerprint of it, so it can't show it again.</p>
      <TextField
        label="Token"
        value={secret}
        readOnly
        spellCheck={false}
        autoComplete="off"
        onFocus={(e) => e.target.select()}
      />
      <Button variant="secondary" icon={<Copy size={20} strokeWidth={1.75} aria-hidden="true" />} onClick={copy}>
        Copy token
      </Button>
      <TextField
        label="Address"
        hint="In Home Assistant, add a REST sensor that reads this address, sending the token as a bearer token. grower's docs have an example."
        value={address}
        readOnly
        spellCheck={false}
        onFocus={(e) => e.target.select()}
      />
      <Button fullWidth onClick={onDone}>
        Done
      </Button>
    </div>
  );
}
