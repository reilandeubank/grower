import { NavLink } from "react-router";
import { CalendarCheck, Settings, Sprout, type LucideIcon } from "lucide-react";
import { Logo } from "./Logo";
import styles from "./BottomTabBar.module.css";

interface Tab {
  to: string;
  label: string;
  icon: LucideIcon;
}

const TABS: Tab[] = [
  { to: "/", label: "Today", icon: CalendarCheck },
  { to: "/plants", label: "Plants", icon: Sprout },
  { to: "/settings", label: "Settings", icon: Settings },
];

/** Bottom tab bar on phones; a left rail from 900px up (see the CSS). */
export function BottomTabBar() {
  return (
    <nav className={styles.bar} aria-label="Main">
      <div className={styles.railLogo}>
        <Logo size={40} />
      </div>
      <ul className={styles.list}>
        {TABS.map(({ to, label, icon: Icon }) => (
          <li key={to} className={styles.item}>
            <NavLink to={to} end={to === "/"} className={({ isActive }) => `${styles.tab} ${isActive ? styles.active : ""}`}>
              <span className={styles.pill} aria-hidden="true">
                <Icon size={24} strokeWidth={1.75} />
              </span>
              <span className={`t-caption ${styles.label}`}>{label}</span>
            </NavLink>
          </li>
        ))}
      </ul>
    </nav>
  );
}
