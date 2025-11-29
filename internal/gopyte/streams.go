package gopyte

import (
	"log"
	"regexp"
	"strconv"
	"strings"
)

type Stream struct {
	listener Screen
	strict   bool
	useUTF8  bool

	// Parser state
	state           ParserState
	takingPlainText bool
	params          []int
	currentParam    string
	private         bool
	oscParam        string

	// Character sets
	g0Charset []rune
	g1Charset []rune
	charset   int // 0 for G0, 1 for G1

	// Event mappings
	basic  map[string]string
	escape map[string]string
	sharp  map[string]string
	csi    map[string]string
}

type ParserState int

const (
	StateGround ParserState = iota
	StateEscape
	StateCSI
	StateOSC
	StateCharset
	StateSharp
)

var textPattern = regexp.MustCompile(`[^\x00-\x1f\x7f\x9b]+`)

func NewStream(screen Screen, strict bool) *Stream {
	s := &Stream{
		listener:  screen,
		strict:    strict,
		useUTF8:   true,
		state:     StateGround,
		g0Charset: LAT1_MAP,
		g1Charset: VT100_MAP,
		charset:   0,

		// Direct translation of Python dicts
		basic: map[string]string{
			BEL: "bell",
			BS:  "backspace",
			HT:  "tab",
			LF:  "linefeed",
			VT:  "linefeed",
			FF:  "linefeed",
			CR:  "carriage_return",
			SO:  "shift_out",
			SI:  "shift_in",
		},

		escape: map[string]string{
			RIS:   "reset",
			IND:   "index",
			NEL:   "linefeed",
			RI:    "reverse_index",
			HTS:   "set_tab_stop",
			DECSC: "save_cursor",
			DECRC: "restore_cursor",
		},

		sharp: map[string]string{
			DECALN: "alignment_display",
		},

		csi: map[string]string{
			ICH:     "insert_characters",
			CUU:     "cursor_up",
			CUD:     "cursor_down",
			CUF:     "cursor_forward",
			CUB:     "cursor_back",
			CNL:     "cursor_down1",
			CPL:     "cursor_up1",
			CHA:     "cursor_to_column",
			CUP:     "cursor_position",
			ED:      "erase_in_display",
			EL:      "erase_in_line",
			IL:      "insert_lines",
			DL:      "delete_lines",
			DCH:     "delete_characters",
			ECH:     "erase_characters",
			HPR:     "cursor_forward",
			DA:      "report_device_attributes",
			VPA:     "cursor_to_line",
			VPR:     "cursor_down",
			HVP:     "cursor_position",
			TBC:     "clear_tab_stop",
			SM:      "set_mode",
			RM:      "reset_mode",
			SGR:     "select_graphic_rendition",
			DSR:     "report_device_status",
			DECSTBM: "set_margins",
			HPA:     "cursor_to_column",
		},
	}

	return s
}

func (s *Stream) Feed(data string) {
	for i := 0; i < len(data); {
		switch s.state {
		case StateGround:
			char := string(data[i])

			// Check for special characters first
			switch char {
			case ESC:
				s.state = StateEscape
				i++
			case string(CSI_C1):
				s.state = StateCSI
				s.params = []int{}
				s.currentParam = ""
				s.private = false
				i++
			case string(OSC_C1):
				s.state = StateOSC
				s.oscParam = ""
				i++
			default:
				if handler, ok := s.basic[char]; ok {
					// Skip SI/SO in UTF-8 mode
					if (char == SI || char == SO) && s.useUTF8 {
						i++
						continue
					}
					s.dispatch(handler)
					i++
				} else if char != NUL && char != DEL {
					// Collect printable text in a batch
					start := i
					for i < len(data) {
						ch := data[i]
						// Stop at any control character or escape
						if ch < 0x20 || ch == 0x7f || ch == 0x1b || ch == 0x9b {
							break
						}
						i++
					}
					// Draw the batch of text
					if i > start {
						s.draw(data[start:i])
					}
				} else {
					i++
				}
			}

		case StateEscape:
			char := string(data[i])
			switch char {
			case "[":
				s.state = StateCSI
				s.params = []int{}
				s.currentParam = ""
				s.private = false
			case "]":
				s.state = StateOSC
				s.oscParam = ""
			case "#":
				s.state = StateSharp
			case "%":
				s.state = StateCharset
			case "(", ")":
				if i+1 < len(data) {
					code := string(data[i+1])
					if !s.useUTF8 {
						s.defineCharset(code, char)
					}
					i++ // Skip the next character
				}
				s.state = StateGround
			default:
				if handler, ok := s.escape[char]; ok {
					s.dispatch(handler)
				}
				s.state = StateGround
			}
			i++

		case StateSharp:
			char := string(data[i])
			if handler, ok := s.sharp[char]; ok {
				s.dispatch(handler)
			}
			s.state = StateGround
			i++

		case StateCharset:
			// Handle charset selection (simplified)
			char := string(data[i])
			s.selectOtherCharset(char)
			s.state = StateGround
			i++

		case StateCSI:
			char := string(data[i])

			// Handle CSI parameters with defensive parsing
			switch {
			case char == "?":
				s.private = true
			case char >= "0" && char <= "9":
				s.currentParam += char
				// Prevent parameter overflow
				if len(s.currentParam) > 6 {
					s.currentParam = s.currentParam[:6]
				}
			case char == ";":
				val := 0
				if s.currentParam != "" {
					parsed, err := strconv.Atoi(s.currentParam)
					if err == nil && parsed >= 0 {
						val = parsed
						if val > 9999 {
							val = 9999
						}
					}
				}
				s.params = append(s.params, val)
				s.currentParam = ""
				// Prevent too many parameters
				if len(s.params) > 16 {
					s.params = s.params[:16]
				}
			case char == "$":
				// XTerm specific, skip next char
				if i+1 < len(data) {
					i++
				}
				s.state = StateGround
			case strings.Contains(" >", char):
				// Secondary DA, ignore
			case char == CAN || char == SUB:
				// Cancel sequence
				s.draw(char)
				s.state = StateGround
			case strings.Contains("\x07\x08\x09\x0a\x0b\x0c\x0d", char):
				// Allowed in CSI
				if handler, ok := s.basic[char]; ok {
					s.dispatch(handler)
				}
			default:
				// End of CSI sequence
				if s.currentParam != "" {
					parsed, err := strconv.Atoi(s.currentParam)
					if err == nil && parsed >= 0 {
						val := parsed
						if val > 9999 {
							val = 9999
						}
						s.params = append(s.params, val)
					}
				}

				if handler, ok := s.csi[char]; ok {
					s.dispatchCSI(handler, s.params, s.private)
				}

				// Reset state
				s.params = []int{}
				s.currentParam = ""
				s.private = false
				s.state = StateGround
			}
			i++

		case StateOSC:
			char := string(data[i])

			// Look for terminator
			if char == BEL || char == string(ST_C0) || char == string(ST_C1) {
				// Process OSC command
				if len(s.oscParam) > 0 {
					parts := strings.SplitN(s.oscParam, ";", 2)
					if len(parts) == 2 {
						code := parts[0]
						param := parts[1]

						switch code {
						case "0", "1":
							s.listener.SetIconName(param)
						case "2":
							s.listener.SetTitle(param)
						}
					}
				}
				s.state = StateGround
			} else if char == ESC {
				// Check for ST_C0 (ESC \)
				if i+1 < len(data) && string(data[i+1]) == "\\" {
					// Same as above - process OSC
					if len(s.oscParam) > 0 {
						parts := strings.SplitN(s.oscParam, ";", 2)
						if len(parts) == 2 {
							code := parts[0]
							param := parts[1]

							switch code {
							case "0", "1":
								s.listener.SetIconName(param)
							case "2":
								s.listener.SetTitle(param)
							}
						}
					}
					i++ // Skip the backslash
					s.state = StateGround
				}
			} else {
				s.oscParam += char
			}
			i++
		}
	}
}

func (s *Stream) dispatch(handler string) {
	switch handler {
	case "bell":
		s.listener.Bell()
	case "backspace":
		s.listener.Backspace()
	case "tab":
		s.listener.Tab()
	case "linefeed":
		s.listener.Linefeed()
	case "carriage_return":
		s.listener.CarriageReturn()
	case "shift_out":
		s.listener.ShiftOut()
	case "shift_in":
		s.listener.ShiftIn()
	case "reset":
		s.listener.Reset()
	case "index":
		s.listener.Index()
	case "reverse_index":
		s.listener.ReverseIndex()
	case "set_tab_stop":
		s.listener.SetTabStop()
	case "save_cursor":
		s.listener.SaveCursor()
	case "restore_cursor":
		s.listener.RestoreCursor()
	case "alignment_display":
		s.listener.AlignmentDisplay()
	default:
		s.listener.Debug("Unknown handler:", handler)
	}
}

func (s *Stream) dispatchCSI(handler string, params []int, private bool) {
	// DEBUG: Log all cursor-related CSI commands
	if strings.Contains(handler, "cursor") {
		//log.Printf("STREAMS DEBUG: CSI %s, params=%v, private=%v", handler, params, private)
	}

	// Default parameter handling
	if len(params) == 0 {
		params = []int{0}
	}

	switch handler {
	case "cursor_up", "cursor_down", "cursor_forward", "cursor_back",
		"cursor_up1", "cursor_down1":
		count := 1
		if len(params) > 0 && params[0] > 0 {
			count = params[0]
		}

		// Add parameter validation to prevent excessive movements
		if count > 9999 {
			count = 9999
			log.Printf("STREAMS DEBUG: Clamped %s count from %d to 9999", handler, params[0])
		}

		// DEBUG: Log cursor movements
		log.Printf("STREAMS DEBUG: Executing %s(count=%d)", handler, count)

		switch handler {
		case "cursor_up":
			s.listener.CursorUp(count)
		case "cursor_down":
			s.listener.CursorDown(count)
		case "cursor_forward":
			s.listener.CursorForward(count)
		case "cursor_back":
			s.listener.CursorBack(count)
		case "cursor_up1":
			s.listener.CursorUp1(count)
		case "cursor_down1":
			s.listener.CursorDown1(count)
		}

	case "cursor_position":
		line, column := 1, 1
		if len(params) > 0 && params[0] > 0 {
			line = params[0]
		}
		if len(params) > 1 && params[1] > 0 {
			column = params[1]
		}

		// Add bounds validation for cursor positioning
		if line > 9999 {
			line = 9999
		}
		if column > 9999 {
			column = 9999
		}

		// DEBUG: Log cursor positioning
		log.Printf("STREAMS DEBUG: CursorPosition(line=%d, column=%d)", line, column)

		s.listener.CursorPosition(line, column)

	case "cursor_to_column":
		column := 1
		if len(params) > 0 && params[0] > 0 {
			column = params[0]
		}

		// Add bounds validation
		if column > 9999 {
			column = 9999
		}

		// DEBUG: Log cursor to column
		log.Printf("STREAMS DEBUG: CursorToColumn(column=%d)", column)

		s.listener.CursorToColumn(column)

	case "cursor_to_line":
		line := 1
		if len(params) > 0 && params[0] > 0 {
			line = params[0]
		}

		// Add bounds validation
		if line > 9999 {
			line = 9999
		}

		// DEBUG: Log cursor to line
		log.Printf("STREAMS DEBUG: CursorToLine(line=%d)", line)

		s.listener.CursorToLine(line)

	case "erase_in_display":
		how := 0
		if len(params) > 0 {
			how = params[0]
		}
		s.listener.EraseInDisplay(how)

	case "erase_in_line":
		how := 0
		if len(params) > 0 {
			how = params[0]
		}
		s.listener.EraseInLine(how, private)

	case "insert_lines", "delete_lines", "insert_characters",
		"delete_characters", "erase_characters":
		count := 1
		if len(params) > 0 && params[0] > 0 {
			count = params[0]
		}
		switch handler {
		case "insert_lines":
			s.listener.InsertLines(count)
		case "delete_lines":
			s.listener.DeleteLines(count)
		case "insert_characters":
			s.listener.InsertCharacters(count)
		case "delete_characters":
			s.listener.DeleteCharacters(count)
		case "erase_characters":
			s.listener.EraseCharacters(count)
		}

	case "clear_tab_stop":
		how := 0
		if len(params) > 0 {
			how = params[0]
		}
		s.listener.ClearTabStop(how)

	case "set_mode", "reset_mode":
		if handler == "set_mode" {
			s.listener.SetMode(params, private)
		} else {
			s.listener.ResetMode(params, private)
		}

	case "select_graphic_rendition":
		s.listener.SelectGraphicRendition(params)

	case "report_device_attributes":
		mode := 0
		if len(params) > 0 {
			mode = params[0]
		}
		s.listener.ReportDeviceAttributes(mode, private)

	case "report_device_status":
		mode := 0
		if len(params) > 0 {
			mode = params[0]
		}
		s.listener.ReportDeviceStatus(mode)

	case "set_margins":
		var top, bottom int
		if len(params) > 0 {
			top = params[0]
		}
		if len(params) > 1 {
			bottom = params[1]
		}
		s.listener.SetMargins(top, bottom)

	default:
		s.listener.Debug("Unknown CSI handler:", handler, params, private)
	}
}

func (s *Stream) draw(text string) {
	// DEBUG: Log text drawing (but limit to avoid spam)
	if len(text) > 10 {
		// log.Printf("STREAMS DEBUG: draw(text=%q...)", text[:10])
	} else {
		// log.Printf("STREAMS DEBUG: draw(text=%q)", text)
	}

	// Apply character set translation
	if s.charset == 1 {
		text = TranslateCharset(text, s.g1Charset)
	} else {
		text = TranslateCharset(text, s.g0Charset)
	}
	s.listener.Draw(text)
}

func (s *Stream) defineCharset(code, mode string) {
	if charset, ok := MAPS[code]; ok {
		if mode == "(" {
			s.g0Charset = charset
		} else if mode == ")" {
			s.g1Charset = charset
		}
	}
}

func (s *Stream) selectOtherCharset(code string) {
	switch code {
	case "@":
		s.useUTF8 = false
	case "G", "8":
		s.useUTF8 = true
	}
}
