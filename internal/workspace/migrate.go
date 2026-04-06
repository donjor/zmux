package workspace

import "time"

// migrateV1toV2 converts the old flat session→workspace map to v2 workspace objects.
func migrateV1toV2(v1 State) StateV2 {
	v2 := emptyStateV2()
	now := time.Now()

	for sess, wsName := range v1.Sessions {
		ws, ok := v2.Workspaces[wsName]
		if !ok {
			ws = &Workspace{
				Name:      wsName,
				Sessions:  []string{},
				CreatedAt: now,
				UpdatedAt: now,
			}
			v2.Workspaces[wsName] = ws
		}
		ws.Sessions = append(ws.Sessions, sess)
		if ws.LastActiveSession == "" {
			ws.LastActiveSession = sess
		}
	}

	// Sort sessions within each workspace for deterministic order.
	for _, ws := range v2.Workspaces {
		sortStrings(ws.Sessions)
	}

	return v2
}
