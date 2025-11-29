package gopyte

// Control characters
const (
	SP  = " "
	NUL = "\x00"
	BEL = "\x07"
	BS  = "\x08"
	HT  = "\x09"
	LF  = "\n"
	VT  = "\x0b"
	FF  = "\x0c"
	CR  = "\r"
	SO  = "\x0e"
	SI  = "\x0f"
	CAN = "\x18"
	SUB = "\x1a"
	ESC = "\x1b"
	DEL = "\x7f"

	CSI_C0 = ESC + "["
	CSI_C1 = "\x9b"
	CSI    = CSI_C0

	ST_C0 = ESC + "\\"
	ST_C1 = "\x9c"
	ST    = ST_C0

	OSC_C0 = ESC + "]"
	OSC_C1 = "\x9d"
	OSC    = OSC_C0
)
