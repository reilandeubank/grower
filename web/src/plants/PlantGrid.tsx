import type { Location, Plant } from "../api/client";
import { PlantCard } from "../design";
import styles from "./PlantGrid.module.css";

export function PlantGrid({ plants, locations = [] }: { plants: Plant[]; locations?: Location[] }) {
  const locationNames = new Map(locations.map((l) => [l.id, l.name]));
  return (
    <ul className={styles.grid}>
      {plants.map((p) => (
        <li key={p.id}>
          <PlantCard
            to={`/plants/${p.id}`}
            name={p.name}
            species={p.species}
            location={p.location_id ? locationNames.get(p.location_id) : undefined}
            photo={p.cover?.urls.small}
          />
        </li>
      ))}
    </ul>
  );
}
