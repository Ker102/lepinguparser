package docx

import (
	"bytes"
	"encoding/xml"
	"io"
	"strconv"
	"strings"
)

type level struct {
	start      int
	format     string
	text       string
	restart    int // -1 = vaikimisi (taaskäivitub kõrgema taseme järel), 0 = ei taaskäivitu
	isLegal    bool
	hasRestart bool
}

type abstractNum struct {
	id     int
	levels [9]level
}

type numInstance struct {
	abstractID int
	overrides  map[int]int // tase -> startOverride
}

type numbering struct {
	abstracts map[int]*abstractNum
	nums      map[int]*numInstance
}

func defaultLevel() level {
	return level{start: 1, format: "decimal", restart: -1}
}

func parseNumbering(b []byte) (*numbering, error) {
	dec := xml.NewDecoder(bytes.NewReader(b))
	n := &numbering{abstracts: map[int]*abstractNum{}, nums: map[int]*numInstance{}}
	var curAbs *abstractNum
	var curNum *numInstance
	curLvl := -1
	overrideLvl := -1
	for {
		tok, err := dec.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return n, err
		}
		switch t := tok.(type) {
		case xml.StartElement:
			switch t.Name.Local {
			case "abstractNum":
				v, _ := attr(t, "abstractNumId")
				curAbs = &abstractNum{id: atoiDefault(v, -1)}
				for i := range curAbs.levels {
					curAbs.levels[i] = defaultLevel()
				}
			case "lvl":
				v, _ := attr(t, "ilvl")
				curLvl = atoiDefault(v, -1)
			case "start":
				v, _ := attr(t, "val")
				if curAbs != nil && curLvl >= 0 && curLvl < 9 && overrideLvl < 0 {
					curAbs.levels[curLvl].start = atoiDefault(v, 1)
				}
			case "numFmt":
				v, _ := attr(t, "val")
				if curAbs != nil && curLvl >= 0 && curLvl < 9 && overrideLvl < 0 {
					curAbs.levels[curLvl].format = v
				}
			case "lvlText":
				v, _ := attr(t, "val")
				if curAbs != nil && curLvl >= 0 && curLvl < 9 && overrideLvl < 0 {
					curAbs.levels[curLvl].text = v
				}
			case "lvlRestart":
				v, _ := attr(t, "val")
				if curAbs != nil && curLvl >= 0 && curLvl < 9 {
					curAbs.levels[curLvl].restart = atoiDefault(v, -1)
					curAbs.levels[curLvl].hasRestart = true
				}
			case "isLgl":
				if curAbs != nil && curLvl >= 0 && curLvl < 9 {
					curAbs.levels[curLvl].isLegal = true
				}
			case "num":
				v, _ := attr(t, "numId")
				curNum = &numInstance{abstractID: -1, overrides: map[int]int{}}
				n.nums[atoiDefault(v, -1)] = curNum
			case "abstractNumId":
				if curNum != nil {
					v, _ := attr(t, "val")
					curNum.abstractID = atoiDefault(v, -1)
				}
			case "lvlOverride":
				v, _ := attr(t, "ilvl")
				overrideLvl = atoiDefault(v, -1)
			case "startOverride":
				if curNum != nil && overrideLvl >= 0 {
					v, _ := attr(t, "val")
					curNum.overrides[overrideLvl] = atoiDefault(v, 1)
				}
			}
		case xml.EndElement:
			switch t.Name.Local {
			case "abstractNum":
				if curAbs != nil {
					n.abstracts[curAbs.id] = curAbs
				}
				curAbs = nil
			case "lvl":
				curLvl = -1
			case "num":
				curNum = nil
			case "lvlOverride":
				overrideLvl = -1
			}
		}
	}
	return n, nil
}

// engine arvutab automaatnumbreid dokumendi läbimise järjekorras.
// Loendurid on abstraktse nummerduse põhised (nagu Wordis): sama abstraktset
// definitsiooni kasutavad numId-d jätkavad sama loendit, välja arvatud kui
// numId-l on startOverride.
type engine struct {
	num      *numbering
	counters map[int]*[9]int
	started  map[int]*[9]bool
	seenNum  map[int]bool
	startOv  map[int]map[int]int // abstraktne id -> tase -> startOverride
	missing  int
}

func newEngine(n *numbering) *engine {
	return &engine{num: n, counters: map[int]*[9]int{}, started: map[int]*[9]bool{}, seenNum: map[int]bool{}, startOv: map[int]map[int]int{}}
}

func (e *engine) next(numID, ilvl int) (string, bool) {
	if e.num == nil {
		e.missing++
		return "", false
	}
	inst, ok := e.num.nums[numID]
	if !ok {
		e.missing++
		return "", false
	}
	abs, ok := e.num.abstracts[inst.abstractID]
	if !ok {
		e.missing++
		return "", false
	}
	if ilvl < 0 || ilvl > 8 {
		ilvl = 0
	}
	c, ok := e.counters[abs.id]
	if !ok {
		c = &[9]int{}
		e.counters[abs.id] = c
		e.started[abs.id] = &[9]bool{}
	}
	st := e.started[abs.id]
	if e.startOv[abs.id] == nil {
		e.startOv[abs.id] = map[int]int{}
	}
	algus := func(k int) int {
		if v, ok := e.startOv[abs.id][k]; ok {
			return v
		}
		return abs.levels[k].start
	}
	if !e.seenNum[numID] {
		e.seenNum[numID] = true
		// startOverride: see loendi eksemplar alustab taset antud väärtusest
		for lv, v := range inst.overrides {
			if lv >= 0 && lv < 9 {
				e.startOv[abs.id][lv] = v
				st[lv] = false
				for d := lv + 1; d < 9; d++ {
					st[d] = false
				}
			}
		}
	}
	if !st[ilvl] {
		c[ilvl] = algus(ilvl) - 1
		st[ilvl] = true
	}
	c[ilvl]++
	// Sügavamad tasemed taaskäivituvad. w:lvlRestart="N" tähendab, et tase
	// taaskäivitub ainult siis, kui kasutatakse taset N (1-põhine) või kõrgemat;
	// "0" tähendab, et tase ei taaskäivitu kunagi.
	for d := ilvl + 1; d < 9; d++ {
		r := abs.levels[d].restart
		if !abs.levels[d].hasRestart || r < 0 || (r > 0 && ilvl <= r-1) {
			st[d] = false
		}
	}
	lv := abs.levels[ilvl]
	if lv.format == "bullet" || lv.format == "none" {
		return "", true
	}
	text := lv.text
	if text == "" {
		text = "%" + strconv.Itoa(ilvl+1) + "."
	}
	var b strings.Builder
	for i := 0; i < len(text); i++ {
		ch := text[i]
		if ch == '%' && i+1 < len(text) && text[i+1] >= '1' && text[i+1] <= '9' {
			k := int(text[i+1] - '1')
			if !st[k] {
				// Viidatud kõrgem tase pole selles loendis veel alanud: Word ja
				// LibreOffice kuvavad selle algväärtust ja loevad taseme alanuks,
				// nii et järgmine selle taseme lõik saab väärtuse algus+1.
				c[k] = algus(k)
				st[k] = true
			}
			val := c[k]
			f := abs.levels[k].format
			if lv.isLegal {
				f = "decimal"
			}
			b.WriteString(formatNumber(val, f))
			i++
			continue
		}
		b.WriteByte(ch)
	}
	return strings.TrimSpace(b.String()), true
}

func formatNumber(n int, format string) string {
	switch format {
	case "lowerLetter":
		return letters(n, false)
	case "upperLetter":
		return letters(n, true)
	case "lowerRoman":
		return strings.ToLower(roman(n))
	case "upperRoman":
		return roman(n)
	case "decimalZero":
		if n < 10 && n >= 0 {
			return "0" + strconv.Itoa(n)
		}
		return strconv.Itoa(n)
	default:
		return strconv.Itoa(n)
	}
}

func letters(n int, upper bool) string {
	if n <= 0 {
		return strconv.Itoa(n)
	}
	// Word: a..z, aa..zz, aaa..
	base := byte('a')
	if upper {
		base = 'A'
	}
	rep := (n-1)/26 + 1
	ch := base + byte((n-1)%26)
	return strings.Repeat(string(ch), rep)
}

func roman(n int) string {
	if n <= 0 || n >= 4000 {
		return strconv.Itoa(n)
	}
	vals := []int{1000, 900, 500, 400, 100, 90, 50, 40, 10, 9, 5, 4, 1}
	syms := []string{"M", "CM", "D", "CD", "C", "XC", "L", "XL", "X", "IX", "V", "IV", "I"}
	var b strings.Builder
	for i, v := range vals {
		for n >= v {
			b.WriteString(syms[i])
			n -= v
		}
	}
	return b.String()
}
