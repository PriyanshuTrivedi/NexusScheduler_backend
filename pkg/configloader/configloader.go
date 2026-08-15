// Package configloader reads each service's environment-scoped YAML config
// (code/<service>/config/{development,staging,production}.yaml) and reads
// required secrets from the environment. Secrets never live in the YAML
// files — only non-secret tuning (ports, pool sizes, timeouts, log level).
package configloader

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// envVar is the environment variable that selects which YAML file to load.
// Defaults to "development" so a bare `go run` works locally without any
// setup.
const envVar = "ENV"

const defaultEnv = "development"

// Load reads code/<service>/config/<env>.yaml, where <env> comes from the
// ENV environment variable (default "development"), and unmarshals it into
// cfg. cfg must be a pointer.
func Load(service string, cfg interface{}) error {
	env := os.Getenv(envVar)
	if env == "" {
		env = defaultEnv
	}

	path := fmt.Sprintf("code/%s/config/%s.yaml", service, env)
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("configloader: read %s: %w", path, err)
	}
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return fmt.Errorf("configloader: parse %s: %w", path, err)
	}
	return nil
}

// MustGetEnv reads a required environment variable and panics if it's
// unset — used for secrets (DSNs, addresses, broker lists) that must never
// be committed to a YAML file. Panicking on startup is intentional: a
// service missing a required secret should fail fast, not serve traffic
// with a broken dependency.
func MustGetEnv(key string) string {
	v := os.Getenv(key)
	if v == "" {
		panic(fmt.Sprintf("configloader: required environment variable %s is not set", key))
	}
	return v
}
