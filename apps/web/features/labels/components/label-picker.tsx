"use client";

import { useState } from "react";
import { Check, Plus, Tag } from "lucide-react";
import {
  Popover,
  PopoverTrigger,
  PopoverContent,
} from "@/components/ui/popover";
import type { Label } from "@/shared/types";
import { useLabelStore } from "../store";
import { api } from "@/shared/api";
import { toast } from "sonner";

const PRESET_COLORS = [
  "#ef4444", "#f97316", "#eab308", "#22c55e",
  "#06b6d4", "#3b82f6", "#8b5cf6", "#ec4899",
];

export function LabelPicker({
  issueId,
  selected,
  onUpdate,
  align = "start",
  triggerRender,
}: {
  issueId: string;
  selected: Label[];
  onUpdate: (labels: Label[]) => void;
  align?: "start" | "center" | "end";
  triggerRender?: React.ReactElement;
}) {
  const allLabels = useLabelStore((s) => s.labels);
  const [open, setOpen] = useState(false);
  const [filter, setFilter] = useState("");
  const [creating, setCreating] = useState(false);
  const [newColor, setNewColor] = useState("#ef4444");

  const selectedIds = new Set(selected.map((l) => l.id));
  const query = filter.toLowerCase();
  const filtered = allLabels.filter((l) =>
    l.name.toLowerCase().includes(query),
  );
  const exactMatch = allLabels.some(
    (l) => l.name.toLowerCase() === query,
  );

  const toggle = async (label: Label) => {
    const next = selectedIds.has(label.id)
      ? selected.filter((l) => l.id !== label.id)
      : [...selected, label];
    const nextIds = next.map((l) => l.id);
    onUpdate(next);
    try {
      await api.setIssueLabels(issueId, nextIds);
    } catch {
      onUpdate(selected);
      toast.error("Failed to update labels");
    }
  };

  const handleCreate = async () => {
    if (!filter.trim()) return;
    setCreating(true);
    try {
      const label = await api.createLabel({
        name: filter.trim(),
        color: newColor,
      });
      useLabelStore.getState().addLabel(label);
      const next = [...selected, label];
      onUpdate(next);
      await api.setIssueLabels(issueId, next.map((l) => l.id));
      setFilter("");
    } catch {
      toast.error("Failed to create label");
    } finally {
      setCreating(false);
    }
  };

  return (
    <Popover
      open={open}
      onOpenChange={(v) => {
        setOpen(v);
        if (!v) setFilter("");
      }}
    >
      <PopoverTrigger
        className={triggerRender ? undefined : "flex items-center gap-1.5 cursor-pointer rounded px-1 -mx-1 hover:bg-accent/30 transition-colors overflow-hidden"}
        render={triggerRender}
      >
        {selected.length > 0 ? (
          <div className="flex items-center gap-1 overflow-hidden">
            {selected.slice(0, 3).map((l) => (
              <span
                key={l.id}
                className="inline-flex items-center gap-1 rounded-full px-1.5 py-0.5 text-xs border truncate"
                style={{
                  borderColor: l.color + "40",
                  backgroundColor: l.color + "15",
                }}
              >
                <span
                  className="h-2 w-2 rounded-full shrink-0"
                  style={{ backgroundColor: l.color }}
                />
                <span className="truncate max-w-[80px]">{l.name}</span>
              </span>
            ))}
            {selected.length > 3 && (
              <span className="text-xs text-muted-foreground">
                +{selected.length - 3}
              </span>
            )}
          </div>
        ) : (
          <span className="flex items-center gap-1.5 text-muted-foreground">
            <Tag className="h-3.5 w-3.5" />
            <span>Labels</span>
          </span>
        )}
      </PopoverTrigger>
      <PopoverContent align={align} className="w-52 p-0">
        <div className="px-2 py-1.5 border-b">
          <input
            type="text"
            value={filter}
            onChange={(e) => setFilter(e.target.value)}
            placeholder="Filter labels..."
            className="w-full bg-transparent text-sm placeholder:text-muted-foreground outline-none"
          />
        </div>
        <div className="p-1 max-h-60 overflow-y-auto">
          {filtered.map((label) => (
            <button
              key={label.id}
              type="button"
              onClick={() => toggle(label)}
              className="flex w-full items-center gap-2 rounded-md px-2 py-1.5 text-sm hover:bg-accent transition-colors"
            >
              <span
                className="h-3 w-3 rounded-full shrink-0"
                style={{ backgroundColor: label.color }}
              />
              <span className="flex-1 text-left truncate">{label.name}</span>
              {selectedIds.has(label.id) && (
                <Check className="h-3.5 w-3.5 text-muted-foreground shrink-0" />
              )}
            </button>
          ))}
          {filtered.length === 0 && !filter && (
            <div className="px-2 py-3 text-center text-sm text-muted-foreground">
              No labels yet
            </div>
          )}
          {filter.trim() && !exactMatch && (
            <button
              type="button"
              disabled={creating}
              onClick={handleCreate}
              className="flex w-full items-center gap-2 rounded-md px-2 py-1.5 text-sm hover:bg-accent transition-colors border-t mt-1 pt-1.5"
            >
              <Plus className="h-3.5 w-3.5 text-muted-foreground" />
              <span>
                Create &ldquo;<span className="font-medium">{filter.trim()}</span>&rdquo;
              </span>
              <span
                className="ml-auto h-3 w-3 rounded-full shrink-0"
                style={{ backgroundColor: newColor }}
              />
            </button>
          )}
        </div>
        {filter.trim() && !exactMatch && (
          <div className="border-t px-2 py-1.5 flex gap-1">
            {PRESET_COLORS.map((c) => (
              <button
                key={c}
                type="button"
                onClick={() => setNewColor(c)}
                className="h-4 w-4 rounded-full cursor-pointer transition-transform hover:scale-110"
                style={{
                  backgroundColor: c,
                  outline: c === newColor ? "2px solid currentColor" : "none",
                  outlineOffset: "1px",
                }}
              />
            ))}
          </div>
        )}
      </PopoverContent>
    </Popover>
  );
}
