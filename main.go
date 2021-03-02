package main

import (
	"flag"
	"fmt"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"log"
	"main/pkg/client"
	"main/pkg/metric"
	"net/http"
	"strings"
	"sync"
	"time"
)

var (
	addr          = flag.String("listen-address", ":8090", "The address to listen on for HTTP requests.")
	sourceURL     = flag.String("source-url", "http://10.11.12.13:9200/", "Elasticsearch source url.")
	indexList     = flag.String("index-list", "index-1-*, index-2-*", "Elasticsearch index")
	componentList = flag.String("component-list", "component-1, component-2, component-3", "List of components")
	delimiter     = flag.String("delimiter", "__", "Config file id delimiter.")
	interval      = flag.Int("interval", 10, "Request to Elasticsearch url interval in second")
	labels        = flag.String("labels", "label_1, label_2, label_3, label_4, label_5, label_6", "The labels that will be exported.")
	numLabels     = len(strings.Split(*labels, ","))
)

var (
	cochGauge        = &prometheus.GaugeVec{}
	cochOptimalGauge = &prometheus.GaugeVec{}
	cochBucketsGauge = &prometheus.GaugeVec{}
	cochInvalid      = prometheus.NewGauge(prometheus.GaugeOpts{})
)

func init() {
	flag.Parse()

	cochGauge = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "conformance_checker_gauge",
		Help: "Conformance Checker Gauge",
	}, strings.Split(strings.ReplaceAll(*labels, " ", ""), ","))

	cochOptimalGauge = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "conformance_checker_optimal_gauge",
		Help: "Conformance Checker Optimal Gauge",
	}, strings.Split(strings.ReplaceAll(*labels, " ", ""), ","))

	cochBucketsGauge = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "conformance_checker_buckets_gauge",
		Help: "Conformance Checker Buckets Gauge",
	}, []string{"index", "component"})

	cochInvalid = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "conformance_checker_invalid_config_file_id_gauge",
		Help: "Conformance Checker Invalid Config File ID Gauge",
	})

	// Register the summary and the histogram with Prometheus's default registry.
	prometheus.MustRegister(cochGauge)
	prometheus.MustRegister(cochOptimalGauge)
	prometheus.MustRegister(cochBucketsGauge)

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
	diffs, optimals, buckets, numInvalid := searchElasticsearchAggregation()

	cochGauge.Reset()
	for _, diff := range diffs {
		cochGauge.WithLabelValues(
			diff.ConfigFileIDs...,
		).Set(diff.Metric)
	}
	cochOptimalGauge.Reset()
	for _, optimal := range optimals {
		cochOptimalGauge.WithLabelValues(
			optimal.ConfigFileIDs...,
		).Set(optimal.Metric)
	}
	cochBucketsGauge.Reset()
	for _, bucket := range buckets {
		cochBucketsGauge.WithLabelValues(
			bucket.Index,
			bucket.Component,
		).Set(float64(bucket.Metric))
	}

	cochInvalid.Set(float64(numInvalid))
}

func searchElasticsearchAggregation() ([]*metric.CochMetric, []*metric.CochMetric, []*metric.CochBucketMetric, int) {
	idxList := strings.Split(strings.ReplaceAll(*indexList, " ", ""), ",")
	compList := strings.Split(strings.ReplaceAll(*componentList, " ", ""), ",")

	resultDiffs := []*metric.CochMetric{}
	resultOptimals := []*metric.CochMetric{}
	resultBuckets := []*metric.CochBucketMetric{}
	resultNumInvalid := 0

	var wg sync.WaitGroup
	fmt.Printf("Requesting %v searches ...\n", len(idxList)*len(compList))

	for _, idx := range idxList {
		for _, comp := range compList {
			wg.Add(1)
			go func(index, component string) {
				defer wg.Done()
				fmt.Printf("Requesting index %v; component %v...\n", index, component)

				reqBody := generateRequestBody(component)
				source := fmt.Sprintf("%s/%s/_search?size=0", *sourceURL, index)
				c := client.ClientElasticsearch{RequestBody: reqBody, SourceURL: source}
				jsonBlob, _ := c.GetAggregationRecord()
				diffs, optimals, numInvalid := metric.ParseToCochMetric(jsonBlob, *delimiter, numLabels)
				bucket := metric.ParseToCochBucketMetric(jsonBlob, index, component)

				resultDiffs = append(resultDiffs, diffs...)
				resultOptimals = append(resultOptimals, optimals...)
				resultBuckets = append(resultBuckets, bucket)
				resultNumInvalid = resultNumInvalid + numInvalid
			}(idx, comp)
		}
	}
	wg.Wait()

	return resultDiffs, resultOptimals, resultBuckets, resultNumInvalid
}

func generateRequestBody(componentName string) []byte {
	return []byte(fmt.Sprintf(`{
	  "aggs": {
	    "CONFIG_FILE_ID": {
	      "terms": {
	        "field": "config_file_id.keyword",
	        "order": {
	          "1": "desc"
	        },
	        "size": 10000
	      },
	      "aggs": {
	        "1": {
	          "cardinality": {
	            "field": "metric"
	          }
	        },
	        "TIMESTAMP": {
	          "terms": {
	            "field": "timestamp",
	            "order": {
	              "_key": "desc"
	            },
	            "size": 1
	          },
	          "aggs": {
	            "KEY_VALUE_TYPE": {
	              "terms": {
	                "script": {
	                  "source": "doc['key.keyword'] + ' ' + doc['value.keyword'] + ' ' + doc['type.keyword']",
	                  "lang": "painless"
	                },
	                "size": 10000
	              },
	              "aggs": {
	                "1": {
	                  "cardinality": {
	                    "field": "metric"
	                  }
	                },
	                "MAX": {
	                  "max": {
	                    "field": "metric"
	                  }
	                },
	                "MIN": {
	                  "min": {
	                    "field": "metric"
	                  }
	                }
	              }
	            }
	          }
	        }
	      }
	    }
	  },
	  "size": 0,
	  "_source": {
	    "excludes": []
	  },
	  "stored_fields": [
	    "*"
	  ],
	  "script_fields": {},
	  "docvalue_fields": [
	    {
	      "field": "@timestamp",
	      "format": "date_time"
	    },
	    {
	      "field": "timestamp",
	      "format": "date_time"
	    }
	  ],
	  "query": {
	    "bool": {
	      "must": [],
	      "filter": [
	        {
	          "bool": {
	            "should": [
	              {
	                "query_string": {
	                  "fields": [
	                    "config_file_id.keyword"
	                  ],
	                  "query": "*%s*"
	                }
	              }
	            ],
	            "minimum_should_match": 1
	          }
	        },
	        {
	          "range": {
	            "@timestamp": {
	              "gte": "now-8m",
	              "lte": "now"
	            }
	          }
	        }
	      ],
	      "should": [],
	      "must_not": []
	    }
	  }
	}`, componentName))
}
