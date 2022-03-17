package auth

import "go.mongodb.org/mongo-driver/bson"

func (actions Actions) MarshalBSON() ([]byte, error) {
	out := make(map[string]map[string]int)
	for ref, actions := range actions {
		out[ref.String()] = actions
	}
	return bson.Marshal(out)
}
