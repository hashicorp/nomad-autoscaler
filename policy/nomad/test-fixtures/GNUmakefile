SHELL = bash
HCL = $(wildcard ./src/*.hcl)
GOLDEN = $(patsubst ./src/%.hcl, %.json.golden, $(HCL))

default: golden

.PHONY: clean
clean: clean-golden

.PHONY: golden
golden: $(GOLDEN)
	@echo "==> Done"

.PHONY: clean-golden
clean-golden:
	@echo "==> Removing existing golden files"
	@rm *.golden *.golden.tmp 2> /dev/null || true

%.json.golden:
	@echo "==> Generating golden file $@"
	@nomad run ./src/$*.hcl > /dev/null
	@nomad inspect $* > $@
	@nomad stop -purge $* > /dev/null
	@# hardcode scaling policy ID so tests don't break every time the files are regenerated
	@jq -rM 'if .Job.TaskGroups[0].Scaling then.Job.TaskGroups[0].Scaling.ID = "id" else . end' $@  > $@.tmp && mv $@{.tmp,}
