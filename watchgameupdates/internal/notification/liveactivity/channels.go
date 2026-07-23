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
//
// Source of truth: the iOS repo (github.com/FirepowerApp/ios) at
// main:Firepower/NHLTeams.swift (prodChannelIds). Devices subscribe to these
// exact IDs, so the backend replicates that map verbatim. IDs are opaque tokens
// minted by App Store Connect and are NOT derivable from the tricode. When iOS
// mints or rotates a channel, re-copy the value here. TestReplicateAll32ProdChannels
// enforces that every team stays populated.
var prodChannels = map[string]string{
	"ANA": "UhYmnn8mEfEAAOYc0Q7CQQ==",
	"BOS": "+YMYln8mEfEAADq3u4xYzw==",
	"BUF": "Au/kwn8nEfEAAOYc0Q7CQQ==",
	"CAR": "d4E+F1ikEfEAAF7OhT1omw==",
	"CBJ": "Cd+IhX8nEfEAAOK+11eG0w==",
	"CGY": "ETvY2H8nEfEAAFr0zqCBeA==",
	"CHI": "VeHR2HNPEfEAAIIz9BtJIg==",
	"COL": "S7cBYlilEfEAADY6cc28lw==",
	"DAL": "GNBrYX8nEfEAALJMNogb7Q==",
	"DET": "IwHOp38nEfEAADq3u4xYzw==",
	"EDM": "KeUc6H8nEfEAADq3u4xYzw==",
	"FLA": "Z9sW/X8pEfEAAOYc0Q7CQQ==",
	"LAK": "b92xa38pEfEAAOK+11eG0w==",
	"MIN": "eotUn38pEfEAAOYc0Q7CQQ==",
	"MTL": "GOmex1ilEfEAAOYo81Crcg==",
	"NJD": "g1DIan8pEfEAAC4RS0Xe8Q==",
	"NSH": "imyy238pEfEAAC4RS0Xe8Q==",
	"NYI": "kgA+TH8pEfEAAC4RS0Xe8Q==",
	"NYR": "miMsfn8pEfEAADq3u4xYzw==",
	"OTT": "oLgaDH8pEfEAAOK+11eG0w==",
	"PHI": "yMEWan8pEfEAAJYcSnrw9A==",
	"PIT": "z8aSq38pEfEAAJYcSnrw9A==",
	"SEA": "1tEcRn8pEfEAAD7+DDlNqw==",
	"SJS": "3qTJ0X8pEfEAAOYc0Q7CQQ==",
	"STL": "5Qj3Bn8pEfEAACopEMQCsQ==",
	"TBL": "7Sjk538pEfEAAFr0zqCBeA==",
	"TOR": "8yE1938pEfEAACopEMQCsQ==",
	"UTA": "+NY7bn8pEfEAAC4RS0Xe8Q==",
	"VAN": "/1iF5H8pEfEAAOYc0Q7CQQ==",
	"VGK": "Y8GZXlikEfEAAE4quYgbSQ==",
	"WPG": "BdfcOX8qEfEAACopEMQCsQ==",
	"WSH": "DMyFbX8qEfEAAJYcSnrw9A==",
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
