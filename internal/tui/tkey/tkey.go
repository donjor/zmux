// Package tkey builds Bubble Tea v2 key-press messages for tests.
//
// Bubble Tea v2 replaced the v1 KeyMsg struct (Type + Runes) with a KeyMsg
// interface whose concrete press type is KeyPressMsg{Code, Mod, Text, ...}.
// The convenience KeyType constants (KeyCtrlC, KeyShiftTab, …) were removed:
// modifiers now live in the Mod bitmask. Centralising construction here keeps
// the v2 key semantics in one spot instead of scattered across test files.
package tkey

import tea "charm.land/bubbletea/v2"

// Special keys.
func Enter() tea.KeyPressMsg    { return tea.KeyPressMsg{Code: tea.KeyEnter} }
func Esc() tea.KeyPressMsg      { return tea.KeyPressMsg{Code: tea.KeyEsc} }
func Tab() tea.KeyPressMsg      { return tea.KeyPressMsg{Code: tea.KeyTab} }
func ShiftTab() tea.KeyPressMsg { return tea.KeyPressMsg{Code: tea.KeyTab, Mod: tea.ModShift} }
func Up() tea.KeyPressMsg       { return tea.KeyPressMsg{Code: tea.KeyUp} }
func Down() tea.KeyPressMsg     { return tea.KeyPressMsg{Code: tea.KeyDown} }
func Left() tea.KeyPressMsg     { return tea.KeyPressMsg{Code: tea.KeyLeft} }
func Right() tea.KeyPressMsg    { return tea.KeyPressMsg{Code: tea.KeyRight} }
func Space() tea.KeyPressMsg    { return tea.KeyPressMsg{Code: tea.KeySpace, Text: " "} }

// Ctrl builds a Ctrl-modified key press, e.g. Ctrl('c') == ctrl+c.
func Ctrl(r rune) tea.KeyPressMsg { return tea.KeyPressMsg{Code: r, Mod: tea.ModCtrl} }

// Rune builds a single typed character.
func Rune(r rune) tea.KeyPressMsg { return tea.KeyPressMsg{Code: r, Text: string(r)} }

// Type builds a key press carrying typed text. Code is the first rune; Text
// carries the full string (matching how v2 surfaces typed input).
func Type(s string) tea.KeyPressMsg {
	var code rune
	if r := []rune(s); len(r) > 0 {
		code = r[0]
	}
	return tea.KeyPressMsg{Code: code, Text: s}
}
