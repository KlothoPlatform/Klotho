package resources

import (
	"fmt"
	"github.com/klothoplatform/klotho/pkg/core"
	"github.com/klothoplatform/klotho/pkg/engine/classification"
	"github.com/klothoplatform/klotho/pkg/provider"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type (
	Namespace struct {
		Name            string
		ConstructRefs   core.BaseConstructSet `yaml:"-"`
		Object          *corev1.Namespace
		Transformations map[string]core.IaCValue
		FilePath        string
		Cluster         core.ResourceId
	}
)

const (
	NAMESPACE_TYPE = "namespace"
)

func (namespace *Namespace) BaseConstructRefs() core.BaseConstructSet {
	return namespace.ConstructRefs
}

func (namespace *Namespace) Id() core.ResourceId {
	return core.ResourceId{
		Provider: provider.KUBERNETES,
		Type:     NAMESPACE_TYPE,
		Name:     namespace.Name,
	}
}

func (namespace *Namespace) DeleteContext() core.DeleteContext {
	return core.DeleteContext{
		RequiresNoUpstream: true,
	}
}

func (namespace *Namespace) GetObject() v1.Object {
	return namespace.Object
}

func (namespace *Namespace) Kind() string {
	return namespace.Object.Kind
}

func (namespace *Namespace) Path() string {
	return namespace.FilePath
}

func (namespace *Namespace) GetResourcesInNamespace(dag *core.ResourceGraph) []core.Resource {
	var resources []core.Resource
	for _, res := range dag.GetAllUpstreamResources(namespace) {
		if manifest, ok := res.(ManifestFile); ok {
			if manifest.GetObject() != nil && manifest.GetObject().GetNamespace() == namespace.Name {
				resources = append(resources, manifest)
			}
		}
	}
	return resources
}

func (namespace *Namespace) MakeOperational(dag *core.ResourceGraph, appName string, classifier classification.Classifier) error {
	if namespace.Cluster.Name == "" {
		return fmt.Errorf("namespace %s has no cluster", namespace.Name)
	}

	SetDefaultObjectMeta(namespace, namespace.Object.GetObjectMeta())
	namespace.FilePath = ManifestFilePath(namespace)
	return nil
}

func (namespace *Namespace) GetValues() map[string]core.IaCValue {
	return namespace.Transformations
}
