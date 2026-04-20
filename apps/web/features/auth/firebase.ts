"use client";

import { getApp, getApps, initializeApp } from "firebase/app";
import {
  getAuth,
  GoogleAuthProvider,
  isSignInWithEmailLink,
  sendSignInLinkToEmail,
  signInWithEmailLink,
  signInWithPopup,
} from "firebase/auth";

type FirebaseConfig = {
  apiKey: string;
  authDomain: string;
  projectId: string;
  appId: string;
  messagingSenderId?: string;
};

// Email used in the most recent sendSignInLinkToEmail call. The Firebase docs
// recommend persisting it so we can complete sign-in on the same device
// without prompting again.
const EMAIL_LINK_STORAGE_KEY = "multica_email_link_email";

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

export async function signInWithFirebaseGoogle(): Promise<string> {
  const auth = getAuth(firebaseApp());
  const provider = new GoogleAuthProvider();

  provider.setCustomParameters({ prompt: "select_account" });

  const result = await signInWithPopup(auth, provider);
  const idToken = await result.user.getIdToken();

  await auth.signOut();

  return idToken;
}

// Sends a passwordless sign-in link to the given email. The link points at
// `returnUrl`, which must be on a domain authorized in the Firebase Console.
// Email/Password sign-in method with "Email link (passwordless sign-in)" must
// be enabled in Firebase Console → Authentication → Sign-in method.
export async function sendFirebaseEmailLink(
  email: string,
  returnUrl: string
): Promise<void> {
  const auth = getAuth(firebaseApp());
  await sendSignInLinkToEmail(auth, email, {
    url: returnUrl,
    handleCodeInApp: true,
  });
  if (typeof window !== "undefined") {
    window.localStorage.setItem(EMAIL_LINK_STORAGE_KEY, email);
  }
}

export function isFirebaseEmailLink(url: string): boolean {
  try {
    const auth = getAuth(firebaseApp());
    return isSignInWithEmailLink(auth, url);
  } catch {
    return false;
  }
}

export function getStoredEmailLinkEmail(): string | null {
  if (typeof window === "undefined") return null;
  return window.localStorage.getItem(EMAIL_LINK_STORAGE_KEY);
}

export function clearStoredEmailLinkEmail(): void {
  if (typeof window === "undefined") return;
  window.localStorage.removeItem(EMAIL_LINK_STORAGE_KEY);
}

// Completes a passwordless sign-in started via sendFirebaseEmailLink. The
// returned ID token is verified by the backend the same way Google ID tokens
// are — both share the standard Firebase issuer/audience.
export async function completeFirebaseEmailLinkSignIn(
  email: string,
  url: string
): Promise<string> {
  const auth = getAuth(firebaseApp());
  const result = await signInWithEmailLink(auth, email, url);
  const idToken = await result.user.getIdToken();
  await auth.signOut();
  clearStoredEmailLinkEmail();
  return idToken;
}
