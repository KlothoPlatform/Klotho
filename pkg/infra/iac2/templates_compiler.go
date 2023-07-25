package iac2

import (
	"bytes"
	"embed"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"path"
	"reflect"
	"sort"
	"strings"
	"text/template"

	"github.com/klothoplatform/klotho/pkg/provider/imports"

	"github.com/klothoplatform/klotho/pkg/core"
	"github.com/klothoplatform/klotho/pkg/lang/javascript"
	"github.com/klothoplatform/klotho/pkg/multierr"
	"github.com/klothoplatform/klotho/pkg/provider/aws/resources"
	kubernetes "github.com/klothoplatform/klotho/pkg/provider/kubernetes/resources"
	"github.com/pkg/errors"
	"go.uber.org/zap"
)

type (
	stringTemplateValue struct {
		raw   any
		value string
	}

	templateValue interface {
		Parse() (string, error)
		Raw() any
	}

	templatesProvider struct {
		// templates is the fs.FS where we read all of our `<struct>/factory.ts` files
		templates fs.FS
		// resourceTemplatesByStructName is a cache from struct name (e.g. "CloudwatchLogs") to the template for that struct.
		resourceTemplatesByStructName map[string]ResourceCreationTemplate
		childTemplatesByPath          map[string]*template.Template
	}

	// TemplatesCompiler renders a graph of [core.Resource] nodes by combining each one with its corresponding
	// ResourceCreationTemplate
	TemplatesCompiler struct {
		*templatesProvider
		// resourceGraph is the graph of resources to render
		resourceGraph *core.ResourceGraph // TODO make this be a core.ResourceGraph, and un-expose that struct's Underlying
		// resourceVarNames is a set of all variable names
		resourceVarNames map[string]struct{}
		// resourceVarNamesById is a map from resource id to the variable name for that resource
		resourceVarNamesById map[core.ResourceId]string
		// ctx is a pointer to the current context being used within the templates compiler. This context is used when parsing values within nested templates.
		ctx *NestedCtx
	}
	NestedCtx struct {
		useDoubleQuotes bool
		appliedOutputs  *[]AppliedOutput
		rootVal         *reflect.Value
	}
)

var (
	//go:embed templates/*/factory.ts templates/*/package.json templates/*/*.ts.tmpl
	standardTemplates embed.FS
)

var (
	errType = reflect.TypeOf((*error)(nil)).Elem()
)

func (s stringTemplateValue) Parse() (string, error) {
	return s.value, nil
}

func (s stringTemplateValue) Raw() interface{} {
	return s.raw
}

func CreateTemplatesCompiler(resources *core.ResourceGraph) *TemplatesCompiler {
	return &TemplatesCompiler{
		templatesProvider:    standardTemplatesProvider(),
		resourceGraph:        resources,
		resourceVarNames:     make(map[string]struct{}),
		resourceVarNamesById: make(map[core.ResourceId]string),
	}
}

func standardTemplatesProvider() *templatesProvider {
	subTemplates, err := fs.Sub(standardTemplates, "templates")
	if err != nil {
		panic(err) // unexpected, since standardTemplates is statically built into klotho
	}
	return &templatesProvider{
		templates:                     subTemplates,
		resourceTemplatesByStructName: make(map[string]ResourceCreationTemplate),
		childTemplatesByPath:          make(map[string]*template.Template),
	}
}

func (tc TemplatesCompiler) RenderBody(out io.Writer) error {
	errs := multierr.Error{}
	res, err := tc.resourceGraph.ReverseTopologicalSort()
	if err != nil {
		return err
	}
	for i, resource := range res {
		switch resource.(type) {
		case *resources.AccountId, *resources.Region:
			continue // skip resources that we know are rendered outside of the body
		case *imports.Imported:
			// Imported resources are handled by the rendering of their base resource
			//? Should this ignore all .Provider == "internal" instead?
			continue
		}
		err := tc.renderResource(out, resource)
		errs.Append(err)
		if i < len(res)-1 {
			_, err = out.Write([]byte("\n\n"))
			if err != nil {
				return err
			}
		}
	}
	return errs.ErrOrNil()
}

func (tc TemplatesCompiler) RenderImports(out io.Writer) error {
	errs := multierr.Error{}

	allImports := make(map[string]struct{})
	for _, res := range tc.resourceGraph.ListResources() {
		switch res.(type) {
		case *imports.Imported:
			continue
		}
		tmpl, err := tc.getTemplate(res)
		if err != nil {
			errs.Append(err)
			continue
		}
		for statement := range tmpl.Imports {
			allImports[statement] = struct{}{}
		}
	}
	if err := errs.ErrOrNil(); err != nil {
		return err
	}

	sortedImports := make([]string, 0, len(allImports))
	for statement := range allImports {
		sortedImports = append(sortedImports, statement)
	}

	sort.Strings(sortedImports)
	for _, statement := range sortedImports {
		if _, err := out.Write([]byte(statement)); err != nil {
			return err
		}
		if _, err := out.Write([]byte("\n")); err != nil {
			return err
		}
	}

	return nil
}

func (tc TemplatesCompiler) RenderPackageJSON() (*javascript.NodePackageJson, error) {
	errs := multierr.Error{}
	mainPJson := javascript.NodePackageJson{}
	for _, res := range tc.resourceGraph.ListResources() {
		pJson, err := tc.GetPackageJSON(res)
		if err != nil {
			errs.Append(err)
			continue
		}
		if pJson != nil {
			mainPJson.Merge(pJson)
		}
	}
	if err := errs.ErrOrNil(); err != nil {
		return &mainPJson, err
	}
	return &mainPJson, nil
}

func validTemplateMethod(method reflect.Value) error {
	if !method.IsValid() {
		return errors.New("no method found")
	}
	if method.Type().NumIn() != 0 {
		return errors.Errorf("too many inputs (%d) in method", method.Type().NumIn())
	}
	if method.Type().NumOut() < 1 || method.Type().NumOut() > 2 {
		return errors.Errorf("invalid number of outputs (%d) in method", method.Type().NumOut())
	}
	if method.Type().NumOut() > 1 && !method.Type().Out(1).Implements(errType) {
		return errors.New("second output of method must be an error")
	}
	return nil
}

func (tc TemplatesCompiler) renderResource(out io.Writer, resource core.Resource) error {
	defer func() {
		r := recover()
		if r == nil {
			return
		}
		panic(errors.Errorf("panic rendering resource %s: %v", resource.Id(), r))
	}()

	tmpl, err := tc.getTemplate(resource)
	if err != nil {
		return err
	}

	deps := tc.resourceGraph.GetDownstreamResources(resource)
	for _, dep := range deps {
		imp, ok := dep.(*imports.Imported)
		if ok {
			return tc.renderResourceImport(out, resource, imp, tmpl)
		}
	}

	errs := multierr.Error{}

	baseResourceVal := reflect.ValueOf(resource)
	resourceVal := baseResourceVal
	for resourceVal.Kind() == reflect.Pointer {
		resourceVal = resourceVal.Elem()
	}
	inputArgs := make(map[string]templateValue)
	for fieldName := range tmpl.InputTypes {
		func(fieldName string) {
			defer func() {
				r := recover()
				if r == nil {
					return
				}
				panic(errors.Errorf("panic rendering field %s: %v", fieldName, r))
			}()
			switch fieldName {
			// dependsOn will be a reserved field for us to use to map dependencies. If specified as an Arg we will automatically call resolveDependencies
			case "dependsOn":
				inputArgs[fieldName] = stringTemplateValue{value: tc.resolveDependencies(resource)}
				return
			case "protect":
				inputArgs[fieldName] = stringTemplateValue{value: "protect", raw: "protect"}
				return
			case "awsProfile":
				inputArgs[fieldName] = stringTemplateValue{value: "awsProfile", raw: "awsProfile"}
				return
			}
			childVal := resourceVal.FieldByName(fieldName)
			if !childVal.IsValid() {
				// Not a field, try method similar to how text/template does
				method := resourceVal.MethodByName(fieldName)
				if !method.IsValid() {
					// Not a method with non-pointer receiver, try on the base value for pointer receiver method
					method = baseResourceVal.MethodByName(fieldName)
				}
				if err := validTemplateMethod(method); err != nil {
					errs.Append(err)
					return
				}
				eval := method.Call(nil)
				childVal = eval[0]
			}

			var appliedoutputs []AppliedOutput
			buf := strings.Builder{}
			strValue, err := tc.resolveStructInput(&resourceVal, childVal, false, &appliedoutputs)
			if err != nil {
				errs.Append(err)
				return
			}
			uniqueOutputs, err := deduplicateAppliedOutputs(appliedoutputs)
			if err != nil {
				errs.Append(err)
				return
			}
			_, err = buf.WriteString(appliedOutputsToString(uniqueOutputs))
			if err != nil {
				errs.Append(err)
				return
			}
			buf.WriteString(strValue)
			if len(uniqueOutputs) > 0 {
				_, err = buf.WriteString("})")
				if err != nil {
					errs.Append(err)
					return
				}
			}

			var rawVal any
			if childVal.IsValid() {
				rawVal = childVal.Interface()
			}

			resolvedValue := stringTemplateValue{value: buf.String(), raw: rawVal}

			if err != nil {
				errs.Append(err)
			} else {
				inputArgs[fieldName] = resolvedValue
			}
		}(fieldName)
	}
	if err := errs.ErrOrNil(); err != nil {
		return err
	}

	if tmpl.OutputType != "void" {
		varName := tc.getVarName(resource)
		fmt.Fprintf(out, `const %s = `, varName)
	}
	errs.Append(tmpl.RenderCreate(out, inputArgs, tc))
	_, err = out.Write([]byte(";"))
	if err != nil {
		return err
	}
	errs.Append(tc.renderGlueVars(out, resource))
	return errs.ErrOrNil()
}

// resolveDependencies creates a string which models an array containing all the variable names, which the resource depends on.
func (tc TemplatesCompiler) resolveDependencies(resource core.Resource) string {
	type pulumiAllWrap struct {
		actualVars []string
		methodVars []string
	}
	var wrapping pulumiAllWrap
	buf := strings.Builder{}
	buf.WriteRune('[')
	upstreamResources := tc.resourceGraph.GetDownstreamResources(resource)
	numDeps := len(upstreamResources)
	for i := 0; i < numDeps; i++ {
		res := upstreamResources[i]
		switch res.(type) {
		case *resources.Region, *resources.AvailabilityZones, *resources.AccountId:
			continue
		case *kubernetes.HelmChart:
			wrapping.actualVars = append(wrapping.actualVars, fmt.Sprintf("%s.ready", tc.getVarName(res)))
			wrapping.methodVars = append(wrapping.methodVars, tc.getVarName(res))
			buf.WriteString(fmt.Sprintf("...%s,", tc.getVarName(res)))
		case *kubernetes.Manifest, *kubernetes.KustomizeDirectory:
			wrapping.actualVars = append(wrapping.actualVars, fmt.Sprintf("%s.resources", tc.getVarName(res)))
			wrapping.methodVars = append(wrapping.methodVars, tc.getVarName(res))
			buf.WriteString(fmt.Sprintf("...Object.values(%s),", tc.getVarName(res)))
		default:
			buf.WriteString(fmt.Sprintf("%s,", tc.getVarName(res)))
		}
	}
	buf.WriteRune(']')
	if len(wrapping.actualVars) == 0 {
		return buf.String()
	}
	wrappedBuf := strings.Builder{}
	wrappedBuf.WriteString("pulumi.all([")
	for i := 0; i < len(wrapping.actualVars); i++ {
		wrappedBuf.WriteString(wrapping.actualVars[i])
		if i < len(wrapping.actualVars)-1 {
			wrappedBuf.WriteString(", ")
		}
	}
	wrappedBuf.WriteString("]).apply(([")
	for i := 0; i < len(wrapping.methodVars); i++ {
		wrappedBuf.WriteString(wrapping.methodVars[i])
		if i < len(wrapping.methodVars)-1 {
			wrappedBuf.WriteString(", ")
		}
	}
	wrappedBuf.WriteString("]) => { return ")
	wrappedBuf.WriteString(buf.String())
	wrappedBuf.WriteString("})")
	return wrappedBuf.String()
}

// resolveStructInput translates a value to a form suitable to inject into the typescript as an input to a function.
func (tc TemplatesCompiler) resolveStructInput(resourceVal *reflect.Value, childVal reflect.Value, useDoubleQuotedStrings bool, appliedOutputs *[]AppliedOutput) (string, error) {
	tc.ctx = &NestedCtx{
		useDoubleQuotes: useDoubleQuotedStrings,
		appliedOutputs:  appliedOutputs,
		rootVal:         resourceVal,
	}
	var zeroValue reflect.Value
	if childVal == zeroValue {
		return `null`, nil
	}
	switch childVal.Kind() {
	case reflect.Bool,
		reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64,
		reflect.Float32, reflect.Float64:
		return fmt.Sprintf("%v", childVal.Interface()), nil
	case reflect.String:
		return quoteTsString(childVal.Interface().(string), useDoubleQuotedStrings), nil
	case reflect.Struct, reflect.Pointer:
		if childVal.Kind() == reflect.Pointer && childVal.IsNil() {
			return "null", nil
		}
		if typedChild, ok := childVal.Interface().(core.Resource); ok {
			return tc.getVarName(typedChild), nil
		} else if typedChild, ok := childVal.Interface().(core.ResourceId); ok {
			return tc.getVarNameByResourceId(typedChild), nil
		} else if typedChild, ok := childVal.Interface().(core.IaCValue); ok {
			output, err := tc.handleIaCValue(typedChild, appliedOutputs, resourceVal)
			if err != nil {
				return output, err
			}
			return output, nil
		} else {
			val := childVal
			correspondingStruct := val
			for correspondingStruct.Kind() == reflect.Pointer {
				correspondingStruct = val.Elem()
			}

			// Check to see if there is a nested tempalte and if there is use that
			tmpl, err := tc.getNestedTemplate(path.Join(
				camelToSnake(resourceVal.Type().Name()),
				camelToSnake(correspondingStruct.Type().Name()),
			), tc)
			if err != nil {
				return "", err
			}
			if tmpl != nil {
				zap.S().Debugf("Rendering nested template %s, for resource %s", tmpl.Name(), correspondingStruct.Type())
				output := bytes.NewBuffer([]byte{})
				err = tmpl.Execute(output, childVal.Interface())
				return output.String(), err
			}
			zap.S().Debugf("Rendering resource %s, as document", correspondingStruct.Type())

			// Last resort, render as a document
			output := strings.Builder{}
			output.WriteString("{")
			for i := 0; i < correspondingStruct.NumField(); i++ {

				childVal := correspondingStruct.Field(i)
				fieldName := correspondingStruct.Type().Field(i).Name

				// If the struct type is PolicyDocument, pass that down to our recursive calls to keep field name upperCased
				if correspondingStruct.Type() == reflect.TypeOf((*resources.PolicyDocument)(nil)).Elem() {
					resourceVal = &correspondingStruct
				}

				resolvedValue, err := tc.resolveStructInput(resourceVal, childVal, false, appliedOutputs)

				if err != nil {
					return output.String(), err
				}

				// If the struct type is not PolicyDocument, we want to camelCase our field names to follow pulumi format
				if resourceVal.Type() != reflect.TypeOf((*resources.PolicyDocument)(nil)).Elem() {
					fieldName = strings.ToLower(string(fieldName[0])) + fieldName[1:]
				}

				// To Prevent us from rendering fields which are not set, only right if the value is non zero for its type
				if !childVal.IsZero() {
					output.WriteString(fmt.Sprintf("%s: %s,\n", fieldName, resolvedValue))
				}
			}
			output.WriteString("}")
			return output.String(), nil
		}
	case reflect.Array, reflect.Slice:
		sliceLen := childVal.Len()

		buf := strings.Builder{}
		buf.WriteRune('[')
		for i := 0; i < sliceLen; i++ {
			output, err := tc.resolveStructInput(resourceVal, childVal.Index(i), false, appliedOutputs)
			if err != nil {
				return output, err
			}
			buf.WriteString(output)
			if i < (sliceLen - 1) {
				buf.WriteRune(',')
			}
		}
		buf.WriteRune(']')
		return buf.String(), nil
	case reflect.Map:
		mapLen := childVal.Len()

		buf := strings.Builder{}
		buf.WriteRune('{')
		for i, key := range childVal.MapKeys() {
			output, err := tc.resolveStructInput(resourceVal, key, true, appliedOutputs)
			if err != nil {
				return output, nil
			}
			// Pulumi requires the conditional fields of policy document to have its keys wrapped in [] so we need special handling here
			if resourceVal.Type() == reflect.TypeOf((*resources.PolicyDocument)(nil)).Elem() && childVal.Type() == reflect.TypeOf((map[core.IaCValue]string)(nil)) {
				buf.WriteString("[")
				buf.WriteString(output)
				buf.WriteString("]")
			} else {
				buf.WriteString(output)
			}
			buf.WriteRune(':')
			output, err = tc.resolveStructInput(resourceVal, childVal.MapIndex(key), false, appliedOutputs)
			if err != nil {
				return output, err
			}
			buf.WriteString(output)
			if i < (mapLen - 1) {
				buf.WriteRune(',')
			}
		}
		buf.WriteRune('}')
		return buf.String(), nil
	case reflect.Interface:
		// This happens when the value is inside a map, slice, or array. Basically, the reflected type is interface{},
		// instead of being the actual type. So, we basically pull the item out of the collection, and then reflect on
		// it directly.
		underlyingVal := childVal.Interface()
		return tc.resolveStructInput(resourceVal, reflect.ValueOf(underlyingVal), false, appliedOutputs)
	}
	return "", nil
}

// handleIaCValue determines how to retrieve values from a resource given a specific value identifier.
func (tc TemplatesCompiler) handleIaCValue(v core.IaCValue, appliedOutputs *[]AppliedOutput, resourceVal *reflect.Value) (string, error) {
	resource := tc.resourceGraph.GetResource(v.ResourceId)
	property := v.Property

	if resource == nil {
		output, err := tc.resolveStructInput(nil, reflect.ValueOf(property), false, appliedOutputs)
		if err != nil {
			return output, err
		}
		return output, nil
	} else if _, ok := resource.(*resources.AvailabilityZones); ok {
		return fmt.Sprintf("%s.names[%s]", tc.getVarName(resource), property), nil
	}
	switch property {
	case string(core.SECRET_NAME):
		secret := resource.(*resources.Secret)
		return quoteTsString(secret.Name, true), nil
	case string(core.BUCKET_NAME):
		return fmt.Sprintf("%s.bucket", tc.getVarName(resource)), nil
	case string(core.KV_DYNAMODB_TABLE_NAME):
		return fmt.Sprintf("%s.name", tc.getVarName(resource)), nil
	case resources.BUCKET_REGIONAL_DOMAIN_NAME_IAC_VALUE:
		return fmt.Sprintf("%s.bucketRegionalDomainName", tc.getVarName(resource)), nil
	case resources.IAM_ARN_IAC_VALUE:
		return fmt.Sprintf("%s.iamArn", tc.getVarName(resource)), nil
	case resources.CLOUDFRONT_ACCESS_IDENTITY_PATH_IAC_VALUE:
		return fmt.Sprintf("%s.cloudfrontAccessIdentityPath", tc.getVarName(resource)), nil
	case resources.ARN_IAC_VALUE:
		return fmt.Sprintf("%s.arn", tc.getVarName(resource)), nil
	case resources.NAME_IAC_VALUE:
		return fmt.Sprintf("%s.name", tc.getVarName(resource)), nil
	case resources.ID_IAC_VALUE:
		return fmt.Sprintf("%s.id", tc.getVarName(resource)), nil
	case resources.ALL_BUCKET_DIRECTORY_IAC_VALUE:
		return fmt.Sprintf("pulumi.interpolate`${%s.arn}/*`", tc.getVarName(resource)), nil
	case resources.DYNAMODB_TABLE_BACKUP_IAC_VALUE,
		resources.DYNAMODB_TABLE_INDEX_IAC_VALUE,
		resources.DYNAMODB_TABLE_EXPORT_IAC_VALUE,
		resources.DYNAMODB_TABLE_STREAM_IAC_VALUE:
		prop := strings.Split(property, "__")[1]
		return fmt.Sprintf("pulumi.interpolate`${%s.arn}/%s/*`", tc.getVarName(resource), prop), nil
	case resources.LAMBDA_INTEGRATION_URI_IAC_VALUE:
		return fmt.Sprintf("%s.invokeArn", tc.getVarName(resource)), nil
	case core.ALL_RESOURCES_IAC_VALUE:
		return "*", nil
	case resources.API_GATEWAY_EXECUTION_CHILD_RESOURCES_IAC_VALUE:
		return fmt.Sprintf("pulumi.interpolate`${%s.executionArn}/*`", tc.getVarName(resource)), nil

	case string(core.HOST):
		switch resource.(type) {
		case *resources.ElasticacheCluster:
			return fmt.Sprintf("%s.cacheNodes[0].address", tc.getVarName(resource)), nil
		default:
			return "", errors.Errorf("unsupported resource type %T for '%s'", resource, property)
		}
	case string(core.PORT):
		switch resource.(type) {
		case *resources.ElasticacheCluster:
			return fmt.Sprintf("%s.cacheNodes[0].port.apply(port => port.toString())", tc.getVarName(resource)), nil
		default:
			return "", errors.Errorf("unsupported resource type %T for '%s'", resource, property)
		}
	case string(core.CONNECTION_STRING):
		switch res := resource.(type) {
		case *resources.RdsProxy:
			downResources := tc.resourceGraph.GetUpstreamDependencies(res)
			var instance *resources.RdsInstance
			for _, resource := range downResources {
				if rdsProxyTargetGroup, ok := resource.Source.(*resources.RdsProxyTargetGroup); ok {
					instance = rdsProxyTargetGroup.RdsInstance
				}
			}
			if instance == nil {
				return "", errors.Errorf("Rds Proxy, %s, must have an associated instance", resource.Id())
			}

			fetchUsername := fmt.Sprintf(`fs.readFileSync('%s', 'utf-8').split("\n")[1].split('"')[3]`, instance.CredentialsPath)
			fetchPassword := fmt.Sprintf(`fs.readFileSync('%s', 'utf-8').split("\n")[2].split('"')[3]`, instance.CredentialsPath)
			return fmt.Sprintf("pulumi.interpolate`postgresql://${%s}:${%s}@${%s.endpoint}:5432/%s`", fetchUsername, fetchPassword,
				tc.getVarName(resource), instance.DatabaseName), nil
		default:
			return "", errors.Errorf("unsupported resource type %T for '%s'", resource, property)
		}

	case resources.OIDC_SUB_IAC_VALUE:
		varName := "cluster_oidc_url"
		*appliedOutputs = append(*appliedOutputs, AppliedOutput{
			appliedName: fmt.Sprintf("%s.url", tc.getVarName(resource)),
			varName:     varName,
		})
		return fmt.Sprintf("`${%s}:sub`", varName), nil
	case resources.OIDC_AUD_IAC_VALUE:
		varName := "cluster_oidc_url"
		*appliedOutputs = append(*appliedOutputs, AppliedOutput{
			appliedName: fmt.Sprintf("%s.url", tc.getVarName(resource)),
			varName:     varName,
		})
		return fmt.Sprintf("`${%s}:aud`", varName), nil
	case resources.CLUSTER_CA_DATA_IAC_VALUE:
		return fmt.Sprintf("%s.certificateAuthorities[0].data", tc.getVarName(resource)), nil
	case resources.CLUSTER_ENDPOINT_IAC_VALUE:
		return fmt.Sprintf("%s.endpoint", tc.getVarName(resource)), nil
	case resources.CLUSTER_PROVIDER_IAC_VALUE:
		if kcfg, ok := resource.(*resources.EksCluster); ok {
			p := &KubernetesProvider{Name: fmt.Sprintf("%s-provider", kcfg.Name)}
			return tc.getVarNameByResourceId(p.Id()), nil
		}
	case resources.CLUSTER_SECURITY_GROUP_ID_IAC_VALUE:
		return fmt.Sprintf("%s.vpcConfig.clusterSecurityGroupId", tc.getVarName(resource)), nil
	case resources.STAGE_INVOKE_URL_IAC_VALUE:
		return fmt.Sprintf("%s.invokeUrl.apply((d) => d.split('//')[1].split('/')[0])", tc.getVarName(resource)), nil
	case resources.ECR_IMAGE_NAME_IAC_VALUE:
		return fmt.Sprintf(`%s.imageName`, tc.getVarName(resource)), nil
	case resources.NLB_INTEGRATION_URI_IAC_VALUE:
		integration, ok := resourceVal.Interface().(resources.ApiIntegration)
		if !ok {
			return "", errors.Errorf("Unable to handle iac value for %s on type %s", resources.NLB_INTEGRATION_URI_IAC_VALUE, resourceVal.Type().Name())
		}
		return fmt.Sprintf("pulumi.interpolate`http://${%s.dnsName}%s`", tc.getVarName(resource), strings.ReplaceAll(integration.Route, "+", "")), nil
	case resources.RDS_CONNECTION_ARN_IAC_VALUE:
		switch res := resource.(type) {
		case *resources.RdsInstance:
			accountId := resources.NewAccountId()
			region := resources.NewRegion()
			fetchUsername := fmt.Sprintf(`fs.readFileSync('%s', 'utf-8').split("\n")[1].split('"')[3]`, res.CredentialsPath)
			return fmt.Sprintf("pulumi.interpolate`arn:aws:rds-db:${%s.name}:${%s.accountId}:dbuser:${%s.resourceId}/${%s}`", tc.getVarName(region), tc.getVarName(accountId), tc.getVarName(res), fetchUsername), nil
		default:
			return "", errors.Errorf("unsupported resource type %T for '%s'", resource, property)
		}
	case resources.CIDR_BLOCK_IAC_VALUE:
		return fmt.Sprintf(`%s.cidrBlock`, tc.getVarName(resource)), nil
	case resources.AWS_OBSERVABILITY_CONFIG_MAP_REGION_IAC_VALUE:
		region := resources.NewRegion()
		return fmt.Sprintf(`pulumi.all([obj.data["output.conf"], %s.name, %s.name]).apply(([obj, regionName, clusterName]) => obj.replace("region-code",regionName).replace("my-logs","/fargate/" +clusterName))`,
			tc.getVarName(region), tc.getVarName(resource)), nil
	case resources.NODE_GROUP_NAME_IAC_VALUE:
		return fmt.Sprintf(`%s.nodeGroupName`, tc.getVarName(resource)), nil
	case resources.API_STAGE_PATH_VALUE:
		return fmt.Sprintf("pulumi.interpolate`/${%s.stageName}`", tc.getVarName(resource)), nil
	case resources.TARGET_GROUP_ARN_IAC_VALUE:
		return fmt.Sprintf("%s.targetGroupArn", tc.getVarName(resource)), nil

	}

	return "", errors.Errorf("unsupported IaC Value Property %T.%s", resource, property)
}

func (tc TemplatesCompiler) handleSingleIaCValue(v core.IaCValue) (string, error) {
	return tc.handleIaCValue(v, nil, nil)
}

// getVarName gets a unique but nice-looking variable for the given item.
//
// It does this by first calculating an ideal variable name, which is a camel-cased ${structName}${Id}. For example, if
// you had an object CoolResource{id: "foo-bar"}, the ideal variable name is coolResourceFooBar.
//
// If that ideal variable name hasn't been used yet, this function returns it. If it has been used, we append `_${i}` to
// it, where ${i} is the lowest positive integer that would give us a new, unique variable name. This isn't expected
// to happen often, if at all, since ids are globally unique.
func (tc TemplatesCompiler) getVarName(v core.Resource) string {
	return tc.getVarNameByResourceId(v.Id())
}

func (tc TemplatesCompiler) getVarNameByResourceId(id core.ResourceId) string {
	if name, alreadyResolved := tc.resourceVarNamesById[id]; alreadyResolved {
		return name
	}
	// Generate something like "lambdaFoo", where Lambda is the type of the resource and "foo" is the id
	// Omit the provider for shorter, easier names. For the most part there will only be 1 per file.
	desiredName := lowercaseFirst(toUpperCamel(fmt.Sprintf("%s:%s:%s", id.Namespace, id.Type, id.Name)))
	resolvedName := desiredName
	for i := 0; ; i++ {
		_, varNameTaken := tc.resourceVarNames[resolvedName]
		if varNameTaken {
			if i == 0 {
				resolvedName = lowercaseFirst(toUpperCamel(id.String()))
			} else {
				resolvedName = fmt.Sprintf("%s_%d", desiredName, i)
			}
		} else {
			break
		}
	}
	tc.resourceVarNames[resolvedName] = struct{}{}
	tc.resourceVarNamesById[id] = resolvedName
	return resolvedName
}

// parseVal parses the supplied value for nested tempaltes
func (tc TemplatesCompiler) parseVal(val reflect.Value) (string, error) {
	return tc.resolveStructInput(tc.ctx.rootVal, val, tc.ctx.useDoubleQuotes, tc.ctx.appliedOutputs)
}

func (tp templatesProvider) getTemplate(v core.Resource) (ResourceCreationTemplate, error) {
	return tp.getTemplateForType(structName(v))
}

func (tp templatesProvider) getTemplateForType(typeName string) (ResourceCreationTemplate, error) {
	existing, ok := tp.resourceTemplatesByStructName[typeName]
	if ok {
		return existing, nil
	}
	templateName := camelToSnake(typeName)
	contents, err := fs.ReadFile(tp.templates, templateName+`/factory.ts`)
	if err != nil {
		return ResourceCreationTemplate{}, errors.Wrapf(err, "could not find template for %s", typeName)
	}
	template := ParseResourceCreationTemplate(typeName, contents)
	tp.resourceTemplatesByStructName[typeName] = template
	return template, nil
}

func (tp templatesProvider) getNestedTemplate(templatePath string, tc TemplatesCompiler) (*template.Template, error) {
	templateFilePaths := []string{
		templatePath + ".ts.tmpl",
		templatePath + ".ts",
	}

	existing, ok := tp.childTemplatesByPath[templatePath]
	if ok {
		return existing, nil
	}

	var contents []byte
	var merr multierr.Error
	var err error
	for _, tfPath := range templateFilePaths {
		contents, err = fs.ReadFile(tp.templates, tfPath)
		if err == nil {
			break
		} else {
			merr.Append(err)
		}
	}
	// If we dont have any contents we dont have a nested template for the resource, so fall back to the document route
	if len(contents) == 0 {
		return nil, nil
	}

	tmpl, err := template.New(templatePath).Funcs(template.FuncMap{
		"parseVal": tc.parseVal,
	}).Parse(string(contents))
	if err != nil {
		return nil, errors.Wrapf(err, `while writing template for %s`, templatePath)
	}
	tp.childTemplatesByPath[templatePath] = tmpl
	return tmpl, nil
}

func (tc TemplatesCompiler) GetPackageJSON(v core.Resource) (*javascript.NodePackageJson, error) {
	typeName := structName(v)
	templateName := camelToSnake(typeName)
	templateFilePath := templateName + `/package.json`
	contents, err := fs.ReadFile(tc.templates, templateFilePath)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}
	var packageContent *javascript.NodePackageJson
	err = json.NewDecoder(bytes.NewReader(contents)).Decode(&packageContent)
	if err != nil {
		return packageContent, err
	}
	return packageContent, nil
}

// renderGlueVars renders additional variables associated with a given resource that do not represent specific cloud resources
func (tc TemplatesCompiler) renderGlueVars(out io.Writer, resource core.Resource) error {
	var errs multierr.Error
	switch resource := resource.(type) {
	case *resources.EksCluster:
		errs.Append(tc.renderKubernetesProvider(out, resource))
		errs.Append(tc.addIngressRuleToCluster(out, resource))
	case *resources.RouteTable:
		errs.Append(tc.associateRouteTable(out, resource))
	case *resources.TargetGroup:
		errs.Append(tc.attachToTargetGroup(out, resource))
	}
	return errs.ErrOrNil()
}

func (tc TemplatesCompiler) addIngressRuleToCluster(out io.Writer, cluster *resources.EksCluster) error {
	var errs multierr.Error

	_, err := out.Write([]byte("\n\n"))
	errs.Append(err)

	cidrBlocks := make([]core.IaCValue, len(cluster.Subnets))
	for i, subnet := range cluster.Subnets {
		cidrBlocks[i] = core.IaCValue{
			ResourceId: subnet.Id(),
			Property:   resources.CIDR_BLOCK_IAC_VALUE,
		}
	}

	sgRule := &SecurityGroupRule{
		ConstructRefs: cluster.ConstructRefs,
		Name:          fmt.Sprintf("%s-ingress", cluster.Name),
		Description:   "Allows access to cluster from the VPCs private and public subnets",
		FromPort:      0,
		ToPort:        0,
		Protocol:      "-1",
		CidrBlocks:    cidrBlocks,
		SecurityGroupId: core.IaCValue{
			ResourceId: cluster.Id(),
			Property:   resources.CLUSTER_SECURITY_GROUP_ID_IAC_VALUE,
		},
		Type: "ingress",
	}
	errs.Append(tc.renderResource(out, sgRule))
	return errs.ErrOrNil()
}

func (tc TemplatesCompiler) renderKubernetesProvider(out io.Writer, cluster *resources.EksCluster) error {
	var errs multierr.Error

	_, err := out.Write([]byte("\n\n"))
	errs.Append(err)
	errs.Append(tc.renderResource(out, cluster.Kubeconfig))

	provider := &KubernetesProvider{
		Name:          fmt.Sprintf("%s-provider", cluster.Name),
		ConstructRefs: cluster.ConstructRefs,
		KubeConfig:    cluster.Kubeconfig,
	}
	_, err = out.Write([]byte("\n\n"))
	errs.Append(err)
	errs.Append(tc.renderResource(out, provider))
	return errs.ErrOrNil()
}

func (tc TemplatesCompiler) associateRouteTable(out io.Writer, rt *resources.RouteTable) error {
	var errs multierr.Error

	_, err := out.Write([]byte("\n\n"))
	errs.Append(err)

	for _, resource := range tc.resourceGraph.GetDownstreamResources(rt) {
		if subnet, ok := resource.(*resources.Subnet); ok {

			association := &RouteTableAssociation{
				Name:       subnet.Name,
				Subnet:     subnet,
				RouteTable: rt,
			}
			errs.Append(tc.renderResource(out, association))

			_, err := out.Write([]byte("\n\n"))
			errs.Append(err)
		}
	}

	return errs.ErrOrNil()
}

func (tc TemplatesCompiler) attachToTargetGroup(out io.Writer, tg *resources.TargetGroup) error {
	var errs multierr.Error

	_, err := out.Write([]byte("\n\n"))
	errs.Append(err)

	for _, target := range tg.Targets {

		attachment := &TargetGroupAttachment{
			Name:           target.Id.ResourceId.String(),
			Port:           target.Port,
			TargetGroupArn: core.IaCValue{ResourceId: tg.Id(), Property: resources.ARN_IAC_VALUE},
			TargetId:       core.IaCValue{ResourceId: target.Id.ResourceId, Property: resources.ID_IAC_VALUE},
		}
		errs.Append(tc.renderResource(out, attachment))

		_, err := out.Write([]byte("\n\n"))
		errs.Append(err)

	}

	return errs.ErrOrNil()
}

func (tc TemplatesCompiler) renderResourceImport(out io.Writer, source core.Resource, imp *imports.Imported, tmpl ResourceCreationTemplate) error {
	// TODO delegate to a factory 'import' function on the template or something to allow for customisation
	varName := tc.getVarName(source)
	_, err := fmt.Fprintf(out, `const %s = %s.get("%s", "%s")`, varName, tmpl.OutputType, source.Id().Name, imp.ID)
	return err
}
