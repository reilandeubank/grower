import type { Me } from "../api/client";
import { useMe } from "./hooks";

/** For screens under <RequireAuth>, where /me has already resolved to a user. */
export function useCurrentUser(): Me {
  const { data } = useMe();
  if (!data) throw new Error("useCurrentUser used outside RequireAuth");
  return data;
}
