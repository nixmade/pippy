package core

import (
	"time"
)

// ClientState as reported by clients,
//
//		transformed to EntityTarget
//	 Orchestrator updates with target state
//	 Client could belong to a specific group making it easier to orchestrate
type ClientState struct {
	Name    string `json:"name,omitempty"`
	Group   string `json:"group,omitempty"`
	Tags    string `json:"tags,omitempty"`
	Version string `json:"version,omitempty"`
	Message string `json:"message,omitempty"`
	IsError bool   `json:"isError,omitempty"`
}

// Message reported for each target
type Message struct {
	Message   string    `json:"message,omitempty"`
	Timestamp time.Time `json:"timestamp,omitempty"`
	IsError   bool      `json:"isError,omitempty"`
}

// EntityTargetVersion used as an input, otherwise unused anywhere else
type EntityTargetVersion struct {
	Version string `json:"version,omitempty"`
}

// EntityVersionInfo contains version information
type EntityVersionInfo struct {
	Version string `json:"version,omitempty"`
	// changeTimestamp at which this version was created
	// typically this is the time the version was switched
	ChangeTimestamp time.Time `json:"changetimestamp,omitempty"`
	// last message
	LastMessage Message `json:"lastmessage,omitempty"`
}

// EntityTargetState and list of properties stored internally
type EntityTargetState struct {
	CurrentVersion       EntityVersionInfo `json:"currentversion,omitempty"`
	TargetVersion        EntityVersionInfo `json:"targetversion,omitempty"`
	LastUpdatedTimestamp time.Time         `json:"lastupdatedtimestamp,omitempty"`
}

// EntityTarget contains Entity name, and any properties,
// to uniquely identify a target
type EntityTarget struct {
	Name  string            `json:"name,omitempty"`
	Group string            `json:"group,omitempty"`
	Tags  string            `json:"tags,omitempty"`
	State EntityTargetState `json:"state,omitempty"`
}

type EntityTargets = []*EntityTarget

func (m *Message) Success(message string) {
	m.Message = message
	m.Timestamp = time.Now().UTC()
	m.IsError = false
}

func (m *Message) Error(message string) {
	m.Message = message
	m.Timestamp = time.Now().UTC()
	m.IsError = true
}
