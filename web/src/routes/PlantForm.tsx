import { useCallback, useRef, useState, type ChangeEvent, type FormEvent } from "react";
import { useLocation, useNavigate, useParams } from "react-router";
import { Archive, ArchiveRestore, ArrowLeft, Plus, X } from "lucide-react";
import { problemMessage, type IdentifyCandidate, type Plant } from "../api/client";
import {
  AppShell,
  Button,
  ConfirmButton,
  ErrorText,
  IconButton,
  SelectField,
  Switch,
  TextAreaField,
  TextField,
  TopBar,
  useToast,
} from "../design";
import { DISTANCE_LABELS } from "../light/copy";
import { useBack } from "../lib/useBack";
import { LocationSheet } from "../locations/LocationSheet";
import { LIGHT_LABELS, useLocations } from "../locations/queries";
import { POT_LABELS } from "../plants/copy";
import {
  plantInput,
  plantValues,
  POT_SIZE_MAX,
  POT_SIZE_MIN,
  validatePlant,
  type PlantErrors,
  type PlantValues,
} from "../plants/form";
import { CoverPhotoField } from "../plants/CoverPhotoField";
import { candidatePrefill, type CandidatePrefill } from "../plants/identify";
import { IdentifyStep } from "../plants/IdentifyStep";
import { clearDraft, getDraft, type PhotoDraft } from "../plants/photoDraft";
import { plantUpdate, useCreatePlant, useDeletePlant, usePlant, useSavePlant } from "../plants/queries";
import { SpeciesPicker } from "../species/SpeciesPicker";
import { LoadError } from "./LoadError";
import { Pending } from "./Pending";
import styles from "./PlantForm.module.css";

const POT_OPTIONS = [
  { value: "", label: "Not set" },
  ...Object.entries(POT_LABELS).map(([value, label]) => ({ value, label })),
];

const DISTANCE_OPTIONS = [
  { value: "", label: "Not set" },
  ...Object.entries(DISTANCE_LABELS).map(([value, label]) => ({ value, label })),
];

const LIGHT_OPTIONS = [
  { value: "", label: "Not set" },
  ...Object.entries(LIGHT_LABELS).map(([value, label]) => ({ value, label })),
];

const DRAINAGE_OPTIONS = [
  { value: "", label: "Not set" },
  { value: "yes", label: "Yes" },
  { value: "no", label: "No" },
];

/** Add plant; from "Add from photo" it first identifies the staged photo (see photoDraft). */
export function NewPlant() {
  const { state } = useLocation();
  const [draft] = useState(() => getDraft(state));
  // undefined until the identify step answers; null for no species.
  const [prefill, setPrefill] = useState<CandidatePrefill | null | undefined>(draft ? undefined : null);
  const onIdentified = useCallback(
    (c: IdentifyCandidate | null) => setPrefill(c ? candidatePrefill(c) : null),
    [],
  );
  if (draft && prefill === undefined) return <IdentifyStep draft={draft} onDone={onIdentified} />;
  return <PlantForm draft={draft} prefill={prefill} />;
}

export function EditPlant() {
  const { id = "" } = useParams();
  const plant = usePlant(id);
  const back = useBack(`/plants/${id}`);
  if (plant.isSuccess) return <PlantForm plant={plant.data} />;
  return (
    <AppShell
      tabBar={false}
      topBar={
        <TopBar
          title="Edit plant"
          leading={<IconButton aria-label="Back" icon={<ArrowLeft size={20} strokeWidth={1.75} />} onClick={back} />}
        />
      }
    >
      {plant.isPending ? <Pending inline /> : <LoadError onRetry={() => void plant.refetch()} />}
    </AppShell>
  );
}

/** Full-screen add/edit page: Close at the top left, Save at the bottom. */
function PlantForm({
  plant,
  draft = null,
  prefill = null,
}: {
  plant?: Plant;
  draft?: PhotoDraft | null;
  prefill?: CandidatePrefill | null | undefined;
}) {
  const [values, setValues] = useState<PlantValues>(() => ({ ...plantValues(plant), ...prefill?.values }));
  const [errors, setErrors] = useState<PlantErrors>({});
  const [sheetOpen, setSheetOpen] = useState(false);
  const [photo, setPhoto] = useState(draft);
  const nameRef = useRef<HTMLInputElement>(null);
  const potSizeRef = useRef<HTMLInputElement>(null);
  const hoursRef = useRef<HTMLInputElement>(null);
  const locations = useLocations();
  const toast = useToast();
  const create = useCreatePlant();
  const save = useSavePlant();
  const archive = useSavePlant();
  const remove = useDeletePlant();
  const navigate = useNavigate();
  const close = useBack(plant ? `/plants/${plant.id}` : "/plants");

  const set = (key: keyof PlantValues) => (e: ChangeEvent<HTMLInputElement | HTMLSelectElement | HTMLTextAreaElement>) =>
    setValues((v) => ({ ...v, [key]: e.target.value }));

  const onSubmit = (e: FormEvent<HTMLFormElement>) => {
    e.preventDefault();
    const found = validatePlant(values);
    setErrors(found);
    if (found.name) nameRef.current?.focus();
    else if (found.growLightHours) hoursRef.current?.focus();
    else if (found.potSize) potSizeRef.current?.focus();
    if (Object.keys(found).length > 0) return;
    const body = plantInput(values);
    if (plant) {
      save.mutate({ id: plant.id, body: { ...body, archived: plant.archived } }, { onSuccess: close });
      return;
    }
    // The identified species is created on save, and only if the field still names it.
    const pending = prefill?.newSpecies;
    const newSpecies = pending && !values.speciesId && body.species === pending.scientific_name ? pending : null;
    create.mutate(
      { body, newSpecies, photo: photo?.photo ?? null },
      {
        onSuccess: ({ plant: p, photoError }) => {
          if (photoError) toast({ message: `Added ${p.name}, but the photo didn't upload. Add it from Photos.` });
          navigate(`/plants/${p.id}`, { replace: true });
          clearDraft();
        },
      },
    );
  };
  const saving = plant ? save : create;

  const locationOptions = [
    { value: "", label: "No location" },
    ...(locations.data ?? []).map((l) => ({ value: l.id, label: l.name })),
  ];
  const otherFailure = archive.error ?? remove.error;

  return (
    <AppShell
      tabBar={false}
      topBar={
        <TopBar
          title={plant ? "Edit plant" : "Add plant"}
          leading={<IconButton aria-label="Close" icon={<X size={20} strokeWidth={1.75} />} onClick={close} />}
        />
      }
    >
      <form className={styles.form} onSubmit={onSubmit} noValidate aria-label={plant ? "Edit plant" : "Add plant"}>
        <div className={styles.fields}>
          {photo && <CoverPhotoField url={photo.url} onRemove={() => setPhoto(null)} />}
          <TextField
            ref={nameRef}
            label="Name"
            value={values.name}
            onChange={set("name")}
            error={errors.name}
            maxLength={80}
            autoComplete="off"
            autoCapitalize="words"
          />
          <SpeciesPicker
            text={values.species}
            speciesId={values.speciesId}
            onChange={({ text, speciesId }) => setValues((v) => ({ ...v, species: text, speciesId }))}
          />
          <div className={styles.location}>
            <SelectField
              label="Location"
              value={values.locationId}
              options={locationOptions}
              onChange={set("locationId")}
            />
            <Button
              variant="ghost"
              className={styles.newLocation}
              icon={<Plus size={20} strokeWidth={1.75} aria-hidden="true" />}
              onClick={() => setSheetOpen(true)}
            >
              New location
            </Button>
          </div>
          <TextField label="Acquired" type="date" value={values.acquiredOn} onChange={set("acquiredOn")} />
          <TextAreaField label="Notes" value={values.notes} onChange={set("notes")} maxLength={4000} />

          <fieldset className={styles.group}>
            <legend className={`t-heading ${styles.legend}`}>Light</legend>
            <SelectField
              label="Distance from the window"
              value={values.windowDistance}
              options={DISTANCE_OPTIONS}
              onChange={set("windowDistance")}
            />
            <div>
              <Switch
                label="Grow light"
                checked={values.growLight}
                onChange={(growLight) => setValues((v) => ({ ...v, growLight }))}
              />
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
                  onChange={set("growLightHours")}
                  error={errors.growLightHours}
                />
              )}
            </div>
            <SelectField
              label="Light level (optional)"
              hint="Overrides the room's light."
              value={values.light}
              options={LIGHT_OPTIONS}
              onChange={set("light")}
            />
          </fieldset>

          <fieldset className={styles.group}>
            <legend className={`t-heading ${styles.legend}`}>Pot and soil</legend>
            <SelectField label="Pot type" value={values.potType} options={POT_OPTIONS} onChange={set("potType")} />
            <div className={styles.pair}>
              <TextField
                ref={potSizeRef}
                label="Pot size"
                hint="Inches across the top"
                suffix="in"
                type="number"
                inputMode="decimal"
                step={0.5}
                min={POT_SIZE_MIN}
                max={POT_SIZE_MAX}
                value={values.potSize}
                onChange={set("potSize")}
                error={errors.potSize}
              />
              <SelectField
                label="Drainage holes"
                value={values.drainage}
                options={DRAINAGE_OPTIONS}
                onChange={set("drainage")}
              />
            </div>
            <TextField
              label="Soil"
              placeholder="e.g. Aroid mix"
              value={values.soil}
              onChange={set("soil")}
              maxLength={120}
              autoComplete="off"
            />
          </fieldset>
        </div>

        {plant && (
          <div className={styles.manage}>
            <Button
              variant="secondary"
              fullWidth
              loading={archive.isPending}
              icon={
                plant.archived ? (
                  <ArchiveRestore size={20} strokeWidth={1.75} aria-hidden="true" />
                ) : (
                  <Archive size={20} strokeWidth={1.75} aria-hidden="true" />
                )
              }
              onClick={() =>
                archive.mutate({ id: plant.id, body: plantUpdate(plant, { archived: !plant.archived }) }, { onSuccess: close })
              }
            >
              {plant.archived ? "Restore plant" : "Archive plant"}
            </Button>
            <ConfirmButton
              label="Delete plant"
              question="Delete plant? This can't be undone."
              loading={remove.isPending}
              onConfirm={() => remove.mutate(plant.id, { onSuccess: () => navigate("/plants", { replace: true }) })}
            />
            {otherFailure && <ErrorText>{problemMessage(otherFailure)}</ErrorText>}
          </div>
        )}

        <div className={styles.footer}>
          {saving.error && <ErrorText>{problemMessage(saving.error)}</ErrorText>}
          <Button type="submit" fullWidth loading={saving.isPending}>
            Save
          </Button>
        </div>
      </form>

      <LocationSheet
        open={sheetOpen}
        onClose={() => setSheetOpen(false)}
        onSaved={(l) => setValues((v) => ({ ...v, locationId: l.id }))}
      />
    </AppShell>
  );
}
