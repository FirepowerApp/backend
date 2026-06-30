package liveactivity

// debugChannels maps NHL team tricodes to APNs broadcast channel IDs created in
// the Development environment (App Store Connect → Push Notifications → Broadcast → Development).
// Use these with sandbox APNs and debug/simulator builds.
var debugChannels = map[string]string{
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
	"MTL": "uj76s1ikEfEAAObg6VLmpQ==",
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
	"VGK": "CVEIvlilEfEAAK6J2RjHtg==",
	"WPG": "",
	"WSH": "",
}

// prodChannels maps NHL team tricodes to APNs broadcast channel IDs created in
// the Production environment (App Store Connect → Push Notifications → Broadcast → Production).
// Use these with production APNs and App Store / TestFlight builds.
var prodChannels = map[string]string{
	"ANA": "",
	"BOS": "",
	"BUF": "",
	"CAR": "d4E+F1ikEfEAAF7OhT1omw==",
	"CBJ": "",
	"CGY": "",
	"CHI": "VeHR2HNPEfEAAIIz9BtJIg==",
	"COL": "S7cBYlilEfEAADY6cc28lw==",
	"DAL": "",
	"DET": "",
	"EDM": "",
	"FLA": "",
	"LAK": "",
	"MIN": "",
	"MTL": "GOmex1ilEfEAAOYo81Crcg==",
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
	"VGK": "Y8GZXlikEfEAAE4quYgbSQ==",
	"WPG": "",
	"WSH": "",
}

// channelForTeam looks up the APNs broadcast channel ID for a tricode.
// Returns ("", false) if the team is unknown or its channel ID is not populated.
func channelForTeam(tricode string, useDevChannels bool) (string, bool) {
	m := prodChannels
	if useDevChannels {
		m = debugChannels
	}
	id, known := m[tricode]
	if !known || id == "" {
		return "", false
	}
	return id, true
}
