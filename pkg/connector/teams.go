package connector

import (
	"context"
	"fmt"
	"strconv"

	v2 "github.com/conductorone/baton-sdk/pb/c1/connector/v2"
	"github.com/conductorone/baton-sdk/pkg/annotations"
	"github.com/conductorone/baton-sdk/pkg/connectorbuilder"
	"github.com/conductorone/baton-sdk/pkg/types/entitlement"
	"github.com/conductorone/baton-sdk/pkg/types/grant"
	resourceSdk "github.com/conductorone/baton-sdk/pkg/types/resource"
	"github.com/conductorone/baton-terraform-cloud/pkg/client"
	"github.com/hashicorp/go-tfe"
)

const teamMembership = "member"

var _ connectorbuilder.ResourceSyncerV2 = (*teamBuilder)(nil)

type teamBuilder struct {
	client *client.Client
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
		nil,
		resourceSdk.WithResourceProfile(profile),
		resourceSdk.WithParentResourceID(parentID),
	)
}

func (o *teamBuilder) List(ctx context.Context, parentResourceID *v2.ResourceId, opts resourceSdk.SyncOpAttrs) ([]*v2.Resource, *resourceSdk.SyncOpResults, error) {
	if parentResourceID == nil {
		return nil, nil, nil
	}

	var page int
	var err error
	if opts.PageToken.Token != "" {
		page, err = strconv.Atoi(opts.PageToken.Token)
		if err != nil {
			return nil, nil, fmt.Errorf("baton-terraform-cloud: failed to parse page token: %w", err)
		}
	}

	teams, err := o.client.Teams.List(ctx, parentResourceID.Resource, &tfe.TeamListOptions{
		ListOptions: client.ListOptions(page),
	})

	if err != nil {
		return nil, nil, fmt.Errorf("baton-terraform-cloud: failed to list teams: %w", err)
	}

	rv := []*v2.Resource{}
	for _, team := range teams.Items {
		resource, err := newTeamResource(team, parentResourceID)
		if err != nil {
			return nil, nil, fmt.Errorf("baton-terraform-cloud: failed to create team resource: %w", err)
		}
		rv = append(rv, resource)
	}

	var nextPage string
	if teams.CurrentPage < teams.TotalPages {
		nextPage = strconv.Itoa(teams.CurrentPage + 1)
	}

	return rv, &resourceSdk.SyncOpResults{NextPageToken: nextPage}, nil
}

func (o *teamBuilder) Entitlements(_ context.Context, resource *v2.Resource, _ resourceSdk.SyncOpAttrs) ([]*v2.Entitlement, *resourceSdk.SyncOpResults, error) {
	return nil, nil, nil
}

func (o *teamBuilder) StaticEntitlements(_ context.Context, _ resourceSdk.SyncOpAttrs) ([]*v2.Entitlement, *resourceSdk.SyncOpResults, error) {
	return []*v2.Entitlement{
		entitlement.NewAssignmentEntitlement(
			nil,
			teamMembership,
			entitlement.WithGrantableTo(userResourceType),
			entitlement.WithDescription("Member of team"),
			entitlement.WithDisplayName("Member of team"),
		),
	}, nil, nil
}

func (o *teamBuilder) Grants(ctx context.Context, resource *v2.Resource, opts resourceSdk.SyncOpAttrs) ([]*v2.Grant, *resourceSdk.SyncOpResults, error) {
	users, err := o.client.TeamMembers.List(ctx, resource.Id.Resource)
	if err != nil {
		return nil, nil, fmt.Errorf("baton-terraform-cloud: failed to get team members: %w", err)
	}

	rv := []*v2.Grant{}
	for _, user := range users {
		// skipping non user accounts since there's no way to keep track of them
		if user.IsServiceAccount {
			continue
		}
		principalID, err := resourceSdk.NewResourceID(userResourceType, user.ID)
		if err != nil {
			return nil, nil, fmt.Errorf("baton-terraform-cloud: failed to create resource ID for user %v: %w", user.ID, err)
		}

		rv = append(rv, grant.NewGrant(
			resource,
			teamMembership,
			principalID,
		))
	}

	return rv, nil, nil
}

func (o *teamBuilder) isTeamMember(ctx context.Context, teamID, userID string) (bool, error) {
	members, err := o.client.TeamMembers.List(ctx, teamID)
	if err != nil {
		return false, fmt.Errorf("baton-terraform-cloud: failed to list team members: %w", err)
	}
	for _, m := range members {
		if m.ID == userID {
			return true, nil
		}
	}
	return false, nil
}

func (o *teamBuilder) Grant(ctx context.Context, principal *v2.Resource, entitlement *v2.Entitlement) (annotations.Annotations, error) {
	teamID := entitlement.Resource.Id.Resource
	username := principal.DisplayName

	isMember, err := o.isTeamMember(ctx, teamID, principal.Id.Resource)
	if err != nil {
		return nil, err
	}
	if isMember {
		return annotations.New(&v2.GrantAlreadyExists{}), nil
	}

	err = o.client.TeamMembers.Add(ctx, teamID, tfe.TeamMemberAddOptions{
		Usernames: []string{username},
	})
	if err != nil {
		return nil, fmt.Errorf("baton-terraform-cloud: failed to add user to team: %w", err)
	}

	return nil, nil
}

func (o *teamBuilder) Revoke(ctx context.Context, grant *v2.Grant) (annotations.Annotations, error) {
	entitlement := grant.Entitlement
	teamID := entitlement.Resource.Id.Resource
	username := grant.Principal.DisplayName

	isMember, err := o.isTeamMember(ctx, teamID, grant.Principal.Id.Resource)
	if err != nil {
		return nil, err
	}
	if !isMember {
		return annotations.New(&v2.GrantAlreadyRevoked{}), nil
	}

	err = o.client.TeamMembers.Remove(ctx, teamID, tfe.TeamMemberRemoveOptions{
		Usernames: []string{username},
	})
	if err != nil {
		return nil, fmt.Errorf("baton-terraform-cloud: failed to remove user from team: %w", err)
	}
	return nil, nil
}

func newTeamBuilder(client *client.Client) *teamBuilder {
	return &teamBuilder{
		client: client,
	}
}
