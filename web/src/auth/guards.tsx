import { Navigate, Outlet, useLocation, useSearchParams } from "react-router";
import { useMe } from "./hooks";
import { Pending } from "../routes/Pending";
import { Unreachable } from "../routes/Unreachable";

/** Wraps signed-in routes: sends signed-out visitors to /login?next=<where they were going>. */
export function RequireAuth() {
  const me = useMe();
  const location = useLocation();
  if (me.isPending) return <Pending />;
  if (me.isError) return <Unreachable onRetry={() => void me.refetch()} />;
  if (!me.data) {
    const next = location.pathname + location.search;
    const to = next === "/" ? "/login" : `/login?next=${encodeURIComponent(next)}`;
    return <Navigate to={to} replace />;
  }
  return <Outlet />;
}

/** Wraps /login: already signed in means there is nothing to do here. */
export function RedirectIfSignedIn() {
  const me = useMe();
  const [params] = useSearchParams();
  if (me.isPending) return <Pending />;
  if (me.data) return <Navigate to={safeNext(params.get("next"))} replace />;
  return <Outlet />;
}

/** Only same-origin paths are allowed as a post-login destination. */
export function safeNext(next: string | null): string {
  if (!next || !next.startsWith("/") || next.startsWith("//") || next.startsWith("/\\")) return "/";
  return next;
}
