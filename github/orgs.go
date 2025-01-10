package github

import (
	"context"

	"github.com/google/go-github/v68/github"
)

type Org struct {
	Name, Login, Url, Company, AvatarURL string
	Id                                   int64
}

func (i Org) Title() string       { return i.Login }
func (i Org) Description() string { return i.AvatarURL }
func (i Org) FilterValue() string { return i.Login }

func (g *Github) ListOrgsForUser() ([]Org, error) {
	client, err := g.New()
	if err != nil {
		return nil, err
	}

	opt := &github.ListOptions{}
	orgs, _, err := client.Organizations.List(context.Background(), "", opt)
	if err != nil {
		return nil, err
	}

	var orgItems []Org

	for i := 0; i < len(orgs); i++ {
		orgItems = append(orgItems, Org{
			Name:      orgs[i].GetName(),
			Id:        orgs[i].GetID(),
			Login:     orgs[i].GetLogin(),
			Url:       orgs[i].GetHTMLURL(),
			Company:   orgs[i].GetCompany(),
			AvatarURL: orgs[i].GetAvatarURL(),
		})
	}

	return orgItems, nil
}
