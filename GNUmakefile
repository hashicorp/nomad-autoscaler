SHELL = bash
default: lint check test build

GIT_COMMIT := $(shell git rev-parse --short HEAD)
GIT_DIRTY := $(if $(shell git status --porcelain),+CHANGES)

GO_LDFLAGS := "-X github.com/hashicorp/nomad-autoscaler/version.GitCommit=$(GIT_COMMIT)$(GIT_DIRTY)"

.PHONY: tools
tools: ## Install the tools used to test and build
	@echo "==> Installing tools"
	GO111MODULE=off go get -u github.com/golangci/golangci-lint/cmd/golangci-lint
	@echo "==> Done"

.PHONY: build
build:
	@echo "==> Building autoscaler..."
	@CGO_ENABLED=0 GO111MODULE=on \
	go build \
	-ldflags $(GO_LDFLAGS) \
	-o ./bin/nomad-autoscaler
	@echo "==> Done"

.PHONY: lint
lint: ## Lint the source code
	@echo "==> Linting source code..."
	@golangci-lint run -j 1
	@echo "==> Done"

.PHONY: check
check: check-sdk check-mod

.PHONY: check-sdk
check-sdk: ## Checks the SDK pkg is isolated
	@echo "==> Checking SDK package is isolated..."
	@if go list --test -f '{{ join .Deps "\n" }}' ./sdk | grep github.com/hashicorp/nomad-autoscaler/ | grep -v -e /nomad-autoscaler/sdk/ -e nomad-autoscaler/sdk.test; \
		then echo " /sdk package depends the ^^ above internal packages. Remove such dependency"; \
		exit 1; fi
	@echo "==> Done"

.PHONEY: check-mod
check-mod: ## Checks the Go mod is tidy
	@echo "==> Checking Go mod and Go sum..."
	@GO111MODULE=on go mod tidy
	@if (git status --porcelain | grep -Eq "go\.(mod|sum)"); then \
		echo go.mod or go.sum needs updating; \
		git --no-pager diff go.mod; \
		git --no-pager diff go.sum; \
		exit 1; fi
	@echo "==> Done"

.PHONY: test
test: ## Test the source code
	@echo "==> Testing source code..."
	@$(MAKE) -C plugins/test
	@go test -v -race -cover ./...
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

bin/plugins/nomad-apm:
	@echo "==> Building $@"
	@mkdir -p $$(dirname $@)
	@cd ./plugins/builtin/apm/nomad && go build -o ../../../../$@
	@echo "==> Done"

bin/plugins/nomad-target:
	@echo "==> Building $@"
	@mkdir -p $$(dirname $@)
	@cd ./plugins/builtin/target/nomad && go build -o ../../../../$@
	@echo "==> Done"

bin/plugins/prometheus:
	@echo "==> Building $@"
	@mkdir -p $$(dirname $@)
	@cd ./plugins/builtin/apm/prometheus && go build -o ../../../../$@
	@echo "==> Done"

bin/plugins/target-value:
	@echo "==> Building $@"
	@mkdir -p $$(dirname $@)
	@cd ./plugins/builtin/strategy/target-value && go build -o ../../../../$@
	@echo "==> Done"

bin/plugins/aws-asg:
	@echo "==> Building $@"
	@mkdir -p $$(dirname $@)
	@cd ./plugins/builtin/target/aws-asg && go build -o ../../../../$@
	@echo "==> Done"

bin/plugins/datadog:
	@echo "==> Building $@"
	@mkdir -p $$(dirname $@)
	@cd ./plugins/builtin/apm/datadog && go build -o ../../../../$@
	@echo "==> Done"

bin/plugins/azure-vmss:
	@echo "==> Building $@"
	@mkdir -p $$(dirname $@)
	@cd ./plugins/builtin/target/azure-vmss && go build -o ../../../../$@
	@echo "==> Done"

.PHONY: plugins
plugins: bin/plugins/nomad-apm bin/plugins/nomad-target bin/plugins/prometheus bin/plugins/target-value bin/plugins/aws-asg bin/plugins/datadog bin/plugins/azure-vmss
