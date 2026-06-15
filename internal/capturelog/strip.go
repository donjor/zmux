package capturelog

// ansiStripper removes terminal escape/control sequences from a byte stream,
// leaving readable text. It is a state machine so sequences split across
// successive feed() calls (chunk boundaries from pipe-pane) are handled
// correctly — the in-progress state carries over.
//
// Handled: CSI (ESC [ … final 0x40–0x7e), OSC (ESC ] … BEL or ST), charset
// designators (ESC ( / ) / * / + + one byte), and other two-byte ESC sequences.
// Printable bytes plus \n, \t, \r pass through; remaining C0 controls (BEL,
// backspace, …) and DEL are dropped. This is best-effort plain text: it tames
// line-oriented output but cannot meaningfully linearise a fullscreen TUI.
type ansiStripper struct {
	state stripState
}

type stripState int

const (
	stNormal stripState = iota
	stEsc
	stCSI
	stOSC
	stCharset
)

func (a *ansiStripper) feed(p []byte) []byte {
	out := make([]byte, 0, len(p))
	for _, b := range p {
		switch a.state {
		case stNormal:
			switch b {
			case 0x1b: // ESC — start of an escape sequence
				a.state = stEsc
			case '\n', '\t', '\r':
				out = append(out, b)
			default:
				if b >= 0x20 && b != 0x7f { // printable, not DEL
					out = append(out, b)
				}
				// other C0 control bytes (BEL, BS, …) are dropped
			}
		case stEsc:
			switch b {
			case '[':
				a.state = stCSI
			case ']':
				a.state = stOSC
			case '(', ')', '*', '+':
				a.state = stCharset // one designator byte follows
			default:
				a.state = stNormal // simple two-byte ESC seq (ESC c, ESC =, …)
			}
		case stCSI:
			// parameter/intermediate bytes 0x20–0x3f; a final byte 0x40–0x7e ends it
			if b >= 0x40 && b <= 0x7e {
				a.state = stNormal
			}
		case stOSC:
			// terminated by BEL, or by ST (ESC \). On ESC we re-enter stEsc so
			// the trailing '\' is consumed there and emits nothing.
			switch b {
			case 0x07:
				a.state = stNormal
			case 0x1b:
				a.state = stEsc
			}
		case stCharset:
			a.state = stNormal // consume the single charset designator byte
		}
	}
	return out
}
