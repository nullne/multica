package handler

import "encoding/json"

const (
	TelegramCategoryAssignmentsMentions = "assignments_mentions"
	TelegramCategoryComments            = "comments"
	TelegramCategoryFieldChanges        = "field_changes"
	TelegramCategoryTaskResults         = "task_results"
	TelegramCategoryReactions           = "reactions"
	TelegramCategoryNewIssues           = "new_issues"
)

var telegramPreferenceCategories = []string{
	TelegramCategoryAssignmentsMentions,
	TelegramCategoryComments,
	TelegramCategoryFieldChanges,
	TelegramCategoryTaskResults,
	TelegramCategoryReactions,
	TelegramCategoryNewIssues,
}

func defaultTelegramPreferences() map[string]bool {
	prefs := make(map[string]bool, len(telegramPreferenceCategories))
	for _, category := range telegramPreferenceCategories {
		prefs[category] = true
	}
	return prefs
}

// NormalizeTelegramPreferences applies the default-on behavior and drops
// unknown categories so stored settings remain stable as UI labels evolve.
func NormalizeTelegramPreferences(input map[string]bool) map[string]bool {
	prefs := defaultTelegramPreferences()
	for _, category := range telegramPreferenceCategories {
		if value, ok := input[category]; ok {
			prefs[category] = value
		}
	}
	return prefs
}

func TelegramPreferencesFromRaw(raw []byte) map[string]bool {
	var prefs map[string]bool
	if len(raw) > 0 {
		_ = json.Unmarshal(raw, &prefs)
	}
	return NormalizeTelegramPreferences(prefs)
}

func encodeTelegramPreferences(input map[string]bool) ([]byte, error) {
	return json.Marshal(NormalizeTelegramPreferences(input))
}

func TelegramPreferenceEnabled(preferences map[string]bool, category string) bool {
	if category == "" {
		return true
	}
	enabled, ok := NormalizeTelegramPreferences(preferences)[category]
	return !ok || enabled
}
