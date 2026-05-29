"use client";

import { create } from "zustand";
import type { User } from "@/shared/types";
import { api } from "@/shared/api";
import { setLoggedInCookie, clearLoggedInCookie } from "./auth-cookie";
import {
  completeFirebaseEmailLinkSignIn,
  completeFirebaseGoogleRedirectSignIn,
  sendFirebaseEmailLink,
  signInWithFirebaseGoogle,
} from "./firebase";
import { isAuthError, type LoginResponse } from "@/shared/api/client";

// Persists tokens from a successful login into localStorage and the api client.
function persistCredentials(res: LoginResponse) {
  localStorage.setItem("multica_token", res.token);
  localStorage.setItem("multica_refresh_token", res.refresh_token);
  api.setToken(res.token);
  api.setRefreshToken(res.refresh_token);
  setLoggedInCookie();
}

interface AuthState {
  user: User | null;
  isLoading: boolean;

  initialize: () => Promise<void>;
  signInWithGoogle: () => Promise<LoginResponse | null>;
  completeGoogleRedirectSignIn: () => Promise<LoginResponse | null>;
  sendEmailSignInLink: (email: string, returnUrl: string) => Promise<void>;
  signInWithEmailLink: (email: string, url: string) => Promise<LoginResponse>;
  signInAsDev: (email: string, name?: string) => Promise<LoginResponse>;
  logout: () => void;
  setUser: (user: User) => void;
}

export const useAuthStore = create<AuthState>((set) => ({
  user: null,
  isLoading: true,

  initialize: async () => {
    const token = localStorage.getItem("multica_token");
    if (!token) {
      set({ isLoading: false });
      return;
    }

    api.setToken(token);
    api.setRefreshToken(localStorage.getItem("multica_refresh_token"));

    try {
      const user = await api.getMe();
      set({ user, isLoading: false });
    } catch (err) {
      // Only drop the session on a genuine auth failure. A transient
      // server/network error must not discard valid credentials.
      if (isAuthError(err)) {
        api.setToken(null);
        api.setRefreshToken(null);
        api.setWorkspaceId(null);
        localStorage.removeItem("multica_token");
        localStorage.removeItem("multica_refresh_token");
        localStorage.removeItem("multica_workspace_id");
        set({ user: null });
      }
      set({ isLoading: false });
    }
  },

  signInWithGoogle: async () => {
    const firebaseToken = await signInWithFirebaseGoogle();
    if (!firebaseToken) {
      return null;
    }
    const res = await api.loginWithFirebase(firebaseToken);
    persistCredentials(res);
    set({ user: res.user });
    return res;
  },

  completeGoogleRedirectSignIn: async () => {
    const firebaseToken = await completeFirebaseGoogleRedirectSignIn();
    if (!firebaseToken) {
      return null;
    }
    const res = await api.loginWithFirebase(firebaseToken);
    persistCredentials(res);
    set({ user: res.user });
    return res;
  },

  sendEmailSignInLink: async (email, returnUrl) => {
    await sendFirebaseEmailLink(email, returnUrl);
  },

  signInWithEmailLink: async (email, url) => {
    const firebaseToken = await completeFirebaseEmailLinkSignIn(email, url);
    const res = await api.loginWithFirebase(firebaseToken);
    persistCredentials(res);
    set({ user: res.user });
    return res;
  },

  signInAsDev: async (email, name) => {
    const res = await api.loginAsDev(email, name);
    persistCredentials(res);
    set({ user: res.user });
    return res;
  },

  logout: () => {
    const refreshToken = localStorage.getItem("multica_refresh_token");
    void api.logout(refreshToken);
    localStorage.removeItem("multica_token");
    localStorage.removeItem("multica_refresh_token");
    localStorage.removeItem("multica_workspace_id");
    api.setToken(null);
    api.setRefreshToken(null);
    api.setWorkspaceId(null);
    clearLoggedInCookie();
    set({ user: null });
  },

  setUser: (user: User) => {
    set({ user });
  },
}));
