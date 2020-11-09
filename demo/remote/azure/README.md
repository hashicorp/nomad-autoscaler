### Microsoft Azure
Some of the steps below will require that you have the
[Azure CLI installed](https://docs.microsoft.com/en-us/cli/azure/install-azure-cli)
in your machine.

### Environment setup
The first step is to login using the CLI:

```shellsession
$ az login
[
  {
    "cloudName": "AzureCloud",
    "id": "<SUBSCRIPTION_ID>",
    "isDefault": true,
    "name": "Free Trial",
    "state": "Enabled",
    "tenantId": "<TENANT_ID>",
    "user": {
      "name": "user@example.com",
      "type": "user"
    }
  }
```

Take a note of the values for `<SUBSCRIPTION_ID>` and `<TENANT_ID>` end export
them as environment variables:

```shellsession
$ export ARM_SUBSCRIPTION_ID=<SUBSCRIPTION_ID>
$ export ARM_TENANT_ID=<TENANT_ID>
```

Next, create an application ID and password that will be used to run Terraform:

```shellsession
$ az ad sp create-for-rbac --role="Owner" --scopes="/subscriptions/$ARM_SUBSCRIPTION_ID"
{
  "appId": "<CLIENT_ID>",
  "displayName": "azure-cli-...",
  "name": "http://azure-cli-...",
  "password": "<CLIENT_SECRET>",
  "tenant": "<TENANT_ID>"
}
```

Export the values for `<CLIENT_ID>` and `<CLIENT_SECRET>` as environment
variables as well:

```shellsession
$ export ARM_CLIENT_ID=<CLIENT_ID>
$ export ARM_CLIENT_SECRET=<CLIENT_SECRET>
```

# Running Terraform
Navigate to the Terraform control folder and execute the Terraform
configuration to deploy the demo infrastructure:

```shellsession
$ cd ./terraform/control
$ terraform init
$ terraform plan
$ terraform apply --auto-approve
```

Once the Terraform apply finishes, a number of useful pieces of information
should be output to your console. These include URLs to deployed resources as
well as a semi-populated Nomad Autoscaler config.

```
ip_addresses =
Server IPs:
 * instance server-1 - Public: 52.188.111.20, Private: 10.0.2.4


To connect, add your private key and SSH into any client or server with
`ssh -i azure-hashistack.pem -o IdentitiesOnly=yes ubuntu@PUBLIC_IP`.
You can test the integrity of the cluster by running:

  $ consul members
  $ nomad server members
  $ nomad node status

The Nomad UI can be accessed at http://52.249.185.10:4646/ui
The Consul UI can be accessed at http://52.249.185.10:8500/ui
Grafana dashbaord can be accessed at http://52.249.187.190:3000/d/AQphTqmMk/demo?orgId=1&refresh=5s
Traefik can be accessed at http://52.249.187.190:8081
Prometheus can be accessed at http://52.249.187.190:9090
Webapp can be accessed at http://52.249.187.190:80

CLI environment variables:
export NOMAD_CLIENT_DNS=http://52.249.187.190
export NOMAD_ADDR=http://52.249.185.10:4646
```

You can visit the URLs and explore what has been created. This will include
registration of a number of Nomad jobs which provide metrics and dashboarding
as well as a demo application and routing provided by Traefik. It may take a
few seconds for all the applications to start, so if any of the URLs doesn't
load the first time, wait a little bit and give it a try again.

Please also copy the export commands and run these in the terminal where the
rest of the demo will be run.

The application is pre-configured with a scaling policy, you can view this by
opening the job file or calling the Nomad API. The application scales based on
the average number of active connections, and we are targeting an average of 10
per instance of our app.
```
curl $NOMAD_ADDR/v1/scaling/policies
```

## Run the Autoscaler
The Autoscaler is not triggered automatically. This provides the opportunity to
look through the jobfile to understand it better before deploying. The most
important parts of the `azure_autoscaler.nomad` file are the template sections.
The first defines our agent config where we configure the `prometheus`,
`azure-vmss` and `target-value` plugins. The second is where we define our
cluster scaling policy and write this to a local directory for reading. Once
you have an understanding of the job file, submit it to the Nomad cluster
ensuring the `NOMAD_ADDR` env var has been exported.

```shellsession
$ nomad run azure_autoscaler.nomad
```

If you wish, in another terminal window you can export the `NOMAD_ADDR` env var
and then follow the Nomad Autoscaler logs.

```
$ nomad logs -stderr -f <alloc-id>
```

You can now return to the [demo instrunctions](../README.md#the-demo).
