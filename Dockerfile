FROM alpine:3.9
MAINTAINER Nomad Team <nomad@hashicorp.com>

COPY ./bin/nomad-autoscaler-linux-amd64 /bin/nomad-autoscaler

ENTRYPOINT ["nomad-autoscaler"]

CMD ["run"]
