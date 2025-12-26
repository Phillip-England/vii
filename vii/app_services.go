package vii

// Use registers global services on the app.
// These services run for every request that reaches a mounted route (and all WS phases).
//
// Ordering:
// - Before: global services, then route services
// - After: reverse
func (a *App) Use(svcs ...Service) *App {
	if a == nil {
		return a
	}
	for _, s := range svcs {
		if s == nil {
			continue
		}
		a.services = append(a.services, s)
	}
	return a
}

// GlobalServices returns a snapshot of global services (safe to iterate).
func (a *App) GlobalServices() []Service {
	if a == nil || len(a.services) == 0 {
		return nil
	}
	out := make([]Service, len(a.services))
	copy(out, a.services)
	return out
}
