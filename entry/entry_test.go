package entry

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestRead(t *testing.T) {
	testEntry := &Entry{
		Record: map[string]interface{}{
			"string_field": "string_val",
			"byte_field":   []byte(`test`),
			"map_string_interface_field": map[string]interface{}{
				"nested": "interface_val",
			},
			"map_string_interface_nonstring_field": map[string]interface{}{
				"nested": 111,
			},
			"map_string_string_field": map[string]string{
				"nested": "string_val",
			},
			"map_interface_interface_field": map[interface{}]interface{}{
				"nested": "interface_val",
			},
			"map_interface_interface_nonstring_key_field": map[interface{}]interface{}{
				100: "interface_val",
			},
			"map_interface_interface_nonstring_value_field": map[interface{}]interface{}{
				"nested": 100,
			},
		},
	}

	t.Run("field not exist error", func(t *testing.T) {
		var s string
		err := testEntry.Read(NewRecordField("nonexistant_field"), &s)
		require.Error(t, err)
	})

	t.Run("unsupported type error", func(t *testing.T) {
		var s **string
		err := testEntry.Read(NewRecordField("string_field"), &s)
		require.Error(t, err)
	})

	t.Run("string", func(t *testing.T) {
		var s string
		err := testEntry.Read(NewRecordField("string_field"), &s)
		require.NoError(t, err)
		require.Equal(t, "string_val", s)
	})

	t.Run("string error", func(t *testing.T) {
		var s string
		err := testEntry.Read(NewRecordField("map_string_interface_field"), &s)
		require.Error(t, err)
	})

	t.Run("map[string]interface{}", func(t *testing.T) {
		var m map[string]interface{}
		err := testEntry.Read(NewRecordField("map_string_interface_field"), &m)
		require.NoError(t, err)
		require.Equal(t, map[string]interface{}{"nested": "interface_val"}, m)
	})

	t.Run("map[string]interface{} error", func(t *testing.T) {
		var m map[string]interface{}
		err := testEntry.Read(NewRecordField("string_field"), &m)
		require.Error(t, err)
	})

	t.Run("map[string]string from map[string]interface{}", func(t *testing.T) {
		var m map[string]string
		err := testEntry.Read(NewRecordField("map_string_interface_field"), &m)
		require.NoError(t, err)
		require.Equal(t, map[string]string{"nested": "interface_val"}, m)
	})

	t.Run("map[string]string from map[string]interface{} err", func(t *testing.T) {
		var m map[string]string
		err := testEntry.Read(NewRecordField("map_string_interface_nonstring_field"), &m)
		require.Error(t, err)
	})

	t.Run("map[string]string from map[interface{}]interface{}", func(t *testing.T) {
		var m map[string]string
		err := testEntry.Read(NewRecordField("map_interface_interface_field"), &m)
		require.NoError(t, err)
		require.Equal(t, map[string]string{"nested": "interface_val"}, m)
	})

	t.Run("map[string]string from map[interface{}]interface{} nonstring key error", func(t *testing.T) {
		var m map[string]string
		err := testEntry.Read(NewRecordField("map_interface_interface_nonstring_key_field"), &m)
		require.Error(t, err)
	})

	t.Run("map[string]string from map[interface{}]interface{} nonstring value error", func(t *testing.T) {
		var m map[string]string
		err := testEntry.Read(NewRecordField("map_interface_interface_nonstring_value_field"), &m)
		require.Error(t, err)
	})

	t.Run("interface{} from any", func(t *testing.T) {
		var i interface{}
		err := testEntry.Read(NewRecordField("map_interface_interface_field"), &i)
		require.NoError(t, err)
		require.Equal(t, map[interface{}]interface{}{"nested": "interface_val"}, i)
	})

	t.Run("string from []byte", func(t *testing.T) {
		var i string
		err := testEntry.Read(NewRecordField("byte_field"), &i)
		require.NoError(t, err)
		require.Equal(t, "test", i)
	})
}

func TestCopy(t *testing.T) {
	entry := New()
	entry.Severity = Severity(0)
	entry.Timestamp = time.Time{}
	entry.Record = "test"
	entry.Labels = map[string]string{"label": "value"}
	entry.Tags = []string{"tag"}
	copy := entry.Copy()

	entry.Severity = Severity(1)
	entry.Timestamp = time.Now()
	entry.Record = "new"
	entry.Labels = map[string]string{"label": "new value"}
	entry.Tags = []string{"new tag"}

	require.Equal(t, time.Time{}, copy.Timestamp)
	require.Equal(t, Severity(0), copy.Severity)
	require.Equal(t, []string{"tag"}, copy.Tags)
	require.Equal(t, map[string]string{"label": "value"}, copy.Labels)
	require.Equal(t, "test", copy.Record)
}

func TestFieldFromString(t *testing.T) {
	cases := []struct {
		name          string
		input         string
		output        Field
		expectedError bool
	}{
		{
			"SimpleRecord",
			"test",
			Field{RecordField{[]string{"test"}}},
			false,
		},
		{
			"PrefixedRecord",
			"$.test",
			Field{RecordField{[]string{"test"}}},
			false,
		},
		{
			"FullPrefixedRecord",
			"$record.test",
			Field{RecordField{[]string{"test"}}},
			false,
		},
		{
			"SimpleLabel",
			"$labels.test",
			Field{LabelField{"test"}},
			false,
		},
		{
			"LabelsTooManyFields",
			"$labels.test.bar",
			Field{},
			true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f, err := fieldFromString(tc.input)
			if tc.expectedError {
				require.Error(t, err)
				return
			}

			require.Equal(t, tc.output, f)
		})
	}
}
