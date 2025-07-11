package kafka

const (
	cleanExpiredTopic = "clean.expired"
)

type Event interface {
	Key() string
	Topic() string
}
type CleanExpired struct {
	ObjectIDs []string `json:"object_ids"`
}

func (e CleanExpired) Key() string   { return e.ObjectIDs[0] }
func (e CleanExpired) Topic() string { return cleanExpiredTopic }
