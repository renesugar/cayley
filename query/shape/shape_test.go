// Copyright 2014 The Cayley Authors. All rights reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package shape_test

import (
	"reflect"
	"testing"

	"github.com/cayleygraph/cayley/graph"
	"github.com/cayleygraph/cayley/graph/graphmock"
	"github.com/cayleygraph/cayley/graph/iterator"
	"github.com/cayleygraph/cayley/graph/values"
	"github.com/cayleygraph/cayley/quad"
	"github.com/cayleygraph/cayley/query"
	. "github.com/cayleygraph/cayley/query/shape"
	"github.com/cayleygraph/cayley/query/shape/gshape"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func intVal(v int) values.Ref {
	return graphmock.IntVal(v)
}

var _ Optimizer = ValLookup(nil)
var _ graph.QuadStore = ValLookup(nil)

type ValLookup map[quad.Value]values.Ref

func (qs ValLookup) OptimizeShape(s Shape) (Shape, bool) {
	return s, false // emulate dumb quad store
}
func (qs ValLookup) OptimizeExpr(s ValShape) (ValShape, bool) {
	return s, false // emulate dumb quad store
}
func (qs ValLookup) ValueOf(v quad.Value) values.Ref {
	return qs[v]
}
func (qs ValLookup) ToValue(s Shape) ValShape {
	return gshape.ToValues(qs, s)
}
func (qs ValLookup) ToRef(s ValShape) Shape {
	return gshape.ToRefs(qs, s)
}
func (ValLookup) ApplyDeltas(_ []graph.Delta, _ graph.IgnoreOpts) error {
	panic("not implemented")
}
func (ValLookup) Quad(_ values.Ref) quad.Quad {
	panic("not implemented")
}
func (ValLookup) QuadIterator(_ quad.Direction, _ values.Ref) Shape {
	return fakeAll{}
}
func (ValLookup) AllNodes() Shape {
	return fakeAll{}
}

func (ValLookup) AllQuads() Shape {
	return fakeAll{}
}
func (ValLookup) NameOf(_ values.Ref) quad.Value {
	panic("not implemented")
}
func (ValLookup) Stats() graph.Stats {
	panic("not implemented")
}
func (ValLookup) Close() error {
	panic("not implemented")
}
func (ValLookup) QuadDirection(_ values.Ref, _ quad.Direction) values.Ref {
	panic("not implemented")
}
func (ValLookup) Type() string {
	panic("not implemented")
}

func emptySet() Shape {
	return gshape.NodesFrom{
		Dir: quad.Predicate,
		Quads: gshape.Intersect{gshape.Quads{
			{Dir: quad.Object,
				Values: gshape.Lookup{quad.IRI("not-existent")},
			},
		}},
	}
}

type fakeAll struct{}

func (fakeAll) BuildIterator() iterator.Iterator {
	panic("implement me")
}

func (s fakeAll) Optimize(r Optimizer) (Shape, bool) {
	return s, false
}

var optimizeCases = []struct {
	name   string
	from   Shape
	expect Shape
	opt    bool
	qs     ValLookup
}{
	{
		name:   "all",
		from:   fakeAll{},
		opt:    false,
		expect: fakeAll{},
	},
	{
		name: "page min limit",
		from: Page{
			Limit: 5,
			From: Page{
				Limit: 3,
				From:  fakeAll{},
			},
		},
		opt: true,
		expect: Page{
			Limit: 3,
			From:  fakeAll{},
		},
	},
	{
		name: "page skip and limit",
		from: Page{
			Skip: 3, Limit: 3,
			From: Page{
				Skip: 2, Limit: 5,
				From: fakeAll{},
			},
		},
		opt: true,
		expect: Page{
			Skip: 5, Limit: 2,
			From: fakeAll{},
		},
	},
	{
		name: "intersect quads and lookup resolution",
		from: gshape.Intersect{
			gshape.Quads{
				{Dir: quad.Subject, Values: gshape.Lookup{quad.IRI("bob")}},
			},
			gshape.Quads{
				{Dir: quad.Object, Values: gshape.Lookup{quad.IRI("alice")}},
			},
		},
		opt: true,
		expect: gshape.Quads{
			{Dir: quad.Subject, Values: Fixed{intVal(1)}},
			{Dir: quad.Object, Values: Fixed{intVal(2)}},
		},
		qs: ValLookup{
			quad.IRI("bob"):   intVal(1),
			quad.IRI("alice"): intVal(2),
		},
	},
	{
		name: "intersect nodes, remove all, join intersects",
		from: gshape.Intersect{
			fakeAll{},
			gshape.NodesFrom{Dir: quad.Subject, Quads: gshape.Quads{}},
			gshape.Intersect{
				gshape.Lookup{quad.IRI("alice")},
				Unique{gshape.NodesFrom{Dir: quad.Object, Quads: gshape.Quads{}}},
			},
		},
		opt: true,
		expect: gshape.Intersect{
			Fixed{intVal(1)},
			gshape.QuadsAction{Result: quad.Subject},
			Unique{gshape.QuadsAction{Result: quad.Object}},
		},
		qs: ValLookup{
			quad.IRI("alice"): intVal(1),
		},
	},
	{
		name: "push Save out of intersect",
		from: gshape.Intersect{
			Save{
				Tags: []string{"id"},
				From: gshape.NodesFrom{Dir: quad.Subject, Quads: gshape.Quads{}},
			},
			Unique{gshape.NodesFrom{Dir: quad.Object, Quads: gshape.Quads{}}},
		},
		opt: true,
		expect: Save{
			Tags: []string{"id"},
			From: gshape.Intersect{
				gshape.QuadsAction{Result: quad.Subject},
				Unique{gshape.QuadsAction{Result: quad.Object}},
			},
		},
	},
	{
		name: "collapse empty set",
		from: gshape.Intersect{gshape.Quads{
			{Dir: quad.Subject, Values: Union{
				Unique{emptySet()},
			}},
		}},
		opt:    true,
		expect: Null{},
	},
	{ // remove "all nodes" in intersect, merge Fixed and order them first
		name: "remove all in intersect and reorder",
		from: gshape.Intersect{
			fakeAll{},
			Fixed{intVal(1), intVal(2)},
			Save{From: fakeAll{}, Tags: []string{"all"}},
			Fixed{intVal(2)},
		},
		opt: true,
		expect: Save{
			From: gshape.Intersect{
				Fixed{intVal(1), intVal(2)},
				Fixed{intVal(2)},
			},
			Tags: []string{"all"},
		},
	},
	{
		name: "remove HasA-LinksTo pairs",
		from: gshape.NodesFrom{
			Dir: quad.Subject,
			Quads: gshape.Quads{{
				Dir:    quad.Subject,
				Values: Fixed{intVal(1)},
			}},
		},
		opt:    true,
		expect: Fixed{intVal(1)},
	},
	{ // pop fixed tags to the top of the tree
		name: "pop fixed tags",
		from: gshape.NodesFrom{Dir: quad.Subject, Quads: gshape.Quads{
			gshape.QuadFilter{Dir: quad.Predicate, Values: gshape.Intersect{
				FixedTags{
					Tags: map[string]values.Ref{"foo": intVal(1)},
					On: gshape.NodesFrom{Dir: quad.Subject,
						Quads: gshape.Quads{
							gshape.QuadFilter{Dir: quad.Object, Values: FixedTags{
								Tags: map[string]values.Ref{"bar": intVal(2)},
								On:   Fixed{intVal(3)},
							}},
						},
					},
				},
			}},
		}},
		opt: true,
		expect: FixedTags{
			Tags: map[string]values.Ref{"foo": intVal(1), "bar": intVal(2)},
			On: gshape.NodesFrom{Dir: quad.Subject, Quads: gshape.Quads{
				gshape.QuadFilter{Dir: quad.Predicate, Values: gshape.QuadsAction{
					Result: quad.Subject,
					Filter: map[quad.Direction]values.Ref{quad.Object: intVal(3)},
				}},
			}},
		},
	},
	{ // remove optional empty set from intersect
		name: "remove optional empty set",
		from: gshape.IntersectOptional{
			Intersect: gshape.Intersect{
				fakeAll{},
				Save{From: fakeAll{}, Tags: []string{"all"}},
				Fixed{intVal(2)},
			},
			Optional: []Shape{
				Save{
					From: emptySet(),
					Tags: []string{"name"},
				},
			},
		},
		opt: true,
		expect: Save{
			From: Fixed{intVal(2)},
			Tags: []string{"all"},
		},
	},
	{ // push fixed node from intersect into nodes.quads
		name: "push fixed into nodes.quads",
		from: gshape.Intersect{
			Fixed{intVal(1)},
			gshape.NodesFrom{
				Dir: quad.Subject,
				Quads: gshape.Quads{
					{Dir: quad.Predicate, Values: Fixed{intVal(2)}},
					{
						Dir: quad.Object,
						Values: gshape.NodesFrom{
							Dir: quad.Subject,
							Quads: gshape.Quads{
								{Dir: quad.Predicate, Values: Fixed{intVal(2)}},
							},
						},
					},
				},
			},
		},
		opt: true,
		expect: gshape.NodesFrom{
			Dir: quad.Subject,
			Quads: gshape.Quads{
				{Dir: quad.Subject, Values: Fixed{intVal(1)}},
				{Dir: quad.Predicate, Values: Fixed{intVal(2)}},
				{
					Dir: quad.Object,
					Values: gshape.QuadsAction{
						Result: quad.Subject,
						Filter: map[quad.Direction]values.Ref{
							quad.Predicate: intVal(2),
						},
					},
				},
			},
		},
	},
}

func TestOptimize(t *testing.T) {
	for _, c := range optimizeCases {
		t.Run(c.name, func(t *testing.T) {
			qs := c.qs
			got, opt := query.Optimize(c.from, qs)
			assert.Equal(t, c.expect, got)
			assert.Equal(t, c.opt, opt)
		})
	}
}

func TestWalk(t *testing.T) {
	var s Shape = gshape.NodesFrom{
		Dir: quad.Subject,
		Quads: gshape.Quads{
			{Dir: quad.Subject, Values: Fixed{intVal(1)}},
			{Dir: quad.Predicate, Values: Fixed{intVal(2)}},
			{
				Dir: quad.Object,
				Values: gshape.QuadsAction{
					Result: quad.Subject,
					Filter: map[quad.Direction]values.Ref{
						quad.Predicate: intVal(2),
					},
				},
			},
		},
	}
	var types []string
	Walk(s, func(s Shape) bool {
		types = append(types, reflect.TypeOf(s).String())
		return true
	})
	require.Equal(t, []string{
		"gshape.NodesFrom",
		"gshape.Quads",
		"shape.Fixed",
		"shape.Fixed",
		"gshape.QuadsAction",
	}, types)
}
