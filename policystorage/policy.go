package policystorage

type PolicyStorage interface {
	List() ([]*PolicyListStub, error)
	Get(string) (Policy, error)
}

type Policy struct {
	ID       string
	Source   string
	Query    string
	Target   *Target
	Strategy *Strategy
}

type PolicyListStub struct {
	ID     string
	Source string
	Query  string
	Target
	Strategy
}

type Strategy struct {
	Name   string
	Min    int
	Max    int
	Config map[string]string
}

type Target struct {
	Name   string
	Config map[string]string
}
