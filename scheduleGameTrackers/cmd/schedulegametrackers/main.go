// Package main implements a program that fetches NHL game schedules and creates
// Google Cloud Tasks for game tracking. It supports various command-line options
// for customizing team selection, date ranges, and task queue destinations.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	taskspb "google.golang.org/genproto/googleapis/cloud/tasks/v2"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/timestamppb"
)

const (
	// DefaultTeamID represents the Dallas Stars team ID in the NHL API
	DefaultTeamID = 25
	// TestGameID is a predefined game ID used in test mode
	TestGameID = "2023020001"
	// NHLAPIBaseURL is the base URL for NHL API endpoints
	NHLAPIBaseURL = "https://api-web.nhle.com/v1"
)

// Config holds the configuration for the application
type Config struct {
	Date       string // Date to query games for (YYYY-MM-DD format)
	Teams      []int  // Team IDs to filter games for
	TestMode   bool   // Whether to run in test mode
	AllTeams   bool   // Whether to include all teams
	Production bool   // Whether to use production task queue
	ProjectID  string // GCP Project ID
	Location   string // GCP Location
	QueueName  string // Task Queue name
}

// Game represents a single NHL game with relevant information
type Game struct {
	ID        int    `json:"id"`
	GameDate  string `json:"gameDate"`
	StartTime string `json:"startTimeUTC"`
	AwayTeam  struct {
		ID int `json:"id"`
	} `json:"awayTeam"`
	HomeTeam struct {
		ID int `json:"id"`
	} `json:"homeTeam"`
}

// ScheduleResponse represents the NHL API schedule response
type ScheduleResponse struct {
	GameWeek []struct {
		Date  string `json:"date"`
		Games []Game `json:"games"`
	} `json:"gameWeek"`
}

// TaskPayload represents the payload structure for cloud tasks, matching existing system
type TaskPayload struct {
	GameID       string  `json:"game_id"`
	ExecutionEnd *string `json:"execution_end,omitempty"`
}

// parseFlags parses and validates command-line flags
func parseFlags() *Config {
	config := &Config{}

	var teamsStr string
	flag.StringVar(&config.Date, "date", "", "Specific date to query (YYYY-MM-DD format). Defaults to today.")
	flag.StringVar(&teamsStr, "teams", "", "Comma-separated list of team IDs. Defaults to Dallas Stars (25).")
	flag.BoolVar(&config.TestMode, "test", false, "Run in test mode with predefined game ID")
	flag.BoolVar(&config.AllTeams, "all", false, "Include all teams playing on the specified date")
	flag.BoolVar(&config.Production, "prod", false, "Send tasks to production queue instead of local emulator")
	flag.StringVar(&config.ProjectID, "project", "localproject", "GCP Project ID")
	flag.StringVar(&config.Location, "location", "us-south1", "GCP Location")
	flag.StringVar(&config.QueueName, "queue", "gameschedule", "Task Queue name")

	flag.Parse()

	// Set default date to today if not provided
	if config.Date == "" {
		config.Date = time.Now().Format("2006-01-02")
	}

	// Parse team IDs
	if config.AllTeams {
		config.Teams = []int{} // Empty slice means all teams
	} else if teamsStr != "" {
		teamStrs := strings.Split(teamsStr, ",")
		config.Teams = make([]int, len(teamStrs))
		for i, teamStr := range teamStrs {
			teamID, err := strconv.Atoi(strings.TrimSpace(teamStr))
			if err != nil {
				log.Fatalf("Invalid team ID: %s", teamStr)
			}
			config.Teams[i] = teamID
		}
	} else {
		config.Teams = []int{DefaultTeamID} // Default to Dallas Stars
	}

	return config
}

// fetchGamesForDate retrieves games for a specific date from the NHL API
func fetchGamesForDate(date string) ([]Game, error) {
	url := fmt.Sprintf("%s/schedule/%s", NHLAPIBaseURL, date)

	log.Printf("Fetching games from NHL API: %s", url)

	resp, err := http.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch schedule: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("NHL API returned status: %d", resp.StatusCode)
	}

	var schedule ScheduleResponse
	if err := json.NewDecoder(resp.Body).Decode(&schedule); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	var games []Game
	for _, week := range schedule.GameWeek {
		for _, game := range week.Games {
			games = append(games, game)
		}
	}

	log.Printf("Found %d games for date %s", len(games), date)
	return games, nil
}

// filterGamesForTeams filters games to include only those involving specified teams
func filterGamesForTeams(games []Game, teams []int) []Game {
	if len(teams) == 0 {
		// Return all games if no specific teams are specified
		return games
	}

	teamMap := make(map[int]bool)
	for _, teamID := range teams {
		teamMap[teamID] = true
	}

	var filteredGames []Game
	for _, game := range games {
		if teamMap[game.HomeTeam.ID] || teamMap[game.AwayTeam.ID] {
			filteredGames = append(filteredGames, game)
		}
	}

	log.Printf("Filtered to %d games involving specified teams", len(filteredGames))
	return filteredGames
}

// createTestGame creates a test game with predefined data for testing purposes
func createTestGame() Game {
	return Game{
		ID:        20242025,
		GameDate:  time.Now().Format("2006-01-02"),
		StartTime: time.Now().Format(time.RFC3339),
		AwayTeam: struct {
			ID int `json:"id"`
		}{ID: DefaultTeamID},
		HomeTeam: struct {
			ID int `json:"id"`
		}{ID: 1}, // Boston Bruins
	}
}

// createQueue creates a task queue if it doesn't exist
func createQueue(client taskspb.CloudTasksClient, ctx context.Context, config *Config) error {
	// projects/localproject/locations/us-south1/queues/gameschedule
	queuePath := fmt.Sprintf("projects/%s/locations/%s/queues/%s", config.ProjectID, config.Location, config.QueueName)
	parentPath := fmt.Sprintf("projects/%s/locations/%s", config.ProjectID, config.Location)

	req := &taskspb.CreateQueueRequest{
		Parent: parentPath,
		Queue: &taskspb.Queue{
			Name: queuePath,
		},
	}
	_, err := client.CreateQueue(ctx, req)
	if err != nil {
		if strings.Contains(err.Error(), "already exists") || strings.Contains(err.Error(), "AlreadyExists") {
			log.Printf("Queue %s already exists, skipping creation", config.QueueName)
			return nil
		}
		return fmt.Errorf("failed to create queue: %w", err)
	}
	log.Printf("Created queue: %s", queuePath)
	return nil
}

// createCloudTask creates a Google Cloud Task for a given game using direct GRPC
func createCloudTask(ctx context.Context, client taskspb.CloudTasksClient, config *Config, game Game) error {
	// Create execution end time (game start time + 4 hours for typical game duration)
	startTime, err := time.Parse(time.RFC3339, game.StartTime)
	if err != nil {
		return fmt.Errorf("failed to parse start time: %w", err)
	}

	executionEnd := startTime.Add(4 * time.Hour).Format(time.RFC3339)

	// Prepare the task payload matching existing system structure
	payload := TaskPayload{
		GameID:       strconv.Itoa(game.ID),
		ExecutionEnd: &executionEnd,
	}

	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal payload: %w", err)
	}

	// Determine the target URL based on production flag
	var targetURL string
	if config.Production {
		targetURL = fmt.Sprintf("https://%s-%s.cloudfunctions.net/watchGameUpdates", config.ProjectID, config.Location)
	} else {
		targetURL = "http://host.docker.internal:8080"
	}

	// Schedule task to run 5 minutes before game start
	scheduleTime := startTime.Add(-5 * time.Minute)

	// Create the task request using taskspb format (works for emulator)
	queuePath := fmt.Sprintf("projects/%s/locations/%s/queues/%s", config.ProjectID, config.Location, config.QueueName)

	req := &taskspb.CreateTaskRequest{
		Parent: queuePath,
		Task: &taskspb.Task{
			MessageType: &taskspb.Task_HttpRequest{
				HttpRequest: &taskspb.HttpRequest{
					HttpMethod: taskspb.HttpMethod_POST,
					Url:        targetURL,
					Headers: map[string]string{
						"Content-Type": "application/json",
					},
					Body: payloadJSON,
				},
			},
			ScheduleTime: timestamppb.New(scheduleTime),
		},
	}

	// Create the task
	task, err := client.CreateTask(ctx, req)
	if err != nil {
		return fmt.Errorf("failed to create task: %w", err)
	}

	log.Printf("Created task %s for game %d, scheduled for %s", task.Name, game.ID, scheduleTime.Format(time.RFC3339))
	return nil
}

// connectToTasksService connects to Cloud Tasks service (emulator or production)
func connectToTasksService(ctx context.Context, config *Config) (taskspb.CloudTasksClient, *grpc.ClientConn, error) {
	if !config.Production {
		// Connect to local emulator using direct GRPC (like localCloudTasksTest)
		endpoint := "localhost:8123"
		log.Printf("Connecting to local Cloud Tasks emulator at %s", endpoint)

		conn, err := grpc.DialContext(ctx, endpoint, grpc.WithInsecure(), grpc.WithBlock())
		if err != nil {
			return nil, nil, fmt.Errorf("failed to connect to local Cloud Tasks emulator at %s - ensure the emulator is running: %w", endpoint, err)
		}

		client := taskspb.NewCloudTasksClient(conn)
		return client, conn, nil
	} else {
		// For production mode, we would need to implement the official client approach
		// This is a placeholder - in practice you'd use the official Cloud Tasks client
		return nil, nil, fmt.Errorf("production mode not implemented in this version")
	}
}

// processGames processes a list of games and creates cloud tasks for each
func processGames(ctx context.Context, client taskspb.CloudTasksClient, config *Config, games []Game) error {
	if len(games) == 0 {
		log.Println("No games found to process")
		return nil
	}

	// Create queue if it doesn't exist
	if err := createQueue(client, ctx, config); err != nil {
		log.Printf("Warning: Failed to create queue: %v", err)
	}

	log.Printf("Processing %d games", len(games))

	for _, game := range games {
		log.Printf("Processing game %d: %s", game.ID, game.StartTime)

		if err := createCloudTask(ctx, client, config, game); err != nil {
			log.Printf("Failed to create task for game %d: %v", game.ID, err)
			continue
		}
	}

	return nil
}

// main is the entry point of the application
func main() {
	// Parse command-line flags
	config := parseFlags()

	log.Printf("Starting NHL Game Tracker Scheduler")
	log.Printf("Configuration: Date=%s, Teams=%v, TestMode=%t, AllTeams=%t, Production=%t",
		config.Date, config.Teams, config.TestMode, config.AllTeams, config.Production)

	ctx := context.Background()

	// Connect to Cloud Tasks service (emulator or production)
	client, conn, err := connectToTasksService(ctx, config)
	if err != nil {
		log.Fatalf("Failed to connect to tasks service: %v", err)
	}
	defer conn.Close()

	var games []Game

	if config.TestMode {
		log.Println("Running in test mode with predefined game data")
		games = []Game{createTestGame()}
	} else {
		// Fetch games from NHL API
		fetchedGames, err := fetchGamesForDate(config.Date)
		if err != nil {
			log.Fatalf("Failed to fetch games: %v", err)
		}

		// Filter games based on team selection
		games = filterGamesForTeams(fetchedGames, config.Teams)
	}

	// Process games and create tasks
	if err := processGames(ctx, client, config, games); err != nil {
		log.Fatalf("Failed to process games: %v", err)
	}

	log.Printf("Successfully processed %d games", len(games))
}
