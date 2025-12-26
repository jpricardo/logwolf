package data

type JSONLogPayload struct {
	Name     string   `json:"name"`
	Data     string   `json:"data"`
	Severity string   `json:"severity"`
	Tags     []string `json:"tags"`
}

type RPCLogPayload struct {
	Name     string
	Data     string
	Severity string
	Tags     []string
}
