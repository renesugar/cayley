package shape

import (
	"github.com/cayleygraph/cayley/graph/iterator"
	"github.com/cayleygraph/cayley/quad"
)

// ValShape represent a query tree shape that returns values.
type ValShape interface {
	// BuildIterator constructs an iterator tree from a given shapes and binds it to QuadStore.
	BuildIterator() iterator.VIterator
	// Optimize runs an optimization pass over a query shape.
	//
	// It returns a bool that indicates if shape was replaced and should always return a copy of shape in this case.
	// In case no optimizations were made, it returns the same unmodified shape.
	//
	// If Optimizer is specified, it will be used instead of default optimizations.
	Optimize(r Optimizer) (ValShape, bool)
}

type ValMorphism interface {
	// Apply is a curried function that can generates a new iterator based on some prior iterator.
	Apply(ValShape) ValShape
}

type ValMorphismFunc func(ValShape) ValShape

func (m ValMorphismFunc) Apply(s ValShape) ValShape {
	return m(s)
}

// NullV represent an empty set. Mostly used as a safe alias for nil shape.
type NullV struct{}

func (NullV) BuildIterator() iterator.VIterator {
	return iterator.NewNullV()
}
func (s NullV) Optimize(r Optimizer) (ValShape, bool) {
	if r != nil {
		return r.OptimizeValShape(s)
	}
	return nil, true
}

// IsNullExpr safely checks if shape represents an empty set. It accounts for both Null and nil.
func IsNullExpr(s ValShape) bool {
	_, ok := s.(NullV)
	return s == nil || ok
}

var _ ValShape = Values{}

type Values []quad.Value

func (s *Values) Add(v ...quad.Value) {
	*s = append(*s, v...)
}

func (s Values) BuildIterator() iterator.VIterator {
	return iterator.NewValues(s...)
}

func (s Values) Optimize(r Optimizer) (ValShape, bool) {
	if len(s) == 0 {
		return nil, true
	}
	if r != nil {
		return r.OptimizeValShape(s)
	}
	return s, false
}

// Count returns a count of objects in source as a single value. It always returns exactly one value.
type Count struct {
	Values Shape
}

func (s Count) BuildIterator() iterator.VIterator {
	var it iterator.Iterator
	if IsNull(s.Values) {
		it = iterator.NewNull()
	} else {
		it = s.Values.BuildIterator()
	}
	return iterator.NewCount(it)
}

func (s Count) Optimize(r Optimizer) (ValShape, bool) {
	if IsNull(s.Values) {
		return Values{quad.Int(0)}, true
	}
	var opt bool
	s.Values, opt = s.Values.Optimize(r)
	if IsNull(s.Values) {
		return Values{quad.Int(0)}, true
	}
	if r != nil {
		ns, nopt := r.OptimizeValShape(s)
		return ns, opt || nopt
	}
	// TODO: ask QS to estimate size - if it exact, then we can use it
	return s, opt
}
