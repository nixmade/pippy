package core

import (
	"fmt"

	"github.com/nixmade/orchestrator/store"
	"github.com/rs/zerolog"
)

const (
	entityPrefix = "entity:"
)

// Namespace holds the list of entities
type Namespace struct {
	// list of entities
	Name   string         `json:"name,omitempty"`
	store  store.Store    `json:"-"`
	logger zerolog.Logger `json:"-"`
}

// CreateNamespace creates namespace
func (e *Engine) createNamespace(name string) (*Namespace, error) {
	e.logger.Info().Str("Namespace", name).Msg("Creating new namespace")

	n := &Namespace{
		Name:   name,
		logger: e.logger.With().Str("Namespace", name).Logger(),
		store:  e.store,
	}

	return n, e.store.SaveJSON(namespaceKey(name), n)
}

func (n *Namespace) entityKey(name string) string {
	return fmt.Sprintf("%s%s/%s", entityPrefix, n.Name, name)
}

// SaveEntity saves entity to store
func (n *Namespace) SaveEntity(entityName string) error {
	_, err := n.findorCreateEntity(entityName)

	return err
}

func (n *Namespace) findEntity(name string) (*Entity, error) {
	entity := &Entity{}

	if err := n.store.LoadJSON(n.entityKey(name), entity); err != nil {
		return nil, err
	}

	entity.store = n.store
	entity.logger = n.logger.With().Str("Entity", name).Logger()

	return entity, nil
}

func (n *Namespace) findorCreateEntity(name string) (*Entity, error) {
	entity, err := n.findEntity(name)
	if err == store.ErrKeyNotFound {
		return n.createEntity(name)
	}

	if err != nil {
		return nil, err
	}

	return entity, nil
}

// orchestrate provided entityName over list of targets, updates targetVersion
func (n *Namespace) orchestrate(entityName string, targets []*ClientState) ([]*ClientState, error) {
	entity, err := n.findorCreateEntity(entityName)
	if err != nil {
		return nil, err
	}
	return entity.orchestrate(targets)
}

// orchestrateasync records list of input targets
func (n *Namespace) orchestrateasync(entityName string, targets []*ClientState) error {
	entity, err := n.findorCreateEntity(entityName)
	if err != nil {
		return err
	}
	return entity.orchestrateasync(targets)
}

// getClientState provided entityName, returns current target state
func (n *Namespace) getClientState(entityName string) ([]*ClientState, error) {
	entity, err := n.findEntity(entityName)
	if err != nil {
		return nil, err
	}
	return entity.getClientState()
}

// getClientGroupState provided entityName, returns current target state
func (n *Namespace) getClientGroupState(entityName, groupName string) ([]*ClientState, error) {
	entity, err := n.findEntity(entityName)
	if err != nil {
		return nil, err
	}
	return entity.getClientGroupState(groupName)
}

// Below are supporting case for front end API

// getEntities gets list of entities owned by this namespace
func (n *Namespace) getEntities() ([]string, error) {
	entities, err := n.store.LoadKeys(entityPrefix)

	if err != nil {
		return nil, err
	}

	return entities, nil
}

// getRolloutInfo gets current rollout information for the entity
func (n *Namespace) getRolloutInfo(entityName string) (*RolloutState, error) {
	entity, err := n.findEntity(entityName)
	if err != nil {
		return nil, err
	}
	return entity.getRolloutInfo()
}
