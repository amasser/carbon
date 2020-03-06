package plugins

import (
	"encoding/json"
	"fmt"
	"sync"

	e "github.com/bluemedora/bplogagent/entry"
	pg "github.com/bluemedora/bplogagent/plugin"
)

func init() {
	pg.RegisterConfig("json_parser", &JSONParserConfig{})
}

type JSONParserConfig struct {
	pg.DefaultPluginConfig    `mapstructure:",squash" yaml:",inline"`
	pg.DefaultOutputterConfig `mapstructure:",squash" yaml:",inline"`
	pg.DefaultInputterConfig  `mapstructure:",squash" yaml:",inline"`

	// TODO design these params better
	Field            string
	DestinationField string
}

func (c JSONParserConfig) Build(context pg.BuildContext) (pg.Plugin, error) {
	defaultPlugin, err := c.DefaultPluginConfig.Build(context.Logger)
	if err != nil {
		return nil, fmt.Errorf("failed to build default plugin: %s", err)
	}

	defaultInputter, err := c.DefaultInputterConfig.Build()
	if err != nil {
		return nil, fmt.Errorf("failed to build default inputter: %s", err)
	}

	defaultOutputter, err := c.DefaultOutputterConfig.Build(context.Plugins)
	if err != nil {
		return nil, fmt.Errorf("failed to build default outputter: %s", err)
	}

	if c.Field == "" {
		return nil, fmt.Errorf("missing required field 'field'")
	}

	plugin := &JSONParser{
		DefaultPlugin:    defaultPlugin,
		DefaultInputter:  defaultInputter,
		DefaultOutputter: defaultOutputter,

		field:            c.Field,
		destinationField: c.DestinationField,
	}

	return plugin, nil
}

type JSONParser struct {
	pg.DefaultPlugin
	pg.DefaultOutputter
	pg.DefaultInputter

	field            string
	destinationField string
}

func (p *JSONParser) Start(wg *sync.WaitGroup) error {
	go func() {
		defer wg.Done()
		for {
			entry, ok := <-p.Input()
			if !ok {
				return
			}

			newEntry, err := p.processEntry(entry)
			if err != nil {
				// TODO better error handling
				p.Warnw("Failed to process entry", "error", err)
				continue
			}

			p.Output() <- newEntry
		}
	}()

	return nil
}

func (p *JSONParser) processEntry(entry e.Entry) (e.Entry, error) {
	message, ok := entry.Record[p.field]
	if !ok {
		return e.Entry{}, fmt.Errorf("field '%s' does not exist on the record", p.field)
	}

	messageString, ok := message.(string)
	if !ok {
		return e.Entry{}, fmt.Errorf("field '%s' can not be parsed as JSON because it is of type %T", p.field, message)
	}

	// TODO consider using faster json decoder (fastjson?)
	var parsedMessage map[string]interface{}
	err := json.Unmarshal([]byte(messageString), &parsedMessage)
	if err != nil {
		return e.Entry{}, fmt.Errorf("failed to parse field %s as JSON: %w", p.field, err)
	}

	if p.destinationField == "" {
		entry.Record[p.field] = parsedMessage
	} else {
		entry.Record[p.destinationField] = parsedMessage
	}

	return entry, nil
}