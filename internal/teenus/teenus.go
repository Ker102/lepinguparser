// Package teenus on CLI ja MCP serveri ühine kiht: faili avamine,
// juurdepääsupiirangud ja tulemuste kuju.
package teenus

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/Ker102/lepinguparser/internal/analyys"
	"github.com/Ker102/lepinguparser/internal/docx"
	"github.com/Ker102/lepinguparser/internal/konteiner"
	"github.com/Ker102/lepinguparser/internal/pdf"
)

// MaxFailiSuurus piirab loetava faili suurust.
const MaxFailiSuurus = 100 << 20

// JuurteMuutuja on keskkonnamuutuja, millega saab lubatud kaustu piirata
// (eraldajaks ":" või ";"). Kui see on määramata, on lubatud kõik failid.
const JuurteMuutuja = "LEPINGUPARSER_JUURED"

// ErrKeelatud tagastatakse, kui fail on väljaspool lubatud kaustu.
var ErrKeelatud = errors.New("fail on väljaspool lubatud kaustu (" + JuurteMuutuja + ")")

// Ava loeb ja analüüsib .docx või PDF-faili või digiallkirja konteineri
// sees oleva .docx või PDF-faili.
func Ava(tee string) (*analyys.Dokument, error) {
	return AvaSees(tee, "")
}

// AvaSees on nagu Ava, kuid konteineri puhul saab määrata sisemise faili nime.
func AvaSees(tee, sisemine string) (*analyys.Dokument, error) {
	b, ext, nimi, err := loeDokument(tee, sisemine)
	if err != nil {
		return nil, err
	}
	var d *docx.Document
	if ext == ".pdf" {
		d, err = pdf.Read(b)
	} else {
		d, err = docx.Read(b)
	}
	if err != nil {
		return nil, err
	}
	doc := analyys.Koosta(d)
	if nimi != "" {
		doc.Hoiatused = append([]string{"analüüsitud digiallkirja konteineri fail " + nimi + " (allkirjade kehtivust ei kontrollitud)"}, doc.Hoiatused...)
	}
	return doc, nil
}

// loeDokument loeb analüüsitava faili baidid. Konteineri puhul tagastab
// sisemise faili ja selle nime.
func loeDokument(tee, sisemine string) ([]byte, string, string, error) {
	b, ext, err := loeFail(tee)
	if err != nil {
		return nil, "", "", err
	}
	switch {
	case ext == ".docx" || ext == ".pdf":
		return b, ext, "", nil
	case konteiner.Laiendid[ext]:
		data, nimi, _, err := konteiner.ValiDokument(b, sisemine)
		if err != nil {
			return nil, "", "", err
		}
		return data, strings.ToLower(filepath.Ext(nimi)), nimi, nil
	case ext == ".ddoc":
		return nil, "", "", konteiner.ErrDDOC
	}
	return nil, "", "", fmt.Errorf("toetatud on .docx, .pdf ja digiallkirja konteinerid (.asice, .bdoc); sain %q", ext)
}

// PDFTeave tagastab PDF-faili (ka konteineri sees oleva) metaandmed,
// lehekülgede arvu ja PDF-i manustatud digiallkirjad.
func PDFTeave(tee, sisemine string) (*pdf.Teave, error) {
	b, ext, nimi, err := loeDokument(tee, sisemine)
	if err != nil {
		return nil, err
	}
	if ext != ".pdf" {
		return nil, fmt.Errorf("%q ei ole PDF-fail", filepath.Base(tee))
	}
	t, err := pdf.Info(b)
	if err != nil {
		return nil, err
	}
	if nimi != "" {
		t.Hoiatused = append([]string{"konteineri fail " + nimi}, t.Hoiatused...)
	}
	return t, nil
}

// KonteineriSisu loetleb digiallkirja konteineri failid.
func KonteineriSisu(tee string) (*konteiner.Sisu, error) {
	b, ext, err := loeFail(tee)
	if err != nil {
		return nil, err
	}
	if !konteiner.Laiendid[ext] && ext != ".ddoc" {
		return nil, fmt.Errorf("%q ei ole digiallkirja konteiner (.asice, .bdoc)", ext)
	}
	s, _, err := konteiner.Loe(b)
	return s, err
}

func loeFail(tee string) ([]byte, string, error) {
	abs, err := filepath.Abs(tee)
	if err != nil {
		return nil, "", err
	}
	if err := kontrolliJuuri(abs); err != nil {
		return nil, "", err
	}
	st, err := os.Stat(abs)
	if err != nil {
		return nil, "", err
	}
	if st.Size() > MaxFailiSuurus {
		return nil, "", fmt.Errorf("fail on liiga suur (%d baiti, piir %d)", st.Size(), MaxFailiSuurus)
	}
	b, err := os.ReadFile(abs)
	if err != nil {
		return nil, "", err
	}
	return b, strings.ToLower(filepath.Ext(abs)), nil
}

func kontrolliJuuri(abs string) error {
	v := strings.TrimSpace(os.Getenv(JuurteMuutuja))
	if v == "" {
		return nil
	}
	real, err := filepath.EvalSymlinks(abs)
	if err != nil {
		real = abs
	}
	for _, j := range strings.FieldsFunc(v, func(r rune) bool { return r == ':' || r == ';' }) {
		ja, err := filepath.Abs(j)
		if err != nil {
			continue
		}
		if jr, err := filepath.EvalSymlinks(ja); err == nil {
			ja = jr
		}
		rel, err := filepath.Rel(ja, real)
		if err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			return nil
		}
	}
	return ErrKeelatud
}

// SisukorraRida on sisukorra kirje.
type SisukorraRida struct {
	Number   string `json:"number"`
	Pealkiri string `json:"pealkiri"`
	Tase     int    `json:"tase"`
	Allikas  string `json:"allikas"`
	Loik     int    `json:"loik"`
}

// Sisukord tagastab dokumendi struktuuri.
type Sisukord struct {
	Read      []SisukorraRida `json:"read"`
	Hoiatused []string        `json:"hoiatused"`
}

func TeeSisukord(d *analyys.Dokument, maxTase int) Sisukord {
	out := Sisukord{Read: []SisukorraRida{}, Hoiatused: nonNil(d.Hoiatused)}
	for _, s := range d.Sektsioonid {
		if maxTase > 0 && s.Tase > maxTase {
			continue
		}
		out.Read = append(out.Read, SisukorraRida{Number: s.Number, Pealkiri: s.Pealkiri, Tase: s.Tase, Allikas: s.Allikas, Loik: s.AlgusLoik})
	}
	return out
}

// SektsiooniVastus on ühe punkti tekst.
type SektsiooniVastus struct {
	Number   string       `json:"number"`
	Leitud   bool         `json:"leitud"`
	Loigud   []LoiguTekst `json:"loigud"`
	Sarnased []string     `json:"sarnased,omitempty"`
}

// LoiguTekst on lõigu number ja tekst.
type LoiguTekst struct {
	Loik   int    `json:"loik"`
	Number string `json:"number,omitempty"`
	Tekst  string `json:"tekst"`
}

func TeeSektsioon(d *analyys.Dokument, nr string) SektsiooniVastus {
	s := d.LeiaSektsioon(nr)
	v := SektsiooniVastus{Number: nr, Loigud: []LoiguTekst{}}
	if s == nil {
		key := analyys.NormaliseeriNumber(nr)
		for _, x := range d.Sektsioonid {
			if strings.HasPrefix(x.Number, key) || strings.HasPrefix(key, x.Number+".") {
				v.Sarnased = append(v.Sarnased, x.Number)
				if len(v.Sarnased) >= 10 {
					break
				}
			}
		}
		return v
	}
	v.Leitud = true
	v.Number = s.Number
	for _, p := range d.SektsiooniTekst(s) {
		if p.Text == "" {
			continue
		}
		lt := LoiguTekst{Loik: p.Index, Tekst: p.Text}
		if ss := d.SektsioonLoigule(p.Index); ss != nil && ss.AlgusLoik == p.Index {
			lt.Number = ss.Number
		}
		v.Loigud = append(v.Loigud, lt)
	}
	return v
}

func nonNil(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}

// TekstiLoik on dokumendi lõik koos numbri ja leheküljega.
type TekstiLoik struct {
	Loik     int    `json:"loik"`
	Lk       int    `json:"lk,omitempty"`
	Number   string `json:"number,omitempty"`
	Pealkiri bool   `json:"pealkiri,omitempty"`
	Tekst    string `json:"tekst"`
}

// Tekst on dokumendi kogu tekst lõikudena.
type Tekst struct {
	Loigud    []TekstiLoik `json:"loigud"`
	Hoiatused []string     `json:"hoiatused"`
}

// TeeTekst tagastab dokumendi lõigud koos punktinumbritega (Wordi
// automaatnummerduse numbrid on lõigu ees, nagu dokumendis näha).
func TeeTekst(d *analyys.Dokument, alates, kuni int) Tekst {
	out := Tekst{Loigud: []TekstiLoik{}, Hoiatused: nonNil(d.Hoiatused)}
	for _, p := range d.Loigud {
		if p.Text == "" && p.Label == "" || alates > 0 && p.Index < alates || kuni > 0 && p.Index > kuni {
			continue
		}
		out.Loigud = append(out.Loigud, TekstiLoik{Loik: p.Index, Lk: p.Page, Number: p.Label, Pealkiri: p.HeadingLevel > 0, Tekst: p.Text})
	}
	return out
}
