package connector

import (
	v2 "github.com/conductorone/baton-sdk/pb/c1/connector/v2"
)

// The user resource type is for all user objects from the database.
var userResourceType = &v2.ResourceType{
	Id:          "user",
	DisplayName: "User",
	Traits:      []v2.ResourceType_Trait{v2.ResourceType_TRAIT_USER},
}

var organizationResourceType = &v2.ResourceType{
	Id:          "organization",
	DisplayName: "Organization",
	Traits:      []v2.ResourceType_Trait{v2.ResourceType_TRAIT_GROUP},
}

var projectResourceType = &v2.ResourceType{
	Id:          "project",
	DisplayName: "Project",
	Traits: []v2.ResourceType_Trait{
		v2.ResourceType_TRAIT_GROUP,
	},
}

var workspaceResourceType = &v2.ResourceType{
	Id:          "workspace",
	DisplayName: "Workspace",
	Traits: []v2.ResourceType_Trait{
		v2.ResourceType_TRAIT_GROUP,
	},
}

// requires: team management requires paid plan
// https://www.hashicorp.com/en/pricing?tab=terraform
var teamResourceType = &v2.ResourceType{
	Id:          "team",
	DisplayName: "Team",
	Traits:      []v2.ResourceType_Trait{v2.ResourceType_TRAIT_GROUP},
}
