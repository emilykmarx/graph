package graph

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"runtime"
	"time"
)

type directed[K comparable, T any] struct {
	hash   Hash[K, T]
	traits *Traits
	store  Store[K, T]
	log    *slog.Logger
}

func newDirected[K comparable, T any](hash Hash[K, T], traits *Traits, store Store[K, T], log *slog.Logger) *directed[K, T] {
	return &directed[K, T]{
		hash:   hash,
		traits: traits,
		store:  store,
		log:    log,
	}
}
func (d *directed[K, T]) Log() *slog.Logger {
	return d.log
}

func (d *directed[K, T]) Traits() *Traits {
	return d.traits
}

func (d *directed[K, T]) AddVertex(value T, options ...func(*VertexProperties)) error {
	hash := d.hash(value)
	properties := VertexProperties{
		Weight:     0,
		Attributes: make(map[string]string),
	}

	for _, option := range options {
		option(&properties)
	}

	return d.store.AddVertex(hash, value, properties)
}

func Logf(log *slog.Logger, lvl slog.Level, format string, args ...any) {
	if log == nil || !log.Enabled(context.Background(), lvl) {
		return
	}
	var pcs [1]uintptr
	runtime.Callers(2, pcs[:])
	r := slog.NewRecord(time.Now(), lvl, fmt.Sprintf(format, args...), pcs[0])
	_ = log.Handler().Handle(context.Background(), r)
}

// panic on error
func (d *directed[K, T]) LogEdges(hash K) {
	adjacencyMap, err := d.AdjacencyMap()
	if err != nil {
		panic(err)
	}
	outEdges := adjacencyMap[hash]
	predecessorMap, err := d.PredecessorMap()
	if err != nil {
		panic(err)
	}
	inEdges := predecessorMap[hash]
	Logf(d.Log(), slog.LevelDebug, "out_edges[%v]:", hash)
	for _, edge := range outEdges {
		Logf(d.Log(), slog.LevelDebug, "=> %v", edge.Target)
	}
	Logf(d.Log(), slog.LevelDebug, "in_edges[%v]:", hash)
	for _, edge := range inEdges {
		Logf(d.Log(), slog.LevelDebug, "%v =>", edge.Source)
	}
}

func (d *directed[K, T]) combineWarn(edge Edge[K], existingHash K, combineHash *K) {
	if combineHash != nil {
		if (edge.Source == existingHash && edge.Target == *combineHash) ||
			(edge.Source == *combineHash && edge.Target == existingHash) {
			Logf(d.Log(), slog.LevelWarn, "Combining edge %+v into self edge", edge)
		}
	}
}

// update edge source or target in place, if it's one of [existingHash, combineHash]
func maybeUpdate[K comparable](hash *K, existingHash K, combineHash *K, newHash K) {
	if *hash == existingHash || (combineHash != nil && *hash == *combineHash) {
		*hash = newHash
	}
}

// Update the vertex with hash `existingHash` to `value.`
// `value` may have a different hash, if the new hash doesn't already exist.
// Add `combineHash` vertex's edges to the new node, and remove `combineHash`.
// The new node will have an edge to itself in these cases:
// 1. existing or combine node had an edge to itself
// 2. Had edge between existing and combine - log a warning
// Panic on err.
// NOT thread-safe.
// Edge properties are preserved, but properties for the updated vertex must be passed in.
func (d *directed[K, T]) UpdateVertex(existingHash K, value T, combineHash *K, options ...func(*VertexProperties)) {
	newHash := d.hash(value)

	c := "<nil>"
	if combineHash != nil {
		c = fmt.Sprintf("%v", *combineHash)
	}
	Logf(d.Log(), slog.LevelDebug, "Enter UpdateVertex(existingHash %v, value %+v, combineHash %v) - edges before:", existingHash, value, c)

	d.LogEdges(existingHash)
	if combineHash != nil {
		d.LogEdges(*combineHash)
	}

	defer func() {
		Logf(d.Log(), slog.LevelDebug, "Exit UpdateVertex(newHash %v) - edges after:", newHash)
		d.LogEdges(newHash)
	}()

	// Check old vertex exists
	if _, err := d.Vertex(existingHash); err != nil {
		panic(err)
	}
	// Check vertex to be removed exists, and isn't the same vertex as existing
	if combineHash != nil {
		if _, err := d.Vertex(*combineHash); err != nil {
			panic(err)
		}
		if existingHash == *combineHash {
			panic("existing == remove")
		}
	}

	// panic if new value hashes to some third node that already exists (neither existing nor combine) -
	// unclear what caller wants
	if newHash != existingHash && (combineHash == nil || newHash != *combineHash) {
		_, err := d.Vertex(newHash)
		if !errors.Is(err, ErrVertexNotFound) {
			if err == nil {
				panic(ErrVertexAlreadyExists)
			} else {
				panic(err)
			}
		}
	}

	// 1. Remove old edges (to/from existing and combine)
	// (must remove edges before removing vertex, even if vertex will be kept with same hash)
	oldHashes := []K{existingHash}
	if combineHash != nil {
		oldHashes = append(oldHashes, *combineHash)
	}
	var oldEdges []Edge[K]

	for _, oldHash := range oldHashes {
		adjacencyMap, err := d.AdjacencyMap()
		if err != nil {
			panic(err)
		}
		outEdges := adjacencyMap[oldHash]
		predecessorMap, err := d.PredecessorMap()
		if err != nil {
			panic(err)
		}
		inEdges := predecessorMap[oldHash]

		// OUT: SOURCE = old hash
		for _, edge := range outEdges {
			d.combineWarn(edge, existingHash, combineHash)

			if err := d.RemoveEdge(edge.Source, edge.Target); err != nil {
				Logf(d.Log(), slog.LevelError, "RemoveEdge %+v", edge) // log which edge failed
				panic(err)
			}

			edge.Source = newHash // FROM new hash
			maybeUpdate(&edge.Target, existingHash, combineHash, newHash)

			oldEdges = append(oldEdges, edge)
		}

		// IN: TARGET = old hash
		for _, edge := range inEdges {
			d.combineWarn(edge, existingHash, combineHash)

			if edge.Source == edge.Target {
				// Edge to self => already handled above
				// (loop over outEdges removes from both outEdges and inEdges, and appends edge that AddEdge() will add to both)
				continue
			}

			if err := d.RemoveEdge(edge.Source, edge.Target); err != nil {
				Logf(d.Log(), slog.LevelError, "RemoveEdge %+v", edge) // log which edge failed
				panic(err)
			}

			edge.Target = newHash // TO new hash
			maybeUpdate(&edge.Source, existingHash, combineHash, newHash)

			oldEdges = append(oldEdges, edge)
		}

		// 2. Remove old vertex
		if err := d.RemoveVertex(oldHash); err != nil {
			panic(err)
		}
	}

	// 3. Add new vertex
	if err := d.AddVertex(value, options...); err != nil {
		panic(err)
	}

	// 4. Add new edges
	// (to new vertex, preserving properties)
	for _, edge := range oldEdges {
		if err := d.AddEdge(copyEdge(edge)); err != nil {
			if !errors.Is(err, ErrEdgeAlreadyExists) {
				Logf(d.Log(), slog.LevelError, "AddEdge %+v", edge) // log which edge failed
				panic(err)
			} else {
				// may have dups, e.g. if combining 2 nodes with same edge in
			}
		}
	}
}

func (d *directed[K, T]) AddVerticesFrom(g Graph[K, T]) error {
	adjacencyMap, err := g.AdjacencyMap()
	if err != nil {
		return fmt.Errorf("failed to get adjacency map: %w", err)
	}

	for hash := range adjacencyMap {
		vertex, properties, err := g.VertexWithProperties(hash)
		if err != nil {
			return fmt.Errorf("failed to get vertex %v: %w", hash, err)
		}

		if err = d.AddVertex(vertex, copyVertexProperties(properties)); err != nil {
			return fmt.Errorf("failed to add vertex %v: %w", hash, err)
		}
	}

	return nil
}

func (d *directed[K, T]) Vertex(hash K) (T, error) {
	vertex, _, err := d.store.Vertex(hash)
	return vertex, err
}

func (d *directed[K, T]) VertexWithProperties(hash K) (T, VertexProperties, error) {
	vertex, properties, err := d.store.Vertex(hash)
	if err != nil {
		return vertex, VertexProperties{}, err
	}

	return vertex, properties, nil
}

func (d *directed[K, T]) RemoveVertex(hash K) error {
	return d.store.RemoveVertex(hash)
}

func (d *directed[K, T]) AddEdge(sourceHash, targetHash K, options ...func(*EdgeProperties)) error {
	_, _, err := d.store.Vertex(sourceHash)
	if err != nil {
		return fmt.Errorf("source vertex %v: %w", sourceHash, err)
	}

	_, _, err = d.store.Vertex(targetHash)
	if err != nil {
		return fmt.Errorf("target vertex %v: %w", targetHash, err)
	}

	if _, err := d.Edge(sourceHash, targetHash); !errors.Is(err, ErrEdgeNotFound) {
		return ErrEdgeAlreadyExists
	}

	// If the user opted in to preventing cycles, run a cycle check.
	if d.traits.PreventCycles {
		createsCycle, err := d.createsCycle(sourceHash, targetHash)
		if err != nil {
			return fmt.Errorf("check for cycles: %w", err)
		}
		if createsCycle {
			return ErrEdgeCreatesCycle
		}
	}

	edge := Edge[K]{
		Source: sourceHash,
		Target: targetHash,
		Properties: EdgeProperties{
			Attributes: make(map[string]string),
		},
	}

	for _, option := range options {
		option(&edge.Properties)
	}

	return d.addEdge(sourceHash, targetHash, edge)
}

func (d *directed[K, T]) AddEdgesFrom(g Graph[K, T]) error {
	edges, err := g.Edges()
	if err != nil {
		return fmt.Errorf("failed to get edges: %w", err)
	}

	for _, edge := range edges {
		if err := d.AddEdge(copyEdge(edge)); err != nil {
			return fmt.Errorf("failed to add (%v, %v): %w", edge.Source, edge.Target, err)
		}
	}

	return nil
}

func (d *directed[K, T]) Edge(sourceHash, targetHash K) (Edge[T], error) {
	edge, err := d.store.Edge(sourceHash, targetHash)
	if err != nil {
		return Edge[T]{}, err
	}

	sourceVertex, _, err := d.store.Vertex(sourceHash)
	if err != nil {
		return Edge[T]{}, fmt.Errorf("err getting vertex %v: %v", sourceHash, err.Error())
	}

	targetVertex, _, err := d.store.Vertex(targetHash)
	if err != nil {
		return Edge[T]{}, fmt.Errorf("err getting vertex %v: %v", targetHash, err.Error())
	}

	return Edge[T]{
		Source: sourceVertex,
		Target: targetVertex,
		Properties: EdgeProperties{
			Weight:     edge.Properties.Weight,
			Attributes: edge.Properties.Attributes,
			Data:       edge.Properties.Data,
		},
	}, nil
}

func (d *directed[K, T]) Edges() ([]Edge[K], error) {
	return d.store.ListEdges()
}

func (d *directed[K, T]) UpdateEdge(source, target K, options ...func(properties *EdgeProperties)) error {
	existingEdge, err := d.store.Edge(source, target)
	if err != nil {
		return err
	}

	for _, option := range options {
		option(&existingEdge.Properties)
	}

	return d.store.UpdateEdge(source, target, existingEdge)
}

func (d *directed[K, T]) RemoveEdge(source, target K) error {
	if _, err := d.Edge(source, target); err != nil {
		return fmt.Errorf("failed to remove edge from %v to %v: %w", source, target, err)
	}

	if err := d.store.RemoveEdge(source, target); err != nil {
		return fmt.Errorf("failed to remove edge from %v to %v: %w", source, target, err)
	}

	return nil
}

func (d *directed[K, T]) AdjacencyMap() (map[K]map[K]Edge[K], error) {
	vertices, err := d.store.ListVertices()
	if err != nil {
		return nil, fmt.Errorf("failed to list vertices: %w", err)
	}

	edges, err := d.store.ListEdges()
	if err != nil {
		return nil, fmt.Errorf("failed to list edges: %w", err)
	}

	m := make(map[K]map[K]Edge[K], len(vertices))

	for _, vertex := range vertices {
		m[vertex] = make(map[K]Edge[K])
	}

	for _, edge := range edges {
		m[edge.Source][edge.Target] = edge
	}

	return m, nil
}

func (d *directed[K, T]) PredecessorMap() (map[K]map[K]Edge[K], error) {
	vertices, err := d.store.ListVertices()
	if err != nil {
		return nil, fmt.Errorf("failed to list vertices: %w", err)
	}

	edges, err := d.store.ListEdges()
	if err != nil {
		return nil, fmt.Errorf("failed to list edges: %w", err)
	}

	m := make(map[K]map[K]Edge[K], len(vertices))

	for _, vertex := range vertices {
		m[vertex] = make(map[K]Edge[K])
	}

	for _, edge := range edges {
		if _, ok := m[edge.Target]; !ok {
			m[edge.Target] = make(map[K]Edge[K])
		}
		m[edge.Target][edge.Source] = edge
	}

	return m, nil
}

func (d *directed[K, T]) addEdge(sourceHash, targetHash K, edge Edge[K]) error {
	return d.store.AddEdge(sourceHash, targetHash, edge)
}

func (d *directed[K, T]) Clone() (Graph[K, T], error) {
	traits := &Traits{
		IsDirected:    d.traits.IsDirected,
		IsAcyclic:     d.traits.IsAcyclic,
		IsWeighted:    d.traits.IsWeighted,
		IsRooted:      d.traits.IsRooted,
		PreventCycles: d.traits.PreventCycles,
	}

	clone := &directed[K, T]{
		hash:   d.hash,
		traits: traits,
		store:  newMemoryStore[K, T](),
	}

	if err := clone.AddVerticesFrom(d); err != nil {
		return nil, fmt.Errorf("failed to add vertices: %w", err)
	}

	if err := clone.AddEdgesFrom(d); err != nil {
		return nil, fmt.Errorf("failed to add edges: %w", err)
	}

	return clone, nil
}

func (d *directed[K, T]) Order() (int, error) {
	return d.store.VertexCount()
}

func (d *directed[K, T]) Size() (int, error) {
	return d.store.EdgeCount()
}

func (d *directed[K, T]) edgesAreEqual(a, b Edge[T]) bool {
	aSourceHash := d.hash(a.Source)
	aTargetHash := d.hash(a.Target)
	bSourceHash := d.hash(b.Source)
	bTargetHash := d.hash(b.Target)

	return aSourceHash == bSourceHash && aTargetHash == bTargetHash
}

func (d *directed[K, T]) createsCycle(source, target K) (bool, error) {
	// If the underlying store implements CreatesCycle, use that fast path.
	if cc, ok := d.store.(interface {
		CreatesCycle(source, target K) (bool, error)
	}); ok {
		return cc.CreatesCycle(source, target)
	}

	// Slow path.
	return CreatesCycle(Graph[K, T](d), source, target)
}

// copyEdge returns an argument list suitable for the Graph.AddEdge method. This
// argument list is derived from the given edge, hence the name copyEdge.
//
// The last argument is a custom functional option that sets the edge properties
// to the properties of the original edge.
func copyEdge[K comparable](edge Edge[K]) (K, K, func(properties *EdgeProperties)) {
	copyProperties := func(p *EdgeProperties) {
		for k, v := range edge.Properties.Attributes {
			p.Attributes[k] = v
		}
		p.Weight = edge.Properties.Weight
		p.Data = edge.Properties.Data
	}

	return edge.Source, edge.Target, copyProperties
}
