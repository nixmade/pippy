package core

import "errors"

var (
	// ErrNamespaceNotCreated returns an error if namespace is not created
	ErrNamespaceNotCreated = errors.New("namespace not created")
	// ErrEntityNotCreated returns an error if entity is not created
	ErrEntityNotCreated = errors.New("entity not created")
	// ErrInvalidTargetVersion returns an error if target version is invalid
	ErrInvalidTargetVersion = errors.New("invalid Target Version")
	// ErrExternalControllerFailure returns an error if call to external controller failed
	ErrExternalControllerFailure = errors.New("failure calling external controller")
)
