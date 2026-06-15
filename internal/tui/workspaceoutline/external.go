package workspaceoutline

import (
	"fmt"

	"github.com/sahilm/fuzzy"

	"github.com/donjor/zmux/internal/source"
	"github.com/donjor/zmux/internal/tui/outline"
)

// BuildExternalRows builds the external-source section of the outline: an
// "── external ──" divider followed by one RowExternalGroup per source group
// and, for each expanded group, its RowExternalEntry children.
//
// query filters the section: "" keeps every group/entry (the picker's
// behaviour); a non-empty query keeps a group whose source label or kind
// matches (all its entries show) or whose entries match (only those show), and
// force-expands the survivors (the dashboard's behaviour). The divider is
// emitted only when at least one group survives. Returns nil when the catalog
// has no external groups or none survive the filter.
func BuildExternalRows(cat *source.Catalog, tree *outline.Tree, query string) []outline.Row {
	if cat == nil || len(cat.External) == 0 {
		return nil
	}

	var body []outline.Row
	for i := range cat.External {
		g := &cat.External[i]
		kind := string(g.Source.Kind)
		key := source.GroupKey(g)
		groupID := outline.ExternalGroupID(kind, key)

		groupMatch := query == "" || matchExternal(query, g.Source.Label) || matchExternal(query, kind)
		var matchingEntries []int
		if query != "" && !groupMatch {
			for j := range g.Entries {
				if matchExternal(query, g.Entries[j].Session) {
					matchingEntries = append(matchingEntries, j)
				}
			}
			if len(matchingEntries) == 0 {
				continue
			}
		}

		label := fmt.Sprintf("%s: %s", kind, g.Source.Label)
		if n := len(g.Entries); n > 0 {
			label += fmt.Sprintf("  (%d)", n)
		}
		if g.Source.Health == source.HealthDegraded {
			label += "  [degraded]"
		}

		expanded := tree.IsExpanded(groupID)
		if query != "" {
			expanded = true
		}

		body = append(body, outline.Row{
			ID:         groupID,
			Kind:       outline.RowExternalGroup,
			Label:      label,
			Selectable: true,
			Expanded:   expanded,
			Data:       g,
		})

		if !expanded {
			continue
		}

		emit := func(j int) {
			entry := g.Entries[j]
			body = append(body, outline.Row{
				ID:         outline.ExternalEntryID(kind, entry.Session),
				Kind:       outline.RowExternalEntry,
				Label:      entry.Session,
				Depth:      1,
				ParentID:   groupID,
				Selectable: true,
				Attached:   entry.Attached,
				Data:       &entry,
			})
		}
		if query != "" && !groupMatch {
			for _, j := range matchingEntries {
				emit(j)
			}
		} else {
			for j := range g.Entries {
				emit(j)
			}
		}
	}

	if len(body) == 0 {
		return nil
	}

	rows := []outline.Row{{
		ID:    "divider:external",
		Kind:  outline.RowDivider,
		Label: "── external ──",
	}}
	return append(rows, body...)
}

// ExternalGroupForRow returns the source group owning an external group or
// entry row, found by matching the row's stable ID against the catalog. Used by
// both surfaces' Enter / action handlers to recover the source.
func ExternalGroupForRow(cat *source.Catalog, row *outline.Row) (*source.SourceGroup, bool) {
	if cat == nil || row == nil {
		return nil, false
	}
	switch row.Kind {
	case outline.RowExternalGroup:
		if g, ok := outline.RowData[source.SourceGroup](row); ok {
			return g, true
		}
	case outline.RowExternalEntry:
		for i := range cat.External {
			g := &cat.External[i]
			kind := string(g.Source.Kind)
			for j := range g.Entries {
				if outline.ExternalEntryID(kind, g.Entries[j].Session) == row.ID {
					return g, true
				}
			}
		}
	}
	return nil, false
}

// ExternalSourceForRow returns the source owning an external entry/group row.
func ExternalSourceForRow(cat *source.Catalog, row *outline.Row) *source.Source {
	if g, ok := ExternalGroupForRow(cat, row); ok {
		return &g.Source
	}
	return nil
}

// matchExternal reports whether target fuzzy-matches the (non-empty) query.
func matchExternal(query, target string) bool {
	if query == "" {
		return true
	}
	return len(fuzzy.Find(query, []string{target})) > 0
}
