"use client";

import { Suspense, useEffect, useState } from "react";
import { useSearchParams, useRouter } from "next/navigation";
import { useAuthStore } from "@/features/auth";
import { useWorkspaceStore } from "@/features/workspace";
import { api } from "@/shared/api";
import {
  Card,
  CardHeader,
  CardTitle,
  CardDescription,
  CardContent,
} from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import type { User } from "@/shared/types";

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

function LoginPageContent() {
  const router = useRouter();
  const user = useAuthStore((s) => s.user);
  const isLoading = useAuthStore((s) => s.isLoading);
  const signInWithGoogle = useAuthStore((s) => s.signInWithGoogle);
  const hydrateWorkspace = useWorkspaceStore((s) => s.hydrateWorkspace);
  const searchParams = useSearchParams();

  const [error, setError] = useState("");
  const [submitting, setSubmitting] = useState(false);
  const [existingUser, setExistingUser] = useState<User | null>(null);

  useEffect(() => {
    if (!isLoading && user && !searchParams.get("cli_callback")) {
      router.replace(searchParams.get("next") || "/issues");
    }
  }, [isLoading, user, router, searchParams]);

  useEffect(() => {
    const cliCallback = searchParams.get("cli_callback");
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
  }, [searchParams]);

  const handleCliAuthorize = async () => {
    const cliCallback = searchParams.get("cli_callback");
    const token = localStorage.getItem("multica_token");
    if (!cliCallback || !token) return;

    const cliState = searchParams.get("cli_state") || "";
    setSubmitting(true);
    redirectToCliCallback(cliCallback, token, cliState);
  };

  const handleGoogleSignIn = async () => {
    setError("");
    setSubmitting(true);

    try {
      const { token } = await signInWithGoogle();
      const cliCallback = searchParams.get("cli_callback");

      if (cliCallback) {
        if (!validateCliCallback(cliCallback)) {
          setError("Invalid callback URL");
          return;
        }
        redirectToCliCallback(cliCallback, token, searchParams.get("cli_state") || "");
        return;
      }

      const wsList = await api.listWorkspaces();
      await hydrateWorkspace(wsList);
      router.push(searchParams.get("next") || "/issues");
    } catch (err) {
      setError(
        err instanceof Error ? err.message : "Failed to sign in with Google"
      );
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
          <Button
            onClick={handleGoogleSignIn}
            disabled={submitting}
            className="w-full"
            size="lg"
          >
            {submitting ? "Signing in..." : "Continue with Google"}
          </Button>
          <p className="text-center text-sm text-muted-foreground">
            Sign in with your Firebase-enabled Google account.
          </p>
          {error ? <p className="text-sm text-destructive">{error}</p> : null}
        </CardContent>
      </Card>
    </div>
  );
}

export default function LoginPage() {
  return (
    <Suspense fallback={null}>
      <LoginPageContent />
    </Suspense>
  );
}
