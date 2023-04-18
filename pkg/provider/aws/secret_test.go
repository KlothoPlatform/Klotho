package aws

import (
	"fmt"
	"testing"

	"github.com/klothoplatform/klotho/pkg/annotation"
	"github.com/klothoplatform/klotho/pkg/config"
	"github.com/klothoplatform/klotho/pkg/core"
	"github.com/klothoplatform/klotho/pkg/core/coretesting"
	"github.com/klothoplatform/klotho/pkg/provider/aws/resources"
	"github.com/stretchr/testify/assert"
)

// (The filename says this is a secret test! So shh, don't tell anyone about it!)

func TestGenerateSecretsResources(t *testing.T) {
	const AppName = "AppName"
	const secretsConstructId = "MySecrets"

	execUnit := &core.ExecutionUnit{AnnotationKey: core.AnnotationKey{ID: "TestUnit", Capability: annotation.ExecutionUnitCapability}}

	cases := []struct {
		name         string
		secrets      []string
		execUnit     *core.ExecutionUnit
		want         coretesting.ResourcesExpectation
		wantPolicies func(secretResolver func(string) *resources.Secret) []resources.StatementEntry
	}{
		{
			name:     "two secrets",
			secrets:  []string{`secret1`, `secret2`},
			execUnit: execUnit,
			want: coretesting.ResourcesExpectation{
				Nodes: []string{
					"aws:iam_policy:AppName-persist_MySecrets",
					"aws:secret:AppName-MySecrets-secret1",
					"aws:secret:AppName-MySecrets-secret2",
					"aws:secret_version:AppName-MySecrets-secret1",
					"aws:secret_version:AppName-MySecrets-secret2",
				},
				Deps: []coretesting.StringDep{
					{Source: "aws:iam_policy:AppName-persist_MySecrets", Destination: "aws:secret_version:AppName-MySecrets-secret1"},
					{Source: "aws:iam_policy:AppName-persist_MySecrets", Destination: "aws:secret_version:AppName-MySecrets-secret2"},
					{Source: "aws:secret_version:AppName-MySecrets-secret1", Destination: "aws:secret:AppName-MySecrets-secret1"},
					{Source: "aws:secret_version:AppName-MySecrets-secret2", Destination: "aws:secret:AppName-MySecrets-secret2"},
				},
			},
			wantPolicies: func(secretResolver func(string) *resources.Secret) []resources.StatementEntry {
				secret1 := secretResolver(fmt.Sprintf(`aws:secret:%s-%s-secret1`, AppName, secretsConstructId))
				secret2 := secretResolver(fmt.Sprintf(`aws:secret:%s-%s-secret2`, AppName, secretsConstructId))
				return []resources.StatementEntry{
					{
						Effect: "Allow",
						Action: []string{`secretsmanager:DescribeSecret`, `secretsmanager:GetSecretValue`},
						Resource: []core.IaCValue{
							{
								Resource: secret1,
								Property: resources.ARN_IAC_VALUE,
							},
						},
					},
					{
						Effect: "Allow",
						Action: []string{`secretsmanager:DescribeSecret`, `secretsmanager:GetSecretValue`},
						Resource: []core.IaCValue{
							{
								Resource: secret2,
								Property: resources.ARN_IAC_VALUE,
							},
						},
					},
				}
			},
		},
		{
			name:         "no secrets",
			execUnit:     execUnit,
			want:         coretesting.ResourcesExpectation{},
			wantPolicies: func(secretResolver func(string) *resources.Secret) []resources.StatementEntry { return nil },
		},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			assert := assert.New(t)
			aws := AWS{
				Config: &config.Application{
					AppName: AppName,
				},
				PolicyGenerator: resources.NewPolicyGenerator(),
			}
			secretsConstruct := &core.Secrets{
				AnnotationKey: core.AnnotationKey{ID: secretsConstructId, Capability: annotation.PersistCapability},
				Secrets:       tt.secrets,
			}

			constructGraph := core.NewConstructGraph()
			constructGraph.AddConstruct(secretsConstruct)
			if tt.execUnit != nil {
				constructGraph.AddConstruct(tt.execUnit)
				constructGraph.AddDependency(execUnit.Id(), secretsConstruct.Id())
			}

			dag := core.NewResourceGraph()
			err := aws.GenerateSecretsResources(secretsConstruct, constructGraph, dag)
			if !assert.NoError(err) {
				return
			}

			tt.want.Assert(t, dag)

			wantPolicies := tt.wantPolicies(func(secretId string) *resources.Secret {
				resource := dag.GetResource(secretId)
				assert.NotNil(resource)
				if secret, foundSecret := resource.(*resources.Secret); foundSecret {
					return secret
				} else {
					assert.Failf("resource not a Secret", `found a resource with id="%s", but it was %T`, secretId, resource)
				}
				return nil
			})
			var actualPolicies []resources.StatementEntry
			if policies := aws.PolicyGenerator.GetUnitPolicies(execUnit.Id()); policies != nil {
				actualPolicies = policies[0].Policy.Statement
			}
			assert.Equal(wantPolicies, actualPolicies)
		})
	}
}
