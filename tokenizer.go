package jsonpathcty

import (
	"io"
	"strings"
	. "github.com/clean8s/jsonpathcty/internal"
)

type buffer struct {
	data   []byte
	length int
	index  int

	last  States
	state States
	class Classes
}

const invalidState = -1

const (
	quotes       byte = '"'
	quote        byte = '\''
	coma         byte = ','
	colon        byte = ':'
	backslash    byte = '\\'
	skipS        byte = ' '
	skipN        byte = '\n'
	skipR        byte = '\r'
	skipT        byte = '\t'
	bracketL     byte = '['
	bracketR     byte = ']'
	bracesL      byte = '{'
	bracesR      byte = '}'
	parenthesesL byte = '('
	parenthesesR byte = ')'
	dollar       byte = '$'
	at           byte = '@'
	dot          byte = '.'
	asterisk     byte = '*'
	plus         byte = '+'
	minus        byte = '-'
	division     byte = '/'
	exclamation  byte = '!'
	caret        byte = '^'
	signL        byte = '<'
	signG        byte = '>'
	signE        byte = '='
	ampersand    byte = '&'
	pipe         byte = '|'
	question     byte = '?'
)

type (
	rpn    []string
	tokens []string
)


func newTokenizer(body []byte) (b *buffer) {
	b = &buffer{
		length: len(body),
		data:   body,
		last:   GO,
		state:  GO,
	}
	return
}

func (b *buffer) current() (c byte, err error) {
	if b.index < b.length {
		return b.data[b.index], nil
	}
	return 0, io.EOF
}

func (b *buffer) next() (c byte, err error) {
	err = b.step()
	if err != nil {
		return 0, err
	}
	return b.data[b.index], nil
}

func (b *buffer) reset() {
	b.last = GO
}

func (b *buffer) first() (c byte, err error) {
	for ; b.index < b.length; b.index++ {
		c = b.data[b.index]
		if !(c == skipS || c == skipR || c == skipN || c == skipT) {
			return c, nil
		}
	}
	return 0, io.EOF
}

func (b *buffer) backslash() (result bool) {
	for i := b.index - 1; i >= 0; i-- {
		if b.data[i] == backslash {
			result = !result
		} else {
			break
		}
	}
	return
}

func (b *buffer) skip(s byte) error {
	for ; b.index < b.length; b.index++ {
		if b.data[b.index] == s && !b.backslash() {
			return nil
		}
	}
	return io.EOF
}

func (b *buffer) skipAny(s map[byte]bool) error {
	for ; b.index < b.length; b.index++ {
		if s[b.data[b.index]] && !b.backslash() {
			return nil
		}
	}
	return io.EOF
}

// if token is true - skip error from StateTransitionTable, just stop on unknown state
func (b *buffer) numeric(token bool) error {
	if token {
		b.last = GO
	}
	for ; b.index < b.length; b.index++ {
		b.class = b.getClasses(quotes)
		if b.class == invalidState {
			return b.errorSymbol()
		}
		b.state = StateTransitionTable[b.last][b.class]
		if b.state == invalidState {
			if token {
				break
			}
			return b.errorSymbol()
		}
		if b.state < invalidState {
			return nil
		}
		if b.state < MI || b.state > E3 {
			return nil
		}
		b.last = b.state
	}
	if b.last != ZE && b.last != IN && b.last != FR && b.last != E3 {
		return b.errorSymbol()
	}
	return nil
}

func (b *buffer) getClasses(search byte) Classes {
	if b.data[b.index] >= 128 {
		return C_ETC
	}
	if search == quote {
		return QuoteAsciiClasses[b.data[b.index]]
	}
	return AsciiClasses[b.data[b.index]]
}

func (b *buffer) string(search byte, token bool) error {
	if token {
		b.last = GO
	}
	for ; b.index < b.length; b.index++ {
		b.class = b.getClasses(search)

		if b.class == invalidState {
			return b.errorSymbol()
		}
		b.state = StateTransitionTable[b.last][b.class]
		if b.state == invalidState {
			return b.errorSymbol()
		}
		if b.state < invalidState {
			return nil
		}
		b.last = b.state
	}
	return b.errorSymbol()
}

func (b *buffer) step() error {
	if b.index+1 < b.length {
		b.index++
		return nil
	}
	return io.EOF
}

// reads until the end of the token e.g.: `@.length`, `@['foo'].bar[(@.length - 1)].baz`
func (b *buffer) token() (err error) {
	var (
		c     byte
		stack = make([]byte, 0)
		first = b.index
		start int
		find  bool
	)
tokenLoop:
	for ; b.index < b.length; b.index++ {
		c = b.data[b.index]
		switch {
		case c == quotes:
			fallthrough
		case c == quote:
			find = true
			err = b.step()
			if err != nil {
				return b.errorEOF()
			}
			err = b.skip(c)
			if err == io.EOF {
				return b.errorEOF()
			}
		case c == bracketL:
			find = true
			stack = append(stack, c)
		case c == bracketR:
			find = true
			if len(stack) == 0 {
				if first == b.index {
					return b.errorSymbol()
				}
				break tokenLoop
			}
			if stack[len(stack)-1] != bracketL {
				return b.errorSymbol()
			}
			stack = stack[:len(stack)-1]
		case c == parenthesesL:
			find = true
			stack = append(stack, c)
		case c == parenthesesR:
			find = true
			if len(stack) == 0 {
				if first == b.index {
					return b.errorSymbol()
				}
				break tokenLoop
			}
			if stack[len(stack)-1] != parenthesesL {
				return b.errorSymbol()
			}
			stack = stack[:len(stack)-1]
		case c == dot || c == at || c == dollar || c == question || c == asterisk || (c >= 'A' && c <= 'z') || (c >= '0' && c <= '9'): // standard token name
			find = true
			continue
		case len(stack) != 0:
			find = true
			continue
		case c == minus || c == plus:
			if !find {
				find = true
				start = b.index
				err = b.numeric(true)
				if err == nil || err == io.EOF {
					b.index--
					continue
				}
				b.index = start
			}
			fallthrough
		default:
			break tokenLoop
		}
	}
	if len(stack) != 0 {
		return b.errorEOF()
	}
	if first == b.index {
		return b.step()
	}
	if b.index >= b.length {
		return io.EOF
	}
	return nil
}

// Builder for `Reverse Polish notation`
func (b *buffer) rpn() (result rpn, err error) {
	var (
		c        byte
		start    int
		temp     string
		current  string
		found    bool
		variable bool
		stack    = make([]string, 0)
	)
	for {
		b.reset()
		c, err = b.first()
		if err != nil {
			break
		}
		switch true {
		case c == asterisk || c == division || c == minus || c == plus || c == caret || c == ampersand || c == pipe || c == signL || c == signG || c == signE || c == exclamation: // operations
			if variable {
				variable = false
				current = string(c)

				c, err = b.next()
				if err == nil {
					temp = current + string(c)
					if priority[temp] != 0 {
						current = temp
					} else {
						b.index--
					}
				} else {
					err = nil
				}

				for len(stack) > 0 {
					temp = stack[len(stack)-1]
					found = false
					if temp[0] >= 'A' && temp[0] <= 'z' { // function
						found = true
					} else if priority[temp] != 0 { // operation
						if priority[temp] > priority[current] {
							found = true
						} else if priority[temp] == priority[current] && !rightOp[temp] {
							found = true
						}
					}

					if found {
						stack = stack[:len(stack)-1]
						result = append(result, temp)
					} else {
						break
					}
				}
				stack = append(stack, current)
				break
			}
			if c != minus && c != plus {
				return nil, b.errorSymbol()
			}
			fallthrough // for numbers like `-1e6`
		case (c >= '0' && c <= '9') || c == '.': // numbers
			variable = true
			start = b.index
			err = b.numeric(true)
			if err != nil {
				return nil, err
			}
			current = string(b.data[start:b.index])
			result = append(result, current)
			b.index--
		case c == quotes: // string
			fallthrough
		case c == quote: // string
			variable = true
			start = b.index
			err = b.string(c, true)
			if err != nil {
				return nil, b.errorEOF()
			}
			current = string(b.data[start : b.index+1])
			result = append(result, current)
		case c == dollar || c == at: // variable : like @.length , $.expensive, etc.
			variable = true
			start = b.index
			err = b.token()
			if err != nil {
				if err != io.EOF {
					return nil, err
				}
			}
			current = string(b.data[start:b.index])
			result = append(result, current)
			if err != nil {
				err = nil
			} else {
				b.index--
			}
		case c == parenthesesL: // (
			variable = false
			current = string(c)
			stack = append(stack, current)
		case c == parenthesesR: // )
			variable = true
			found = false
			for len(stack) > 0 {
				temp = stack[len(stack)-1]
				stack = stack[:len(stack)-1]
				if temp == "(" {
					found = true
					break
				}
				result = append(result, temp)
			}
			if !found { // have no parenthesesL
				return nil, errorRequest("formula has no left parentheses")
			}
		default: // prefix functions or etc.
			start = b.index
			variable = true
			for ; b.index < b.length; b.index++ {
				c = b.data[b.index]
				if c == parenthesesL { // function detection, example: sin(...), round(...), etc.
					variable = false
					break
				}
				if c < 'A' || c > 'z' {
					if !(c >= '0' && c <= '9') && c != '_' { // constants detection, example: true, false, null, PI, e, etc.
						break
					}
				}
			}
			current = strings.ToLower(string(b.data[start:b.index]))
			b.index--
			if !variable {
				if _, found = functions[current]; !found {
					return nil, errorRequest("wrong formula, '%s' is not a function", current)
				}
				stack = append(stack, current)
			} else {
				if _, found = constants[current]; !found {
					return nil, errorRequest("wrong formula, '%s' is not a constant", current)
				}
				result = append(result, current)
			}
		}
		err = b.step()
		if err != nil {
			break
		}
	}
	if err == io.EOF {
		err = nil // only io.EOF can be here
	}

	for len(stack) > 0 {
		temp = stack[len(stack)-1]
		_, ok := functions[temp]
		if priority[temp] == 0 && !ok { // operations only
			return nil, errorRequest("wrong formula, '%s' is not an operation or function", temp)
		}
		result = append(result, temp)
		stack = stack[:len(stack)-1]
	}

	if len(result) == 0 {
		return nil, b.errorEOF()
	}

	return
}

func (b *buffer) tokenize() (result tokens, err error) {
	var (
		c        byte
		start    int
		temp     string
		current  string
		variable bool
	)
	for {
		b.reset()
		c, err = b.first()
		if err != nil {
			break
		}
		switch true {
		case priorityChar[c]: // operations
			if variable || (c != minus && c != plus) {
				variable = false
				current = string(c)

				c, err = b.next()
				if err == nil {
					temp = current + string(c)
					if priority[temp] != 0 {
						current = temp
					} else {
						b.index--
					}
				} else {
					err = nil
				}

				result = append(result, current)
				break
			}
			fallthrough // for numbers like `-1e6`
		case (c >= '0' && c <= '9') || c == dot: // numbers
			variable = true
			start = b.index
			err = b.numeric(true)
			if err != nil && err != io.EOF {
				if c == dot {
					err = nil
					result = append(result, ".")
					b.index = start
					break
				}
				return nil, err
			}
			current = string(b.data[start:b.index])
			result = append(result, current)
			b.index--
		case c == quotes: // string
			fallthrough
		case c == quote: // string
			variable = true
			start = b.index
			err = b.string(c, true)
			if err != nil {
				return nil, b.errorEOF()
			}
			current = string(b.data[start : b.index+1])
			result = append(result, current)
		case c == dollar || c == at: // variable : like @.length , $.expensive, etc.
			variable = true
			start = b.index
			err = b.token()
			if err != nil {
				if err != io.EOF {
					return nil, err
				}
			}
			current = string(b.data[start:b.index])
			result = append(result, current)
			if err != nil {
				err = nil
			} else {
				b.index--
			}
		case c == parenthesesL: // (
			variable = false
			current = string(c)
			result = append(result, current)
		case c == parenthesesR: // )
			variable = true
			current = string(c)
			result = append(result, current)
		default: // prefix functions or etc.
			start = b.index
			variable = true
			for ; b.index < b.length; b.index++ {
				c = b.data[b.index]
				if c == parenthesesL { // function detection, example: sin(...), round(...), etc.
					variable = false
					break
				}
				if c < 'A' || c > 'z' {
					if !(c >= '0' && c <= '9') && c != '_' { // constants detection, example: true, false, null, PI, e, etc.
						break
					}
				}
			}
			if start == b.index {
				err = b.step()
				if err != nil {
					err = nil
					current = strings.ToLower(string(b.data[start : b.index+1]))
				} else {
					current = strings.ToLower(string(b.data[start:b.index]))
					b.index--
				}
			} else {
				current = strings.ToLower(string(b.data[start:b.index]))
				b.index--
			}
			result = append(result, current)
		}
		err = b.step()
		if err != nil {
			break
		}
	}

	if err == io.EOF {
		err = nil
	}

	return
}

func (b *buffer) errorEOF() error {
	return errorEOF(b)
}

func (b *buffer) errorSymbol() error {
	return errorSymbol(b)
}

func tokenize(cmd string) (result tokens, err error) {
	buf := newTokenizer([]byte(cmd))
	return buf.tokenize()
}

func (t tokens) exists(find string) bool {
	for _, s := range t {
		if s == find {
			return true
		}
	}
	return false
}

func (t tokens) count(find string) int {
	i := 0
	for _, s := range t {
		if s == find {
			i++
		}
	}
	return i
}

func (t tokens) slice(find string) []string {
	n := len(t)
	result := make([]string, 0, t.count(find))
	from := 0
	for i := 0; i < n; i++ {
		if t[i] == find {
			result = append(result, strings.Join(t[from:i], ""))
			from = i + 1
		}
	}
	result = append(result, strings.Join(t[from:n], ""))
	return result
}

func plainString(key string) (string, bool) {
	bString := []byte(key)
	from := len(bString)
	// TODO: Investigate why ajson wrote this the way they did
	if from > 1 && (bString[0] == quotes && bString[from-1] == quotes) {
		return unquote(bString, quotes)
	}
	if from > 1 && (bString[0] == quote && bString[from-1] == quote) {
		return unquote(bString, quote)
	}
	return key, true
}