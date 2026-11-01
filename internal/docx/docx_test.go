package docx

import "testing"

func TestFormatNumber(t *testing.T) {
	cases := []struct {
		n    int
		f    string
		want string
	}{
		{1, "decimal", "1"}, {3, "lowerLetter", "c"}, {27, "lowerLetter", "aa"}, {4, "upperRoman", "IV"}, {9, "lowerRoman", "ix"}, {5, "decimalZero", "05"},
	}
	for _, c := range cases {
		if got := formatNumber(c.n, c.f); got != c.want {
			t.Errorf("formatNumber(%d,%s)=%s, ootasin %s", c.n, c.f, got, c.want)
		}
	}
}

func TestPoleDocx(t *testing.T) {
	if _, err := Read([]byte("see ei ole zip")); err == nil {
		t.Fatal("ootasin viga")
	}
}

func TestLvlRestartNull(t *testing.T) {
	num := &numbering{
		abstracts: map[int]*abstractNum{0: {id: 0}},
		nums:      map[int]*numInstance{1: {abstractID: 0, overrides: map[int]int{}}},
	}
	for i := range num.abstracts[0].levels {
		num.abstracts[0].levels[i] = defaultLevel()
	}
	num.abstracts[0].levels[0].text = "%1."
	num.abstracts[0].levels[1].text = "%2)"
	num.abstracts[0].levels[1].restart = 0
	num.abstracts[0].levels[1].hasRestart = true
	e := newEngine(num)
	seq := []int{0, 1, 1, 0, 1}
	var got []string
	for _, l := range seq {
		s, _ := e.next(1, l)
		got = append(got, s)
	}
	want := []string{"1.", "1)", "2)", "2.", "3)"}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("lvlRestart=0: sain %v, ootasin %v", got, want)
		}
	}
}
