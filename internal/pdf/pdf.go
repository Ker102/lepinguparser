// Package pdf loeb PDF-dokumendist teksti lõikudena, et lepingu struktuuri
// saaks analüüsida samamoodi nagu .docx faili.
//
// PDF-is ei ole lõike ega nummerdust, vaid ainult lehele paigutatud märgid.
// Pakett taastab read märkide asukohtade järgi, eemaldab korduvad päised,
// jalused ja leheküljenumbrid, ühendab read lõikudeks (sh poolitatud sõnad)
// ja tuvastab rea alguses olevad punktinumbrid (7.2., (a), i)). Eesti tähed
// (õ, ä, ö, ü, š, ž) dekodeeritakse fondi ToUnicode tabelist, kodeeringust
// või glüüfinimedest. Skannitud PDF-il (tekstikihita) teksti ei ole.
package pdf

import (
	"errors"
	"fmt"
	"strings"

	"github.com/Ker102/lepinguparser/internal/docx"
)

// ErrTekstita tagastatakse, kui PDF-is ei ole üldse teksti.
var ErrTekstita = errors.New("PDF-is ei ole tekstikihti (tõenäoliselt skannitud dokument): tekst tuleb enne OCR-tarkvaraga tuvastada")

type leheInfo struct {
	d       Dict
	res     Dict
	laius   float64
	korgus  float64
	poorata maatriks
}

// lehed tagastab lehekülgede loetelu koos päritud atribuutidega.
func (f *fail) lehed() []leheInfo {
	juur := f.sonastik(f.trailer["Root"])
	var out []leheInfo
	nahtud := map[any]bool{}
	var kaia func(o any, res Dict, kast Array, rot int, sug int)
	kaia = func(o any, res Dict, kast Array, rot int, sug int) {
		if sug > 64 || len(out) > 100000 {
			return
		}
		if r, ok := o.(Ref); ok {
			if nahtud[r] {
				return
			}
			nahtud[r] = true
		}
		d := f.sonastik(o)
		if d == nil {
			return
		}
		if r := f.sonastik(d["Resources"]); r != nil {
			res = r
		}
		if k := f.massiiv(d["MediaBox"]); len(k) == 4 {
			kast = k
		}
		if k := f.massiiv(d["CropBox"]); len(k) == 4 {
			kast = k
		}
		if v, ok := f.arv(d["Rotate"]); ok {
			rot = int(v)
		}
		kids := f.massiiv(d["Kids"])
		if f.nimi(d["Type"]) == "Pages" || kids != nil && d["Contents"] == nil {
			for _, k := range kids {
				kaia(k, res, kast, rot, sug+1)
			}
			return
		}
		li := leheInfo{d: d, res: res, laius: 595, korgus: 842}
		var x0, y0 float64
		if len(kast) == 4 {
			var v [4]float64
			for i := range v {
				v[i], _ = f.arv(kast[i])
			}
			x0, y0 = min(v[0], v[2]), min(v[1], v[3])
			li.laius, li.korgus = max(v[0], v[2])-x0, max(v[1], v[3])-y0
		}
		nihe := maatriks{1, 0, 0, 1, -x0, -y0}
		w, h := li.laius, li.korgus
		switch ((rot % 360) + 360) % 360 {
		case 90:
			li.poorata = nihe.korda(maatriks{0, -1, 1, 0, 0, w})
			li.laius, li.korgus = h, w
		case 180:
			li.poorata = nihe.korda(maatriks{-1, 0, 0, -1, w, h})
		case 270:
			li.poorata = nihe.korda(maatriks{0, 1, -1, 0, h, 0})
			li.laius, li.korgus = h, w
		default:
			li.poorata = nihe
		}
		out = append(out, li)
	}
	if juur != nil {
		kaia(juur["Pages"], nil, nil, 0, 0)
	}
	return out
}

// Read loeb PDF-i ja tagastab lõigud samas kujus nagu docx.Read.
func Read(data []byte) (doc *docx.Document, err error) {
	defer func() {
		if r := recover(); r != nil {
			doc, err = nil, fmt.Errorf("%w: PDF-i lugemine katkes (%v)", ErrNotPDF, r)
		}
	}()
	f, err := ava(data)
	if err != nil {
		return nil, err
	}
	lehed := f.lehed()
	if len(lehed) == 0 {
		return nil, fmt.Errorf("%w: lehekülgi ei leitud", ErrNotPDF)
	}
	r := &lugeja{f: f, fondid: map[Ref]*font{}}
	read := make([][]rida, len(lehed))
	korgused := make([]float64, len(lehed))
	var tekstita, vigased []int
	tundmatu, margid, pooratud := 0, 0, 0
	for i, li := range lehed {
		li.d["Resources"] = li.res
		tul := r.leheMargid(li.d, li.poorata)
		korgused[i] = li.korgus
		read[i] = reaKs(tul.margid, i+1)
		if len(read[i]) == 0 {
			tekstita = append(tekstita, i+1)
		}
		if tul.vigased {
			vigased = append(vigased, i+1)
		}
		tundmatu += tul.tundmatu
		margid += len(tul.margid)
		pooratud += tul.poorata
	}
	if len(tekstita) == len(lehed) {
		return nil, ErrTekstita
	}
	read, _ = eemaldaPaisedJalused(read, korgused)
	doc = &docx.Document{Format: "pdf", Paragraphs: koostaLoigud(read, korgused)}
	doc.Warnings = append(doc.Warnings, "PDF: lõigud ja punktinumbrid on taastatud lehe paigutusest (PDF-is ei ole Wordi nummerdust); kontrolli olulisi kohti originaalist")
	if len(tekstita) > 0 {
		doc.Warnings = append(doc.Warnings, fmt.Sprintf("lehekülgedel %s ei ole teksti (skannitud leht või pilt?); nende sisu analüüsis ei ole", loetelu(tekstita)))
	}
	if len(vigased) > 0 {
		doc.Warnings = append(doc.Warnings, fmt.Sprintf("lehekülgede %s sisu ei õnnestunud täielikult lugeda", loetelu(vigased)))
	}
	if margid > 0 && tundmatu*50 > margid {
		doc.Warnings = append(doc.Warnings, fmt.Sprintf("%d märki ei õnnestunud tekstiks teisendada (fondil puudub Unicode'i tabel); tekstis võib olla puuduvaid tähti", tundmatu))
	}
	if pooratud > 20 {
		doc.Warnings = append(doc.Warnings, "pööratud või püstine tekst (nt lehe äärel) jäeti välja")
	}
	if f.kruptija != nil {
		doc.Warnings = append(doc.Warnings, "PDF on kaitstud omaniku parooliga (nt kopeerimis- või muutmispiirang); tekst loeti analüüsiks")
	}
	if all := f.allkirjad(); len(all) > 0 {
		doc.Warnings = append(doc.Warnings, fmt.Sprintf("PDF sisaldab %d digiallkirja või ajatemplit; kehtivust ei kontrollita (vt pdf_teave)", len(all)))
		if !all[len(all)-1].KatabKogu {
			doc.Warnings = append(doc.Warnings, "PDF-i on pärast viimast allkirja muudetud: analüüsitud tekst ei pruugi olla allkirjastatud versioon")
		}
	}
	return doc, nil
}

func loetelu(n []int) string {
	var s []string
	for i, x := range n {
		if i == 15 {
			s = append(s, "…")
			break
		}
		s = append(s, fmt.Sprint(x))
	}
	return strings.Join(s, ", ")
}

// Teave kirjeldab PDF-faili: metaandmed, lehekülgede arv ja allkirjad.
type Teave struct {
	Lehekulgi     int       `json:"lehekulgi"`
	Pealkiri      string    `json:"pealkiri,omitempty"`
	Autor         string    `json:"autor,omitempty"`
	Rakendus      string    `json:"rakendus,omitempty"`
	Tootja        string    `json:"tootja,omitempty"`
	Loodud        string    `json:"loodud,omitempty"`
	Muudetud      string    `json:"muudetud,omitempty"`
	Kaitstud      bool      `json:"kaitstud"`
	Margendatud   bool      `json:"margendatud"`
	TekstitaLehed []int     `json:"tekstita_lehed"`
	Allkirjad     []Allkiri `json:"allkirjad"`
	Hoiatused     []string  `json:"hoiatused"`
}

// Info tagastab PDF-faili kirjelduse.
func Info(data []byte) (t *Teave, err error) {
	defer func() {
		if r := recover(); r != nil {
			t, err = nil, fmt.Errorf("%w: PDF-i lugemine katkes (%v)", ErrNotPDF, r)
		}
	}()
	f, err := ava(data)
	if err != nil {
		return nil, err
	}
	t = &Teave{TekstitaLehed: []int{}, Allkirjad: []Allkiri{}, Hoiatused: []string{}}
	lehed := f.lehed()
	t.Lehekulgi = len(lehed)
	if info := f.sonastik(f.trailer["Info"]); info != nil {
		s := func(k Name) string { return strings.TrimSpace(pdfDoc(strOf(f.lahenda(info[k])))) }
		t.Pealkiri, t.Autor, t.Rakendus, t.Tootja = s("Title"), s("Author"), s("Creator"), s("Producer")
		t.Loodud, t.Muudetud = pdfKuupaev(s("CreationDate")), pdfKuupaev(s("ModDate"))
	}
	t.Kaitstud = f.trailer["Encrypt"] != nil
	if juur := f.sonastik(f.trailer["Root"]); juur != nil {
		if mi := f.sonastik(juur["MarkInfo"]); mi != nil {
			t.Margendatud, _ = f.lahenda(mi["Marked"]).(bool)
		}
	}
	r := &lugeja{f: f, fondid: map[Ref]*font{}}
	for i, li := range lehed {
		li.d["Resources"] = li.res
		if len(r.leheMargid(li.d, li.poorata).margid) == 0 {
			t.TekstitaLehed = append(t.TekstitaLehed, i+1)
		}
	}
	if len(t.TekstitaLehed) == len(lehed) {
		t.Hoiatused = append(t.Hoiatused, ErrTekstita.Error())
	}
	if all := f.allkirjad(); all != nil {
		t.Allkirjad = all
	}
	if len(t.Allkirjad) > 0 {
		t.Hoiatused = append(t.Hoiatused, "allkirjade kehtivust ei kontrollita – kasuta DigiDoc4 rakendust")
		if !t.Allkirjad[len(t.Allkirjad)-1].KatabKogu {
			t.Hoiatused = append(t.Hoiatused, "PDF-i on pärast viimast allkirja muudetud")
		}
	}
	return t, nil
}

// pdfKuupaev teisendab "D:20240115123045+02'00'" kujule "2024-01-15T12:30:45+02:00".
func pdfKuupaev(s string) string {
	s = strings.TrimPrefix(strings.TrimSpace(s), "D:")
	if len(s) < 8 {
		return s
	}
	for _, c := range s[:8] {
		if c < '0' || c > '9' {
			return s
		}
	}
	osa := func(a, b int, vaike string) string {
		if len(s) >= b {
			return s[a:b]
		}
		return vaike
	}
	out := s[0:4] + "-" + s[4:6] + "-" + s[6:8]
	if len(s) >= 12 {
		out += "T" + osa(8, 10, "00") + ":" + osa(10, 12, "00") + ":" + osa(12, 14, "00")
		if len(s) > 14 {
			tz := strings.ReplaceAll(s[14:], "'", "")
			switch {
			case tz == "Z" || strings.HasPrefix(tz, "Z"):
				out += "Z"
			case len(tz) >= 5 && (tz[0] == '+' || tz[0] == '-'):
				out += tz[:3] + ":" + tz[3:5]
			case len(tz) == 3 && (tz[0] == '+' || tz[0] == '-'):
				out += tz + ":00"
			}
		}
	}
	return out
}
