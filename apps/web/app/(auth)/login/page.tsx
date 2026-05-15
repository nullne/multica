import { LoginPageClient } from "./login-page-client";

type LoginPageProps = {
  searchParams?: Promise<Record<string, string | string[] | undefined>>;
};

function firstParam(value: string | string[] | undefined): string | null {
  if (Array.isArray(value)) {
    return value[0] ?? null;
  }
  return value ?? null;
}

export default async function LoginPage({ searchParams }: LoginPageProps) {
  const params = (await searchParams) ?? {};

  return (
    <LoginPageClient
      cliCallback={firstParam(params.cli_callback)}
      cliState={firstParam(params.cli_state) ?? ""}
      nextPath={firstParam(params.next) ?? "/home"}
    />
  );
}
