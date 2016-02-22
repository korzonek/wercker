//   Copyright 2016 Wercker Holding BV
//
//   Licensed under the Apache License, Version 2.0 (the "License");
//   you may not use this file except in compliance with the License.
//   You may obtain a copy of the License at
//
//       http://www.apache.org/licenses/LICENSE-2.0
//
//   Unless required by applicable law or agreed to in writing, software
//   distributed under the License is distributed on an "AS IS" BASIS,
//   WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
//   See the License for the specific language governing permissions and
//   limitations under the License.

package event

import (
	"errors"
	"fmt"
	"time"

	"github.com/inconshreveable/go-keen"
	"github.com/wercker/wercker/core"
	"github.com/wercker/wercker/util"
)

// NewMetricsHandler will create a new NewMetricsHandler.
func NewMetricsHandler(opts *core.PipelineOptions) (*MetricsEventHandler, error) {
	if "" == opts.KeenProjectWriteKey {
		return nil, errors.New("No KeenProjectWriteKey specified")
	}

	if "" == opts.KeenProjectID {
		return nil, errors.New("No KeenProjectID specified")
	}

	keenInstance := &keen.Client{
		WriteKey:  opts.KeenProjectWriteKey,
		ProjectID: opts.KeenProjectID,
	}

	versions := util.GetVersions()

	return &MetricsEventHandler{
		keen:      keenInstance,
		versions:  versions,
		startStep: make(map[string]time.Time),
	}, nil
}

// A MetricsEventHandler reporting to keen.io.
type MetricsEventHandler struct {
	keen          *keen.Client
	startStep     map[string]time.Time
	startRun      time.Time
	versions      *util.Versions
	numSteps      int
	numAfterSteps int
}

// ListenTo will add eventhandlers to e.
func (h *MetricsEventHandler) ListenTo(e *core.NormalizedEmitter) {
	e.AddListener(core.BuildStepStarted, h.BuildStepStarted)
	e.AddListener(core.BuildStepFinished, h.BuildStepFinished)
	e.AddListener(core.BuildStepsAdded, h.BuildStepsAdded)

	e.AddListener(core.BuildStarted, h.BuildStarted)
	e.AddListener(core.BuildFinished, h.BuildFinished)
}

func newMetricsKeenPayload(now time.Time) *metricsKeenPayload {
	return &metricsKeenPayload{Timestamp: now.Format(time.RFC3339)}
}

// BuildStarted responds to the BuildStarted event.
func (h *MetricsEventHandler) BuildStarted(args *core.BuildStartedArgs) {
	now := time.Now()

	h.startRun = now

	p := &MetricsPayload{}
	h.sendPayload(&sendPayloadArgs{
		p:         p,
		options:   args.Options,
		now:       now,
		eventName: "runStarted",
	})
}

// BuildFinished responds to the BuildFinished event.
func (h *MetricsEventHandler) BuildFinished(args *core.BuildFinishedArgs) {
	now := time.Now()

	elapsed := now.Sub(h.startRun)
	duration := int64(elapsed.Seconds())

	success := args.Result == "passed"

	p := &MetricsPayload{
		Duration: &duration,
		Success:  &success,
	}
	h.sendPayload(&sendPayloadArgs{
		p:         p,
		options:   args.Options,
		box:       args.Box,
		now:       now,
		eventName: "runFinished",
	})
}

// BuildStepStarted responds to the BuildStepStarted event.
func (h *MetricsEventHandler) BuildStepStarted(args *core.BuildStepStartedArgs) {
	now := time.Now()

	h.startStep[args.Step.SafeID()] = now

	p := &MetricsPayload{
		Step:      newMetricStepPayload(args.Step),
		StepName:  formatUniqueStepName(args.Step),
		StepOrder: args.Order,
	}
	h.sendPayload(&sendPayloadArgs{
		p:         p,
		options:   args.Options,
		box:       args.Box,
		now:       now,
		eventName: "runStepStarted",
	})
}

// BuildStepFinished responds to the BuildStepFinished event.
func (h *MetricsEventHandler) BuildStepFinished(args *core.BuildStepFinishedArgs) {
	now := time.Now()

	var duration int64
	begin, ok := h.startStep[args.Step.SafeID()]
	if ok {
		elapsed := now.Sub(begin)
		duration = int64(elapsed.Seconds())
		delete(h.startStep, args.Step.SafeID())
	}

	p := &MetricsPayload{
		Step:      newMetricStepPayload(args.Step),
		StepName:  formatUniqueStepName(args.Step),
		StepOrder: args.Order,
		Duration:  &duration,
		Success:   &args.Successful,
		Message:   args.Message,
	}
	h.sendPayload(&sendPayloadArgs{
		p:         p,
		options:   args.Options,
		box:       args.Box,
		now:       now,
		eventName: "runStepFinished",
	})
}

// BuildStepsAdded handles the BuildStepsAdded event.
func (h *MetricsEventHandler) BuildStepsAdded(args *core.BuildStepsAddedArgs) {
	h.numSteps = len(args.Steps)
	h.numAfterSteps = len(args.AfterSteps)
}

type sendPayloadArgs struct {
	p         *MetricsPayload
	options   *core.PipelineOptions
	box       core.Box
	now       time.Time
	eventName string
}

func (h *MetricsEventHandler) sendPayload(args *sendPayloadArgs) {
	collection := getCollection(args.options)
	boxName, boxTag := getBoxDetails(args.box)

	p := args.p

	p.Keen = newMetricsKeenPayload(args.now)
	p.Timestamp = args.now.Unix()
	p.RunID = args.options.RunID
	p.Event = args.eventName
	p.StartedBy = args.options.ApplicationStartedByName
	p.MetricsApplicationPayload = &metricsApplicationPayload{
		ID:        args.options.ApplicationID,
		Name:      args.options.ApplicationName,
		OwnerName: args.options.ApplicationOwnerName,
	}
	p.SentCli = h.versions
	p.Stack = 5
	p.NumSteps = h.numSteps
	p.NumAfterSteps = h.numAfterSteps
	p.PipelineName = args.options.Pipeline
	p.BoxName = boxName
	p.BoxTag = boxTag
	h.keen.AddEvent(collection, p)
}

type metricsKeenPayload struct {
	Timestamp string `json:"timestamp"`
}

// metricsApplicationPayload is the app data we're reporting
// to keen. Part of MetricsPayload.
type metricsApplicationPayload struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	OwnerName string `json:"ownerName"`
}

func newMetricStepPayload(step core.Step) *metricStepPayload {
	return &metricStepPayload{
		Owner:      step.Owner(),
		Name:       step.Name(),
		Version:    step.Version(),
		FullName:   fmt.Sprintf("%s/%s", step.Owner(), step.Name()),
		UniqueName: formatUniqueStepName(step),
	}
}

type metricStepPayload struct {
	Owner   string `json:"owner,omitempty"`
	Name    string `json:"name,omitempty"`
	Version string `json:"version,omitempty"`

	FullName   string `json:"uniqueName,omitempty"` // Contains owner/name
	UniqueName string `json:"fullName,omitempty"`   // Contains owner/name@version
}

// MetricsPayload is the data we're sending to keen.
type MetricsPayload struct {
	Keen         *metricsKeenPayload `json:"keen"`
	Timestamp    int64               `json:"timestamp"`
	Event        string              `json:"event"`
	Stack        int                 `json:"stack,omitempty"`
	SentCli      *util.Versions      `json:"wercker,omitempty"`
	Grappler     *util.Versions      `json:"grappler,omitempty"`
	PipelineName string              `json:"pipelineName,omitempty"`

	RunID string `json:"runId,omitempty"`

	NumSteps      int `json:"numSteps,omitempty"`
	NumAfterSteps int `json:"numAfterSteps,omitempty"`

	BoxName string `json:"box,omitempty"`
	BoxTag  string `json:"boxTag,omitempty"`

	Step *metricStepPayload `json:"step,omitempty"`

	// Required for backwards compatibility:
	StepName  string `json:"stepName,omitempty"` // <- owner/name@version
	StepOrder int    `json:"stepOrder,omitempty"`

	Success   *bool  `json:"success,omitempty"`
	Message   string `json:"message,omitempty"`
	Duration  *int64 `json:"duration,omitempty"`
	StartedBy string `json:"startedBy,omitempty"`

	VCS                       string                     `json:"versionControl,omitempty"`
	MetricsApplicationPayload *metricsApplicationPayload `json:"application,omitempty"`
}

func getCollection(options *core.PipelineOptions) string {
	return "pipeline-events"
}

func getBoxDetails(box core.Box) (boxName string, boxTag string) {
	if box == nil {
		return
	}

	return box.GetName(), box.GetTag()
}

func formatUniqueStepName(step core.Step) string {
	return fmt.Sprintf("%s/%s@%s", step.Owner(), step.Name(), step.Version())
}
