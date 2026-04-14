"use client";

import { useEffect, type ReactNode } from "react";
import { useAuthStore } from "./store";
import { useWorkspaceStore } from "@/features/workspace";
import { api } from "@/shared/api";
import { createLogger } from "@/shared/logger";
import { setLoggedInCookie, clearLoggedInCookie } from "./auth-cookie";

const logger = createLogger("auth");

function hydrateWithToken(token: string) {
  api.setToken(token);
  const wsId = localStorage.getItem("multica_workspace_id");

  const mePromise = api.getMe();
  const wsPromise = api.listWorkspaces();

  Promise.all([mePromise, wsPromise])
    .then(([user, wsList]) => {
      setLoggedInCookie();
      useAuthStore.setState({ user, isLoading: false });
      useWorkspaceStore.getState().hydrateWorkspace(wsList, wsId);
    })
    .catch((err) => {
      logger.error("auth init failed", err);
      api.setToken(null);
      api.setWorkspaceId(null);
      localStorage.removeItem("multica_token");
      localStorage.removeItem("multica_workspace_id");
      clearLoggedInCookie();
      useAuthStore.setState({ user: null, isLoading: false });
    });
}

/**
 * Initializes auth + workspace state on mount.
 *
 * 1. If a cached multica_token exists in localStorage, use it directly.
 * 2. Otherwise, attempt to exchange the gateway cookie (_claw_auth) for a
 *    multica JWT via POST /auth/gateway-login (sent with credentials so the
 *    httpOnly cookie is included).
 * 3. If both fail, mark loading as done — the middleware or dashboard layout
 *    will handle the redirect.
 */
export function AuthInitializer({ children }: { children: ReactNode }) {
  useEffect(() => {
    const token = localStorage.getItem("multica_token");
    if (token) {
      hydrateWithToken(token);
      return;
    }

    // No cached token — try exchanging the gateway cookie.
    api
      .gatewayLogin()
      .then(({ token: newToken }) => {
        localStorage.setItem("multica_token", newToken);
        hydrateWithToken(newToken);
      })
      .catch(() => {
        clearLoggedInCookie();
        useAuthStore.setState({ isLoading: false });
      });
  }, []);

  return <>{children}</>;
}
