// Package testdocx koostab testides kasutatavaid .docx faile.
package testdocx

import (
	"archive/zip"
	"bytes"
	"fmt"
	"html"
	"strings"
)

// Loik on testdokumendi lõik.
type Loik struct {
	Tekst      string
	Stiil      string // nt "Heading1"
	NumID      int    // 0 = nummerdamata
	Tase       int    // ilvl
	Lisatud    string // jälgitav lisandus lõigu lõppu
	Kustutatud string // jälgitav kustutus lõigu lõppu
}

const ns = `xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main"`

// Nummerdus on lihtne mitmetasemeline nummerdus: 1. / 1.1. / (a)
const Nummerdus = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<w:numbering ` + ns + `>
 <w:abstractNum w:abstractNumId="0">
  <w:lvl w:ilvl="0"><w:start w:val="1"/><w:numFmt w:val="decimal"/><w:lvlText w:val="%1."/></w:lvl>
  <w:lvl w:ilvl="1"><w:start w:val="1"/><w:numFmt w:val="decimal"/><w:lvlText w:val="%1.%2."/></w:lvl>
  <w:lvl w:ilvl="2"><w:start w:val="1"/><w:numFmt w:val="lowerLetter"/><w:lvlText w:val="(%3)"/></w:lvl>
 </w:abstractNum>
 <w:abstractNum w:abstractNumId="1">
  <w:lvl w:ilvl="0"><w:start w:val="1"/><w:numFmt w:val="bullet"/><w:lvlText w:val="•"/></w:lvl>
 </w:abstractNum>
 <w:num w:numId="1"><w:abstractNumId w:val="0"/></w:num>
 <w:num w:numId="2"><w:abstractNumId w:val="1"/></w:num>
 <w:num w:numId="3"><w:abstractNumId w:val="0"/><w:lvlOverride w:ilvl="0"><w:startOverride w:val="1"/></w:lvlOverride></w:num>
</w:numbering>`

const stiilid = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<w:styles ` + ns + `>
 <w:style w:type="paragraph" w:styleId="Normal"><w:name w:val="Normal"/></w:style>
 <w:style w:type="paragraph" w:styleId="Heading1"><w:name w:val="heading 1"/><w:basedOn w:val="Normal"/></w:style>
 <w:style w:type="paragraph" w:styleId="Punkt2"><w:name w:val="Punkt 2"/><w:basedOn w:val="Normal"/><w:pPr><w:numPr><w:ilvl w:val="1"/><w:numId w:val="1"/></w:numPr></w:pPr></w:style>
</w:styles>`

// Koosta loob .docx baidid. Kui nummerdus on tühi, numbering.xml jäetakse välja.
func Koosta(loigud []Loik, nummerdus string) []byte {
	var body strings.Builder
	for _, l := range loigud {
		body.WriteString("<w:p><w:pPr>")
		if l.Stiil != "" {
			fmt.Fprintf(&body, `<w:pStyle w:val="%s"/>`, l.Stiil)
		}
		if l.NumID > 0 {
			fmt.Fprintf(&body, `<w:numPr><w:ilvl w:val="%d"/><w:numId w:val="%d"/></w:numPr>`, l.Tase, l.NumID)
		}
		body.WriteString("</w:pPr>")
		fmt.Fprintf(&body, `<w:r><w:t xml:space="preserve">%s</w:t></w:r>`, html.EscapeString(l.Tekst))
		if l.Lisatud != "" {
			fmt.Fprintf(&body, `<w:ins w:id="1" w:author="T"><w:r><w:t xml:space="preserve">%s</w:t></w:r></w:ins>`, html.EscapeString(l.Lisatud))
		}
		if l.Kustutatud != "" {
			fmt.Fprintf(&body, `<w:del w:id="2" w:author="T"><w:r><w:delText xml:space="preserve">%s</w:delText></w:r></w:del>`, html.EscapeString(l.Kustutatud))
		}
		body.WriteString("</w:p>\n")
	}
	doc := `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<w:document ` + ns + `><w:body>` + body.String() + `</w:body></w:document>`

	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	add := func(name, content string) {
		f, _ := zw.Create(name)
		f.Write([]byte(content))
	}
	add("[Content_Types].xml", `<?xml version="1.0" encoding="UTF-8"?><Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types"><Default Extension="xml" ContentType="application/xml"/></Types>`)
	add("word/document.xml", doc)
	add("word/styles.xml", stiilid)
	if nummerdus != "" {
		add("word/numbering.xml", nummerdus)
	}
	zw.Close()
	return buf.Bytes()
}

// NaidisLeping on eestikeelne näidisleping testideks.
func NaidisLeping() []Loik {
	return []Loik{
		{Tekst: "TEENUSE OSUTAMISE LEPING", Stiil: "Heading1"},
		{Tekst: "Käesoleva lepingu (edaspidi „Leping“) sõlmisid Näidis AS (edaspidi „Tellija“) ja Teenus OÜ (edaspidi „Teostaja“), edaspidi koos „Pooled“ ja eraldi „Pool“."},
		{Tekst: "Mõisted", NumID: 1, Tase: 0},
		{Tekst: "„Teenus“ tähendab Lepingu Lisas 1 kirjeldatud tarkvaraarendust.", NumID: 1, Tase: 1},
		{Tekst: "„Konfidentsiaalne Teave“ tähendab igasugust Poolte vahel vahetatud mitteavalikku teavet.", NumID: 1, Tase: 1},
		{Tekst: "„Tasu“ – Lepingu punktis 3.1 sätestatud summa.", NumID: 1, Tase: 1},
		{Tekst: "„Vääramatu Jõud“ tähendab VÕS § 103 lg 2 nimetatud asjaolu.", NumID: 1, Tase: 1},
		{Tekst: "Teenuse osutamine", NumID: 1, Tase: 0},
		{Tekst: "Teostaja osutab Teenust kooskõlas Lepingu punktiga 7.4 ja Lisaga 1.", NumID: 1, Tase: 1},
		{Tekst: "Teostaja hoiab Konfidentsiaalset Teavet saladuses ja ei avalda Konfidentsiaalse Teabe sisu kolmandatele isikutele.", NumID: 1, Tase: 1},
		{Tekst: "Tasu ja maksmine", NumID: 1, Tase: 0},
		{Tekst: "Tellija maksab Teostajale Tasu 10 000 eurot.", Stiil: "Punkt2"},
		{Tekst: "Tasu makstakse arve alusel, välja arvatud punktis 2.1 sätestatud juhul.", Stiil: "Punkt2", Lisatud: " Arve maksetähtaeg on 14 päeva.", Kustutatud: " Arve maksetähtaeg on 7 päeva."},
		{Tekst: "Tasust võib kinni pidada:", Stiil: "Punkt2"},
		{Tekst: "leppetrahvi summa;", NumID: 1, Tase: 2},
		{Tekst: "kahju hüvitise summa.", NumID: 1, Tase: 2},
		{Tekst: "Lõppsätted", NumID: 1, Tase: 0},
		{Tekst: "Poole vastutus on piiratud punkti 3.3(b) alusel; Tööhõiveleping ei kuulu kohaldamisele.", NumID: 1, Tase: 1},
		{Tekst: "Leping jõustub allkirjastamisel. Pooled lahendavad vaidlused Harju Maakohtus.", NumID: 1, Tase: 1},
		{Tekst: "LISA 1 Teenuse kirjeldus", Stiil: "Heading1"},
		{Tekst: "Teenus hõlmab analüüsi, arendust ja testimist."},
	}
}
