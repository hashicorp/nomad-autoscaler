package api

import "log"

type Client struct{}
type Policy struct{}
type PolicyList struct {
	ID     string
	JobID  string
	Min    int
	Max    int
	Source string
	Query  string
	Strategy
}
type Strategy struct {
	Name   string
	Config map[string]interface{}
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
			JobID:  "2",
			Source: "prometheus",
			Query:  `scalar(avg((haproxy_server_current_sessions{backend="http_back"}) and (haproxy_server_up{backend="http_back"} == 1)))`,
			Strategy: Strategy{
				Name: "target",
				Config: map[string]interface{}{
					"target": "20",
				},
			},
		},
	}
	return policies, nil
}

func (j *Jobs) Scale(req JobScaleRequest) error {
	log.Printf("scaled job %s to %d\n", req.JobID, req.Count)
	return nil
}
