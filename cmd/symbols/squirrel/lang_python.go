package squirrel

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	sitter "github.com/smacker/go-tree-sitter"

	"github.com/sourcegraph/sourcegraph/internal/types"
)

func (s *SquirrelService) getDefPython(ctx context.Context, node Node) (ret *Node, err error) {
	defer s.onCall(node, String(node.Type()), lazyNodeStringer(&ret))()

	switch node.Type() {
	case "identifier":
		ident := node.Content(node.Contents)

		cur := node.Node

		for {
			prev := cur
			cur = cur.Parent()
			if cur == nil {
				s.breadcrumb(node, "getDefPython: ran out of parents")
				return nil, nil
			}

			switch cur.Type() {

			case "module":
				found := s.findNodeInScopePython(swapNode(node, cur), ident)
				if found != nil {
					return found, nil
				}
				return s.getDefInImports(ctx, swapNode(node, cur), ident)

			case "attribute":
				object := cur.ChildByFieldName("object")
				if object == nil {
					s.breadcrumb(node, "getDefPython: attribute has no object field")
					return nil, nil
				}
				attribute := cur.ChildByFieldName("attribute")
				if attribute == nil {
					s.breadcrumb(node, "getDefPython: attribute has no attribute field")
					return nil, nil
				}
				if nodeId(object) == nodeId(prev) {
					continue
				}
				return s.getFieldPython(ctx, swapNode(node, object), attribute.Content(node.Contents))

			case "for_statement":
				left := cur.ChildByFieldName("left")
				if left == nil {
					continue
				}
				if left.Type() == "identifier" {
					if left.Content(node.Contents) == ident {
						return swapNodePtr(node, left), nil
					}
				}
				continue

			case "with_statement":
				for _, child := range children(cur) {
					if child.Type() == "with_clause" {
						for _, child := range children(cur) {
							if child.Type() == "with_item" {
								value := child.ChildByFieldName("value")
								if value == nil {
									continue
								}
								if value.Type() == "identifier" && value.Content(node.Contents) == ident {
									return swapNodePtr(node, value), nil
								}
							}
						}
					}
				}
				continue

			case "except_clause":
				if cur.NamedChildCount() < 1 {
					continue
				}
				// (except_clause
				//  (as_pattern (identifier) alias: (as_pattern_target (identifier) @definition)))
				//        vvvvvvvvv identifier
				//                     v identifier
				//                      v block
				// except Exception as e:
				asPattern := cur.NamedChild(0)
				if asPattern == nil || asPattern.Type() != "as_pattern" {
					continue
				}
				asPatternTarget := asPattern.ChildByFieldName("alias")
				if asPatternTarget == nil || asPatternTarget.Type() != "as_pattern_target" {
					continue
				}
				exceptIdent := asPatternTarget.Child(0)
				if exceptIdent == nil || exceptIdent.Type() != "identifier" {
					continue
				}
				if exceptIdent.Content(node.Contents) == ident {
					return swapNodePtr(node, exceptIdent), nil
				}
				continue

			case "lambda":
				// Check the parameters
				query := `
					(lambda parameters:
						(lambda_parameters [
							(identifier) @ident
							(default_parameter name: (identifier) @ident)
							(list_splat_pattern (identifier) @ident)
							(dictionary_splat_pattern (identifier) @ident)
						])
					)
				`
				captures := allCaptures(query, swapNode(node, cur))
				for _, capture := range captures {
					if capture.Content(capture.Contents) == ident {
						return swapNodePtr(node, capture.Node), nil
					}
				}
				continue

			case "function_definition":
				// Check the function name and parameters
				name := cur.ChildByFieldName("name")
				if name != nil && name.Type() == "identifier" && name.Content(node.Contents) == ident {
					return swapNodePtr(node, name), nil
				}
				parameters := cur.ChildByFieldName("parameters")
				if parameters == nil {
					continue
				}
				query := `
					(parameters [
						(identifier) @ident
						(default_parameter name: (identifier) @ident)
						(list_splat_pattern (identifier) @ident)
						(dictionary_splat_pattern (identifier) @ident)

						(typed_parameter [
							(identifier) @ident
							(list_splat_pattern (identifier) @ident)
							(dictionary_splat_pattern (identifier) @ident)
						])
						(typed_default_parameter name: (identifier) @ident)
					])
				`
				captures := allCaptures(query, swapNode(node, parameters))
				for _, capture := range captures {
					if capture.Content(capture.Contents) == ident {
						return swapNodePtr(node, capture.Node), nil
					}
				}

				// Check the function body by doing an in-order traversal of all expression-statements
				// scoped to this function.
				body := cur.ChildByFieldName("body")
				if body == nil || body.Type() != "block" {
					s.breadcrumb(swapNode(node, cur), "getDefPython: expected function_definition to have a block body")
					continue
				}
				found := s.findNodeInScopePython(swapNode(node, body), ident)
				if found != nil {
					return found, nil
				}

				continue

			case "block":
				continue // Blocks are not scopes in Python, so keep looking up the tree

			// Skip all other nodes
			default:
				continue
			}
		}

	// No other nodes have a definition
	default:
		return nil, nil
	}
}

func (s *SquirrelService) findNodeInScopePython(block Node, ident string) (ret *Node) {
	defer s.onCall(block, &Tuple{String(block.Type()), String(ident)}, lazyNodeStringer(&ret))()

	for i := range int(block.NamedChildCount()) {
		child := block.NamedChild(i)

		switch child.Type() {
		case "function_definition":
			name := child.ChildByFieldName("name")
			if name != nil && name.Type() == "identifier" && name.Content(block.Contents) == ident {
				return swapNodePtr(block, name)
			}
			continue
		case "class_definition":
			name := child.ChildByFieldName("name")
			if name != nil && name.Type() == "identifier" && name.Content(block.Contents) == ident {
				return swapNodePtr(block, name)
			}
			continue
		case "expression_statement":
			query := `(expression_statement (assignment left: (identifier) @ident))`
			captures := allCaptures(query, swapNode(block, child))
			for _, capture := range captures {
				if capture.Content(capture.Contents) == ident {
					return swapNodePtr(block, capture.Node)
				}
			}
			continue
		case "if_statement":
			var found *Node
			next := child.ChildByFieldName("consequence")
			if next == nil {
				return nil
			}
			found = s.findNodeInScopePython(swapNode(block, next), ident)
			if found != nil {
				return found
			}
			elseClause := child.ChildByFieldName("alternative")
			if elseClause == nil {
				continue
			}
			next = elseClause.ChildByFieldName("body")
			if next == nil {
				return nil
			}
			found = s.findNodeInScopePython(swapNode(block, next), ident)
			if found != nil {
				return found
			}
			continue
		case "while_statement":
			fallthrough
		case "for_statement":
			next := child.ChildByFieldName("body")
			if next == nil {
				return nil
			}
			found := s.findNodeInScopePython(swapNode(block, next), ident)
			if found != nil {
				return found
			}
			continue
		case "try_statement":
			next := child.ChildByFieldName("body")
			if next == nil {
				return nil
			}
			found := s.findNodeInScopePython(swapNode(block, next), ident)
			if found != nil {
				return found
			}
			for j := range int(child.NamedChildCount()) {
				tryChild := child.NamedChild(j)
				if tryChild.Type() == "except_clause" {
					for k := range int(tryChild.NamedChildCount()) {
						exceptChild := tryChild.NamedChild(k)
						if exceptChild.Type() == "block" {
							next := exceptChild
							found := s.findNodeInScopePython(swapNode(block, next), ident)
							if found != nil {
								return found
							}
						}
					}
				}
			}
			continue
		default:
			continue
		}
	}

	return nil
}

func (s *SquirrelService) getFieldPython(ctx context.Context, object Node, field string) (ret *Node, err error) {
	defer s.onCall(object, &Tuple{String(object.Type()), String(field)}, lazyNodeStringer(&ret))()

	ty, err := s.getTypeDefPython(ctx, object)
	if err != nil {
		return nil, err
	}
	if ty == nil {
		return nil, nil
	}
	return s.lookupFieldPython(ctx, ty, field)
}

func (s *SquirrelService) lookupFieldPython(ctx context.Context, ty TypePython, field string) (ret *Node, err error) {
	defer s.onCall(ty.node(), &Tuple{String(ty.variant()), String(field)}, lazyNodeStringer(&ret))()

	switch ty2 := ty.(type) {
	case ModuleTypePython:
		return s.findNodeInScopePython(ty2.module, field), nil
	case ClassTypePython:
		body := ty2.def.ChildByFieldName("body")
		if body == nil {
			return nil, nil
		}
		for _, child := range children(body) {
			switch child.Type() {
			case "expression_statement":
				query := `(expression_statement (assignment left: (identifier) @ident))`
				captures := allCaptures(query, swapNode(ty2.def, child))
				for _, capture := range captures {
					if capture.Content(capture.Contents) == field {
						return swapNodePtr(ty2.def, capture.Node), nil
					}
				}
				continue
			case "function_definition":
				name := child.ChildByFieldName("name")
				if name == nil {
					continue
				}
				if name.Content(ty2.def.Contents) == field {
					return swapNodePtr(ty2.def, name), nil
				}
				if name.Content(ty2.def.Contents) == "__init__" {
					query := `
						(expression_statement
							(assignment
								left: (attribute
									object: (identifier) @object
									attribute: (identifier) @attribute
								)
							)
						)
					`
					var found *Node
					forEachCapture(query, swapNode(ty2.def, child), func(nameToNode map[string]Node) {
						object, ok := nameToNode["object"]
						if !ok || object.Content(ty2.def.Contents) != "self" {
							return
						}
						attribute, ok := nameToNode["attribute"]
						if !ok || attribute.Content(ty2.def.Contents) != field {
							return
						}
						found = &attribute
					})
					if found != nil {
						return found, nil
					}
				}
			case "class_definition":
				name := child.ChildByFieldName("name")
				if name == nil {
					continue
				}
				if name.Content(ty2.def.Contents) == field {
					return swapNodePtr(ty2.def, name), nil
				}
			}
		}
		for _, super := range getSuperclassesPython(ty2.def) {
			found, err := s.getFieldPython(ctx, super, field)
			if err != nil {
				return nil, err
			}
			if found != nil {
				return found, nil
			}
		}
		return nil, nil
	case FnTypePython:
		s.breadcrumb(ty.node(), fmt.Sprintf("lookupFieldPython: unexpected object type %s", ty.variant()))
		return nil, nil
	case PrimTypePython:
		s.breadcrumb(ty.node(), fmt.Sprintf("lookupFieldPython: unexpected object type %s", ty.variant()))
		return nil, nil
	default:
		s.breadcrumb(ty.node(), fmt.Sprintf("lookupFieldPython: unrecognized type variant %q", ty.variant()))
		return nil, nil
	}
}

func (s *SquirrelService) getTypeDefPython(ctx context.Context, node Node) (ret TypePython, err error) {
	defer s.onCall(node, String(node.Type()), lazyTypePythonStringer(&ret))()

	onIdent := func() (TypePython, error) {
		found, err := s.getDefPython(ctx, node)
		if err != nil {
			return nil, err
		}
		if found == nil {
			return nil, nil
		}
		if isRecursiveDefinitionPython(node, *found) {
			return nil, nil
		}
		return s.defToTypePython(ctx, *found)
	}

	switch node.Type() {
	case "type":
		for _, child := range children(node.Node) {
			return s.getTypeDefPython(ctx, swapNode(node, child))
		}
		return nil, nil
	case "identifier":
		return onIdent()
	case "attribute":
		object := node.ChildByFieldName("object")
		if object == nil {
			return nil, nil
		}
		attribute := node.ChildByFieldName("attribute")
		if attribute == nil {
			return nil, nil
		}
		objectType, err := s.getTypeDefPython(ctx, swapNode(node, object))
		if err != nil {
			return nil, err
		}
		if objectType == nil {
			return nil, nil
		}
		found, err := s.lookupFieldPython(ctx, objectType, attribute.Content(node.Contents))
		if err != nil {
			return nil, err
		}
		if found == nil {
			return nil, nil
		}
		return s.defToTypePython(ctx, *found)
	case "call":
		fn := node.ChildByFieldName("function")
		if fn == nil {
			return nil, nil
		}
		ty, err := s.getTypeDefPython(ctx, swapNode(node, fn))
		if err != nil {
			return nil, err
		}
		if ty == nil {
			return nil, nil
		}
		switch ty2 := ty.(type) {
		case FnTypePython:
			return ty2.ret, nil
		case ClassTypePython:
			return ty2, nil
		default:
			s.breadcrumb(ty.node(), fmt.Sprintf("getTypeDefPython: expected function, got %q", ty.variant()))
			return nil, nil
		}
	default:
		s.breadcrumb(node, fmt.Sprintf("getTypeDefPython: unrecognized node type %q", node.Type()))
		return nil, nil
	}
}

func (s *SquirrelService) getDefInImports(ctx context.Context, program Node, ident string) (ret *Node, err error) {
	defer s.onCall(program, &Tuple{String(program.Type()), String(ident)}, lazyNodeStringer(&ret))()

	findModuleOrPkg := func(moduleOrPkg *sitter.Node) *Node {
		if moduleOrPkg == nil {
			return nil
		}

		path := program.RepoCommitPath.Path
		path = strings.TrimSuffix(path, filepath.Base(path))
		path = strings.TrimSuffix(path, "/")

		var dottedName *sitter.Node
		if moduleOrPkg.Type() == "relative_import" {
			if moduleOrPkg.NamedChildCount() < 1 {
				return nil
			}
			importPrefix := moduleOrPkg.NamedChild(0)
			if importPrefix == nil || importPrefix.Type() != "import_prefix" {
				return nil
			}
			dots := int(importPrefix.ChildCount())
			for range dots - 1 {
				path = strings.TrimSuffix(path, filepath.Base(path))
				path = strings.TrimSuffix(path, "/")
			}
			if moduleOrPkg.NamedChildCount() > 1 {
				dottedName = moduleOrPkg.NamedChild(1)
			}
		} else {
			dottedName = moduleOrPkg
		}

		if dottedName == nil || dottedName.Type() != "dotted_name" {
			return nil
		}

		for _, component := range children(dottedName) {
			if component.Type() != "identifier" {
				return nil
			}
			path = filepath.Join(path, component.Content(program.Contents))
		}
		// TODO support package imports
		path += ".py"
		result, _ := s.parse(ctx, types.RepoCommitPath{
			Repo:   program.RepoCommitPath.Repo,
			Commit: program.RepoCommitPath.Commit,
			Path:   path,
		})
		return result
	}

	findModuleIdent := func(module *sitter.Node, ident2 string) *Node {
		foundModule := findModuleOrPkg(module)
		if foundModule != nil {
			return s.findNodeInScopePython(*foundModule, ident2)
		}
		return nil
	}

	query := `[
		(import_statement) @import
		(import_from_statement) @import
	]`
	captures := allCaptures(query, program)
	for _, stmt := range captures {
		switch stmt.Type() {
		case "import_statement":
			for _, importChild := range children(stmt.Node) {
				switch importChild.Type() {
				case "dotted_name":
					if importChild.NamedChildCount() == 0 {
						continue
					}
					lastChild := importChild.NamedChild(int(importChild.NamedChildCount()) - 1)
					if lastChild == nil || lastChild.Type() != "identifier" {
						continue
					}
					if lastChild.Content(program.Contents) != ident {
						continue
					}
					return findModuleOrPkg(importChild), nil
				case "aliased_import":
					alias := importChild.ChildByFieldName("alias")
					if alias == nil || alias.Type() != "identifier" {
						continue
					}
					if alias.Content(program.Contents) != ident {
						continue
					}
					name := importChild.ChildByFieldName("name")
					return findModuleOrPkg(name), nil
				}
			}
		case "import_from_statement":
			moduleName := stmt.ChildByFieldName("module_name")
			if moduleName == nil {
				continue
			}

			// Advance a cursor to just past the "import" keyword
			i := 0
			for ; i < int(stmt.ChildCount()); i++ {
				if stmt.Child(i).Type() == "import" {
					i++
					break
				}
			}
			if i == 0 || i >= int(stmt.ChildCount()) {
				continue
			}

			// Check if it's a wildcard import
			if stmt.Child(i).Type() == "wildcard_import" {
				found := findModuleIdent(moduleName, ident)
				if found != nil {
					return found, nil
				}
			}

			// Loop through the imports
			for ; i < int(stmt.ChildCount()); i++ {
				child := stmt.Child(i)
				if !child.IsNamed() {
					continue
				}
				switch child.Type() {
				case "dotted_name":
					if child.NamedChildCount() == 0 {
						continue
					}
					childIdent := child.NamedChild(0)
					if childIdent.Type() != "identifier" {
						continue
					}
					if childIdent.Content(program.Contents) != ident {
						continue
					}
					found := findModuleIdent(moduleName, ident)
					if found != nil {
						return found, nil
					}
				case "aliased_import":
					alias := child.ChildByFieldName("alias")
					if alias == nil || alias.Type() != "identifier" {
						continue
					}
					if alias.Content(program.Contents) != ident {
						continue
					}
					name := child.ChildByFieldName("name")
					if name == nil || name.Type() != "dotted_name" {
						continue
					}
					if name.NamedChildCount() == 0 {
						continue
					}
					nameIdent := name.NamedChild(0)
					if nameIdent == nil || nameIdent.Type() != "identifier" {
						continue
					}
					found := findModuleIdent(moduleName, nameIdent.Content(program.Contents))
					if found != nil {
						return found, nil
					}
				}
			}
		}
	}

	return nil, nil
}

func (s *SquirrelService) defToTypePython(ctx context.Context, def Node) (TypePython, error) {
	if def.Node.Type() == "module" {
		return (TypePython)(ModuleTypePython{module: def}), nil
	}

	parent := def.Node.Parent()
	if parent == nil {
		return nil, nil
	}

	switch parent.Type() {
	case "parameters":
		if def.Node.Type() == "identifier" && def.Node.Content(def.Contents) == "self" {
			fn := parent.Parent()
			if fn == nil || fn.Type() != "function_definition" {
				return nil, nil
			}
			block := fn.Parent()
			if block == nil || block.Type() != "block" {
				return nil, nil
			}
			class := block.Parent()
			if class == nil || class.Type() != "class_definition" {
				return nil, nil
			}
			name := class.ChildByFieldName("name")
			if name == nil {
				return nil, nil
			}
			return s.defToTypePython(ctx, swapNode(def, name))
		}
		fmt.Println("TODO defToTypePython:", parent.Type())
		return nil, nil
	case "typed_parameter":
		ty := parent.ChildByFieldName("type")
		if ty == nil {
			return nil, nil
		}
		return s.getTypeDefPython(ctx, swapNode(def, ty))
	case "class_definition":
		return (TypePython)(ClassTypePython{def: swapNode(def, parent)}), nil
	case "function_definition":
		retTyNode := parent.ChildByFieldName("return_type")
		if retTyNode == nil {
			return (TypePython)(FnTypePython{
				ret:  nil,
				noad: swapNode(def, parent),
			}), nil
		}
		retTy, err := s.getTypeDefPython(ctx, swapNode(def, retTyNode))
		if err != nil {
			return nil, err
		}
		return (TypePython)(FnTypePython{
			ret:  retTy,
			noad: swapNode(def, parent),
		}), nil
	case "assignment":
		ty := parent.ChildByFieldName("type")
		if ty == nil {
			right := parent.ChildByFieldName("right")
			if right == nil {
				return nil, nil
			}
			return s.getTypeDefPython(ctx, swapNode(def, right))
		}
		return s.getTypeDefPython(ctx, swapNode(def, ty))
	default:
		s.breadcrumb(swapNode(def, parent), fmt.Sprintf("unrecognized def parent %q", parent.Type()))
		return nil, nil
	}
}

func getSuperclassesPython(definition Node) []Node {
	supers := []Node{}
	for _, super := range children(definition.ChildByFieldName("superclasses")) {
		supers = append(supers, swapNode(definition, super))
	}
	return supers
}

type TypePython interface {
	variant() string
	node() Node
}

type FnTypePython struct {
	ret  TypePython
	noad Node
}

func (t FnTypePython) variant() string {
	return "fn"
}

func (t FnTypePython) node() Node {
	return t.noad
}

type ClassTypePython struct {
	def Node
}

func (t ClassTypePython) variant() string {
	return "class"
}

func (t ClassTypePython) node() Node {
	return t.def
}

type ModuleTypePython struct {
	module Node
}

func (t ModuleTypePython) variant() string {
	return "module"
}

func (t ModuleTypePython) node() Node {
	return t.module
}

type PrimTypePython struct {
	noad    Node
	varient string
}

func (t PrimTypePython) variant() string {
	return fmt.Sprintf("prim:%s", t.varient)
}

func (t PrimTypePython) node() Node {
	return t.noad
}

func lazyTypePythonStringer(ty *TypePython) func() fmt.Stringer {
	return func() fmt.Stringer {
		if ty != nil && *ty != nil {
			return String((*ty).variant())
		} else {
			return String("<nil>")
		}
	}
}

// isRecursiveDefinitionPython detects cases like `x = x.foo` that would cause infinite recursion when
// attempting to determine the type of `x`. This is known to happen in the wild, but it's not clear (to
// me) what the proper type should be or how to find it, so it's simply unsupported.
func isRecursiveDefinitionPython(node Node, def Node) bool {
	if node.RepoCommitPath != def.RepoCommitPath {
		return false
	}
	if def.Type() != "identifier" {
		return false
	}
	if def.Parent() == nil {
		return false
	}
	if def.Parent().Type() != "assignment" {
		return false
	}
	assignment := def.Parent()
	nodeAncestor := node.Parent()
	for nodeAncestor != nil {
		if nodeId(nodeAncestor) == nodeId(assignment) {
			return true
		}
		nodeAncestor = nodeAncestor.Parent()
	}
	return false
}
