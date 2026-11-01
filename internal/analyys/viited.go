package analyys

import (
	"regexp"
	"sort"
	"strings"
	"unicode/utf8"
)

// Viide on sisemine ristviide teisele punktile või lisale.
type Viide struct {
	Loik      int    `json:"loik"`
	Sektsioon string `json:"sektsioon,omitempty"`
	Tekst     string `json:"tekst"`
	Siht      string `json:"siht"`
	// Olek: "leitud", "puudub" (sihtpunkti ei ole), "lisa_eraldi" (lisa ei ole
	// samas failis), "valine" (viide seadusele või muule aktile).
	Olek     string `json:"olek"`
	Kontekst string `json:"kontekst"`
}

const nrMuster = `\d{1,3}(?:\.\d{1,3})*\.?(?:\s?\([a-zA-Z0-9]{1,4}\))?`

var (
	viiteRe = regexp.MustCompile(`(?i)\b(punkt[a-zõäöü]*|alapunkt[a-zõäöü]*|p-[a-zõäöü]+|p\.|p|lõi[a-zõäöü]*|lg|jaotis[a-zõäöü]*|peatük[a-zõäöü]*|klausl[a-zõäöü]*|clauses?|sections?|paragraphs?|articles?|artikl[a-zõäöü]*)\s+(` + nrMuster + `(?:\s*(?:,|ja|või|and|or|to|–|-|kuni)\s*` + nrMuster + `)*)`)
	uksNr   = regexp.MustCompile(nrMuster)
	// "Lisa 1", "lisas 2", "Lisadest 1–2", "Annex A"; tähtlisa peab olema suurtäht
	lisaRe = regexp.MustCompile(`\b(?i:(lisa(?:s|st|sse|le|l|lt|ga|ks|na|ni|ta|d|de|des|dest|desse|dele|del|delt|dega|deks|dena|deni|deta|sid|t)?|annex(?:es)?|schedules?|appendix|appendices))\s+(?:(?i:nr)\.?\s*)?(\d{1,2}|[A-Z])\b`)
	// välisele aktile viitav kontekst enne viidet: §, seaduse/määruse/direktiivi
	// nimi või seaduse lühend (VÕS, TsÜS, KeÜS – suurtähtedega, lõpus S)
	valineEel = regexp.MustCompile(`§|(?i:seadus|seadustik|määrus|direktiiv|regulation|directive|konventsioon)|\b[A-ZÕÄÖÜ][A-ZÕÄÖÜa-zõäöü]{0,4}S\b`)
	// välisele aktile viitav kontekst vahetult pärast viidet: "of the Act", "seaduse"
	valineJarel = regexp.MustCompile(`^\s*(?:(?i:of|in|under)\s+(?:the\s+)?[A-Z]|[,]?\s*(?i:seaduse|seadustiku|määruse|direktiivi|regulation|directive))`)
)

// eelAken tagastab kuni n baiti enne positsiooni, lõigates UTF-8 märgi piiril.
func eelAken(t string, pos, n int) string {
	a := pos - n
	if a < 0 {
		a = 0
	}
	for a > 0 && a < len(t) && !utf8.RuneStart(t[a]) {
		a++
	}
	return t[a:pos]
}

func jarelAken(t string, pos, n int) string {
	b := pos + n
	if b > len(t) {
		b = len(t)
	}
	for b < len(t) && !utf8.RuneStart(t[b]) {
		b++
	}
	return t[pos:b]
}

// LeiaViited leiab sisemised ristviited ja kontrollib, kas sihtpunkt on olemas.
func (d *Dokument) LeiaViited() []Viide {
	numbrid := map[string]bool{}
	lisad := map[string]bool{}
	for _, s := range d.Sektsioonid {
		numbrid[s.Number] = true
		if strings.HasPrefix(s.Number, "Lisa ") {
			lisad[strings.TrimPrefix(s.Number, "Lisa ")] = true
		}
	}
	var out []Viide
	for i, p := range d.Loigud {
		t := p.Text
		for _, loc := range viiteRe.FindAllStringSubmatchIndex(t, -1) {
			marksona := strings.ToLower(t[loc[2]:loc[3]])
			loend := t[loc[4]:loc[5]]
			eel := eelAken(t, loc[0], 30)
			jarel := jarelAken(t, loc[1], 30)
			valine := valineEel.MatchString(eel) || valineJarel.MatchString(jarel)
			if marksona == "lg" || strings.HasPrefix(marksona, "lõi") {
				// "lg"/"lõige" viitab lepingus peaaegu alati seaduse sättele
				valine = valine || strings.Contains(eelAken(t, loc[0], 60), "§")
			}
			for _, nr := range uksNr.FindAllString(loend, -1) {
				siht := NormaliseeriNumber(strings.ReplaceAll(nr, " ", ""))
				olek := "puudub"
				switch {
				case valine:
					olek = "valine"
				case numbrid[siht]:
					olek = "leitud"
				}
				out = append(out, Viide{Loik: i, Sektsioon: d.sektsiooniNr(i), Tekst: strings.TrimSpace(t[loc[0]:loc[1]]), Siht: siht, Olek: olek, Kontekst: kontekst(t, t[loc[0]:loc[1]])})
			}
		}
		for _, m := range lisaRe.FindAllStringSubmatchIndex(t, -1) {
			// lisa pealkiri ise ei ole viide
			if m[0] == 0 && d.SektsioonLoigule(i) != nil && d.SektsioonLoigule(i).AlgusLoik == i {
				continue
			}
			nr := strings.ToUpper(t[m[4]:m[5]])
			olek := "lisa_eraldi"
			if lisad[nr] {
				olek = "leitud"
			}
			out = append(out, Viide{Loik: i, Sektsioon: d.sektsiooniNr(i), Tekst: t[m[0]:m[1]], Siht: "Lisa " + nr, Olek: olek, Kontekst: kontekst(t, t[m[0]:m[1]])})
		}
	}
	return out
}

// KusViidatakse tagastab viited, mille siht on antud punkt või selle alapunkt.
func (d *Dokument) KusViidatakse(nr string, viited []Viide) []Viide {
	key := NormaliseeriNumber(nr)
	if strings.HasPrefix(strings.ToLower(key), "lisa") {
		key = "Lisa " + strings.ToUpper(strings.TrimSpace(key[4:]))
	}
	var out []Viide
	for _, v := range viited {
		if v.Siht == key || strings.HasPrefix(v.Siht, key+".") || strings.HasPrefix(v.Siht, key+"(") {
			out = append(out, v)
		}
	}
	return out
}

func max0(i int) int {
	if i < 0 {
		return 0
	}
	return i
}

// ---------- koondkontroll ----------

// Aruanne on koondkontrolli tulemus.
type Aruanne struct {
	Loike                      int                       `json:"loike"`
	Sektsioone                 int                       `json:"sektsioone"`
	Definitsioone              int                       `json:"definitsioone"`
	KatkisedViited             []Viide                   `json:"katkised_viited"`
	KasutamataTerminid         []Definitsioon            `json:"kasutamata_terminid"`
	TopeltDefinitsioonid       map[string][]Definitsioon `json:"topelt_definitsioonid"`
	KasutatudEnneDefineerimist []TerminEnne              `json:"kasutatud_enne_defineerimist"`
	DefineerimataKandidaadid   []Kandidaat               `json:"defineerimata_kandidaadid"`
	Hoiatused                  []string                  `json:"hoiatused"`
}

// TerminEnne näitab, et terminit kasutati enne definitsiooni lõiku.
type TerminEnne struct {
	Termin            string  `json:"termin"`
	DefinitsiooniLoik int     `json:"definitsiooni_loik"`
	EsimeneKasutus    Kasutus `json:"esimene_kasutus"`
}

// Kontroll teeb kõik kontrollid korraga.
func (d *Dokument) Kontroll() Aruanne {
	defs := d.LeiaDefinitsioonid()
	viited := d.LeiaViited()
	ar := Aruanne{Loike: len(d.Loigud), Sektsioone: len(d.Sektsioonid), Definitsioone: len(defs), TopeltDefinitsioonid: map[string][]Definitsioon{}, Hoiatused: d.Hoiatused}
	for _, v := range viited {
		if v.Olek == "puudub" {
			ar.KatkisedViited = append(ar.KatkisedViited, v)
		}
	}
	perTermin := map[string][]Definitsioon{}
	for _, df := range defs {
		perTermin[df.Termin] = append(perTermin[df.Termin], df)
	}
	terminid := make([]string, 0, len(perTermin))
	for t := range perTermin {
		terminid = append(terminid, t)
	}
	sort.Strings(terminid)
	for _, t := range terminid {
		ds := perTermin[t]
		if len(ds) > 1 {
			ar.TopeltDefinitsioonid[t] = ds
		}
		def := ds[0]
		kasutus := d.TerminiKasutus(t)
		muud := 0
		var esimene *Kasutus
		for k := range kasutus {
			if kasutus[k].Loik == def.Loik {
				continue
			}
			muud++
			if esimene == nil {
				esimene = &kasutus[k]
			}
		}
		if muud == 0 {
			ar.KasutamataTerminid = append(ar.KasutamataTerminid, def)
		}
		if esimene != nil && esimene.Loik < def.Loik && !d.onDefinitsiooniSektsioonis(esimene.Loik) && !d.onPealkiriLoik(esimene.Loik) {
			ar.KasutatudEnneDefineerimist = append(ar.KasutatudEnneDefineerimist, TerminEnne{Termin: t, DefinitsiooniLoik: def.Loik, EsimeneKasutus: *esimene})
		}
	}
	ar.DefineerimataKandidaadid = d.DefineerimataKandidaadid(defs, 30)
	if ar.KatkisedViited == nil {
		ar.KatkisedViited = []Viide{}
	}
	if ar.KasutamataTerminid == nil {
		ar.KasutamataTerminid = []Definitsioon{}
	}
	if ar.KasutatudEnneDefineerimist == nil {
		ar.KasutatudEnneDefineerimist = []TerminEnne{}
	}
	if ar.Hoiatused == nil {
		ar.Hoiatused = []string{}
	}
	return ar
}

// onPealkiriLoik: punkti alguslõik, mis on lühike (pealkiri, nt "6. Tasu").
func (d *Dokument) onPealkiriLoik(loik int) bool {
	s := d.SektsioonLoigule(loik)
	return s != nil && s.AlgusLoik == loik && utf8.RuneCountInString(d.Loigud[loik].Text) <= 80 && !strings.ContainsAny(strings.TrimRight(d.Loigud[loik].Text, ". "), ".;")
}

// Pealkirja- ja preambulilõigud (nt pooled) võivad termineid kasutada enne
// definitsioonide punkti; neid ei loeta veaks.
func (d *Dokument) onDefinitsiooniSektsioonis(loik int) bool {
	return d.SektsioonLoigule(loik) == nil
}
