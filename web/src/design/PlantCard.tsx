import { Link } from "react-router";
import styles from "./PlantCard.module.css";

export interface PlantPhotoProps {
  name: string;
  /** `thumb` is the small square in task rows. */
  size?: "card" | "hero" | "thumb";
}

const INITIAL_TYPE = { card: "t-title", hero: "t-display", thumb: "t-body-strong" } as const;

/** The photo slot. Until photos arrive it is a calm brand-soft tile with the plant's initial. */
export function PlantPhoto({ name, size = "card" }: PlantPhotoProps) {
  const initial = Array.from(name.trim())[0]?.toUpperCase() ?? "";
  return (
    <div className={`${styles.photo} ${styles[size]}`} aria-hidden="true">
      <span className={INITIAL_TYPE[size]}>{initial}</span>
    </div>
  );
}

export interface PlantCardProps {
  to: string;
  name: string;
  species?: string;
  /** Location name, shown as an uppercase tag. */
  location?: string | undefined;
}

export function PlantCard({ to, name, species, location }: PlantCardProps) {
  return (
    <Link to={to} className={styles.card}>
      <PlantPhoto name={name} />
      <span className={styles.text}>
        <span className={`t-body-strong ${styles.clamp}`}>{name}</span>
        {species && <span className={`t-caption ${styles.muted} ${styles.clamp}`}>{species}</span>}
        {location && <span className={`t-label ${styles.muted} ${styles.clamp}`}>{location}</span>}
      </span>
    </Link>
  );
}
