package planner

import (
	"github.com/DBCDK/morph/healthchecks"
	"github.com/DBCDK/morph/steps"
)

func HealthCheckToCommand(check healthchecks.CmdHealthCheck) CommandPlus {
	return CommandPlus{
		Command: Command{
			Description: check.Description,
			Command:     []string{"/bin/true"},
		},
		Period:  check.Period,
		Timeout: check.Timeout,
	}
}

func HealthChecksToCommands(checks []healthchecks.CmdHealthCheck) []CommandPlus {
	commands := make([]CommandPlus, 0)

	for _, check := range checks {
		commands = append(commands, HealthCheckToCommand(check))
	}

	return commands
}

func HealthCheckToRequest(check healthchecks.HttpHealthCheck) RequestPlus {

	return RequestPlus{
		Request: steps.Request{
			Description: check.Description,
			Headers:     check.Headers,
			Host:        check.Host,
			InsecureSSL: check.InsecureSSL,
			Path:        check.Path,
			Port:        check.Port,
			Scheme:      check.Scheme,
		},
		Period:  check.Period,
		Timeout: check.Timeout,
	}
}

func HealthChecksToRequests(checks []healthchecks.HttpHealthCheck) []RequestPlus {
	reqs := make([]RequestPlus, 0)

	for _, check := range checks {
		reqs = append(reqs, HealthCheckToRequest(check))
	}

	return reqs
}
