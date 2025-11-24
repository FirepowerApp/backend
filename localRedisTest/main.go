package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"time"

	"github.com/hibiken/asynq"
)

type Payload struct {
	Game         Game    `json:"game"`
	ExecutionEnd *string `json:"execution_end,omitempty"`
}

type Game struct {
	ID        string `json:"id"`
	GameDate  string `json:"gameDate"`
	StartTime string `json:"startTimeUTC"`
	HomeTeam  Team   `json:"homeTeam"`
	AwayTeam  Team   `json:"awayTeam"`
}

type Team struct {
	ID                       int               `json:"id"`
	CommonName               map[string]string `json:"commonName"`
	PlaceName                map[string]string `json:"placeName"`
	PlaceNameWithPreposition map[string]string `json:"placeNameWithPreposition"`
	Abbrev                   string            `json:"abbrev"`
}

func main() {
	// Command line flags
	gameID := flag.String("game", "2024030411", "NHL Game ID")
	redisAddr := flag.String("redis", "localhost:6379", "Redis address")
	duration := flag.Duration("duration", 12*time.Minute, "Execution duration")
	immediate := flag.Bool("now", false, "Schedule task immediately instead of delaying")
	flag.Parse()

	// Create Asynq client
	client := asynq.NewClient(asynq.RedisClientOpt{
		Addr: *redisAddr,
	})
	defer client.Close()

	// Calculate execution end time
	executionEnd := time.Now().Add(*duration).Format(time.RFC3339)

	// Create task payload
	payload := Payload{
		Game: Game{
			ID:        *gameID,
			GameDate:  "2024-06-13",
			StartTime: "2024-06-14T00:00:00Z",
			HomeTeam: Team{
				ID: 13,
				CommonName: map[string]string{
					"default": "Panthers",
				},
				PlaceName: map[string]string{
					"default": "Florida",
				},
				PlaceNameWithPreposition: map[string]string{
					"default": "at Florida",
				},
				Abbrev: "FLA",
			},
			AwayTeam: Team{
				ID: 14,
				CommonName: map[string]string{
					"default": "Oilers",
				},
				PlaceName: map[string]string{
					"default": "Edmonton",
				},
				PlaceNameWithPreposition: map[string]string{
					"default": "at Edmonton",
				},
				Abbrev: "EDM",
			},
		},
		ExecutionEnd: &executionEnd,
	}

	// Marshal payload to JSON
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		log.Fatalf("Failed to marshal payload: %v", err)
	}

	// Create Asynq task
	task := asynq.NewTask("game:check", payloadBytes)

	// Enqueue options
	var opts []asynq.Option
	if *immediate {
		// Immediate execution
		opts = append(opts,
			asynq.Queue("critical"), // High priority queue
			asynq.MaxRetry(3),
			asynq.Timeout(5*time.Minute),
		)
	} else {
		// Delayed execution (10 seconds from now for testing)
		opts = append(opts,
			asynq.ProcessIn(10*time.Second),
			asynq.Queue("default"),
			asynq.MaxRetry(3),
			asynq.Timeout(5*time.Minute),
		)
	}

	// Enqueue the task
	info, err := client.Enqueue(task, opts...)
	if err != nil {
		log.Fatalf("Failed to enqueue task: %v", err)
	}

	fmt.Printf("✓ Task enqueued successfully!\n")
	fmt.Printf("  Task ID:        %s\n", info.ID)
	fmt.Printf("  Queue:          %s\n", info.Queue)
	fmt.Printf("  Game ID:        %s\n", *gameID)
	fmt.Printf("  Execution End:  %s\n", executionEnd)
	if *immediate {
		fmt.Printf("  Schedule:       Immediate\n")
	} else {
		fmt.Printf("  Schedule:       10 seconds from now\n")
	}
	fmt.Printf("\nThe game monitoring will run for %v\n", *duration)
	fmt.Printf("Monitor logs with: docker compose logs -f backend\n")
}
