// Package mcpserver avaldab lepinguparseri tööriistad MCP serverina (stdio).
package mcpserver

import (
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/Ker102/lepinguparser/internal/analyys"
	"github.com/Ker102/lepinguparser/internal/konteiner"
	"github.com/Ker102/lepinguparser/internal/pdf"
	"github.com/Ker102/lepinguparser/internal/teenus"
)

type failSisend struct {
	Fail     string `json:"fail" jsonschema:".docx või PDF-faili või digiallkirja konteineri (.asice, .bdoc) absoluutne tee"`
	Sisemine string `json:"sisemine,omitempty" jsonschema:"konteineri puhul sisemise .docx või PDF-faili nimi, kui neid on mitu"`
}

type sisukordSisend struct {
	Fail     string `json:"fail" jsonschema:".docx või PDF-faili või digiallkirja konteineri (.asice, .bdoc) absoluutne tee"`
	Sisemine string `json:"sisemine,omitempty" jsonschema:"konteineri puhul sisemise .docx või PDF-faili nimi, kui neid on mitu"`
	MaxTase  int    `json:"max_tase,omitempty" jsonschema:"näita ainult kuni selle tasemeni (0 = kõik)"`
}

type sektsioonSisend struct {
	Fail     string `json:"fail" jsonschema:".docx või PDF-faili või digiallkirja konteineri (.asice, .bdoc) absoluutne tee"`
	Sisemine string `json:"sisemine,omitempty" jsonschema:"konteineri puhul sisemise .docx või PDF-faili nimi, kui neid on mitu"`
	Number   string `json:"number" jsonschema:"punkti number, nt 7.2, 7.2(a) või Lisa 1"`
}

type terminSisend struct {
	Fail     string `json:"fail" jsonschema:".docx või PDF-faili või digiallkirja konteineri (.asice, .bdoc) absoluutne tee"`
	Sisemine string `json:"sisemine,omitempty" jsonschema:"konteineri puhul sisemise .docx või PDF-faili nimi, kui neid on mitu"`
	Termin   string `json:"termin" jsonschema:"defineeritud termin algvormis, nt Leping"`
}

type kandidaadidSisend struct {
	Fail     string `json:"fail" jsonschema:".docx või PDF-faili või digiallkirja konteineri (.asice, .bdoc) absoluutne tee"`
	Sisemine string `json:"sisemine,omitempty" jsonschema:"konteineri puhul sisemise .docx või PDF-faili nimi, kui neid on mitu"`
	Piir     int    `json:"piir,omitempty" jsonschema:"maksimaalne tulemuste arv (vaikimisi 50)"`
}

type viitedSisend struct {
	Fail     string `json:"fail" jsonschema:".docx või PDF-faili või digiallkirja konteineri (.asice, .bdoc) absoluutne tee"`
	Sisemine string `json:"sisemine,omitempty" jsonschema:"konteineri puhul sisemise .docx või PDF-faili nimi, kui neid on mitu"`
	Olek     string `json:"olek,omitempty" jsonschema:"filtreeri oleku järgi: leitud, puudub, lisa_eraldi, valine"`
}

type tekstSisend struct {
	Fail     string `json:"fail" jsonschema:".docx või PDF-faili või digiallkirja konteineri (.asice, .bdoc) absoluutne tee"`
	Sisemine string `json:"sisemine,omitempty" jsonschema:"konteineri puhul sisemise .docx või PDF-faili nimi, kui neid on mitu"`
	Alates   int    `json:"alates,omitempty" jsonschema:"esimene lõik (loik), vaikimisi algusest"`
	Kuni     int    `json:"kuni,omitempty" jsonschema:"viimane lõik (loik), vaikimisi lõpuni"`
}

type definitsioonidVastus struct {
	Definitsioonid []analyys.Definitsioon `json:"definitsioonid"`
}
type kasutusVastus struct {
	Termin  string            `json:"termin"`
	Kasutus []analyys.Kasutus `json:"kasutus"`
}
type kandidaadidVastus struct {
	Markus      string              `json:"markus"`
	Kandidaadid []analyys.Kandidaat `json:"kandidaadid"`
}
type viitedVastus struct {
	Viited []analyys.Viide `json:"viited"`
}

// Uus loob serveri kõigi tööriistadega.
func Uus(versioon string) *mcp.Server {
	s := mcp.NewServer(&mcp.Implementation{Name: "lepinguparser", Version: versioon}, nil)

	mcp.AddTool(s, &mcp.Tool{Name: "sisukord", Description: "Lepingu (.docx või PDF) struktuur: punktide numbrid Wordi automaatnummerdusest või tekstist, pealkirjad ja tasemed. PDF-i puhul taastatakse numbrid lehe paigutusest."},
		func(ctx context.Context, _ *mcp.CallToolRequest, in sisukordSisend) (*mcp.CallToolResult, teenus.Sisukord, error) {
			d, err := teenus.AvaSees(in.Fail, in.Sisemine)
			if err != nil {
				return nil, teenus.Sisukord{}, err
			}
			return nil, teenus.TeeSisukord(d, in.MaxTase), nil
		})

	mcp.AddTool(s, &mcp.Tool{Name: "tekst", Description: "Dokumendi tekst lõikudena koos punktinumbrite ja (PDF-i puhul) lehekülgedega. PDF-ist eemaldatakse korduvad päised, jalused ja leheküljenumbrid ning poolitatud sõnad ühendatakse. Pika dokumendi korral kasuta alates/kuni."},
		func(ctx context.Context, _ *mcp.CallToolRequest, in tekstSisend) (*mcp.CallToolResult, teenus.Tekst, error) {
			d, err := teenus.AvaSees(in.Fail, in.Sisemine)
			if err != nil {
				return nil, teenus.Tekst{}, err
			}
			return nil, teenus.TeeTekst(d, in.Alates, in.Kuni), nil
		})

	mcp.AddTool(s, &mcp.Tool{Name: "sektsioon", Description: "Tagastab lepingu ühe punkti (koos alapunktidega) teksti numbri järgi, nt 7.2 või Lisa 1."},
		func(ctx context.Context, _ *mcp.CallToolRequest, in sektsioonSisend) (*mcp.CallToolResult, teenus.SektsiooniVastus, error) {
			d, err := teenus.AvaSees(in.Fail, in.Sisemine)
			if err != nil {
				return nil, teenus.SektsiooniVastus{}, err
			}
			return nil, teenus.TeeSektsioon(d, in.Number), nil
		})

	mcp.AddTool(s, &mcp.Tool{Name: "definitsioonid", Description: "Leiab lepingus defineeritud terminid (nt (edaspidi „Leping“), „Termin“ tähendab …, \"Term\" means …)."},
		func(ctx context.Context, _ *mcp.CallToolRequest, in failSisend) (*mcp.CallToolResult, definitsioonidVastus, error) {
			d, err := teenus.AvaSees(in.Fail, in.Sisemine)
			if err != nil {
				return nil, definitsioonidVastus{}, err
			}
			return nil, definitsioonidVastus{Definitsioonid: nonNilDefs(d.LeiaDefinitsioonid())}, nil
		})

	mcp.AddTool(s, &mcp.Tool{Name: "termini_kasutus", Description: "Leiab termini kõik esinemised, sh eesti käändevormid (Leping → Lepingu, Lepingus; Pool → Poole, Pooled)."},
		func(ctx context.Context, _ *mcp.CallToolRequest, in terminSisend) (*mcp.CallToolResult, kasutusVastus, error) {
			d, err := teenus.AvaSees(in.Fail, in.Sisemine)
			if err != nil {
				return nil, kasutusVastus{}, err
			}
			k := d.TerminiKasutus(in.Termin)
			if k == nil {
				k = []analyys.Kasutus{}
			}
			return nil, kasutusVastus{Termin: in.Termin, Kasutus: k}, nil
		})

	mcp.AddTool(s, &mcp.Tool{Name: "defineerimata_terminid", Description: "Heuristiline: suure tähega sõnad/fraasid lause keskel, mis ei vasta ühelegi defineeritud terminile. Tulemus on kandidaatide loetelu, mitte kindel viga."},
		func(ctx context.Context, _ *mcp.CallToolRequest, in kandidaadidSisend) (*mcp.CallToolResult, kandidaadidVastus, error) {
			d, err := teenus.AvaSees(in.Fail, in.Sisemine)
			if err != nil {
				return nil, kandidaadidVastus{}, err
			}
			piir := in.Piir
			if piir <= 0 {
				piir = 50
			}
			k := d.DefineerimataKandidaadid(d.LeiaDefinitsioonid(), piir)
			if k == nil {
				k = []analyys.Kandidaat{}
			}
			return nil, kandidaadidVastus{Markus: "heuristiline loetelu; kontrolli käsitsi", Kandidaadid: k}, nil
		})

	mcp.AddTool(s, &mcp.Tool{Name: "viited", Description: "Leiab sisemised ristviited (punkt 7.2, p-s 3.1, clause 4, Lisa 1) ja märgib, kas sihtpunkt on olemas (olek: leitud, puudub, lisa_eraldi, valine)."},
		func(ctx context.Context, _ *mcp.CallToolRequest, in viitedSisend) (*mcp.CallToolResult, viitedVastus, error) {
			d, err := teenus.AvaSees(in.Fail, in.Sisemine)
			if err != nil {
				return nil, viitedVastus{}, err
			}
			all := d.LeiaViited()
			out := []analyys.Viide{}
			for _, v := range all {
				if in.Olek == "" || v.Olek == in.Olek {
					out = append(out, v)
				}
			}
			return nil, viitedVastus{Viited: out}, nil
		})

	mcp.AddTool(s, &mcp.Tool{Name: "kus_viidatakse", Description: "Leiab kõik kohad, kus viidatakse antud punktile või selle alapunktidele."},
		func(ctx context.Context, _ *mcp.CallToolRequest, in sektsioonSisend) (*mcp.CallToolResult, viitedVastus, error) {
			d, err := teenus.AvaSees(in.Fail, in.Sisemine)
			if err != nil {
				return nil, viitedVastus{}, err
			}
			v := d.KusViidatakse(in.Number, d.LeiaViited())
			if v == nil {
				v = []analyys.Viide{}
			}
			return nil, viitedVastus{Viited: v}, nil
		})

	mcp.AddTool(s, &mcp.Tool{Name: "kontroll", Description: "Koondkontroll: katkised ristviited, kasutamata ja topelt definitsioonid, enne defineerimist kasutatud terminid, defineerimata terminite kandidaadid ja hoiatused."},
		func(ctx context.Context, _ *mcp.CallToolRequest, in failSisend) (*mcp.CallToolResult, analyys.Aruanne, error) {
			d, err := teenus.AvaSees(in.Fail, in.Sisemine)
			if err != nil {
				return nil, analyys.Aruanne{}, err
			}
			return nil, d.Kontroll(), nil
		})

	mcp.AddTool(s, &mcp.Tool{Name: "konteineri_sisu", Description: "Digiallkirja konteineri (.asice, .bdoc) failide loetelu ja allkirjafailide arv. Allkirjade kehtivust ei kontrollita."},
		func(ctx context.Context, _ *mcp.CallToolRequest, in failSisend) (*mcp.CallToolResult, *konteiner.Sisu, error) {
			k, err := teenus.KonteineriSisu(in.Fail)
			return nil, k, err
		})

	mcp.AddTool(s, &mcp.Tool{Name: "pdf_teave", Description: "PDF-faili (ka konteineri sees oleva) metaandmed, lehekülgede arv, tekstita (skannitud) lehed ja PDF-i manustatud digiallkirjad (PAdES): allkirjastaja nimi, sertifikaadi väljaandja, aeg ja kas allkiri katab kogu faili. Kehtivust ei kontrollita."},
		func(ctx context.Context, _ *mcp.CallToolRequest, in failSisend) (*mcp.CallToolResult, *pdf.Teave, error) {
			t, err := teenus.PDFTeave(in.Fail, in.Sisemine)
			return nil, t, err
		})

	return s
}

func nonNilDefs(d []analyys.Definitsioon) []analyys.Definitsioon {
	if d == nil {
		return []analyys.Definitsioon{}
	}
	return d
}

// Kaivita käivitab serveri stdio kaudu.
func Kaivita(ctx context.Context, versioon string) error {
	return Uus(versioon).Run(ctx, &mcp.StdioTransport{})
}
