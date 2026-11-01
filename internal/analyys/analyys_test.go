package analyys_test

import (
	"strings"
	"testing"

	"github.com/Ker102/lepinguparser/internal/analyys"
	"github.com/Ker102/lepinguparser/internal/docx"
	"github.com/Ker102/lepinguparser/internal/testdocx"
)

func naidis(t *testing.T) *analyys.Dokument {
	t.Helper()
	d, err := docx.Read(testdocx.Koosta(testdocx.NaidisLeping(), testdocx.Nummerdus))
	if err != nil {
		t.Fatal(err)
	}
	return analyys.Koosta(d)
}

func TestNummerdusJaSektsioonid(t *testing.T) {
	d := naidis(t)
	var nr []string
	for _, s := range d.Sektsioonid {
		if !strings.HasPrefix(s.Number, "pealkiri:") {
			nr = append(nr, s.Number)
		}
	}
	got := strings.Join(nr, " ")
	want := "1 1.1 1.2 1.3 1.4 2 2.1 2.2 3 3.1 3.2 3.3 3.3(a) 3.3(b) 4 4.1 4.2 Lisa 1"
	if got != want {
		t.Fatalf("numbrid\n got: %s\nwant: %s", got, want)
	}
	s := d.LeiaSektsioon("3.3.")
	if s == nil || len(d.SektsiooniTekst(s)) != 3 {
		t.Fatalf("punkt 3.3 peab sisaldama 3 lõiku, sain %v", s)
	}
	if d.LeiaSektsioon("lisa 1") == nil {
		t.Fatal("Lisa 1 ei leitud")
	}
}

func TestDefinitsioonid(t *testing.T) {
	d := naidis(t)
	defs := d.LeiaDefinitsioonid()
	have := map[string]string{}
	for _, df := range defs {
		have[df.Termin] = df.Viis
	}
	for _, want := range []string{"Leping", "Tellija", "Teostaja", "Pooled", "Pool", "Teenus", "Konfidentsiaalne Teave", "Tasu", "Vääramatu Jõud"} {
		if _, ok := have[want]; !ok {
			t.Errorf("definitsioon puudub: %s (leitud: %v)", want, have)
		}
	}
	if have["Leping"] != "edaspidi" || have["Teenus"] != "loetelu" {
		t.Errorf("vale definitsiooniviis: %v", have)
	}
}

func TestKaandevormid(t *testing.T) {
	d := naidis(t)
	vormid := map[string]bool{}
	for _, k := range d.TerminiKasutus("Konfidentsiaalne Teave") {
		vormid[k.Vorm] = true
	}
	for _, w := range []string{"Konfidentsiaalset Teavet", "Konfidentsiaalse Teabe"} {
		if !vormid[w] {
			t.Errorf("käändevorm leidmata: %q (leitud %v)", w, vormid)
		}
	}
	pool := map[string]bool{}
	for _, k := range d.TerminiKasutus("Pool") {
		pool[k.Vorm] = true
	}
	if !pool["Poolte"] || !pool["Poole"] {
		t.Errorf("Pool käändevormid: %v", pool)
	}
	for _, k := range d.TerminiKasutus("Leping") {
		if k.Vorm == "Lepingu" {
			return
		}
	}
	t.Error("vorm Lepingu leidmata")
}

func TestViited(t *testing.T) {
	d := naidis(t)
	v := d.LeiaViited()
	olek := map[string]string{}
	for _, x := range v {
		olek[x.Siht] = x.Olek
	}
	cases := map[string]string{"3.1": "leitud", "7.4": "puudub", "2.1": "leitud", "3.3(b)": "leitud", "Lisa 1": "leitud", "2": "valine"}
	for siht, want := range cases {
		if olek[siht] != want {
			t.Errorf("viide %s: olek %q, ootasin %q (kõik: %v)", siht, olek[siht], want, olek)
		}
	}
	if kv := d.KusViidatakse("3", v); len(kv) < 2 {
		t.Errorf("punktile 3 peaks viitama vähemalt 2 kohta, sain %d", len(kv))
	}
}

func TestKontroll(t *testing.T) {
	d := naidis(t)
	a := d.Kontroll()
	if len(a.KatkisedViited) != 1 || a.KatkisedViited[0].Siht != "7.4" {
		t.Fatalf("ootasin ühte katkist viidet 7.4, sain %+v", a.KatkisedViited)
	}
	kasutamata := map[string]bool{}
	for _, k := range a.KasutamataTerminid {
		kasutamata[k.Termin] = true
	}
	if !kasutamata["Vääramatu Jõud"] {
		t.Errorf("Vääramatu Jõud peaks olema kasutamata: %v", kasutamata)
	}
	if kasutamata["Tasu"] || kasutamata["Konfidentsiaalne Teave"] {
		t.Errorf("kasutatud termin märgiti kasutamata: %v", kasutamata)
	}
	kand := map[string]bool{}
	for _, k := range a.DefineerimataKandidaadid {
		kand[k.Fraas] = true
	}
	if !kand["Tööhõiveleping"] {
		t.Errorf("Tööhõiveleping peaks olema defineerimata kandidaat: %v", kand)
	}
	for _, bad := range []string{"Harju Maakohtus", "Näidis", "Teenus"} {
		if kand[bad] {
			t.Errorf("valepositiivne kandidaat: %s", bad)
		}
	}
}

func TestJalgitavadMuudatused(t *testing.T) {
	d, err := docx.Read(testdocx.Koosta(testdocx.NaidisLeping(), testdocx.Nummerdus))
	if err != nil {
		t.Fatal(err)
	}
	if !d.TrackedChanges {
		t.Fatal("jälgitavad muudatused tuvastamata")
	}
	for _, p := range d.Paragraphs {
		if strings.Contains(p.Text, "7 päeva") {
			t.Error("kustutatud tekst jäi alles")
		}
		if strings.Contains(p.Text, "Arve maksetähtaeg on 14 päeva") {
			return
		}
	}
	t.Error("lisatud tekst puudub")
}

func TestKasitsiNummerdus(t *testing.T) {
	loigud := []testdocx.Loik{
		{Tekst: "1. Üldsätted"},
		{Tekst: "1.1. Lepingu (edaspidi „Leping“) ese on kirjeldatud punktis 2.1."},
		{Tekst: "2. Ese"},
		{Tekst: "2.1. Leping hõlmab teenust."},
	}
	d, err := docx.Read(testdocx.Koosta(loigud, ""))
	if err != nil {
		t.Fatal(err)
	}
	a := analyys.Koosta(d)
	if a.LeiaSektsioon("2.1") == nil {
		t.Fatal("käsitsi nummerdatud punkt 2.1 leidmata")
	}
	if len(a.Hoiatused) == 0 {
		t.Error("hoiatus automaatnummerduse puudumisest puudub")
	}
	if k := a.Kontroll(); len(k.KatkisedViited) != 0 {
		t.Errorf("katkiseid viiteid ei tohiks olla: %+v", k.KatkisedViited)
	}
}

func TestJutumarkidetaDefinitsioonid(t *testing.T) {
	loigud := []testdocx.Loik{
		{Tekst: "Sotsiaalministeerium, registrikood 70001952 (edaspidi nimetatud ka tellija), ja AS Näide (edaspidi täitja), edaspidi eraldi pool või koos pooled, sõlmisid käesoleva hankelepingu (edaspidi leping) alljärgnevas:"},
		{Tekst: "Lepingu ese", NumID: 1, Tase: 0},
		{Tekst: "Tellija tellib ja täitja osutab teenust. Pooled lepivad kokku.", NumID: 1, Tase: 1},
	}
	d, err := docx.Read(testdocx.Koosta(loigud, testdocx.Nummerdus))
	if err != nil {
		t.Fatal(err)
	}
	a := analyys.Koosta(d)
	have := map[string]bool{}
	for _, df := range a.LeiaDefinitsioonid() {
		have[df.Termin] = true
	}
	for _, w := range []string{"tellija", "täitja", "pool", "pooled", "leping"} {
		if !have[w] {
			t.Errorf("jutumärkideta definitsioon leidmata: %q (leitud %v)", w, have)
		}
	}
	// suure algustähega "Tellija" lause alguses on väiketähega termini kasutus
	n := 0
	for _, k := range a.TerminiKasutus("tellija") {
		if k.Vorm == "Tellija" {
			n++
		}
	}
	if n != 1 {
		t.Errorf("Tellija lause alguses peab sobima terminiga tellija, sain %d", n)
	}
}

func TestLisaValepositiivid(t *testing.T) {
	loigud := []testdocx.Loik{
		{Tekst: "Ese", NumID: 1, Tase: 0},
		{Tekst: "Hinnale lisandub k äibemaks ning lisatud n dokumenti; vt Lisadest 2 ja annex B.", NumID: 1, Tase: 1},
	}
	d, _ := docx.Read(testdocx.Koosta(loigud, testdocx.Nummerdus))
	a := analyys.Koosta(d)
	var sihid []string
	for _, v := range a.LeiaViited() {
		sihid = append(sihid, v.Siht)
	}
	if strings.Join(sihid, ",") != "Lisa 2,Lisa B" {
		t.Errorf("lisa viited: %v", sihid)
	}
}
