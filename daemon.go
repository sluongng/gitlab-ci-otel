package main

import (
	"context"
	"log"
	"sync"
	"time"

	"github.com/xanzy/go-gitlab"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/ratelimit"
)

// daemon contains all the info needed by the goroutines inside the long-lived process
type daemon struct {
	lastFinishedAt time.Time
	tracer         trace.Tracer
	gl             *gitlab.Client
	rl             ratelimit.Limiter
	projectIDs     []string
	wg             *sync.WaitGroup
	cacheFilePath  string
	sleepDuration  time.Duration
}

// NewDaemon produce daemon struct that can be executed as a long-lived process
func NewDaemon(
	tracer trace.Tracer,
	gl *gitlab.Client,
	limitRPS int,
	projectIDs []string,
	sleepDuration time.Duration,
	cacheFilePath string,
) *daemon {
	wg := &sync.WaitGroup{}

	// Default to HoneycombMaxRetention on initial run
	// should be updated on subsequent runs
	lastFinishedAt := time.Now().Add(-1 * HoneycombMaxRetention)

	return &daemon{
		lastFinishedAt: lastFinishedAt,
		tracer:         tracer,
		gl:             gl,
		rl:             ratelimit.New(limitRPS),
		projectIDs:     projectIDs,
		wg:             wg,
		sleepDuration:  sleepDuration,
		cacheFilePath:  cacheFilePath,
	}
}

// Exec execute the daemon as a long-lived process
func (d *daemon) Exec(ctx context.Context) {
	// TODO: implement graceful shutdown when SIGTERM/SIGKILL
	for {
		for _, projectID := range d.projectIDs {
			d.wg.Add(1)
			go d.processProject(ctx, projectID)
		}
		d.wg.Wait()

		log.Printf("sleeping for %s", d.sleepDuration)
		time.Sleep(d.sleepDuration)
	}
}

// BuildKite pagination loop
func (d *daemon) processProject(ctx context.Context, projectID string) {
	cache := NewCache(d.cacheFilePath)
	defer cache.fileStore.Close()

	cachedPipelineIDs := cache.loadCache()

	pageOpt := gitlab.ListOptions{
		Page:    1,
		PerPage: 100,
	}
	for {
		pls, resp, err := d.gl.Pipelines.ListProjectPipelines(
			projectID,
			&gitlab.ListProjectPipelinesOptions{
				Scope:        gitlab.String("finished"),
				ListOptions:  pageOpt,
				UpdatedAfter: gitlab.Time(time.Now().Add(-2 * 7 * 24 * time.Hour)),
			},
			gitlab.WithContext(ctx),
		)
		if err != nil {
			log.Fatalf("could not list project pipelines: %v", err)
		}

		for _, p := range pls {
			if _, ok := cachedPipelineIDs[p.ID]; ok {
				log.Println("Skipping cached pipeline:", p.ID)
				continue
			}

			d.wg.Add(1)
			go d.processPipeline(ctx, projectID, p.ID)

			cachedPipelineIDs[p.ID] = struct{}{}
		}

		if resp.NextPage == 0 || resp.NextPage <= pageOpt.Page {
			break
		}

		pageOpt.Page = resp.NextPage

		// store all build IDs each run into cache
		if err := cache.writeCache(cachedPipelineIDs); err != nil {
			log.Fatalf("error writing cache: %v", err)
		}
	}

	d.wg.Done()
}
