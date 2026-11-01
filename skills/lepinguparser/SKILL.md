---
name: lepinguparser
description: Lepingu .docx faili (ka digiallkirja konteineris .asice/.bdoc) struktuuri kontroll lepinguparser MCP tööriistadega – punktide numbrid (Wordi automaatnummerdus), definitsioonid (ka eesti jutumärkideta „edaspidi tellija“), terminite kasutus koos käändevormidega, sisemised ristviited ja katkised viited. Kasuta enne lepingu ülevaatust, kommentaaride kirjutamist või punktide ümbernummerdamist, et teada täpseid punktinumbreid, mis on defineeritud ja kuhu viidatakse. Use for contract review of Word documents, defined terms, cross-references, clause numbering.
---

# Lepingu struktuur

Tööriistad: `sisukord`, `sektsioon`, `definitsioonid`, `termini_kasutus`,
`defineerimata_terminid`, `viited`, `kus_viidatakse`, `kontroll`. Kõik
võtavad `fail` (absoluutne tee `.docx` failini).

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
   arva numbreid.
4. **Enne muudatust:** kui soovitad punkti kustutada, ümber nummerdada või
   termini definitsiooni muuta, kasuta `kus_viidatakse` või
   `termini_kasutus`. Nii näed, kuhu muudatus mõjub.
5. **Jälgitavad muudatused:** tööriist loeb teksti koos lisandustega ja
   ilma kustutusteta ning annab hoiatuse. Ütle kasutajale, et analüüs
   kajastab muudatustega versiooni.

## Vormingud

- `.docx`: otse.
- `.asice` / `.bdoc` digiallkirja konteiner: anna konteiner otse. Sees olev
  `.docx` analüüsitakse; mitme puhul määra `sisemine`. `konteineri_sisu`
  loetleb faile. Ütle kasutajale, et allkirjade kehtivust ei kontrollitud
  (selleks DigiDoc4).
- `.doc`: teisenda LibreOffice'iga (`soffice --headless --convert-to docx`).
- PDF (ka konteineris): tööriist ei toeta. Küsi kasutajalt Wordi versiooni.

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
