import { useId, useState, type KeyboardEvent, type ReactNode } from "react";
import { Check, Globe, LoaderCircle, Plus } from "lucide-react";
import { problemMessage, type Species, type SpeciesCandidate } from "../api/client";
import { ErrorText } from "../design";
import { Field, fieldStyles } from "../design/Field";
import { useDebounced } from "../lib/useDebounced";
import { lookupNotes, SOURCE_LABELS } from "./copy";
import { useImportSpecies, useLookupSpecies, useSaveSpecies, useSavedSpecies } from "./queries";
import styles from "./SpeciesPicker.module.css";

export interface SpeciesPickerProps {
  text: string;
  /** The linked species' id, or "" for free text. */
  speciesId: string;
  onChange: (next: { text: string; speciesId: string }) => void;
}

type Option =
  | { type: "saved"; species: Species }
  | { type: "online"; candidate: SpeciesCandidate }
  | { type: "create" }
  | { type: "search" };

const MIN_QUERY = 2;

/**
 * The plant form's Species field: free text, with saved species suggested as you type. Searching
 * online costs provider calls, so it runs only from its own row. Editing the text unlinks.
 */
export function SpeciesPicker({ text, speciesId, onChange }: SpeciesPickerProps) {
  const [open, setOpen] = useState(false);
  const [active, setActive] = useState(-1);
  const listId = useId();
  const q = text.trim();
  const saved = useSavedSpecies(useDebounced(q, 250));
  const lookup = useLookupSpecies();
  const importer = useImportSpecies();
  const create = useSaveSpecies();

  const typed = q.length >= MIN_QUERY;
  // Online results belong to the text they were searched for.
  const online = lookup.variables === q ? lookup : undefined;
  const local = typed ? (saved.data ?? []) : [];
  const exists = (name: string) => name.toLowerCase() === q.toLowerCase();
  const known =
    local.some((s) => exists(s.scientific_name)) || online?.data?.items.some((c) => exists(c.scientific_name));

  const options: Option[] = [
    ...local.map((species) => ({ type: "saved", species }) as const),
    ...(online?.data?.items ?? [])
      .filter((c) => !local.some((s) => s.id === c.saved_id))
      .map((candidate) => ({ type: "online", candidate }) as const),
    ...(known || speciesId ? [] : [{ type: "create" } as const]),
    ...(online?.data ? [] : [{ type: "search" } as const]),
  ];
  // Species first, then notes about the online search, then the create and search rows.
  const results = options.filter((o) => o.type === "saved" || o.type === "online").length;
  const expanded = open && typed;
  const busy = importer.isPending || create.isPending;
  const optionId = (i: number) => `${listId}-${i}`;

  const close = () => {
    setOpen(false);
    setActive(-1);
  };
  const link = (s: Pick<Species, "id" | "scientific_name">) => {
    onChange({ text: s.scientific_name, speciesId: s.id });
    close();
  };

  const pick = (o: Option) => {
    if (busy) return;
    if (o.type === "search") {
      setActive(-1);
      if (!lookup.isPending) lookup.mutate(q);
    } else if (o.type === "saved") {
      link(o.species);
    } else if (o.type === "online") {
      const { saved_id, scientific_name, source, ref } = o.candidate;
      if (saved_id) link({ id: saved_id, scientific_name });
      else importer.mutate({ source, ref }, { onSuccess: link });
    } else {
      create.mutate({ body: { scientific_name: q, common_name: "", notes: "" } }, { onSuccess: link });
    }
  };

  const onKeyDown = (e: KeyboardEvent<HTMLInputElement>) => {
    if ((e.key === "ArrowDown" || e.key === "ArrowUp") && typed) {
      e.preventDefault();
      setOpen(true);
      const n = options.length;
      if (n === 0) return;
      if (e.key === "ArrowDown") setActive((a) => (a + 1) % n);
      else setActive((a) => (a <= 0 ? n - 1 : a - 1));
    } else if (e.key === "Enter" && expanded && options[active]) {
      e.preventDefault();
      pick(options[active]);
    } else if (e.key === "Escape" && expanded) {
      e.preventDefault();
      close();
    }
  };

  const failure = importer.error ?? create.error;
  const notes = online?.data ? lookupNotes(online.data) : online?.isError ? ["Couldn't search online. Try again."] : [];

  const row = (o: Option, i: number) => (
    <li
      key={optionKey(o)}
      id={optionId(i)}
      role="option"
      aria-selected={i === active}
      className={[
        styles.option,
        (o.type === "create" || o.type === "search") && styles.action,
        o.type === "online" && options[i - 1]?.type === "saved" && styles.divided,
      ]
        .filter(Boolean)
        .join(" ")}
      // Keep focus in the input so the list stays open through the tap.
      onMouseDown={(e) => e.preventDefault()}
      onClick={() => pick(o)}
    >
      {optionBody(o, q, {
        searching: lookup.isPending,
        adding:
          (o.type === "online" && importer.isPending && importer.variables.ref === o.candidate.ref) ||
          (o.type === "create" && create.isPending),
      })}
    </li>
  );

  return (
    <div className={styles.picker}>
      <Field label="Species" hint={speciesId ? "Linked to a saved species" : "Optional, like Monstera deliciosa"}>
        {(control) => (
          <>
            <input
              {...control}
              className={`t-body ${fieldStyles.input}`}
              role="combobox"
              aria-expanded={expanded}
              aria-controls={expanded ? listId : undefined}
              aria-autocomplete="list"
              aria-activedescendant={expanded && options[active] ? optionId(active) : undefined}
              value={text}
              maxLength={120}
              autoComplete="off"
              autoCapitalize="sentences"
              onChange={(e) => {
                onChange({ text: e.target.value, speciesId: "" });
                setOpen(true);
                setActive(-1);
                importer.reset();
                create.reset();
              }}
              onKeyDown={onKeyDown}
              onBlur={close}
            />
            {expanded && (
              <ul id={listId} role="listbox" aria-label="Species suggestions" className={styles.list}>
                {options.slice(0, results).map(row)}
                {notes.map(note)}
                {options.slice(results).map((o, i) => row(o, results + i))}
              </ul>
            )}
          </>
        )}
      </Field>
      {failure && <ErrorText>{problemMessage(failure)}</ErrorText>}
    </div>
  );
}

function optionKey(o: Option): string {
  if (o.type === "saved") return o.species.id;
  if (o.type === "online") return `${o.candidate.source}:${o.candidate.ref}`;
  return o.type;
}

/** A line of the list that can't be picked, like "No matches online."; arrows skip it. */
function note(text: string) {
  return (
    <li key={text} role="option" aria-disabled="true" aria-selected={false} className={`t-caption ${styles.note}`}>
      {text}
    </li>
  );
}

function Names({ scientific, common }: { scientific: string; common: string }) {
  return (
    <span className={styles.names}>
      <span className={`t-body ${styles.scientific}`}>{scientific}</span>
      {common && <span className={`t-caption ${styles.muted}`}>{common}</span>}
    </span>
  );
}

const spinner = <LoaderCircle className={styles.spin} size={20} strokeWidth={1.75} aria-hidden="true" />;

function optionBody(o: Option, q: string, state: { searching: boolean; adding: boolean }): ReactNode {
  switch (o.type) {
    case "saved":
      return <Names scientific={o.species.scientific_name} common={o.species.common_name} />;
    case "online":
      return (
        <>
          <Names scientific={o.candidate.scientific_name} common={o.candidate.common_name} />
          <span className={styles.badges}>
            {state.adding ? (
              <span className={`t-caption ${styles.muted}`}>Adding…</span>
            ) : (
              <>
                <span className={`t-caption ${styles.badge}`}>{SOURCE_LABELS[o.candidate.source]}</span>
                {o.candidate.saved_id && (
                  <span className={`t-caption ${styles.badge} ${styles.saved}`}>
                    <Check size={14} strokeWidth={2} aria-hidden="true" />
                    Saved
                  </span>
                )}
              </>
            )}
          </span>
        </>
      );
    case "create":
      return (
        <>
          {state.adding ? spinner : <Plus size={20} strokeWidth={1.75} aria-hidden="true" />}
          <span className="t-body">Save as a new species “{q}”</span>
        </>
      );
    case "search":
      return state.searching ? (
        <>
          {spinner}
          <span className="t-body">Searching online…</span>
        </>
      ) : (
        <>
          <Globe size={20} strokeWidth={1.75} aria-hidden="true" />
          <span className="t-body">Search online for “{q}”</span>
        </>
      );
  }
}
