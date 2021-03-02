package acc

type Streams struct {
	m map[string]StreamRef
}

type StreamRef struct {
	Job, Stream string
}

