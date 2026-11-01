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
)

// MaxFailiSuurus piirab loetava faili suurust.
const MaxFailiSuurus = 100 << 20

// JuurteMuutuja on keskkonnamuutuja, millega saab lubatud kaustu piirata
// (eraldajaks ":" või ";"). Kui see on määramata, on lubatud kõik failid.
const JuurteMuutuja = "LEPINGUPARSER_JUURED"

// ErrKeelatud tagastatakse, kui fail on väljaspool lubatud kaustu.
var ErrKeelatud = errors.New("fail on väljaspool lubatud kaustu (" + JuurteMuutuja + ")")

// Ava loeb ja analüüsib .docx faili või digiallkirja konteineri sees oleva
// .docx faili.
func Ava(tee string) (*analyys.Dokument, error) {
	return AvaSees(tee, "")
}

// AvaSees on nagu Ava, kuid konteineri puhul saab määrata sisemise faili nime.
func AvaSees(tee, sisemine string) (*analyys.Dokument, error) {
	b, ext, err := loeFail(tee)
	if err != nil {
		return nil, err
	}
	var hoiatus string
	switch {
	case ext == ".docx":
	case konteiner.Laiendid[ext]:
		data, nimi, _, err := konteiner.ValiDocx(b, sisemine)
		if err != nil {
			return nil, err
		}
		b = data
		hoiatus = "analüüsitud digiallkirja konteineri fail " + nimi + " (allkirjade kehtivust ei kontrollitud)"
	case ext == ".ddoc":
		return nil, konteiner.ErrDDOC
	default:
		return nil, fmt.Errorf("toetatud on .docx ja digiallkirja konteinerid (.asice, .bdoc); sain %q", ext)
	}
	d, err := docx.Read(b)
	if err != nil {
		return nil, err
	}
	doc := analyys.Koosta(d)
	if hoiatus != "" {
		doc.Hoiatused = append([]string{hoiatus}, doc.Hoiatused...)
	}
	return doc, nil
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
