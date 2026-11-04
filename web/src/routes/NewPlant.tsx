import { useCallback, useRef, useState, type ReactNode } from "react";
import { Navigate, useLocation, useNavigate } from "react-router";
import { ArrowLeft, Plus, X } from "lucide-react";
import { problemMessage, type IdentifyCandidate, type WindowDistance } from "../api/client";
import {
  AppShell,
  Button,
  Choice,
  choiceStyles,
  ErrorText,
  IconButton,
  Switch,
  TextField,
  TopBar,
  useToast,
} from "../design";
import { DISTANCE_LABELS } from "../light/copy";
import { addDays, deviceToday } from "../lib/dates";
import { useBack } from "../lib/useBack";
import { LocationSheet } from "../locations/LocationSheet";
import { LIGHT_LABELS, useLocations } from "../locations/queries";
import { addedButMessage, REPOT_OPTIONS } from "../plants/copy";
import { CoverPhotoField } from "../plants/CoverPhotoField";
import { plantInput, plantValues, validatePlant, type PlantErrors, type PlantValues } from "../plants/form";
import { candidatePrefill, type CandidatePrefill } from "../plants/identify";
import { IdentifyStep } from "../plants/IdentifyStep";
import { clearDraft, getDraft, type PhotoDraft } from "../plants/photoDraft";
import { useCreatePlant } from "../plants/queries";
import { SpeciesPicker } from "../species/SpeciesPicker";
import styles from "./NewPlant.module.css";

const DISTANCES = Object.entries(DISTANCE_LABELS) as [WindowDistance, string][];
const ICON = { size: 20, strokeWidth: 1.75, "aria-hidden": true } as const;

/** Add plant; from "Add from photo" it first identifies the staged photo (see photoDraft). */
export function NewPlant() {
  const { state } = useLocation();
  const [draft] = useState(() => getDraft(state));
  // undefined until the identify step answers; null for no species.
  const [prefill, setPrefill] = useState<CandidatePrefill | null | undefined>(draft ? undefined : null);
  const onIdentified = useCallback((c: IdentifyCandidate | null) => setPrefill(c ? candidatePrefill(c) : null), []);
  if (draft && prefill === undefined) return <IdentifyStep draft={draft} onDone={onIdentified} />;
  return <AddPlantSteps draft={draft} prefill={prefill ?? null} />;
}

function stepOf(state: unknown): number {
  const step = (state as { step?: unknown } | null)?.step;
  return typeof step === "number" ? step : 0;
}

/**
 * One question a screen: the plant, its room, its window, its last repot. Each step is a history
 * entry (router state `step`), so Back goes to the previous question.
 */
function AddPlantSteps({ draft, prefill }: { draft: PhotoDraft | null; prefill: CandidatePrefill | null }) {
  const location = useLocation();
  const navigate = useNavigate();
  const step = stepOf(location.state);
  // Opened mid-way (a reload, or Back into a finished add): the answers are gone, so start over.
  const [stale, setStale] = useState(step > 0);
  if (stale && step === 0) setStale(false);

  const [values, setValues] = useState<PlantValues>(() => ({ ...plantValues(), ...prefill?.values }));
  const [errors, setErrors] = useState<PlantErrors>({});
  const [photo, setPhoto] = useState(draft);
  const [sheetOpen, setSheetOpen] = useState(false);
  const nameRef = useRef<HTMLInputElement>(null);
  const hoursRef = useRef<HTMLInputElement>(null);
  const locations = useLocations();
  const create = useCreatePlant();
  const toast = useToast();
  const close = useBack("/plants");

  if (stale) return <Navigate to={location.pathname} replace state={{ ...location.state, step: 0 }} />;

  const go = (next: number, replace = false) =>
    void navigate(location.pathname, { replace, state: { ...location.state, step: next } });
  const update = (changes: Partial<PlantValues>) => setValues((v) => ({ ...v, ...changes }));

  /** On to the next step, unless this step's field is invalid. */
  const next = () => {
    const key = step === 0 ? "name" : "growLightHours";
    const error = validatePlant(values)[key];
    setErrors(error ? { [key]: error } : {});
    if (error) (step === 0 ? nameRef : hoursRef).current?.focus();
    else go(step + 1);
  };

  const save = (repotOn: string | null) => {
    const body = plantInput(values);
    // The identified species is created on save, and only if the field still names it.
    const pending = prefill?.newSpecies;
    const newSpecies = pending && !values.speciesId && body.species === pending.scientific_name ? pending : null;
    create.mutate(
      { body, newSpecies, photo: photo?.photo ?? null, repotOn },
      {
        onSuccess: async ({ plant, photoError, repotError }) => {
          const message = addedButMessage(plant.display_name, photoError !== null, repotError !== null);
          if (message) toast({ message });
          // Drop the steps from history, so Back from the plant returns to Plants.
          await navigate(-step);
          await navigate(`/plants/${plant.id}`, { replace: true });
          clearDraft();
        },
      },
    );
  };

  let question: string;
  let hint: string | null = null;
  let content: ReactNode;
  let footer: ReactNode;
  const nextButton = (
    <Button type="button" onClick={next}>
      Next
    </Button>
  );

  if (step === 0) {
    question = "What plant is it?";
    content = (
      <>
        {photo && <CoverPhotoField url={photo.url} onRemove={() => setPhoto(null)} />}
        <SpeciesPicker
          text={values.species}
          speciesId={values.speciesId}
          onChange={({ text, speciesId }) => update({ species: text, speciesId })}
        />
        <TextField
          ref={nameRef}
          label="Name"
          hint="Optional if you add a species: the plant goes by that."
          value={values.name}
          onChange={(e) => update({ name: e.target.value })}
          error={errors.name}
          maxLength={80}
          autoComplete="off"
          autoCapitalize="words"
        />
      </>
    );
    footer = nextButton;
  } else if (step === 1) {
    question = "Where does it live?";
    content = (
      <ul className={choiceStyles.choices}>
        {(locations.data ?? []).map((l) => {
          const light = l.light ?? l.suggested_light;
          return (
            <li key={l.id}>
              <button
                type="button"
                className={choiceStyles.choice}
                aria-pressed={values.locationId === l.id}
                onClick={() => {
                  update({ locationId: l.id });
                  go(2);
                }}
              >
                <Choice title={l.name} detail={light ? LIGHT_LABELS[light] : undefined} />
              </button>
            </li>
          );
        })}
        <li>
          <button type="button" className={choiceStyles.choice} onClick={() => setSheetOpen(true)}>
            <Choice icon={<Plus {...ICON} />} title="New location" />
          </button>
        </li>
      </ul>
    );
    footer = (
      <Button
        type="button"
        variant="secondary"
        onClick={() => {
          update({ locationId: "" });
          go(2);
        }}
      >
        Skip
      </Button>
    );
  } else if (step === 2) {
    question = "How far is it from a window?";
    hint = "Light fades quickly away from the glass.";
    content = (
      <>
        <ul className={choiceStyles.choices}>
          {DISTANCES.map(([value, label]) => (
            <li key={value}>
              <button
                type="button"
                className={choiceStyles.choice}
                aria-pressed={values.windowDistance === value}
                onClick={() => update({ windowDistance: value })}
              >
                <Choice title={label} />
              </button>
            </li>
          ))}
        </ul>
        <Switch label="Grow light" checked={values.growLight} onChange={(growLight) => update({ growLight })} />
        {values.growLight && (
          <TextField
            ref={hoursRef}
            label="Hours a day"
            suffix="h"
            type="number"
            inputMode="numeric"
            step={1}
            min={1}
            max={24}
            value={values.growLightHours}
            onChange={(e) => update({ growLightHours: e.target.value })}
            error={errors.growLightHours}
          />
        )}
      </>
    );
    footer = (
      <>
        <Button
          type="button"
          variant="secondary"
          onClick={() => {
            update({ windowDistance: "", growLight: false });
            setErrors({});
            go(3);
          }}
        >
          Skip
        </Button>
        {nextButton}
      </>
    );
  } else {
    question = "When was it last repotted?";
    hint = "grower reminds you to repot about once a year.";
    content = (
      <>
        <ul className={choiceStyles.choices}>
          {REPOT_OPTIONS.map((o) => (
            <li key={o.label}>
              <button
                type="button"
                className={choiceStyles.choice}
                disabled={create.isPending}
                onClick={() => save(addDays(deviceToday(), o.dueInDays))}
              >
                <Choice title={o.label} />
              </button>
            </li>
          ))}
        </ul>
        {create.error && <ErrorText>{problemMessage(create.error)}</ErrorText>}
      </>
    );
    footer = (
      <Button type="button" variant="secondary" loading={create.isPending} onClick={() => save(null)}>
        Skip and save
      </Button>
    );
  }

  return (
    <AppShell
      tabBar={false}
      topBar={
        <TopBar
          title="Add plant"
          leading={
            step === 0 ? (
              <IconButton aria-label="Close" icon={<X {...ICON} />} onClick={close} />
            ) : (
              <IconButton aria-label="Back" icon={<ArrowLeft {...ICON} />} onClick={() => void navigate(-1)} />
            )
          }
        />
      }
    >
      <div className={styles.page}>
        <h2 className="t-title">{question}</h2>
        {hint && <p className={`t-body ${styles.muted}`}>{hint}</p>}
        {content}
        <div className={styles.footer}>{footer}</div>
      </div>
      <LocationSheet
        open={sheetOpen}
        onClose={() => setSheetOpen(false)}
        // Replace the sheet's history entry, as when leaving a page from a sheet.
        onSaved={(l) => {
          update({ locationId: l.id });
          go(2, true);
        }}
      />
    </AppShell>
  );
}
