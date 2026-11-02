// Käsk lepinguparser analüüsib lepingute .docx ja PDF-faile: struktuur,
// definitsioonid, terminite kasutus ja sisemised ristviited.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"

	"github.com/Ker102/lepinguparser/internal/analyys"
	"github.com/Ker102/lepinguparser/internal/mcpserver"
	"github.com/Ker102/lepinguparser/internal/teenus"
)

var versioon = "0.2.0"

const kasutus = `lepinguparser – lepingu .docx või PDF-faili struktuuri ja terminite analüüs

Kasutus:
  lepinguparser <käsk> [valikud] FAIL [argument]

Käsud:
  sisukord FAIL                 punktide numbrid ja pealkirjad
  sektsioon FAIL NR             punkti tekst (nt 7.2, 7.2(a), "Lisa 1")
  definitsioonid FAIL           defineeritud terminid
  termin FAIL TERMIN            termini kõik esinemised (käändevormid kaasa arvatud)
  defineerimata FAIL            defineerimata terminite kandidaadid (heuristiline)
  viited FAIL                   sisemised ristviited ja nende olek
  kus-viidatakse FAIL NR        kus viidatakse punktile NR
  kontroll FAIL                 koondkontroll (katkised viited, kasutamata terminid jm)
  tekst FAIL                    kogu tekst lõikudena koos punktinumbrite ja lk-ga
  konteiner FAIL                digiallkirja konteineri (.asice, .bdoc) failid
  pdf FAIL                      PDF-i metaandmed, lehekülgede arv ja PDF-i allkirjad
  mcp                           käivita MCP server (stdio)
  versioon                      näita versiooni

Valikud (enne faili nime):
  -t                            inimloetav tekst JSONi asemel
  -tase N                       sisukord: näita kuni tasemeni N
  -olek OLEK                    viited: filtreeri (leitud, puudub, lisa_eraldi, valine)
  -piir N                       defineerimata: tulemuste arv (vaikimisi 50)
  -sees NIMI                    konteineri puhul sisemise faili nimi

FAIL võib olla .docx, .pdf või digiallkirja konteiner (.asice, .bdoc), mille
sees on .docx või .pdf. PDF-i lõigud ja punktinumbrid taastatakse lehe
paigutusest. Allkirjade kehtivust ei kontrollita.

Väljumiskoodid: 0 = korras, 1 = viga, 2 = vale kasutus,
                3 = kontroll leidis katkiseid viiteid.
`

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, out, errw io.Writer) int {
	if len(args) == 0 || args[0] == "-h" || args[0] == "--help" || args[0] == "abi" {
		fmt.Fprint(out, kasutus)
		return 0
	}
	cmd, rest := args[0], args[1:]
	switch cmd {
	case "versioon", "--version":
		fmt.Fprintln(out, "lepinguparser", versioon)
		return 0
	case "mcp":
		ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
		defer stop()
		if err := mcpserver.Kaivita(ctx, versioon); err != nil {
			fmt.Fprintln(errw, "viga:", err)
			return 1
		}
		return 0
	}

	fs := flag.NewFlagSet(cmd, flag.ContinueOnError)
	fs.SetOutput(errw)
	tekst := fs.Bool("t", false, "inimloetav väljund")
	tase := fs.Int("tase", 0, "sisukorra maksimaalne tase")
	olek := fs.String("olek", "", "viidete filter")
	piir := fs.Int("piir", 50, "tulemuste piir")
	sees := fs.String("sees", "", "konteineri sisemine fail")
	if err := fs.Parse(rest); err != nil {
		return 2
	}
	pos := fs.Args()
	if cmd == "konteiner" {
		if len(fs.Args()) < 1 {
			fmt.Fprintln(errw, "käsk \"konteiner\" vajab faili")
			return 2
		}
		k, err := teenus.KonteineriSisu(fs.Args()[0])
		if err != nil {
			fmt.Fprintln(errw, "viga:", err)
			return 1
		}
		enc := json.NewEncoder(out)
		enc.SetIndent("", "  ")
		enc.SetEscapeHTML(false)
		_ = enc.Encode(k)
		return 0
	}
	if cmd == "pdf" {
		if len(pos) < 1 {
			fmt.Fprintln(errw, "käsk \"pdf\" vajab faili")
			return 2
		}
		t, err := teenus.PDFTeave(pos[0], *sees)
		if err != nil {
			fmt.Fprintln(errw, "viga:", err)
			return 1
		}
		enc := json.NewEncoder(out)
		enc.SetIndent("", "  ")
		enc.SetEscapeHTML(false)
		_ = enc.Encode(t)
		return 0
	}
	need := map[string]int{"tekst": 1, "sisukord": 1, "sektsioon": 2, "definitsioonid": 1, "termin": 2, "defineerimata": 1, "viited": 1, "kus-viidatakse": 2, "kontroll": 1}
	n, ok := need[cmd]
	if !ok {
		fmt.Fprintf(errw, "tundmatu käsk %q\n\n%s", cmd, kasutus)
		return 2
	}
	if len(pos) < n {
		fmt.Fprintf(errw, "käsk %q vajab %d argumenti\n", cmd, n)
		return 2
	}
	d, err := teenus.AvaSees(pos[0], *sees)
	if err != nil {
		fmt.Fprintln(errw, "viga:", err)
		return 1
	}
	arg := strings.Join(pos[1:], " ")

	var res any
	code := 0
	switch cmd {
	case "tekst":
		res = teenus.TeeTekst(d, 0, 0)
	case "sisukord":
		res = teenus.TeeSisukord(d, *tase)
	case "sektsioon":
		v := teenus.TeeSektsioon(d, arg)
		if !v.Leitud {
			code = 1
		}
		res = v
	case "definitsioonid":
		res = d.LeiaDefinitsioonid()
	case "termin":
		res = d.TerminiKasutus(arg)
	case "defineerimata":
		res = d.DefineerimataKandidaadid(d.LeiaDefinitsioonid(), *piir)
	case "viited":
		var f []analyys.Viide
		for _, v := range d.LeiaViited() {
			if *olek == "" || v.Olek == *olek {
				f = append(f, v)
			}
		}
		res = f
	case "kus-viidatakse":
		res = d.KusViidatakse(arg, d.LeiaViited())
	case "kontroll":
		a := d.Kontroll()
		if len(a.KatkisedViited) > 0 {
			code = 3
		}
		res = a
	}
	if *tekst {
		prindiTekst(out, cmd, res)
	} else {
		enc := json.NewEncoder(out)
		enc.SetIndent("", "  ")
		enc.SetEscapeHTML(false)
		if err := enc.Encode(res); err != nil {
			fmt.Fprintln(errw, "viga:", err)
			return 1
		}
	}
	return code
}

func prindiTekst(w io.Writer, cmd string, res any) {
	switch v := res.(type) {
	case teenus.Sisukord:
		for _, r := range v.Read {
			fmt.Fprintf(w, "%s%-10s %s\n", strings.Repeat("  ", max(0, r.Tase-1)), r.Number, r.Pealkiri)
		}
		hoiatused(w, v.Hoiatused)
	case teenus.Tekst:
		lk := 0
		for _, l := range v.Loigud {
			if l.Lk != lk && l.Lk > 0 {
				lk = l.Lk
				fmt.Fprintf(w, "--- lk %d ---\n", lk)
			}
			if l.Number != "" {
				fmt.Fprintf(w, "%s %s\n", l.Number, l.Tekst)
			} else {
				fmt.Fprintln(w, l.Tekst)
			}
		}
		hoiatused(w, v.Hoiatused)
	case teenus.SektsiooniVastus:
		if !v.Leitud {
			fmt.Fprintf(w, "Punkti %s ei leitud.", v.Number)
			if len(v.Sarnased) > 0 {
				fmt.Fprintf(w, " Sarnased: %s", strings.Join(v.Sarnased, ", "))
			}
			fmt.Fprintln(w)
			return
		}
		for _, l := range v.Loigud {
			fmt.Fprintln(w, l.Tekst)
		}
	case []analyys.Definitsioon:
		for _, d := range v {
			fmt.Fprintf(w, "%-30s p %-8s (%s)\n", d.Termin, d.Sektsioon, d.Viis)
		}
	case []analyys.Kasutus:
		for _, k := range v {
			fmt.Fprintf(w, "p %-8s %-20s %s\n", k.Sektsioon, k.Vorm, k.Kontekst)
		}
	case []analyys.Kandidaat:
		for _, k := range v {
			fmt.Fprintf(w, "%3d× %-30s p %s\n", k.Kordi, k.Fraas, k.Sektsioon)
		}
	case []analyys.Viide:
		for _, x := range v {
			fmt.Fprintf(w, "p %-8s → %-10s %-12s %s\n", x.Sektsioon, x.Siht, x.Olek, x.Tekst)
		}
	case analyys.Aruanne:
		fmt.Fprintf(w, "Lõike %d, punkte %d, definitsioone %d\n", v.Loike, v.Sektsioone, v.Definitsioone)
		fmt.Fprintf(w, "\nKatkised viited (%d):\n", len(v.KatkisedViited))
		for _, x := range v.KatkisedViited {
			fmt.Fprintf(w, "  p %-8s → %-10s %s\n", x.Sektsioon, x.Siht, x.Tekst)
		}
		fmt.Fprintf(w, "\nKasutamata terminid (%d):\n", len(v.KasutamataTerminid))
		for _, d := range v.KasutamataTerminid {
			fmt.Fprintf(w, "  %s (p %s)\n", d.Termin, d.Sektsioon)
		}
		fmt.Fprintf(w, "\nTopelt defineeritud (%d):\n", len(v.TopeltDefinitsioonid))
		for t, ds := range v.TopeltDefinitsioonid {
			fmt.Fprintf(w, "  %s: %d korda\n", t, len(ds))
		}
		fmt.Fprintf(w, "\nKasutatud enne defineerimist (%d):\n", len(v.KasutatudEnneDefineerimist))
		for _, t := range v.KasutatudEnneDefineerimist {
			fmt.Fprintf(w, "  %s: esmakasutus p %s, definitsioon lõigus %d\n", t.Termin, t.EsimeneKasutus.Sektsioon, t.DefinitsiooniLoik)
		}
		fmt.Fprintf(w, "\nDefineerimata terminite kandidaadid (%d, heuristiline):\n", len(v.DefineerimataKandidaadid))
		for _, k := range v.DefineerimataKandidaadid {
			fmt.Fprintf(w, "  %3d× %s (p %s)\n", k.Kordi, k.Fraas, k.Sektsioon)
		}
		hoiatused(w, v.Hoiatused)
	default:
		b, _ := json.MarshalIndent(res, "", "  ")
		fmt.Fprintln(w, string(b))
	}
}

func hoiatused(w io.Writer, h []string) {
	if len(h) == 0 {
		return
	}
	fmt.Fprintln(w, "\nHoiatused:")
	for _, x := range h {
		fmt.Fprintln(w, "  -", x)
	}
}
