package github

import (
	"context"

	"github.com/google/go-github/v66/github"
)

type Repo struct {
	Name, Url, Detail string
}

func (i Repo) Title() string       { return i.Name }
func (i Repo) Description() string { return i.Detail }
func (i Repo) FilterValue() string { return i.Name }

func (g *Github) ListRepos(repoType string) ([]Repo, error) {
	client, err := g.New()
	if err != nil {
		return nil, err
	}

	opt := &github.RepositoryListByAuthenticatedUserOptions{Type: repoType}
	repos, _, err := client.Repositories.ListByAuthenticatedUser(context.Background(), opt)
	if err != nil {
		return nil, err
	}

	var repoItems []Repo

	for i := 0; i < len(repos); i++ {
		repoItems = append(repoItems, Repo{
			Name:   repos[i].GetFullName(),
			Url:    repos[i].GetHTMLURL(),
			Detail: repos[i].GetDescription(),
		})
	}

	return repoItems, nil
}
