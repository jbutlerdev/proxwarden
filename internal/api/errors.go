package api

import "fmt"

// ContainerError represents errors related to container operations
type ContainerError struct {
	Message     string
	ContainerID int
}

func (e *ContainerError) Error() string {
	if e.ContainerID > 0 {
		return fmt.Sprintf("container %d: %s", e.ContainerID, e.Message)
	}
	return e.Message
}

// Common error instances
var (
	ErrContainerNotFound = &ContainerError{Message: "container not found"}
	ErrNodeNotFound      = &ContainerError{Message: "node not found"}
	ErrMigrationFailed   = &ContainerError{Message: "migration failed"}
)