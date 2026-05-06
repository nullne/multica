import fs from "fs";
import path from "path";

export type ChangelogEntry = {
  version: string;
  date: string;
  title: string;
  changes: string[];
};

// Resolves CHANGELOG.md relative to the monorepo root.
// process.cwd() is apps/web when running `next build` via pnpm --filter.
function changelogPath(): string {
  return path.join(process.cwd(), "../../CHANGELOG.md");
}

export function parseChangelog(): ChangelogEntry[] {
  const content = fs.readFileSync(changelogPath(), "utf-8");
  const entries: ChangelogEntry[] = [];

  // Split into per-version sections on lines starting with "## ["
  const sections = content.split(/^(?=## \[)/m).filter((s) => s.trim());

  for (const section of sections) {
    const [headerLine, ...bodyLines] = section.split("\n");

    // Match: ## [0.1.4] - 2026-04-01
    const headerMatch = (headerLine ?? "").match(
      /^## \[(\d+\.\d+\.\d+)\] - (\d{4}-\d{2}-\d{2})/,
    );
    if (!headerMatch) continue;

    const version = headerMatch[1] ?? "";
    const date = headerMatch[2] ?? "";
    let title = "";
    const changes: string[] = [];

    for (const raw of bodyLines) {
      const line = raw.trim();
      if (line.startsWith("### ")) {
        title = line.slice(4).trim();
      } else if (line.startsWith("- ")) {
        changes.push(line.slice(2).trim());
      }
    }

    entries.push({ version, date, title, changes });
  }

  return entries;
}
