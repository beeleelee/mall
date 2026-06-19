package metrics

type Label struct {
	Name  string
	Value string
}

type MetricsRecorder interface {
	IncCounter(name string, labels ...Label)
	ObserveHistogram(name string, value float64, labels ...Label)
	SetGauge(name string, value float64, labels ...Label)
}

func NewLabel(name, value string) Label {
	return Label{Name: name, Value: value}
}
