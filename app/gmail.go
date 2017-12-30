package main

import (
	"context"
	"log"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"golang.org/x/oauth2"
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

func parseHeader(msg *gmail.Message, headerName string) string {
	for _, header := range msg.Payload.Headers {
		if header.Name == headerName {
			return header.Value
		}
	}
	return ""
}

func writeAllMessages(ctx context.Context, service *gmail.Service, ch <-chan string) {
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

func getPageOfMessages(service *gmail.Service, pageToken string) (*gmail.ListMessagesResponse, error) {
	start := time.Now()
	req := service.Users.Messages.List(currentUser)
	if pageToken != "" {
		req.PageToken(pageToken)
	}
	r, err := req.Do()
	emailIdFetchHistogram.Observe(time.Since(start).Seconds())
	return r, err
}

func writeMessagesIdsToChannel(service *gmail.Service, ch chan string) {
	pageToken := ""
	for {
		r, err := getPageOfMessages(service, pageToken)
		if err != nil {
			log.Printf("Unable to retrieve message ids. %v", err)
		}
		for _, message := range r.Messages {
			ch <- message.Id
		}
		pageToken = r.NextPageToken
		if pageToken == "" {
			close(ch)
			break
		}
	}
}

// IndexAllEmails will fetch all messages from the user whom the token
// belongs to and index them into elasticsearch
func IndexAllEmails(ctx context.Context, token *oauth2.Token) error {
	client := googleOauthConfig.Client(ctx, token)

	service, err := gmail.New(client)
	if err != nil {
		log.Fatalf("Unable to retrieve gmail Client %v", err)
		return err
	}
	idChannel := make(chan string, 20000)
	go writeMessagesIdsToChannel(service, idChannel)
	var wg sync.WaitGroup
	for i := 0; i < 16; i++ {
		wg.Add(1)
		go func() {
			writeAllMessages(ctx, service, idChannel)
			wg.Done()
		}()
	}
	wg.Wait()
	return nil
}

func init() {
	prometheus.MustRegister(emailIdFetchHistogram)
	prometheus.MustRegister(emailWriteHistogram)
}
