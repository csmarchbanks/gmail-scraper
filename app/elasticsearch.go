package main

import (
	"encoding/base64"
	"log"
	"os"
	"strings"
	"time"

	"github.com/olivere/elastic"
	"github.com/prometheus/client_golang/prometheus"
	"golang.org/x/net/context"
	gmail "google.golang.org/api/gmail/v1"
)

type Email struct {
	To      string `json:"to"`
	From    string `json:"from"`
	Subject string `json:"subject"`
	Content string `json:"content"`
}

var emailIndexHistogram = prometheus.NewHistogram(prometheus.HistogramOpts{
	Name: "email_scraper_index_email_seconds",
	Help: "Time taken to index an email to elasticsearch",
})

const (
	index   = "emails"
	mapping = `{
	"mappings":{
		"email":{
			"properties":{
				"to":{
					"type":"keyword"
				},
				"from":{
					"type":"keyword"
				},
				"subject":{
					"type":"text"
				},
				"content":{
					"type":"text"
				}
			}
		}
	}
}`
)

var elasticClient *elastic.Client

func NewEmail(msg *gmail.Message) (Email, error) {
	if msg == nil || msg.Payload == nil || msg.Payload.Body == nil {
		log.Fatalf("WTF: %v\n", msg)
	}
	data := msg.Payload.Body.Data
	content, err := base64.URLEncoding.DecodeString(data)
	if err != nil {
		return Email{}, err
	}
	return Email{
		To:      parseHeader(msg, "To"),
		From:    parseHeader(msg, "From"),
		Subject: parseHeader(msg, "Subject"),
		Content: string(content),
	}, nil
}

func indexEmail(ctx context.Context, id string, email Email) error {
	start := time.Now()
	defer func() {
		emailIndexHistogram.Observe(time.Since(start).Seconds())
	}()
	_, err := elasticClient.Index().
		Index(index).
		Type("email").
		Id(id).
		BodyJson(email).
		Do(ctx)
	return err
}

func elasticURLs() []string {
	urls := os.Getenv("ELASTICSEARCH_URLS")
	if urls == "" {
		return []string{}
	}
	return strings.Split(urls, ",")
}

func init() {
	// metrics
	prometheus.MustRegister(emailIndexHistogram)

	// elasticsearch client
	ctx := context.Background()
	var err error
	elasticClient, err = elastic.NewClient(
		elastic.SetURL(elasticURLs()...))
	if err != nil {
		log.Fatalf("Could not make elasticsearch client: %v\n", err)
	}

	// elasticsearch indices
	exists, err := elasticClient.IndexExists(index).Do(ctx)
	if err != nil {
		log.Fatalf("Could not see if index exists: %v\n", err)
	}
	if !exists {
		// Create a new index.
		createIndex, err := elasticClient.CreateIndex(index).BodyString(mapping).Do(ctx)
		if err != nil {
			log.Fatalf("Could not create necessary index: %v\n", err)
		}
		if !createIndex.Acknowledged {
			log.Println("Index creation not acknowledged")
		}
	}
}
