package scip

// NOTE(scip-migration): This file has been copied from .../uploads/internal/lsifstore/insert_util.go.

type treeNode[T traversable] struct {
	id       int
	children map[string]T
}

func newNodeWithID[T traversable](id int) treeNode[T] {
	return treeNode[T]{id: id, children: map[string]T{}}
}

type descriptor struct {
	id int
}

type DescriptorNode = treeNode[descriptor]
type NamespaceNode = treeNode[DescriptorNode]
type PackageVersionNode = treeNode[NamespaceNode]
type PackageNameNode = treeNode[PackageVersionNode]
type PackageManagerNode = treeNode[PackageNameNode]
type SchemeNode = treeNode[PackageManagerNode]

type traversable interface {
	traverse(name string, depth int, parentID *int, visit visitorFunc) error
}

type visitorFunc func(name string, id, depth int, parentID *int) error

func (n treeNode[T]) traverse(name string, depth int, parentID *int, visit visitorFunc) error {
	id := n.id
	idp := &id

	if err := visit(name, id, depth, parentID); err != nil {
		return err
	}

	for name, root := range n.children {
		if err := root.traverse(name, depth+1, idp, visit); err != nil {
			return err
		}
	}

	return nil
}

func (s descriptor) traverse(name string, depth int, parentID *int, visit visitorFunc) error {
	return visit(name, s.id, depth, parentID)
}

func traverse[T traversable](roots map[string]treeNode[T], visit visitorFunc) error {
	for name, root := range roots {
		if err := root.traverse(name, 0, nil, visit); err != nil {
			return err
		}
	}

	return nil
}

func getOrCreate[T any](m map[string]T, key string, factory func() T) T {
	if v, ok := m[key]; ok {
		return v
	}

	v := factory()
	m[key] = v
	return v
}
