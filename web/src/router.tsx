import { Navigate, type RouteObject } from "react-router";
import { RedirectIfSignedIn, RequireAuth } from "./auth/guards";
import { Login } from "./routes/Login";
import { Plants } from "./routes/Plants";
import { Settings } from "./routes/Settings";
import { Today } from "./routes/Today";

export const routes: RouteObject[] = [
  {
    element: <RedirectIfSignedIn />,
    children: [{ path: "/login", element: <Login /> }],
  },
  {
    element: <RequireAuth />,
    children: [
      { path: "/", element: <Today /> },
      { path: "/plants", element: <Plants /> },
      { path: "/settings", element: <Settings /> },
    ],
  },
  { path: "*", element: <Navigate to="/" replace /> },
];
