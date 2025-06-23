// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package file

import (
	"context"
	"crypto/md5"
	"fmt"
	"reflect"
	"sync"

	hclog "github.com/hashicorp/go-hclog"
	multierror "github.com/hashicorp/go-multierror"
	"github.com/hashicorp/nomad-autoscaler/policy"
	"github.com/hashicorp/nomad-autoscaler/sdk"
	fileHelper "github.com/hashicorp/nomad-autoscaler/sdk/helper/file"
	"github.com/hashicorp/nomad-autoscaler/sdk/helper/uuid"
)

// Ensure NomadSource satisfies the Source interface.
var _ policy.Source = (*Source)(nil)

// pathMD5Sum is the key used in the idMap. Having this as a type makes it
// clearer to readers what this represents.
type pathMD5Sum [16]byte

// Source is the File implementation of the policy.Source interface.
type Source struct {
	dir             string
	log             hclog.Logger
	policyProcessor *policy.Processor

	// idMap stores a mapping between between the md5sum of the file path and
	// the associated policyID. This allows us to keep a consistent PolicyID in
	// the event of policy changes.
	idMap     map[pathMD5Sum]policy.PolicyID
	idMapLock sync.RWMutex

	// reloadChannels help coordinate reloading the of the MonitorIDs routine.
	reloadCh         chan struct{}
	reloadCompleteCh chan struct{}

	// policyMap maps our policyID to the file and policy which was decode from
	// the file. This is required with the current policy.Source interface
	// implementation, as the GetLatestVersion function only has access to the
	// policyID and not the underlying file path.
	policyMap     map[policy.PolicyID]*filePolicy
	policyMapLock sync.RWMutex
}

// filePolicy is a wrapper around a scaling policy that also provides the file
// and name that it came from.
type filePolicy struct {
	file   string
	name   string
	policy *sdk.ScalingPolicy
}

func NewFileSource(log hclog.Logger, dir string, policyProcessor *policy.Processor) policy.Source {
	return &Source{
		dir:              dir,
		log:              log.ResetNamed("file_policy_source"),
		idMap:            make(map[pathMD5Sum]policy.PolicyID),
		policyMap:        make(map[policy.PolicyID]*filePolicy),
		reloadCh:         make(chan struct{}),
		reloadCompleteCh: make(chan struct{}, 1),
		policyProcessor:  policyProcessor,
	}
}

// Name satisfies the Name function of the policy.Source interface.
func (s *Source) Name() policy.SourceName {
	return policy.SourceNameFile
}

// MonitorIDs satisfies the MonitorIDs function of the policy.Source interface.
func (s *Source) MonitorIDs(ctx context.Context, req policy.MonitorIDsReq) {
	s.log.Debug("starting file policy source ID monitor")

	// Run the policyID identification method before entering the loop so we do
	// a first pass on the policies. Otherwise we wouldn't load any until a
	// reload is triggered.
	s.identifyPolicyIDs(req.ResultCh, req.ErrCh)

	for {
		select {
		case <-ctx.Done():
			s.log.Trace("stopping file policy source ID monitor")
			return

		case <-s.reloadCh:
			s.log.Info("file policy source ID monitor received reload signal")
			s.identifyPolicyIDs(req.ResultCh, req.ErrCh)
			s.reloadCompleteCh <- struct{}{}
		}
	}
}

// ReloadIDsMonitor satisfies the ReloadIDsMonitor function of the
// policy.Source interface.
func (s *Source) ReloadIDsMonitor() {
	s.reloadCh <- struct{}{}
	<-s.reloadCompleteCh
}

// handleIndividualPolicyRead reads the policy from disk and compares it to the
// stored version if there is one. If there is a difference, `changed` will be
// true to indicate that reload is required.
// This function is not thread safe, so the caller should obtain at least
// a read lock on policyMapLock.
func (s *Source) handleIndividualPolicyRead(ID policy.PolicyID, path, name string) (
	policy *sdk.ScalingPolicy, changed bool, err error) {

	// Decode the file into a new policy to allow comparison to our stored
	// policy. Make sure to add the ID string and defaults, we are responsible
	// for managing this and if we don't add it, there will always be a
	// difference.
	policies, err := decodeFile(path)
	if err != nil {
		return nil, false, fmt.Errorf("failed to decode file %s: %v", path, err)
	}

	newPolicy, ok := policies[name]
	if !ok {
		return nil, false, fmt.Errorf("policy %q doesn't exist in file %s", name, path)
	}

	newPolicy.ID = ID
	s.policyProcessor.ApplyPolicyDefaults(newPolicy)

	if err := s.policyProcessor.ValidatePolicy(newPolicy); err != nil {
		return nil, false, fmt.Errorf("failed to validate file %s: %v", path, err)
	}

	for _, c := range newPolicy.Checks {
		s.policyProcessor.CanonicalizeCheck(c, newPolicy.Target)
	}

	val, ok := s.policyMap[ID]
	if !ok || val.policy == nil {
		return newPolicy, true, nil
	}

	// Check the new policy against the stored. If they are the same, and
	// therefore the policy has not changed indicate that to the caller.
	changed = !reflect.DeepEqual(newPolicy, val.policy)

	return newPolicy, changed, nil
}

// identifyPolicyIDs iterates the configured directory, identifying the
// configured policyIDs. The IDs will be wrapped and sent to the resultCh so
// the policy manager can do its work.
func (s *Source) identifyPolicyIDs(resultCh chan<- policy.IDMessage, errCh chan<- error) {
	ids, err := s.handleDir()
	if err != nil {
		policy.HandleSourceError(s.Name(), err, errCh)
	}

	// Even if we receive an error we may have IDs to send. Otherwise it may be
	// that all policies have been removed so we should even send the empty
	// list so handlers can be cleaned.
	resultCh <- policy.IDMessage{IDs: ids, Source: s.Name()}
}

// handleDir iterates through the configured directory, attempting to decode
// and store all HCL and JSON files as scaling policies.
func (s *Source) handleDir() (map[policy.PolicyID]bool, error) {

	// Obtain a list of all files in the directory which have the suffixes we
	// can handle as scaling policies.
	files, err := fileHelper.GetFileListFromDir(s.dir, ".hcl", ".json")
	if err != nil {
		return nil, fmt.Errorf("failed to list files in directory: %v", err)
	}

	policyIDs := map[policy.PolicyID]bool{}
	var mErr *multierror.Error

	for _, file := range files {

		// We have to decode the file to read the policies name and check
		// whether they are enabled or not.
		// If we cannot decode the file, append an error but do not bail on
		// the process. A single decode failure shouldn't stop us decoding the
		// rest of the files in the directory.
		policies, err := decodeFile(file)
		if err != nil {
			mErr = multierror.Append(fmt.Errorf("failed to decode file %s: %v", file, err), mErr)
			continue
		}

		for name, scalingPolicy := range policies {
			// Get the policyID for the file.
			policyID := s.getFilePolicyID(file, name)
			scalingPolicy.ID = policyID

			if !scalingPolicy.Enabled {
				s.log.Trace("policy is disabled",
					"policy_id", scalingPolicy.ID, "file", file)
				// If the policy is disabled, we do not need to process it
				// further. We can skip it and continue to the next one.
				continue
			}

			s.policyProcessor.ApplyPolicyDefaults(scalingPolicy)

			if err := s.policyProcessor.ValidatePolicy(scalingPolicy); err != nil {
				mErr = multierror.Append(fmt.Errorf("failed to validate file %s: %v", file, err), mErr)
				continue
			}

			for _, c := range scalingPolicy.Checks {
				s.policyProcessor.CanonicalizeCheck(c, scalingPolicy.Target)
			}

			// Store the file/name>id mapping if it doesn't exist. This makes the
			// GetLatestVersion function simpler as we have an easy mapping of the
			// policyID to the file it came from.
			//
			// The OK check is performed because this function gets called during
			// the initial load and then on reloads of the monitor IDs routine.
			// When we are asked to reload, if we overwrite what is stored, when we
			// subsequently trigger reload of the individual policy monitor we can
			// never tell whether the newly read policy differs from the stored
			// policy.
			s.policyMapLock.Lock()
			if _, ok := s.policyMap[policyID]; !ok {
				s.policyMap[policyID] = &filePolicy{file: file, name: name}
			}
			s.policyMapLock.Unlock()

			// The update is always true because the file source only reads the
			// policies from disk once at the start of MonitorIDs, when all the
			// policies are loaded.
			policyIDs[policyID] = true
		}
	}

	return policyIDs, mErr.ErrorOrNil()
}

// getFilePolicyID translates the file into its policyID. This is done by
// firstly checking our internal state. If it isn't found, we generate and
// store the ID in our state.
func (s *Source) getFilePolicyID(file, name string) policy.PolicyID {

	// The function performs at least a read and potentially a write so obtain
	// a lock.
	s.idMapLock.Lock()
	defer s.idMapLock.Unlock()

	// MD5 the file path so we have our map key to perform lookups.
	md5Sum := md5Sum(file + "/" + name)

	// Attempt to lookup the policyID. If we do not find it within our map then
	// this is the first time we have seen this file. Therefore generate a UUID
	// and store this.
	policyID, ok := s.idMap[md5Sum]
	if !ok {
		policyID = uuid.Generate()
		s.idMap[md5Sum] = policyID
	}

	return policyID
}

func md5Sum(i interface{}) [16]byte {
	return md5.Sum([]byte(fmt.Sprintf("%v", i)))
}

func (s *Source) GetLatestVersion(_ context.Context, policyID policy.PolicyID) (*sdk.ScalingPolicy, error) {
	s.policyMapLock.Lock()
	defer s.policyMapLock.Unlock()

	val, ok := s.policyMap[policyID]
	if !ok {
		return nil, fmt.Errorf("failed to get policy %s", policyID)
	}

	return val.policy, nil
}
