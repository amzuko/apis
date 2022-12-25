package apis

import "testing"

func TestCreationAndComplement(t *testing.T) {
	cases := [][]string {
		{"-infinity", "0", "[-Infinity, 0]", "[0, Infinity]"},
		{"3", "3.0", "(3, 3)", "[-Infinity, 3), (3, Infinity]"},
		{"1", "2.0", "[1, 2.0]", "[-Infinity, 1], [2.0, Infinity]"},
	}

	for _, c := range cases {
		t.Run(c[0]+"+"+c[1], func(t *testing.T) {
			s, err := New(c[0], c[1])
			if err != nil {
				t.Fatal(err)
			}
			err = s.Validate()
			if err != nil {
				t.Fatal(err)
			}
			r := s.String()
			if r != c[2] {
				t.Fatalf("Expected creation '%v', but got '%v'", c[2], r)
			}

			comp := s.Complement()
			r = comp.String()
			if r!=c[3] {
				t.Fatalf("Expected complement'%v', but got '%v'", c[3], r)
			}
		})
	}
}

func TestUnion(t *testing.T) {
	cases := [][]string{
		{"-infinity", "0", "0", "infinity", "[-Infinity, Infinity]"},
		{"0", "1", "2", "3", "[0, 1], [2, 3]"},
		{"0", "1", "1", "1", "[0, 1]"},
		{"1", "1", "0", "1", "[0, 1]"},
		{"-infinity", "2", "0", "infinity", "[-Infinity, Infinity]"},
	}

	for _, c := range cases {
		t.Run(c[4], func(t *testing.T) {
			l, err := New(c[0], c[1])
			if err != nil {
				t.Fatal(err)
			}
			err = l.Validate()
			if err != nil {
				t.Fatal(err)
			}

			m, err := New(c[2], c[3])
			if err != nil {
				t.Fatal(err)
			}
			err = m.Validate()
			if err != nil {
				t.Fatal(err)
			}

			n := l.Union(m)

			r := n.String()
			if r != c[4] {
				t.Fatalf("Expected '%v', but got '%v'", c[2], r)
			}
		})
	}
}


func getValue(b byte) string {
	switch b%8 {
	case 0:
		return "-Infinity"
	case 1:
		return "0"
	case 2:
		return "2"
	case 3:
		return "3"
	case 4:
		return "4"
	case 5:
		return "5"
	case 6:
		return "6"
	case 7:
		return "Infinity"
	}
	panic("should never happen")
}


func FuzzOperations(f *testing.F) {
	f.Fuzz(func(t *testing.T, b []byte) {
		// Create an initial set
		if len(b) < 2 {
			t.SkipNow()
		}
		s, err := New(getValue(b[0]), getValue(b[1]))
		if err != nil {
			t.Fatal(err)
		}
		t.Logf("Created '%v'", s.String())

		i := 3
		for {
			if i >= len(b) {
				break
			}
			switch(b[i]%3){
			case 0:
				s = s.Complement()
				t.Logf("Completment '%v'", s.String())

				i++
			case 1:
				if i + 2 >= len(b) {
					i += 3
					break
				}
				u, err := New(getValue(b[i+1]), getValue(b[i+2]))
				t.Logf("Union with '%v'", u.String())

				if err != nil {
					t.Fatal(err)
				}
				s = s.Union(u)
				t.Logf("Result '%v'", s.String())

				i+=3
			case 2:
				if i + 2 >= len(b) {
					i += 3
					break
				}
				u, err := New(getValue(b[i+1]), getValue(b[i+2]))
				t.Logf("Union with complement of '%v'", u.String())

				if err != nil {
					t.Fatal(err)
				}
				s = s.Union(u.Complement())
				t.Logf("Result '%v'", s.String())
				i += 3
			}
		}

		err = s.Validate()
		if err != nil {
			t.Fatal(err)
		}
	})
}