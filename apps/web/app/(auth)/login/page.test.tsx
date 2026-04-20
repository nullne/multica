import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";

// Mock next/navigation
vi.mock("next/navigation", () => ({
  useRouter: () => ({ push: vi.fn(), replace: vi.fn() }),
  usePathname: () => "/login",
  useSearchParams: () => new URLSearchParams(),
}));

// Mock auth store
const mockSignInWithGoogle = vi.fn();
const mockSignInAsDev = vi.fn();
const mockSendEmailSignInLink = vi.fn();
const mockSignInWithEmailLink = vi.fn();
vi.mock("@/features/auth", () => ({
  useAuthStore: (selector: (s: any) => any) =>
    selector({
      user: null,
      isLoading: false,
      signInWithGoogle: mockSignInWithGoogle,
      signInAsDev: mockSignInAsDev,
      sendEmailSignInLink: mockSendEmailSignInLink,
      signInWithEmailLink: mockSignInWithEmailLink,
    }),
}));

// Mock firebase helpers used by the login page directly. The tests do not
// exercise the actual email-link callback flow.
vi.mock("@/features/auth/firebase", () => ({
  isFirebaseEmailLink: vi.fn().mockReturnValue(false),
  getStoredEmailLinkEmail: vi.fn().mockReturnValue(null),
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

  it("shows 'Signing in...' while submitting", async () => {
    mockSignInWithGoogle.mockReturnValueOnce(new Promise(() => {}));
    const user = userEvent.setup();
    render(<LoginPage />);

    await user.click(screen.getByRole("button", { name: "Continue with Google" }));

    await waitFor(() => {
      expect(screen.getByText("Signing in...")).toBeInTheDocument();
    });
  });

  it("shows helper copy for Firebase auth", () => {
    render(<LoginPage />);

    expect(
      screen.getByText("Sign in with Google or get a one-time link by email.")
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

  it("sends a passwordless email link and shows the inbox screen", async () => {
    mockSendEmailSignInLink.mockResolvedValueOnce(undefined);
    const user = userEvent.setup();
    render(<LoginPage />);

    await user.type(
      screen.getByPlaceholderText("you@example.com"),
      "alice@example.com"
    );
    await user.click(
      screen.getByRole("button", { name: "Email me a sign-in link" })
    );

    await waitFor(() => {
      expect(mockSendEmailSignInLink).toHaveBeenCalledWith(
        "alice@example.com",
        expect.stringMatching(/\/login$/)
      );
      expect(screen.getByText("Check your inbox")).toBeInTheDocument();
    });
  });

  it("shows error when sending the email link fails", async () => {
    mockSendEmailSignInLink.mockRejectedValueOnce(new Error("Quota exceeded"));
    const user = userEvent.setup();
    render(<LoginPage />);

    await user.type(
      screen.getByPlaceholderText("you@example.com"),
      "alice@example.com"
    );
    await user.click(
      screen.getByRole("button", { name: "Email me a sign-in link" })
    );

    await waitFor(() => {
      expect(screen.getByText("Quota exceeded")).toBeInTheDocument();
    });
  });
});
