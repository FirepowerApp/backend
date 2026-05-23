package liveactivity

// teamChannels maps NHL team tricodes to their Apple-assigned APNs broadcast
// channel IDs. Generate a channel ID from the iOS app for each team, then
// paste the value next to the corresponding tricode.
//
// Teams with an empty string are skipped silently at push time — the backend
// will still push to the other team in the game if its channel ID is populated.
var teamChannels = map[string]string{
	"ANA": "PHP5yU/pEfEAAMK1EJkzxg==",
	"BOS": "",
	"BUF": "iqg6AFASEfEAAL4dEtalNQ==",
	"CAR": "5GIeaFVyEfEAAObg6VLmpQ==",
	"CBJ": "",
	"CGY": "",
	"CHI": "",
	"COL": "+pSGy0vgEfEAAKqhstn/Jg==",
	"DAL": "",
	"DET": "",
	"EDM": "",
	"FLA": "",
	"LAK": "",
	"MIN": "",
	"MTL": "",
	"NJD": "",
	"NSH": "",
	"NYI": "",
	"NYR": "",
	"OTT": "",
	"PHI": "",
	"PIT": "",
	"SEA": "",
	"SJS": "",
	"STL": "",
	"TBL": "",
	"TOR": "",
	"UTA": "",
	"VAN": "",
	"VGK": "",
	"WPG": "",
	"WSH": "",
}

// channelForTeam looks up the APNs broadcast channel ID for a tricode.
// Returns ("", false) if the team is unknown or its channel ID is not yet populated.
func channelForTeam(tricode string) (string, bool) {
	id, known := teamChannels[tricode]
	if !known || id == "" {
		return "", false
	}
	return id, true
}
