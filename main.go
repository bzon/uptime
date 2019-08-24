package main

import (
	"fmt"
	"net/http"
	"net/url"
	"os"
	"time"

	"github.com/go-kit/kit/log"
	"github.com/pkg/errors"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type TargetConfig struct {
	Address      string
	User         string
	Password     string
	ExpectStatus int
	Timeout      time.Duration
	Interval     time.Duration
}

func main() {
	var logger log.Logger
	{
		logger = log.NewJSONLogger(log.NewSyncWriter(os.Stdout))
		logger = log.With(logger, "ts", log.DefaultTimestamp, "caller", log.DefaultCaller)
	}

	opsProcessed := promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "uptime_ops_total",
		Help: "The total number of processed events",
	},
		[]string{"address", "status", "success"},
	)

	go func(uptime *prometheus.CounterVec) {
		// TODO make this configurable via yaml, json or something using viper.
		targets := []TargetConfig{
			TargetConfig{
				Address:      os.Getenv("ADDRESS"),
				User:         os.Getenv("USER"),
				Password:     os.Getenv("PASSWORD"),
				ExpectStatus: 200,
				Timeout:      5 * time.Second,
			},
		}
		for {
			for _, t := range targets {
				_, err := url.ParseRequestURI(t.Address)
				if err != nil {
					err = errors.Wrapf(err, "skipping target %s", t.Address)
					logger.Log("message", err)
					continue
				}
				req, err := http.NewRequest("GET", t.Address, nil)
				if err != nil {
					err = errors.Wrapf(err, "skipping target %s", t.Address)
					logger.Log("message", err)
					continue
				}
				if len(t.User) > 0 && len(t.Password) > 0 {
					req.SetBasicAuth(t.User, t.Password)
				}
				client := &http.Client{}
				if t.Timeout != 0 {
					client.Timeout = t.Timeout
				}
				resp, err := client.Do(req)
				if err != nil {
					err = errors.Wrapf(err, "skipping target %s", t.Address)
					logger.Log("message", err)
					continue
				}
				uptime.With(
					prometheus.Labels{
						"address": t.Address,
						"success": fmt.Sprint(resp.StatusCode == t.ExpectStatus),
						"status":  fmt.Sprint(resp.StatusCode),
					},
				).Inc()
			}
			time.Sleep(5 * time.Second)
		}
	}(opsProcessed)

	http.Handle("/metrics", promhttp.Handler())
	if err := http.ListenAndServe(":2112", nil); err != nil {
		logger.Log("exit", err)
	}
}
