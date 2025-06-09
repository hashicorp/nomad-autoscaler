.PHONY: all
all: binary plugins

.PHONY: plugins
plugins:
	go build -ldflags "${GO_KIT_LDFLAGS}"  -o output/static-count-apm ./plugins/kentik/apm/static-count
	go build -ldflags "${GO_KIT_LDFLAGS}"  -o output/nomad-meta-apm-v2 ./plugins/kentik/apm/nomad-meta-v2
	go build -ldflags "${GO_KIT_LDFLAGS}"  -o output/kentik-nomad-target ./plugins/kentik/target/nomad

.PHONY: binary
binary:
	go build -o nomad-autoscaler .

.PHONY: test
test:
	go test $(shell go list ./... | grep -v integration)
