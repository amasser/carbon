// +build linux

package input

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"strconv"
	"sync"
	"time"

	jsoniter "github.com/json-iterator/go"
	"github.com/observiq/carbon/entry"
	"github.com/observiq/carbon/operator"
	"github.com/observiq/carbon/operator/helper"
	"go.uber.org/zap"
)

func init() {
	operator.Register("journald_input", func() operator.Builder { return NewJournaldInputConfig("") })
}

func NewJournaldInputConfig(operatorID string) *JournaldInputConfig {
	return &JournaldInputConfig{
		InputConfig: helper.NewInputConfig(operatorID, "journald_input"),
	}
}

// JournaldInputConfig is the configuration of a journald input operator
type JournaldInputConfig struct {
	helper.InputConfig `yaml:",inline"`

	Directory *string  `json:"directory,omitempty" yaml:"directory,omitempty"`
	Files     []string `json:"files,omitempty"     yaml:"files,omitempty"`
}

// Build will build a journald input operator from the supplied configuration
func (c JournaldInputConfig) Build(buildContext operator.BuildContext) (operator.Operator, error) {
	inputOperator, err := c.InputConfig.Build(buildContext)
	if err != nil {
		return nil, err
	}

	args := make([]string, 0, 10)

	// Export logs in UTC time
	args = append(args, "--utc")

	// Export logs as JSON
	args = append(args, "--output=json")

	// Continue watching logs until cancelled
	args = append(args, "--follow")

	switch {
	case c.Directory != nil:
		args = append(args, "--directory", *c.Directory)
	case len(c.Files) > 0:
		for _, file := range c.Files {
			args = append(args, "--file", file)
		}
	}

	journaldInput := &JournaldInput{
		InputOperator: inputOperator,
		persist:       helper.NewScopedDBPersister(buildContext.Database, c.ID()),
		newCmd: func(ctx context.Context, cursor []byte) cmd {
			if cursor != nil {
				args = append(args, "--after-cursor", string(cursor))
			}
			return exec.CommandContext(ctx, "journalctl", args...)
		},
		json: jsoniter.ConfigFastest,
	}
	return journaldInput, nil
}

// JournaldInput is an operator that process logs using journald
type JournaldInput struct {
	helper.InputOperator

	newCmd func(ctx context.Context, cursor []byte) cmd

	persist helper.Persister
	json    jsoniter.API
	cancel  context.CancelFunc
	wg      *sync.WaitGroup
}

type cmd interface {
	StdoutPipe() (io.ReadCloser, error)
	Start() error
}

var lastReadCursorKey = "lastReadCursor"

// Start will start generating log entries.
func (operator *JournaldInput) Start() error {
	ctx, cancel := context.WithCancel(context.Background())
	operator.cancel = cancel
	operator.wg = &sync.WaitGroup{}

	err := operator.persist.Load()
	if err != nil {
		return err
	}

	// Start from a cursor if there is a saved offset
	cursor := operator.persist.Get(lastReadCursorKey)

	// Start journalctl
	cmd := operator.newCmd(ctx, cursor)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("failed to get journalctl stdout: %s", err)
	}
	err = cmd.Start()
	if err != nil {
		return fmt.Errorf("start journalctl: %s", err)
	}

	// Start a goroutine to periodically flush the offsets
	operator.wg.Add(1)
	go func() {
		defer operator.wg.Done()
		for {
			select {
			case <-ctx.Done():
				return
			case <-time.After(time.Second):
				operator.syncOffsets()
			}
		}
	}()

	// Start the reader goroutine
	operator.wg.Add(1)
	go func() {
		defer operator.wg.Done()
		defer operator.syncOffsets()

		stdoutBuf := bufio.NewReader(stdout)

		for {
			line, err := stdoutBuf.ReadBytes('\n')
			if err != nil {
				if err != io.EOF {
					operator.Errorw("Received error reading from journalctl stdout", zap.Error(err))
				}
				return
			}

			entry, cursor, err := operator.parseJournalEntry(line)
			if err != nil {
				operator.Warnw("Failed to parse journal entry", zap.Error(err))
				continue
			}
			operator.persist.Set(lastReadCursorKey, []byte(cursor))
			operator.Write(ctx, entry)
		}
	}()

	return nil
}

func (operator *JournaldInput) parseJournalEntry(line []byte) (*entry.Entry, string, error) {
	var record map[string]interface{}
	err := operator.json.Unmarshal(line, &record)
	if err != nil {
		return nil, "", err
	}

	timestamp, ok := record["__REALTIME_TIMESTAMP"]
	if !ok {
		return nil, "", errors.New("journald record missing __REALTIME_TIMESTAMP field")
	}

	timestampString, ok := timestamp.(string)
	if !ok {
		return nil, "", errors.New("journald field for timestamp is not a string")
	}

	timestampInt, err := strconv.ParseInt(timestampString, 10, 64)
	if err != nil {
		return nil, "", fmt.Errorf("parse timestamp: %s", err)
	}

	delete(record, "__REALTIME_TIMESTAMP")

	cursor, ok := record["__CURSOR"]
	if !ok {
		return nil, "", errors.New("journald record missing __CURSOR field")
	}

	cursorString, ok := cursor.(string)
	if !ok {
		return nil, "", errors.New("journald field for cursor is not a string")
	}

	entry := operator.NewEntry(record)
	entry.Timestamp = time.Unix(0, timestampInt*1000) // in microseconds
	return entry, cursorString, nil
}

func (operator *JournaldInput) syncOffsets() {
	err := operator.persist.Sync()
	if err != nil {
		operator.Errorw("Failed to sync offsets", zap.Error(err))
	}
}

// Stop will stop generating logs.
func (operator *JournaldInput) Stop() error {
	operator.cancel()
	operator.wg.Wait()
	return nil
}
