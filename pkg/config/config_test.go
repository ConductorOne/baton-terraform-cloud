package config_test

import (
	"testing"

	"github.com/conductorone/baton-sdk/pkg/test"
	"github.com/conductorone/baton-sdk/pkg/ustrings"
	"github.com/conductorone/baton-terraform-cloud/pkg/config"
)

func TestConfigs(t *testing.T) {
	test.ExerciseTestCasesFromExpressions(
		t,
		config.Config,
		nil,
		ustrings.ParseFlags,
		[]test.TestCaseFromExpression{
			{
				"",
				false,
				"empty",
			},
			{
				"--token 1",
				true,
				"token only (address defaulted)",
			},
			{
				"--token 1 --address https://tfe.example.com",
				true,
				"token and address",
			},
			{
				"--address https://tfe.example.com",
				false,
				"address missing token",
			},
		},
	)
}
