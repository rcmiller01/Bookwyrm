package contract

type EntityRef struct {
	Type      string
	ID        string
	ParentIDs map[string]string
}
