package nomadmeta

import "sync"

type DiffReporter struct {
	last map[string][]string // job to list of nodes
	lock sync.RWMutex        // protects last
}

func NewDiffReporter() *DiffReporter {
	return &DiffReporter{
		last: make(map[string][]string),
		lock: sync.RWMutex{},
	}
}

func (d *DiffReporter) Diff(job string, nodes []string) (added, removed []string, before []string) {
	d.lock.Lock()
	defer d.lock.Unlock()

	lastNodes, ok := d.last[job]
	if !ok {
		d.last[job] = nodes
		return nil, nil, nil
	}

	added = diff(nodes, lastNodes)
	removed = diff(lastNodes, nodes)

	d.last[job] = nodes

	return added, removed, lastNodes
}

func diff(a, b []string) []string {
	m := make(map[string]struct{}, len(b))
	for _, item := range b {
		m[item] = struct{}{}
	}

	result := make([]string, 0, len(a))

	for _, item := range a {
		if _, found := m[item]; !found {
			result = append(result, item)
		}
	}

	return result
}
