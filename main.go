package main

import (
	"flag"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"log"
	"main/pkg/client"
	"main/pkg/metric"
	"net/http"
	"path/filepath"
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
	cochGauge = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "conformance_checker_gauge",
		Help: "Conformance Checker Gauge",
	}, strings.Split(strings.ReplaceAll(*labels, " ", ""), ","))
	cochInvalid = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "conformance_checker_invalid_config_file_id_gauge",
		Help: "Conformance Checker Invalid Config File ID Gauge",
	})
)

func init() {
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
	flag.Parse()
	recordMetric(*interval, collect)
	http.Handle("/metrics", promhttp.Handler())
	log.Fatal(http.ListenAndServe(*addr, nil))
}

func collect() {
	absPath, _ := filepath.Abs("./pkg/client/request_body.json")
	c := client.ClientElasticsearch{RequestBodyAbsPath: absPath, SourceURL: *sourceURL}
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
