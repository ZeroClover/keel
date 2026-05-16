package http

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/keel-hq/keel/types"

	"github.com/prometheus/client_golang/prometheus"

	log "github.com/sirupsen/logrus"
)

var newDockerhubWebhooksCounter = prometheus.NewCounterVec(
	prometheus.CounterOpts{
		Name: "dockerhub_webhook_requests_total",
		Help: "How many /v1/webhooks/dockerhub requests processed, partitioned by image.",
	},
	[]string{"image"},
)

func init() {
	prometheus.MustRegister(newDockerhubWebhooksCounter)
}

// Example of dockerhub trigger
// {
// 	"push_data": {
// 		"pushed_at": 1497467660,
// 		"images": [],
// 		"tag": "0.1.7",
// 		"pusher": "karolisr"
// 	},
// 	"callback_url": "https://registry.hub.docker.com/u/karolisr/keel/hook/22hagb51h1gfb4eefc5f1g4j3abi0beg4/",
// 	"repository": {
// 		"status": "Active",
// 		"description": "",
// 		"is_trusted": false,
// 		"full_description": "Keel DockerHub webhook example",
// 		"repo_url": "https://hub.docker.com/r/karolisr/keel",
// 		"owner": "karolisr",
// 		"is_official": false,
// 		"is_private": false,
// 		"name": "keel",
// 		"namespace": "karolisr",
// 		"star_count": 0,
// 		"comment_count": 0,
// 		"date_created": 1497032538,
// 		"dockerfile": "FROM golang:1.8.1-alpine\nCOPY . /go/src/github.com/keel-hq/keel\nWORKDIR /go/src/github.com/keel-hq/keel\nRUN apk add --no-cache git && go get\nRUN CGO_ENABLED=0 GOOS=linux go build -a -tags netgo -ldflags -'w' -o keel .\n\nFROM alpine:latest\nRUN apk --no-cache add ca-certificates\nCOPY --from=0 /go/src/github.com/keel-hq/keel/keel /bin/keel\nENTRYPOINT [\"/bin/keel\"]\n\nEXPOSE 9300",
// 		"repo_name": "karolisr/keel"
// 	}
// }

type dockerHubWebhook struct {
	PushData struct {
		PushedAt int           `json:"pushed_at"`
		Images   []interface{} `json:"images"`
		Tag      string        `json:"tag"`
		Pusher   string        `json:"pusher"`
	} `json:"push_data"`
	CallbackURL string `json:"callback_url"`
	Repository  struct {
		Status          string `json:"status"`
		Description     string `json:"description"`
		IsTrusted       bool   `json:"is_trusted"`
		FullDescription string `json:"full_description"`
		RepoURL         string `json:"repo_url"`
		Owner           string `json:"owner"`
		IsOfficial      bool   `json:"is_official"`
		IsPrivate       bool   `json:"is_private"`
		Name            string `json:"name"`
		Namespace       string `json:"namespace"`
		StarCount       int    `json:"star_count"`
		CommentCount    int    `json:"comment_count"`
		DateCreated     int    `json:"date_created"`
		Dockerfile      string `json:"dockerfile"`
		RepoName        string `json:"repo_name"`
	} `json:"repository"`
}

// dockerHubHandler - used to react to dockerhub webhooks
func (s *TriggerServer) dockerHubHandler(resp http.ResponseWriter, req *http.Request) {
	dw := dockerHubWebhook{}
	if err := json.NewDecoder(req.Body).Decode(&dw); err != nil {
		log.WithFields(log.Fields{
			"error": err,
		}).Error("trigger.dockerHubHandler: failed to decode request")
		resp.WriteHeader(http.StatusBadRequest)
		return
	}

	if dw.Repository.RepoName == "" {
		resp.WriteHeader(http.StatusBadRequest)
		fmt.Fprintf(resp, "repository name cannot be empty")
		return
	}

	if dw.PushData.Tag == "" {
		resp.WriteHeader(http.StatusBadRequest)
		fmt.Fprintf(resp, "repository tag cannot be empty")
		return
	}

	event := types.Event{}
	event.CreatedAt = time.Now()
	event.TriggerName = "dockerhub"
	event.Repository.Name = dw.Repository.RepoName
	event.Repository.Tag = dw.PushData.Tag

	s.trigger(event)

	resp.WriteHeader(http.StatusOK)

	newDockerhubWebhooksCounter.With(prometheus.Labels{"image": event.Repository.Name}).Inc()
}
