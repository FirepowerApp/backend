# Schedule Game Trackers

This program fetches NHL game schedules and creates Google Cloud Tasks for game tracking. It's designed to work with the CrashTheCrease backend system to automatically schedule game monitoring tasks.

## Features

- **Automatic Game Detection**: Fetches games for today or a specified date using the NHL API
- **Team Filtering**: Supports filtering games by specific teams using city codes (CHI, DAL) or numeric IDs (defaults to Dallas Stars)
- **Today's Upcoming Games**: New `-today` flag filters to only today's games that haven't started yet
- **Flexible Scheduling**: Can schedule tasks for future dates
- **Test Mode**: Includes a test mode with predefined game data for development
- **Production Support**: Configurable for both local development and production environments
- **Cloud Task Integration**: Creates Google Cloud Tasks that integrate with the existing game monitoring system

## Usage

### Basic Usage

Run with default settings (Dallas Stars games for today, local task queue):
```bash
./schedulegametrackers
```

### Command Line Options

- `-date YYYY-MM-DD`: Specify a future date to query (default: today)
- `-teams ID1,ID2,ID3`: Comma-separated list of NHL team IDs or city codes to filter for (supports both formats)
- `-today`: Filter for today's upcoming games only (overrides -date)
- `-all`: Include all teams playing on the specified date
- `-test`: Run in test mode with predefined game data
- `-prod`: Send tasks to production queue instead of local emulator
- `-project PROJECT_ID`: GCP Project ID (default: "crash-the-crease")
- `-location LOCATION`: GCP Location (default: "us-central1")
- `-queue QUEUE_NAME`: Task Queue name (default: "game-updates")

### Examples

**Get Dallas Stars games for today (default)**:
```bash
./schedulegametrackers
```

**Get today's upcoming games for Chicago Blackhawks**:
```bash
./schedulegametrackers -today -teams CHI
```

**Get today's upcoming games for multiple teams using city codes**:
```bash
./schedulegametrackers -today -teams CHI,DAL,BOS
```

**Get games for specific teams on a future date (mixing city codes and IDs)**:
```bash
./schedulegametrackers -date 2024-03-15 -teams CHI,25,1
```

**Get all games for tomorrow**:
```bash
./schedulegametrackers -date 2024-03-16 -all
```

**Run in test mode**:
```bash
./schedulegametrackers -test
```

**Send tasks to production**:
```bash
./schedulegametrackers -prod -date 2024-03-20
```

## NHL Team IDs and City Codes

The program now supports both numeric team IDs and city codes for team filtering:

### Common Teams (City Code - Team ID - Team Name):
- **CHI - 16** - Chicago Blackhawks
- **DAL - 25** - Dallas Stars
- **BOS - 1** - Boston Bruins
- **TOR - 10** - Toronto Maple Leafs
- **NYR - 3** - New York Rangers
- **DET - 17** - Detroit Red Wings

### All Supported City Codes:
- **ANA** (24) - Anaheim Ducks
- **ARI** (53) - Arizona Coyotes
- **BOS** (1) - Boston Bruins
- **BUF** (7) - Buffalo Sabres
- **CAR** (12) - Carolina Hurricanes
- **CBJ** (29) - Columbus Blue Jackets
- **CGY** (20) - Calgary Flames
- **CHI** (16) - Chicago Blackhawks
- **COL** (21) - Colorado Avalanche
- **DAL** (25) - Dallas Stars
- **DET** (17) - Detroit Red Wings
- **EDM** (22) - Edmonton Oilers
- **FLA** (13) - Florida Panthers
- **LAK** (26) - Los Angeles Kings
- **MIN** (30) - Minnesota Wild
- **MTL** (8) - Montreal Canadiens
- **NJD** (6) - New Jersey Devils
- **NSH** (18) - Nashville Predators
- **NYI** (2) - New York Islanders
- **NYR** (3) - New York Rangers
- **OTT** (9) - Ottawa Senators
- **PHI** (4) - Philadelphia Flyers
- **PIT** (5) - Pittsburgh Penguins
- **SEA** (55) - Seattle Kraken
- **SJS** (28) - San Jose Sharks
- **STL** (19) - St. Louis Blues
- **TBL** (14) - Tampa Bay Lightning
- **TOR** (10) - Toronto Maple Leafs
- **VAN** (23) - Vancouver Canucks
- **VGK** (54) - Vegas Golden Knights
- **WPG** (52) - Winnipeg Jets
- **WSH** (15) - Washington Capitals

You can use either format: `-teams CHI,DAL` or `-teams 16,25` or mix them: `-teams CHI,25,BOS`

## Task Scheduling

The program schedules Google Cloud Tasks to run 30 minutes before each game's start time. Each task contains:
- Game ID
- Game execution end time (game start + 4 hours)

These tasks are consumed by the existing `watchGameUpdates` service in the CrashTheCrease backend.

## Development

### Building

Use the project's build system:
```bash
go run build.go -target schedulegametrackers
```

### Dependencies

The program requires:
- Go 1.21+
- Google Cloud Tasks API access
- Internet connectivity for NHL API access

### Testing

Test mode can be used for development without making actual API calls:
```bash
./schedulegametrackers -test
```

### Local Development

For local development, ensure the local Cloud Tasks emulator is running and use the default settings (without `-prod` flag).

## Configuration

### Environment Variables

While the program primarily uses command-line flags, you may need to set up Google Cloud credentials:

```bash
export GOOGLE_APPLICATION_CREDENTIALS="path/to/service-account-key.json"
```

### Production Configuration

When using `-prod` flag, ensure:
1. GCP credentials are properly configured
2. The target Cloud Function is deployed
3. The task queue exists in the specified project/location

## Error Handling

The program includes comprehensive error handling for:
- NHL API connectivity issues
- Invalid team IDs
- Google Cloud Tasks creation failures
- Date parsing errors

Failed task creations are logged but don't stop processing of other games.

## Integration

This program integrates with:
- **NHL API**: For fetching game schedules
- **Google Cloud Tasks**: For task scheduling
- **CrashTheCrease Backend**: Via the `watchGameUpdates` handler

## Troubleshooting

### Common Issues

1. **NHL API Errors**: Check internet connectivity and try again
2. **Cloud Tasks Errors**: Verify GCP credentials and project configuration
3. **Invalid Team IDs**: Refer to NHL API documentation for correct team IDs
4. **Date Format Errors**: Use YYYY-MM-DD format for dates

### Logging

The program provides detailed logging of:
- Configuration settings
- API requests and responses
- Task creation results
- Error conditions

## Future Enhancements

Potential improvements:
- Support for playoff schedules
- Retry mechanisms for failed API calls
- Configuration file support
- Multiple date range support
- Team name to ID resolution
