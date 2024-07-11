package core

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/nixmade/orchestrator/store"
	"github.com/rs/zerolog"
)

const (
	zombieTargetTimeout = 900 * time.Second
	rolloutPrefix       = "rollout:"
	entityTargetPrefix  = "entitytarget:"
)

// Entity has a list of Targets
// We do need to serialize controller, which could be endpoints
type Entity struct {
	Name      string         `json:"name,omitempty"`
	Namespace string         `json:"namespace,omitempty"`
	store     store.Store    `json:"-"`
	logger    zerolog.Logger `json:"-"`
}

// CreateEntity creates entity
func (n *Namespace) createEntity(name string) (*Entity, error) {
	n.logger.Info().Str("Entity", name).Msg("Creating new entity")

	e := &Entity{
		Name:      name,
		Namespace: n.Name,
		store:     n.store,
		logger:    n.logger.With().Str("Entity", name).Logger(),
	}

	return e, n.store.SaveJSON(n.entityKey(name), e)
}

func (e *Entity) rolloutKey() string {
	return fmt.Sprintf("%s%s/%s", rolloutPrefix, e.Namespace, e.Name)
}

func (e *Entity) entityTargetKey(group, name string) string {
	return fmt.Sprintf("%s%s/%s/%s/%s", entityTargetPrefix, e.Namespace, e.Name, group, name)
}

func (e *Entity) findOrCreateRollout() (*Rollout, error) {
	rollout := &Rollout{}
	err := e.store.LoadJSON(e.rolloutKey(), rollout)
	if err == store.ErrKeyNotFound {
		e.logger.Info().Msg("Creating new rollout")
		rollout := &Rollout{
			State: RolloutState{
				RolloutVersionInfo: RolloutVersionInfo{},
				Options:            DefaultRolloutOptions(),
			},
			TargetController:     SerializedEntityTargetController{EntityTargetController: &NoOpEntityTargetController{}},
			MonitoringController: SerializedEntityMonitoringController{EntityMonitoringController: &NoOpEntityMonitoringController{}},
			entity:               e,
			logger:               e.logger,
		}

		return rollout, e.store.SaveJSON(e.rolloutKey(), rollout)
	}

	if err != nil {
		return nil, err
	}

	rollout.entity = e
	rollout.logger = e.logger

	return rollout, nil
}

func (e *Entity) getEntityTargets() ([]*EntityTarget, error) {
	return e.getGroupEntityTargets("")
}

func (e *Entity) getGroupEntityTargets(groupName string) ([]*EntityTarget, error) {
	prefix := fmt.Sprintf("%s%s/%s/%s", entityTargetPrefix, e.Namespace, e.Name, groupName)

	var entityTargets []*EntityTarget
	entityTargetItr := func(key any, value any) error {
		entityTarget := &EntityTarget{}
		if err := json.Unmarshal([]byte(value.(string)), entityTarget); err != nil {
			return err
		}
		entityTargets = append(entityTargets, entityTarget)
		return nil
	}
	err := e.store.LoadValues(prefix, entityTargetItr)
	if err != nil {
		return nil, err
	}

	return entityTargets, nil
}

func (e *Entity) findOrCreateEntityTarget(clientTarget *ClientState) (*EntityTarget, error) {
	entityTarget := &EntityTarget{}
	err := e.store.LoadJSON(e.entityTargetKey(clientTarget.Group, clientTarget.Name), entityTarget)
	if err == store.ErrKeyNotFound {
		e.logger.Info().
			Str("Name", clientTarget.Name).
			Str("Group", clientTarget.Group).
			Str("Version", clientTarget.Version).
			Bool("IsError", clientTarget.IsError).
			Msg("Creating new target")
		nowTime := time.Now().UTC()
		rollout, err := e.findOrCreateRollout()
		if err != nil {
			return nil, err
		}
		entityTarget := &EntityTarget{
			Name:  clientTarget.Name,
			Group: clientTarget.Group,
			Tags:  clientTarget.Tags,
			State: EntityTargetState{
				CurrentVersion: EntityVersionInfo{
					Version:         clientTarget.Version,
					ChangeTimestamp: nowTime.Add(-time.Second * time.Duration(rollout.State.Options.SuccessTimeoutSecs)),
					LastMessage: Message{
						Message:   clientTarget.Message,
						Timestamp: nowTime,
						IsError:   clientTarget.IsError,
					},
				},
				TargetVersion: EntityVersionInfo{
					Version:         rollout.State.LastKnownGoodVersion,
					ChangeTimestamp: nowTime.Add(-time.Second * time.Duration(rollout.State.Options.SuccessTimeoutSecs)),
					LastMessage: Message{
						Message:   "new target, setting lkg",
						Timestamp: nowTime,
						IsError:   false,
					},
				},
				LastUpdatedTimestamp: nowTime,
			},
		}

		return entityTarget, e.store.SaveJSON(e.entityTargetKey(clientTarget.Group, clientTarget.Name), entityTarget)
	}

	if err != nil {
		return nil, err
	}

	return entityTarget, nil
}

func (e *Entity) deleteEntityTarget(clientTarget *ClientState) error {
	return e.store.Delete(e.entityTargetKey(clientTarget.Group, clientTarget.Name))
}

func (e *Entity) setTargetController(controller EntityTargetController) error {
	rollout, err := e.findOrCreateRollout()
	if err != nil {
		return err
	}

	if err = rollout.setTargetController(controller); err != nil {
		return err
	}

	return e.store.SaveJSON(e.rolloutKey(), rollout)
}

func (e *Entity) setMonitoringController(controller EntityMonitoringController) error {
	rollout, err := e.findOrCreateRollout()
	if err != nil {
		return err
	}

	if err = rollout.setMonitoringController(controller); err != nil {
		return err
	}

	return e.store.SaveJSON(e.rolloutKey(), rollout)
}

func copyClientState(clientTarget *ClientState, entityTarget *EntityTarget) {
	nowTime := time.Now().UTC()
	entityTarget.State.LastUpdatedTimestamp = nowTime
	// record only on error switches or when version changes
	if entityTarget.State.CurrentVersion.LastMessage.IsError != clientTarget.IsError ||
		entityTarget.State.CurrentVersion.Version != clientTarget.Version {
		entityTarget.State.CurrentVersion.LastMessage.Timestamp = nowTime
	}
	if entityTarget.State.CurrentVersion.Version != clientTarget.Version {
		entityTarget.State.CurrentVersion.ChangeTimestamp = nowTime
		entityTarget.State.CurrentVersion.Version = clientTarget.Version
	}
	entityTarget.State.CurrentVersion.LastMessage.Message = clientTarget.Message
	entityTarget.State.CurrentVersion.LastMessage.IsError = clientTarget.IsError
}

func (e *Entity) updateEntityTarget(clientTarget *ClientState, entityTarget *EntityTarget) error {
	e.logger.Info().
		Str("Name", clientTarget.Name).
		Str("Group", clientTarget.Group).
		Str("Version", clientTarget.Version).
		Bool("IsError", clientTarget.IsError).
		Msg("Updating target")

	copyClientState(clientTarget, entityTarget)

	return e.store.SaveJSON(e.entityTargetKey(clientTarget.Group, clientTarget.Name), entityTarget)
}

// refreshes internal entity target state
func (e *Entity) updateEntityTargets(targets []*ClientState) error {
	for _, clientTarget := range targets {
		entityTarget, err := e.findOrCreateEntityTarget(clientTarget)
		if err != nil {
			return err
		}
		if err := e.updateEntityTarget(clientTarget, entityTarget); err != nil {
			return err
		}
	}

	entityTargets, err := e.getEntityTargets()
	if err != nil {
		return err
	}

	// cleanup zombie targets after specific timeout
	for _, entityTarget := range entityTargets {
		if time.Since(entityTarget.State.LastUpdatedTimestamp) > zombieTargetTimeout {
			if err := e.store.Delete(e.entityTargetKey(entityTarget.Group, entityTarget.Name)); err != nil {
				return err
			}
		}
	}
	return nil
}

func (e *Entity) saveEntityTarget(entityTarget *EntityTarget) error {
	return e.store.SaveJSON(e.entityTargetKey(entityTarget.Group, entityTarget.Name), entityTarget)
}

// checkpoint internal entity target state
func (e *Entity) saveEntityTargets(entityTargets EntityTargets) error {
	for _, entityTarget := range entityTargets {
		err := e.saveEntityTarget(entityTarget)
		if err != nil {
			return err
		}
	}

	return nil
}

// returns updated Client state from current Entity Target state matching groupName
func (e *Entity) returnClientGroupState(groupName string) ([]*ClientState, error) {
	entityTargets, err := e.getGroupEntityTargets(groupName)
	if err != nil {
		return nil, err
	}
	var retTargets []*ClientState
	for _, entityTarget := range entityTargets {
		message := fmt.Sprintf("%s at %s", entityTarget.State.TargetVersion.LastMessage.Message, entityTarget.State.TargetVersion.LastMessage.Timestamp)
		e.logger.Debug().
			Str("Name", entityTarget.Name).
			Str("Group", entityTarget.Group).
			Str("Version", entityTarget.State.TargetVersion.Version).
			Str("LastMessage", message).
			Bool("IsError", entityTarget.State.TargetVersion.LastMessage.IsError).
			Msg("Returning Target")
		clientTarget := &ClientState{
			Name:    entityTarget.Name,
			Group:   entityTarget.Group,
			Version: entityTarget.State.TargetVersion.Version,
			Message: message,
			IsError: entityTarget.State.TargetVersion.LastMessage.IsError,
		}
		retTargets = append(retTargets, clientTarget)
	}
	return retTargets, nil
}

// returns updated Client state from current Entity Target state
func (e *Entity) returnClientState() ([]*ClientState, error) {
	return e.returnClientGroupState("")
}

// SetTargetVersion sets the targetversion
func (e *Entity) setTargetVersion(version string, force bool) error {
	rollout, err := e.findOrCreateRollout()
	if err != nil {
		return err
	}

	if err = rollout.setTargetVersion(version, force); err != nil {
		return err
	}

	return e.store.SaveJSON(e.rolloutKey(), rollout)
}

// SetRolloutOptions sets the rolloutOptions
func (e *Entity) setRolloutOptions(options *RolloutOptions) error {
	rollout, err := e.findOrCreateRollout()
	if err != nil {
		return err
	}

	if err = rollout.setRolloutOptions(options); err != nil {
		return err
	}

	return e.store.SaveJSON(e.rolloutKey(), rollout)
}

// Orchestrate over current entity and list of targets
// Checks if there is an ongoing rollout, takes the current state and determines target state
// If no rollout, Checks if there is a new version/action for registered entities
// Creates new rollout if required
// Orchestrate rollouts in order
func (e *Entity) orchestrate(targets []*ClientState) ([]*ClientState, error) {
	e.logger.Info().Msg("Refreshing target state")

	if err := e.updateEntityTargets(targets); err != nil {
		return nil, err
	}

	e.logger.Info().Msg("Orchestrate rollout")

	if err := e.rolloutOrchestrate(); err != nil {
		return nil, err
	}

	e.logger.Info().Msg("returning client state")

	return e.returnClientState()
}

func (e *Entity) rolloutOrchestrate() error {
	e.logger.Info().Msg("Orchestrate rollout")

	rollout, err := e.findOrCreateRollout()
	if err != nil {
		return err
	}

	entityTargets, err := e.getEntityTargets()
	if err != nil {
		return err
	}

	if err := rollout.orchestrate(entityTargets); err != nil {
		return err
	}

	return e.store.SaveJSON(e.rolloutKey(), rollout)
}

// orchestrateasync records input target state and sends an async message to orchestrate
func (e *Entity) orchestrateasync(targets []*ClientState) error {
	e.logger.Info().Msg("Refreshing target state")

	if err := e.updateEntityTargets(targets); err != nil {
		return err
	}

	go func() {
		if err := e.rolloutOrchestrate(); err != nil {
			e.logger.Error().Err(err).Msg("Async rollout orchestrate failed")
		}
	}()

	return nil
}

func (e *Entity) getClientState() ([]*ClientState, error) {
	return e.returnClientState()
}

func (e *Entity) getClientGroupState(groupName string) ([]*ClientState, error) {
	return e.returnClientGroupState(groupName)
}

// We do not want to expose internal structures to app, may be in future all returns would be json serialized
func (e *Entity) getRolloutInfo() (*RolloutState, error) {
	rollout, err := e.findOrCreateRollout()
	if err != nil {
		return nil, err
	}
	return &rollout.State, nil
}
