"use client";

import { create } from "zustand";
import type { User } from "@/shared/types";
import { api } from "@/shared/api";
import { setLoggedInCookie, clearLoggedInCookie } from "./auth-cookie";
import {
  completeFirebaseGoogleRedirectSignIn,
  signInWithFirebaseGoogle,
} from "./firebase";
import type { LoginResponse } from "@/shared/api/client";

interface AuthState {
  user: User | null;
  isLoading: boolean;

  initialize: () => Promise<void>;
  signInWithGoogle: () => Promise<LoginResponse | null>;
  completeGoogleRedirectSignIn: () => Promise<LoginResponse | null>;
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

    try {
      const user = await api.getMe();
      set({ user, isLoading: false });
    } catch {
      api.setToken(null);
      api.setWorkspaceId(null);
      localStorage.removeItem("multica_token");
      localStorage.removeItem("multica_workspace_id");
      set({ user: null, isLoading: false });
    }
  },

  signInWithGoogle: async () => {
    const firebaseToken = await signInWithFirebaseGoogle();
    if (!firebaseToken) {
      return null;
    }
    const { token, user } = await api.loginWithFirebase(firebaseToken);
    localStorage.setItem("multica_token", token);
    api.setToken(token);
    setLoggedInCookie();
    set({ user });
    return { token, user };
  },

  completeGoogleRedirectSignIn: async () => {
    const firebaseToken = await completeFirebaseGoogleRedirectSignIn();
    if (!firebaseToken) {
      return null;
    }
    const { token, user } = await api.loginWithFirebase(firebaseToken);
    localStorage.setItem("multica_token", token);
    api.setToken(token);
    setLoggedInCookie();
    set({ user });
    return { token, user };
  },

  logout: () => {
    localStorage.removeItem("multica_token");
    localStorage.removeItem("multica_workspace_id");
    api.setToken(null);
    api.setWorkspaceId(null);
    clearLoggedInCookie();
    set({ user: null });
  },

  setUser: (user: User) => {
    set({ user });
  },
}));
