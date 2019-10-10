package k8splugins

import (
	"fmt"
	"github.com/lyft/flyteplugins/go/tasks/v1/types"
	"bytes"
)

const statusKey = "ObjectStatus"
const terminalTaskPhaseKey = "TerminalTaskPhase"

// This status internal state of the object not read/updated by upstream components (eg. Node manager)
type K8sObjectStatus int

const (
	K8sObjectUnknown K8sObjectStatus = iota
	K8sObjectExists
	K8sObjectDeleted
)

func (q K8sObjectStatus) String() string {
	switch q {
	case K8sObjectUnknown:
		return "NotStarted"
	case K8sObjectExists:
		return "Running"
	case K8sObjectDeleted:
		return "Deleted"
	}
	return "IllegalK8sObjectStatus"
}

func RetrieveK8sObjectStatus(customState map[string]interface{}) (K8sObjectStatus, types.TaskPhase, error) {
	if customState == nil {
		return K8sObjectUnknown, types.TaskPhaseUnknown, nil
	}

	status := K8sObjectUnknown
	terminalTaskPhase := types.TaskPhaseUnknown
	foundStatus := false
	foundPhase := false
	for k, v := range customState {
		if k == statusKey {
			status, foundStatus = v.(K8sObjectStatus)
		} else if k == terminalTaskPhaseKey {
			terminalTaskPhase, foundPhase = v.(types.TaskPhase)
		}
	}

	if !(foundPhase && foundStatus) {
		return K8sObjectUnknown, types.TaskPhaseUnknown, fmt.Errorf("invalid custom state %v", mapToString(customState))
	}

	return status, terminalTaskPhase, nil
}

func StoreK8sObjectStatus(status K8sObjectStatus, phase types.TaskPhase) map[string]interface{} {
	customState := make(map[string]interface{})
	customState[statusKey] = status
	customState[terminalTaskPhaseKey] = phase
	return customState
}

func mapToString(m map[string]interface{}) string {
	b := new(bytes.Buffer)
	for key, value := range m {
		fmt.Fprintf(b, "%s=\"%v\"\n", key, value)
	}
	return b.String()
}