package analyys

import (
	"regexp"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"
)

// Definitsioon on lepingus defineeritud termin.
type Definitsioon struct {
	Termin    string `json:"termin"`
	Loik      int    `json:"loik"`
	Sektsioon string `json:"sektsioon,omitempty"`
	// Viis: "edaspidi", "sulgudes", "loetelu" (termin lõigu alguses), "means".
	Viis     string `json:"viis"`
	Kontekst string `json:"kontekst"`
}

// Kasutus on termini esinemine tekstis.
type Kasutus struct {
	Loik      int    `json:"loik"`
	Sektsioon string `json:"sektsioon,omitempty"`
	Vorm      string `json:"vorm"`
	Kontekst  string `json:"kontekst"`
}

const (
	ava  = `[„“"«‘‚']`
	sulg = `[“”"»’‘']`
	keha = `([^„“”"«»\n]{1,80}?)`
)

var (
	sulud         = regexp.MustCompile(`\(([^()]{0,400})\)`)
	tsitaat       = regexp.MustCompile(ava + keha + sulg)
	edaspidi      = regexp.MustCompile(`(?i)\b(edaspidi|hereinafter|nimetatud ka|eraldi|koos|together|individually|each a|the)\b`)
	edaspidiAlgus = regexp.MustCompile(`(?i)\b(edaspidi|hereinafter)\b`)
	loeteluAlgus  = regexp.MustCompile(`^\s*(?:\(?[0-9a-z]{1,4}[.)]\s*)?` + ava + keha + sulg + `\s*(?:[-–—:]|tähendab|tähistab|on\b|means|shall mean|has the meaning|includes)`)
	meansInline   = regexp.MustCompile(ava + keha + sulg + `\s+(?:means|shall mean|has the meaning|tähendab|tähistab)\b`)
)

// LeiaDefinitsioonid leiab dokumendist defineeritud terminid.
func (d *Dokument) LeiaDefinitsioonid() []Definitsioon {
	var out []Definitsioon
	seen := map[string]bool{}
	add := func(term string, i int, viis string) {
		term = strings.TrimSpace(strings.Trim(term, " ,.;:"))
		if term == "" || utf8.RuneCountInString(term) > 80 {
			return
		}
		// Jutumärkideta definitsioonid on Eesti avaliku sektori lepingutes
		// tavaliselt väiketähega ("edaspidi tellija"); muudel juhtudel
		// eeldame suurtähte.
		if viis != "edaspidi_jutumarkideta" && !algabSuurega(term) {
			return
		}
		key := term + "|" + itoa(i)
		if seen[key] {
			return
		}
		seen[key] = true
		out = append(out, Definitsioon{Termin: term, Loik: i, Sektsioon: d.sektsiooniNr(i), Viis: viis, Kontekst: kontekst(d.Loigud[i].Text, term)})
	}
	for i, p := range d.Loigud {
		t := p.Text
		if t == "" {
			continue
		}
		if m := loeteluAlgus.FindStringSubmatch(t); m != nil {
			add(m[1], i, "loetelu")
		}
		for _, m := range meansInline.FindAllStringSubmatch(t, -1) {
			add(m[1], i, "means")
		}
		// "edaspidi koos „Pooled“ ja eraldi „Pool“" ilma sulgudeta: võta
		// tsitaadid märksõnale järgnevast lõigust kuni lause või sulu lõpuni.
		for _, loc := range edaspidiAlgus.FindAllStringIndex(t, -1) {
			aken := t[loc[1]:]
			if j := strings.IndexAny(aken, ").;"); j >= 0 {
				aken = aken[:j]
			}
			if len(aken) > 250 {
				aken = aken[:250]
			}
			qs := tsitaat.FindAllStringSubmatch(aken, -1)
			for _, q := range qs {
				add(q[1], i, "edaspidi")
			}
			if len(qs) == 0 {
				for _, term := range jutumarkidetaTerminid(aken) {
					add(term, i, "edaspidi_jutumarkideta")
				}
			}
		}
		for _, sm := range sulud.FindAllStringSubmatch(t, -1) {
			sees := sm[1]
			qs := tsitaat.FindAllStringSubmatch(sees, -1)
			if len(qs) == 0 {
				continue
			}
			ilmaTsitaatideta := strings.TrimSpace(tsitaat.ReplaceAllString(sees, ""))
			viis := ""
			switch {
			case edaspidi.MatchString(sees):
				viis = "edaspidi"
			case len(strings.Fields(ilmaTsitaatideta)) <= 3:
				viis = "sulgudes"
			default:
				continue // sulgudes tsitaat, mis ei näe välja nagu definitsioon
			}
			for _, q := range qs {
				add(q[1], i, viis)
			}
		}
	}
	return out
}

var (
	jmEraldaja   = regexp.MustCompile(`(?i)\s+(?:või|ja|and|or)\s+|,`)
	jmTaitesonad = regexp.MustCompile(`(?i)^(?:(?:nimetatud|nimetatakse|ka|kui|eraldi|koos|ühiselt|kumbki|individually|together|referred to as|as|the|:)\s*)+`)
)

// jutumarkidetaTerminid leiab terminid fraasist "edaspidi eraldi pool või koos
// pooled" või "(edaspidi nimetatud ka tellija". Aken lõpeb sulu, punkti või
// semikooloniga; koma lõpetab termini.
func jutumarkidetaTerminid(aken string) []string {
	if i := strings.Index(aken, ","); i >= 0 {
		// koma järel algab tavaliselt muu info (registrikood, aadress)
		aken = aken[:i]
	}
	var out []string
	for _, osa := range jmEraldaja.Split(aken, -1) {
		osa = strings.TrimSpace(jmTaitesonad.ReplaceAllString(strings.TrimSpace(osa), ""))
		osa = strings.Trim(osa, " :–-")
		n := len(strings.Fields(osa))
		if n == 0 || n > 3 {
			continue
		}
		r, _ := utf8.DecodeRuneInString(osa)
		if !unicode.IsLetter(r) {
			continue
		}
		out = append(out, osa)
	}
	return out
}

func (d *Dokument) sektsiooniNr(i int) string {
	if s := d.SektsioonLoigule(i); s != nil {
		return s.Number
	}
	return ""
}

func algabSuurega(s string) bool {
	r, _ := utf8.DecodeRuneInString(s)
	return unicode.IsUpper(r) || unicode.IsDigit(r)
}

func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	neg := i < 0
	if neg {
		i = -i
	}
	var b [20]byte
	p := len(b)
	for i > 0 {
		p--
		b[p] = byte('0' + i%10)
		i /= 10
	}
	if neg {
		p--
		b[p] = '-'
	}
	return string(b[p:])
}

func kontekst(text, fragment string) string {
	r := []rune(text)
	i := strings.Index(text, fragment)
	if i < 0 {
		if len(r) > 160 {
			return string(r[:160]) + "…"
		}
		return text
	}
	start := utf8.RuneCountInString(text[:i])
	a := start - 60
	if a < 0 {
		a = 0
	}
	b := start + utf8.RuneCountInString(fragment) + 80
	if b > len(r) {
		b = len(r)
	}
	s := string(r[a:b])
	if a > 0 {
		s = "…" + s
	}
	if b < len(r) {
		s += "…"
	}
	return s
}

// ---------- käänamistaluvus ----------

// Eesti käändelõpud koos levinud tüvevokaalidega. Kasutatakse lühikeste
// terminite (kuni 4 tähte, nt "Pool") puhul, kus tüveprefiks oleks liiga lai.
var lopud = func() map[string]bool {
	base := []string{"", "t", "d", "s", "l", "le", "lt", "ga", "ks", "na", "ta", "ni", "st", "sse", "de", "te", "id", "sid", "ile", "il", "ilt", "iga", "iks", "ina", "ita", "ini", "is", "ist", "isse", "e", "i", "u", "a"}
	m := map[string]bool{}
	for _, l := range base {
		m[l] = true
		for _, v := range []string{"e", "i", "u", "a"} {
			m[v+l] = true
			m[v+"d"+l] = true
		}
	}
	// ingliskeelsed mitmus/omastav
	for _, l := range []string{"s", "es", "'s", "’s", "s'"} {
		m[l] = true
	}
	return m
}()

type sonaSobitaja struct {
	vorm   string
	lyhike bool
	tyvi   string
	// vaike: termin on defineeritud väiketähega, seega sobib ka suurtähega
	// algav vorm (lause alguses).
	vaike bool
}

func vaiketahtAlgus(w string) string {
	r, n := utf8.DecodeRuneInString(w)
	return string(unicode.ToLower(r)) + w[n:]
}

func uusSobitaja(sona string) sonaSobitaja {
	n := utf8.RuneCountInString(sona)
	r0, _ := utf8.DecodeRuneInString(sona)
	s := sonaSobitaja{vorm: sona, vaike: unicode.IsLower(r0)}
	if n <= 4 {
		s.lyhike = true
		return s
	}
	r := []rune(sona)
	k := n - 2
	if k < 3 {
		k = 3
	}
	s.tyvi = string(r[:k])
	return s
}

func (s sonaSobitaja) sobib(w string) bool {
	if s.vaike {
		w = vaiketahtAlgus(w)
	}
	if w == s.vorm {
		return true
	}
	if s.lyhike {
		if !strings.HasPrefix(w, s.vorm) {
			return false
		}
		return lopud[strings.ToLower(w[len(s.vorm):])]
	}
	if !strings.HasPrefix(w, s.tyvi) {
		return false
	}
	return utf8.RuneCountInString(w)-utf8.RuneCountInString(s.tyvi) <= 8
}

type sona struct {
	tekst string
	algus int // baidipositsioon
}

var sonaRe = regexp.MustCompile(`[\p{L}\p{N}][\p{L}\p{N}'’\-]*`)

func sonad(t string) []sona {
	var out []sona
	for _, loc := range sonaRe.FindAllStringIndex(t, -1) {
		out = append(out, sona{t[loc[0]:loc[1]], loc[0]})
	}
	return out
}

// Terminisobitaja leiab termini käänatud vormid.
type Terminisobitaja struct {
	termin string
	osad   []sonaSobitaja
}

func UusTerminisobitaja(termin string) *Terminisobitaja {
	t := &Terminisobitaja{termin: termin}
	for _, w := range sonad(termin) {
		t.osad = append(t.osad, uusSobitaja(w.tekst))
	}
	return t
}

// leia tagastab vastete [algus, lõpp) baidipositsioonid.
func (t *Terminisobitaja) leia(text string) [][2]int {
	if len(t.osad) == 0 {
		return nil
	}
	ws := sonad(text)
	var out [][2]int
	for i := 0; i+len(t.osad) <= len(ws); i++ {
		ok := true
		for j, o := range t.osad {
			w := ws[i+j].tekst
			if !o.sobib(w) {
				ok = false
				break
			}
			// vahel ei tohi olla lauset lõpetavat märki
			if j > 0 && strings.ContainsAny(text[ws[i+j-1].algus+len(ws[i+j-1].tekst):ws[i+j].algus], ".;:!?") {
				ok = false
				break
			}
		}
		if ok {
			last := ws[i+len(t.osad)-1]
			out = append(out, [2]int{ws[i].algus, last.algus + len(last.tekst)})
			i += len(t.osad) - 1
		}
	}
	return out
}

// TerminiKasutus leiab termini esinemised (käänatud vormid kaasa arvatud).
func (d *Dokument) TerminiKasutus(termin string) []Kasutus {
	m := UusTerminisobitaja(termin)
	var out []Kasutus
	for i, p := range d.Loigud {
		for _, loc := range m.leia(p.Text) {
			vorm := p.Text[loc[0]:loc[1]]
			out = append(out, Kasutus{Loik: i, Sektsioon: d.sektsiooniNr(i), Vorm: vorm, Kontekst: kontekst(p.Text, vorm)})
		}
	}
	return out
}

// ---------- defineerimata terminid ----------

// Kandidaat on suure tähega sõna või fraas, mis võib olla defineerimata termin.
type Kandidaat struct {
	Fraas     string `json:"fraas"`
	Kordi     int    `json:"kordi"`
	Esimene   int    `json:"esimene_loik"`
	Sektsioon string `json:"sektsioon,omitempty"`
	Kontekst  string `json:"kontekst"`
}

var vaikimisiValistus = map[string]bool{
	"Eesti": true, "Vabariik": true, "Vabariigi": true, "Euroopa": true, "Liit": true, "Liidu": true, "EL": true,
	"AS": true, "OÜ": true, "MTÜ": true, "SA": true, "TÜ": true, "UÜ": true, "FIE": true, "Ltd": true, "Inc": true, "LLC": true, "GmbH": true, "AB": true, "Oy": true,
	"EUR": true, "RT": true, "Riigi": true, "Teataja": true, "Riigikohus": true, "Riigikohtu": true,
	"Lisa": true, "Lisad": true, "Lisas": true, "Lisa-s": true, "Annex": true, "Schedule": true,
	"January": true, "February": true, "March": true, "April": true, "May": true, "June": true, "July": true, "August": true, "September": true, "October": true, "November": true, "December": true,
	"Monday": true, "Tuesday": true, "Wednesday": true, "Thursday": true, "Friday": true, "Saturday": true, "Sunday": true,
	"I": true, "II": true, "III": true, "IV": true, "V": true, "VI": true, "VII": true, "VIII": true, "IX": true, "X": true,
	"Tallinn": true, "Tallinna": true, "Tartu": true, "Harju": true, "Maakohus": true, "Maakohtu": true,
}

// Semikoolon ei lõpeta lauset (loetelu), seega ei loeta järgmist sõna lause alguseks.
var lauseLopp = regexp.MustCompile(`[.!?:]["”“»)]*\s*$`)

// Välistussõnade käändevormid (Lisaga, Maakohtus, Tallinnas).
var valistusSobitajad = func() []sonaSobitaja {
	var out []sonaSobitaja
	for w := range vaikimisiValistus {
		if utf8.RuneCountInString(w) >= 4 {
			out = append(out, uusSobitaja(w))
		}
	}
	return out
}()

func onValistatud(w string) bool {
	if vaikimisiValistus[w] {
		return true
	}
	for _, s := range valistusSobitajad {
		if s.sobib(w) {
			return true
		}
	}
	return false
}

// DefineerimataKandidaadid leiab suure tähega sõnad/fraasid lause keskel, mis
// ei vasta ühelegi defineeritud terminile. Tulemus on heuristiline.
func (d *Dokument) DefineerimataKandidaadid(defs []Definitsioon, piir int) []Kandidaat {
	var sobitajad []*Terminisobitaja
	for _, df := range defs {
		sobitajad = append(sobitajad, UusTerminisobitaja(df.Termin))
	}
	kogum := map[string]*Kandidaat{}
	for i, p := range d.Loigud {
		// Preambul (enne esimest punkti) sisaldab poolte nimesid, aadresse ja
		// esindajaid, mis ei ole terminid.
		if d.SektsioonLoigule(i) == nil {
			continue
		}
		t := p.Text
		ws := sonad(t)
		// defineeritud terminitega kaetud baidivahemikud
		var katted [][2]int
		for _, s := range sobitajad {
			katted = append(katted, s.leia(t)...)
		}
		kaetud := func(w sona) bool {
			for _, k := range katted {
				if k[0] <= w.algus && w.algus+len(w.tekst) <= k[1] {
					return true
				}
			}
			return false
		}
		sobilik := func(w sona) bool {
			return onKandidaatSona(w.tekst) && !onValistatud(w.tekst) && !kaetud(w)
		}
		for j := 0; j < len(ws); j++ {
			w := ws[j]
			if !sobilik(w) {
				continue
			}
			// lause algus?
			eel := strings.TrimSpace(t[:w.algus])
			if eel == "" || lauseLopp.MatchString(eel) || kasitsiNumber.MatchString(eel+" x") || strings.HasSuffix(eel, "(") || strings.HasSuffix(eel, "„") || strings.HasSuffix(eel, "\"") || strings.HasSuffix(eel, "“") {
				continue
			}
			// koonda kuni 3 järjestikust sobilikku suure tähega sõna
			k := j
			for k+1 < len(ws) && k-j < 2 && sobilik(ws[k+1]) && !strings.ContainsAny(t[ws[k].algus+len(ws[k].tekst):ws[k+1].algus], ".,;:()") {
				k++
			}
			a, b := w.algus, ws[k].algus+len(ws[k].tekst)
			fraas := t[a:b]
			j = k
			if onEttevotteNimi(t, b) {
				continue
			}
			if c, ok := kogum[fraas]; ok {
				c.Kordi++
				continue
			}
			kogum[fraas] = &Kandidaat{Fraas: fraas, Kordi: 1, Esimene: i, Sektsioon: d.sektsiooniNr(i), Kontekst: kontekst(t, fraas)}
		}
	}
	var out []Kandidaat
	for _, c := range kogum {
		out = append(out, *c)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Kordi != out[j].Kordi {
			return out[i].Kordi > out[j].Kordi
		}
		return out[i].Esimene < out[j].Esimene
	})
	if piir > 0 && len(out) > piir {
		out = out[:piir]
	}
	return out
}

func onKandidaatSona(w string) bool {
	r, _ := utf8.DecodeRuneInString(w)
	if !unicode.IsUpper(r) {
		return false
	}
	if vaikimisiValistus[w] {
		return false
	}
	// lühendid (kõik suurtähed) ja numbreid sisaldavad sõnad jäetakse välja
	koikSuured := true
	for _, c := range w {
		if unicode.IsDigit(c) {
			return false
		}
		if unicode.IsLower(c) {
			koikSuured = false
		}
	}
	return !koikSuured
}

var ettevotteVorm = regexp.MustCompile(`^\s*(AS|OÜ|MTÜ|SA|TÜ|UÜ|Ltd|Inc|LLC|GmbH|AB|Oy)\b`)

func onEttevotteNimi(t string, lopp int) bool {
	return ettevotteVorm.MatchString(t[lopp:])
}
