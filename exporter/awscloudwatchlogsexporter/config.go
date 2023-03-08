// Copyright 2020, OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package awscloudwatchlogsexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/awscloudwatchlogsexporter"

import (
	"errors"
	"regexp"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/awsutil"
)

// Config represent a configuration for the CloudWatch logs exporter.
type Config struct {
	exporterhelper.RetrySettings `mapstructure:"retry_on_failure"`

	// LogGroupName is the name of CloudWatch log group which defines group of log streams
	// that share the same retention, monitoring, and access control settings.
	LogGroupName string `mapstructure:"log_group_name"`

	// LogStreamName is the name of CloudWatch log stream which is a sequence of log events
	// that share the same source.
	LogStreamName string `mapstructure:"log_stream_name"`

	// Endpoint is the CloudWatch Logs service endpoint which the requests
	// are forwarded to. https://docs.aws.amazon.com/general/latest/gr/cwl_region.html
	// e.g. logs.us-east-1.amazonaws.com
	// Optional.
	Endpoint string `mapstructure:"endpoint"`

	// LogRetention is the option to set the log retention policy for the CloudWatch Log Group. Defaults to Never Expire if not specified or set to 0
	// Possible values are 1, 3, 5, 7, 14, 30, 60, 90, 120, 150, 180, 365, 400, 545, 731, 1827, 2192, 2557, 2922, 3288, or 3653
	LogRetention int64 `mapstructure:"log_retention"`

	// Tags is the option to set tags for the CloudWatch Log Group.  If specified, please add add at least 1 and at most 50 tags.  Input is a string to string map like so: { 'key': 'value' }
	// Keys must be between 1-128 characters and follow the regex pattern: ^([\p{L}\p{Z}\p{N}_.:/=+\-@]+)$
	// Values must be between 1-256 characters and follow the regex pattern: ^([\p{L}\p{Z}\p{N}_.:/=+\-@]*)$
	Tags map[string]*string `mapstructure:"tags"`

	// QueueSettings is a subset of exporterhelper.QueueSettings,
	// because only QueueSize is user-settable due to how AWS CloudWatch API works
	QueueSettings QueueSettings `mapstructure:"sending_queue"`

	logger *zap.Logger

	awsutil.AWSSessionSettings `mapstructure:",squash"`

	// Export raw log string instead of log wrapper
	// Required for emf logs
	RawLog bool `mapstructure:"raw_log,omitempty"`
}

type QueueSettings struct {
	// QueueSize set the length of the sending queue
	QueueSize int `mapstructure:"queue_size"`
}

var _ component.Config = (*Config)(nil)

// Validate config
func (config *Config) Validate() error {
	if config.LogGroupName == "" {
		return errors.New("'log_group_name' must be set")
	}
	if config.LogStreamName == "" {
		return errors.New("'log_stream_name' must be set")
	}
	if config.QueueSettings.QueueSize < 1 {
		return errors.New("'sending_queue.queue_size' must be 1 or greater")
	}
	if !isValidRetentionValue(config.LogRetention) {
		return errors.New("invalid value for retention policy.  Please make sure to use the following values: 0 (Never Expire), 1, 3, 5, 7, 14, 30, 60, 90, 120, 150, 180, 365, 400, 545, 731, 1827, 2192, 2557, 2922, 3288, or 3653")
	}
	tagInputReturnVal := isValidTagsInput(config.Tags)
	if tagInputReturnVal != "Valid" {
		return errors.New(tagInputReturnVal)
	}
	return nil
}

// Added function to check if value is an accepted number of log retention days
func isValidRetentionValue(input int64) bool {
	switch input {
	case
		0,
		1,
		3,
		5,
		7,
		14,
		30,
		60,
		90,
		120,
		150,
		180,
		365,
		400,
		545,
		731,
		1827,
		2192,
		2557,
		2922,
		3288,
		3653:
		return true
	}
	return false
}

// Check if the tags input is valid
func isValidTagsInput(input map[string]*string) string {
	if len(input) > 50 {
		return "invalid amount of items. Please input at most 50 tags."
	}
	validKeyPattern := regexp.MustCompile(`^([\p{L}\p{Z}\p{N}_.:/=+\-@]+)$`)
	validValuePattern := regexp.MustCompile(`^([\p{L}\p{Z}\p{N}_.:/=+\-@]*)$`)
	for key, value := range input {
		if len(key) < 1 || len(key) > 128 {
			return "key - " + key + " has an invalid length. Please use keys with a length of 1 to 128 characters"
		}
		if len(*value) < 1 || len(*value) > 256 {
			return "value - " + *value + " has an invalid length. Please use values with a length of 1 to 256 characters"
		}
		if !validKeyPattern.MatchString(key) {
			return "key - " + key + " does not follow the regex pattern" + `^([\p{L}\p{Z}\p{N}_.:/=+\-@]+)$`
		}
		if !validValuePattern.MatchString(*value) {
			return "value - " + *value + " does not follow the regex pattern" + `^([\p{L}\p{Z}\p{N}_.:/=+\-@]*)$`
		}
	}

	return "Valid"
}

func (config *Config) enforcedQueueSettings() exporterhelper.QueueSettings {
	return exporterhelper.QueueSettings{
		Enabled: true,
		// due to the sequence token, there can be only one request in flight
		NumConsumers: 1,
		QueueSize:    config.QueueSettings.QueueSize,
	}
}

// TODO(jbd): Add ARN role to config.
