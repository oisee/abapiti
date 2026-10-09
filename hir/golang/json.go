package golang

const jsonRuntimeSource = `
// Strict JSON over UTF-16: escaped lone surrogates remain individual units.
type jsonParser struct {
	input  jsString
	cursor int32
}

func jsonFail()             { panic(trap{Source: "input is outside the supported strict JSON subset"}) }
func unit(c int32) jsString { b := []byte{byte(c), byte(c >> 8)}; return jsString(b) }
func (p *jsonParser) peek() int32 {
	if p.cursor >= p.input.length() {
		return -1
	}
	return p.input.charCodeAt(p.cursor)
}
func (p *jsonParser) whitespace() {
	for {
		c := p.peek()
		if c != 32 && c != 9 && c != 10 && c != 13 {
			return
		}
		p.cursor++
	}
}
func (p *jsonParser) take(c int32) {
	if p.peek() != c {
		jsonFail()
	}
	p.cursor++
}
func (p *jsonParser) text() jsString {
	p.take(34)
	out := jsString("")
	for {
		c := p.peek()
		p.cursor++
		if c == 34 {
			return out
		}
		if c < 32 {
			jsonFail()
		}
		if c != 92 {
			out += unit(c)
			continue
		}
		c = p.peek()
		p.cursor++
		switch c {
		case 34, 92, 47:
			out += unit(c)
		case 98:
			out += unit(8)
		case 102:
			out += unit(12)
		case 110:
			out += unit(10)
		case 114:
			out += unit(13)
		case 116:
			out += unit(9)
		case 117:
			v := int32(0)
			for j := 0; j < 4; j++ {
				c = p.peek()
				p.cursor++
				digit := int32(-1)
				switch {
				case c >= 48 && c <= 57:
					digit = c - 48
				case c >= 65 && c <= 70:
					digit = c - 55
				case c >= 97 && c <= 102:
					digit = c - 87
				}
				if digit < 0 {
					jsonFail()
				}
				v = v*16 + digit
			}
			out += unit(v)
		default:
			jsonFail()
		}
	}
}
func (p *jsonParser) literal(s string, d *dynamic) *dynamic {
	for _, c := range s {
		p.take(int32(c))
	}
	return d
}
func (p *jsonParser) value() *dynamic {
	p.whitespace()
	switch p.peek() {
	case 34:
		return box(p.text())
	case 110:
		return p.literal("null", dynNull())
	case 116:
		return p.literal("true", box(true))
	case 102:
		return p.literal("false", box(false))
	case 123:
		p.cursor++
		d := &dynamic{Tag: tagObject, Value: &orderedMap[jsString, *dynamic]{}}
		p.whitespace()
		if p.peek() == 125 {
			p.cursor++
			return d
		}
		for {
			p.whitespace()
			k := p.text()
			p.whitespace()
			p.take(58)
			d.put(k, p.value())
			p.whitespace()
			if p.peek() == 125 {
				p.cursor++
				return d
			}
			p.take(44)
		}
	case 91:
		p.cursor++
		a := &array[*dynamic]{}
		p.whitespace()
		if p.peek() == 93 {
			p.cursor++
			return &dynamic{tagArray, a}
		}
		for {
			a.push(p.value())
			p.whitespace()
			if p.peek() == 93 {
				p.cursor++
				return &dynamic{tagArray, a}
			}
			p.take(44)
		}
	default:
		start := p.cursor
		if p.peek() == 45 {
			p.cursor++
		}
		c := p.peek()
		if c == 48 {
			p.cursor++
		} else if c >= 49 && c <= 57 {
			for p.peek() >= 48 && p.peek() <= 57 {
				p.cursor++
			}
		} else {
			jsonFail()
		}
		if p.peek() == 46 {
			p.cursor++
			if p.peek() < 48 || p.peek() > 57 {
				jsonFail()
			}
			for p.peek() >= 48 && p.peek() <= 57 {
				p.cursor++
			}
		}
		if p.peek() == 101 || p.peek() == 69 {
			p.cursor++
			if p.peek() == 43 || p.peek() == 45 {
				p.cursor++
			}
			if p.peek() < 48 || p.peek() > 57 {
				jsonFail()
			}
			for p.peek() >= 48 && p.peek() <= 57 {
				p.cursor++
			}
		}
		v, err := strconv.ParseFloat(p.input[2*start:2*p.cursor].String(), 64)
		if err != nil {
			jsonFail()
		}
		return box(finite(v))
	}
}
func parseJSON(input jsString) *dynamic {
	p := &jsonParser{input: input}
	d := p.value()
	p.whitespace()
	if p.cursor != input.length() {
		jsonFail()
	}
	return d
}
`
