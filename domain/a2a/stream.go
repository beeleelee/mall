package a2a

type StreamResponse struct {
	Task               *Task                    `json:"task,omitempty"`
	Message            *Message                 `json:"message,omitempty"`
	TaskStatusUpdate   *TaskStatusUpdateEvent   `json:"statusUpdateEvent,omitempty"`
	TaskArtifactUpdate *TaskArtifactUpdateEvent `json:"artifactUpdateEvent,omitempty"`
	Error              *StreamError             `json:"error,omitempty"`
}

type TaskStatusUpdateEvent struct {
	TaskID string     `json:"taskId"`
	Status TaskStatus `json:"status"`
}

type TaskArtifactUpdateEvent struct {
	TaskID   string   `json:"taskId"`
	Artifact Artifact `json:"artifact"`
}

type StreamError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}
