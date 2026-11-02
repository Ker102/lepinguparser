# lepinguparser – täielik juhend

Lepingute `.docx` ja PDF-failide struktuurianalüüs eesti ja inglise keeles:
punktide numbrid, definitsioonid, terminite kasutus (koos eesti
käändevormidega) ja sisemised ristviited. Faili võib anda ka digiallkirja
konteineris (`.asice`, `.bdoc`). Töötab käsurealt ja MCP serverina, nii et Claude ja
teised agendid saavad seda tööriistana kasutada.

Tööriist ei saada dokumente kuhugi: kogu analüüs toimub kohalikus arvutis.

## Mida see teeb

| Käsk / MCP tööriist | Mida tagastab |
| --- | --- |
| `sisukord` | punktide numbrid ja pealkirjad, sh Wordi automaatnummerdusest arvutatud numbrid |
| `tekst` | kogu tekst lõikudena koos punktinumbrite ja (PDF-i puhul) lehekülgedega |
| `sektsioon` | ühe punkti (koos alapunktidega) tekst, nt `7.2`, `7.2(a)`, `Lisa 1` |
| `definitsioonid` | defineeritud terminid |
| `termin` / `termini_kasutus` | termini kõik esinemised, sh käändevormid |
| `defineerimata` / `defineerimata_terminid` | suure algustähega sõnad, mis ei vasta ühelegi definitsioonile (kandidaadid) |
| `viited` | sisemised ristviited ja nende olek |
| `kus-viidatakse` / `kus_viidatakse` | kõik kohad, kus viidatakse antud punktile |
| `konteiner` / `konteineri_sisu` | digiallkirja konteineri failid ja allkirjafailide arv |
| `pdf` / `pdf_teave` | PDF-i metaandmed, lehekülgede arv, tekstita (skannitud) lehed ja PDF-i manustatud allkirjad |
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

### PDF

PDF-is ei ole lõike ega Wordi nummerdust, vaid lehele paigutatud märgid.
Tööriist taastab teksti ise (ilma väliste programmideta):

- **Eesti tähed** (õ, ä, ö, ü, š, ž) ja jutumärgid „ “ loetakse fondi
  ToUnicode tabelist, kodeeringust (WinAnsi, MacRoman, `/Differences`) või
  glüüfinimedest. Eraldi joonistatud rõhumärgid (`o` + `˜`) ühendatakse.
- **Read ja lõigud** taastatakse märkide asukohast. Poolitatud sõnad
  ühendatakse (`kokku-` + `leppe` → `kokkulepe`), aga `e-posti` ja
  `IT-süsteem` jäävad sidekriipsuga. Lehtede vahel jätkuv lõik ühendatakse.
- **Päised, jalused ja leheküljenumbrid** (`Lk 2/5`, `2 (5)`) eemaldatakse:
  nii lehe servas korduvad read kui ka Wordi poolt artefaktiks märgitud
  päised. Lisa päis (`Lisa 1`) jäetakse esimesel lehel alles, sest see
  märgib lisa algust. Joonealused märkused ei katkesta lõiku.
- **Punktinumbrid** (`7.2.`, `7.2`, `(a)`, `b)`, `(iii)`) tuvastatakse rea
  algusest. Numbri järjestust kontrollitakse, et rea alguses olev kuupäev
  (`15. jaanuaril`) või summa ei muutuks punktiks.
- **Iga lõigu juures on lehekülg** (`lk`), et punkti saaks originaalist leida.
- **Kaitstud PDF:** omaniku parooliga (kopeerimis- või muutmiskeeluga)
  PDF loetakse (RC4, AES-128, AES-256). Kasutajaparooliga PDF-i jaoks antakse
  selge veateade.
- **PDF-i allkirjad** (PAdES, nt Smart-ID või ID-kaardiga PDF-i sisse
  allkirjastatud): `pdf_teave` näitab allkirjastaja nime, sertifikaadi
  väljaandjat, aega ja seda, kas faili on pärast viimast allkirja muudetud.
  Isikukoodi väljundisse ei panda. **Kehtivust ei kontrollita.**

Täpsust kontrolliti üheksa avaliku Eesti lepingu ja kohtuotsuse peal (Word,
LibreOffice ja muud tootjad): sõnade kokkulangevus `pdftotext`-iga oli
99,3–100 % (erinevused on peamiselt eemaldatud leheküljenumbrid) ja kõik
punktinumbrid tuvastati.

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
| PDF | tekstikihiga PDF; lõigud ja numbrid taastatakse paigutusest (vt eespool) |
| Digiallkirja konteiner `.asice`, `.bdoc` (ka `.sce`, `.edoc`) | sees olev `.docx` või PDF analüüsitakse otse, lahti pakkimata. Eelistatakse `.docx` faili. Kui faile on mitu, vali üks võtmega `-sees NIMI` (MCP: `sisemine`). Konteiner konteineris (topeltallkiri) avatakse. `konteiner FAIL` loetleb sisu. **Allkirjade kehtivust ei kontrollita** – selleks on DigiDoc4. |
| Skannitud PDF (tekstikihita) | ei toeta; tee enne OCR |
| Vana `.ddoc` | ei toeta; ava DigiDoc4-s ja salvesta sisu |
| `.doc` | teisenda: `soffice --headless --convert-to docx fail.doc` |

Eesti õiguspraktikas on mustandid ja läbirääkimised (jälgitavad muudatused)
Wordis, allkirjastatud lõppversioonid aga sageli konteineris PDF-ina. Kui
Wordi versioon on olemas, on see täpsem (nummerdus tuleb otse failist).

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
- PDF-i lõigud ja punktinumbrid on tuletatud paigutusest. Tabelid loetakse
  ridade kaupa, mitmeveeruline paigutus ei ole toetatud ja ebatavaline
  nummerdus (nt numbrita alapunktid) võib jääda tuvastamata. Kontrolli
  olulisi kohti originaalist; abiks on iga lõigu leheküljenumber.

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
lepinguparser kontroll -t leping.pdf
lepinguparser tekst -t leping.pdf                   # tekst lõikudena, lk kaupa
lepinguparser pdf leping.pdf                        # metaandmed ja PDF-i allkirjad
lepinguparser sisukord -t -sees "Lisa 1.pdf" leping.asice
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
   Lisa test, mis koostab probleemse struktuuri `internal/testdocx` või
   `internal/testpdf` abil.

**Ära lisa probleemi juurde kliendi lepingut.** Hoidla on avalik.
Kirjelda struktuuri (nt „punkt 3.3 alapunkt (b) nummerdatakse valesti,
kui …“) või koosta testis väike näidis.

## Arendus

```bash
go test ./...
go vet ./...
```

Testid koostavad testdokumendid koodis (`internal/testdocx`,
`internal/testpdf`), seega pole repos kliendidokumente. Uue mustri lisamisel
lisa test failidesse `internal/analyys/analyys_test.go`,
`internal/docx/docx_test.go` või `internal/pdf/pdf_test.go`.

Nummerduse kontrollimiseks päris dokumendi peal saab tulemust võrrelda
LibreOffice'i tekstiekspordiga:

```bash
soffice --headless --convert-to "txt:Text (encoded):UTF8" leping.docx
lepinguparser sisukord -t leping.docx
```

PDF-i teksti saab võrrelda Poppleri `pdftotext`-iga:

```bash
pdftotext -enc UTF-8 -layout leping.pdf -
lepinguparser tekst -t leping.pdf
```

### Ülesehitus

```
cmd/lepinguparser/      käsurida ja MCP käivitus
internal/docx/          .docx lugemine, stiilid, Wordi nummerdusmootor
internal/pdf/           PDF-i lugemine: xref, filtrid, krüpto, fondid, paigutus, allkirjad
internal/konteiner/     digiallkirja konteinerid (.asice, .bdoc)
internal/analyys/       struktuur, definitsioonid, käändevormid, viited, kontroll
internal/teenus/        faili avamine, juurdepääsupiirang, ühised vastused
internal/mcpserver/     MCP tööriistad
internal/testdocx/      testdokumentide koostaja
internal/testpdf/       test-PDF-ide koostaja
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
