package entity

type PineconeVector struct {
	ID       string
	Values   []float32
	Metadata map[string]interface{}
}

type PineconeFilter map[string]interface{}
