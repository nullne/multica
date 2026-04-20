"use client";

import { create } from "zustand";
import type { User } from "@/shared/types";
import { api } from "@/shared/api";
import { setLoggedInCookie, clearLoggedInCookie } from "./auth-cookie";
import { signInWithFirebaseGoogle } from "./firebase";
import type { LoginResponse } from "@/shared/api/client";

interface AuthState {
  user: User | null;
  isLoading: boolean;

  initialize: () => Promise<void>;
  signInWithGoogle: () => Promise<LoginResponse>;
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
    const { token, user } = await api.loginWithFirebase(firebaseToken);
    localStorage.setItem("multica_token", token);
    api.setToken(token);
    setLoggedInCookie();
    set({ user });
    return { token, user };
  },

  signInAsDev: async (email, name) => {
    const { token, user } = await api.loginAsDev(email, name);
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
