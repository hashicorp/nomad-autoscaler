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
./nomad-autoscaler run -config ./example/config.hcl
```

### Generate load

Use a load testing tool to generate traffic agains our web application. Using [hey](https://github.com/rakyll/hey):

```sh
hey -z 5m -c 30 http://127.0.0.1:8080
```

This will run the load for 5 minutes with 30 connections in parallel. You should see the autoscaler output printing scaling intents:

```sh
❯ ./nomad-autoscaler run -config ./example/config.hcl
2020/01/24 17:06:49 loading plugin: {prometheus prometheus [] map[address:http://192.168.50.66:9090]}
2020-01-24T17:06:49.015-0500 [DEBUG] plugin: starting plugin: path=plugins/prometheus args=[plugins/prometheus]
2020-01-24T17:06:49.017-0500 [DEBUG] plugin: plugin started: path=plugins/prometheus pid=75314
2020-01-24T17:06:49.017-0500 [DEBUG] plugin: waiting for RPC address: path=plugins/prometheus
2020-01-24T17:06:49.034-0500 [DEBUG] plugin: using plugin: version=1
2020-01-24T17:06:49.034-0500 [DEBUG] plugin.prometheus: plugin address: address=/var/folders/ws/vnlrm7x11pv9j16jnhbq02rc0000gp/T/plugin985253353 network=unix timestamp=2020-01-24T17:06:49.034-0500
2020-01-24T17:06:49.036-0500 [DEBUG] plugin.prometheus: 2020/01/24 17:06:49 config: map[address:http://192.168.50.66:9090]
2020/01/24 17:06:54 Scaled job 2 to 2. Reason: scaling up because factor is 1.500000

```

