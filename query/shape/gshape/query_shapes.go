package gshape

import (
	"fmt"

	"github.com/cayleygraph/cayley/graph"
	"github.com/cayleygraph/cayley/graph/iterator"
	"github.com/cayleygraph/cayley/graph/iterator/giterator"
	"github.com/cayleygraph/cayley/graph/values"
	"github.com/cayleygraph/cayley/quad"
	"github.com/cayleygraph/cayley/query/shape"
)

var ErrNoQuadStore = fmt.Errorf("query should be bound to quad store")

func errNoQsIterator() iterator.Iterator {
	return iterator.NewError(ErrNoQuadStore)
}

type Bindable interface {
	shape.Shape
	BindTo(qs graph.QuadStore) shape.Shape
}

type ValBindable interface {
	shape.ValShape
	BindTo(qs graph.QuadStore) shape.ValShape
}

var _ Bindable = AllNodes{}

// AllNodes represents all nodes in QuadStore.
type AllNodes struct{}

func (s AllNodes) BindTo(qs graph.QuadStore) shape.Shape {
	return qs.AllNodes()
}

func (s AllNodes) BuildIterator() iterator.Iterator {
	return errNoQsIterator()
}
func (s AllNodes) Optimize(r shape.Optimizer) (shape.Shape, bool) {
	if qs, ok := r.(graph.QuadStore); ok {
		return s.BindTo(qs), true
	}
	if r != nil {
		return r.OptimizeShape(s)
	}
	return s, false
}

// Except excludes a set on nodes from a source. If source is nil, AllNodes is assumed.
type Except struct {
	Exclude shape.Shape // nodes to exclude
	From    shape.Shape // a set of all nodes to exclude from
}

func (s Except) BuildIterator() iterator.Iterator {
	var all iterator.Iterator
	if s.From != nil {
		all = s.From.BuildIterator()
	} else {
		panic("From should be set")
	}
	if shape.IsNull(s.Exclude) {
		return all
	}
	return iterator.NewNot(s.Exclude.BuildIterator(), all)
}
func (s Except) Optimize(r shape.Optimizer) (shape.Shape, bool) {
	if s.From == nil {
		// won't even try; wait for BuildIterator to panic
		return s, false
	}
	var opt bool
	s.Exclude, opt = s.Exclude.Optimize(r)

	var opta bool
	s.From, opta = s.From.Optimize(r)

	opt = opt || opta
	if r != nil {
		ns, nopt := r.OptimizeShape(s)
		return ns, opt || nopt
	}
	if shape.IsNull(s.Exclude) {
		return s.From, true
	} else if _, ok := s.Exclude.(AllNodes); ok {
		return nil, true
	}
	return s, opt
}

var _ Bindable = Lookup{}

// Lookup is a static set of values that must be resolved to nodes by QuadStore.
type Lookup []quad.Value

func (s *Lookup) Add(v ...quad.Value) {
	*s = append(*s, v...)
}
func (s Lookup) bindTo(qs graph.Resolver) shape.Shape {
	return qs.ToRef(shape.Values(s))
}
func (s Lookup) BindTo(qs graph.QuadStore) shape.Shape {
	return s.bindTo(qs)
}
func (s Lookup) BuildIterator() iterator.Iterator {
	return errNoQsIterator()
}
func (s Lookup) Optimize(r shape.Optimizer) (shape.Shape, bool) {
	if r == nil {
		return s, false
	}
	ns, opt := r.OptimizeShape(s)
	if opt {
		return ns, true
	}
	if qs, ok := r.(graph.Resolver); ok {
		ns, opt = s.bindTo(qs), true
	}
	return ns, opt
}

// QuadFilter is a constraint used to filter quads that have a certain set of values on a given direction.
// Analog of LinksTo iterator.
type QuadFilter struct {
	Dir    quad.Direction
	Values shape.Shape
}

// buildIterator is not exposed to force to use Quads and group filters together.
func (s QuadFilter) buildShape(qs graph.QuadIndexer) shape.Shape {
	if s.Values == nil {
		return shape.Null{}
	} else if v, ok := shape.One(s.Values); ok {
		return qs.QuadIterator(s.Dir, v)
	}
	if s.Dir == quad.Any {
		panic("direction is not set")
	}
	return linksTo{qs: qs, dir: s.Dir, values: s.Values}
}

type linksTo struct {
	qs     graph.QuadIndexer
	dir    quad.Direction
	values shape.Shape
}

func (s linksTo) BuildIterator() iterator.Iterator {
	sub := s.values.BuildIterator()
	return giterator.NewLinksTo(s.qs, sub, s.dir)
}

func (s linksTo) Optimize(r shape.Optimizer) (shape.Shape, bool) {
	var opt bool
	s.values, opt = s.values.Optimize(r)
	if r == nil {
		return s, opt
	}
	if sn, ok := r.OptimizeShape(s); ok {
		return sn, true
	}
	return s, opt
}

var _ Bindable = Quads{}

// Quads is a selector of quads with a given set of node constraints. Empty or nil Quads is equivalent to AllQuads.
// Equivalent to And(AllQuads,LinksTo*) iterator tree.
type Quads []QuadFilter

func (s *Quads) Intersect(q ...QuadFilter) {
	*s = append(*s, q...)
}
func (s Quads) BindTo(qs graph.QuadStore) shape.Shape {
	if len(s) == 0 {
		return qs.AllQuads()
	}
	all := make(Intersect, 0, len(s))
	for _, sub := range s {
		ss := sub.buildShape(qs)
		if shape.IsNull(ss) {
			return shape.Null{}
		}
		all = append(all, ss)
	}
	return all
}
func (s Quads) BuildIterator() iterator.Iterator {
	return errNoQsIterator()
}
func (s Quads) Optimize(r shape.Optimizer) (shape.Shape, bool) {
	var opt bool
	sw := 0
	realloc := func() {
		if !opt {
			opt = true
			nq := make(Quads, len(s))
			copy(nq, s)
			s = nq
		}
	}
	// TODO: multiple constraints on the same dir -> merge as Intersect on Values of this dir
	for i := 0; i < len(s); i++ {
		f := s[i]
		if f.Values == nil {
			return nil, true
		}
		v, ok := f.Values.Optimize(r)
		if v == nil {
			return nil, true
		}
		if ok {
			realloc()
			s[i].Values = v
		}
		switch s[i].Values.(type) {
		case shape.Fixed:
			realloc()
			s[sw], s[i] = s[i], s[sw]
			sw++
		}
	}
	if r != nil {
		ns, nopt := r.OptimizeShape(s)
		return ns, opt || nopt
	}
	return s, opt
}

type nodesFrom struct {
	qs    graph.QuadIndexer
	dir   quad.Direction
	quads shape.Shape
}

func (s nodesFrom) BuildIterator() iterator.Iterator {
	sub := s.quads.BuildIterator()
	return giterator.NewHasA(s.qs, sub, s.dir)
}

func (s nodesFrom) Optimize(r shape.Optimizer) (shape.Shape, bool) {
	var opt bool
	s.quads, opt = s.quads.Optimize(r)
	if r == nil {
		return s, opt
	}
	if sn, ok := r.OptimizeShape(s); ok {
		return sn, true
	}
	return s, opt
}

var _ Bindable = NodesFrom{}

// NodesFrom extracts nodes on a given direction from source quads. Similar to HasA iterator.
type NodesFrom struct {
	Dir   quad.Direction
	Quads shape.Shape
}

func (s NodesFrom) BindTo(qs graph.QuadStore) shape.Shape {
	if shape.IsNull(s.Quads) {
		return shape.Null{}
	}
	if s.Dir == quad.Any {
		panic("direction is not set")
	}
	return nodesFrom{qs: qs, dir: s.Dir, quads: s.Quads}
}
func (s NodesFrom) BuildIterator() iterator.Iterator {
	return errNoQsIterator()
}
func (s NodesFrom) Optimize(r shape.Optimizer) (shape.Shape, bool) {
	if shape.IsNull(s.Quads) {
		return nil, true
	}
	var opt bool
	s.Quads, opt = s.Quads.Optimize(r)
	if r != nil {
		// ignore default optimizations
		ns, nopt := r.OptimizeShape(s)
		return ns, opt || nopt
	}
	q, ok := s.Quads.(Quads)
	if !ok {
		return s, opt
	}
	// HasA(x, LinksTo(x, y)) == y
	if len(q) == 1 && q[0].Dir == s.Dir {
		return q[0].Values, true
	}
	// collect all fixed tags and push them up the tree
	var (
		tags  map[string]values.Ref
		nquad Quads
	)
	for i, f := range q {
		if ft, ok := f.Values.(shape.FixedTags); ok {
			if tags == nil {
				// allocate map and clone quad filters
				tags = make(map[string]values.Ref)
				nquad = make([]QuadFilter, len(q))
				copy(nquad, q)
				q = nquad
			}
			q[i].Values = ft.On
			for k, v := range ft.Tags {
				tags[k] = v
			}
		}
	}
	if tags != nil {
		// re-run optimization without fixed tags
		ns, _ := NodesFrom{Dir: s.Dir, Quads: q}.Optimize(r)
		return shape.FixedTags{On: ns, Tags: tags}, true
	}
	var (
		// if quad filter contains one fixed value, it will be added to the map
		filt map[quad.Direction]values.Ref
		// if we see a Save from AllNodes, we will write it here, since it's a Save on quad direction
		save map[quad.Direction][]string
		// how many filters are recognized
		n int
	)
	for _, f := range q {
		if v, ok := shape.One(f.Values); ok {
			if filt == nil {
				filt = make(map[quad.Direction]values.Ref)
			}
			if _, ok := filt[f.Dir]; ok {
				return s, opt // just to be safe
			}
			filt[f.Dir] = v
			n++
		} else if sv, ok := f.Values.(shape.Save); ok {
			if _, ok = sv.From.(AllNodes); ok {
				if save == nil {
					save = make(map[quad.Direction][]string)
				}
				save[f.Dir] = append(save[f.Dir], sv.Tags...)
				n++
			}
		}
	}
	if n == len(q) {
		// if all filters were recognized we can merge this tree as a single iterator with multiple
		// constraints and multiple save commands over the same set of quads
		ns, _ := QuadsAction{
			Result: s.Dir, // this is still a HasA, remember?
			Filter: filt,
			Save:   save,
		}.Optimize(r)
		return ns, true
	}
	// TODO
	return s, opt
}

var _ shape.Composite = QuadsAction{}

// QuadsAction represents a set of actions that can be done to a set of quads in a single scan pass.
// It filters quads according to Filter constraints (equivalent of LinksTo), tags directions using tags in Save field
// and returns a specified quad direction as result of the iterator (equivalent of HasA).
// Optionally, Size field may be set to indicate an approximate number of quads that will be returned by this query.
type QuadsAction struct {
	Size   int64 // approximate size; zero means undefined
	Result quad.Direction
	Save   map[quad.Direction][]string
	Filter map[quad.Direction]values.Ref
}

func (s *QuadsAction) SetFilter(d quad.Direction, v values.Ref) {
	if s.Filter == nil {
		s.Filter = make(map[quad.Direction]values.Ref)
	}
	s.Filter[d] = v
}

func (s QuadsAction) Clone() QuadsAction {
	if n := len(s.Save); n != 0 {
		s2 := make(map[quad.Direction][]string, n)
		for k, v := range s.Save {
			s2[k] = v
		}
		s.Save = s2
	} else {
		s.Save = nil
	}
	if n := len(s.Filter); n != 0 {
		f2 := make(map[quad.Direction]values.Ref, n)
		for k, v := range s.Filter {
			f2[k] = v
		}
		s.Filter = f2
	} else {
		s.Filter = nil
	}
	return s
}
func (s QuadsAction) simplify() NodesFrom {
	q := make(Quads, 0, len(s.Save)+len(s.Filter))
	for dir, val := range s.Filter {
		q = append(q, QuadFilter{Dir: dir, Values: shape.Fixed{val}})
	}
	for dir, tags := range s.Save {
		q = append(q, QuadFilter{Dir: dir, Values: shape.Save{From: AllNodes{}, Tags: tags}})
	}
	return NodesFrom{Dir: s.Result, Quads: q}
}
func (s QuadsAction) Simplify() shape.Shape {
	return s.simplify()
}
func (s QuadsAction) BuildIterator() iterator.Iterator {
	h := s.simplify()
	return h.BuildIterator()
}
func (s QuadsAction) Optimize(r shape.Optimizer) (shape.Shape, bool) {
	if r != nil {
		if sn, ok := r.OptimizeShape(s); ok {
			return sn, true
		}
		if sn, ok := r.OptimizeShape(s.Simplify()); ok {
			return sn, true
		}
		return s, false
	}
	// if optimizer has stats for quad indexes we can use them to do more
	ind, ok := r.(shape.QuadIndexer)
	if !ok {
		return s, false
	}
	if s.Size > 0 { // already optimized; specific for QuadIndexer optimization
		return s, false
	}
	sz, exact := ind.SizeOfIndex(s.Filter)
	if !exact {
		return s, false
	}
	s.Size = sz // computing size is already an optimization
	if sz == 0 {
		// nothing here, collapse the tree
		return nil, true
	} else if sz == 1 {
		// only one quad matches this set of filters
		// try to load it from quad store, do all operations and bake result as a fixed node/tags
		if q, ok := ind.LookupQuadIndex(s.Filter); ok {
			fx := shape.Fixed{q.Get(s.Result)}
			if len(s.Save) == 0 {
				return fx, true
			}
			ft := shape.FixedTags{On: fx, Tags: make(map[string]values.Ref)}
			for d, tags := range s.Save {
				for _, t := range tags {
					ft.Tags[t] = q.Get(d)
				}
			}
			return ft, true
		}
	}
	if sz < int64(shape.MaterializeThreshold) {
		// if this set is small enough - materialize it
		return shape.Materialize{Values: s, Size: int(sz)}, true
	}
	return s, true
}

func ToValues(qs giterator.Namer, refs shape.Shape) shape.ValShape {
	return toValues{qs: qs, refs: refs}
}

var _ ValBindable = RefsToValues{}

type RefsToValues struct {
	Refs shape.Shape
}

func (s RefsToValues) BuildIterator() iterator.VIterator {
	return iterator.NewErrorV(ErrNoQuadStore)
}

func (s RefsToValues) Optimize(r shape.Optimizer) (shape.ValShape, bool) {
	var opt bool
	s.Refs, opt = s.Refs.Optimize(r)
	if r == nil {
		return s, opt
	}
	if sn, ok := r.OptimizeValShape(s); ok {
		return sn, true
	}
	return s, opt
}

func (s RefsToValues) BindTo(qs graph.QuadStore) shape.ValShape {
	return qs.ToValue(s.Refs)
}

var _ Bindable = ValuesToRefs{}

type ValuesToRefs struct {
	Values shape.ValShape
}

func (s ValuesToRefs) BuildIterator() iterator.Iterator {
	return errNoQsIterator()
}

func (s ValuesToRefs) Optimize(r shape.Optimizer) (shape.Shape, bool) {
	var opt bool
	s.Values, opt = s.Values.Optimize(r)
	if r == nil {
		return s, opt
	}
	if sn, ok := r.OptimizeShape(s); ok {
		return sn, true
	}
	return s, opt
}

func (s ValuesToRefs) BindTo(qs graph.QuadStore) shape.Shape {
	return qs.ToRef(s.Values)
}

type toValues struct {
	qs   giterator.Namer
	refs shape.Shape
}

func (s toValues) Optimize(r shape.Optimizer) (shape.ValShape, bool) {
	var opt bool
	s.refs, opt = s.refs.Optimize(r)
	return s, opt
}

func (s toValues) BuildIterator() iterator.VIterator {
	return giterator.NewRefToValue(s.qs, s.refs.BuildIterator())
}

func ToRefs(qs giterator.Namer, vals shape.ValShape) shape.Shape {
	return toRefs{qs: qs, vals: vals}
}

type toRefs struct {
	qs   giterator.Namer
	vals shape.ValShape
}

func (s toRefs) Optimize(r shape.Optimizer) (shape.Shape, bool) {
	var opt bool
	s.vals, opt = s.vals.Optimize(r)
	return s, opt
}

func (s toRefs) BuildIterator() iterator.Iterator {
	return giterator.NewValueToRef(s.qs, s.vals.BuildIterator())
}

func CompareNodes(nodes shape.Shape, op shape.CmpOperator, v quad.Value) shape.Shape {
	if to, ok := nodes.(ValuesToRefs); ok {
		to.Values = shape.Compare(to.Values, op, v)
		return to
	}
	return ValuesToRefs{
		Values: shape.Compare(
			RefsToValues{Refs: nodes}, op, v,
		),
	}
}
