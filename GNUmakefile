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

.PHONY: plugins
plugins:
	@for d in apm/plugins target/plugins strategy/plugins; do \
	   for p in $${d}/*; do \
		   plugin=$$(basename $$p); \
			 echo "==> Building $${plugin}..."; \
			 pushd $$p > /dev/null; \
			 go build -o ../../../plugins/$$plugin; \
			 popd > /dev/null;  \
			 echo "==> Done"; \
     done; \
   done

.PHONY: build-docker
build-docker:
	@echo "==> Building autoscaler docker container..."
	@env GOOS=linux GOARCH=amd64 go build -v -o ./bin/nomad-autoscaler-linux-amd64
	@docker build .
	@echo "==> Done"
