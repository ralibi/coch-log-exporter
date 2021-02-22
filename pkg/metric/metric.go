package metric

import (
	"encoding/json"
	"fmt"
	"log"
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

func calcBucketMetric(b interface{}, cfType string) float64 {
	switch cfType {
	case "DIFF_CONFIGURATION":
		min := b.(map[string]interface{})["MIN"].(map[string]interface{})["value"].(float64)
		max := b.(map[string]interface{})["MAX"].(map[string]interface{})["value"].(float64)
		count := b.(map[string]interface{})["1"].(map[string]interface{})["value"].(float64)
		return ((min + max) * count) / 2
	case "YGGDRASIL_OPTIMAL_CONFIGURATION":
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

func ParseToCochMetric(jsonBlob []byte, delimiter string, numLabels int) ([]CochMetric, []*CochMetric, int) {
	j := make(map[string]interface{})
	err := json.Unmarshal(jsonBlob, &j)
	if err != nil {
		log.Fatal(err)
	}

	cfBuckets := getBuckets(j["aggregations"], "CONFIG_FILE_ID")

	arr := []CochMetric{}
	yggOptimal := map[string]*CochMetric{}
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
			cm := CochMetric{
				Timestamp:     int(getBucketValue(getBuckets(cf, "TIMESTAMP")[0]).(float64)),
				Lines:         lines,
				Metric:        avgLinesMetric(lines),
				ConfigFileIDs: sids,
			}
			arr = append(arr, cm)
		case "YGGDRASIL_OPTIMAL_CONFIGURATION":
			yggOptimal[cfid] = &CochMetric{
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

	optimals := mergeOptimals(vmOptimal, yggOptimal, delimiter)

	return arr, optimals, numInvalid
}

func mergeOptimals(vmOptimal, yggOptimal map[string]*CochMetric, delimiter string) []*CochMetric {
	serviceNameIndex := 2
	optimals := []*CochMetric{}
	for _, vm := range vmOptimal {
		cfids := make([]string, len(vm.ConfigFileIDs))
		copy(cfids, vm.ConfigFileIDs)
		cfids[serviceNameIndex] = "optimal"
		yggCfid := strings.Join(cfids, delimiter)

		if yggOptimal[yggCfid] != nil {
			vm.Metric = calcOptimalMetric(vm.Lines, yggOptimal[yggCfid].Lines)
		} else {
			vm.Metric = 1
		}

		optimals = append(optimals, vm)
	}

	return optimals
}

func calcOptimalMetric(vmLines, yggLines []CochConfigFileLine) float64 {
	kvt := map[string]float64{}

	for _, v := range append(vmLines, yggLines...) {
		kvt[v.KeyValueType] = kvt[v.KeyValueType] + v.Metric
	}

	sum := 0.0
	for _, m := range kvt {
		sum = sum + m
	}

	return sum / float64(len(kvt))
}

func configFileType(labels []string) string {
	serviceNameIndex := 2
	if strings.Contains(labels[1], "optimal") {
		if labels[serviceNameIndex] == "optimal" {
			// "project-a", "terraform-module--optimal", "optimal", "ansible-xyz", "v1_4_7", "-etc-another-config-conf"
			return "YGGDRASIL_OPTIMAL_CONFIGURATION"
		}
		// "project-a", "terraform-module--optimal", "project-a-pilot-01", "ansible-xyz", "v1_4_7", "-etc-another-config-conf"
		return "VM_OPTIMAL_CONFIGURATION"
	}
	// "project-a", "terraform-module", "project-a-pilot-01", "ansible-xyz", "v1_4_7", "-etc-another-config-conf"
	return "DIFF_CONFIGURATION"
}
