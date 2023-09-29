package construct2

import (
	"errors"

	"github.com/dominikbraun/graph"
)

func copyVertexProps(p graph.VertexProperties) func(*graph.VertexProperties) {
	return func(dst *graph.VertexProperties) {
		*dst = p
	}
}

func copyEdgeProps(p graph.EdgeProperties) func(*graph.EdgeProperties) {
	return func(dst *graph.EdgeProperties) {
		*dst = p
	}
}

// UpdateResourceId is used when a resource's ID changes. It updates the graph in-place, using the resource
// currently referenced by `old`. No-op if the resource ID hasn't changed.
// Also updates any property references (as [ResourceId] or [PropertyRef]) of the old ID to the new ID in any
// resource that depends on or is depended on by the resource.
func UpdateResourceId(g Graph, old ResourceId) error {
	r, props, err := g.VertexWithProperties(old)
	if err != nil {
		return err
	}
	// Short circuit if the resource ID hasn't changed.
	if old == r.ID {
		return nil
	}

	err = g.AddVertex(r, copyVertexProps(props))
	if err != nil {
		return err
	}

	neighbors := make(map[ResourceId]struct{})
	adj, err := g.AdjacencyMap()
	if err != nil {
		return err
	}
	for _, edge := range adj[old] {
		err = errors.Join(
			err,
			g.AddEdge(r.ID, edge.Target, copyEdgeProps(edge.Properties)),
			g.RemoveEdge(edge.Source, edge.Target),
		)
		neighbors[edge.Target] = struct{}{}
	}
	if err != nil {
		return err
	}

	pred, err := g.PredecessorMap()
	if err != nil {
		return err
	}
	for _, edge := range pred[old] {
		err = errors.Join(
			err,
			g.AddEdge(edge.Source, r.ID, copyEdgeProps(edge.Properties)),
			g.RemoveEdge(edge.Source, edge.Target),
		)
		neighbors[edge.Source] = struct{}{}
	}
	if err != nil {
		return err
	}

	if err := g.RemoveVertex(old); err != nil {
		return err
	}

	for neighborId := range neighbors {
		neighbor, err := g.Vertex(neighborId)
		if err != nil {
			return err
		}
		err = neighbor.WalkProperties(func(path PropertyPath, err error) error {
			propVal := path.Get()
			propId, ok := propVal.(ResourceId)
			if ok && propId == old {
				return errors.Join(err, path.Set(r.ID))
			}
			propRef, ok := propVal.(PropertyRef)
			if ok && propRef.Resource == old {
				propRef.Resource = r.ID
				return errors.Join(err, path.Set(propRef))
			}
			return err
		})
		if err != nil {
			return err
		}
	}
	return nil
}

// RemoveResource removes all edges from the resource. any property references (as [ResourceId] or [PropertyRef])
// to the resource, and finally the resource itself.
func RemoveResource(g Graph, id ResourceId) error {
	r, props, err := g.VertexWithProperties(id)
	if err != nil {
		return err
	}

	err = g.AddVertex(r, copyVertexProps(props))
	if err != nil {
		return err
	}

	neighbors := make(map[ResourceId]struct{})
	adj, err := g.AdjacencyMap()
	if err != nil {
		return err
	}
	for _, edge := range adj[id] {
		err = errors.Join(
			err,
			g.RemoveEdge(edge.Source, edge.Target),
		)
		neighbors[edge.Target] = struct{}{}
	}
	if err != nil {
		return err
	}

	pred, err := g.PredecessorMap()
	if err != nil {
		return err
	}
	for _, edge := range pred[id] {
		err = errors.Join(
			err,
			g.RemoveEdge(edge.Source, edge.Target),
		)
		neighbors[edge.Source] = struct{}{}
	}
	if err != nil {
		return err
	}

	for neighborId := range neighbors {
		neighbor, err := g.Vertex(neighborId)
		if err != nil {
			return err
		}
		err = neighbor.WalkProperties(func(path PropertyPath, err error) error {
			propVal := path.Get()
			propId, ok := propVal.(ResourceId)
			if ok && propId == id {
				return errors.Join(err, path.Remove(nil))
			}
			propRef, ok := propVal.(PropertyRef)
			if ok && propRef.Resource == id {
				return errors.Join(err, path.Remove(nil))
			}
			return err
		})
		if err != nil {
			return err
		}
	}
	return g.RemoveVertex(id)
}
