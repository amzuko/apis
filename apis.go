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
	// TODO: consider having explicit inclusion and exclusion bounds?
	// This might simplify some of the bookkeeping around whether the 'both' bound is in-set or out-of-set.
	both
)

type item struct {
	b bound
	d apd.Decimal
	open bool
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
			if i > 0 {
				b.WriteString((", "))
			}
			if c.open {
				b.WriteString(("("))
			} else {
				b.WriteString(("["))
			}
			_, _ = b.WriteString(fmt.Sprintf("%v, ", &c.d))
			currentInSet = true
		case upper:
			_, _ = b.WriteString(fmt.Sprintf("%v", &c.d))
			if c.open {
				b.WriteString((")"))
			} else {
				b.WriteString(("]"))
			}
			currentInSet = false
		case both:
			if currentInSet {
				_, _ = b.WriteString(fmt.Sprintf("%v), (%v, ", &c.d, &c.d))
			} else {
				if i > 0 {
					b.WriteString((", "))
				}
				_, _ = b.WriteString(fmt.Sprintf("[%v, %v]", &c.d, &c.d))
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

		if v.d.Form == apd.Infinite {
			if !v.open {
				return fmt.Errorf("%v/%v is infinite, so cannot be closed", i, &v.d)
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
			if v.open {
				return fmt.Errorf("%v/%v is an exclusion or inclusion, so cannot be open", i, &v.d)
			}
		}

		// Reset
		currentD = v.d
	}

	if currentInSet {
		return fmt.Errorf("Last interval is not bounded.")
	}

	return nil
}

func New(l apd.Decimal, lOpen bool, u apd.Decimal, uOpen bool) Set {
	c := apd.Decimal{}
	apd.BaseContext.Cmp(&c, &l, &u)


	// Make inputs well formed
	if l.Form == apd.Infinite {
		lOpen = true
	}
	if u.Form == apd.Infinite {
		uOpen = true
	}
	
	var s Set
	if c.IsZero() {
		if l.Form == apd.Infinite && u.Form == apd.Infinite {
			// If we have a set that only contains the infinite value, make it empty
			return Set{}
		}
		s = Set{
			items: []item{
				{
				d: l,
				b: both,
				open: false, // by definition
				},
			},
		}
	} else {
		if c.Negative {
			s = Set{
				items: []item{
					{
					d: l,
					b: lower,
					open: lOpen,
					},
					{
					d: u,
					b: upper,
					open: uOpen,
					},
				},
			}
		} else {
			// swap order
			s = Set{
				items: []item{
					{
					d: u,
					b: lower,
					open: uOpen,
					},
					{
					d: l,
					b: upper,
					open: lOpen,
					},
				},
			}
		}
	}
	return s
}

func NewFromStrings(ls string, us string) (Set, error) {
	l, _, err := apd.BaseContext.NewFromString(ls)
	if err != nil {
		return Set{}, err
	}
	u, _, err := apd.BaseContext.NewFromString(us)
	if err != nil {
		return Set{}, err
	}
	return New(*l, false, *u, false), nil
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
		// Special case. The 
		newItems = append(newItems, item{
			d: negativeInfinity,
			b: lower,
			open: true, // By definiton
		})
		newItems = append(newItems, item{
			d: a.items[0].d,
			b: both,
			open: false, // By definition
		})
		newItems = append(newItems, item{
			d: positiveInfinity,
			b: upper,
			open: true, // By definiton
		})
	} else {
		for i, v := range a.items {
			if v.b == both {
				// No need to change open/closed, as singular values are by definition closed.
				if i == 0 {
					newItems = append(newItems, item{
						d: negativeInfinity,
						b: lower,
						open:true, // By definition
					})
					newItems = append(newItems, v)
				} else if i == len(a.items) - 1 {
					newItems = append(newItems, v)
					newItems = append(newItems, item{
						d: positiveInfinity,
						b: upper,
						open:true, // By definition
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
							open: true, // by definition
						})
						newItems = append(newItems, item{
							d: v.d,
							b: upper,
							open: !v.open,
						})
					}
				} else {
					newItems = append(newItems, item{
						d:v.d,
						b:upper,
						open: !v.open,
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
							open: !v.open,
						})
						newItems = append(newItems, item{
							d: positiveInfinity,
							b: upper,
							open: true,
						})

					}
				} else {
					newItems = append(newItems, item{
						d:v.d,
						b:lower,
						open: !v.open,
					})
				}
			}
		}
	}

	return Set{
		items: newItems,
	}
}

func (a Set)Intersection(b Set) Set {
	newItems := []item{}
	ai := 0
	bi := 0

	aCurrentInSet := false
	bCurrentInSet := false

	var c apd.Decimal

	for {
		if ai >= len(a.items)  || bi >= len(b.items){
			// No more intersections possible.
			break
		}

		av := a.items[ai]
		bv := b.items[bi]

		// TODO: errors
		_, _ = apd.BaseContext.Cmp(&c, &av.d, &bv.d)

		if c.IsZero() {
			if av.b == both {
				if aCurrentInSet {
					// Exclusion in A
					if bv.b == both {
						// Exlcuded in both
						if bCurrentInSet {
							newItems = append(newItems, av)
						} else {
							// Inclusion in B, exclusion in A, skip as no set is open.
						}
					} else if bv.b == lower {
						// Not currently in any set, boundary is excluded, but A is not in-set above the point
						newItems = append(newItems, item {
							lower,
							bv.d,
							true,
						})
					} else if bv.b == upper {
						// Interval is terminating, boundary value explictly excluded from A, so boundary is open.
						newItems = append(newItems, item {
							upper,
							bv.d,
							true,
						})
					}
				} else {
					// Inclusion in A
					if bv.b == both {
						if bCurrentInSet {
							// Exclusion in B, no set is open, no need to exclude.
						} else {
							// Both explicitly included.
							newItems = append(newItems, av)
						}
					} else if bv.b == lower {
						newItems = append(newItems, item{
							both,
							av.d,
							false,
						})
					} else if bv.b == upper {
						// Intersection not in current interval as A not in current internal
					}
				}
			} else if av.b == lower {
				if bv.b == lower {
					// Both opening, interval is closed if both are closed
					newItems = append(newItems, item{
						lower,
						av.d,
						av.open || bv.open,
					})
				} else if bv.b == upper {
					// A terminating, b starting
					// If neither is open
					if !(av.open || bv.open) {
						newItems = append(newItems, item{
							both,
							av.d,
							false,	// by defintiion
						})
					}
				} else if bv.b == both {
					if bCurrentInSet {
						// Exclusion in B, A is beginning, 
						newItems = append(newItems, item {
							lower,
							av.d,
							true,
						})
					} else {
						// Inclusion in B, open interval in A
						if !av.open {
							newItems = append(newItems, item {
								both,
								av.d,
								false,
							})
						}
					}
				}
			} else if av.b == upper{
				// terminating A
				if bv.b == lower {
					if !(av.open || bv.open) {
						newItems = append(newItems, item {
							both,
							av.d,
							false,
						})
					}
				} else if bv.b == upper {
					newItems = append(newItems, item{
						upper,
						av.d,
						av.open || bv.open,
					})
				} else if bv.b == both {
					if bCurrentInSet {
						newItems = append(newItems, item{
							upper,
							av.d,
							true,
						})
					} else {
						if !av.open {
							newItems = append(newItems, item{
								both,
								av.d,
								false,
							})
						}
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
				// If a is termianting, terminate both,
				// If a is beginning, begin both,
				// If a is an exlcusion or inclusion, it applies.
				newItems = append(newItems, av)
			} else {
				// Need both to be active
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
				newItems = append(newItems, bv)
			} else {
			
			}
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

	return Set{newItems}
}


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
						if bv.open {
							// The exlcusion is not included in the opening B interval
							newItems = append(newItems, av)
						} else {
							// B includes the exlcusion in A, keep the interval open./
						}
					} else {
						// b is leaving set, a will remain open 
						if bv.open {
							// b does not contain the value
							newItems = append(newItems, av)
						} else {
							// Skip
						}
					}
				} else {
					// Inclusion in A
					if bv.b == both {
						if bCurrentInSet {
							// Exlusion in B, inclusion in A. Cancel each other out -- skip
						} else {
							// Both explictly included
							newItems = append(newItems, av)
						}
					} else {
						// REgardless of whether b was open or closed, this boundary is now closed because 
						// A explicitly inclues that value
						newItems = append(newItems, item{
							bv.b,
							bv.d,
							false,	// The boundary point is included. 
						})
					}
				}
			} else if av.b == lower {
				// fmt.Println("avb lower")

				if bv.b == lower  {
					// Both lower bounds, set is closed if either are closed
					newItems = append(newItems, item{
						bv.b,
						bv.d,
						bv.open && av.open,
					})
				} else if bv.b == upper {
					// If  both are open, need a new inclusion
					if bv.open && av.open {
						newItems = append(newItems, item{
							both,
							bv.d,
							false,
						})
					} else {
						// skip both, 
					}
				} else {
					if bCurrentInSet {
						// Exclusion in b, end of A
						if !av.open {
							// Skip both
						} else {
							newItems = append(newItems, bv)
						}
					} else {
						// Inclusion in B, set opening A
						// By definition closed
						newItems = append(newItems, item{
							av.b,
							av.d,
							false,
						})
					}
				}
			} else {
				// fmt.Println("avb upper")
				// A ending
				if bv.b == upper  {
					newItems = append(newItems, item{
						bv.b,
						bv.d,
						bv.open && av.open,
					})
				} else if bv.b == lower {
					// B begining
					if av.open && bv.open {
						newItems = append(newItems, item{
							both,
							bv.d,
							false,
						})
					} else {
						// AT least one is closed, skip both
					}
				} else {
					if bCurrentInSet {
						if av.open {
							newItems = append(newItems, bv)
						} else {
							// Skip -- point is included in A, so interval continues
						}
						// Exclusion in b, inclusion in A (if closed)
						// Skip both
					} else {
						// Inclusion in B, set closing A
						// result is always closed
						newItems = append(newItems, item{
							av.b,
							av.d,
							false,
						})
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