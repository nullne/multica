"use client";

import { useRef } from "react";
import { Loader2, Paperclip } from "lucide-react";
import { cn } from "@/lib/utils";

interface FileUploadButtonProps {
  /** Called with the selected File — caller handles upload. */
  onSelect: (file: File) => void;
  /** Disables the button without indicating any specific reason. */
  disabled?: boolean;
  /**
   * Indicates an upload is in progress: swaps the icon to a spinner and
   * prevents clicks so users don't accidentally start another upload.
   */
  busy?: boolean;
  className?: string;
  size?: "sm" | "default";
}

function FileUploadButton({
  onSelect,
  disabled,
  busy,
  className,
  size = "default",
}: FileUploadButtonProps) {
  const inputRef = useRef<HTMLInputElement>(null);

  const handleChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0];
    if (!file) return;
    e.target.value = "";
    onSelect(file);
  };

  const iconSize = size === "sm" ? "h-3.5 w-3.5" : "h-4 w-4";
  const btnSize = size === "sm" ? "h-6 w-6" : "h-7 w-7";

  return (
    <>
      <button
        type="button"
        onClick={() => inputRef.current?.click()}
        disabled={disabled || busy}
        aria-busy={busy || undefined}
        aria-label={busy ? "Uploading file" : "Attach file"}
        className={cn(
          "inline-flex items-center justify-center rounded-full text-muted-foreground hover:bg-accent hover:text-foreground transition-colors disabled:pointer-events-none",
          busy ? "opacity-100 text-foreground" : "disabled:opacity-50",
          btnSize,
          className,
        )}
      >
        {busy ? (
          <Loader2 className={cn(iconSize, "animate-spin")} />
        ) : (
          <Paperclip className={iconSize} />
        )}
      </button>
      <input
        ref={inputRef}
        type="file"
        className="hidden"
        onChange={handleChange}
      />
    </>
  );
}

export { FileUploadButton, type FileUploadButtonProps };
