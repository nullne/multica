"use client";

import {
  forwardRef,
  useCallback,
  useEffect,
  useImperativeHandle,
  useRef,
  useState,
} from "react";
import { ReactRenderer } from "@tiptap/react";
import { computePosition, offset, flip, shift } from "@floating-ui/dom";
import { useWorkspaceStore } from "@/features/workspace";
import { useIssueStore } from "@/features/issues";
import { useRuntimeStore } from "@/features/runtimes";
import { ActorAvatar } from "@/components/common/actor-avatar";
import { StatusIcon } from "@/features/issues/components/status-icon";
import { Badge } from "@/components/ui/badge";
import { hasCompleteAgentDispatchDefaults } from "@/features/issues/utils/dispatch";
import type { IssueStatus } from "@/shared/types";
import type { SuggestionOptions, SuggestionProps } from "@tiptap/suggestion";

// ---------------------------------------------------------------------------
// Types
// ---------------------------------------------------------------------------

export interface MentionItem {
  id: string;
  label: string;
  type: "member" | "agent" | "issue" | "all";
  /** Secondary text shown beside the label (e.g. issue title) */
  description?: string;
  /** Issue status for StatusIcon rendering */
  status?: IssueStatus;
  disabled?: boolean;
  disabledReason?: string;
}

interface MentionListProps {
  items: MentionItem[];
  command: (item: MentionItem) => void;
}

export interface MentionListRef {
  onKeyDown: (props: { event: KeyboardEvent }) => boolean;
}

// ---------------------------------------------------------------------------
// Group items by section
// ---------------------------------------------------------------------------

interface MentionGroup {
  label: string;
  items: MentionItem[];
}

function getFirstEnabledIndex(items: MentionItem[]): number {
  return items.findIndex((item) => !item.disabled);
}

function getNextEnabledIndex(
  items: MentionItem[],
  currentIndex: number,
  direction: 1 | -1,
): number {
  if (items.length === 0) return -1;
  for (let offset = 1; offset <= items.length; offset += 1) {
    const nextIndex = (currentIndex + direction * offset + items.length) % items.length;
    if (!items[nextIndex]?.disabled) {
      return nextIndex;
    }
  }
  return -1;
}

function groupItems(items: MentionItem[]): MentionGroup[] {
  const users: MentionItem[] = [];
  const issues: MentionItem[] = [];

  for (const item of items) {
    if (item.type === "issue") {
      issues.push(item);
    } else {
      users.push(item);
    }
  }

  const groups: MentionGroup[] = [];
  if (users.length > 0) groups.push({ label: "Users", items: users });
  if (issues.length > 0) groups.push({ label: "Issues", items: issues });
  return groups;
}

// ---------------------------------------------------------------------------
// MentionList — the popup rendered inside the editor
// ---------------------------------------------------------------------------

const MentionList = forwardRef<MentionListRef, MentionListProps>(
  function MentionList({ items, command }, ref) {
    const [selectedIndex, setSelectedIndex] = useState(() => getFirstEnabledIndex(items));
    const itemRefs = useRef<(HTMLButtonElement | null)[]>([]);

    useEffect(() => {
      setSelectedIndex(getFirstEnabledIndex(items));
    }, [items]);

    useEffect(() => {
      itemRefs.current[selectedIndex]?.scrollIntoView({ block: "nearest" });
    }, [selectedIndex]);

    const selectItem = useCallback(
      (index: number) => {
        const item = items[index];
        if (item && !item.disabled) command(item);
      },
      [items, command],
    );

    useImperativeHandle(ref, () => ({
      onKeyDown: ({ event }) => {
        if (event.key === "ArrowUp") {
          setSelectedIndex((i) => getNextEnabledIndex(items, i, -1));
          return true;
        }
        if (event.key === "ArrowDown") {
          setSelectedIndex((i) => getNextEnabledIndex(items, i, 1));
          return true;
        }
        if (event.key === "Enter") {
          if (selectedIndex >= 0) {
            selectItem(selectedIndex);
          }
          return true;
        }
        return false;
      },
    }));

    if (items.length === 0) {
      return (
        <div className="rounded-md border bg-popover p-2 text-xs text-muted-foreground shadow-md">
          No results
        </div>
      );
    }

    const groups = groupItems(items);

    // Build a flat index mapping: globalIndex → item
    let globalIndex = 0;

    return (
      <div className="rounded-md border bg-popover py-1 shadow-md w-72 max-h-[300px] overflow-y-auto">
        {groups.map((group) => (
          <div key={group.label}>
            <div className="px-3 py-1.5 text-xs font-medium text-muted-foreground">
              {group.label}
            </div>
            {group.items.map((item) => {
              const idx = globalIndex++;
              return (
                <MentionRow
                  key={`${item.type}-${item.id}`}
                  item={item}
                  selected={idx === selectedIndex}
                  onSelect={() => selectItem(idx)}
                  buttonRef={(el) => { itemRefs.current[idx] = el; }}
                />
              );
            })}
          </div>
        ))}
      </div>
    );
  },
);

// ---------------------------------------------------------------------------
// MentionRow — single item in the list
// ---------------------------------------------------------------------------

function MentionRow({
  item,
  selected,
  onSelect,
  buttonRef,
}: {
  item: MentionItem;
  selected: boolean;
  onSelect: () => void;
  buttonRef: (el: HTMLButtonElement | null) => void;
}) {
  if (item.type === "issue") {
    return (
      <button
        ref={buttonRef}
        className={`flex w-full items-center gap-2.5 px-3 py-1.5 text-left text-xs transition-colors ${
          selected ? "bg-accent" : "hover:bg-accent/50"
        }`}
        onClick={onSelect}
      >
        {item.status && (
          <StatusIcon status={item.status} className="h-3.5 w-3.5 shrink-0" />
        )}
        <span className="shrink-0 text-muted-foreground">{item.label}</span>
        {item.description && (
          <span className="truncate text-muted-foreground">{item.description}</span>
        )}
      </button>
    );
  }

  return (
    <button
      ref={buttonRef}
      disabled={item.disabled}
      aria-disabled={item.disabled}
      className={`flex w-full items-center gap-2.5 px-3 py-1.5 text-left text-xs transition-colors ${
        item.disabled
          ? "cursor-not-allowed opacity-60"
          : selected
            ? "bg-accent"
            : "hover:bg-accent/50"
      }`}
      onClick={onSelect}
    >
      <ActorAvatar
        actorType={item.type === "all" ? "member" : item.type}
        actorId={item.id}
        size={20}
      />
      <div className="min-w-0 flex-1">
        <div className="flex items-center gap-2">
          <span className="truncate font-medium">{item.label}</span>
          {item.type === "agent" && (
            <Badge variant="outline" className="ml-auto text-[10px] h-4 px-1.5">Agent</Badge>
          )}
        </div>
        {item.disabledReason && (
          <div className="truncate text-[11px] text-muted-foreground">
            {item.disabledReason}
          </div>
        )}
      </div>
    </button>
  );
}

// ---------------------------------------------------------------------------
// Suggestion config factory
// ---------------------------------------------------------------------------

export function createMentionSuggestion(): Omit<
  SuggestionOptions<MentionItem>,
  "editor"
> {
  return {
    items: ({ query }) => {
      const { members, agents } = useWorkspaceStore.getState();
      const { runtimes } = useRuntimeStore.getState();
      const { issues } = useIssueStore.getState();
      const q = query.toLowerCase();

      // Show "All members" option when query is empty or matches "all"
      const allItem: MentionItem[] =
        "all members".includes(q) || "all".includes(q)
          ? [{ id: "all", label: "All members", type: "all" as const }]
          : [];

      const memberItems: MentionItem[] = members
        .filter((m) => m.name.toLowerCase().includes(q))
        .map((m) => ({
          id: m.user_id,
          label: m.name,
          type: "member" as const,
        }));

      const agentItems: MentionItem[] = agents
        .filter((a) => {
          if (a.archived_at || !a.name.toLowerCase().includes(q)) return false;
          return true;
        })
        .map((a) => {
          let disabledReason: string | undefined;
          if (!hasCompleteAgentDispatchDefaults(a)) {
            disabledReason = "Configure default provider and environment first";
          } else if (!runtimes.some(
            (rt) =>
              rt.daemon_ref === a.default_daemon_id &&
              rt.provider === a.default_provider &&
              rt.status === "online",
          )) {
            disabledReason = "Default provider/environment is offline";
          }

          return {
            id: a.id,
            label: a.name,
            type: "agent" as const,
            disabled: !!disabledReason,
            disabledReason,
          };
        })
        .sort((a, b) => Number(a.disabled) - Number(b.disabled));

      const issueItems: MentionItem[] = issues
        .filter(
          (i) =>
            i.identifier.toLowerCase().includes(q) ||
            i.title.toLowerCase().includes(q),
        )
        .map((i) => ({
          id: i.id,
          label: i.identifier,
          type: "issue" as const,
          description: i.title,
          status: i.status as IssueStatus,
        }));

      return [...allItem, ...memberItems, ...agentItems, ...issueItems].slice(0, 10);
    },

    render: () => {
      let renderer: ReactRenderer<MentionListRef> | null = null;
      let popup: HTMLDivElement | null = null;

      return {
        onStart: (props: SuggestionProps<MentionItem>) => {
          renderer = new ReactRenderer(MentionList, {
            props: { items: props.items, command: props.command },
            editor: props.editor,
          });

          popup = document.createElement("div");
          popup.style.position = "fixed";
          popup.style.zIndex = "50";
          popup.appendChild(renderer.element);
          document.body.appendChild(popup);

          updatePosition(popup, props.clientRect);
        },

        onUpdate: (props: SuggestionProps<MentionItem>) => {
          renderer?.updateProps({
            items: props.items,
            command: props.command,
          });
          if (popup) updatePosition(popup, props.clientRect);
        },

        onKeyDown: (props: { event: KeyboardEvent }) => {
          if (props.event.key === "Escape") {
            cleanup();
            return true;
          }
          return renderer?.ref?.onKeyDown(props) ?? false;
        },

        onExit: () => {
          cleanup();
        },
      };

      function updatePosition(
        el: HTMLDivElement,
        clientRect: (() => DOMRect | null) | null | undefined,
      ) {
        if (!clientRect) return;
        const virtualEl = {
          getBoundingClientRect: () => clientRect() ?? new DOMRect(),
        };
        computePosition(virtualEl, el, {
          placement: "bottom-start",
          strategy: "fixed",
          middleware: [offset(4), flip(), shift({ padding: 8 })],
        }).then(({ x, y }) => {
          el.style.left = `${x}px`;
          el.style.top = `${y}px`;
        });
      }

      function cleanup() {
        renderer?.destroy();
        renderer = null;
        popup?.remove();
        popup = null;
      }
    },
  };
}
