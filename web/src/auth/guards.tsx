import { Navigate, Outlet, useLocation, useSearchParams } from "react-router";
import { useMe, useSetupStatus } from "./hooks";
import { Pending } from "../routes/Pending";
import { Unreachable } from "../routes/Unreachable";

/**
 * Wraps signed-in routes: sends signed-out visitors to /login?next=<where they were going>, or to
 * /setup while the first account still needs creating.
 */
export function RequireAuth() {
  const me = useMe();
  const setup = useSetupStatus(me.data === null);
  const location = useLocation();
  if (me.isPending) return <Pending />;
  if (me.isError) return <Unreachable onRetry={() => void me.refetch()} />;
  if (!me.data) {
    if (setup.isPending) return <Pending />;
    if (setup.data?.needed) return <Navigate to="/setup" replace />;
    const next = location.pathname + location.search;
    const to = next === "/" ? "/login" : `/login?next=${encodeURIComponent(next)}`;
    return <Navigate to={to} replace />;
  }
  return <Outlet />;
}

/** Wraps /login: already signed in means there is nothing to do here. */
export function RedirectIfSignedIn() {
  const me = useMe();
  const setup = useSetupStatus(me.data === null);
  const [params] = useSearchParams();
  if (me.isPending) return <Pending />;
  if (me.data) return <Navigate to={safeNext(params.get("next"))} replace />;
  if (setup.isPending) return <Pending />;
  if (setup.data?.needed) return <Navigate to="/setup" replace />;
  return <Outlet />;
}

/** Wraps /setup: only reachable while signed out and before the first account exists. */
export function RequireSetupNeeded() {
  const me = useMe();
  const setup = useSetupStatus(me.data === null);
  if (me.isPending) return <Pending />;
  if (me.data) return <Navigate to="/" replace />;
  if (setup.isPending) return <Pending />;
  if (!setup.data?.needed) return <Navigate to="/login" replace />;
  return <Outlet />;
}

/** Only same-origin paths are allowed as a post-login destination. */
export function safeNext(next: string | null): string {
  if (!next || !next.startsWith("/") || next.startsWith("//") || next.startsWith("/\\")) return "/";
  return next;
}
