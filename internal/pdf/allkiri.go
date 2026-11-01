package pdf

import (
	"crypto/x509"
	"encoding/asn1"
	"strings"
)

// Allkiri on PDF-i sisse manustatud digiallkiri (PAdES / PKCS#7) või
// dokumendi ajatempel. Kehtivust ei kontrollita.
type Allkiri struct {
	Vali          string `json:"vali,omitempty"`
	Tyyp          string `json:"tyyp"`
	Allkirjastaja string `json:"allkirjastaja,omitempty"`
	Valjaandja    string `json:"valjaandja,omitempty"`
	Aeg           string `json:"aeg,omitempty"`
	Pohjus        string `json:"pohjus,omitempty"`
	Asukoht       string `json:"asukoht,omitempty"`
	// KatabKogu näitab, kas allkiri katab kogu faili. Kui ei, lisati faili
	// pärast allkirjastamist muudatusi (nt järgmine allkiri või muudatus).
	KatabKogu bool `json:"katab_kogu_faili"`
	bait      int
}

func (f *fail) allkirjad() []Allkiri {
	juur := f.sonastik(f.trailer["Root"])
	if juur == nil {
		return nil
	}
	af := f.sonastik(juur["AcroForm"])
	if af == nil {
		return nil
	}
	var out []Allkiri
	nahtud := map[any]bool{}
	var kaia func(o any, nimi string, ft Name, sug int)
	kaia = func(o any, nimi string, ft Name, sug int) {
		if sug > 32 || len(out) > 1000 {
			return
		}
		if r, ok := o.(Ref); ok {
			if nahtud[r] {
				return
			}
			nahtud[r] = true
		}
		d := f.sonastik(o)
		if d == nil {
			return
		}
		if t := pdfDoc(strOf(f.lahenda(d["T"]))); t != "" {
			if nimi != "" {
				nimi += "."
			}
			nimi += t
		}
		if n := f.nimi(d["FT"]); n != "" {
			ft = n
		}
		for _, k := range f.massiiv(d["Kids"]) {
			kaia(k, nimi, ft, sug+1)
		}
		if ft != "Sig" {
			return
		}
		v := f.sonastik(d["V"])
		if v == nil {
			return // allkirjastamata väli
		}
		out = append(out, f.loeAllkiri(v, nimi))
	}
	for _, x := range f.massiiv(af["Fields"]) {
		kaia(x, "", "", 0)
	}
	// järjesta faili järjekorras (ByteRange lõpu järgi)
	for i := 1; i < len(out); i++ {
		for j := i; j > 0 && out[j].bait < out[j-1].bait; j-- {
			out[j], out[j-1] = out[j-1], out[j]
		}
	}
	return out
}

func (f *fail) loeAllkiri(v Dict, vali string) Allkiri {
	a := Allkiri{Vali: vali}
	s := func(k Name) string { return strings.TrimSpace(pdfDoc(strOf(f.lahenda(v[k])))) }
	alam := f.nimi(v["SubFilter"])
	switch {
	case f.nimi(v["Type"]) == "DocTimeStamp" || alam == "ETSI.RFC3161":
		a.Tyyp = "ajatempel"
	case alam == "ETSI.CAdES.detached":
		a.Tyyp = "PAdES"
	case alam != "":
		a.Tyyp = string(alam)
	default:
		a.Tyyp = "tundmatu"
	}
	a.Allkirjastaja = s("Name")
	a.Aeg = pdfKuupaev(s("M"))
	a.Pohjus, a.Asukoht = s("Reason"), s("Location")
	if br := f.massiiv(v["ByteRange"]); len(br) == 4 {
		var n [4]int
		for i := range n {
			n[i], _ = taisarv(f.lahenda(br[i]))
		}
		a.bait = n[2] + n[3]
		a.KatabKogu = n[0] == 0 && a.bait >= len(f.b)-2 // lõpus võib olla reavahetus
		if !a.KatabKogu && a.bait < len(f.b) {
			// faili lõpus ainult tühikud või %%EOF kordus
			saba := strings.TrimSpace(strings.ReplaceAll(string(f.b[a.bait:]), "%%EOF", ""))
			a.KatabKogu = saba == ""
		}
	}
	if nimi, valj := sertifikaadist([]byte(strOf(f.lahenda(v["Contents"])))); nimi != "" || valj != "" {
		if nimi != "" {
			a.Allkirjastaja = nimi
		}
		a.Valjaandja = valj
	}
	return a
}

type contentInfo struct {
	Tyyp asn1.ObjectIdentifier
	Sisu asn1.RawValue `asn1:"explicit,optional,tag:0"`
}

type signedData struct {
	Versioon      int
	Algoritmid    asn1.RawValue
	Sisu          asn1.RawValue
	Sertifikaadid asn1.RawValue `asn1:"optional,tag:0"`
	Muu           asn1.RawValue `asn1:"optional"`
	Muu2          asn1.RawValue `asn1:"optional"`
}

// sertifikaadist loeb PKCS#7 / CMS allkirjast allkirjastaja nime ja
// sertifikaadi väljaandja. Eesti ID-kaardi sertifikaadis on nimi kujul
// "PEREKONNANIMI,EESNIMI,isikukood"; tagastatakse ainult nimi.
func sertifikaadist(der []byte) (string, string) {
	var ci contentInfo
	if _, err := asn1.Unmarshal(der, &ci); err != nil || len(ci.Sisu.FullBytes) == 0 {
		return "", ""
	}
	var sd signedData
	if _, err := asn1.Unmarshal(ci.Sisu.Bytes, &sd); err != nil {
		if _, err := asn1.Unmarshal(ci.Sisu.FullBytes, &sd); err != nil {
			return "", ""
		}
	}
	if len(sd.Sertifikaadid.Bytes) == 0 {
		return "", ""
	}
	certs, err := x509.ParseCertificates(sd.Sertifikaadid.Bytes)
	if err != nil || len(certs) == 0 {
		return "", ""
	}
	// lõppkasutaja sertifikaat: ei ole CA ega teise sertifikaadi väljaandja
	var leht *x509.Certificate
	for _, c := range certs {
		if c.IsCA {
			continue
		}
		valjaandja := false
		for _, o := range certs {
			if o != c && string(o.RawIssuer) == string(c.RawSubject) {
				valjaandja = true
			}
		}
		if !valjaandja {
			leht = c
			break
		}
	}
	if leht == nil {
		leht = certs[0]
	}
	nimi := strings.TrimSpace(strings.Join(atribuudid(leht, asn1.ObjectIdentifier{2, 5, 4, 42}), " ") + " " + strings.Join(atribuudid(leht, asn1.ObjectIdentifier{2, 5, 4, 4}), " "))
	if nimi == "" {
		nimi = leht.Subject.CommonName
		if osad := strings.Split(nimi, ","); len(osad) == 3 && strings.Trim(osad[2], "0123456789") == "" {
			nimi = strings.TrimSpace(osad[1]) + " " + strings.TrimSpace(osad[0])
		}
	}
	valj := leht.Issuer.CommonName
	if valj == "" && len(leht.Issuer.Organization) > 0 {
		valj = leht.Issuer.Organization[0]
	}
	return nimi, valj
}

func atribuudid(c *x509.Certificate, oid asn1.ObjectIdentifier) []string {
	var out []string
	for _, n := range c.Subject.Names {
		if n.Type.Equal(oid) {
			if s, ok := n.Value.(string); ok {
				out = append(out, s)
			}
		}
	}
	return out
}
