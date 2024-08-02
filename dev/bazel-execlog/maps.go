package main

import "github.com/sourcegraph/sourcegraph/dev/bazel-execlog/proto"

type Map map[string][]*proto.SpawnExec

func (m Map) Intersection(o Map) (Map, Map) {
	m2 := make(Map)
	m3 := make(Map)
	for k, v := range m {
		if _, ok := o[k]; !ok {
			continue
		}
		m2[k] = v
		m3[k] = o[k]
	}
	return m2, m3
}

func (m Map) Minus(o Map) Map {
	m2 := make(Map)
	for k, v := range m {
		if _, ok := o[k]; ok {
			continue
		}
		m2[k] = v
	}
	return m2
}
