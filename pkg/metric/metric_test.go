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

func TestAggregatedMetric(t *testing.T) {
	cm := &CochMetric{
		Metric:       875.625,
		BothCount:    15,
		StorageCount: 3,
		VMCount:      0,
	}
	want := 1015003000087.5625
	got := cm.AggregatedMetric()
	assert.Equal(t, got, want)
}

func TestSplitConfigFileID(t *testing.T) {
	n := "project-a__module-json__service-json__ansible-json__v1_0_1__-opt-config-json"
	want := []string{"project-a", "module-json", "service-json", "ansible-json", "v1_0_1", "-opt-config-json"}
	got, _ := splitConfigFileID(n, "__", 6)
	assert.Equal(t, got, want)
}

func TestDiffStatus(t *testing.T) {
	ms := []float64{1, 1.0, 1.6, 500, 88.88, 1000.0, 1000.0001, 1001}
	wants := []float64{2, 2, 1, 1, 1, 3, 1, 4}
	for i, m := range ms {
		t.Run(fmt.Sprintf("Should got correct diff status for %v", m), func(t *testing.T) {
			got := diffStatus(m)
			assert.Equal(t, got, wants[i])
		})
	}
}

func TestParseToCochMetric(t *testing.T) {
	abs, _ := filepath.Abs("./../../examples/respond.json")
	c := client.ClientFile{FileAbsPath: abs}
	jsonBlob, _ := c.GetAggregationRecord()

	want := []CochMetric{
		{
			Timestamp:     1613630700000,
			Metric:        875.625,
			ConfigFileIDs: []string{"project-a", "terraform-module", "v1_4_7", "project-a-pilot-01", "provisioner-xyz", "-etc-another-config-conf"},
			BothCount:     4,
			StorageCount:  3,
			VMCount:       1,
		},
		{
			Timestamp:     1613630700000,
			Metric:        1001,
			ConfigFileIDs: []string{"project-a", "terraform-module", "v1_4_7", "project-a-pilot-01", "provisioner-xyz", "-var-lib-config-auto-conf"},
			BothCount:     20,
			StorageCount:  0,
			VMCount:       0,
		},
	}
	cms, optimals, _ := ParseToCochMetric(jsonBlob, "__", 6)
	for i, got := range cms {
		t.Run(fmt.Sprintf("Should got correct timestamp at %v", i), func(t *testing.T) {
			assert.Equal(t, got.Timestamp, want[i].Timestamp)
		})
		t.Run(fmt.Sprintf("Should got correct metric at %v", i), func(t *testing.T) {
			assert.Equal(t, got.Metric, want[i].Metric)
		})
		t.Run(fmt.Sprintf("Should got correct config_file_id at %v", i), func(t *testing.T) {
			assert.Equal(t, got.ConfigFileIDs, want[i].ConfigFileIDs)
		})
		t.Run(fmt.Sprintf("Should got correct both count at %v", i), func(t *testing.T) {
			assert.Equal(t, got.BothCount, want[i].BothCount)
		})
		t.Run(fmt.Sprintf("Should got correct storage count at %v", i), func(t *testing.T) {
			assert.Equal(t, got.StorageCount, want[i].StorageCount)
		})
		t.Run(fmt.Sprintf("Should got correct vm count at %v", i), func(t *testing.T) {
			assert.Equal(t, got.VMCount, want[i].VMCount)
		})
	}

	want = []CochMetric{
		{
			Timestamp:     1613630700000,
			Metric:        900.8,
			ConfigFileIDs: []string{"project-a", "terraform-module--optimal", "v1_4_7", "application-abc", "provisioner-xyz-0", "-etc-another-config-conf"},
			BothCount:     7,
			StorageCount:  2,
			VMCount:       1,
		},
	}
	for i, got := range optimals {
		t.Run(fmt.Sprintf("Should got correct timestamp at %v", i), func(t *testing.T) {
			assert.Equal(t, got.Timestamp, want[i].Timestamp)
		})
		t.Run(fmt.Sprintf("Should got correct metric at %v", i), func(t *testing.T) {
			assert.Equal(t, got.Metric, want[i].Metric)
		})
		t.Run(fmt.Sprintf("Should got correct config_file_id at %v", i), func(t *testing.T) {
			assert.Equal(t, got.ConfigFileIDs, want[i].ConfigFileIDs)
		})
		t.Run(fmt.Sprintf("Should got correct both count at %v", i), func(t *testing.T) {
			assert.Equal(t, got.BothCount, want[i].BothCount)
		})
		t.Run(fmt.Sprintf("Should got correct storage count at %v", i), func(t *testing.T) {
			assert.Equal(t, got.StorageCount, want[i].StorageCount)
		})
		t.Run(fmt.Sprintf("Should got correct vm count at %v", i), func(t *testing.T) {
			assert.Equal(t, got.VMCount, want[i].VMCount)
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
	wants := []string{"STORAGE_OPTIMAL_CONFIGURATION", "VM_OPTIMAL_CONFIGURATION", "DIFF_CONFIGURATION"}
	for i, input := range inputs {
		t.Run(fmt.Sprintf("Should got correct Config Filt Type at %v", i), func(t *testing.T) {
			got := configFileType(input)
			assert.Equal(t, got, wants[i])
		})
	}
}
