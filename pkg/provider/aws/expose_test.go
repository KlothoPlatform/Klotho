package aws

import (
	"testing"

	"github.com/klothoplatform/klotho/pkg/annotation"
	"github.com/klothoplatform/klotho/pkg/config"
	"github.com/klothoplatform/klotho/pkg/core"
	"github.com/klothoplatform/klotho/pkg/core/coretesting"
	"github.com/stretchr/testify/assert"
)

func Test_ExpandExpose(t *testing.T) {
	gw := &core.Gateway{AnnotationKey: core.AnnotationKey{ID: "test", Capability: annotation.ExecutionUnitCapability}}
	cases := []struct {
		name    string
		gw      *core.Gateway
		config  *config.Application
		want    coretesting.ResourcesExpectation
		wantErr bool
	}{
		{
			name:   "single expose",
			gw:     gw,
			config: &config.Application{AppName: "my-app", Defaults: config.Defaults{Expose: config.KindDefaults{Type: ApiGateway}}},
			want: coretesting.ResourcesExpectation{
				Nodes: []string{
					"aws:api_deployment:my-app-test",
					"aws:api_stage:my-app-test",
					"aws:rest_api:my-app-test",
				},
				Deps: []coretesting.StringDep{
					{Source: "aws:api_deployment:my-app-test", Destination: "aws:rest_api:my-app-test"},
					{Source: "aws:api_stage:my-app-test", Destination: "aws:api_deployment:my-app-test"},
					{Source: "aws:api_stage:my-app-test", Destination: "aws:rest_api:my-app-test"},
				},
			},
		},
		{
			name:    "unsupported type",
			gw:      gw,
			config:  &config.Application{AppName: "my-app", Defaults: config.Defaults{Expose: config.KindDefaults{Type: Lambda}}},
			wantErr: true,
		},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			assert := assert.New(t)
			dag := core.NewResourceGraph()

			aws := AWS{
				Config: tt.config,
			}
			err := aws.expandExpose(dag, tt.gw)

			if tt.wantErr {
				assert.Error(err)
				return
			}
			if !assert.NoError(err) {
				return
			}
			tt.want.Assert(t, dag)
			resource := aws.GetResourceTiedToConstruct(tt.gw)
			assert.NotNil(resource)
		})
	}
}
