# Nomad Autoscaler HTTP API
The Nomad Autoscaler exposes a small, simple API to be used for health checking the agent.

## Health API
This endpoint can be used to query the Nomad Autoscaler agent aliveness. If the agent is alive, the request will return a `200 OK`, otherwise it will return a `503 ServiceUnavailable`.

| Method   | Path                         |
| :--------------------------- | :--------------------- |
| `GET`    | `/v1/health`              |

### Sample Request
```
$ curl http://127.0.0.1:8080/v1/health
```
