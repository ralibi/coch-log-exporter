package metric

import (
	"encoding/json"
	"fmt"
	"log"
	"main/pkg/client"
	"testing"

	"github.com/go-playground/assert/v2"
	"path/filepath"
)

func TestSplitConfigFileID(t *testing.T) {
	n := "project-a__module-json__service-json__ansible-json__v1_0_1__-opt-config-json"
	want := []string{"project-a", "module-json", "service-json", "ansible-json", "v1_0_1", "-opt-config-json"}
	got, _ := splitConfigFileID(n, "__", 6)
	assert.Equal(t, got, want)
}

func TestParseToCochMetric(t *testing.T) {
	abs, _ := filepath.Abs("./../../examples/respond.json")
	c := client.ClientFile{FileAbsPath: abs}
	jsonBlob, _ := c.GetAggregationRecord()

	want := []CochMetric{
		{
			Timestamp:     1613630700000,
			Metric:        1001,
			ConfigFileIDs: []string{"project-a", "terraform-module", "project-a-pilot-01", "service-xyz", "v1_4_7", "-etc-another-config-conf"},
		},
		{
			Timestamp:     1613630700000,
			Metric:        1001,
			ConfigFileIDs: []string{"project-a", "terraform-module", "project-a-pilot-01", "service-xyz", "v1_4_7", "-var-lib-config-auto-conf"},
		},
	}
	cms, _ := ParseToCochMetric(jsonBlob, "__", 6)
	for i, got := range cms {
		t.Run(fmt.Sprintf("Should got correct bucket at %v", i), func(t *testing.T) {
			assert.Equal(t, got, want[i])
		})
	}
}

func TestGetBucketValue(t *testing.T) {
	j := map[string]interface{}{}
	err := json.Unmarshal([]byte(`{"key": "abc", "KEYWORD": {"buckets": [{"key": "bar"}, {"fizz": "buzz"}]}}`), &j)
	if err != nil {
		log.Fatal(err)
	}
	want := "abc"
	got := getBucketValue(j).(string)
	assert.Equal(t, got, want)
}

func TestGetBuckets(t *testing.T) {
	j := map[string]interface{}{}
	err := json.Unmarshal([]byte(`{"key": "abc", "KEYWORD": {"buckets": [{"foo": "bar"}, {"fizz": "buzz"}]}}`), &j)
	if err != nil {
		log.Fatal(err)
	}
	want := []interface{}{map[string]interface{}{"foo": "bar"}, map[string]interface{}{"fizz": "buzz"}}
	got := getBuckets(j, "KEYWORD")
	for i, b := range got {
		t.Run(fmt.Sprintf("Should got correct bucket at %v", i), func(t *testing.T) {
			assert.Equal(t, b.(map[string]interface{}), want[i])
		})
	}
}
