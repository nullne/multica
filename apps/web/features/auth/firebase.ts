"use client";

import { getApp, getApps, initializeApp } from "firebase/app";
import {
  getAuth,
  getRedirectResult,
  GoogleAuthProvider,
  signInWithPopup,
  signInWithRedirect,
} from "firebase/auth";

type FirebaseConfig = {
  apiKey: string;
  authDomain: string;
  projectId: string;
  appId: string;
  messagingSenderId?: string;
};

const GOOGLE_REDIRECT_PENDING_KEY = "multica_google_redirect_pending";

export function hasPendingFirebaseGoogleRedirectSignIn(): boolean {
  if (typeof window === "undefined") {
    return false;
  }

  return localStorage.getItem(GOOGLE_REDIRECT_PENDING_KEY) === "1";
}

function firebaseConfig(): FirebaseConfig {
  const apiKey = process.env.NEXT_PUBLIC_FIREBASE_API_KEY;
  const authDomain = process.env.NEXT_PUBLIC_FIREBASE_AUTH_DOMAIN;
  const projectId = process.env.NEXT_PUBLIC_FIREBASE_PROJECT_ID;
  const appId = process.env.NEXT_PUBLIC_FIREBASE_APP_ID;
  const messagingSenderId = process.env.NEXT_PUBLIC_FIREBASE_MESSAGING_SENDER_ID;

  if (!apiKey || !authDomain || !projectId || !appId) {
    throw new Error("Firebase auth is not configured");
  }

  return {
    apiKey,
    authDomain,
    projectId,
    appId,
    messagingSenderId,
  };
}

function firebaseApp() {
  if (getApps().length > 0) return getApp();
  return initializeApp(firebaseConfig());
}

function isStandalonePwa(): boolean {
  if (typeof window === "undefined") {
    return false;
  }

  return (
    window.matchMedia("(display-mode: standalone)").matches ||
    window.matchMedia("(display-mode: fullscreen)").matches ||
    ("standalone" in navigator &&
      Boolean((navigator as Navigator & { standalone?: boolean }).standalone))
  );
}

export async function signInWithFirebaseGoogle(): Promise<string | null> {
  const auth = getAuth(firebaseApp());
  const provider = new GoogleAuthProvider();

  provider.setCustomParameters({ prompt: "select_account" });

  if (isStandalonePwa()) {
    localStorage.setItem(GOOGLE_REDIRECT_PENDING_KEY, "1");
    await signInWithRedirect(auth, provider);
    return null;
  }

  const result = await signInWithPopup(auth, provider);
  const idToken = await result.user.getIdToken();

  await auth.signOut();

  return idToken;
}

export async function completeFirebaseGoogleRedirectSignIn(): Promise<string | null> {
  if (!hasPendingFirebaseGoogleRedirectSignIn()) {
    return null;
  }

  const auth = getAuth(firebaseApp());
  try {
    const result = await getRedirectResult(auth);
    if (!result) {
      return null;
    }

    const idToken = await result.user.getIdToken();
    await auth.signOut();
    return idToken;
  } finally {
    localStorage.removeItem(GOOGLE_REDIRECT_PENDING_KEY);
  }
}
