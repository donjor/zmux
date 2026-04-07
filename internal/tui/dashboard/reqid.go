package dashboard

import "sync/atomic"

// NextReqID returns a monotonically increasing request ID used by
// dashboard tabs to filter out stale async messages.
//
// Before this helper, each tab declared its own file-scope
// `atomic.Int64` — currentReqCounter, sessionsReqCounter,
// themesReqCounter, settingsReqCounter. The counters were independent
// because each tab only compares incoming messages against its own
// `t.reqID` field, so collisions across tabs aren't possible, but
// maintaining N copies of the same pattern was noise.
//
// One shared monotonic counter satisfies the same contract. It is
// package-level so tab code can call it without plumbing a dependency.
var reqIDCounter atomic.Int64

// NextReqID bumps and returns the next request ID.
func NextReqID() int64 { return reqIDCounter.Add(1) }
