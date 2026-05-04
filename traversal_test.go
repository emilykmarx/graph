package graph

import (
	"errors"
	"log"
	"reflect"
	"testing"
)

func TestDirectedDFS(t *testing.T) {
	type test struct {
		vertices       []int
		edges          []Edge[int]
		startHash      int
		expectedVisits [][]int // any are acceptable
		stopAtVertex   int     // tests stopping early due to the visit function
	}
	cycle := test{
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
	diamond := test{
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

	tests := map[string]test{
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
			graph := New(IntHash, Directed())

			for _, vertex := range test.vertices {
				_ = graph.AddVertex(vertex)
			}

			for _, edge := range test.edges {
				if err := graph.AddEdge(edge.Source, edge.Target); err != nil {
					t.Fatalf("%s: failed to add edge: %s", name, err.Error())
				}
			}

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
			dfs_err := DFS(graph, test.startHash, visit, all_paths)

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

		_ = DFS(graph, test.startHash, visit, false)

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
