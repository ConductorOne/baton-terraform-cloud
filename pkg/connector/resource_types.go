package connector

import (
	v2 "github.com/conductorone/baton-sdk/pb/c1/connector/v2"
	"github.com/conductorone/baton-sdk/pkg/annotations"
)

const (
	userResourceTypeID = "user"
	emailProfileKey    = "email"
)

// The user resource type is for all user objects from the database.
var userResourceType = &v2.ResourceType{
	Id:          userResourceTypeID,
	DisplayName: "User",
	Traits:      []v2.ResourceType_Trait{v2.ResourceType_TRAIT_USER},
}

var organizationResourceType = &v2.ResourceType{
	Id:          "organization",
	DisplayName: "Organization",
	Traits:      []v2.ResourceType_Trait{v2.ResourceType_TRAIT_GROUP},
	Annotations: annotations.New(&v2.SkipEntitlements{}),
}

var projectResourceType = &v2.ResourceType{
	Id:          "project",
	DisplayName: "Project",
	Traits: []v2.ResourceType_Trait{
		v2.ResourceType_TRAIT_GROUP,
	},
	Annotations: annotations.New(&v2.SkipEntitlements{}),
}

var workspaceResourceType = &v2.ResourceType{
	Id:          "workspace",
	DisplayName: "Workspace",
	Traits: []v2.ResourceType_Trait{
		v2.ResourceType_TRAIT_GROUP,
	},
	Annotations: annotations.New(&v2.SkipEntitlements{}),
}

var agentTokenResourceType = &v2.ResourceType{
	Id:          "agentToken",
	DisplayName: "Agent Token",
	Traits: []v2.ResourceType_Trait{
		v2.ResourceType_TRAIT_SECRET,
	},
	Annotations: annotations.New(&v2.SkipEntitlementsAndGrants{}),
}

// requires: team management requires paid plan
// https://www.hashicorp.com/en/pricing?tab=terraform
var teamResourceType = &v2.ResourceType{
	Id:          "team",
	DisplayName: "Team",
	Traits:      []v2.ResourceType_Trait{v2.ResourceType_TRAIT_GROUP},
	Annotations: annotations.New(&v2.SkipEntitlements{}),
}
