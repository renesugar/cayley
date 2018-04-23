package query

import (
	"context"

	"github.com/cayleygraph/cayley/clog"
	"github.com/cayleygraph/cayley/graph"
	"github.com/cayleygraph/cayley/graph/iterator"
	"github.com/cayleygraph/cayley/query/shape"
	"github.com/cayleygraph/cayley/query/shape/gshape"
)

const debugOptimizer = false

type Shape = shape.Shape
type ValShape = shape.ValShape

// On binds a query to a specific QuadStore.
func On(qs graph.QuadStore, s Shape) Shape {
	if s == nil {
		return shape.Null{}
	} else if qs == nil {
		return s
	}
	s, _ = s.Optimize(bindTo{qs: qs})
	return s
}

type bindTo struct {
	qs graph.QuadStore
}

func (r bindTo) OptimizeShape(s Shape) (Shape, bool) {
	if l, ok := s.(gshape.Bindable); ok {
		s, _ = l.BindTo(r.qs).Optimize(r)
		return s, true
	}
	return s, false
}

func (r bindTo) OptimizeValShape(s ValShape) (ValShape, bool) {
	if l, ok := s.(gshape.ValBindable); ok {
		s, _ = l.BindTo(r.qs).Optimize(r)
		return s, true
	}
	return s, false
}

// Optimize applies generic optimizations for the tree.
// If quad store is specified it will also resolve Lookups and apply any specific optimizations.
// Should not be used with Simplify - it will fold query to a compact form again.
func Optimize(s Shape, qs graph.QuadStore) (Shape, bool) {
	if s == nil {
		return nil, false
	}
	qs = graph.Unwrap(qs)
	if s == nil {
		return shape.Null{}, true
	}
	// generic optimizations
	var opt bool
	s, opt = s.Optimize(nil)
	if s == nil {
		return shape.Null{}, true
	}
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

// BuildIterator optimizes the shape and builds a corresponding iterator tree.
func BuildIterator(qs graph.QuadStore, s Shape) iterator.Iterator {
	qs = graph.Unwrap(qs)
	if s != nil {
		if clog.V(2) || debugOptimizer {
			clog.Infof("shape: %#v", s)
		}
		s, _ = Optimize(s, qs)
		if clog.V(2) || debugOptimizer {
			clog.Infof("optimized: %#v", s)
		}
	}
	if shape.IsNull(s) {
		return iterator.NewNull()
	}
	s = On(qs, s)
	if clog.V(2) || debugOptimizer {
		clog.Infof("bound: %#v", s)
	}
	return s.BuildIterator()
}

func Iterate(ctx context.Context, qs graph.QuadStore, s Shape) *graph.IterateChain {
	it := BuildIterator(qs, s)
	return graph.Iterate(ctx, it).On(qs)
}
