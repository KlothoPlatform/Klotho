package knowledgebase

import (
	"fmt"
	"reflect"

	"github.com/klothoplatform/klotho/pkg/core"
	"github.com/klothoplatform/klotho/pkg/graph"
	"github.com/klothoplatform/klotho/pkg/multierr"
	"go.uber.org/zap"
)

type (
	// Edge defines an entry in a Knowledge base. An Edge represents a valid linking between two types of resources
	Edge struct {
		// Source represents the source resource in the edge
		Source reflect.Type
		// Destination represents the target resource in the edge
		Destination reflect.Type
	}

	// EdgeDetails defines the set of characteristics and edge in the knowledge base contains. The details are used to ensure graph correctness for ResourceGraphs
	EdgeDetails struct {
		// ExpansionFunc is a function used to create the To and From resource and necessary intermediate resources, if any do not exist, to ensure the nodes are in place for correct functionality.
		ExpansionFunc ExpandEdge
		// Configure is a function used to configure the To and From resources and necessary dependent resources, to ensure the nodes will guarantee correct functionality.
		Configure ConfigureEdge
		// ValidDestinations is a list of end destinations the edge supports.
		// This field is used within determining the path (edge expansion) between two resources.
		ValidDestinations []reflect.Type
		// ReverseDirection is specified when the data flow is in the opposite direction of the edge
		// This is used in scenarios where we want to find paths, only allowing specific edges to be bidirectional
		ReverseDirection bool
	}

	// EdgeKB is a map (knowledge base) of edges and their respective details used to configure ResourceGraphs
	EdgeKB map[Edge]EdgeDetails

	// EdgeExpander is a function used to create the To and From resource and necessary intermediate resources, if any do not exist, to ensure the nodes are in place for correct functionality.
	ExpandEdge func(source, dest core.Resource, dag *core.ResourceGraph, data EdgeData) error
	// EdgeConfigurer is a function used to configure the To and From resources and necessary dependent resources, to ensure the nodes will guarantee correct functionality.
	ConfigureEdge func(source, dest core.Resource, dag *core.ResourceGraph, data EdgeData) error

	// EdgeConstraint is an object defined on EdgeData which can influence the path picked when expansion occurs.
	EdgeConstraint struct {
		// NodeMustExist specifies a list of resources which must exist in the path when edge expansion occurs. The resources type will be correlated to the types in the generated paths
		NodeMustExist []core.Resource
		// NodeMustNotExist specifies a list of resources which must not exist when edge expansion occurs. The resources type will be correlated to the types in the generated paths
		NodeMustNotExist []core.Resource
	}

	// EdgeData is an object attached to edges in the ResourceGraph to help the knowledge base understand context when performing expansion and configuration tasks
	EdgeData struct {
		// AppName refers to the application name of the global ResourceGraph
		AppName string
		// EnvironmentVaribles specify and environment variables which will need to be configured during the edge expansion process
		EnvironmentVariables []core.EnvironmentVariable
		// Constraint refers to the EdgeConstraints defined during the edge expansion
		Constraint EdgeConstraint
		// Source refers to the initial source resource node when edge expansion is called
		Source core.Resource
		// Destination refers to the initial target resource node when edge expansion is called
		Destination core.Resource
		// Routes refers to any api routes being satisfied by the edge
		Routes []core.Route
		// SourceRef denotes the source annotation being used during expansion or configuration
		// This is a temporary field due to helm chart being the lowest level of kubernetes resource at the moment
		SourceRef core.BaseConstruct
	}

	Path []Edge
)

func NewEdge[Src core.Resource, Dest core.Resource]() Edge {
	var src Src
	var dest Dest
	return Edge{Source: reflect.TypeOf(src), Destination: reflect.TypeOf(dest)}
}

// GetEdge takes in a source and target to retrieve the edge details for the given key. Will return nil if no edge exists for the given source and target
func (kb EdgeKB) GetEdge(source core.Resource, target core.Resource) (EdgeDetails, bool) {
	return kb.GetEdgeDetails(reflect.TypeOf(source), reflect.TypeOf(target))
}

// GetEdgeDetails takes in a source and target to retrieve the edge details for the given key. Will return nil if no edge exists for the given source and target
func (kb EdgeKB) GetEdgeDetails(source reflect.Type, target reflect.Type) (EdgeDetails, bool) {
	detail, found := kb[Edge{Source: source, Destination: target}]
	return detail, found
}

// GetEdgesWithSource will return all edges where the source type parameter is the From of the edge
func (kb EdgeKB) GetEdgesWithSource(source reflect.Type) []Edge {
	result := []Edge{}
	for edge := range kb {
		if edge.Source == source {
			result = append(result, edge)
		}
	}
	return result
}

// GetEdgesWithTarget will return all edges where the target type parameter is the To of the edge
func (kb EdgeKB) GetEdgesWithTarget(target reflect.Type) []Edge {
	result := []Edge{}
	for edge := range kb {
		if edge.Destination == target {
			result = append(result, edge)
		}
	}
	return result
}

// FindPaths takes in a source and destination type and finds all valid paths to get from source to destination.
//
// Find paths does a Depth First Search to search through all edges in the knowledge base.
// The function tracks visited edges to prevent cycles during execution
// It also checks the ValidDestinations for each edge against the original destination node to ensure that the edge is allowed to be used in the instance of the path generation
//
// The method will return all paths found
func (kb EdgeKB) FindPaths(source reflect.Type, dest reflect.Type) []Path {
	zap.S().Debugf("Finding Paths from %s -> %s", source, dest)
	visitedEdges := map[reflect.Type]bool{}
	stack := []Edge{}
	return kb.findPaths(source, dest, stack, visitedEdges)
}

// findPaths performs the recursive calls of the parent FindPath function
//
// It works under the assumption that an edge is bidirectional and uses the edges ValidDestinations field to determine when that assumption is incorrect
func (kb EdgeKB) findPaths(source reflect.Type, dest reflect.Type, stack []Edge, visited map[reflect.Type]bool) (result []Path) {
	visited[source] = true
	if source == dest {
		// For resources which can have dependencies between themselves we have to add that path to the stack if it is a valid edge
		if len(stack) == 0 {
			if _, found := kb.GetEdgeDetails(source, dest); found {
				stack = append(stack, Edge{Source: source, Destination: dest})
			}
		}
		if len(stack) != 0 {
			result = append(result, stack)
		}
	} else {
		// When we are not at the destination we want to recursively call findPaths on all edges which have the source as the current node
		// This is checking all edges which have a direction of From -> To
		for _, e := range kb.GetEdgesWithSource(source) {
			det, _ := kb.GetEdgeDetails(e.Source, e.Destination)
			if !det.ReverseDirection && e.Source == source && !visited[e.Destination] && kb.isValidForPath(e, dest) {
				result = append(result, kb.findPaths(e.Destination, dest, append(stack, e), visited)...)
			}
		}
		// When we are not at the destination we want to recursively call findPaths on all edges which have the target as the current node
		// This is checking all edges which have a path direction of To -> From, which is opposite of their dependencies on each other
		//
		// An example of this scenario is in the AWS knowledge base where RdsProxyTarget -> RdsProxy  and RdsProxyTarget -> RdsInstance are valid edges
		// However we would expect the path to be RdsProxy -> RdsProxyTarget -> RdsInstance, so to satisfy understanding the path to connect other nodes, we must understand the direction of both the IaC dependency and data flow dependency
		for _, e := range kb.GetEdgesWithTarget(source) {
			det, _ := kb.GetEdgeDetails(e.Source, e.Destination)
			if det.ReverseDirection && e.Destination == source && !visited[e.Source] && kb.isValidForPath(e, dest) {
				result = append(result, kb.findPaths(e.Source, dest, append(stack, e), visited)...)
			}
		}
	}
	delete(visited, source)
	return result
}

// isValidForPath determines if an edge is valid for an instance of path generation.
//
// The criteria is:
//   - check to see if the path generations destination is valid for the edge
func (kb EdgeKB) isValidForPath(edge Edge, dest reflect.Type) bool {
	edgeDetail, _ := kb.GetEdgeDetails(edge.Source, edge.Destination)
	for _, validDest := range edgeDetail.ValidDestinations {
		if validDest == dest {
			return true
		}
	}
	return false
}

// ExpandEdges performs calculations to determine the proper path to be inserted into the ResourceGraph.
//
// The workflow of the edge expansion is as follows:
//   - Find all valid paths from the dependencies source node to the dependencies target node
//   - Check each of the valid paths against constraints passed in on the edge data
//   - At this point we should only have 1 valid path (If we have more than 1 edge choose direct connection otherwise error)
//   - Iterate through each edge in path calling expansion function on edge
func (kb EdgeKB) ExpandEdges(dag *core.ResourceGraph, appName string) (err error) {
	zap.S().Debug("Expanding Edges")
	var merr multierr.Error
	// It does not matter what order we go in as each edge should be expanded independently. They can still reuse resources since the create methods should be idempotent if resources are the same.
	for _, dep := range dag.ListDependencies() {
		zap.S().Debugf("Expanding Edge for %s -> %s", dep.Source.Id(), dep.Destination.Id())

		// We want to retrieve the edge data from the edge in the resource graph to use during expansion
		edgeData := EdgeData{}
		data, ok := dep.Properties.Data.(EdgeData)
		fmt.Println(data)
		if !ok && dep.Properties.Data != nil {
			merr.Append(fmt.Errorf("edge properties for edge %s -> %s, do not satisfy edge data format during expansion", dep.Source.Id(), dep.Destination.Id()))
		} else if dep.Properties.Data != nil {
			edgeData = data
		}
		edgeData.AppName = appName
		// We attach the dependencies source and destination nodes for context during expansion
		edgeData.Source = dep.Source
		edgeData.Destination = dep.Destination
		// Find all possible paths given the initial source and destination node
		paths := kb.FindPaths(reflect.TypeOf(dep.Source), reflect.TypeOf(dep.Destination))
		validPaths := [][]Edge{}
		for _, path := range paths {
			// Ensure that the path satisfies the NodeMustExist edge constraint
			if edgeData.Constraint.NodeMustExist != nil {
				nodeFound := false
				for _, res := range path {
					for _, mustExistRes := range edgeData.Constraint.NodeMustExist {
						if res.Source == reflect.TypeOf(mustExistRes) || res.Destination == reflect.TypeOf(mustExistRes) {
							nodeFound = true
						}
					}
				}
				if !nodeFound {
					continue
				}
			}

			// Ensure that the path satisfies the NodeMustNotExist edge constraint
			if edgeData.Constraint.NodeMustNotExist != nil {
				nodeFound := false
				for _, res := range path {
					for _, mustNotExistRes := range edgeData.Constraint.NodeMustNotExist {
						if res.Source == reflect.TypeOf(mustNotExistRes) || res.Destination == reflect.TypeOf(mustNotExistRes) {
							nodeFound = true
						}
					}
				}
				if nodeFound {
					continue
				}
			}
			validPaths = append(validPaths, path)
		}

		zap.S().Debugf("Found valid paths %s", validPaths)
		var validPath []Edge
		// Get the shortest route that satisfied constraints
		for _, path := range validPaths {
			if len(validPath) == 0 {
				validPath = path
			} else if len(path) < len(validPath) {
				validPath = path
			} else if len(path) == len(validPath) {
				merr.Append(fmt.Errorf("found multiple paths which satisfy constraints for edge %s -> %s and are the same length. \n Paths: %s", dep.Source.Id(), dep.Destination.Id(), validPaths))
				continue
			}
		}
		if len(validPath) == 0 {
			merr.Append(fmt.Errorf("found no paths which satisfy constraints for edge %s -> %s. \n Paths: %s", dep.Source.Id(), dep.Destination.Id(), validPaths))
			continue
		}

		// If the valid path is not the original direct path, we want to remove the initial direct dependency so we can fill in the new edges with intermediate nodes
		if len(validPath) > 1 {
			zap.S().Debugf("Removing dependency from %s -> %s", dep.Source.Id(), dep.Destination.Id())
			err := dag.RemoveDependency(dep.Source.Id(), dep.Destination.Id())
			if err != nil {
				merr.Append(err)
				continue
			}
		}

		// resourceCache is used to always pass the graphs nodes into the Expand functions if they exist. We do this so that we operate on nodes which already exist
		resourceCache := map[reflect.Type]core.Resource{}
		for _, edge := range validPath {
			source := edge.Source
			dest := edge.Destination
			edgeDetail, _ := kb.GetEdgeDetails(source, dest)
			sourceNode := resourceCache[source]
			if source == reflect.TypeOf(dep.Source) {
				sourceNode = dep.Source
			}
			if sourceNode == nil {
				sourceNode = reflect.New(source.Elem()).Interface().(core.Resource)
				for _, mustExistRes := range edgeData.Constraint.NodeMustExist {
					if mustExistRes.Id().Type == sourceNode.Id().Type && mustExistRes.Id().Provider == sourceNode.Id().Provider && mustExistRes.Id().Namespace == sourceNode.Id().Namespace {
						sourceNode = mustExistRes
					}
				}
			}

			destNode := resourceCache[dest]
			if dest == reflect.TypeOf(dep.Destination) {
				destNode = dep.Destination
			}
			if destNode == nil {
				destNode = reflect.New(dest.Elem()).Interface().(core.Resource)
				for _, mustExistRes := range edgeData.Constraint.NodeMustExist {
					if mustExistRes.Id().Type == destNode.Id().Type && mustExistRes.Id().Provider == destNode.Id().Provider && mustExistRes.Id().Namespace == destNode.Id().Namespace {
						destNode = mustExistRes
					}
				}

			}

			if edgeDetail.ExpansionFunc != nil {
				err := edgeDetail.ExpansionFunc(sourceNode, destNode, dag, edgeData)
				merr.Append(err)
			}

			if sourceNode != nil {
				resourceCache[source] = sourceNode
			}
			sourceNodeInGraph := dag.GetResource(sourceNode.Id())
			if sourceNodeInGraph != nil {
				resourceCache[source] = sourceNodeInGraph
			}
			if destNode != nil {
				resourceCache[dest] = destNode
			}
			destNodeInGraph := dag.GetResource(destNode.Id())
			if destNodeInGraph != nil {
				resourceCache[dest] = destNodeInGraph
			}
		}

	}
	return merr.ErrOrNil()
}

// ConfigureFromEdgeData calls each edges configure function.
func (kb EdgeKB) ConfigureFromEdgeData(dag *core.ResourceGraph) (err error) {
	zap.S().Debug("Configuring Edges")
	var merr multierr.Error
	for _, dep := range dag.ListDependencies() {
		zap.S().Debugf("Configuring Edge for %s -> %s", dep.Source.Id(), dep.Destination.Id())
		source := reflect.TypeOf(dep.Source)
		destination := reflect.TypeOf(dep.Destination)
		edgeData := EdgeData{}
		data, ok := dep.Properties.Data.(EdgeData)
		if !ok && dep.Properties.Data != nil {
			merr.Append(fmt.Errorf("edge properties for edge %s -> %s, do not satisfy edge data format during edge configuration", dep.Source.Id(), dep.Destination.Id()))
		} else if dep.Properties.Data != nil {
			edgeData = data
		}
		edgeDetail, found := kb.GetEdgeDetails(source, destination)
		if !found {
			merr.Append(fmt.Errorf("internal error invalid edge for edge %s -> %s (no such edge in Edge KB)", dep.Source.Id(), dep.Destination.Id()))
			continue
		}
		if edgeDetail.Configure != nil {
			err := edgeDetail.Configure(dep.Source, dep.Destination, dag, edgeData)
			merr.Append(err)
		}
	}
	return merr.ErrOrNil()
}

// FindPathsInGraph takes in a source and destination type and finds all valid paths to get from source to destination.
//
// Find paths does a Depth First Search to search through all edges in the knowledge base.
// The function tracks visited edges to prevent cycles during execution
// It also checks the ValidDestinations for each edge against the original destination node to ensure that the edge is allowed to be used in the instance of the path generation
//
// The method will return all paths found
func (kb EdgeKB) FindPathsInGraph(source core.Resource, dest core.Resource, dag *core.ResourceGraph) [][]graph.Edge[core.Resource] {
	zap.S().Debugf("Finding Paths from %s -> %s", source, dest)
	visitedEdges := map[core.Resource]bool{}
	stack := []graph.Edge[core.Resource]{}
	return kb.findPathsInGraph(source, dest, stack, visitedEdges, dag)
}

// findPathsInGraph performs the recursive calls of the parent FindPath function
//
// It works under the assumption that an edge is bidirectional and uses the edges ValidDestinations field to determine when that assumption is incorrect
func (kb EdgeKB) findPathsInGraph(source, dest core.Resource, stack []graph.Edge[core.Resource], visited map[core.Resource]bool, dag *core.ResourceGraph) (result [][]graph.Edge[core.Resource]) {
	visited[source] = true
	if source == dest {
		if len(stack) != 0 {
			result = append(result, stack)
		}
	} else {
		// When we are not at the destination we want to recursively call findPaths on all edges which have the source as the current node
		// This is checking all edges which have a direction of From -> To
		for _, edge := range dag.GetDownstreamDependencies(source) {
			det, _ := kb.GetEdgeDetails(reflect.TypeOf(edge.Source), reflect.TypeOf(edge.Destination))
			if !det.ReverseDirection && edge.Source == source && !visited[edge.Destination] {
				result = append(result, kb.findPathsInGraph(edge.Destination, dest, append(stack, edge), visited, dag)...)
			}
		}
		// When we are not at the destination we want to recursively call findPaths on all edges which have the target as the current node
		// This is checking all edges which have a path direction of To -> From, which is opposite of their dependencies on each other
		//
		// An example of this scenario is in the AWS knowledge base where RdsProxyTarget -> RdsProxy  and RdsProxyTarget -> RdsInstance are valid edges
		// However we would expect the path to be RdsProxy -> RdsProxyTarget -> RdsInstance, so to satisfy understanding the path to connect other nodes, we must understand the direction of both the IaC dependency and data flow dependency

		for _, edge := range dag.GetUpstreamDependencies(source) {
			det, _ := kb.GetEdgeDetails(reflect.TypeOf(edge.Source), reflect.TypeOf(edge.Destination))
			if det.ReverseDirection && edge.Destination == source && !visited[edge.Source] {
				result = append(result, kb.findPathsInGraph(edge.Source, dest, append(stack, edge), visited, dag)...)
			}
		}
	}
	delete(visited, source)
	return result
}
