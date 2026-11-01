package pdf

import (
	"bytes"
	"compress/flate"
	"compress/zlib"
	"errors"
	"fmt"
	"io"
)

// MaxVoog piirab ühe lahtipakitud voo suurust (pommi kaitse).
const MaxVoog = 200 << 20

// dekodeeri tagastab voo lahtipakitud sisu. Pildifiltrid (DCT, JPX, CCITT,
// JBIG2) jäetakse vahele, sest teksti jaoks neid vaja ei ole.
func (f *fail) dekodeeri(s *Stream) ([]byte, error) {
	data := s.Raw
	if f.kruptija != nil && s.Dict["Type"] != Name("XRef") && !onIdentityCrypt(f, s.Dict) {
		data = f.kruptija.voog(data, s.ref)
	}
	var filtrid []Name
	var parmid []Dict
	switch v := f.lahenda(s.Dict["Filter"]).(type) {
	case Name:
		filtrid = []Name{v}
		parmid = []Dict{f.sonastik(s.Dict["DecodeParms"])}
	case Array:
		pa := f.massiiv(s.Dict["DecodeParms"])
		for i, x := range v {
			filtrid = append(filtrid, f.nimi(x))
			var p Dict
			if i < len(pa) {
				p = f.sonastik(pa[i])
			}
			parmid = append(parmid, p)
		}
	}
	var err error
	for i, fn := range filtrid {
		switch fn {
		case "FlateDecode", "Fl":
			data, err = flateLahti(data)
			if err == nil {
				data, err = ennustaja(data, parmid[i])
			}
		case "ASCIIHexDecode", "AHx":
			l := &lekser{b: append(append([]byte{}, data...), '>')}
			data = []byte(l.hex())
		case "ASCII85Decode", "A85":
			data, err = ascii85Lahti(data)
		case "LZWDecode", "LZW":
			vara := 1
			if v, ok := taisarv(parmid[i]["EarlyChange"]); ok {
				vara = v
			}
			data, err = lzwLahti(data, vara)
			if err == nil {
				data, err = ennustaja(data, parmid[i])
			}
		case "RunLengthDecode", "RL":
			data = runLengthLahti(data)
		case "Crypt":
		default:
			return nil, fmt.Errorf("filtrit %s ei toetata", fn)
		}
		if err != nil {
			return nil, err
		}
	}
	return data, nil
}

func onIdentityCrypt(f *fail, d Dict) bool {
	fl, ok := f.lahenda(d["Filter"]).(Array)
	if !ok || len(fl) == 0 || f.nimi(fl[0]) != "Crypt" {
		return false
	}
	return true
}

func flateLahti(b []byte) ([]byte, error) {
	var r io.Reader
	if zr, err := zlib.NewReader(bytes.NewReader(b)); err == nil {
		r = zr
	} else {
		r = flate.NewReader(bytes.NewReader(b))
	}
	out, err := io.ReadAll(io.LimitReader(r, MaxVoog+1))
	if len(out) > MaxVoog {
		return nil, errors.New("voog on liiga suur")
	}
	if err != nil {
		// katkenud või vigase kontrollsummaga voog: kasuta loetud osa
		if len(out) > 0 {
			return out, nil
		}
		// mõni tootja jätab zlib päise ära
		out2, err2 := io.ReadAll(io.LimitReader(flate.NewReader(bytes.NewReader(b)), MaxVoog))
		if len(out2) > 0 {
			return out2, nil
		}
		if err2 != nil {
			return nil, fmt.Errorf("FlateDecode: %v", err)
		}
	}
	return out, nil
}

// ennustaja rakendab PNG/TIFF ennustaja (/Predictor) pöördteisenduse.
func ennustaja(b []byte, p Dict) ([]byte, error) {
	pr, _ := taisarv(p["Predictor"])
	if pr <= 1 {
		return b, nil
	}
	kolumnid, varvid, bitid := 1, 1, 8
	if v, ok := taisarv(p["Columns"]); ok && v > 0 {
		kolumnid = v
	}
	if v, ok := taisarv(p["Colors"]); ok && v > 0 {
		varvid = v
	}
	if v, ok := taisarv(p["BitsPerComponent"]); ok && v > 0 {
		bitid = v
	}
	bpp := (varvid*bitid + 7) / 8
	rida := (kolumnid*varvid*bitid + 7) / 8
	if pr == 2 {
		if bitid != 8 {
			return b, nil
		}
		out := append([]byte{}, b...)
		for r := 0; r+rida <= len(out); r += rida {
			for i := bpp; i < rida; i++ {
				out[r+i] += out[r+i-bpp]
			}
		}
		return out, nil
	}
	var out []byte
	eel := make([]byte, rida)
	for pos := 0; pos < len(b); pos += rida + 1 {
		t := b[pos]
		lopp := pos + 1 + rida
		if lopp > len(b) {
			lopp = len(b)
		}
		cur := make([]byte, rida)
		copy(cur, b[pos+1:lopp])
		for i := 0; i < rida; i++ {
			var a, c byte
			if i >= bpp {
				a = cur[i-bpp]
				c = eel[i-bpp]
			}
			u := eel[i]
			switch t {
			case 1:
				cur[i] += a
			case 2:
				cur[i] += u
			case 3:
				cur[i] += byte((int(a) + int(u)) / 2)
			case 4:
				cur[i] += paeth(a, u, c)
			}
		}
		out = append(out, cur...)
		eel = cur
	}
	return out, nil
}

func paeth(a, b, c byte) byte {
	p := int(a) + int(b) - int(c)
	pa, pb, pc := abs(p-int(a)), abs(p-int(b)), abs(p-int(c))
	if pa <= pb && pa <= pc {
		return a
	}
	if pb <= pc {
		return b
	}
	return c
}

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

func ascii85Lahti(b []byte) ([]byte, error) {
	var out []byte
	var grupp [5]byte
	n := 0
	b = bytes.TrimPrefix(bytes.TrimSpace(b), []byte("<~"))
	for _, c := range b {
		if onTyhik(c) {
			continue
		}
		if c == '~' {
			break
		}
		if c == 'z' && n == 0 {
			out = append(out, 0, 0, 0, 0)
			continue
		}
		if c < '!' || c > 'u' {
			return nil, errors.New("ASCII85Decode: vigane märk")
		}
		grupp[n] = c - '!'
		n++
		if n == 5 {
			v := uint32(0)
			for _, g := range grupp {
				v = v*85 + uint32(g)
			}
			out = append(out, byte(v>>24), byte(v>>16), byte(v>>8), byte(v))
			n = 0
		}
	}
	if n > 1 {
		for i := n; i < 5; i++ {
			grupp[i] = 84
		}
		v := uint32(0)
		for _, g := range grupp {
			v = v*85 + uint32(g)
		}
		tmp := []byte{byte(v >> 24), byte(v >> 16), byte(v >> 8), byte(v)}
		out = append(out, tmp[:n-1]...)
	}
	return out, nil
}

func runLengthLahti(b []byte) []byte {
	var out []byte
	for i := 0; i < len(b); {
		n := int(b[i])
		i++
		switch {
		case n == 128:
			return out
		case n < 128:
			lopp := i + n + 1
			if lopp > len(b) {
				lopp = len(b)
			}
			out = append(out, b[i:lopp]...)
			i = lopp
		default:
			if i < len(b) {
				out = append(out, bytes.Repeat([]byte{b[i]}, 257-n)...)
				i++
			}
		}
	}
	return out
}

// lzwLahti on PDF-i LZW (Go standardteegi lzw ei toeta EarlyChange'i).
func lzwLahti(b []byte, vara int) ([]byte, error) {
	var out []byte
	sonastik := make([][]byte, 0, 4096)
	reset := func() {
		sonastik = sonastik[:0]
		for i := 0; i < 256; i++ {
			sonastik = append(sonastik, []byte{byte(i)})
		}
		sonastik = append(sonastik, nil, nil) // 256 = clear, 257 = EOD
	}
	reset()
	laius := 9
	var bitid uint32
	nbitid := 0
	var eel []byte
	for i := 0; i < len(b); {
		for nbitid < laius && i < len(b) {
			bitid = bitid<<8 | uint32(b[i])
			nbitid += 8
			i++
		}
		if nbitid < laius {
			break
		}
		kood := int(bitid>>(nbitid-laius)) & (1<<laius - 1)
		nbitid -= laius
		switch {
		case kood == 256:
			reset()
			laius = 9
			eel = nil
			continue
		case kood == 257:
			return out, nil
		}
		var kirje []byte
		if kood < len(sonastik) && sonastik[kood] != nil {
			kirje = sonastik[kood]
		} else if kood == len(sonastik) && eel != nil {
			kirje = append(append([]byte{}, eel...), eel[0])
		} else {
			return out, errors.New("LZWDecode: vigane kood")
		}
		out = append(out, kirje...)
		if len(out) > MaxVoog {
			return nil, errors.New("voog on liiga suur")
		}
		if eel != nil && len(sonastik) < 4096 {
			sonastik = append(sonastik, append(append([]byte{}, eel...), kirje[0]))
		}
		eel = kirje
		if len(sonastik)+vara >= 1<<laius && laius < 12 {
			laius++
		}
	}
	return out, nil
}
