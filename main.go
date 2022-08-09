package main

import (
	"context"
	"log"
	"os"
	"strings"
	"time"

	"github.com/xanzy/go-gitlab"
	"go.uber.org/ratelimit"
)

var (
	ServiceVersion   = "v0.0.1"
	ServiceName      = "GitlabCIExporter"
	ServiceCachePath = "/tmp/gitlab-id-cache.txt"

	GitlabAPIToken   = os.Getenv("GITLAB_TOKEN")
	GitlabProjectIDs = os.Getenv("GITLAB_PROJECT_IDS")
	GitlabRateLimit  = 1

	HoneycombEndPoint = "api.honeycomb.io:443"
	HoneycombHeaders  = map[string]string{
		"x-honeycomb-team":    os.Getenv("HONEYCOMB_API_KEY"),
		"x-honeycomb-dataset": os.Getenv("HONEYCOMB_DATASET"),
	}
	HoneycombMaxRetention = 60 * 24 * time.Hour
)

type CustomRateLimiter struct {
	ratelimit.Limiter
}

func NewCustomRateLimiter(limit int) *CustomRateLimiter {
	return &CustomRateLimiter{
		ratelimit.New(limit, ratelimit.WithoutSlack),
	}
}

func (crl *CustomRateLimiter) Wait(ctx context.Context) error {
	crl.Take()

	return nil
}

func main() {
	gl, err := gitlab.NewClient(GitlabAPIToken, gitlab.WithCustomLimiter(NewCustomRateLimiter(GitlabRateLimit)))
	if err != nil {
		log.Fatalf("could not initialize gitlab client: %v", err)
	}

	ctx := context.Background()

	tracer, shutdown := initOtel(ctx, ServiceName)
	defer shutdown()

	sleepDuration := 15 * time.Minute

	projectIDs := strings.Split(GitlabProjectIDs, ",")

	NewDaemon(tracer, gl, GitlabRateLimit, projectIDs, sleepDuration, ServiceCachePath).Exec(ctx)
}
