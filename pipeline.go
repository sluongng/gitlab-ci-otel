package main

import (
	"context"
	"fmt"
	"log"

	"github.com/xanzy/go-gitlab"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

func (d *daemon) processPipeline(ctx context.Context, projectID string, pipelineID int) {
	defer d.wg.Done()

	p, _, err := d.gl.Pipelines.GetPipeline(projectID, pipelineID)
	if err != nil {
		log.Printf("Error getting project %s pipeline %d: %v", projectID, pipelineID, err)
	}

	if p.StartedAt == nil || p.FinishedAt == nil {
		return
	}

	log.Printf("processing build %d finished at %s", p.ID, p.FinishedAt)

	// create build span
	buildCtx, buildSpan := d.tracer.Start(ctx, fmt.Sprintf("Pipeline %d", p.ID), trace.WithTimestamp(*p.StartedAt))

	// build timing
	// reference: https://buildkite.com/docs/apis/rest-api/builds#timestamp-attributes
	buildSpan.SetAttributes(attribute.Int("duration_s", p.Duration))
	buildSpan.SetAttributes(attribute.Int("queued_duration_s", p.QueuedDuration))

	// build state
	buildSpan.SetAttributes(attribute.String("status", p.Status))
	switch p.Status {
	case "failed", "canceled":
		buildSpan.SetStatus(codes.Error, p.Status)
	case "success":
		buildSpan.SetStatus(codes.Ok, p.Status)
	default:
		buildSpan.SetStatus(codes.Unset, p.Status)
	}

	// build metadata
	buildSpan.SetAttributes(attribute.String("source", p.Source))
	buildSpan.SetAttributes(attribute.String("ref", p.Ref))
	buildSpan.SetAttributes(attribute.String("sha", p.SHA))
	buildSpan.SetAttributes(attribute.String("before_sha", p.BeforeSHA))
	buildSpan.SetAttributes(attribute.Bool("is_tag", p.Tag))

	buildSpan.SetAttributes(attribute.String("user", p.User.Username))
	buildSpan.SetAttributes(attribute.String("web_url", p.WebURL))

	// create job spans
	pageOpt := gitlab.ListOptions{
		Page:    1,
		PerPage: 100,
	}
	for {
		jobs, resp, err := d.gl.Jobs.ListPipelineJobs(
			projectID,
			p.ID,
			&gitlab.ListJobsOptions{
				Scope:       &[]gitlab.BuildStateValue{"failed", "success", "canceled", "skipped", "manual"},
				ListOptions: pageOpt,
			},
		)
		if err != nil {
			log.Printf("Error getting project %s pipeline %d jobs page %d: %v", projectID, pipelineID, pageOpt.Page, err)
		}

		for _, job := range jobs {
			d.processJob(buildCtx, projectID, pipelineID, job)
		}

		if resp.NextPage == 0 || resp.NextPage <= pageOpt.Page {
			break
		}

		pageOpt.Page = resp.NextPage
	}

	buildSpan.End(trace.WithTimestamp(*p.FinishedAt))
}
