package pkg

import "bytes"

type Deserializer interface {
	Deserialize([]byte) Serializable
}

type Serializable interface {
	Serialize(*bytes.Buffer)
}
