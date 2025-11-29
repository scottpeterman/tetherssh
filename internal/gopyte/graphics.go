package gopyte

// Text attributes
var TEXT = map[int]string{
	1:  "+bold",
	3:  "+italics",
	4:  "+underscore",
	5:  "+blink",
	7:  "+reverse",
	9:  "+strikethrough",
	22: "-bold",
	23: "-italics",
	24: "-underscore",
	25: "-blink",
	27: "-reverse",
	29: "-strikethrough",
}

// Foreground colors
var FG_ANSI = map[int]string{
	30: "black",
	31: "red",
	32: "green",
	33: "brown",
	34: "blue",
	35: "magenta",
	36: "cyan",
	37: "white",
	39: "default",
}

// Background colors
var BG_ANSI = map[int]string{
	40: "black",
	41: "red",
	42: "green",
	43: "brown",
	44: "blue",
	45: "magenta",
	46: "cyan",
	47: "white",
	49: "default",
}

// High intensity foreground colors
var FG_AIXTERM = map[int]string{
	90: "brightblack",
	91: "brightred",
	92: "brightgreen",
	93: "brightbrown",
	94: "brightblue",
	95: "brightmagenta",
	96: "brightcyan",
	97: "brightwhite",
}

// High intensity background colors
var BG_AIXTERM = map[int]string{
	100: "brightblack",
	101: "brightred",
	102: "brightgreen",
	103: "brightbrown",
	104: "brightblue",
	105: "brightmagenta",
	106: "brightcyan",
	107: "brightwhite",
}

const (
	FG_256 = 38
	BG_256 = 48
)
