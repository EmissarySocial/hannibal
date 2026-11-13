package delivery

import "github.com/benpate/turbine/queue"

// pipelineConsumer runs a Pipeline's two tasks on a turbine queue
type pipelineConsumer struct {
	pipeline Pipeline
}

// Consumer returns a turbine queue.Consumer that runs the fan-out and send tasks of a Pipeline
func Consumer(pipeline Pipeline) queue.Consumer {
	return &pipelineConsumer{pipeline: pipeline}
}

// OnPublish implements the queue.Consumer interface
func (consumer *pipelineConsumer) OnPublish(*queue.Task) error {
	// A delivery needs nothing changed on its way onto the queue
	return nil
}

// Run executes one attempt of a fan-out or send task, and ignores every other task
func (consumer *pipelineConsumer) Run(task queue.Task) queue.Result {

	switch task.Name {

	case TaskFanOut:
		return consumer.pipeline.FanOut(task.Arguments)

	case TaskSend:
		return consumer.pipeline.Send(task.Arguments)
	}

	// Every other task name belongs to another consumer
	return queue.Ignored()
}

// OnSuccess implements the queue.Consumer interface
func (consumer *pipelineConsumer) OnSuccess(queue.Task) error {
	// A delivered activity reports nothing
	return nil
}

// OnError implements the queue.Consumer interface
func (consumer *pipelineConsumer) OnError(queue.Task, error) error {
	// A delivery that will be retried reports nothing
	return nil
}

// OnFailure implements the queue.Consumer interface
func (consumer *pipelineConsumer) OnFailure(queue.Task, error) error {
	// The queue reports a failed delivery once, so there is nothing to add
	return nil
}
