package bigtable

import (
	"embed"
	"log"
	"os"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	testingutils "terraform-provider-alis/internal/testing"
)

//go:embed templates/*
var TerraformFiles embed.FS

var (
	providerConfig string
	tableConfig    string
)

func init() {
	if config, err := getConfigFromFile("templates/provider.tf"); err != nil {
		providerConfig = config
	} else {
		log.Fatalf("Error reading provider config: %v", err)
	}

	if config, err := getConfigFromFile("templates/google_bigtable_table.tf"); err != nil {
		tableConfig = config
	} else {
		log.Fatalf("Error reading table config: %v", err)
	}

}

func TestAccBigtableTableResource(t *testing.T) {
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testingutils.TestAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Create and read a Bigtable table resource.
			{
				Config: providerConfig + tableConfig,
				Check:  resource.ComposeAggregateTestCheckFunc(
				//resource.TestCheckResourceAttr("google_bigtable_table.foo", "name", "foo"),
				),
			},
		},
	})
}

// getConfigFromFile reads a file from the embedded filesystem and returns its content.
func getConfigFromFile(path string) (string, error) {

	contentBytes, err := TerraformFiles.ReadFile(path)
	if err != nil {
		return "", err
	}

	content := string(contentBytes)

	if os.Getenv("TEST_PROJECT_ID") != "" {
		strings.ReplaceAll(content, "{{TEST_PROJECT_ID}}", os.Getenv("TEST_PROJECT_ID"))
	}
	if os.Getenv("TEST_INSTANCE_ID") != "" {
		strings.ReplaceAll(content, "{{TEST_INSTANCE_ID}}", os.Getenv("TEST_INSTANCE_ID"))
	}

	return content, nil
}
