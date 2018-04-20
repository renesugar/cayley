package graphmock

import (
	"strconv"

	"github.com/cayleygraph/cayley/graph"
	"github.com/cayleygraph/cayley/graph/iterator"
	"github.com/cayleygraph/cayley/graph/values"
	"github.com/cayleygraph/cayley/quad"
	"github.com/cayleygraph/cayley/query/shape"
	"github.com/cayleygraph/cayley/query/shape/gshape"
)

var (
	_ values.Ref = IntVal(0)
	_ values.Ref = StringNode("")
)

type IntVal int

func (v IntVal) Key() interface{} { return v }

type StringNode string

func (s StringNode) Key() interface{} { return s }

// Oldstore is a mocked version of the QuadStore interface, for use in tests.
type Oldstore struct {
	Parse bool
	Data  []string
	Iter  shape.Shape
}

func (qs *Oldstore) valueAt(i int) quad.Value {
	if !qs.Parse {
		return quad.Raw(qs.Data[i])
	}
	iv, err := strconv.Atoi(qs.Data[i])
	if err == nil {
		return quad.Int(iv)
	}
	return quad.String(qs.Data[i])
}

func (qs *Oldstore) ValueOf(s quad.Value) values.Ref {
	if s == nil {
		return nil
	}
	for i := range qs.Data {
		if va := qs.valueAt(i); va != nil && s.String() == va.String() {
			return iterator.Int64Node(i)
		}
	}
	return nil
}

func (qs *Oldstore) ApplyDeltas([]graph.Delta, graph.IgnoreOpts) error { return nil }

func (qs *Oldstore) Quad(values.Ref) quad.Quad { return quad.Quad{} }

func (qs *Oldstore) QuadIterator(d quad.Direction, i values.Ref) shape.Shape {
	return qs.Iter
}

func (qs *Oldstore) AllNodes() shape.Shape { return shape.Null{} }

func (qs *Oldstore) AllQuads() shape.Shape { return shape.Null{} }

func (qs *Oldstore) NameOf(v values.Ref) quad.Value {
	switch v.(type) {
	case iterator.Int64Node:
		i := int(v.(iterator.Int64Node))
		if i < 0 || i >= len(qs.Data) {
			return nil
		}
		return qs.valueAt(i)
	case StringNode:
		if qs.Parse {
			return quad.String(v.(StringNode))
		}
		return quad.Raw(string(v.(StringNode)))
	default:
		return nil
	}
}

func (qs *Oldstore) Size() int64 { return 0 }

func (qs *Oldstore) DebugPrint() {}

func (qs *Oldstore) OptimizeIterator(it iterator.Iterator) (iterator.Iterator, bool) {
	return &iterator.Null{}, false
}

func (qs *Oldstore) Close() error { return nil }

func (qs *Oldstore) QuadDirection(values.Ref, quad.Direction) values.Ref {
	return iterator.Int64Node(0)
}

func (qs *Oldstore) RemoveQuad(t quad.Quad) {}

func (qs *Oldstore) Type() string { return "oldmockstore" }

type Store struct {
	Data []quad.Quad
}

var _ graph.QuadStore = &Store{}

func (qs *Store) ToValue(s shape.Shape) shape.ValShape {
	return gshape.ToValues(qs, s)
}

func (qs *Store) ToRef(s shape.ValShape) shape.Shape {
	return gshape.ToRefs(qs, s)
}
func (qs *Store) ValueOf(s quad.Value) values.Ref {
	return values.PreFetched(s)
}

func (qs *Store) ApplyDeltas([]graph.Delta, graph.IgnoreOpts) error { return nil }

type quadValue struct {
	q quad.Quad
}

func (q quadValue) Key() interface{} {
	return q.q.String()
}

func (qs *Store) Quad(v values.Ref) quad.Quad { return v.(quadValue).q }

func (qs *Store) NameOf(v values.Ref) quad.Value {
	if v == nil {
		return nil
	}
	return v.(values.PreFetchedValue).NameOf()
}

func (qs *Store) RemoveQuad(t quad.Quad) {}

func (qs *Store) QuadDirection(v values.Ref, d quad.Direction) values.Ref {
	return values.PreFetched(qs.Quad(v).Get(d))
}

func (qs *Store) Close() error { return nil }

func (qs *Store) DebugPrint() {}

func (qs *Store) QuadIterator(d quad.Direction, i values.Ref) shape.Shape {
	var fixed shape.Fixed
	v := i.(values.PreFetchedValue).NameOf()
	for _, q := range qs.Data {
		if q.Get(d) == v {
			fixed.Add(quadValue{q})
		}
	}
	return fixed
}

func (qs *Store) AllNodes() shape.Shape {
	set := make(map[string]bool)
	for _, q := range qs.Data {
		for _, d := range quad.Directions {
			n := qs.NameOf(values.PreFetched(q.Get(d)))
			if n != nil {
				set[n.String()] = true
			}
		}
	}
	var fixed shape.Fixed
	for k := range set {
		fixed.Add(values.PreFetched(quad.Raw(k)))
	}
	return fixed
}

func (qs *Store) AllQuads() shape.Shape {
	var fixed shape.Fixed
	for _, q := range qs.Data {
		fixed.Add(quadValue{q})
	}
	return fixed
}

func (qs *Store) Stats() graph.Stats {
	return graph.Stats{
		Links: int64(len(qs.Data)),
	}
}
