package config

import (
	"errors"
	"fmt"
	"github.com/BurntSushi/toml"
	"github.com/openshift-virtualization/wasp-agent/pkg/log"
	"os"
	"path/filepath"
)

const (
	defaultRuntime = "crun"
)

type RuntimeConfig struct {
	DefaultRuntime string `toml:"default_runtime"`
}

type Config struct {
	crioMainConfPath string
	crioDropInPath   string
	RuntimeConfig
}

func New(mainPath, dropInPath string) *Config {
	conf := defaultConfig()
	conf.crioMainConfPath = mainPath
	conf.crioDropInPath = dropInPath

	return conf
}

func (c *Config) GetRuntime() (string, error) {
	if err := c.UpdateFromFile(c.crioMainConfPath); err != nil {
		isNotExistErr := errors.Is(err, os.ErrNotExist)
		if isNotExistErr {
			log.Log.Infof("Skipping not-existing config file %q", c.crioMainConfPath)
		} else {
			return "", err
		}
	}

	if err := c.UpdateFromPath(c.crioDropInPath); err != nil {
		return "", err
	}

	return c.DefaultRuntime, nil
}

// reference: github.com/cri-o/pkg/config/config.go
func (c *Config) UpdateFromFile(path string) error {
	log.Log.Infof("Updating config from file: %s", path)

	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	t := new(tomlConfig)

	_, err = toml.Decode(string(data), t)
	if err != nil {
		return fmt.Errorf("unable to decode configuration %v: %w", path, err)
	}

	t.toConfig(c)

	return nil
}

// reference: github.com/cri-o/pkg/config/config.go
func (c *Config) UpdateFromPath(path string) error {
	log.Log.Infof("Updating config from path: %s", path)
	if _, err := os.Stat(path); err != nil && os.IsNotExist(err) {
		return nil
	}

	if err := filepath.Walk(path,
		func(p string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if info.IsDir() {
				return nil
			}

			return c.UpdateFromFile(p)
		}); err != nil {
		return fmt.Errorf("walk path: %w", err)
	}

	return nil
}

// reference: github.com/cri-o/pkg/config/config.go
type tomlConfig struct {
	Crio struct {
		Runtime struct{ RuntimeConfig } `toml:"runtime"`
	} `toml:"crio"`
}

// reference: github.com/cri-o/pkg/config/config.go
func (t *tomlConfig) toConfig(c *Config) {
	if t.Crio.Runtime.RuntimeConfig.DefaultRuntime != "" {
		c.DefaultRuntime = t.Crio.Runtime.RuntimeConfig.DefaultRuntime
	}
}

// DefaultConfig returns the default configuration for crio.
func defaultConfig() *Config {
	return &Config{
		RuntimeConfig: RuntimeConfig{
			DefaultRuntime: defaultRuntime,
		},
	}
}
