package github

import (
	"context"
	"encoding/base64"
	"fmt"
	"net/http"

	"github.com/bradleyfalzon/ghinstallation/v2"
	"github.com/google/go-github/v71/github"
	"github.com/nixmade/pippy/users"
)

type contextKey struct {
	name string
}

func (k *contextKey) String() string {
	return "context value " + k.name
}

var (
	AccessTokenCtx    = &contextKey{"AccessToken"}
	PrivateKeyCtx     = &contextKey{"PrivateKey"}
	AppIDCtx          = &contextKey{"AppID"}
	InstallationIDCtx = &contextKey{"InstallationID"}
)

type Client interface {
	ListRepos(repoType string) ([]Repo, error)
	GetWorkflow(org, repo string, id int64) (*Workflow, error)
	ListWorkflows(org, repo string) ([]Workflow, error)
	ListWorkflowRuns(org, repo string, workflowID int64, created string) ([]WorkflowRun, error)
	CreateWorkflowDispatch(org, repo string, workflowID int64, ref string, inputs map[string]interface{}) error
	ValidateWorkflow(org, repo, path string) ([]string, map[string]string, error)
	ValidateWorkflowFull(org, repo, path string) (string, string, error)
	ListOrgsForUser() ([]Org, error)
}

type Github struct {
	context.Context
}

func getAccessToken(ctx context.Context) (accessToken string, err error) {
	accessTokenCtx := ctx.Value(AccessTokenCtx)
	if accessTokenCtx != nil {
		return accessTokenCtx.(string), nil
	}

	return users.GetCachedAccessToken()
}

func (g *Github) New() (*github.Client, error) {
	privateKeyCtx := g.Context.Value(PrivateKeyCtx)
	if privateKeyCtx != nil {
		privateKey, err := base64.StdEncoding.DecodeString(privateKeyCtx.(string))
		if err != nil {
			return nil, err
		}
		appIDCtx := g.Context.Value(AppIDCtx)
		if appIDCtx == nil {
			return nil, fmt.Errorf("app id is not provided or empty")
		}
		installationIDCtx := g.Context.Value(InstallationIDCtx)
		if installationIDCtx == nil {
			return nil, fmt.Errorf("installation id is not provided or empty")
		}
		tr, err := ghinstallation.New(http.DefaultTransport, appIDCtx.(int64), installationIDCtx.(int64), privateKey)
		if err != nil {
			return nil, err
		}
		return github.NewClient(&http.Client{Transport: tr}), nil
	}

	accessToken, err := getAccessToken(g.Context)
	if err != nil {
		return nil, err
	}
	return github.NewClient(nil).WithAuthToken(accessToken), nil
}

var (
	DefaultClient Client = &Github{Context: context.Background()}
)
