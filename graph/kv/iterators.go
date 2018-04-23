package kv

import (
	"fmt"

	"github.com/cayleygraph/cayley/graph/iterator"
	"github.com/cayleygraph/cayley/graph/values"
	"github.com/cayleygraph/cayley/quad"
	"github.com/cayleygraph/cayley/query/shape"
)

func (qs *QuadStore) AllNodes() shape.Shape {
	return NewAllIterator(true, qs, nil)
}

func (qs *QuadStore) AllQuads() shape.Shape {
	return NewAllIterator(false, qs, nil)
}

func (qs *QuadStore) QuadIterator(dir quad.Direction, v values.Ref) iterator.Iterator {
	if v == nil {
		return iterator.NewNull()
	}
	vi, ok := v.(Int64Value)
	if !ok {
		return iterator.NewError(fmt.Errorf("unexpected node type: %T", v))
	}

	qs.indexes.RLock()
	all := qs.indexes.all
	qs.indexes.RUnlock()
	for _, ind := range all {
		if len(ind.Dirs) == 1 && ind.Dirs[0] == dir {
			return NewQuadIterator(qs, ind, []uint64{uint64(vi)})
		}
	}
	return NewAllIterator(false, qs, &constraint{
		dir: dir,
		val: vi,
	})
}
