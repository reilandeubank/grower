import { Link } from "react-router";
import styles from "./PlantCard.module.css";

export interface PlantPhotoProps {
  name: string;
  /** The cover image URL; without it, a calm brand-soft tile with the plant's initial. */
  src?: string | undefined;
  /** `thumb` is the small square in task rows. */
  size?: "card" | "hero" | "thumb";
}

const INITIAL_TYPE = { card: "t-title", hero: "t-display", thumb: "t-body-strong" } as const;
// Not styles[size]: `.card` is the PlantCard link's own class.
const SIZE_CLASS = { card: styles.cardPhoto, hero: styles.hero, thumb: styles.thumb };

/** The plant's cover photo, or its initial until it has one. */
export function PlantPhoto({ name, src, size = "card" }: PlantPhotoProps) {
  if (src) {
    // Cards and rows print the name beside the photo, so only the hero needs alt text.
    const lazy = size !== "hero";
    // The frame keeps the slot's shape whatever the image's own; brand-soft shows while it loads.
    return (
      <div className={`${styles.photo} ${SIZE_CLASS[size]}`}>
        <img className={styles.image} src={src} alt={lazy ? "" : name} loading={lazy ? "lazy" : undefined} decoding="async" />
      </div>
    );
  }
  const initial = Array.from(name.trim())[0]?.toUpperCase() ?? "";
  return (
    <div className={`${styles.photo} ${SIZE_CLASS[size]}`} aria-hidden="true">
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
  /** The cover's small image URL. */
  photo?: string | undefined;
}

export function PlantCard({ to, name, species, location, photo }: PlantCardProps) {
  return (
    <Link to={to} className={styles.card}>
      <PlantPhoto name={name} src={photo} />
      <span className={styles.text}>
        <span className={`t-body-strong ${styles.clamp}`}>{name}</span>
        {species && <span className={`t-caption ${styles.muted} ${styles.clamp}`}>{species}</span>}
        {location && <span className={`t-label ${styles.muted} ${styles.clamp}`}>{location}</span>}
      </span>
    </Link>
  );
}
