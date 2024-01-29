package main

import (
	"github.com/prometheus/client_golang/prometheus"
)

var (
	accessCounter = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "rte_access_total",
			Help: "Total number of accesses to the service",
		},
	)
	compileErrorCounter = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "rte_compile_error_total",
			Help: "Total number of test runs that had compile errors",
		},
	)
	testCount = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "rte_test_execution_count",
			Help: "Number of tests executed",
		},
		[]string{"test"},
	)
	testFailCount = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "rte_test_fail_count",
			Help: "Number of failed tests",
		},
		[]string{"test"},
	)
	junitIncompatibilityCount = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "rte_junit_incompatibilities_total",
			Help: "Total number of incompatibilities between tests and uploaded solution",
		},
		[]string{"test"},
	)
	errorCounter = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "rte_error_total",
			Help: "Number of errors",
		},
		[]string{"phase"},
	)
	testExecutionTimeHistogram = prometheus.NewHistogram(
		prometheus.HistogramOpts{
			Name:    "rte_test_execution_time",
			Help:    "The execution time of tests in seconds",
			Buckets: []float64{0.1, 0.5, 1.0, 2.0, 4.0, 8.0, 16.0},
		},
	)
)

func InitMonitoring() {
	prometheus.MustRegister(accessCounter)
	prometheus.MustRegister(compileErrorCounter)
	prometheus.MustRegister(testCount, testFailCount)
	prometheus.MustRegister(junitIncompatibilityCount)
	prometheus.MustRegister(errorCounter)
	prometheus.MustRegister(testExecutionTimeHistogram)

	for _, phase := range []string{"startup", "upload", "create", "listing", "compile", "test"} {
		errorCounter.WithLabelValues(phase).Add(0)
	}
}
