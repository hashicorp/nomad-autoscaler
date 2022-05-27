SHELL = bash
default: lint check test build

GIT_COMMIT := $(shell git rev-parse --short HEAD)
GIT_DIRTY := $(if $(shell git status --porcelain),+CHANGES)

GO_LDFLAGS := "-X github.com/hashicorp/nomad-autoscaler/version.GitCommit=$(GIT_COMMIT)$(GIT_DIRTY)"

# Attempt to use gotestsum for running tests, otherwise fallback to go test.
GO_TEST_CMD = $(if $(shell command -v gotestsum 2>/dev/null),gotestsum --,go test)

.PHONY: tools
tools: lint-tools test-tools generate-tools

.PHONY: generate-tools
generate-tools: ## Install the tools used to generate code
	@echo "==> Installing code generate tools..."
	GO111MODULE=on cd tools && go get github.com/bufbuild/buf/cmd/buf@v0.36.0
	GO111MODULE=on cd tools && go get github.com/golang/protobuf/protoc-gen-go@v1.4.3
	@echo "==> Done"

.PHONY: test-tools
test-tools: ## Install the tools used to run tests
	@echo "==> Installing test tools..."
	GO111MODULE=on cd tools && go get gotest.tools/gotestsum@v0.6.0
	@echo "==> Done"

.PHONY: lint-tools
lint-tools: ## Install the tools used to lint
	@echo "==> Installing lint tools..."
	GO111MODULE=on cd tools && go get github.com/golangci/golangci-lint/cmd/golangci-lint@v1.24.0
	GO111MODULE=on cd tools && go get -u honnef.co/go/tools/cmd/staticcheck@2020.1.6
	GO111MODULE=on cd tools && go get github.com/hashicorp/go-hclog/hclogvet@v0.1.3
	GO111MODULE=on cd tools && go get github.com/hashicorp/hcl/v2/cmd/hclfmt@d0c4fa8b0bbc2e4eeccd1ed2a32c2089ed8c5cf1
	@echo "==> Done"

pkg/%/nomad-autoscaler: GO_OUT ?= $@
pkg/windows_%/nomad-autoscaler: GO_OUT = $@.exe
pkg/%/nomad-autoscaler: ## Build Nomad Autoscaler for GOOS_GOARCH, e.g. pkg/linux_amd64/nomad
	@echo "==> Building $@ with tags $(GO_TAGS)..."
	@CGO_ENABLED=0 \
		GOOS=$(firstword $(subst _, ,$*)) \
		GOARCH=$(lastword $(subst _, ,$*)) \
		go build -trimpath -ldflags $(GO_LDFLAGS) -tags "$(GO_TAGS)" -o $(GO_OUT)

.PRECIOUS: pkg/%/nomad-autoscaler
pkg/%.zip: pkg/%/nomad-autoscaler
	@echo "==> Packaging for $@..."
	zip -j $@ $(dir $<)*

.PHONY: dev
dev: lint
	@echo "==> Building autoscaler..."
	@CGO_ENABLED=0 GO111MODULE=on \
	go build \
	-ldflags $(GO_LDFLAGS) \
	-o ./bin/nomad-autoscaler
	@echo "==> Done"

.PHONY: proto
proto: ## Generate the protocol buffers
	@echo "==> Generating proto bindings..."
	@buf --config tools/buf/buf.yaml --template tools/buf/buf.gen.yaml generate
	@echo "==> Done"

.PHONY: lint
lint: hclfmt ## Lint the source code
	@echo "==> Linting source code..."
	@golangci-lint run -j 1
	@staticcheck ./...
	@hclogvet .
	@buf lint --config=./tools/buf/buf.yaml
	@echo "==> Done"

.PHONY: hclfmt
hclfmt: ## Format HCL files with hclfmt
	@echo "--> Formatting HCL"
	@find . -name '.git' -prune \
	        -o \( -name '*.nomad' -o -name '*.hcl' -o -name '*.tf' \) \
	      -print0 | xargs -0 hclfmt -w

.PHONY: check
check: check-sdk check-root-mod check-tools-mod check-protobuf

.PHONY: check-sdk
check-sdk: ## Checks the SDK pkg is isolated
	@echo "==> Checking SDK package is isolated..."
	@if go list --test -f '{{ join .Deps "\n" }}' ./sdk | grep github.com/hashicorp/nomad-autoscaler/ | grep -v -e /nomad-autoscaler/sdk/ -e nomad-autoscaler/sdk.test; \
		then echo " /sdk package depends the ^^ above internal packages. Remove such dependency"; \
		exit 1; fi
	@echo "==> Done"

.PHONY: check-root-mod
check-root-mod: ## Checks the root Go mod is tidy
	@echo "==> Checking Go mod and Go sum..."
	@GO111MODULE=on go mod tidy
	@if (git status --porcelain | grep -Eq "go\.(mod|sum)"); then \
		echo go.mod or go.sum needs updating; \
		git --no-pager diff go.mod; \
		git --no-pager diff go.sum; \
		exit 1; fi
	@echo "==> Done"

.PHONY: check-tools-mod
check-tools-mod: ## Checks the tools Go mod is tidy
	@echo "==> Checking tools Go mod and Go sum..."
	@GO111MODULE=on cd tools && go mod tidy
	@if (git status --porcelain | grep -Eq "go\.(mod|sum)"); then \
		echo tools go.mod or go.sum needs updating; \
		git --no-pager diff go.mod; \
		git --no-pager diff go.sum; \
		exit 1; fi
	@echo "==> Done"

.PHONY: check-protobuf
check-protobuf: ## Checks the protobuf files are in-sync
	@$(MAKE) proto
	@echo "==> Checking proto files are in-sync..."
	@if (git status -s | grep -q .pb.go); then echo the following proto files are out of sync; git status -s | grep .pb.go; exit 1; fi
	@echo "==> Done"

.PHONY: test
test: ## Test the source code
	@$(MAKE) -C plugins/test
	@echo "==> Testing source code..."
	@$(GO_TEST_CMD) -v -race -cover ./...
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
	@echo "==> Building $@..."
	@mkdir -p $$(dirname $@)
	@cd ./plugins/builtin/apm/nomad && go build -o ../../../../$@
	@echo "==> Done"

bin/plugins/nomad-target:
	@echo "==> Building $@..."
	@mkdir -p $$(dirname $@)
	@cd ./plugins/builtin/target/nomad && go build -o ../../../../$@
	@echo "==> Done"

bin/plugins/prometheus:
	@echo "==> Building $@..."
	@mkdir -p $$(dirname $@)
	@cd ./plugins/builtin/apm/prometheus && go build -o ../../../../$@
	@echo "==> Done"

bin/plugins/target-value:
	@echo "==> Building $@..."
	@mkdir -p $$(dirname $@)
	@cd ./plugins/builtin/strategy/target-value && go build -o ../../../../$@
	@echo "==> Done"

bin/plugins/fixed-value:
	@echo "==> Building $@..."
	@mkdir -p $$(dirname $@)
	@cd ./plugins/builtin/strategy/fixed-value && go build -o ../../../../$@
	@echo "==> Done"

bin/plugins/pass-through:
	@echo "==> Building $@..."
	@mkdir -p $$(dirname $@)
	@cd ./plugins/builtin/strategy/pass-through && go build -o ../../../../$@
	@echo "==> Done"

bin/plugins/threshold:
	@echo "==> Building $@..."
	@mkdir -p $$(dirname $@)
	@cd ./plugins/builtin/strategy/threshold && go build -o ../../../../$@
	@echo "==> Done"

bin/plugins/aws-asg:
	@echo "==> Building $@..."
	@mkdir -p $$(dirname $@)
	@cd ./plugins/builtin/target/aws-asg && go build -o ../../../../$@
	@echo "==> Done"

bin/plugins/datadog:
	@echo "==> Building $@..."
	@mkdir -p $$(dirname $@)
	@cd ./plugins/builtin/apm/datadog && go build -o ../../../../$@
	@echo "==> Done"

bin/plugins/azure-vmss:
	@echo "==> Building $@..."
	@mkdir -p $$(dirname $@)
	@cd ./plugins/builtin/target/azure-vmss && go build -o ../../../../$@
	@echo "==> Done"

bin/plugins/gce-mig:
	@echo "==> Building $@..."
	@mkdir -p $$(dirname $@)
	@cd ./plugins/builtin/target/gce-mig && go build -o ../../../../$@
	@echo "==> Done"

.PHONY: plugins
plugins: \
	bin/plugins/nomad-apm \
	bin/plugins/nomad-target \
	bin/plugins/prometheus \
	bin/plugins/target-value \
	bin/plugins/fixed-value \
	bin/plugins/pass-through \
	bin/plugins/threshold \
	bin/plugins/aws-asg \
	bin/plugins/datadog \
	bin/plugins/azure-vmss \
	bin/plugins/gce-mig

.PHONY: version
version:
ifneq (,$(wildcard version/version_ent.go))
	@$(CURDIR)/scripts/version.sh version/version.go version/version_ent.go
else
	@$(CURDIR)/scripts/version.sh version/version.go version/version.go
endif
