package presto

import (
	"github.com/lyft/flytestdlib/promutils"
	"github.com/lyft/flytestdlib/promutils/labeled"
	"github.com/prometheus/client_golang/prometheus"
)

type ExecutorMetrics struct {
	Scope                 promutils.Scope
	ReleaseResourceFailed labeled.Counter
	AllocationGranted     labeled.Counter
	AllocationNotGranted  labeled.Counter
	ResourceWaitTime      prometheus.Summary
}

var (
	tokenAgeObjectives = map[float64]float64{0.5: 0.05, 0.9: 0.01, 0.99: 0.001, 1.0: 0.0}
)

func getPrestoExecutorMetrics(scope promutils.Scope) ExecutorMetrics {
	return ExecutorMetrics{
		Scope: scope,
		ReleaseResourceFailed: labeled.NewCounter("released_resource_failed",
			"Error releasing allocation token", scope),
		AllocationGranted: labeled.NewCounter("allocation_granted",
			"Allocation request granted", scope),
		AllocationNotGranted: labeled.NewCounter("allocation_not_granted",
			"Allocation request did not fail but not granted", scope),
		ResourceWaitTime: scope.MustNewSummaryWithOptions("resource_wait_time", "Duration the execution has been waiting for a resource allocation token",
			promutils.SummaryOptions{Objectives: tokenAgeObjectives}),
	}
}