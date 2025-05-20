package main

import (
	"github.com/conductorone/baton-sdk/pkg/field"
	"github.com/spf13/viper"
)

var (
	TokenField = field.StringField(
		"token",
		field.WithDescription("The API token used to authenticate with terraform cloud."),
		field.WithRequired(true),
	)

	// OrgID = field.StringField(
	// 	"orgID",
	// 	field.WithDescription("The organization ID used in terraform cloud."),
	// 	field.WithRequired(true),
	// )

	Address = field.StringField(
		"address",
		field.WithDescription("The address of the terraform instance. Default: https://app.terraform.io"),
		field.WithRequired(false),
		field.WithDefaultValue("https://app.terraform.io"),
	)
	// ConfigurationFields defines the external configuration required for the
	// connector to run. Note: these fields can be marked as optional or
	// required.
	ConfigurationFields = []field.SchemaField{
		TokenField,
		Address,
	}
	// ConfigurationFields = []field.SchemaField{
	// 	TokenField,
	// 	OrgID,
	// 	Address,
	// }

	// FieldRelationships defines relationships between the fields listed in
	// ConfigurationFields that can be automatically validated. For example, a
	// username and password can be required together, or an access token can be
	// marked as mutually exclusive from the username password pair.
	FieldRelationships = []field.SchemaFieldRelationship{}
)

// ValidateConfig is run after the configuration is loaded, and should return an
// error if it isn't valid. Implementing this function is optional, it only
// needs to perform extra validations that cannot be encoded with configuration
// parameters.
func ValidateConfig(v *viper.Viper) error {
	return nil
}
