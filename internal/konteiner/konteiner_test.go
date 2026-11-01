package konteiner

import (
	"archive/zip"
	"bytes"
	"strings"
	"testing"

	"github.com/Ker102/lepinguparser/internal/docx"
	"github.com/Ker102/lepinguparser/internal/testdocx"
)

func asice(t *testing.T, failid map[string][]byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	w, _ := zw.CreateHeader(&zip.FileHeader{Name: "mimetype", Method: zip.Store})
	w.Write([]byte("application/vnd.etsi.asic-e+zip"))
	for n, b := range failid {
		w, _ := zw.Create(n)
		w.Write(b)
	}
	w, _ = zw.Create("META-INF/manifest.xml")
	w.Write([]byte("<manifest/>"))
	w, _ = zw.Create("META-INF/signatures0.xml")
	w.Write([]byte("<XAdESSignatures/>"))
	zw.Close()
	return buf.Bytes()
}

func TestValiDocx(t *testing.T) {
	leping := testdocx.Koosta(testdocx.NaidisLeping(), testdocx.Nummerdus)
	k := asice(t, map[string][]byte{"Leping.docx": leping, "Lisa1.pdf": []byte("%PDF-1.4")})
	b, nimi, s, err := ValiDocx(k, "")
	if err != nil || nimi != "Leping.docx" {
		t.Fatalf("%v %s", err, nimi)
	}
	if s.Allkirjafaile != 1 || len(s.Failid) != 2 || !strings.Contains(s.Mimetype, "asic-e") {
		t.Errorf("sisu: %+v", s)
	}
	if _, err := docx.Read(b); err != nil {
		t.Errorf("eraldatud docx ei ole loetav: %v", err)
	}

	mitu := asice(t, map[string][]byte{"a.docx": leping, "b.docx": leping})
	if _, _, _, err := ValiDocx(mitu, ""); err == nil || !strings.Contains(err.Error(), "mitu") {
		t.Errorf("mitu docx: %v", err)
	}
	if _, nimi, _, err := ValiDocx(mitu, "b.docx"); err != nil || nimi != "b.docx" {
		t.Errorf("nimega valik: %v %s", err, nimi)
	}
	pdf := asice(t, map[string][]byte{"Leping.pdf": []byte("%PDF-1.4")})
	if _, _, _, err := ValiDocx(pdf, ""); err == nil || !strings.Contains(err.Error(), "Leping.pdf") {
		t.Errorf("ainult PDF: %v", err)
	}
	if _, _, err := Loe([]byte(`<?xml version="1.0"?><SignedDoc/>`)); err != ErrDDOC {
		t.Errorf("ddoc: %v", err)
	}
}
