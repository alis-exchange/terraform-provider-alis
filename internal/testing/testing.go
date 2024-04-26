package testing

import (
	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"terraform-provider-alis/internal/provider"
)

const (
	ProviderConfig = `
	provider "google" {}
	`
)

var (
	TestAccProtoV6ProviderFactories = map[string]func() (tfprotov6.ProviderServer, error){
		"google": providerserver.NewProtocol6WithError(provider.NewProvider("test")()),
	}
)
