package graphlog

import (
	"bytes"
	"sort"

	"github.com/cayleygraph/cayley/graph"
	"github.com/cayleygraph/cayley/graph/values"
	"github.com/cayleygraph/cayley/quad"
)

type Op interface {
	isOp()
}

var (
	_ Op = NodeUpdate{}
	_ Op = QuadUpdate{}
)

type NodeUpdate struct {
	Hash   values.ValueHash
	Val    quad.Value
	RefInc int
}

func (NodeUpdate) isOp() {}

type QuadUpdate struct {
	Ind  int
	Quad values.QuadHash
	Del  bool
}

func (QuadUpdate) isOp() {}

type Deltas struct {
	IncNode []NodeUpdate
	DecNode []NodeUpdate
	QuadAdd []QuadUpdate
	QuadDel []QuadUpdate
}

func SplitDeltas(in []graph.Delta) *Deltas {
	hnodes := make(map[values.ValueHash]*NodeUpdate, len(in)*2)
	quadAdd := make([]QuadUpdate, 0, len(in))
	quadDel := make([]QuadUpdate, 0, len(in)/2)
	var nadd, ndel int
	for i, d := range in {
		dn := 0
		switch d.Action {
		case graph.Add:
			dn = +1
			nadd++
		case graph.Delete:
			dn = -1
			ndel++
		default:
			panic("unknown action")
		}
		var q values.QuadHash
		for _, dir := range quad.Directions {
			v := d.Quad.Get(dir)
			if v == nil {
				continue
			}
			h := values.HashOf(v)
			q.Set(dir, h)
			n := hnodes[h]
			if n == nil {
				n = &NodeUpdate{Hash: h, Val: v}
				hnodes[h] = n
			}
			n.RefInc += dn
		}
		u := QuadUpdate{Ind: i, Quad: q, Del: d.Action == graph.Delete}
		if !u.Del {
			quadAdd = append(quadAdd, u)
		} else {
			quadDel = append(quadDel, u)
		}
	}
	incNodes := make([]NodeUpdate, 0, nadd)
	decNodes := make([]NodeUpdate, 0, ndel)
	for _, n := range hnodes {
		if n.RefInc >= 0 {
			incNodes = append(incNodes, *n)
		} else {
			decNodes = append(decNodes, *n)
		}
	}
	sort.Slice(incNodes, func(i, j int) bool {
		return bytes.Compare(incNodes[i].Hash[:], incNodes[j].Hash[:]) < 0
	})
	sort.Slice(decNodes, func(i, j int) bool {
		return bytes.Compare(decNodes[i].Hash[:], decNodes[j].Hash[:]) < 0
	})
	hnodes = nil
	return &Deltas{
		IncNode: incNodes, DecNode: decNodes,
		QuadAdd: quadAdd, QuadDel: quadDel,
	}
}
