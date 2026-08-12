package connector

import (
	"context"
	"fmt"
	"strconv"

	v2 "github.com/conductorone/baton-sdk/pb/c1/connector/v2"
	"github.com/conductorone/baton-sdk/pkg/connectorbuilder"
	resourceSdk "github.com/conductorone/baton-sdk/pkg/types/resource"
	"github.com/conductorone/baton-terraform-cloud/pkg/client"
	"github.com/hashicorp/go-tfe"
)

var _ connectorbuilder.ResourceSyncerV2 = (*agentTokenBuilder)(nil)

type agentTokenBuilder struct {
	client *client.Client
}

func (o *agentTokenBuilder) ResourceType(ctx context.Context) *v2.ResourceType {
	return agentTokenResourceType
}

func newAgentTokenResource(agentToken *tfe.AgentToken, parentID *v2.ResourceId) (*v2.Resource, error) {
	return resourceSdk.NewSecretResource(
		agentToken.Description,
		agentTokenResourceType,
		agentToken.ID,
		[]resourceSdk.SecretTraitOption{
			resourceSdk.WithSecretLastUsedAt(agentToken.LastUsedAt),
		},
		resourceSdk.WithResourceCreatedAt(agentToken.CreatedAt),
		resourceSdk.WithParentResourceID(parentID),
	)
}

// List returns all the agentTokens from the database as resource objects.
// AgentTokens include a AgentTokenTrait because they are the 'shape' of a standard agentToken.
func (o *agentTokenBuilder) List(ctx context.Context, parentResourceID *v2.ResourceId, opts resourceSdk.SyncOpAttrs) ([]*v2.Resource, *resourceSdk.SyncOpResults, error) {
	if parentResourceID == nil {
		return nil, &resourceSdk.SyncOpResults{}, nil
	}

	var page int
	var err error
	if opts.PageToken.Token != "" {
		page, err = strconv.Atoi(opts.PageToken.Token)
		if err != nil {
			return nil, nil, fmt.Errorf("baton-terraform-cloud: failed to parse page token: %w", err)
		}
	}

	agentPools, err := o.client.AgentPools.List(ctx, parentResourceID.Resource, &tfe.AgentPoolListOptions{
		ListOptions: client.ListOptions(page),
	})

	if err != nil {
		return nil, nil, fmt.Errorf("baton-terraform-cloud: failed to list agent pools: %w", err)
	}

	if len(agentPools.Items) == 0 {
		return nil, &resourceSdk.SyncOpResults{}, nil
	}

	rv := []*v2.Resource{}
	for _, pool := range agentPools.Items {
		agentTokens, err := o.client.AgentTokens.List(ctx, pool.ID)
		if err != nil {
			return nil, nil, fmt.Errorf("baton-terraform-cloud: failed to list agentTokens: %w", err)
		}
		for _, agentToken := range agentTokens.Items {
			resource, err := newAgentTokenResource(agentToken, parentResourceID)
			if err != nil {
				return nil, nil, fmt.Errorf("baton-terraform-cloud: failed to create agentToken resource: %w", err)
			}

			rv = append(rv, resource)
		}
	}

	var nextPage string
	if agentPools.CurrentPage < agentPools.TotalPages {
		nextPage = strconv.Itoa(agentPools.CurrentPage + 1)
	}

	return rv, &resourceSdk.SyncOpResults{NextPageToken: nextPage}, nil
}

// Entitlements always returns an empty slice for secrets.
func (o *agentTokenBuilder) Entitlements(_ context.Context, resource *v2.Resource, _ resourceSdk.SyncOpAttrs) ([]*v2.Entitlement, *resourceSdk.SyncOpResults, error) {
	return nil, &resourceSdk.SyncOpResults{}, nil
}

// Grants always returns an empty slice for secrets since they don't have any entitlements.
func (o *agentTokenBuilder) Grants(ctx context.Context, resource *v2.Resource, opts resourceSdk.SyncOpAttrs) ([]*v2.Grant, *resourceSdk.SyncOpResults, error) {
	return nil, &resourceSdk.SyncOpResults{}, nil
}

func newAgentTokenBuilder(client *client.Client) *agentTokenBuilder {
	return &agentTokenBuilder{
		client: client,
	}
}
