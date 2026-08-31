package servers

import (
	"devenv/internal/detector"
	"devenv/internal/detectors/simple"
	"devenv/internal/probe"
)

// postgresPort is the default PostgreSQL port.
const postgresPort = 5432

// NewPostgres detects PostgreSQL and whether a server is answering.
func NewPostgres(p probe.Prober) detector.Detector {
	return simple.Service{
		Tool: simple.Tool{
			ItemName: "PostgreSQL",
			Cat:      detector.CategoryServer,
			// psql is the client; postgres is the server binary. Either
			// presence means PostgreSQL is installed here.
			Bins: []string{"psql", "postgres", "pg_ctl"},
			P:    p,
		},
		Port: postgresPort,
		// pg_isready is the purpose-built liveness check and never blocks.
		Ping: []string{"pg_isready", "-q", "-p", "5432"},
	}
}
