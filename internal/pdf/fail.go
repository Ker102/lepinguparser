package pdf

import (
	"bytes"
	"errors"
	"fmt"
	"regexp"
	"strconv"
)

// ErrNotPDF tagastatakse, kui fail ei ole loetav PDF.
var ErrNotPDF = errors.New("fail ei ole korrektne PDF dokument")

// ErrParool tagastatakse, kui PDF-i avamiseks on vaja parooli.
var ErrParool = errors.New("PDF on parooliga kaitstud: ava see parooliga ja salvesta kaitseta koopia")

type xrefKirje struct {
	tyyp   int // 1 = failis nihkega, 2 = objektivoos
	nihe   int // tüüp 1: nihe; tüüp 2: objektivoo number
	indeks int // tüüp 2: indeks voos
}

// fail on avatud PDF.
type fail struct {
	b        []byte
	xref     map[int]xrefKirje
	trailer  Dict
	vahemalu map[int]any
	objVood  map[int]*objVoog
	laeb     map[int]bool
	skanni   map[int]int // varutabel: objekti number -> nihe
	kruptija *kruptija
	paise    int // %PDF- päise nihe
}

type objVoog struct {
	andmed []byte
	nihked map[int]int // indeks -> nihe andmetes
	nrid   []int
}

func ava(b []byte) (*fail, error) {
	p := bytes.Index(b, []byte("%PDF-"))
	if p < 0 || p > 1024 {
		return nil, fmt.Errorf("%w: päis %%PDF- puudub", ErrNotPDF)
	}
	f := &fail{b: b, xref: map[int]xrefKirje{}, vahemalu: map[int]any{}, objVood: map[int]*objVoog{}, laeb: map[int]bool{}, paise: p}
	if err := f.loeXref(); err != nil || f.trailer == nil || f.trailer["Root"] == nil {
		// vigane või puuduv xref: taasta objektid failist otsides
		f.xref = map[int]xrefKirje{}
		f.trailer = nil
		f.skanniObjektid()
		if f.trailer == nil || f.trailer["Root"] == nil {
			return nil, fmt.Errorf("%w: dokumendi juurobjekti ei leitud", ErrNotPDF)
		}
	}
	if enc := f.trailer["Encrypt"]; enc != nil {
		k, err := uusKruptija(f, enc)
		if err != nil {
			return nil, err
		}
		f.kruptija = k
	}
	return f, nil
}

var startxrefRe = regexp.MustCompile(`startxref\s+(\d+)`)

func (f *fail) loeXref() error {
	saba := f.b
	if len(saba) > 4096 {
		saba = saba[len(saba)-4096:]
	}
	m := startxrefRe.FindAllSubmatch(saba, -1)
	if len(m) == 0 {
		return errors.New("startxref puudub")
	}
	nihe, _ := strconv.Atoi(string(m[len(m)-1][1]))
	nahtud := map[int]bool{}
	for k := 0; k < 100; k++ {
		if nahtud[nihe] {
			break
		}
		nahtud[nihe] = true
		tr, err := f.loeXrefJaotis(nihe)
		if err != nil && f.paise > 0 {
			tr, err = f.loeXrefJaotis(nihe + f.paise)
		}
		if err != nil {
			if f.trailer != nil {
				break // vanem jaotis on vigane, uuemad on loetud
			}
			return err
		}
		if f.trailer == nil {
			f.trailer = tr
		}
		// hübriidfail: tabeli kõrval xref-voog
		if xs, ok := taisarv(tr["XRefStm"]); ok {
			_, _ = f.loeXrefJaotis(xs)
		}
		prev, ok := taisarv(tr["Prev"])
		if !ok {
			break
		}
		nihe = prev
	}
	return nil
}

func (f *fail) loeXrefJaotis(nihe int) (Dict, error) {
	if nihe < 0 || nihe >= len(f.b) {
		return nil, errors.New("xref nihe on väljaspool faili")
	}
	l := &lekser{b: f.b, pos: nihe}
	l.jataVahele()
	if bytes.HasPrefix(f.b[l.pos:], []byte("xref")) {
		l.pos += 4
		return f.loeXrefTabel(l)
	}
	// xref-voog: "N G obj << ... >> stream"
	o, err := f.loeObjektNihkelt(l.pos, -1)
	if err != nil {
		return nil, err
	}
	s, ok := o.(*Stream)
	if !ok || s.Dict["Type"] != Name("XRef") {
		return nil, errors.New("xref ei ole tabel ega voog")
	}
	if err := f.loeXrefVoog(s); err != nil {
		return nil, err
	}
	return s.Dict, nil
}

func (f *fail) loeXrefTabel(l *lekser) (Dict, error) {
	for {
		l.jataVahele()
		if bytes.HasPrefix(f.b[l.pos:], []byte("trailer")) {
			l.pos += 7
			o, _ := l.objekt(0)
			d, ok := o.(Dict)
			if !ok {
				return nil, errors.New("trailer ei ole sõnastik")
			}
			return d, nil
		}
		a, err1 := strconv.Atoi(string(l.sona()))
		l.jataVahele()
		n, err2 := strconv.Atoi(string(l.sona()))
		if err1 != nil || err2 != nil || n < 0 || n > 10_000_000 {
			return nil, errors.New("vigane xref tabel")
		}
		for i := 0; i < n; i++ {
			l.jataVahele()
			off, e1 := strconv.Atoi(string(l.sona()))
			l.jataVahele()
			l.sona()
			l.jataVahele()
			t := l.sona()
			if e1 != nil || len(t) != 1 {
				return nil, errors.New("vigane xref kirje")
			}
			nr := a + i
			if _, olemas := f.xref[nr]; olemas {
				continue
			}
			if t[0] == 'n' {
				f.xref[nr] = xrefKirje{tyyp: 1, nihe: off}
			} else {
				f.xref[nr] = xrefKirje{tyyp: 0}
			}
		}
	}
}

func (f *fail) loeXrefVoog(s *Stream) error {
	data, err := f.dekodeeri(s)
	if err != nil {
		return err
	}
	wa, _ := s.Dict["W"].(Array)
	if len(wa) != 3 {
		return errors.New("xref-voo /W on vigane")
	}
	var w [3]int
	rida := 0
	for i := range w {
		w[i], _ = taisarv(wa[i])
		if w[i] < 0 || w[i] > 8 {
			return errors.New("xref-voo /W on vigane")
		}
		rida += w[i]
	}
	if rida == 0 {
		return errors.New("xref-voo /W on vigane")
	}
	size, _ := taisarv(s.Dict["Size"])
	idx := []int{0, size}
	if ia, ok := s.Dict["Index"].(Array); ok && len(ia)%2 == 0 {
		idx = idx[:0]
		for _, v := range ia {
			n, _ := taisarv(v)
			idx = append(idx, n)
		}
	}
	valja := func(b []byte) int {
		v := 0
		for _, c := range b {
			v = v<<8 | int(c)
		}
		return v
	}
	pos := 0
	for k := 0; k+1 < len(idx); k += 2 {
		for i := 0; i < idx[k+1]; i++ {
			if pos+rida > len(data) {
				return nil
			}
			r := data[pos : pos+rida]
			pos += rida
			t := 1
			if w[0] > 0 {
				t = valja(r[:w[0]])
			}
			a := valja(r[w[0] : w[0]+w[1]])
			c := valja(r[w[0]+w[1]:])
			nr := idx[k] + i
			if _, olemas := f.xref[nr]; olemas {
				continue
			}
			switch t {
			case 1:
				f.xref[nr] = xrefKirje{tyyp: 1, nihe: a}
			case 2:
				f.xref[nr] = xrefKirje{tyyp: 2, nihe: a, indeks: c}
			default:
				f.xref[nr] = xrefKirje{tyyp: 0}
			}
		}
	}
	return nil
}

var objRe = regexp.MustCompile(`(?m)(\d+)[ \t\r\n\f]+(\d+)[ \t\r\n\f]+obj\b`)

// skanniObjektid koostab objektide tabeli kogu faili läbi otsides.
func (f *fail) skanniObjektid() {
	if f.skanni != nil {
		return
	}
	f.skanni = map[int]int{}
	for _, m := range objRe.FindAllSubmatchIndex(f.b, -1) {
		if m[0] > 0 && !onTyhik(f.b[m[0]-1]) && !onEraldaja(f.b[m[0]-1]) {
			continue
		}
		nr, _ := strconv.Atoi(string(f.b[m[2]:m[3]]))
		f.skanni[nr] = m[0] // hilisem esinemine võidab
	}
	if f.trailer != nil {
		return
	}
	// trailer: viimane "trailer" sõnastik või xref-voog, millel on /Root
	for p := len(f.b); ; {
		i := bytes.LastIndex(f.b[:p], []byte("trailer"))
		if i < 0 {
			break
		}
		l := &lekser{b: f.b, pos: i + 7}
		if o, _ := l.objekt(0); o != nil {
			if d, ok := o.(Dict); ok && d["Root"] != nil {
				f.trailer = d
				return
			}
		}
		p = i
	}
	for nr, off := range f.skanni {
		o, err := f.loeObjektNihkelt(off, nr)
		if err != nil {
			continue
		}
		var d Dict
		switch v := o.(type) {
		case *Stream:
			d = v.Dict
		case Dict:
			d = v
		}
		if d == nil {
			continue
		}
		if d["Type"] == Name("XRef") && d["Root"] != nil {
			f.trailer = d
			return
		}
		if d["Type"] == Name("Catalog") {
			f.trailer = Dict{"Root": Ref{nr, 0}}
		}
	}
}

// loeObjektNihkelt loeb kaudse objekti "N G obj ... endobj" antud nihkelt.
// Kui oodatud >= 0, kontrollitakse objekti numbrit.
func (f *fail) loeObjektNihkelt(nihe, oodatud int) (any, error) {
	if nihe < 0 || nihe >= len(f.b) {
		return nil, errors.New("nihe on väljaspool faili")
	}
	l := &lekser{b: f.b, pos: nihe}
	l.jataVahele()
	nr, err := strconv.Atoi(string(l.sona()))
	if err != nil {
		return nil, errors.New("objekti number puudub")
	}
	l.jataVahele()
	gen, _ := strconv.Atoi(string(l.sona()))
	l.jataVahele()
	if string(l.sona()) != "obj" {
		return nil, errors.New("'obj' puudub")
	}
	if oodatud >= 0 && nr != oodatud {
		return nil, fmt.Errorf("ootasin objekti %d, leidsin %d", oodatud, nr)
	}
	o, _ := l.objekt(0)
	if d, ok := o.(Dict); ok {
		l.jataVahele()
		if bytes.HasPrefix(f.b[l.pos:], []byte("stream")) {
			l.pos += 6
			if l.pos < len(f.b) && f.b[l.pos] == '\r' {
				l.pos++
			}
			if l.pos < len(f.b) && f.b[l.pos] == '\n' {
				l.pos++
			}
			return &Stream{Dict: d, Raw: f.vooAndmed(d, l.pos, nr), ref: Ref{nr, gen}}, nil
		}
	}
	return o, nil
}

func (f *fail) vooAndmed(d Dict, algus, nr int) []byte {
	pikkus := -1
	switch v := d["Length"].(type) {
	case int64:
		pikkus = int(v)
	case Ref:
		if v.Num != nr && !f.laeb[v.Num] {
			if n, ok := taisarv(f.lahenda(v)); ok {
				pikkus = n
			}
		}
	}
	if pikkus >= 0 && algus+pikkus <= len(f.b) {
		saba := f.b[algus+pikkus:]
		if len(saba) > 40 {
			saba = saba[:40]
		}
		if bytes.Contains(saba, []byte("endstream")) {
			return f.b[algus : algus+pikkus]
		}
	}
	// /Length puudub või on vale: otsi "endstream"
	i := bytes.Index(f.b[algus:], []byte("endstream"))
	if i < 0 {
		return f.b[algus:]
	}
	lopp := algus + i
	if lopp > algus && f.b[lopp-1] == '\n' {
		lopp--
	}
	if lopp > algus && f.b[lopp-1] == '\r' {
		lopp--
	}
	return f.b[algus:lopp]
}

// lahenda tagastab viidatud objekti (või objekti enda, kui see pole viide).
func (f *fail) lahenda(o any) any {
	for k := 0; k < 32; k++ {
		r, ok := o.(Ref)
		if !ok {
			return o
		}
		o = f.objekt(r.Num)
	}
	return nil
}

func (f *fail) objekt(nr int) any {
	if o, ok := f.vahemalu[nr]; ok {
		return o
	}
	if f.laeb[nr] {
		return nil // tsükkel
	}
	f.laeb[nr] = true
	defer delete(f.laeb, nr)
	o, err := f.laeObjekt(nr)
	if err != nil {
		f.skanniObjektid()
		if off, ok := f.skanni[nr]; ok {
			o, err = f.loeObjektNihkelt(off, nr)
			if err == nil {
				o = f.dekryptiObjekt(o, nr)
			}
		}
	}
	if err != nil {
		o = nil
	}
	f.vahemalu[nr] = o
	return o
}

func (f *fail) laeObjekt(nr int) (any, error) {
	k, ok := f.xref[nr]
	if !ok {
		return nil, errors.New("objekt puudub tabelist")
	}
	switch k.tyyp {
	case 1:
		o, err := f.loeObjektNihkelt(k.nihe, nr)
		if err != nil && f.paise > 0 {
			o, err = f.loeObjektNihkelt(k.nihe+f.paise, nr)
		}
		if err != nil {
			return nil, err
		}
		return f.dekryptiObjekt(o, nr), nil
	case 2:
		v, err := f.objektivoog(k.nihe)
		if err != nil {
			return nil, err
		}
		off, ok := v.nihked[k.indeks]
		if !ok || (k.indeks < len(v.nrid) && v.nrid[k.indeks] != nr) {
			// indeks ei klapi: otsi numbri järgi
			ok = false
			for i, n := range v.nrid {
				if n == nr {
					off, ok = v.nihked[i], true
				}
			}
			if !ok {
				return nil, errors.New("objekt puudub objektivoost")
			}
		}
		l := &lekser{b: v.andmed, pos: off}
		o, _ := l.objekt(0)
		return o, nil
	}
	return nil, nil
}

func (f *fail) objektivoog(nr int) (*objVoog, error) {
	if v, ok := f.objVood[nr]; ok {
		return v, nil
	}
	s, ok := f.lahenda(Ref{nr, 0}).(*Stream)
	if !ok {
		return nil, errors.New("objektivoogu ei leitud")
	}
	data, err := f.dekodeeri(s)
	if err != nil {
		return nil, err
	}
	n, _ := taisarv(s.Dict["N"])
	first, _ := taisarv(s.Dict["First"])
	v := &objVoog{andmed: data, nihked: map[int]int{}}
	l := &lekser{b: data}
	for i := 0; i < n && i < 1_000_000; i++ {
		l.jataVahele()
		on, e1 := strconv.Atoi(string(l.sona()))
		l.jataVahele()
		off, e2 := strconv.Atoi(string(l.sona()))
		if e1 != nil || e2 != nil {
			break
		}
		v.nrid = append(v.nrid, on)
		v.nihked[i] = first + off
	}
	f.objVood[nr] = v
	return v, nil
}

// Lugemise abifunktsioonid, mis lahendavad viited.

func (f *fail) sonastik(o any) Dict {
	switch v := f.lahenda(o).(type) {
	case Dict:
		return v
	case *Stream:
		return v.Dict
	}
	return nil
}

func (f *fail) massiiv(o any) Array {
	a, _ := f.lahenda(o).(Array)
	return a
}

func (f *fail) arv(o any) (float64, bool) {
	return arvuna(f.lahenda(o))
}

func (f *fail) nimi(o any) Name {
	n, _ := f.lahenda(o).(Name)
	return n
}
