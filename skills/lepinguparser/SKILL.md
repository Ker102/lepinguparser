---
name: lepinguparser
description: Lepingu .docx või PDF-faili (ka digiallkirja konteineris .asice/.bdoc) struktuuri kontroll lepinguparser MCP tööriistadega – punktide numbrid (Wordi automaatnummerdus), definitsioonid (ka eesti jutumärkideta „edaspidi tellija“), terminite kasutus koos käändevormidega, sisemised ristviited ja katkised viited. Kasuta enne lepingu ülevaatust, kommentaaride kirjutamist või punktide ümbernummerdamist, et teada täpseid punktinumbreid, mis on defineeritud ja kuhu viidatakse. Use for contract review of Word or PDF documents (Estonian), defined terms, cross-references, clause numbering, signed PDFs.
---

# Lepingu struktuur

Tööriistad: `sisukord`, `tekst`, `sektsioon`, `definitsioonid`,
`termini_kasutus`, `defineerimata_terminid`, `viited`, `kus_viidatakse`,
`kontroll`, `konteineri_sisu`, `pdf_teave`. Kõik võtavad `fail` (absoluutne
tee `.docx`, `.pdf` või konteineri failini).

## Töövoog lepingu ülevaatusel

1. **Kontroll esmalt:** `kontroll` annab ülevaate. Katkised viited (olek
   `puudub`), kasutamata ja topelt definitsioonid ning enne defineerimist
   kasutatud terminid on tavaliselt päris vead. Teata need kasutajale koos
   punktinumbritega.
2. **Kandidaadid ei ole vead:** `defineerimata_kandidaadid` on heuristiline
   (asutuste ja isikute nimed jms). Esita need kui „võimalikud defineerimata
   terminid“, mitte kui kindlad vead.
3. **Täpsed numbrid:** viita punktidele `sisukord` või `sektsioon` numbrite
   järgi. Wordi automaatnummerdust ei näe failist tekstina, seega ära
   arva numbreid. Kogu teksti lugemiseks kasuta `tekst` (pika dokumendi
   puhul osade kaupa `alates`/`kuni`).
4. **Enne muudatust:** kui soovitad punkti kustutada, ümber nummerdada või
   termini definitsiooni muuta, kasuta `kus_viidatakse` või
   `termini_kasutus`. Nii näed, kuhu muudatus mõjub.
5. **Jälgitavad muudatused:** tööriist loeb teksti koos lisandustega ja
   ilma kustutusteta ning annab hoiatuse. Ütle kasutajale, et analüüs
   kajastab muudatustega versiooni.

## Vormingud

- `.docx`: otse.
- PDF: otse. Lõigud ja punktinumbrid taastatakse lehe paigutusest; päised,
  jalused ja leheküljenumbrid eemaldatakse. Viita punktidele numbri järgi ja
  lisa vajadusel lehekülg (`lk`), et kasutaja leiaks koha originaalist.
  Loe hoiatusi: tekstita lehed (skannitud, vaja OCR-i), dekodeerimata märgid
  ja „PDF-i on pärast viimast allkirja muudetud“ tuleb kasutajale öelda.
  Kui Wordi versioon on olemas, on see täpsem.
- PDF-i sisse allkirjastatud fail (PAdES): `pdf_teave` näitab allkirjastajat,
  aega ja seda, kas allkiri katab kogu faili. Kehtivust ei kontrollita.
- `.asice` / `.bdoc` digiallkirja konteiner: anna konteiner otse. Sees olev
  `.docx` või PDF analüüsitakse (`.docx` eelistatakse); mitme puhul määra
  `sisemine`. `konteineri_sisu` loetleb faile. Ütle kasutajale, et
  allkirjade kehtivust ei kontrollitud (selleks DigiDoc4).
- `.doc`: teisenda LibreOffice'iga (`soffice --headless --convert-to docx`).
- Skannitud PDF (tekstikihita): tööriist annab vea; vaja on OCR-i.

## Vead

- **Juurdepääsu viga** (`LEPINGUPARSER_JUURED`): fail on väljaspool lubatud
  kausta. Ütle kasutajale, ära ürita mööda minna.
- **Ootamatu viga või ilmselgelt vale tulemus** (nt numbrid ei vasta Wordis
  nähtule): ütle kasutajale, et tööriist võib vajada parandust. Soovita
  uuendada (`go install github.com/Ker102/lepinguparser/cmd/lepinguparser@latest`),
  vaadata https://github.com/Ker102/lepinguparser/issues ning teatada
  probleemist või saata parandus (pull request), kui tal on GitHub.
  **Kliendi lepingut ei tohi probleemi juurde lisada.** Kirjelda struktuuri
  üldiselt.
