package api

import "log"

type Client struct{}
type Policy struct{}
type PolicyList struct {
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
type Jobs struct{}

type JobScaleRequest struct {
	JobID  string
	Count  int
	Reason string
}

func NewClient(opts map[string]string) (*Client, error) {
	return &Client{}, nil
}

func (c *Client) Policies() *Policy {
	return &Policy{}
}

func (c *Client) Jobs() *Jobs {
	return &Jobs{}
}

func (p *Policy) List() ([]*PolicyList, error) {
	policies := []*PolicyList{
		{
			ID:     "1",
			Source: "prometheus",
			Query:  `scalar(avg((haproxy_server_current_sessions{backend="http_back"}) and (haproxy_server_up{backend="http_back"} == 1)))`,
			Strategy: Strategy{
				Name: "target-value",
				Min:  1,
				Max:  10,
				Config: map[string]string{
					"target": "20",
				},
			},
			Target: Target{
				Name: "local-nomad",
				Config: map[string]string{
					"job_id":   "webapp",
					"group":    "demo",
					"property": "count",
				},
			},
		},
	}
	return policies, nil
}

func (j *Jobs) Scale(req JobScaleRequest) error {
	log.Printf("Scaled job %s to %d. Reason: %s\n", req.JobID, req.Count, req.Reason)
	return nil
}
