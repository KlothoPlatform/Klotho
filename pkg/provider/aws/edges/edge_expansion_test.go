package edges

import (
	"testing"

	dgraph "github.com/dominikbraun/graph"
	"github.com/klothoplatform/klotho/pkg/annotation"
	"github.com/klothoplatform/klotho/pkg/core"
	"github.com/klothoplatform/klotho/pkg/core/coretesting"
	"github.com/klothoplatform/klotho/pkg/graph"
	"github.com/klothoplatform/klotho/pkg/provider/aws/resources"
	"github.com/stretchr/testify/assert"
)

func Test_ExpandEdges(t *testing.T) {
	orm := &core.Orm{AnnotationKey: core.AnnotationKey{ID: "test", Capability: annotation.PersistCapability}}
	cases := []struct {
		name string
		edge graph.Edge[core.Resource]
		want coretesting.ResourcesExpectation
	}{
		{
			name: "single rds orm",
			edge: graph.Edge[core.Resource]{
				Source:      &resources.LambdaFunction{Name: "lambda"},
				Destination: &resources.RdsInstance{Name: "rds"},
				Properties: dgraph.EdgeProperties{
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
				},
			},
			want: coretesting.ResourcesExpectation{
				Nodes: []string{
					"aws:availability_zones:AvailabilityZones",
					"aws:elastic_ip:my_app_0",
					"aws:elastic_ip:my_app_1",
					"aws:iam_policy:my-app-my-app-rds-ormsecretpolicy",
					"aws:iam_role:my-app-rds-ProxyRole",
					"aws:internet_gateway:my_app_igw",
					"aws:lambda_function:lambda",
					"aws:nat_gateway:my_app_0",
					"aws:nat_gateway:my_app_1",
					"aws:rds_instance:rds",
					"aws:rds_proxy:my-app-rds",
					"aws:rds_proxy_target_group:my-app-rds",
					"aws:route_table:my_app_private0",
					"aws:route_table:my_app_private1",
					"aws:route_table:my_app_public",
					"aws:secret:my-app-my-app-rds",
					"aws:secret_version:my-app-my-app-rds",
					"aws:security_group:my_app:my-app",
					"aws:subnet_private:my_app:my_app_private0",
					"aws:subnet_private:my_app:my_app_private1",
					"aws:subnet_public:my_app:my_app_public0",
					"aws:subnet_public:my_app:my_app_public1",
					"aws:vpc:my_app",
				},
				Deps: []coretesting.StringDep{
					{Source: "aws:iam_policy:my-app-my-app-rds-ormsecretpolicy", Destination: "aws:secret_version:my-app-my-app-rds"},
					{Source: "aws:iam_role:my-app-rds-ProxyRole", Destination: "aws:iam_policy:my-app-my-app-rds-ormsecretpolicy"},
					{Source: "aws:internet_gateway:my_app_igw", Destination: "aws:vpc:my_app"},
					{Source: "aws:nat_gateway:my_app_0", Destination: "aws:elastic_ip:my_app_0"},
					{Source: "aws:nat_gateway:my_app_0", Destination: "aws:subnet_public:my_app:my_app_public0"},
					{Source: "aws:nat_gateway:my_app_1", Destination: "aws:elastic_ip:my_app_1"},
					{Source: "aws:nat_gateway:my_app_1", Destination: "aws:subnet_public:my_app:my_app_public1"},
					{Source: "aws:rds_proxy:my-app-rds", Destination: "aws:iam_role:my-app-rds-ProxyRole"},
					{Source: "aws:rds_proxy:my-app-rds", Destination: "aws:secret:my-app-my-app-rds"},
					{Source: "aws:rds_proxy:my-app-rds", Destination: "aws:secret_version:my-app-my-app-rds"},
					{Source: "aws:rds_proxy:my-app-rds", Destination: "aws:security_group:my_app:my-app"},
					{Source: "aws:rds_proxy:my-app-rds", Destination: "aws:subnet_private:my_app:my_app_private0"},
					{Source: "aws:rds_proxy:my-app-rds", Destination: "aws:subnet_private:my_app:my_app_private1"},
					{Source: "aws:rds_proxy_target_group:my-app-rds", Destination: "aws:rds_instance:rds"},
					{Source: "aws:rds_proxy_target_group:my-app-rds", Destination: "aws:rds_proxy:my-app-rds"},
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
					{Source: "aws:secret_version:my-app-my-app-rds", Destination: "aws:secret:my-app-my-app-rds"},
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

			dag.AddDependencyWithData(tt.edge.Source, tt.edge.Destination, tt.edge.Properties.Data)

			err := core.ExpandEdges(KnowledgeBase, dag)
			if !assert.NoError(err) {
				return
			}
			tt.want.Assert(t, dag)
		})
	}
}
