"use client";

import { useEffect, type ReactNode } from "react";
import { useAuthStore } from "./store";
import { useWorkspaceStore } from "@/features/workspace";
import { api } from "@/shared/api";
import { isAuthError } from "@/shared/api/client";
import { createLogger } from "@/shared/logger";
import { setLoggedInCookie, clearLoggedInCookie } from "./auth-cookie";

const logger = createLogger("auth");

/**
 * Initializes auth + workspace state from localStorage on mount.
 * Fires getMe() and listWorkspaces() in parallel when a cached token exists.
 */
export function AuthInitializer({ children }: { children: ReactNode }) {
  useEffect(() => {
    const token = localStorage.getItem("multica_token");
    if (!token) {
      clearLoggedInCookie();
      useAuthStore.setState({ isLoading: false });
      return;
    }

    api.setToken(token);
    api.setRefreshToken(localStorage.getItem("multica_refresh_token"));
    const wsId = localStorage.getItem("multica_workspace_id");

    // Fire getMe and listWorkspaces in parallel
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
        // Only clear credentials on a real auth failure (the access token
        // expired and the refresh token could not renew it). Transient
        // server/network errors must keep the session so a later reload
        // restores it instead of forcing a re-login.
        if (isAuthError(err)) {
          api.setToken(null);
          api.setRefreshToken(null);
          api.setWorkspaceId(null);
          localStorage.removeItem("multica_token");
          localStorage.removeItem("multica_refresh_token");
          localStorage.removeItem("multica_workspace_id");
          clearLoggedInCookie();
          useAuthStore.setState({ user: null });
        }
        useAuthStore.setState({ isLoading: false });
      });
  }, []);

  return <>{children}</>;
}
