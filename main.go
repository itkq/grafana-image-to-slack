package main

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"

	"github.com/slack-go/slack"
)

func main() {
	slackToken := os.Getenv("SLACK_TOKEN")
	if slackToken == "" {
		log.Fatal("SLACK_TOKEN is required")
	}
	slackClient := slack.New(slackToken)

	grafanaApiKey := os.Getenv("GRAFANA_API_KEY")
	if grafanaApiKey == "" {
		log.Fatal("GRAFANA_API_KEY is required")
	}

	handler := &Handler{
		slackClient:   slackClient,
		grafanaApiKey: grafanaApiKey,
	}

	log.Print("starting server...")
	http.Handle("/", handler)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
		log.Printf("defaulting to port %s", port)
	}

	log.Printf("listening on port %s", port)
	if err := http.ListenAndServe(":"+port, nil); err != nil {
		log.Fatal(err)
	}
}

type Handler struct {
	slackClient   *slack.Client
	grafanaApiKey string
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, fmt.Sprintf("Invalid method: %s\n", r.Method), http.StatusBadRequest)
		return
	}

	contentType := r.Header.Get("Content-Type")
	if contentType != "application/json" {
		http.Error(w, fmt.Sprintf("Invalid content-type: %s\n", contentType), http.StatusBadRequest)
		return
	}

	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Failed to read body", http.StatusBadRequest)
		return
	}

	var request Request
	if err := json.Unmarshal(body, &request); err != nil {
		http.Error(w, "Failed to parse body", http.StatusBadRequest)
		return
	}

	if err := request.Validate(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	imageIO, err := h.loadGrafanaImage(request)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if err := h.uploadFileToSlack(request, imageIO); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	fmt.Fprintf(w, "OK\n")
}

func (h *Handler) loadGrafanaImage(request Request) (io.ReadCloser, error) {
	req, err := http.NewRequest(http.MethodGet, request.GrafanaImageUrl, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", h.grafanaApiKey))

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}

	return resp.Body, nil
}

func (h *Handler) uploadFileToSlack(request Request, imageIO io.ReadCloser) error {
	defer imageIO.Close()
	params := slack.FileUploadParameters{
		Title:          request.Title,
		Filename:       request.Title,
		InitialComment: request.Comment,
		Reader:         imageIO,
		Channels:       []string{request.Channel},
	}

	if _, err := h.slackClient.UploadFile(params); err != nil {
		return err
	}

	return nil
}

type Request struct {
	Title           string `json:"title"`
	Comment         string `json:"comment"`
	Channel         string `json:"channel"`
	GrafanaImageUrl string `json:"grafana_image_url"`
}

func (r *Request) Validate() error {
	if r.Channel == "" {
		return fmt.Errorf("channel is required")
	}

	if r.Title == "" {
		return fmt.Errorf("title is required")
	}

	if r.GrafanaImageUrl == "" {
		return fmt.Errorf("grafana_image_url is required")
	}

	return nil
}
