package main

import (
	"context"
	"fmt"

	"github.com/xanzy/go-gitlab"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

func (d *daemon) processJob(ctx context.Context, projectID string, pipelineID int, j *gitlab.Job) {
	if j.StartedAt == nil || j.FinishedAt == nil {
		return
	}

	_, jSpan := d.tracer.Start(ctx, fmt.Sprintf("Job %d", j.ID), trace.WithTimestamp(*j.StartedAt))

	jSpan.SetAttributes(attribute.Float64("duration_s", j.Duration))
	jSpan.SetAttributes(attribute.Float64("queued_duration_s", j.QueuedDuration))

	jSpan.SetAttributes(attribute.Bool("allow_failure", j.AllowFailure))

	jSpan.SetAttributes(attribute.String("status", j.Status))
	switch j.Status {
	case "failed", "canceled":
		jSpan.SetStatus(codes.Error, j.Status+j.FailureReason)
	case "success":
		jSpan.SetStatus(codes.Ok, j.Status)
	default:
		jSpan.SetStatus(codes.Unset, j.Status)
	}

	jSpan.SetAttributes(attribute.String("stage", j.Stage))
	jSpan.SetAttributes(attribute.String("sha", j.Commit.ID))
	jSpan.SetAttributes(attribute.String("ref", j.Ref))
	jSpan.SetAttributes(attribute.Bool("is_tag", j.Tag))

	jSpan.SetAttributes(attribute.StringSlice("tag_list", j.TagList))

	jSpan.SetAttributes(attribute.Int("artifact_count", len(j.Artifacts)))
	jSpan.SetAttributes(attribute.String("web_url", j.WebURL))

	jSpan.SetAttributes(attribute.String("user", j.User.Username))

	jSpan.SetAttributes(attribute.Int("runner_id", j.Runner.ID))
	jSpan.SetAttributes(attribute.String("runner_desc", j.Runner.Description))
	jSpan.SetAttributes(attribute.String("runner_name", j.Runner.Name))
	jSpan.SetAttributes(attribute.Bool("runner_is_shared", j.Runner.IsShared))

	jSpan.End(trace.WithTimestamp(*j.FinishedAt))
}
