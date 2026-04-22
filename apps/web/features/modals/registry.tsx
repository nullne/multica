"use client";

import { useEffect } from "react";
import { useModalStore } from "./store";
import { CreateWorkspaceModal } from "./create-workspace";
import { CreateIssueModal } from "./create-issue";

export function ModalRegistry() {
  const modal = useModalStore((s) => s.modal);
  const data = useModalStore((s) => s.data);
  const close = useModalStore((s) => s.close);

  useEffect(() => {
    const handler = (e: KeyboardEvent) => {
      if (e.code === "KeyO" && e.shiftKey && (e.metaKey || e.ctrlKey)) {
        e.preventDefault();
        const store = useModalStore.getState();
        if (store.modal === "create-issue") {
          store.close();
        } else {
          store.open("create-issue");
        }
      }
    };
    window.addEventListener("keydown", handler);
    return () => window.removeEventListener("keydown", handler);
  }, []);

  switch (modal) {
    case "create-workspace":
      return <CreateWorkspaceModal onClose={close} />;
    case "create-issue":
      return <CreateIssueModal onClose={close} data={data} />;
    default:
      return null;
  }
}
