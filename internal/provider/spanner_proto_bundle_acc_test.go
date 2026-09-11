package provider_test

import (
	"fmt"
	"path/filepath"
	"regexp"
	"testing"

	"terraform-provider-alis/internal/acctest"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
)

func TestAccSpannerProtoBundle_basic(t *testing.T) {
	env := acctest.Setup(t)
	const (
		address = "alis_google_spanner_proto_bundle.test"
		pkg     = "tftest.v1"
	)
	fixture := func(name string) string {
		abs, err := filepath.Abs(filepath.Join("..", "spanner", "schema", "testdata", "proto_bundle", name))
		if err != nil {
			t.Fatal(err)
		}
		return abs
	}

	config := func(fds string) string {
		return env.ProviderBlock() + fmt.Sprintf(`
resource "alis_google_spanner_proto_bundle" "test" {
  project  = %q
  instance = %q
  database = %q
  packages = [%q]
  sources  = [{ local_path = %q }]
}
`, env.Project, env.Instance, env.Database, pkg, fds)
	}

	bundleGone := acctest.CheckNotFound("proto bundle", pkg, func() error {
		_, err := env.Service.GetProtoBundleTypes(t.Context(), env.DatabaseName, []string{pkg})
		return err
	})

	var shaV1 string
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories(),
		CheckDestroy:             bundleGone,
		Steps: []resource.TestStep{
			{
				Config: config(fixture("owned_including_imports.fds")),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(address, "types.#", "3"),
					resource.TestCheckTypeSetElemAttr(address, "types.*", "tftest.v1.Simple"),
					resource.TestCheckResourceAttrWith(address, "descriptors_sha256", func(v string) error {
						if len(v) != 64 {
							return fmt.Errorf("descriptors_sha256 = %q, want 64 hex chars", v)
						}
						shaV1 = v
						return nil
					}),
				),
			},
			{
				// Same sources, same packages: nothing to do.
				Config:   config(fixture("owned_including_imports.fds")),
				PlanOnly: true,
			},
			{
				// A changed descriptor set (extra field) is an in-place update.
				Config: config(fixture("owned_v2.fds")),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction(address, plancheck.ResourceActionUpdate),
					},
				},
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(address, "types.#", "3"),
					resource.TestCheckResourceAttrWith(address, "descriptors_sha256", func(v string) error {
						if v == shaV1 {
							return fmt.Errorf("descriptors_sha256 unchanged after the descriptor set changed")
						}
						return nil
					}),
				),
			},
			{
				// The data source lists the owned types the bundle now holds.
				Config: config(fixture("owned_v2.fds")) + fmt.Sprintf(`
data "alis_google_spanner_proto_bundle" "read" {
  project  = %q
  instance = %q
  database = %q
  packages = [%q]

  depends_on = [alis_google_spanner_proto_bundle.test]
}
`, env.Project, env.Instance, env.Database, pkg),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.alis_google_spanner_proto_bundle.read", "types.#", "3"),
					resource.TestCheckTypeSetElemAttr("data.alis_google_spanner_proto_bundle.read", "types.*", "tftest.v1.Simple"),
				),
			},
			{
				ResourceName:                         address,
				ImportState:                          true,
				ImportStateId:                        env.DatabaseName + "/protoBundles/" + pkg,
				ImportStateVerify:                    true,
				ImportStateVerifyIdentifierAttribute: "database",
				// Sources live only in configuration, and the hash is derived
				// from them, so neither can be seeded by an import.
				ImportStateVerifyIgnore: []string{"sources", "descriptors_sha256"},
			},
			{
				// A package with no types in the bundle is an error, not an empty set.
				Config: config(fixture("owned_v2.fds")) + fmt.Sprintf(`
data "alis_google_spanner_proto_bundle" "missing" {
  project  = %q
  instance = %q
  database = %q
  packages = ["tftest.absent"]
}
`, env.Project, env.Instance, env.Database),
				PlanOnly:    true,
				ExpectError: regexp.MustCompile(`Error Reading Proto Bundle`),
			},
		},
	})
}
