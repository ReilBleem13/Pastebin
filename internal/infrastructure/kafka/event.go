package kafka

const (
	textIndexedTopic = "text.indexed"
)

type Event interface {
	Key() string
	Topic() string
}

type TextIndexed struct {
	ObjectID string `json:"object_id"`
	Index    string `json:"index"`
}

func (e TextIndexed) Key() string   { return e.ObjectID }
func (e TextIndexed) Topic() string { return textIndexedTopic }
