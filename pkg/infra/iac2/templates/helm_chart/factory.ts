import * as pulumi from '@pulumi/pulumi'
import * as pulumi_k8s from '@pulumi/kubernetes'

interface Args {
    Name: string
    Directory?: string
    Chart?: string
    dependsOn: pulumi.Input<pulumi.Input<pulumi.Resource>[]> | pulumi.Input<pulumi.Resource>
}

// This template is not actually rendered in index.ts right now, but is required for unit tests.

// noinspection JSUnusedLocalSymbols
function create(args: Args): pulumi_k8s.helm.v3.Chart {
    return new pulumi_k8s.helm.v3.Chart(
        args.Name,
        {
            //TMPL {{- if .Chart.Raw }}
            chart: args.Chart,
            //TMPL {{- end }}
            //TMPL {{- if not .Chart.Raw }}
            path: `./charts/${args.Directory}`,
            //TMPL {{- end }}
            //TMPL {{- if .Values.Raw }}
        },
        {
            provider: undefined,
            //TMPL {{- if .dependsOn.Raw }}
            dependsOn: args.dependsOn,
            //TMPL {{- end }}
        }
    )
}
