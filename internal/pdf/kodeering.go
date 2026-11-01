package pdf

import (
	"strconv"
	"strings"
)

// winAnsi on WinAnsiEncoding (cp1252). Eesti tähed Š š Ž ž on 0x8A, 0x9A,
// 0x8E, 0x9E; Õ õ Ä ä Ö ö Ü ü on Latin-1 osas.
var winAnsi = func() [256]rune {
	var t [256]rune
	for i := 0; i < 256; i++ {
		t[i] = rune(i)
	}
	ulemine := []rune{0x20AC, 0, 0x201A, 0x0192, 0x201E, 0x2026, 0x2020, 0x2021, 0x02C6, 0x2030, 0x0160, 0x2039, 0x0152, 0, 0x017D, 0,
		0, 0x2018, 0x2019, 0x201C, 0x201D, 0x2022, 0x2013, 0x2014, 0x02DC, 0x2122, 0x0161, 0x203A, 0x0153, 0, 0x017E, 0x0178}
	copy(t[0x80:0xA0], ulemine)
	for i := 0; i < 32; i++ {
		t[i] = 0
	}
	t[0xAD] = 0x2D // pehme poolitus kuvatakse sidekriipsuna
	return t
}()

var macRoman = func() [256]rune {
	var t [256]rune
	for i := 32; i < 127; i++ {
		t[i] = rune(i)
	}
	u := []rune{0xC4, 0xC5, 0xC7, 0xC9, 0xD1, 0xD6, 0xDC, 0xE1, 0xE0, 0xE2, 0xE4, 0xE3, 0xE5, 0xE7, 0xE9, 0xE8,
		0xEA, 0xEB, 0xED, 0xEC, 0xEE, 0xEF, 0xF1, 0xF3, 0xF2, 0xF4, 0xF6, 0xF5, 0xFA, 0xF9, 0xFB, 0xFC,
		0x2020, 0xB0, 0xA2, 0xA3, 0xA7, 0x2022, 0xB6, 0xDF, 0xAE, 0xA9, 0x2122, 0xB4, 0xA8, 0x2260, 0xC6, 0xD8,
		0x221E, 0xB1, 0x2264, 0x2265, 0xA5, 0xB5, 0x2202, 0x2211, 0x220F, 0x3C0, 0x222B, 0xAA, 0xBA, 0x3A9, 0xE6, 0xF8,
		0xBF, 0xA1, 0xAC, 0x221A, 0x192, 0x2248, 0x2206, 0xAB, 0xBB, 0x2026, 0xA0, 0xC0, 0xC3, 0xD5, 0x152, 0x153,
		0x2013, 0x2014, 0x201C, 0x201D, 0x2018, 0x2019, 0xF7, 0x25CA, 0xFF, 0x178, 0x2044, 0x20AC, 0x2039, 0x203A, 0xFB01, 0xFB02,
		0x2021, 0xB7, 0x201A, 0x201E, 0x2030, 0xC2, 0xCA, 0xC1, 0xCB, 0xC8, 0xCD, 0xCE, 0xCF, 0xCC, 0xD3, 0xD4,
		0xF8FF, 0xD2, 0xDA, 0xDB, 0xD9, 0x131, 0x2C6, 0x2DC, 0xAF, 0x2D8, 0x2D9, 0x2DA, 0xB8, 0x2DD, 0x2DB, 0x2C7}
	copy(t[0x80:], u)
	return t
}()

// standard on Adobe StandardEncoding (erinevused ASCII-st ja ülemine pool).
var standard = func() [256]rune {
	var t [256]rune
	for i := 32; i < 127; i++ {
		t[i] = rune(i)
	}
	t['\''] = 0x2019
	t['`'] = 0x2018
	m := map[int]rune{0xA1: 0xA1, 0xA2: 0xA2, 0xA3: 0xA3, 0xA4: 0x2044, 0xA5: 0xA5, 0xA6: 0x192, 0xA7: 0xA7, 0xA8: 0xA4,
		0xA9: '\'', 0xAA: 0x201C, 0xAB: 0xAB, 0xAC: 0x2039, 0xAD: 0x203A, 0xAE: 0xFB01, 0xAF: 0xFB02, 0xB1: 0x2013,
		0xB2: 0x2020, 0xB3: 0x2021, 0xB4: 0xB7, 0xB6: 0xB6, 0xB7: 0x2022, 0xB8: 0x201A, 0xB9: 0x201E, 0xBA: 0x201D,
		0xBB: 0xBB, 0xBC: 0x2026, 0xBD: 0x2030, 0xBF: 0xBF, 0xC1: '`', 0xC2: 0xB4, 0xC3: 0x2C6, 0xC4: 0x2DC, 0xC5: 0xAF,
		0xC6: 0x2D8, 0xC7: 0x2D9, 0xC8: 0xA8, 0xCA: 0x2DA, 0xCB: 0xB8, 0xCD: 0x2DD, 0xCE: 0x2DB, 0xCF: 0x2C7, 0xD0: 0x2014,
		0xE1: 0xC6, 0xE3: 0xAA, 0xE8: 0x141, 0xE9: 0xD8, 0xEA: 0x152, 0xEB: 0xBA, 0xF1: 0xE6, 0xF5: 0x131, 0xF8: 0x142,
		0xF9: 0xF8, 0xFA: 0x153, 0xFB: 0xDF}
	for k, v := range m {
		t[k] = v
	}
	return t
}()

// pdfDoc dekodeerib PDFDocEncoding'u või UTF-16BE stringi (metaandmed,
// allkirja nimi).
func pdfDoc(s string) string {
	b := []byte(s)
	if len(b) >= 2 && b[0] == 0xFE && b[1] == 0xFF {
		return utf16be(b[2:])
	}
	if len(b) >= 3 && b[0] == 0xEF && b[1] == 0xBB && b[2] == 0xBF {
		return string(b[3:])
	}
	var sb strings.Builder
	for _, c := range b {
		switch {
		case c >= 0x80 && c <= 0x9F:
			// PDFDocEncoding erineb siin cp1252-st; olulisemad
			r := map[byte]rune{0x80: 0x2022, 0x81: 0x2020, 0x82: 0x2021, 0x83: 0x2026, 0x84: 0x2014, 0x85: 0x2013,
				0x86: 0x192, 0x87: 0x2044, 0x88: 0x2039, 0x89: 0x203A, 0x8A: 0x2212, 0x8B: 0x2030, 0x8C: 0x201E,
				0x8D: 0x201C, 0x8E: 0x201D, 0x8F: 0x2018, 0x90: 0x2019, 0x91: 0x201A, 0x92: 0x2122, 0x93: 0xFB01,
				0x94: 0xFB02, 0x95: 0x141, 0x96: 0x152, 0x97: 0x160, 0x98: 0x178, 0x99: 0x17D, 0x9A: 0x131,
				0x9B: 0x142, 0x9C: 0x153, 0x9D: 0x161, 0x9E: 0x17E}[c]
			if r != 0 {
				sb.WriteRune(r)
			}
		default:
			sb.WriteRune(rune(c))
		}
	}
	return sb.String()
}

func utf16be(b []byte) string {
	var out []rune
	for i := 0; i+1 < len(b); i += 2 {
		c := rune(b[i])<<8 | rune(b[i+1])
		if c >= 0xD800 && c < 0xDC00 && i+3 < len(b) {
			c2 := rune(b[i+2])<<8 | rune(b[i+3])
			if c2 >= 0xDC00 && c2 < 0xE000 {
				out = append(out, 0x10000+(c-0xD800)<<10+(c2-0xDC00))
				i += 2
				continue
			}
		}
		out = append(out, c)
	}
	return string(out)
}

// glyfinimed on Adobe Glyph Listi osa: ladina tähed (sh eesti ja teiste
// Euroopa keelte tähed), kirjavahemärgid ja levinumad sümbolid.
var glyfinimed = func() map[string]rune {
	m := map[string]rune{}
	for c := 'a'; c <= 'z'; c++ {
		m[string(c)] = c
		m[string(c-32)] = c - 32
	}
	numbrid := []string{"zero", "one", "two", "three", "four", "five", "six", "seven", "eight", "nine"}
	for i, n := range numbrid {
		m[n] = rune('0' + i)
	}
	const tabel = `space 20;exclam 21;quotedbl 22;numbersign 23;dollar 24;percent 25;ampersand 26;quotesingle 27;
parenleft 28;parenright 29;asterisk 2A;plus 2B;comma 2C;hyphen 2D;period 2E;slash 2F;colon 3A;semicolon 3B;
less 3C;equal 3D;greater 3E;question 3F;at 40;bracketleft 5B;backslash 5C;bracketright 5D;asciicircum 5E;
underscore 5F;grave 60;braceleft 7B;bar 7C;braceright 7D;asciitilde 7E;quoteleft 2018;quoteright 2019;
quotesinglbase 201A;quotedblleft 201C;quotedblright 201D;quotedblbase 201E;guilsinglleft 2039;guilsinglright 203A;
guillemotleft AB;guillemotright BB;guillemetleft AB;guillemetright BB;endash 2013;emdash 2014;bullet 2022;
ellipsis 2026;dagger 2020;daggerdbl 2021;perthousand 2030;trademark 2122;Euro 20AC;euro 20AC;fi FB01;fl FB02;
ff FB00;ffi FB03;ffl FB04;minus 2212;periodcentered B7;middot B7;section A7;paragraph B6;copyright A9;
registered AE;degree B0;plusminus B1;multiply D7;divide F7;nbspace A0;nonbreakingspace A0;sfthyphen AD;
softhyphen AD;exclamdown A1;cent A2;sterling A3;currency A4;yen A5;brokenbar A6;dieresis A8;ordfeminine AA;
logicalnot AC;macron AF;twosuperior B2;threesuperior B3;acute B4;mu B5;cedilla B8;onesuperior B9;
ordmasculine BA;onequarter BC;onehalf BD;threequarters BE;questiondown BF;circumflex 2C6;tilde 2DC;caron 2C7;
breve 2D8;dotaccent 2D9;ring 2DA;ogonek 2DB;hungarumlaut 2DD;dotlessi 131;florin 192;fraction 2044;
numero 2116;afii61352 2116;estimated 212E;lozenge 25CA;notequal 2260;lessequal 2264;greaterequal 2265;
infinity 221E;partialdiff 2202;summation 2211;product 220F;integral 222B;radical 221A;approxequal 2248;
Delta 2206;Omega 3A9;pi 3C0;arrowright 2192;arrowleft 2190;checkmark 2713;square 25A1;filledbox 25A0;
uni00A0 A0;Agrave C0;Aacute C1;Acircumflex C2;Atilde C3;Adieresis C4;Aring C5;AE C6;Ccedilla C7;Egrave C8;
Eacute C9;Ecircumflex CA;Edieresis CB;Igrave CC;Iacute CD;Icircumflex CE;Idieresis CF;Eth D0;Ntilde D1;
Ograve D2;Oacute D3;Ocircumflex D4;Otilde D5;Odieresis D6;Oslash D8;Ugrave D9;Uacute DA;Ucircumflex DB;
Udieresis DC;Yacute DD;Thorn DE;germandbls DF;agrave E0;aacute E1;acircumflex E2;atilde E3;adieresis E4;
aring E5;ae E6;ccedilla E7;egrave E8;eacute E9;ecircumflex EA;edieresis EB;igrave EC;iacute ED;
icircumflex EE;idieresis EF;eth F0;ntilde F1;ograve F2;oacute F3;ocircumflex F4;otilde F5;odieresis F6;
oslash F8;ugrave F9;uacute FA;ucircumflex FB;udieresis FC;yacute FD;thorn FE;ydieresis FF;Ydieresis 178;
Amacron 100;amacron 101;Aogonek 104;aogonek 105;Cacute 106;cacute 107;Ccaron 10C;ccaron 10D;Dcaron 10E;
dcaron 10F;Dcroat 110;dcroat 111;Emacron 112;emacron 113;Edotaccent 116;edotaccent 117;Eogonek 118;
eogonek 119;Ecaron 11A;ecaron 11B;Gcommaaccent 122;gcommaaccent 123;Imacron 12A;imacron 12B;Iogonek 12E;
iogonek 12F;Idotaccent 130;Kcommaaccent 136;kcommaaccent 137;Lacute 139;lacute 13A;Lcommaaccent 13B;
lcommaaccent 13C;Lslash 141;lslash 142;Nacute 143;nacute 144;Ncommaaccent 145;ncommaaccent 146;Ncaron 147;
ncaron 148;Omacron 14C;omacron 14D;Ohungarumlaut 150;ohungarumlaut 151;OE 152;oe 153;Racute 154;racute 155;
Rcaron 158;rcaron 159;Sacute 15A;sacute 15B;Scedilla 15E;scedilla 15F;Scaron 160;scaron 161;Tcaron 164;
tcaron 165;Umacron 16A;umacron 16B;Uring 16E;uring 16F;Uhungarumlaut 170;uhungarumlaut 171;Uogonek 172;
uogonek 173;Zacute 179;zacute 17A;Zdotaccent 17B;zdotaccent 17C;Zcaron 17D;zcaron 17E;Gbreve 11E;gbreve 11F;
Scommaaccent 218;scommaaccent 219;Tcommaaccent 21A;tcommaaccent 21B`
	for _, kirje := range strings.Split(strings.ReplaceAll(tabel, "\n", ""), ";") {
		f := strings.Fields(kirje)
		if len(f) != 2 {
			continue
		}
		v, err := strconv.ParseUint(f[1], 16, 32)
		if err == nil {
			m[f[0]] = rune(v)
		}
	}
	return m
}()

// glyfistTeksti teisendab glüüfi nime tekstiks: "otilde" -> "õ",
// "uni00F5" -> "õ", "u1F600", "f_i" -> "fi". Tundmatu nime korral "".
func glyfistTeksti(nimi string) string {
	if i := strings.IndexByte(nimi, '.'); i > 0 {
		nimi = nimi[:i] // "a.sc", "one.oldstyle"
	}
	if r, ok := glyfinimed[nimi]; ok {
		return string(r)
	}
	if strings.Contains(nimi, "_") {
		var sb strings.Builder
		for _, osa := range strings.Split(nimi, "_") {
			t := glyfistTeksti(osa)
			if t == "" {
				return ""
			}
			sb.WriteString(t)
		}
		return sb.String()
	}
	if strings.HasPrefix(nimi, "uni") && len(nimi) >= 7 && (len(nimi)-3)%4 == 0 {
		var out []rune
		for i := 3; i+4 <= len(nimi); i += 4 {
			v, err := strconv.ParseUint(nimi[i:i+4], 16, 32)
			if err != nil {
				return ""
			}
			out = append(out, rune(v))
		}
		return string(out)
	}
	if len(nimi) >= 5 && len(nimi) <= 7 && nimi[0] == 'u' {
		if v, err := strconv.ParseUint(nimi[1:], 16, 32); err == nil {
			return string(rune(v))
		}
	}
	return ""
}
