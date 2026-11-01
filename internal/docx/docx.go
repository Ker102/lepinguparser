// Package docx loeb Wordi .docx faili lõigud koos stiilide ja nummerdusega.
//
// Pakett ei püüa Wordi vormingut täielikult taastada. Eesmärk on saada iga
// lõigu tekst, stiil, pealkirjatase ja Wordi automaatnummerdusest arvutatud
// number (nt "7.2."), sest just need määravad lepingu struktuuri.
package docx

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"regexp"
	"strconv"
	"strings"
)

// Paragraph on üks dokumendi lõik.
type Paragraph struct {
	Index        int    `json:"indeks"`
	Text         string `json:"tekst"`
	StyleID      string `json:"stiil,omitempty"`
	HeadingLevel int    `json:"pealkirjatase,omitempty"`
	// Label on Wordi automaatnummerdusest arvutatud number, nt "7.2." või "(a)".
	Label string `json:"number,omitempty"`
	// NumLevel on nummerduse tase (0-põhine), -1 kui lõik pole nummerdatud.
	NumLevel int  `json:"-"`
	InTable  bool `json:"tabelis,omitempty"`
	// Page on PDF-i lehekülg (1-põhine); .docx puhul 0.
	Page int `json:"lk,omitempty"`
	// LabelFromText on true, kui number on loetud lõigu tekstist (PDF), mitte
	// Wordi automaatnummerdusest.
	LabelFromText bool `json:"-"`
}

// Document on loetud dokument.
type Document struct {
	Paragraphs []Paragraph
	// Warnings sisaldab hoiatusi, mis võivad analüüsi täpsust mõjutada.
	Warnings []string
	// TrackedChanges on true, kui dokumendis on jälgitavaid muudatusi.
	TrackedChanges bool
	// Format on "docx" või "pdf".
	Format string
}

// ErrNotDocx tagastatakse, kui fail ei ole loetav .docx fail.
var ErrNotDocx = errors.New("fail ei ole korrektne .docx dokument")

// ReadFile loeb .docx faili baidid.
func Read(data []byte) (*Document, error) {
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrNotDocx, err)
	}
	files := map[string]*zip.File{}
	for _, f := range zr.File {
		files[f.Name] = f
	}
	docXML, err := readZip(files, "word/document.xml")
	if err != nil {
		return nil, fmt.Errorf("%w: word/document.xml puudub (%v)", ErrNotDocx, err)
	}
	doc := &Document{Format: "docx"}

	styles := map[string]*style{}
	if b, err := readZip(files, "word/styles.xml"); err == nil {
		if styles, err = parseStyles(b); err != nil {
			doc.Warnings = append(doc.Warnings, "styles.xml lugemine ebaõnnestus: "+err.Error())
		}
	}
	var num *numbering
	if b, err := readZip(files, "word/numbering.xml"); err == nil {
		if num, err = parseNumbering(b); err != nil {
			doc.Warnings = append(doc.Warnings, "numbering.xml lugemine ebaõnnestus, automaatnumbrid puuduvad: "+err.Error())
		}
	}
	raw, tracked, err := parseDocument(docXML)
	if err != nil {
		return nil, fmt.Errorf("%w: document.xml: %v", ErrNotDocx, err)
	}
	doc.TrackedChanges = tracked
	if tracked {
		doc.Warnings = append(doc.Warnings, "dokumendis on jälgitavaid muudatusi: analüüs kasutab teksti koos lisandustega ja ilma kustutusteta")
	}

	eng := newEngine(num)
	for i, rp := range raw {
		p := Paragraph{Index: i, Text: rp.text, StyleID: rp.styleID, InTable: rp.inTable, NumLevel: -1}
		st := resolveStyle(styles, rp.styleID)
		p.HeadingLevel = st.headingLevel
		if rp.outlineLvl >= 0 && rp.outlineLvl < 9 {
			p.HeadingLevel = rp.outlineLvl + 1
		}
		numID, ilvl := st.numID, st.ilvl
		if rp.hasNumPr {
			if rp.numID >= 0 {
				numID = rp.numID
			}
			if rp.ilvl >= 0 {
				ilvl = rp.ilvl
			}
		}
		if numID > 0 && strings.TrimSpace(rp.text) != "" {
			if label, ok := eng.next(numID, ilvl); ok {
				p.Label = label
				p.NumLevel = ilvl
			} else if num == nil {
				// nummerdus viidatud, kuid numbering.xml puudub
				p.NumLevel = ilvl
			}
		}
		doc.Paragraphs = append(doc.Paragraphs, p)
	}
	if eng.missing > 0 {
		doc.Warnings = append(doc.Warnings, fmt.Sprintf("%d lõigu nummerdust ei õnnestunud arvutada (viidatud nummerdusdefinitsioon puudub)", eng.missing))
	}
	return doc, nil
}

func readZip(files map[string]*zip.File, name string) ([]byte, error) {
	f, ok := files[name]
	if !ok {
		return nil, errors.New("puudub")
	}
	rc, err := f.Open()
	if err != nil {
		return nil, err
	}
	defer rc.Close()
	return io.ReadAll(io.LimitReader(rc, 200<<20))
}

func attr(se xml.StartElement, local string) (string, bool) {
	for _, a := range se.Attr {
		if a.Name.Local == local {
			return a.Value, true
		}
	}
	return "", false
}

func atoiDefault(s string, d int) int {
	n, err := strconv.Atoi(strings.TrimSpace(s))
	if err != nil {
		return d
	}
	return n
}

// ---------- document.xml ----------

type rawPara struct {
	text       string
	styleID    string
	hasNumPr   bool
	numID      int
	ilvl       int
	outlineLvl int
	inTable    bool
}

func parseDocument(b []byte) ([]rawPara, bool, error) {
	dec := xml.NewDecoder(bytes.NewReader(b))
	var out []rawPara
	var cur *rawPara
	var sb strings.Builder
	tableDepth := 0
	inPPr, inNumPr := false, false
	inDel, inInstr := 0, false
	inText := false
	tracked := false
	pDepth := 0
	for {
		tok, err := dec.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, tracked, err
		}
		switch t := tok.(type) {
		case xml.StartElement:
			switch t.Name.Local {
			case "tbl":
				tableDepth++
			case "p":
				pDepth++
				if pDepth == 1 {
					cur = &rawPara{numID: -1, ilvl: -1, outlineLvl: -1, inTable: tableDepth > 0}
					sb.Reset()
				}
			case "pPr":
				inPPr = true
			case "numPr":
				if inPPr && cur != nil {
					inNumPr = true
					cur.hasNumPr = true
				}
			case "pStyle":
				if inPPr && cur != nil {
					cur.styleID, _ = attr(t, "val")
				}
			case "ilvl":
				if inNumPr && cur != nil {
					v, _ := attr(t, "val")
					cur.ilvl = atoiDefault(v, 0)
				}
			case "numId":
				if inNumPr && cur != nil {
					v, _ := attr(t, "val")
					cur.numID = atoiDefault(v, 0)
				}
			case "outlineLvl":
				if inPPr && cur != nil {
					v, _ := attr(t, "val")
					cur.outlineLvl = atoiDefault(v, -1)
				}
			case "del", "moveFrom":
				inDel++
				tracked = true
			case "ins", "moveTo":
				tracked = true
			case "instrText":
				inInstr = true
			case "t":
				inText = true
			case "tab":
				if cur != nil && !inPPr && inDel == 0 {
					sb.WriteString("\t")
				}
			case "br", "cr":
				if cur != nil && inDel == 0 {
					sb.WriteString("\n")
				}
			case "noBreakHyphen":
				if cur != nil && inDel == 0 {
					sb.WriteString("-")
				}
			}
		case xml.EndElement:
			switch t.Name.Local {
			case "tbl":
				tableDepth--
			case "p":
				pDepth--
				if pDepth == 0 && cur != nil {
					cur.text = normalizeSpace(sb.String())
					out = append(out, *cur)
					cur = nil
				}
			case "pPr":
				inPPr = false
			case "numPr":
				inNumPr = false
			case "del", "moveFrom":
				inDel--
			case "instrText":
				inInstr = false
			case "t":
				inText = false
			}
		case xml.CharData:
			if cur != nil && inText && inDel == 0 && !inInstr {
				sb.Write(t)
			}
		}
	}
	return out, tracked, nil
}

var multiSpace = regexp.MustCompile(`[ \x{00A0}]+`)

func normalizeSpace(s string) string {
	s = strings.ReplaceAll(s, "­", "") // pehme poolitus
	s = multiSpace.ReplaceAllString(s, " ")
	return strings.TrimSpace(s)
}

// ---------- styles.xml ----------

type style struct {
	id           string
	name         string
	basedOn      string
	hasNumPr     bool
	numID        int
	ilvl         int
	outlineLvl   int
	headingLevel int
}

var headingName = regexp.MustCompile(`(?i)^(heading|pealkiri|überschrift|rubrik)\s*([1-9])$`)

func parseStyles(b []byte) (map[string]*style, error) {
	dec := xml.NewDecoder(bytes.NewReader(b))
	out := map[string]*style{}
	var cur *style
	inPPr, inNumPr := false, false
	for {
		tok, err := dec.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return out, err
		}
		switch t := tok.(type) {
		case xml.StartElement:
			switch t.Name.Local {
			case "style":
				id, _ := attr(t, "styleId")
				cur = &style{id: id, outlineLvl: -1, ilvl: 0}
			case "name":
				if cur != nil {
					cur.name, _ = attr(t, "val")
				}
			case "basedOn":
				if cur != nil {
					cur.basedOn, _ = attr(t, "val")
				}
			case "pPr":
				inPPr = true
			case "numPr":
				if inPPr && cur != nil {
					inNumPr = true
					cur.hasNumPr = true
				}
			case "numId":
				if inNumPr && cur != nil {
					v, _ := attr(t, "val")
					cur.numID = atoiDefault(v, 0)
				}
			case "ilvl":
				if inNumPr && cur != nil {
					v, _ := attr(t, "val")
					cur.ilvl = atoiDefault(v, 0)
				}
			case "outlineLvl":
				if inPPr && cur != nil {
					v, _ := attr(t, "val")
					cur.outlineLvl = atoiDefault(v, -1)
				}
			}
		case xml.EndElement:
			switch t.Name.Local {
			case "style":
				if cur != nil && cur.id != "" {
					out[cur.id] = cur
				}
				cur = nil
			case "pPr":
				inPPr = false
			case "numPr":
				inNumPr = false
			}
		}
	}
	return out, nil
}

// resolveStyle arvutab stiili efektiivsed omadused basedOn-ahelat järgides.
func resolveStyle(styles map[string]*style, id string) style {
	res := style{outlineLvl: -1}
	seen := map[string]bool{}
	gotNum, gotHeading := false, false
	for id != "" && !seen[id] {
		seen[id] = true
		s, ok := styles[id]
		if !ok {
			break
		}
		if !gotNum && s.hasNumPr {
			res.numID, res.ilvl = s.numID, s.ilvl
			gotNum = true
		}
		if !gotHeading {
			if m := headingName.FindStringSubmatch(strings.TrimSpace(s.name)); m != nil {
				res.headingLevel = atoiDefault(m[2], 0)
				gotHeading = true
			} else if m := headingName.FindStringSubmatch(s.id); m != nil {
				res.headingLevel = atoiDefault(m[2], 0)
				gotHeading = true
			} else if s.outlineLvl >= 0 && s.outlineLvl < 9 {
				res.headingLevel = s.outlineLvl + 1
				gotHeading = true
			}
		}
		id = s.basedOn
	}
	return res
}
