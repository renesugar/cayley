package gshape

import (
	"github.com/cayleygraph/cayley/graph/iterator"
	"github.com/cayleygraph/cayley/graph/values"
	. "github.com/cayleygraph/cayley/query/shape"
)

func clearFixedTags(arr []Shape) ([]Shape, map[string]values.Ref) {
	var tags map[string]values.Ref
	for i := 0; i < len(arr); i++ {
		if ft, ok := arr[i].(FixedTags); ok {
			if tags == nil {
				tags = make(map[string]values.Ref)
				na := make([]Shape, len(arr))
				copy(na, arr)
				arr = na
			}
			arr[i] = ft.On
			for k, v := range ft.Tags {
				tags[k] = v
			}
		}
	}
	return arr, tags
}

// Intersect computes an intersection of nodes between multiple queries. Similar to And iterator.
type Intersect []Shape

func (s Intersect) BuildIterator() iterator.Iterator {
	if len(s) == 0 {
		return iterator.NewNull()
	}
	sub := make([]iterator.Iterator, 0, len(s))
	for _, c := range s {
		sub = append(sub, c.BuildIterator())
	}
	if len(sub) == 1 {
		return sub[0]
	}
	return iterator.NewAnd(sub...)
}
func (s Intersect) Optimize(r Optimizer) (sout Shape, opt bool) {
	if len(s) == 0 {
		return nil, true
	}
	// function to lazily reallocate a copy of Intersect slice
	realloc := func() {
		if !opt {
			arr := make(Intersect, len(s))
			copy(arr, s)
			s = arr
		}
	}
	// optimize sub-iterators, return empty set if Null is found
	for i := 0; i < len(s); i++ {
		c := s[i]
		if IsNull(c) {
			return nil, true
		}
		v, ok := c.Optimize(r)
		if !ok {
			continue
		}
		realloc()
		opt = true
		if IsNull(v) {
			return nil, true
		}
		s[i] = v
	}
	if r != nil {
		ns, nopt := r.OptimizeShape(s)
		return ns, opt || nopt
	}
	if arr, ft := clearFixedTags([]Shape(s)); ft != nil {
		ns, _ := FixedTags{On: Intersect(arr), Tags: ft}.Optimize(r)
		return ns, true
	}
	var (
		onlyAll = true   // contains only AllNodes shapes
		fixed   []Fixed  // we will collect all Fixed, and will place it as a first iterator
		tags    []string // if we find a Save inside, we will push it outside of Intersect
		quads   Quads    // also, collect all quad filters into a single set
	)
	remove := func(i *int, optimized bool) {
		realloc()
		if optimized {
			opt = true
		}
		v := *i
		s = append(s[:v], s[v+1:]...)
		v--
		*i = v
	}
	// second pass - remove AllNodes, merge Quads, collect Fixed, collect Save, merge Intersects
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch c := c.(type) {
		case AllNodes: // remove AllNodes - it's useless
			remove(&i, true)
			// prevent resetting of onlyAll
			continue
		case Quads: // merge all quad filters
			remove(&i, false)
			if quads == nil {
				quads = c[:len(c):len(c)]
			} else {
				opt = true
				quads = append(quads, c...)
			}
		case Fixed: // collect all Fixed sets
			remove(&i, true)
			fixed = append(fixed, c)
		case Intersect: // merge with other Intersects
			remove(&i, true)
			s = append(s, c...)
		case Save: // push Save outside of Intersect
			realloc()
			opt = true
			tags = append(tags, c.Tags...)
			s[i] = c.From
			i--
		}
		onlyAll = false
	}
	if onlyAll {
		return AllNodes{}, true
	}
	if len(tags) != 0 {
		// don't forget to move Save outside of Intersect at the end
		defer func() {
			if IsNull(sout) {
				return
			}
			sv := Save{From: sout, Tags: tags}
			var topt bool
			sout, topt = sv.Optimize(r)
			opt = opt || topt
		}()
	}
	if quads != nil {
		nq, qopt := quads.Optimize(r)
		if IsNull(nq) {
			return nil, true
		}
		opt = opt || qopt
		s = append(s, nq)
	}
	// TODO: intersect fixed
	if len(fixed) == 1 {
		fix := fixed[0]
		if len(s) == 1 {
			// try to push fixed down the tree
			switch sf := s[0].(type) {
			case QuadsAction:
				// TODO: accept an array of Fixed values
				if len(fix) == 1 {
					// we have a single value in Fixed that is intersected with HasA tree
					// this means we can add a new constraint: LinksTo(HasA.Dir, fixed)
					// result direction of HasA will be preserved
					fv := fix[0]
					if v := sf.Filter[sf.Result]; v != nil {
						// we have the same direction set as a fixed constraint - do filtering
						if values.ToKey(v) != values.ToKey(fv) {
							return nil, true
						} else {
							return sf, true
						}
					}
					sf = sf.Clone()
					sf.SetFilter(sf.Result, fv) // LinksTo(HasA.Dir, fixed)
					sf.Size = 0                 // re-calculate size
					ns, _ := sf.Optimize(r)
					return ns, true
				}
			case NodesFrom:
				if sq, ok := sf.Quads.(Quads); ok {
					// an optimization above is valid for NodesFrom+Quads as well
					// we can add the same constraint to Quads and remove Fixed
					qi := -1
					for i, qf := range sq {
						if qf.Dir == sf.Dir {
							qi = i
							break
						}
					}
					if qi < 0 {
						// no filter on this direction - append
						sf.Quads = append(Quads{
							{Dir: sf.Dir, Values: fix},
						}, sq...)
					} else {
						// already have a filter on this direction - push Fixed inside it
						sq = append(Quads{}, sq...)
						sf.Quads = sq
						qf := &sq[qi]
						qf.Values = IntersectShapes(fix, qf.Values)
					}
					return sf, true
				}
			}
		}
		// place fixed as a first iterator
		s = append(s, nil)
		copy(s[1:], s)
		s[0] = fix
	} else if len(fixed) > 1 {
		ns := make(Intersect, len(s)+len(fixed))
		for i, f := range fixed {
			ns[i] = f
		}
		copy(ns[len(fixed):], s)
		s = ns
	}
	if len(s) == 0 {
		return nil, true
	} else if len(s) == 1 {
		return s[0], true
	}
	// TODO: optimize order
	return s, opt
}

// IntersectOptional is the same as Intersect, but includes a list of optional query paths that will only affect tagging.
type IntersectOptional struct {
	Intersect Intersect
	Optional  []Shape
}

func (s *IntersectOptional) Add(arr ...Shape) {
	s.Intersect = append(s.Intersect, arr...)
}

func (s *IntersectOptional) AddOptional(arr ...Shape) {
	s.Optional = append(s.Optional, arr...)
}

func (s IntersectOptional) BuildIterator() iterator.Iterator {
	it := s.Intersect.BuildIterator()
	if len(s.Optional) == 0 {
		return it
	}
	and, ok := it.(*iterator.And)
	if !ok {
		// no, sorry, can only add optional to And
		and = iterator.NewAnd(it)
	}
	for _, sub := range s.Optional {
		and.AddOptionalIterator(sub.BuildIterator())
	}
	return and
}
func (s IntersectOptional) Optimize(r Optimizer) (Shape, bool) {
	// function to lazily reallocate a copy of Optional slice
	alloc := false
	opt := false
	realloc := func() {
		if !alloc {
			alloc = true
			s.Optional = append([]Shape{}, s.Optional...)
		}
		opt = true
	}
	remove := func(i int) {
		opt = true
		if n := len(s.Optional); i == n-1 {
			if !alloc {
				s.Optional = s.Optional[:i:i]
			} else {
				s.Optional = s.Optional[:i]
			}
			return
		}
		if !alloc {
			alloc = true
			old := s.Optional
			s.Optional = make([]Shape, 0, len(s.Optional)-1)
			s.Optional = append(s.Optional, old[:i]...)
			s.Optional = append(s.Optional, old[i+1:]...)
		} else {
			s.Optional = append(s.Optional[:i], s.Optional[i+1:]...)
		}
	}
	for i := 0; i < len(s.Optional); i++ {
		sub := s.Optional[i]
		// remove nulls from Optional
		if IsNull(sub) {
			remove(i)
			i--
			continue
		}
		// and optimize sub-queries
		if ns, opt2 := sub.Optimize(r); opt2 {
			if IsNull(ns) {
				remove(i)
				i--
			} else {
				realloc()
				s.Optional[i] = ns
			}
		}
	}
	var (
		ns   Shape
		opti bool
	)
	if len(s.Intersect) == 1 {
		// since we will automatically add Intersect{ns} in case of one node, we should not consider it
		// an optimization, or we can hit infinite loop
		ns, opti = s.Intersect[0].Optimize(r)
	} else {
		ns, opti = s.Intersect.Optimize(r)
	}
	opt = opt || opti
	if len(s.Optional) == 0 {
		return ns, true
	}
	and, ok := ns.(Intersect)
	if !ok {
		// no, sorry, can only add optional to Intersect
		and = Intersect{ns}
	}
	s.Intersect = and
	return s, opt
}
