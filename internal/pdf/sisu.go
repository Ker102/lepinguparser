package pdf

import (
	"math"
)

// lugeja käib läbi lehekülgede sisuvood ja kogub märgid koos asukohaga.
type lugeja struct {
	f      *fail
	fondid map[Ref]*font
	vormid int // Form XObjecti rekursiooni sügavus
}

// mark on üks lehel näidatud märk (või ligatuur) lehe koordinaatides.
type mark struct {
	tekst    string
	x, y     float64 // algjoone alguspunkt
	laius    float64
	suurus   float64
	paks     bool
	artefakt bool // /Artifact märgistatud sisu (päis, jalus, leheküljenumber)
	// tykk on stringi järjekorranumber sisuvoos: ühe stringi märgid
	// hoitakse rea koostamisel joonistamise järjekorras
	tykk int
}

type maatriks [6]float64

var yhik = maatriks{1, 0, 0, 1, 0, 0}

func (m maatriks) korda(n maatriks) maatriks {
	return maatriks{
		m[0]*n[0] + m[1]*n[2], m[0]*n[1] + m[1]*n[3],
		m[2]*n[0] + m[3]*n[2], m[2]*n[1] + m[3]*n[3],
		m[4]*n[0] + m[5]*n[2] + n[4], m[4]*n[1] + m[5]*n[3] + n[5],
	}
}

func (m maatriks) punkt(x, y float64) (float64, float64) {
	return x*m[0] + y*m[2] + m[4], x*m[1] + y*m[3] + m[5]
}

type gOlek struct {
	ctm maatriks
	// teksti olek kuulub graafikaolekusse
	font     *font
	suurus   float64
	tc, tw   float64
	th       float64
	tl, rise float64
	rezhiim  int
}

type leheTulemus struct {
	margid   []mark
	pilte    int
	poorata  int // pööratud märke (jäetakse välja)
	vigased  bool
	tundmatu int
	tykke    int
}

type sisuKontekst struct {
	res  Dict
	olek gOlek
	pinu []gOlek
	tm   maatriks
	tlm  maatriks
	mc   []mcKirje
	tul  *leheTulemus
	leht maatriks // lehe pööramine
}

type mcKirje struct {
	artefakt   bool
	actualText *string
	kasutatud  bool
}

func (r *lugeja) leheMargid(leht Dict, pooramine maatriks) *leheTulemus {
	tul := &leheTulemus{}
	var sisu []byte
	switch v := r.f.lahenda(leht["Contents"]).(type) {
	case *Stream:
		b, err := r.f.dekodeeri(v)
		if err != nil {
			tul.vigased = true
		}
		sisu = b
	case Array:
		for _, x := range v {
			if s, ok := r.f.lahenda(x).(*Stream); ok {
				b, err := r.f.dekodeeri(s)
				if err != nil {
					tul.vigased = true
					continue
				}
				sisu = append(sisu, b...)
				sisu = append(sisu, '\n')
			}
		}
	}
	k := &sisuKontekst{res: r.ressursid(leht), tul: tul, leht: pooramine}
	k.olek = gOlek{ctm: yhik, th: 1}
	r.taida(k, sisu)
	return tul
}

func (r *lugeja) ressursid(leht Dict) Dict {
	return r.f.sonastik(leht["Resources"])
}

// taida tõlgendab sisuvoo operaatoreid.
func (r *lugeja) taida(k *sisuKontekst, sisu []byte) {
	l := &lekser{b: sisu}
	var op []any
	for {
		o, ok := l.objekt(0)
		if !ok {
			return
		}
		kw, isKw := o.(Keyword)
		if !isKw {
			if _, ok := o.(lopp); !ok {
				op = append(op, o)
				if len(op) > 64 {
					op = op[len(op)-64:]
				}
			}
			continue
		}
		r.operaator(k, string(kw), op, l)
		op = op[:0]
	}
}

func nr(op []any, i int) float64 {
	if i < 0 || i >= len(op) {
		return 0
	}
	v, _ := arvuna(op[i])
	return v
}

func (r *lugeja) operaator(k *sisuKontekst, kw string, op []any, l *lekser) {
	o := &k.olek
	switch kw {
	case "q":
		if len(k.pinu) < 256 {
			k.pinu = append(k.pinu, *o)
		}
	case "Q":
		if n := len(k.pinu); n > 0 {
			*o = k.pinu[n-1]
			k.pinu = k.pinu[:n-1]
		}
	case "cm":
		if len(op) >= 6 {
			m := maatriks{nr(op, 0), nr(op, 1), nr(op, 2), nr(op, 3), nr(op, 4), nr(op, 5)}
			o.ctm = m.korda(o.ctm)
		}
	case "BT":
		k.tm, k.tlm = yhik, yhik
	case "ET":
	case "Tf":
		if len(op) >= 2 {
			if n, ok := op[0].(Name); ok {
				fd := r.f.sonastik(k.res["Font"])
				o.font = r.laeFont(fd[n])
			}
			o.suurus = nr(op, 1)
		}
	case "Tc":
		o.tc = nr(op, 0)
	case "Tw":
		o.tw = nr(op, 0)
	case "Tz":
		o.th = nr(op, 0) / 100
	case "TL":
		o.tl = nr(op, 0)
	case "Ts":
		o.rise = nr(op, 0)
	case "Tr":
		o.rezhiim = int(nr(op, 0))
	case "Td":
		k.tlm = maatriks{1, 0, 0, 1, nr(op, 0), nr(op, 1)}.korda(k.tlm)
		k.tm = k.tlm
	case "TD":
		o.tl = -nr(op, 1)
		k.tlm = maatriks{1, 0, 0, 1, nr(op, 0), nr(op, 1)}.korda(k.tlm)
		k.tm = k.tlm
	case "Tm":
		if len(op) >= 6 {
			k.tlm = maatriks{nr(op, 0), nr(op, 1), nr(op, 2), nr(op, 3), nr(op, 4), nr(op, 5)}
			k.tm = k.tlm
		}
	case "T*":
		k.reaVahetus()
	case "Tj":
		if len(op) >= 1 {
			if s, ok := op[len(op)-1].(String); ok {
				r.naita(k, string(s))
			}
		}
	case "'":
		k.reaVahetus()
		if len(op) >= 1 {
			if s, ok := op[len(op)-1].(String); ok {
				r.naita(k, string(s))
			}
		}
	case "\"":
		if len(op) >= 3 {
			o.tw, o.tc = nr(op, 0), nr(op, 1)
			k.reaVahetus()
			if s, ok := op[2].(String); ok {
				r.naita(k, string(s))
			}
		}
	case "TJ":
		if len(op) >= 1 {
			if a, ok := op[len(op)-1].(Array); ok {
				for _, x := range a {
					switch v := x.(type) {
					case String:
						r.naita(k, string(v))
					case int64, float64:
						n, _ := arvuna(v)
						tx := -n / 1000 * o.suurus * o.th
						k.tm = maatriks{1, 0, 0, 1, tx, 0}.korda(k.tm)
					}
				}
			}
		}
	case "Do":
		if len(op) >= 1 {
			if n, ok := op[0].(Name); ok {
				r.xobjekt(k, n)
			}
		}
	case "BI":
		// inline-pilt: jäta andmed vahele kuni "EI"
		k.tul.pilte++
		for {
			x, ok := l.objekt(0)
			if !ok {
				return
			}
			if kw, ok := x.(Keyword); ok && kw == "ID" {
				break
			}
		}
		for l.pos+2 < len(l.b) {
			if l.b[l.pos] == 'E' && l.b[l.pos+1] == 'I' && onTyhik(l.b[l.pos-1]) && (l.pos+2 == len(l.b) || onTyhik(l.b[l.pos+2])) {
				l.pos += 2
				return
			}
			l.pos++
		}
		l.pos = len(l.b)
	case "BMC":
		k.mc = append(k.mc, mcKirje{artefakt: len(op) > 0 && op[0] == Name("Artifact")})
	case "BDC":
		e := mcKirje{}
		if len(op) >= 1 && op[0] == Name("Artifact") {
			e.artefakt = true
		}
		if len(op) >= 2 {
			var p Dict
			switch v := op[1].(type) {
			case Dict:
				p = v
			case Name:
				p = r.f.sonastik(r.f.sonastik(k.res["Properties"])[v])
			}
			if s, ok := r.f.lahenda(p["ActualText"]).(String); ok {
				t := pdfDoc(string(s))
				e.actualText = &t
			}
		}
		k.mc = append(k.mc, e)
	case "EMC":
		if n := len(k.mc); n > 0 {
			k.mc = k.mc[:n-1]
		}
	}
}

func (k *sisuKontekst) reaVahetus() {
	k.tlm = maatriks{1, 0, 0, 1, 0, -k.olek.tl}.korda(k.tlm)
	k.tm = k.tlm
}

func (r *lugeja) xobjekt(k *sisuKontekst, n Name) {
	xo := r.f.sonastik(k.res["XObject"])
	s, ok := r.f.lahenda(xo[n]).(*Stream)
	if !ok {
		return
	}
	switch r.f.nimi(s.Dict["Subtype"]) {
	case "Image":
		k.tul.pilte++
	case "Form":
		if r.vormid >= 12 {
			return
		}
		data, err := r.f.dekodeeri(s)
		if err != nil {
			k.tul.vigased = true
			return
		}
		r.vormid++
		defer func() { r.vormid-- }()
		uus := &sisuKontekst{res: k.res, tul: k.tul, olek: k.olek, mc: append([]mcKirje{}, k.mc...), leht: k.leht}
		if res := r.f.sonastik(s.Dict["Resources"]); res != nil {
			uus.res = res
		}
		if m := r.f.massiiv(s.Dict["Matrix"]); len(m) == 6 {
			var mm maatriks
			for i := range mm {
				mm[i], _ = r.f.arv(m[i])
			}
			uus.olek.ctm = mm.korda(k.olek.ctm)
		}
		r.taida(uus, data)
	}
}

// naita lisab stringi märgid ja liigutab tekstimaatriksit.
func (r *lugeja) naita(k *sisuKontekst, s string) {
	o := &k.olek
	ft := o.font
	if ft == nil {
		ft = &font{kodeering: &winAnsi, laiused: map[uint32]float64{}, skaala: 0.001, vaikeLaius: -1}
		o.font = ft
	}
	enne := ft.tundmatu
	glyfid := ft.dekodeeri(s)
	k.tul.tykke++
	k.tul.tundmatu += ft.tundmatu - enne
	artefakt := false
	var akt *mcKirje
	for i := range k.mc {
		if k.mc[i].artefakt {
			artefakt = true
		}
		if k.mc[i].actualText != nil {
			akt = &k.mc[i]
		}
	}
	for _, g := range glyfid {
		trm := maatriks{o.suurus * o.th, 0, 0, o.suurus, 0, o.rise}.korda(k.tm).korda(o.ctm).korda(k.leht)
		x, y := trm.punkt(0, 0)
		tx := (g.laius*o.suurus + o.tc) * o.th
		if g.tyhik {
			tx += o.tw * o.th
		}
		// märgi laius lehe ruumis
		x2, y2 := maatriks{1, 0, 0, 1, tx, 0}.korda(k.tm).korda(o.ctm).korda(k.leht).punkt(0, 0)
		k.tm = maatriks{1, 0, 0, 1, tx, 0}.korda(k.tm)
		suurus := math.Hypot(trm[2], trm[3])
		if suurus <= 0.01 {
			continue
		}
		tekst := g.tekst
		if akt != nil {
			if akt.kasutatud {
				continue
			}
			akt.kasutatud = true
			tekst = *akt.actualText
		}
		if tekst == "" {
			continue
		}
		// horisontaalne tekst: x-telg paremale, y üles
		if math.Abs(trm[1]) > 0.1*math.Abs(trm[0]) || trm[0] <= 0 || trm[3] <= 0 {
			k.tul.poorata++
			continue
		}
		k.tul.margid = append(k.tul.margid, mark{
			tekst: tekst, x: x, y: y, laius: math.Hypot(x2-x, y2-y), suurus: suurus,
			paks: ft.paks || o.rezhiim == 2, artefakt: artefakt, tykk: k.tul.tykke,
		})
	}
}
