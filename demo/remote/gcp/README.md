# Google Cloud Platform
In order to setup some basic GCP resources for the demo, we will utilise the
[gcloud CLI tool](https://cloud.google.com/sdk/gcloud/). Please ensure this is installed before
continuing.

### Environment Setup
The first step is to authenticate with Google Cloud:

```shellsession
$ gcloud auth login
```

Next, you will need to pick which organization and billing account to use. Take a note of their
IDs, we will use them in the next step:

```shellsession
$ gcloud organizations list
DISPLAY_NAME                  ID  DIRECTORY_CUSTOMER_ID
org1               1111111111111              ZZZZZZZZZ
org2                222222222222              ZZZZZZZZZ

$ gcloud alpha billing accounts list
ACCOUNT_ID            NAME         OPEN  MASTER_ACCOUNT_ID
111111-AAAAAA-222222  Account 1    True  AAAAAA-BBBBBB-CCCCCC
BBBBBB-333333-DDDDDD  Account 2    True  AAAAAA-BBBBBB-CCCCCC
```

# Running Terraform
The Terraform setup handles creating all GCP resources as well as the compute images via
[Packer](https://www.packer.io/). In particular all resources will be placed into a newly created
project for isolation.

```shellsession
$ cd ./terraform/control
```

Make a copy of the `terraform.tfvars.sample` file and call it `terraform.tfvars`:

```shellsession
$ cp terraform.tfvars.sample terraform.tfvars
```

Fill in the IDs for the organization and billing account that were retrieved before:

```hcl
org_id          = "1111111111111"
billing_account = "BBBBBB-333333-DDDDDD"
```

We are now ready to initialize and run Terraform:

```shellsession
$ terraform init
$ terraform plan
$ terraform apply --auto-approve
```

You may see this error message:

```
Error: Error creating Network: googleapi: Error 403: Compute Engine API has not been used in project 603655938425 before or it is disabled. Enable it by visiting https://console.developers.google.com/apis/api/compute.googleapis.com/overview?project=603655938425 then retry. If you enabled this API recently, wait a few minutes for the action to propagate to our systems and retry., accessNotConfigured
```

As the message indicates, it may take a few minutes for the Compute API to be enabled in the new
project. Please wait and try again.

Once the Terraform apply finishes, a number of useful pieces of information should be output to
your console. These include some commands to run, URLs to deployed resources as well as a
populated Nomad Autoscaler job.

```
You can set the gcloud project setting for CLI use with `gcloud config set project
projects/hashistack-big-sponge`, otherwise you will need to set the `--project`
flag on each command.

To connect to any instance running within the environment you can use the
`gcloud compute ssh ubuntu@<instance_name>` command within your terminal or use the UI.

You can test the integrity of the cluster by running:

  $ consul members
  $ nomad server members
  $ nomad node status

The Nomad UI can be accessed at http://34.121.186.59:4646/ui
The Consul UI can be accessed at http://34.121.186.59:8500/ui
Grafana dashbaord can be accessed at http://35.224.138.121:3000/d/AQphTqmMk/demo?orgId=1&refresh=5s
Traefik can be accessed at http://35.224.138.121:8081
Prometheus can be accessed at http://35.224.138.121:9090
Webapp can be accessed at http://35.224.138.121:80

CLI environment variables:
export NOMAD_CLIENT_DNS=http://35.224.138.121
export NOMAD_ADDR=http://34.121.186.59:4646
```

You can visit the URLs and explore what has been created. This will include registration of a
number of Nomad jobs which provide metrics and dashboarding as well as a demo application and
routing provided by Traefik. It may take a few seconds for all the applications to start, so if
any of the URLs doesn't load the first time, wait a little bit and give it a try again.

Please also copy the export commands and run these in the terminal where the rest of the demo will
be run.

The application is pre-configured with a scaling policy, you can view this by opening the job file
or calling the Nomad API. The application scales based on the average number of active connections,
and we are targeting an average of 10 per instance of our app.

```shellsession
$ curl $NOMAD_ADDR/v1/scaling/policies
```

## Run the Autoscaler
The Autoscaler is not triggered automatically. This provides the opportunity to look through the
jobfile to understand it better before deploying. The most important parts of the `gcp_autoscaler.nomad`
file are the template sections. The first defines our agent config where we configure the
`prometheus`, `gce-mig` and `target-value` plugins. The second is where we define our cluster
scaling policy and write this to a local directory for reading. Once you have an understanding of
the job file, submit it to the Nomad cluster ensuring the `NOMAD_ADDR` env var has been exported.

```shellsession
$ nomad run gcp_autoscaler.nomad
```

If you wish, in another terminal window you can export the `NOMAD_ADDR` env var and then follow
the Nomad Autoscaler logs.

```shellsession
$ nomad logs -stderr -f <alloc-id>
```

You can now return to the [demo instrunctions](../README.md#the-demo).
