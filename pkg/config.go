package config

import (
	"log"
	"os"

	"gopkg.in/yaml.v2"
)

type Bundle struct {
	Apps BundleAppConfig `yaml:"bundles"`
}

type BundleAppConfig struct {
	Apps map[string]BundleApp `yaml:"apps"`
}

type BundleApp struct {
	Namespace    string        `yaml:"namespace,omitempty"`
	Enabled      bool          `yaml:"enabled,omitempty"`
	Version      string        `yaml:"version,omitempty"`
	ExtraConfigs []ExtraConfig `yaml:"extraConfigs,omitempty"`
}

type AppConfig struct {
	// ExtraConfigs is the list of extra configs to be added to the configmap
	ExtraConfigs []ExtraConfig `yaml:"extraConfigs"`
	Enabled      bool          `yaml:"enabled"`
}

type ExtraConfig struct {
	// Kind is the kind of the source
	Kind string `yaml:"kind"`
	// Name is the name of the config source
	Name string `yaml:"name"`
	// Namespace is the namespace of the config source
	Namespace string `yaml:"namespace"`
	// Priority is the priority to be used for the config source
	Priority int `yaml:"priority,omitempty"`
}

func New(configPath string) map[string]Bundle {
	appConfigs := renderAppConfigs(configPath)
	if len(appConfigs) == 0 {
		log.Fatalf("no app configs found in %s", configPath)
	}
	return appConfigs
}

func renderAppConfigs(configPath string) map[string]Bundle {
	// Read the YAML file from configPath
	configYaml, err := os.ReadFile(configPath)
	if err != nil {
		log.Fatalf("failed to read config file: %v", err)
	}

	var bundles map[string]Bundle
	err = yaml.Unmarshal(configYaml, &bundles)
	if err != nil {
		log.Fatalf("failed to unmarshal config file: %v", err)
	}

	return bundles
}
