import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";

const mockRouterPush = vi.fn();
const mockRouterReplace = vi.fn();

// Mock next/navigation
vi.mock("next/navigation", () => ({
  useRouter: () => ({ push: mockRouterPush, replace: mockRouterReplace }),
  usePathname: () => "/login",
  useSearchParams: () => new URLSearchParams(),
}));

// Mock auth store
const mockSignInWithGoogle = vi.fn();
const mockCompleteGoogleRedirectSignIn = vi.fn();
vi.mock("@/features/auth", () => ({
  useAuthStore: (selector: (s: any) => any) =>
    selector({
      user: null,
      isLoading: false,
      signInWithGoogle: mockSignInWithGoogle,
      completeGoogleRedirectSignIn: mockCompleteGoogleRedirectSignIn,
    }),
}));

// Mock workspace store
const mockHydrateWorkspace = vi.fn();
vi.mock("@/features/workspace", () => ({
  useWorkspaceStore: (selector: (s: any) => any) =>
    selector({
      hydrateWorkspace: mockHydrateWorkspace,
    }),
}));

// Mock api
vi.mock("@/shared/api", () => ({
  api: {
    listWorkspaces: vi.fn().mockResolvedValue([]),
    setToken: vi.fn(),
    getMe: vi.fn(),
  },
}));

import LoginPage from "./page";

describe("LoginPage", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    localStorage.removeItem("multica_google_redirect_pending");
    mockCompleteGoogleRedirectSignIn.mockResolvedValue(null);
  });

  it("renders login form with email input and continue button", () => {
    render(<LoginPage />);

    expect(screen.getByText("Multica")).toBeInTheDocument();
    expect(screen.getByText("Turn coding agents into real teammates")).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: "Continue with Google" })
    ).toBeInTheDocument();
  });

  it("calls signInWithGoogle when clicking continue", async () => {
    mockSignInWithGoogle.mockResolvedValueOnce({
      token: "token",
      user: { id: "u1", email: "test@multica.ai" },
    });
    const user = userEvent.setup();
    render(<LoginPage />);

    await user.click(screen.getByRole("button", { name: "Continue with Google" }));

    await waitFor(() => {
      expect(mockSignInWithGoogle).toHaveBeenCalled();
    });
  });

  it("completes redirect sign-in when a pending redirect is present", async () => {
    localStorage.setItem("multica_google_redirect_pending", "1");
    mockCompleteGoogleRedirectSignIn.mockResolvedValueOnce({
      token: "token",
      user: { id: "u1", email: "test@multica.ai" },
    });

    render(<LoginPage />);

    await waitFor(() => {
      expect(mockCompleteGoogleRedirectSignIn).toHaveBeenCalled();
    });
  });

  it("shows 'Signing in...' while submitting", async () => {
    mockSignInWithGoogle.mockReturnValueOnce(new Promise(() => {}));
    const user = userEvent.setup();
    render(<LoginPage />);

    await user.click(screen.getByRole("button", { name: "Continue with Google" }));

    await waitFor(() => {
      expect(screen.getByText("Signing in...")).toBeInTheDocument();
    });
  });

  it("shows helper copy for Firebase Google auth", () => {
    render(<LoginPage />);

    expect(
      screen.getByText("Sign in with your Firebase-enabled Google account.")
    ).toBeInTheDocument();
  });

  it("shows error when Google sign-in fails", async () => {
    mockSignInWithGoogle.mockRejectedValueOnce(new Error("Network error"));
    const user = userEvent.setup();
    render(<LoginPage />);

    await user.click(screen.getByRole("button", { name: "Continue with Google" }));

    await waitFor(() => {
      expect(screen.getByText("Network error")).toBeInTheDocument();
    });
  });
});
