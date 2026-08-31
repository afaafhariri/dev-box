package servers

import (
	"devenv/internal/detector"
	"devenv/internal/detectors/simple"
	"devenv/internal/probe"
)

// mysqlPort is the default MySQL port.
const mysqlPort = 3306

// NewMySQL detects MySQL (or MariaDB, which ships the same client names) and
// whether a server is answering.
func NewMySQL(p probe.Prober) detector.Detector {
	return simple.Service{
		Tool: simple.Tool{
			ItemName: "MySQL",
			Cat:      detector.CategoryServer,
			Bins:     []string{"mysql", "mysqld", "mariadb"},
			P:        p,
		},
		Port: mysqlPort,
	}
}
