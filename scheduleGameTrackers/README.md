# Schedule Game Trackers

This program fetches NHL game schedules and creates Google Cloud Tasks for game tracking. It's designed to work with the CrashTheCrease backend system to automatically schedule game monitoring tasks.

## Features

- **Automatic Game Detection**: Fetches games for today or a specified date using the NHL API
- **Team Filtering**: Supports filtering games by specific teams (defaults to Dallas Stars)
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
- `-teams ID1,ID2,ID3`: Comma-separated list of NHL team IDs to filter for
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

**Get games for specific teams on a future date**:
```bash
./schedulegametrackers -date 2024-03-15 -teams 25,1,24
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

## NHL Team IDs

Some common NHL team IDs:
- Dallas Stars: 25
- Boston Bruins: 1
- Toronto Maple Leafs: 10
- New York Rangers: 3
- Chicago Blackhawks: 16
- Detroit Red Wings: 17

For a complete list, refer to the NHL API documentation.

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
