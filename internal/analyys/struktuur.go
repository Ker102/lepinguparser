// Package analyys koostab .docx lõikudest lepingu struktuuri ning leiab
// definitsioonid, terminite kasutuse ja sisemised ristviited.
package analyys

import (
	"regexp"
	"sort"
	"strings"

	"github.com/Ker102/lepinguparser/internal/docx"
)

// Sektsioon on lepingu punkt, alapunkt või pealkirjaga osa.
type Sektsioon struct {
	// Number on normaliseeritud number, nt "7.2" või "7.2(a)" või "Lisa 1".
	Number string `json:"number"`
	// Silt on number dokumendis kuvatud kujul, nt "7.2." või "(a)".
	Silt     string `json:"silt,omitempty"`
	Pealkiri string `json:"pealkiri,omitempty"`
	Tase     int    `json:"tase"`
	// Allikas: "automaatnummerdus", "kasitsi" (number tekstis) või "pealkiri".
	Allikas     string `json:"allikas"`
	AlgusLoik   int    `json:"algus_loik"`
	LoppLoik    int    `json:"lopp_loik"`
	Vanem       string `json:"vanem,omitempty"`
	tekstLoigud []int
}

// Dokument on analüüsiks ette valmistatud leping.
type Dokument struct {
	Loigud      []docx.Paragraph
	Sektsioonid []*Sektsioon
	// loik -> sisemise sektsiooni indeks (-1 kui enne esimest sektsiooni)
	loikSektsioon []int
	Hoiatused     []string
}

var (
	kasitsiNumber = regexp.MustCompile(`^\s*((?:\d{1,3}\.)+\d{0,3}|\d{1,3}\.)\s+\S`)
	lisaPealkiri  = regexp.MustCompile(`(?i)^\s*(lisa|annex|schedule|appendix)\s+(nr\.?\s*)?(\d{1,2}|[A-Z])\b`)
	numbriTee     = regexp.MustCompile(`^\d+(?:\.\d+)*$`)
	sildiPuhastus = regexp.MustCompile(`^[\s(\[]*|[\s.)\]:]*$`)
)

// NormaliseeriNumber teisendab sildi võrreldavaks võtmeks: "7.2." -> "7.2", "(a)" -> "a".
func NormaliseeriNumber(s string) string {
	s = strings.TrimSpace(s)
	// "3.3(b)" säilitab sulud; "(a)" ja "a)" muutuvad kujule "a"
	if i := strings.Index(s, "("); i > 0 && strings.HasSuffix(strings.TrimRight(s, ".: "), ")") {
		return strings.ReplaceAll(strings.TrimRight(s, ".: "), " ", "")
	}
	s = sildiPuhastus.ReplaceAllString(s, "")
	s = strings.TrimSuffix(s, ".")
	return s
}

// Koosta analüüsib lõigud sektsioonideks.
func Koosta(d *docx.Document) *Dokument {
	doc := &Dokument{Loigud: d.Paragraphs, Hoiatused: append([]string(nil), d.Warnings...)}
	doc.loikSektsioon = make([]int, len(d.Paragraphs))
	type avatud struct {
		s   *Sektsioon
		idx int
	}
	var pinu []avatud
	cur := -1
	autoNummerdus := 0
	for i, p := range d.Paragraphs {
		var s *Sektsioon
		switch {
		case p.Label != "" && p.NumLevel >= 0:
			s = &Sektsioon{Silt: p.Label, Number: NormaliseeriNumber(p.Label), Tase: p.NumLevel + 1, Allikas: "automaatnummerdus"}
			autoNummerdus++
		case lisaPealkiri.MatchString(p.Text) && (p.HeadingLevel > 0 || len(p.Text) < 120):
			m := lisaPealkiri.FindStringSubmatch(p.Text)
			s = &Sektsioon{Silt: strings.TrimSpace(m[0]), Number: "Lisa " + strings.ToUpper(m[3]), Tase: 1, Allikas: "pealkiri"}
		case kasitsiNumber.MatchString(p.Text) && !p.InTable:
			m := kasitsiNumber.FindStringSubmatch(p.Text)
			nr := NormaliseeriNumber(m[1])
			s = &Sektsioon{Silt: m[1], Number: nr, Tase: strings.Count(nr, ".") + 1, Allikas: "kasitsi"}
		case p.HeadingLevel > 0 && strings.TrimSpace(p.Text) != "":
			s = &Sektsioon{Silt: "", Number: "", Tase: p.HeadingLevel, Allikas: "pealkiri"}
		}
		if s != nil {
			s.AlgusLoik = i
			s.Pealkiri = pealkiri(p.Text, s.Allikas == "kasitsi")
			for len(pinu) > 0 && pinu[len(pinu)-1].s.Tase >= s.Tase {
				pinu = pinu[:len(pinu)-1]
			}
			if len(pinu) > 0 {
				vanem := pinu[len(pinu)-1].s
				s.Vanem = vanem.Number
				// Tähtedega alapunktid seotakse vanema numbriga: 7.2 + (a) -> 7.2(a)
				if s.Number != "" && !numbriTee.MatchString(s.Number) && !strings.HasPrefix(s.Number, "Lisa ") && numbriTee.MatchString(lastNumeric(vanem.Number)) {
					s.Number = vanem.Number + "(" + s.Number + ")"
				}
			}
			if s.Number == "" {
				s.Number = "pealkiri:" + s.Pealkiri
			}
			doc.Sektsioonid = append(doc.Sektsioonid, s)
			cur = len(doc.Sektsioonid) - 1
			pinu = append(pinu, avatud{s, cur})
		}
		doc.loikSektsioon[i] = cur
		for _, a := range pinu {
			a.s.LoppLoik = i
		}
	}
	korduvad := map[string]int{}
	for _, s := range doc.Sektsioonid {
		if !strings.HasPrefix(s.Number, "pealkiri:") {
			korduvad[s.Number]++
		}
	}
	var dup []string
	for nr, n := range korduvad {
		if n > 1 {
			dup = append(dup, nr)
		}
	}
	if len(dup) > 0 {
		sort.Strings(dup)
		if len(dup) > 10 {
			dup = append(dup[:10], "…")
		}
		doc.Hoiatused = append(doc.Hoiatused, "korduvad punktinumbrid (viited neile on mitmetähenduslikud, kasutatakse esimest): "+strings.Join(dup, ", "))
	}
	if autoNummerdus == 0 && len(d.Paragraphs) > 0 {
		doc.Hoiatused = append(doc.Hoiatused, "dokumendis ei leitud Wordi automaatnummerdust; struktuur põhineb tekstis kirjutatud numbritel ja pealkirjadel")
	}
	return doc
}

func lastNumeric(s string) string {
	if i := strings.Index(s, "("); i > 0 {
		return s[:i]
	}
	return s
}

func pealkiri(text string, eemaldaNumber bool) string {
	t := text
	if eemaldaNumber {
		if loc := kasitsiNumber.FindStringSubmatchIndex(text); loc != nil {
			t = text[loc[3]:]
		}
	}
	t = strings.TrimSpace(t)
	if i := strings.IndexAny(t, "\n"); i > 0 {
		t = t[:i]
	}
	r := []rune(t)
	if len(r) > 90 {
		// lühike pealkiri: esimene lause või 90 märki
		cut := 90
		for j := 20; j < len(r) && j < 90; j++ {
			if r[j] == '.' || r[j] == ':' {
				cut = j
				break
			}
		}
		return strings.TrimSpace(string(r[:cut])) + "…"
	}
	return t
}

// SektsioonLoigule tagastab lõigu sisemise sektsiooni (või nil).
func (d *Dokument) SektsioonLoigule(i int) *Sektsioon {
	if i < 0 || i >= len(d.loikSektsioon) || d.loikSektsioon[i] < 0 {
		return nil
	}
	return d.Sektsioonid[d.loikSektsioon[i]]
}

// LeiaSektsioon otsib sektsiooni numbri järgi ("7.2", "7.2.", "Lisa 1").
func (d *Dokument) LeiaSektsioon(nr string) *Sektsioon {
	key := NormaliseeriNumber(nr)
	if strings.HasPrefix(strings.ToLower(key), "lisa") {
		key = "Lisa " + strings.ToUpper(strings.TrimSpace(key[4:]))
	}
	for _, s := range d.Sektsioonid {
		if s.Number == key || strings.EqualFold(s.Number, key) {
			return s
		}
	}
	return nil
}

// SektsiooniTekst tagastab sektsiooni lõigud (koos alapunktidega).
func (d *Dokument) SektsiooniTekst(s *Sektsioon) []docx.Paragraph {
	if s == nil {
		return nil
	}
	return d.Loigud[s.AlgusLoik : s.LoppLoik+1]
}
