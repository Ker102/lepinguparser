package pdf

import (
	"bytes"
	"strconv"
)

// PDF-i objektid. Täisarvud on int64, reaalarvud float64, tõeväärtused bool,
// null on nil.
type (
	// Name on PDF-i nimi ilma kaldkriipsuta, nt "Font".
	Name string
	// String on PDF-i string (baidid, kodeering sõltub kontekstist).
	String string
	// Keyword on operaator või võtmesõna (nt "Tj", "obj", "R").
	Keyword string
	Array   []any
	Dict    map[Name]any
	Ref     struct{ Num, Gen int }
	// Stream on voog: sõnastik ja dekodeerimata andmed.
	Stream struct {
		Dict Dict
		Raw  []byte
		ref  Ref
	}
)

// lekser loeb PDF-i süntaksit baidijadast.
type lekser struct {
	b   []byte
	pos int
}

func onTyhik(c byte) bool {
	return c == ' ' || c == '\n' || c == '\r' || c == '\t' || c == '\f' || c == 0
}

func onEraldaja(c byte) bool {
	switch c {
	case '(', ')', '<', '>', '[', ']', '{', '}', '/', '%':
		return true
	}
	return false
}

func (l *lekser) jataVahele() {
	for l.pos < len(l.b) {
		c := l.b[l.pos]
		if onTyhik(c) {
			l.pos++
		} else if c == '%' {
			for l.pos < len(l.b) && l.b[l.pos] != '\n' && l.b[l.pos] != '\r' {
				l.pos++
			}
		} else {
			return
		}
	}
}

// sona loeb tavalise märgijada (arv või võtmesõna).
func (l *lekser) sona() []byte {
	a := l.pos
	for l.pos < len(l.b) && !onTyhik(l.b[l.pos]) && !onEraldaja(l.b[l.pos]) {
		l.pos++
	}
	return l.b[a:l.pos]
}

type lopp struct{ s string }

// objekt loeb järgmise objekti. Massiivi või sõnastiku lõpu korral tagastab
// lopp-väärtuse; faili lõpus nil, false.
func (l *lekser) objekt(sugavus int) (any, bool) {
	l.jataVahele()
	if l.pos >= len(l.b) || sugavus > 200 {
		return nil, false
	}
	c := l.b[l.pos]
	switch c {
	case '/':
		l.pos++
		return l.nimi(), true
	case '(':
		l.pos++
		return l.literaal(), true
	case '<':
		if l.pos+1 < len(l.b) && l.b[l.pos+1] == '<' {
			l.pos += 2
			return l.sonastik(sugavus), true
		}
		l.pos++
		return l.hex(), true
	case '>':
		if l.pos+1 < len(l.b) && l.b[l.pos+1] == '>' {
			l.pos += 2
			return lopp{">>"}, true
		}
		l.pos++
		return Keyword(">"), true
	case '[':
		l.pos++
		var a Array
		for {
			o, ok := l.objekt(sugavus + 1)
			if !ok {
				return a, true
			}
			if e, ok := o.(lopp); ok {
				if e.s == "]" {
					return a, true
				}
				continue
			}
			a = append(a, o)
		}
	case ']':
		l.pos++
		return lopp{"]"}, true
	case '{', '}', ')':
		l.pos++
		return Keyword(string(c)), true
	}
	w := l.sona()
	if len(w) == 0 {
		l.pos++
		return Keyword(string(c)), true
	}
	if n, ok := arv(w); ok {
		// "12 0 R" viide
		if i, isInt := n.(int64); isInt && i >= 0 {
			save := l.pos
			l.jataVahele()
			w2 := l.sona()
			if g, err := strconv.Atoi(string(w2)); err == nil && len(w2) > 0 {
				l.jataVahele()
				if l.pos < len(l.b) && l.b[l.pos] == 'R' && (l.pos+1 == len(l.b) || onTyhik(l.b[l.pos+1]) || onEraldaja(l.b[l.pos+1])) {
					l.pos++
					return Ref{int(i), g}, true
				}
			}
			l.pos = save
		}
		return n, true
	}
	switch string(w) {
	case "true":
		return true, true
	case "false":
		return false, true
	case "null":
		return nil, true
	}
	return Keyword(w), true
}

func arv(w []byte) (any, bool) {
	c := w[0]
	if !(c >= '0' && c <= '9' || c == '-' || c == '+' || c == '.') {
		return nil, false
	}
	if !bytes.ContainsAny(w, ".") {
		if n, err := strconv.ParseInt(string(w), 10, 64); err == nil {
			return n, true
		}
	}
	// tolerants: "--5", "5-" jms esinevad vigastes failides
	s := string(bytes.TrimLeft(w, "+"))
	for len(s) > 1 && s[0] == '-' && s[1] == '-' {
		s = s[1:]
	}
	if f, err := strconv.ParseFloat(s, 64); err == nil {
		return f, true
	}
	if f, err := strconv.ParseFloat(string(bytes.TrimRight([]byte(s), "-+.")), 64); err == nil {
		return f, true
	}
	return nil, false
}

func (l *lekser) nimi() Name {
	w := l.sona()
	if bytes.IndexByte(w, '#') < 0 {
		return Name(w)
	}
	var out []byte
	for i := 0; i < len(w); i++ {
		if w[i] == '#' && i+2 < len(w) {
			if v, err := strconv.ParseUint(string(w[i+1:i+3]), 16, 8); err == nil {
				out = append(out, byte(v))
				i += 2
				continue
			}
		}
		out = append(out, w[i])
	}
	return Name(out)
}

func (l *lekser) literaal() String {
	var out []byte
	sug := 1
	for l.pos < len(l.b) {
		c := l.b[l.pos]
		l.pos++
		switch c {
		case '(':
			sug++
		case ')':
			sug--
			if sug == 0 {
				return String(out)
			}
		case '\\':
			if l.pos >= len(l.b) {
				return String(out)
			}
			e := l.b[l.pos]
			l.pos++
			switch e {
			case 'n':
				out = append(out, '\n')
			case 'r':
				out = append(out, '\r')
			case 't':
				out = append(out, '\t')
			case 'b':
				out = append(out, '\b')
			case 'f':
				out = append(out, '\f')
			case '\r':
				if l.pos < len(l.b) && l.b[l.pos] == '\n' {
					l.pos++
				}
			case '\n':
			default:
				if e >= '0' && e <= '7' {
					v := int(e - '0')
					for k := 0; k < 2 && l.pos < len(l.b) && l.b[l.pos] >= '0' && l.b[l.pos] <= '7'; k++ {
						v = v*8 + int(l.b[l.pos]-'0')
						l.pos++
					}
					out = append(out, byte(v))
				} else {
					out = append(out, e)
				}
			}
			continue
		}
		out = append(out, c)
	}
	return String(out)
}

func (l *lekser) hex() String {
	var out []byte
	var hi int = -1
	for l.pos < len(l.b) {
		c := l.b[l.pos]
		l.pos++
		if c == '>' {
			break
		}
		v := hexVaartus(c)
		if v < 0 {
			continue
		}
		if hi < 0 {
			hi = v
		} else {
			out = append(out, byte(hi<<4|v))
			hi = -1
		}
	}
	if hi >= 0 {
		out = append(out, byte(hi<<4))
	}
	return String(out)
}

func hexVaartus(c byte) int {
	switch {
	case c >= '0' && c <= '9':
		return int(c - '0')
	case c >= 'a' && c <= 'f':
		return int(c-'a') + 10
	case c >= 'A' && c <= 'F':
		return int(c-'A') + 10
	}
	return -1
}

func (l *lekser) sonastik(sugavus int) Dict {
	d := Dict{}
	for {
		k, ok := l.objekt(sugavus + 1)
		if !ok {
			return d
		}
		if e, ok := k.(lopp); ok {
			if e.s == ">>" {
				return d
			}
			continue
		}
		key, isName := k.(Name)
		if !isName {
			continue
		}
		v, ok := l.objekt(sugavus + 1)
		if !ok {
			return d
		}
		if e, ok := v.(lopp); ok {
			if e.s == ">>" {
				return d
			}
			continue
		}
		d[key] = v
	}
}

// Abifunktsioonid väärtuste lugemiseks.

func arvuna(o any) (float64, bool) {
	switch v := o.(type) {
	case int64:
		return float64(v), true
	case float64:
		return v, true
	}
	return 0, false
}

func taisarv(o any) (int, bool) {
	switch v := o.(type) {
	case int64:
		return int(v), true
	case float64:
		return int(v), true
	}
	return 0, false
}
