import { useEffect, useRef } from "react";

/** The history.state key holding an overlay's depth: 1, or 2 for an overlay opened over another. */
const KEY = "overlay";

function depthOf(state: unknown): number {
  const depth = (state as Record<string, unknown> | null)?.[KEY];
  return typeof depth === "number" ? depth : 0;
}

/**
 * Lets the Android back button close an overlay instead of leaving the page.
 *
 * Opening pushes a history entry for the same URL, copying the router's state, so React Router
 * sees Back as a pop to where it already is. Back then calls `onClose`. Closing from the UI pops
 * the entry itself, so history ends up as it was.
 *
 * To leave the page from an open overlay, navigate with `replace: true`: the new page takes the
 * overlay's entry, and Back from it returns to the page under the overlay.
 */
export function useCloseOnBack(open: boolean, onClose: () => void) {
  const onCloseRef = useRef(onClose);
  useEffect(() => {
    onCloseRef.current = onClose;
  });
  const pendingPop = useRef<number | undefined>(undefined);

  useEffect(() => {
    if (!open) return;
    let depth: number;
    if (pendingPop.current !== undefined) {
      // StrictMode ran the effect twice: keep the entry the first run pushed.
      window.clearTimeout(pendingPop.current);
      pendingPop.current = undefined;
      depth = depthOf(window.history.state);
    } else {
      depth = depthOf(window.history.state) + 1;
      window.history.pushState({ ...window.history.state, [KEY]: depth }, "");
    }

    // Only the top overlay closes: Back over a sheet in the photo viewer closes just the sheet.
    const onPop = () => {
      if (depthOf(window.history.state) < depth) onCloseRef.current();
    };
    window.addEventListener("popstate", onPop);
    return () => {
      window.removeEventListener("popstate", onPop);
      // A task later, so a navigation started with the close has written history first. If our
      // entry is still current, the close came from the UI and the entry has to go.
      pendingPop.current = window.setTimeout(() => {
        pendingPop.current = undefined;
        if (depthOf(window.history.state) === depth) window.history.back();
      });
    };
  }, [open]);
}
