package pdf

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/md5"
	"crypto/rc4"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/binary"
	"fmt"
)

// kruptija dekrüpteerib standardse turvakäsitlejaga (/Standard) kaitstud
// PDF-i tühja kasutajaparooliga. Selline kaitse (ainult omaniku parool, nt
// printimise või kopeerimise keeld) on ametlikes dokumentides tavaline.
type kruptija struct {
	votme   []byte
	aesVoog bool
	aesStr  bool
	identV  bool // vood krüpteerimata
	identS  bool // stringid krüpteerimata
	v5      bool
	encNr   int // /Encrypt sõnastiku objekti number (seda ei dekrüpteerita)
}

var taide = []byte{0x28, 0xBF, 0x4E, 0x5E, 0x4E, 0x75, 0x8A, 0x41, 0x64, 0x00, 0x4E, 0x56, 0xFF, 0xFA, 0x01, 0x08,
	0x2E, 0x2E, 0x00, 0xB6, 0xD0, 0x68, 0x3E, 0x80, 0x2F, 0x0C, 0xA9, 0xFE, 0x64, 0x53, 0x69, 0x7A}

func uusKruptija(f *fail, encObj any) (*kruptija, error) {
	k := &kruptija{encNr: -1}
	if r, ok := encObj.(Ref); ok {
		k.encNr = r.Num
	}
	enc := f.sonastik(encObj)
	if enc == nil {
		return nil, fmt.Errorf("%w: /Encrypt sõnastik puudub", ErrNotPDF)
	}
	if n := f.nimi(enc["Filter"]); n != "Standard" {
		return nil, fmt.Errorf("PDF on krüpteeritud turvakäsitlejaga %q (nt sertifikaadiga), mida ei toetata", n)
	}
	v, _ := taisarv(f.lahenda(enc["V"]))
	r, _ := taisarv(f.lahenda(enc["R"]))
	o := []byte(strOf(f.lahenda(enc["O"])))
	u := []byte(strOf(f.lahenda(enc["U"])))
	p, _ := taisarv(f.lahenda(enc["P"]))
	encMeta := true
	if b, ok := f.lahenda(enc["EncryptMetadata"]).(bool); ok {
		encMeta = b
	}
	var id0 []byte
	if ida := f.massiiv(f.trailer["ID"]); len(ida) > 0 {
		id0 = []byte(strOf(f.lahenda(ida[0])))
	}
	pikkus := 40
	if l, ok := taisarv(f.lahenda(enc["Length"])); ok && l >= 40 && l <= 256 {
		pikkus = l
	}
	if v == 4 || v == 5 {
		cf := f.sonastik(enc["CF"])
		meetod := func(nimi Name) (aesV, ident bool) {
			if nimi == "" || nimi == "Identity" {
				return false, nimi == "Identity" || nimi == ""
			}
			d := f.sonastik(cf[nimi])
			switch f.nimi(d["CFM"]) {
			case "AESV2", "AESV3":
				aesV = true
			case "None":
				ident = true
			}
			if l, ok := taisarv(f.lahenda(d["Length"])); ok && v == 4 {
				if l <= 32 {
					pikkus = l * 8
				} else {
					pikkus = l
				}
			}
			return
		}
		k.aesVoog, k.identV = meetod(f.nimi(enc["StmF"]))
		k.aesStr, k.identS = meetod(f.nimi(enc["StrF"]))
	}
	if v == 5 || r >= 5 {
		k.v5 = true
		k.aesVoog, k.aesStr = !k.identV, !k.identS
		if len(u) < 48 {
			return nil, fmt.Errorf("%w: /U on vigane", ErrNotPDF)
		}
		ue := []byte(strOf(f.lahenda(enc["UE"])))
		var h []byte
		if r >= 6 {
			h = alg2B(u[32:40], nil, nil)
		} else {
			s := sha256.Sum256(u[32:40])
			h = s[:]
		}
		if !bytes.Equal(h, u[:32]) {
			return nil, ErrParool
		}
		var vahe []byte
		if r >= 6 {
			vahe = alg2B(u[40:48], nil, nil)
		} else {
			s := sha256.Sum256(u[40:48])
			vahe = s[:]
		}
		if len(ue) < 32 {
			return nil, fmt.Errorf("%w: /UE on vigane", ErrNotPDF)
		}
		blk, _ := aes.NewCipher(vahe)
		k.votme = make([]byte, 32)
		cipher.NewCBCDecrypter(blk, make([]byte, 16)).CryptBlocks(k.votme, ue[:32])
		return k, nil
	}

	// R2–R4 (RC4 / AES-128)
	n := pikkus / 8
	if r == 2 {
		n = 5
	}
	h := md5.New()
	h.Write(taide)
	h.Write(o)
	var pb [4]byte
	binary.LittleEndian.PutUint32(pb[:], uint32(int32(p)))
	h.Write(pb[:])
	h.Write(id0)
	if r >= 4 && !encMeta {
		h.Write([]byte{0xff, 0xff, 0xff, 0xff})
	}
	key := h.Sum(nil)
	if r >= 3 {
		for i := 0; i < 50; i++ {
			s := md5.Sum(key[:n])
			key = s[:]
		}
	}
	key = key[:n]
	// kontrolli tühja kasutajaparooli
	ok := false
	if r == 2 {
		c, _ := rc4.NewCipher(key)
		out := make([]byte, 32)
		c.XORKeyStream(out, taide)
		ok = bytes.Equal(out, u[:min(32, len(u))])
	} else {
		s := md5.New()
		s.Write(taide)
		s.Write(id0)
		x := s.Sum(nil)
		for i := 0; i < 20; i++ {
			kk := make([]byte, len(key))
			for j := range key {
				kk[j] = key[j] ^ byte(i)
			}
			c, _ := rc4.NewCipher(kk)
			c.XORKeyStream(x, x)
		}
		ok = len(u) >= 16 && bytes.Equal(x[:16], u[:16])
	}
	if !ok {
		return nil, ErrParool
	}
	k.votme = key
	return k, nil
}

func strOf(o any) string {
	s, _ := o.(String)
	return string(s)
}

// alg2B on PDF 2.0 (R6) räsifunktsioon. Tühja parooli korral pwd ja udata on nil.
func alg2B(sool, pwd, udata []byte) []byte {
	in := append(append(append([]byte{}, pwd...), sool...), udata...)
	s := sha256.Sum256(in)
	k := s[:]
	for round := 0; ; round++ {
		k1 := bytes.Repeat(append(append(append([]byte{}, pwd...), k...), udata...), 64)
		blk, _ := aes.NewCipher(k[:16])
		e := make([]byte, len(k1))
		cipher.NewCBCEncrypter(blk, k[16:32]).CryptBlocks(e, k1)
		sum := 0
		for _, c := range e[:16] {
			sum += int(c)
		}
		switch sum % 3 {
		case 0:
			x := sha256.Sum256(e)
			k = x[:]
		case 1:
			x := sha512.Sum384(e)
			k = x[:]
		default:
			x := sha512.Sum512(e)
			k = x[:]
		}
		if round >= 63 && int(e[len(e)-1]) <= round-31 {
			break
		}
	}
	return k[:32]
}

func (k *kruptija) objektiVotme(r Ref, aesV bool) []byte {
	if k.v5 {
		return k.votme
	}
	b := append([]byte{}, k.votme...)
	b = append(b, byte(r.Num), byte(r.Num>>8), byte(r.Num>>16), byte(r.Gen), byte(r.Gen>>8))
	if aesV {
		b = append(b, 's', 'A', 'l', 'T')
	}
	s := md5.Sum(b)
	n := len(k.votme) + 5
	if n > 16 {
		n = 16
	}
	return s[:n]
}

func (k *kruptija) dekrypti(data []byte, r Ref, aesV bool) []byte {
	key := k.objektiVotme(r, aesV)
	if !aesV {
		c, err := rc4.NewCipher(key)
		if err != nil {
			return data
		}
		out := make([]byte, len(data))
		c.XORKeyStream(out, data)
		return out
	}
	if len(data) < 32 || len(data)%16 != 0 {
		return data
	}
	blk, err := aes.NewCipher(key)
	if err != nil {
		return data
	}
	out := make([]byte, len(data)-16)
	cipher.NewCBCDecrypter(blk, data[:16]).CryptBlocks(out, data[16:])
	if p := int(out[len(out)-1]); p >= 1 && p <= 16 && p <= len(out) {
		out = out[:len(out)-p]
	}
	return out
}

func (k *kruptija) voog(data []byte, r Ref) []byte {
	if k.identV {
		return data
	}
	return k.dekrypti(data, r, k.aesVoog)
}

// dekryptiObjekt dekrüpteerib kaudse objekti stringid.
func (f *fail) dekryptiObjekt(o any, nr int) any {
	k := f.kruptija
	if k == nil || k.identS || nr == k.encNr {
		return o
	}
	r := Ref{nr, 0}
	if k2, ok := f.xref[nr]; ok && k2.tyyp == 1 {
		// põlvkonnanumber loetakse päisest
		l := &lekser{b: f.b, pos: k2.nihe}
		l.jataVahele()
		l.sona()
		l.jataVahele()
		fmt.Sscan(string(l.sona()), &r.Gen)
	}
	var kaia func(x any, sug int) any
	kaia = func(x any, sug int) any {
		if sug > 50 {
			return x
		}
		switch v := x.(type) {
		case String:
			return String(k.dekrypti([]byte(v), r, k.aesStr))
		case Array:
			for i := range v {
				v[i] = kaia(v[i], sug+1)
			}
		case Dict:
			// allkirja /Contents ei ole krüpteeritud
			if v["Type"] == Name("Sig") || v["ByteRange"] != nil {
				for key, val := range v {
					if key != "Contents" {
						v[key] = kaia(val, sug+1)
					}
				}
				return v
			}
			for key, val := range v {
				v[key] = kaia(val, sug+1)
			}
		case *Stream:
			for key, val := range v.Dict {
				v.Dict[key] = kaia(val, sug+1)
			}
		}
		return x
	}
	return kaia(o, 0)
}
