package main

import (
	"bytes"
	"testing"
)

func TestCompletionBashProducesOutput(t *testing.T) {
	var buf bytes.Buffer
	if err := rootCmd.GenBashCompletion(&buf); err != nil {
		t.Fatalf("GenBashCompletion failed: %v", err)
	}
	if buf.Len() == 0 {
		t.Error("expected non-empty bash completion output")
	}
}

func TestCompletionZshProducesOutput(t *testing.T) {
	var buf bytes.Buffer
	if err := rootCmd.GenZshCompletion(&buf); err != nil {
		t.Fatalf("GenZshCompletion failed: %v", err)
	}
	if buf.Len() == 0 {
		t.Error("expected non-empty zsh completion output")
	}
}

func TestCompletionFishProducesOutput(t *testing.T) {
	var buf bytes.Buffer
	if err := rootCmd.GenFishCompletion(&buf, true); err != nil {
		t.Fatalf("GenFishCompletion failed: %v", err)
	}
	if buf.Len() == 0 {
		t.Error("expected non-empty fish completion output")
	}
}

func TestStatusExitsZero(t *testing.T) {
	rootCmd.SetArgs([]string{"status"})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("status command failed: %v", err)
	}
}

func TestHelpExitsZero(t *testing.T) {
	rootCmd.SetArgs([]string{"help"})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("help command failed: %v", err)
	}
}

func TestVersionExitsZero(t *testing.T) {
	rootCmd.SetArgs([]string{"version"})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("version command failed: %v", err)
	}
}
