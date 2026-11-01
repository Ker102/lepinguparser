package pdf

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

// font teisendab teksti näitamise operaatorite baidid Unicode'iks ja annab
// iga märgi laiuse.
type font struct {
	nimi     string
	paks     bool
	kaheBait bool
	// koodiruum ToUnicode CMapist (märgikoodide pikkus baitides)
	koodiruum  []koodivahemik
	toUni      map[uint32]string
	kodeering  *[256]rune
	erinevus   map[int]string // /Differences: kood -> tekst
	laiused    map[uint32]float64
	vaikeLaius float64
	// maatriks teisendab glüüfiruumi tekstiruumi (Type3 puhul FontMatrix)
	skaala   float64
	sumbol   bool
	tundmatu int // dekodeerimata märgid
	// standard on standardfondi (Helvetica, Times, Courier) mõõdik, kui
	// failis laiusi ei ole
	standard func(r rune) float64
}

type koodivahemik struct {
	pikkus int
	lo, hi uint32
}

type glyf struct {
	tekst string
	laius float64 // tekstiruumis (fondi suurusega 1)
	tyhik bool    // ühebaidine kood 32 (sõnavahe /Tw)
}

func (r *lugeja) laeFont(o any) *font {
	ref, isRef := o.(Ref)
	if isRef {
		if f, ok := r.fondid[ref]; ok {
			return f
		}
	}
	d := r.f.sonastik(o)
	ft := &font{laiused: map[uint32]float64{}, skaala: 0.001, vaikeLaius: -1}
	if d == nil {
		ft.kodeering = &winAnsi
		return ft
	}
	alam := r.f.nimi(d["Subtype"])
	ft.nimi = string(r.f.nimi(d["BaseFont"]))
	if i := strings.IndexByte(ft.nimi, '+'); i == 6 {
		ft.nimi = ft.nimi[7:] // alamhulga prefiks "ABCDEF+"
	}
	alus := strings.ToLower(ft.nimi)
	ft.paks = strings.Contains(alus, "bold") || strings.Contains(alus, "black") || strings.Contains(alus, "heavy") || strings.Contains(alus, "semibold") || strings.Contains(alus, "demi")
	ft.sumbol = strings.Contains(alus, "symbol") || strings.Contains(alus, "dingbat") || strings.Contains(alus, "wingding")

	var kirjeldus Dict
	if alam == "Type0" {
		ft.kaheBait = true
		ft.vaikeLaius = 1
		if desc := r.f.massiiv(d["DescendantFonts"]); len(desc) > 0 {
			dd := r.f.sonastik(desc[0])
			kirjeldus = r.f.sonastik(dd["FontDescriptor"])
			if v, ok := r.f.arv(dd["DW"]); ok {
				ft.vaikeLaius = v / 1000
			}
			ft.loeW(r.f, r.f.massiiv(dd["W"]))
		}
		if enc, ok := r.f.lahenda(d["Encoding"]).(*Stream); ok {
			if data, err := r.f.dekodeeri(enc); err == nil {
				// manustatud CMap: kasuta ainult koodiruumi
				tmp := &font{}
				ft.koodiruum = tmp.loeCMap(data)
			}
		}
	} else {
		kirjeldus = r.f.sonastik(d["FontDescriptor"])
		if alam == "Type3" {
			if m := r.f.massiiv(d["FontMatrix"]); len(m) >= 1 {
				if v, ok := r.f.arv(m[0]); ok && v != 0 {
					ft.skaala = v
				}
			}
		}
		first := 0
		if v, ok := r.f.arv(d["FirstChar"]); ok {
			first = int(v)
		}
		for i, w := range r.f.massiiv(d["Widths"]) {
			if v, ok := r.f.arv(w); ok {
				ft.laiused[uint32(first+i)] = v * ft.skaala
			}
		}
		ft.loeKodeering(r.f, d)
	}
	if kirjeldus != nil {
		if fl, ok := r.f.arv(kirjeldus["Flags"]); ok {
			if int(fl)&(1<<18) != 0 {
				ft.paks = true
			}
		}
		if v, ok := r.f.arv(kirjeldus["MissingWidth"]); ok && v > 0 && ft.vaikeLaius < 0 {
			ft.vaikeLaius = v / 1000
		}
		if w, ok := r.f.arv(kirjeldus["FontWeight"]); ok && w >= 600 {
			ft.paks = true
		}
	}
	if tu, ok := r.f.lahenda(d["ToUnicode"]).(*Stream); ok {
		if data, err := r.f.dekodeeri(tu); err == nil {
			// ToUnicode koodiruumi ei kasutata: lihtfondi koodid on alati
			// ühebaidised ja Type0 koodid määrab /Encoding
			ft.loeCMap(data)
		}
	}
	if alam != "Type0" && alam != "Type3" && len(ft.laiused) < 2 {
		ft.standard = standardMoodik(ft.nimi)
	}
	if isRef {
		r.fondid[ref] = ft
	}
	return ft
}

func (ft *font) loeW(f *fail, w Array) {
	for i := 0; i < len(w); {
		c1, ok := taisarv(f.lahenda(w[i]))
		if !ok || i+1 >= len(w) {
			return
		}
		switch v := f.lahenda(w[i+1]).(type) {
		case Array:
			for j, x := range v {
				if n, ok := f.arv(x); ok {
					ft.laiused[uint32(c1+j)] = n / 1000
				}
			}
			i += 2
		default:
			if i+2 >= len(w) {
				return
			}
			c2, _ := taisarv(v)
			n, _ := f.arv(w[i+2])
			if c2-c1 > 65536 {
				c2 = c1 + 65536
			}
			for c := c1; c <= c2; c++ {
				ft.laiused[uint32(c)] = n / 1000
			}
			i += 3
		}
	}
}

func (ft *font) loeKodeering(f *fail, d Dict) {
	ft.kodeering = &winAnsi
	enc := f.lahenda(d["Encoding"])
	var alus Name
	var dif Array
	switch v := enc.(type) {
	case Name:
		alus = v
	case Dict:
		alus = f.nimi(v["BaseEncoding"])
		dif = f.massiiv(v["Differences"])
	}
	switch alus {
	case "MacRomanEncoding":
		ft.kodeering = &macRoman
	case "StandardEncoding":
		ft.kodeering = &standard
	}
	if len(dif) > 0 {
		ft.erinevus = map[int]string{}
		kood := 0
		for _, x := range dif {
			switch v := f.lahenda(x).(type) {
			case int64:
				kood = int(v)
			case float64:
				kood = int(v)
			case Name:
				ft.erinevus[kood] = glyfistTeksti(string(v))
				kood++
			}
		}
	}
}

// loeCMap loeb ToUnicode CMapi (bfchar, bfrange) ja tagastab koodiruumi.
func (ft *font) loeCMap(data []byte) (koodiruum []koodivahemik) {
	if ft.toUni == nil {
		ft.toUni = map[uint32]string{}
	}
	l := &lekser{b: data}
	var eel []any
	for {
		o, ok := l.objekt(0)
		if !ok {
			return koodiruum
		}
		kw, isKw := o.(Keyword)
		if !isKw {
			eel = append(eel, o)
			if len(eel) > 3 {
				eel = eel[1:]
			}
			continue
		}
		switch kw {
		case "begincodespacerange":
			for {
				a, ok1 := l.objekt(0)
				if k, ok := a.(Keyword); ok && k == "endcodespacerange" || !ok1 {
					break
				}
				b, _ := l.objekt(0)
				as, _ := a.(String)
				bs, _ := b.(String)
				if len(as) > 0 && len(as) == len(bs) {
					koodiruum = append(koodiruum, koodivahemik{len(as), koodiks(string(as)), koodiks(string(bs))})
				}
			}
		case "beginbfchar":
			for {
				a, ok1 := l.objekt(0)
				if k, ok := a.(Keyword); ok && k == "endbfchar" || !ok1 {
					break
				}
				b, _ := l.objekt(0)
				as, _ := a.(String)
				switch bv := b.(type) {
				case String:
					ft.toUni[koodiks(string(as))] = utf16be([]byte(bv))
				case Name:
					ft.toUni[koodiks(string(as))] = glyfistTeksti(string(bv))
				}
			}
		case "beginbfrange":
			for {
				a, ok1 := l.objekt(0)
				if k, ok := a.(Keyword); ok && k == "endbfrange" || !ok1 {
					break
				}
				b, _ := l.objekt(0)
				c, _ := l.objekt(0)
				as, _ := a.(String)
				bs, _ := b.(String)
				lo, hi := koodiks(string(as)), koodiks(string(bs))
				if hi < lo || hi-lo > 65535 {
					continue
				}
				switch cv := c.(type) {
				case String:
					sihtB := []byte(cv)
					for k := uint32(0); k <= hi-lo; k++ {
						t := append([]byte{}, sihtB...)
						// suurenda viimast kahte baiti
						if len(t) >= 2 {
							v := uint32(t[len(t)-2])<<8 | uint32(t[len(t)-1])
							v += k
							t[len(t)-2], t[len(t)-1] = byte(v>>8), byte(v)
						} else if len(t) == 1 {
							t[0] += byte(k)
						}
						ft.toUni[lo+k] = utf16be(t)
					}
				case Array:
					for k, x := range cv {
						if s, ok := x.(String); ok {
							ft.toUni[lo+uint32(k)] = utf16be([]byte(s))
						}
					}
				}
			}
		}
	}
}

func koodiks(s string) uint32 {
	var v uint32
	for i := 0; i < len(s) && i < 4; i++ {
		v = v<<8 | uint32(s[i])
	}
	return v
}

// koodid jagab stringi märgikoodideks.
func (ft *font) koodid(s string) [][2]uint32 { // {kood, pikkus}
	var out [][2]uint32
	for i := 0; i < len(s); {
		n := 1
		if ft.kaheBait {
			n = 2
		}
		if ft.kaheBait && len(ft.koodiruum) > 0 {
			leitud := false
			for pikk := 1; pikk <= 4 && i+pikk <= len(s) && !leitud; pikk++ {
				c := koodiks(s[i : i+pikk])
				for _, v := range ft.koodiruum {
					if v.pikkus == pikk && c >= v.lo && c <= v.hi {
						n, leitud = pikk, true
						break
					}
				}
			}
		}
		if i+n > len(s) {
			n = len(s) - i
		}
		out = append(out, [2]uint32{koodiks(s[i : i+n]), uint32(n)})
		i += n
	}
	return out
}

func (ft *font) dekodeeri(s string) []glyf {
	var out []glyf
	for _, kp := range ft.koodid(s) {
		k, n := kp[0], kp[1]
		g := glyf{tyhik: n == 1 && k == 32}
		t, ok := ft.toUni[k]
		if ok && !kasutuKaart(t) {
			g.tekst = t
		} else if !ft.kaheBait {
			if e, ok := ft.erinevus[int(k)]; ok && e != "" {
				g.tekst = e
			} else if k < 256 && !ft.sumbol {
				if r := ft.kodeering[k]; r != 0 {
					g.tekst = string(r)
				}
			}
			if g.tekst == "" && ok {
				g.tekst = t
			}
		} else if ok {
			g.tekst = t
		}
		if g.tekst == "" && !(n == 1 && k < 32) {
			ft.tundmatu++
		}
		if w, ok := ft.laiused[k]; ok {
			g.laius = w
		} else if ft.vaikeLaius >= 0 {
			g.laius = ft.vaikeLaius
		} else if ft.standard != nil {
			r, _ := utf8.DecodeRuneInString(g.tekst)
			g.laius = ft.standard(r)
		} else {
			g.laius = hinnangulineLaius(g.tekst)
		}
		out = append(out, g)
	}
	return out
}

// kasutuKaart: ToUnicode kirje, mis ei anna loetavat teksti (tühi,
// asendusmärk või erakasutuse ala).
func kasutuKaart(t string) bool {
	if t == "" {
		return true
	}
	r, _ := utf8.DecodeRuneInString(t)
	return r == utf8.RuneError || r == 0 || (r >= 0xE000 && r <= 0xF8FF) || (r < 32 && r != '\t')
}

// hinnangulineLaius on laius standardfontide jaoks, millel /Widths puudub.
func hinnangulineLaius(t string) float64 {
	r, _ := utf8.DecodeRuneInString(t)
	switch {
	case r == ' ':
		return 0.25
	case r == 'i' || r == 'l' || r == 'j' || r == '.' || r == ',' || r == '\'' || r == ':' || r == ';' || r == '|':
		return 0.25
	case r == 'm' || r == 'w' || r == 'M' || r == 'W':
		return 0.8
	case unicode.IsUpper(r):
		return 0.65
	}
	return 0.5
}
