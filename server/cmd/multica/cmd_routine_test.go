package main

import (
	"testing"

	"github.com/spf13/cobra"
)

func TestRoutineSubcommandsRegistered(t *testing.T) {
	subcommands := []string{"list", "get", "create", "update", "delete", "run", "runs", "token-draft"}
	for _, sub := range subcommands {
		t.Run(sub, func(t *testing.T) {
			if _, _, err := routineCmd.Find([]string{sub}); err != nil {
				t.Fatalf("expected routine %s command to exist: %v", sub, err)
			}
		})
	}
}

func TestRoutineCmdRegisteredOnRoot(t *testing.T) {
	if _, _, err := rootCmd.Find([]string{"routine"}); err != nil {
		t.Fatalf("expected routine command registered on rootCmd: %v", err)
	}
	if _, _, err := rootCmd.Find([]string{"routines"}); err != nil {
		t.Fatalf("expected routines alias registered on rootCmd: %v", err)
	}
}

func TestParseRoutineJSONListFlag(t *testing.T) {
	cmd := &cobra.Command{}
	cmd.Flags().StringArray("trigger-json", nil, "")

	t.Run("single object", func(t *testing.T) {
		if err := cmd.Flags().Set("trigger-json", `{"trigger_type":"schedule","schedule":"0 9 * * *","timezone":"UTC"}`); err != nil {
			t.Fatalf("set flag: %v", err)
		}
		got, err := parseRoutineJSONListFlag(cmd, "trigger-json")
		if err != nil {
			t.Fatalf("parseRoutineJSONListFlag() error = %v", err)
		}
		if len(got) != 1 || got[0]["trigger_type"] != "schedule" || got[0]["timezone"] != "UTC" {
			t.Fatalf("unexpected parsed trigger list: %#v", got)
		}
	})

	t.Run("invalid object", func(t *testing.T) {
		cmd := &cobra.Command{}
		cmd.Flags().StringArray("action-json", []string{"not-json"}, "")
		if _, err := parseRoutineJSONListFlag(cmd, "action-json"); err == nil {
			t.Fatal("expected invalid JSON error")
		}
	})
}
