package pdf

import (
	"math"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/Ker102/lepinguparser/internal/docx"
)

// leheMoot on lehe mõõdud ja tekstiala piirid.
type leheMoot struct {
	korgus       float64
	vasak, parem float64 // põhiteksti vasak ja parem serv
	reavahe      float64
}

func moodaLeht(read []rida, korgus, keha float64) leheMoot {
	m := leheMoot{korgus: korgus}
	var x0s, x1s []float64
	vahed := map[float64]int{}
	for i, r := range read {
		if math.Abs(r.suurus-keha) > 0.6 {
			continue
		}
		x0s = append(x0s, math.Round(r.x0))
		x1s = append(x1s, r.x1)
		if i > 0 && math.Abs(read[i-1].suurus-keha) <= 0.6 {
			d := math.Round((read[i-1].y-r.y)*2) / 2
			if d > 0 {
				vahed[d]++
			}
		}
	}
	if len(x0s) > 0 {
		m.vasak = mood(x0s)
		sort.Float64s(x1s)
		m.parem = x1s[int(math.Round(float64(len(x1s)-1)*0.9))]
	}
	best := 0
	for d, n := range vahed {
		if n > best || n == best && d < m.reavahe {
			m.reavahe, best = d, n
		}
	}
	if m.reavahe < keha {
		m.reavahe = keha * 1.2
	}
	return m
}

func mood(v []float64) float64 {
	c := map[float64]int{}
	var best float64
	bn := 0
	for _, x := range v {
		c[x]++
		if c[x] > bn || c[x] == bn && x < best {
			best, bn = x, c[x]
		}
	}
	return best
}

// kehaSuurus on põhiteksti fondi suurus (enim märke).
func kehaSuurus(lehed [][]rida) float64 {
	c := map[float64]int{}
	for _, lr := range lehed {
		for _, r := range lr {
			c[math.Round(r.suurus*2)/2] += utf8.RuneCountInString(r.tekst)
		}
	}
	var best float64
	bn := -1
	for s, n := range c {
		if n > bn {
			best, bn = s, n
		}
	}
	if best <= 0 {
		best = 11
	}
	return best
}

// margiJoonealused märgib lehe lõpus väiksemas kirjas olevad read
// (joonealused märkused), et need ei katkestaks lehtede vahel jätkuvat lõiku.
func margiJoonealused(read []rida, korgus, keha float64) {
	for i := len(read) - 1; i >= 0; i-- {
		r := &read[i]
		if r.suurus >= keha*0.88 || r.y > korgus*0.4 {
			break
		}
		r.joonealune = true
	}
}

type loiguKoostaja struct {
	keha    float64
	out     []docx.Paragraph
	cur     *docx.Paragraph
	curRida rida
	curRead int
	esimene rida
	ootel   []docx.Paragraph // joonealused, mis lisatakse lõigu lõppu
	sildid  sildiOlek
}

func (k *loiguKoostaja) sulge() {
	if k.cur != nil {
		k.cur.Text = strings.TrimSpace(k.cur.Text)
		if k.cur.Label == "" && k.curRead <= 3 && k.cur.HeadingLevel == 0 {
			k.cur.HeadingLevel = pealkirjaTase(k.esimene, k.cur.Text, k.keha)
		}
		if k.cur.Text != "" || k.cur.Label != "" {
			k.out = append(k.out, *k.cur)
		}
		k.cur = nil
	}
	k.out = append(k.out, k.ootel...)
	k.ootel = nil
}

func (k *loiguKoostaja) alusta(r rida, s *silt) {
	k.sulge()
	p := docx.Paragraph{Text: r.tekst, NumLevel: -1, Page: r.leht}
	if s != nil {
		p.Label = s.silt
		p.NumLevel = s.tase
		p.LabelFromText = true
		p.Text = strings.TrimSpace(strings.TrimPrefix(r.tekst, s.silt))
	}
	k.cur = &p
	k.curRida = r
	k.esimene = r
	k.curRead = 1
}

// liida lisab rea jooksvale lõigule, ühendades poolitatud sõnad.
func (k *loiguKoostaja) liida(r rida) {
	t := k.cur.Text
	switch {
	case strings.HasSuffix(t, "\u00AD"):
		k.cur.Text = strings.TrimSuffix(t, "\u00AD") + r.tekst
	case poolitus(t, r.tekst):
		k.cur.Text = t[:len(t)-1] + r.tekst
	case strings.HasSuffix(t, "-"):
		// sidekriipsuga liitsõna või lühend reavahetusel (e-, IT-)
		k.cur.Text = t + r.tekst
	default:
		k.cur.Text = t + " " + r.tekst
	}
	k.curRida = r
	k.curRead++
}

// poolitus: rida lõpeb sõnasisese sidekriipsuga ja järgmine rida jätkab
// väikese tähega ("kokku-" + "lepe" -> "kokkulepe"). Ühetähelise või
// suurtähtedega osa korral (e-post, IT-süsteem) sidekriips jääb.
func poolitus(t, jargmine string) bool {
	if !strings.HasSuffix(t, "-") || len(t) < 3 {
		return false
	}
	r, _ := utf8.DecodeRuneInString(jargmine)
	if !unicode.IsLower(r) {
		return false
	}
	sona := t[:len(t)-1]
	if i := strings.LastIndexAny(sona, " (\"„/"); i >= 0 {
		sona = sona[i+1:]
	}
	tahti, suuri := 0, 0
	for _, c := range sona {
		if !unicode.IsLetter(c) {
			return false
		}
		tahti++
		if unicode.IsUpper(c) {
			suuri++
		}
	}
	return tahti >= 2 && suuri*2 < tahti
}

// koostaLoigud ühendab read lõikudeks ja tuvastab punktinumbrid.
func koostaLoigud(lehed [][]rida, korgused []float64) []docx.Paragraph {
	keha := kehaSuurus(lehed)
	k := &loiguKoostaja{keha: keha}
	for li, lr := range lehed {
		mm := moodaLeht(lr, korgused[li], keha)
		margiJoonealused(lr, korgused[li], keha)
		for i, r := range lr {
			if r.joonealune {
				// joonealune märkus: eraldi lõik, lisatakse pärast jooksvat lõiku
				if len(k.ootel) > 0 && i > 0 && lr[i-1].joonealune && !numbriSilt.MatchString(r.tekst) && !onJoonealuseAlgus(r.tekst) {
					k.ootel[len(k.ootel)-1].Text += " " + r.tekst
				} else {
					k.ootel = append(k.ootel, docx.Paragraph{Text: r.tekst, NumLevel: -1, Page: r.leht})
				}
				continue
			}
			if r.lisaPais {
				k.alusta(r, nil)
				k.cur.HeadingLevel = 1
				k.sulge()
				continue
			}
			if k.cur == nil {
				s, ok := k.sildid.tuvastaSilt(r, true, "")
				if ok {
					k.alusta(r, &s)
				} else {
					k.alusta(r, nil)
				}
				continue
			}
			eel := k.curRida
			eelTekst := k.cur.Text
			uusLeht := eel.leht != r.leht
			eelLopp := loppMark.MatchString(eelTekst)
			lyhike := eel.x1 < mm.parem-math.Max(0.15*(mm.parem-mm.vasak), 4*keha)
			vahe := !uusLeht && (eel.y-r.y > 1.4*math.Max(mm.reavahe, r.suurus) || r.y > eel.y)
			suurusMuutus := math.Abs(eel.suurus-r.suurus) > 0.12*math.Max(eel.suurus, r.suurus)
			pealkiri := pealkirjaTase(eel, eelTekst, keha) > 0 && k.curRead == 1
			taane := r.x0 > mm.vasak+1.5*keha && eel.x0 <= mm.vasak+0.5*keha
			tugev := eelLopp && (lyhike || vahe || uusLeht || taane) || vahe || pealkiri || suurusMuutus

			// punkti number võib alata ka lühikese rea järel (nt loetelu "(lisa 1)")
			if s, ok := k.sildid.tuvastaSilt(r, tugev || eelLopp && !uusLeht || lyhike && !loendiJatk.MatchString(eelTekst), eelTekst); ok {
				k.alusta(r, &s)
				continue
			}
			if taplipunkt.MatchString(r.tekst) && (eelLopp || loendiJatk.MatchString(eelTekst) || vahe) {
				k.alusta(r, nil)
				continue
			}
			// paremale joondatud või tsentreeritud lühike rida (päis, pealkiri)
			laius := mm.parem - mm.vasak
			eraldi := func(x rida) bool {
				return x.x0 > mm.vasak+0.2*laius && x.x1-x.x0 < 0.6*laius
			}
			katkesta := vahe || suurusMuutus || pealkiri || (eraldi(eel) || eraldi(r)) && !loendiJatk.MatchString(eelTekst) ||
				eelLopp && (lyhike || taane) ||
				uusLeht && eelLopp && lyhike ||
				// lühike rida ilma kirjavahemärgita (pealkiri) ja uus algab suurtähega
				!uusLeht && eel.x1 < mm.parem-0.35*(mm.parem-mm.vasak) && !loendiJatk.MatchString(eelTekst) && algabSuurega(r.tekst) && k.curRead == 1 ||
				eel.paks != r.paks && k.curRead == 1 && eel.paks
			if katkesta {
				k.alusta(r, nil)
			} else {
				k.liida(r)
			}
		}
	}
	k.sulge()
	out := k.out
	for i := range out {
		out[i].Index = i
	}
	return out
}

func onJoonealuseAlgus(t string) bool {
	r, _ := utf8.DecodeRuneInString(t)
	return unicode.IsDigit(r) || r == '*' || r == '¹' || r == '²' || r == '³'
}

func algabSuurega(t string) bool {
	for _, c := range t {
		if unicode.IsLetter(c) {
			return unicode.IsUpper(c)
		}
		if unicode.IsDigit(c) {
			return false
		}
	}
	return false
}

// pealkirjaTase: 1, kui rida on suurem kui põhitekst või suurtähtedega paks
// lühike rida; muidu 0.
func pealkirjaTase(r rida, tekst string, keha float64) int {
	if utf8.RuneCountInString(tekst) > 150 {
		return 0
	}
	if r.suurus >= keha*1.18 {
		return 1
	}
	tahti, suuri := 0, 0
	for _, c := range tekst {
		if unicode.IsLetter(c) {
			tahti++
			if unicode.IsUpper(c) {
				suuri++
			}
		}
	}
	if tahti >= 4 && suuri*10 >= tahti*9 && (r.paks || r.suurus > keha*1.04) && !loppMark.MatchString(tekst) {
		return 1
	}
	return 0
}
