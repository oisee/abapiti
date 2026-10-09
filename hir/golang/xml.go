package golang

// Mirrors the reviewed ABAP abapGit subset, including explicit rejections.
const xmlRuntimeSource = `
type xmlParser struct {
	input  jsString
	cursor int32
}

func xmlFail() { panic(trap{Source: "input is outside the reviewed abapGit XML subset"}) }

var xmlHeader = regexp.MustCompile("^[A-Za-z_][A-Za-z0-9_.:-]*([ \\t\\n\\r\\f]+[A-Za-z_][A-Za-z0-9_.:-]*[ \\t\\n\\r\\f]*=[ \\t\\n\\r\\f]*(\"[^\"]*\"|'[^']*'))*[ \\t\\n\\r\\f]*/?$")

func (p *xmlParser) header() jsString {
	p.cursor++
	start := p.cursor
	quote := int32(0)
	for p.cursor < p.input.length() {
		c := p.input.charCodeAt(p.cursor)
		if quote != 0 {
			if c == quote {
				quote = 0
			}
		} else if c == 34 || c == 39 {
			quote = c
		} else if c == 62 {
			h := p.input[2*start : 2*p.cursor]
			p.cursor++
			return h
		} else if c == 60 {
			xmlFail()
		}
		p.cursor++
	}
	xmlFail()
	return ""
}
func xmlDecode(s jsString) jsString {
	out := jsString("")
	for i := int32(0); i < s.length(); i++ {
		c := s.charAt(i)
		if c != str("&") {
			out += c
			continue
		}
		j := i + 1
		for j < s.length() && s.charAt(j) != str(";") {
			j++
		}
		if j == s.length() {
			xmlFail()
		}
		entity := s[2*i : 2*(j+1)].String()
		switch entity {
		case "&amp;":
			out += str("&")
		case "&lt;":
			out += str("<")
		case "&gt;":
			out += str(">")
		case "&quot;":
			out += str("\"")
		case "&apos;":
			out += str("'")
		default:
			xmlFail()
		}
		i = j
	}
	return out
}
func xmlAdd(d *dynamic, k jsString, v *dynamic) {
	old := d.get(k)
	if old == nil {
		d.put(k, v)
	} else if old.Tag == tagArray {
		old.Value.(*array[*dynamic]).push(v)
	} else {
		d.put(k, &dynamic{tagArray, &array[*dynamic]{Items: []*dynamic{old, v}}})
	}
}
func (p *xmlParser) node(closing jsString) *dynamic {
	d := &dynamic{tagObject, &orderedMap[jsString, *dynamic]{}}
	text := jsString("")
	closed := false
	for p.cursor < p.input.length() {
		start := p.cursor
		for p.cursor < p.input.length() && p.input.charAt(p.cursor) != str("<") {
			p.cursor++
		}
		if p.cursor == p.input.length() && closing == "" {
			break
		}
		text += p.input[2*start : 2*p.cursor]
		if p.cursor == p.input.length() {
			xmlFail()
		}
		h := p.header()
		if h == "" {
			xmlFail()
		}
		if h.startsWith(str("/")) {
			if closing == "" || h[2:] != closing {
				xmlFail()
			}
			closed = true
			break
		}
		if h.startsWith(str("!")) {
			xmlFail()
		}
		if h.startsWith(str("?")) {
			if closing != "" || !h.startsWith(str("?xml ")) || !h.endsWith(str("?")) {
				xmlFail()
			}
			if text != "" {
				d.put(str("#text"), box(xmlDecode(text)))
			}
			text = ""
			xmlAdd(d, str("?xml"), box(jsString("")))
			continue
		}
		if closing == "" {
			text = ""
		}
		n := int32(0)
		for n < h.length() {
			c := h.charCodeAt(n)
			if c == 32 || c == 47 || c == 10 || c == 9 {
				break
			}
			n++
		}
		if n == 0 || !xmlHeader.MatchString(h.String()) {
			xmlFail()
		}
		name := h[:2*n]
		if name == str("__proto__") || name == str("constructor") || name == str("prototype") {
			xmlFail()
		}
		var child *dynamic
		if h.endsWith(str("/")) {
			child = box(jsString(""))
		} else {
			child = p.node(name)
		}
		xmlAdd(d, name, child)
	}
	if closing != "" && !closed {
		xmlFail()
	}
	if closing != "" && len(d.Value.(*orderedMap[jsString, *dynamic]).Entries) == 0 {
		return box(xmlDecode(text))
	}
	if closing != "" && text != "" {
		d.put(str("#text"), box(xmlDecode(text)))
	}
	return d
}
func parseXML(input jsString) *dynamic {
	input = input.replaceAll(str("\r\n"), str("\n")).replaceAll(str("\r"), str("\n"))
	return (&xmlParser{input: input}).node("")
}
`
