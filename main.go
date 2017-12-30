package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"sync"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	gmail "google.golang.org/api/gmail/v1"
)

const htmlIndex = `<html><body>
<a href="/googlelogin">Index GMail Account</a>
</body></html>
`

var (
	googleOauthConfig = &oauth2.Config{
		RedirectURL:  "http://localhost:8080/googlecallback",
		ClientID:     os.Getenv("GOOGLE_CLIENT_ID"),
		ClientSecret: os.Getenv("GOOGLE_CLIENT_SECRET"),
		Scopes:       []string{gmail.GmailReadonlyScope},
		Endpoint:     google.Endpoint,
	}
)

func handleMain(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, htmlIndex)
}

func handleGoogleLogin(w http.ResponseWriter, r *http.Request) {
	url := googleOauthConfig.AuthCodeURL("woot")
	http.Redirect(w, r, url, http.StatusTemporaryRedirect)
}

func handleGoogleCallback(w http.ResponseWriter, r *http.Request) {
	state := r.FormValue("state")
	if state != "woot" {
		log.Println("invalid oauth state")
		http.Redirect(w, r, "/", http.StatusTemporaryRedirect)
	}

	code := r.FormValue("code")
	token, err := googleOauthConfig.Exchange(oauth2.NoContext, code)
	if err != nil {
		log.Printf("Code exchange failed: %v\n", err)
		http.Redirect(w, r, "/", http.StatusTemporaryRedirect)
	}

	client := googleOauthConfig.Client(r.Context(), token)

	WriteEmails(w, r, client)
}

func WriteEmails(w http.ResponseWriter, r *http.Request, client *http.Client) {
	ctx := r.Context()

	srv, err := gmail.New(client)
	if err != nil {
		log.Fatalf("Unable to retrieve gmail Client %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	idChannel := GetAllMessages(srv)
	var wg sync.WaitGroup
	for i := 0; i < 16; i++ {
		wg.Add(1)
		go func() {
			WriteAllMessages(ctx, srv, idChannel)
			wg.Done()
		}()
	}
	wg.Wait()
	fmt.Fprintf(w, "Done")
}

func main() {
	http.Handle("/metrics", promhttp.Handler())
	http.HandleFunc("/", handleMain)
	http.HandleFunc("/googlelogin", handleGoogleLogin)
	http.HandleFunc("/googlecallback", handleGoogleCallback)
	log.Println("Starting server at :8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}
