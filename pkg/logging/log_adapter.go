package logging

import (
	"regexp"
	"strings"

	"code.cloudfoundry.org/lager"
	"github.com/go-logr/logr"
)

type LogAdapter struct {
	logger         lager.Logger
	debugThreshold int
}

var _ logr.LogSink = &LogAdapter{}
var cleanRegexp regexp.Regexp = *regexp.MustCompile("[^a-zA-Z0-9-]+")

func NewLogAdapter(logger lager.Logger) LogAdapter {
	return NewLogAdapterWithDebugThreshold(logger, 0)
}

func NewLogAdapterWithDebugThreshold(logger lager.Logger, debugThreshold int) LogAdapter {
	return LogAdapter{
		logger:         logger,
		debugThreshold: debugThreshold,
	}
}

func (LogAdapter) Init(info logr.RuntimeInfo) {}

func (LogAdapter) Enabled(level int) bool {
	return true
}

func (l LogAdapter) Info(level int, msg string, keysAndValues ...interface{}) {
	msg = toConcourseStyle(msg)
	if level <= l.debugThreshold {
		l.logger.Info(msg, toLagerData(keysAndValues...))
	} else {
		l.logger.Debug(msg, toLagerData(keysAndValues...))
	}
}

func (l LogAdapter) Error(err error, msg string, keysAndValues ...interface{}) {
	l.logger.Error(toConcourseStyle(msg), err, toLagerData(keysAndValues...))
}

func (l LogAdapter) WithValues(keysAndValues ...interface{}) logr.LogSink {
	return LogAdapter{
		logger: l.logger.WithData(toLagerData(keysAndValues...)),
	}
}

func (l LogAdapter) WithName(name string) logr.LogSink {
	return LogAdapter{
		logger: l.logger.Session(toConcourseStyle(name)),
	}
}

func toConcourseStyle(str string) string {
	str = strings.ReplaceAll(strings.ToLower(str), " ", "-")
	return cleanRegexp.ReplaceAllString(str, "")
}

func toLagerData(keysAndValues ...interface{}) lager.Data {
	data := lager.Data{}
	for i := 0; i < len(keysAndValues); i += 2 {
		data[keysAndValues[i].(string)] = keysAndValues[i+1]
	}

	return data
}
