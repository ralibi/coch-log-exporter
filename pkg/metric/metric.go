package metric

import (
	"encoding/json"
	"fmt"
	"log"
	"strings"
)

type CochMetric struct {
	Timestamp     int
	Metric        float64
	ConfigFileIDs []string
}

type CochConfigFileLine struct {
	ConfigFileID string
	Key          string
	Value        string
	Type         string
	Metric       float64
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

func calcBucketMetric(b interface{}) float64 {
	min := b.(map[string]interface{})["MIN"].(map[string]interface{})["value"].(float64)
	max := b.(map[string]interface{})["MAX"].(map[string]interface{})["value"].(float64)
	count := b.(map[string]interface{})["1"].(map[string]interface{})["value"].(float64)
	return ((min + max) * count) / 2
}

func getLines(configFile interface{}) []CochConfigFileLine {
	lines := []CochConfigFileLine{}

	timestampBuckets := getBuckets(configFile, "TIMESTAMP")
	for _, k := range getBuckets(timestampBuckets[0], "KEY") {
		kval := getBucketValue(k).(string)
		for _, v := range getBuckets(k, "VALUE") {
			vval := getBucketValue(v).(string)
			for _, t := range getBuckets(v, "TYPE") {
				tval := getBucketValue(t).(string)
				cfl := CochConfigFileLine{
					ConfigFileID: getBucketValue(configFile).(string),
					Key:          kval,
					Value:        vval,
					Type:         tval,
					Metric:       calcBucketMetric(t),
				}

				lines = append(lines, cfl)
			}
		}
	}

	return lines
}

func ParseToCochMetric(jsonBlob []byte, delimiter string, numLabels int) ([]CochMetric, int) {
	j := make(map[string]interface{})
	err := json.Unmarshal(jsonBlob, &j)
	if err != nil {
		log.Fatal(err)
	}

	cfBuckets := getBuckets(j["aggregations"], "CONFIG_FILE_ID")

	arr := []CochMetric{}
	numInvalid := 0

	for _, cf := range cfBuckets {
		sids, err := splitConfigFileID(getBucketValue(cf).(string), delimiter, numLabels)
		if err != nil {
			numInvalid++
			continue
		}

		cm := CochMetric{
			Timestamp:     int(getBucketValue(getBuckets(cf, "TIMESTAMP")[0]).(float64)),
			Metric:        avgLinesMetric(getLines(cf)),
			ConfigFileIDs: sids,
		}
		arr = append(arr, cm)
	}

	return arr, numInvalid
}
