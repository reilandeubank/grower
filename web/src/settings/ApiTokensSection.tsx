import { useState } from "react";
import { ChevronRight, KeyRound, Plus } from "lucide-react";
import type { ApiToken } from "../api/client";
import { BottomSheet, ConfirmButton, ErrorText, useToast } from "../design";
import { CreateTokenForm } from "./CreateTokenForm";
import { tokenLine, useApiTokens, useRevokeApiToken } from "./tokens";
import styles from "../routes/Settings.module.css";
import own from "./ApiTokensSection.module.css";

type Sheet = { kind: "create" } | { kind: "token"; token: ApiToken } | null;

/** API tokens: read-only keys for other apps, such as Home Assistant. */
export function ApiTokensSection() {
  const tokens = useApiTokens();
  const [sheet, setSheet] = useState<Sheet>(null);
  const close = () => setSheet(null);

  return (
    <div className={styles.card}>
      <p className={`t-caption ${styles.muted} ${own.intro}`}>
        API tokens let other apps, like Home Assistant, see what needs care. They can't change anything.
      </p>
      {tokens.isError && <ErrorText>Couldn't load your API tokens. Check your connection, then try again.</ErrorText>}
      {tokens.data?.map((token) => (
        <button
          key={token.id}
          type="button"
          className={`${styles.row} ${styles.action}`}
          aria-haspopup="dialog"
          onClick={() => setSheet({ kind: "token", token })}
        >
          <span className={own.token}>
            <KeyRound size={20} strokeWidth={1.75} aria-hidden="true" className={styles.muted} />
            <span className={styles.rowText}>
              <span className={`t-body ${styles.block} ${styles.value}`}>{token.label}</span>{" "}
              <span className={`t-caption ${styles.muted} ${styles.block}`}>{tokenLine(token)}</span>
            </span>
          </span>
          <ChevronRight size={20} strokeWidth={1.75} aria-hidden="true" className={styles.muted} />
        </button>
      ))}
      <button
        type="button"
        className={`${styles.row} ${styles.action}`}
        aria-haspopup="dialog"
        onClick={() => setSheet({ kind: "create" })}
      >
        <span className="t-body">Create API token</span>
        <Plus size={20} strokeWidth={1.75} aria-hidden="true" className={styles.muted} />
      </button>

      <BottomSheet open={sheet?.kind === "create"} onClose={close} title="Create API token">
        <CreateTokenForm onDone={close} />
      </BottomSheet>
      <BottomSheet
        open={sheet?.kind === "token"}
        onClose={close}
        title={sheet?.kind === "token" ? sheet.token.label : "API token"}
      >
        {sheet?.kind === "token" && <TokenDetails token={sheet.token} onDone={close} />}
      </BottomSheet>
    </div>
  );
}

function TokenDetails({ token, onDone }: { token: ApiToken; onDone: () => void }) {
  const revoke = useRevokeApiToken();
  const toast = useToast();
  return (
    <div className={own.form}>
      <p className={`t-body ${styles.muted}`}>{tokenLine(token)}</p>
      <ConfirmButton
        label="Revoke token"
        question="Revoke this token? Anything using it stops working right away."
        confirmLabel="Revoke"
        loading={revoke.isPending}
        onConfirm={() =>
          revoke.mutate(token.id, {
            onSuccess: () => {
              toast({ message: "Token revoked." });
              onDone();
            },
          })
        }
      />
      {revoke.isError && <ErrorText>Couldn't revoke the token. Check your connection, then try again.</ErrorText>}
    </div>
  );
}
