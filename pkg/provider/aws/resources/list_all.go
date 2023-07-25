package resources

import (
	"github.com/klothoplatform/klotho/pkg/core"
)

func ListAll() []core.Resource {
	return []core.Resource{
		&AccountId{},
		&ApiDeployment{},
		&AMI{},
		&ApiIntegration{},
		&ApiMethod{},
		&ApiResource{},
		&ApiStage{},
		&AvailabilityZones{},
		&CloudfrontDistribution{},
		&DynamodbTable{},
		&EcrImage{},
		&EcrRepository{},
		&Ec2Instance{},
		&EcsCluster{},
		&EcsService{},
		&EcsTaskDefinition{},
		&EksAddon{},
		&EksCluster{},
		&EksFargateProfile{},
		&EksNodeGroup{},
		&ElasticIp{},
		&ElasticacheCluster{},
		&ElasticacheSubnetgroup{},
		&IamPolicy{},
		&IamRole{},
		&InstanceProfile{},
		&InternetGateway{},
		&KinesisStreamConsumer{},
		&KinesisStream{},
		&KmsAlias{},
		&KmsKey{},
		&KmsReplicaKey{},
		&LambdaFunction{},
		&LambdaPermission{},
		&Listener{},
		&LoadBalancer{},
		&LogGroup{},
		&NatGateway{},
		&OpenIdConnectProvider{},
		&OriginAccessIdentity{},
		&PrivateDnsNamespace{},
		&RdsInstance{},
		&RdsProxyTargetGroup{},
		&RdsProxy{},
		&RdsSubnetGroup{},
		&Region{},
		&RestApi{},
		&RolePolicyAttachment{},
		&RouteTable{},
		&Route53HealthCheck{},
		&Route53HostedZone{},
		&Route53Record{},
		&S3BucketPolicy{},
		&S3Bucket{},
		&S3Object{},
		&SecretVersion{},
		&Secret{},
		&SecurityGroup{},
		&SnsTopic{},
		&SnsSubscription{},
		&Subnet{Type: PrivateSubnet},
		&Subnet{Type: PublicSubnet},
		&SqsQueuePolicy{},
		&SqsQueue{},
		&TargetGroup{},
		&VpcEndpoint{},
		&VpcLink{},
		&Vpc{},
	}
}
