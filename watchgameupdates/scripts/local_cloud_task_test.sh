# Run cloud task emulator locally
docker run --name cloudtasks-emulator --network net -p 8123:8123 ghcr.io/aertje/cloud-tasks-emulator:latest --host=0.0.0.0

# If it's already running, remove it and run again
docker rm cloudtasks-emulator

# Or start it again and attach to stdout
docker start -a cloudtasks-emulator

# Run game updates function locally
docker run -p 8080:8080 --name sendgameupdates --network net sendgameupdates

# Once all containers are running, make the requests using the local cloud tasks go code

# Curl command to trigger the deployed function with a completed game and max execution time of 12 minutes
curl -X POST https://send-game-updates-715155183718.us-south1.run.app -H "Content-Type: application/json" -d '{"game_id": "2024030411", "max_execution_time": "'"$(date -u -v+12M +%Y-%m-%dT%H:%M:%SZ)"'"}'

# Curl commands to get the game ID for today from the NHL API and inject it into a request to the deployed function, without a max execution time
local today
today=$(date -v-0d +%F)

curl -s -X GET "https://api-web.nhle.com/v1/score/$today" -o data/schedule_response.json

if command -v python3 > /dev/null; then
    python3 -m json.tool data/schedule_response.json > data/format_schedule_response.json
fi

local game_id
game_id=$(tr -d '\n' < data/schedule_response.json \
| sed 's/{"id"/\n{"id"/g' \
| grep "\"gameDate\":\"$today\"" \
| head -n1 \
| grep -o '"id":[0-9]*' \
| grep -o '[0-9]*')

echo "$game_id"

curl -X POST https://send-game-updates-715155183718.us-south1.run.app -H "Content-Type: application/json" -d '{"game_id": "2024030411"}'


# Full breakdown
## Make sure network is created
## Remove any existing emulator and handler containers
## create the 3 tmux sessions
## Session 1 (left side):
### - cd to test client directory
## Session 2 (center):
### - run the cloud tasks emulator
## Session 3 (right side):
### - Build the handler container
### - Run the handler container (including .env)