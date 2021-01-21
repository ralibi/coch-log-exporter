package client

import (
	"bytes"
	"io/ioutil"
	"log"
	"net/http"
	"time"
)

// Client interface used to abstract and mock several function
type Client interface {
	GetAggregationRecord() ([]byte, error)
}

// ClientFile ...
type ClientFile struct {
	FileAbsPath string
}

func (c *ClientFile) GetAggregationRecord() ([]byte, error) {
	json, err := ioutil.ReadFile(c.FileAbsPath)
	return json, err
}

// ClientElasticsearch ...
type ClientElasticsearch struct {
	RequestBody []byte
	SourceURL   string
}

func (c *ClientElasticsearch) GetAggregationRecord() ([]byte, error) {
	client := &http.Client{}
	client.Timeout = time.Second * 10

	req, err := http.NewRequest(http.MethodGet, c.SourceURL, bytes.NewBuffer(c.RequestBody))
	req.Header.Set("Content-Type", "application/json")
	if err != nil {
		log.Fatal(err)
	}

	resp, err := client.Do(req)
	if err != nil {
		log.Fatal(err)
	}

	json, err := ioutil.ReadAll(resp.Body)
	defer resp.Body.Close()
	if err != nil {
		log.Fatal(err)
	}

	return json, err
}
