SHELL = bash
default: lint test build

.PHONY: tools
tools: ## Install the tools used to test and build
	@echo "==> Installing tools"
	GO111MODULE=off go get -u github.com/golangci/golangci-lint/cmd/golangci-lint
	@echo "==> Done"

.PHONY: build
build:
	@echo "==> Building autoscaler..."
	@GOPRIVATE=$$GOPRIVATE,github.com/hashicorp/nomad-private go build -o ./bin/nomad-autoscaler
	@echo "==> Done"

.PHONY: lint
lint: ## Lint the source code
	@echo "==> Linting source code..."
	@golangci-lint run -j 1
	@echo "==> Done"

.PHONY: test
test: ## Test the source code
	@echo "==> Testing source code..."
	@go test -v -race -cover ./...
	@echo "==> Done"

.PHONY: build-docker
build-docker:
	@echo "==> Building autoscaler docker container..."
	@env GOOS=linux GOARCH=amd64 go build -v -o ./bin/nomad-autoscaler-linux-amd64
	@docker build .
	@echo "==> Done"

.PHONY: clean-plugins
clean-plugins:
	@echo "==> Cleaning plugins..."
	@rm -rf ./bin/plugins/
	@echo "==> Done"

.PHONY: clean
clean: clean-plugins
	@echo "==> Cleaning build artifacts..."
	@rm -f ./bin/nomad-autoscaler
	@echo "==> Done"

bin/plugins/nomad:
	@echo "==> Building $@"
	@mkdir -p $$(dirname $@)
	@cd ./plugins/nomad && go build -o ../../$@
	@echo "==> Done"

bin/plugins/prometheus:
	@echo "==> Building $@"
	@mkdir -p $$(dirname $@)
	@cd ./plugins/prometheus && go build -o ../../$@
	@echo "==> Done"

bin/plugins/target-value:
	@echo "==> Building $@"
	@mkdir -p $$(dirname $@)
	@cd ./plugins/target-value && go build -o ../../$@
	@echo "==> Done"

.PHONY: plugins
plugins: bin/plugins/nomad bin/plugins/prometheus bin/plugins/target-value
