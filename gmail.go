package main

import (
	"context"
	"log"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	gmail "google.golang.org/api/gmail/v1"
)

var (
	emailIdFetchHistogram = prometheus.NewHistogram(prometheus.HistogramOpts{
		Name: "email_scraper_get_id_seconds",
		Help: "Time taken to fetch email ids",
	})
	emailWriteHistogram = prometheus.NewHistogram(prometheus.HistogramOpts{
		Name: "email_scraper_writer_email_seconds",
		Help: "Time taken to get and index an email",
	})
)

const currentUser = "me"

// GetAllMessages returns a channel that will have the id of every message
//in an inbox put onto it
func GetAllMessages(service *gmail.Service) chan string {
	ch := make(chan string, 1000)
	go writeMessagesIdsToChannel(service, ch)
	return ch
}

func parseHeader(msg *gmail.Message, headerName string) string {
	for _, header := range msg.Payload.Headers {
		if header.Name == headerName {
			return header.Value
		}
	}
	return ""
}

func WriteAllMessages(ctx context.Context, service *gmail.Service, ch <-chan string) {
	for id := range ch {
		start := time.Now()
		r, err := service.Users.Messages.Get(currentUser, id).Do()
		if err != nil {
			log.Printf("Unable to get message. %v", err)
		}
		email := NewEmail(r)
		indexEmail(ctx, id, email)

		emailWriteHistogram.Observe(time.Since(start).Seconds())
	}
}

func writeMessagesIdsToChannel(service *gmail.Service, ch chan string) {
	pageToken := ""
	for {
		start := time.Now()
		req := service.Users.Messages.List(currentUser)
		if pageToken != "" {
			req.PageToken(pageToken)
		}
		r, err := req.Do()
		if err != nil {
			log.Printf("Unable to retrieve message ids. %v", err)
		}
		for _, message := range r.Messages {
			ch <- message.Id
		}
		emailIdFetchHistogram.Observe(time.Since(start).Seconds())
		pageToken = r.NextPageToken
		if pageToken == "" {
			close(ch)
			break
		}
	}
}

func init() {
	prometheus.MustRegister(emailIdFetchHistogram)
	prometheus.MustRegister(emailWriteHistogram)
}
