import { useLocation, useNavigate } from "react-router";

/** Goes back in history, or to `fallback` when the page was opened directly (nothing to go back to). */
export function useBack(fallback: string) {
  const navigate = useNavigate();
  const location = useLocation();
  return () => {
    if (location.key === "default") void navigate(fallback, { replace: true });
    else void navigate(-1);
  };
}
