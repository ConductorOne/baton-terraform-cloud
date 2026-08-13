package main

import (
	"github.com/conductorone/baton-sdk/pkg/config"
	cfg "github.com/conductorone/baton-terraform-cloud/pkg/config"
)

func main() {
	config.Generate("terraform-cloud", cfg.Config)
}
