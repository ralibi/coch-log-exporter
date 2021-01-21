package main

import (
	"flag"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"log"
	"main/pkg/client"
	"main/pkg/metric"
	"net/http"
	"strings"
	"time"
)

var (
	addr      = flag.String("listen-address", ":8090", "The address to listen on for HTTP requests.")
	sourceURL = flag.String("source-url", "http://10.11.12.13:9200/some-index-*/_search?size=10000", "Elasticsearch source url.")
	delimiter = flag.String("delimiter", "__", "Config file id delimiter.")
	interval  = flag.Int("interval", 2, "Request to Elasticsearch url interval in second")
	labels    = flag.String("labels", "label_1, label_2, label_3, label_4, label_5, label_6", "The labels that will be exported.")
	numLabels = len(strings.Split(*labels, ","))
)

var (
	cochGauge   = &prometheus.GaugeVec{}
	cochInvalid = prometheus.NewGauge(prometheus.GaugeOpts{})
)

func init() {
	flag.Parse()

	cochGauge = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "conformance_checker_gauge",
		Help: "Conformance Checker Gauge",
	}, strings.Split(strings.ReplaceAll(*labels, " ", ""), ","))
	cochInvalid = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "conformance_checker_invalid_config_file_id_gauge",
		Help: "Conformance Checker Invalid Config File ID Gauge",
	})

	// Register the summary and the histogram with Prometheus's default registry.
	prometheus.MustRegister(cochGauge)
	prometheus.MustRegister(cochInvalid)
	// Add Go module build info.
	prometheus.MustRegister(prometheus.NewBuildInfoCollector())
}

func recordMetric(duration int, fn func()) {
	go func() {
		for {
			fn()
			time.Sleep(time.Duration(duration) * time.Second)
		}
	}()
}

func main() {
	recordMetric(*interval, collect)
	http.Handle("/metrics", promhttp.Handler())
	log.Fatal(http.ListenAndServe(*addr, nil))
}

func collect() {
	reqBody := []byte(`{"aggs":{"CONFIG_FILE_ID":{"terms":{"field":"config_file_id.keyword","order":{"1":"desc"},"size":10000},"aggs":{"1":{"cardinality":{"field":"metric"}},"TIMESTAMP":{"terms":{"field":"timestamp","order":{"_key":"desc"},"size":1},"aggs":{"KEY":{"terms":{"field":"key.keyword","order":{"1":"desc"},"size":10000},"aggs":{"1":{"cardinality":{"field":"metric"}},"VALUE":{"terms":{"field":"value.keyword","order":{"1":"desc"},"size":10000},"aggs":{"1":{"cardinality":{"field":"metric"}},"TYPE":{"terms":{"field":"type.keyword","order":{"1":"desc"},"size":10000},"aggs":{"1":{"cardinality":{"field":"metric"}},"MAX":{"max":{"field":"metric"}},"MIN":{"min":{"field":"metric"}}}}}}}}}}}}},"size":0,"_source":{"excludes":[]},"stored_fields":["*"],"script_fields":{},"docvalue_fields":[{"field":"@timestamp","format":"date_time"},{"field":"timestamp","format":"date_time"}],"query":{"bool":{"must":[],"filter":[{"match_all":{}},{"range":{"@timestamp":{"gte":"now-8m","lte":"now"}}}],"should":[],"must_not":[]}}}`)
	c := client.ClientElasticsearch{RequestBody: reqBody, SourceURL: *sourceURL}
	jsonBlob, _ := c.GetAggregationRecord()
	cms, numInvalid := metric.ParseToCochMetric(jsonBlob, *delimiter, numLabels)
	cochGauge.Reset()
	for _, cm := range cms {
		cochGauge.WithLabelValues(
			cm.ConfigFileIDs...,
		).Set(cm.Metric)
	}
	cochInvalid.Set(float64(numInvalid))
}
