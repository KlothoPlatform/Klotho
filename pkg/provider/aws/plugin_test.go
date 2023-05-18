package aws

import (
	"fmt"
	"testing"

	dgraph "github.com/dominikbraun/graph"
	"github.com/klothoplatform/klotho/pkg/annotation"
	"github.com/klothoplatform/klotho/pkg/config"
	"github.com/klothoplatform/klotho/pkg/core"
	"github.com/klothoplatform/klotho/pkg/core/coretesting"
	"github.com/klothoplatform/klotho/pkg/graph"
	"github.com/klothoplatform/klotho/pkg/infra/kubernetes"
	"github.com/klothoplatform/klotho/pkg/provider/aws/resources"
	"github.com/stretchr/testify/assert"
)

func Test_ExpandConstructs(t *testing.T) {
	eu := &core.ExecutionUnit{AnnotationKey: core.AnnotationKey{ID: "test", Capability: annotation.ExecutionUnitCapability}, DockerfilePath: "path"}
	orm := &core.Orm{AnnotationKey: core.AnnotationKey{ID: "test", Capability: annotation.PersistCapability}}
	cases := []struct {
		name       string
		constructs []core.Construct
		config     *config.Application
		want       coretesting.ResourcesExpectation
	}{
		{
			name:       "lambda and rds",
			constructs: []core.Construct{eu, orm},
			config: &config.Application{
				AppName: "my-app",
				Defaults: config.Defaults{
					ExecutionUnit: config.KindDefaults{Type: Lambda},
					PersistOrm:    defaultConfig.PersistOrm,
				},
			},
			want: coretesting.ResourcesExpectation{
				Nodes: []string{
					"aws:availability_zones:AvailabilityZones",
					"aws:ecr_image:my-app-test",
					"aws:ecr_repo:my-app",
					"aws:elastic_ip:my_app_0",
					"aws:elastic_ip:my_app_1",
					"aws:iam_role:my-app-test-ExecutionRole",
					"aws:internet_gateway:my_app_igw",
					"aws:lambda_function:my-app-test",
					"aws:log_group:my-app-test",
					"aws:nat_gateway:my_app_0",
					"aws:nat_gateway:my_app_1",
					"aws:rds_instance:my-app-test",
					"aws:rds_subnet_group:my-app-test",
					"aws:route_table:my_app_private0",
					"aws:route_table:my_app_private1",
					"aws:route_table:my_app_public",
					"aws:security_group:my_app:my-app",
					"aws:subnet_private:my_app:my_app_private0",
					"aws:subnet_private:my_app:my_app_private1",
					"aws:subnet_public:my_app:my_app_public0",
					"aws:subnet_public:my_app:my_app_public1",
					"aws:vpc:my_app",
				},
				Deps: []coretesting.StringDep{
					{Source: "aws:ecr_image:my-app-test", Destination: "aws:ecr_repo:my-app"},
					{Source: "aws:internet_gateway:my_app_igw", Destination: "aws:vpc:my_app"},
					{Source: "aws:lambda_function:my-app-test", Destination: "aws:ecr_image:my-app-test"},
					{Source: "aws:lambda_function:my-app-test", Destination: "aws:iam_role:my-app-test-ExecutionRole"},
					{Source: "aws:lambda_function:my-app-test", Destination: "aws:log_group:my-app-test"},
					{Source: "aws:nat_gateway:my_app_0", Destination: "aws:elastic_ip:my_app_0"},
					{Source: "aws:nat_gateway:my_app_0", Destination: "aws:subnet_public:my_app:my_app_public0"},
					{Source: "aws:nat_gateway:my_app_1", Destination: "aws:elastic_ip:my_app_1"},
					{Source: "aws:nat_gateway:my_app_1", Destination: "aws:subnet_public:my_app:my_app_public1"},
					{Source: "aws:rds_instance:my-app-test", Destination: "aws:rds_subnet_group:my-app-test"},
					{Source: "aws:rds_instance:my-app-test", Destination: "aws:security_group:my_app:my-app"},
					{Source: "aws:rds_subnet_group:my-app-test", Destination: "aws:subnet_private:my_app:my_app_private0"},
					{Source: "aws:rds_subnet_group:my-app-test", Destination: "aws:subnet_private:my_app:my_app_private1"},
					{Source: "aws:route_table:my_app_private0", Destination: "aws:nat_gateway:my_app_0"},
					{Source: "aws:route_table:my_app_private0", Destination: "aws:subnet_private:my_app:my_app_private0"},
					{Source: "aws:route_table:my_app_private0", Destination: "aws:vpc:my_app"},
					{Source: "aws:route_table:my_app_private1", Destination: "aws:nat_gateway:my_app_1"},
					{Source: "aws:route_table:my_app_private1", Destination: "aws:subnet_private:my_app:my_app_private1"},
					{Source: "aws:route_table:my_app_private1", Destination: "aws:vpc:my_app"},
					{Source: "aws:route_table:my_app_public", Destination: "aws:internet_gateway:my_app_igw"},
					{Source: "aws:route_table:my_app_public", Destination: "aws:subnet_public:my_app:my_app_public0"},
					{Source: "aws:route_table:my_app_public", Destination: "aws:subnet_public:my_app:my_app_public1"},
					{Source: "aws:route_table:my_app_public", Destination: "aws:vpc:my_app"},
					{Source: "aws:security_group:my_app:my-app", Destination: "aws:vpc:my_app"},
					{Source: "aws:subnet_private:my_app:my_app_private0", Destination: "aws:availability_zones:AvailabilityZones"},
					{Source: "aws:subnet_private:my_app:my_app_private0", Destination: "aws:vpc:my_app"},
					{Source: "aws:subnet_private:my_app:my_app_private1", Destination: "aws:availability_zones:AvailabilityZones"},
					{Source: "aws:subnet_private:my_app:my_app_private1", Destination: "aws:vpc:my_app"},
					{Source: "aws:subnet_public:my_app:my_app_public0", Destination: "aws:availability_zones:AvailabilityZones"},
					{Source: "aws:subnet_public:my_app:my_app_public0", Destination: "aws:vpc:my_app"},
					{Source: "aws:subnet_public:my_app:my_app_public1", Destination: "aws:availability_zones:AvailabilityZones"},
					{Source: "aws:subnet_public:my_app:my_app_public1", Destination: "aws:vpc:my_app"},
				},
			},
		},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			assert := assert.New(t)
			dag := core.NewResourceGraph()
			result := core.NewConstructGraph()

			for _, construct := range tt.constructs {
				result.AddConstruct(construct)
			}

			aws := AWS{
				Config: tt.config,
			}
			err := aws.ExpandConstructs(result, dag)

			if !assert.NoError(err) {
				return
			}
			tt.want.Assert(t, dag)
			fmt.Println(coretesting.ResoucesFromDAG(dag).GoString())
		})
	}
}

func Test_CopyConstructEdgesToDag(t *testing.T) {
	orm := &core.Orm{AnnotationKey: core.AnnotationKey{ID: "test", Capability: annotation.PersistCapability}}

	eu := &core.ExecutionUnit{
		AnnotationKey:        core.AnnotationKey{ID: "test", Capability: annotation.ExecutionUnitCapability},
		EnvironmentVariables: core.EnvironmentVariables{core.GenerateOrmConnStringEnvVar(orm)},
	}
	cases := []struct {
		name                 string
		constructs           []graph.Edge[core.Construct]
		config               *config.Application
		constructResourceMap map[string]core.Resource
		want                 []*graph.Edge[core.Resource]
	}{
		{
			name: "lambda and rds",
			constructs: []graph.Edge[core.Construct]{
				{Source: eu, Destination: orm},
			},
			config: &config.Application{
				AppName: "my-app",
			},
			constructResourceMap: map[string]core.Resource{
				"execution_unit:test": &resources.LambdaFunction{Name: "lambda"},
				"persist:test":        &resources.RdsInstance{Name: "rds"},
			},
			want: []*graph.Edge[core.Resource]{
				{Source: &resources.LambdaFunction{Name: "lambda"}, Destination: &resources.RdsInstance{Name: "rds"}, Properties: dgraph.EdgeProperties{
					Attributes: make(map[string]string),
					Data: core.EdgeData{
						AppName:     "my-app",
						Source:      &resources.LambdaFunction{Name: "lambda"},
						Destination: &resources.RdsInstance{Name: "rds"},
						Constraint: core.EdgeConstraint{
							NodeMustExist:    []core.Resource{&resources.RdsProxy{}},
							NodeMustNotExist: []core.Resource{&resources.IamRole{}},
						},
						EnvironmentVariables: []core.EnvironmentVariable{core.GenerateOrmConnStringEnvVar(orm)},
					},
				}},
			},
		},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			assert := assert.New(t)
			dag := core.NewResourceGraph()
			result := core.NewConstructGraph()

			for _, dep := range tt.constructs {
				result.AddConstruct(dep.Source)
				result.AddConstruct(dep.Destination)
				result.AddDependency(dep.Source.Id(), dep.Destination.Id())
			}
			for _, res := range tt.constructResourceMap {
				dag.AddResource(res)
			}
			aws := AWS{
				Config:                tt.config,
				constructIdToResource: tt.constructResourceMap,
			}
			err := aws.CopyConstructEdgesToDag(result, dag)

			if !assert.NoError(err) {
				return
			}
			for _, dep := range tt.want {
				edge := dag.GetDependency(dep.Source, dep.Destination)
				fmt.Println(edge)
				assert.Equal(edge, dep)
			}
		})
	}
}

func Test_configureResources(t *testing.T) {

	cases := []struct {
		name       string
		config     *config.Application
		constructs []core.Construct
		resources  []core.Resource
		want       []core.Resource
	}{
		{
			name: "lambda and rds",
			config: &config.Application{
				AppName: "my-app",
				ExecutionUnits: map[string]*config.ExecutionUnit{
					"test": &config.ExecutionUnit{
						InfraParams: config.ConvertToInfraParams(config.ServerlessTypeParams{Timeout: 100, Memory: 200}),
					},
				},
			},
			constructs: []core.Construct{
				&core.ExecutionUnit{
					AnnotationKey:        core.AnnotationKey{ID: "test", Capability: annotation.ExecutionUnitCapability},
					EnvironmentVariables: core.EnvironmentVariables{core.NewEnvironmentVariable("env1", nil, "val1")}},
			},
			resources: []core.Resource{&resources.LambdaFunction{Name: "lambda", ConstructsRef: []core.AnnotationKey{{ID: "test", Capability: annotation.ExecutionUnitCapability}}}, &resources.RdsProxy{Name: "rds"}},
			want: []core.Resource{
				&resources.LambdaFunction{Name: "lambda", Timeout: 100, MemorySize: 200, ConstructsRef: []core.AnnotationKey{{ID: "test", Capability: annotation.ExecutionUnitCapability}}, EnvironmentVariables: resources.EnvironmentVariables{"env1": core.IaCValue{Property: "val1"}}},
				&resources.RdsProxy{Name: "rds", EngineFamily: "POSTGRESQL", IdleClientTimeout: 1800}},
		},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			assert := assert.New(t)
			dag := core.NewResourceGraph()
			result := core.NewConstructGraph()
			for _, construct := range tt.constructs {
				result.AddConstruct(construct)
			}
			for _, res := range tt.resources {
				dag.AddResource(res)
			}
			aws := AWS{
				Config: tt.config,
			}
			err := aws.configureResources(result, dag)

			if !assert.NoError(err) {
				return
			}
			for _, res := range tt.want {
				graphRes := dag.GetResource(res.Id())
				assert.Equal(graphRes, res)
			}
		})
	}
}

func Test_shouldCreateNetwork(t *testing.T) {
	cases := []struct {
		name       string
		constructs []core.Construct
		config     *config.Application
		want       bool
	}{
		{
			name:       "lambda",
			constructs: []core.Construct{&core.ExecutionUnit{}},
			config:     &config.Application{Defaults: config.Defaults{ExecutionUnit: config.KindDefaults{Type: Lambda}}},
			want:       false,
		},
		{
			name:       "kubernetes",
			constructs: []core.Construct{&core.ExecutionUnit{}},
			config:     &config.Application{Defaults: config.Defaults{ExecutionUnit: config.KindDefaults{Type: kubernetes.KubernetesType}}},
			want:       true,
		},
		{
			name:       "orm",
			constructs: []core.Construct{&core.Orm{}},
			want:       true,
		},
		{
			name:       "redis Node",
			constructs: []core.Construct{&core.RedisNode{}},
			want:       true,
		},
		{
			name:       "redis Cluster",
			constructs: []core.Construct{&core.RedisCluster{}},
			want:       true,
		},
		{
			name: "remaining resources",
			constructs: []core.Construct{
				&core.StaticUnit{},
				&core.Secrets{},
				&core.Fs{},
				&core.Kv{},
				&core.Config{},
				&core.InternalResource{},
				&core.Gateway{},
				&core.PubSub{},
			},
			want: false,
		},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			assert := assert.New(t)
			aws := AWS{
				Config: tt.config,
			}
			result := core.NewConstructGraph()
			for _, construct := range tt.constructs {
				result.AddConstruct(construct)
			}
			should, err := aws.shouldCreateNetwork(result)
			if !assert.NoError(err) {
				return
			}
			assert.Equal(tt.want, should)
		})

	}
}

func Test_createEksClusters(t *testing.T) {
	cases := []struct {
		name   string
		units  []*core.ExecutionUnit
		config *config.Application
		want   []*resources.EksCluster
	}{
		{
			name: `no clusters created`,
			units: []*core.ExecutionUnit{
				{AnnotationKey: core.AnnotationKey{ID: "test", Capability: annotation.ExecutionUnitCapability}},
			},
			config: &config.Application{
				AppName: "test",
				ExecutionUnits: map[string]*config.ExecutionUnit{
					"test": {Type: Lambda},
				},
			},
		},
		{
			name: `one exec unit, no cluster id`,
			units: []*core.ExecutionUnit{
				{AnnotationKey: core.AnnotationKey{ID: "test", Capability: annotation.ExecutionUnitCapability}},
			},
			config: &config.Application{
				AppName: "test",
				ExecutionUnits: map[string]*config.ExecutionUnit{
					"test": {Type: kubernetes.KubernetesType},
				},
			},
			want: []*resources.EksCluster{
				{Name: "test-eks-cluster", ConstructsRef: []core.AnnotationKey{
					{ID: "test", Capability: annotation.ExecutionUnitCapability},
				}},
			},
		},
		{
			name: `one exec unit, none eks`,
			units: []*core.ExecutionUnit{
				{AnnotationKey: core.AnnotationKey{ID: "test", Capability: annotation.ExecutionUnitCapability}},
			},
			config: &config.Application{
				AppName: "test",
			},
		},
		{
			name: `two eks units, unassigned`,
			units: []*core.ExecutionUnit{
				{AnnotationKey: core.AnnotationKey{ID: "test", Capability: annotation.ExecutionUnitCapability}},
				{AnnotationKey: core.AnnotationKey{ID: "test2", Capability: annotation.ExecutionUnitCapability}},
			},
			config: &config.Application{
				AppName: "test",
				ExecutionUnits: map[string]*config.ExecutionUnit{
					"test":  {Type: kubernetes.KubernetesType},
					"test2": {Type: kubernetes.KubernetesType},
				},
			},
			want: []*resources.EksCluster{
				{Name: "test-eks-cluster", ConstructsRef: []core.AnnotationKey{
					{ID: "test", Capability: annotation.ExecutionUnitCapability},
					{ID: "test2", Capability: annotation.ExecutionUnitCapability},
				}},
			},
		},
		{
			name: `two eks units, one unassigned`,
			units: []*core.ExecutionUnit{
				{AnnotationKey: core.AnnotationKey{ID: "test", Capability: annotation.ExecutionUnitCapability}},
				{AnnotationKey: core.AnnotationKey{ID: "test2", Capability: annotation.ExecutionUnitCapability}},
			},
			config: &config.Application{
				AppName: "test",
				ExecutionUnits: map[string]*config.ExecutionUnit{
					"test":  {Type: kubernetes.KubernetesType},
					"test2": {Type: kubernetes.KubernetesType, InfraParams: config.ConvertToInfraParams(config.KubernetesTypeParams{ClusterId: "cluster2"})},
				},
			},
			want: []*resources.EksCluster{
				{Name: "test-cluster2", ConstructsRef: []core.AnnotationKey{
					{ID: "test", Capability: annotation.ExecutionUnitCapability},
					{ID: "test2", Capability: annotation.ExecutionUnitCapability},
				}},
			},
		},
		{
			name: `two eks units, separate assignment`,
			units: []*core.ExecutionUnit{
				{AnnotationKey: core.AnnotationKey{ID: "test", Capability: annotation.ExecutionUnitCapability}},
				{AnnotationKey: core.AnnotationKey{ID: "test2", Capability: annotation.ExecutionUnitCapability}},
			},
			config: &config.Application{
				AppName: "test",
				ExecutionUnits: map[string]*config.ExecutionUnit{
					"test":  {Type: kubernetes.KubernetesType, InfraParams: config.ConvertToInfraParams(config.KubernetesTypeParams{ClusterId: "cluster1"})},
					"test2": {Type: kubernetes.KubernetesType, InfraParams: config.ConvertToInfraParams(config.KubernetesTypeParams{ClusterId: "cluster2"})},
				},
			},
			want: []*resources.EksCluster{
				{Name: "test-cluster1", ConstructsRef: []core.AnnotationKey{
					{ID: "test", Capability: annotation.ExecutionUnitCapability},
				}},
				{Name: "test-cluster2", ConstructsRef: []core.AnnotationKey{
					{ID: "test2", Capability: annotation.ExecutionUnitCapability},
				}},
			},
		},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			assert := assert.New(t)
			aws := AWS{
				Config: tt.config,
			}

			result := core.NewConstructGraph()
			for _, unit := range tt.units {
				result.AddConstruct(unit)
			}
			dag := core.NewResourceGraph()

			err := aws.createEksClusters(result, dag)
			if !assert.NoError(err) {
				return
			}
			numEksClusters := 0
			for _, res := range dag.ListResources() {
				if _, ok := res.(*resources.EksCluster); ok {
					numEksClusters++
				}
			}
			assert.Equal(numEksClusters, len(tt.want))

			for _, cluster := range tt.want {
				resource := dag.GetResource(cluster.Id())
				if !assert.NotNil(resource, fmt.Sprintf("Did not find cluster with id, %s", cluster.Id())) {
					return
				}
				assert.ElementsMatch(resource.KlothoConstructRef(), cluster.ConstructsRef)
			}

			if len(tt.want) > 0 {
				sg := resources.GetSecurityGroup(aws.Config, dag)
				assert.Contains(sg.IngressRules, resources.SecurityGroupRule{
					Description: "Allows ingress traffic from the EKS control plane",
					FromPort:    9443,
					Protocol:    "TCP",
					ToPort:      9443,
					CidrBlocks: []core.IaCValue{
						{Property: "0.0.0.0/0"},
					},
				})
			}
		})

	}
}
