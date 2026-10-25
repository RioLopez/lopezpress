package services

import "time"

// ReadinessProbe answers "can this process actually serve a request right now",
// as opposed to "is the process running".
//
// The distinction matters for deployment, and getting it wrong is worse than
// having no probe at all:
//
//   - Liveness (the process is up) must NOT touch the database. If the disk is
//     full or the SQLite file is corrupt, restarting the process does not fix
//     anything — an orchestrator that treats a DB failure as "dead process"
//     will restart-loop the service and turn a degraded state into an outage.
//   - Readiness (this instance should receive traffic) MUST touch it, because a
//     process that is running but cannot read its own database is precisely the
//     case a load balancer or a supervisor needs to know about.
//
// The old /health returned {"status":"ok"} unconditionally, so a site whose
// database had become unreadable looked perfectly healthy to every monitor
// while every real request failed. That is the failure mode this exists to
// remove.
//
// The probe is deliberately tiny — one `SELECT 1` with a short deadline — so it
// can be called by a supervisor every few seconds without cost, and short
// enough that it never holds a write lock behind a slow operation.
func (s *Service) ReadinessProbe() error {
	done := make(chan error, 1)
	go func() {
		var one int
		done <- s.db.Raw("SELECT 1").Scan(&one).Error
	}()
	select {
	case err := <-done:
		return err
	case <-time.After(readinessTimeout):
		// A wedged database is not ready. This is a distinct outcome from an
		// error: the connection is alive but not answering, which usually means
		// a lock held by another connection or an I/O stall.
		return errReadinessTimeout
	}
}

// readinessTimeout bounds the probe. 2 s is long enough for a busy 1-core box to
// answer a trivial query and short enough that a supervisor with a 5 s period
// still sees a verdict every cycle.
const readinessTimeout = 2 * time.Second

type readinessError string

func (e readinessError) Error() string { return string(e) }

const errReadinessTimeout = readinessError("database did not answer the readiness probe in time")
