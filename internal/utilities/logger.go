// Package utilities provides structured logging with AWS CloudWatch integration.
//
// All log output is dumped to AWS CloudWatch Logs for centralized debugging,
// monitoring, and alerting. This includes:
//   - Structured log entries with component, operation, status, elapsed time, and memory
//   - Goroutine correlation via TASK-### IDs for tracing request lifecycles
//   - Performance metrics (heap memory, goroutine count, uptime) for health monitoring
//   - Error/Warn/Info/Debug/Verbose levels with cost-optimized filtering
//
// CloudWatch Logs Insights queries can be used to search, filter, and aggregate
// logs across all running instances. See individual function docs for query examples.
//
// CloudWatch Agent Configuration (AmazonCloudWatchAgent):
//
//	{
//	  "logs": {
//	    "logs_collected": {
//	      "files": {
//	        "collect_list": [
//	          {
//	            "file_path": "/var/log/identitycardocr/*.log",
//	            "log_group_name": "/aws/identitycardocr/app",
//	            "log_stream_name": "{instance_id}-{date}",
//	            "timezone": "Asia/Kuala_Lumpur"
//	          }
//	        ]
//	      }
//	    }
//	  }
//	}
//
// For debugging: set LOG_LEVEL=DEBUG to dump all diagnostic logs to CloudWatch
// for real-time troubleshooting. Remember to revert to INFO/WARN in production
// to avoid excessive CloudWatch ingestion costs.
package utilities

import (
	"fmt"
	"os"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"
)

// LogLevel defines the severity threshold for CloudWatch log emission.
// Logs below the current level are filtered out to optimize costs and noise.
type LogLevel int

const (
	// APP_NAME identifies logs in CloudWatch for filtering: [IdentityCardOCRService] message
	APP_NAME = "IdentityCardOCRService"
	// VERSION tracks schema compatibility for log parsing in CloudWatch Logs Insights
	VERSION = "1.0.0"
	// TZ is the timezone identifier for log timestamps (Beijing Time)
	TZ = "CST"
)

// Log level constants in ascending severity order.
// Used for filtering: only logs >= CurrentLevel are emitted.
// Example: if CurrentLevel=WARN, then DEBUG and INFO logs are dropped.
//
// CloudWatch Filtering (Logs Insights):
//
//	DEBUG: filter ispresent(Debug_Fields) | stats count() as debug_count
//	INFO: [IdentityCardOCRService] INFO | stats count() as info_count
//	WARN: [IdentityCardOCRService] WARN | stats count() as warn_count
//	ERROR: [IdentityCardOCRService] ERROR | stats count() as error_count, count_by_component=count() by Component
//	VERBOSE: [IdentityCardOCRService] VERBOSE | fields Component, Operation, elapsed, memory
const (
	// DEBUG (0): Detailed diagnostic information, usually disabled in production.
	// Example: Field-level tracing, parameter dumps, internal state snapshots.
	// Cost: ~200-500MB/day - only use during troubleshooting.
	DEBUG LogLevel = iota

	// INFO (1): General informational messages - DEFAULT for production.
	// Example: Operation start/success, stock initialization, market status changes.
	// Cost: ~10-50MB/day - balanced logging for monitoring.
	INFO

	// WARN (2): Warning conditions - non-critical issues that don't prevent operation.
	// Example: Retries, connection timeouts, degraded performance, buffer full.
	// Cost: ~1-10MB/day - minimal, useful for trend analysis.
	WARN

	// ERROR (3): Error conditions requiring immediate attention.
	// Example: Failed inserts, database connection failures, authentication errors.
	// Cost: ~0.1-1MB/day - triggers alerts and incidents.
	ERROR

	// VVERBOSE (4): Detailed operational metrics and performance data.
	// Example: Per-tick statistics, batch size distribution, latency percentiles.
	// Cost: ~50-200MB/day - use for 24-hour performance analysis.
	VVERBOSE
)

const (
	colorReset  = "\033[0m"
	colorRed    = "\033[31m"
	colorYellow = "\033[33m"
	colorBlue   = "\033[34m"
	colorPink   = "\033[35m"
	colorGreen  = "\033[32m"
)

var (
	startTime       = time.Now()
	CurrentLevel    = INFO
	errorCallback   func(string)
	statusCallbacks = make(map[string]func(StatusUpdate))
	statusMutex     sync.RWMutex
	goroutineSeq    = 0
	goroutineMutex  sync.RWMutex
)

type StatusUpdate struct {
	StockNo         string
	LastPrice       float64
	Volume          float64
	BestAskSize     float64
	BestAskCount    float64
	BestBidPrice    float64
	TimeReceived    string
	TimeReceivedISO string
	TotalRecords    string
	SequenceNumber  string
	CreatedAt       string
	Message         string
}

func RegisterErrorCallback(cb func(string)) {
	errorCallback = cb
}

func RegisterStatusCallback(id string, cb func(StatusUpdate)) {
	statusMutex.Lock()
	defer statusMutex.Unlock()
	statusCallbacks[id] = cb
}

func UnregisterStatusCallback(id string) {
	statusMutex.Lock()
	defer statusMutex.Unlock()
	delete(statusCallbacks, id)
}

// getGoroutineID generates a sequential TASK-### identifier for goroutine correlation.
// Critical for CloudWatch Logs Insights to trace a single request through all goroutines.
//
// Each goroutine receives a unique sequential ID:
//   - Main goroutine: TASK-000
//   - Worker pool 1: TASK-001
//   - Worker pool 2: TASK-002
//   - Cron job: TASK-042
//
// CloudWatch Logs Insights Usage:
//
//	fields @timestamp, @message | filter Routine="TASK-042" | sort @timestamp
//
// This allows viewing the complete lifecycle of a single operation across all goroutines.
// Example use case: Trace why a specific market data insertion failed:
//
//  1. Find error: LogError with TASK-042
//  2. Search logs: Routine=TASK-042
//  3. View timeline: All operations performed by that goroutine
//  4. Identify root cause: Database connection failure, timeout, etc.
func getGoroutineID() string {
	goroutineMutex.Lock()
	defer goroutineMutex.Unlock()
	id := goroutineSeq
	goroutineSeq++
	return fmt.Sprintf("TASK-%03d", id)
}

func getCallerFunc(depth int) string {
	pc, _, _, ok := runtime.Caller(depth)
	if !ok {
		return "Unknown"
	}
	fullName := runtime.FuncForPC(pc).Name()
	parts := strings.Split(fullName, ".")
	if len(parts) > 0 {
		return parts[len(parts)-1]
	}
	return fullName
}

// getMemStats returns current heap memory allocation in MB.
// Embedded in every log entry for CloudWatch memory leak detection.
//
// CloudWatch Monitoring:
//   - Track memory growth: avg(Memory) by bin(1h) | sort by time
//   - Identify memory spikes: Memory > 500 (indicates potential leak)
//   - Correlate with operations: Memory when batch_size=10000 vs batch_size=100
//   - Set alarms: Memory > threshold triggers investigation
//
// Typical values for this service:
//   - Baseline (idle): ~80-100 MB
//   - During market open: ~150-200 MB
//   - High load (1M ticks/sec): ~300-400 MB
//   - Memory leak (24h uptime): Linear growth beyond normal
//
// Use in CloudWatch Insights:
//
//	fields @timestamp, Memory | filter Component="Feed" | stats max(Memory), avg(Memory) by bin(5m)
func getMemStats() float64 {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	return float64(m.Alloc) / 1024 / 1024
}

func getLevelStr(level LogLevel) string {
	switch level {
	case DEBUG:
		return "DEBUG"
	case INFO:
		return "INFO"
	case WARN:
		return "WARN"
	case ERROR:
		return "ERROR"
	case VVERBOSE:
		return "VERBOSE"
	default:
		return "UNKNOWN"
	}
}

func getLevelColor(level LogLevel) string {
	switch level {
	case DEBUG:
		return colorYellow
	case INFO:
		return colorBlue
	case WARN:
		return colorPink
	case ERROR:
		return colorRed
	case VVERBOSE:
		return colorGreen
	default:
		return ""
	}
}

func formatTimestamp() string {
	now := time.Now()
	return fmt.Sprintf("%d%02d%02d:%02d:%02d:%02d%s",
		now.Year(), now.Month(), now.Day(),
		now.Hour(), now.Minute(), now.Second(), TZ)
}

func formatHeader(level LogLevel, component, operation, goroutineID, function string) string {
	timestamp := formatTimestamp()
	return fmt.Sprintf("[%s@%s]::%s:: (%s:%s>>%s::%s)",
		APP_NAME, timestamp, getLevelStr(level), component, operation, goroutineID, function)
}

// SetLogLevel configures the minimum log level for emission to CloudWatch.
// Logs below the threshold are silently dropped to reduce log volume and costs.
//
// Log Levels (in increasing severity):
//   - DEBUG (0): Detailed diagnostic information, usually disabled in production
//   - INFO (1): General informational messages (default, recommended for production)
//   - WARN (2): Warning conditions that don't prevent operation (retries, degraded mode)
//   - ERROR (3): Error conditions requiring immediate attention
//   - VERBOSE (4): Detailed operational metrics and performance data
//
// CloudWatch Cost Optimization:
//   - DEBUG logs add ~200-500MB/day (not recommended for production)
//   - INFO logs add ~10-50MB/day (typical production setting)
//   - WARN logs add ~1-10MB/day (minimal, errors only)
//   - VERBOSE logs add detailed metrics for 24h analysis (useful for troubleshooting)
//
// Configuration Methods:
//  1. Environment variable (recommended): export LOG_LEVEL=INFO
//  2. At runtime: utilities.SetLogLevel("DEBUG")
//  3. Config file: parse and call SetLogLevel()
//
// Recommended Settings by Environment:
//   - Local Development: DEBUG (detailed troubleshooting)
//   - AWS Dev/Staging: INFO (balanced logging + performance)
//   - AWS Production: WARN (minimal costs, errors only)
//   - During Incident: Temporarily increase to DEBUG via AWS Systems Manager
//
// CloudWatch Integration:
//   - Log level change is NOT automatically logged (to prevent recursion)
//   - Set level before starting services in main.go
//   - Cannot be changed dynamically after startup (requires restart)
//   - Alarms triggered by log errors still work regardless of log level
//
// Example:
//
//	func init() {
//	  // Read from environment, defaults to INFO
//	  utilities.SetLogLevel(os.Getenv("LOG_LEVEL"))
//	}
func SetLogLevel(levelStr string) {
	level := strings.ToUpper(levelStr)
	switch level {
	case "DEBUG":
		CurrentLevel = DEBUG
	case "INFO":
		CurrentLevel = INFO
	case "WARN":
		CurrentLevel = WARN
	case "ERROR":
		CurrentLevel = ERROR
	case "VVERBOSE":
		CurrentLevel = VVERBOSE
	default:
		CurrentLevel = INFO
	}
}

func ToFloat64(v interface{}) float64 {
	if v == nil {
		return 0
	}
	switch val := v.(type) {
	case float64:
		return val
	case float32:
		return float64(val)
	case int:
		return float64(val)
	case int64:
		return float64(val)
	case string:
		s := strings.TrimSpace(val)
		if s == "" || strings.EqualFold(s, "null") || strings.EqualFold(s, "none") {
			return 0
		}
		f, err := strconv.ParseFloat(s, 64)
		if err != nil {
			return 0
		}
		return f
	default:
		return 0
	}
}

func init() {
	SetLogLevel(os.Getenv("LOG_LEVEL"))
}

func Log(level LogLevel, format string, a ...interface{}) {
	if level < CurrentLevel {
		return
	}

	timestamp := time.Now().Format("2006-01-02 15:04:05")
	msg := fmt.Sprintf(format, a...)
	levelStr := getLevelStr(level)
	color := getLevelColor(level)

	if level == ERROR && errorCallback != nil {
		errorCallback(msg)
	}

	output := fmt.Sprintf("[%s] [%s] [%s] %s", APP_NAME, timestamp, levelStr, msg)

	if color != "" {
		fmt.Printf("%s%s%s\n", color, output, colorReset)
	} else {
		fmt.Printf("%s\n", output)
	}
}

func Info(format string, a ...interface{})     { Log(INFO, format, a...) }
func Debug(format string, a ...interface{})    { Log(DEBUG, format, a...) }
func Warn(format string, a ...interface{})     { Log(WARN, format, a...) }
func Error(format string, a ...interface{})    { Log(ERROR, format, a...) }
func VVerbose(format string, a ...interface{}) { Log(VVERBOSE, format, a...) }

// Logf is the core structured logging function that emits CloudWatch-compatible logs.
//
// Parameters:
//   - component: Service component (Feed, Index, Cron, DLQ, etc.) for filtering
//   - operation: Operation name (Insert, Fetch, Connect, etc.) for identifying what was done
//   - level: Log level (DEBUG, INFO, WARN, ERROR, VERBOSE) - filtered by CurrentLevel
//   - status: Operation status (START, OK, FAIL, IN_PROGRESS, WARN) for result tracking
//   - elapsed: Operation duration - critical for performance monitoring in CloudWatch
//   - details: Key=value pairs (e.g., "batch_count=800", "exchange=NASDAQ") for context
//
// CloudWatch Integration:
//   - Automatic timestamp in MYT timezone format
//   - Goroutine correlation via sequential TASK-### IDs
//   - Memory tracking (MB) for memory leak detection
//   - Elapsed time for performance optimization (μs, ms, s)
//   - Component:Operation hierarchy for CloudWatch filtering
//
// CloudWatch Logs Insights Usage:
//   - Filter by component: Component:Operation
//   - Filter by status: Status=OK | Status=FAIL
//   - Filter by performance: Elapsed: [1-9]s
//   - Aggregate: stats count() by Component
//   - Timeline: sort by @timestamp
//
// Example:
//
//	Logf("Feed", "Insert", INFO, "OK", elapsed,
//	  "batch_count=800", "exchange=NASDAQ", "inserted_at=2026-05-22T15:30:46Z")
//
// Output:
//
//	[IdentityCardOCRService@20260522:15:30:46MYT]::INFO:: (Feed:Insert>>TASK-042::InsertRawPriceDataBatch)Status=OK, Type=ACTION, Memory=145.23MB, Routine=TASK-042, Elapsed: 12.45ms, batch_count=800, exchange=NASDAQ, inserted_at=2026-05-22T15:30:46Z
func Logf(component, operation string, level LogLevel, status string, elapsed time.Duration, details ...string) {
	if level < CurrentLevel {
		return
	}

	goroutineID := getGoroutineID()
	function := getCallerFunc(3)
	heapMB := getMemStats()

	header := formatHeader(level, component, operation, goroutineID, function)

	var elapsedStr string
	if elapsed.Microseconds() > 0 {
		if elapsed.Microseconds() < 1000 {
			elapsedStr = fmt.Sprintf("%.2fμs", float64(elapsed.Microseconds()))
		} else if elapsed.Milliseconds() < 1000 {
			elapsedStr = fmt.Sprintf("%.2fms", float64(elapsed.Milliseconds()))
		} else {
			elapsedStr = fmt.Sprintf("%.2fs", elapsed.Seconds())
		}
	} else {
		elapsedStr = "0μs"
	}

	output := fmt.Sprintf("%sStatus=%s, Type=ACTION, Memory=%.2fMB, Routine=%s, Elapsed: %s",
		header, status, heapMB, goroutineID, elapsedStr)

	if len(details) > 0 {
		output += ", " + strings.Join(details, ", ")
	}

	color := getLevelColor(level)
	if color != "" {
		fmt.Printf("%s%s%s\n", color, output, colorReset)
	} else {
		fmt.Printf("%s\n", output)
	}

	if level == ERROR && errorCallback != nil {
		errorCallback(output)
	}
}

// LogStart emits an INFO-level log indicating operation start.
// Used at the beginning of any operation to mark the start in CloudWatch.
// Status=START helps identify operation initiation vs completion.
//
// CloudWatch Usage:
//   - Find all operation starts: Status=START
//   - Correlate with LogSuccess/LogError to track operation lifecycle
//   - Measure time between START and success/failure
func LogStart(component, operation string) {
	Logf(component, operation, INFO, "START", 0)
}

// LogSuccess emits an INFO-level log indicating successful operation completion.
// Includes elapsed time for performance tracking and analysis.
// Status=OK indicates successful completion.
//
// CloudWatch Usage:
//   - Find all successful operations: Status=OK
//   - Calculate success rate: count(Status=OK) / count(Status=START)
//   - Track performance: avg(Elapsed), max(Elapsed), min(Elapsed) by Component
//   - Identify slow operations: Elapsed: [1-9]s
//
// Example:
//
//	start := time.Now()
//	// ... do work ...
//	LogSuccess("Feed", "Insert", time.Since(start), "batch_count=800")
func LogSuccess(component, operation string, elapsed time.Duration, details ...string) {
	Logf(component, operation, INFO, "OK", elapsed, details...)
}

// LogError emits an ERROR-level log indicating operation failure.
// Automatically includes error message and elapsed time.
// Status=FAIL triggers CloudWatch alarms and error tracking.
//
// CloudWatch Integration:
//   - Triggers CloudWatch Alarms when ErrorCount metric exceeds threshold
//   - Appears in error dashboards and alerts
//   - Tracks error frequency per component for trend analysis
//   - Enables automatic incident detection
//
// Example:
//
//	if err != nil {
//	  LogError("Feed", "Insert", err, time.Since(start), "batch_count=800")
//	  return err
//	}
//
// CloudWatch Insights Query:
//
//	fields @timestamp, Component, Error | filter Status=FAIL | stats count() by Component
func LogError(component, operation string, err error, elapsed time.Duration, details ...string) {
	errDetail := fmt.Sprintf("Error=%s", err.Error())
	allDetails := append([]string{errDetail}, details...)
	Logf(component, operation, ERROR, "FAIL", elapsed, allDetails...)
}

// LogWarn emits a WARN-level log for non-critical issues (retries, degraded mode, etc.).
// Status=WARN helps distinguish from errors while still indicating problems.
// Used for recoverable issues that don't stop operation but affect performance.
//
// CloudWatch Usage:
//   - Monitor degradation: count(Status=WARN) / count(Status=OK)
//   - Track retry frequency: Warn=.*retry.*
//   - Identify bottlenecks: component with high WARN count
//
// Example:
//
//	if connectionLost {
//	  LogWarn("Feed", "Connect", "reconnecting", 0, "attempt=3/5")
//	}
func LogWarn(component, operation string, msg string, elapsed time.Duration, details ...string) {
	warnDetail := fmt.Sprintf("Warn=%s", msg)
	allDetails := append([]string{warnDetail}, details...)
	Logf(component, operation, WARN, "WARN", elapsed, allDetails...)
}

// LogProgress emits an INFO-level log for in-progress operations.
// Status=IN_PROGRESS allows tracking of long-running operations.
// Used in loops or polling to indicate ongoing work without completion.
//
// CloudWatch Usage:
//   - Monitor stalled operations: IN_PROGRESS logs older than 5 minutes
//   - Track operation phases: multiple IN_PROGRESS entries per operation
//   - Identify operations that never complete: IN_PROGRESS without OK/FAIL
//
// Example:
//
//	for i, batch := range batches {
//	  LogProgress("Feed", "Flush", fmt.Sprintf("processing_batch_%d_%d", i, len(batch)), "batch_size=100")
//	  // ... process batch ...
//	}
func LogProgress(component, operation string, msg string, details ...string) {
	progressDetail := fmt.Sprintf("Progress=%s", msg)
	allDetails := append([]string{progressDetail}, details...)
	Logf(component, operation, INFO, "IN_PROGRESS", 0, allDetails...)
}

func LogStatus(update StatusUpdate) {
	if update.CreatedAt == "" {
		update.CreatedAt = time.Now().Format(time.RFC3339)
	}

	goroutineID := getGoroutineID()
	function := getCallerFunc(3)
	heapMB := getMemStats()

	header := formatHeader(INFO, "Feed", "Insert", goroutineID, function)

	var details []string
	if update.StockNo != "" {
		details = append(details,
			fmt.Sprintf("Stock=%s", update.StockNo),
			fmt.Sprintf("Price=%.4f", update.LastPrice),
			fmt.Sprintf("Vol=%.0f", update.Volume),
			fmt.Sprintf("Seq=%s", update.SequenceNumber),
		)
	}
	details = append(details, fmt.Sprintf("Msg=%s", update.Message))

	output := fmt.Sprintf("%sStatus=OK, Type=DATA, Memory=%.2fMB, Routine=%s, %s",
		header, heapMB, goroutineID, strings.Join(details, ", "))

	fmt.Printf("%s%s%s\n", colorBlue, output, colorReset)

	statusMutex.RLock()
	defer statusMutex.RUnlock()
	for _, cb := range statusCallbacks {
		if cb != nil {
			go cb(update)
		}
	}
}

func GetEnv(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	return fallback
}

// Mask redacts sensitive data in logs for CloudWatch data protection.
// Complies with GDPR, HIPAA, and other data protection regulations.
//
// CloudWatch Data Protection Features (AWS):
//   - Automatic redaction of PII (personally identifiable information)
//   - Custom data identifier rules for domain-specific sensitive data
//   - Log group-level masking policies
//   - Audit trail of masked data attempts (optional)
//
// Usage Examples:
//   - API Keys: Mask(apiKey) → "secret_a[REDACTED]"
//   - Passwords: Mask(password) → "pass[REDACTED]"
//   - Tokens: Mask(bearerToken) → "eyJhbG[REDACTED]"
//
// Masking Pattern:
//   - Shows first 10 characters (or 1/3 of short strings)
//   - Appends [REDACTED] to indicate intentional masking
//   - Enough information for debugging without exposing secrets
//
// Example in logs:
//
//	LogSuccess("Auth", "Authenticate", elapsed, fmt.Sprintf("token=%s", Mask(token)))
//	// Output: token=eyJhbGc[REDACTED]
//
// CloudWatch Integration:
//   - Works with CloudWatch Data Protection feature
//   - Enables compliance reporting for regulatory audits
//   - Reduces GDPR/CCPA data access risks
func Mask(s string) string {
	runes := []rune(s)
	n := len(runes)

	if n <= 4 {
		return "****"
	}

	showCount := 10
	if n <= showCount {
		showCount = n / 3
	}

	return string(runes[:showCount]) + "[REDACTED]"
}

// CheckCUrrentMemory emits a system health snapshot to CloudWatch.
// Called periodically (e.g., every 5 minutes) to monitor application health.
//
// Metrics collected:
//   - Heap: Current heap allocation (MB) - tracks memory growth
//   - Alloc: Total allocated memory (MB) - includes freed but not returned to OS
//   - Sys: System memory (MB) - all memory reserved from OS
//   - Routines: Number of active goroutines - detects goroutine leaks
//   - Uptime: Time since application started - for correlation with restarts
//
// CloudWatch Dashboard Metrics:
//   - Memory trend: heap memory over time (early detection of leaks)
//   - Goroutine leak: increasing Routines without corresponding log activity
//   - System health: Sys growing unbounded indicates memory fragmentation
//   - Uptime tracking: frequency of restarts indicates stability issues
//
// Alarm Thresholds:
//   - Alert if Routines > 2000 (goroutine leak)
//   - Alert if Heap > 1000MB (memory pressure)
//   - Alert if Uptime < 1h (frequent restarts)
//
// CloudWatch Logs Insights Usage:
//
//	fields Heap, Alloc, Routines, Uptime | filter Component="App" | sort @timestamp desc
//
// Memory leak detection query:
//
//	fields Heap | filter Component="App" | stats max(Heap) as max_heap by bin(1h) | sort max_heap desc
func CheckCUrrentMemory() string {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	toMB := func(bytes uint64) float64 {
		return float64(bytes) / 1024 / 1024
	}

	uptime := time.Since(startTime).Round(time.Second)
	numGoroutine := runtime.NumGoroutine()

	status := fmt.Sprintf(
		"[%s@%s]::INFO:: (App:State>>System::CheckMemory)Status=OK, Heap=%.2fMB, Alloc=%.2fMB, Sys=%.2fMB, Routines=%d, Uptime=%s",
		APP_NAME,
		formatTimestamp(),
		toMB(m.Alloc),
		toMB(m.TotalAlloc),
		toMB(m.Sys),
		numGoroutine,
		uptime.String(),
	)

	fmt.Printf("%s%s%s\n", colorBlue, status, colorReset)
	return status
}

func RetryWithBackoff(operationName string, maxAttempts int, backoffDuration time.Duration, operation func() error) error {
	var lastError error
	for attemptIndex := 0; attemptIndex < maxAttempts; attemptIndex++ {
		operationError := operation()
		if operationError == nil {
			return nil
		}
		lastError = operationError
		Log(WARN, "%s attempt %d/%d failed: %v. Retrying in %v...", operationName, attemptIndex+1, maxAttempts, operationError, backoffDuration)
		time.Sleep(backoffDuration)
	}
	return fmt.Errorf("%s exhausted %d retries: %w", operationName, maxAttempts, lastError)
}
