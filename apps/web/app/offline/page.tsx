import Link from "next/link";
import { Button } from "@/components/ui/button";

export default function OfflinePage() {
  return (
    <main className="flex min-h-dvh items-center justify-center bg-background px-6 py-10">
      <div className="w-full max-w-md rounded-3xl border bg-card p-8 text-center shadow-sm">
        <div className="mx-auto flex size-14 items-center justify-center rounded-2xl border bg-muted/60">
          <div
            aria-hidden="true"
            className="size-6 bg-foreground"
            style={{
              clipPath:
                "polygon(45% 62.1%,45% 100%,55% 100%,55% 62.1%,81.8% 88.9%,88.9% 81.8%,62.1% 55%,100% 55%,100% 45%,62.1% 45%,88.9% 18.2%,81.8% 11.1%,55% 37.9%,55% 0%,45% 0%,45% 37.9%,18.2% 11.1%,11.1% 18.2%,37.9% 45%,0% 45%,0% 55%,37.9% 55%,11.1% 81.8%,18.2% 88.9%)",
            }}
          />
        </div>
        <h1 className="mt-5 text-2xl font-semibold tracking-tight">You&apos;re offline</h1>
        <p className="mt-3 text-sm leading-6 text-muted-foreground">
          Multica can reopen once your connection returns. Cached pages stay available, and new data
          will sync after reconnecting.
        </p>
        <div className="mt-6 flex justify-center">
          <Link href="/issues">
            <Button>Try again</Button>
          </Link>
        </div>
      </div>
    </main>
  );
}
