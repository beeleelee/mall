package a2a

import "github.com/beeleelee/mall/domain/kernel"

type PushNotificationConfig struct {
	kernel.AggregateRoot
	TaskID   kernel.ID `json:"taskId"`
	URL      string    `json:"url"`
	AuthInfo AuthInfo  `json:"authInfo,omitempty"`
}

type AuthInfo struct {
	Scheme      string `json:"scheme"`
	Credentials string `json:"credentials,omitempty"`
}

func NewPushNotificationConfig(id, taskID kernel.ID, url string, auth AuthInfo) *PushNotificationConfig {
	return &PushNotificationConfig{
		AggregateRoot: kernel.NewAggregateRoot(id),
		TaskID:        taskID,
		URL:           url,
		AuthInfo:      auth,
	}
}
