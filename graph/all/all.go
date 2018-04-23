package all

import (
	// supported backends
	_ "github.com/cayleygraph/cayley/graph/kv/all"
	_ "github.com/cayleygraph/cayley/graph/memstore"
	_ "github.com/cayleygraph/cayley/graph/nosql/all"
	_ "github.com/cayleygraph/cayley/graph/sql/all"
)
