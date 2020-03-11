SHELL = bash
default: lint build

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

.PHONY: plugins
plugins:
	@for d in apm/plugins target/plugins strategy/plugins; do \
	   for p in $${d}/*; do \
		   plugin=$$(basename $$p); \
			 echo "==> Building $${plugin}..."; \
			 pushd $$p > /dev/null; \
			 go build -o ../../../plugins/$$plugin; \
			 popd > /dev/null;  \
     done; \
   done
	@echo "==> Done"

.PHONY: build-docker
build-docker:
	@echo "==> Building autoscaler docker container..."
	@env GOOS=linux GOARCH=amd64 go build -v -o ./bin/nomad-autoscaler-linux-amd64
	@docker build .
	@echo "==> Done"
