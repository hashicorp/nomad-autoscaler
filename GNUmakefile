SHELL = bash
default: build

.PHONY: build
build:
	@echo "Building autoscaler..."
	@go build -o ./bin/nomad-autoscaler
	@echo "Done"

.PHONY: plugins
plugins:
	@for d in apm/plugins target/plugins strategy/plugins; do \
	   for p in $${d}/*; do \
		   plugin=$$(basename $$p); \
			 echo "Building $${plugin}..."; \
			 pushd $$p > /dev/null; \
			 go build -o ../../../plugins/$$plugin; \
			 popd > /dev/null;  \
     done; \
   done
	@echo "Done"

.PHONY: build-docker
build-docker:
	@echo "Building autoscaler docker container..."
	@env GOOS=linux GOARCH=amd64 go build -v -o ./bin/nomad-autoscaler-linux-amd64
	@docker build .
	@echo "Done"
