package graph

import (
	"fmt"
)

// DFS performs a depth-first search on the graph, starting from the given vertex. The visit
// function will be invoked with the hash of the vertex currently visited. If it returns false, DFS
// will continue traversing the graph, and if it returns true, the traversal will be stopped. In
// case the graph is disconnected, only the vertices joined with the starting vertex are visited.
//
// This example prints all vertices of the graph in DFS-order:
//
//	g := graph.New(graph.IntHash)
//
//	_ = g.AddVertex(1)
//	_ = g.AddVertex(2)
//	_ = g.AddVertex(3)
//
//	_ = g.AddEdge(1, 2)
//	_ = g.AddEdge(2, 3)
//	_ = g.AddEdge(3, 1)
//
//	_ = graph.DFS(g, 1, func(value int) bool {
//		fmt.Println(value)
//		return false
//	})
//
// Similarly, if you have a graph of City vertices and the traversal should stop at London, the
// visit function would look as follows:
//
//	func(c City) bool {
//		return c.Name == "London"
//	}
//
// DFS is non-recursive and maintains a stack instead.
// If `all_paths`: Visit all paths (meaning nodes reachable via multiple paths will be visited multiple times),
// stopping only if there is a cycle.
// If `all_paths` and a cycle is found: visit all paths but return error.
// If `pretty_print`: print tabs indicating level of tree (before calling visit) - could be removed (caller can use update_vertices to do it themselves).
// If `backwards`: visit in opposite order of edges.
func DFS[K comparable, T any](g Graph[K, T], start K, visit func(K) bool, update_vertices UpdatePathVertices[T], all_paths bool, pretty_print bool, backwards bool) error {
	var m map[K]map[K]Edge[K]
	var err error
	if backwards {
		m, err = g.PredecessorMap()
	} else {
		m, err = g.AdjacencyMap()
	}
	if err != nil {
		return fmt.Errorf("DFS could not get adjacency/predecessor map: %w", err)
	}

	if _, ok := m[start]; !ok {
		return fmt.Errorf("DFS could not find start vertex with hash %v", start)
	}

	type stackNode struct {
		hash         K
		indent_level int
	}
	stack := newStack[stackNode]()
	visited := make(map[K]bool)
	// Nodes visited on the current path
	visited_path := make(map[K]bool)
	cycle := false

	stack.push(stackNode{hash: start})

	for !stack.isEmpty() {
		cur, _ := stack.pop()
		currentHash := cur.hash
		indent_level := cur.indent_level

		_, visited_ever := visited[currentHash]
		_, visited_on_path := visited_path[currentHash]
		should_visit := !visited_ever
		if all_paths {
			should_visit = !visited_on_path
			if !should_visit {
				// Already visited this node on this path => cycle
				cycle = true
			}
		}

		if should_visit {
			if pretty_print {
				for i := 0; i < indent_level; i++ {
					fmt.Printf("\t")
				}
			}

			// Stop traversing the graph if the visit function returns true.
			if stop := visit(currentHash); stop {
				break
			}
			visited[currentHash] = true
			visited_path[currentHash] = true

			leaf := true
			for neighHash := range m[currentHash] {
				stack.push(stackNode{hash: neighHash, indent_level: indent_level + 1}) // indent children by one more
				err = updatePathVertices(g, currentHash, neighHash, update_vertices, backwards)
				if err != nil {
					return err
				}

				leaf = false // has outgoing edge
			}
			if leaf {
				// End of path
				visited_path = make(map[K]bool)
			}
		}
	}

	if cycle {
		return ErrCycleFound
	}

	return nil
}

// Functions to edit nodes in-place while traversing.
// Note if node is passed to one of the functions twice (see below for when they're called),
// second edit will override the first.
// Edits change the node value, but CANNOT change its hash, since we get `m` once at the beginning
// So if hash changes, `m` won't be updated, so info won't propagate -
// could support by having UpdateVertex take an adjacency/predecessor map to update)
type UpdatePathVertices[NodeT any] struct {
	// Graph has edge `parent` => `child`.
	// Both are called when pushing a neighbor; UpdateParent is called first, then UpdateChild is called with the updated parent.
	// If forwards DFS: just visited a parent and pushing its child
	// If backwards DFS: just visited a child and pushing its parent
	UpdateParent *func(parent NodeT, child NodeT) NodeT
	UpdateChild  *func(parent NodeT, child NodeT) NodeT
}

func updatePathVertices[K comparable, NodeT any](g Graph[K, NodeT], currentHash K, neighHash K, update_vertices UpdatePathVertices[NodeT], backwards bool) error {
	directed, ok := g.(*directed[K, NodeT])
	if !ok {
		if update_vertices.UpdateChild != nil || update_vertices.UpdateParent != nil {
			// not supported (and presumably caller wanted it)
			return fmt.Errorf("DFS does not support updating path vertices for non-directed graphs")
		}
	}

	cur, err := g.Vertex(currentHash)
	if err != nil {
		return fmt.Errorf("DFS could not find current vertex with hash %v", currentHash)
	}
	neigh, err := g.Vertex(neighHash)
	if err != nil {
		return fmt.Errorf("DFS could not find neighbor vertex with hash %v", neighHash)
	}

	parent := cur
	parentHash := currentHash
	child := neigh
	childHash := neighHash
	if backwards {
		parent = neigh
		parentHash = neighHash
		child = cur
		childHash = currentHash
	}

	if update_vertices.UpdateParent != nil {
		parent = (*(update_vertices.UpdateParent))(parent, child)
		newHash := directed.hash(parent)
		if parentHash != newHash {
			return fmt.Errorf("DFS can only update path vertex if hash stays the same - old %v != new %v", parentHash, newHash)
		}
		err = g.UpdateVertex(parentHash, parent, func(vp *VertexProperties) {})
		if err != nil {
			return err
		}
	}

	if update_vertices.UpdateChild != nil {
		new_node := (*(update_vertices.UpdateChild))(parent, child)
		newHash := directed.hash(new_node)
		if childHash != newHash {
			return fmt.Errorf("DFS can only update path vertex if hash stays the same - old %v != new %v", childHash, newHash)
		}
		err = g.UpdateVertex(childHash, new_node, func(vp *VertexProperties) {})
		if err != nil {
			return err
		}
	}

	return nil
}

// Do DFS from all roots if forwards, else all leaves
func DFSAllStartingNodes[K comparable, T any](g Graph[K, T], visit func(K) bool, update_vertices UpdatePathVertices[T], all_paths bool, pretty_print bool, backwards bool) error {
	var m map[K]map[K]Edge[K]
	var err error
	if backwards {
		m, err = g.AdjacencyMap() // outgoing
	} else {
		m, err = g.PredecessorMap() // incoming
	}
	if err != nil {
		return fmt.Errorf("DFSAllStartingNodes could not get adjacency/predecessor map: %w", err)
	}

	for hash, edges := range m {
		// If forwards: a root (no incoming edges), else a leaf
		if len(edges) == 0 {
			err = DFS(g, hash, visit, update_vertices, all_paths, pretty_print, backwards)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

// BFS performs a breadth-first search on the graph, starting from the given vertex. The visit
// function will be invoked with the hash of the vertex currently visited. If it returns false, BFS
// will continue traversing the graph, and if it returns true, the traversal will be stopped. In
// case the graph is disconnected, only the vertices joined with the starting vertex are visited.
//
// This example prints all vertices of the graph in BFS-order:
//
//	g := graph.New(graph.IntHash)
//
//	_ = g.AddVertex(1)
//	_ = g.AddVertex(2)
//	_ = g.AddVertex(3)
//
//	_ = g.AddEdge(1, 2)
//	_ = g.AddEdge(2, 3)
//	_ = g.AddEdge(3, 1)
//
//	_ = graph.BFS(g, 1, func(value int) bool {
//		fmt.Println(value)
//		return false
//	})
//
// Similarly, if you have a graph of City vertices and the traversal should stop at London, the
// visit function would look as follows:
//
//	func(c City) bool {
//		return c.Name == "London"
//	}
//
// BFS is non-recursive and maintains a stack instead.
func BFS[K comparable, T any](g Graph[K, T], start K, visit func(K) bool) error {
	ignoreDepth := func(vertex K, _ int) bool {
		return visit(vertex)
	}
	return BFSWithDepth(g, start, ignoreDepth)
}

// BFSWithDepth works just as BFS and performs a breadth-first search on the graph, but its
// visit function is passed the current depth level as a second argument. Consequently, the
// current depth can be used for deciding whether or not to proceed past a certain depth.
//
//	_ = graph.BFSWithDepth(g, 1, func(value int, depth int) bool {
//		fmt.Println(value)
//		return depth > 3
//	})
//
// With the visit function from the example, the BFS traversal will stop once a depth greater
// than 3 is reached.
func BFSWithDepth[K comparable, T any](g Graph[K, T], start K, visit func(K, int) bool) error {
	adjacencyMap, err := g.AdjacencyMap()
	if err != nil {
		return fmt.Errorf("could not get adjacency map: %w", err)
	}

	if _, ok := adjacencyMap[start]; !ok {
		return fmt.Errorf("could not find start vertex with hash %v", start)
	}

	queue := make([]K, 0)
	visited := make(map[K]bool)

	visited[start] = true
	queue = append(queue, start)
	depth := 0

	for len(queue) > 0 {
		currentHash := queue[0]

		queue = queue[1:]
		depth++

		// Stop traversing the graph if the visit function returns true.
		if stop := visit(currentHash, depth); stop {
			break
		}

		for adjacency := range adjacencyMap[currentHash] {
			if _, ok := visited[adjacency]; !ok {
				visited[adjacency] = true
				queue = append(queue, adjacency)
			}
		}

	}

	return nil
}
