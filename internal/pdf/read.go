package pdf

import (
	"math"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

// rida on lehe tekstirida.
type rida struct {
	tekst     string
	x0, x1, y float64
	suurus    float64
	paks      bool
	leht      int // 1-põhine
	// esimeseVahe on vahe esimese sõna järel (em-ides); nummerdatud punkti
	// number ja tekst on tavaliselt eraldatud tabulaatoriga.
	esimeseVahe float64
	joonealune  bool
	// artefakt: rida on märgendatud PDF-is /Artifact (päis, jalus)
	artefakt bool
	// lisaPais: lisa algust märkiv päis ("Lisa 1"), mis jäeti alles
	lisaPais bool
}

// reaKs koostab märkidest read.
func reaKs(margid []mark, leht int) []rida {
	if len(margid) == 0 {
		return nil
	}
	sort.SliceStable(margid, func(i, j int) bool { return margid[i].y > margid[j].y })
	var grupid [][]mark
	var gy, gs float64
	for _, m := range margid {
		if len(grupid) > 0 && gy-m.y <= 0.45*math.Max(gs, m.suurus) {
			grupid[len(grupid)-1] = append(grupid[len(grupid)-1], m)
			if m.suurus > gs {
				gs = m.suurus
			}
			continue
		}
		grupid = append(grupid, []mark{m})
		gy, gs = m.y, m.suurus
	}
	var out []rida
	sisu := false
	for _, g := range grupid {
		if r, ok := koostaRida(g, leht); ok {
			out = append(out, r)
			sisu = sisu || !r.artefakt
		}
	}
	if !sisu {
		// lehel on ainult artefaktiks märgitud tekst: kasuta seda
		for i := range out {
			out[i].artefakt = false
		}
	}
	return out
}

func koostaRida(g []mark, leht int) (rida, bool) {
	g = jarjestaTykid(g)
	// rea põhisuurus ja algjoon: kõige rohkem märke kandev suurus
	loend := map[float64]int{}
	for _, m := range g {
		loend[math.Round(m.suurus*2)/2]++
	}
	var suurus float64
	best := -1
	for s, n := range loend {
		if n > best || n == best && s > suurus {
			suurus, best = s, n
		}
	}
	baas := g[0].y
	for _, m := range g {
		if math.Round(m.suurus*2)/2 == suurus {
			baas = m.y
			break
		}
	}
	g = koondaDiakriitikud(g)
	var sb strings.Builder
	r := rida{leht: leht, suurus: suurus, y: baas, x0: math.NaN(), esimeseVahe: -1}
	paksud, koik := 0, 0
	var eelLopp float64
	var eel *mark
	esimeneSonaLopp := math.NaN()
	tyhikLisatud := false
	for i := range g {
		m := &g[i]
		// ülaindeksis väike number (joonealuse märkuse viide) jäetakse välja
		if m.suurus < 0.8*suurus && m.y > baas+0.2*suurus && onNumbrid(m.tekst) {
			continue
		}
		if eel != nil && m.tekst == eel.tekst && math.Abs(m.x-eel.x) < 0.15*m.suurus && math.Abs(m.y-eel.y) < 0.1*m.suurus {
			continue // topelt joonistatud märk (võltspaks kiri)
		}
		if strings.TrimSpace(m.tekst) == "" {
			if eel != nil && !tyhikLisatud {
				sb.WriteByte(' ')
				tyhikLisatud = true
			}
			continue
		}
		if eel != nil && !tyhikLisatud {
			vahe := m.x - eelLopp
			if vahe > 0.18*math.Min(m.suurus, eel.suurus) {
				sb.WriteByte(' ')
				tyhikLisatud = true
			}
		}
		if tyhikLisatud && r.esimeseVahe < 0 && !math.IsNaN(esimeneSonaLopp) {
			r.esimeseVahe = (m.x - esimeneSonaLopp) / suurus
		}
		if math.IsNaN(r.x0) {
			r.x0 = m.x
		}
		sb.WriteString(m.tekst)
		tyhikLisatud = false
		n := utf8.RuneCountInString(m.tekst)
		koik += n
		if m.paks {
			paksud += n
		}
		eel = m
		eelLopp = m.x + m.laius
		if r.esimeseVahe < 0 && !strings.Contains(sb.String(), " ") {
			esimeneSonaLopp = eelLopp
		}
		r.x1 = eelLopp
	}
	r.tekst = normaliseeri(sb.String())
	if harvendatud.MatchString(r.tekst) {
		r.tekst = strings.ReplaceAll(r.tekst, " ", "")
	}
	if r.tekst == "" {
		return r, false
	}
	r.paks = koik > 0 && paksud*10 >= koik*8
	artefakte := 0
	for _, m := range g {
		if m.artefakt {
			artefakte++
		}
	}
	r.artefakt = artefakte*2 > len(g)
	return r, true
}

// jarjestaTykid järjestab rea märgid: iga sisuvoo string (tükk) jääb
// joonistamise järjekorda ja tükid järjestatakse alguspunkti järgi. Nii ei
// sega ebatäpsed laiused tähtede järjekorda. Topelt joonistatud tükid
// (varjuga või võltspaks kiri) jäetakse välja.
func jarjestaTykid(g []mark) []mark {
	type tykk struct {
		m     []mark
		x     float64
		tekst string
	}
	var tykid []*tykk
	indeks := map[int]*tykk{}
	for _, m := range g {
		t, ok := indeks[m.tykk]
		// string võib sisaldada suuri hüppeid tagasi (nt ' või T*)
		if ok && m.x < t.m[len(t.m)-1].x-0.5*m.suurus {
			ok = false
		}
		if !ok {
			t = &tykk{x: m.x}
			tykid = append(tykid, t)
			indeks[m.tykk] = t
		}
		t.m = append(t.m, m)
		t.tekst += m.tekst
	}
	sort.SliceStable(tykid, func(i, j int) bool { return tykid[i].x < tykid[j].x })
	var out []mark
	for i, t := range tykid {
		topelt := false
		for _, e := range tykid[:i] {
			if e.tekst == t.tekst && strings.TrimSpace(t.tekst) != "" && math.Abs(e.x-t.x) < 0.1*t.m[0].suurus && math.Abs(e.m[0].y-t.m[0].y) < 0.3*t.m[0].suurus {
				topelt = true
				break
			}
		}
		if !topelt {
			out = append(out, t.m...)
		}
	}
	return out
}

func onNumbrid(s string) bool {
	if s == "" {
		return false
	}
	for _, c := range s {
		if !unicode.IsDigit(c) && c != '*' && c != ',' {
			return false
		}
	}
	return true
}

// Eraldi märkidena joonistatud diakriitikud (nt "o" + "˜") ühendatakse
// täheks. Eesti tähtede jaoks on see vajalik mõne TeX-i ja vanema
// tootja PDF-is.
var (
	diakriitik = map[string]rune{"˜": '~', "~": '~', "¨": '"', "ˇ": 'v', "´": '\'', "`": '`', "ˆ": '^', "\u0303": '~', "\u0308": '"', "\u030C": 'v', "\u0301": '\'', "\u0300": '`', "\u0302": '^'}
	koondatud  = map[string]string{
		"o~": "õ", "O~": "Õ", "a\"": "ä", "A\"": "Ä", "o\"": "ö", "O\"": "Ö", "u\"": "ü", "U\"": "Ü",
		"sv": "š", "Sv": "Š", "zv": "ž", "Zv": "Ž", "cv": "č", "Cv": "Č", "e'": "é", "E'": "É", "a'": "á",
		"o'": "ó", "i'": "í", "u'": "ú", "e`": "è", "a`": "à", "e^": "ê", "a~": "ã", "n~": "ñ", "e\"": "ë", "i\"": "ï",
		"ı\"": "ï", "ı'": "í",
	}
)

func koondaDiakriitikud(g []mark) []mark {
	jata := map[int]bool{}
	for i := range g {
		d, ok := diakriitik[g[i].tekst]
		if !ok {
			continue
		}
		kesk := g[i].x + g[i].laius/2
		r0, _ := utf8.DecodeRuneInString(g[i].tekst)
		kombineeriv := r0 >= 0x300 && r0 <= 0x36f
		for _, j := range []int{i - 1, i + 1} {
			if j < 0 || j >= len(g) || jata[j] {
				continue
			}
			t := g[j]
			// kombineeriv märk järgneb tähele; eraldi aktsent asub tähe kohal
			kattub := kesk >= t.x-0.05*t.suurus && kesk <= t.x+t.laius+0.05*t.suurus
			if !(kattub || kombineeriv && j == i-1) {
				continue
			}
			if c, ok := koondatud[t.tekst+string(d)]; ok {
				g[j].tekst = c
				jata[i] = true
				break
			}
		}
	}
	if len(jata) == 0 {
		return g
	}
	out := g[:0:0]
	for i, m := range g {
		if !jata[i] {
			out = append(out, m)
		}
	}
	return out
}

var ligatuurid = strings.NewReplacer("\uFB00", "ff", "\uFB01", "fi", "\uFB02", "fl", "\uFB03", "ffi", "\uFB04", "ffl", "\uFB05", "st", "\uFB06", "st",
	"\u00A0", " ", "\u2009", " ", "\u202F", " ", "\u2002", " ", "\u2003", " ", "\t", " ", "\u200B", "", "\uFEFF", "")

func normaliseeri(s string) string {
	s = ligatuurid.Replace(s)
	return strings.Join(strings.Fields(s), " ")
}

// ---------- päised ja jalused ----------

var lisaPais = regexp.MustCompile(`(?i)^\s*(lisa|annex|appendix)\s+(nr\.?\s*)?(\d{1,2}|[A-Z])\b`)

// harvendatud: suurtähed ühe tühikuga eraldatud ("K O H T U O T S U S")
var harvendatud = regexp.MustCompile(`^(\p{Lu} ){3,}\p{Lu}$`)

var leheNumber = regexp.MustCompile(`(?i)^(?:(?:lk|lehekülg|lehekylg|leht|page|pg|p)\.?\s*)?[-–(]?\s*\d{1,4}\s*[)–-]?\s*(?:(?:/|of|kokku|\(|-|–|\s)\s*\d{1,4}\s*\)?)?\s*$`)

// eemaldaPaisedJalused eemaldab lehe üla- ja alaservas korduvad read ja
// leheküljenumbrid.
func eemaldaPaisedJalused(lehed [][]rida, korgused []float64) ([][]rida, int) {
	tsoon := func(r rida, h float64) bool { return r.y > h*0.9 || r.y < h*0.1 }
	votme := func(t string) string {
		var sb strings.Builder
		for _, c := range strings.ToLower(t) {
			if unicode.IsDigit(c) {
				c = '#'
			}
			if !unicode.IsSpace(c) {
				sb.WriteRune(c)
			}
		}
		return sb.String()
	}
	loend := map[string]int{}
	for i, lr := range lehed {
		nahtud := map[string]bool{}
		for _, r := range lr {
			if tsoon(r, korgused[i]) {
				k := votme(r.tekst)
				if !nahtud[k] {
					nahtud[k] = true
					loend[k]++
				}
			}
		}
	}
	piir := int(math.Max(2, math.Ceil(float64(len(lehed))*0.4)))
	eemaldatud := 0
	out := make([][]rida, len(lehed))
	lisad := map[string]bool{}
	for i, lr := range lehed {
		for _, r := range lr {
			if r.artefakt || tsoon(r, korgused[i]) && (loend[votme(r.tekst)] >= piir || leheNumber.MatchString(r.tekst)) {
				// lisa päis ("Lisa 1") jäetakse esimesel lehel alles: see
				// märgib lisa algust
				if lisaPais.MatchString(r.tekst) && !lisad[r.tekst] {
					lisad[r.tekst] = true
					r.lisaPais = true
					out[i] = append(out[i], r)
					continue
				}
				eemaldatud++
				continue
			}
			out[i] = append(out[i], r)
		}
	}
	return out, eemaldatud
}

// ---------- lõigud ----------

var (
	numbriSilt   = regexp.MustCompile(`^(\d{1,3}(?:\.\d{1,3})*)(\.?)\s+(\S)`)
	taheSilt     = regexp.MustCompile(`^(\(([a-zõäöü]|[ivx]{1,5})\)|([a-zõäöü]|[ivx]{1,5})[).])\s+\S`)
	taplipunkt   = regexp.MustCompile(`^[•◦▪▫■□●○·‣⁃∙\x{F0B7}\x{F0A7}\x{F076}\x{F0D8}\x{F0FC}*–—-]\s+\S`)
	loppMark     = regexp.MustCompile(`[.:;!?][\s"”“»)\]']*$`)
	loendiJatk   = regexp.MustCompile(`(?i)(;|,|\b(ja|või|ning|and|or))\s*$`)
	romaanilised = []string{"i", "ii", "iii", "iv", "v", "vi", "vii", "viii", "ix", "x", "xi", "xii", "xiii", "xiv", "xv"}
)

type silt struct {
	silt   string
	tase   int
	loppNr []int
}

// sildiOlek jälgib punktinumbrite järjestust, et eristada punkti numbrit
// rea alguses olevast arvust (nt "15. jaanuaril").
type sildiOlek struct {
	nr       []int
	taht     rune // viimane tähtpunkt (0 = tähtloendit ei ole)
	tahtTase int
	rooma    int // viimane rooma number (0 = puudub)
}

func (s *sildiOlek) numberSobib(c []int) bool {
	p := s.nr
	if len(p) == 0 {
		return true
	}
	vordne := func(a, b []int) bool {
		if len(a) != len(b) {
			return false
		}
		for i := range a {
			if a[i] != b[i] {
				return false
			}
		}
		return true
	}
	for k := 0; k < len(p); k++ {
		if len(c) == k+1 && vordne(c[:k], p[:k]) && c[k] == p[k]+1 {
			return true // järgmine samal tasemel
		}
		if len(c) == k+2 && vordne(c[:k], p[:k]) && c[k] == p[k]+1 && c[k+1] == 1 {
			return true // järgmise punkti esimene alapunkt (pealkiri numbrita)
		}
	}
	if len(c) > len(p) && len(c) <= len(p)+2 && vordne(c[:len(p)], p) {
		for _, v := range c[len(p):] {
			if v != 1 {
				return false
			}
		}
		return true // esimene alapunkt
	}
	return false
}

// tuvastaSilt otsustab, kas rida algab punkti numbriga. tugevVahe näitab, et
// eelmine rida lõppes lõigu lõpuna (kirjavahemärk, tühi rida, uus leht).
func (s *sildiOlek) tuvastaSilt(r rida, tugevVahe bool, eelTekst string) (silt, bool) {
	t := r.tekst
	if m := numbriSilt.FindStringSubmatch(t); m != nil && tugevVahe {
		var c []int
		for _, osa := range strings.Split(m[1], ".") {
			n, _ := strconv.Atoi(osa)
			c = append(c, n)
		}
		punktiga := m[2] == "."
		suur := unicode.IsUpper([]rune(m[3])[0]) || strings.ContainsAny(m[3], "„\"“«(")
		sobib := s.numberSobib(c)
		ok := false
		switch {
		case len(c) >= 2 && (sobib || suur):
			ok = true
		case len(c) == 1 && punktiga && (sobib || suur && r.esimeseVahe >= 0.8):
			ok = true
		case len(c) == 1 && !punktiga && r.esimeseVahe >= 0.8 && suur && sobib:
			ok = true
		}
		if ok {
			s.nr, s.taht, s.rooma = c, 0, 0
			return silt{silt: m[1] + m[2], tase: len(c) - 1}, true
		}
	}
	if m := taheSilt.FindStringSubmatch(t); m != nil {
		sisu := m[2]
		if sisu == "" {
			sisu = m[3]
		}
		sildiTekst := m[1]
		eelLopp := loppMark.MatchString(eelTekst) || loendiJatk.MatchString(eelTekst) || tugevVahe
		if !eelLopp {
			return silt{}, false
		}
		r1, _ := utf8.DecodeRuneInString(sisu)
		onTaht := utf8.RuneCountInString(sisu) == 1
		// tähtloendi jätk: järgmine täht
		if onTaht && s.taht != 0 && r1 == s.taht+1 {
			s.taht, s.rooma = r1, 0
			return silt{silt: sildiTekst, tase: s.tahtTase}, true
		}
		// rooma numbrid tähtpunkti all või iseseisvalt
		if ri := roomaIndeks(sisu); ri > 0 && !(onTaht && s.taht == 'h' && sisu == "i") {
			if ri == 1 && strings.HasSuffix(strings.TrimSpace(eelTekst), ":") || s.rooma > 0 && ri == s.rooma+1 {
				s.rooma = ri
				tase := len(s.nr)
				if s.taht != 0 {
					tase = s.tahtTase + 1
				}
				return silt{silt: sildiTekst, tase: tase}, true
			}
		}
		if onTaht && r1 == 'a' {
			s.taht, s.tahtTase, s.rooma = 'a', len(s.nr), 0
			return silt{silt: sildiTekst, tase: s.tahtTase}, true
		}
	}
	return silt{}, false
}

func roomaIndeks(s string) int {
	for i, r := range romaanilised {
		if s == r {
			return i + 1
		}
	}
	return 0
}
