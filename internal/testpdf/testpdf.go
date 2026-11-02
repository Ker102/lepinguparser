// Package testpdf koostab testides kasutatavaid PDF-faile. Fondiprogramme
// ei manustata: tekstikihi lugemiseks piisab kodeeringust, laiustest ja
// ToUnicode tabelist.
package testpdf

import (
	"bytes"
	"compress/zlib"
	"crypto/md5"
	"crypto/rc4"
	"encoding/binary"
	"fmt"
	"sort"
	"strings"
)

// Rida on lehele paigutatud tekstirida (koordinaadid punktides, lehe
// alumisest vasakust nurgast).
type Rida struct {
	X, Y     float64
	Tekst    string
	Suurus   float64 // vaikimisi 11
	Paks     bool
	Artefakt bool // märgi /Artifact (päis, jalus)
}

// Valikud määravad faili ülesehituse.
type Valikud struct {
	// Type0 kasutab Identity-H fonti ToUnicode tabeliga (nagu Wordi
	// PDF-eksport); muidu WinAnsiEncoding lihtfont.
	Type0 bool
	// Pakitud: vood FlateDecode'iga, objektid objektivoos ja xref-voog.
	Pakitud bool
	// RC4 krüpteerib faili omaniku parooliga (tühi kasutajaparool, R3).
	RC4 bool
	// Allkiri lisab allkirjavälja, mille /Contents on see PKCS#7 struktuur.
	Allkiri []byte
}

// Laius on testfondi märgi laius (tuhandikes em-ist).
func Laius(r rune) int {
	if r == ' ' {
		return 250
	}
	return 500
}

// Koosta loob PDF-i. Iga lehe suurus on A4 (595 × 842).
func Koosta(lehed [][]Rida, v Valikud) []byte {
	b := &ehitaja{v: v}
	if v.RC4 {
		b.id = []byte("testpdf-id-12345")
	}
	// objektid: 1 kataloog, 2 lehtede puu, 3 font, 4 fondi alam (Type0),
	// 5 ToUnicode, siis lehed ja sisuvood
	runid := map[rune]int{}
	var jarjestus []rune
	for _, leht := range lehed {
		for _, r := range leht {
			for _, c := range r.Tekst {
				if _, ok := runid[c]; !ok {
					runid[c] = len(jarjestus) + 1
					jarjestus = append(jarjestus, c)
				}
			}
		}
	}
	kodeeri := func(s string) string {
		if v.Type0 {
			var sb strings.Builder
			sb.WriteByte('<')
			for _, c := range s {
				fmt.Fprintf(&sb, "%04X", runid[c])
			}
			sb.WriteByte('>')
			return sb.String()
		}
		var sb strings.Builder
		sb.WriteByte('<')
		for _, c := range s {
			fmt.Fprintf(&sb, "%02X", cp1252(c))
		}
		sb.WriteByte('>')
		return sb.String()
	}

	var lapsed []string
	nr := 6
	type lehtObj struct{ leht, sisu int }
	var lo []lehtObj
	for range lehed {
		lo = append(lo, lehtObj{nr, nr + 1})
		lapsed = append(lapsed, fmt.Sprintf("%d 0 R", nr))
		nr += 2
	}
	allkiriNr := nr
	b.lisa(1, func() string {
		s := "<< /Type /Catalog /Pages 2 0 R /MarkInfo << /Marked true >>"
		if v.Allkiri != nil {
			s += fmt.Sprintf(" /AcroForm << /Fields [%d 0 R] /SigFlags 3 >>", allkiriNr)
		}
		return s + " >>"
	}(), nil)
	b.lisa(2, fmt.Sprintf("<< /Type /Pages /Kids [%s] /Count %d >>", strings.Join(lapsed, " "), len(lehed)), nil)
	if v.Type0 {
		b.lisa(3, "<< /Type /Font /Subtype /Type0 /BaseFont /ABCDEF+TestSans /Encoding /Identity-H /DescendantFonts [4 0 R] /ToUnicode 5 0 R >>", nil)
		var w strings.Builder
		for i, c := range jarjestus {
			fmt.Fprintf(&w, "%d [%d] ", i+1, Laius(c))
		}
		b.lisa(4, "<< /Type /Font /Subtype /CIDFontType2 /BaseFont /ABCDEF+TestSans /CIDSystemInfo << /Registry (Adobe) /Ordering (Identity) /Supplement 0 >> /DW 500 /W ["+w.String()+"] >>", nil)
		var cm strings.Builder
		cm.WriteString("/CIDInit /ProcSet findresource begin 12 dict begin begincmap\n1 begincodespacerange <0000> <FFFF> endcodespacerange\n")
		fmt.Fprintf(&cm, "%d beginbfchar\n", len(jarjestus))
		for i, c := range jarjestus {
			u := utf16(c)
			fmt.Fprintf(&cm, "<%04X> <%s>\n", i+1, u)
		}
		cm.WriteString("endbfchar\nendcmap CMapName currentdict /CMap defineresource pop end end\n")
		b.lisa(5, "", []byte(cm.String()))
	} else {
		var w []string
		for c := 32; c < 256; c++ {
			w = append(w, fmt.Sprint(Laius(rune(c))))
		}
		b.lisa(3, "<< /Type /Font /Subtype /TrueType /BaseFont /TestSans /FirstChar 32 /LastChar 255 /Encoding /WinAnsiEncoding /Widths ["+strings.Join(w, " ")+"] >>", nil)
		b.lisa(4, "<< /Type /Font /Subtype /TrueType /BaseFont /TestSans,Bold /FirstChar 32 /LastChar 255 /Encoding /WinAnsiEncoding /Widths ["+strings.Join(w, " ")+"] >>", nil)
		b.lisa(5, "null", nil)
	}
	for i, leht := range lehed {
		var c strings.Builder
		for _, r := range leht {
			s := r.Suurus
			if s == 0 {
				s = 11
			}
			f := "F1"
			if r.Paks && !v.Type0 {
				f = "F2"
			}
			if r.Artefakt {
				c.WriteString("/Artifact <</Type /Pagination>> BDC\n")
			}
			fmt.Fprintf(&c, "BT /%s %g Tf %g %g Td %s Tj ET\n", f, s, r.X, r.Y, kodeeri(r.Tekst))
			if r.Artefakt {
				c.WriteString("EMC\n")
			}
		}
		fondid := "/F1 3 0 R"
		if !v.Type0 {
			fondid += " /F2 4 0 R"
		}
		b.lisa(lo[i].leht, fmt.Sprintf("<< /Type /Page /Parent 2 0 R /MediaBox [0 0 595 842] /Resources << /Font << %s >> >> /Contents %d 0 R >>", fondid, lo[i].sisu), nil)
		b.lisa(lo[i].sisu, "", []byte(c.String()))
	}
	if v.Allkiri != nil {
		b.lisa(allkiriNr, fmt.Sprintf("<< /FT /Sig /T (Allkiri1) /V %d 0 R /Type /Annot /Subtype /Widget /Rect [0 0 0 0] >>", allkiriNr+1), nil)
		b.allkiri = allkiriNr + 1
		b.lisa(allkiriNr+1, "", nil)
	}
	return b.kirjuta()
}

// cp1252 kodeerib märgi WinAnsiEncoding'u (Windows-1252) baidiks.
func cp1252(c rune) byte {
	ulemine := map[rune]byte{'€': 0x80, '‚': 0x82, '„': 0x84, '…': 0x85, 'Š': 0x8A, 'Ž': 0x8E, '‘': 0x91, '’': 0x92,
		'“': 0x93, '”': 0x94, '•': 0x95, '–': 0x96, '—': 0x97, 'š': 0x9A, 'ž': 0x9E}
	if b, ok := ulemine[c]; ok {
		return b
	}
	if c < 256 && (c < 0x80 || c >= 0xA0) {
		return byte(c)
	}
	panic(fmt.Sprintf("märk %q ei ole Windows-1252 tabelis", c))
}

func utf16(c rune) string {
	if c < 0x10000 {
		return fmt.Sprintf("%04X", c)
	}
	c -= 0x10000
	return fmt.Sprintf("%04X%04X", 0xD800+(c>>10), 0xDC00+(c&0x3FF))
}

type obj struct {
	nr   int
	dict string
	voog []byte
}

type ehitaja struct {
	v       Valikud
	objs    []obj
	id      []byte
	votme   []byte
	allkiri int
}

func (b *ehitaja) lisa(nr int, dict string, voog []byte) {
	b.objs = append(b.objs, obj{nr, dict, voog})
}

func (b *ehitaja) objektiVotme(nr int) []byte {
	k := append(append([]byte{}, b.votme...), byte(nr), byte(nr>>8), byte(nr>>16), 0, 0)
	s := md5.Sum(k)
	return s[:]
}

func (b *ehitaja) rc4(nr int, data []byte) []byte {
	c, _ := rc4.NewCipher(b.objektiVotme(nr))
	out := make([]byte, len(data))
	c.XORKeyStream(out, data)
	return out
}

var taide = []byte{0x28, 0xBF, 0x4E, 0x5E, 0x4E, 0x75, 0x8A, 0x41, 0x64, 0x00, 0x4E, 0x56, 0xFF, 0xFA, 0x01, 0x08,
	0x2E, 0x2E, 0x00, 0xB6, 0xD0, 0x68, 0x3E, 0x80, 0x2F, 0x0C, 0xA9, 0xFE, 0x64, 0x53, 0x69, 0x7A}

func (b *ehitaja) kirjuta() []byte {
	sort.Slice(b.objs, func(i, j int) bool { return b.objs[i].nr < b.objs[j].nr })
	var encDict string
	if b.v.RC4 {
		// standardne turvakäsitleja R3, 128-bitine võti, tühi kasutajaparool
		o := bytes.Repeat([]byte{0x42}, 32)
		p := int32(-3904) // kopeerimine keelatud
		h := md5.New()
		h.Write(taide)
		h.Write(o)
		var pb [4]byte
		binary.LittleEndian.PutUint32(pb[:], uint32(p))
		h.Write(pb[:])
		h.Write(b.id)
		key := h.Sum(nil)
		for i := 0; i < 50; i++ {
			s := md5.Sum(key)
			key = s[:]
		}
		b.votme = key
		s := md5.New()
		s.Write(taide)
		s.Write(b.id)
		x := s.Sum(nil)
		for i := 0; i < 20; i++ {
			kk := make([]byte, 16)
			for j := range key {
				kk[j] = key[j] ^ byte(i)
			}
			c, _ := rc4.NewCipher(kk)
			c.XORKeyStream(x, x)
		}
		u := append(x, make([]byte, 16)...)
		encDict = fmt.Sprintf("<< /Filter /Standard /V 2 /R 3 /Length 128 /O <%X> /U <%X> /P %d >>", o, u, p)
	}
	var out bytes.Buffer
	out.WriteString("%PDF-1.7\n%\xE2\xE3\xCF\xD3\n")
	nihked := map[int]int{}
	maxNr := 0
	for _, o := range b.objs {
		if o.nr > maxNr {
			maxNr = o.nr
		}
	}
	encNr := maxNr + 1
	if b.v.RC4 {
		b.objs = append(b.objs, obj{encNr, encDict, nil})
	}
	kirjutaObj := func(o obj) {
		nihked[o.nr] = out.Len()
		if o.nr == b.allkiri && b.v.Allkiri != nil {
			// /Contents on kuueteistkümnendsüsteemis; ByteRange täidetakse hiljem
			fmt.Fprintf(&out, "%d 0 obj\n<< /Type /Sig /Filter /Adobe.PPKLite /SubFilter /ETSI.CAdES.detached /M (D:20260115103000+02'00') /Reason (Lepingu s\xF5lmimine) /ByteRange [0 0000000000 0000000000 0000000000] /Contents <", o.nr)
			fmt.Fprintf(&out, "%X", b.v.Allkiri)
			out.WriteString("> >>\nendobj\n")
			return
		}
		if o.voog == nil {
			fmt.Fprintf(&out, "%d 0 obj\n%s\nendobj\n", o.nr, o.dict)
			return
		}
		data := o.voog
		filter := ""
		if b.v.Pakitud {
			var z bytes.Buffer
			w := zlib.NewWriter(&z)
			w.Write(data)
			w.Close()
			data = z.Bytes()
			filter = " /Filter /FlateDecode"
		}
		if b.v.RC4 {
			data = b.rc4(o.nr, data)
		}
		fmt.Fprintf(&out, "%d 0 obj\n<< /Length %d%s >>\nstream\n", o.nr, len(data), filter)
		out.Write(data)
		out.WriteString("\nendstream\nendobj\n")
	}
	if !b.v.Pakitud {
		for _, o := range b.objs {
			kirjutaObj(o)
		}
		xref := out.Len()
		n := 0
		for _, o := range b.objs {
			if o.nr > n {
				n = o.nr
			}
		}
		fmt.Fprintf(&out, "xref\n0 %d\n0000000000 65535 f \n", n+1)
		for i := 1; i <= n; i++ {
			fmt.Fprintf(&out, "%010d 00000 n \n", nihked[i])
		}
		trailer := fmt.Sprintf("<< /Size %d /Root 1 0 R", n+1)
		if b.v.RC4 {
			trailer += fmt.Sprintf(" /Encrypt %d 0 R /ID [<%X> <%X>]", encNr, b.id, b.id)
		}
		fmt.Fprintf(&out, "trailer\n%s >>\nstartxref\n%d\n%%%%EOF\n", trailer, xref)
		return b.taidaByteRange(out.Bytes())
	}
	// pakitud: sõnastikud objektivoos, vood eraldi, xref-voog
	var sees []obj
	for _, o := range b.objs {
		if o.voog == nil && o.nr != b.allkiri && !(b.v.RC4 && o.nr == encNr) {
			sees = append(sees, o)
		} else {
			kirjutaObj(o)
		}
	}
	ovNr := encNr + 1
	var paise, keha strings.Builder
	indeks := map[int]int{}
	for i, o := range sees {
		fmt.Fprintf(&paise, "%d %d ", o.nr, keha.Len())
		keha.WriteString(o.dict + "\n")
		indeks[o.nr] = i
	}
	first := paise.Len()
	var z bytes.Buffer
	w := zlib.NewWriter(&z)
	w.Write([]byte(paise.String() + keha.String()))
	w.Close()
	ovData := z.Bytes()
	if b.v.RC4 {
		ovData = b.rc4(ovNr, ovData)
	}
	nihked[ovNr] = out.Len()
	fmt.Fprintf(&out, "%d 0 obj\n<< /Type /ObjStm /N %d /First %d /Length %d /Filter /FlateDecode >>\nstream\n", ovNr, len(sees), first, len(ovData))
	out.Write(ovData)
	out.WriteString("\nendstream\nendobj\n")
	xrefNr := ovNr + 1
	nihked[xrefNr] = out.Len()
	var rows bytes.Buffer
	for i := 0; i <= xrefNr; i++ {
		if j, ok := indeks[i]; ok {
			rows.Write([]byte{2, 0, 0, byte(ovNr >> 8), byte(ovNr), 0, byte(j)})
			continue
		}
		off, ok := nihked[i]
		if !ok {
			rows.Write([]byte{0, 0, 0, 0, 0, 0, 0})
			continue
		}
		rows.Write([]byte{1, byte(off >> 24), byte(off >> 16), byte(off >> 8), byte(off), 0, 0})
	}
	trailer := fmt.Sprintf("/Root 1 0 R /Size %d /W [1 4 2]", xrefNr+1)
	if b.v.RC4 {
		trailer += fmt.Sprintf(" /Encrypt %d 0 R /ID [<%X> <%X>]", encNr, b.id, b.id)
	}
	fmt.Fprintf(&out, "%d 0 obj\n<< /Type /XRef %s /Length %d >>\nstream\n", xrefNr, trailer, rows.Len())
	out.Write(rows.Bytes())
	fmt.Fprintf(&out, "\nendstream\nendobj\nstartxref\n%d\n%%%%EOF\n", nihked[xrefNr])
	return b.taidaByteRange(out.Bytes())
}

// taidaByteRange täidab allkirja /ByteRange nii, et see katab kogu faili
// peale /Contents väärtuse.
func (b *ehitaja) taidaByteRange(f []byte) []byte {
	if b.v.Allkiri == nil {
		return f
	}
	i := bytes.Index(f, []byte("/ByteRange [0 0000000000 0000000000 0000000000]"))
	c := bytes.Index(f, []byte("/Contents <"))
	if i < 0 || c < 0 {
		return f
	}
	a := c + len("/Contents ")
	e := bytes.IndexByte(f[a:], '>') + a + 1
	br := fmt.Sprintf("/ByteRange [0 %010d %010d %010d]", a, e, len(f)-e)
	copy(f[i:], br)
	return f
}
