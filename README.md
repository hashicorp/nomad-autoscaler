# nomad-autoscaler

***Project in very early stage***

## Building

```sh
# build auto-scaler
make build
# build plugins
make plugins
```

## Running

First we'll need a test server to monitor. The easiest way to get started is by using the Vagrant box and Nomad jobs provided in the `example` folder:

```sh
cd example
vagrant up
vagrant provision --provision-with app
```

Inside this box we have Nomad and Consul running. The second command will start the Nomad jobs for our application. Once it's complete we'll have a demo web app behind an HAProxy server and a monitoring stack running Prometheus and Grafana.

### Custom binaries

By default the Vagrant box will contain an official release of Nomad and Consul. You can use a custom built binary by placing them inside the `example` folder as `./example/nomad` and `./example/consul`.

### Exposed services

The Vagrant box exposes multiple ports to your host so they can be accessed via `localhost` and facilitate development:

* **1936**: HAProxy stats page
* **3000**: Grafana
* **4646**: Nomad UI
* **8080**: Demo web application
* **8500**: Consul UI
* **9090**: Prometheus

Check each of those ports to make sure everything is running properly.

### Grafana

You can import the sample dashboard to track connections to the web application. After setting up Prometheus as a data source, click on the `+` button in the sidebar and then `Import`. Upload the file `example/grafana-dashboard.json`.

### Start autoscaler

```sh
./bin/nomad-autoscaler run -config ./example/config.hcl
```

### Generate load

Use a load testing tool to generate traffic agains our web application. Using [hey](https://github.com/rakyll/hey):

```sh
hey -z 5m -c 30 http://127.0.0.1:8080
```

This will run the load for 5 minutes with 30 connections in parallel. You should see the autoscaler output printing scaling intents:

```sh
❯ ./nomad-autoscaler run -config ./example/config.hcl
...
2020-01-29T21:16:24.813-0500 [INFO]  agent: reading policies: policy_storage=policystorage.Nomad
2020-01-29T21:16:24.816-0500 [INFO]  agent: found 1 policies: policy_storage=policystorage.Nomad
2020-01-29T21:16:24.818-0500 [INFO]  agent: fetching current count: policy_id=be2442c9-8627-3bda-106f-fc219ee10230 source=prometheus strategy=target-value target=local-nomad
2020-01-29T21:16:24.825-0500 [INFO]  agent: querying APM: policy_id=be2442c9-8627-3bda-106f-fc219ee10230 source=prometheus strategy=target-value target=local-nomad
2020-01-29T21:16:24.827-0500 [INFO]  agent: calculating new count: policy_id=be2442c9-8627-3bda-106f-fc219ee10230 source=prometheus strategy=target-value target=local-nomad
2020-01-29T21:16:24.827-0500 [INFO]  agent: scaling target: policy_id=be2442c9-8627-3bda-106f-fc219ee10230 source=prometheus strategy=target-value target=local-nomad target_config="map[group:demo job_id:webapp]" from=1 to=3 reason="scaling up because factor is 3.000000"
```

