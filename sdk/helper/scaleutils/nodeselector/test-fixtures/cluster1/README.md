# `cluster1` test fixtures

The files in this directory are used to mock the `/v1/node/<id>/allocations`
endpoint from the Nomad API. It contains actual JSON responses from a Nomad
clusters with 3 nodes with the following allocations:

* `151be1dc-92e7-a488-f7dc-49faaf5a4c96`: 1 system job, 1 running job, and 1 complete job.
* `b2d5bbb6-a61d-ee1f-a896-cbc6efe4f7fe`: 1 system job and 1 complete job.
* `b535e699-1112-c379-c020-ebd80fdd9f09`: 1 complete job.

All clients were AWS t2.small instances, with 1x2.5 GHz CPU and 2GB of memory.
