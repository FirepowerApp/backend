// Package season determines whether the NHL season is currently active and
// resolves which data source (live NHL/MoneyPuck APIs, or the staging game
// data emulator) the pipeline should use.
//
//	SEASON_OVERRIDE set?  ──yes──►  use override, skip probe
//	       │no
//	       ▼
//	probe NHL_SEASON_API_BASE_URL/v1/schedule/{today}  (bounded ~5-10s context)
//	offseason ⟺ today < preSeasonStartDate && numberOfGames == 0
//	any error / timeout / bad response / unparseable date  ──►  in-season (fail-safe)
package season

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"
)

const defaultNHLSeasonAPIBaseURL = "https://api-web.nhle.com"

// probeTimeout bounds the live NHL detection probe so a hanging API can't
// starve the scheduler's overall run budget (2 minutes in
// cmd/schedulegametrackers/main.go). On timeout the probe fails and the
// caller falls back to in-season.
const probeTimeout = 8 * time.Second

// scheduleResponse mirrors the fields of the NHL schedule API response
// (GET /v1/schedule/{date}) relevant to offseason detection. Verified against
// a live response 2026-08-23: querying an offseason date returns the
// *upcoming* season's boundaries (e.g. preSeasonStartDate in the future) with
// numberOfGames == 0.
type scheduleResponse struct {
	PreSeasonStartDate string `json:"preSeasonStartDate"`
	NumberOfGames      int    `json:"numberOfGames"`
}

// Detector determines whether today is in the NHL offseason.
type Detector struct {
	// BaseURL is the fixed, live NHL API used for detection. It is
	// intentionally independent of the swappable schedule/data-fetch base
	// URLs so detection can never be circular (asking the emulator whether
	// to use the emulator).
	BaseURL string
	// Override, if non-empty, short-circuits detection: "offseason" or
	// "inseason". Any other value is ignored (falls through to the probe).
	// Lets `make schedule-test` and other local runs exercise the offseason
	// routing path on demand instead of waiting for the real calendar.
	Override string

	httpClient *http.Client
}

// NewDetector creates a Detector from environment configuration.
// baseURL defaults to the live NHL API when empty.
func NewDetector(baseURL, override string) *Detector {
	if baseURL == "" {
		baseURL = defaultNHLSeasonAPIBaseURL
	}
	return &Detector{
		BaseURL:    baseURL,
		Override:   override,
		httpClient: &http.Client{Timeout: probeTimeout},
	}
}

// IsOffseason reports whether today is in the NHL offseason. It fails safe:
// any override value other than "offseason"/"inseason", or any probe error,
// results in "in-season" (false) so a detection failure never accidentally
// routes staging (or, gated separately, production) to simulated data.
func (d *Detector) IsOffseason(ctx context.Context) bool {
	switch d.Override {
	case "offseason":
		return true
	case "inseason":
		return false
	}

	probeCtx, cancel := context.WithTimeout(ctx, probeTimeout)
	defer cancel()

	offseason, err := d.probe(probeCtx, time.Now().UTC().Format("2006-01-02"))
	if err != nil {
		log.Printf("season: offseason detection probe failed, defaulting to in-season: %v", err)
		return false
	}
	return offseason
}

// probe queries the live NHL schedule API for the given date and applies the
// offseason rule: today < preSeasonStartDate, corroborated by numberOfGames
// == 0 for the probed window (guards the brief window right after playoffs
// end where the API may not yet report next-season context).
func (d *Detector) probe(ctx context.Context, today string) (bool, error) {
	url := fmt.Sprintf("%s/v1/schedule/%s", d.BaseURL, today)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return false, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := d.httpClient.Do(req)
	if err != nil {
		return false, fmt.Errorf("failed to fetch schedule: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return false, fmt.Errorf("NHL API returned status %d", resp.StatusCode)
	}

	var sched scheduleResponse
	if err := json.NewDecoder(resp.Body).Decode(&sched); err != nil {
		return false, fmt.Errorf("failed to decode schedule response: %w", err)
	}

	if sched.PreSeasonStartDate == "" {
		return false, fmt.Errorf("schedule response missing preSeasonStartDate")
	}

	todayDate, err := time.Parse("2006-01-02", today)
	if err != nil {
		return false, fmt.Errorf("failed to parse probe date %q: %w", today, err)
	}
	preSeasonDate, err := time.Parse("2006-01-02", sched.PreSeasonStartDate)
	if err != nil {
		return false, fmt.Errorf("failed to parse preSeasonStartDate %q: %w", sched.PreSeasonStartDate, err)
	}

	return todayDate.Before(preSeasonDate) && sched.NumberOfGames == 0, nil
}

// OverrideFromEnv reads SEASON_OVERRIDE from the environment.
func OverrideFromEnv() string {
	return os.Getenv("SEASON_OVERRIDE")
}
