import * as aws from '@pulumi/aws'

interface Args {
    Name: string
    Engine: string
    CloudwatchGroup: aws.cloudwatch.LogGroup
    SubnetGroup: aws.elasticache.SubnetGroup
    SecurityGroups: aws.ec2.SecurityGroup[]
}

function create(args: Args): aws.elasticache.Cluster {
    return new aws.elasticache.Cluster(args.Name, {
        engine: 'redis',
        clusterId: args.Engine,
        logDeliveryConfigurations: [
            {
                destination: args.CloudwatchGroup.name,
                destinationType: 'cloudwatch-logs',
                logFormat: 'text',
                logType: 'slow-log',
            },
            {
                destination: args.CloudwatchGroup.name,
                destinationType: 'cloudwatch-logs',
                logFormat: 'json',
                logType: 'engine-log',
            },
        ],
        subnetGroupName: args.SubnetGroup.name,
        securityGroupIds: args.SecurityGroups.map((sg) => sg.id),
    })
}
