"use client";

import { useEffect, useRef, useState } from "react";
import { useRouter } from "next/navigation";
import { useAuthStore } from "@/features/auth";
import {
  getStoredEmailLinkEmail,
  hasPendingFirebaseGoogleRedirectSignIn,
  isFirebaseEmailLink,
} from "@/features/auth/firebase";
import { useWorkspaceStore } from "@/features/workspace";
import { api } from "@/shared/api";
import { createLogger } from "@/shared/logger";
import {
  Card,
  CardHeader,
  CardTitle,
  CardDescription,
  CardContent,
} from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import type { User } from "@/shared/types";

const logger = createLogger("login-page");

type LoginPageClientProps = {
  cliCallback: string | null;
  cliState: string;
  nextPath: string;
};

function validateCliCallback(cliCallback: string): boolean {
  try {
    const cbUrl = new URL(cliCallback);
    if (cbUrl.protocol !== "http:") return false;
    if (cbUrl.hostname !== "localhost" && cbUrl.hostname !== "127.0.0.1") {
      return false;
    }
    return true;
  } catch {
    return false;
  }
}

function redirectToCliCallback(
  cliCallback: string,
  token: string,
  cliState: string
) {
  const separator = cliCallback.includes("?") ? "&" : "?";
  window.location.href = `${cliCallback}${separator}token=${encodeURIComponent(token)}&state=${encodeURIComponent(cliState)}`;
}

// When set (only via docker-compose.dev.yml), the login page shows a
// one-click dev login that calls /auth/dev. The backend gates that endpoint
// behind DEV_AUTH_BYPASS=1, so this is inert in prod.
const DEV_LOGIN_EMAIL = process.env.NEXT_PUBLIC_DEV_EMAIL ?? "";

// When the Firebase web config is also present, the Firebase buttons stay
// visible alongside the dev button so both flows can be exercised in dev.
const FIREBASE_CONFIGURED = Boolean(process.env.NEXT_PUBLIC_FIREBASE_API_KEY);

export function LoginPageClient({
  cliCallback,
  cliState,
  nextPath,
}: LoginPageClientProps) {
  const router = useRouter();
  const user = useAuthStore((s) => s.user);
  const isLoading = useAuthStore((s) => s.isLoading);
  const signInWithGoogle = useAuthStore((s) => s.signInWithGoogle);
  const completeGoogleRedirectSignIn = useAuthStore(
    (s) => s.completeGoogleRedirectSignIn
  );
  const signInAsDev = useAuthStore((s) => s.signInAsDev);
  const sendEmailSignInLink = useAuthStore((s) => s.sendEmailSignInLink);
  const signInWithEmailLink = useAuthStore((s) => s.signInWithEmailLink);
  const hydrateWorkspace = useWorkspaceStore((s) => s.hydrateWorkspace);

  const [error, setError] = useState("");
  const [submitting, setSubmitting] = useState(false);
  const [existingUser, setExistingUser] = useState<User | null>(null);
  const [email, setEmail] = useState("");
  const [emailLinkSent, setEmailLinkSent] = useState(false);
  const emailLinkProcessed = useRef(false);

  useEffect(() => {
    if (!isLoading && user && !cliCallback) {
      router.replace(nextPath);
    }
  }, [cliCallback, isLoading, nextPath, router, user]);

  useEffect(() => {
    let cancelled = false;

    const finishRedirectSignIn = async () => {
      if (!hasPendingFirebaseGoogleRedirectSignIn()) {
        return;
      }

      logger.info("detected pending Google redirect sign-in");
      setSubmitting(true);

      try {
        const login = await completeGoogleRedirectSignIn();
        if (!login || cancelled) {
          if (!cancelled) {
            logger.warn("Google redirect sign-in produced no login result");
          }
          return;
        }

        logger.info("Google redirect sign-in completed");

        if (cliCallback) {
          if (!validateCliCallback(cliCallback)) {
            setError("Invalid callback URL");
            return;
          }
          redirectToCliCallback(cliCallback, login.token, cliState);
          return;
        }

        const wsList = await api.listWorkspaces();
        if (cancelled) {
          return;
        }
        await hydrateWorkspace(wsList);
        logger.info("workspace hydration scheduled after Google redirect sign-in");
        router.push(nextPath);
      } catch (err) {
        logger.error("Google redirect sign-in failed", err);
        if (!cancelled) {
          setError(
            err instanceof Error ? err.message : "Failed to sign in with Google"
          );
        }
      } finally {
        if (!cancelled) {
          setSubmitting(false);
        }
      }
    };

    void finishRedirectSignIn();

    return () => {
      cancelled = true;
    };
  }, [
    cliCallback,
    cliState,
    completeGoogleRedirectSignIn,
    hydrateWorkspace,
    nextPath,
    router,
  ]);

  useEffect(() => {
    if (!cliCallback) return;

    const token = localStorage.getItem("multica_token");
    if (!token || !validateCliCallback(cliCallback)) return;

    api.setToken(token);
    api
      .getMe()
      .then((loggedInUser) => {
        setExistingUser(loggedInUser);
      })
      .catch(() => {
        api.setToken(null);
        localStorage.removeItem("multica_token");
      });
  }, [cliCallback]);

  const handleCliAuthorize = async () => {
    const token = localStorage.getItem("multica_token");
    if (!cliCallback || !token) return;

    setSubmitting(true);
    redirectToCliCallback(cliCallback, token, cliState);
  };

  const handleSignIn = async (
    signIn: () => Promise<{ token: string } | null>
  ) => {
    setError("");
    setSubmitting(true);

    try {
      const login = await signIn();
      if (!login) {
        return;
      }

      if (cliCallback) {
        if (!validateCliCallback(cliCallback)) {
          setError("Invalid callback URL");
          return;
        }
        redirectToCliCallback(cliCallback, login.token, cliState);
        return;
      }

      const wsList = await api.listWorkspaces();
      await hydrateWorkspace(wsList);
      router.push(nextPath);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to sign in");
    } finally {
      setSubmitting(false);
    }
  };

  // When the user clicks the email link, Firebase appends an oobCode to the
  // current URL. Detect it and complete the sign-in. Email is recovered from
  // localStorage when on the same device, otherwise we prompt for it.
  useEffect(() => {
    if (!FIREBASE_CONFIGURED) return;
    if (emailLinkProcessed.current) return;
    if (typeof window === "undefined") return;
    if (!isFirebaseEmailLink(window.location.href)) return;

    emailLinkProcessed.current = true;
    const url = window.location.href;
    const stored = getStoredEmailLinkEmail();
    const targetEmail =
      stored ?? window.prompt("Please confirm the email you used to sign in")?.trim();
    if (!targetEmail) {
      setError("Email is required to complete sign-in");
      return;
    }

    void handleSignIn(() => signInWithEmailLink(targetEmail, url));
    // handleSignIn is intentionally not in deps - it changes on every render.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [signInWithEmailLink]);

  const handleGoogleSignIn = () => handleSignIn(() => signInWithGoogle());
  const handleDevSignIn = () =>
    handleSignIn(() => signInAsDev(DEV_LOGIN_EMAIL, "Dev User"));

  const handleSendEmailLink = async (e: React.FormEvent) => {
    e.preventDefault();
    const trimmed = email.trim();
    if (!trimmed) {
      setError("Email is required");
      return;
    }
    setError("");
    setSubmitting(true);
    try {
      // Firebase requires an authorized return URL. We send the user back to
      // /login so the email-link callback effect above can finish sign-in.
      const returnUrl = `${window.location.origin}/login`;
      await sendEmailSignInLink(trimmed, returnUrl);
      setEmailLinkSent(true);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to send sign-in link");
    } finally {
      setSubmitting(false);
    }
  };

  if (existingUser) {
    return (
      <div className="flex min-h-screen items-center justify-center">
        <Card className="w-full max-w-sm">
          <CardHeader className="text-center">
            <CardTitle className="text-2xl">Authorize CLI</CardTitle>
            <CardDescription>
              Allow the CLI to access Multica as{" "}
              <span className="font-medium text-foreground">
                {existingUser.email}
              </span>
              ?
            </CardDescription>
          </CardHeader>
          <CardContent className="flex flex-col gap-3">
            <Button
              onClick={handleCliAuthorize}
              disabled={submitting}
              className="w-full"
              size="lg"
            >
              {submitting ? "Authorizing..." : "Authorize"}
            </Button>
            <Button
              variant="ghost"
              className="w-full"
              onClick={() => {
                setExistingUser(null);
              }}
            >
              Use a different account
            </Button>
          </CardContent>
        </Card>
      </div>
    );
  }

  if (emailLinkSent) {
    return (
      <div className="flex min-h-screen items-center justify-center">
        <Card className="w-full max-w-sm">
          <CardHeader className="text-center">
            <CardTitle className="text-2xl">Check your inbox</CardTitle>
            <CardDescription>
              We sent a sign-in link to{" "}
              <span className="font-medium text-foreground">{email}</span>. Open
              it on this device to finish signing in.
            </CardDescription>
          </CardHeader>
          <CardContent className="flex flex-col gap-3">
            <Button
              variant="ghost"
              className="w-full"
              onClick={() => {
                setEmailLinkSent(false);
                setError("");
              }}
            >
              Use a different email
            </Button>
          </CardContent>
        </Card>
      </div>
    );
  }

  const showDevLogin = Boolean(DEV_LOGIN_EMAIL);
  const showFirebase = FIREBASE_CONFIGURED || !DEV_LOGIN_EMAIL;
  const helperCopy = showDevLogin
    ? "Dev auth bypass - only enabled when DEV_AUTH_BYPASS=1."
    : "Sign in with Google or get a one-time link by email.";

  return (
    <div className="flex min-h-screen items-center justify-center">
      <Card className="w-full max-w-sm">
        <CardHeader className="text-center">
          <CardTitle className="text-2xl">Multica</CardTitle>
          <CardDescription>
            Turn coding agents into real teammates
          </CardDescription>
        </CardHeader>
        <CardContent className="space-y-4">
          {showDevLogin ? (
            <Button
              onClick={handleDevSignIn}
              disabled={submitting}
              className="w-full"
              size="lg"
            >
              {submitting
                ? "Signing in..."
                : `Continue as ${DEV_LOGIN_EMAIL} (dev)`}
            </Button>
          ) : null}

          {showFirebase ? (
            <>
              <Button
                onClick={handleGoogleSignIn}
                disabled={submitting}
                variant={DEV_LOGIN_EMAIL ? "outline" : "default"}
                className="w-full"
                size="lg"
              >
                {submitting ? "Signing in..." : "Continue with Google"}
              </Button>

              <div className="flex items-center gap-3 text-xs text-muted-foreground">
                <div className="h-px flex-1 bg-border" />
                <span>or</span>
                <div className="h-px flex-1 bg-border" />
              </div>

              <form onSubmit={handleSendEmailLink} className="space-y-2">
                <Input
                  type="email"
                  placeholder="you@example.com"
                  value={email}
                  onChange={(e) => setEmail(e.target.value)}
                  disabled={submitting}
                  autoComplete="email"
                  required
                />
                <Button
                  type="submit"
                  variant="outline"
                  disabled={submitting}
                  className="w-full"
                  size="lg"
                >
                  {submitting ? "Sending..." : "Email me a sign-in link"}
                </Button>
              </form>
            </>
          ) : null}

          <p className="text-center text-sm text-muted-foreground">
            {helperCopy}
          </p>
          {error ? <p className="text-sm text-destructive">{error}</p> : null}
        </CardContent>
      </Card>
    </div>
  );
}
