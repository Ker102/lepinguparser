# lepinguparser

Lepingute `.docx` failide struktuurianalüüs eesti ja inglise keeles: punktide
numbrid, definitsioonid, terminite kasutus (koos eesti käändevormidega) ja
sisemised ristviited. Töötab käsurealt ja MCP serverina, nii et Claude ja
teised agendid saavad seda tööriistana kasutada.

Tööriist ei saada dokumente kuhugi: kogu analüüs toimub kohalikus arvutis.

## Mida see teeb

| Käsk / MCP tööriist | Mida tagastab |
| --- | --- |
| `sisukord` | punktide numbrid ja pealkirjad, sh Wordi automaatnummerdusest arvutatud numbrid |
| `sektsioon` | ühe punkti (koos alapunktidega) tekst, nt `7.2`, `7.2(a)`, `Lisa 1` |
| `definitsioonid` | defineeritud terminid |
| `termin` / `termini_kasutus` | termini kõik esinemised, sh käändevormid |
| `defineerimata` / `defineerimata_terminid` | suure algustähega sõnad, mis ei vasta ühelegi definitsioonile (kandidaadid) |
| `viited` | sisemised ristviited ja nende olek |
| `kus-viidatakse` / `kus_viidatakse` | kõik kohad, kus viidatakse antud punktile |
| `konteiner` / `konteineri_sisu` | digiallkirja konteineri failid ja allkirjafailide arv |
| `kontroll` | koondaruanne: katkised viited, kasutamata ja topelt definitsioonid, enne defineerimist kasutatud terminid, kandidaadid, hoiatused |

### Mida see tunneb ära

- **Wordi automaatnummerdus** (`numbering.xml`): mitmetasemelised loendid,
  stiilidest päritud nummerdus, `startOverride`, `lvlRestart`, tähed ja
  rooma numbrid. Testitud nelja avaliku hankelepingu peal: kõik 173
  punktinumbrit langesid kokku LibreOffice'i kuvatavaga.
- **Käsitsi kirjutatud numbrid** (`7.2. Tekst`), kui automaatnummerdust pole.
- **Definitsioonid:**
  - `(edaspidi „Leping“)`, `edaspidi koos „Pooled“ ja eraldi „Pool“`;
  - jutumärkideta, nagu avaliku sektori lepingutes tavaline:
    `(edaspidi nimetatud ka tellija)`, `edaspidi eraldi pool või koos pooled`;
  - loetelu: `„Teenus“ tähendab …`, `„Tasu“ – …`;
  - inglise keel: `"Agreement" means …`, `(the "Agreement")`.
- **Käändevormid:** `Leping` → `Lepingu`, `Lepingus`; `Pool` → `Poole`,
  `Poolte`; `Konfidentsiaalne Teave` → `Konfidentsiaalse Teabe`. Väiketähega
  defineeritud termin (`tellija`) sobib ka lause alguses (`Tellija`).
- **Viited:** `punkt 7.2`, `punktis 7.2`, `p-s 3.1`, `punktides 7.1 ja 7.2`,
  `3.3(b)`, `clause 4`, `Section 5`, `Lisa 1`, `lisas 2`, `Lisadest 1`,
  `Annex A`.
- **Jälgitavad muudatused:** analüüs kasutab teksti koos lisandustega ja ilma
  kustutusteta ning annab sellest hoiatuse.

### Viidete olekud

| Olek | Tähendus |
| --- | --- |
| `leitud` | sihtpunkt on dokumendis olemas |
| `puudub` | **sihtpunkti pole** – tõenäoline viga |
| `lisa_eraldi` | lisa ei ole samas failis (tavaliselt eraldi dokument) |
| `valine` | viide seadusele või muule aktile (nt `VÕS § 103 lg 2`, `direktiivi artikli 15`) |

### Toetatud vormingud

| Vorming | Tugi |
| --- | --- |
| `.docx` | täielik |
| Digiallkirja konteiner `.asice`, `.bdoc` (ka `.sce`, `.edoc`) | sees olev `.docx` analüüsitakse otse, lahti pakkimata. Kui `.docx` faile on mitu, vali üks võtmega `-sees NIMI` (MCP: `sisemine`). `konteiner FAIL` loetleb sisu. **Allkirjade kehtivust ei kontrollita** – selleks on DigiDoc4. |
| Vana `.ddoc` | ei toeta; ava DigiDoc4-s ja salvesta sisu |
| `.doc` | teisenda: `soffice --headless --convert-to docx fail.doc` |
| PDF (ka konteineri sees) | praegu ei toeta |

Eesti õiguspraktikas on mustandid ja läbirääkimised (jälgitavad muudatused)
Wordis, allkirjastatud lõppversioonid aga sageli konteineris. Kui konteineris
on PDF, küsi Wordi versiooni.

### Piirangud

- Definitsioonide ja viidete tuvastus põhineb mustritel. Ebatavaline
  sõnastus võib jääda leidmata, seega ei tähenda tühi tulemus, et vigu pole.
- `defineerimata` on **heuristiline**: see loetleb kandidaate (nt
  asutuste nimed, isikunimed), mitte kindlaid vigu. Preambul jäetakse välja.
- Käändevormide tuvastus kasutab tüve ja lõppude loendit, mitte
  morfoloogilist analüsaatorit. Harvad tüvemuutused võivad jääda leidmata.
- Korduvate punktinumbrite korral (nt kaks punkti 1.1) antakse hoiatus ja
  viide seotakse esimesega.
- Kui `numbering.xml` puudub või on vigane, antakse hoiatus. Struktuur
  põhineb siis tekstis olevatel numbritel ja pealkirjadel.

## Paigaldamine

Vaja on Go 1.25 või uuemat.

```bash
go install github.com/Ker102/lepinguparser/cmd/lepinguparser@latest
```

või repost:

```bash
go build -o lepinguparser ./cmd/lepinguparser
```

Tulemuseks on üks käivitatav fail, millel pole muid sõltuvusi.

## Kasutamine käsurealt

Vaikimisi väljund on JSON (agentidele). Võti `-t` annab inimloetava teksti.

```bash
lepinguparser kontroll -t leping.docx
lepinguparser sisukord -t -tase 2 leping.docx
lepinguparser sektsioon leping.docx 7.2
lepinguparser termin -t leping.docx "Konfidentsiaalne Teave"
lepinguparser viited -olek puudub leping.docx
lepinguparser kus-viidatakse -t leping.docx 3
lepinguparser konteiner leping.asice
lepinguparser kontroll -t leping.asice              # .docx konteineri seest
lepinguparser kontroll -t -sees Leping.docx leping.asice
```

Väljumiskoodid: `0` korras, `1` viga (nt faili ei saa lugeda või punkti ei
leitud), `2` vale kasutus, `3` kontroll leidis katkiseid viiteid.

## Kasutamine MCP serverina

Server töötab stdio kaudu:

```bash
lepinguparser mcp
```

Claude Code'is:

```bash
claude mcp add lepinguparser -- lepinguparser mcp
```

Claude Desktopi `claude_desktop_config.json`:

```json
{
  "mcpServers": {
    "lepinguparser": {
      "command": "/tee/lepinguparser",
      "args": ["mcp"],
      "env": { "LEPINGUPARSER_JUURED": "/Users/nimi/Dokumendid/Lepingud" }
    }
  }
}
```

Kõik tööriistad võtavad argumendiks `fail` (faili absoluutne tee).

### Juurdepääsu piiramine

Keskkonnamuutuja `LEPINGUPARSER_JUURED` piirab, milliseid kaustu tööriist
lugeda tohib. Kaustade eraldaja on `:` (Windowsis `;`). Kui muutuja on
määramata, on lubatud kõik failid, mida kasutaja saab lugeda. MCP kasutuse
puhul soovitame alati kausta piirata.

## Vead ja parandused

Kui tööriist annab vale tulemuse või vea:

1. Uuenda: `go install github.com/Ker102/lepinguparser/cmd/lepinguparser@latest`.
2. Vaata teadaolevaid probleeme: https://github.com/Ker102/lepinguparser/issues
3. Teata probleemist või saada parandus (fork → haru → pull request).
   Lisa test, mis koostab probleemse struktuuri `internal/testdocx` abil.

**Ära lisa probleemi juurde kliendi lepingut.** Hoidla on avalik.
Kirjelda struktuuri (nt „punkt 3.3 alapunkt (b) nummerdatakse valesti,
kui …“) või koosta testis väike näidis.

## Arendus

```bash
go test ./...
go vet ./...
```

Testid koostavad testdokumendid koodis (`internal/testdocx`), seega pole
repos kliendidokumente. Uue mustri lisamisel lisa test failidesse
`internal/analyys/analyys_test.go` ja `internal/docx/docx_test.go`.

Nummerduse kontrollimiseks päris dokumendi peal saab tulemust võrrelda
LibreOffice'i tekstiekspordiga:

```bash
soffice --headless --convert-to "txt:Text (encoded):UTF8" leping.docx
lepinguparser sisukord -t leping.docx
```

### Ülesehitus

```
cmd/lepinguparser/      käsurida ja MCP käivitus
internal/docx/          .docx lugemine, stiilid, Wordi nummerdusmootor
internal/analyys/       struktuur, definitsioonid, käändevormid, viited, kontroll
internal/teenus/        faili avamine, juurdepääsupiirang, ühised vastused
internal/mcpserver/     MCP tööriistad
internal/testdocx/      testdokumentide koostaja
skills/lepinguparser/   agendi oskus (skill): millal ja kuidas tööriista kasutada
```

## Litsents

Apache License 2.0 koos tingimusega **Commons Clause** (vt `LICENSE` ja
`NOTICE`). Lihtsalt öeldes:

- tööriista võib vabalt kasutada, ka ettevõttes ja kliendi töös;
- seda võib muuta ja edasi jagada, kui `LICENSE` ja `NOTICE` (autori
  märge) jäävad alles;
- tööriista ennast ega sellel põhinevat teenust ei tohi **müüa** (sh tasu
  eest majutada või pakkuda tasulist tuge, mille väärtus tuleneb peamiselt
  sellest tööriistast).

Tegu ei ole OSI mõttes avatud lähtekoodiga litsentsiga ("source-available").
Muudeks kasutusviisideks küsi luba autorilt.
