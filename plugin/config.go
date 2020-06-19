package plugin

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"go.etcd.io/bbolt"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest"
)

// Config is the configuration of a plugin
type Config struct {
	Builder
}

// Builder is an entity that can build plugins
type Builder interface {
	ID() string
	Type() string
	Build(BuildContext) (Plugin, error)
	SetNamespace(namespace string, exclude ...string)
}

// BuildContext supplies contextual resources when building a plugin.
type BuildContext struct {
	CustomRegistry CustomRegistry
	Database       Database
	Logger         *zap.SugaredLogger
}

type Database interface {
	Close() error
	Sync() error
	Update(func(*bbolt.Tx) error) error
	View(func(*bbolt.Tx) error) error
}

func NewTestDatabase(t *testing.T) *bbolt.DB {
	tempDir, err := ioutil.TempDir("", "")
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		os.RemoveAll(tempDir)
	})

	db, err := bbolt.Open(filepath.Join(tempDir, "test.db"), 0666, nil)
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		db.Close()
	})

	return db
}

type StubDatabase struct{}

func (d *StubDatabase) Close() error                          { return nil }
func (d *StubDatabase) Sync() error                           { return nil }
func (d *StubDatabase) Update(func(tx *bbolt.Tx) error) error { return nil }
func (d *StubDatabase) View(func(tx *bbolt.Tx) error) error   { return nil }

func NewStubDatabase() *StubDatabase {
	return &StubDatabase{}
}

// NewTestBuildContext returns a build context with a temporary database
// and a logger that writes to the test
func NewTestBuildContext(t *testing.T) BuildContext {
	return BuildContext{
		Database: NewTestDatabase(t),
		Logger:   zaptest.NewLogger(t).Sugar(),
	}
}

// registry is a global registry of plugin types to plugin builders.
var registry = make(map[string]func() Builder)

// Register will register a function to a plugin type.
// This function will return a builder for the supplied type.
func Register(pluginType string, builder Builder) {
	registry[pluginType] = func() Builder {
		val := reflect.New(reflect.TypeOf(builder).Elem()).Interface()
		return val.(Builder)
	}
}

// IsDefined will return a boolean indicating if a plugin type is registered and defined.
func IsDefined(pluginType string) bool {
	_, ok := registry[pluginType]
	return ok
}

// UnmarshalJSON will unmarshal a config from JSON.
func (c *Config) UnmarshalJSON(bytes []byte) error {
	var baseConfig struct {
		ID   string
		Type string
	}

	err := json.Unmarshal(bytes, &baseConfig)
	if err != nil {
		return fmt.Errorf("failed to unmarshal json to base config: %s", err)
	}

	if baseConfig.Type == "" {
		return fmt.Errorf("failed to unmarshal json to undefined plugin type")
	}

	builderFunc, ok := registry[baseConfig.Type]
	if !ok {
		return fmt.Errorf("failed to unmarshal json to unsupported type: %s", baseConfig.Type)
	}

	builder := builderFunc()
	err = json.Unmarshal(bytes, builder)
	if err != nil {
		return fmt.Errorf("failed to unmarshal json to %s: %s", baseConfig.Type, err)
	}

	c.Builder = builder
	return nil
}

// MarshalJSON will marshal a config to JSON.
func (c Config) MarshalJSON() ([]byte, error) {
	return json.Marshal(c.Builder)
}

// UnmarshalYAML will unmarshal a config from YAML.
func (c *Config) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var baseConfig struct {
		ID   string
		Type string
	}

	err := unmarshal(&baseConfig)
	if err != nil {
		return fmt.Errorf("failed to unmarshal yaml to base config: %s", err)
	}

	if baseConfig.Type == "" {
		return fmt.Errorf("failed to unmarshal yaml to undefined plugin type")
	}

	builderFunc, ok := registry[baseConfig.Type]
	if !ok {
		return fmt.Errorf("failed to unmarshal yaml to unsupported type: %s", baseConfig.Type)
	}

	builder := builderFunc()
	err = unmarshal(builder)
	if err != nil {
		return fmt.Errorf("failed to unmarshal yaml to %s: %s", baseConfig.Type, err)
	}

	c.Builder = builder
	return nil
}

// MarshalYAML will marshal a config to YAML.
func (c Config) MarshalYAML() (interface{}, error) {
	return c.Builder, nil
}
