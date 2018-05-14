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

package pathtest

import (
	"context"
	"reflect"
	"regexp"
	"sort"
	"testing"
	"time"

	. "github.com/cayleygraph/cayley/graph/path"
	"github.com/cayleygraph/cayley/graph/values"

	"github.com/cayleygraph/cayley/graph"
	"github.com/cayleygraph/cayley/graph/graphtest/testutil"
	"github.com/cayleygraph/cayley/quad"
	"github.com/cayleygraph/cayley/query/shape"
	_ "github.com/cayleygraph/cayley/writer"
	"github.com/stretchr/testify/require"
)

// This is a simple test graph.
//
//  +-------+                        +------+
//  | alice |-----                 ->| fred |<--
//  +-------+     \---->+-------+-/  +------+   \-+-------+
//                ----->| #bob# |       |         | emily |
//  +---------+--/  --->+-------+       |         +-------+
//  | charlie |    /                    v
//  +---------+   /                  +--------+
//    \---    +--------+             | #greg# |
//        \-->| #dani# |------------>+--------+
//            +--------+

func makeTestStore(t testing.TB, fnc *testutil.Database, quads ...quad.Quad) (graph.QuadStore, func()) {
	if len(quads) == 0 {
		quads = testutil.LoadGraph(t, "data/testdata.nq")
	}
	var (
		qs     graph.QuadStore
		opts   graph.Options
		closer = func() {}
	)
	if fnc != nil {
		qs, opts, closer = fnc.Run(t)
	} else {
		qs, _ = graph.NewQuadStore("memstore", "", nil)
	}
	_ = testutil.MakeWriter(t, qs, opts, quads...)
	return qs, closer
}

func runTopLevel(qs graph.QuadStore, path *Path) ([]quad.Value, error) {
	pb := path.Iterate(context.TODO(), qs)
	return pb.Paths(false).AllValues(qs)
}

func runTag(qs graph.QuadStore, path *Path, tag string) ([]quad.Value, error) {
	var out []quad.Value
	ctx := context.TODO()
	pb := path.Iterate(ctx, qs)
	err := pb.Paths(true).On(qs).TagEach(func(tags map[string]values.Ref) {
		if t, ok := tags[tag]; ok {
			v, _ := graph.ValueOf(ctx, qs, t)
			out = append(out, v)
		}
	})
	return out, err
}

// Define morphisms without a QuadStore

const (
	vFollows   = quad.IRI("follows")
	vAre       = quad.IRI("are")
	vStatus    = quad.IRI("status")
	vPredicate = quad.IRI("predicates")

	vCool       = quad.String("cool_person")
	vSmart      = quad.String("smart_person")
	vSmartGraph = quad.IRI("smart_graph")

	vAlice   = quad.IRI("alice")
	vBob     = quad.IRI("bob")
	vCharlie = quad.IRI("charlie")
	vDani    = quad.IRI("dani")
	vFred    = quad.IRI("fred")
	vGreg    = quad.IRI("greg")
	vEmily   = quad.IRI("emily")
)

var (
	grandfollows = StartMorphism().Out(vFollows).Out(vFollows)
)

var testSet = []struct {
	skip      bool
	message   string
	path      *Path
	expect    []quad.Value
	expectAlt [][]quad.Value
	tag       string
}{
	{
		message: "out",
		path:    StartPath(vAlice).Out(vFollows),
		expect:  []quad.Value{vBob},
	},
	{
		message: "out (any)",
		path:    StartPath(vBob).Out(),
		expect:  []quad.Value{vFred, vCool},
	},
	{
		message: "out (raw)",
		path:    StartPath(quad.Raw(vAlice.String())).Out(quad.Raw(vFollows.String())),
		expect:  []quad.Value{vBob},
	},
	{
		message: "in",
		path:    StartPath(vBob).In(vFollows),
		expect:  []quad.Value{vAlice, vCharlie, vDani},
	},
	{
		message: "in (any)",
		path:    StartPath(vBob).In(),
		expect:  []quad.Value{vAlice, vCharlie, vDani},
	},
	{
		message: "filter nodes",
		path:    StartPath().Filter(shape.CompareGT, quad.IRI("p")),
		expect:  []quad.Value{vPredicate, vSmartGraph, vStatus},
	},
	{
		message: "in with filter",
		path:    StartPath(vBob).In(vFollows).Filter(shape.CompareGT, quad.IRI("c")),
		expect:  []quad.Value{vCharlie, vDani},
	},
	{
		message: "in with regex",
		path:    StartPath(vBob).In(vFollows).Regex(regexp.MustCompile("ar?li.*e")),
		expect:  nil,
	},
	{
		message: "in with regex (include IRIs)",
		path:    StartPath(vBob).In(vFollows).RegexWithRefs(regexp.MustCompile("ar?li.*e")),
		expect:  []quad.Value{vAlice, vCharlie},
	},
	{
		message: "path Out",
		path:    StartPath(vBob).Out(StartPath(vPredicate).Out(vAre)),
		expect:  []quad.Value{vFred, vCool},
	},
	{
		message: "path Out (raw)",
		path:    StartPath(quad.Raw(vBob.String())).Out(StartPath(quad.Raw(vPredicate.String())).Out(quad.Raw(vAre.String()))),
		expect:  []quad.Value{vFred, vCool},
	},
	{
		message: "And",
		path: StartPath(vDani).Out(vFollows).And(
			StartPath(vCharlie).Out(vFollows)),
		expect: []quad.Value{vBob},
	},
	{
		message: "Or",
		path: StartPath(vFred).Out(vFollows).Or(
			StartPath(vAlice).Out(vFollows)),
		expect: []quad.Value{vBob, vGreg},
	},
	{
		message: "implicit All",
		path:    StartPath(),
		expect:  []quad.Value{vAlice, vBob, vCharlie, vDani, vEmily, vFred, vGreg, vFollows, vStatus, vCool, vPredicate, vAre, vSmartGraph, vSmart},
	},
	{
		message: "follow",
		path:    StartPath(vCharlie).Follow(StartMorphism().Out(vFollows).Out(vFollows)),
		expect:  []quad.Value{vBob, vFred, vGreg},
	},
	{
		message: "followR",
		path:    StartPath(vFred).FollowReverse(StartMorphism().Out(vFollows).Out(vFollows)),
		expect:  []quad.Value{vAlice, vCharlie, vDani},
	},
	{
		message: "is, tag, instead of FollowR",
		path:    StartPath().Tag("first").Follow(StartMorphism().Out(vFollows).Out(vFollows)).Is(vFred),
		expect:  []quad.Value{vAlice, vCharlie, vDani},
		tag:     "first",
	},
	{
		message: "Except to filter out a single vertex",
		path:    StartPath(vAlice, vBob).Except(StartPath(vAlice)),
		expect:  []quad.Value{vBob},
	},
	{
		message: "chained Except",
		path:    StartPath(vAlice, vBob, vCharlie).Except(StartPath(vBob)).Except(StartPath(vAlice)),
		expect:  []quad.Value{vCharlie},
	},
	{
		message: "Unique",
		path:    StartPath(vAlice, vBob, vCharlie).Out(vFollows).Unique(),
		expect:  []quad.Value{vBob, vDani, vFred},
	},
	{
		message: "simple save",
		path:    StartPath().Save(vStatus, "somecool"),
		tag:     "somecool",
		expect:  []quad.Value{vCool, vCool, vCool, vSmart, vSmart},
	},
	{
		message: "simple saveR",
		path:    StartPath(vCool).SaveReverse(vStatus, "who"),
		tag:     "who",
		expect:  []quad.Value{vGreg, vDani, vBob},
	},
	{
		message: "simple Has",
		path:    StartPath().Has(vStatus, vCool),
		expect:  []quad.Value{vGreg, vDani, vBob},
	},
	{
		message: "filter nodes with has",
		path: StartPath().HasFilter(vFollows, false, shape.Comparison{
			Op: shape.CompareGT, Val: quad.IRI("f"),
		}),
		expect: []quad.Value{vBob, vDani, vEmily, vFred},
	},
	{
		message: "string prefix",
		path: StartPath().Filters(shape.Wildcard{
			Pattern: `bo%`,
		}),
		expect: []quad.Value{vBob},
	},
	{
		message: "three letters and range",
		path: StartPath().Filters(shape.Wildcard{
			Pattern: `???`,
		}, shape.Comparison{
			Op: shape.CompareGT, Val: quad.IRI("b"),
		}),
		expect: []quad.Value{vBob},
	},
	{
		message: "part in string",
		path: StartPath().Filters(shape.Wildcard{
			Pattern: `%ed%`,
		}),
		expect: []quad.Value{vFred, vPredicate},
	},
	{
		message: "Limit",
		path:    StartPath().Has(vStatus, vCool).Limit(2),
		// TODO(dennwc): resolve this ordering issue
		expectAlt: [][]quad.Value{
			{vBob, vGreg},
			{vBob, vDani},
			{vDani, vGreg},
		},
	},
	{
		message: "Skip",
		path:    StartPath().Has(vStatus, vCool).Skip(2),
		expectAlt: [][]quad.Value{
			{vBob},
			{vDani},
			{vGreg},
		},
	},
	{
		message: "Skip and Limit",
		path:    StartPath().Has(vStatus, vCool).Skip(1).Limit(1),
		expectAlt: [][]quad.Value{
			{vBob},
			{vDani},
			{vGreg},
		},
	},
	{
		message: "double Has",
		path:    StartPath().Has(vStatus, vCool).Has(vFollows, vFred),
		expect:  []quad.Value{vBob},
	},
	{
		message: "simple HasReverse",
		path:    StartPath().HasReverse(vStatus, vBob),
		expect:  []quad.Value{vCool},
	},
	{
		message: ".Tag()-.Is()-.Back()",
		path:    StartPath(vBob).In(vFollows).Tag("foo").Out(vStatus).Is(vCool).Back("foo"),
		expect:  []quad.Value{vDani},
	},
	{
		message: "do multiple .Back()s",
		path:    StartPath(vEmily).Out(vFollows).Tag("f").Out(vFollows).Out(vStatus).Is(vCool).Back("f").In(vFollows).In(vFollows).Tag("acd").Out(vStatus).Is(vCool).Back("f"),
		tag:     "acd",
		expect:  []quad.Value{vDani},
	},
	{
		message: "Labels()",
		path:    StartPath(vGreg).Labels(),
		expect:  []quad.Value{vSmartGraph},
	},
	{
		message: "InPredicates()",
		path:    StartPath(vBob).InPredicates(),
		expect:  []quad.Value{vFollows},
	},
	{
		message: "OutPredicates()",
		path:    StartPath(vBob).OutPredicates(),
		expect:  []quad.Value{vFollows, vStatus},
	},
	{
		message: "SavePredicates(in)",
		path:    StartPath(vBob).SavePredicates(true, "pred"),
		expect:  []quad.Value{vFollows, vFollows, vFollows},
		tag:     "pred",
	},
	{
		message: "SavePredicates(out)",
		path:    StartPath(vBob).SavePredicates(false, "pred"),
		expect:  []quad.Value{vFollows, vStatus},
		tag:     "pred",
	},
	// Morphism tests
	{
		message: "simple morphism",
		path:    StartPath(vCharlie).Follow(grandfollows),
		expect:  []quad.Value{vGreg, vFred, vBob},
	},
	{
		message: "reverse morphism",
		path:    StartPath(vFred).FollowReverse(grandfollows),
		expect:  []quad.Value{vAlice, vCharlie, vDani},
	},
	// Context tests
	{
		message: "query without label limitation",
		path:    StartPath(vGreg).Out(vStatus),
		expect:  []quad.Value{vSmart, vCool},
	},
	{
		message: "query with label limitation",
		path:    StartPath(vGreg).LabelContext(vSmartGraph).Out(vStatus),
		expect:  []quad.Value{vSmart},
	},
	{
		message: "reverse context",
		path:    StartPath(vGreg).Tag("base").LabelContext(vSmartGraph).Out(vStatus).Tag("status").Back("base"),
		expect:  []quad.Value{vGreg},
	},
	// Optional tests
	{
		message: "save limits top level",
		path:    StartPath(vBob, vCharlie).Out(vFollows).Save(vStatus, "statustag"),
		expect:  []quad.Value{vBob, vDani},
	},
	{
		message: "optional still returns top level",
		path:    StartPath(vBob, vCharlie).Out(vFollows).SaveOptional(vStatus, "statustag"),
		expect:  []quad.Value{vBob, vFred, vDani},
	},
	{
		message: "optional has the appropriate tags",
		path:    StartPath(vBob, vCharlie).Out(vFollows).SaveOptional(vStatus, "statustag"),
		tag:     "statustag",
		expect:  []quad.Value{vCool, vCool},
	},
	{
		message: "composite paths (clone paths)",
		path: func() *Path {
			alice_path := StartPath(vAlice)
			_ = alice_path.Out(vFollows)

			return alice_path
		}(),
		expect: []quad.Value{vAlice},
	},
	{
		message: "follow recursive",
		path:    StartPath(vCharlie).FollowRecursive(vFollows, 0, nil),
		expect:  []quad.Value{vBob, vDani, vFred, vGreg},
	},
	{
		message: "follow recursive (limit depth)",
		path:    StartPath(vCharlie).FollowRecursive(vFollows, 1, nil),
		expect:  []quad.Value{vBob, vDani},
	},
	{
		message: "find non-existent",
		path:    StartPath(quad.IRI("<not-existing>")),
		expect:  nil,
	},
}

func RunTestMorphisms(t *testing.T, fnc *testutil.Database) {
	for _, ftest := range []func(*testing.T, *testutil.Database){
		testFollowRecursive,
	} {
		ftest(t, fnc)
	}
	qs, closer := makeTestStore(t, fnc)
	defer closer()

	for _, test := range testSet {
		name := test.message
		t.Run(name, func(t *testing.T) {
			if test.skip {
				t.SkipNow()
			}
			var (
				got []quad.Value
				err error
			)
			start := time.Now()
			if test.tag == "" {
				got, err = runTopLevel(qs, test.path)
			} else {
				got, err = runTag(qs, test.path, test.tag)
			}
			dt := time.Since(start)
			if err != nil {
				t.Error(err)
				return
			}
			sort.Sort(quad.ByValueString(got))
			var eq bool
			exp := test.expect
			if test.expectAlt != nil {
				for _, alt := range test.expectAlt {
					exp = alt
					sort.Sort(quad.ByValueString(exp))
					eq = reflect.DeepEqual(got, exp)
					if eq {
						break
					}
				}
			} else {
				sort.Sort(quad.ByValueString(test.expect))
				eq = reflect.DeepEqual(got, test.expect)
			}
			if !eq {
				t.Errorf("got: %v(%d) expected: %v(%d)", got, len(got), exp, len(exp))
			} else {
				t.Logf("%12v %v", dt, name)
			}
		})
	}
}

func testFollowRecursive(t *testing.T, fnc *testutil.Database) {
	qs, closer := makeTestStore(t, fnc, []quad.Quad{
		quad.MakeIRI("a", "parent", "b", ""),
		quad.MakeIRI("b", "parent", "c", ""),
		quad.MakeIRI("c", "parent", "d", ""),
		quad.MakeIRI("c", "labels", "tag", ""),
		quad.MakeIRI("d", "parent", "e", ""),
		quad.MakeIRI("d", "labels", "tag", ""),
	}...)
	defer closer()

	qu := StartPath(quad.IRI("a")).FollowRecursive(
		StartMorphism().Out(quad.IRI("parent")), 0, nil,
	).Has(quad.IRI("labels"), quad.IRI("tag"))

	expect := []quad.Value{quad.IRI("c"), quad.IRI("d")}

	t.Run("follows recursive order", func(t *testing.T) {
		got, err := runTopLevel(qs, qu)
		if err != nil {
			require.NoError(t, err)
		}
		sort.Sort(quad.ByValueString(got))
		sort.Sort(quad.ByValueString(expect))
		require.Equal(t, got, expect)
	})
}
