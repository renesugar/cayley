package testutil

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/cayleygraph/cayley/graph"
	"github.com/cayleygraph/cayley/quad"
	"github.com/cayleygraph/cayley/quad/nquads"
	"github.com/cayleygraph/cayley/writer"
	"github.com/stretchr/testify/require"
)

type Config struct {
	NoPrimitives bool
	UnTyped      bool // converts all values to Raw representation
	TimeInMs     bool
	TimeInMcs    bool
	TimeRound    bool
	PageSize     int // result page size for pagination (large iterator) tests

	OptimizesComparison bool

	AlwaysRunIntegration bool // always run integration tests

	SkipDeletedFromIterator  bool
	SkipSizeCheckAfterDelete bool
}

type Database struct {
	Config Config
	Run    func(t testing.TB) (graph.QuadStore, graph.Options, func())
}

func LoadGraph(t testing.TB, path string) []quad.Quad {
	var (
		f   *os.File
		err error
	)
	const levels = 5
	for i := 0; i < levels; i++ {
		f, err = os.Open(path)
		if i+1 < levels && os.IsNotExist(err) {
			path = filepath.Join("../", path)
		} else if err != nil {
			t.Fatalf("Failed to open %q: %v", path, err)
		} else {
			break
		}
	}
	defer f.Close()
	dec := nquads.NewReader(f, false)
	quads, err := quad.ReadAll(dec)
	if err != nil {
		t.Fatalf("Failed to Unmarshal: %v", err)
	}
	return quads
}

func MakeWriter(t testing.TB, qs graph.QuadStore, opts graph.Options, data ...quad.Quad) graph.QuadWriter {
	w, err := writer.NewSingleReplication(qs, opts)
	require.NoError(t, err)
	if len(data) > 0 {
		err = w.AddQuadSet(data)
		require.NoError(t, err)
	}
	return w
}
