# COCH Log Exporter

```
Usage:
  -delimiter string
      Config file id delimiter. (default "__")
  -interval int
      Request to Elasticsearch url interval in second (default 2)
  -labels string
      The labels that will be exported. (default "label_1, label_2, label_3, label_4, label_5, label_6")
  -listen-address string
      The address to listen on for HTTP requests. (default ":8090")
  -source-url string
      Elasticsearch source url. (default "http://10.11.12.13:9200/some-index-*/_search?size=10000")
```
