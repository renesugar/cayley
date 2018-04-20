package shape

import (
	"github.com/cayleygraph/cayley/graph/iterator"
	"github.com/cayleygraph/cayley/quad"
)

func AddFilters(nodes ValShape, filters ...ValueFilter) ValShape {
	if len(filters) == 0 {
		return nodes
	}
	if s, ok := nodes.(Filter); ok {
		arr := make([]ValueFilter, 0, len(s.Filters)+len(filters))
		arr = append(arr, s.Filters...)
		arr = append(arr, filters...)
		return Filter{From: s.From, Filters: arr}
	}
	if nodes == nil {
		panic("values shape not set")
	}
	return Filter{
		From:    nodes,
		Filters: filters,
	}
}

func Compare(nodes ValShape, op iterator.Operator, v quad.Value) ValShape {
	return AddFilters(nodes, Comparison{Op: op, Val: v})
}
