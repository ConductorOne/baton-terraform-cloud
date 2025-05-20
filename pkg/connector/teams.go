package connector

import (
	"context"
	"fmt"
	"strconv"
	"sync"

	v2 "github.com/conductorone/baton-sdk/pb/c1/connector/v2"
	"github.com/conductorone/baton-sdk/pkg/annotations"
	"github.com/conductorone/baton-sdk/pkg/pagination"
	"github.com/conductorone/baton-sdk/pkg/types/entitlement"
	"github.com/conductorone/baton-sdk/pkg/types/grant"
	resourceSdk "github.com/conductorone/baton-sdk/pkg/types/resource"
	"github.com/conductorone/baton-terraform-cloud/pkg/client"
	"github.com/hashicorp/go-tfe"
)

const teamMembership = "member"

type teamBuilder struct {
	client      *client.Client
	m           *sync.Mutex
	teamMembers map[string][]*tfe.User
}

func (o *teamBuilder) cacheTeamMembers(teams *tfe.TeamList) {
	o.m.Lock()
	defer o.m.Unlock()
	for _, team := range teams.Items {
		o.teamMembers[team.ID] = team.Users
	}
}

func (o *teamBuilder) getTeamMembers(ctx context.Context, teamID string) ([]*tfe.User, error) {
	o.m.Lock()
	defer o.m.Unlock()

	users, ok := o.teamMembers[teamID]
	if ok {
		return users, nil
	}

	team, err := o.client.Teams.Read(ctx, teamID)
	if err != nil {
		return nil, err
	}

	return team.Users, nil
}

func (o *teamBuilder) ResourceType(ctx context.Context) *v2.ResourceType {
	return teamResourceType
}

func newTeamResource(team *tfe.Team, parentID *v2.ResourceId) (*v2.Resource, error) {
	profile := map[string]interface{}{
		"visibility": team.Visibility,
		"userCount":  team.UserCount,
		"isUnified":  team.IsUnified,
	}

	return resourceSdk.NewGroupResource(
		team.Name,
		teamResourceType,
		team.ID,
		[]resourceSdk.GroupTraitOption{
			resourceSdk.WithGroupProfile(profile),
		},
		resourceSdk.WithParentResourceID(parentID),
	)
}

func (o *teamBuilder) List(ctx context.Context, parentResourceID *v2.ResourceId, pToken *pagination.Token) ([]*v2.Resource, string, annotations.Annotations, error) {
	if parentResourceID == nil {
		return nil, "", nil, nil
	}

	var page int
	var err error
	if pToken.Token != "" {
		page, err = strconv.Atoi(pToken.Token)
		if err != nil {
			return nil, "", nil, fmt.Errorf("baton-terraform-cloud: failed to parse page token: %w", err)
		}
	}

	teams, err := o.client.Teams.List(ctx, parentResourceID.Resource, &tfe.TeamListOptions{
		ListOptions: client.ListOptions(page),
		Include: []tfe.TeamIncludeOpt{
			"users",
		},
	})

	o.cacheTeamMembers(teams)

	if err != nil {
		return nil, "", nil, fmt.Errorf("baton-terraform-cloud: failed to list teams: %w", err)
	}
	if len(teams.Items) == 0 {
		return nil, "", nil, nil
	}

	rv := []*v2.Resource{}
	for _, team := range teams.Items {
		resource, err := newTeamResource(team, parentResourceID)
		if err != nil {
			return nil, "", nil, fmt.Errorf("baton-terraform-cloud: failed to create team resource: %w", err)
		}
		rv = append(rv, resource)
	}

	var nextPage string
	if teams.CurrentPage < teams.TotalPages {
		nextPage = strconv.Itoa(page + 1)
	}

	return rv, nextPage, nil, nil
}

func (o *teamBuilder) Entitlements(_ context.Context, resource *v2.Resource, _ *pagination.Token) ([]*v2.Entitlement, string, annotations.Annotations, error) {
	return []*v2.Entitlement{
		entitlement.NewAssignmentEntitlement(
			resource,
			teamMembership,
			entitlement.WithGrantableTo(userResourceType),
			entitlement.WithDescription(fmt.Sprintf("Member of %s team", resource.DisplayName)),
			entitlement.WithDisplayName(fmt.Sprintf("Member of %s team", resource.DisplayName)),
		),
	}, "", nil, nil
}

func (o *teamBuilder) Grants(ctx context.Context, resource *v2.Resource, pToken *pagination.Token) ([]*v2.Grant, string, annotations.Annotations, error) {
	users, err := o.getTeamMembers(ctx, resource.Id.Resource)
	if err != nil {
		return nil, "", nil, fmt.Errorf("baton-terraform-cloud: failed to get team members: %w", err)
	}

	rv := []*v2.Grant{}
	for _, user := range users {
		// skipping non user accounts since there's no way to keep track of them
		if user.IsServiceAccount {
			continue
		}
		principalID, err := resourceSdk.NewResourceID(userResourceType, user.ID)
		if err != nil {
			return nil, "", nil, fmt.Errorf("baton-terraform-cloud: failed to create resource ID for user %v: %w", user.ID, err)
		}

		rv = append(rv, grant.NewGrant(
			resource,
			teamMembership,
			principalID,
		))
	}

	return rv, "", nil, nil
}

func (o *teamBuilder) Grant(ctx context.Context, principal *v2.Resource, entitlement *v2.Entitlement) (annotations.Annotations, error) {
	teamID := entitlement.Resource.Id.Resource

	err := o.client.TeamMembers.Add(ctx, teamID, tfe.TeamMemberAddOptions{
		Usernames: []string{principal.DisplayName},
	})
	if err != nil {
		return nil, fmt.Errorf("baton-terraform-cloud: failed to add user to team: %w", err)
	}

	return nil, nil
}

func (o *teamBuilder) Revoke(ctx context.Context, grant *v2.Grant) (annotations.Annotations, error) {
	entitlement := grant.Entitlement
	teamID := entitlement.Resource.Id.Resource

	err := o.client.TeamMembers.Remove(ctx, teamID, tfe.TeamMemberRemoveOptions{
		Usernames: []string{grant.Principal.DisplayName},
	})
	if err != nil {
		return nil, fmt.Errorf("baton-terraform-cloud: failed to remove user from team: %w", err)
	}
	return nil, nil
}

func newTeamBuilder(client *client.Client) *teamBuilder {
	return &teamBuilder{
		client:      client,
		m:           &sync.Mutex{},
		teamMembers: make(map[string][]*tfe.User),
	}
}
