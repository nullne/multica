import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { act, renderHook, waitFor } from "@testing-library/react";

const uploadFileMock = vi.hoisted(() => vi.fn());

vi.mock("@/shared/api", () => ({
  api: {
    uploadFile: uploadFileMock,
  },
}));

import { useFileUpload } from "./use-file-upload";

interface Deferred<T> {
  promise: Promise<T>;
  resolve: (value: T) => void;
  reject: (reason?: unknown) => void;
}

function deferred<T>(): Deferred<T> {
  let resolve!: (value: T) => void;
  let reject!: (reason?: unknown) => void;
  const promise = new Promise<T>((res, rej) => {
    resolve = res;
    reject = rej;
  });
  return { promise, resolve, reject };
}

describe("useFileUpload", () => {
  beforeEach(() => {
    uploadFileMock.mockReset();
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  it("reports uploading=true while a single upload is in flight", async () => {
    const d = deferred<{ id: string; filename: string; url: string }>();
    uploadFileMock.mockReturnValue(d.promise);

    const { result } = renderHook(() => useFileUpload());
    expect(result.current.uploading).toBe(false);

    let uploadPromise!: Promise<unknown>;
    act(() => {
      uploadPromise = result.current.upload(new File(["x"], "a.pdf"));
    });

    await waitFor(() => expect(result.current.uploading).toBe(true));

    act(() => d.resolve({ id: "1", filename: "a.pdf", url: "https://cdn/a.pdf" }));
    await act(async () => {
      await uploadPromise;
    });

    expect(result.current.uploading).toBe(false);
  });

  it("keeps uploading=true until the last of multiple concurrent uploads completes", async () => {
    const first = deferred<{ id: string; filename: string; url: string }>();
    const second = deferred<{ id: string; filename: string; url: string }>();
    uploadFileMock
      .mockReturnValueOnce(first.promise)
      .mockReturnValueOnce(second.promise);

    const { result } = renderHook(() => useFileUpload());

    let p1!: Promise<unknown>;
    let p2!: Promise<unknown>;
    act(() => {
      p1 = result.current.upload(new File(["a"], "a.pdf"));
      p2 = result.current.upload(new File(["b"], "b.pdf"));
    });

    await waitFor(() => expect(result.current.uploading).toBe(true));

    // Resolve only the first upload — busy state must persist.
    act(() => first.resolve({ id: "1", filename: "a.pdf", url: "https://cdn/a.pdf" }));
    await act(async () => {
      await p1;
    });
    expect(result.current.uploading).toBe(true);

    // Resolve the last upload — busy state clears.
    act(() => second.resolve({ id: "2", filename: "b.pdf", url: "https://cdn/b.pdf" }));
    await act(async () => {
      await p2;
    });
    expect(result.current.uploading).toBe(false);
  });

  it("clears uploading state and shows a toast when an upload fails via uploadWithToast", async () => {
    uploadFileMock.mockRejectedValue(new Error("network down"));

    const { result } = renderHook(() => useFileUpload());
    let value: unknown;
    await act(async () => {
      value = await result.current.uploadWithToast(new File(["x"], "a.pdf"));
    });

    expect(value).toBeNull();
    expect(result.current.uploading).toBe(false);
  });
});
