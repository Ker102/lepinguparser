package pdf

import (
	"strings"
	"unicode"
)

// Standardfontide (PDF-i 14 põhifonti) laiused ASCII märkidele 32–126
// tuhandikes em-ist (Adobe AFM failidest). Rõhumärgiga tähe laius on sama
// mis põhitähel (õ = o).
var (
	helvetica = []int{278, 278, 355, 556, 556, 889, 667, 191, 333, 333, 389, 584, 278, 333, 278, 278,
		556, 556, 556, 556, 556, 556, 556, 556, 556, 556, 278, 278, 584, 584, 584, 556, 1015,
		667, 667, 722, 722, 667, 611, 778, 722, 278, 500, 667, 556, 833, 722, 778, 667, 778, 722, 667, 611, 722, 667, 944, 667, 667, 611,
		278, 278, 278, 469, 556, 333,
		556, 556, 500, 556, 556, 278, 556, 556, 222, 222, 500, 222, 833, 556, 556, 556, 556, 333, 500, 278, 556, 500, 722, 500, 500, 500,
		334, 260, 334, 584}
	helveticaPaks = []int{278, 333, 474, 556, 556, 889, 722, 238, 333, 333, 389, 584, 278, 333, 278, 278,
		556, 556, 556, 556, 556, 556, 556, 556, 556, 556, 333, 333, 584, 584, 584, 611, 975,
		722, 722, 722, 722, 667, 611, 778, 722, 278, 556, 722, 611, 833, 722, 778, 667, 778, 722, 667, 611, 722, 667, 944, 667, 667, 611,
		333, 278, 333, 584, 556, 333,
		556, 611, 556, 611, 556, 333, 611, 611, 278, 278, 556, 278, 889, 611, 611, 611, 611, 389, 556, 333, 611, 556, 778, 556, 556, 500,
		389, 280, 389, 584}
	times = []int{250, 333, 408, 500, 500, 833, 778, 180, 333, 333, 500, 564, 250, 333, 250, 278,
		500, 500, 500, 500, 500, 500, 500, 500, 500, 500, 278, 278, 564, 564, 564, 444, 921,
		722, 667, 667, 722, 611, 556, 722, 722, 333, 389, 722, 611, 889, 722, 722, 556, 722, 667, 556, 611, 722, 722, 944, 722, 722, 611,
		333, 278, 333, 469, 500, 333,
		444, 500, 444, 500, 444, 333, 500, 500, 278, 278, 500, 278, 778, 500, 500, 500, 500, 333, 389, 278, 500, 500, 722, 500, 500, 444,
		480, 200, 480, 541}
	timesPaks = []int{250, 333, 555, 500, 500, 1000, 833, 278, 333, 333, 500, 570, 250, 333, 250, 278,
		500, 500, 500, 500, 500, 500, 500, 500, 500, 500, 333, 333, 570, 570, 570, 500, 930,
		722, 667, 722, 722, 667, 611, 778, 778, 389, 500, 778, 667, 944, 722, 778, 611, 778, 722, 556, 667, 722, 722, 1000, 722, 722, 667,
		333, 278, 333, 581, 500, 333,
		500, 556, 444, 556, 444, 333, 500, 556, 278, 333, 556, 278, 833, 556, 500, 556, 556, 444, 389, 333, 556, 500, 722, 500, 500, 444,
		394, 220, 394, 520}
)

// standardMoodik tagastab laiusfunktsiooni fondi nime järgi (ka Arial ja
// Times New Roman, mida Windowsi tootjad laiusteta viitavad).
func standardMoodik(nimi string) func(rune) float64 {
	n := strings.ToLower(nimi)
	paks := strings.Contains(n, "bold") || strings.Contains(n, "black")
	var t []int
	switch {
	case strings.Contains(n, "courier") || strings.Contains(n, "mono"):
		return func(rune) float64 { return 0.6 }
	case strings.Contains(n, "times") || strings.Contains(n, "serif") && !strings.Contains(n, "sans") || strings.Contains(n, "georgia") || strings.Contains(n, "cambria"):
		t = times
		if paks {
			t = timesPaks
		}
	default:
		t = helvetica
		if paks {
			t = helveticaPaks
		}
	}
	return func(r rune) float64 {
		if r > 126 {
			r = alustaht(r)
		}
		if r >= 32 && r <= 126 {
			return float64(t[r-32]) / 1000
		}
		return hinnangulineLaius(string(r))
	}
}

const (
	rohugaTahed = "ÀÁÂÃÄÅàáâãäåÇçÈÉÊËèéêëÌÍÎÏìíîïÑñÒÓÔÕÖØòóôõöøÙÚÛÜùúûüÝýÿŠšŽžČčĆćŁłŃńŚśŹźŻżĄąĘę"
	alusTahed   = "AAAAAAaaaaaaCcEEEEeeeeIIIIiiiiNnOOOOOOooooooUUUUuuuuYyySsZzCcCcLlNnSsZzZzAaEe"
)

// alustaht tagastab laiuse hindamiseks rõhumärgita tähe (õ -> o, Š -> S).
func alustaht(r rune) rune {
	for i, c := range []rune(rohugaTahed) {
		if c == r {
			return rune(alusTahed[i])
		}
	}
	switch r {
	case '„', '“', '”', '‘', '’', '‚':
		return '"'
	case '–':
		return '-'
	case '—':
		return 'M'
	case 0xA0:
		return ' '
	}
	if unicode.IsUpper(r) {
		return 'N'
	}
	return 'n'
}
