package servers

import (
	"devenv/internal/detector"
	"devenv/internal/detectors/simple"
	"devenv/internal/probe"
)

// mongoPort is the default MongoDB port.
const mongoPort = 27017

// NewMongoDB detects MongoDB and whether a server is answering.
func NewMongoDB(p probe.Prober) detector.Detector {
	return simple.Service{
		Tool: simple.Tool{
			ItemName: "MongoDB",
			Cat:      detector.CategoryServer,
			// mongosh is the modern shell, mongo the legacy one, mongod the
			// server itself.
			Bins: []string{"mongod", "mongosh", "mongo"},
			P:    p,
		},
		Port: mongoPort,
	}
}
