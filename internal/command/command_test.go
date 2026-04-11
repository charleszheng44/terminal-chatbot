package command

import (
	"strings"
	"testing"
)

func TestParse(t *testing.T) {
	cases := []struct {
		name      string
		input     string
		wantName  string
		wantArgs  string
		wantIsCmd bool
	}{
		{"simple command", "/help", "help", "", true},
		{"command with args", "/model claude-sonnet", "model", "claude-sonnet", true},
		{"command with multi-word args", "/system be concise and clear", "system", "be concise and clear", true},
		{"not a command", "hello world", "", "", false},
		{"empty string", "", "", "", false},
		{"leading whitespace", "   /help", "help", "", true},
		{"trailing whitespace arg", "/model  claude  ", "model", "claude", true},
		{"slash only", "/", "", "", true},
		{"whitespace only", "   ", "", "", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			name, args, isCmd := Parse(tc.input)
			if name != tc.wantName || args != tc.wantArgs || isCmd != tc.wantIsCmd {
				t.Errorf("Parse(%q) = (%q, %q, %v), want (%q, %q, %v)",
					tc.input, name, args, isCmd, tc.wantName, tc.wantArgs, tc.wantIsCmd)
			}
		})
	}
}

func TestRegistry_RegisterAndGet(t *testing.T) {
	reg := NewRegistry()
	cmd := Command{
		Name:        "foo",
		Description: "a foo command",
		Handler: func(app *AppState, args string) (string, error) {
			return "foo:" + args, nil
		},
	}
	reg.Register(cmd)

	got := reg.Get("foo")
	if got == nil {
		t.Fatal("Get(foo) returned nil after Register")
	}
	if got.Name != "foo" || got.Description != "a foo command" {
		t.Errorf("retrieved cmd mismatched: %+v", got)
	}
	out, err := got.Handler(nil, "bar")
	if err != nil {
		t.Fatalf("handler err: %v", err)
	}
	if out != "foo:bar" {
		t.Errorf("handler returned %q", out)
	}
}

func TestRegistry_GetMissing(t *testing.T) {
	reg := NewRegistry()
	if got := reg.Get("nope"); got != nil {
		t.Errorf("Get(nope) = %+v, want nil", got)
	}
}

func TestRegistry_AllSorted(t *testing.T) {
	reg := NewRegistry()
	reg.Register(Command{Name: "zebra"})
	reg.Register(Command{Name: "alpha"})
	reg.Register(Command{Name: "mike"})

	all := reg.All()
	if len(all) != 3 {
		t.Fatalf("All() len = %d", len(all))
	}
	want := []string{"alpha", "mike", "zebra"}
	for i, cmd := range all {
		if cmd.Name != want[i] {
			t.Errorf("All()[%d] = %q, want %q", i, cmd.Name, want[i])
		}
	}
}

func TestRegisterAll_IncludesExpectedCommands(t *testing.T) {
	reg := NewRegistry()
	RegisterAll(reg)

	expected := []string{"help", "model", "config", "system", "history", "clear", "copy", "save", "load", "retry", "temp", "exit"}
	for _, name := range expected {
		if reg.Get(name) == nil {
			t.Errorf("command %q not registered by RegisterAll", name)
		}
	}
}

func TestHandleHelp_ListsAllCommands(t *testing.T) {
	out, err := handleHelp(nil, "")
	if err != nil {
		t.Fatalf("handleHelp err: %v", err)
	}
	if !strings.HasPrefix(out, "Available commands:") {
		t.Errorf("handleHelp output missing header: %q", out)
	}
	for _, name := range []string{"help", "model", "clear", "exit", "retry", "temp"} {
		if !strings.Contains(out, "/"+name) {
			t.Errorf("handleHelp output missing command %q:\n%s", name, out)
		}
	}
}
