package tui

import (
	"fmt"

	"github.com/donjor/zmux/internal/source"
	"github.com/donjor/zmux/internal/tui/outline"
)

// externalEntryKey returns the stable key for a single catalog entry
// within a group. The tmux session name is stable per-source.
func externalEntryKey(e source.CatalogEntry) string {
	return e.Session
}

// buildExternalRows builds the outline rows for the external source
// section of the picker. The tree's expansion state controls whether
// each group's entries are included.
//
// Emits (in order):
//  1. A RowDivider ("── external ──") if the catalog has any external
//     groups.
//  2. For each group: one RowExternalGroup row.
//  3. If the group is expanded: its RowExternalEntry rows.
//
// Returns nil if the catalog has no external groups.
func buildExternalRows(cat *source.Catalog, tree *outline.Tree) []outline.Row {
	if cat == nil || len(cat.External) == 0 {
		return nil
	}

	rows := []outline.Row{{
		ID:         "divider:external",
		Kind:       outline.RowDivider,
		Label:      "── external ──",
		Selectable: false,
	}}

	for i := range cat.External {
		g := &cat.External[i]
		kind := string(g.Source.Kind)
		key := source.GroupKey(g)
		groupID := outline.ExternalGroupID(kind, key)

		groupLabel := fmt.Sprintf("%s: %s", kind, g.Source.Label)
		if n := len(g.Entries); n > 0 {
			groupLabel += fmt.Sprintf("  (%d)", n)
		}
		if g.Source.Health == source.HealthDegraded {
			groupLabel += "  [degraded]"
		}

		rows = append(rows, outline.Row{
			ID:         groupID,
			Kind:       outline.RowExternalGroup,
			Label:      groupLabel,
			Selectable: true,
			Expanded:   tree.IsExpanded(groupID),
			Data:       g,
		})

		if !tree.IsExpanded(groupID) {
			continue
		}
		for j := range g.Entries {
			entry := g.Entries[j]
			rows = append(rows, outline.Row{
				ID:         outline.ExternalEntryID(kind, externalEntryKey(entry)),
				Kind:       outline.RowExternalEntry,
				Label:      entry.Session,
				Depth:      1,
				ParentID:   groupID,
				Selectable: true,
				Attached:   entry.Attached,
				Data:       &entry,
			})
		}
	}

	return rows
}

// externalEntrySource returns the source group that owns the entry row,
// found by walking the catalog and matching on the entry's stable ID.
// Used by the picker's Enter handler to pick overmind-connect vs
// external-attach semantics.
func externalEntrySource(cat *source.Catalog, row *outline.Row) *source.Source {
	if cat == nil || row == nil {
		return nil
	}
	for i := range cat.External {
		g := &cat.External[i]
		kind := string(g.Source.Kind)
		for j := range g.Entries {
			id := outline.ExternalEntryID(kind, externalEntryKey(g.Entries[j]))
			if id == row.ID {
				return &g.Source
			}
		}
	}
	return nil
}
