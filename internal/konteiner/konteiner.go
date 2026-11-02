// Package konteiner loeb Eesti digiallkirja konteinereid (ASiC-E: .asice,
// .bdoc, .sce, .edoc). Konteiner on ZIP-fail, mille sees on allkirjastatud
// failid ja META-INF kaustas allkirjad. Pakett ei kontrolli allkirjade
// kehtivust (selleks on DigiDoc4 / digidoc-tool), vaid ainult loetleb ja
// eraldab sisu.
package konteiner

import (
	"archive/zip"
	"bytes"
	"errors"
	"fmt"
	"io"
	"path"
	"sort"
	"strings"
)

// Laiendid, mida käsitletakse konteinerina.
var Laiendid = map[string]bool{".asice": true, ".bdoc": true, ".sce": true, ".edoc": true, ".asics": true, ".scs": true}

// MaxSisemine piirab ühe lahtipakitud faili suurust (ZIP-pommi kaitse).
const MaxSisemine = 100 << 20

// Fail on konteineris olev allkirjastatud fail.
type Fail struct {
	Nimi   string `json:"nimi"`
	Suurus int64  `json:"suurus"`
}

// Sisu kirjeldab konteinerit.
type Sisu struct {
	Mimetype      string   `json:"mimetype"`
	Failid        []Fail   `json:"failid"`
	Allkirjafaile int      `json:"allkirjafaile"`
	Hoiatused     []string `json:"hoiatused"`
}

// ErrDDOC tagastatakse vana DDOC-vormingu korral.
var ErrDDOC = errors.New("vana DigiDoc (.ddoc) vormingut ei toetata: ava fail DigiDoc4 rakenduses ja salvesta sisu eraldi")

// Loe tagastab konteineri sisukirjelduse.
func Loe(b []byte) (*Sisu, *zip.Reader, error) {
	if bytes.HasPrefix(bytes.TrimSpace(b), []byte("<?xml")) {
		return nil, nil, ErrDDOC
	}
	zr, err := zip.NewReader(bytes.NewReader(b), int64(len(b)))
	if err != nil {
		return nil, nil, fmt.Errorf("fail ei ole digiallkirja konteiner (ZIP): %w", err)
	}
	s := &Sisu{Failid: []Fail{}, Hoiatused: []string{"allkirjade kehtivust ei kontrollita – kasuta DigiDoc4 rakendust"}}
	for _, f := range zr.File {
		switch {
		case f.Name == "mimetype":
			if rc, err := f.Open(); err == nil {
				m, _ := io.ReadAll(io.LimitReader(rc, 200))
				rc.Close()
				s.Mimetype = strings.TrimSpace(string(m))
			}
		case strings.HasPrefix(f.Name, "META-INF/"):
			if strings.Contains(strings.ToLower(path.Base(f.Name)), "signature") {
				s.Allkirjafaile++
			}
		case strings.HasSuffix(f.Name, "/"):
		default:
			s.Failid = append(s.Failid, Fail{Nimi: f.Name, Suurus: int64(f.UncompressedSize64)})
		}
	}
	sort.Slice(s.Failid, func(i, j int) bool { return s.Failid[i].Nimi < s.Failid[j].Nimi })
	if !strings.Contains(s.Mimetype, "asic") {
		s.Hoiatused = append(s.Hoiatused, fmt.Sprintf("ootamatu mimetype %q – kas see on digiallkirja konteiner?", s.Mimetype))
	}
	if s.Allkirjafaile == 0 {
		s.Hoiatused = append(s.Hoiatused, "konteineris ei ole allkirjafaile (allkirjastamata?)")
	}
	return s, zr, nil
}

// Analuusitavad on konteineri sees analüüsitavad vormingud.
var Analuusitavad = map[string]bool{".docx": true, ".pdf": true}

// ValiDokument leiab konteinerist analüüsitava faili (.docx või .pdf). Kui
// nimi on tühi, eelistatakse ainsat .docx faili, selle puudumisel ainsat
// .pdf faili. Kui konteineri sees on ainult teine konteiner (topeltallkiri),
// otsitakse sealt.
func ValiDokument(b []byte, nimi string) ([]byte, string, *Sisu, error) {
	return valiDokument(b, nimi, 0)
}

func valiDokument(b []byte, nimi string, sugavus int) ([]byte, string, *Sisu, error) {
	s, zr, err := Loe(b)
	if err != nil {
		return nil, "", nil, err
	}
	var docxid, pdfid, sisemised []*zip.File
	var valitud *zip.File
	for _, f := range zr.File {
		if strings.HasPrefix(f.Name, "META-INF/") || strings.HasSuffix(f.Name, "/") {
			continue
		}
		if nimi != "" {
			if f.Name == nimi || path.Base(f.Name) == nimi {
				valitud = f
				break
			}
			continue
		}
		switch ext := strings.ToLower(path.Ext(f.Name)); {
		case ext == ".docx":
			docxid = append(docxid, f)
		case ext == ".pdf":
			pdfid = append(pdfid, f)
		case Laiendid[ext]:
			sisemised = append(sisemised, f)
		}
	}
	nimed := func() string {
		var n []string
		for _, f := range s.Failid {
			n = append(n, f.Nimi)
		}
		return strings.Join(n, ", ")
	}
	switch {
	case nimi != "" && valitud == nil:
		return nil, "", s, fmt.Errorf("konteineris ei ole faili %q; failid: %s", nimi, nimed())
	case valitud != nil:
	case len(docxid) == 1:
		valitud = docxid[0]
	case len(docxid) > 1:
		return nil, "", s, fmt.Errorf("konteineris on mitu .docx faili, vali üks (sisemine): %s", nimed())
	case len(pdfid) == 1:
		valitud = pdfid[0]
	case len(pdfid) > 1:
		return nil, "", s, fmt.Errorf("konteineris on mitu PDF-faili, vali üks (sisemine): %s", nimed())
	case len(sisemised) == 1 && sugavus < 3:
		data, err := loeSisemine(sisemised[0])
		if err != nil {
			return nil, "", s, err
		}
		d, n, _, err := valiDokument(data, "", sugavus+1)
		if err != nil {
			return nil, "", s, fmt.Errorf("%s: %w", sisemised[0].Name, err)
		}
		return d, sisemised[0].Name + "/" + n, s, nil
	default:
		return nil, "", s, fmt.Errorf("konteineris ei ole .docx ega .pdf faili (failid: %s)", nimed())
	}
	ext := strings.ToLower(path.Ext(valitud.Name))
	if !Analuusitavad[ext] {
		return nil, "", s, fmt.Errorf("%s ei ole .docx ega .pdf fail; muid vorminguid tööriist ei analüüsi", valitud.Name)
	}
	data, err := loeSisemine(valitud)
	if err != nil {
		return nil, "", s, err
	}
	return data, valitud.Name, s, nil
}

func loeSisemine(f *zip.File) ([]byte, error) {
	if f.UncompressedSize64 > MaxSisemine {
		return nil, fmt.Errorf("%s on liiga suur (%d baiti)", f.Name, f.UncompressedSize64)
	}
	rc, err := f.Open()
	if err != nil {
		return nil, err
	}
	defer rc.Close()
	data, err := io.ReadAll(io.LimitReader(rc, MaxSisemine+1))
	if err != nil {
		return nil, err
	}
	if len(data) > MaxSisemine {
		return nil, fmt.Errorf("%s on liiga suur", f.Name)
	}
	return data, nil
}
