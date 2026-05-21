"use client";

import { useRef, useState } from "react";
import { ChevronRight, Copy, Info, MoreHorizontal, Pencil, Trash2 } from "lucide-react";
import { toast } from "sonner";
import { Card } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import {
  DropdownMenu,
  DropdownMenuTrigger,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
} from "@/components/ui/dropdown-menu";
import { Tooltip, TooltipTrigger, TooltipContent } from "@/components/ui/tooltip";
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
import { Collapsible, CollapsibleTrigger, CollapsibleContent } from "@/components/ui/collapsible";
import { ActorAvatar } from "@/components/common/actor-avatar";
import { ReactionBar } from "@/components/common/reaction-bar";
import { AttachmentList } from "@/components/common/attachment-list";
import { QuickEmojiPicker } from "@/components/common/quick-emoji-picker";
import { cn } from "@/lib/utils";
import { useActorName } from "@/features/workspace";
import { timeAgo } from "@/shared/utils";
import { RichTextEditor, type RichTextEditorRef } from "@/components/common/rich-text-editor";
import { ReadonlyEditor } from "@/components/common/readonly-editor";
import { FileUploadButton } from "@/components/common/file-upload-button";
import { useFileUpload } from "@/shared/hooks/use-file-upload";
import { ReplyInput } from "./reply-input";
import type { TimelineEntry } from "@/shared/types";

// ---------------------------------------------------------------------------
// Types
// ---------------------------------------------------------------------------

interface CommentCardProps {
  issueId: string;
  entry: TimelineEntry;
  allReplies: Map<string, TimelineEntry[]>;
  currentUserId?: string;
  onReply: (parentId: string, content: string, attachmentIds?: string[]) => Promise<void>;
  onEdit: (commentId: string, content: string) => Promise<void>;
  onDelete: (commentId: string) => void;
  onToggleReaction: (commentId: string, emoji: string) => void;
  /** ID of the comment to highlight (flash animation). */
  highlightedCommentId?: string | null;
  /**
   * Run trace nodes to embed inside this thread, keyed by the reply comment
   * id they belong to. The trace renders between the header and the content
   * body of whichever entry matches — parent or any nested reply. Used to
   * surface "how did the agent arrive at this reply" right under the reply
   * itself instead of as a sibling card.
   */
  runTraces?: Map<string, React.ReactNode>;
}

// ---------------------------------------------------------------------------
// Shared delete confirmation dialog
// ---------------------------------------------------------------------------

function DeleteCommentDialog({
  open,
  onOpenChange,
  onConfirm,
  hasReplies,
}: {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  onConfirm: () => void;
  hasReplies?: boolean;
}) {
  return (
    <AlertDialog open={open} onOpenChange={onOpenChange}>
      <AlertDialogContent>
        <AlertDialogHeader>
          <AlertDialogTitle>Delete comment</AlertDialogTitle>
          <AlertDialogDescription>
            {hasReplies
              ? "This comment and all its replies will be permanently deleted. This cannot be undone."
              : "This comment will be permanently deleted. This cannot be undone."}
          </AlertDialogDescription>
        </AlertDialogHeader>
        <AlertDialogFooter>
          <AlertDialogCancel>Cancel</AlertDialogCancel>
          <AlertDialogAction variant="destructive" onClick={onConfirm}>
            Delete
          </AlertDialogAction>
        </AlertDialogFooter>
      </AlertDialogContent>
    </AlertDialog>
  );
}

// ---------------------------------------------------------------------------
// Single comment row (used for both parent and replies within the same Card)
// ---------------------------------------------------------------------------

function SystemNoticeRow({ entry }: { entry: TimelineEntry }) {
  return (
    <div className="py-2.5 flex items-start gap-2 text-sm text-muted-foreground">
      <Info className="mt-0.5 h-3.5 w-3.5 shrink-0 text-amber-500" />
      <ReadonlyEditor content={entry.content ?? ""} />
    </div>
  );
}

function CommentRow({
  issueId,
  entry,
  currentUserId,
  onEdit,
  onDelete,
  onToggleReaction,
  runTrace,
}: {
  issueId: string;
  entry: TimelineEntry;
  currentUserId?: string;
  onEdit: (commentId: string, content: string) => Promise<void>;
  onDelete: (commentId: string) => void;
  onToggleReaction: (commentId: string, emoji: string) => void;
  runTrace?: React.ReactNode;
}) {
  const { getActorName } = useActorName();
  const [editing, setEditing] = useState(false);
  const editEditorRef = useRef<RichTextEditorRef>(null);
  const cancelledRef = useRef(false);
  const { uploadWithToast } = useFileUpload();

  const isOwn = entry.actor_type === "member" && entry.actor_id === currentUserId;
  const isTemp = entry.id.startsWith("temp-");
  const [confirmDelete, setConfirmDelete] = useState(false);

  if (entry.actor_type === "system") {
    return <SystemNoticeRow entry={entry} />;
  }

  const startEdit = () => {
    cancelledRef.current = false;
    setEditing(true);
  };

  const cancelEdit = () => {
    cancelledRef.current = true;
    setEditing(false);
  };

  const saveEdit = async () => {
    if (cancelledRef.current) return;
    const trimmed = editEditorRef.current
      ?.getMarkdown()
      ?.replace(/(\n\s*)+$/, "")
      .trim();
    if (!trimmed || trimmed === (entry.content ?? "").trim()) {
      setEditing(false);
      return;
    }
    try {
      await onEdit(entry.id, trimmed);
      setEditing(false);
    } catch {
      toast.error("Failed to update comment");
    }
  };

  const reactions = entry.reactions ?? [];

  return (
    <div className={`py-3${isTemp ? " opacity-60" : ""}`}>
      <div className="flex items-center gap-2.5">
        <ActorAvatar actorType={entry.actor_type} actorId={entry.actor_id} size={24} />
        <span className="text-sm font-medium">
          {getActorName(entry.actor_type, entry.actor_id)}
        </span>
        <Tooltip>
          <TooltipTrigger
            render={
              <span className="text-xs text-muted-foreground cursor-default">
                {timeAgo(entry.created_at)}
              </span>
            }
          />
          <TooltipContent side="top">
            {new Date(entry.created_at).toLocaleString()}
          </TooltipContent>
        </Tooltip>

        {!isTemp && (
          <div className="ml-auto flex items-center gap-0.5">
            <QuickEmojiPicker
              onSelect={(emoji) => onToggleReaction(entry.id, emoji)}
              align="end"
            />
          <DropdownMenu>
            <DropdownMenuTrigger
              render={
                <Button variant="ghost" size="icon-xs" className="text-muted-foreground">
                  <MoreHorizontal className="h-4 w-4" />
                </Button>
              }
            />
            <DropdownMenuContent align="end">
              <DropdownMenuItem onClick={() => {
                navigator.clipboard.writeText(entry.content ?? "");
                toast.success("Copied");
              }}>
                <Copy className="h-3.5 w-3.5" />
                Copy
              </DropdownMenuItem>
              {isOwn && (
                <>
                  <DropdownMenuSeparator />
                  <DropdownMenuItem onClick={startEdit}>
                    <Pencil className="h-3.5 w-3.5" />
                    Edit
                  </DropdownMenuItem>
                  <DropdownMenuSeparator />
                  <DropdownMenuItem onClick={() => setConfirmDelete(true)} variant="destructive">
                    <Trash2 className="h-3.5 w-3.5" />
                    Delete
                  </DropdownMenuItem>
                </>
              )}
            </DropdownMenuContent>
          </DropdownMenu>
          <DeleteCommentDialog
            open={confirmDelete}
            onOpenChange={setConfirmDelete}
            onConfirm={() => onDelete(entry.id)}
          />
          </div>
        )}
      </div>

      {runTrace && (
        <div className="mt-2 pl-8 animate-in fade-in slide-in-from-top-1 duration-300">
          {runTrace}
        </div>
      )}

      {editing ? (
        <div
          className="mt-1.5 pl-8"
          onKeyDown={(e) => { if (e.key === "Escape") cancelEdit(); }}
        >
          <div className="max-h-48 overflow-y-auto text-sm leading-relaxed">
            <RichTextEditor
              ref={editEditorRef}
              defaultValue={entry.content ?? ""}
              placeholder="Edit comment..."
              onSubmit={saveEdit}
              onUploadFile={(file) => uploadWithToast(file, { issueId })}
              debounceMs={100}
            />
          </div>
          <div className="flex items-center justify-between mt-2">
            <FileUploadButton
              size="sm"
              onSelect={(file) => editEditorRef.current?.uploadFile(file)}
            />
            <div className="flex items-center gap-2">
              <Button size="sm" variant="ghost" onClick={cancelEdit}>Cancel</Button>
              <Button size="sm" variant="outline" onClick={saveEdit}>Save</Button>
            </div>
          </div>
        </div>
      ) : (
        <>
          <div className="mt-1.5 pl-8 text-sm leading-relaxed text-foreground/85">
            <ReadonlyEditor content={entry.content ?? ""} />
          </div>
          {(entry.attachments?.length ?? 0) > 0 && (
            <AttachmentList
              attachments={entry.attachments ?? []}
              className="mt-2 pl-8"
            />
          )}
          {!isTemp && (
            <ReactionBar
              reactions={reactions}
              currentUserId={currentUserId}
              onToggle={(emoji) => onToggleReaction(entry.id, emoji)}
              hideAddButton
              className="mt-1.5 pl-8"
            />
          )}
        </>
      )}
    </div>
  );
}

// ---------------------------------------------------------------------------
// CommentCard — One Card per thread (parent + all replies flat inside)
// ---------------------------------------------------------------------------

function CommentCard({
  issueId,
  entry,
  allReplies,
  currentUserId,
  onReply,
  onEdit,
  onDelete,
  onToggleReaction,
  highlightedCommentId,
  runTraces,
}: CommentCardProps) {
  const { getActorName } = useActorName();
  const { uploadWithToast } = useFileUpload();
  const [open, setOpen] = useState(true);
  const [editing, setEditing] = useState(false);
  const editEditorRef = useRef<RichTextEditorRef>(null);
  const cancelledRef = useRef(false);

  const isOwn = entry.actor_type === "member" && entry.actor_id === currentUserId;
  const isTemp = entry.id.startsWith("temp-");
  const [confirmDelete, setConfirmDelete] = useState(false);

  const startEdit = () => {
    cancelledRef.current = false;
    setEditing(true);
  };

  const cancelEdit = () => {
    cancelledRef.current = true;
    setEditing(false);
  };

  const saveEdit = async () => {
    if (cancelledRef.current) return;
    const trimmed = editEditorRef.current
      ?.getMarkdown()
      ?.replace(/(\n\s*)+$/, "")
      .trim();
    if (!trimmed || trimmed === (entry.content ?? "").trim()) {
      setEditing(false);
      return;
    }
    try {
      await onEdit(entry.id, trimmed);
      setEditing(false);
    } catch {
      toast.error("Failed to update comment");
    }
  };

  // Collect all nested replies recursively into a flat list
  const allNestedReplies: TimelineEntry[] = [];
  const collectReplies = (parentId: string) => {
    const children = allReplies.get(parentId) ?? [];
    for (const child of children) {
      allNestedReplies.push(child);
      collectReplies(child.id);
    }
  };
  collectReplies(entry.id);

  const replyCount = allNestedReplies.length;
  const contentPreview = (entry.content ?? "").replace(/\n/g, " ").slice(0, 80);
  const reactions = entry.reactions ?? [];

  const isHighlighted = highlightedCommentId === entry.id;

  return (
    <Card className={cn("!py-0 !gap-0 overflow-hidden transition-colors duration-700", isTemp && "opacity-60", isHighlighted && "ring-2 ring-brand/50 bg-brand/5")}>
      <Collapsible open={open} onOpenChange={setOpen}>
        {/* Header — always visible, acts as toggle */}
        <div className="px-4 py-3">
          <div className="flex items-center gap-2.5">
            <CollapsibleTrigger className="shrink-0 rounded p-0.5 text-muted-foreground hover:bg-muted hover:text-foreground transition-colors">
              <ChevronRight className={cn("h-3.5 w-3.5 transition-transform", open && "rotate-90")} />
            </CollapsibleTrigger>
            <ActorAvatar actorType={entry.actor_type} actorId={entry.actor_id} size={24} />
            <span className="shrink-0 text-sm font-medium">
              {getActorName(entry.actor_type, entry.actor_id)}
            </span>
            <Tooltip>
              <TooltipTrigger
                render={
                  <span className="shrink-0 text-xs text-muted-foreground cursor-default">
                    {timeAgo(entry.created_at)}
                  </span>
                }
              />
              <TooltipContent side="top">
                {new Date(entry.created_at).toLocaleString()}
              </TooltipContent>
            </Tooltip>

            {!open && contentPreview && (
              <span className="min-w-0 flex-1 truncate text-xs text-muted-foreground">
                {contentPreview}
              </span>
            )}
            {!open && replyCount > 0 && (
              <span className="shrink-0 text-xs text-muted-foreground">
                {replyCount} {replyCount === 1 ? "reply" : "replies"}
              </span>
            )}

            {open && !isTemp && (
              <div className="ml-auto flex items-center gap-0.5">
                <QuickEmojiPicker
                  onSelect={(emoji) => onToggleReaction(entry.id, emoji)}
                  align="end"
                />
              <DropdownMenu>
                <DropdownMenuTrigger
                  render={
                    <Button variant="ghost" size="icon-xs" className="text-muted-foreground">
                      <MoreHorizontal className="h-4 w-4" />
                    </Button>
                  }
                />
                <DropdownMenuContent align="end">
                  <DropdownMenuItem onClick={() => {
                    navigator.clipboard.writeText(entry.content ?? "");
                    toast.success("Copied");
                  }}>
                    <Copy className="h-3.5 w-3.5" />
                    Copy
                  </DropdownMenuItem>
                  {isOwn && (
                    <>
                      <DropdownMenuSeparator />
                      <DropdownMenuItem onClick={startEdit}>
                        <Pencil className="h-3.5 w-3.5" />
                        Edit
                      </DropdownMenuItem>
                      <DropdownMenuSeparator />
                      <DropdownMenuItem onClick={() => setConfirmDelete(true)} variant="destructive">
                        <Trash2 className="h-3.5 w-3.5" />
                        Delete
                      </DropdownMenuItem>
                    </>
                  )}
                </DropdownMenuContent>
              </DropdownMenu>
              <DeleteCommentDialog
                open={confirmDelete}
                onOpenChange={setConfirmDelete}
                onConfirm={() => onDelete(entry.id)}
                hasReplies
              />
              </div>
            )}
          </div>
        </div>

        {/* Collapsible body */}
        <CollapsibleContent>
          {/* Parent comment body */}
          <div className="px-4 pb-3">
            {editing ? (
              <div
                className="pl-10"
                onKeyDown={(e) => { if (e.key === "Escape") cancelEdit(); }}
              >
                <div className="max-h-48 overflow-y-auto text-sm leading-relaxed">
                  <RichTextEditor
                    ref={editEditorRef}
                    defaultValue={entry.content ?? ""}
                    placeholder="Edit comment..."
                    onSubmit={saveEdit}
                    debounceMs={100}
                  />
                </div>
                <div className="flex items-center justify-between mt-2">
                  <FileUploadButton
                    size="sm"
                    onSelect={(file) => editEditorRef.current?.uploadFile(file)}
                  />
                  <div className="flex items-center gap-2">
                    <Button size="sm" variant="ghost" onClick={cancelEdit}>Cancel</Button>
                    <Button size="sm" variant="outline" onClick={saveEdit}>Save</Button>
                  </div>
                </div>
              </div>
            ) : (
              <>
                {runTraces?.get(entry.id) && (
                  <div className="mt-2 pl-10 animate-in fade-in slide-in-from-top-1 duration-300">
                    {runTraces.get(entry.id)}
                  </div>
                )}
                <div className={cn("pl-10 text-sm leading-relaxed text-foreground/85", runTraces?.get(entry.id) && "mt-2")}>
                  <ReadonlyEditor content={entry.content ?? ""} />
                </div>
                {(entry.attachments?.length ?? 0) > 0 && (
                  <AttachmentList
                    attachments={entry.attachments ?? []}
                    className="mt-2 pl-10"
                  />
                )}
                {!isTemp && (
                  <ReactionBar
                    reactions={reactions}
                    currentUserId={currentUserId}
                    onToggle={(emoji) => onToggleReaction(entry.id, emoji)}
                    className="mt-1.5 pl-10"
                  />
                )}
              </>
            )}
          </div>

          {/* Replies */}
          {allNestedReplies.map((reply) => (
            <div key={reply.id} id={`comment-${reply.id}`} className={cn("border-t border-border/50 px-4 transition-colors duration-700", highlightedCommentId === reply.id && "bg-brand/5")}>
              <CommentRow
                issueId={issueId}
                entry={reply}
                currentUserId={currentUserId}
                onEdit={onEdit}
                onDelete={onDelete}
                onToggleReaction={onToggleReaction}
                runTrace={runTraces?.get(reply.id)}
              />
            </div>
          ))}

          {/* Reply input */}
          <div className="border-t border-border/50 px-4 py-2.5">
            <ReplyInput
              issueId={issueId}
              placeholder="Leave a reply..."
              size="sm"
              avatarType="member"
              avatarId={currentUserId ?? ""}
              onSubmit={(content, attachmentIds) => onReply(entry.id, content, attachmentIds)}
            />
          </div>
        </CollapsibleContent>
      </Collapsible>
    </Card>
  );
}

export { CommentCard, type CommentCardProps };
