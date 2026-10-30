import { Navigate, type RouteObject } from "react-router";
import {
  RedirectIfSignedIn,
  RequireAuth,
  RequireSetupNeeded,
} from "./auth/guards";
import { ArchivedPlants } from "./routes/ArchivedPlants";
import { Locations } from "./routes/Locations";
import { Login } from "./routes/Login";
import { PlantDetail } from "./routes/PlantDetail";
import { EditPlant, NewPlant } from "./routes/PlantForm";
import { Plants } from "./routes/Plants";
import { Settings } from "./routes/Settings";
import { Setup } from "./routes/Setup";
import { Today } from "./routes/Today";

export const routes: RouteObject[] = [
  {
    element: <RedirectIfSignedIn />,
    children: [{ path: "/login", element: <Login /> }],
  },
  {
    element: <RequireSetupNeeded />,
    children: [{ path: "/setup", element: <Setup /> }],
  },
  {
    element: <RequireAuth />,
    children: [
      { path: "/", element: <Today /> },
      { path: "/plants", element: <Plants /> },
      { path: "/plants/new", element: <NewPlant /> },
      { path: "/plants/archived", element: <ArchivedPlants /> },
      { path: "/plants/:id", element: <PlantDetail /> },
      { path: "/plants/:id/edit", element: <EditPlant /> },
      { path: "/settings", element: <Settings /> },
      { path: "/settings/locations", element: <Locations /> },
    ],
  },
  { path: "*", element: <Navigate to="/" replace /> },
];
