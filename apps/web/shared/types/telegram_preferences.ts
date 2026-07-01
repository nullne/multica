export const telegramPreferenceCategories = [
  {
    id: "assignments_mentions",
    label: "Assignments & mentions",
    description: "Direct assignments, unassignments, and @mentions.",
  },
  {
    id: "comments",
    label: "Comments",
    description: "New comments on subscribed issues.",
  },
  {
    id: "field_changes",
    label: "Field changes",
    description: "Status, priority, due date, and assignee updates.",
  },
  {
    id: "task_results",
    label: "Task results",
    description: "Agent task completions and failures.",
  },
  {
    id: "reactions",
    label: "Reactions",
    description: "Emoji reactions on issues and comments.",
  },
  {
    id: "new_issues",
    label: "New issues",
    description: "Issues created in the workspace group.",
  },
] as const;

export type TelegramPreferenceCategory = typeof telegramPreferenceCategories[number]["id"];
export type TelegramNotificationPreferences = Record<TelegramPreferenceCategory, boolean>;

export function defaultTelegramPreferences(): TelegramNotificationPreferences {
  return Object.fromEntries(
    telegramPreferenceCategories.map((category) => [category.id, true]),
  ) as TelegramNotificationPreferences;
}

export function normalizeTelegramPreferences(
  preferences?: Partial<Record<string, boolean>> | null,
): TelegramNotificationPreferences {
  const normalized = defaultTelegramPreferences();
  for (const category of telegramPreferenceCategories) {
    const value = preferences?.[category.id];
    if (typeof value === "boolean") {
      normalized[category.id] = value;
    }
  }
  return normalized;
}

export const personalTelegramPreferenceCategories = telegramPreferenceCategories.filter(
  (category) => category.id !== "new_issues",
);
