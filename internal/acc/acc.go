package acc

import (
	"encoding/base64"
	"os"
	"strings"
	"sync"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/loafoe/terraform-provider-ssh/ssh"
)

const (
	// ProviderName for single configuration testing
	ProviderName = "ssh"
)

// Skip implements a wrapper for (*testing.T).Skip() to prevent unused linting reports
//
// Reference: https://github.com/dominikh/go-tools/issues/633#issuecomment-606560616
func Skip(t *testing.T, message string) {
	t.Skip(message)
}

// ProviderFactories is a static map containing only the main provider instance
//
// Use other ProviderFactories functions, such as FactoriesAlternate,
// for tests requiring special provider configurations.
var ProviderFactories map[string]func() (*schema.Provider, error)

// testAccProviderConfigure ensures Provider is only configured once
//
// The PreCheck(t) function is invoked for every test and this prevents
// extraneous reconfiguration to the same values each time. However, this does
// not prevent reconfiguration that may happen should the address of
// Provider be erroneously reused in ProviderFactories.
var testAccProviderConfigure sync.Once

func init() {
	// Always allocate a new provider instance each invocation, otherwise gRPC
	// ProviderConfigure() can overwrite configuration during concurrent testing.
	ProviderFactories = map[string]func() (*schema.Provider, error){
		ProviderName: func() (*schema.Provider, error) { return ssh.Provider(), nil }, //nolint:unparam
	}
}

// PreCheck verifies and sets required provider testing configuration
//
// This PreCheck function should be present in every acceptance test. It allows
// test configurations to omit a provider configuration with region and ensures
// testing functions that attempt to call AWS APIs are previously configured.
//
// These verifications and configuration are preferred at this level to prevent
// provider developers from experiencing less clear errors for every test.
func PreCheck(t *testing.T) {
	// Since we are outside the scope of the Terraform configuration we must
	// call Configure() to properly initialize the provider configuration.
	testAccProviderConfigure.Do(func() {
		// TODO: add additional pre-checks here
		if AccHostname() == "" {
			t.Fatalf("SSH_ACC_HOSTNAME be set")
		}
		if AccUsername() == "" {
			t.Fatalf("SSH_ACC_USERNAME must be set")
		}
		if AccPrivateKey() == "" {
			t.Fatalf("SSH_ACC_PRIVATE_KEY_BASE64 must be set")
		}
	})
}

func AccHostname() string {
	return os.Getenv("SSH_ACC_HOSTNAME")
}

func AccUsername() string {
	return os.Getenv("SSH_ACC_USERNAME")
}

func AccPrivateKey() string {
	encoded := os.Getenv("SSH_ACC_PRIVATE_KEY_BASE64")
	if encoded == "" {
		return ""
	}
	decoded, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return ""
	}
	return strings.ReplaceAll(string(decoded), "\n", "")
}
