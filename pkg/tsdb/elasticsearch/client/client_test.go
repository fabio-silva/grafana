package es

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Masterminds/semver"
	"github.com/grafana/grafana-plugin-sdk-go/backend"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/grafana/grafana/pkg/components/simplejson"
)

func TestClient_ExecuteMultisearch(t *testing.T) {
	t.Run("Given a fake http client and a client with response", func(t *testing.T) {
		var request *http.Request
		var requestBody *bytes.Buffer

		ts := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
			request = r
			buf, err := io.ReadAll(r.Body)
			require.NoError(t, err)

			requestBody = bytes.NewBuffer(buf)

			rw.Header().Set("Content-Type", "application/x-ndjson")
			_, err = rw.Write([]byte(
				`{
				"responses": [
					{
						"hits": {	"hits": [], "max_score": 0,	"total": { "value": 4656, "relation": "eq"}	},
						"status": 200
					}
				]
			}`))
			require.NoError(t, err)
			rw.WriteHeader(200)
		}))

		version, err := semver.NewVersion("8.0.0")
		require.NoError(t, err)

		ds := DatasourceInfo{
			URL:                        ts.URL,
			HTTPClient:                 ts.Client(),
			Database:                   "[metrics-]YYYY.MM.DD",
			ESVersion:                  version,
			TimeField:                  "@timestamp",
			Interval:                   "Daily",
			MaxConcurrentShardRequests: 6,
			IncludeFrozen:              true,
			XPack:                      true,
		}

		from := time.Date(2018, 5, 15, 17, 50, 0, 0, time.UTC)
		to := time.Date(2018, 5, 15, 17, 55, 0, 0, time.UTC)
		timeRange := backend.TimeRange{
			From: from,
			To:   to,
		}

		c, err := NewClient(context.Background(), &ds, timeRange)
		require.NoError(t, err)
		require.NotNil(t, c)

		t.Cleanup(func() {
			ts.Close()
		})

		ms, err := createMultisearchForTest(t, c)
		require.NoError(t, err)
		res, err := c.ExecuteMultisearch(ms)
		require.NoError(t, err)

		require.NotNil(t, request)
		assert.Equal(t, http.MethodPost, request.Method)
		assert.Equal(t, "/_msearch", request.URL.Path)
		assert.Equal(t, "max_concurrent_shard_requests=6&ignore_throttled=false", request.URL.RawQuery)

		require.NotNil(t, requestBody)

		headerBytes, err := requestBody.ReadBytes('\n')
		require.NoError(t, err)
		bodyBytes := requestBody.Bytes()

		jHeader, err := simplejson.NewJson(headerBytes)
		require.NoError(t, err)

		jBody, err := simplejson.NewJson(bodyBytes)
		require.NoError(t, err)

		assert.Equal(t, "metrics-2018.05.15", jHeader.Get("index").MustString())
		assert.True(t, jHeader.Get("ignore_unavailable").MustBool(false))
		assert.Equal(t, "query_then_fetch", jHeader.Get("search_type").MustString())
		assert.Empty(t, jHeader.Get("max_concurrent_shard_requests"))
		assert.False(t, jHeader.Get("ignore_throttled").MustBool())

		assert.Equal(t, "15000*@hostname", jBody.GetPath("aggs", "2", "aggs", "1", "avg", "script").MustString())

		assert.Equal(t, "15s", jBody.GetPath("aggs", "2", "date_histogram", "fixed_interval").MustString())

		assert.Equal(t, 200, res.Status)
		require.Len(t, res.Responses, 1)
	})
}

func createMultisearchForTest(t *testing.T, c Client) (*MultiSearchRequest, error) {
	t.Helper()

	msb := c.MultiSearch()
	s := msb.Search(15 * time.Second)
	s.Agg().DateHistogram("2", "@timestamp", func(a *DateHistogramAgg, ab AggBuilder) {
		a.FixedInterval = "$__interval"

		ab.Metric("1", "avg", "@hostname", func(a *MetricAggregation) {
			a.Settings["script"] = "$__interval_ms*@hostname"
		})
	})
	return msb.Build()
}
