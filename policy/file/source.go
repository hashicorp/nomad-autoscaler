package file

import (
	"context"
	"crypto/md5"
	"errors"
	"fmt"
	"sync"

	hclog "github.com/hashicorp/go-hclog"
	multierror "github.com/hashicorp/go-multierror"
	fileHelper "github.com/hashicorp/nomad-autoscaler/helper/file"
	"github.com/hashicorp/nomad-autoscaler/helper/uuid"
	"github.com/hashicorp/nomad-autoscaler/policy"
)

// Ensure NomadSource satisfies the Source interface.
var _ policy.Source = (*Source)(nil)

// pathMD5Sum is the key used in the idMap. Having this as a type makes it
// clearer to readers what this represents.
type pathMD5Sum [16]byte

// Source is the File implementation of the policy.Source interface.
type Source struct {
	config *policy.ConfigDefaults
	dir    string
	log    hclog.Logger

	// reloadChan is the channel that the agent sends to in order to trigger a
	// reload of all file policies.
	//
	// TODO(jrasell) the reloadChan is not yet triggered by anything and needs
	//  to be hooked up via the agent.
	reloadChan chan bool

	// policyReloadChan is used internally to trigger a reload of individual
	// policies from disk.
	policyReloadChan chan bool

	// idMap stores a mapping between between the md5sum of the file path and
	// the associated policyID. This allows us to keep a consistent PolicyID in
	// the event of policy changes.
	idMap     map[pathMD5Sum]policy.PolicyID
	idMapLock sync.RWMutex

	// policyMap maps our policyID to the file and policy which was decode from
	// the file. This is required with the current policy.Source interface
	// implementation, as the MonitorPolicy function only has access to the
	// policyID and not the underlying file path.
	policyMap     map[policy.PolicyID]*filePolicy
	policyMapLock sync.RWMutex
}

// filePolicy is a wrapper around a scaling policy that also provides the file
// that it came from.
type filePolicy struct {
	file   string
	policy *policy.Policy
}

func NewFileSource(log hclog.Logger, cfg *policy.ConfigDefaults, dir string, reloadCh chan bool) policy.Source {
	return &Source{
		config:           cfg,
		dir:              dir,
		log:              log.ResetNamed("file_policy_source"),
		idMap:            make(map[pathMD5Sum]policy.PolicyID),
		policyMap:        make(map[policy.PolicyID]*filePolicy),
		reloadChan:       reloadCh,
		policyReloadChan: make(chan bool),
	}
}

// Name satisfies the Name function of the policy.Source interface.
func (s *Source) Name() policy.SourceName {
	return policy.SourceNameFile
}

// MonitorIDs satisfies the MonitorIDs function of the policy.Source interface.
func (s *Source) MonitorIDs(ctx context.Context, resultCh chan<- policy.IDMessage, errCh chan<- error) {
	s.log.Debug("starting file policy source ID monitor")

	// Run the policyID identification method before entering the loop so we do
	// a first pass on the policies. Otherwise we wouldn't load any until a
	// reload is triggered.
	s.identifyPolicyIDs(resultCh, errCh)

	for {
		select {
		case <-ctx.Done():
			s.log.Trace("stopping file policy source ID monitor")
			return

		case <-s.reloadChan:
			s.log.Info("file policy source ID monitor received reload signal")

			// We are reloading all files within the directory so wipe our
			// current mapping data.
			s.idMapLock.Lock()
			s.idMap = make(map[pathMD5Sum]policy.PolicyID)
			s.idMapLock.Unlock()

			s.identifyPolicyIDs(resultCh, errCh)

			// Tell the MonitorPolicy routines to reload their policy.
			s.policyReloadChan <- true
		}
	}
}

// MonitorPolicy satisfies the MonitorPolicy function of the policy.Source
// interface.
func (s *Source) MonitorPolicy(ctx context.Context, ID policy.PolicyID, resultCh chan<- policy.Policy, errCh chan<- error) {

	// Close channels when done with the monitoring loop.
	defer close(resultCh)
	defer close(errCh)

	s.policyMapLock.RLock()

	// There isn't a possibility that I can think of where this call wouldn't
	// be ok. Nevertheless check it to be safe before sending the policy to the
	// handler which starts the evaluation ticker.
	val, ok := s.policyMap[ID]
	if !ok {
		errCh <- fmt.Errorf("failed to get policy %s", ID)
	} else {
		resultCh <- *val.policy
	}
	s.policyMapLock.RUnlock()

	// Technically the log message should come further up, but its quite
	// helpful to have the file path logged with the policyID otherwise there
	// is no way to understand the ID->File mapping.
	log := s.log.With("policy_id", ID, "file", val.file)
	log.Debug("starting file policy monitor")

	for {
		select {
		case <-ctx.Done():
			log.Debug("stopping file source ID monitor")
			return

		case <-s.policyReloadChan:
			s.log.Info("file policy source monitor received reload signal")

			newPolicy, err := s.handlerPolicyReload(ID)
			if err != nil {
				errCh <- fmt.Errorf("failed to get policy: %v", err)
				continue
			}

			// A non-nil policy indicates a change, therefore we send this to
			// the handler.
			if newPolicy != nil {
				resultCh <- *newPolicy
			}
		}
	}
}

// handlerPolicyReload reads the policy from disk and compares it to the stored
// version. If there is a difference the new policy will be returned, otherwise
// we return nil to indicate no reload is required.
func (s *Source) handlerPolicyReload(ID policy.PolicyID) (*policy.Policy, error) {

	val, ok := s.policyMap[ID]
	if !ok {
		return nil, errors.New("policy not found within internal store")
	}

	newPolicy := &policy.Policy{}

	// Decode the file into a new policy to allow comparison to our stored
	// policy. Make sure to add the ID string and defaults, we are responsible
	// for managing this and if we don't add it, there will always be a
	// difference.
	if err := decodeFile(val.file, newPolicy); err != nil {
		return nil, fmt.Errorf("failed to decode file %s: %v", val.file, err)
	}
	newPolicy.ID = ID.String()
	newPolicy.ApplyDefaults(s.config)

	for _, c := range newPolicy.Checks {
		c.CanonicalizeAPMQuery(newPolicy.Target)
	}

	// Check the new policy against the stored. If they are the same, and
	// therefore the policy has not changed indicate that to the caller.
	if md5Sum(newPolicy) == md5Sum(val) {
		return nil, nil
	}
	return newPolicy, nil
}

// identifyPolicyIDs iterates the configured directory, identifying the
// configured policyIDs. The IDs will be wrapped and sent to the resultCh so
// the policy manager can do its work.
func (s *Source) identifyPolicyIDs(resultCh chan<- policy.IDMessage, errCh chan<- error) {
	ids, err := s.handleDir()
	if err != nil {
		errCh <- err
	}

	// Even if we receive an error we may have IDs to send. Otherwise it may be
	// that all policies have been removed so we should even send the empty
	// list so handlers can be cleaned.
	resultCh <- policy.IDMessage{IDs: ids, Source: s.Name()}
}

// handleDir iterates through the configured directory, attempting to decode
// and store all HCL and JSON files as scaling policies. If the policy is not
// enabled it will be ignored.
func (s *Source) handleDir() ([]policy.PolicyID, error) {

	// Obtain a list of all files in the directory which have the suffixes we
	// can handle as scaling policies.
	files, err := fileHelper.GetFileListFromDir(s.dir, ".hcl", ".json")
	if err != nil {
		return nil, fmt.Errorf("failed to list files in directory: %v", err)
	}

	var policyIDs []policy.PolicyID
	var mErr *multierror.Error

	for _, file := range files {

		var scalingPolicy policy.Policy

		// We have to decode the file to check whether the policy is enabled or
		// not. If we cannot decode the file, append an error but do not bail
		// on the process. A single decode failure shouldn't stop us decoding
		// the rest of the files in the directory.
		if err := decodeFile(file, &scalingPolicy); err != nil {
			mErr = multierror.Append(fmt.Errorf("failed to decode file %s: %v", file, err), mErr)
			continue
		}

		// Get the policyID for the file add it to the policy.
		policyID := s.getFilePolicyID(file)
		scalingPolicy.ID = policyID.String()

		// Ignore the policy if its disabled. The log line is because I
		// (jrasell) have spent too much time figuring out why a policy doesn't
		// get tracked here.
		if !scalingPolicy.Enabled {
			s.log.Trace("policy is disabled therefore ignoring",
				"policy_id", scalingPolicy.ID, "file", file)
			continue
		}

		scalingPolicy.ApplyDefaults(s.config)

		for _, c := range scalingPolicy.Checks {
			c.CanonicalizeAPMQuery(scalingPolicy.Target)
		}

		// We have had to decode the file, so store the information. This makes
		// the MonitorPolicy function simpler as we have an easy mapping of the
		// policyID to the file it came from.
		s.policyMapLock.Lock()
		s.policyMap[policyID] = &filePolicy{file: file, policy: &scalingPolicy}
		s.policyMapLock.Unlock()

		policyIDs = append(policyIDs, policyID)
	}

	return policyIDs, mErr.ErrorOrNil()
}

// getFilePolicyID translates the file into its policyID. This is done by
// firstly checking our internal state. If it isn't found, we generate and
// store the ID in our state.
func (s *Source) getFilePolicyID(file string) policy.PolicyID {

	// The function performs at least a read and potentially a write so obtain
	// a lock.
	s.idMapLock.Lock()
	defer s.idMapLock.Unlock()

	// MD5 the file path so we have our map key to perform lookups.
	md5Sum := md5Sum(file)

	// Attempt to lookup the policyID. If we do not find it within our map then
	// this is the first time we have seen this file. Therefore generate a UUID
	// and store this.
	policyID, ok := s.idMap[md5Sum]
	if !ok {
		policyID = policy.PolicyID(uuid.Generate())
		s.idMap[md5Sum] = policyID
	}

	return policyID
}

func md5Sum(i interface{}) [16]byte {
	return md5.Sum([]byte(fmt.Sprintf("%v", i)))
}
