"use client";

import { useState } from "react";
import { Pencil, Plus, Trash2, X } from "lucide-react";
import { toast } from "sonner";
import { Button } from "@/components/ui/button";
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from "@/components/ui/alert-dialog";
import type { Label } from "@/shared/types";
import { api } from "@/shared/api";
import { useLabelStore } from "../store";

const PRESET_COLORS = [
  "#ef4444", "#f97316", "#eab308", "#22c55e",
  "#06b6d4", "#3b82f6", "#8b5cf6", "#ec4899",
  "#64748b", "#0ea5e9", "#14b8a6", "#a855f7",
];

function LabelForm({
  initial,
  onSubmit,
  onCancel,
  submitLabel,
}: {
  initial?: { name: string; color: string };
  onSubmit: (data: { name: string; color: string }) => Promise<void>;
  onCancel: () => void;
  submitLabel: string;
}) {
  const [name, setName] = useState(initial?.name ?? "");
  const [color, setColor] = useState(initial?.color ?? "#ef4444");
  const [submitting, setSubmitting] = useState(false);

  const handleSubmit = async () => {
    if (!name.trim() || submitting) return;
    setSubmitting(true);
    try {
      await onSubmit({ name: name.trim(), color });
    } finally {
      setSubmitting(false);
    }
  };

  return (
    <div className="flex items-center gap-3 rounded-lg border p-3">
      <div className="flex gap-1.5 shrink-0">
        {PRESET_COLORS.map((c) => (
          <button
            key={c}
            type="button"
            onClick={() => setColor(c)}
            className="h-5 w-5 rounded-full cursor-pointer transition-transform hover:scale-110"
            style={{
              backgroundColor: c,
              outline: c === color ? "2px solid currentColor" : "none",
              outlineOffset: "2px",
            }}
          />
        ))}
      </div>
      <input
        type="text"
        value={name}
        onChange={(e) => setName(e.target.value)}
        placeholder="Label name"
        onKeyDown={(e) => { if (e.key === "Enter") handleSubmit(); }}
        className="flex-1 min-w-0 rounded border px-2 py-1 text-sm bg-transparent outline-none focus:ring-1 focus:ring-ring"
      />
      <div className="flex items-center gap-1.5">
        <Button size="sm" onClick={handleSubmit} disabled={!name.trim() || submitting}>
          {submitting ? "..." : submitLabel}
        </Button>
        <Button size="sm" variant="ghost" onClick={onCancel}>
          <X className="h-4 w-4" />
        </Button>
      </div>
    </div>
  );
}

export function LabelManager() {
  const labels = useLabelStore((s) => s.labels);
  const [showCreate, setShowCreate] = useState(false);
  const [editingId, setEditingId] = useState<string | null>(null);
  const [deletingLabel, setDeletingLabel] = useState<Label | null>(null);

  const handleCreate = async (data: { name: string; color: string }) => {
    try {
      const label = await api.createLabel(data);
      useLabelStore.getState().addLabel(label);
      setShowCreate(false);
      toast.success("Label created");
    } catch {
      toast.error("Failed to create label");
    }
  };

  const handleUpdate = async (id: string, data: { name: string; color: string }) => {
    try {
      const label = await api.updateLabel(id, data);
      useLabelStore.getState().updateLabel(id, label);
      setEditingId(null);
      toast.success("Label updated");
    } catch {
      toast.error("Failed to update label");
    }
  };

  const handleDelete = async () => {
    if (!deletingLabel) return;
    try {
      await api.deleteLabel(deletingLabel.id);
      useLabelStore.getState().removeLabel(deletingLabel.id);
      toast.success("Label deleted");
    } catch {
      toast.error("Failed to delete label");
    } finally {
      setDeletingLabel(null);
    }
  };

  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between">
        <div>
          <h3 className="text-sm font-semibold">Labels</h3>
          <p className="text-xs text-muted-foreground mt-0.5">
            Manage labels for organizing issues in this workspace.
          </p>
        </div>
        {!showCreate && (
          <Button size="sm" variant="outline" onClick={() => setShowCreate(true)}>
            <Plus className="h-4 w-4 mr-1" />
            New label
          </Button>
        )}
      </div>

      {showCreate && (
        <LabelForm
          onSubmit={handleCreate}
          onCancel={() => setShowCreate(false)}
          submitLabel="Create"
        />
      )}

      <div className="divide-y rounded-lg border">
        {labels.length === 0 && (
          <div className="px-4 py-8 text-center text-sm text-muted-foreground">
            No labels yet. Create one to start organizing issues.
          </div>
        )}
        {labels.map((label) =>
          editingId === label.id ? (
            <div key={label.id} className="p-2">
              <LabelForm
                initial={{ name: label.name, color: label.color }}
                onSubmit={(data) => handleUpdate(label.id, data)}
                onCancel={() => setEditingId(null)}
                submitLabel="Save"
              />
            </div>
          ) : (
            <div
              key={label.id}
              className="flex items-center gap-3 px-4 py-2.5 group"
            >
              <span
                className="h-4 w-4 rounded-full shrink-0"
                style={{ backgroundColor: label.color }}
              />
              <span className="flex-1 text-sm font-medium truncate">
                {label.name}
              </span>
              <div className="flex items-center gap-1 opacity-0 group-hover:opacity-100 transition-opacity">
                <Button
                  size="icon-xs"
                  variant="ghost"
                  onClick={() => setEditingId(label.id)}
                >
                  <Pencil className="h-3.5 w-3.5" />
                </Button>
                <Button
                  size="icon-xs"
                  variant="ghost"
                  onClick={() => setDeletingLabel(label)}
                  className="text-destructive hover:text-destructive"
                >
                  <Trash2 className="h-3.5 w-3.5" />
                </Button>
              </div>
            </div>
          ),
        )}
      </div>

      <AlertDialog open={!!deletingLabel} onOpenChange={(v) => { if (!v) setDeletingLabel(null); }}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>Delete label</AlertDialogTitle>
            <AlertDialogDescription>
              This will remove the label &ldquo;{deletingLabel?.name}&rdquo; from all issues. This action cannot be undone.
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>Cancel</AlertDialogCancel>
            <AlertDialogAction onClick={handleDelete} className="bg-destructive text-destructive-foreground hover:bg-destructive/90">
              Delete
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </div>
  );
}
