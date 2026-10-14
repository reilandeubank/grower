import { useNavigate, useParams } from "react-router";
import { Archive, ArrowLeft, CalendarClock, Camera, SearchX } from "lucide-react";
import { ApiError, problemMessage } from "../api/client";
import { AppShell, Button, EmptyState, ErrorText, IconButton, PlantPhoto, Tabs, TopBar } from "../design";
import { formatMonthYear } from "../lib/dates";
import { useBack } from "../lib/useBack";
import { useLocations } from "../locations/queries";
import { plantUpdate, usePlant, useSavePlant } from "../plants/queries";
import { LoadError } from "./LoadError";
import { Pending } from "./Pending";
import styles from "./PlantDetail.module.css";

export function PlantDetail() {
  const { id = "" } = useParams();
  const plant = usePlant(id);
  const locations = useLocations();
  const restore = useSavePlant();
  const navigate = useNavigate();
  const back = useBack("/plants");

  const backButton = <IconButton aria-label="Back" icon={<ArrowLeft size={20} strokeWidth={1.75} />} onClick={back} />;

  if (plant.isPending) {
    return (
      <AppShell topBar={<TopBar title="Plant" leading={backButton} />}>
        <Pending inline />
      </AppShell>
    );
  }

  if (plant.isError) {
    const notFound = plant.error instanceof ApiError && plant.error.status === 404;
    return (
      <AppShell topBar={<TopBar title="Plant" leading={backButton} />}>
        {notFound ? (
          <EmptyState
            icon={<SearchX size={28} strokeWidth={1.75} />}
            title="Plant not found"
            body="It may have been deleted."
            action={
              <Button variant="secondary" onClick={() => navigate("/plants")}>
                Back to plants
              </Button>
            }
          />
        ) : (
          <LoadError onRetry={() => void plant.refetch()} />
        )}
      </AppShell>
    );
  }

  const p = plant.data;
  const location = locations.data?.find((l) => l.id === p.location_id);

  return (
    <AppShell
      topBar={
        <TopBar
          leading={backButton}
          actions={
            <Button variant="ghost" onClick={() => navigate(`/plants/${p.id}/edit`)}>
              Edit
            </Button>
          }
        />
      }
    >
      <div className={styles.page}>
        <PlantPhoto name={p.name} size="hero" />
        <div className={styles.heading}>
          <h1 className="t-title">{p.name}</h1>
          {p.species && <p className={`t-caption ${styles.muted}`}>{p.species}</p>}
          {(location || p.acquired_on) && (
            <p className={styles.meta}>
              {location && <span className="t-label">{location.name}</span>}
              {p.acquired_on && <span className="t-caption">Since {formatMonthYear(p.acquired_on)}</span>}
            </p>
          )}
        </div>

        {p.archived && (
          <div className={styles.archived}>
            <span className={`t-caption ${styles.chip}`}>
              <Archive size={16} strokeWidth={1.75} aria-hidden="true" />
              Archived
            </span>
            <Button
              variant="secondary"
              loading={restore.isPending}
              onClick={() => restore.mutate({ id: p.id, body: plantUpdate(p, { archived: false }) })}
            >
              Restore
            </Button>
          </div>
        )}
        {restore.isError && (
          <ErrorText>{problemMessage(restore.error)}</ErrorText>
        )}

        <Tabs
          label="Plant details"
          tabs={[
            {
              label: "Care",
              panel: (
                <EmptyState
                  as="h3"
                  icon={<CalendarClock size={28} strokeWidth={1.75} />}
                  title="No care schedule yet"
                  body="Care schedules arrive in the next update."
                />
              ),
            },
            {
              label: "Photos",
              panel: (
                <EmptyState
                  as="h3"
                  icon={<Camera size={28} strokeWidth={1.75} />}
                  title="No photos yet"
                  body="Photos arrive in a later update."
                />
              ),
            },
            {
              label: "Notes",
              panel: p.notes ? (
                <p className={`t-body ${styles.notes}`}>{p.notes}</p>
              ) : (
                <p className={`t-body ${styles.muted}`}>No notes yet.</p>
              ),
            },
          ]}
        />
      </div>
    </AppShell>
  );
}
