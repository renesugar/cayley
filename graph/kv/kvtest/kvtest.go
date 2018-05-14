package kvtest

import (
	"context"
	"reflect"
	"testing"

	"github.com/cayleygraph/cayley/graph"
	"github.com/cayleygraph/cayley/graph/graphtest"
	"github.com/cayleygraph/cayley/graph/graphtest/testutil"
	"github.com/cayleygraph/cayley/graph/kv"
	"github.com/cayleygraph/cayley/graph/values"
	"github.com/cayleygraph/cayley/quad"
	"github.com/cayleygraph/cayley/query"
	"github.com/cayleygraph/cayley/query/shape/gshape"
	hkv "github.com/nwca/hidalgo/kv"
	"github.com/stretchr/testify/require"
)

type DatabaseFunc func(t testing.TB) (hkv.KV, graph.Options, func())

type Config struct {
	AlwaysRunIntegration bool
}

func NewQuadStoreFunc(gen DatabaseFunc, c Config) testutil.Database {
	return testutil.Database{
		Config: graphtest.Config{
			NoPrimitives:         true,
			AlwaysRunIntegration: c.AlwaysRunIntegration,
		},
		Run: func(t testing.TB) (graph.QuadStore, graph.Options, func()) {
			return NewQuadStore(t, gen)
		},
	}
}

func NewQuadStore(t testing.TB, gen DatabaseFunc) (graph.QuadStore, graph.Options, func()) {
	db, opt, closer := gen(t)
	err := kv.Init(db, opt)
	if err != nil {
		db.Close()
		closer()
		require.Fail(t, "init failed", "%v", err)
	}
	kdb, err := kv.New(db, opt)
	if err != nil {
		db.Close()
		closer()
		require.Fail(t, "create failed", "%v", err)
	}
	return kdb, opt, func() {
		kdb.Close()
		closer()
	}
}

func TestAll(t *testing.T, gen DatabaseFunc, conf *Config) {
	if conf == nil {
		conf = &Config{}
	}
	qsgen := NewQuadStoreFunc(gen, *conf)
	t.Run("qs", func(t *testing.T) {
		graphtest.TestAll(t, qsgen)
	})
	t.Run("optimize", func(t *testing.T) {
		testOptimize(t, gen, conf)
	})
}

func testOptimize(t *testing.T, gen DatabaseFunc, _ *Config) {
	ctx := context.TODO()
	qs, opts, closer := NewQuadStore(t, gen)
	defer closer()

	testutil.MakeWriter(t, qs, opts, graphtest.MakeQuadSet()...)

	// With an linksto-fixed pair
	s, _ := query.Optimize(gshape.Quads{
		{Dir: quad.Object, Values: gshape.Lookup{quad.Raw("F")}},
	}, qs)
	newIt := query.BuildIterator(qs, s)

	oldIt := query.BuildIterator(qs, gshape.Quads{
		{Dir: quad.Object, Values: gshape.Lookup{quad.Raw("F")}},
	})
	if _, ok := newIt.(*kv.QuadIterator); !ok {
		t.Errorf("Optimized iterator type does not match original, got: %T", newIt)
	}

	newQuads := graphtest.IteratedQuads(t, qs, newIt)
	oldQuads := graphtest.IteratedQuads(t, qs, oldIt)
	if !reflect.DeepEqual(newQuads, oldQuads) {
		t.Errorf("Optimized iteration does not match original")
	}

	oldIt.Next(ctx)
	oldResults := make(map[string]values.Ref)
	oldIt.TagResults(oldResults)
	newIt.Next(ctx)
	newResults := make(map[string]values.Ref)
	newIt.TagResults(newResults)
	if !reflect.DeepEqual(newResults, oldResults) {
		t.Errorf("Discordant tag results, new:%v old:%v", newResults, oldResults)
	}
}

func BenchmarkAll(t *testing.B, gen DatabaseFunc, conf *Config) {
	if conf == nil {
		conf = &Config{}
	}
	qsgen := NewQuadStoreFunc(gen, *conf)
	t.Run("qs", func(t *testing.B) {
		graphtest.BenchmarkAll(t, qsgen)
	})
}
