package pdf

import (
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/asn1"
	"errors"
	"math/big"
	"strings"
	"testing"
	"time"

	"github.com/Ker102/lepinguparser/internal/analyys"
	"github.com/Ker102/lepinguparser/internal/docx"
	"github.com/Ker102/lepinguparser/internal/testpdf"
)

const (
	vasak  = 72.0
	jatk   = 100.0 // rippuva taandega jätkuread
	laius  = 451.0
	suurus = 11.0
)

func reaLaius(s string) float64 {
	w := 0
	for _, c := range s {
		w += testpdf.Laius(c)
	}
	return float64(w) / 1000 * suurus
}

// murra jagab lõigu ridadeks (sõnade kaupa) nagu tekstitöötlus.
func murra(x0, x1 float64, y *float64, tekst string) []testpdf.Rida {
	var out []testpdf.Rida
	x := x0
	var rida []string
	for _, s := range strings.Fields(tekst) {
		proov := strings.Join(append(rida, s), " ")
		if len(rida) > 0 && x+reaLaius(proov) > vasak+laius {
			out = append(out, testpdf.Rida{X: x, Y: *y, Tekst: strings.Join(rida, " ")})
			*y -= 13
			x, rida = x1, nil
		}
		rida = append(rida, s)
	}
	if len(rida) > 0 {
		out = append(out, testpdf.Rida{X: x, Y: *y, Tekst: strings.Join(rida, " ")})
		*y -= 13
	}
	return out
}

// naidisLeping on kahelehekülgne leping: korduv päis, leheküljenumbrid,
// käsitsi numbrid, poolitatud sõna, rea alguses kuupäev, tähtpunktid,
// lehtede vahel jätkuv lõik, joonealune märkus ja lisa.
func naidisLeping() [][]testpdf.Rida {
	var l1, l2 []testpdf.Rida
	pais := func(lk string) []testpdf.Rida {
		return []testpdf.Rida{
			{X: vasak, Y: 815, Tekst: "Teenusleping nr 7/2026", Suurus: 9},
			{X: 280, Y: 30, Tekst: lk, Suurus: 9},
		}
	}
	l1 = append(l1, pais("Lk 1/2")...)
	l1 = append(l1, testpdf.Rida{X: 190, Y: 770, Tekst: "TEENUSE OSUTAMISE LEPING", Suurus: 14, Paks: true})
	y := 740.0
	l1 = append(l1, murra(vasak, vasak, &y, "Näidis AS (edaspidi „Tellija“) ja Teenus OÜ (edaspidi „Teostaja“), edaspidi koos „Pooled“ ja eraldi „Pool“, sõlmisid alljärgneva lepingu (edaspidi „Leping“).")...)
	y -= 8
	l1 = append(l1, testpdf.Rida{X: vasak, Y: y, Tekst: "1. Üldsätted", Paks: true})
	y -= 18
	// käsitsi murtud read: poolitus, "e-" ja kuupäev rea alguses
	for _, r := range []struct {
		x float64
		t string
	}{
		{vasak, "1.1. Teostaja osutab Tellijale tarkvaraarenduse teenust (edaspidi „Teenus“) kokku-"},
		{jatk, "leppe alusel ning annab Teenuse tulemused Tellijale üle hiljemalt aasta"},
		{jatk, "15. jaanuaril. Teated saadetakse Lepingus märgitud aadressile tavapärase e-"},
		{jatk, "posti teel ja loetakse kättesaaduks kolmandal tööpäeval pärast saatmist."},
	} {
		l1 = append(l1, testpdf.Rida{X: r.x, Y: y, Tekst: r.t})
		y -= 13
	}
	l1 = append(l1, testpdf.Rida{X: vasak, Y: y, Tekst: "1.2."})
	l1 = append(l1, murra(jatk, jatk, &y, "Vääramatu jõu korral ei vastuta Pool Lepingu rikkumise eest, kui ta teavitab teist Poolt viivitamata ja kirjalikku taasesitamist võimaldavas vormis.")...)
	y -= 8
	l1 = append(l1, testpdf.Rida{X: vasak, Y: y, Tekst: "2. Teenuse osutamine", Paks: true})
	y -= 18
	l1 = append(l1, testpdf.Rida{X: vasak, Y: y, Tekst: "2.1."})
	pikk := "Teostaja osutab Teenust kooskõlas Lepingu ja selle lisadega, järgides head tava ja õigusaktides sätestatud nõudeid. Teostaja tagab, et Teenust osutavad isikud on vajaliku kvalifikatsiooniga ning et Teenuse osutamisel kasutatav tarkvara on litsentsitud ja ajakohane. Teostaja hoiab Tellija andmeid konfidentsiaalsena ja kasutab neid üksnes Lepingu täitmiseks"
	read := murra(jatk, jatk, &y, pikk)
	// viimased kaks rida lähevad teisele lehele
	l1 = append(l1, read[:len(read)-2]...)
	l1 = append(l1, testpdf.Rida{X: vasak, Y: 60, Tekst: "1 Joonealune märkus: käesolev leping on näidis.", Suurus: 8})

	l2 = append(l2, pais("Lk 2/2")...)
	y = 770
	for _, r := range read[len(read)-2:] {
		r.Y = y
		l2 = append(l2, r)
		y -= 13
	}
	l2[len(l2)-1].Tekst += " ning ei avalda neid kolmandatele isikutele."
	y -= 8
	l2 = append(l2, testpdf.Rida{X: vasak, Y: y, Tekst: "2.2."})
	l2 = append(l2, testpdf.Rida{X: jatk, Y: y, Tekst: "Teostaja kohustub:"})
	y -= 13
	l2 = append(l2, testpdf.Rida{X: jatk, Y: y, Tekst: "a) tagama Teenuse kvaliteedi;"})
	y -= 13
	l2 = append(l2, testpdf.Rida{X: jatk, Y: y, Tekst: "b) teavitama Tellijat viivitamata, kui Teenuse osutamine on takistatud."})
	y -= 21
	l2 = append(l2, testpdf.Rida{X: vasak, Y: y, Tekst: "3. Tasu", Paks: true})
	y -= 18
	l2 = append(l2, testpdf.Rida{X: vasak, Y: y, Tekst: "3.1."})
	l2 = append(l2, murra(jatk, jatk, &y, "Tasu on 10 000 eurot, mis makstakse vastavalt punktile 2.2 ja punktile 4.1 ning Lisale 1.")...)
	y -= 20
	l2 = append(l2, testpdf.Rida{X: vasak, Y: y, Tekst: "LISA 1 Teenuse kirjeldus", Paks: true})
	y -= 18
	l2 = append(l2, murra(vasak, vasak, &y, "Teenus hõlmab analüüsi, arendust, testimist ja šifreeritud varukoopiate haldamist žürii nõuete kohaselt.")...)
	return [][]testpdf.Rida{l1, l2}
}

func koosta(t *testing.T, v testpdf.Valikud) *analyys.Dokument {
	t.Helper()
	// "1.2." ja "2.1." number on eraldi tekstiobjektina samal real (Wordi
	// eksport), nagu tabulaatoriga eraldatud number
	d, err := Read(testpdf.Koosta(naidisLeping(), v))
	if err != nil {
		t.Fatal(err)
	}
	return analyys.Koosta(d)
}

func leiaLoik(d *analyys.Dokument, nr string) *docx.Paragraph {
	s := d.LeiaSektsioon(nr)
	if s == nil {
		return nil
	}
	return &d.Loigud[s.AlgusLoik]
}

func TestLepinguStruktuur(t *testing.T) {
	for _, v := range []testpdf.Valikud{{}, {Type0: true}, {Pakitud: true}, {Type0: true, Pakitud: true}, {RC4: true}, {RC4: true, Pakitud: true}} {
		d := koosta(t, v)
		var numbrid []string
		for _, s := range d.Sektsioonid {
			numbrid = append(numbrid, s.Number)
		}
		koik := strings.Join(numbrid, " ")
		for _, nr := range []string{"1", "1.1", "1.2", "2", "2.1", "2.2", "2.2(a)", "2.2(b)", "3", "3.1", "Lisa 1"} {
			if d.LeiaSektsioon(nr) == nil {
				t.Errorf("%+v: punkti %s ei leitud; sektsioonid: %s", v, nr, koik)
			}
		}
		if d.LeiaSektsioon("15") != nil {
			t.Errorf("%+v: kuupäev rea alguses tõlgendati punktiks 15", v)
		}
		p := leiaLoik(d, "1.1")
		if p == nil {
			t.Fatalf("%+v: 1.1 puudub", v)
		}
		for _, oodatud := range []string{"kokkuleppe alusel", "hiljemalt aasta 15. jaanuaril", "tavapärase e-posti teel"} {
			if !strings.Contains(p.Text, oodatud) {
				t.Errorf("%+v: 1.1 tekstis puudub %q: %q", v, oodatud, p.Text)
			}
		}
		if p.Page != 1 || p.Label != "1.1." {
			t.Errorf("%+v: 1.1 lk %d silt %q", v, p.Page, p.Label)
		}
		p21 := leiaLoik(d, "2.1")
		if p21 == nil || !strings.HasSuffix(p21.Text, "kasutab neid üksnes Lepingu täitmiseks ning ei avalda neid kolmandatele isikutele.") || !strings.HasPrefix(p21.Text, "Teostaja osutab") {
			t.Errorf("%+v: lehtede vahel jätkuv lõik ei ühendatud: %+v", v, p21)
		}
		if p := leiaLoik(d, "2.2(b)"); p == nil || p.Page != 2 || !strings.HasPrefix(p.Text, "teavitama Tellijat") {
			t.Errorf("%+v: 2.2(b): %+v", v, p)
		}
		for _, l := range d.Loigud {
			if strings.Contains(l.Text, "Teenusleping nr 7") || strings.Contains(l.Text, "Lk 1/2") || strings.Contains(l.Text, "Lk 2/2") {
				t.Errorf("%+v: päis või jalus jäi teksti: %q", v, l.Text)
			}
			if strings.Contains(l.Text, "Joonealune") && strings.Contains(l.Text, "Teostaja") {
				t.Errorf("%+v: joonealune märkus segunes lõiguga: %q", v, l.Text)
			}
		}
		if s := d.LeiaSektsioon("Lisa 1"); s == nil || !strings.Contains(d.Loigud[s.LoppLoik].Text, "šifreeritud varukoopiate haldamist žürii") {
			t.Errorf("%+v: lisa tekst või eesti tähed: %+v", v, s)
		}
		// definitsioonid ja viited töötavad PDF-i peal samamoodi
		defs := map[string]bool{}
		for _, df := range d.LeiaDefinitsioonid() {
			defs[df.Termin] = true
		}
		for _, termin := range []string{"Tellija", "Teostaja", "Pooled", "Pool", "Leping", "Teenus"} {
			if !defs[termin] {
				t.Errorf("%+v: definitsioon %q puudub: %v", v, termin, defs)
			}
		}
		katkised := d.Kontroll().KatkisedViited
		if len(katkised) != 1 || katkised[0].Siht != "4.1" {
			t.Errorf("%+v: ootasin katkist viidet 4.1, sain %+v", v, katkised)
		}
		for _, h := range d.Hoiatused {
			if strings.Contains(h, "Wordi automaatnummerdust") {
				t.Errorf("%+v: PDF-i kohta Wordi hoiatus: %s", v, h)
			}
		}
		if v.RC4 && !strings.Contains(strings.Join(d.Hoiatused, "\n"), "omaniku parooliga") {
			t.Errorf("krüpteeritud faili hoiatus puudub: %v", d.Hoiatused)
		}
	}
}

func TestPoolitus(t *testing.T) {
	cases := []struct {
		t, j string
		ok   bool
	}{
		{"kokku-", "leppe", true},
		{"tavapärase e-", "posti", false},
		{"IT-", "süsteem", false},
		{"Lepingu-", "Pool", false},
		{"lepingu", "kohaselt", false},
	}
	for _, c := range cases {
		if poolitus(c.t, c.j) != c.ok {
			t.Errorf("poolitus(%q, %q) != %v", c.t, c.j, c.ok)
		}
	}
}

func TestSildid(t *testing.T) {
	s := &sildiOlek{nr: []int{7, 2}}
	for _, c := range []struct {
		c  []int
		ok bool
	}{{[]int{7, 3}, true}, {[]int{8}, true}, {[]int{7, 2, 1}, true}, {[]int{8, 1}, true}, {[]int{15}, false}, {[]int{7, 5}, false}} {
		if s.numberSobib(c.c) != c.ok {
			t.Errorf("numberSobib(%v) pärast 7.2 != %v", c.c, c.ok)
		}
	}
	// kuupäev rea alguses ei ole punkt, kui eelmine rida ei lõppenud
	s = &sildiOlek{nr: []int{1, 1}}
	if _, ok := s.tuvastaSilt(rida{tekst: "15. jaanuaril tasub Tellija"}, false, "hiljemalt"); ok {
		t.Error("15. jaanuaril tuvastati punktiks")
	}
	if sl, ok := s.tuvastaSilt(rida{tekst: "1.2. Tellija tasub"}, true, "Lepingu."); !ok || sl.silt != "1.2." || sl.tase != 1 {
		t.Errorf("1.2.: %+v %v", sl, ok)
	}
	if sl, ok := s.tuvastaSilt(rida{tekst: "a) leppetrahv;"}, false, "kinni pidada:"); !ok || sl.tase != 2 {
		t.Errorf("a): %+v %v", sl, ok)
	}
	if sl, ok := s.tuvastaSilt(rida{tekst: "b) kahjuhüvitis."}, false, "leppetrahv;"); !ok || sl.silt != "b)" {
		t.Errorf("b): %+v %v", sl, ok)
	}
	if _, ok := s.tuvastaSilt(rida{tekst: "d) kolmas"}, false, "kahjuhüvitis;"); ok {
		t.Error("d) pärast b) ei tohi olla punkt")
	}
}

func TestDiakriitikud(t *testing.T) {
	g := []mark{
		{tekst: "o", x: 10, laius: 5, suurus: 10},
		{tekst: "˜", x: 10.5, laius: 4, suurus: 10},
		{tekst: "s", x: 15, laius: 5, suurus: 10},
		{tekst: "̌", x: 20, laius: 0, suurus: 10},
	}
	var sb strings.Builder
	for _, m := range koondaDiakriitikud(g) {
		sb.WriteString(m.tekst)
	}
	if sb.String() != "õš" {
		t.Errorf("sain %q", sb.String())
	}
}

func TestHarvendatudJaDuplikaat(t *testing.T) {
	lehed := [][]testpdf.Rida{{
		{X: 200, Y: 700, Tekst: "K O H T U O T S U S", Suurus: 16},
		// võltspaks kiri: sama tekst kaks korda väikese nihkega
		{X: 72, Y: 650, Tekst: "Tallinna Ringkonnakohus"},
		{X: 72.3, Y: 650, Tekst: "Tallinna Ringkonnakohus"},
	}}
	d, err := Read(testpdf.Koosta(lehed, testpdf.Valikud{}))
	if err != nil {
		t.Fatal(err)
	}
	if d.Paragraphs[0].Text != "KOHTUOTSUS" || d.Paragraphs[1].Text != "Tallinna Ringkonnakohus" {
		t.Errorf("sain %q / %q", d.Paragraphs[0].Text, d.Paragraphs[1].Text)
	}
}

func TestVigased(t *testing.T) {
	if _, err := Read([]byte("see ei ole PDF")); !errors.Is(err, ErrNotPDF) {
		t.Errorf("mitte-PDF: %v", err)
	}
	tyhi := testpdf.Koosta([][]testpdf.Rida{{}}, testpdf.Valikud{})
	if _, err := Read(tyhi); !errors.Is(err, ErrTekstita) {
		t.Errorf("tekstita PDF: %v", err)
	}
	// vale startxref: objektid leitakse failist otsides
	b := testpdf.Koosta(naidisLeping(), testpdf.Valikud{})
	i := bytes.LastIndex(b, []byte("startxref\n"))
	vigane := append(append([]byte{}, b[:i]...), []byte("startxref\n999999\n%%EOF\n")...)
	d, err := Read(vigane)
	if err != nil || len(d.Paragraphs) < 10 {
		t.Errorf("vigase xref-iga fail: %v", err)
	}
	// kärbitud fail ei tohi paanikat tekitada
	for _, n := range []int{100, 1000, len(b) / 2} {
		_, _ = Read(b[:n])
		_, _ = Info(b[:n])
	}
}

// pkcs7 koostab minimaalse CMS SignedData struktuuri sertifikaatidega.
func pkcs7(t *testing.T, certs ...[]byte) []byte {
	t.Helper()
	var raw []byte
	for _, c := range certs {
		raw = append(raw, c...)
	}
	type signedData struct {
		Version    int
		Digest     asn1.RawValue
		Encap      struct{ OID asn1.ObjectIdentifier }
		Certs      asn1.RawValue `asn1:"tag:0"`
		SignerInfo asn1.RawValue
	}
	sd := signedData{
		Version:    1,
		Digest:     asn1.RawValue{Class: 0, Tag: 17, IsCompound: true},
		Certs:      asn1.RawValue{Class: 2, Tag: 0, IsCompound: true, Bytes: raw},
		SignerInfo: asn1.RawValue{Class: 0, Tag: 17, IsCompound: true},
	}
	sd.Encap.OID = asn1.ObjectIdentifier{1, 2, 840, 113549, 1, 7, 1}
	sdb, err := asn1.Marshal(sd)
	if err != nil {
		t.Fatal(err)
	}
	ci := struct {
		OID  asn1.ObjectIdentifier
		Sisu asn1.RawValue
	}{asn1.ObjectIdentifier{1, 2, 840, 113549, 1, 7, 2}, asn1.RawValue{Class: 2, Tag: 0, IsCompound: true, Bytes: sdb}}
	b, err := asn1.Marshal(ci)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestAllkiri(t *testing.T) {
	ca, _ := rsa.GenerateKey(rand.Reader, 2048)
	caT := &x509.Certificate{SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "TEST of ESTEID2018", Country: []string{"EE"}},
		NotBefore: time.Now(), NotAfter: time.Now().Add(time.Hour), IsCA: true, BasicConstraintsValid: true, KeyUsage: x509.KeyUsageCertSign}
	caDER, _ := x509.CreateCertificate(rand.Reader, caT, caT, &ca.PublicKey, ca)
	caCert, _ := x509.ParseCertificate(caDER)
	key, _ := rsa.GenerateKey(rand.Reader, 2048)
	leaf := &x509.Certificate{SerialNumber: big.NewInt(2), NotBefore: time.Now(), NotAfter: time.Now().Add(time.Hour),
		Subject: pkix.Name{CommonName: "TAMM,MARI-LIIS,49001010000", Country: []string{"EE"}, ExtraNames: []pkix.AttributeTypeAndValue{
			{Type: asn1.ObjectIdentifier{2, 5, 4, 42}, Value: "MARI-LIIS"}, {Type: asn1.ObjectIdentifier{2, 5, 4, 4}, Value: "TAMM"}}}}
	leafDER, _ := x509.CreateCertificate(rand.Reader, leaf, caCert, &key.PublicKey, ca)

	b := testpdf.Koosta(naidisLeping(), testpdf.Valikud{Allkiri: pkcs7(t, caDER, leafDER)})
	info, err := Info(b)
	if err != nil {
		t.Fatal(err)
	}
	if len(info.Allkirjad) != 1 {
		t.Fatalf("allkirjad: %+v", info.Allkirjad)
	}
	a := info.Allkirjad[0]
	if a.Allkirjastaja != "MARI-LIIS TAMM" || a.Valjaandja != "TEST of ESTEID2018" || a.Tyyp != "PAdES" || !a.KatabKogu ||
		a.Pohjus != "Lepingu sõlmimine" || a.Aeg != "2026-01-15T10:30:00+02:00" || a.Vali != "Allkiri1" {
		t.Errorf("allkiri: %+v", a)
	}
	if strings.Contains(a.Allkirjastaja, "4900") {
		t.Error("isikukood ei tohi väljundisse jõuda")
	}
	d, err := Read(b)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(strings.Join(d.Warnings, "\n"), "1 digiallkirja") {
		t.Errorf("hoiatused: %v", d.Warnings)
	}
	// muudatus pärast allkirjastamist (inkrementaalne uuendus)
	muudetud := append(append([]byte{}, b...), []byte("99 0 obj\n(lisa)\nendobj\n")...)
	info, _ = Info(muudetud)
	if info.Allkirjad[0].KatabKogu {
		t.Error("pärast allkirjastamist lisatud muudatust ei tuvastatud")
	}
	d, _ = Read(muudetud)
	if !strings.Contains(strings.Join(d.Warnings, "\n"), "pärast viimast allkirja muudetud") {
		t.Errorf("hoiatused: %v", d.Warnings)
	}
}

func TestPdfKuupaev(t *testing.T) {
	for in, out := range map[string]string{
		"D:20240115123045+02'00'": "2024-01-15T12:30:45+02:00",
		"D:20240115123045Z":       "2024-01-15T12:30:45Z",
		"D:20240115":              "2024-01-15",
		"tundmatu":                "tundmatu",
	} {
		if g := pdfKuupaev(in); g != out {
			t.Errorf("%s: %s", in, g)
		}
	}
}
