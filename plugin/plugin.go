//go:generate mockery -name=Plugin -output=./testutil -outpkg=testutil -case=snake
package plugin

import (
	"github.com/bluemedora/bplogagent/entry"
)

type Plugin interface {
	ID() PluginID
	Type() string
}

type Outputter interface {
	Plugin
	Outputs() []Inputter
}

type Inputter interface {
	Plugin
	// TODO should this take a pointer or a value?
	Input(*entry.Entry) error
}

type Stopper interface {
	Plugin
	Stop()
}

type Starter interface {
	Plugin
	Start() error
}

type PluginID string
