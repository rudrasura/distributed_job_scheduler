package observability

import (
	"log"
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Ingestion Metrics
var (
	HttpRequestsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "http_requests_total",
		Help: "Total number of HTTP requests",
	}, []string{"method", "path", "status"})

	HttpRequestDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name: "http_request_duration_seconds",
		Help: "HTTP request duration in seconds",
	}, []string{"method", "path"})

	JobsCreatedTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "jobs_created_total",
		Help: "Total number of jobs created",
	}, []string{"user_id"})

	PayloadStorageDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name: "payload_storage_duration_seconds",
		Help: "Time taken to store payload (S3/Scylla)",
	}, []string{"storage_type"}) // s3 or scylla

	KafkaPublishDuration = promauto.NewHistogram(prometheus.HistogramOpts{
		Name: "kafka_publish_duration_seconds",
		Help: "Time taken to publish to Kafka",
	})

	KafkaPublishErrors = promauto.NewCounter(prometheus.CounterOpts{
		Name: "kafka_publish_errors_total",
		Help: "Total number of Kafka publish errors",
	})
)

// Picker Metrics
var (
	PickerScansTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "picker_scans_total",
		Help: "Total number of shard scans",
	}, []string{"shard_id"})

	JobsScannedTotal = promauto.NewCounter(prometheus.CounterOpts{
		Name: "jobs_scanned_total",
		Help: "Total number of jobs scanned from queue",
	})

	JobsEnqueuedTotal = promauto.NewCounter(prometheus.CounterOpts{
		Name: "jobs_enqueued_total",
		Help: "Total number of jobs enqueued to SQS",
	})

	ScanCycleDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name: "scan_cycle_duration_seconds",
		Help: "Time taken for a scan cycle",
	}, []string{"shard_id"})

	SQSEnqueueDuration = promauto.NewHistogram(prometheus.HistogramOpts{
		Name: "sqs_enqueue_duration_seconds",
		Help: "Time taken to enqueue to SQS",
	})

	SQSEnqueueErrors = promauto.NewCounter(prometheus.CounterOpts{
		Name: "sqs_enqueue_errors_total",
		Help: "Total number of SQS enqueue errors",
	})
)

// Worker Metrics
var (
	JobsExecutedTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "jobs_executed_total",
		Help: "Total number of jobs executed",
	}, []string{"status"}) // success, failed

	JobExecutionDuration = promauto.NewHistogram(prometheus.HistogramOpts{
		Name: "job_execution_duration_seconds",
		Help: "Duration of job execution",
	})

	S3OperationsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "s3_operations_total",
		Help: "Total number of S3 operations",
	}, []string{"operation"}) // upload, download
)

// InitMetrics starts the Prometheus metrics server
func InitMetrics() {
	go func() {
		http.Handle("/metrics", promhttp.Handler())
		log.Println("Starting Prometheus metrics server on :2112")
		if err := http.ListenAndServe(":2112", nil); err != nil {
			log.Printf("Failed to start metrics server: %v", err)
		}
	}()
}
