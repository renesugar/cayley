package kv

import (
	"github.com/cayleygraph/cayley/graph"
	"github.com/nwca/hidalgo/kv"
)

func init() {
	for _, r := range kv.List() {
		Register(r.Name, Registration{
			InitFunc: func(s string, options graph.Options) (kv.KV, error) {
				return r.OpenPath(s)
			},
			NewFunc: func(s string, options graph.Options) (kv.KV, error) {
				return r.OpenPath(s)
			},
			IsPersistent: !r.Volatile,
		})
	}
}
