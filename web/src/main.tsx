import { StrictMode } from "react";
import { createRoot } from "react-dom/client";
import { QueryClientProvider } from "@tanstack/react-query";
import { createBrowserRouter } from "react-router";
import { RouterProvider } from "react-router/dom";
import { ToastProvider } from "./design";
import { OfflineToast } from "./lib/OfflineToast";
import { applyTheme, readTheme } from "./lib/theme";
import { createQueryClient } from "./queryClient";
import { routes } from "./router";
import "./design/global.css";

// public/theme.js already set the theme before paint; this also sets the theme-color meta.
applyTheme(readTheme());

const queryClient = createQueryClient();
const router = createBrowserRouter(routes);

const root = document.getElementById("root");
if (!root) throw new Error("missing #root");

createRoot(root).render(
  <StrictMode>
    <QueryClientProvider client={queryClient}>
      <ToastProvider>
        <OfflineToast />
        <RouterProvider router={router} />
      </ToastProvider>
    </QueryClientProvider>
  </StrictMode>,
);
