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
			ConfigFileIDs: []string{"project-a", "terraform-module", "v1_4_7", "project-a-pilot-01", "provisioner-xyz", "-etc-another-config-conf"},
		},
		{
			Timestamp:     1613630700000,
			Metric:        1001,
			ConfigFileIDs: []string{"project-a", "terraform-module", "v1_4_7", "project-a-pilot-01", "provisioner-xyz", "-var-lib-config-auto-conf"},
		},
	}
	cms, _, _ := ParseToCochMetric(jsonBlob, "__", 6)
	for i, got := range cms {
		t.Run(fmt.Sprintf("Should got correct bucket at %v", i), func(t *testing.T) {
			assert.Equal(t, got.Timestamp, want[i].Timestamp)
			assert.Equal(t, got.Metric, want[i].Metric)
			assert.Equal(t, got.ConfigFileIDs, want[i].ConfigFileIDs)
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

func TestConfigFileType(t *testing.T) {
	inputs := [][]string{
		[]string{"project-a", "terraform-module--optimal", "v1_4_7", "optimal", "ansible-xyz", "-etc-another-config-conf"},
		[]string{"project-a", "terraform-module--optimal", "v1_4_7", "project-a-pilot-01", "ansible-xyz", "-etc-another-config-conf"},
		[]string{"project-a", "terraform-module", "v1_4_7", "project-a-pilot-01", "ansible-xyz", "-etc-another-config-conf"},
	}
	wants := []string{"YGGDRASIL_OPTIMAL_CONFIGURATION", "VM_OPTIMAL_CONFIGURATION", "DIFF_CONFIGURATION"}
	for i, input := range inputs {
		t.Run(fmt.Sprintf("Should got correct Config Filt Type at %v", i), func(t *testing.T) {
			got := configFileType(input)
			assert.Equal(t, got, wants[i])
		})
	}
}
