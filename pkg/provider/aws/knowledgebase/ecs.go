package knowledgebase

import (
	"fmt"

	"github.com/klothoplatform/klotho/pkg/core"
	knowledgebase "github.com/klothoplatform/klotho/pkg/knowledge_base"
	"github.com/klothoplatform/klotho/pkg/provider/aws/resources"
)

var EcsKB = knowledgebase.Build(
	knowledgebase.EdgeBuilder[*resources.EcsService, *resources.Vpc]{},
	knowledgebase.EdgeBuilder[*resources.EcsService, *resources.Subnet]{},
	knowledgebase.EdgeBuilder[*resources.EcsService, *resources.SecurityGroup]{},
	knowledgebase.EdgeBuilder[*resources.EcsService, *resources.EcsCluster]{
		Configure: func(service *resources.EcsService, cluster *resources.EcsCluster, dag *core.ResourceGraph, data knowledgebase.EdgeData) error {
			if service.Cluster != nil && service.Cluster != cluster {
				return fmt.Errorf("cannot configure edge %s -> %s, service already tied to cluster %s", service.Id(), cluster.Id(), service.Cluster.Id())
			}
			service.Cluster = cluster
			return nil
		},
	},
	knowledgebase.EdgeBuilder[*resources.EcsService, *resources.EcsTaskDefinition]{
		Configure: func(service *resources.EcsService, taskDefinition *resources.EcsTaskDefinition, dag *core.ResourceGraph, data knowledgebase.EdgeData) error {
			if service.TaskDefinition != nil && service.TaskDefinition != taskDefinition {
				return fmt.Errorf("cannot configure edge %s -> %s, service already tied to task definition %s", service.Id(), taskDefinition.Id(), service.TaskDefinition.Id())
			}
			service.TaskDefinition = taskDefinition
			return nil
		},
	},
	knowledgebase.EdgeBuilder[*resources.EcsTaskDefinition, *resources.EcrImage]{},
	knowledgebase.EdgeBuilder[*resources.EcsTaskDefinition, *resources.Region]{},
	knowledgebase.EdgeBuilder[*resources.EcsTaskDefinition, *resources.LogGroup]{
		Configure: func(taskDef *resources.EcsTaskDefinition, lg *resources.LogGroup, dag *core.ResourceGraph, data knowledgebase.EdgeData) error {
			if taskDef.LogGroup != lg {
				return nil // this log group doesn't belong to this task definition and is configured elsewhere
			}

			// configure the task definition's log group
			lg.LogGroupName = fmt.Sprintf("/aws/ecs/%s", taskDef.Name)
			return nil
		},
	},
	knowledgebase.EdgeBuilder[*resources.EcsService, *resources.DynamodbTable]{
		Configure: func(service *resources.EcsService, table *resources.DynamodbTable, dag *core.ResourceGraph, data knowledgebase.EdgeData) error {
			err := checkServiceForRole(service, table)
			if err != nil {
				return err
			}
			dag.AddDependency(service.TaskDefinition.ExecutionRole, table)
			for _, env := range data.EnvironmentVariables {
				service.TaskDefinition.EnvironmentVariables[env.GetName()] = core.IaCValue{ResourceId: table.Id(), Property: env.GetValue()}
			}
			return nil
		},
	},
	knowledgebase.EdgeBuilder[*resources.EcsService, *resources.ElasticacheCluster]{
		Configure: func(service *resources.EcsService, cluster *resources.ElasticacheCluster, dag *core.ResourceGraph, data knowledgebase.EdgeData) error {
			for _, env := range data.EnvironmentVariables {
				service.TaskDefinition.EnvironmentVariables[env.GetName()] = core.IaCValue{ResourceId: cluster.Id(), Property: env.GetValue()}
			}
			return nil
		},
	},
	knowledgebase.EdgeBuilder[*resources.EcsService, *resources.S3Bucket]{
		Configure: func(service *resources.EcsService, bucket *resources.S3Bucket, dag *core.ResourceGraph, data knowledgebase.EdgeData) error {
			err := checkServiceForRole(service, bucket)
			if err != nil {
				return err
			}
			dag.AddDependency(service.TaskDefinition.ExecutionRole, bucket)
			for _, env := range data.EnvironmentVariables {
				service.TaskDefinition.EnvironmentVariables[env.GetName()] = core.IaCValue{ResourceId: bucket.Id(), Property: env.GetValue()}
			}
			return nil
		},
	},
	knowledgebase.EdgeBuilder[*resources.EcsService, *resources.Secret]{
		Configure: func(service *resources.EcsService, secret *resources.Secret, dag *core.ResourceGraph, data knowledgebase.EdgeData) error {
			err := checkServiceForRole(service, secret)
			if err != nil {
				return err
			}
			dag.AddDependency(service.TaskDefinition.ExecutionRole, secret)
			for _, env := range data.EnvironmentVariables {
				service.TaskDefinition.EnvironmentVariables[env.GetName()] = core.IaCValue{ResourceId: secret.Id(), Property: env.GetValue()}
			}
			return nil
		},
	},
	knowledgebase.EdgeBuilder[*resources.EcsService, *resources.RdsInstance]{
		Configure: func(service *resources.EcsService, instance *resources.RdsInstance, dag *core.ResourceGraph, data knowledgebase.EdgeData) error {
			for _, env := range data.EnvironmentVariables {
				service.TaskDefinition.EnvironmentVariables[env.GetName()] = core.IaCValue{ResourceId: instance.Id(), Property: env.GetValue()}
			}
			return nil
		},
	},
	knowledgebase.EdgeBuilder[*resources.EcsService, *resources.RdsProxy]{
		Configure: func(service *resources.EcsService, proxy *resources.RdsProxy, dag *core.ResourceGraph, data knowledgebase.EdgeData) error {
			for _, env := range data.EnvironmentVariables {
				service.TaskDefinition.EnvironmentVariables[env.GetName()] = core.IaCValue{ResourceId: proxy.Id(), Property: env.GetValue()}
			}
			return nil
		},
	},
	knowledgebase.EdgeBuilder[*resources.TargetGroup, *resources.EcsService]{
		DeploymentOrderReversed: true,
		Configure: func(tg *resources.TargetGroup, service *resources.EcsService, dag *core.ResourceGraph, data knowledgebase.EdgeData) error {
			if service.TaskDefinition == nil || len(service.TaskDefinition.PortMappings) != 1 {
				return fmt.Errorf("cannot configure edge %s -> %s, the service's task definition does not have exactly one port mapping", service.Id(), tg.Id())
			}
			service.LoadBalancers = []resources.EcsServiceLoadBalancerConfig{
				{
					TargetGroupArn: core.IaCValue{ResourceId: tg.Id(), Property: resources.ARN_IAC_VALUE},
					ContainerName:  service.Name,
					ContainerPort:  service.TaskDefinition.PortMappings[0].ContainerPort,
				},
			}
			tg.Port = 3000
			tg.Protocol = "TCP"
			tg.TargetType = "ip"
			return nil
		},
	},
)

func checkServiceForRole(service *resources.EcsService, dest core.Resource) error {
	if service.TaskDefinition == nil {
		return fmt.Errorf("cannot configure edge %s -> %s, missing task definition", service.Id(), dest.Id())
	} else if service.TaskDefinition.ExecutionRole == nil {
		return fmt.Errorf("cannot configure edge %s -> %s, missing task definition execution role", service.Id(), dest.Id())
	}
	return nil
}
