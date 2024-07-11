package core

import (
	"encoding/json"
	"reflect"
)

// EntityController defines set of callback functions wrapper around RolloutController and could be more
type EntityTargetController interface {
	// TargetSelection notifies for selecting targets based on batchsize
	// It could return new set of targets, typical pattern of blue-green deployments
	TargetSelection([]*ClientState, int) ([]*ClientState, error)

	// TargetApproval notifies external controller just before rolling out
	//	At this stage external controller could reject it
	TargetApproval([]*ClientState) ([]*ClientState, error)

	// TargetRemoval notifies external controller to remove set of old targets
	// Sends old sets of target with size of new successful rolled out targets
	// Sends new set of target with size of new successful rolled out targets
	// All of the above is set to max batch size
	TargetRemoval([]*ClientState, int) ([]*ClientState, error)

	// TargetMonitoring checks for any additional monitoring for individual target
	//	typically health information is included in messages, but this provides another opportunity
	TargetMonitoring(*ClientState) error
}

type EntityMonitoringController interface {
	// ExternalMonitoring communicates with external monitoring system,
	// 	in case rollout has degraded the system as a whole
	ExternalMonitoring([]*ClientState) error
}

// We need to register all known message types here to be able to unmarshal them to the correct interface type.
var RegisteredTargetControllers = []EntityTargetController{
	&NoOpEntityTargetController{},
	&EntityWebTargetController{},
}

var RegisteredMonitoringControllers = []EntityMonitoringController{
	&NoOpEntityMonitoringController{},
	&EntityWebMonitoringController{},
}

type SerializedEntityTargetController struct {
	EntityTargetController `json:"value"`
}

type SerializedEntityMonitoringController struct {
	EntityMonitoringController `json:"value"`
}

func (c *SerializedEntityTargetController) UnmarshalJSON(bytes []byte) error {
	var data struct {
		Type  interface{}
		Value json.RawMessage
	}
	if err := json.Unmarshal(bytes, &data); err != nil {
		return err
	}

	for _, registeredController := range RegisteredTargetControllers {
		registeredControllerType := reflect.TypeOf(registeredController)
		if registeredControllerType.String() == data.Type {
			// Create a new pointer to a value of the concrete message type
			target := reflect.New(registeredControllerType)
			// Unmarshal the data to an interface to the concrete value (which will act as a pointer, don't ask why)
			if err := json.Unmarshal(data.Value, target.Interface()); err != nil {
				return err
			}
			// Now we get the element value of the target and convert it to the interface type (this is to get rid of a pointer type instead of a plain struct value)
			c.EntityTargetController = target.Elem().Interface().(EntityTargetController)
			return nil
		}
	}
	return nil
}

func (c SerializedEntityTargetController) MarshalJSON() ([]byte, error) {
	// Marshal to type and actual data to handle unmarshaling to specific interface type
	return json.Marshal(struct {
		Type  interface{}
		Value any
	}{
		Type:  reflect.TypeOf(c.EntityTargetController).String(),
		Value: c.EntityTargetController,
	})
}

func (c *SerializedEntityMonitoringController) UnmarshalJSON(bytes []byte) error {
	var data struct {
		Type  interface{}
		Value json.RawMessage
	}
	if err := json.Unmarshal(bytes, &data); err != nil {
		return err
	}

	for _, registeredController := range RegisteredMonitoringControllers {
		registeredControllerType := reflect.TypeOf(registeredController)
		if registeredControllerType.String() == data.Type {
			// Create a new pointer to a value of the concrete message type
			target := reflect.New(registeredControllerType)
			// Unmarshal the data to an interface to the concrete value (which will act as a pointer, don't ask why)
			if err := json.Unmarshal(data.Value, target.Interface()); err != nil {
				return err
			}
			// Now we get the element value of the target and convert it to the interface type (this is to get rid of a pointer type instead of a plain struct value)
			c.EntityMonitoringController = target.Elem().Interface().(EntityMonitoringController)
			return nil
		}
	}
	return nil
}

func (c SerializedEntityMonitoringController) MarshalJSON() ([]byte, error) {
	// Marshal to type and actual data to handle unmarshaling to specific interface type
	return json.Marshal(struct {
		Type  interface{}
		Value any
	}{
		Type:  reflect.TypeOf(c.EntityMonitoringController).String(),
		Value: c.EntityMonitoringController,
	})
}

// NoOpEntityController is a dummy default controller
type NoOpEntityTargetController struct {
}

type NoOpEntityMonitoringController struct {
}

// TargetSelection selects everything
func (e *NoOpEntityTargetController) TargetSelection(t []*ClientState, s int) ([]*ClientState, error) {
	return t, nil
}

// TargetApproval approves everything
func (e *NoOpEntityTargetController) TargetApproval(t []*ClientState) ([]*ClientState, error) {
	return t, nil
}

// TargetMonitoring returns success always
func (e *NoOpEntityTargetController) TargetMonitoring(t *ClientState) error {
	return nil
}

// TargetRemoval removes nothing
func (e *NoOpEntityTargetController) TargetRemoval(t []*ClientState, s int) ([]*ClientState, error) {
	return nil, nil
}

// ExternalMonitoring return success always
func (e *NoOpEntityMonitoringController) ExternalMonitoring([]*ClientState) error {
	return nil
}

func getClientTarget(entityTarget *EntityTarget) *ClientState {
	return &ClientState{
		Name:    entityTarget.Name,
		Group:   entityTarget.Group,
		Version: entityTarget.State.TargetVersion.Version,
		Message: entityTarget.State.TargetVersion.LastMessage.Message,
		IsError: entityTarget.State.TargetVersion.LastMessage.IsError,
	}
}

func getClientTargets(entityTargets EntityTargets) (clientTargets []*ClientState) {
	for _, entityTarget := range entityTargets {
		clientTargets = append(clientTargets, getClientTarget(entityTarget))
	}
	return
}
