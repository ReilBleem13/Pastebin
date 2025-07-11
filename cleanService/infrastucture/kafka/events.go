package kafka

type CleanExpired struct {
	ObjectIDs []string `json:"object_ids"`
	Index     string   `json:"index"`
}
