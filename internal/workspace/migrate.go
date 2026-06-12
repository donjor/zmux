package workspace

import "time"

type workspaceV2Disk struct {
	Name              string    `toml:"-"`
	RootDir           string    `toml:"root_dir,omitempty"`
	LastActiveSession string    `toml:"last_active_session,omitempty"`
	Sessions          []string  `toml:"sessions"`
	CreatedAt         time.Time `toml:"created_at,omitempty"`
	UpdatedAt         time.Time `toml:"updated_at,omitempty"`
}

type stateV2Disk struct {
	Version    int                         `toml:"version"`
	Workspaces map[string]*workspaceV2Disk `toml:"workspaces"`
}

// migrateV1toV3 converts the old flat session→workspace map to v3 workspace objects.
func migrateV1toV3(v1 State) StateV3 {
	v3 := emptyStateV3()
	now := time.Now()

	for sess, wsName := range v1.Sessions {
		ws, ok := v3.Workspaces[wsName]
		if !ok {
			ws = &Workspace{
				Name:      wsName,
				Sessions:  []WorkspaceSession{},
				CreatedAt: now,
				UpdatedAt: now,
			}
			v3.Workspaces[wsName] = ws
		}
		rec := legacySessionRecord(wsName, sess, now)
		ws.Sessions = appendSessionRecordUnique(ws.Sessions, rec)
		if ws.LastActiveSessionID == "" {
			ws.LastActiveSessionID = rec.ID
			ws.LastActiveSession = rec.Label
		}
	}

	// Sort sessions within each workspace for deterministic order.
	for _, ws := range v3.Workspaces {
		sortSessionRecords(ws.Sessions)
	}

	return v3
}

// migrateV2toV3 converts workspace objects with raw session-name arrays to
// workspace-local session records.
func migrateV2toV3(v2 stateV2Disk) StateV3 {
	v3 := emptyStateV3()
	now := time.Now()
	for wsName, old := range v2.Workspaces {
		if old == nil {
			continue
		}
		ws := &Workspace{
			Name:      wsName,
			RootDir:   old.RootDir,
			Sessions:  []WorkspaceSession{},
			CreatedAt: old.CreatedAt,
			UpdatedAt: old.UpdatedAt,
		}
		if ws.CreatedAt.IsZero() {
			ws.CreatedAt = now
		}
		if ws.UpdatedAt.IsZero() {
			ws.UpdatedAt = now
		}
		for _, raw := range old.Sessions {
			rec := legacySessionRecord(wsName, raw, now)
			ws.Sessions = appendSessionRecordUnique(ws.Sessions, rec)
			if raw == old.LastActiveSession || rec.Label == old.LastActiveSession {
				ws.LastActiveSessionID = rec.ID
				ws.LastActiveSession = rec.Label
			}
		}
		if ws.LastActiveSessionID == "" && len(ws.Sessions) > 0 {
			ws.LastActiveSessionID = ws.Sessions[0].ID
			ws.LastActiveSession = ws.Sessions[0].Label
		}
		v3.Workspaces[wsName] = ws
	}
	return v3
}

func legacySessionRecord(wsName, raw string, now time.Time) WorkspaceSession {
	label := raw
	if parsedWS, parsedLabel, ok := ParseRawSessionName(raw); ok {
		if parsedWS == wsName {
			label = parsedLabel
		}
	} else if raw == "main-"+wsName {
		label = "main"
	}
	rec, err := NewSessionRecord(wsName, label)
	if err != nil {
		id := StableSessionID(wsName, label)
		rec = WorkspaceSession{ID: id, Label: label, TmuxName: raw}
	} else if rec.TmuxName != raw {
		rec.LegacyTmuxName = raw
	}
	rec.CreatedAt = now
	rec.UpdatedAt = now
	return rec
}
