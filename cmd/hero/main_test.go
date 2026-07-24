package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestRootCommand_Version(t *testing.T) {
	root := newRootCommand()

	var out bytes.Buffer
	root.SetOut(&out)

	root.SetArgs([]string{"version"})
	if err := root.Execute(); err != nil {
		t.Fatalf("execute version: %v", err)
	}

	output := out.String()
	if !strings.Contains(output, "hero version") {
		t.Errorf("expected 'hero version' in output, got: %q", output)
	}
	if !strings.Contains(output, version) {
		t.Errorf("expected version %q in output, got: %q", version, output)
	}
}

func TestRootCommand_Help(t *testing.T) {
	root := newRootCommand()

	var out bytes.Buffer
	root.SetOut(&out)
	root.SetArgs([]string{"help"})
	// Help doesn't return an error.
	_ = root.Execute()

	output := out.String()
	if !strings.Contains(output, "hero") {
		t.Errorf("expected 'hero' in help output, got: %q", output)
	}
}

func TestRootCommand_AllSubcommandsRegistered(t *testing.T) {
	root := newRootCommand()

	want := []string{"install", "upgrade", "uninstall", "doctor", "status", "variables", "update-models", "version"}
	registered := make(map[string]bool)
	for _, cmd := range root.Commands() {
		registered[cmd.Name()] = true
	}

	for _, name := range want {
		if !registered[name] {
			t.Errorf("command %q is not registered", name)
		}
	}
}

func TestInstallCommand_RequiresToolsFlag(t *testing.T) {
	root := newRootCommand()

	var errOut bytes.Buffer
	root.SetErr(&errOut)
	root.SetArgs([]string{"install"})

	err := root.Execute()
	// install requires --tools, so it should fail.
	if err == nil {
		t.Error("expected error when --tools is not provided")
	}
}

func TestVersionCommand_DefaultIsSet(t *testing.T) {
	if version == "" {
		t.Error("version must not be empty")
	}
}
