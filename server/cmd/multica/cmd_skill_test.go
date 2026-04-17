package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSkillSubcommandsRegistered(t *testing.T) {
	subcommands := []string{"list", "get", "create", "update", "delete", "import", "assign"}
	for _, sub := range subcommands {
		t.Run(sub, func(t *testing.T) {
			if _, _, err := skillCmd.Find([]string{sub}); err != nil {
				t.Fatalf("expected skill %s command to exist: %v", sub, err)
			}
		})
	}
}

func TestSkillCmdRegisteredOnRoot(t *testing.T) {
	if _, _, err := rootCmd.Find([]string{"skill"}); err != nil {
		t.Fatalf("expected skill command registered on rootCmd: %v", err)
	}
}

func TestFormatSkillsSummary(t *testing.T) {
	tests := []struct {
		name string
		in   any
		want string
	}{
		{
			name: "nil",
			in:   nil,
			want: "",
		},
		{
			name: "empty slice",
			in:   []any{},
			want: "",
		},
		{
			name: "single skill",
			in: []any{
				map[string]any{"id": "abcdefgh-1234", "name": "Foo"},
			},
			want: "Foo(abcdefgh)",
		},
		{
			name: "multiple skills",
			in: []any{
				map[string]any{"id": "abcdefgh-1234", "name": "Foo"},
				map[string]any{"id": "ijklmnop-5678", "name": "Bar"},
			},
			want: "Foo(abcdefgh), Bar(ijklmnop)",
		},
		{
			name: "missing name uses id",
			in: []any{
				map[string]any{"id": "abcdefgh-1234"},
			},
			want: "abcdefgh",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatSkillsSummary(tt.in)
			if got != tt.want {
				t.Errorf("formatSkillsSummary() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestResolveSkillRef(t *testing.T) {
	all := []map[string]any{
		{"id": "skill-1111", "name": "Frontend Reviewer"},
		{"id": "skill-2222", "name": "Backend Reviewer"},
		{"id": "skill-3333", "name": "Docs Writer"},
	}

	t.Run("exact id match", func(t *testing.T) {
		got, err := resolveSkillRef(all, "skill-2222")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != "skill-2222" {
			t.Errorf("got %q, want skill-2222", got)
		}
	})

	t.Run("case-insensitive name match", func(t *testing.T) {
		got, err := resolveSkillRef(all, "docs")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != "skill-3333" {
			t.Errorf("got %q, want skill-3333", got)
		}
	})

	t.Run("no match", func(t *testing.T) {
		if _, err := resolveSkillRef(all, "nope"); err == nil {
			t.Fatal("expected error for missing match")
		}
	})

	t.Run("ambiguous match", func(t *testing.T) {
		if _, err := resolveSkillRef(all, "reviewer"); err == nil {
			t.Fatal("expected error for ambiguous match")
		}
	})
}

func TestReadSkillFiles(t *testing.T) {
	dir := t.TempDir()
	localPath := filepath.Join(dir, "hello.md")
	if err := os.WriteFile(localPath, []byte("hello"), 0o600); err != nil {
		t.Fatalf("write fixture: %v", err)
	}

	t.Run("nothing returns nil", func(t *testing.T) {
		cmd := testCmd()
		cmd.Flags().StringSlice("file", nil, "")
		out, err := readSkillFiles(cmd)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if out != nil {
			t.Errorf("expected nil, got %v", out)
		}
	})

	t.Run("local-only entry uses basename", func(t *testing.T) {
		cmd := testCmd()
		cmd.Flags().StringSlice("file", []string{localPath}, "")
		out, err := readSkillFiles(cmd)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(out) != 1 || out[0]["path"] != "hello.md" || out[0]["content"] != "hello" {
			t.Errorf("unexpected files payload: %+v", out)
		}
	})

	t.Run("remote=local entry uses remote path", func(t *testing.T) {
		cmd := testCmd()
		cmd.Flags().StringSlice("file", []string{"references/foo.md=" + localPath}, "")
		out, err := readSkillFiles(cmd)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(out) != 1 || out[0]["path"] != "references/foo.md" || out[0]["content"] != "hello" {
			t.Errorf("unexpected files payload: %+v", out)
		}
	})

	t.Run("missing file returns error", func(t *testing.T) {
		cmd := testCmd()
		cmd.Flags().StringSlice("file", []string{filepath.Join(dir, "missing.md")}, "")
		if _, err := readSkillFiles(cmd); err == nil {
			t.Fatal("expected error for missing file")
		}
	})
}
