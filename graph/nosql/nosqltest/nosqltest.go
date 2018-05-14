package nosqltest

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/cayleygraph/cayley/graph"
	"github.com/cayleygraph/cayley/graph/graphtest"
	"github.com/cayleygraph/cayley/graph/graphtest/testutil"
	gnosql "github.com/cayleygraph/cayley/graph/nosql"
	"github.com/nwca/hidalgo/legacy/nosql"
	"github.com/nwca/hidalgo/legacy/nosql/nosqltest"
)

func toConfig(c nosql.Traits) graphtest.Config {
	return graphtest.Config{
		NoPrimitives:             true,
		TimeInMs:                 c.TimeInMs,
		OptimizesComparison:      true,
		SkipDeletedFromIterator:  true,
		SkipSizeCheckAfterDelete: true,
	}
}

func NewQuadStore(t testing.TB, gen nosqltest.Database) (graph.QuadStore, graph.Options, func()) {
	db, closer := gen.Run(t)
	err := gnosql.Init(db, nil)
	if err != nil {
		db.Close()
		closer()
		require.Fail(t, "init failed", "%v", err)
	}
	tr := gen.Traits
	kdb, err := gnosql.NewQuadStore(db, &tr, nil)
	if err != nil {
		db.Close()
		closer()
		require.Fail(t, "create failed", "%v", err)
	}
	return kdb, nil, func() {
		kdb.Close()
		closer()
	}
}

func TestAll(t *testing.T, gen nosqltest.Database) {
	graphtest.TestAll(t, testutil.Database{
		Config: toConfig(gen.Traits),
		Run: func(t testing.TB) (graph.QuadStore, graph.Options, func()) {
			return NewQuadStore(t, gen)
		},
	})
}

func BenchmarkAll(t *testing.B, gen nosqltest.Database) {
	graphtest.BenchmarkAll(t, testutil.Database{
		Config: toConfig(gen.Traits),
		Run: func(t testing.TB) (graph.QuadStore, graph.Options, func()) {
			return NewQuadStore(t, gen)
		},
	})
}
