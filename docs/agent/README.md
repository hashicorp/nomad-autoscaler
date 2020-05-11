# Nomad Autoscaler Agent
Nomad Autoscaler agents have a variety of parameters that can be specified via configuration files or command-line flags. Configuration files are written in [HCL](https://github.com/hashicorp/hcl). The Nomad Autoscaler can read and combine parameters from multiple configuration files or directories to configure the agent.

## Nomad Namespaces

The Nomad Autoscaler currently has limited support for 
[Nomad Namespaces](https://learn.hashicorp.com/nomad/governance-and-policy/namespaces).
The `nomad` configuration below supports specifying a namespace; if configured with a namespace,
the Autoscaler will retrieve scaling policies and perform autoscaling only for jobs in that namespace.
Future version will include support for multiple namespaces.

## Nomad ACLs

The Nomad Autoscaler can be configured to interact with an 
[ACL-enabled](https://learn.hashicorp.com/nomad?track=acls#acls) Nomad cluster.
Nomad 0.11 includes the `scale` ACL policy disposition specifically for supporting the operations of the Nomad Autoscaler.
The Technical Preview has the additional requirement for the `read-job` ACL capability. Therefore, the following policy
is sufficienty for creating an ACL token that can be used by the autoscaler:
```hcl
namespace "default" {
  policy = "scale"
  capabilities = ["read-job"]
}
```
A token created using this policy should be provided in the `nomad` configuration, as described below.

## Load Order and Merging
The Nomad Autoscaler agent supports multiple configuration files, which can be provided using the `-config` CLI flag. The flag can accept either a file or folder. In the case of a folder, any `.hcl` and `.json` files in the folder will be loaded and merged in lexicographical order. Directories are not loaded recursively.

For example:
```
$ nomad-autoscaler agent -config=autoscaler.conf -config=/etc/nomad-autoscaler -config=extra.json
```
This will load configuration from `autoscaler.conf`, from `.hcl` and `.json` files under `/etc/nomad-autoscaler`, and finally from `extra.json`.

As each file is processed, its contents are merged into the existing configuration. When merging, any non-empty values from the latest config file will append or replace parameters in the current configuration. An empty value means `""` for strings, `0` for integer or float values, and `false` for booleans.

## General Parameters
 * `log_level` `(string: "INFO")` -  Specify the verbosity level of Nomad Autoscaler's logs. Valid values include `DEBUG`, `INFO`, and `WARN`, in decreasing order of verbosity.
 * `log_json` `(bool: false)` - Output logs in a JSON format.
 * `plugin_dir` `(string: "./plugins")` - The plugin directory used to discover Nomad Autoscaler plugins.
 * `default_evaluation_interval` `(duration: "10s")` - The default time to use when a policy doesn't specify an `evaluation_interval`.

## `http` Block
The `http` block configures the Nomad Autoscaler's HTTP health endpoint.
```hcl
http {
  bind_address = "10.0.0.10"
  bind_port    = 9999
}
```

### `http` Parameters
 * `bind_address` `(string "127.0.0.1")` - The HTTP address that the health server will bind to.
 * `bind_port` `(int 8080)` - The port that the health server will bind to.

## `nomad` Block
The `nomad` block configures the Nomad Autoscaler's Nomad client.
```hcl
nomad {
  address = "http://my-nomad.systems:4646"
  region  = "esp-vlc-1"
}
```

### `nomad` Parameters
 * `address` `(string "http://127.0.0.1:4646")` - The address of the Nomad server in the form of `protocol://addr:port`.
 * `region` `(string "global")` - The region of the Nomad servers to connect with.
 * `namespace` `(string "")` - The target namespace for queries and actions bound to a namespace.
 * `token` `(string "")` - The `SecretID` of an ACL token to use to authenticate API requests with.
 * `http_auth` `(string "")` - The authentication information to use when connecting to a Nomad API which is using HTTP authentication.
 * `ca_cert` `(string "")` - Path to a PEM encoded CA cert file to use to verify the Nomad server SSL certificate.
 * `ca_path` `(string "")` - Path to a directory of PEM encoded CA cert files to verify the Nomad server SSL certificate.
 * `client_cert` `(string "")` - Path to a PEM encoded client certificate for TLS authentication to the Nomad server.
 * `client_key` `(string "")` - Path to an unencrypted PEM encoded private key matching the client certificate.
 * `tls_server_name` `(string "")` - The server name to use as the SNI host when connecting via TLS.
 * `skip_verify` `(bool false)` - Do not verify TLS certificates. This is strongly discouraged.

## `policy` Block
The `policy` block configures the Nomad Autoscaler's policy handling.
```hcl
policy {
  default_cooldown = "2m"
}
```

### `policy` Parameters
 * `default_cooldown` `(string "5m")` - The default cooldown that will be applied to all scaling policies which do not specify a cooldown period.

## `apm` Block
The `apm` block is used to configure application performance metric (APM) plugins.
```hcl
apm "example-apm-plugin" {
  driver = "example-apm-plugin"
  args   = ["-my-flag"]

  config = {
    address = "http://127.0.0.1:9090"
  }
}
```

### `apm` Parameters
 * `args` `(array<string>: [])` - Specifies a set of arguments to pass to the plugin binary when it is executed.
 * `driver` `(string: "")` - The plugin's executable name relative to to the `plugin_dir`. If the plugin has a suffix, such as `.exe`, this should be omitted.
 * `config` `(hcl/json: nil)` - Specifies configuration values for the plugin either as HCL or JSON. The accepted values are plugin specific. Please refer to the individual plugin's documentation.

## `target` Block
The `target` block is used to configure scaling target plugins.
```hcl
target "example-target-plugin" {
  driver = "example-target-plugin"
  args   = ["-my-flag"]

  config = {
    region = "esp-vlc-1"
  }
}
```

### `target` Parameters
 * `args` `(array<string>: [])` - Specifies a set of arguments to pass to the plugin binary when it is executed.
 * `driver` `(string: "")` - The plugin's executable name relative to to the plugin_dir. If the plugin has a suffix, such as `.exe`, this should be omitted.
 * `config` `(hcl/json: nil)` - Specifies configuration values for the plugin either as HCL or JSON. The accepted values are plugin specific. Please refer to the individual plugin's documentation.

## `strategy` Block
The `strategy` block is used to configure scaling strategy plugins.
```hcl
strategy "example-strategy-plugin" {
  driver = "example-strategy-plugin"
  args   = ["-my-flag"]

  config = {
    algorithm = "complex"
  }
}
```

### `strategy` Parameters
 * `args` `(array<string>: [])` - Specifies a set of arguments to pass to the plugin binary when it is executed.
 * `driver` `(string: "")` - The plugin's executable name relative to to the plugin_dir. If the plugin has a suffix, such as `.exe`, this should be omitted.
 * `config` `(hcl/json: nil)` - Specifies configuration values for the plugin either as HCL or JSON. The accepted values are plugin specific. Please refer to the individual plugin's documentation.
