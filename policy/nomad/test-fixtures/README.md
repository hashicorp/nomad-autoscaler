# Test Fixtures
This directory contains fixture files used for testing the Nomad policy source.

To add a new test, create a Nomad jobspec inside the `src` directory and
generate the golden file using the instructions below.

# Regenerating golden files
Golden files are generate from the source HCL jobspec inside `src`. The
generation process uses the Nomad API to parse the HCL into JSON, so first you
will need to have a Nomad agent running:

```shellsession
$ nomad agent -dev
```

## Regenerating all golden files
To regenerate all the files run:

```shellsession
$ make -B
```

You can also generate multiple in parallel to speed the process:

```shellsession
$ make -B -j <NUM OF CORES>
```

## Regenerating specific golden file
You can also generate specific golden files by specifying a target:

```shellsession
$ make -B full-scaling.json.golden
```
