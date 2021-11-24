//go:build !ent
// +build !ent

package config

// DefaultEntConfig allows configuring enterprise only default configuration
// values.
func DefaultEntConfig() *Agent { return &Agent{} }
