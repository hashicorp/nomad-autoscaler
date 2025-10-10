// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package file

import (
	"time"

	multierror "github.com/hashicorp/go-multierror"
	"github.com/hashicorp/hcl/v2/hclsimple"
	"github.com/hashicorp/nomad-autoscaler/sdk"
)

func decodeFile(file string) (map[string]*sdk.ScalingPolicy, error) {
	policies := make(map[string]*sdk.ScalingPolicy)

	filePolicies := sdk.FileDecodeScalingPolicies{}
	if err := hclsimple.DecodeFile(file, nil, &filePolicies); err != nil {
		return nil, err
	}

	var mErr *multierror.Error
	for _, p := range filePolicies.ScalingPolicies {
		if err := decodePolicyDoc(p); err != nil {
			mErr = multierror.Append(mErr, multierror.Prefix(err, p.Name))
		}
		policies[p.Name] = p.Translate()
	}
	if mErr != nil {
		return nil, mErr.ErrorOrNil()
	}

	return policies, nil

}

func decodePolicyDoc(decodePolicy *sdk.FileDecodeScalingPolicy) error {
	// Assume file policies are cluster policies unless specificied.
	// TODO: revisit this assumption.
	if decodePolicy.Type == "" {
		decodePolicy.Type = sdk.ScalingPolicyTypeCluster
	}

	if decodePolicy.Doc.CooldownHCL != "" {
		d, err := time.ParseDuration(decodePolicy.Doc.CooldownHCL)
		if err != nil {
			return err
		}
		decodePolicy.Doc.Cooldown = d
	}

	if decodePolicy.Doc.CooldownOnScaleUpHCL != "" {
		d, err := time.ParseDuration(decodePolicy.Doc.CooldownOnScaleUpHCL)
		if err != nil {
			return err
		}
		decodePolicy.Doc.CooldownOnScaleUp = d
	}

	if decodePolicy.Doc.EvaluationIntervalHCL != "" {
		d, err := time.ParseDuration(decodePolicy.Doc.EvaluationIntervalHCL)
		if err != nil {
			return err
		}
		decodePolicy.Doc.EvaluationInterval = d
	}

	// Parse query window for each check.
	for i := 0; i < len(decodePolicy.Doc.Checks); i++ {
		check := decodePolicy.Doc.Checks[i]

		if check.QueryWindowHCL != "" {
			w, err := time.ParseDuration(check.QueryWindowHCL)
			if err != nil {
				return err
			}
			decodePolicy.Doc.Checks[i].QueryWindow = w
		}

		if check.QueryWindowOffsetHCL != "" {
			o, err := time.ParseDuration(check.QueryWindowOffsetHCL)
			if err != nil {
				return err
			}
			decodePolicy.Doc.Checks[i].QueryWindowOffset = o
		}
	}

	return nil
}
