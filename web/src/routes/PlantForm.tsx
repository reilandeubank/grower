import { useRef, useState, type ChangeEvent, type FormEvent } from "react";
import { useNavigate, useParams } from "react-router";
import { Archive, ArchiveRestore, ArrowLeft, Plus, X } from "lucide-react";
import { problemMessage, type Plant, type PlantInput } from "../api/client";
import {
  AppShell,
  Button,
  ConfirmButton,
  ErrorText,
  IconButton,
  SelectField,
  TextAreaField,
  TextField,
  TopBar,
} from "../design";
import { useBack } from "../lib/useBack";
import { LocationSheet } from "../locations/LocationSheet";
import { useLocations } from "../locations/queries";
import { plantUpdate, useDeletePlant, usePlant, useSavePlant } from "../plants/queries";
import { LoadError } from "./LoadError";
import { Pending } from "./Pending";
import styles from "./PlantForm.module.css";

interface Values {
  name: string;
  species: string;
  locationId: string;
  acquiredOn: string;
  notes: string;
}

export function validatePlant(values: Values): { name?: string } {
  return values.name.trim() ? {} : { name: "Give your plant a name." };
}

export function NewPlant() {
  return <PlantForm />;
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
function PlantForm({ plant }: { plant?: Plant }) {
  const [values, setValues] = useState<Values>({
    name: plant?.name ?? "",
    species: plant?.species ?? "",
    locationId: plant?.location_id ?? "",
    acquiredOn: plant?.acquired_on ?? "",
    notes: plant?.notes ?? "",
  });
  const [errors, setErrors] = useState<{ name?: string }>({});
  const [sheetOpen, setSheetOpen] = useState(false);
  const nameRef = useRef<HTMLInputElement>(null);
  const locations = useLocations();
  const save = useSavePlant();
  const archive = useSavePlant();
  const remove = useDeletePlant();
  const navigate = useNavigate();
  const close = useBack(plant ? `/plants/${plant.id}` : "/plants");

  const set = (key: keyof Values) => (e: ChangeEvent<HTMLInputElement | HTMLSelectElement | HTMLTextAreaElement>) =>
    setValues((v) => ({ ...v, [key]: e.target.value }));

  const onSubmit = (e: FormEvent<HTMLFormElement>) => {
    e.preventDefault();
    const found = validatePlant(values);
    setErrors(found);
    if (found.name) {
      nameRef.current?.focus();
      return;
    }
    const body: PlantInput = {
      name: values.name.trim(),
      species: values.species.trim(),
      location_id: values.locationId || null,
      acquired_on: values.acquiredOn || null,
      notes: values.notes,
    };
    if (plant) save.mutate({ id: plant.id, body: { ...body, archived: plant.archived } }, { onSuccess: close });
    else save.mutate({ body }, { onSuccess: (p) => navigate(`/plants/${p.id}`, { replace: true }) });
  };

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
          <TextField
            label="Species"
            hint="Optional, like Monstera deliciosa"
            value={values.species}
            onChange={set("species")}
            maxLength={120}
            autoComplete="off"
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
          {save.isError && <ErrorText>{problemMessage(save.error)}</ErrorText>}
          <Button type="submit" fullWidth loading={save.isPending}>
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
