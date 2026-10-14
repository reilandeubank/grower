import { WifiOff } from "lucide-react";
import { useOnline } from "../lib/online";
import styles from "./OfflineBanner.module.css";

/** A calm note at the top of the page while the device is offline; screens show cached data. */
export function OfflineBanner() {
  const online = useOnline();
  // The live region stays mounted so going offline is announced.
  return (
    <div role="status" className={online ? undefined : styles.banner}>
      {!online && (
        <>
          <WifiOff size={20} strokeWidth={1.75} aria-hidden="true" />
          <p className="t-caption">You're offline. Showing what was loaded last.</p>
        </>
      )}
    </div>
  );
}
