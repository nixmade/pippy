package core

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/nixmade/orchestrator/httpclient"
)

// EntityWebController defines set of callback functions wrapper around RolloutController and could be more
// Right now its just a string to store the web endpoint, later could have auth and something else
type EntityWebTargetController struct {
	// SelectionEndpoint notifies for selecting targets based on batchsize
	// It could return new set of targets, typical pattern of blue-green deployments
	SelectionEndpoint string `json:"selection,omitempty"`

	// ApprovalEndpoint notifies external controller just before rolling out
	//	At this stage external controller could reject it
	ApprovalEndpoint string `json:"approval,omitempty"`

	// RemovalEndpoint notifies external controller to remove set of old targets
	// Sends old sets of target with size of new successful rolled out targets
	// Sends new set of target with size of new successful rolled out targets
	// All of the above is set to max batch size
	RemovalEndpoint string `json:"removal,omitempty"`

	// MonitoringEndpoint checks for any additional monitoring for individual target
	//	typically health information is included in messages, but this provides another opportunity
	MonitoringEndpoint string `json:"monitoring,omitempty"`
}

type EntityWebMonitoringController struct {
	// ExternalMonitoringEndpoint communicates with external monitoring system,
	// 	in case rollout has degraded the system as a whole
	ExternalMonitoringEndpoint string `json:"externalmonitoring,omitempty"`
}

// TargetSelectionRequest request of target list with count
type TargetSelectionRequest struct {
	Targets []*ClientState `json:"targets,omitempty"`
	Count   int            `json:"count,omitempty"`
}

// TargetSelectionResponse response of target list
type TargetSelectionResponse struct {
	Targets []*ClientState `json:"targets,omitempty"`
}

// TargetApprovalRequest request with target list
type TargetApprovalRequest struct {
	Targets []*ClientState `json:"targets,omitempty"`
}

// TargetApprovalResponse response with list of targets
type TargetApprovalResponse struct {
	Targets []*ClientState `json:"targets,omitempty"`
}

// TargetMonitoringRequest request with target name
type TargetMonitoringRequest struct {
	Target *ClientState `json:"target,omitempty"`
}

// TargetMonitoringResponse response with target status
type TargetMonitoringResponse struct {
	Status  string `json:"status,omitempty"`
	Message string `json:"message,omitempty"`
}

// TargetRemovalRequest request of target list with count
type TargetRemovalRequest struct {
	Targets []*ClientState `json:"targets,omitempty"`
	Count   int            `json:"count,omitempty"`
}

// TargetRemovalResponse response of target list
type TargetRemovalResponse struct {
	Targets []*ClientState `json:"targets,omitempty"`
}

// ExternalMonitoringRequest request for external monitoring
type ExternalMonitoringRequest struct {
	Targets []*ClientState `json:"targets,omitempty"`
}

// ExternalMonitoringResponse Response for external monitoring
type ExternalMonitoringResponse struct {
	Status  string `json:"status,omitempty"`
	Message string `json:"message,omitempty"`
}

// TargetSelection selects list of targets
func (e *EntityWebTargetController) TargetSelection(clientTargets []*ClientState, numSelection int) ([]*ClientState, error) {
	if e.SelectionEndpoint == "" {
		return clientTargets, nil
	}

	var trResponse TargetSelectionResponse

	if err := httpclient.PostJSON(e.SelectionEndpoint, "", TargetSelectionRequest{Targets: clientTargets, Count: numSelection}, &trResponse); err != nil {
		return nil, err
	}

	return trResponse.Targets, nil
}

// TargetApproval gets approval for list of targets
func (e *EntityWebTargetController) TargetApproval(clientTargets []*ClientState) ([]*ClientState, error) {
	if e.ApprovalEndpoint == "" {
		return clientTargets, nil
	}

	postBuf, err := json.Marshal(TargetApprovalRequest{Targets: clientTargets})
	if err != nil {
		return nil, err
	}

	respBody, err := makeRequest(e.ApprovalEndpoint, postBuf)
	if err != nil {
		return nil, err
	}
	defer respBody.Close()

	var trResponse TargetApprovalResponse
	if err := json.NewDecoder(respBody).Decode(&trResponse); err != nil {
		return nil, err
	}

	var approvedTargets []*ClientState

	for _, targetName := range trResponse.Targets {
		for _, clientTarget := range clientTargets {
			if clientTarget.Name == targetName.Name {
				// only known targets can get approved
				approvedTargets = append(approvedTargets, clientTarget)
			}
		}
	}

	return approvedTargets, nil
}

// TargetMonitoring monitors the provided target
func (e *EntityWebTargetController) TargetMonitoring(clientTarget *ClientState) error {
	if e.MonitoringEndpoint == "" {
		return nil
	}

	postBuf, err := json.Marshal(TargetMonitoringRequest{Target: clientTarget})
	if err != nil {
		return err
	}

	respBody, err := makeRequest(e.MonitoringEndpoint, postBuf)
	if err != nil {
		return err
	}
	defer respBody.Close()

	var trResponse TargetMonitoringResponse
	if err := json.NewDecoder(respBody).Decode(&trResponse); err != nil {
		return err
	}

	if strings.ToLower(trResponse.Status) != "ok" {
		return fmt.Errorf("%s %s", trResponse.Status, trResponse.Message)
	}

	return nil
}

// TargetRemoval removes optional set of additional targets
func (e *EntityWebTargetController) TargetRemoval(clientTargets []*ClientState, numRemoval int) ([]*ClientState, error) {

	if e.RemovalEndpoint == "" {
		return nil, nil
	}

	postBuf, err := json.Marshal(TargetRemovalRequest{Targets: clientTargets, Count: numRemoval})
	if err != nil {
		return nil, err
	}

	respBody, err := makeRequest(e.RemovalEndpoint, postBuf)
	if err != nil {
		return nil, err
	}
	defer respBody.Close()

	var trResponse TargetRemovalResponse
	if err := json.NewDecoder(respBody).Decode(&trResponse); err != nil {
		return nil, err
	}

	var removedTargets []*ClientState

	for _, targetName := range trResponse.Targets {
		for _, clientTarget := range clientTargets {
			if clientTarget.Name == targetName.Name {
				// only known targets can get approved
				removedTargets = append(removedTargets, clientTarget)
			}
		}
	}

	return removedTargets, nil
}

// ExternalMonitoring for list of client targets
func (e *EntityWebMonitoringController) ExternalMonitoring(clientTargets []*ClientState) error {

	if e.ExternalMonitoringEndpoint == "" {
		return nil
	}

	postBuf, err := json.Marshal(ExternalMonitoringRequest{Targets: clientTargets})
	if err != nil {
		return err
	}

	respBody, err := makeRequest(e.ExternalMonitoringEndpoint, postBuf)
	if err != nil {
		return err
	}
	defer respBody.Close()

	var extResponse ExternalMonitoringResponse
	if err := json.NewDecoder(respBody).Decode(&extResponse); err != nil {
		return err
	}

	if strings.ToLower(extResponse.Status) != "ok" {
		return fmt.Errorf("%s %s", extResponse.Status, extResponse.Message)
	}

	return nil
}

func makeRequest(req string, postBuf []byte) (io.ReadCloser, error) {
	resp, err := http.Post(req, "application/json", bytes.NewBuffer(postBuf))

	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		return nil, ErrExternalControllerFailure
	}

	return resp.Body, err
}
