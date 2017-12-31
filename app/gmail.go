package main

import (
	"context"
	"log"
	"os"
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"golang.org/x/oauth2"
	gmail "google.golang.org/api/gmail/v1"
	tomb "gopkg.in/tomb.v2"
)

var (
	nWorkers              = 8
	emailIdFetchHistogram = prometheus.NewHistogram(prometheus.HistogramOpts{
		Name: "email_scraper_get_id_seconds",
		Help: "Time taken to fetch email ids",
	})
	emailGetHistogram = prometheus.NewHistogram(prometheus.HistogramOpts{
		Name: "email_scraper_get_email_seconds",
		Help: "Time taken to get an email",
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

func getMessage(service *gmail.Service, id string) (*gmail.Message, error) {
	start := time.Now()
	defer func() {
		emailGetHistogram.Observe(time.Since(start).Seconds())
	}()
	return service.Users.Messages.Get(currentUser, id).Do()
}

func indexMessages(ctx context.Context, service *gmail.Service, ch <-chan string) error {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case id, ok := <-ch:
			if !ok {
				return nil
			}
			r, err := getMessage(service, id)
			if err != nil {
				log.Printf("Error getting message from gmail: %v\n", err)
				return err
			}
			email, err := NewEmail(r)
			if err != nil {
				log.Printf("Error decoding content: %v", err)
				return err
			}
			err = indexEmail(ctx, id, email)
			if err != nil {
				log.Printf("Error indexing to elasticsearch: %v", err)
				return err
			}
		}
	}
}

func getPageOfMessages(service *gmail.Service, pageToken string) (*gmail.ListMessagesResponse, error) {
	start := time.Now()
	defer func() {
		emailIdFetchHistogram.Observe(time.Since(start).Seconds())
	}()
	req := service.Users.Messages.List(currentUser)
	if pageToken != "" {
		req.PageToken(pageToken)
	}
	r, err := req.Do()
	return r, err
}

func writeMessagesIdsToChannel(ctx context.Context, service *gmail.Service, ch chan string) error {
	defer close(ch)
	pageToken := ""
	for {
		select {
		case <-ctx.Done():
			return nil
		default:
			r, err := getPageOfMessages(service, pageToken)
			if err != nil {
				log.Printf("Unable to retrieve message ids. %v", err)
				return err
			}
			for _, message := range r.Messages {
				ch <- message.Id
			}
			pageToken = r.NextPageToken
			if pageToken == "" {
				return nil
			}
		}
	}
}

// IndexAllEmails will fetch all messages from the user whom the token
// belongs to and index them into elasticsearch
func IndexAllEmails(ctx context.Context, token *oauth2.Token) error {
	t, ctx := tomb.WithContext(ctx)
	client := googleOauthConfig.Client(ctx, token)

	service, err := gmail.New(client)
	if err != nil {
		log.Printf("Unable to retrieve gmail Client %v", err)
		return err
	}

	idChannel := make(chan string, 1000)
	t.Go(func() error {
		return writeMessagesIdsToChannel(ctx, service, idChannel)
	})
	for i := 0; i < nWorkers; i++ {
		t.Go(func() error {
			return indexMessages(ctx, service, idChannel)
		})
	}
	return t.Wait()
}

func initNWorkers() {
	v, err := strconv.Atoi(os.Getenv("WORKERS"))
	if err != nil {
		return
	}
	nWorkers = v
}

func init() {
	prometheus.MustRegister(emailIdFetchHistogram)
	prometheus.MustRegister(emailGetHistogram)
	initNWorkers()
}
