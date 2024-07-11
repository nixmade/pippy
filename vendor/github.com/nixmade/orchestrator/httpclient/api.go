package httpclient

import "fmt"

type API struct {
	endpoint string
	version  string
	resource string
}

func NewAPI(endpoint, version, resource string) *API {
	return &API{endpoint: endpoint, version: version, resource: resource}
}

func (api *API) URL() string {
	return fmt.Sprintf("%s/%s/%s", api.endpoint, api.version, api.resource)
}

type OrchestratorAPI struct {
	*API
}

func NewOrchestratorAPI(endpoint string) *OrchestratorAPI {
	return &OrchestratorAPI{API: NewAPI(endpoint, "v1", "orchestrate")}
}

func (api *OrchestratorAPI) Orchestrate(namespace, entity string) string {
	return fmt.Sprintf("%s/%s/%s", api.URL(), namespace, entity)
}

func (api *OrchestratorAPI) TargetVersion(namespace, entity string) string {
	return fmt.Sprintf("%s/%s/%s/version", api.URL(), namespace, entity)
}

func (api *OrchestratorAPI) RolloutOptions(namespace, entity string) string {
	return fmt.Sprintf("%s/%s/%s/options", api.URL(), namespace, entity)
}

func (api *OrchestratorAPI) EntityTargetController(namespace, entity string) string {
	return fmt.Sprintf("%s/%s/%s/target/controller", api.URL(), namespace, entity)
}

func (api *OrchestratorAPI) EntityMonitoringController(namespace, entity string) string {
	return fmt.Sprintf("%s/%s/%s/monitoring/controller", api.URL(), namespace, entity)
}

func (api *OrchestratorAPI) Status(namespace, entity string) string {
	return fmt.Sprintf("%s/%s/%s/status", api.URL(), namespace, entity)
}

func (api *OrchestratorAPI) Namespaces() string {
	return fmt.Sprintf("%s/namespaces", api.URL())
}

func (api *OrchestratorAPI) Entities(namespace string) string {
	return fmt.Sprintf("%s/%s/entities", api.URL(), namespace)
}

func (api *OrchestratorAPI) RolloutInfo(namespace, entity string) string {
	return fmt.Sprintf("%s/%s/%s/rollout", api.URL(), namespace, entity)
}

func (api *OrchestratorAPI) Targets(namespace, entity string) string {
	return fmt.Sprintf("%s/%s/%s/targets", api.URL(), namespace, entity)
}

func (api *OrchestratorAPI) GroupStatus(namespace, entity, group string) string {
	return fmt.Sprintf("%s/%s/%s/%s/status", api.URL(), namespace, entity, group)
}
