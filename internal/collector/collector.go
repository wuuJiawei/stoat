package collector

import (
	"context"
	"fmt"

	"github.com/wuuJiawei/stoat/internal/model"
)

type Warning struct {
	Collector string `json:"collector"`
	Path      string `json:"path,omitempty"`
	Message   string `json:"message"`
}

func NewWarning(collectorName, path string, err error) Warning {
	return Warning{Collector: collectorName, Path: path, Message: err.Error()}
}

func (w Warning) Error() string {
	if w.Path == "" {
		return fmt.Sprintf("%s: %s", w.Collector, w.Message)
	}
	return fmt.Sprintf("%s (%s): %s", w.Collector, w.Path, w.Message)
}

type Result struct {
	Items    []model.PersistenceItem
	Warnings []Warning
}

type Collector interface {
	Name() string
	Collect(ctx context.Context) Result
}
