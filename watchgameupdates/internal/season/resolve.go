package season

// DataSource identifies which backend a task's data fetches should hit.
// It travels on the task payload (models.Payload.DataSource) rather than raw
// URLs, so the enum is the only thing that crosses the Cloud Tasks wire and
// the handler owns the mapping to concrete hostnames.
type DataSource string

const (
	// DataSourceLive is the default: live NHL API / MoneyPuck. An empty
	// string on a payload (e.g. a task enqueued before this deploy) is
	// treated identically to DataSourceLive.
	DataSourceLive DataSource = "live"
	// DataSourceEmulator routes to the in-cluster game data emulator.
	// Only ever produced for APP_ENV=="staging"; see ResolveDataSource.
	DataSourceEmulator DataSource = "emulator"
)

// ResolveDataSource decides the effective DataSource for a scheduler run.
// Emulator routing is gated hard on APP_ENV=="staging" — production always
// resolves to live regardless of the offseason signal, so a detection false
// positive can never route real users to simulated data.
func ResolveDataSource(appEnv string, offseason bool) DataSource {
	if appEnv == "staging" && offseason {
		return DataSourceEmulator
	}
	return DataSourceLive
}

// BaseURLs holds the live and emulator base URLs for a single service.
type BaseURLs struct {
	Live     string
	Emulator string
}

// ResolveBaseURL maps a task's DataSource to a concrete base URL. It re-gates
// on appEnv (not just the DataSource value) as defense in depth: a task
// carrying DataSourceEmulator that somehow reaches a non-staging handler
// (replayed task, misconfigured env) falls back to live rather than resolving
// to an emulator URL that may be empty or unreachable outside staging.
func ResolveBaseURL(ds DataSource, appEnv string, urls BaseURLs) string {
	if ds == DataSourceEmulator && appEnv == "staging" {
		return urls.Emulator
	}
	return urls.Live
}
