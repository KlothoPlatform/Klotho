package resources

import (
	"crypto/rand"
	"fmt"
	"math/big"
	"strings"

	"github.com/klothoplatform/klotho/pkg/config"
	"github.com/klothoplatform/klotho/pkg/core"
	"github.com/klothoplatform/klotho/pkg/sanitization/aws"
)

var (
	RDS_ASSUME_ROLE_POLICY = &PolicyDocument{
		Version: VERSION,
		Statement: []StatementEntry{
			{
				Effect: "Allow",
				Principal: &Principal{
					Service: "rds.amazonaws.com",
				},
				Action: []string{"sts:AssumeRole"},
			},
		},
	}
	rdsInstanceSanitizer = aws.RdsInstanceSanitizer
	rdsSubnetSanitizer   = aws.RdsSubnetGroupSanitizer
	rdsProxySanitizer    = aws.RdsProxySanitizer
)

const (
	RDS_INSTANCE_TYPE      = "rds_instance"
	RDS_SUBNET_GROUP_TYPE  = "rds_subnet_group"
	RDS_PROXY_TYPE         = "rds_proxy"
	RDS_PROXY_TARGET_GROUP = "rds_proxy_target_group"

	RDS_CONNECTION_ARN_IAC_VALUE = "rds_connection_arn"
)

type (
	// RdsInstance represents an AWS RDS db instance
	RdsInstance struct {
		Name                             string
		ConstructsRef                    []core.AnnotationKey
		SubnetGroup                      *RdsSubnetGroup
		SecurityGroups                   []*SecurityGroup
		DatabaseName                     string
		IamDatabaseAuthenticationEnabled bool
		Username                         string
		Password                         string
		Engine                           string
		EngineVersion                    string
		InstanceClass                    string
		SkipFinalSnapshot                bool
		AllocatedStorage                 int
		CredentialsFile                  core.File
		CredentialsPath                  string
	}

	// RdsSubnetGroup represents an AWS RDS subnet group
	RdsSubnetGroup struct {
		Name          string
		ConstructsRef []core.AnnotationKey
		Subnets       []*Subnet
		Tags          map[string]string
	}

	// RdsProxy represents an AWS RDS proxy instance
	RdsProxy struct {
		Name              string
		ConstructsRef     []core.AnnotationKey
		DebugLogging      bool
		EngineFamily      string
		IdleClientTimeout int
		RequireTls        bool
		Role              *IamRole
		SecurityGroups    []*SecurityGroup
		Subnets           []*Subnet
		Auths             []*ProxyAuth `render:"document"`
	}

	// ProxyAuth represents an authorization configuration for an AWS RDS Proxy instance
	ProxyAuth struct {
		AuthScheme string
		IamAuth    string
		SecretArn  core.IaCValue
	}

	// RdsProxyTargetGroup represents an AWS RDS proxy target group
	RdsProxyTargetGroup struct {
		Name                            string
		ConstructsRef                   []core.AnnotationKey
		RdsInstance                     *RdsInstance
		RdsProxy                        *RdsProxy
		TargetGroupName                 string
		ConnectionPoolConfigurationInfo *ConnectionPoolConfigurationInfo `render:"document"`
	}

	// ConnectionPoolConfigurationInfo represents the connection pool configuration within a RDS proxy target group
	ConnectionPoolConfigurationInfo struct {
		ConnectionBorrowTimeout   int
		InitQuery                 string
		MaxConnectionsPercent     int
		MaxIdleConnectionsPercent int
		SessionPinningFilters     []string
	}
)

type RdsInstanceCreateParams struct {
	AppName string
	Refs    []core.AnnotationKey
	Name    string
}

// Create takes in an all necessary parameters to generate the RdsInstance name and ensure that the RdsInstance is correlated to the constructs which required its creation.
//
// This method will also create dependent resources which are necessary for functionality. Those resources are:
//   - RDS Subnet Group
//   - Security Groups
func (instance *RdsInstance) Create(dag *core.ResourceGraph, params RdsInstanceCreateParams) error {

	name := rdsInstanceSanitizer.Apply(fmt.Sprintf("%s-%s", params.AppName, params.Name))
	instance.Name = name
	instance.ConstructsRef = params.Refs

	existingInstance := dag.GetResource(instance.Id())
	if existingInstance != nil {
		return fmt.Errorf("RdsInstance with name %s already exists", name)
	}

	instance.SecurityGroups = make([]*SecurityGroup, 1)
	subParams := map[string]any{
		"SecurityGroups": []SecurityGroupCreateParams{
			{
				AppName: params.AppName,
				Refs:    params.Refs,
			},
		},
		"SubnetGroup": params,
	}
	err := dag.CreateDependencies(instance, subParams)
	return err
}

type RdsInstanceConfigureParams struct {
	DatabaseName      string
	Username          string
	Password          string
	Engine            string
	EngineVersion     string
	InstanceClass     string
	SkipFinalSnapshot bool
	AllocatedStorage  int
}

// Configure sets the intristic characteristics of a vpc based on parameters passed in
func (instance *RdsInstance) Configure(params RdsInstanceConfigureParams) error {
	instance.IamDatabaseAuthenticationEnabled = true
	instance.SkipFinalSnapshot = true
	instance.DatabaseName = params.DatabaseName
	instance.Username = generateUsername()
	instance.Password = generatePassword()

	instance.Engine = "postgres"
	instance.EngineVersion = "13.7"
	instance.InstanceClass = "db.t4g.micro"
	instance.AllocatedStorage = 20
	credsBytes := []byte(fmt.Sprintf("{\n\"username\": \"%s\",\n\"password\": \"%s\"\n}", instance.Username, instance.Password))
	credsPath := fmt.Sprintf("secrets/%s", instance.Name)
	instance.CredentialsFile = &core.RawFile{
		FPath:   credsPath,
		Content: credsBytes,
	}
	instance.CredentialsPath = credsPath

	return nil
}

type RdsSubnetGroupCreateParams struct {
	AppName string
	Name    string
	Refs    []core.AnnotationKey
}

// Create takes in an all necessary parameters to generate the RdsSubnetGroup name and ensure that the RdsSubnetGroup is correlated to the constructs which required its creation.
//
// This method will also create dependent resources which are necessary for functionality. Those resources are:
//   - Subnets
func (subnetGroup *RdsSubnetGroup) Create(dag *core.ResourceGraph, params RdsSubnetGroupCreateParams) error {
	subnetGroup.Name = rdsSubnetSanitizer.Apply(fmt.Sprintf("%s-%s", params.AppName, params.Name))
	subnetGroup.ConstructsRef = params.Refs

	existingSubnetGroup := dag.GetResource(subnetGroup.Id())
	if existingSubnetGroup != nil {
		graphSubnetGroup := existingSubnetGroup.(*RdsSubnetGroup)
		graphSubnetGroup.ConstructsRef = core.DedupeAnnotationKeys(append(graphSubnetGroup.KlothoConstructRef(), params.Refs...))
	} else {
		subnetGroup.Subnets = make([]*Subnet, 2)
		err := dag.CreateDependencies(subnetGroup, map[string]any{
			"Subnets": []SubnetCreateParams{
				{
					AppName: params.AppName,
					Refs:    params.Refs,
					AZ:      "0",
					Type:    PrivateSubnet,
				},
				{
					AppName: params.AppName,
					Refs:    params.Refs,
					AZ:      "1",
					Type:    PrivateSubnet,
				},
			},
		})
		if err != nil {
			return err
		}
	}
	return nil
}

type RdsProxyCreateParams struct {
	AppName string
	Name    string
	Refs    []core.AnnotationKey
}

// Create takes in an all necessary parameters to generate the RdsProxy name and ensure that the RdsProxy is correlated to the constructs which required its creation.
//
// This method will also create dependent resources which are necessary for functionality. Those resources are:
//   - Subnets
//   - SecurityGroups
func (proxy *RdsProxy) Create(dag *core.ResourceGraph, params RdsProxyCreateParams) error {
	proxy.Name = rdsSubnetSanitizer.Apply(fmt.Sprintf("%s-%s", params.AppName, params.Name))
	proxy.ConstructsRef = params.Refs

	existingProxy := dag.GetResource(proxy.Id())
	if existingProxy != nil {
		graphProxy := existingProxy.(*RdsProxy)
		graphProxy.ConstructsRef = core.DedupeAnnotationKeys(append(graphProxy.KlothoConstructRef(), params.Refs...))
	} else {
		proxy.Subnets = make([]*Subnet, 2)
		proxy.SecurityGroups = make([]*SecurityGroup, 1)
		err := dag.CreateDependencies(proxy, map[string]any{
			"Role": RoleCreateParams{
				AppName: params.AppName,
				Name:    fmt.Sprintf("%s-ProxyRole", params.Name),
				Refs:    proxy.ConstructsRef,
			},
			"SecurityGroups": []SecurityGroupCreateParams{
				{
					AppName: params.AppName,
					Refs:    params.Refs,
				},
			},
			"Subnets": []SubnetCreateParams{
				{
					AppName: params.AppName,
					Refs:    params.Refs,
					AZ:      "0",
					Type:    PrivateSubnet,
				},
				{
					AppName: params.AppName,
					Refs:    params.Refs,
					AZ:      "1",
					Type:    PrivateSubnet,
				},
			},
		})
		if err != nil {
			return err
		}
	}
	return nil
}

type RdsProxyConfigureParams struct {
	EngineFamily      string
	DebugLogging      bool
	IdleClientTimeout int
	RequireTls        bool
}

// Configure sets the intristic characteristics of a vpc based on parameters passed in
func (proxy *RdsProxy) Configure(params RdsProxyConfigureParams) error {
	proxy.DebugLogging = false
	proxy.EngineFamily = "POSTGRESQL"
	proxy.IdleClientTimeout = 1800
	proxy.RequireTls = false
	return nil
}

type RdsProxyTargetGroupCreateParams struct {
	AppName string
	Name    string
	Refs    []core.AnnotationKey
}

// Create takes in an all necessary parameters to generate the RdsProxyTargetGroup name and ensure that the RdsProxyTargetGroup is correlated to the constructs which required its creation.
func (tg *RdsProxyTargetGroup) Create(dag *core.ResourceGraph, params RdsProxyTargetGroupCreateParams) error {

	tg.Name = rdsProxySanitizer.Apply(fmt.Sprintf("%s-%s", params.AppName, params.Name))
	tg.ConstructsRef = params.Refs
	existingTG := dag.GetResource(tg.Id())
	if existingTG != nil {
		graphTG := existingTG.(*RdsProxyTargetGroup)
		graphTG.ConstructsRef = core.DedupeAnnotationKeys(append(graphTG.KlothoConstructRef(), params.Refs...))
	} else {
		dag.AddResource(tg)
	}
	return nil
}

type RdsProxyTargetGroupConfigureParams struct {
	ConnectionPoolConfigurationInfo ConnectionPoolConfigurationInfo
}

// Configure sets the intristic characteristics of a vpc based on parameters passed in
func (targetGroup *RdsProxyTargetGroup) Configure(params RdsProxyTargetGroupConfigureParams) error {
	targetGroup.ConnectionPoolConfigurationInfo = &ConnectionPoolConfigurationInfo{
		ConnectionBorrowTimeout:   120,
		MaxConnectionsPercent:     100,
		MaxIdleConnectionsPercent: 50,
	}
	return nil
}

// CreateRdsInstance takes in an orm construct and creates the necessary resources to support creating a functional RDS Orm implementation
//
// If proxy is enabled, a corresponding proxy, secret, and remaining resources will be created.
// A username and password are generated for the rds instance and proxy credentials and are written to the compiled directory to be used within the IaC.
func CreateRdsInstance(cfg *config.Application, orm *core.Orm, proxyEnabled bool, subnets []*Subnet, securityGroups []*SecurityGroup, dag *core.ResourceGraph) (*RdsInstance, *RdsProxy, error) {

	subnetGroup := NewRdsSubnetGroup(orm, cfg.AppName, subnets)

	instance := NewRdsInstance(orm, cfg.AppName, subnetGroup, securityGroups)
	credsBytes := []byte(fmt.Sprintf("{\n\"username\": \"%s\",\n\"password\": \"%s\"\n}", instance.Username, instance.Password))
	credsPath := fmt.Sprintf("secrets/%s", orm.Id())
	instance.CredentialsFile = &core.RawFile{
		FPath:   credsPath,
		Content: credsBytes,
	}
	instance.CredentialsPath = credsPath

	var proxy *RdsProxy
	if proxyEnabled {
		role := NewIamRole(cfg.AppName, fmt.Sprintf("%s-ormsecretrole", orm.ID), []core.AnnotationKey{orm.Provenance()}, RDS_ASSUME_ROLE_POLICY)
		secret := NewSecret(orm.Provenance(), orm.Id(), cfg.AppName)

		secretVersion := NewSecretVersion(secret, credsPath)
		secretVersion.Type = "string"
		secretPolicyDoc := CreateAllowPolicyDocument([]string{"secretsmanager:GetSecretValue"}, []core.IaCValue{{Resource: secret, Property: ARN_IAC_VALUE}})
		secretPolicy := NewIamPolicy(cfg.AppName, fmt.Sprintf("%s-ormsecretpolicy", orm.ID), orm.AnnotationKey, secretPolicyDoc)
		role.ManagedPolicies = append(role.ManagedPolicies, core.IaCValue{Resource: secretPolicy, Property: ARN_IAC_VALUE})
		dag.AddDependency(secretPolicy, secret)

		proxy = NewRdsProxy(orm, cfg.AppName, securityGroups, subnets, role, secret)
		dag.AddDependency(proxy, secret)
		proxyTargetGroup := NewRdsProxyTargetGroup(orm, cfg.AppName, instance, proxy)
		dag.AddDependenciesReflect(secretVersion)
		dag.AddDependenciesReflect(proxyTargetGroup)
		dag.AddDependenciesReflect(proxy)
		dag.AddDependenciesReflect(role)
		dag.AddDependenciesReflect(secretPolicy)
	}
	dag.AddDependenciesReflect(instance)
	dag.AddDependenciesReflect(subnetGroup)
	return instance, proxy, nil
}

func (rds *RdsInstance) GetConnectionPolicyDocument() *PolicyDocument {
	return CreateAllowPolicyDocument(
		[]string{"rds-db:connect"},
		[]core.IaCValue{{Resource: rds, Property: RDS_CONNECTION_ARN_IAC_VALUE}})
}

// generateUsername generates a random username for the rds instance.
//
// The first letter of an RDS username must be a letter
func generateUsername() string {
	chars := []rune("ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789")
	length := 9
	var b strings.Builder
	b.WriteString("KLO")
	for i := 0; i < length; i++ {
		num, err := rand.Int(rand.Reader, big.NewInt(int64(len(chars))))
		if err != nil {
			panic(err.Error())
		}
		b.WriteRune(chars[num.Int64()])
	}
	return b.String()
}

// generatePassword generates a random password for the rds instance.
func generatePassword() string {
	chars := []rune("ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789")
	length := 16
	var b strings.Builder
	for i := 0; i < length; i++ {
		num, err := rand.Int(rand.Reader, big.NewInt(int64(len(chars))))
		if err != nil {
			panic(err.Error())
		}
		b.WriteRune(chars[num.Int64()])
	}
	return b.String()
}

func NewRdsInstance(orm *core.Orm, appName string, subnetGroup *RdsSubnetGroup, securityGroups []*SecurityGroup) *RdsInstance {
	return &RdsInstance{
		Name:                             rdsInstanceSanitizer.Apply(fmt.Sprintf("%s-%s", appName, orm.ID)),
		ConstructsRef:                    []core.AnnotationKey{orm.Provenance()},
		SubnetGroup:                      subnetGroup,
		SecurityGroups:                   securityGroups,
		IamDatabaseAuthenticationEnabled: true,
		DatabaseName:                     orm.ID,
		Username:                         generateUsername(),
		Password:                         generatePassword(),
		Engine:                           "postgres",
		EngineVersion:                    "13.7",
		InstanceClass:                    "db.t4g.micro",
		SkipFinalSnapshot:                true,
		AllocatedStorage:                 20,
	}
}

// KlothoConstructRef returns AnnotationKey of the klotho resource the cloud resource is correlated to
func (rds *RdsInstance) KlothoConstructRef() []core.AnnotationKey {
	return rds.ConstructsRef
}

// Id returns the id of the cloud resource
func (rds *RdsInstance) Id() core.ResourceId {
	return core.ResourceId{
		Provider: AWS_PROVIDER,
		Type:     RDS_INSTANCE_TYPE,
		Name:     rds.Name,
	}
}
func (rds *RdsInstance) GetOutputFiles() []core.File {
	return []core.File{rds.CredentialsFile}
}

func NewRdsSubnetGroup(orm *core.Orm, appName string, subnets []*Subnet) *RdsSubnetGroup {
	return &RdsSubnetGroup{
		Name:          rdsSubnetSanitizer.Apply(fmt.Sprintf("%s-%s", appName, orm.ID)),
		ConstructsRef: []core.AnnotationKey{orm.Provenance()},
		Subnets:       subnets,
	}
}

// KlothoConstructRef returns AnnotationKey of the klotho resource the cloud resource is correlated to
func (rds *RdsSubnetGroup) KlothoConstructRef() []core.AnnotationKey {
	return rds.ConstructsRef
}

// Id returns the id of the cloud resource
func (rds *RdsSubnetGroup) Id() core.ResourceId {
	return core.ResourceId{
		Provider: AWS_PROVIDER,
		Type:     RDS_SUBNET_GROUP_TYPE,
		Name:     rds.Name,
	}
}

func NewRdsProxy(orm *core.Orm, appName string, securityGroups []*SecurityGroup, subnets []*Subnet, role *IamRole, secret *Secret) *RdsProxy {
	return &RdsProxy{
		Name:              rdsProxySanitizer.Apply(fmt.Sprintf("%s-%s", appName, orm.ID)),
		ConstructsRef:     []core.AnnotationKey{orm.Provenance()},
		DebugLogging:      false,
		EngineFamily:      "POSTGRESQL",
		IdleClientTimeout: 1800,
		RequireTls:        false,
		Role:              role,
		SecurityGroups:    securityGroups,
		Subnets:           subnets,
		Auths: []*ProxyAuth{
			{
				AuthScheme: "SECRETS",
				IamAuth:    "DISABLED",
				SecretArn:  core.IaCValue{Resource: secret, Property: ARN_IAC_VALUE},
			},
		},
	}
}

// KlothoConstructRef returns AnnotationKey of the klotho resource the cloud resource is correlated to
func (rds *RdsProxy) KlothoConstructRef() []core.AnnotationKey {
	return rds.ConstructsRef
}

// Id returns the id of the cloud resource
func (rds *RdsProxy) Id() core.ResourceId {
	return core.ResourceId{
		Provider: AWS_PROVIDER,
		Type:     RDS_PROXY_TYPE,
		Name:     rds.Name,
	}
}

func NewRdsProxyTargetGroup(orm *core.Orm, appName string, instance *RdsInstance, proxy *RdsProxy) *RdsProxyTargetGroup {
	return &RdsProxyTargetGroup{
		Name:          lambdaFunctionSanitizer.Apply(fmt.Sprintf("%s-%s", appName, orm.ID)),
		ConstructsRef: []core.AnnotationKey{orm.Provenance()},
		RdsInstance:   instance,
		RdsProxy:      proxy,
		ConnectionPoolConfigurationInfo: &ConnectionPoolConfigurationInfo{
			ConnectionBorrowTimeout:   120,
			MaxConnectionsPercent:     100,
			MaxIdleConnectionsPercent: 50,
		},
		TargetGroupName: "default",
	}
}

// KlothoConstructRef returns AnnotationKey of the klotho resource the cloud resource is correlated to
func (rds *RdsProxyTargetGroup) KlothoConstructRef() []core.AnnotationKey {
	return rds.ConstructsRef
}

// Id returns the id of the cloud resource
func (rds *RdsProxyTargetGroup) Id() core.ResourceId {
	return core.ResourceId{
		Provider: AWS_PROVIDER,
		Type:     RDS_PROXY_TARGET_GROUP,
		Name:     rds.Name,
	}
}
