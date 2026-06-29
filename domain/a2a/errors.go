package a2a

import "errors"

var (
	ErrTaskNotFound                = errors.New("task not found")
	ErrTaskNotCancelable           = errors.New("task is not cancelable")
	ErrPushNotificationNotSupported = errors.New("push notifications not supported")
	ErrUnsupportedOperation        = errors.New("unsupported operation")
	ErrContentTypeNotSupported     = errors.New("content type not supported")
	ErrInvalidAgentResponse        = errors.New("invalid agent response")
	ErrVersionNotSupported         = errors.New("a2a version not supported")
	ErrExtensionSupportRequired    = errors.New("extension support required")
	ErrSkillNotFound               = errors.New("skill not found")
)

type A2AError struct {
	Code    string
	Message string
	Err     error
}

func (e *A2AError) Error() string {
	if e.Err != nil {
		return e.Code + ": " + e.Err.Error()
	}
	return e.Code + ": " + e.Message
}

func (e *A2AError) Unwrap() error {
	return e.Err
}

func NewA2AError(err error, msg string) *A2AError {
	return &A2AError{Code: err.Error(), Message: msg, Err: err}
}
