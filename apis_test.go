package apis

import (
	"testing"

	"github.com/cockroachdb/apd/v3"
)

type boundDef struct {
	s string
	o bool
}

func newFromBounds(b1 boundDef, b2 boundDef) Set {
	l, _, _ := apd.BaseContext.NewFromString(b1.s)
	u, _, _ := apd.BaseContext.NewFromString(b2.s)
	return New(*l, b1.o, *u, b2.o)
}

func TestCreationAndComplement(t *testing.T) {
	type testcase struct {
		b1 boundDef
		b2 boundDef
		creationResult string
		complementResult string
	}
	cases := []testcase {
		{boundDef{"-infinity", true}, boundDef{"0", false}, "(-Infinity, 0]", "(0, Infinity)"},
		{boundDef{"-infinity", true}, boundDef{"0", true}, "(-Infinity, 0)", "[0, Infinity)"},
		{boundDef{"3", false}, boundDef{"3.0", false}, "[3, 3]", "(-Infinity, 3), (3, Infinity)"},
		{boundDef{"1", false}, boundDef{"2.0", false}, "[1, 2.0]", "(-Infinity, 1), (2.0, Infinity)"},
	}

	for _, c := range cases {
		t.Run(c.creationResult, func(t *testing.T) {
			s := newFromBounds(c.b1, c.b2)

			err := s.Validate()
			if err != nil {
				t.Fatal(err)
			}
			r := s.String()
			if r != c.creationResult {
				t.Fatalf("Expected creation '%v', but got '%v'", c.creationResult, r)
			}

			comp := s.Complement()
			r = comp.String()
			if r!=c.complementResult {
				t.Fatalf("Expected complement'%v', but got '%v'", c.complementResult, r)
			}
		})
	}
}

func TestUnion(t *testing.T) {

	type testcase struct {
		b []boundDef
		unionResult string
	}

	cases := []testcase{
		{[]boundDef{{"-infinity", true}, { "0", false}, {"0", false}, {"infinity", true}}, "(-Infinity, Infinity)"},
		{[]boundDef{{"0", false}, { "1", false}, {"2", false}, {"3", true}}, "[0, 1], [2, 3)"},
		{[]boundDef{{"0", false}, { "1", true}, {"1", false}, {"1", false}}, "[0, 1]"},
		{[]boundDef{{"-infinity", true}, { "2", false}, {"0", false}, {"infinity", true}}, "(-Infinity, Infinity)"},
	}

	for _, c := range cases {
		t.Run(c.unionResult, func(t *testing.T) {
			l := newFromBounds(c.b[0], c.b[1])
			err := l.Validate()
			if err != nil {
				t.Fatal(err)
			}

			m := newFromBounds(c.b[2], c.b[3])

			err = m.Validate()
			if err != nil {
				t.Fatal(err)
			}

			n := l.Union(m)

			r := n.String()
			if r != c.unionResult {
				t.Fatalf("Expected '%v', but got '%v'", c.unionResult, r)
			}
		})
	}
}


func TestIntersect(t *testing.T) {

	type testcase struct {
		b []boundDef
		intersectionResult string
	}

	cases := []testcase{
		{[]boundDef{{"0", false}, { "0", false}, {"0", false}, {"infinity", true}}, "[0, 0]"},
	}

	for _, c := range cases {
		t.Run(c.intersectionResult, func(t *testing.T) {
			l := newFromBounds(c.b[0], c.b[1])
			err := l.Validate()
			if err != nil {
				t.Fatal(err)
			}

			m := newFromBounds(c.b[2], c.b[3])

			err = m.Validate()
			if err != nil {
				t.Fatal(err)
			}

			n := l.Intersection(m)

			r := n.String()
			if r != c.intersectionResult {
				t.Fatalf("Expected '%v', but got '%v'", c.intersectionResult, r)
			}
		})
	}
}



func getValue(b byte) apd.Decimal {
	var d *apd.Decimal
	switch b%8 {
	case 0:
		d, _, _ = apd.BaseContext.NewFromString("-Infinity")
	case 1:
		d, _, _ = apd.BaseContext.NewFromString( "0")
	case 2:
		d, _, _ = apd.BaseContext.NewFromString( "2")
	case 3:
		d, _, _ = apd.BaseContext.NewFromString( "3")
	case 4:
		d, _, _ = apd.BaseContext.NewFromString( "4")
	case 5:
		d, _, _ = apd.BaseContext.NewFromString( "5")
	case 6:
		d, _, _ = apd.BaseContext.NewFromString( "6")
	case 7:
		d, _, _ = apd.BaseContext.NewFromString( "Infinity")
	}
	return *d
}

func getOpen(b byte) bool {
	return b%2 == 1
}

func FuzzOperations(f *testing.F) {
	f.Fuzz(func(t *testing.T, b []byte) {
		// Create an initial set
		if len(b) < 4 {
			t.SkipNow()
		}
		s := New(getValue(b[0]), getOpen(b[1]), getValue(b[2]), getOpen(b[2]))

		t.Logf("Created '%v'", s.String())

		i := 3
		for {
			if i >= len(b) {
				break
			}
			switch(b[i]%5){
			case 0:
				s = s.Complement()
				t.Logf("Completment")
				t.Logf("Result '%v'", s.String())

				i++
			case 1:
				if i + 4 >= len(b) {
					i += 5
					break
				}
				u := New(getValue(b[i+1]), getOpen(b[i+2]), getValue(b[i+3]), getOpen(b[i+4]))
				t.Logf("Union with '%v'", u.String())

				s = s.Union(u)
				t.Logf("Result '%v'", s.String())

				i+=5
			case 2:
				if i + 4 >= len(b) {
					i += 5
					break
				}
				u := New(getValue(b[i+1]), getOpen(b[i+2]), getValue(b[i+3]), getOpen(b[i+4]))
				t.Logf("Union with complement of '%v'", u.String())

				s = s.Union(u.Complement())
				t.Logf("Result '%v'", s.String())
				i += 5
			case 3:
				if i + 4 >= len(b) {
					i += 5
					break
				}
				u := New(getValue(b[i+1]), getOpen(b[i+2]), getValue(b[i+3]), getOpen(b[i+4]))
				t.Logf("Intersect with  '%v'", u.String())
				s = s.Intersection(u)
				t.Logf("Result '%v'", s.String())
				i += 5
			case 4:
				if i + 4 >= len(b) {
					i += 5
					break
				}
				u := New(getValue(b[i+1]), getOpen(b[i+2]), getValue(b[i+3]), getOpen(b[i+4]))
				t.Logf("Intersect with  complement of '%v'", u.String())
				s = s.Intersection(u.Complement())
				t.Logf("Result '%v'", s.String())
				i += 5
			}
		}

		err := s.Validate()
		if err != nil {
			t.Fatal(err)
		}
	})
}