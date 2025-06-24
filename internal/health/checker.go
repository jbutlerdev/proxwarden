package health

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"time"

	"github.com/jbutlerdev/proxwarden/internal/config"
	"github.com/sirupsen/logrus"
)

type CheckResult struct {
	Type      string
	Target    string
	Success   bool
	Error     error
	Duration  time.Duration
	Timestamp time.Time
}

type Checker struct {
	logger *logrus.Logger
}

func NewChecker(logger *logrus.Logger) *Checker {
	return &Checker{
		logger: logger,
	}
}

func (c *Checker) RunHealthCheck(ctx context.Context, check config.HealthCheck) *CheckResult {
	start := time.Now()
	result := &CheckResult{
		Type:      check.Type,
		Target:    check.Target,
		Timestamp: start,
	}

	checkCtx, cancel := context.WithTimeout(ctx, check.Timeout)
	defer cancel()

	switch check.Type {
	case "ping", "icmp":
		result.Success, result.Error = c.pingCheck(checkCtx, check.Target)
	case "tcp":
		result.Success, result.Error = c.tcpCheck(checkCtx, check.Target, check.Port)
	case "http", "https":
		result.Success, result.Error = c.httpCheck(checkCtx, check.Type, check.Target, check.Port, check.Path)
	default:
		result.Error = fmt.Errorf("unknown health check type: %s", check.Type)
	}

	result.Duration = time.Since(start)
	
	if result.Error != nil {
		c.logger.WithFields(logrus.Fields{
			"type":     check.Type,
			"target":   check.Target,
			"duration": result.Duration,
			"error":    result.Error,
		}).Debug("Health check failed")
	} else {
		c.logger.WithFields(logrus.Fields{
			"type":     check.Type,
			"target":   check.Target,
			"duration": result.Duration,
			"success":  result.Success,
		}).Debug("Health check completed")
	}

	return result
}

func (c *Checker) pingCheck(ctx context.Context, target string) (bool, error) {
	dialer := &net.Dialer{}
	conn, err := dialer.DialContext(ctx, "ip4:icmp", target)
	if err != nil {
		return false, fmt.Errorf("ping failed: %w", err)
	}
	defer conn.Close()
	return true, nil
}

func (c *Checker) tcpCheck(ctx context.Context, target string, port int) (bool, error) {
	dialer := &net.Dialer{}
	address := fmt.Sprintf("%s:%d", target, port)
	conn, err := dialer.DialContext(ctx, "tcp", address)
	if err != nil {
		return false, fmt.Errorf("tcp check failed: %w", err)
	}
	defer conn.Close()
	return true, nil
}

func (c *Checker) httpCheck(ctx context.Context, scheme, target string, port int, path string) (bool, error) {
	if path == "" {
		path = "/"
	}
	
	var url string
	if port > 0 {
		url = fmt.Sprintf("%s://%s:%d%s", scheme, target, port, path)
	} else {
		url = fmt.Sprintf("%s://%s%s", scheme, target, path)
	}

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return false, fmt.Errorf("failed to create request: %w", err)
	}

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return false, fmt.Errorf("http check failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 200 && resp.StatusCode < 400 {
		return true, nil
	}

	return false, fmt.Errorf("http check failed with status: %d", resp.StatusCode)
}