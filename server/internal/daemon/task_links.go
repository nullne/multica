package daemon

import "regexp"

var githubPRURLPattern = regexp.MustCompile(`https://github\.com/[A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+/pull/[0-9]+`)

func extractPRURL(text string) string {
	if text == "" {
		return ""
	}
	return githubPRURLPattern.FindString(text)
}
