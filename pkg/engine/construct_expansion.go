package engine

import (
	"errors"
	"fmt"

	"github.com/klothoplatform/klotho/pkg/collectionutil"
	"github.com/klothoplatform/klotho/pkg/core"
	"github.com/klothoplatform/klotho/pkg/engine/constraints"
	knowledgebase "github.com/klothoplatform/klotho/pkg/knowledge_base"
	"go.uber.org/zap"
)

type (
	ExpansionSet struct {
		Functionality core.Functionality
		Attributes    []string
	}
)

// ExpandConstructsAndCopyEdges expands all constructs in the working state using the engines provider
//
// The resources that result from the expanded constructs are written to the engines resource graph
// All dependencies are copied over to the resource graph
// If a dependency in the working state included a construct, the engine copies the dependency to all directly linked resources
func (e *Engine) ExpandConstructs() error {
	var joinedErr error
	for _, res := range e.Context.WorkingState.ListConstructs() {
		if e.Context.ExpandendOrCopiedBaseConstructs[res.Id()] {
			continue
		}
		// If the res is a resource, copy it over directly, otherwise we need to expand it
		if res.Id().Provider == core.AbstractConstructProvider {
			zap.S().Debugf("Expanding construct %s", res.Id())
			construct, ok := res.(core.Construct)
			if !ok {
				joinedErr = errors.Join(joinedErr, fmt.Errorf("unable to cast base construct %s to construct while expanding construct", res.Id()))
				continue
			}

			// We want to see if theres any constraint nodes before we expand so that the constraint is expanded corretly
			// right now we will just look at the first constraint for the construct
			// TODO: Combine all constraints when needed for expansion
			constructType := ""
			attributes := make(map[string]any)
			for _, constraint := range e.Context.Constraints[constraints.ConstructConstraintScope] {
				constructConstraint, ok := constraint.(*constraints.ConstructConstraint)
				if !ok {
					joinedErr = errors.Join(joinedErr, fmt.Errorf(" constraint %s is incorrect type. Expected to be a construct constraint while expanding construct", constraint))
					continue
				}

				if constructConstraint.Target == construct.Id() {
					constructType = constructConstraint.Type
					attributes = constructConstraint.Attributes
					break
				}
			}
			var expandError error
			for _, provider := range e.Providers {
				mappedResources, err := provider.ExpandConstruct(construct, e.Context.WorkingState, e.Context.EndState, constructType, attributes)
				if err == nil {
					e.Context.constructToResourceMapping[res.Id()] = append(e.Context.constructToResourceMapping[res.Id()], mappedResources...)
					expandError = nil
					break
				} else {
					expandError = errors.Join(joinedErr, fmt.Errorf("unable to expand construct %s, %s", res.Id(), err.Error()))
				}

			}
			if expandError != nil {
				joinedErr = errors.Join(joinedErr, fmt.Errorf("unable to expand construct %s, %s", res.Id(), expandError.Error()))
			}
		} else {
			zap.S().Debugf("Copying resource over %s", res.Id())
			resource, ok := res.(core.Resource)
			if !ok {
				joinedErr = errors.Join(joinedErr, fmt.Errorf("unable to cast base construct %s to resource while copying over resource", res.Id()))
				continue
			}
			e.Context.EndState.AddResource(resource)
		}
		e.Context.ExpandendOrCopiedBaseConstructs[res.Id()] = true
	}
	return joinedErr
}

// CopyEdges copies all edges from the working state to the resource graph
func (e *Engine) CopyEdges() error {
	var joinedErr error
	for _, dep := range e.Context.WorkingState.ListDependencies() {

		srcNodes := []core.Resource{}
		dstNodes := []core.Resource{}
		if dep.Source.Id().Provider == core.AbstractConstructProvider {
			srcResources, ok := e.Context.constructToResourceMapping[dep.Source.Id()]
			if !ok {
				joinedErr = errors.Join(joinedErr, fmt.Errorf("unable to find resources for construct %s", dep.Source.Id()))
				continue
			}
			srcNodes = append(srcNodes, srcResources...)
		} else {
			resource, ok := dep.Source.(core.Resource)
			if !ok {
				joinedErr = errors.Join(joinedErr, fmt.Errorf("unable to cast base construct %s to resource", dep.Source.Id()))
				continue
			}
			srcNodes = append(srcNodes, resource)
		}

		if dep.Destination.Id().Provider == core.AbstractConstructProvider {
			dstResources, ok := e.Context.constructToResourceMapping[dep.Destination.Id()]
			if !ok {
				joinedErr = errors.Join(joinedErr, fmt.Errorf("unable to find resources for construct %s", dep.Destination.Id()))
				continue
			}
			dstNodes = append(dstNodes, dstResources...)
		} else {
			resource, ok := dep.Destination.(core.Resource)
			if !ok {
				joinedErr = errors.Join(joinedErr, fmt.Errorf("unable to cast base construct %s to resource", dep.Destination.Id()))
				continue
			}
			dstNodes = append(dstNodes, resource)
		}

		for _, srcNode := range srcNodes {
			for _, dstNode := range dstNodes {
				if e.Context.CopiedEdges[srcNode.Id()] == nil {
					e.Context.CopiedEdges[srcNode.Id()] = make(map[core.ResourceId]bool)
				}
				if e.Context.CopiedEdges[srcNode.Id()][dstNode.Id()] {
					continue
				}

				zap.S().Debugf("Copying dependency %s -> %s", srcNode.Id(), dstNode.Id())
				e.Context.EndState.AddDependency(srcNode, dstNode)
				e.Context.CopiedEdges[srcNode.Id()][dstNode.Id()] = true
			}
		}
	}
	return joinedErr
}

func (e *Engine) expandConstruct(constraint constraints.ConstructConstraint, functionality core.Functionality) ([]*core.ResourceGraph, error) {
	var baseResource core.Resource
	for _, res := range e.ListResources() {
		if res.Id().Type == constraint.Type {
			baseResource = res
		}
	}
	expansionSet := ExpansionSet{Functionality: functionality}
	for attribute := range constraint.Attributes {
		expansionSet.Attributes = append(expansionSet.Attributes, attribute)
	}
	graphs, err := e.findPossibleExpansions(expansionSet, baseResource)
	var result []*core.ResourceGraph
	exists := map[string]*core.ResourceGraph{}
	for _, graph := range graphs {
		s := graph.String()
		if exists[s] == nil {
			result = append(result, graph)
			exists[s] = graph
		}
	}
	return result, err
}

func (e *Engine) findPossibleExpansions(expansionSet ExpansionSet, baseResource core.Resource) ([]*core.ResourceGraph, error) {
	var possibleExpansions []*core.ResourceGraph
	for _, res := range e.ListResources() {
		if baseResource != nil && res.Id().Type != baseResource.Id().Type {
			continue
		}
		classifications := e.ClassificationDocument.GetClassification(res)
		if !collectionutil.Contains(classifications.Is, string(expansionSet.Functionality)) {
			continue
		}
		unsatisfiedAttributes := []string{}
		for _, ms := range expansionSet.Attributes {
			if !collectionutil.Contains(classifications.Is, ms) {
				unsatisfiedAttributes = append(unsatisfiedAttributes, ms)
			}
		}
		graph := core.NewResourceGraph()
		graph.AddResource(res)
		expansions, err := e.findExpansions(unsatisfiedAttributes, graph, res, expansionSet.Functionality)
		if err != nil {
			return nil, err
		}
		possibleExpansions = append(possibleExpansions, expansions...)
	}
	return possibleExpansions, nil
}

func (e *Engine) findExpansions(attributes []string, graph *core.ResourceGraph, baseResource core.Resource, functionality core.Functionality) ([]*core.ResourceGraph, error) {
	if len(attributes) == 0 {
		return []*core.ResourceGraph{graph}, nil
	}
	var possibleExpansions []*core.ResourceGraph
	for _, attribute := range attributes {
		for _, res := range e.ListResources() {
			if res.Id().Type == baseResource.Id().Type {
				continue
			}
			if e.ClassificationDocument.GivesAttributeForFunctionality(res, attribute, functionality) && len(e.KnowledgeBase.FindPaths(baseResource, res, knowledgebase.EdgeConstraint{})) != 0 {
				graph.AddDependency(baseResource, res)
				unsatisfiedAttributes := []string{}
				for _, ms := range attributes {
					if ms != attribute {
						unsatisfiedAttributes = append(unsatisfiedAttributes, ms)
					}
				}

				expansions, err := e.findExpansions(unsatisfiedAttributes, graph.Clone(), baseResource, functionality)
				if err != nil {
					return nil, err
				}
				possibleExpansions = append(possibleExpansions, expansions...)
			}
		}
	}
	return possibleExpansions, nil
}
