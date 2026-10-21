import { Droplet, Gauge, NotebookPen, RotateCw, Shovel, Sparkles, SprayCan, Sprout, type LucideIcon } from "lucide-react";
import styles from "./TaskIcon.module.css";

export type CareKind = "water" | "fertilize" | "mist" | "repot" | "rotate" | "custom" | "check_in";

const KINDS: Record<CareKind, { icon: LucideIcon; name: string }> = {
  water: { icon: Droplet, name: "Water" },
  fertilize: { icon: Sprout, name: "Fertilize" },
  mist: { icon: SprayCan, name: "Mist" },
  repot: { icon: Shovel, name: "Repot" },
  rotate: { icon: RotateCw, name: "Rotate" },
  custom: { icon: Sparkles, name: "Custom" },
  check_in: { icon: NotebookPen, name: "Check in" },
};

/** "Water", "Fertilize", … the label of a custom task, or "Check soil" for a moisture-check water task. */
export function taskName(kind: CareKind, label = "", check = false): string {
  if (check) return "Check soil";
  return kind === "custom" ? label.trim() || KINDS.custom.name : KINDS[kind].name;
}

export interface TaskIconProps {
  kind: CareKind;
  size?: number;
  /** A soil check: the meter's gauge, in the water accent. */
  check?: boolean;
}

/** The care type's icon in its accent colour. Decorative: the task name always sits beside it. */
export function TaskIcon({ kind, size = 20, check = false }: TaskIconProps) {
  const Icon = check ? Gauge : KINDS[kind].icon;
  return <Icon className={styles[check ? "water" : kind]} size={size} strokeWidth={1.75} aria-hidden="true" />;
}
