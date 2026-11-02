package mcpserver

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/Ker102/lepinguparser/internal/testdocx"
	"github.com/Ker102/lepinguparser/internal/testpdf"
)

func asice(t *testing.T, nimi string, sisu []byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	w, _ := zw.CreateHeader(&zip.FileHeader{Name: "mimetype", Method: zip.Store})
	w.Write([]byte("application/vnd.etsi.asic-e+zip"))
	w, _ = zw.Create(nimi)
	w.Write(sisu)
	w, _ = zw.Create("META-INF/signatures0.xml")
	w.Write([]byte("<XAdESSignatures/>"))
	zw.Close()
	return buf.Bytes()
}

func TestMCPTooriistad(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	fail := filepath.Join(dir, "leping.docx")
	if err := os.WriteFile(fail, testdocx.Koosta(testdocx.NaidisLeping(), testdocx.Nummerdus), 0o600); err != nil {
		t.Fatal(err)
	}

	st, ct := mcp.NewInMemoryTransports()
	srv := Uus("test")
	ss, err := srv.Connect(ctx, st, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer ss.Close()
	client := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "0"}, nil)
	cs, err := client.Connect(ctx, ct, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer cs.Close()

	tools, err := cs.ListTools(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(tools.Tools) != 11 {
		t.Fatalf("ootasin 11 tööriista, sain %d", len(tools.Tools))
	}

	res, err := cs.CallTool(ctx, &mcp.CallToolParams{Name: "kontroll", Arguments: map[string]any{"fail": fail}})
	if err != nil {
		t.Fatal(err)
	}
	if res.IsError {
		t.Fatalf("tööriista viga: %+v", res.Content)
	}
	b, _ := json.Marshal(res.StructuredContent)
	var ar struct {
		KatkisedViited []struct{ Siht string } `json:"katkised_viited"`
	}
	if err := json.Unmarshal(b, &ar); err != nil {
		t.Fatal(err)
	}
	if len(ar.KatkisedViited) != 1 || ar.KatkisedViited[0].Siht != "7.4" {
		t.Fatalf("ootasin katkist viidet 7.4, sain %s", b)
	}

	// PDF konteineri sees
	pdfFail := filepath.Join(dir, "leping.asice")
	if err := os.WriteFile(pdfFail, asice(t, "Leping.pdf", testpdf.Koosta([][]testpdf.Rida{{
		{X: 72, Y: 760, Tekst: "1. Lepingu ese"},
		{X: 72, Y: 740, Tekst: "1.1. Teostaja osutab Teenust vastavalt punktile 1.2."},
	}}, testpdf.Valikud{Type0: true})), 0o600); err != nil {
		t.Fatal(err)
	}
	res, err = cs.CallTool(ctx, &mcp.CallToolParams{Name: "tekst", Arguments: map[string]any{"fail": pdfFail}})
	if err != nil || res.IsError {
		t.Fatalf("tekst: %v %+v", err, res)
	}
	b, _ = json.Marshal(res.StructuredContent)
	var tk struct {
		Loigud []struct {
			Number string `json:"number"`
			Lk     int    `json:"lk"`
			Tekst  string `json:"tekst"`
		} `json:"loigud"`
	}
	if err := json.Unmarshal(b, &tk); err != nil || len(tk.Loigud) != 2 || tk.Loigud[1].Number != "1.1." || tk.Loigud[1].Lk != 1 {
		t.Fatalf("tekst: %s", b)
	}
	res, err = cs.CallTool(ctx, &mcp.CallToolParams{Name: "kontroll", Arguments: map[string]any{"fail": pdfFail}})
	if err != nil || res.IsError {
		t.Fatalf("kontroll PDF: %v %+v", err, res)
	}
	b, _ = json.Marshal(res.StructuredContent)
	ar.KatkisedViited = nil
	if err := json.Unmarshal(b, &ar); err != nil || len(ar.KatkisedViited) != 1 || ar.KatkisedViited[0].Siht != "1.2" {
		t.Fatalf("ootasin PDF-is katkist viidet 1.2, sain %s", b)
	}
	res, err = cs.CallTool(ctx, &mcp.CallToolParams{Name: "pdf_teave", Arguments: map[string]any{"fail": pdfFail}})
	if err != nil || res.IsError {
		t.Fatalf("pdf_teave: %v %+v", err, res)
	}
	b, _ = json.Marshal(res.StructuredContent)
	if !strings.Contains(string(b), `"lehekulgi":1`) {
		t.Fatalf("pdf_teave: %s", b)
	}

	// juurdepääsupiirang
	t.Setenv("LEPINGUPARSER_JUURED", filepath.Join(dir, "muu"))
	res, err = cs.CallTool(ctx, &mcp.CallToolParams{Name: "sisukord", Arguments: map[string]any{"fail": fail}})
	if err != nil {
		t.Fatal(err)
	}
	if !res.IsError {
		t.Fatal("piiratud kaustast väljas olev fail peab andma vea")
	}
}
