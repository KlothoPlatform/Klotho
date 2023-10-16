package operational_rule

import (
	"errors"
	"fmt"

	construct "github.com/klothoplatform/klotho/pkg/construct2"
	"github.com/klothoplatform/klotho/pkg/engine2/solution_context"
	knowledgebase "github.com/klothoplatform/klotho/pkg/knowledge_base2"
	"go.uber.org/zap"
)

type (
	OperationalRuleContext struct {
		Solution solution_context.SolutionContext
		Property *knowledgebase.Property
		Data     knowledgebase.DynamicValueData
	}

	Result struct {
		CreatedResources  []*construct.Resource
		AddedDependencies []construct.Edge
	}
)

func (ctx OperationalRuleContext) HandleOperationalRule(rule knowledgebase.OperationalRule) (Result, error) {
	if rule.If != "" {
		result := false
		dyn := solution_context.DynamicCtx(ctx.Solution)
		err := dyn.ExecuteDecode(rule.If, ctx.Data, &result)
		if err != nil {
			return Result{}, fmt.Errorf("could not evaluate if condition for rule: %w", err)
		}
		if !result {
			zap.S().Debugf("rule did not match if condition, skipping")
			return Result{}, nil
		}
	}

	var result Result

	var errs error
	for i, operationalStep := range rule.Steps {
		stepResult, err := ctx.HandleOperationalStep(operationalStep)
		if err != nil {
			errs = errors.Join(errs, fmt.Errorf("could not apply step %d: %w", i, err))
			continue
		}
		result.Append(stepResult)
	}

	for i, operationalConfig := range rule.ConfigurationRules {
		err := ctx.HandleConfigurationRule(operationalConfig)
		if err != nil {
			errs = errors.Join(errs, fmt.Errorf("could not apply configuration rule %d: %w", i, err))
		}
	}

	return result, errs
}

func (r *Result) Append(other Result) {
	r.CreatedResources = append(r.CreatedResources, other.CreatedResources...)
	r.AddedDependencies = append(r.AddedDependencies, other.AddedDependencies...)
}
