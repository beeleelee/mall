package a2a

type Role string

const (
	RoleUser  Role = "user"
	RoleAgent Role = "agent"
)

type PartType string

const (
	PartTypeText PartType = "text"
	PartTypeFile PartType = "file"
	PartTypeData PartType = "data"
)

type FileContent struct {
	MimeType string `json:"mimeType"`
	Data     string `json:"data"`
	Name     string `json:"name,omitempty"`
}

type Part struct {
	Type     PartType       `json:"type"`
	Text     string         `json:"text,omitempty"`
	File     *FileContent   `json:"file,omitempty"`
	Data     any            `json:"data,omitempty"`
	Metadata map[string]any `json:"metadata,omitempty"`
}

func TextPart(text string) Part {
	return Part{Type: PartTypeText, Text: text}
}

func FilePart(mimeType, data, name string) Part {
	return Part{
		Type: PartTypeFile,
		File: &FileContent{MimeType: mimeType, Data: data, Name: name},
	}
}

func DataPart(data any) Part {
	return Part{Type: PartTypeData, Data: data}
}

type Message struct {
	Role     Role           `json:"role"`
	Parts    []Part         `json:"parts"`
	Metadata map[string]any `json:"metadata,omitempty"`
}
