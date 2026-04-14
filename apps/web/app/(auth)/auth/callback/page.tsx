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
  CardFooter,
} from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Loader2 } from "lucide-react";

function OAuthCallbackContent() {
  const router = useRouter();
  const searchParams = useSearchParams();
  const loginWithOAuth = useAuthStore((s) => s.loginWithOAuth);
  const hydrateWorkspace = useWorkspaceStore((s) => s.hydrateWorkspace);

  const [error, setError] = useState("");
  const [exchanging, setExchanging] = useState(true);

  useEffect(() => {
    const code = searchParams.get("code");
    const errorParam = searchParams.get("error");

    if (errorParam) {
      setError(
        errorParam === "access_denied"
          ? "Authorization was denied."
          : `Authorization failed: ${errorParam}`
      );
      setExchanging(false);
      return;
    }

    if (!code) {
      setError("Missing authorization code.");
      setExchanging(false);
      return;
    }

    const redirectUri = `${window.location.origin}/auth/callback`;

    (async () => {
      try {
        await loginWithOAuth(code, redirectUri);
        const wsList = await api.listWorkspaces();
        await hydrateWorkspace(wsList);
        router.push("/issues");
      } catch (err) {
        setError(
          err instanceof Error ? err.message : "Failed to complete login."
        );
        setExchanging(false);
      }
    })();
  }, [searchParams, loginWithOAuth, hydrateWorkspace, router]);

  if (exchanging && !error) {
    return (
      <div className="flex min-h-screen items-center justify-center">
        <Card className="w-full max-w-sm">
          <CardHeader className="text-center">
            <CardTitle className="text-2xl">Signing in</CardTitle>
            <CardDescription>
              Completing authentication...
            </CardDescription>
          </CardHeader>
          <CardContent className="flex justify-center py-6">
            <Loader2 className="h-8 w-8 animate-spin text-muted-foreground" />
          </CardContent>
        </Card>
      </div>
    );
  }

  return (
    <div className="flex min-h-screen items-center justify-center">
      <Card className="w-full max-w-sm">
        <CardHeader className="text-center">
          <CardTitle className="text-2xl">Login Failed</CardTitle>
          <CardDescription>{error}</CardDescription>
        </CardHeader>
        <CardFooter>
          <Button
            className="w-full"
            onClick={() => router.push("/login")}
          >
            Back to login
          </Button>
        </CardFooter>
      </Card>
    </div>
  );
}

export default function OAuthCallbackPage() {
  return (
    <Suspense
      fallback={
        <div className="flex min-h-screen items-center justify-center">
          <Loader2 className="h-8 w-8 animate-spin text-muted-foreground" />
        </div>
      }
    >
      <OAuthCallbackContent />
    </Suspense>
  );
}
