package kv

import (
	"github.com/cayleygraph/cayley/graph/iterator"
	"github.com/cayleygraph/cayley/graph/values"
	"github.com/cayleygraph/cayley/quad"
	"github.com/cayleygraph/cayley/query/shape"
)

func (qs *QuadStore) AllNodes() shape.Shape {
	return scanPrimitives{qs: qs, nodes: true}
}

func (qs *QuadStore) AllQuads() shape.Shape {
	return scanPrimitives{qs: qs, nodes: false}
}

type scanPrimitives struct {
	qs    *QuadStore
	nodes bool
	c     *constraint
}

func (s scanPrimitives) BuildIterator() iterator.Iterator {
	return NewAllIterator(s.nodes, s.qs, s.c)
}

func (s scanPrimitives) Optimize(r shape.Optimizer) (shape.Shape, bool) {
	return s, false
}

type scanIndex struct {
	qs   *QuadStore
	ind  QuadIndex
	pref []uint64
}

func (s scanIndex) BuildIterator() iterator.Iterator {
	return NewQuadIterator(s.qs, s.ind, s.pref)
}

func (s scanIndex) Optimize(r shape.Optimizer) (shape.Shape, bool) {
	return s, false
}

func (qs *QuadStore) QuadIterator(dir quad.Direction, v values.Ref) shape.Shape {
	vi, ok := v.(Int64Value)
	if !ok {
		return shape.Null{}
	}
	qs.indexes.RLock()
	all := qs.indexes.all
	qs.indexes.RUnlock()
	for _, ind := range all {
		if len(ind.Dirs) == 1 && ind.Dirs[0] == dir {
			return scanIndex{qs: qs, ind: ind, pref: []uint64{uint64(vi)}}
		}
	}
	return scanPrimitives{qs: qs, nodes: false, c: &constraint{
		dir: dir,
		val: vi,
	}}
}
