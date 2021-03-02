package metric

import (
	"encoding/json"
	"fmt"
	"log"
	"math"
	"strings"
)

type CochConfigFileLine struct {
	ConfigFileID string
	KeyValueType string
	Metric       float64
}

type CochMetric struct {
	Timestamp     int
	Lines         []CochConfigFileLine
	Metric        float64
	ConfigFileIDs []string
	BothCount     float64
	StorageCount  float64
	VMCount       float64
}

type CochBucketMetric struct {
	Index     string
	Component string
	Metric    int
}

func (cm *CochMetric) AggregatedMetric() float64 {
	bc := cm.BothCount * math.Pow(10, 9)
	sc := cm.StorageCount * math.Pow(10, 6)
	vc := cm.VMCount * math.Pow(10, 3)
	mt := cm.Metric / 10.0

	return bc + sc + vc + mt
}

func splitConfigFileID(name, delimiter string, numLabels int) ([]string, error) {
	result := strings.Split(name, delimiter)
	if len(result) != numLabels {
		return []string{name}, fmt.Errorf("Name %v is not valid given delimiter %v", name, delimiter)
	}
	return result, nil
}

func getBucketValue(b interface{}) interface{} {
	return b.(map[string]interface{})["key"]
}

func getBuckets(v interface{}, keyword string) []interface{} {
	return v.(map[string]interface{})[keyword].(map[string]interface{})["buckets"].([]interface{})
}

func avgLinesMetric(lines []CochConfigFileLine) float64 {
	sum := 0.0
	for _, l := range lines {
		sum = sum + l.Metric
	}

	return sum / float64(len(lines))
}

func countMetric(lines []CochConfigFileLine) (float64, float64, float64, float64) {
	var bothCount, storageCount, vmCount, avg, sum float64
	for _, l := range lines {
		sum = sum + l.Metric

		switch l.Metric {
		case 1:
			vmCount = vmCount + 1
		case 1000:
			storageCount = storageCount + 1
		case 1001:
			bothCount = bothCount + 1
		}
	}

	avg = sum / float64(len(lines))
	return bothCount, storageCount, vmCount, avg
}

func calcBucketMetric(b interface{}, cfType string) float64 {
	switch cfType {
	case "DIFF_CONFIGURATION":
		min := b.(map[string]interface{})["MIN"].(map[string]interface{})["value"].(float64)
		max := b.(map[string]interface{})["MAX"].(map[string]interface{})["value"].(float64)
		count := b.(map[string]interface{})["1"].(map[string]interface{})["value"].(float64)
		return ((min + max) * count) / 2
	case "STORAGE_OPTIMAL_CONFIGURATION":
		return 1000
	case "VM_OPTIMAL_CONFIGURATION":
		return 1
	default:
		return 0
	}
}

func getLines(configFile interface{}, cfType string) []CochConfigFileLine {
	lines := []CochConfigFileLine{}
	timestampBuckets := getBuckets(configFile, "TIMESTAMP")

	for _, kvt := range getBuckets(timestampBuckets[0], "KEY_VALUE_TYPE") {
		tval := getBucketValue(kvt).(string)
		cfl := CochConfigFileLine{
			ConfigFileID: getBucketValue(configFile).(string),
			KeyValueType: tval,
			Metric:       calcBucketMetric(kvt, cfType),
		}
		lines = append(lines, cfl)
	}

	return lines
}

func ParseToCochMetric(jsonBlob []byte, delimiter string, numLabels int) ([]*CochMetric, []*CochMetric, int) {
	j := make(map[string]interface{})
	err := json.Unmarshal(jsonBlob, &j)
	if err != nil {
		log.Fatal(err)
	}

	cfBuckets := getBuckets(j["aggregations"], "CONFIG_FILE_ID")

	diffs := []*CochMetric{}
	storageOptimal := map[string]*CochMetric{}
	vmOptimal := map[string]*CochMetric{}
	numInvalid := 0

	for _, cf := range cfBuckets {
		cfid := getBucketValue(cf).(string)
		sids, err := splitConfigFileID(cfid, delimiter, numLabels)
		if err != nil {
			numInvalid++
			continue
		}

		cft := configFileType(sids)
		lines := getLines(cf, cft)
		switch cft {
		case "DIFF_CONFIGURATION":
			bCount, sCount, vCount, avg := countMetric(lines)
			diff := &CochMetric{
				Timestamp:     int(getBucketValue(getBuckets(cf, "TIMESTAMP")[0]).(float64)),
				Lines:         lines,
				Metric:        avg,
				BothCount:     bCount,
				StorageCount:  sCount,
				VMCount:       vCount,
				ConfigFileIDs: sids,
			}
			diffs = append(diffs, diff)
		case "STORAGE_OPTIMAL_CONFIGURATION":
			storageOptimal[cfid] = &CochMetric{
				Timestamp:     int(getBucketValue(getBuckets(cf, "TIMESTAMP")[0]).(float64)),
				Lines:         lines,
				Metric:        0,
				ConfigFileIDs: sids,
			}
		case "VM_OPTIMAL_CONFIGURATION":
			vmOptimal[cfid] = &CochMetric{
				Timestamp:     int(getBucketValue(getBuckets(cf, "TIMESTAMP")[0]).(float64)),
				Lines:         lines,
				Metric:        0,
				ConfigFileIDs: sids,
			}
		}
	}

	optimals := mergeOptimals(vmOptimal, storageOptimal, delimiter)

	return diffs, optimals, numInvalid
}

func mergeOptimals(vmOptimal, storageOptimal map[string]*CochMetric, delimiter string) []*CochMetric {
	hostnameIndex := 3
	optimals := []*CochMetric{}
	for _, vm := range vmOptimal {
		cfids := make([]string, len(vm.ConfigFileIDs))
		copy(cfids, vm.ConfigFileIDs)
		cfids[hostnameIndex] = "optimal"
		storageCfid := strings.Join(cfids, delimiter)

		if storageOptimal[storageCfid] != nil {
			vm.BothCount, vm.StorageCount, vm.VMCount, vm.Metric = calcOptimalMetric(vm.Lines, storageOptimal[storageCfid].Lines)
		} else {
			vm.BothCount, vm.StorageCount, vm.VMCount, vm.Metric = 0, 0, float64(len(vm.Lines)), 1
		}

		optimals = append(optimals, vm)
	}

	return optimals
}

func calcOptimalMetric(vmLines, storageLines []CochConfigFileLine) (float64, float64, float64, float64) {
	kvt := map[string]float64{}

	for _, v := range append(vmLines, storageLines...) {
		kvt[v.KeyValueType] = kvt[v.KeyValueType] + v.Metric
	}

	var bothCount, storageCount, vmCount, avg, sum float64
	for _, m := range kvt {
		sum = sum + m

		switch m {
		case 1:
			vmCount = vmCount + 1
		case 1000:
			storageCount = storageCount + 1
		case 1001:
			bothCount = bothCount + 1
		}
	}

	avg = sum / float64(len(kvt))
	return bothCount, storageCount, vmCount, avg
}

func configFileType(labels []string) string {
	hostnameIndex := 3
	if strings.Contains(labels[1], "optimal") {
		if labels[hostnameIndex] == "optimal" {
			// "project-a", "terraform-module--optimal", "v1_4_7", "optimal", "ansible-xyz", "-etc-another-config-conf"
			return "STORAGE_OPTIMAL_CONFIGURATION"
		}
		// "project-a", "terraform-module--optimal", "v1_4_7", "project-a-pilot-01", "ansible-xyz", "-etc-another-config-conf"
		return "VM_OPTIMAL_CONFIGURATION"
	}
	// "project-a", "terraform-module", "v1_4_7", "project-a-pilot-01", "ansible-xyz", "-etc-another-config-conf"
	return "DIFF_CONFIGURATION"
}

func ParseToCochBucketMetric(jsonBlob []byte, index, component string) *CochBucketMetric {
	return &CochBucketMetric{
		Index:     index,
		Component: component,
		Metric:    strings.Count(string(jsonBlob), `"key":"`),
	}
}
