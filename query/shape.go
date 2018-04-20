package query

import (
	"context"

	"github.com/cayleygraph/cayley/clog"
	"github.com/cayleygraph/cayley/graph"
	"github.com/cayleygraph/cayley/graph/iterator"
	"github.com/cayleygraph/cayley/query/shape"
	"github.com/cayleygraph/cayley/query/shape/gshape"
)

type Shape = shape.Shape
type ValShape = shape.ValShape

// Optimize applies generic optimizations for the tree.
// If quad store is specified it will also resolve Lookups and apply any specific optimizations.
// Should not be used with Simplify - it will fold query to a compact form again.
func Optimize(s Shape, qs graph.QuadStore) (Shape, bool) {
	if s == nil {
		return nil, false
	}
	qs = graph.Unwrap(qs)
	var opt bool
	if qs != nil {
		// resolve all lookups earlier
		s, opt = s.Optimize(resolveValues{qs: qs})
	}
	if s == nil {
		return shape.Null{}, true
	}
	// generic optimizations
	var opt1 bool
	s, opt1 = s.Optimize(nil)
	if s == nil {
		return shape.Null{}, true
	}
	opt = opt || opt1
	// apply quadstore-specific optimizations
	if so, ok := qs.(shape.Optimizer); ok && s != nil {
		var opt2 bool
		s, opt2 = s.Optimize(so)
		opt = opt || opt2
	}
	if s == nil {
		return shape.Null{}, true
	}
	return s, opt
}

type resolveValues struct {
	qs graph.QuadStore
}

func (r resolveValues) OptimizeShape(s Shape) (Shape, bool) {
	if l, ok := s.(gshape.Bindable); ok {
		return l.BindTo(r.qs), true
	}
	return s, false
}

func (r resolveValues) OptimizeExpr(s ValShape) (ValShape, bool) {
	return s, false
}

// BuildIterator optimizes the shape and builds a corresponding iterator tree.
func BuildIterator(qs graph.QuadStore, s Shape) iterator.Iterator {
	qs = graph.Unwrap(qs)
	if s != nil {
		if clog.V(2) {
			clog.Infof("shape: %#v", s)
		}
		s, _ = Optimize(s, qs)
		if clog.V(2) {
			clog.Infof("optimized: %#v", s)
		}
	}
	if shape.IsNull(s) {
		return iterator.NewNull()
	}
	return s.BuildIterator()
}

func Iterate(ctx context.Context, qs graph.QuadStore, s Shape) *graph.IterateChain {
	it := BuildIterator(qs, s)
	return graph.Iterate(ctx, it).On(qs)
}
