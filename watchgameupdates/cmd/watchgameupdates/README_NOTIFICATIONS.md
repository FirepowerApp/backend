# Discord Notification System

A flexible notification system for Go applications that provides a clean interface for sending notifications to various platforms. Currently implements Discord bot notifications with support for future platforms like Apple Push Notifications.

## Architecture

The notification system is designed with a clean separation of concerns:

- **Interface Layer**: `Notifier` interface abstracts notification implementation details
- **Implementation Layer**: Platform-specific implementations (Discord, future APNS)
- **Configuration Layer**: Environment-based configuration management
- **Application Layer**: Simple integration into existing code

## Files Overview

| File | Purpose |
|------|---------|
| `notification.go` | Core interface definitions and types |
| `discord_notifier.go` | Discord bot implementation |
| `notification_example.go` | Usage examples and integration patterns |

## Core Interface

The `Notifier` interface provides:

```go
type Notifier interface {
    // Send single notification with async confirmation
    SendNotification(ctx context.Context, req NotificationRequest) (<-chan NotificationResult, error)

    // Send batch notifications
    SendBatch(ctx context.Context, batch NotificationBatch) (<-chan NotificationResult, error)

    // Clean shutdown
    Close() error
}
```

### Key Features

- **Asynchronous Delivery**: All notifications return immediately with async confirmation via channels
- **Batch Support**: Send multiple notifications efficiently
- **Error Handling**: Structured error reporting with detailed feedback
- **Context Support**: Full context cancellation and timeout support
- **Clean Shutdown**: Proper resource cleanup

## Discord Bot Setup

### 1. Create Discord Bot

1. Go to [Discord Developer Portal](https://discord.com/developers/applications)
2. Click "New Application" and give it a name
3. Go to the "Bot" section in the sidebar
4. Click "Add Bot"
5. Copy the bot token (keep this secure!)

### 2. Configure Bot Permissions

In the Bot section, grant these permissions:
- Send Messages
- Read Message History
- Use Slash Commands (optional)

### 3. Add Bot to Your Server

1. Go to the "OAuth2" > "URL Generator" section
2. Select "bot" scope
3. Select required permissions (Send Messages, Read Message History)
4. Copy the generated URL and open it in your browser
5. Select your Discord server and authorize the bot

### 4. Get Channel ID

1. Enable Developer Mode in Discord (User Settings > Advanced > Developer Mode)
2. Right-click on the channel where you want notifications
3. Click "Copy ID"
4. Update the hardcoded channel ID in `discord_notifier.go`:

```go
// Update this line with your actual channel ID
channelID := "1234567890123456789"
```

### 5. Configure Environment

Update your `.env` file:

```env
# Replace with your actual bot token
DISCORD_BOT_TOKEN=your_actual_bot_token_here
```

## Usage Examples

### Basic Single Notification

```go
// Load configuration
config, err := LoadDiscordConfigFromEnv()
if err != nil {
    log.Fatalf("Failed to load config: %v", err)
}

// Create notifier
notifier, err := NewDiscordNotifier(config)
if err != nil {
    log.Fatalf("Failed to create notifier: %v", err)
}
defer notifier.Close()

// Send notification
ctx := context.Background()
req := NotificationRequest{
    Team1ID: "BOS",
    Team2ID: "MTL",
    Data: map[string]float64{
        "goals_team1": 2,
        "goals_team2": 1,
        "period": 2,
    },
}

resultChan, err := notifier.SendNotification(ctx, req)
if err != nil {
    log.Printf("Failed to send: %v", err)
    return
}

// Handle result asynchronously
go func() {
    result := <-resultChan
    if result.Success {
        log.Printf("Sent successfully: %s", result.ID)
    } else {
        log.Printf("Failed: %v", result.Error)
    }
}()
```

### Batch Notifications

```go
batch := NotificationBatch{
    Requests: []NotificationRequest{
        {
            Team1ID: "TOR",
            Team2ID: "OTT",
            Data: map[string]float64{"goals_team1": 3, "goals_team2": 2},
        },
        {
            Team1ID: "NYR",
            Team2ID: "NYI",
            Data: map[string]float64{"goals_team1": 1, "goals_team2": 0},
        },
    },
}

resultChan, err := notifier.SendBatch(ctx, batch)
if err != nil {
    log.Printf("Batch failed: %v", err)
    return
}

// Process results as they arrive
for result := range resultChan {
    if result.Success {
        log.Printf("Batch item sent: %s", result.ID)
    } else {
        log.Printf("Batch item failed: %v", result.Error)
    }
}
```

### Integration with Existing Handler

```go
func YourExistingHandler(w http.ResponseWriter, r *http.Request) {
    // Your existing logic...

    // Initialize notifier (do this once, reuse the instance)
    config, _ := LoadDiscordConfigFromEnv()
    notifier, _ := NewDiscordNotifier(config)
    defer notifier.Close()

    // When a game event occurs, send notification
    notification := NotificationRequest{
        Team1ID: homeTeam,
        Team2ID: awayTeam,
        Data: gameStats,
    }

    ctx := context.WithTimeout(context.Background(), 10*time.Second)
    resultChan, err := notifier.SendNotification(ctx, notification)
    if err != nil {
        log.Printf("Notification error: %v", err)
        // Continue with your handler logic
        return
    }

    // Handle result asynchronously to avoid blocking
    go func() {
        select {
        case result := <-resultChan:
            if !result.Success {
                log.Printf("Notification failed: %v", result.Error)
            }
        case <-ctx.Done():
            log.Printf("Notification timeout")
        }
    }()

    // Your existing response logic...
}
```

## Message Format

Discord messages are automatically formatted with:

- **Header**: "🏒 **Game Update: TEAM1 vs TEAM2**"
- **Statistics Section**: Each data key-value pair formatted as bullet points
- **Footer**: Timestamp of when notification was sent

Example output:
```
🏒 **Game Update: BOS vs MTL**

📊 **Game Statistics:**
• **Goals Team1:** 2
• **Goals Team2:** 1
• **Shots On Goal Team1:** 15
• **Power Play Goals:** 1
• **Face Off Win Pct:** 58.50

*Notification sent at 14:23:15 MST*
```

## Error Handling

The system provides structured error handling:

```go
type NotificationResult struct {
    ID        string    // Unique notification identifier
    Success   bool      // Whether notification succeeded
    Error     error     // Error details if failed
    Timestamp time.Time // When processed
}
```

Common error scenarios:
- Invalid bot token
- Bot not in target server
- Missing channel permissions
- Network connectivity issues
- Rate limiting

## Performance Considerations

- **Rate Limiting**: Built-in 100ms delay between batch messages
- **Connection Reuse**: Discord connection is reused across notifications
- **Async Processing**: Non-blocking notification delivery
- **Context Timeout**: Configurable timeouts prevent hanging

## Future Platform Support

The interface is designed to easily support additional platforms:

```go
// Future: Apple Push Notifications
apnsNotifier, err := NewAPNSNotifier(apnsConfig)

// Future: Slack notifications
slackNotifier, err := NewSlackNotifier(slackConfig)

// Same interface, different implementation
resultChan, err := apnsNotifier.SendNotification(ctx, req)
```

## Security Notes

- Keep bot tokens secure - never commit them to version control
- Use environment variables for all sensitive configuration
- Bot has minimal required permissions
- Consider IP restrictions for production deployments

## Troubleshooting

### Bot Not Sending Messages
1. Verify bot token is correct
2. Check bot is in target server
3. Confirm channel ID is correct
4. Verify bot has "Send Messages" permission in the channel

### Permission Errors
- Ensure bot role has necessary permissions
- Check channel-specific permission overrides
- Verify bot role is above any restrictive roles

### Connection Issues
- Check network connectivity
- Verify no firewall blocking Discord API
- Consider proxy configuration if needed

## Testing

Run the example functions to test your setup:
```go
// Test basic functionality
ExampleNotificationUsage()

// Test integration patterns
IntegrateWithExistingHandler()
