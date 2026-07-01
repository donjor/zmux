package cli

import (
	"fmt"
	"os"
	"time"

	apppkg "github.com/donjor/zmux/internal/app"
	"github.com/donjor/zmux/internal/debug"
	"github.com/donjor/zmux/internal/tabs"
	"github.com/donjor/zmux/internal/tabstate"
	"github.com/spf13/cobra"
)

func newTabPeerCmd(app *apppkg.App) *cobra.Command {
	var (
		targetFlag   string
		sessionFlag  string
		roleFlag     string
		hostTabFlag  string
		hostPaneFlag string
		topicFlag    string
		ttlFlag      time.Duration
		sourceFlag   string
		msgFlag      string
		quietFlag    bool
	)

	cmd := &cobra.Command{
		Use:   "peer <start|running|waiting|attention|consumed|park|keep|clear-keep> [target]",
		Short: "Record peer/agent-turn lifecycle metadata",
		Long: `Record semantic peer lifecycle metadata on a tab.

This is the machine-readable peer/turn layer. It complements, but does not
replace, the human-facing glyph written by 'zmux tab state'. Prompt-scoped peers
should use timestamped keep/park metadata here instead of @zmux_keep=1.

Typical flow:
  zmux tab peer start claude-peer --role claude --host-tab ztab_... --topic 'plan review'
  zmux tab peer running claude-peer
  zmux tab peer waiting claude-peer --source claude-stop
  zmux tab peer consumed claude-peer
  zmux tab peer park claude-peer --ttl 30m`,
		Args: cobra.RangeArgs(1, 2),
		RunE: func(cmd *cobra.Command, args []string) error {
			svc := tabstate.New(app.Runner, os.Getenv)
			tgt, err := resolveTabStateTarget(app, svc, tabStateArgs{
				positional: argOrEmpty(args, 1),
				target:     targetFlag,
				session:    sessionFlag,
			})
			if err != nil {
				if quietFlag {
					debug.Log("tab peer --quiet swallowed resolve error", "err", err)
					return nil
				}
				return err
			}
			err = runTabPeerAction(app, svc, tgt, args[0], tabPeerOptions{
				role:     roleFlag,
				hostTab:  hostTabFlag,
				hostPane: hostPaneFlag,
				topic:    topicFlag,
				ttl:      ttlFlag,
				source:   sourceFlag,
				msg:      msgFlag,
			})
			if quietFlag {
				if err != nil {
					debug.Log("tab peer --quiet swallowed error", "err", err)
				}
				return nil
			}
			return err
		},
	}
	cmd.Flags().StringVarP(&targetFlag, "target", "t", "", "target pane/window/tab (overrides positional)")
	cmd.Flags().StringVarP(&sessionFlag, "session", "s", "", "session for tab-name targets (default: current)")
	cmd.Flags().StringVar(&roleFlag, "role", "", "peer role/CLI label (claude, codex, pi, ...)")
	cmd.Flags().StringVar(&hostTabFlag, "host-tab", "", "stable host logical tab id")
	cmd.Flags().StringVar(&hostPaneFlag, "host-pane", "", "host pane id")
	cmd.Flags().StringVar(&topicFlag, "topic", "", "sanitized display topic/title")
	cmd.Flags().DurationVar(&ttlFlag, "ttl", 0, "retention ttl for park/keep (default for park: 30m; keep requires explicit ttl)")
	cmd.Flags().StringVar(&sourceFlag, "source", "peer", "lifecycle source label")
	cmd.Flags().StringVar(&msgFlag, "msg", "", "optional glyph message")
	cmd.Flags().BoolVar(&quietFlag, "quiet", false, "hook mode: never fail, never print")
	return cmd
}

type tabPeerOptions struct {
	role     string
	hostTab  string
	hostPane string
	topic    string
	ttl      time.Duration
	source   string
	msg      string
}

func requirePeerScope(app *apppkg.App, paneID string) error {
	scope, err := app.Runner.ShowPaneOption(paneID, tabs.OptScope)
	if err != nil {
		return err
	}
	if scope != tabs.ScopePeer {
		return fmt.Errorf("pane %s is not a peer tab (scope=%q); run `zmux tab peer start` first", paneID, scope)
	}
	return nil
}

func runTabPeerAction(app *apppkg.App, svc *tabstate.Service, tgt tabstate.Target, action string, o tabPeerOptions) error {
	now := time.Now()
	source := o.source
	if source == "" {
		source = "peer"
	}
	switch action {
	case "start", "running":
		if err := tabs.StampPeer(app.Runner, tgt.PaneID, tabs.PeerMetadata{Role: o.role, HostTab: o.hostTab, HostPane: o.hostPane, Topic: o.topic}, now); err != nil {
			return err
		}
		if err := tabs.SetTurnState(app.Runner, tgt.PaneID, tabs.TurnRunning, now); err != nil {
			return err
		}
		return svc.Set(tgt, tabstate.StateRunning, source, o.msg)
	case "waiting":
		if err := requirePeerScope(app, tgt.PaneID); err != nil {
			return err
		}
		if err := tabs.SetTurnState(app.Runner, tgt.PaneID, tabs.TurnWaiting, now); err != nil {
			return err
		}
		msg := o.msg
		if msg == "" {
			msg = "peer waiting"
		}
		return svc.Set(tgt, tabstate.StateAttention, source, msg)
	case "attention":
		if err := requirePeerScope(app, tgt.PaneID); err != nil {
			return err
		}
		if err := tabs.SetTurnState(app.Runner, tgt.PaneID, tabs.TurnAttention, now); err != nil {
			return err
		}
		return svc.Set(tgt, tabstate.StateAttention, source, o.msg)
	case "consumed":
		if err := requirePeerScope(app, tgt.PaneID); err != nil {
			return err
		}
		if err := tabs.SetTurnState(app.Runner, tgt.PaneID, tabs.TurnConsumed, now); err != nil {
			return err
		}
		return svc.Clear(tgt)
	case "park", "parked":
		if err := requirePeerScope(app, tgt.PaneID); err != nil {
			return err
		}
		if err := tabs.SetTurnState(app.Runner, tgt.PaneID, tabs.TurnParked, now); err != nil {
			return err
		}
		ttl := o.ttl
		if ttl <= 0 {
			ttl = tabs.DefaultPeerParkTTL
		}
		if err := tabs.SetPeerParkUntil(app.Runner, tgt.PaneID, now.Add(ttl)); err != nil {
			return err
		}
		return svc.Clear(tgt)
	case "keep":
		if err := requirePeerScope(app, tgt.PaneID); err != nil {
			return err
		}
		if o.ttl <= 0 {
			return fmt.Errorf("tab peer keep requires --ttl (timestamped keep, not @zmux_keep=1)")
		}
		return tabs.SetPeerKeepUntil(app.Runner, tgt.PaneID, now.Add(o.ttl))
	case "clear-keep":
		if err := requirePeerScope(app, tgt.PaneID); err != nil {
			return err
		}
		return tabs.ClearPeerKeepUntil(app.Runner, tgt.PaneID)
	default:
		return fmt.Errorf("unknown peer lifecycle action %q", action)
	}
}
