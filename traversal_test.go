package graph

import (
	"errors"
	"log"
	"reflect"
	"sort"
	"strings"
	"testing"
)

type testdirected struct {
	vertices       []int
	edges          []Edge[int]
	startHash      int
	expectedVisits [][]int // any are acceptable
	stopAtVertex   int     // tests stopping early due to the visit function
}

func directedGraph(t *testing.T, test_name string, test testdirected) Graph[int, int] {
	graph := New(IntHash, Directed())

	for _, vertex := range test.vertices {
		_ = graph.AddVertex(vertex)
	}

	for _, edge := range test.edges {
		if err := graph.AddEdge(edge.Source, edge.Target); err != nil {
			t.Fatalf("%s: failed to add edge: %s", test_name, err.Error())
		}
	}
	return graph
}

// Hash is last character
func Postfix(v string) string {
	return string(v[len(v)-1])
}

// Other tests already check order - just check values (forwards, from all roots)
// (Takes multiple possible expected for tests where result could depend both on order visited in original DFS, and order visited here)
func checkVertexValues(t *testing.T, test_name string, graph Graph[string, string], expected_new_nodes [][]string) {
	visited := []string{}
	err := DFSAllStartingNodes(graph, func(hash string) bool {
		n, _ := graph.Vertex(hash)
		visited = append(visited, n)
		return false
	}, UpdatePathVertices[string]{}, false, false, false)

	if err != nil {
		t.Errorf("%s: DFS to check vertices - %v", test_name, err.Error())
	}

	sort.Strings(visited)

	visit_ok := false
	for _, expected_visit := range expected_new_nodes {
		sort.Strings(expected_visit)
		if reflect.DeepEqual(visited, expected_visit) {
			visit_ok = true
			break
		}
	}
	if !visit_ok {
		t.Errorf("%s: expected one of these visit sequences: %v, got %v", test_name, expected_new_nodes, visited)
	}
}

// Also tests DFS backwards and DFSAllStartingNodes
func TestDFSUpdatePathVertices(t *testing.T) {
	graph := New(Postfix, Directed())

	test_name := "DFS path info accumulation, backwards, and all starting nodes"
	// Paths: a => b => c => e, a => b => d
	vertices := []string{"a", "b", "c", "e", "d"}

	for i, _ := range vertices {
		if i < len(vertices)-1 {
			_ = graph.AddVertex(vertices[i])
			_ = graph.AddVertex(vertices[i+1])
		}
		if i < len(vertices)-2 {
			if err := graph.AddEdge(vertices[i], vertices[i+1]); err != nil {
				t.Fatalf("%s: failed to add edge: %s", test_name, err.Error())
			}
		}
	}
	if err := graph.AddEdge(vertices[1], vertices[4]); err != nil {
		t.Fatalf("%s: failed to add edge: %s", test_name, err.Error())
	}

	// 1. PUSH DOWN
	UpdateChild := func(parent, child string) string {
		// keep child hash (last char) the same
		return parent + "." + child
	}
	update_vertices := UpdatePathVertices[string]{
		UpdateChild: &UpdateChild,
	}
	err := DFSAllStartingNodes(graph, func(i string) bool { return false }, update_vertices, true, false, false) // forwards
	if err != nil {
		t.Fatalf("%s: Unexpected error from DFS to push info up: %v", test_name, err)
	}

	expected_new_nodes_pushdown := [][]string{{"a", "a.b", "a.b.c", "a.b.c.e", "a.b.d"}}
	checkVertexValues(t, test_name, graph, expected_new_nodes_pushdown)

	// 2. PUSH UP
	UpdateParent := func(parent, child string) string {
		// keep parent hash (last char) the same
		full_path := strings.Split(child, "-")[0] // This will always be the full path as we go up
		// <full path>-<orig hash>
		return full_path + "-" + Postfix(parent)
	}
	update_vertices = UpdatePathVertices[string]{
		UpdateParent: &UpdateParent,
	}
	err = DFSAllStartingNodes(graph, func(i string) bool { return false }, update_vertices, true, false, true) // backwards
	if err != nil {
		t.Fatalf("%s: Unexpected error from DFS to push info down: %v", test_name, err)
	}

	// B has two children, so whichever path is executed second overwrites the first in B and A
	expected_new_nodes_pushup := [][]string{
		// b<=d first so e=>c=>b=>a wins
		{"a.b.d" /* visit B and A here, but will be overwritten */, "a.b.c.e", "a.b.c.e-c", "a.b.c.e-b", "a.b.c.e-a"},
		// e=>c=>b=>a first, so b=>d wins
		{"a.b.c.e", "a.b.c.e-c" /* visit B and A here, but will be overwritten */, "a.b.d", "a.b.d-b", "a.b.d-a"},
	}
	checkVertexValues(t, test_name, graph, expected_new_nodes_pushup)
}

func TestDirectedDFS(t *testing.T) {
	cycle := testdirected{
		vertices: []int{1, 2, 3},
		edges: []Edge[int]{
			{Source: 1, Target: 2},
			{Source: 2, Target: 3},
			{Source: 3, Target: 1},
		},
		startHash:      1,
		expectedVisits: [][]int{{1, 2, 3}},
		stopAtVertex:   -1,
	}
	cycle_testname := "traverse entire directed graph with cycle"
	cycle_stop_early := cycle
	cycle_stop_early.stopAtVertex = 2
	cycle_stop_early.expectedVisits = [][]int{{1, 2}}

	diamond_testname := "traverse directed graph with a node reachable by two paths (no cycles though)"
	diamond := testdirected{
		vertices: []int{1, 2, 3, 4},
		edges: []Edge[int]{
			{Source: 1, Target: 2},
			{Source: 1, Target: 3},
			{Source: 2, Target: 4},
			{Source: 3, Target: 4},
		},
		startHash: 1,
		// 4 is reachable by 2 paths - if `all_paths`: should be visited twice (in either order)
		expectedVisits: [][]int{{1, 2, 4, 3, 4}, {1, 3, 4, 2, 4}},
		stopAtVertex:   -1,
	}

	tests := map[string]testdirected{
		"traverse entire directed graph with 3 vertices": {
			vertices: []int{1, 2, 3},
			edges: []Edge[int]{
				{Source: 1, Target: 2},
				{Source: 1, Target: 3},
			},
			startHash:      1,
			expectedVisits: [][]int{{1, 2, 3}, {1, 3, 2}}, // visit direct children in either order
			stopAtVertex:   -1,
		},
		cycle_testname: cycle,
		"traverse directed graph with cycle stopping early due to visit func": cycle_stop_early,
		"traverse a disconnected directed graph": {
			vertices: []int{1, 2, 3, 4},
			edges: []Edge[int]{
				{Source: 1, Target: 2},
				{Source: 3, Target: 4},
			},
			startHash:      1,
			expectedVisits: [][]int{{1, 2}},
			stopAtVertex:   -1,
		},
		diamond_testname: diamond,
	}

	for _, all_paths := range []bool{false, true} {
		for name, test := range tests {
			graph := directedGraph(t, name, test)
			visited := []int{}

			visit := func(value int) bool {
				visited = append(visited, value)

				if test.stopAtVertex != -1 {
					if value == test.stopAtVertex {
						return true // should stop visiting
					}
				}
				return false
			}

			if (name == diamond_testname) && !all_paths {
				// If !all_paths: Only visit 4 on one path
				test.expectedVisits = [][]int{{1, 2, 4, 3}, {1, 3, 4, 2}}
			}
			dfs_err := DFS(graph, test.startHash, visit, UpdatePathVertices[int]{}, all_paths, false, false)

			// 1. Check nodes visited
			visit_ok := false
			for _, expected_visit := range test.expectedVisits {
				if reflect.DeepEqual(visited, expected_visit) {
					visit_ok = true
					break
				}
			}
			if !visit_ok {
				t.Errorf("%s: expected one of these visit sequences: %v, got %v", name, test.expectedVisits, visited)
			}

			// 2. Check error
			if (name == cycle_testname) && all_paths {
				if !errors.Is(dfs_err, ErrCycleFound) {
					t.Fatalf("%s: cycle not detected - instead, error was: %v", name, dfs_err)
				}
			} else {
				if dfs_err != nil {
					t.Fatalf("%s: Unexpected DFS error: %v", name, dfs_err)
				}
			}
		}
	}
}

func TestUndirectedDFS(t *testing.T) {
	tests := map[string]struct {
		vertices  []int
		edges     []Edge[int]
		startHash int
		// It is not possible to expect a strict list of vertices to be visited.
		// If stopAtVertex is a neighbor of another vertex, that other vertex
		// might be visited before stopAtVertex.
		expectedMinimumVisits []int
		// In case stopAtVertex has downstream neighbors, those neighbors must
		// not be visited.
		forbiddenVisits []int
		stopAtVertex    int
	}{
		"traverse entire undirected graph with 3 vertices": {
			vertices: []int{1, 2, 3},
			edges: []Edge[int]{
				{Source: 1, Target: 2},
				{Source: 1, Target: 3},
			},
			startHash:             1,
			expectedMinimumVisits: []int{1, 2, 3},
			stopAtVertex:          -1,
		},
		"traverse entire undirected triangle graph": {
			vertices: []int{1, 2, 3},
			edges: []Edge[int]{
				{Source: 1, Target: 2},
				{Source: 2, Target: 3},
				{Source: 3, Target: 1},
			},
			startHash:             1,
			expectedMinimumVisits: []int{1, 2, 3},
			stopAtVertex:          -1,
		},
		"traverse undirected graph with 3 vertices until vertex 2": {
			vertices: []int{1, 2, 3},
			edges: []Edge[int]{
				{Source: 1, Target: 2},
				{Source: 2, Target: 3},
				{Source: 3, Target: 1},
			},
			startHash:             1,
			expectedMinimumVisits: []int{1, 2},
			stopAtVertex:          2,
		},
		"traverse undirected graph with 7 vertices until vertex 4": {
			vertices: []int{1, 2, 3, 4, 5, 6, 7},
			edges: []Edge[int]{
				{Source: 1, Target: 2},
				{Source: 1, Target: 3},
				{Source: 2, Target: 4},
				{Source: 2, Target: 5},
				{Source: 4, Target: 6},
				{Source: 5, Target: 7},
			},
			startHash:             1,
			expectedMinimumVisits: []int{1, 2, 4},
			forbiddenVisits:       []int{6},
			stopAtVertex:          4,
		},
		"traverse undirected graph with 15 vertices until vertex 11": {
			vertices: []int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15},
			edges: []Edge[int]{
				{Source: 1, Target: 2},
				{Source: 1, Target: 3},
				{Source: 3, Target: 4},
				{Source: 3, Target: 5},
				{Source: 3, Target: 6},
				{Source: 4, Target: 7},
				{Source: 5, Target: 13},
				{Source: 5, Target: 14},
				{Source: 6, Target: 7},
				{Source: 7, Target: 8},
				{Source: 7, Target: 9},
				{Source: 8, Target: 10},
				{Source: 9, Target: 11},
				{Source: 9, Target: 12},
				{Source: 10, Target: 14},
				{Source: 11, Target: 15},
			},
			startHash:             1,
			expectedMinimumVisits: []int{1, 3, 7, 9, 11},
			forbiddenVisits:       []int{15},
			stopAtVertex:          11,
		},
		"traverse a disconnected undirected graph": {
			vertices: []int{1, 2, 3, 4},
			edges: []Edge[int]{
				{Source: 1, Target: 2},
				{Source: 3, Target: 4},
			},
			startHash:             1,
			expectedMinimumVisits: []int{1, 2},
			stopAtVertex:          -1,
		},
	}

	for name, test := range tests {
		graph := New(IntHash)

		for _, vertex := range test.vertices {
			_ = graph.AddVertex(vertex)
		}

		for _, edge := range test.edges {
			if err := graph.AddEdge(edge.Source, edge.Target); err != nil {
				t.Fatalf("%s: failed to add edge: %s", name, err.Error())
			}
		}

		visited := make(map[int]struct{})

		visit := func(value int) bool {
			visited[value] = struct{}{}

			if test.stopAtVertex != -1 {
				if value == test.stopAtVertex {
					return true
				}
			}
			return false
		}

		err := DFS(graph, test.startHash, visit, UpdatePathVertices[int]{}, false, false, false)
		if err != nil {
			t.Fatalf("%s: Unexpected error from DFS: %v", name, err)
		}

		if len(visited) < len(test.expectedMinimumVisits) {
			t.Fatalf("%s: expected number of minimum visits doesn't match: expected %v, got %v", name, len(test.expectedMinimumVisits), len(visited))
		}

		if test.forbiddenVisits != nil {
			for _, forbiddenVisit := range test.forbiddenVisits {
				if _, ok := visited[forbiddenVisit]; ok {
					t.Errorf("%s: expected vertex %v to not be visited, but it is", name, forbiddenVisit)
				}
			}
		}

		for _, expectedVisit := range test.expectedMinimumVisits {
			if _, ok := visited[expectedVisit]; !ok {
				t.Errorf("%s: expected vertex %v to be visited, but it isn't", name, expectedVisit)
			}
		}
	}
}

func TestDirectedBFS(t *testing.T) {
	tests := map[string]struct {
		vertices       []int
		edges          []Edge[int]
		startHash      int
		expectedVisits []int
		stopAtVertex   int
	}{
		"traverse entire graph with 3 vertices": {
			vertices: []int{1, 2, 3},
			edges: []Edge[int]{
				{Source: 1, Target: 2},
				{Source: 1, Target: 3},
			},
			startHash:      1,
			expectedVisits: []int{1, 2, 3},
			stopAtVertex:   -1,
		},
		"traverse graph with 6 vertices until vertex 4": {
			vertices: []int{1, 2, 3, 4, 5, 6},
			edges: []Edge[int]{
				{Source: 1, Target: 2},
				{Source: 1, Target: 3},
				{Source: 2, Target: 4},
				{Source: 2, Target: 5},
				{Source: 3, Target: 6},
			},
			startHash:      1,
			expectedVisits: []int{1, 2, 3, 4},
			stopAtVertex:   4,
		},
		"traverse a disconnected graph": {
			vertices: []int{1, 2, 3, 4},
			edges: []Edge[int]{
				{Source: 1, Target: 2},
				{Source: 3, Target: 4},
			},
			startHash:      1,
			expectedVisits: []int{1, 2},
			stopAtVertex:   -1,
		},
	}

	for name, test := range tests {
		graph := New(IntHash, Directed())

		for _, vertex := range test.vertices {
			_ = graph.AddVertex(vertex)
		}

		for _, edge := range test.edges {
			if err := graph.AddEdge(edge.Source, edge.Target); err != nil {
				t.Fatalf("%s: failed to add edge: %s", name, err.Error())
			}
		}

		visited := make(map[int]struct{})

		visit := func(value int) bool {
			visited[value] = struct{}{}

			if test.stopAtVertex != -1 {
				if value == test.stopAtVertex {
					return true
				}
			}
			return false
		}

		_ = BFS(graph, test.startHash, visit)

		for _, expectedVisit := range test.expectedVisits {
			if _, ok := visited[expectedVisit]; !ok {
				t.Errorf("%s: expected vertex %v to be visited, but it isn't", name, expectedVisit)
			}
		}

		visitWithDepth := func(value int, depth int) bool {
			visited[value] = struct{}{}
			log.Printf("cur depth: %d", depth)

			if test.stopAtVertex != -1 {
				if value == test.stopAtVertex {
					return true
				}
			}
			return false
		}
		_ = BFSWithDepth(graph, test.startHash, visitWithDepth)
	}
}

func TestUndirectedBFS(t *testing.T) {
	tests := map[string]struct {
		vertices       []int
		edges          []Edge[int]
		startHash      int
		expectedVisits []int
		stopAtVertex   int
	}{
		"traverse entire graph with 3 vertices": {
			vertices: []int{1, 2, 3},
			edges: []Edge[int]{
				{Source: 1, Target: 2},
				{Source: 1, Target: 3},
			},
			startHash:      1,
			expectedVisits: []int{1, 2, 3},
			stopAtVertex:   -1,
		},
		"traverse graph with 6 vertices until vertex 4": {
			vertices: []int{1, 2, 3, 4, 5, 6},
			edges: []Edge[int]{
				{Source: 1, Target: 2},
				{Source: 1, Target: 3},
				{Source: 2, Target: 4},
				{Source: 2, Target: 5},
				{Source: 3, Target: 6},
			},
			startHash:      1,
			expectedVisits: []int{1, 2, 3, 4},
			stopAtVertex:   4,
		},
		"traverse a disconnected graph": {
			vertices: []int{1, 2, 3, 4},
			edges: []Edge[int]{
				{Source: 1, Target: 2},
				{Source: 3, Target: 4},
			},
			startHash:      1,
			expectedVisits: []int{1, 2},
			stopAtVertex:   -1,
		},
	}

	for name, test := range tests {
		graph := New(IntHash)

		for _, vertex := range test.vertices {
			_ = graph.AddVertex(vertex)
		}

		for _, edge := range test.edges {
			if err := graph.AddEdge(edge.Source, edge.Target); err != nil {
				t.Fatalf("%s: failed to add edge: %s", name, err.Error())
			}
		}

		visited := make(map[int]struct{})

		visit := func(value int) bool {
			visited[value] = struct{}{}

			if test.stopAtVertex != -1 {
				if value == test.stopAtVertex {
					return true
				}
			}
			return false
		}

		_ = BFS(graph, test.startHash, visit)

		for _, expectedVisit := range test.expectedVisits {
			if _, ok := visited[expectedVisit]; !ok {
				t.Errorf("%s: expected vertex %v to be visited, but it isn't", name, expectedVisit)
			}
		}
	}
}
