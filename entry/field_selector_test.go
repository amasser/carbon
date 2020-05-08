package entry

import (
	"encoding/json"
	"testing"

	"github.com/mitchellh/mapstructure"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	yaml "gopkg.in/yaml.v2"
)

func TestFieldSelectorGet(t *testing.T) {
	standardRecord := map[string]interface{}{
		"testkey": "testval",
		"testnested": map[string]interface{}{
			"testnestedkey": "testnestedval",
		},
	}

	cases := []struct {
		name        string
		selector    FieldSelector
		record      interface{}
		expectedVal interface{}
		expectedOk  bool
	}{
		{
			"Empty Selector",
			[]string{},
			standardRecord,
			standardRecord,
			true,
		},
		{
			"String Field",
			[]string{"testkey"},
			standardRecord,
			"testval",
			true,
		},
		{
			"Map Field",
			[]string{"testnested"},
			standardRecord,
			standardRecord["testnested"],
			true,
		},
		{
			"Nested",
			[]string{"testnested", "testnestedkey"},
			standardRecord,
			"testnestedval",
			true,
		},
		{
			"Missing",
			[]string{"invalid"},
			standardRecord,
			nil,
			false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			entry := NewEntry()
			entry.Record = tc.record

			val, ok := entry.Get(tc.selector)
			if !assert.Equal(t, tc.expectedOk, ok) {
				return
			}
			if !assert.Equal(t, tc.expectedVal, val) {
				return
			}
		})
	}
}

func TestFieldSelectorDelete(t *testing.T) {
	newStandardRecord := func() map[string]interface{} {
		standardRecord := map[string]interface{}{
			"testkey": "testval",
			"testnested": map[string]interface{}{
				"testnestedkey": "testnestedval",
			},
		}
		return standardRecord
	}

	cases := []struct {
		name             string
		selector         FieldSelector
		record           interface{}
		expectedRecord   interface{}
		expectedReturned interface{}
		expectedOk       bool
	}{
		{
			"Simple",
			[]string{"deletedKey"},
			map[string]interface{}{
				"deletedKey": "deletedVal",
			},
			map[string]interface{}{},
			"deletedVal",
			true,
		},
		{
			"Empty Selector Empty Record",
			[]string{},
			nil,
			map[string]interface{}{},
			nil,
			true,
		},
		{
			"Empty selector Nonempty record",
			[]string{},
			newStandardRecord(),
			map[string]interface{}{},
			newStandardRecord(),
			true,
		},
		{
			"Empty map",
			[]string{"deletedKey"},
			map[string]interface{}{},
			map[string]interface{}{},
			nil,
			false,
		},
		{
			"Delete Nested Key",
			[]string{"testnested", "testnestedkey"},
			newStandardRecord(),
			map[string]interface{}{
				"testkey":    "testval",
				"testnested": map[string]interface{}{},
			},
			"testnestedval",
			true,
		},
		{
			"Delete Nested Map",
			[]string{"testnested"},
			newStandardRecord(),
			map[string]interface{}{
				"testkey": "testval",
			},
			map[string]interface{}{
				"testnestedkey": "testnestedval",
			},
			true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			entry := NewEntry()
			entry.Record = tc.record

			deleted, ok := entry.Delete(tc.selector)
			assert.Equal(t, tc.expectedOk, ok)
			assert.Equal(t, tc.expectedReturned, deleted)
			assert.Equal(t, tc.expectedRecord, entry.Record)
		})
	}
}

func TestFieldSelectorSet(t *testing.T) {
	standardRecord := map[string]interface{}{
		"testkey": "testval",
		"testnested": map[string]interface{}{
			"testnestedkey": "testnestedval",
		},
	}

	cases := []struct {
		name        string
		selector    FieldSelector
		record      interface{}
		setTo       interface{}
		expectedVal interface{}
	}{
		{
			"Empty Selector Empty Record",
			[]string{},
			nil,
			"inserted",
			"inserted",
		},
		{
			"Empty selector Nonempty record",
			[]string{},
			standardRecord,
			"inserted",
			"inserted",
		},
		{
			"Empty Map",
			[]string{"insertedKey"},
			map[string]interface{}{},
			"insertedVal",
			map[string]interface{}{"insertedKey": "insertedVal"},
		},
		{
			"Nested Map",
			[]string{"testnested", "insertedKey"},
			standardRecord,
			"insertedVal",
			map[string]interface{}{
				"testkey": "testval",
				"testnested": map[string]interface{}{
					"testnestedkey": "testnestedval",
					"insertedKey":   "insertedVal",
				},
			},
		},
		{
			"Overwrite Nested Map",
			[]string{"testnested"},
			standardRecord,
			"insertedVal",
			map[string]interface{}{
				"testkey":    "testval",
				"testnested": "insertedVal",
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			entry := NewEntry()
			entry.Record = tc.record

			entry.Set(tc.selector, tc.setTo)
			assert.Equal(t, tc.expectedVal, entry.Record)
		})
	}
}

func TestFieldSelectorDecode(t *testing.T) {
	type decodeTarget struct {
		Fs    FieldSelector
		Fsptr *FieldSelector
		S     string
	}

	cases := []struct {
		name        string
		input       map[string]interface{}
		expected    decodeTarget
		expectedErr bool
	}{
		{
			"simple",
			map[string]interface{}{"fs": "test"},
			decodeTarget{
				Fs: FieldSelector([]string{"test"}),
			},
			false,
		},
		{
			"multi",
			map[string]interface{}{"fs": []string{"test1", "test2"}},
			decodeTarget{
				Fs: FieldSelector([]string{"test1", "test2"}),
			},
			false,
		},
		{
			"simple pointer",
			map[string]interface{}{"fsptr": "test"},
			decodeTarget{
				Fsptr: func() *FieldSelector {
					var fs FieldSelector = FieldSelector([]string{"test"})
					return &fs
				}(),
			},
			false,
		},
		{
			"multi pointer",
			map[string]interface{}{"fsptr": []string{"test1", "test2"}},
			decodeTarget{
				Fsptr: func() *FieldSelector {
					var fs FieldSelector = FieldSelector([]string{"test1", "test2"})
					return &fs
				}(),
			},
			false,
		},
		{
			"bad type",
			map[string]interface{}{"fsptr": []byte("test1")},
			decodeTarget{},
			true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var target decodeTarget
			cfg := &mapstructure.DecoderConfig{
				Result:     &target,
				DecodeHook: FieldSelectorDecoder,
			}

			decoder, err := mapstructure.NewDecoder(cfg)
			if !assert.NoError(t, err) {
				t.FailNow()
			}

			err = decoder.Decode(tc.input)
			if tc.expectedErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}

			assert.Equal(t, tc.expected, target)
		})
	}
}

func TestFieldSelectorUnmarshalJSON(t *testing.T) {
	cases := []struct {
		name     string
		input    []byte
		expected FieldSelector
	}{
		{
			"simple string",
			[]byte(`"message"`),
			FieldSelector([]string{"message"}),
		},
		{
			"nested string",
			[]byte(`["message","nested"]`),
			FieldSelector([]string{"message", "nested"}),
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var f FieldSelector
			err := json.Unmarshal(tc.input, &f)
			require.NoError(t, err)

			require.Equal(t, tc.expected, f)
		})
	}
}

func TestFieldSelectorMarshalJSON(t *testing.T) {
	cases := []struct {
		name     string
		input    FieldSelector
		expected []byte
	}{
		{
			"simple string",
			FieldSelector([]string{"message"}),
			[]byte(`"message"`),
		},
		{
			"nested string",
			FieldSelector([]string{"message", "nested"}),
			[]byte(`["message","nested"]`),
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			res, err := json.Marshal(tc.input)
			require.NoError(t, err)

			require.Equal(t, tc.expected, res)
		})
	}
}

func TestFieldSelectorUnmarshalYAML(t *testing.T) {
	cases := []struct {
		name     string
		input    []byte
		expected FieldSelector
	}{
		{
			"simple string",
			[]byte(`"message"`),
			FieldSelector([]string{"message"}),
		},
		{
			"unquoted string",
			[]byte(`message`),
			FieldSelector([]string{"message"}),
		},
		{
			"nested string",
			[]byte(`["message","nested"]`),
			FieldSelector([]string{"message", "nested"}),
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var f FieldSelector
			err := yaml.Unmarshal(tc.input, &f)
			require.NoError(t, err)

			require.Equal(t, tc.expected, f)
		})
	}
}

func TestFieldSelectorMarshalYAML(t *testing.T) {
	cases := []struct {
		name     string
		input    interface{}
		expected []byte
	}{
		{
			"simple string",
			FieldSelector([]string{"message"}),
			[]byte("message\n"),
		},
		{
			"nested string",
			FieldSelector([]string{"message", "nested"}),
			[]byte("- message\n- nested\n"),
		},
		{
			"nested string inline",
			struct {
				Field FieldSelector `yaml:"field,flow"`
			}{
				FieldSelector([]string{"message", "nested"}),
			},
			[]byte("field: [message, nested]\n"),
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			res, err := yaml.Marshal(tc.input)
			require.NoError(t, err)

			require.Equal(t, tc.expected, res)
		})
	}
}
