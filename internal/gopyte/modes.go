package gopyte

const (
	LNM = 20
	IRM = 4

	// Private modes (shifted left by 5)
	DECTCEM = 25 << 5
	DECSCNM = 5 << 5
	DECOM   = 6 << 5
	DECAWM  = 7 << 5
	DECCOLM = 3 << 5
)
