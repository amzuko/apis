package apis

import (
	"fmt"
	"strings"

	"github.com/cockroachdb/apd/v3"
)

type bound int
const (
	lower = iota
	upper
	both
)

type item struct {
	// TODO: these are all closed sets. Open sets too.
	b bound
	d apd.Decimal
}

type Set struct {
	items []item
}

func (s *Set) String() string {
	b := strings.Builder{}
	currentInSet := false
	for i, c := range s.items {
		switch c.b {
		// Ignoring all errors
		case lower:
			if currentInSet {
				panic("should never happen")
			} else {
				if i > 0 {
					b.WriteString((", "))
				}
				_, _ = b.WriteString(fmt.Sprintf("[%v, ", &c.d))
			}
			currentInSet = true
		case upper:
			if currentInSet {
				_, _ = b.WriteString(fmt.Sprintf("%v]", &c.d))
			} else {
				panic("should never happen")

			}
			currentInSet = false
		case both:
			if currentInSet {
				_, _ = b.WriteString(fmt.Sprintf("%v), (%v, ", &c.d, &c.d))
			} else {
				if i > 0 {
					b.WriteString((", "))
				}
				_, _ = b.WriteString(fmt.Sprintf("(%v, %v)", &c.d, &c.d))
			}
		}
	}
	return b.String()
}


func (s *Set)Validate() (error) {
	currentInSet := false
	var currentD apd.Decimal
	var sign apd.Decimal
	for i,v := range s.items {
		if i > 0 {
			// Validate that the decimal values are sorted
			// TODO: condition handling?
			_, err := apd.BaseContext.Cmp(&sign, &currentD, &v.d)
			if err != nil {
				return err
			}
			if !sign.Negative {
				return fmt.Errorf("%v is not less than %v", v.d.String(), currentD.String())
			}
		}

		switch v.b {
		case lower:
			if currentInSet {
				return fmt.Errorf("%v/%v is a lower bound on an interval, but numbers below the bound are in-set as well.", i, &v.d)				
			} else {
				// Transition into the set
				currentInSet = true
			}
		case upper:
			if currentInSet {
				// Transition out of the set
				currentInSet = false
			} else {
				return fmt.Errorf("%v/%v is an upper bound on an interval, but numbers below the bound are out-of-set as well.", i, &v.d)				
			}
		case both:
			// In the case of being inset, a bound that is "both" is an excluded discrete number.
			// In the case of being out of set, a bound that is "both" is an included discrete number.

		}

		// Reset
		currentD = v.d
	}

	if currentInSet {
		return fmt.Errorf("Last interval is not bounded.")
	}

	return nil
}

func New(ls string, us string) (Set, error) {
	l, _, err := apd.BaseContext.NewFromString(ls)
	if err != nil {
		return Set{}, err
	}
	u, _, err := apd.BaseContext.NewFromString(us)
	if err != nil {
		return Set{}, err
	}
	c := apd.Decimal{}
	apd.BaseContext.Cmp(&c, l, u)
	
	var s Set
	if c.IsZero() {
		if l.Form == apd.Infinite && u.Form == apd.Infinite {
			// If we have a set that only contains the infinite value, make it empty
			return Set{}, nil
		}
		s = Set{
			items: []item{
				{
				d: *l,
				b: both,
				},
			},
		}
	} else {
		if c.Negative {
			s = Set{
				items: []item{
					{
					d: *l,
					b: lower,
					},
					{
					d: *u,
					b: upper,
					},
				},
			}
		} else {
			// swap order
			s = Set{
				items: []item{
					{
					d: *u,
					b: lower,
					},
					{
					d: *l,
					b: upper,
					},
				},
			}
		}
	}
	return s, nil
}

func isNegativeInfinity(d apd.Decimal) bool{
	return d.Form == apd.Infinite && d.Negative
}

func isPositiveInfinity(d apd.Decimal) bool {
	return d.Form == apd.Infinite && !d.Negative
}

var negativeInfinity apd.Decimal
var positiveInfinity apd.Decimal

func init() {
	ni, _, _ := apd.BaseContext.NewFromString("-infinity")
	pi, _, _ := apd.BaseContext.NewFromString("infinity")

	negativeInfinity = *ni
	positiveInfinity = *pi
}

func (a Set) Complement() Set {
	newItems := []item{}


	if len(a.items) == 1 {
		// Special case
		newItems = append(newItems, item{
			d: negativeInfinity,
			b: lower,
		})
		newItems = append(newItems, item{
			d: a.items[0].d,
			b: both,
		})
		newItems = append(newItems, item{
			d: positiveInfinity,
			b: upper,
		})
	} else {
		for i, v := range a.items {
			if v.b == both {
				if i == 0 {
					newItems = append(newItems, item{
						d: negativeInfinity,
						b: lower,
					})
					newItems = append(newItems, v)
				} else if i == len(a.items) - 1 {
					newItems = append(newItems, v)
					newItems = append(newItems, item{
						d: positiveInfinity,
						b: upper,
					})
				} else {
					newItems = append(newItems, v)
				}
				continue
			}
			if v.b == lower {
				if i == 0 {
					if isNegativeInfinity(v.d) {
						//Don't need to include this
					} else {
						newItems = append(newItems, item{
							d: negativeInfinity,
							b: lower,
						})
						newItems = append(newItems, item{
							d: v.d,
							b: upper,
						})
					}
				} else {
					newItems = append(newItems, item{
						d:v.d,
						b:upper,
					})
				}
			} else {
				if i == len(a.items) - 1 {
					if isPositiveInfinity(v.d) {
						// Skip
					} else {
						newItems = append(newItems, item{
							d: v.d,
							b: lower,
						})
						newItems = append(newItems, item{
							d: positiveInfinity,
							b: upper,
						})

					}
				} else {
					newItems = append(newItems, item{
						d:v.d,
						b:lower,
					})
				}
			}
		}
	}

	return Set{
		items: newItems,
	}
}


// a is modified in place
func (a Set)Union(b Set) Set {

	newItems := []item{}

	ai := 0
	bi := 0

	aCurrentInSet := false
	bCurrentInSet := false

	var c apd.Decimal

	for {
		// rset := Set{newItems}
		// fmt.Printf("Set: '%v', %v, %v, %v, %v \n", rset.String(), ai, bi, aCurrentInSet, bCurrentInSet)
		if ai >= len(a.items) {
			// Done with a, flush b into new items
			// fmt.Println("flushing b")
			for ; bi < len(b.items); bi++ {
				newItems = append(newItems, b.items[bi])
			}
			// rset := Set{newItems}
			// fmt.Printf("Final Set: '%v', %v, %v\n", rset.String(), ai, bi)
			break
		}
		if bi >= len(b.items) {
			// Done with b, flush a into new items
			for ; ai < len(a.items); ai++ {
				newItems = append(newItems, a.items[ai])
			}
			break
		}


		av := a.items[ai]
		bv := b.items[bi]

		// TODO: errors
		_, _ = apd.BaseContext.Cmp(&c, &av.d, &bv.d)

		if c.IsZero() {
			// process both?
			// 16 different combinations?
			// fmt.Println("In equal")
			if av.b == both {
				if aCurrentInSet {
					// Exlcusion in A
					if bv.b == both {
						if bCurrentInSet {
							// Exlusion in B -- they are identical. 
							newItems = append(newItems, av)
						} else {
							// Excluded in A, but explicitly included in B
							// skip
						}
					} else if bv.b == lower {
						// TODO: open closed sets change this
						// Skip all, excluded in A but incl;uded in B, set is open going forward
					} else {
						// TODO: open/closed sets change this
						// b is leaving set, a will remain open 
						newItems = append(newItems, av)
					}
				} else {
					if bv.b == both {
						if bCurrentInSet {
							// Exlusion in B, inclusion in A. Cancel each other out -- skip
						} else {
							// Both explictly included
							newItems = append(newItems, av)
						}
					} else if bv.b == lower {
						// TODO: open closed sets change this
						// Open set, don't need to explicitly include
						newItems = append(newItems, bv)
					} else {
						// TODO: open/closed sets change this
						// b is leaving set, a will remain open 
						newItems = append(newItems, bv)
					}
				}
			} else if av.b == lower {
				// fmt.Println("avb lower")

				if bv.b == lower  {
					// Identical
					newItems = append(newItems, bv)
				} else if bv.b == upper {
					// Skip both 
					// TODO: open/closed changes this
				} else {
					if bCurrentInSet {
						// Exclusion in b, inclusion in A (if closed)
						// Skip both
					} else {
						// Inclusion in B, set opening A
						newItems = append(newItems, av)
					}
				}
			} else {
				// fmt.Println("avb upper")

				if bv.b == upper  {
					// Identical
					newItems = append(newItems, bv)
				} else if bv.b == lower {
					// fmt.Println("bvb lower")
					// Skip both 
					// TODO: open/closed changes this
				} else {
					if bCurrentInSet {
						// Exclusion in b, inclusion in A (if closed)
						// Skip both
					} else {
						// Inclusion in B, set closing A
						// TODO: changes with open/closed
						newItems = append(newItems, av)
					}
				}
			}

			if av.b == lower {
				aCurrentInSet = true
			} else if av.b == upper {
				aCurrentInSet = false
			}

			if bv.b == lower {
				bCurrentInSet = true
			} else if bv.b == upper {
				bCurrentInSet = false
			}

			ai++
			bi++
			continue
		}
		if c.Negative {
			// fmt.Println("In negative")
			// process a
			if bCurrentInSet {
					
			} else {
				newItems = append(newItems, av)
			}
			switch av.b {
			case both:
				// Nothing
			case lower:
				aCurrentInSet = true
			case upper:
				aCurrentInSet = false
			}
			ai++
		} else {
			// fmt.Println("In positive")
			// process b
			if aCurrentInSet {
			} else {
				newItems = append(newItems, bv)
			}
			// process a
			switch bv.b {
			case both:
				// Nothing
			case lower:
				bCurrentInSet = true
			case upper:
				bCurrentInSet = false
			}
			bi++
		}
		
	}

	return Set{
		items: newItems,
	}
}