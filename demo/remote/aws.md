### Amazon Web Services
The following steps require you to authenticate with AWS. This can be done by exporting your
credentials locally using the `AWS_SECRET_ACCESS_KEY`, `AWS_ACCESS_KEY_ID` and `AWS_DEFAULT_REGION`
environment variables.

Firstly you will need to build an AMI which is used to launch the Nomad server and client
instances. Please be careful to replace the variables where necessary which will help identify
resources in your environment. The region flag can be omitted if you are using the `us-east-1`
region.
```
$ cd packer
$ packer build \
    -var 'created_email="<your_email_address>"' \
    -var 'created_name="<your_name>"' \
    -var 'region=<your_desired_region>' \
    aws-packer.pkr.hcl
```

You can then navigate to the Terraform AWS environment you will be using to build the
infrastructure components.
```
$ cd ../terraform/env/aws
```

In order for Terraform to run correctly you'll need to provide the appropriate variables within a
file named `terraform.tfvars`. You can rename the `terraform.tfvars.sample` file and then update the
variables. The most important are the `ami` which is the AMI ID you built in the previous step and
the `key_name` which is your EC2 key-pair for authenticating to the instances via SSH.

You can then run the Terraform commands, carefully observing the output of each.
```
$ terraform init
$ terraform plan
$ terraform apply --auto-approve
```

Once the Terraform apply finishes, a number of useful pieces of information should be output to
your console. These include URLs to deployed resources as well as a semi-populated Nomad Autoscaler
config.
```
Apply complete! Resources: 23 added, 0 changed, 0 destroyed.

Outputs:

ip_addresses =
Server IPs:
 * instance hashistack-server-1 - Public: 52.47.76.15, Private: 172.31.18.240

To connect, add your private key and SSH into any client or server with
`ssh ubuntu@PUBLIC_IP`. You can test the integrity of the cluster by running:

  $ consul members
  $ nomad server members
  $ nomad node status

The Nomad UI can be accessed at http://hashistack-nomad-server-1436431625.eu-west-3.elb.amazonaws.com:4646/ui
The Consul UI can be accessed at http://hashistack-nomad-server-1436431625.eu-west-3.elb.amazonaws.com:8500/ui
Grafana can be accessed at http://hashistack-nomad-client-167906299.eu-west-3.elb.amazonaws.com:3000/d/AQphTqmMk/demo?orgId=1&refresh=5s
Traefik can be accessed at http://hashistack-nomad-client-167906299.eu-west-3.elb.amazonaws.com:8081
Prometheus can be accessed at http://hashistack-nomad-client-167906299.eu-west-3.elb.amazonaws.com:9090
Webapp can be accessed at http://hashistack-nomad-client-167906299.eu-west-3.elb.amazonaws.com:80

CLI environment variables:
export NOMAD_CLIENT_DNS=http://hashistack-nomad-client-167906299.eu-west-3.elb.amazonaws.com
export NOMAD_ADDR=http://hashistack-nomad-server-1436431625.eu-west-3.elb.amazonaws.com:4646
export CONSUL_HTTP_ADDR=http://hashistack-nomad-server-1436431625.eu-west-3.elb.amazonaws.com:8500
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
```
curl $NOMAD_ADDR/v1/scaling/policies
```

## Run the Autoscaler
The Autoscaler is not triggered automatically. This provides the opportunity to look through the
jobfile to understand it better before deploying. The most important parts of the `aws_autoscaler.nomad`
file are the template sections. The first defines our agent config where we configure the
`prometheus`, `aws-asg` and `target-value` plugins. The second is where we define our cluster
scaling policy and write this to a local directory for reading. Once you have an understanding of
the job file, submit it to the Nomad cluster ensuring the `NOMAD_ADDR` env var has been exported.
```
$ cd ../../..
$ nomad run aws_autoscaler.nomad
```

If you wish, in another terminal window you can export the `NOMAD_ADDR` env var and then follow
the Nomad Autoscaler logs.
```
$ nomad logs -stderr -f <alloc-id>
```

You can now return to the [demo instrunctions](./README.md#the-demo).

## Post Demo Steps
The AMI is built outside of Terraform's control and therefore needs to be deregistered. This can be
performed via the AWS console or via the [AWS CLI](https://aws.amazon.com/cli/).

```
$ aws ec2 deregister-image --image-id <ami_id>
```
