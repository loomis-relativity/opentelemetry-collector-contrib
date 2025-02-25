// Copyright 2020, OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package splunkhecexporter

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/config/configtls"
	"go.opentelemetry.io/collector/confmap/confmaptest"
	"go.opentelemetry.io/collector/exporter/exporterhelper"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/splunk"
)

func TestLoadConfig(t *testing.T) {
	t.Parallel()

	cm, err := confmaptest.LoadConf(filepath.Join("testdata", "config.yaml"))
	require.NoError(t, err)

	// Endpoint and Token do not have a default value so set them directly.
	defaultCfg := createDefaultConfig().(*Config)
	defaultCfg.Token = "00000000-0000-0000-0000-0000000000000"
	defaultCfg.HTTPClientSettings.Endpoint = "https://splunk:8088/services/collector"

	hundred := 100

	tests := []struct {
		id       component.ID
		expected component.Config
	}{
		{
			id:       component.NewIDWithName(typeStr, ""),
			expected: defaultCfg,
		},
		{
			id: component.NewIDWithName(typeStr, "allsettings"),
			expected: &Config{
				Token:                   "00000000-0000-0000-0000-0000000000000",
				Source:                  "otel",
				SourceType:              "otel",
				Index:                   "metrics",
				SplunkAppName:           "OpenTelemetry-Collector Splunk Exporter",
				SplunkAppVersion:        "v0.0.1",
				LogDataEnabled:          true,
				ProfilingDataEnabled:    true,
				ExportRaw:               true,
				MaxContentLengthLogs:    2 * 1024 * 1024,
				MaxContentLengthMetrics: 2 * 1024 * 1024,
				MaxContentLengthTraces:  2 * 1024 * 1024,
				HTTPClientSettings: confighttp.HTTPClientSettings{
					Timeout:  10 * time.Second,
					Endpoint: "https://splunk:8088/services/collector",
					TLSSetting: configtls.TLSClientSetting{
						TLSSetting: configtls.TLSSetting{
							CAFile:   "",
							CertFile: "",
							KeyFile:  "",
						},
						InsecureSkipVerify: false,
					},
					MaxIdleConns:        &hundred,
					MaxIdleConnsPerHost: &hundred,
				},
				RetrySettings: exporterhelper.RetrySettings{
					Enabled:         true,
					InitialInterval: 10 * time.Second,
					MaxInterval:     1 * time.Minute,
					MaxElapsedTime:  10 * time.Minute,
				},
				QueueSettings: exporterhelper.QueueSettings{
					Enabled:      true,
					NumConsumers: 2,
					QueueSize:    10,
				},
				HecToOtelAttrs: splunk.HecToOtelAttrs{
					Source:     "mysource",
					SourceType: "mysourcetype",
					Index:      "myindex",
					Host:       "myhost",
				},
				HecFields: OtelToHecFields{
					SeverityText:   "myseverityfield",
					SeverityNumber: "myseveritynumfield",
				},
				HealthPath:            "/services/collector/health",
				HecHealthCheckEnabled: false,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.id.String(), func(t *testing.T) {
			factory := NewFactory()
			cfg := factory.CreateDefaultConfig()

			sub, err := cm.Sub(tt.id.String())
			require.NoError(t, err)
			require.NoError(t, component.UnmarshalConfig(sub, cfg))

			assert.NoError(t, component.ValidateConfig(cfg))
			assert.Equal(t, tt.expected, cfg)
		})
	}
}

func TestConfig_Validate(t *testing.T) {
	tests := []struct {
		name    string
		cfg     *Config
		wantErr string
	}{
		{
			name:    "default",
			cfg:     createDefaultConfig().(*Config),
			wantErr: "requires a non-empty \"endpoint\"",
		},
		{
			name: "bad url",
			cfg: func() *Config {
				cfg := createDefaultConfig().(*Config)
				cfg.HTTPClientSettings.Endpoint = "cache_object:foo/bar"
				cfg.Token = "foo"
				return cfg
			}(),
			wantErr: "invalid \"endpoint\": parse \"cache_object:foo/bar\": first path segment in URL cannot contain colon",
		},
		{
			name: "missing token",
			cfg: func() *Config {
				cfg := createDefaultConfig().(*Config)
				cfg.HTTPClientSettings.Endpoint = "http://example.com"
				return cfg
			}(),
			wantErr: "requires a non-empty \"token\"",
		},
		{
			name: "max default content-length for logs",
			cfg: func() *Config {
				cfg := createDefaultConfig().(*Config)
				cfg.HTTPClientSettings.Endpoint = "http://foo_bar.com"
				cfg.MaxContentLengthLogs = maxContentLengthLogsLimit + 1
				cfg.Token = "foo"
				return cfg
			}(),
			wantErr: "requires \"max_content_length_logs\" <= 838860800",
		},
		{
			name: "max default content-length for metrics",
			cfg: func() *Config {
				cfg := createDefaultConfig().(*Config)
				cfg.HTTPClientSettings.Endpoint = "http://foo_bar.com"
				cfg.MaxContentLengthMetrics = maxContentLengthMetricsLimit + 1
				cfg.Token = "foo"
				return cfg
			}(),
			wantErr: "requires \"max_content_length_metrics\" <= 838860800",
		},
		{
			name: "max default content-length for traces",
			cfg: func() *Config {
				cfg := createDefaultConfig().(*Config)
				cfg.HTTPClientSettings.Endpoint = "http://foo_bar.com"
				cfg.MaxContentLengthTraces = maxContentLengthTracesLimit + 1
				cfg.Token = "foo"
				return cfg
			}(),
			wantErr: "requires \"max_content_length_traces\" <= 838860800",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.cfg.Validate()
			if tt.wantErr == "" {
				require.NoError(t, err)
			} else {
				require.EqualError(t, err, tt.wantErr)
			}
		})
	}
}
