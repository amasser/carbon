package parser

import (
	"context"
	"math"
	"testing"
	"time"

	"github.com/bluemedora/bplogagent/entry"
	"github.com/bluemedora/bplogagent/internal/testutil"
	"github.com/bluemedora/bplogagent/plugin"
	"github.com/bluemedora/bplogagent/plugin/helper"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestTimeParser(t *testing.T) {

	testCases := []struct {
		name           string
		sample         string
		gotimeLayout   string
		strptimeLayout string
	}{
		{
			name:           "unix",
			sample:         "Mon Jan 2 15:04:05 MST 2006",
			gotimeLayout:   "Mon Jan 2 15:04:05 MST 2006",
			strptimeLayout: "%a %b %e %H:%M:%S %Z %Y",
		},
		{
			name:           "almost-unix",
			sample:         "Mon Jan 02 15:04:05 MST 2006",
			gotimeLayout:   "Mon Jan 02 15:04:05 MST 2006",
			strptimeLayout: "%a %b %d %H:%M:%S %Z %Y",
		},
		{
			name:           "kitchen",
			sample:         "12:34PM",
			gotimeLayout:   time.Kitchen,
			strptimeLayout: "%H:%M%p",
		},
		{
			name:           "countdown",
			sample:         "-0100 01 01 01 01 01 01",
			gotimeLayout:   "-0700 06 05 04 03 02 01",
			strptimeLayout: "%z %y %S %M %H %e %m",
		},
		{
			name:           "debian-syslog",
			sample:         "Jun 09 11:39:45",
			gotimeLayout:   "Jan 02 15:04:05",
			strptimeLayout: "%b %d %H:%M:%S",
		},
		{
			name:           "opendistro",
			sample:         "2020-06-09T15:39:58",
			gotimeLayout:   "2006-01-02T15:04:05",
			strptimeLayout: "%Y-%m-%dT%H:%M:%S",
		},
		{
			name:           "postgres",
			sample:         "2019-11-05 10:38:35.118 EST",
			gotimeLayout:   "2006-01-02 15:04:05.999 MST",
			strptimeLayout: "%Y-%m-%d %H:%M:%S.%L %Z",
		},
		{
			name:           "ibm-mq",
			sample:         "3/4/2018 11:52:29",
			gotimeLayout:   "1/2/2006 15:04:05",
			strptimeLayout: "%q/%g/%Y %H:%M:%S",
		},
		{
			name:           "cassandra",
			sample:         "2019-11-27T09:34:32.901-0500",
			gotimeLayout:   "2006-01-02T15:04:05.999-0700",
			strptimeLayout: "%Y-%m-%dT%H:%M:%S.%L%z",
		},
		{
			name:           "oracle",
			sample:         "2019-10-15T10:42:01.900436-04:00",
			gotimeLayout:   "2006-01-02T15:04:05.999999-07:00",
			strptimeLayout: "%Y-%m-%dT%H:%M:%S.%f%j",
		},
		{
			name:           "oracle-listener",
			sample:         "22-JUL-2019 15:16:13",
			gotimeLayout:   "02-Jan-2006 15:04:05",
			strptimeLayout: "%d-%b-%Y %H:%M:%S",
		},
		{
			name:           "k8s",
			sample:         "2019-03-08T18:41:12.152531115Z",
			gotimeLayout:   "2006-01-02T15:04:05.999999999Z",
			strptimeLayout: "%Y-%m-%dT%H:%M:%S.%sZ",
		},
		{
			name:           "jetty",
			sample:         "05/Aug/2019:20:38:46 +0000",
			gotimeLayout:   "02/Jan/2006:15:04:05 -0700",
			strptimeLayout: "%d/%b/%Y:%H:%M:%S %z",
		},
		{
			name:           "puppet",
			sample:         "Aug  4 03:26:02",
			gotimeLayout:   "Jan _2 15:04:05",
			strptimeLayout: "%b %e %H:%M:%S",
		},
	}

	rootField := entry.NewField()
	someField := entry.NewField("some_field")

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			expected, err := time.ParseInLocation(tc.gotimeLayout, tc.sample, time.Local)
			require.NoError(t, err, "Test configuration includes invalid timestamp or layout")

			gotimeRootCfg := parseTimeTestConfig(helper.GotimeKey, tc.gotimeLayout, rootField)
			t.Run("gotime-root", runTest(t, gotimeRootCfg, makeTestEntry(rootField, tc.sample), expected))

			gotimeNonRootCfg := parseTimeTestConfig(helper.GotimeKey, tc.gotimeLayout, someField)
			t.Run("gotime-non-root", runTest(t, gotimeNonRootCfg, makeTestEntry(someField, tc.sample), expected))

			strptimeRootCfg := parseTimeTestConfig(helper.StrptimeKey, tc.strptimeLayout, rootField)
			t.Run("strptime-root", runTest(t, strptimeRootCfg, makeTestEntry(rootField, tc.sample), expected))

			strptimeNonRootCfg := parseTimeTestConfig(helper.StrptimeKey, tc.strptimeLayout, someField)
			t.Run("strptime-non-root", runTest(t, strptimeNonRootCfg, makeTestEntry(someField, tc.sample), expected))
		})
	}
}

func TestTimeEpochs(t *testing.T) {

	testCases := []struct {
		name     string
		sample   interface{}
		layout   string
		expected time.Time
		err      bool
		maxLoss  time.Duration
	}{
		{
			name:     "s-default-string",
			sample:   "1136214245",
			layout:   "s",
			expected: time.Unix(1136214245, 0),
		},
		{
			name:     "s-default-int",
			sample:   1136214245,
			layout:   "s",
			expected: time.Unix(1136214245, 0),
		},
		{
			name:     "s-default-float",
			sample:   1136214245.0,
			layout:   "s",
			expected: time.Unix(1136214245, 0),
		},
		{
			name:     "ms-default-string",
			sample:   "1136214245123",
			layout:   "ms",
			expected: time.Unix(1136214245, 123000000),
		},
		{
			name:     "ms-default-int",
			sample:   1136214245123,
			layout:   "ms",
			expected: time.Unix(1136214245, 123000000),
		},
		{
			name:     "ms-default-float",
			sample:   1136214245123.0,
			layout:   "ms",
			expected: time.Unix(1136214245, 123000000),
		},
		{
			name:     "us-default-string",
			sample:   "1136214245123456",
			layout:   "us",
			expected: time.Unix(1136214245, 123456000),
		},
		{
			name:     "us-default-int",
			sample:   1136214245123456,
			layout:   "us",
			expected: time.Unix(1136214245, 123456000),
		},
		{
			name:     "us-default-float",
			sample:   1136214245123456.0,
			layout:   "us",
			expected: time.Unix(1136214245, 123456000),
		},
		{
			name:     "ns-default-string",
			sample:   "1136214245123456789",
			layout:   "ns",
			expected: time.Unix(1136214245, 123456789),
		},
		{
			name:     "ns-default-int",
			sample:   1136214245123456789,
			layout:   "ns",
			expected: time.Unix(1136214245, 123456789),
		},
		{
			name:     "ns-default-float",
			sample:   1136214245123456789.0,
			layout:   "ns",
			expected: time.Unix(1136214245, 123456789),
			maxLoss:  time.Nanosecond * 100,
		},
		{
			name:     "s.ms-default-string",
			sample:   "1136214245.123",
			layout:   "s.ms",
			expected: time.Unix(1136214245, 123000000),
		},
		{
			name:     "s.ms-default-int",
			sample:   1136214245,
			layout:   "s.ms",
			expected: time.Unix(1136214245, 0), // drops subseconds
			maxLoss:  time.Nanosecond * 100,
		},
		{
			name:     "s.ms-default-float",
			sample:   1136214245.123,
			layout:   "s.ms",
			expected: time.Unix(1136214245, 123000000),
		},
		{
			name:     "s.us-default-string",
			sample:   "1136214245.123456",
			layout:   "s.us",
			expected: time.Unix(1136214245, 123456000),
		},
		{
			name:     "s.us-default-int",
			sample:   1136214245,
			layout:   "s.us",
			expected: time.Unix(1136214245, 0), // drops subseconds
			maxLoss:  time.Nanosecond * 100,
		},
		{
			name:     "s.us-default-float",
			sample:   1136214245.123456,
			layout:   "s.us",
			expected: time.Unix(1136214245, 123456000),
		},
		{
			name:     "s.ns-default-string",
			sample:   "1136214245.123456789",
			layout:   "s.ns",
			expected: time.Unix(1136214245, 123456789),
		},
		{
			name:     "s.ns-default-int",
			sample:   1136214245,
			layout:   "s.ns",
			expected: time.Unix(1136214245, 0), // drops subseconds
			maxLoss:  time.Nanosecond * 100,
		},
		{
			name:     "s.ns-default-float",
			sample:   1136214245.123456789,
			layout:   "s.ns",
			expected: time.Unix(1136214245, 123456789),
			maxLoss:  time.Nanosecond * 100,
		},
	}

	rootField := entry.NewField()
	someField := entry.NewField("some_field")

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			rootCfg := parseTimeTestConfig(helper.EpochKey, tc.layout, rootField)
			t.Run("gotime-root", runTestLossy(t, rootCfg, makeTestEntry(rootField, tc.sample), tc.expected, tc.maxLoss))

			nonRootCfg := parseTimeTestConfig(helper.EpochKey, tc.layout, someField)
			t.Run("gotime-non-root", runTestLossy(t, nonRootCfg, makeTestEntry(someField, tc.sample), tc.expected, tc.maxLoss))
		})
	}
}

func makeTestEntry(field entry.Field, value interface{}) *entry.Entry {
	e := entry.New()
	e.Set(field, value)
	return e
}

func runTest(t *testing.T, cfg *TimeParserConfig, ent *entry.Entry, expected time.Time) func(*testing.T) {
	return runTestLossy(t, cfg, ent, expected, time.Duration(0))
}

func runTestLossy(t *testing.T, cfg *TimeParserConfig, ent *entry.Entry, expected time.Time, maxLoss time.Duration) func(*testing.T) {

	return func(t *testing.T) {
		buildContext := testutil.NewBuildContext(t)

		gotimePlugin, err := cfg.Build(buildContext)
		require.NoError(t, err)

		mockOutput := &testutil.Plugin{}
		resultChan := make(chan *entry.Entry, 1)
		mockOutput.On("Process", mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
			resultChan <- args.Get(1).(*entry.Entry)
		}).Return(nil)

		timeParser := gotimePlugin.(*TimeParserPlugin)
		timeParser.OutputPlugins = []plugin.Plugin{mockOutput}

		require.NoError(t, timeParser.Process(context.Background(), ent))

		select {
		case e := <-resultChan:
			diff := time.Duration(math.Abs(float64(expected.Sub(e.Timestamp))))
			require.True(t, diff <= maxLoss)
		case <-time.After(time.Second):
			require.FailNow(t, "Timed out waiting for entry to be processed")
		}
	}
}

func parseTimeTestConfig(layoutType, layout string, parseFrom entry.Field) *TimeParserConfig {
	return &TimeParserConfig{
		TransformerConfig: helper.TransformerConfig{
			BasicConfig: helper.BasicConfig{
				PluginID:   "test_plugin_id",
				PluginType: "time_parser",
			},
			WriterConfig: helper.WriterConfig{
				OutputIDs: []string{"output1"},
			},
		},
		TimeParser: helper.TimeParser{
			LayoutType: layoutType,
			Layout:     layout,
			ParseFrom:  parseFrom,
		},
	}
}
