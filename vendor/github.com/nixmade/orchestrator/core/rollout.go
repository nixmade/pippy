package core

import (
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/rs/zerolog"
)

// Rollout object stores current state
type Rollout struct {
	State                RolloutState                         `json:"state,omitempty"`
	TargetController     SerializedEntityTargetController     `json:"targetcontroller,omitempty"`
	MonitoringController SerializedEntityMonitoringController `json:"monitoringcontroller,omitempty"`
	entity               *Entity                              `json:"-"`
	logger               zerolog.Logger                       `json:"-"`
	lock                 sync.Mutex                           `json:"-"`
}

// RolloutState is state that needs to be serialized to storage
type RolloutState struct {
	RolloutVersionInfo `json:",inline"`
	Options            *RolloutOptions `json:"options,omitempty"`
}

type RolloutVersionInfo struct {
	TargetVersion        string `json:"targetversion,omitempty"`
	RollingVersion       string `json:"rollingversion,omitempty"`
	LastKnownGoodVersion string `json:"lastknowngoodversion,omitempty"`
	LastKnownBadVersion  string `json:"lastknownbadversion,omitempty"`
}

// rolloutInfo stores target version, targets state
type rolloutInfo struct {
	inRolloutTargets EntityTargets
	availableTargets EntityTargets
	successTargets   EntityTargets
	failedTargets    EntityTargets
	totalTargets     EntityTargets
}

// RolloutOptions rollout options
type RolloutOptions struct {
	// Percentage of targets in rollout
	BatchPercent int `json:"batchpercent,omitempty"`
	// Percentage of targets successful to mark rollout as success
	SuccessPercent int `json:"successpercent,omitempty"`
	// Timeout in secs to have successful monitoring window
	SuccessTimeoutSecs int `json:"successtimeoutsecs,omitempty"`
	// Max Duration timeout in secs to wait to have a successful monitoring window
	DurationTimeoutSecs int `json:"durationtimeoutsecs,omitempty"`
}

func (o RolloutOptions) MarshalZerologObject(e *zerolog.Event) {
	e.Int("batchpercent", o.BatchPercent).
		Int("successpercent", o.SuccessPercent).
		Int("successtimeoutsecs", o.SuccessTimeoutSecs).
		Int("durationtimeoutsecs", o.DurationTimeoutSecs)
}

// DefaultRolloutOptions conservative settings
func DefaultRolloutOptions() *RolloutOptions {
	return &RolloutOptions{
		BatchPercent:        5,
		SuccessPercent:      100,
		SuccessTimeoutSecs:  60,
		DurationTimeoutSecs: 120,
	}
}

func createRolloutInfo(targets EntityTargets) *rolloutInfo {
	return &rolloutInfo{
		totalTargets: targets,
	}
}

func (r *Rollout) setTargetVersion(targetVersion string, force bool) error {
	r.lock.Lock()
	defer r.lock.Unlock()

	r.logger.Info().Str("TargetVersion", targetVersion).Msg("Set TargetVersion")
	r.State.TargetVersion = targetVersion
	if force && !strings.EqualFold(r.State.RollingVersion, r.State.LastKnownGoodVersion) && !strings.EqualFold(r.State.RollingVersion, targetVersion) {
		r.State.LastKnownBadVersion = r.State.RollingVersion
	}
	return nil
}

func (r *Rollout) setRolloutOptions(options *RolloutOptions) error {
	r.lock.Lock()
	defer r.lock.Unlock()

	if options == nil {
		options = DefaultRolloutOptions()
	}
	r.logger.Info().EmbedObject(options).Msg("Set RolloutOptions")
	r.State.Options = options
	return nil
}

func (r *Rollout) setTargetController(controller EntityTargetController) error {
	r.lock.Lock()
	defer r.lock.Unlock()
	r.logger.Info().Msg("Set Rollout TargetController")
	r.TargetController = SerializedEntityTargetController{EntityTargetController: controller}
	return nil
}

func (r *Rollout) setMonitoringController(controller EntityMonitoringController) error {
	r.lock.Lock()
	defer r.lock.Unlock()
	r.logger.Info().Msg("Set Rollout MonitoringController")
	r.MonitoringController = SerializedEntityMonitoringController{EntityMonitoringController: controller}
	return nil
}

func (r *Rollout) updateLastKnownVersions(state *rolloutInfo) error {
	successThreshold := int((r.State.Options.SuccessPercent * len(state.totalTargets)) / 100)
	failureThreshold := len(state.totalTargets) - successThreshold

	if failureThreshold <= 0 {
		failureThreshold = 1
	}

	if len(state.failedTargets) >= failureThreshold {
		if r.State.RollingVersion != r.State.LastKnownGoodVersion {
			r.State.LastKnownBadVersion = r.State.RollingVersion
		}
		return nil
	}

	if len(state.successTargets) < successThreshold {
		return nil
	}

	if r.State.RollingVersion != r.State.LastKnownBadVersion {
		r.State.LastKnownGoodVersion = r.State.RollingVersion
	}

	return nil
}

func (r *Rollout) updateRollingVersion(state *rolloutInfo) error {
	successThreshold := int((r.State.Options.SuccessPercent * len(state.totalTargets)) / 100)

	if len(state.successTargets) < successThreshold &&
		!strings.EqualFold(r.State.RollingVersion, r.State.LastKnownGoodVersion) &&
		!strings.EqualFold(r.State.RollingVersion, r.State.LastKnownBadVersion) {
		return nil
	}

	r.logger.Info().Str("RollingVersion", r.State.RollingVersion).Str("TargetVersion", r.State.TargetVersion).Msgf("Updating rolling version to new target version")

	// Update rolling version to latest target version, since current rolling version is successful
	r.State.RollingVersion = r.State.TargetVersion

	return nil
}

func (r *Rollout) isStateChanged(state *rolloutInfo) (bool, error) {
	lastKnownBadVersion := r.State.LastKnownBadVersion
	// check if any of the versions changed, if so abandon this
	if err := r.updateLastKnownVersions(state); err != nil {
		// on any error mark as state changed
		return true, err
	}

	if lastKnownBadVersion != r.State.LastKnownBadVersion {
		// state changed, abandon this call
		r.logger.Info().Str("PreviousKnownBadVersion", lastKnownBadVersion).Str("LastKnownBadVersion", r.State.LastKnownBadVersion).Msg("State changed for rollout, lastknownbadversion updated")
		return true, nil
	}

	return false, nil
}

func (r *Rollout) determineCurrentState(state *rolloutInfo) error {
	targetVersion := r.State.RollingVersion

	if targetVersion == r.State.LastKnownBadVersion {
		r.logger.Info().Str("Version", r.State.LastKnownGoodVersion).Msg("Rolling back to version")
		targetVersion = r.State.LastKnownGoodVersion
	}

	r.logger.Info().Int("TotalTargets", len(state.totalTargets)).Msg("Determine current rollout state of targets")
	for _, entityTarget := range state.totalTargets {
		if entityTarget.State.TargetVersion.Version != targetVersion {
			state.availableTargets = addEntityTarget(state.availableTargets, entityTarget)
			continue
		}
		state.inRolloutTargets = addEntityTarget(state.inRolloutTargets, entityTarget)
	}

	r.logger.Info().Int("AvailableTargets", len(state.availableTargets)).Int("InRolloutTargets", len(state.inRolloutTargets)).Send()

	return nil
}

func removeEntityTarget(entityTargets []*EntityTarget, removeTarget *EntityTarget) EntityTargets {
	var removedTargets EntityTargets
	for _, entityTarget := range entityTargets {
		if entityTarget.Name == removeTarget.Name && entityTarget.Group == removeTarget.Group {
			continue
		}
		removedTargets = append(removedTargets, entityTarget)
	}
	return removedTargets
}

func removeClientTarget(entityTargets []*EntityTarget, clientTarget *ClientState) EntityTargets {
	var removedTargets EntityTargets
	for _, entityTarget := range entityTargets {
		if entityTarget.Name == clientTarget.Name && entityTarget.Group == clientTarget.Group {
			continue
		}
		removedTargets = append(removedTargets, entityTarget)
	}
	return removedTargets
}

func addEntityTarget(entityTargets []*EntityTarget, addTarget *EntityTarget) EntityTargets {
	for _, entityTarget := range entityTargets {
		if entityTarget.Name == addTarget.Name && entityTarget.Group == addTarget.Group {
			return entityTargets
		}
	}
	return append(entityTargets, addTarget)
}

func (r *Rollout) monitorTargets(state *rolloutInfo) error {
	targetVersion := r.State.RollingVersion

	if targetVersion == r.State.LastKnownBadVersion {
		targetVersion = r.State.LastKnownGoodVersion
	}

	r.logger.Info().Int("InRolloutTargets", len(state.inRolloutTargets)).Msg("Checking target external health monitoring")

	if err := r.MonitoringController.ExternalMonitoring(getClientTargets(state.inRolloutTargets)); err != nil {
		return err
	}

	r.logger.Info().Int("InRolloutTargets", len(state.inRolloutTargets)).Msg("Checking inRollout target health")

	for _, entityTarget := range state.inRolloutTargets {
		// version is probably assigned but target hasnt yet switched version
		// keep this target in rollout
		if entityTarget.State.CurrentVersion.Version == targetVersion {
			// call external target monitoring first, if that says failed, then its failed
			if err := r.TargetController.TargetMonitoring(getClientTarget(entityTarget)); err != nil {
				r.logger.Error().Err(err).Str("EntityTarget", entityTarget.Name).Str("Version", targetVersion).Msg("Target failed monitoring")
				state.failedTargets = addEntityTarget(state.failedTargets, entityTarget)
				entityTarget.State.TargetVersion.LastMessage.Error(fmt.Sprintf("Monitoring Failed %s", err))
				if err := r.entity.saveEntityTarget(entityTarget); err != nil {
					return err
				}
				state.inRolloutTargets = removeEntityTarget(state.inRolloutTargets, entityTarget)
				continue
			}

			// check for error
			if !entityTarget.State.CurrentVersion.LastMessage.IsError {
				duration := time.Since(entityTarget.State.CurrentVersion.LastMessage.Timestamp)
				lastMessageDuration := time.Since(entityTarget.State.LastUpdatedTimestamp)
				// if there are no errors, check if success time has passed
				// also make sure that there was a message in the success time
				if int(duration.Seconds()) > r.State.Options.SuccessTimeoutSecs &&
					int(lastMessageDuration.Seconds()) <= int(duration.Seconds()) {
					successMessage := fmt.Sprintf("monitoring successful, success since %s", entityTarget.State.CurrentVersion.LastMessage.Timestamp)
					r.logger.Info().Str("EntityTarget", entityTarget.Name).Time("LastMessage", entityTarget.State.CurrentVersion.LastMessage.Timestamp).Msg("monitoring successful")
					entityTarget.State.TargetVersion.LastMessage.Success(successMessage)
					if err := r.entity.saveEntityTarget(entityTarget); err != nil {
						return err
					}
					state.successTargets = addEntityTarget(state.successTargets, entityTarget)
					state.inRolloutTargets = removeEntityTarget(state.inRolloutTargets, entityTarget)
					continue
				}
			}
		}

		if entityTarget.State.TargetVersion.Version == targetVersion {
			// if the target never switched, may be there is some issue,
			// mark as failure after duration sec
			duration := time.Since(entityTarget.State.TargetVersion.ChangeTimestamp)
			// check to make sure assigned < duration
			if int(duration.Seconds()) > r.State.Options.DurationTimeoutSecs {
				errMessage := fmt.Sprintf("failed monitoring, no success message since %s, last message at %s",
					entityTarget.State.TargetVersion.ChangeTimestamp, entityTarget.State.CurrentVersion.LastMessage.Timestamp)
				r.logger.Error().Str("EntityTarget", entityTarget.Name).Time("LastChange", entityTarget.State.TargetVersion.ChangeTimestamp).Time("LastMessage", entityTarget.State.CurrentVersion.LastMessage.Timestamp).Msg("failed monitoring, no success message")
				state.failedTargets = addEntityTarget(state.failedTargets, entityTarget)
				entityTarget.State.TargetVersion.LastMessage.Error(errMessage)
				if err := r.entity.saveEntityTarget(entityTarget); err != nil {
					return err
				}
				state.inRolloutTargets = removeEntityTarget(state.inRolloutTargets, entityTarget)
			}
		}
	}

	return nil
}

func (r *Rollout) selectTargets(state *rolloutInfo) error {

	batchSizeCount := int(r.State.Options.BatchPercent * len(state.totalTargets) / 100)

	if batchSizeCount == 0 {
		batchSizeCount = 1
	}

	r.logger.Info().Int("InRolloutTargets", len(state.inRolloutTargets)).Int("BatchSize", batchSizeCount).Msg("Current State")

	// If there are targets already in rollout == batchSizeCount return
	if len(state.inRolloutTargets) >= batchSizeCount {
		// clear out since we should stop processing at this point
		state.availableTargets = nil
		return nil
	}

	availableSlots := batchSizeCount - len(state.inRolloutTargets)

	if len(state.availableTargets) <= 0 || availableSlots <= 0 {
		// nothing to select
		state.availableTargets = nil
		return nil
	}

	r.logger.Info().Int("AvailableTargets", len(state.availableTargets)).Int("AvailableSlots", availableSlots).Msg("Calling external target selection")

	availableTargets, err := r.TargetController.TargetSelection(getClientTargets(state.availableTargets), availableSlots)

	if err != nil {
		state.availableTargets = nil
		return nil
	}

	var selectedTargets EntityTargets
	for _, availableTarget := range availableTargets {
		if availableSlots <= 0 {
			break
		}
		found := false
		for _, entityTarget := range state.availableTargets {
			if entityTarget.Name == availableTarget.Name && entityTarget.Group == availableTarget.Group {
				selectedTargets = append(selectedTargets, entityTarget)
				found = true
				availableSlots--
				break
			}
		}
		if !found {
			entityTarget, err := r.entity.findOrCreateEntityTarget(availableTarget)
			if err != nil {
				return err
			}
			selectedTargets = append(selectedTargets, entityTarget)
		}
	}

	state.availableTargets = selectedTargets

	return nil
}

func (r *Rollout) rolloutNewTargets(state *rolloutInfo) error {

	if len(state.availableTargets) <= 0 {
		r.logger.Info().Msg("No Available targets")
		return nil
	}

	targetVersion := r.State.RollingVersion
	message := fmt.Sprintf("Rollout new version %s", targetVersion)

	if targetVersion == r.State.LastKnownGoodVersion {
		// set all the targets to lkg and update
		message = fmt.Sprintf("Setting LKG to version %s", targetVersion)
	}

	if targetVersion == r.State.LastKnownBadVersion {
		targetVersion = r.State.LastKnownGoodVersion
		message = fmt.Sprintf("Rolling back to lkg version %s", targetVersion)
	}

	r.logger.Info().Int("AvailableTargets", len(state.availableTargets)).Msg("Calling external target approval")

	approvedTargets, err := r.TargetController.TargetApproval(getClientTargets(state.availableTargets))

	if err != nil {
		return nil
	}

	r.logger.Info().Str("TargetVersion", targetVersion).Int("ApprovedTargets", len(approvedTargets)).Msg("Assigning version to approved targets")

	for _, approvedTarget := range approvedTargets {
		for _, entityTarget := range state.availableTargets {
			if entityTarget.Name != approvedTarget.Name || entityTarget.Group != approvedTarget.Group {
				continue
			}

			if entityTarget.State.TargetVersion.Version != targetVersion {
				r.logger.Debug().Str("TargetVersion", targetVersion).Str("EntityTarget", entityTarget.Name).Msg("Assigning version to entitytarget")
				entityTarget.State.TargetVersion.Version = targetVersion
				entityTarget.State.TargetVersion.ChangeTimestamp = time.Now().UTC()
				entityTarget.State.TargetVersion.LastMessage.Success(message)
				if err := r.entity.saveEntityTarget(entityTarget); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func (r *Rollout) setAllLastKnownGood(entityTargets EntityTargets) error {
	r.logger.Info().Str("RollingVersion", r.State.RollingVersion).Msg("New Entity setting LKG to version")
	r.State.LastKnownGoodVersion = r.State.RollingVersion
	// This could have been a rollout, but when we dont have something established,
	// its better to mass assign lkg, could be a scope for improvement later
	targetVersion := r.State.LastKnownGoodVersion
	for _, entityTarget := range entityTargets {
		if entityTarget.State.TargetVersion.Version != targetVersion {
			r.logger.Debug().Str("TargetVersion", targetVersion).Str("EntityTarget", entityTarget.Name).Msg("Assigning version to entitytarget")
			entityTarget.State.TargetVersion.Version = targetVersion
			entityTarget.State.TargetVersion.ChangeTimestamp = time.Now().UTC()
			entityTarget.State.TargetVersion.LastMessage.Success(fmt.Sprintf("New Entity, setting LKG to version %s", targetVersion))
			if err := r.entity.saveEntityTarget(entityTarget); err != nil {
				return err
			}
		}
	}
	return nil
}

func (r *Rollout) removeTargets(state *rolloutInfo) error {
	// if its a forward rollout, remove old targets with amount of new success targets
	// else if its a rollback, remove new targets with amount of batch size

	batchSizeCount := int(r.State.Options.BatchPercent * len(state.totalTargets) / 100)

	if batchSizeCount <= 0 {
		batchSizeCount = 1
	}

	keepVersion := r.State.RollingVersion
	targetCount := len(state.successTargets) + len(state.failedTargets) - len(state.inRolloutTargets)

	if targetCount > batchSizeCount {
		targetCount = batchSizeCount
	}

	if r.State.RollingVersion == r.State.LastKnownBadVersion {
		r.logger.Info().Msg("Rolling back to LastKnownGoodVersion")
		keepVersion = r.State.LastKnownGoodVersion
	}

	if targetCount <= 0 {
		r.logger.Info().Str("Version", keepVersion).Int("InRolloutTargets", len(state.inRolloutTargets)).Msg("Removing no targets with version since targets are still in rollout")
		return nil
	}

	r.logger.Info().Str("Version", keepVersion).Int("RemoveCount", targetCount).Msg("Keeping targets with version, removing targets")

	var removeTargets EntityTargets

	// TODO: Remove targets only part of success/failure
	for _, entityTarget := range state.totalTargets {
		if entityTarget.State.CurrentVersion.Version != keepVersion {
			removeTargets = append(removeTargets, entityTarget)
		}
	}

	// No point in calling if there are zero targets to remove
	if len(removeTargets) <= 0 {
		return nil
	}

	removedClientTargets, err := r.TargetController.TargetRemoval(getClientTargets(removeTargets), targetCount)

	if err != nil {
		return err
	}

	for _, clientTarget := range removedClientTargets {
		removeClientTarget(state.totalTargets, clientTarget)
		removeClientTarget(state.inRolloutTargets, clientTarget)
		removeClientTarget(state.availableTargets, clientTarget)
		removeClientTarget(state.successTargets, clientTarget)
		removeClientTarget(state.failedTargets, clientTarget)
		if err := r.entity.deleteEntityTarget(clientTarget); err != nil {
			return err
		}
	}

	return nil
}

// Orchestrate performs following actions in a continuous loop
//   - Determine Current state
//   - Select from intial set of targets
//   - Rollout New Version
//   - Monitoring
//   - Determine Target State
func (r *Rollout) orchestrate(targets EntityTargets) error {
	// Lock here instead of individual functions
	r.lock.Lock()
	defer r.lock.Unlock()

	if len(r.State.RollingVersion) <= 0 {
		r.State.RollingVersion = r.State.TargetVersion
	}

	if len(r.State.RollingVersion) <= 0 {
		// if its empty no point in rolling out
		return errors.New("empty target version, please set target version before calling orchestrate")
	}

	// if LKG is empty then set LKG as rolling version thinking this is first time scenario
	// dont set LKG to all targets, this could help running ops which may start from blank state
	// if len(r.State.LastKnownGoodVersion) <= 0 {
	// 	return r.setAllLastKnownGood(targets)
	// }

	r.logger = r.logger.With().Str("RollingVersion", r.State.RollingVersion).
		Str("LastKnownGoodVersion", r.State.LastKnownGoodVersion).
		Str("LastKnownBadVersion", r.State.LastKnownBadVersion).Logger()

	r.logger.Info().Msg("Creating new rollout state")

	// Create Rollout State
	state := createRolloutInfo(targets)

	// Determine current state
	if err := r.determineCurrentState(state); err != nil {
		return err
	}

	// Monitor already rolled out targets
	if err := r.monitorTargets(state); err != nil {
		return err
	}

	if ok, err := r.isStateChanged(state); ok {
		return err
	}

	// we should get rid of any old targets, otherwise we might be creating new ones unnecessarily
	if err := r.removeTargets(state); err != nil {
		return err
	}

	// Select New Targets if allowed
	if err := r.selectTargets(state); err != nil {
		return err
	}

	// Rollout New Version to targets if any
	if err := r.rolloutNewTargets(state); err != nil {
		return err
	}

	if err := r.updateLastKnownVersions(state); err != nil {
		return err
	}

	if err := r.updateRollingVersion(state); err != nil {
		return err
	}

	return nil
}
