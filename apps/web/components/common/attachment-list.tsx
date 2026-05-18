"use client";

import { Download, FileText } from "lucide-react";
import { cn } from "@/lib/utils";
import type { Attachment } from "@/shared/types";

function formatSize(bytes: number): string {
  if (bytes < 1024) return `${bytes} B`;
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`;
  if (bytes < 1024 * 1024 * 1024) return `${(bytes / (1024 * 1024)).toFixed(1)} MB`;
  return `${(bytes / (1024 * 1024 * 1024)).toFixed(1)} GB`;
}

function isImage(att: Attachment): boolean {
  return att.content_type.startsWith("image/");
}

export function AttachmentList({
  attachments,
  className,
}: {
  attachments: Attachment[];
  className?: string;
}) {
  if (!attachments.length) return null;

  const images = attachments.filter(isImage);
  const files = attachments.filter((a) => !isImage(a));

  return (
    <div className={cn("flex flex-col gap-2", className)}>
      {images.length > 0 && (
        <div className="flex flex-wrap gap-2">
          {images.map((att) => (
            <a
              key={att.id}
              href={att.url}
              target="_blank"
              rel="noopener noreferrer"
              className="block overflow-hidden rounded-md border border-border hover:opacity-90 transition-opacity"
              title={att.filename}
            >
              {/* eslint-disable-next-line @next/next/no-img-element */}
              <img
                src={att.url}
                alt={att.filename}
                className="max-h-48 max-w-xs object-contain"
                loading="lazy"
              />
            </a>
          ))}
        </div>
      )}
      {files.length > 0 && (
        <div className="flex flex-col gap-1">
          {files.map((att) => (
            <a
              key={att.id}
              href={att.download_url}
              target="_blank"
              rel="noopener noreferrer"
              className="inline-flex w-fit max-w-full items-center gap-2 rounded-md border border-border bg-muted/30 px-2.5 py-1.5 text-xs text-foreground hover:bg-muted transition-colors"
            >
              <FileText className="h-3.5 w-3.5 shrink-0 text-muted-foreground" />
              <span className="truncate">{att.filename}</span>
              <span className="shrink-0 text-muted-foreground">
                {formatSize(att.size_bytes)}
              </span>
              <Download className="h-3 w-3 shrink-0 text-muted-foreground" />
            </a>
          ))}
        </div>
      )}
    </div>
  );
}
