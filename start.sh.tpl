#!/bin/bash

go build -o exporter .

export GITLAB_PROJECT_IDS='<gl-project-id>'
export GITLAB_TOKEN='<gl-api-token>'
export HONEYCOMB_DATASET='<honeycomb-dataset-name>'
export HONEYCOMB_API_KEY='<honeycomb-dataset-name>'

./exporter
