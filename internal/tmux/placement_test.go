package tmux

import (
	"strings"
	"testing"
)

func TestBuildJoinPaneArgs(t *testing.T) {
	tests := []struct {
		name    string
		opts    JoinPaneOptions
		want    string
		wantErr bool
	}{
		{
			name: "defaults to vertical",
			opts: JoinPaneOptions{Source: "%5", Target: "%9"},
			want: "join-pane -v -s %5 -t %9",
		},
		{
			name: "right with size detached",
			opts: JoinPaneOptions{Source: "%5", Target: "%9", Direction: SplitRight, Size: "40%", Detached: true},
			want: "join-pane -h -d -l 40% -s %5 -t %9",
		},
		{
			name: "up maps to -v -b",
			opts: JoinPaneOptions{Source: "%5", Target: "%9", Direction: SplitUp},
			want: "join-pane -v -b -s %5 -t %9",
		},
		{
			name:    "missing source errors",
			opts:    JoinPaneOptions{Target: "%9"},
			wantErr: true,
		},
		{
			name:    "unknown direction errors",
			opts:    JoinPaneOptions{Source: "%5", Target: "%9", Direction: "diagonal"},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			args, err := buildJoinPaneArgs(tt.opts)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error, got %v", args)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if got := strings.Join(args, " "); got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestBuildBreakPaneArgs(t *testing.T) {
	tests := []struct {
		name    string
		opts    BreakPaneOptions
		want    string
		wantErr bool
	}{
		{
			name: "minimal returns window id",
			opts: BreakPaneOptions{Source: "%5"},
			want: "break-pane -P -F #{window_id} -s %5",
		},
		{
			name: "dock append with name detached",
			opts: BreakPaneOptions{Source: "%5", Target: "__zmux_dock:", Name: "buddy", Detached: true},
			want: "break-pane -P -F #{window_id} -d -n buddy -s %5 -t __zmux_dock:",
		},
		{
			name: "after anchor window",
			opts: BreakPaneOptions{Source: "%5", Target: "work:2", After: true},
			want: "break-pane -P -F #{window_id} -a -s %5 -t work:2",
		},
		{
			name:    "missing source errors",
			opts:    BreakPaneOptions{Target: "work:"},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			args, err := buildBreakPaneArgs(tt.opts)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error, got %v", args)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if got := strings.Join(args, " "); got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}
